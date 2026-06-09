package cmd

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os/exec"
	"strings"
)

const defaultCommandOutputLimit = 12288

// commandRunConfig 描述一次本地命令执行。
type commandRunConfig struct {
	Dir     string
	Command string
	Stdin   string
	Limit   int
}

// commandRunResult 描述本地命令执行结果。
type commandRunResult struct {
	Stdout   string
	Stderr   string
	ExitCode int
	Err      error
}

// commandRunner 抽象本地命令执行，便于测试替换 agent 和 test 命令。
type commandRunner interface {
	Run(ctx context.Context, cfg commandRunConfig) commandRunResult
}

type commandRunnerFunc func(ctx context.Context, cfg commandRunConfig) commandRunResult

func (f commandRunnerFunc) Run(ctx context.Context, cfg commandRunConfig) commandRunResult {
	return f(ctx, cfg)
}

// shellCommandRunner 通过 sh -c 执行用户配置的命令。
type shellCommandRunner struct{}

func (shellCommandRunner) Run(ctx context.Context, cfg commandRunConfig) commandRunResult {
	limit := cfg.Limit
	if limit <= 0 {
		limit = defaultCommandOutputLimit
	}
	cmd := exec.CommandContext(ctx, "sh", "-c", cfg.Command)
	cmd.Dir = cfg.Dir
	cmd.Stdin = strings.NewReader(cfg.Stdin)
	stdout := newBoundedOutput(limit)
	stderr := newBoundedOutput(limit)
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	err := cmd.Run()
	exitCode := 0
	if err != nil {
		exitCode = 1
		if ee, ok := err.(*exec.ExitError); ok {
			exitCode = ee.ExitCode()
		}
	}
	return commandRunResult{
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
		ExitCode: exitCode,
		Err:      err,
	}
}

type boundedOutput struct {
	buf       bytes.Buffer
	limit     int
	truncated bool
}

func newBoundedOutput(limit int) *boundedOutput {
	return &boundedOutput{limit: limit}
}

func (w *boundedOutput) Write(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	if w.buf.Len() < w.limit {
		remaining := w.limit - w.buf.Len()
		if len(p) <= remaining {
			_, _ = w.buf.Write(p)
			return len(p), nil
		}
		_, _ = w.buf.Write(p[:remaining])
	}
	w.truncated = true
	return len(p), nil
}

func (w *boundedOutput) String() string {
	if w.truncated {
		return w.buf.String() + "\n...[truncated]"
	}
	return w.buf.String()
}

var _ io.Writer = (*boundedOutput)(nil)

func truncateOutput(s string, limit int) string {
	if limit <= 0 || len(s) <= limit {
		return s
	}
	return s[:limit] + "\n...[truncated]"
}

func gitWorkingTreeDirty(ctx context.Context, runner commandRunner, repo string, limit int) (bool, string, error) {
	if limit <= 0 {
		limit = defaultCommandOutputLimit
	}
	res := runner.Run(ctx, commandRunConfig{
		Dir:     repo,
		Command: "git status --porcelain",
		Limit:   limit,
	})
	if res.Err != nil {
		return false, commandFailureDetail(res), res.Err
	}
	return strings.TrimSpace(res.Stdout) != "", res.Stdout, nil
}

func commandFailureDetail(res commandRunResult) string {
	var b strings.Builder
	if res.Err != nil {
		fmt.Fprintf(&b, "Error: %s\n", res.Err)
	}
	fmt.Fprintf(&b, "Exit code: %d", res.ExitCode)
	if res.Stdout != "" {
		fmt.Fprintf(&b, "\nStdout:\n%s", res.Stdout)
	}
	if res.Stderr != "" {
		fmt.Fprintf(&b, "\nStderr:\n%s", res.Stderr)
	}
	return b.String()
}

// bugFixBugDetail 是 agent prompt 所需的缺陷详情快照。
type bugFixBugDetail struct {
	WorkspaceID  string
	ID           string
	StoryID      string
	Title        string
	Status       string
	CurrentOwner string
	Severity     string
	Priority     string
	Description  string
	Comments     []bugFixComment
}

// bugFixStoryDetail 是缺陷关联需求中用于定位 MR 的最小详情。
type bugFixStoryDetail struct {
	WorkspaceID string
	ID          string
	ParentID    string
	Title       string
	Description string
	Comments    []bugFixComment
}

// bugFixComment 是 agent prompt 所需的评论快照。
type bugFixComment struct {
	Author      string
	Created     string
	Description string
}

func buildAgentPrompt(bug bugFixBugDetail, repo, testCmd string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "You are fixing TAPD bug %s in workspace %s.\n\n", bug.ID, bug.WorkspaceID)
	fmt.Fprintf(&b, "Repository: %s\n", repo)
	fmt.Fprintf(&b, "Verification command: %s\n\n", testCmd)
	fmt.Fprintf(&b, "Title: %s\nStatus: %s\nOwner: %s\nSeverity: %s\nPriority: %s\n\n",
		bug.Title, bug.Status, bug.CurrentOwner, bug.Severity, bug.Priority)
	fmt.Fprintf(&b, "Description:\n%s\n\n", bug.Description)
	if len(bug.Comments) > 0 {
		b.WriteString("Recent comments:\n")
		for _, c := range bug.Comments {
			fmt.Fprintf(&b, "- %s %s: %s\n", c.Author, c.Created, c.Description)
		}
		b.WriteString("\n")
	}
	b.WriteString("Constraints:\n")
	b.WriteString("- Modify only code needed for this bug.\n")
	b.WriteString("- Preserve unrelated user changes.\n")
	b.WriteString("- Do not commit, push, create merge requests, deploy, or merge.\n")
	b.WriteString("- Run the requested verification command when possible.\n")
	b.WriteString("- Return a concise summary of files changed and verification results.\n")
	return b.String()
}

func buildSuccessComment(agent commandRunResult, test commandRunResult, verified bool, branch *bugFixBranchContext) string {
	branchText := ""
	if branch != nil {
		branchText = fmt.Sprintf("\n\nMR: %s\nBranch: %s\nMR source: %s", branch.MRURL, branch.LocalBranch, branch.Source)
	}
	return fmt.Sprintf("AI agent run completed.\n\nVerified: %v%s\n\nAgent stdout:\n%s\n\nAgent stderr:\n%s\n\nTest stdout:\n%s\n\nTest stderr:\n%s",
		verified, branchText, agent.Stdout, agent.Stderr, test.Stdout, test.Stderr)
}

func buildFailureComment(stage string, detail string) string {
	return fmt.Sprintf("AI agent bug fix failed.\n\nStage: %s\n\nDetail:\n%s", stage, detail)
}
