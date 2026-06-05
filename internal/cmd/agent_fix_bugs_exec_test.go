package cmd

import (
	"context"
	"errors"
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
	dirty, out, err := gitWorkingTreeDirty(context.Background(), runner, dir, defaultCommandOutputLimit)
	if err != nil {
		t.Fatal(err)
	}
	if !dirty || out != " M file.go\n" {
		t.Fatalf("dirty=%v out=%q", dirty, out)
	}
}

func TestGitWorkingTreeDirtyErrorDetail(t *testing.T) {
	dir := t.TempDir()
	runner := commandRunnerFunc(func(ctx context.Context, cfg commandRunConfig) commandRunResult {
		return commandRunResult{
			Stdout:   "partial stdout\n",
			Stderr:   "fatal stderr\n",
			ExitCode: 128,
			Err:      errors.New("exit status 128"),
		}
	})
	dirty, detail, err := gitWorkingTreeDirty(context.Background(), runner, dir, defaultCommandOutputLimit)
	if err == nil {
		t.Fatal("expected error")
	}
	if dirty {
		t.Fatal("dirty should be false on command error")
	}
	for _, want := range []string{"exit status 128", "Exit code: 128", "partial stdout", "fatal stderr"} {
		if !strings.Contains(detail, want) {
			t.Fatalf("detail missing %q:\n%s", want, detail)
		}
	}
}

func TestGitWorkingTreeDirtyUsesConfiguredLimit(t *testing.T) {
	dir := t.TempDir()
	runner := commandRunnerFunc(func(ctx context.Context, cfg commandRunConfig) commandRunResult {
		if cfg.Limit != 16 {
			t.Fatalf("limit=%d, want 16", cfg.Limit)
		}
		return commandRunResult{Stdout: " M file.go\n", ExitCode: 0}
	})
	dirty, out, err := gitWorkingTreeDirty(context.Background(), runner, dir, 16)
	if err != nil {
		t.Fatal(err)
	}
	if !dirty || out != " M file.go\n" {
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

func TestShellCommandRunnerTruncatesStdoutAndStderr(t *testing.T) {
	runner := shellCommandRunner{}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	res := runner.Run(ctx, commandRunConfig{
		Command: "printf abcdef; printf 123456 >&2",
		Limit:   4,
	})
	if res.Stdout != "abcd\n...[truncated]" {
		t.Fatalf("stdout=%q", res.Stdout)
	}
	if res.Stderr != "1234\n...[truncated]" {
		t.Fatalf("stderr=%q", res.Stderr)
	}
}

func TestBuildSuccessCommentUnverified(t *testing.T) {
	comment := buildSuccessComment(
		commandRunResult{Stdout: "agent out"},
		commandRunResult{Stderr: "test err"},
		false,
	)
	if !strings.Contains(comment, "AI agent run completed") {
		t.Fatalf("comment missing neutral headline:\n%s", comment)
	}
	if strings.Contains(comment, "finished bug fix") {
		t.Fatalf("comment has misleading headline:\n%s", comment)
	}
	if !strings.Contains(comment, "Verified: false") {
		t.Fatalf("comment missing verification status:\n%s", comment)
	}
}
