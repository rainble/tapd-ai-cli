# TAPD Agent Bug Fix Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build `tapd agent fix-bugs`, a local worker that listens for TAPD bug webhook events, invokes a configured coding agent against a local repository, verifies with a configured test command, comments on the TAPD bug, and optionally transitions bug status.

**Architecture:** Keep `interface` as a webhook relay only. Add focused orchestration code under `tapd-ai-cli/internal/cmd`: event parsing, repo/command execution helpers, TAPD writeback pipeline, and Cobra command integration. Tests use pure helper functions, fake TAPD clients, fake command runners, and httptest SSE servers.

**Tech Stack:** Go 1.24+, Cobra, `github.com/studyzy/tapd-sdk-go`, standard `os/exec`, existing `tapd watch` SSE and config patterns.

---

## File Structure

- Create `internal/cmd/agent_fix_bugs_event.go`: parse `streamEvent` payloads, match bug event names, extract workspace ID and bug ID.
- Create `internal/cmd/agent_fix_bugs_event_test.go`: event parsing unit tests.
- Create `internal/cmd/agent_fix_bugs_exec.go`: command runner, git dirty guard, output truncation, prompt and TAPD comment rendering.
- Create `internal/cmd/agent_fix_bugs_exec_test.go`: command runner, dirty guard, prompt, and truncation tests.
- Create `internal/cmd/agent_fix_bugs_runner.go`: one-bug orchestration with dependency interfaces and result records.
- Create `internal/cmd/agent_fix_bugs_runner_test.go`: fake TAPD client and fake runner tests for success/failure paths.
- Create `internal/cmd/agent_fix_bugs.go`: Cobra command flags, config resolution, SSE loop integration, and `agent` parent command.
- Create `internal/cmd/agent_fix_bugs_test.go`: command config and once-mode integration tests.
- Modify `internal/cmd/root.go`: allow `fix-bugs` to initialize TAPD credentials while accepting watch endpoint config.
- Modify `README.md`: document the new command and conservative rollout.

## Task 1: Bug Event Parsing

**Files:**
- Create: `internal/cmd/agent_fix_bugs_event.go`
- Create: `internal/cmd/agent_fix_bugs_event_test.go`

- [ ] **Step 1: Write failing tests for event name matching and ID extraction**

Add `internal/cmd/agent_fix_bugs_event_test.go`:

```go
package cmd

import (
	"encoding/json"
	"testing"
)

func TestIsBugWebhookEvent(t *testing.T) {
	cases := []struct {
		name string
		got  bool
	}{
		{"bug::create", true},
		{"bug::update", true},
		{"bug_create", true},
		{"bug_update", true},
		{"story::create", false},
		{"task_update", false},
		{"", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := isBugWebhookEvent(tc.name); got != tc.got {
				t.Fatalf("isBugWebhookEvent(%q)=%v want=%v", tc.name, got, tc.got)
			}
		})
	}
}

func TestExtractBugEventTarget(t *testing.T) {
	cases := []struct {
		name        string
		payload     string
		wantOK      bool
		wantWS      string
		wantBugID   string
		wantReason  string
	}{
		{
			name:      "event bug id",
			payload:   `{"id":1,"received_at":1,"event":{"event":"bug::create","workspace_id":"123","bug":{"id":"456"}}}`,
			wantOK:    true,
			wantWS:    "123",
			wantBugID: "456",
		},
		{
			name:      "object id",
			payload:   `{"id":2,"received_at":1,"event":{"event":"bug_update","workspace_id":123,"object":{"id":456}}}`,
			wantOK:    true,
			wantWS:    "123",
			wantBugID: "456",
		},
		{
			name:      "data nested id",
			payload:   `{"id":3,"received_at":1,"event":{"event":"bug::update","workspace_id":"123","data":{"bug":{"id":"789"}}}}`,
			wantOK:    true,
			wantWS:    "123",
			wantBugID: "789",
		},
		{
			name:       "story skipped",
			payload:    `{"id":4,"received_at":1,"event":{"event":"story::create","workspace_id":"123","story":{"id":"456"}}}`,
			wantOK:     false,
			wantReason: "not_bug_event",
		},
		{
			name:       "missing workspace",
			payload:    `{"id":5,"received_at":1,"event":{"event":"bug::create","bug":{"id":"456"}}}`,
			wantOK:     false,
			wantReason: "missing_workspace_id",
		},
		{
			name:       "missing bug id",
			payload:    `{"id":6,"received_at":1,"event":{"event":"bug::create","workspace_id":"123"}}`,
			wantOK:     false,
			wantReason: "missing_bug_id",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var ev streamEvent
			if err := json.Unmarshal([]byte(tc.payload), &ev); err != nil {
				t.Fatal(err)
			}
			got, ok, reason := extractBugEventTarget(&ev)
			if ok != tc.wantOK {
				t.Fatalf("ok=%v want=%v reason=%s", ok, tc.wantOK, reason)
			}
			if reason != tc.wantReason {
				t.Fatalf("reason=%q want=%q", reason, tc.wantReason)
			}
			if ok {
				if got.WorkspaceID != tc.wantWS || got.BugID != tc.wantBugID || got.EventID != ev.ID {
					t.Fatalf("target=%+v want workspace=%s bug=%s event=%d", got, tc.wantWS, tc.wantBugID, ev.ID)
				}
			}
		})
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run:

```bash
go test ./internal/cmd -run 'Test(IsBugWebhookEvent|ExtractBugEventTarget)' -count=1
```

Expected: FAIL with undefined identifiers `isBugWebhookEvent`, `extractBugEventTarget`, and `bugEventTarget`.

- [ ] **Step 3: Implement event parsing helpers**

Add `internal/cmd/agent_fix_bugs_event.go`:

```go
package cmd

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// bugEventTarget 是从 TAPD webhook 事件中解析出的缺陷处理目标。
type bugEventTarget struct {
	WorkspaceID string
	BugID       string
	EventID     uint64
}

// isBugWebhookEvent 判断事件名是否是缺陷创建或更新。
func isBugWebhookEvent(name string) bool {
	switch strings.TrimSpace(name) {
	case "bug::create", "bug::update", "bug_create", "bug_update":
		return true
	default:
		return false
	}
}

// extractBugEventTarget 从 watch 的 streamEvent 中提取 workspace_id 和 bug id。
func extractBugEventTarget(ev *streamEvent) (bugEventTarget, bool, string) {
	if ev == nil {
		return bugEventTarget{}, false, "nil_event"
	}
	var payload map[string]interface{}
	if err := json.Unmarshal(ev.Event, &payload); err != nil {
		return bugEventTarget{}, false, "invalid_payload"
	}
	eventName := stringify(payload["event"])
	if !isBugWebhookEvent(eventName) {
		return bugEventTarget{}, false, "not_bug_event"
	}
	workspaceID := stringify(payload["workspace_id"])
	if workspaceID == "" {
		return bugEventTarget{}, false, "missing_workspace_id"
	}
	bugID := firstPathString(payload, [][]string{
		{"bug", "id"},
		{"object", "id"},
		{"id"},
		{"data", "bug", "id"},
		{"data", "id"},
	})
	if bugID == "" {
		return bugEventTarget{}, false, "missing_bug_id"
	}
	return bugEventTarget{WorkspaceID: workspaceID, BugID: bugID, EventID: ev.ID}, true, ""
}

func firstPathString(root map[string]interface{}, paths [][]string) string {
	for _, path := range paths {
		if v, ok := lookupPath(root, path); ok {
			if s := stringify(v); s != "" {
				return s
			}
		}
	}
	return ""
}

func lookupPath(root map[string]interface{}, path []string) (interface{}, bool) {
	var cur interface{} = root
	for _, part := range path {
		obj, ok := cur.(map[string]interface{})
		if !ok {
			return nil, false
		}
		cur, ok = obj[part]
		if !ok {
			return nil, false
		}
	}
	return cur, true
}

func stringify(v interface{}) string {
	switch x := v.(type) {
	case string:
		return strings.TrimSpace(x)
	case float64:
		if x == float64(int64(x)) {
			return strconv.FormatInt(int64(x), 10)
		}
		return fmt.Sprintf("%v", x)
	case json.Number:
		return x.String()
	default:
		return ""
	}
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run:

```bash
go test ./internal/cmd -run 'Test(IsBugWebhookEvent|ExtractBugEventTarget)' -count=1
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/cmd/agent_fix_bugs_event.go internal/cmd/agent_fix_bugs_event_test.go
git commit -m "feat(agent): parse tapd bug webhook events"
```

## Task 2: Local Execution Helpers

**Files:**
- Create: `internal/cmd/agent_fix_bugs_exec.go`
- Create: `internal/cmd/agent_fix_bugs_exec_test.go`

- [ ] **Step 1: Write failing tests for truncation, dirty guard, prompt, and shell runner**

Add `internal/cmd/agent_fix_bugs_exec_test.go`:

```go
package cmd

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestTruncateOutput(t *testing.T) {
	if got := truncateOutput("abcdef", 4); got != "abcd\n...[truncated]" {
		t.Fatalf("truncateOutput=%q", got)
	}
	if got := truncateOutput("abc", 10); got != "abc" {
		t.Fatalf("truncateOutput short=%q", got)
	}
}

func TestGitWorkingTreeDirty(t *testing.T) {
	dir := t.TempDir()
	runner := commandRunnerFunc(func(ctx context.Context, cfg commandRunConfig) commandRunResult {
		if cfg.Dir != dir || cfg.Command != "git status --porcelain" {
			t.Fatalf("unexpected command cfg=%+v", cfg)
		}
		return commandRunResult{Stdout: " M file.go\n", ExitCode: 0}
	})
	dirty, out, err := gitWorkingTreeDirty(context.Background(), runner, dir)
	if err != nil {
		t.Fatal(err)
	}
	if !dirty || out == "" {
		t.Fatalf("dirty=%v out=%q", dirty, out)
	}
}

func TestBuildAgentPrompt(t *testing.T) {
	bug := bugFixBugDetail{
		WorkspaceID:  "123",
		ID:           "456",
		Title:        "panic on nil request",
		Status:       "new",
		CurrentOwner: "alice",
		Severity:     "normal",
		Priority:     "high",
		Description:  "steps to reproduce",
		Comments:     []bugFixComment{{Author: "bob", Description: "please check nil guard"}},
	}
	prompt := buildAgentPrompt(bug, "/repo", "go test ./...")
	for _, want := range []string{
		"TAPD bug 456",
		"panic on nil request",
		"steps to reproduce",
		"/repo",
		"go test ./...",
		"Do not commit",
		"please check nil guard",
	} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("prompt missing %q:\n%s", want, prompt)
		}
	}
}

func TestShellCommandRunner(t *testing.T) {
	dir := t.TempDir()
	outFile := filepath.Join(dir, "stdin.txt")
	runner := shellCommandRunner{}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	res := runner.Run(ctx, commandRunConfig{
		Dir:     dir,
		Command: "cat > stdin.txt && printf ok",
		Stdin:   "payload",
		Limit:   1024,
	})
	if res.Err != nil || res.ExitCode != 0 {
		t.Fatalf("res=%+v", res)
	}
	if res.Stdout != "ok" {
		t.Fatalf("stdout=%q", res.Stdout)
	}
	data, err := os.ReadFile(outFile)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "payload" {
		t.Fatalf("stdin file=%q", data)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run:

```bash
go test ./internal/cmd -run 'Test(TruncateOutput|GitWorkingTreeDirty|BuildAgentPrompt|ShellCommandRunner)' -count=1
```

Expected: FAIL with undefined helper types and functions.

- [ ] **Step 3: Implement execution helpers**

Add `internal/cmd/agent_fix_bugs_exec.go`:

```go
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
```

- [ ] **Step 4: Run tests to verify they pass**

Run:

```bash
go test ./internal/cmd -run 'Test(TruncateOutput|GitWorkingTreeDirty|BuildAgentPrompt|ShellCommandRunner)' -count=1
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/cmd/agent_fix_bugs_exec.go internal/cmd/agent_fix_bugs_exec_test.go
git commit -m "feat(agent): add local execution helpers"
```

## Task 3: One-Bug Orchestration

**Files:**
- Create: `internal/cmd/agent_fix_bugs_runner.go`
- Create: `internal/cmd/agent_fix_bugs_runner_test.go`

- [ ] **Step 1: Write failing orchestration tests**

Add `internal/cmd/agent_fix_bugs_runner_test.go`:

```go
package cmd

import (
	"context"
	"strings"
	"testing"
)

type fakeBugFixTapd struct {
	bug             bugFixBugDetail
	comments        []string
	updates         []bugStatusUpdate
	failComment      bool
	failUpdateStatus bool
}

func (f *fakeBugFixTapd) GetBugDetail(ctx context.Context, workspaceID, bugID string) (bugFixBugDetail, error) {
	f.bug.WorkspaceID = workspaceID
	f.bug.ID = bugID
	return f.bug, nil
}

func (f *fakeBugFixTapd) AddBugComment(ctx context.Context, workspaceID, bugID, description string) error {
	f.comments = append(f.comments, description)
	if f.failComment {
		return errTestFailure
	}
	return nil
}

func (f *fakeBugFixTapd) UpdateBugStatus(ctx context.Context, update bugStatusUpdate) error {
	f.updates = append(f.updates, update)
	if f.failUpdateStatus {
		return errTestFailure
	}
	return nil
}

var errTestFailure = testError("boom")

type testError string

func (e testError) Error() string { return string(e) }

func TestBugFixWorkerHandleSuccess(t *testing.T) {
	tapd := &fakeBugFixTapd{bug: bugFixBugDetail{Title: "panic", Description: "nil pointer"}}
	var commands []string
	runner := commandRunnerFunc(func(ctx context.Context, cfg commandRunConfig) commandRunResult {
		commands = append(commands, cfg.Command)
		switch cfg.Command {
		case "git status --porcelain":
			return commandRunResult{}
		case "agent":
			if !strings.Contains(cfg.Stdin, "panic") {
				t.Fatalf("agent stdin missing bug title: %s", cfg.Stdin)
			}
			return commandRunResult{Stdout: "changed file.go"}
		case "go test ./...":
			return commandRunResult{Stdout: "ok"}
		default:
			t.Fatalf("unexpected command %q", cfg.Command)
			return commandRunResult{}
		}
	})
	worker := bugFixWorker{
		tapd:            tapd,
		runner:          runner,
		repo:            "/repo",
		agentCmd:        "agent",
		testCmd:         "go test ./...",
		onStartStatus:   "in_progress",
		onSuccessStatus: "resolved",
		resolution:      "fixed",
		outputLimit:     1024,
	}
	res := worker.handleTarget(context.Background(), bugEventTarget{WorkspaceID: "123", BugID: "456", EventID: 9})
	if res.Status != "success" || !res.Verified {
		t.Fatalf("result=%+v", res)
	}
	if len(commands) != 3 {
		t.Fatalf("commands=%v", commands)
	}
	if len(tapd.comments) != 1 || !strings.Contains(tapd.comments[0], "AI agent finished") {
		t.Fatalf("comments=%v", tapd.comments)
	}
	if len(tapd.updates) != 2 {
		t.Fatalf("updates=%+v", tapd.updates)
	}
	if tapd.updates[0].Status != "in_progress" || tapd.updates[1].Status != "resolved" {
		t.Fatalf("unexpected updates=%+v", tapd.updates)
	}
}

func TestBugFixWorkerSkipsDirtyRepo(t *testing.T) {
	tapd := &fakeBugFixTapd{bug: bugFixBugDetail{Title: "panic"}}
	runner := commandRunnerFunc(func(ctx context.Context, cfg commandRunConfig) commandRunResult {
		if cfg.Command == "git status --porcelain" {
			return commandRunResult{Stdout: " M file.go\n"}
		}
		t.Fatalf("agent/test should not run when dirty: %q", cfg.Command)
		return commandRunResult{}
	})
	worker := bugFixWorker{
		tapd:        tapd,
		runner:      runner,
		repo:        "/repo",
		agentCmd:    "agent",
		testCmd:     "go test ./...",
		outputLimit: 1024,
	}
	res := worker.handleTarget(context.Background(), bugEventTarget{WorkspaceID: "123", BugID: "456", EventID: 9})
	if res.Status != "failed" || res.Stage != "dirty_repo" {
		t.Fatalf("result=%+v", res)
	}
	if len(tapd.comments) != 1 || !strings.Contains(tapd.comments[0], "dirty_repo") {
		t.Fatalf("comments=%v", tapd.comments)
	}
}

func TestBugFixWorkerTestFailure(t *testing.T) {
	tapd := &fakeBugFixTapd{bug: bugFixBugDetail{Title: "panic"}}
	runner := commandRunnerFunc(func(ctx context.Context, cfg commandRunConfig) commandRunResult {
		switch cfg.Command {
		case "git status --porcelain":
			return commandRunResult{}
		case "agent":
			return commandRunResult{Stdout: "changed"}
		case "go test ./...":
			return commandRunResult{Stderr: "FAIL", ExitCode: 1, Err: errTestFailure}
		default:
			t.Fatalf("unexpected command %q", cfg.Command)
			return commandRunResult{}
		}
	})
	worker := bugFixWorker{
		tapd:            tapd,
		runner:          runner,
		repo:            "/repo",
		agentCmd:        "agent",
		testCmd:         "go test ./...",
		onSuccessStatus: "resolved",
		outputLimit:     1024,
	}
	res := worker.handleTarget(context.Background(), bugEventTarget{WorkspaceID: "123", BugID: "456", EventID: 9})
	if res.Status != "failed" || res.Stage != "test" {
		t.Fatalf("result=%+v", res)
	}
	if len(tapd.updates) != 0 {
		t.Fatalf("success status should not update on test failure: %+v", tapd.updates)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run:

```bash
go test ./internal/cmd -run 'TestBugFixWorker' -count=1
```

Expected: FAIL with undefined `bugFixWorker`, `bugStatusUpdate`, and result types.

- [ ] **Step 3: Implement orchestration**

Add `internal/cmd/agent_fix_bugs_runner.go`:

```go
package cmd

import (
	"context"
	"fmt"
	"sync"
)

// bugFixTapdClient 抽象 TAPD 读写，生产实现包装 SDK，测试使用 fake。
type bugFixTapdClient interface {
	GetBugDetail(ctx context.Context, workspaceID, bugID string) (bugFixBugDetail, error)
	AddBugComment(ctx context.Context, workspaceID, bugID, description string) error
	UpdateBugStatus(ctx context.Context, update bugStatusUpdate) error
}

// bugStatusUpdate 描述一次 TAPD 缺陷状态流转。
type bugStatusUpdate struct {
	WorkspaceID string
	BugID       string
	Status      string
	CurrentUser string
	Resolution  string
}

// bugFixResult 是 stdout 输出的一行机器可读处理结果。
type bugFixResult struct {
	WorkspaceID string `json:"workspace_id"`
	BugID       string `json:"bug_id"`
	EventID     uint64 `json:"event_id"`
	Status      string `json:"status"`
	Stage       string `json:"stage,omitempty"`
	Verified    bool   `json:"verified"`
	Detail      string `json:"detail,omitempty"`
}

// bugFixWorker 负责单个缺陷的本地修复流程。
type bugFixWorker struct {
	mu              sync.Mutex
	tapd            bugFixTapdClient
	runner          commandRunner
	repo            string
	agentCmd        string
	testCmd         string
	onStartStatus   string
	onSuccessStatus string
	onFailureStatus string
	currentUser     string
	resolution      string
	allowDirty      bool
	outputLimit     int
}

func (w *bugFixWorker) handleTarget(ctx context.Context, target bugEventTarget) bugFixResult {
	w.mu.Lock()
	defer w.mu.Unlock()

	result := bugFixResult{WorkspaceID: target.WorkspaceID, BugID: target.BugID, EventID: target.EventID}
	limit := w.outputLimit
	if limit <= 0 {
		limit = 12288
	}

	bug, err := w.tapd.GetBugDetail(ctx, target.WorkspaceID, target.BugID)
	if err != nil {
		return w.fail(ctx, result, "bug_show", err.Error())
	}

	if !w.allowDirty {
		dirty, out, err := gitWorkingTreeDirty(ctx, w.runner, w.repo)
		if err != nil {
			return w.fail(ctx, result, "dirty_check", err.Error())
		}
		if dirty {
			return w.fail(ctx, result, "dirty_repo", out)
		}
	}

	if w.onStartStatus != "" {
		_ = w.tapd.UpdateBugStatus(ctx, bugStatusUpdate{
			WorkspaceID: target.WorkspaceID,
			BugID:       target.BugID,
			Status:      w.onStartStatus,
			CurrentUser: w.currentUser,
		})
	}

	prompt := buildAgentPrompt(bug, w.repo, w.testCmd)
	agent := w.runner.Run(ctx, commandRunConfig{Dir: w.repo, Command: w.agentCmd, Stdin: prompt, Limit: limit})
	if agent.Err != nil || agent.ExitCode != 0 {
		return w.fail(ctx, result, "agent", fmt.Sprintf("stdout:\n%s\n\nstderr:\n%s", agent.Stdout, agent.Stderr))
	}

	test := commandRunResult{}
	verified := false
	if w.testCmd != "" {
		test = w.runner.Run(ctx, commandRunConfig{Dir: w.repo, Command: w.testCmd, Limit: limit})
		if test.Err != nil || test.ExitCode != 0 {
			return w.fail(ctx, result, "test", fmt.Sprintf("stdout:\n%s\n\nstderr:\n%s", test.Stdout, test.Stderr))
		}
		verified = true
	}

	comment := buildSuccessComment(agent, test, verified)
	if err := w.tapd.AddBugComment(ctx, target.WorkspaceID, target.BugID, comment); err != nil {
		return w.failNoComment(result, "comment", err.Error())
	}

	if w.onSuccessStatus != "" {
		err := w.tapd.UpdateBugStatus(ctx, bugStatusUpdate{
			WorkspaceID: target.WorkspaceID,
			BugID:       target.BugID,
			Status:      w.onSuccessStatus,
			CurrentUser: w.currentUser,
			Resolution:  fallbackString(w.resolution, "fixed"),
		})
		if err != nil {
			result.Status = "failed"
			result.Stage = "status_update"
			result.Verified = verified
			result.Detail = err.Error()
			return result
		}
	}

	result.Status = "success"
	result.Verified = verified
	return result
}

func (w *bugFixWorker) fail(ctx context.Context, base bugFixResult, stage, detail string) bugFixResult {
	comment := buildFailureComment(stage, detail)
	_ = w.tapd.AddBugComment(ctx, base.WorkspaceID, base.BugID, comment)
	if w.onFailureStatus != "" {
		_ = w.tapd.UpdateBugStatus(ctx, bugStatusUpdate{
			WorkspaceID: base.WorkspaceID,
			BugID:       base.BugID,
			Status:      w.onFailureStatus,
			CurrentUser: w.currentUser,
		})
	}
	return w.failNoComment(base, stage, detail)
}

func (w *bugFixWorker) failNoComment(base bugFixResult, stage, detail string) bugFixResult {
	base.Status = "failed"
	base.Stage = stage
	base.Detail = truncateOutput(detail, w.outputLimit)
	return base
}

func fallbackString(v, fallback string) string {
	if v != "" {
		return v
	}
	return fallback
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run:

```bash
go test ./internal/cmd -run 'TestBugFixWorker' -count=1
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/cmd/agent_fix_bugs_runner.go internal/cmd/agent_fix_bugs_runner_test.go
git commit -m "feat(agent): orchestrate bug fix runs"
```

## Task 4: TAPD SDK Adapter

**Files:**
- Modify: `internal/cmd/agent_fix_bugs_runner.go`
- Test: `internal/cmd/agent_fix_bugs_runner_test.go`

- [ ] **Step 1: Write failing adapter tests with httptest-backed SDK client**

Append to `internal/cmd/agent_fix_bugs_runner_test.go`:

```go
func TestSDKBugFixTapdClientMapsBugAndComments(t *testing.T) {
	resetFlags()
	var postForms []url.Values
	handler := func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		if r.Method == http.MethodPost {
			cp := url.Values{}
			for k, v := range r.PostForm {
				cp[k] = append([]string(nil), v...)
			}
			postForms = append(postForms, cp)
		}
		path := r.URL.Path
		switch {
		case strings.Contains(path, "/comments") && r.Method == http.MethodPost:
			w.Write([]byte(`{"status":1,"data":{"Comment":{"id":"c2","author":"agent","description":"ok"}}}`))
		case strings.Contains(path, "/comments"):
			w.Write([]byte(`{"status":1,"data":[{"Comment":{"id":"c1","author":"bob","created":"2026-06-04","description":"<p>please check</p>"}}]}`))
		case strings.Contains(path, "/bugs") && r.Method == http.MethodPost:
			w.Write([]byte(`{"status":1,"data":{"Bug":{"id":"456","title":"Test Bug","url":"http://test/bug/456"}}}`))
		default:
			w.Write([]byte(`{"status":1,"data":{"Bug":{"id":"456","title":"Test Bug","description":"<p>Desc</p>","status":"new","current_owner":"alice","severity":"normal","priority_label":"high"}}}`))
		}
	}
	_, cleanup := setupMockServer(t, handler)
	defer cleanup()

	c := sdkBugFixTapdClient{}
	got, err := c.GetBugDetail(context.Background(), "123", "456")
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != "456" || got.Title != "Test Bug" || got.CurrentOwner != "alice" || got.Priority != "high" {
		t.Fatalf("detail=%+v", got)
	}
	if !strings.Contains(got.Description, "Desc") || len(got.Comments) != 1 {
		t.Fatalf("detail markdown/comments=%+v", got)
	}

	if err := c.AddBugComment(context.Background(), "123", "456", "fixed by agent"); err != nil {
		t.Fatal(err)
	}
	if err := c.UpdateBugStatus(context.Background(), bugStatusUpdate{
		WorkspaceID: "123",
		BugID:       "456",
		Status:      "resolved",
		CurrentUser: "agent",
		Resolution:  "fixed",
	}); err != nil {
		t.Fatal(err)
	}
	if len(postForms) < 2 {
		t.Fatalf("postForms=%v", postForms)
	}
	if postForms[0].Get("description") != "fixed by agent" {
		t.Fatalf("comment form=%v", postForms[0])
	}
	last := postForms[len(postForms)-1]
	if last.Get("status") != "resolved" && last.Get("v_status") != "resolved" {
		t.Fatalf("status form=%v", last)
	}
	if last.Get("current_user") != "agent" || last.Get("resolution") != "fixed" {
		t.Fatalf("status form=%v", last)
	}
}
```

Add these imports to `internal/cmd/agent_fix_bugs_runner_test.go`:

```go
import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"testing"
)
```

- [ ] **Step 2: Run test to verify it fails**

Run:

```bash
go test ./internal/cmd -run TestSDKBugFixTapdClientMapsBugAndComments -count=1
```

Expected: FAIL because `sdkBugFixTapdClient` is undefined.

- [ ] **Step 3: Implement SDK adapter**

Append to `internal/cmd/agent_fix_bugs_runner.go`:

```go
// sdkBugFixTapdClient 把 tapd-sdk-go Client 适配成 bugFixTapdClient。
type sdkBugFixTapdClient struct{}

func (sdkBugFixTapdClient) GetBugDetail(ctx context.Context, workspaceID, bugID string) (bugFixBugDetail, error) {
	bug, err := apiClient.GetBug(ctx, workspaceID, bugID)
	if err != nil {
		return bugFixBugDetail{}, err
	}
	comments, _ := apiClient.ListComments(ctx, &model.ListCommentsRequest{
		WorkspaceID: workspaceID,
		EntryType:   "bug",
		EntryID:     bugID,
	})
	mapped := bugFixBugDetail{
		WorkspaceID:  workspaceID,
		ID:           bug.ID,
		Title:        bug.Title,
		Status:       bug.Status,
		CurrentOwner: bug.CurrentOwner,
		Severity:     bug.Severity,
		Priority:     bug.PriorityLabel,
		Description:  htmlToMarkdown(bug.Description),
	}
	for _, c := range comments {
		mapped.Comments = append(mapped.Comments, bugFixComment{
			Author:      c.Author,
			Created:     c.Created,
			Description: htmlToMarkdown(c.Description),
		})
	}
	return mapped, nil
}

func (sdkBugFixTapdClient) AddBugComment(ctx context.Context, workspaceID, bugID, description string) error {
	author := ensureNick()
	_, err := apiClient.AddComment(ctx, &model.AddCommentRequest{
		WorkspaceID: workspaceID,
		EntryType:   "bug",
		EntryID:     bugID,
		Description: description,
		Author:      author,
	})
	return err
}

func (sdkBugFixTapdClient) UpdateBugStatus(ctx context.Context, update bugStatusUpdate) error {
	currentUser := update.CurrentUser
	if currentUser == "" {
		currentUser = ensureNick()
	}
	_, err := apiClient.UpdateBug(ctx, &model.UpdateBugRequest{
		WorkspaceID: update.WorkspaceID,
		ID:          update.BugID,
		VStatus:     update.Status,
		CurrentUser: currentUser,
		Resolution:  update.Resolution,
	})
	return err
}
```

Also add imports to `internal/cmd/agent_fix_bugs_runner.go`:

```go
import (
	"context"
	"fmt"
	"sync"

	"github.com/studyzy/tapd-sdk-go/model"
)
```

- [ ] **Step 4: Run adapter and worker tests**

Run:

```bash
go test ./internal/cmd -run 'Test(SDKBugFixTapdClient|BugFixWorker)' -count=1
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/cmd/agent_fix_bugs_runner.go internal/cmd/agent_fix_bugs_runner_test.go
git commit -m "feat(agent): adapt tapd api for bug fixes"
```

## Task 5: Cobra Command and SSE Loop

**Files:**
- Create: `internal/cmd/agent_fix_bugs.go`
- Create: `internal/cmd/agent_fix_bugs_test.go`
- Modify: `internal/cmd/root.go`

- [ ] **Step 1: Write failing command config tests**

Add `internal/cmd/agent_fix_bugs_test.go`:

```go
package cmd

import "testing"

func TestResolveAgentFixBugsConfig(t *testing.T) {
	t.Cleanup(func() {
		flagAgentRepo = ""
		flagAgentCmd = ""
		flagAgentTestCmd = ""
		flagAgentOnStartStatus = ""
		flagAgentOnSuccessStatus = ""
		flagAgentOnFailureStatus = ""
		flagAgentCurrentUser = ""
		flagAgentResolution = ""
		flagAgentAllowDirty = false
		flagAgentOnce = false
		flagAgentOutputLimit = 0
		flagWatchEndpoint = ""
		flagWatchToken = ""
		appConfig = nil
	})

	flagAgentRepo = "/repo"
	flagAgentCmd = ""
	flagAgentTestCmd = "go test ./..."
	flagAgentOutputLimit = 0
	flagWatchEndpoint = "https://flag/events"
	flagWatchToken = "tok"

	cfg, err := resolveAgentFixBugsConfig()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.repo != "/repo" || cfg.endpoint != "https://flag/events" || cfg.token != "tok" {
		t.Fatalf("cfg=%+v", cfg)
	}
	if cfg.agentCmd != "codex exec --full-auto" {
		t.Fatalf("agent default=%q", cfg.agentCmd)
	}
	if cfg.outputLimit != 12288 {
		t.Fatalf("outputLimit=%d", cfg.outputLimit)
	}
}

func TestResolveAgentFixBugsConfigMissingRepo(t *testing.T) {
	t.Cleanup(func() {
		flagAgentRepo = ""
		flagWatchEndpoint = ""
		flagWatchToken = ""
	})
	flagWatchEndpoint = "https://flag/events"
	flagWatchToken = "tok"
	_, err := resolveAgentFixBugsConfig()
	if err == nil {
		t.Fatal("expected missing repo error")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run:

```bash
go test ./internal/cmd -run 'TestResolveAgentFixBugsConfig' -count=1
```

Expected: FAIL with undefined flags and `resolveAgentFixBugsConfig`.

- [ ] **Step 3: Implement Cobra command and stream handling**

Add `internal/cmd/agent_fix_bugs.go`:

```go
package cmd

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"github.com/studyzy/tapd-ai-cli/internal/output"
)

var (
	flagAgentRepo            string
	flagAgentCmd             string
	flagAgentTestCmd         string
	flagAgentOnStartStatus   string
	flagAgentOnSuccessStatus string
	flagAgentOnFailureStatus string
	flagAgentCurrentUser     string
	flagAgentResolution      string
	flagAgentAllowDirty      bool
	flagAgentOnce            bool
	flagAgentOutputLimit     int
)

var agentCmd = &cobra.Command{
	Use:   "agent",
	Short: "本地 AI agent 自动化",
}

var agentFixBugsCmd = &cobra.Command{
	Use:   "fix-bugs",
	Short: "监听 TAPD bug 事件并在本地仓库自动修复",
	RunE:  runAgentFixBugs,
}

type agentFixBugsConfig struct {
	repo            string
	endpoint        string
	token           string
	agentCmd        string
	testCmd         string
	onStartStatus   string
	onSuccessStatus string
	onFailureStatus string
	currentUser     string
	resolution      string
	allowDirty      bool
	once            bool
	outputLimit     int
	workspaceID     string
}

func init() {
	agentFixBugsCmd.Flags().StringVar(&flagAgentRepo, "repo", "", "本地仓库路径")
	agentFixBugsCmd.Flags().StringVar(&flagWatchEndpoint, "endpoint", "", "SSE 端点 URL，覆盖配置文件中的 watch_endpoint")
	agentFixBugsCmd.Flags().StringVar(&flagWatchToken, "token", "", "订阅鉴权 token，覆盖配置文件中的 subscribe_token")
	agentFixBugsCmd.Flags().StringVar(&flagAgentCmd, "agent-cmd", "", "本地 coding agent 命令")
	agentFixBugsCmd.Flags().StringVar(&flagAgentTestCmd, "test-cmd", "", "修复后验证命令")
	agentFixBugsCmd.Flags().StringVar(&flagAgentOnStartStatus, "on-start-status", "", "开始处理时流转到的 TAPD bug 状态")
	agentFixBugsCmd.Flags().StringVar(&flagAgentOnSuccessStatus, "on-success-status", "", "修复验证成功后流转到的 TAPD bug 状态")
	agentFixBugsCmd.Flags().StringVar(&flagAgentOnFailureStatus, "on-failure-status", "", "失败后流转到的 TAPD bug 状态")
	agentFixBugsCmd.Flags().StringVar(&flagAgentCurrentUser, "current-user", "", "TAPD 状态流转操作人，默认当前认证用户")
	agentFixBugsCmd.Flags().StringVar(&flagAgentResolution, "resolution", "fixed", "成功流转时写入的 resolution")
	agentFixBugsCmd.Flags().BoolVar(&flagAgentAllowDirty, "allow-dirty", false, "允许在 dirty working tree 中运行 agent")
	agentFixBugsCmd.Flags().BoolVar(&flagAgentOnce, "once", false, "处理一个 bug 事件后退出")
	agentFixBugsCmd.Flags().IntVar(&flagAgentOutputLimit, "output-limit", 12288, "写入 TAPD 评论的单段输出截断字节数")

	agentCmd.AddCommand(agentFixBugsCmd)
	rootCmd.AddCommand(agentCmd)
}

func resolveAgentFixBugsConfig() (agentFixBugsConfig, error) {
	endpoint, token := resolveWatchConfig()
	cfg := agentFixBugsConfig{
		repo:            flagAgentRepo,
		endpoint:        endpoint,
		token:           token,
		agentCmd:        fallbackString(flagAgentCmd, "codex exec --full-auto"),
		testCmd:         flagAgentTestCmd,
		onStartStatus:   flagAgentOnStartStatus,
		onSuccessStatus: flagAgentOnSuccessStatus,
		onFailureStatus: flagAgentOnFailureStatus,
		currentUser:     flagAgentCurrentUser,
		resolution:      fallbackString(flagAgentResolution, "fixed"),
		allowDirty:      flagAgentAllowDirty,
		once:            flagAgentOnce,
		outputLimit:     flagAgentOutputLimit,
		workspaceID:     flagWorkspaceID,
	}
	if cfg.outputLimit <= 0 {
		cfg.outputLimit = 12288
	}
	if strings.TrimSpace(cfg.repo) == "" {
		return cfg, fmt.Errorf("--repo is required")
	}
	if _, err := url.Parse(cfg.endpoint); cfg.endpoint == "" || err != nil {
		return cfg, fmt.Errorf("--endpoint or watch_endpoint config is required")
	}
	return cfg, nil
}

func runAgentFixBugs(cmd *cobra.Command, args []string) error {
	cfg, err := resolveAgentFixBugsConfig()
	if err != nil {
		output.PrintError(os.Stderr, "invalid_agent_config", err.Error(), "provide --repo and SSE endpoint config")
		os.Exit(output.ExitParamError)
		return nil
	}
	worker := &bugFixWorker{
		tapd:            sdkBugFixTapdClient{},
		runner:          shellCommandRunner{},
		repo:            cfg.repo,
		agentCmd:        cfg.agentCmd,
		testCmd:         cfg.testCmd,
		onStartStatus:   cfg.onStartStatus,
		onSuccessStatus: cfg.onSuccessStatus,
		onFailureStatus: cfg.onFailureStatus,
		currentUser:     cfg.currentUser,
		resolution:      cfg.resolution,
		allowDirty:      cfg.allowDirty,
		outputLimit:     cfg.outputLimit,
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	runAgentFixBugsStream(ctx, cfg, worker)
	return nil
}

func runAgentFixBugsStream(ctx context.Context, cfg agentFixBugsConfig, worker *bugFixWorker) {
	const minBackoff = time.Second
	const maxBackoff = 30 * time.Second
	backoff := minBackoff
	for {
		err := agentFixBugsStreamOnce(ctx, cfg, worker)
		if ctx.Err() != nil || err == errOnceDone {
			return
		}
		if err != nil {
			fmt.Fprintf(os.Stderr, "agent fix-bugs: connection lost: %v; reconnect in %s\n", err, backoff)
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(backoff):
		}
		backoff *= 2
		if backoff > maxBackoff {
			backoff = maxBackoff
		}
	}
}

func agentFixBugsStreamOnce(ctx context.Context, cfg agentFixBugsConfig, worker *bugFixWorker) error {
	if watchStateRef == nil {
		watchStateRef = newWatchState()
	}
	target, err := injectLastID(cfg.endpoint, watchStateRef.LastSeen())
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Cache-Control", "no-cache")
	if cfg.token != "" {
		req.Header.Set("X-TAPD-Token", cfg.token)
	}
	if v := watchStateRef.LastSeen(); v > 0 {
		req.Header.Set("Last-Event-ID", strconv.FormatUint(v, 10))
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<14))
		return fmt.Errorf("status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return readAgentFixBugsSSE(resp.Body, cfg, worker)
}

func readAgentFixBugsSSE(r io.Reader, cfg agentFixBugsConfig, worker *bugFixWorker) error {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 64*1024), 4*1024*1024)
	var dataLines []string
	flush := func() error {
		if len(dataLines) == 0 {
			return nil
		}
		data := strings.Join(dataLines, "\n")
		dataLines = dataLines[:0]
		handled, err := handleAgentFixBugsEvent(context.Background(), data, cfg, worker)
		if err != nil {
			return err
		}
		if handled && cfg.once {
			return errOnceDone
		}
		return nil
	}
	for scanner.Scan() {
		line := scanner.Text()
		switch {
		case line == "":
			if err := flush(); err != nil {
				return err
			}
		case strings.HasPrefix(line, ":"):
		case strings.HasPrefix(line, "data:"):
			dataLines = append(dataLines, strings.TrimSpace(strings.TrimPrefix(line, "data:")))
		}
	}
	if err := flush(); err != nil {
		return err
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	return io.EOF
}

func handleAgentFixBugsEvent(ctx context.Context, data string, cfg agentFixBugsConfig, worker *bugFixWorker) (bool, error) {
	var ev streamEvent
	if err := json.Unmarshal([]byte(data), &ev); err != nil {
		fmt.Fprintf(os.Stderr, "agent fix-bugs: invalid event json: %v\n", err)
		return false, nil
	}
	target, ok, reason := extractBugEventTarget(&ev)
	if !ok {
		fmt.Fprintf(os.Stderr, "agent fix-bugs: skip event id=%d reason=%s\n", ev.ID, reason)
		if watchStateRef != nil {
			watchStateRef.Update(ev.ID)
		}
		return false, nil
	}
	if cfg.workspaceID != "" && target.WorkspaceID != cfg.workspaceID {
		fmt.Fprintf(os.Stderr, "agent fix-bugs: skip event id=%d workspace=%s\n", ev.ID, target.WorkspaceID)
		if watchStateRef != nil {
			watchStateRef.Update(ev.ID)
		}
		return false, nil
	}
	res := worker.handleTarget(ctx, target)
	_ = output.PrintJSON(os.Stdout, res, true)
	if watchStateRef != nil {
		watchStateRef.Update(ev.ID)
	}
	return true, nil
}
```

Modify `internal/cmd/root.go` inside `PersistentPreRunE` before the normal `initClientAndConfig(cmd)` return:

```go
if cmd.Name() == "fix-bugs" && cmd.Parent() != nil && cmd.Parent().Name() == "agent" {
	return initClientAndConfig(cmd)
}
```

- [ ] **Step 4: Run config tests**

Run:

```bash
go test ./internal/cmd -run 'TestResolveAgentFixBugsConfig' -count=1
```

Expected: PASS.

- [ ] **Step 5: Add once-mode stream test**

Append to `internal/cmd/agent_fix_bugs_test.go`:

```go
func TestReadAgentFixBugsSSEOnce(t *testing.T) {
	t.Cleanup(func() { watchStateRef = nil })
	watchStateRef = &watchState{}
	tapd := &fakeBugFixTapd{bug: bugFixBugDetail{Title: "panic"}}
	runner := commandRunnerFunc(func(ctx context.Context, cfg commandRunConfig) commandRunResult {
		switch cfg.Command {
		case "git status --porcelain":
			return commandRunResult{}
		case "agent":
			return commandRunResult{Stdout: "ok"}
		default:
			return commandRunResult{}
		}
	})
	worker := &bugFixWorker{tapd: tapd, runner: runner, repo: "/repo", agentCmd: "agent", outputLimit: 1024}
	body := strings.NewReader("event: tapd\nid: 1\ndata: {\"id\":1,\"received_at\":1,\"event\":{\"event\":\"bug::create\",\"workspace_id\":\"123\",\"bug\":{\"id\":\"456\"}}}\n\n")
	err := readAgentFixBugsSSE(body, agentFixBugsConfig{once: true}, worker)
	if err != errOnceDone {
		t.Fatalf("err=%v want errOnceDone", err)
	}
	if watchStateRef.LastSeen() != 1 {
		t.Fatalf("lastSeen=%d", watchStateRef.LastSeen())
	}
}
```

Add missing imports to `internal/cmd/agent_fix_bugs_test.go`:

```go
import (
	"context"
	"strings"
	"testing"
)
```

- [ ] **Step 6: Run command integration tests**

Run:

```bash
go test ./internal/cmd -run 'Test(ReadAgentFixBugsSSEOnce|ResolveAgentFixBugsConfig)' -count=1
```

Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/cmd/agent_fix_bugs.go internal/cmd/agent_fix_bugs_test.go internal/cmd/root.go
git commit -m "feat(agent): add fix-bugs command"
```

## Task 6: Documentation and Full Verification

**Files:**
- Modify: `README.md`

- [ ] **Step 1: Update README command tree and webhook docs**

Modify `README.md` command tree to include:

```markdown
├── agent    fix-bugs --repo <path> [--test-cmd <cmd>] [--on-success-status <status>]
```

Add this section after `tapd watch` docs:

```markdown
## 自动修复 TAPD Bug（tapd agent fix-bugs）

`tapd agent fix-bugs` 在本地运行，订阅 TAPD webhook SSE，只处理 bug 创建/更新事件。
命令会拉取 bug 详情，调用本地 coding agent 修改 `--repo` 指向的仓库，运行 `--test-cmd`
验证，然后给 bug 写评论。只有配置了 `--on-success-status` 时才会自动流转状态。

推荐先用一次性、无状态流转模式试跑：

```bash
tapd agent fix-bugs \
  --repo /Users/sunruoyu/go/src/vas/app/upower \
  --test-cmd "go test ./..." \
  --on-start-status "" \
  --on-success-status "" \
  --once
```

确认评论和本地修改符合预期后，再开启状态流转：

```bash
tapd agent fix-bugs \
  --repo /Users/sunruoyu/go/src/vas/app/upower \
  --test-cmd "go test ./..." \
  --on-start-status in_progress \
  --on-success-status resolved
```

默认要求工作区干净；如果 `git status --porcelain` 有输出，命令会跳过自动修复并写 TAPD 评论。
命令不会自动 commit、push、创建 MR、部署或合并。
```
```

- [ ] **Step 2: Run focused tests**

Run:

```bash
go test ./internal/cmd -run 'Test(IsBugWebhookEvent|ExtractBugEventTarget|TruncateOutput|GitWorkingTreeDirty|BuildAgentPrompt|ShellCommandRunner|BugFixWorker|ResolveAgentFixBugsConfig|ReadAgentFixBugsSSEOnce)' -count=1
```

Expected: PASS.

- [ ] **Step 3: Run full test suite**

Run:

```bash
go test ./...
```

Expected: PASS.

- [ ] **Step 4: Check git diff**

Run:

```bash
git status --short
git diff --stat
```

Expected: only files from this plan are modified, plus any pre-existing unrelated user changes still present.

- [ ] **Step 5: Commit docs and final verification**

```bash
git add README.md
git commit -m "docs(agent): document bug fixer workflow"
```

## Notes for Executors

- The repository currently has unrelated uncommitted changes in `internal/cmd/mcp.go`, `internal/cmd/watch.go`, `internal/mcp/server.go`, `internal/mcp/tools.go`, `.agents/`, and several watch/MCP files. Do not revert them. If they conflict, read them and adapt.
- If `go test ./...` fails because of unrelated pre-existing changes, capture the exact failing package and test name before deciding whether it belongs to this task.
- Do not add automatic commit, push, MR, deploy, or merge behavior.
- Keep service-side `vas/app/upower/interface` unchanged unless a test reveals an SSE protocol bug.
