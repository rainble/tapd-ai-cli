package cmd

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

type fakeBugFixTapd struct {
	bug              bugFixBugDetail
	stories          map[string]bugFixStoryDetail
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

func (f *fakeBugFixTapd) GetStoryDetail(ctx context.Context, workspaceID, storyID string) (bugFixStoryDetail, error) {
	if f.stories == nil {
		return bugFixStoryDetail{}, errTestFailure
	}
	story, ok := f.stories[storyID]
	if !ok {
		return bugFixStoryDetail{}, errTestFailure
	}
	story.WorkspaceID = workspaceID
	story.ID = storyID
	return story, nil
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

func TestBugFixWorkerSkipsEmptyDescription(t *testing.T) {
	tapd := &fakeBugFixTapd{bug: bugFixBugDetail{Title: "panic", Description: "   ", CurrentOwner: "agent"}}
	runner := commandRunnerFunc(func(ctx context.Context, cfg commandRunConfig) commandRunResult {
		t.Fatalf("git/agent/test should not run when bug description is empty: %q", cfg.Command)
		return commandRunResult{}
	})
	worker := bugFixWorker{
		tapd:        tapd,
		runner:      runner,
		repo:        "/repo",
		agentCmd:    "agent",
		testCmd:     "go test ./...",
		currentUser: "agent",
		outputLimit: 1024,
	}
	res := worker.handleTarget(context.Background(), bugEventTarget{WorkspaceID: "123", BugID: "456", EventID: 9})
	if res.Status != "skipped" || res.Stage != "empty_content" {
		t.Fatalf("result=%+v", res)
	}
	if len(tapd.comments) != 0 || len(tapd.updates) != 0 {
		t.Fatalf("comments=%v updates=%+v", tapd.comments, tapd.updates)
	}
}

func TestBugFixWorkerSkipsOwnerMismatch(t *testing.T) {
	tapd := &fakeBugFixTapd{bug: bugFixBugDetail{Title: "panic", Description: "nil pointer", CurrentOwner: "bob;carol"}}
	runner := commandRunnerFunc(func(ctx context.Context, cfg commandRunConfig) commandRunResult {
		t.Fatalf("git/agent/test should not run when current user is not owner: %q", cfg.Command)
		return commandRunResult{}
	})
	worker := bugFixWorker{
		tapd:        tapd,
		runner:      runner,
		repo:        "/repo",
		agentCmd:    "agent",
		testCmd:     "go test ./...",
		currentUser: "alice",
		outputLimit: 1024,
	}
	res := worker.handleTarget(context.Background(), bugEventTarget{WorkspaceID: "123", BugID: "456", EventID: 9})
	if res.Status != "skipped" || res.Stage != "owner_mismatch" {
		t.Fatalf("result=%+v", res)
	}
	if len(tapd.comments) != 0 || len(tapd.updates) != 0 {
		t.Fatalf("comments=%v updates=%+v", tapd.comments, tapd.updates)
	}
}

func TestBugFixWorkerLinkedMRUsesStoryMRBeforeBugMR(t *testing.T) {
	tapd := &fakeBugFixTapd{
		bug: bugFixBugDetail{
			Title:       "panic",
			Description: "bug mentions http://git.example.com/team/repo/-/merge_requests/12",
			StoryID:     "story-1",
			Comments: []bugFixComment{{
				Description: "bug MR http://git.example.com/team/repo/-/merge_requests/13",
			}},
		},
		stories: map[string]bugFixStoryDetail{
			"story-1": {
				Title: "linked story",
				Comments: []bugFixComment{{
					Description: "story MR http://git.example.com/team/repo/-/merge_requests/99",
				}},
			},
		},
	}
	var commands []string
	runner := commandRunnerFunc(func(ctx context.Context, cfg commandRunConfig) commandRunResult {
		commands = append(commands, cfg.Command)
		switch cfg.Command {
		case "git status --porcelain":
			return commandRunResult{}
		case "git fetch origin merge-requests/99/head":
			return commandRunResult{Stdout: "fetched"}
		case "git checkout -B tapd-agent/mr-99 FETCH_HEAD":
			return commandRunResult{Stdout: "checked out"}
		case "agent":
			return commandRunResult{Stdout: "changed"}
		case "go test ./...":
			return commandRunResult{Stdout: "ok"}
		default:
			t.Fatalf("unexpected command %q", cfg.Command)
			return commandRunResult{}
		}
	})
	worker := bugFixWorker{
		tapd:           tapd,
		runner:         runner,
		repo:           "/repo",
		agentCmd:       "agent",
		testCmd:        "go test ./...",
		branchStrategy: "linked-mr",
		mrRemote:       "origin",
		mrBranchPrefix: "tapd-agent/mr-",
		outputLimit:    1024,
	}
	res := worker.handleTarget(context.Background(), bugEventTarget{WorkspaceID: "123", BugID: "456", EventID: 9})
	if res.Status != "success" || !res.Verified {
		t.Fatalf("result=%+v", res)
	}
	wantCommands := []string{
		"git status --porcelain",
		"git fetch origin merge-requests/99/head",
		"git checkout -B tapd-agent/mr-99 FETCH_HEAD",
		"agent",
		"go test ./...",
	}
	if strings.Join(commands, "\n") != strings.Join(wantCommands, "\n") {
		t.Fatalf("commands=%v, want %v", commands, wantCommands)
	}
	if len(tapd.comments) != 1 || !strings.Contains(tapd.comments[0], "tapd-agent/mr-99") {
		t.Fatalf("comments=%v", tapd.comments)
	}
}

func TestBugFixWorkerLinkedMRFallsBackToParentStoryBeforeBugMR(t *testing.T) {
	tapd := &fakeBugFixTapd{
		bug: bugFixBugDetail{
			Title:       "panic",
			Description: "bug MR http://git.example.com/team/repo/-/merge_requests/13",
			StoryID:     "story-child",
		},
		stories: map[string]bugFixStoryDetail{
			"story-child": {
				Title:       "child story",
				Description: "no MR here",
				ParentID:    "story-parent",
			},
			"story-parent": {
				Title: "parent story",
				Comments: []bugFixComment{{
					Description: "parent MR http://git.example.com/team/repo/-/merge_requests/77",
				}},
			},
		},
	}
	var commands []string
	runner := commandRunnerFunc(func(ctx context.Context, cfg commandRunConfig) commandRunResult {
		commands = append(commands, cfg.Command)
		switch cfg.Command {
		case "git status --porcelain":
			return commandRunResult{}
		case "git fetch origin merge-requests/77/head":
			return commandRunResult{Stdout: "fetched"}
		case "git checkout -B tapd-agent/mr-77 FETCH_HEAD":
			return commandRunResult{Stdout: "checked out"}
		case "agent":
			return commandRunResult{Stdout: "changed"}
		case "go test ./...":
			return commandRunResult{Stdout: "ok"}
		default:
			t.Fatalf("unexpected command %q", cfg.Command)
			return commandRunResult{}
		}
	})
	worker := bugFixWorker{
		tapd:           tapd,
		runner:         runner,
		repo:           "/repo",
		agentCmd:       "agent",
		testCmd:        "go test ./...",
		branchStrategy: "linked-mr",
		mrRemote:       "origin",
		mrBranchPrefix: "tapd-agent/mr-",
		outputLimit:    1024,
	}
	res := worker.handleTarget(context.Background(), bugEventTarget{WorkspaceID: "123", BugID: "456", EventID: 9})
	if res.Status != "success" || !res.Verified {
		t.Fatalf("result=%+v", res)
	}
	if len(tapd.comments) != 1 || !strings.Contains(tapd.comments[0], "story:story-parent") {
		t.Fatalf("comments=%v", tapd.comments)
	}
	for _, got := range commands {
		if strings.Contains(got, "merge-requests/13/head") {
			t.Fatalf("used bug MR instead of parent story MR: commands=%v", commands)
		}
	}
}

func TestBugFixWorkerLinkedMRFailsWhenNoMRFound(t *testing.T) {
	tapd := &fakeBugFixTapd{bug: bugFixBugDetail{Title: "panic", Description: "steps", StoryID: "story-1"}, stories: map[string]bugFixStoryDetail{
		"story-1": {Title: "story without mr"},
	}}
	var commands []string
	runner := commandRunnerFunc(func(ctx context.Context, cfg commandRunConfig) commandRunResult {
		commands = append(commands, cfg.Command)
		if cfg.Command == "git status --porcelain" {
			return commandRunResult{}
		}
		t.Fatalf("agent/git fetch should not run without linked MR: %q", cfg.Command)
		return commandRunResult{}
	})
	worker := bugFixWorker{
		tapd:           tapd,
		runner:         runner,
		repo:           "/repo",
		agentCmd:       "agent",
		testCmd:        "go test ./...",
		branchStrategy: "linked-mr",
		mrRemote:       "origin",
		mrBranchPrefix: "tapd-agent/mr-",
		outputLimit:    1024,
	}
	res := worker.handleTarget(context.Background(), bugEventTarget{WorkspaceID: "123", BugID: "456", EventID: 9})
	if res.Status != "failed" || res.Stage != "branch_prepare" {
		t.Fatalf("result=%+v", res)
	}
	if len(commands) != 1 || commands[0] != "git status --porcelain" {
		t.Fatalf("commands=%v", commands)
	}
	if len(tapd.comments) != 1 || !strings.Contains(tapd.comments[0], "no GitLab MR link found") {
		t.Fatalf("comments=%v", tapd.comments)
	}
}

func TestBugFixWorkerSkipsDirtyRepo(t *testing.T) {
	tapd := &fakeBugFixTapd{bug: bugFixBugDetail{Title: "panic", Description: "steps"}}
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
	tapd := &fakeBugFixTapd{bug: bugFixBugDetail{Title: "panic", Description: "steps"}}
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

func TestBugFixWorkerStartStatusFailureStopsBeforeAgent(t *testing.T) {
	tapd := &fakeBugFixTapd{bug: bugFixBugDetail{Title: "panic", Description: "steps"}, failUpdateStatus: true}
	var commands []string
	runner := commandRunnerFunc(func(ctx context.Context, cfg commandRunConfig) commandRunResult {
		commands = append(commands, cfg.Command)
		if cfg.Command == "git status --porcelain" {
			return commandRunResult{}
		}
		t.Fatalf("agent/test should not run after start status failure: %q", cfg.Command)
		return commandRunResult{}
	})
	worker := bugFixWorker{
		tapd:          tapd,
		runner:        runner,
		repo:          "/repo",
		agentCmd:      "agent",
		testCmd:       "go test ./...",
		onStartStatus: "in_progress",
		outputLimit:   1024,
	}
	res := worker.handleTarget(context.Background(), bugEventTarget{WorkspaceID: "123", BugID: "456", EventID: 9})
	if res.Status != "failed" || res.Stage != "status_update" {
		t.Fatalf("result=%+v", res)
	}
	if !strings.Contains(res.Detail, "in_progress") || !strings.Contains(res.Detail, "boom") {
		t.Fatalf("detail=%q", res.Detail)
	}
	if len(commands) != 1 || commands[0] != "git status --porcelain" {
		t.Fatalf("commands=%v", commands)
	}
}

func TestBugFixWorkerFailureStatusFailurePreservesOriginalContext(t *testing.T) {
	tapd := &fakeBugFixTapd{bug: bugFixBugDetail{Title: "panic", Description: "steps"}, failUpdateStatus: true}
	runner := commandRunnerFunc(func(ctx context.Context, cfg commandRunConfig) commandRunResult {
		if cfg.Command == "git status --porcelain" {
			return commandRunResult{Stdout: " M file.go\n"}
		}
		t.Fatalf("agent/test should not run when dirty: %q", cfg.Command)
		return commandRunResult{}
	})
	worker := bugFixWorker{
		tapd:            tapd,
		runner:          runner,
		repo:            "/repo",
		agentCmd:        "agent",
		testCmd:         "go test ./...",
		onFailureStatus: "reopened",
		outputLimit:     1024,
	}
	res := worker.handleTarget(context.Background(), bugEventTarget{WorkspaceID: "123", BugID: "456", EventID: 9})
	if res.Status != "failed" || res.Stage != "status_update" {
		t.Fatalf("result=%+v", res)
	}
	for _, want := range []string{"dirty_repo", " M file.go", "reopened", "boom"} {
		if !strings.Contains(res.Detail, want) {
			t.Fatalf("detail=%q missing %q", res.Detail, want)
		}
	}
}

func TestBugFixWorkerSuccessCommentFailureAttemptsFailureStatus(t *testing.T) {
	tapd := &fakeBugFixTapd{bug: bugFixBugDetail{Title: "panic", Description: "steps"}, failComment: true}
	runner := commandRunnerFunc(func(ctx context.Context, cfg commandRunConfig) commandRunResult {
		switch cfg.Command {
		case "git status --porcelain":
			return commandRunResult{}
		case "agent":
			return commandRunResult{Stdout: "changed"}
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
		onFailureStatus: "reopened",
		outputLimit:     1024,
	}
	res := worker.handleTarget(context.Background(), bugEventTarget{WorkspaceID: "123", BugID: "456", EventID: 9})
	if res.Status != "failed" || res.Stage != "comment" || !res.Verified {
		t.Fatalf("result=%+v", res)
	}
	if len(tapd.updates) != 1 || tapd.updates[0].Status != "reopened" {
		t.Fatalf("updates=%+v", tapd.updates)
	}
}

func TestBugFixWorkerSuccessCommentAndFailureStatusFailureKeepsVerified(t *testing.T) {
	tapd := &fakeBugFixTapd{bug: bugFixBugDetail{Title: "panic", Description: "steps"}, failComment: true, failUpdateStatus: true}
	runner := commandRunnerFunc(func(ctx context.Context, cfg commandRunConfig) commandRunResult {
		switch cfg.Command {
		case "git status --porcelain":
			return commandRunResult{}
		case "agent":
			return commandRunResult{Stdout: "changed"}
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
		onFailureStatus: "reopened",
		outputLimit:     1024,
	}
	res := worker.handleTarget(context.Background(), bugEventTarget{WorkspaceID: "123", BugID: "456", EventID: 9})
	if res.Status != "failed" || res.Stage != "status_update" || !res.Verified {
		t.Fatalf("result=%+v", res)
	}
	for _, want := range []string{"comment", "boom"} {
		if !strings.Contains(res.Detail, want) {
			t.Fatalf("detail=%q missing %q", res.Detail, want)
		}
	}
}

func TestBugFixWorkerDefaultOutputLimitTruncatesFailureDetail(t *testing.T) {
	tapd := &fakeBugFixTapd{bug: bugFixBugDetail{Title: "panic", Description: "steps"}}
	longDirty := strings.Repeat("x", defaultCommandOutputLimit+32)
	runner := commandRunnerFunc(func(ctx context.Context, cfg commandRunConfig) commandRunResult {
		if cfg.Command == "git status --porcelain" {
			return commandRunResult{Stdout: longDirty}
		}
		t.Fatalf("agent/test should not run when dirty: %q", cfg.Command)
		return commandRunResult{}
	})
	worker := bugFixWorker{
		tapd:     tapd,
		runner:   runner,
		repo:     "/repo",
		agentCmd: "agent",
		testCmd:  "go test ./...",
	}
	res := worker.handleTarget(context.Background(), bugEventTarget{WorkspaceID: "123", BugID: "456", EventID: 9})
	if res.Status != "failed" || res.Stage != "dirty_repo" {
		t.Fatalf("result=%+v", res)
	}
	if !strings.Contains(res.Detail, "...[truncated]") {
		t.Fatalf("detail was not truncated: len=%d", len(res.Detail))
	}
	if len(res.Detail) >= len(longDirty) {
		t.Fatalf("detail len=%d, want less than %d", len(res.Detail), len(longDirty))
	}
}

func TestBugFixWorkerDirtyRepoUsesSmallOutputLimit(t *testing.T) {
	tapd := &fakeBugFixTapd{bug: bugFixBugDetail{Title: "panic", Description: "steps"}}
	longDirty := " M " + strings.Repeat("x", 80)
	runner := commandRunnerFunc(func(ctx context.Context, cfg commandRunConfig) commandRunResult {
		if cfg.Command != "git status --porcelain" {
			t.Fatalf("agent/test should not run when dirty: %q", cfg.Command)
			return commandRunResult{}
		}
		if cfg.Limit != 16 {
			t.Fatalf("git status limit=%d, want 16", cfg.Limit)
		}
		return commandRunResult{Stdout: truncateOutput(longDirty, cfg.Limit)}
	})
	worker := bugFixWorker{
		tapd:        tapd,
		runner:      runner,
		repo:        "/repo",
		agentCmd:    "agent",
		testCmd:     "go test ./...",
		outputLimit: 16,
	}
	res := worker.handleTarget(context.Background(), bugEventTarget{WorkspaceID: "123", BugID: "456", EventID: 9})
	if res.Status != "failed" || res.Stage != "dirty_repo" {
		t.Fatalf("result=%+v", res)
	}
	if !strings.Contains(res.Detail, "...[truncated]") || len(res.Detail) >= len(longDirty) {
		t.Fatalf("detail=%q", res.Detail)
	}
	if len(tapd.comments) != 1 || !strings.Contains(tapd.comments[0], "...[truncated]") {
		t.Fatalf("comments=%v", tapd.comments)
	}
}

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
		case strings.Contains(path, "/comments") && r.URL.Query().Get("entry_type") == "stories":
			w.Write([]byte(`{"status":1,"data":[{"Comment":{"id":"s-c1","author":"pm","created":"2026-06-04","description":"<p>MR: http://git.example.com/team/repo/-/merge_requests/99</p>"}}]}`))
		case strings.Contains(path, "/comments"):
			w.Write([]byte(`{"status":1,"data":[{"Comment":{"id":"c1","author":"bob","created":"2026-06-04","description":"<p>please check</p>"}}]}`))
		case strings.Contains(path, "/stories"):
			w.Write([]byte(`{"status":1,"data":[{"Story":{"id":"789","name":"Linked Story","description":"<p>Story Desc</p>","parent_id":"456-parent","status":"open"}}]}`))
		case strings.Contains(path, "/bugs") && r.Method == http.MethodPost:
			w.Write([]byte(`{"status":1,"data":{"Bug":{"id":"456","title":"Test Bug","url":"http://test/bug/456"}}}`))
		default:
			w.Write([]byte(`{"status":1,"data":[{"Bug":{"id":"456","title":"Test Bug","description":"<p>Desc</p>","story_id":"789","status":"new","current_owner":"alice","severity":"normal","priority_label":"high"}}]}`))
		}
	}
	_, cleanup := setupMockServer(t, handler)
	defer cleanup()
	apiClient.SetNick("agent")

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
	if got.StoryID != "789" {
		t.Fatalf("story id=%q", got.StoryID)
	}

	story, err := c.GetStoryDetail(context.Background(), "123", got.StoryID)
	if err != nil {
		t.Fatal(err)
	}
	if story.ID != "789" || story.Title != "Linked Story" || story.ParentID != "456-parent" || len(story.Comments) != 1 {
		t.Fatalf("story detail=%+v", story)
	}
	if !strings.Contains(story.Comments[0].Description, "99") {
		t.Fatalf("story comments=%+v", story.Comments)
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
