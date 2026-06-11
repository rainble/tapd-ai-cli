package cmd

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestProjectManagementReportSummarizesProgress(t *testing.T) {
	report := buildProjectManagementReport(projectManagementSnapshot{
		WorkspaceID: "12345",
		IterationID: "it-1",
		Stories: []projectStoryItem{
			{ID: "s1", Name: "需求 A", Status: "done", Owner: "alice"},
			{ID: "s2", Name: "需求 B", Status: "developing", Owner: "bob"},
		},
		Tasks: []projectTaskItem{
			{ID: "t1", Name: "开发任务", Status: "done", Owner: "alice", StoryID: "s1"},
			{ID: "t2", Name: "联调任务", Status: "progressing", Owner: "bob", StoryID: "s2"},
		},
		Bugs: []projectBugItem{
			{ID: "b1", Title: "已关闭缺陷", Status: "closed", CurrentOwner: "qa"},
			{ID: "b2", Title: "待修复缺陷", Status: "new", CurrentOwner: "bob"},
		},
	}, projectManagementOptions{
		Mode:      "progress",
		StaleDays: 7,
		Now:       time.Date(2026, 6, 9, 12, 0, 0, 0, time.Local),
	})

	if report.ProgressPercent != 50 {
		t.Fatalf("ProgressPercent=%d, want 50; report=%+v", report.ProgressPercent, report)
	}
	if report.StoryTotal != 2 || report.TaskTotal != 2 || report.BugTotal != 2 {
		t.Fatalf("unexpected totals: %+v", report)
	}
	if report.OpenBugCount != 1 {
		t.Fatalf("OpenBugCount=%d, want 1", report.OpenBugCount)
	}
	if report.StoryStatus["done"] != 1 || report.TaskStatus["progressing"] != 1 || report.BugStatus["new"] != 1 {
		t.Fatalf("unexpected status maps: %+v", report)
	}
}

func TestProjectManagementReportDetectsBlockers(t *testing.T) {
	report := buildProjectManagementReport(projectManagementSnapshot{
		WorkspaceID: "12345",
		IterationID: "it-1",
		Stories: []projectStoryItem{
			{ID: "s1", Name: "无负责人需求", Status: "open", Modified: "2026-05-20 10:00:00"},
		},
		Tasks: []projectTaskItem{
			{ID: "t1", Name: "超期任务", Status: "progressing", Owner: "alice", Due: "2026-06-01"},
		},
		Bugs: []projectBugItem{
			{ID: "b1", Title: "无人处理缺陷", Status: "new", Modified: "2026-05-28 10:00:00"},
		},
	}, projectManagementOptions{
		Mode:      "blockers",
		StaleDays: 7,
		Now:       time.Date(2026, 6, 9, 12, 0, 0, 0, time.Local),
	})

	if !projectIssueCodes(report.Blockers).Has("missing_story_owner") {
		t.Fatalf("missing missing_story_owner blocker: %+v", report.Blockers)
	}
	if !projectIssueCodes(report.Blockers).Has("overdue_task") {
		t.Fatalf("missing overdue_task blocker: %+v", report.Blockers)
	}
	if !projectIssueCodes(report.Blockers).Has("missing_bug_owner") {
		t.Fatalf("missing missing_bug_owner blocker: %+v", report.Blockers)
	}
	if !projectIssueCodes(report.Risks).Has("stale_story") {
		t.Fatalf("missing stale_story risk: %+v", report.Risks)
	}
}

func TestProjectManagementStatusSuggestKeepsInProgressWithBlockers(t *testing.T) {
	report := buildProjectManagementReport(projectManagementSnapshot{
		WorkspaceID: "12345",
		IterationID: "it-1",
		Stories: []projectStoryItem{
			{ID: "s1", Name: "需求", Status: "done", Owner: "alice"},
		},
		Tasks: []projectTaskItem{
			{ID: "t1", Name: "任务", Status: "done", Owner: "alice"},
		},
		Bugs: []projectBugItem{
			{ID: "b1", Title: "待修复缺陷", Status: "new", CurrentOwner: "alice"},
		},
	}, projectManagementOptions{
		Mode:      "status-suggest",
		StaleDays: 7,
		Now:       time.Date(2026, 6, 9, 12, 0, 0, 0, time.Local),
	})

	if report.StatusSuggestion.Decision != "keep_in_progress" {
		t.Fatalf("decision=%q, want keep_in_progress; suggestion=%+v", report.StatusSuggestion.Decision, report.StatusSuggestion)
	}
	if !strings.Contains(report.StatusSuggestion.Reason, "open bug") {
		t.Fatalf("reason should mention open bug: %+v", report.StatusSuggestion)
	}
}

func TestRunAssistantProjectProgressLoadsIterationItems(t *testing.T) {
	resetFlags()
	t.Cleanup(resetFlags)

	var sawStories, sawTasks, sawBugs bool
	handler := func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("iteration_id"); got != "it-1" {
			t.Fatalf("%s iteration_id=%q, want it-1", r.URL.Path, got)
		}
		switch {
		case strings.Contains(r.URL.Path, "/stories"):
			sawStories = true
			w.Write([]byte(`{"status":1,"data":[{"Story":{"id":"s1","name":"需求","status":"done","owner":"alice","modified":"2026-06-08 10:00:00"}}]}`))
		case strings.Contains(r.URL.Path, "/tasks"):
			sawTasks = true
			w.Write([]byte(`{"status":1,"data":[{"Task":{"id":"t1","name":"任务","status":"done","owner":"alice","story_id":"s1","modified":"2026-06-08 10:00:00"}}]}`))
		case strings.Contains(r.URL.Path, "/bugs"):
			sawBugs = true
			w.Write([]byte(`{"status":1,"data":[{"Bug":{"id":"b1","title":"缺陷","status":"closed","current_owner":"qa","modified":"2026-06-08 10:00:00"}}]}`))
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}
	_, cleanup := setupMockServer(t, handler)
	defer cleanup()
	flagAssistantProjectIterationID = "it-1"

	restore, reader := captureStdout(t)
	err := runAssistantProjectReport(context.Background(), "progress")
	restore()
	out, _ := io.ReadAll(reader)
	_ = reader.Close()
	if err != nil {
		t.Fatal(err)
	}
	if !sawStories || !sawTasks || !sawBugs {
		t.Fatalf("missing API calls: stories=%v tasks=%v bugs=%v", sawStories, sawTasks, sawBugs)
	}
	text := string(out)
	for _, want := range []string{"Project Progress Report", "Stories: 1", "Tasks: 1", "Bugs: 1", "Progress: 100%"} {
		if !strings.Contains(text, want) {
			t.Fatalf("output missing %q:\n%s", want, text)
		}
	}
}

type projectIssueCodeSet []projectIssue

func projectIssueCodes(issues []projectIssue) projectIssueCodeSet {
	return projectIssueCodeSet(issues)
}

func (s projectIssueCodeSet) Has(code string) bool {
	for _, issue := range s {
		if issue.Code == code {
			return true
		}
	}
	return false
}
