package cmd

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
)

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
	cmd := exec.CommandContext(ctx, "sh", "-c", cfg.Command)
	cmd.Dir = cfg.Dir
	cmd.Stdin = strings.NewReader(cfg.Stdin)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	exitCode := 0
	if err != nil {
		exitCode = 1
		if ee, ok := err.(*exec.ExitError); ok {
			exitCode = ee.ExitCode()
		}
	}
	limit := cfg.Limit
	if limit <= 0 {
		limit = 12288
	}
	return commandRunResult{
		Stdout:   truncateOutput(stdout.String(), limit),
		Stderr:   truncateOutput(stderr.String(), limit),
		ExitCode: exitCode,
		Err:      err,
	}
}

func truncateOutput(s string, limit int) string {
	if limit <= 0 || len(s) <= limit {
		return s
	}
	return s[:limit] + "\n...[truncated]"
}

func gitWorkingTreeDirty(ctx context.Context, runner commandRunner, repo string) (bool, string, error) {
	res := runner.Run(ctx, commandRunConfig{
		Dir:     repo,
		Command: "git status --porcelain",
		Limit:   12288,
	})
	if res.Err != nil {
		return false, strings.TrimSpace(res.Stderr), res.Err
	}
	out := strings.TrimSpace(res.Stdout)
	return out != "", out, nil
}

// bugFixBugDetail 是 agent prompt 所需的缺陷详情快照。
type bugFixBugDetail struct {
	WorkspaceID  string
	ID           string
	Title        string
	Status       string
	CurrentOwner string
	Severity     string
	Priority     string
	Description  string
	Comments     []bugFixComment
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

func buildSuccessComment(agent commandRunResult, test commandRunResult, verified bool) string {
	return fmt.Sprintf("AI agent finished bug fix.\n\nVerified: %v\n\nAgent stdout:\n%s\n\nAgent stderr:\n%s\n\nTest stdout:\n%s\n\nTest stderr:\n%s",
		verified, agent.Stdout, agent.Stderr, test.Stdout, test.Stderr)
}

func buildFailureComment(stage string, detail string) string {
	return fmt.Sprintf("AI agent bug fix failed.\n\nStage: %s\n\nDetail:\n%s", stage, detail)
}
