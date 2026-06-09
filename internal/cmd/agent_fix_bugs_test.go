package cmd

import (
	"context"
	"strings"
	"testing"

	"github.com/studyzy/tapd-ai-cli/internal/config"
)

func TestResolveAgentFixBugsConfig(t *testing.T) {
	t.Cleanup(resetAgentFixBugsTestState)
	resetAgentFixBugsTestState()

	flagAgentRepo = "/repo"
	flagAgentTestCmd = "go test ./..."
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
	if cfg.outputLimit != defaultCommandOutputLimit {
		t.Fatalf("outputLimit=%d", cfg.outputLimit)
	}
	if cfg.branchStrategy != "current" || cfg.mrRemote != "origin" || cfg.mrBranchPrefix != "tapd-agent/mr-" {
		t.Fatalf("branch defaults=%+v", cfg)
	}
}

func TestResolveAgentFixBugsConfigFromAppConfig(t *testing.T) {
	t.Cleanup(resetAgentFixBugsTestState)
	resetAgentFixBugsTestState()

	flagAgentRepo = "/repo"
	appConfig = &config.Config{
		WatchEndpoint:  "https://cfg/events",
		SubscribeToken: "cfg-token",
	}

	cfg, err := resolveAgentFixBugsConfig()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.endpoint != "https://cfg/events" || cfg.token != "cfg-token" {
		t.Fatalf("cfg=%+v", cfg)
	}
}

func TestResolveAgentFixBugsConfigMissingRepo(t *testing.T) {
	t.Cleanup(resetAgentFixBugsTestState)
	resetAgentFixBugsTestState()

	flagWatchEndpoint = "https://flag/events"
	flagWatchToken = "tok"
	_, err := resolveAgentFixBugsConfig()
	if err == nil {
		t.Fatal("expected missing repo error")
	}
}

func TestResolveAgentFixBugsConfigSuccessStatusRequiresTestCommand(t *testing.T) {
	t.Cleanup(resetAgentFixBugsTestState)
	resetAgentFixBugsTestState()

	flagAgentRepo = "/repo"
	flagWatchEndpoint = "https://flag/events"
	flagAgentOnSuccessStatus = "resolved"

	_, err := resolveAgentFixBugsConfig()
	if err == nil {
		t.Fatal("expected error when success status is configured without test command")
	}
	if !strings.Contains(err.Error(), "--test-cmd") {
		t.Fatalf("error should mention --test-cmd, got %v", err)
	}
}

func TestResolveAgentFixBugsConfigLinkedMR(t *testing.T) {
	t.Cleanup(resetAgentFixBugsTestState)
	resetAgentFixBugsTestState()

	flagAgentRepo = "/repo"
	flagWatchEndpoint = "https://flag/events"
	flagAgentBranchStrategy = "linked-mr"
	flagAgentMRRemote = "upstream"
	flagAgentMRBranchPrefix = "tapd/mr-"

	cfg, err := resolveAgentFixBugsConfig()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.branchStrategy != "linked-mr" || cfg.mrRemote != "upstream" || cfg.mrBranchPrefix != "tapd/mr-" {
		t.Fatalf("cfg=%+v", cfg)
	}
}

func TestResolveAgentFixBugsConfigInvalidBranchStrategy(t *testing.T) {
	t.Cleanup(resetAgentFixBugsTestState)
	resetAgentFixBugsTestState()

	flagAgentRepo = "/repo"
	flagWatchEndpoint = "https://flag/events"
	flagAgentBranchStrategy = "push-directly"

	_, err := resolveAgentFixBugsConfig()
	if err == nil || !strings.Contains(err.Error(), "--branch-strategy") {
		t.Fatalf("err=%v", err)
	}
}

func TestReadAgentFixBugsSSEOnce(t *testing.T) {
	t.Cleanup(resetAgentFixBugsTestState)
	resetAgentFixBugsTestState()
	watchStateRef = &watchState{}

	tapd := &fakeBugFixTapd{bug: bugFixBugDetail{Title: "panic", Description: "steps"}}
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
	err := readAgentFixBugsSSE(context.Background(), body, agentFixBugsConfig{once: true}, worker)
	if err != errOnceDone {
		t.Fatalf("err=%v want errOnceDone", err)
	}
	if watchStateRef.LastSeen() != 1 {
		t.Fatalf("lastSeen=%d", watchStateRef.LastSeen())
	}
}

func TestHandleAgentFixBugsEventDoesNotAdvanceStateOnFailure(t *testing.T) {
	t.Cleanup(resetAgentFixBugsTestState)
	resetAgentFixBugsTestState()
	watchStateRef = &watchState{}

	tapd := &fakeBugFixTapd{bug: bugFixBugDetail{Title: "panic", Description: "steps"}}
	runner := commandRunnerFunc(func(ctx context.Context, cfg commandRunConfig) commandRunResult {
		if cfg.Command == "git status --porcelain" {
			return commandRunResult{Stdout: " M file.go\n"}
		}
		t.Fatalf("agent/test should not run when repo is dirty: %q", cfg.Command)
		return commandRunResult{}
	})
	worker := &bugFixWorker{tapd: tapd, runner: runner, repo: "/repo", agentCmd: "agent", outputLimit: 1024}
	data := `{"id":7,"received_at":1,"event":{"event":"bug::create","workspace_id":"123","bug":{"id":"456"}}}`

	handled, err := handleAgentFixBugsEvent(context.Background(), data, agentFixBugsConfig{}, worker)
	if err != nil {
		t.Fatal(err)
	}
	if !handled {
		t.Fatal("expected event to be handled")
	}
	if watchStateRef.LastSeen() != 0 {
		t.Fatalf("lastSeen=%d, want 0 after failed handling", watchStateRef.LastSeen())
	}
}

func TestHandleAgentFixBugsEventAdvancesStateOnSkipped(t *testing.T) {
	t.Cleanup(resetAgentFixBugsTestState)
	resetAgentFixBugsTestState()
	watchStateRef = &watchState{}

	tapd := &fakeBugFixTapd{bug: bugFixBugDetail{Title: "panic", Description: "", CurrentOwner: "agent"}}
	runner := commandRunnerFunc(func(ctx context.Context, cfg commandRunConfig) commandRunResult {
		t.Fatalf("git/agent/test should not run when event is skipped: %q", cfg.Command)
		return commandRunResult{}
	})
	worker := &bugFixWorker{tapd: tapd, runner: runner, repo: "/repo", agentCmd: "agent", currentUser: "agent", outputLimit: 1024}
	data := `{"id":7,"received_at":1,"event":{"event":"bug::create","workspace_id":"123","bug":{"id":"456"}}}`

	handled, err := handleAgentFixBugsEvent(context.Background(), data, agentFixBugsConfig{}, worker)
	if err != nil {
		t.Fatal(err)
	}
	if !handled {
		t.Fatal("expected event to be handled")
	}
	if watchStateRef.LastSeen() != 7 {
		t.Fatalf("lastSeen=%d, want 7 after skipped handling", watchStateRef.LastSeen())
	}
}

func TestHandleAgentFixBugsEventFailureBlocksLaterSuccessWatermark(t *testing.T) {
	t.Cleanup(resetAgentFixBugsTestState)
	resetAgentFixBugsTestState()
	watchStateRef = &watchState{}

	dirty := true
	tapd := &fakeBugFixTapd{bug: bugFixBugDetail{Title: "panic", Description: "steps"}}
	runner := commandRunnerFunc(func(ctx context.Context, cfg commandRunConfig) commandRunResult {
		switch cfg.Command {
		case "git status --porcelain":
			if dirty {
				return commandRunResult{Stdout: " M file.go\n"}
			}
			return commandRunResult{}
		case "agent":
			return commandRunResult{Stdout: "ok"}
		default:
			t.Fatalf("unexpected command %q", cfg.Command)
			return commandRunResult{}
		}
	})
	worker := &bugFixWorker{tapd: tapd, runner: runner, repo: "/repo", agentCmd: "agent", outputLimit: 1024}

	failedEvent := `{"id":7,"received_at":1,"event":{"event":"bug::create","workspace_id":"123","bug":{"id":"456"}}}`
	laterEvent := `{"id":8,"received_at":1,"event":{"event":"bug::create","workspace_id":"123","bug":{"id":"789"}}}`
	retryEvent := `{"id":7,"received_at":1,"event":{"event":"bug::create","workspace_id":"123","bug":{"id":"456"}}}`

	if _, err := handleAgentFixBugsEvent(context.Background(), failedEvent, agentFixBugsConfig{}, worker); err != nil {
		t.Fatal(err)
	}
	dirty = false
	if _, err := handleAgentFixBugsEvent(context.Background(), laterEvent, agentFixBugsConfig{}, worker); err != nil {
		t.Fatal(err)
	}
	if watchStateRef.LastSeen() != 0 {
		t.Fatalf("later success advanced blocked watermark to %d", watchStateRef.LastSeen())
	}
	if _, err := handleAgentFixBugsEvent(context.Background(), retryEvent, agentFixBugsConfig{}, worker); err != nil {
		t.Fatal(err)
	}
	if watchStateRef.LastSeen() != 7 {
		t.Fatalf("retry success lastSeen=%d want 7", watchStateRef.LastSeen())
	}
}

func resetAgentFixBugsTestState() {
	flagAgentRepo = ""
	flagAgentCmd = ""
	flagAgentTestCmd = ""
	flagAgentOnStartStatus = ""
	flagAgentOnSuccessStatus = ""
	flagAgentOnFailureStatus = ""
	flagAgentCurrentUser = ""
	flagAgentResolution = "fixed"
	flagAgentAllowDirty = false
	flagAgentOnce = false
	flagAgentOutputLimit = defaultCommandOutputLimit
	flagAgentBranchStrategy = "current"
	flagAgentMRRemote = "origin"
	flagAgentMRBranchPrefix = "tapd-agent/mr-"
	flagWatchEndpoint = ""
	flagWatchToken = ""
	flagWorkspaceID = ""
	appConfig = nil
	watchStateRef = nil
	agentFixBugsBlockedEventID = 0
}
