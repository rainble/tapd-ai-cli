// Package cmd 中的 new_commands_test.go 为新增命令提供 mock server 单元测试
package cmd

import (
	"net/http"
	"strings"
	"testing"
)

// newCmdsHandler 统一处理新增命令的 mock API 响应
func newCmdsHandler(w http.ResponseWriter, r *http.Request) {
	_ = r.ParseForm()
	path := r.URL.Path

	switch {
	// ===== change =====
	case strings.HasSuffix(path, "/story_changes/count"):
		w.Write([]byte(`{"status":1,"data":{"count":3}}`))
	case strings.HasSuffix(path, "/bug_changes/count"):
		w.Write([]byte(`{"status":1,"data":{"count":2}}`))
	case strings.HasSuffix(path, "/task_changes/count"):
		w.Write([]byte(`{"status":1,"data":{"count":1}}`))
	case strings.HasSuffix(path, "/story_changes"):
		w.Write([]byte(`{"status":1,"data":[{"WorkitemChange":{"id":"c1","field":"status","old_value":"open","new_value":"done"}}]}`))
	case strings.HasSuffix(path, "/bug_changes"):
		w.Write([]byte(`{"status":1,"data":[{"BugChange":{"id":"c2","field":"status","old_value":"new","new_value":"fixed"}}]}`))
	case strings.HasSuffix(path, "/task_changes"):
		w.Write([]byte(`{"status":1,"data":[{"WorkitemChange":{"id":"c3","field":"owner","old_value":"alice","new_value":"bob"}}]}`))
	case strings.HasSuffix(path, "/iteration_changes"):
		w.Write([]byte(`{"status":1,"data":[{"IterationChange":{"id":"c4","field":"name","old_value":"S1","new_value":"S2"}}]}`))

	// ===== label =====
	case strings.HasSuffix(path, "/label_pools/count"):
		w.Write([]byte(`{"status":1,"data":{"count":5}}`))
	case strings.HasSuffix(path, "/label/count"):
		w.Write([]byte(`{"status":1,"data":{"count":5}}`))
	case strings.HasSuffix(path, "/label_pools"):
		w.Write([]byte(`{"status":1,"data":[{"LabelPool":{"id":"lbl001","name":"TestLabel","color":"1"}}]}`))
	case path == "/label" || strings.HasSuffix(path, "/label"):
		if r.Method == http.MethodPost {
			w.Write([]byte(`{"status":1,"data":{"LabelPool":{"id":"lbl001","name":"TestLabel","color":"1"}}}`))
		} else {
			w.Write([]byte(`{"status":1,"data":[{"LabelPool":{"id":"lbl001","name":"TestLabel","color":"1"}}]}`))
		}

	// ===== user =====
	case strings.HasSuffix(path, "/users/info"):
		w.Write([]byte(`{"status":1,"data":{"nick":"test","name":"Test User"}}`))
	case strings.HasSuffix(path, "/user_oauth/get_user_view_list"):
		w.Write([]byte(`{"status":1,"data":{"view1":{"id":"v1","name":"我的需求"}}}`))

	// ===== workspace extensions =====
	case strings.HasSuffix(path, "/workspaces/users"):
		w.Write([]byte(`{"status":1,"data":[{"UserWorkspace":{"user":"alice","nick_name":"Alice"}}]}`))
	case strings.HasSuffix(path, "/roles"):
		w.Write([]byte(`{"status":1,"data":{"role_1":"Admin","role_2":"Dev"}}`))
	case strings.HasSuffix(path, "/workspaces/sub_workspaces"):
		w.Write([]byte(`{"status":1,"data":{"Workspace":{"id":"99999","name":"SubProject"}}}`))
	case strings.HasSuffix(path, "/documents/get_workspace_documents"):
		w.Write([]byte(`{"status":1,"data":[{"Document":{"id":"doc1","title":"README"}}]}`))
	case strings.HasSuffix(path, "/workspaces/add_workspace_member"):
		w.Write([]byte(`{"status":1,"data":{"success":true}}`))
	case strings.HasSuffix(path, "/workspaces/get_workspace_setting"):
		w.Write([]byte(`{"status":1,"data":{"value":"1"}}`))

	// ===== story extras =====
	case strings.HasSuffix(path, "/stories/copy_story"):
		w.Write([]byte(`{"status":1,"data":{"Story":{"id":"10002","name":"Copied Story"}}}`))
	case strings.HasSuffix(path, "/stories/get_link_stories"):
		w.Write([]byte(`{"status":1,"data":[{"story_id":"10001","related_story_id":"10002"}]}`))
	case strings.HasSuffix(path, "/stories/add_story_link_relations"):
		w.Write([]byte(`{"status":1,"data":{"success":1}}`))
	case strings.HasSuffix(path, "/stories/remove_story_link_relation"):
		w.Write([]byte(`{"status":1,"data":{"success":1}}`))

	// ===== bug extras =====
	case strings.HasSuffix(path, "/bugs/copy_bug"):
		w.Write([]byte(`{"status":1,"data":{"Bug":{"id":"20002","title":"Copied Bug"}}}`))
	case strings.HasSuffix(path, "/bugs/get_link_bugs"):
		w.Write([]byte(`{"status":1,"data":[{"bug_id":"20001","related_bug_id":"20002","link_id":"lk1"}]}`))
	case strings.HasSuffix(path, "/bugs/link_bugs"):
		w.Write([]byte(`{"status":1,"data":true}`))
	case strings.HasSuffix(path, "/bugs/delete_link_bugs"):
		w.Write([]byte(`{"status":1,"data":true}`))
	case strings.HasSuffix(path, "/bugs/get_related_stories"):
		w.Write([]byte(`{"status":1,"data":[{"bug_id":"20001","story_id":"10001"}]}`))
	case strings.HasSuffix(path, "/bugs/template_list"):
		w.Write([]byte(`{"status":1,"data":[{"WorkitemTemplate":{"id":"tpl1","name":"default"}}]}`))

	// ===== test plan =====
	case strings.HasSuffix(path, "/test_plans/count"):
		w.Write([]byte(`{"status":1,"data":{"count":3}}`))
	case strings.HasSuffix(path, "/test_plans/progress"):
		w.Write([]byte(`{"status":1,"data":{"total":10,"passed":8,"failed":1,"not_run":1}}`))
	case strings.HasSuffix(path, "/test_plans"):
		if r.Method == http.MethodPost {
			w.Write([]byte(`{"status":1,"data":{"TestPlan":{"id":"tp001","name":"Test Plan 1"}}}`))
		} else {
			w.Write([]byte(`{"status":1,"data":[{"TestPlan":{"id":"tp001","name":"Test Plan 1","status":"open"}}]}`))
		}

	// ===== module =====
	case strings.HasSuffix(path, "/modules/count"):
		w.Write([]byte(`{"status":1,"data":{"count":4}}`))
	case strings.HasSuffix(path, "/modules"):
		if r.Method == http.MethodPost {
			w.Write([]byte(`{"status":1,"data":{"Module":{"id":"mod001","name":"ModuleA"}}}`))
		} else {
			w.Write([]byte(`{"status":1,"data":[{"Module":{"id":"mod001","name":"ModuleA"}}]}`))
		}

	// ===== tcase =====
	case strings.HasSuffix(path, "/tcases/count"):
		w.Write([]byte(`{"status":1,"data":{"count":6}}`))
	case strings.HasSuffix(path, "/tcases"):
		if r.Method == http.MethodPost {
			w.Write([]byte(`{"status":1,"data":{"Tcase":{"id":"tc001","name":"Test Case 1"}}}`))
		} else {
			w.Write([]byte(`{"status":1,"data":[{"Tcase":{"id":"tc001","name":"Test Case 1","status":"normal"}}]}`))
		}

	// ===== timesheet =====
	case strings.HasSuffix(path, "/timesheets/count"):
		w.Write([]byte(`{"status":1,"data":{"count":7}}`))
	case strings.HasSuffix(path, "/timesheets/delete_timesheets"):
		w.Write([]byte(`{"status":1,"data":{"success":true}}`))
	case strings.HasSuffix(path, "/timesheets"):
		if r.Method == http.MethodPost {
			w.Write([]byte(`{"status":1,"data":{"Timesheet":{"id":"ts001","timespent":"2h"}}}`))
		} else {
			w.Write([]byte(`{"status":1,"data":[{"Timesheet":{"id":"ts001","timespent":"2h","owner":"alice"}}]}`))
		}

	// ===== attachment =====
	case strings.HasSuffix(path, "/attachments/down"):
		w.Write([]byte(`{"status":1,"data":{"Attachment":{"id":"att001","filename":"test.pdf","download_url":"http://example.com/test.pdf"}}}`))
	case strings.HasSuffix(path, "/attachments"):
		w.Write([]byte(`{"status":1,"data":[{"Attachment":{"id":"att001","filename":"test.pdf"}}]}`))

	default:
		w.Write([]byte(`{"status":1,"data":{}}`))
	}
}

// ===================== Change 命令测试 =====================

func TestNewRunChangeList_Story(t *testing.T) {
	resetFlags()
	_, cleanup := setupMockServer(t, newCmdsHandler)
	defer cleanup()

	flagChangeType = "story"
	flagChangeEntityID = "10001"

	restore, reader := captureStdout(t)
	err := runChangeList(nil, nil)
	restore()
	drainReader(reader)

	if err != nil {
		t.Fatalf("runChangeList story failed: %v", err)
	}
}

func TestNewRunChangeList_Bug(t *testing.T) {
	resetFlags()
	_, cleanup := setupMockServer(t, newCmdsHandler)
	defer cleanup()

	flagChangeType = "bug"
	flagChangeEntityID = "20001"

	restore, reader := captureStdout(t)
	err := runChangeList(nil, nil)
	restore()
	drainReader(reader)

	if err != nil {
		t.Fatalf("runChangeList bug failed: %v", err)
	}
}

func TestNewRunChangeList_Task(t *testing.T) {
	resetFlags()
	_, cleanup := setupMockServer(t, newCmdsHandler)
	defer cleanup()

	flagChangeType = "task"
	flagChangeEntityID = "30001"

	restore, reader := captureStdout(t)
	err := runChangeList(nil, nil)
	restore()
	drainReader(reader)

	if err != nil {
		t.Fatalf("runChangeList task failed: %v", err)
	}
}

func TestNewRunChangeList_Iteration(t *testing.T) {
	resetFlags()
	_, cleanup := setupMockServer(t, newCmdsHandler)
	defer cleanup()

	flagChangeType = "iteration"
	flagChangeEntityID = "40001"

	restore, reader := captureStdout(t)
	err := runChangeList(nil, nil)
	restore()
	drainReader(reader)

	if err != nil {
		t.Fatalf("runChangeList iteration failed: %v", err)
	}
}

func TestNewRunChangeCount_Story(t *testing.T) {
	resetFlags()
	_, cleanup := setupMockServer(t, newCmdsHandler)
	defer cleanup()

	flagChangeType = "story"

	restore, reader := captureStdout(t)
	err := runChangeCount(nil, nil)
	restore()
	drainReader(reader)

	if err != nil {
		t.Fatalf("runChangeCount story failed: %v", err)
	}
}

func TestNewRunChangeCount_Bug(t *testing.T) {
	resetFlags()
	_, cleanup := setupMockServer(t, newCmdsHandler)
	defer cleanup()

	flagChangeType = "bug"

	restore, reader := captureStdout(t)
	err := runChangeCount(nil, nil)
	restore()
	drainReader(reader)

	if err != nil {
		t.Fatalf("runChangeCount bug failed: %v", err)
	}
}

func TestNewRunChangeCount_Task(t *testing.T) {
	resetFlags()
	_, cleanup := setupMockServer(t, newCmdsHandler)
	defer cleanup()

	flagChangeType = "task"

	restore, reader := captureStdout(t)
	err := runChangeCount(nil, nil)
	restore()
	drainReader(reader)

	if err != nil {
		t.Fatalf("runChangeCount task failed: %v", err)
	}
}

// ===================== Label 命令测试 =====================

func TestNewRunLabelList(t *testing.T) {
	resetFlags()
	_, cleanup := setupMockServer(t, newCmdsHandler)
	defer cleanup()

	restore, reader := captureStdout(t)
	err := runLabelList(nil, nil)
	restore()
	drainReader(reader)

	if err != nil {
		t.Fatalf("runLabelList failed: %v", err)
	}
}

func TestNewRunLabelAdd(t *testing.T) {
	resetFlags()
	_, cleanup := setupMockServer(t, newCmdsHandler)
	defer cleanup()

	flagName = "NewTag"
	flagLabelColor = "2"

	restore, reader := captureStdout(t)
	err := runLabelAdd(nil, nil)
	restore()
	drainReader(reader)

	if err != nil {
		t.Fatalf("runLabelAdd failed: %v", err)
	}
}

func TestNewRunLabelUpdate(t *testing.T) {
	resetFlags()
	_, cleanup := setupMockServer(t, newCmdsHandler)
	defer cleanup()

	flagLabelColor = "3"

	restore, reader := captureStdout(t)
	err := runLabelUpdate(nil, []string{"lbl001"})
	restore()
	drainReader(reader)

	if err != nil {
		t.Fatalf("runLabelUpdate failed: %v", err)
	}
}

func TestNewRunLabelCount(t *testing.T) {
	resetFlags()
	_, cleanup := setupMockServer(t, newCmdsHandler)
	defer cleanup()

	restore, reader := captureStdout(t)
	err := runLabelCount(nil, nil)
	restore()
	drainReader(reader)

	if err != nil {
		t.Fatalf("runLabelCount failed: %v", err)
	}
}

// ===================== User 命令测试 =====================

func TestNewRunUserInfo(t *testing.T) {
	resetFlags()
	_, cleanup := setupMockServer(t, newCmdsHandler)
	defer cleanup()

	restore, reader := captureStdout(t)
	err := runUserInfo(nil, nil)
	restore()
	drainReader(reader)

	if err != nil {
		t.Fatalf("runUserInfo failed: %v", err)
	}
}

func TestNewRunUserViews(t *testing.T) {
	resetFlags()
	_, cleanup := setupMockServer(t, newCmdsHandler)
	defer cleanup()

	flagViewType = "story"

	restore, reader := captureStdout(t)
	err := runUserViews(nil, nil)
	restore()
	drainReader(reader)

	if err != nil {
		t.Fatalf("runUserViews failed: %v", err)
	}
}

// ===================== Workspace 扩展命令测试 =====================

func TestNewRunWorkspaceUsers(t *testing.T) {
	resetFlags()
	_, cleanup := setupMockServer(t, newCmdsHandler)
	defer cleanup()

	restore, reader := captureStdout(t)
	err := runWorkspaceUsers(nil, nil)
	restore()
	drainReader(reader)

	if err != nil {
		t.Fatalf("runWorkspaceUsers failed: %v", err)
	}
}

func TestNewRunWorkspaceRoles(t *testing.T) {
	resetFlags()
	_, cleanup := setupMockServer(t, newCmdsHandler)
	defer cleanup()

	restore, reader := captureStdout(t)
	err := runWorkspaceRoles(nil, nil)
	restore()
	drainReader(reader)

	if err != nil {
		t.Fatalf("runWorkspaceRoles failed: %v", err)
	}
}

func TestNewRunWorkspaceSubWorkspaces(t *testing.T) {
	resetFlags()
	_, cleanup := setupMockServer(t, newCmdsHandler)
	defer cleanup()

	restore, reader := captureStdout(t)
	err := runWorkspaceSubWorkspaces(nil, nil)
	restore()
	drainReader(reader)

	if err != nil {
		t.Fatalf("runWorkspaceSubWorkspaces failed: %v", err)
	}
}

func TestNewRunWorkspaceDocuments(t *testing.T) {
	resetFlags()
	_, cleanup := setupMockServer(t, newCmdsHandler)
	defer cleanup()

	restore, reader := captureStdout(t)
	err := runWorkspaceDocuments(nil, nil)
	restore()
	drainReader(reader)

	if err != nil {
		t.Fatalf("runWorkspaceDocuments failed: %v", err)
	}
}

func TestNewRunWorkspaceMembersAdd(t *testing.T) {
	resetFlags()
	_, cleanup := setupMockServer(t, newCmdsHandler)
	defer cleanup()

	flagMemberNick = "new_user"
	flagMemberRoleIDs = "role_1,role_2"

	restore, reader := captureStdout(t)
	err := runWorkspaceMembersAdd(nil, nil)
	restore()
	drainReader(reader)

	if err != nil {
		t.Fatalf("runWorkspaceMembersAdd failed: %v", err)
	}
}

func TestNewRunWorkspaceSettings(t *testing.T) {
	resetFlags()
	_, cleanup := setupMockServer(t, newCmdsHandler)
	defer cleanup()

	flagSettingType = "is_enabled_story_category"

	restore, reader := captureStdout(t)
	err := runWorkspaceSettings(nil, nil)
	restore()
	drainReader(reader)

	if err != nil {
		t.Fatalf("runWorkspaceSettings failed: %v", err)
	}
}

// ===================== Story Extras 命令测试 =====================

func TestNewRunStoryCopy(t *testing.T) {
	resetFlags()
	_, cleanup := setupMockServer(t, newCmdsHandler)
	defer cleanup()

	flagCurrentUser = "alice"

	restore, reader := captureStdout(t)
	err := runStoryCopy(nil, []string{"10001"})
	restore()
	drainReader(reader)

	if err != nil {
		t.Fatalf("runStoryCopy failed: %v", err)
	}
}

func TestNewRunStoryLinkList(t *testing.T) {
	resetFlags()
	_, cleanup := setupMockServer(t, newCmdsHandler)
	defer cleanup()

	flagChangeEntityID = "10001"

	restore, reader := captureStdout(t)
	err := runStoryLinkList(nil, nil)
	restore()
	drainReader(reader)

	if err != nil {
		t.Fatalf("runStoryLinkList failed: %v", err)
	}
}

func TestNewRunStoryLinkAdd(t *testing.T) {
	resetFlags()
	_, cleanup := setupMockServer(t, newCmdsHandler)
	defer cleanup()

	flagChangeEntityID = "10001"
	flagStoryLinkTargetID = "10002"

	restore, reader := captureStdout(t)
	err := runStoryLinkAdd(nil, nil)
	restore()
	drainReader(reader)

	if err != nil {
		t.Fatalf("runStoryLinkAdd failed: %v", err)
	}
}

func TestNewRunStoryLinkRemove(t *testing.T) {
	resetFlags()
	_, cleanup := setupMockServer(t, newCmdsHandler)
	defer cleanup()

	flagChangeEntityID = "10001"
	flagStoryLinkTargetID = "10002"

	restore, reader := captureStdout(t)
	err := runStoryLinkRemove(nil, nil)
	restore()
	drainReader(reader)

	if err != nil {
		t.Fatalf("runStoryLinkRemove failed: %v", err)
	}
}

// ===================== Bug Extras 命令测试 =====================

func TestNewRunBugCopy(t *testing.T) {
	resetFlags()
	_, cleanup := setupMockServer(t, newCmdsHandler)
	defer cleanup()

	flagCurrentUser = "bob"

	restore, reader := captureStdout(t)
	err := runBugCopy(nil, []string{"20001"})
	restore()
	drainReader(reader)

	if err != nil {
		t.Fatalf("runBugCopy failed: %v", err)
	}
}

func TestNewRunBugLinkList(t *testing.T) {
	resetFlags()
	_, cleanup := setupMockServer(t, newCmdsHandler)
	defer cleanup()

	flagChangeEntityID = "20001"

	restore, reader := captureStdout(t)
	err := runBugLinkList(nil, nil)
	restore()
	drainReader(reader)

	if err != nil {
		t.Fatalf("runBugLinkList failed: %v", err)
	}
}

func TestNewRunBugLinkAdd(t *testing.T) {
	resetFlags()
	_, cleanup := setupMockServer(t, newCmdsHandler)
	defer cleanup()

	flagChangeEntityID = "20001"
	flagBugLinkTargetIDs = "20002,20003"

	restore, reader := captureStdout(t)
	err := runBugLinkAdd(nil, nil)
	restore()
	drainReader(reader)

	if err != nil {
		t.Fatalf("runBugLinkAdd failed: %v", err)
	}
}

func TestNewRunBugLinkRemove(t *testing.T) {
	resetFlags()
	_, cleanup := setupMockServer(t, newCmdsHandler)
	defer cleanup()

	flagChangeEntityID = "20001"
	flagBugLinkIDs = "lk1,lk2"

	restore, reader := captureStdout(t)
	err := runBugLinkRemove(nil, nil)
	restore()
	drainReader(reader)

	if err != nil {
		t.Fatalf("runBugLinkRemove failed: %v", err)
	}
}

func TestNewRunBugRelatedStories(t *testing.T) {
	resetFlags()
	_, cleanup := setupMockServer(t, newCmdsHandler)
	defer cleanup()

	flagChangeEntityID = "20001"

	restore, reader := captureStdout(t)
	err := runBugRelatedStories(nil, nil)
	restore()
	drainReader(reader)

	if err != nil {
		t.Fatalf("runBugRelatedStories failed: %v", err)
	}
}

func TestNewRunBugTemplateList(t *testing.T) {
	resetFlags()
	_, cleanup := setupMockServer(t, newCmdsHandler)
	defer cleanup()

	restore, reader := captureStdout(t)
	err := runBugTemplateList(nil, nil)
	restore()
	drainReader(reader)

	if err != nil {
		t.Fatalf("runBugTemplateList failed: %v", err)
	}
}

// ===================== Test Plan 命令测试 =====================

func TestNewRunTestPlanList(t *testing.T) {
	resetFlags()
	_, cleanup := setupMockServer(t, newCmdsHandler)
	defer cleanup()

	restore, reader := captureStdout(t)
	err := runTestPlanList(nil, nil)
	restore()
	drainReader(reader)

	if err != nil {
		t.Fatalf("runTestPlanList failed: %v", err)
	}
}

func TestNewRunTestPlanCreate(t *testing.T) {
	resetFlags()
	_, cleanup := setupMockServer(t, newCmdsHandler)
	defer cleanup()

	flagName = "New Test Plan"
	flagOwner = "tester1"
	flagDescription = "Plan description"

	restore, reader := captureStdout(t)
	err := runTestPlanCreate(nil, nil)
	restore()
	drainReader(reader)

	if err != nil {
		t.Fatalf("runTestPlanCreate failed: %v", err)
	}
}

func TestNewRunTestPlanUpdate(t *testing.T) {
	resetFlags()
	_, cleanup := setupMockServer(t, newCmdsHandler)
	defer cleanup()

	flagName = "Updated Plan"
	flagStatus = "done"

	restore, reader := captureStdout(t)
	err := runTestPlanUpdate(nil, []string{"tp001"})
	restore()
	drainReader(reader)

	if err != nil {
		t.Fatalf("runTestPlanUpdate failed: %v", err)
	}
}

func TestNewRunTestPlanCount(t *testing.T) {
	resetFlags()
	_, cleanup := setupMockServer(t, newCmdsHandler)
	defer cleanup()

	restore, reader := captureStdout(t)
	err := runTestPlanCount(nil, nil)
	restore()
	drainReader(reader)

	if err != nil {
		t.Fatalf("runTestPlanCount failed: %v", err)
	}
}

func TestNewRunTestPlanProgress(t *testing.T) {
	resetFlags()
	_, cleanup := setupMockServer(t, newCmdsHandler)
	defer cleanup()

	flagTestPlanID = "tp001"

	restore, reader := captureStdout(t)
	err := runTestPlanProgress(nil, nil)
	restore()
	drainReader(reader)

	if err != nil {
		t.Fatalf("runTestPlanProgress failed: %v", err)
	}
}

// ===================== Module 命令测试 =====================

func TestNewRunModuleList(t *testing.T) {
	resetFlags()
	_, cleanup := setupMockServer(t, newCmdsHandler)
	defer cleanup()

	restore, reader := captureStdout(t)
	err := runModuleList(nil, nil)
	restore()
	drainReader(reader)

	if err != nil {
		t.Fatalf("runModuleList failed: %v", err)
	}
}

func TestNewRunModuleCreate(t *testing.T) {
	resetFlags()
	_, cleanup := setupMockServer(t, newCmdsHandler)
	defer cleanup()

	flagName = "NewModule"
	flagDescription = "Module desc"
	flagOwner = "alice"

	restore, reader := captureStdout(t)
	err := runModuleCreate(nil, nil)
	restore()
	drainReader(reader)

	if err != nil {
		t.Fatalf("runModuleCreate failed: %v", err)
	}
}

func TestNewRunModuleUpdate(t *testing.T) {
	resetFlags()
	_, cleanup := setupMockServer(t, newCmdsHandler)
	defer cleanup()

	flagName = "UpdatedModule"

	restore, reader := captureStdout(t)
	err := runModuleUpdate(nil, []string{"mod001"})
	restore()
	drainReader(reader)

	if err != nil {
		t.Fatalf("runModuleUpdate failed: %v", err)
	}
}

func TestNewRunModuleCount(t *testing.T) {
	resetFlags()
	_, cleanup := setupMockServer(t, newCmdsHandler)
	defer cleanup()

	restore, reader := captureStdout(t)
	err := runModuleCount(nil, nil)
	restore()
	drainReader(reader)

	if err != nil {
		t.Fatalf("runModuleCount failed: %v", err)
	}
}

// ===================== TCase Update 命令测试 =====================

func TestNewRunTCaseUpdate(t *testing.T) {
	resetFlags()
	_, cleanup := setupMockServer(t, newCmdsHandler)
	defer cleanup()

	flagName = "Updated Case"
	flagStatus = "normal"

	restore, reader := captureStdout(t)
	err := runTCaseUpdate(nil, []string{"tc001"})
	restore()
	drainReader(reader)

	if err != nil {
		t.Fatalf("runTCaseUpdate failed: %v", err)
	}
}

// ===================== Timesheet 命令测试 =====================

func TestNewRunTimesheetCount(t *testing.T) {
	resetFlags()
	_, cleanup := setupMockServer(t, newCmdsHandler)
	defer cleanup()

	flagTimesheetEntityType = "story"
	flagTimesheetEntityID = "10001"

	restore, reader := captureStdout(t)
	err := runTimesheetCount(nil, nil)
	restore()
	drainReader(reader)

	if err != nil {
		t.Fatalf("runTimesheetCount failed: %v", err)
	}
}

func TestNewRunTimesheetDelete(t *testing.T) {
	resetFlags()
	_, cleanup := setupMockServer(t, newCmdsHandler)
	defer cleanup()

	flagTimesheetEntityType = "story"
	flagTimesheetEntityID = "10001"
	flagTimesheetCostIDs = "cost1,cost2"

	restore, reader := captureStdout(t)
	err := runTimesheetDelete(nil, nil)
	restore()
	drainReader(reader)

	if err != nil {
		t.Fatalf("runTimesheetDelete failed: %v", err)
	}
}

func TestNewRunTimesheetList(t *testing.T) {
	resetFlags()
	_, cleanup := setupMockServer(t, newCmdsHandler)
	defer cleanup()

	flagTimesheetEntityType = "task"
	flagTimesheetEntityID = "30001"

	restore, reader := captureStdout(t)
	err := runTimesheetList(nil, nil)
	restore()
	drainReader(reader)

	if err != nil {
		t.Fatalf("runTimesheetList failed: %v", err)
	}
}

// ===================== Attachment 命令测试 =====================

func TestNewRunAttachmentDownload(t *testing.T) {
	resetFlags()
	_, cleanup := setupMockServer(t, newCmdsHandler)
	defer cleanup()

	flagAttachmentID = "att001"

	restore, reader := captureStdout(t)
	err := runAttachmentDownload(nil, nil)
	restore()
	drainReader(reader)

	if err != nil {
		t.Fatalf("runAttachmentDownload failed: %v", err)
	}
}

func TestNewRunAttachmentList(t *testing.T) {
	resetFlags()
	_, cleanup := setupMockServer(t, newCmdsHandler)
	defer cleanup()

	flagAttachmentEntryID = "10001"
	flagAttachmentType = "story"

	restore, reader := captureStdout(t)
	err := runAttachmentList(nil, nil)
	restore()
	drainReader(reader)

	if err != nil {
		t.Fatalf("runAttachmentList failed: %v", err)
	}
}
