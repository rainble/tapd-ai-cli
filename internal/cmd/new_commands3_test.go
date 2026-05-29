// Package cmd 中的 new_commands3_test.go 补充 mock server 单元测试以提升覆盖率
package cmd

import (
	"net/http"
	"os"
	"strings"
	"testing"
)

// newCmds3Handler 统一处理第三批新增命令的 mock API 响应
func newCmds3Handler(w http.ResponseWriter, r *http.Request) {
	_ = r.ParseForm()
	path := r.URL.Path

	switch {
	// ===== comment =====
	case strings.HasSuffix(path, "/comments/count"):
		w.Write([]byte(`{"status":1,"data":{"count":3}}`))
	case strings.HasSuffix(path, "/comments"):
		if r.Method == http.MethodPost {
			w.Write([]byte(`{"status":1,"data":{"Comment":{"id":"1","description":"hello"}}}`))
		} else {
			w.Write([]byte(`{"status":1,"data":[{"Comment":{"id":"1","description":"hello","author":"alice"}}]}`))
		}

	// ===== bug =====
	case strings.HasSuffix(path, "/bugs/count"):
		w.Write([]byte(`{"status":1,"data":{"count":10}}`))
	case strings.HasSuffix(path, "/bugs"):
		if r.Method == http.MethodPost {
			w.Write([]byte(`{"status":1,"data":{"Bug":{"id":"1","title":"new bug","url":"http://test/bug/1"}}}`))
		} else {
			w.Write([]byte(`{"status":1,"data":[{"Bug":{"id":"1","title":"bug1","status":"new"}}]}`))
		}

	// ===== task =====
	case strings.HasSuffix(path, "/tasks/count"):
		w.Write([]byte(`{"status":1,"data":{"count":7}}`))
	case strings.HasSuffix(path, "/tasks"):
		if r.Method == http.MethodPost {
			w.Write([]byte(`{"status":1,"data":{"Task":{"id":"1","name":"new task","url":"http://test/task/1"}}}`))
		} else {
			w.Write([]byte(`{"status":1,"data":[{"Task":{"id":"1","name":"task1","status":"open"}}]}`))
		}

	// ===== story =====
	case strings.HasSuffix(path, "/stories/count"):
		w.Write([]byte(`{"status":1,"data":{"count":5}}`))
	case strings.HasSuffix(path, "/stories"):
		w.Write([]byte(`{"status":1,"data":[{"Story":{"id":"1","name":"story1","status":"open"}}]}`))

	// ===== todo =====
	case strings.HasSuffix(path, "/user_oauth/get_user_todo_story"):
		w.Write([]byte(`{"status":1,"data":[{"Story":{"id":"1","name":"todo story","status":"open"}}]}`))
	case strings.HasSuffix(path, "/user_oauth/get_user_todo_task"):
		w.Write([]byte(`{"status":1,"data":[{"Task":{"id":"1","name":"todo task","status":"open"}}]}`))
	case strings.HasSuffix(path, "/user_oauth/get_user_todo_bug"):
		w.Write([]byte(`{"status":1,"data":[{"Bug":{"id":"1","title":"todo bug","status":"new"}}]}`))

	// ===== custom_field =====
	case strings.Contains(path, "/custom_fields_settings"):
		w.Write([]byte(`{"status":1,"data":[{"CustomFieldConfig":{"custom_field":"custom_field_one","name":"字段","type":"text"}}]}`))
	case strings.HasSuffix(path, "/stories/get_fields_label"):
		w.Write([]byte(`{"status":1,"data":{"Story":{"id":"ID","name":"标题"}}}`))
	case strings.HasSuffix(path, "/stories/get_fields_info"):
		w.Write([]byte(`{"status":1,"data":{"Story":{"id":{"label":"ID","type":"text"}}}}`))
	case strings.HasSuffix(path, "/workitem_types"):
		w.Write([]byte(`{"status":1,"data":[{"WorkitemType":{"id":"1","name":"默认类别"}}]}`))

	// ===== commit_msg =====
	case strings.HasSuffix(path, "/svn_commits/get_scm_copy_keywords"):
		w.Write([]byte(`{"status":1,"data":"story #10001 keyword"}`))

	// ===== category =====
	case strings.HasSuffix(path, "/story_categories"):
		w.Write([]byte(`{"status":1,"data":[{"Category":{"id":"1","name":"cat1"}}]}`))

	// ===== wiki =====
	case strings.HasSuffix(path, "/tapd_wikis/count"):
		w.Write([]byte(`{"status":1,"data":{"count":7}}`))
	case strings.HasSuffix(path, "/tapd_wikis"):
		if r.Method == http.MethodPost {
			w.Write([]byte(`{"status":1,"data":{"Wiki":{"id":"w1","name":"test wiki","url":"http://test/wiki/w1"}}}`))
		} else {
			w.Write([]byte(`{"status":1,"data":[{"Wiki":{"id":"w1","name":"test wiki","creator":"alice"}}]}`))
		}

	// ===== timesheet =====
	case strings.HasSuffix(path, "/timesheets/count"):
		w.Write([]byte(`{"status":1,"data":{"count":4}}`))
	case strings.HasSuffix(path, "/timesheets/delete"):
		w.Write([]byte(`{"status":1,"data":{"num":1}}`))
	case strings.HasSuffix(path, "/timesheets"):
		if r.Method == http.MethodPost {
			w.Write([]byte(`{"status":1,"data":{"Timesheet":{"id":"ts1","timespent":"2h","owner":"alice"}}}`))
		} else {
			w.Write([]byte(`{"status":1,"data":[{"Timesheet":{"id":"ts1","timespent":"2h","owner":"alice","entity_type":"story"}}]}`))
		}

	// ===== workspace =====
	case strings.HasSuffix(path, "/workspaces/user_participant_projects"):
		w.Write([]byte(`{"status":1,"data":[{"Workspace":{"id":"12345","name":"TestWS","pretty_name":"Test Workspace"}}]}`))
	case strings.HasSuffix(path, "/workspaces/get_workspace_info"):
		w.Write([]byte(`{"status":1,"data":{"Workspace":{"id":"12345","name":"TestWS","pretty_name":"Test Workspace"}}}`))
	case strings.HasSuffix(path, "/workspaces/users"):
		w.Write([]byte(`{"status":1,"data":[{"UserWorkspace":{"user":"alice","nick_name":"Alice"}}]}`))
	case strings.HasSuffix(path, "/roles"):
		w.Write([]byte(`{"status":1,"data":{"role_1":"Admin","role_2":"Dev"}}`))

	default:
		w.Write([]byte(`{"status":1,"data":{}}`))
	}
}

// ===================== comment 测试 =====================

func TestNew3RunCommentList(t *testing.T) {
	resetFlags()
	_, cleanup := setupMockServer(t, newCmds3Handler)
	defer cleanup()

	flagEntryType = "stories"
	flagEntryID = "10001"
	flagLimit = 10
	flagPage = 1

	restore, reader := captureStdout(t)
	err := runCommentList(nil, nil)
	restore()
	drainReader(reader)

	if err != nil {
		t.Fatalf("runCommentList failed: %v", err)
	}
}

func TestNew3RunCommentAdd(t *testing.T) {
	resetFlags()
	_, cleanup := setupMockServer(t, newCmds3Handler)
	defer cleanup()

	flagEntryType = "stories"
	flagEntryID = "10001"
	flagDescription = "test comment"
	flagCommentAuthor = "alice"

	restore, reader := captureStdout(t)
	err := runCommentAdd(nil, nil)
	restore()
	drainReader(reader)

	if err != nil {
		t.Fatalf("runCommentAdd failed: %v", err)
	}
}

func TestNew3RunCommentUpdate(t *testing.T) {
	resetFlags()
	_, cleanup := setupMockServer(t, newCmds3Handler)
	defer cleanup()

	flagDescription = "updated comment"

	restore, reader := captureStdout(t)
	err := runCommentUpdate(nil, []string{"1"})
	restore()
	drainReader(reader)

	if err != nil {
		t.Fatalf("runCommentUpdate failed: %v", err)
	}
}

func TestNew3RunCommentCount(t *testing.T) {
	resetFlags()
	_, cleanup := setupMockServer(t, newCmds3Handler)
	defer cleanup()

	flagEntryType = "stories"
	flagEntryID = "10001"

	restore, reader := captureStdout(t)
	err := runCommentCount(nil, nil)
	restore()
	drainReader(reader)

	if err != nil {
		t.Fatalf("runCommentCount failed: %v", err)
	}
}

// ===================== bug 测试 =====================

func TestNew3RunBugCount(t *testing.T) {
	resetFlags()
	_, cleanup := setupMockServer(t, newCmds3Handler)
	defer cleanup()

	restore, reader := captureStdout(t)
	err := runBugCount(nil, nil)
	restore()
	drainReader(reader)

	if err != nil {
		t.Fatalf("runBugCount failed: %v", err)
	}
}

func TestNew3RunBugTodo(t *testing.T) {
	resetFlags()
	_, cleanup := setupMockServer(t, newCmds3Handler)
	defer cleanup()

	flagLimit = 10
	flagPage = 1

	restore, reader := captureStdout(t)
	err := runBugTodo(nil, nil)
	restore()
	drainReader(reader)

	if err != nil {
		t.Fatalf("runBugTodo failed: %v", err)
	}
}

func TestNew3RunBugList(t *testing.T) {
	resetFlags()
	_, cleanup := setupMockServer(t, newCmds3Handler)
	defer cleanup()

	flagLimit = 10
	flagPage = 1

	restore, reader := captureStdout(t)
	err := runBugList(nil, nil)
	restore()
	drainReader(reader)

	if err != nil {
		t.Fatalf("runBugList failed: %v", err)
	}
}

func TestNew3RunBugCreate(t *testing.T) {
	resetFlags()
	_, cleanup := setupMockServer(t, newCmds3Handler)
	defer cleanup()

	flagTitle = "test bug"
	flagDescription = "bug desc"

	restore, reader := captureStdout(t)
	err := runBugCreate(nil, nil)
	restore()
	drainReader(reader)

	if err != nil {
		t.Fatalf("runBugCreate failed: %v", err)
	}
}

func TestNew3RunBugUpdate(t *testing.T) {
	resetFlags()
	_, cleanup := setupMockServer(t, newCmds3Handler)
	defer cleanup()

	flagTitle = "updated bug"

	restore, reader := captureStdout(t)
	err := runBugUpdate(nil, []string{"1"})
	restore()
	drainReader(reader)

	if err != nil {
		t.Fatalf("runBugUpdate failed: %v", err)
	}
}

// ===================== task 测试 =====================

func TestNew3RunTaskCount(t *testing.T) {
	resetFlags()
	_, cleanup := setupMockServer(t, newCmds3Handler)
	defer cleanup()

	restore, reader := captureStdout(t)
	err := runTaskCount(nil, nil)
	restore()
	drainReader(reader)

	if err != nil {
		t.Fatalf("runTaskCount failed: %v", err)
	}
}

func TestNew3RunTaskTodo(t *testing.T) {
	resetFlags()
	_, cleanup := setupMockServer(t, newCmds3Handler)
	defer cleanup()

	flagLimit = 10
	flagPage = 1

	restore, reader := captureStdout(t)
	err := runTaskTodo(nil, nil)
	restore()
	drainReader(reader)

	if err != nil {
		t.Fatalf("runTaskTodo failed: %v", err)
	}
}

func TestNew3RunTaskList(t *testing.T) {
	resetFlags()
	_, cleanup := setupMockServer(t, newCmds3Handler)
	defer cleanup()

	flagLimit = 10
	flagPage = 1

	restore, reader := captureStdout(t)
	err := runTaskList(nil, nil)
	restore()
	drainReader(reader)

	if err != nil {
		t.Fatalf("runTaskList failed: %v", err)
	}
}

func TestNew3RunTaskCreate(t *testing.T) {
	resetFlags()
	_, cleanup := setupMockServer(t, newCmds3Handler)
	defer cleanup()

	flagName = "test task"
	flagDescription = "task desc"

	restore, reader := captureStdout(t)
	err := runTaskCreate(nil, nil)
	restore()
	drainReader(reader)

	if err != nil {
		t.Fatalf("runTaskCreate failed: %v", err)
	}
}

// ===================== custom_field 测试 =====================

func TestNew3RunCustomFieldList(t *testing.T) {
	resetFlags()
	_, cleanup := setupMockServer(t, newCmds3Handler)
	defer cleanup()

	flagEntityType = "stories"

	restore, reader := captureStdout(t)
	err := runCustomFieldList(nil, nil)
	restore()
	drainReader(reader)

	if err != nil {
		t.Fatalf("runCustomFieldList failed: %v", err)
	}
}

func TestNew3RunStoryFieldLabel(t *testing.T) {
	resetFlags()
	_, cleanup := setupMockServer(t, newCmds3Handler)
	defer cleanup()

	restore, reader := captureStdout(t)
	err := runStoryFieldLabel(nil, nil)
	restore()
	drainReader(reader)

	if err != nil {
		t.Fatalf("runStoryFieldLabel failed: %v", err)
	}
}

func TestNew3RunStoryFieldInfo(t *testing.T) {
	resetFlags()
	_, cleanup := setupMockServer(t, newCmds3Handler)
	defer cleanup()

	restore, reader := captureStdout(t)
	err := runStoryFieldInfo(nil, nil)
	restore()
	drainReader(reader)

	if err != nil {
		t.Fatalf("runStoryFieldInfo failed: %v", err)
	}
}

func TestNew3RunWorkitemTypeList(t *testing.T) {
	resetFlags()
	_, cleanup := setupMockServer(t, newCmds3Handler)
	defer cleanup()

	restore, reader := captureStdout(t)
	err := runWorkitemTypeList(nil, nil)
	restore()
	drainReader(reader)

	if err != nil {
		t.Fatalf("runWorkitemTypeList failed: %v", err)
	}
}

// ===================== commit_msg 测试 =====================

func TestNew3RunCommitMsgGet(t *testing.T) {
	resetFlags()
	_, cleanup := setupMockServer(t, newCmds3Handler)
	defer cleanup()

	flagCommitMsgObjectID = "10001"
	flagCommitMsgType = "story"

	restore, reader := captureStdout(t)
	err := runCommitMsgGet(nil, nil)
	restore()
	drainReader(reader)

	if err != nil {
		t.Fatalf("runCommitMsgGet failed: %v", err)
	}
}

// ===================== category 测试 =====================

func TestNew3RunCategoryList(t *testing.T) {
	resetFlags()
	_, cleanup := setupMockServer(t, newCmds3Handler)
	defer cleanup()

	restore, reader := captureStdout(t)
	err := runCategoryList(nil, nil)
	restore()
	drainReader(reader)

	if err != nil {
		t.Fatalf("runCategoryList failed: %v", err)
	}
}

// ===================== story 测试 =====================

func TestNew3RunStoryList(t *testing.T) {
	resetFlags()
	_, cleanup := setupMockServer(t, newCmds3Handler)
	defer cleanup()

	flagLimit = 10
	flagPage = 1

	restore, reader := captureStdout(t)
	err := runStoryList(nil, nil)
	restore()
	drainReader(reader)

	if err != nil {
		t.Fatalf("runStoryList failed: %v", err)
	}
}

func TestNew3RunStoryCount(t *testing.T) {
	resetFlags()
	_, cleanup := setupMockServer(t, newCmds3Handler)
	defer cleanup()

	restore, reader := captureStdout(t)
	err := runStoryCount(nil, nil)
	restore()
	drainReader(reader)

	if err != nil {
		t.Fatalf("runStoryCount failed: %v", err)
	}
}

func TestNew3RunStoryTodo(t *testing.T) {
	resetFlags()
	_, cleanup := setupMockServer(t, newCmds3Handler)
	defer cleanup()

	flagLimit = 10
	flagPage = 1

	restore, reader := captureStdout(t)
	err := runStoryTodo(nil, nil)
	restore()
	drainReader(reader)

	if err != nil {
		t.Fatalf("runStoryTodo failed: %v", err)
	}
}

// ===================== wiki 测试 =====================

func TestNew3RunWikiList(t *testing.T) {
	resetFlags()
	_, cleanup := setupMockServer(t, newCmds3Handler)
	defer cleanup()

	flagLimit = 10
	flagPage = 1

	restore, reader := captureStdout(t)
	err := runWikiList(nil, nil)
	restore()
	drainReader(reader)

	if err != nil {
		t.Fatalf("runWikiList failed: %v", err)
	}
}

func TestNew3RunWikiShow(t *testing.T) {
	resetFlags()
	_, cleanup := setupMockServer(t, newCmds3Handler)
	defer cleanup()
	flagJSON = true

	restore, reader := captureStdout(t)
	err := runWikiShow(nil, []string{"w1"})
	restore()
	drainReader(reader)

	if err != nil {
		t.Fatalf("runWikiShow failed: %v", err)
	}
}

func TestNew3RunWikiCreate(t *testing.T) {
	resetFlags()
	_, cleanup := setupMockServer(t, newCmds3Handler)
	defer cleanup()

	flagWikiName = "test wiki"
	flagCreator = "alice"
	flagWikiContent = "# Hello"

	restore, reader := captureStdout(t)
	err := runWikiCreate(nil, nil)
	restore()
	drainReader(reader)

	if err != nil {
		t.Fatalf("runWikiCreate failed: %v", err)
	}
}

func TestNew3RunWikiUpdate(t *testing.T) {
	resetFlags()
	_, cleanup := setupMockServer(t, newCmds3Handler)
	defer cleanup()

	flagWikiContent = "# Updated"

	restore, reader := captureStdout(t)
	err := runWikiUpdate(nil, []string{"w1"})
	restore()
	drainReader(reader)

	if err != nil {
		t.Fatalf("runWikiUpdate failed: %v", err)
	}
}

func TestNew3ReadWikiContent_Direct(t *testing.T) {
	resetFlags()
	flagWikiContent = "direct content"
	flagWikiFile = ""

	content, err := readWikiContent()
	if err != nil {
		t.Fatalf("readWikiContent failed: %v", err)
	}
	if content != "direct content" {
		t.Errorf("expected 'direct content', got %q", content)
	}
}

func TestNew3ReadWikiContent_File(t *testing.T) {
	resetFlags()
	tmpFile := t.TempDir() + "/wiki.md"
	os.WriteFile(tmpFile, []byte("file content"), 0644)
	flagWikiContent = ""
	flagWikiFile = tmpFile

	content, err := readWikiContent()
	if err != nil {
		t.Fatalf("readWikiContent failed: %v", err)
	}
	if content != "file content" {
		t.Errorf("expected 'file content', got %q", content)
	}
}

// ===================== timesheet 测试 =====================

func TestNew3RunTimesheetAdd(t *testing.T) {
	resetFlags()
	_, cleanup := setupMockServer(t, newCmds3Handler)
	defer cleanup()

	flagTimesheetEntityType = "story"
	flagTimesheetEntityID = "10001"
	flagTimesheetSpent = "2h"
	flagTimesheetOwner = "alice"

	restore, reader := captureStdout(t)
	err := runTimesheetAdd(nil, nil)
	restore()
	drainReader(reader)

	if err != nil {
		t.Fatalf("runTimesheetAdd failed: %v", err)
	}
}

func TestNew3RunTimesheetUpdate(t *testing.T) {
	resetFlags()
	_, cleanup := setupMockServer(t, newCmds3Handler)
	defer cleanup()

	flagTimesheetSpent = "4h"

	restore, reader := captureStdout(t)
	err := runTimesheetUpdate(nil, []string{"ts1"})
	restore()
	drainReader(reader)

	if err != nil {
		t.Fatalf("runTimesheetUpdate failed: %v", err)
	}
}

func TestNew3RunTimesheetList(t *testing.T) {
	resetFlags()
	_, cleanup := setupMockServer(t, newCmds3Handler)
	defer cleanup()

	flagLimit = 10
	flagPage = 1

	restore, reader := captureStdout(t)
	err := runTimesheetList(nil, nil)
	restore()
	drainReader(reader)

	if err != nil {
		t.Fatalf("runTimesheetList failed: %v", err)
	}
}

func TestNew3RunTimesheetCount(t *testing.T) {
	resetFlags()
	_, cleanup := setupMockServer(t, newCmds3Handler)
	defer cleanup()

	restore, reader := captureStdout(t)
	err := runTimesheetCount(nil, nil)
	restore()
	drainReader(reader)

	if err != nil {
		t.Fatalf("runTimesheetCount failed: %v", err)
	}
}

func TestNew3RunTimesheetDelete(t *testing.T) {
	resetFlags()
	_, cleanup := setupMockServer(t, newCmds3Handler)
	defer cleanup()

	flagTimesheetEntityType = "story"
	flagTimesheetEntityID = "10001"
	flagTimesheetCostIDs = "ts1,ts2"

	restore, reader := captureStdout(t)
	err := runTimesheetDelete(nil, nil)
	restore()
	drainReader(reader)

	if err != nil {
		t.Fatalf("runTimesheetDelete failed: %v", err)
	}
}

// ===================== workspace 测试 =====================

func TestNew3RunWorkspaceList(t *testing.T) {
	resetFlags()
	_, cleanup := setupMockServer(t, newCmds3Handler)
	defer cleanup()

	restore, reader := captureStdout(t)
	err := runWorkspaceList(nil, nil)
	restore()
	drainReader(reader)

	if err != nil {
		t.Fatalf("runWorkspaceList failed: %v", err)
	}
}

func TestNew3RunWorkspaceInfo(t *testing.T) {
	resetFlags()
	_, cleanup := setupMockServer(t, newCmds3Handler)
	defer cleanup()

	restore, reader := captureStdout(t)
	err := runWorkspaceInfo(nil, nil)
	restore()
	drainReader(reader)

	if err != nil {
		t.Fatalf("runWorkspaceInfo failed: %v", err)
	}
}

func TestNew3RunWorkspaceUsers(t *testing.T) {
	resetFlags()
	_, cleanup := setupMockServer(t, newCmds3Handler)
	defer cleanup()

	restore, reader := captureStdout(t)
	err := runWorkspaceUsers(nil, nil)
	restore()
	drainReader(reader)

	if err != nil {
		t.Fatalf("runWorkspaceUsers failed: %v", err)
	}
}

func TestNew3RunWorkspaceRoles(t *testing.T) {
	resetFlags()
	_, cleanup := setupMockServer(t, newCmds3Handler)
	defer cleanup()

	restore, reader := captureStdout(t)
	err := runWorkspaceRoles(nil, nil)
	restore()
	drainReader(reader)

	if err != nil {
		t.Fatalf("runWorkspaceRoles failed: %v", err)
	}
}
