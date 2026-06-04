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
