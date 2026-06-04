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

func TestReadAgentFixBugsSSEOnce(t *testing.T) {
	t.Cleanup(resetAgentFixBugsTestState)
	resetAgentFixBugsTestState()
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
	err := readAgentFixBugsSSE(context.Background(), body, agentFixBugsConfig{once: true}, worker)
	if err != errOnceDone {
		t.Fatalf("err=%v want errOnceDone", err)
	}
	if watchStateRef.LastSeen() != 1 {
		t.Fatalf("lastSeen=%d", watchStateRef.LastSeen())
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
	flagWatchEndpoint = ""
	flagWatchToken = ""
	flagWorkspaceID = ""
	appConfig = nil
	watchStateRef = nil
}
