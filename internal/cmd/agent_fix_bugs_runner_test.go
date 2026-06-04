package cmd

import (
	"context"
	"strings"
	"testing"
)

type fakeBugFixTapd struct {
	bug              bugFixBugDetail
	comments         []string
	updates          []bugStatusUpdate
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
	if len(tapd.comments) != 1 || !strings.Contains(tapd.comments[0], "AI agent run completed") {
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
	var commands []string
	runner := commandRunnerFunc(func(ctx context.Context, cfg commandRunConfig) commandRunResult {
		commands = append(commands, cfg.Command)
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
	wantCommands := []string{"git status --porcelain", "agent", "go test ./..."}
	if len(commands) < len(wantCommands) {
		t.Fatalf("commands=%v, want at least %v", commands, wantCommands)
	}
	for i, want := range wantCommands {
		if commands[i] != want {
			t.Fatalf("commands=%v, want prefix %v", commands, wantCommands)
		}
	}
	if len(tapd.updates) != 0 {
		t.Fatalf("success status should not update on test failure: %+v", tapd.updates)
	}
}
