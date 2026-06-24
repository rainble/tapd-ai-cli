package cmd

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/studyzy/tapd-ai-cli/internal/config"
	tapd "github.com/studyzy/tapd-sdk-go"
	"github.com/studyzy/tapd-sdk-go/model"
)

func TestGitLabIssueCreate_PostsIssueAndPrintsSuccess(t *testing.T) {
	resetGitLabFlagsForTest(t)

	var capturedForm url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.EscapedPath() != "/api/v4/projects/go-vas%2Fvas/issues" {
			t.Fatalf("path = %q", r.URL.EscapedPath())
		}
		if r.Header.Get("PRIVATE-TOKEN") != "secret" {
			t.Fatalf("PRIVATE-TOKEN = %q", r.Header.Get("PRIVATE-TOKEN"))
		}
		if err := r.ParseForm(); err != nil {
			t.Fatalf("ParseForm failed: %v", err)
		}
		capturedForm = r.PostForm
		_, _ = w.Write([]byte(`{"id":123,"iid":45,"web_url":"https://git.bilibili.co/go-vas/vas/-/issues/45","project_id":7}`))
	}))
	defer srv.Close()

	flagGitLabBaseURL = srv.URL
	flagGitLabToken = "secret"
	flagGitLabProject = "go-vas/vas"
	flagTitle = "保存失败"
	flagDescription = "复现步骤"
	flagGitLabLabels = "bug,backend"
	flagGitLabAssigneeIDs = "1001,1002"
	flagGitLabDueDate = "2026-06-30"
	flagGitLabConfidential = true
	flagGitLabIssueType = "issue"

	restore, reader := captureStdout(t)
	err := runGitLabIssueCreate(nil, nil)
	restore()
	data, readErr := io.ReadAll(reader)
	_ = reader.Close()
	if readErr != nil {
		t.Fatalf("read stdout failed: %v", readErr)
	}
	out := string(data)
	if err != nil {
		t.Fatalf("runGitLabIssueCreate returned error: %v", err)
	}

	assertFormValue(t, capturedForm, "title", "保存失败")
	assertFormValue(t, capturedForm, "description", "复现步骤")
	assertFormValue(t, capturedForm, "labels", "bug,backend")
	assertFormValue(t, capturedForm, "assignee_ids", "1001,1002")
	assertFormValue(t, capturedForm, "due_date", "2026-06-30")
	assertFormValue(t, capturedForm, "confidential", "true")
	assertFormValue(t, capturedForm, "issue_type", "issue")

	var resp gitLabIssueSuccess
	if err := json.Unmarshal([]byte(out), &resp); err != nil {
		t.Fatalf("stdout is not JSON: %s", out)
	}
	if !resp.Success || resp.IID != 45 || resp.URL == "" || resp.Project != "go-vas/vas" {
		t.Fatalf("unexpected success response: %+v", resp)
	}
}

func TestGitLabIssueCreate_ReadsProjectAndTokenFromConfig(t *testing.T) {
	resetGitLabFlagsForTest(t)
	appConfig = &config.Config{
		GitLabBaseURL: "https://git.example.com",
		GitLabToken:   "cfg_token",
		GitLabProject: "cfg/project",
	}

	opts, err := resolveGitLabOptions()
	if err != nil {
		t.Fatalf("resolveGitLabOptions returned error: %v", err)
	}
	if opts.baseURL != "https://git.example.com" || opts.token != "cfg_token" || opts.project != "cfg/project" {
		t.Fatalf("unexpected options: %+v", opts)
	}
}

func TestGitLabIssueCreate_DescriptionFromFile(t *testing.T) {
	resetGitLabFlagsForTest(t)
	dir := t.TempDir()
	path := filepath.Join(dir, "issue.md")
	if err := os.WriteFile(path, []byte("file description"), 0644); err != nil {
		t.Fatal(err)
	}
	flagDescription = ""
	flagDescFile = path

	desc, err := readGitLabDescription()
	if err != nil {
		t.Fatalf("readGitLabDescription returned error: %v", err)
	}
	if desc != "file description" {
		t.Fatalf("description = %q, want file description", desc)
	}
}

func TestGitLabIssueCreate_MissingToken(t *testing.T) {
	resetGitLabFlagsForTest(t)
	flagGitLabProject = "go-vas/vas"

	_, err := resolveGitLabOptions()
	if err == nil || !strings.Contains(err.Error(), "GitLab token") {
		t.Fatalf("resolveGitLabOptions error = %v, want missing token", err)
	}
}

func TestIsGitLabStandaloneCreate(t *testing.T) {
	if !isGitLabStandaloneCreate(gitlabIssueCreateCmd) {
		t.Fatal("gitlab issue create should skip TAPD init")
	}
	if isGitLabStandaloneCreate(gitlabIssueCreateFromStoryCmd) {
		t.Fatal("create-from-story should require TAPD init")
	}
}

func TestBuildGitLabIssueFromStory_RendersStructuredDescription(t *testing.T) {
	story := &model.Story{
		ID:            "1151081496001028684",
		Name:          "支持自动续费看板",
		Description:   "<h2>背景</h2><p>运营需要展示签约趋势。</p><h2>目标</h2><p>让 PM 快速判断续费表现。</p><h2>方案</h2><p>新增趋势图和明细表。</p><h2>验收标准</h2><p>能按日期筛选并导出。</p><h2>风险</h2><p>依赖数据仓库产出。</p><p>补充说明：历史数据只保留 180 天。</p>",
		URL:           "https://tapd.example.com/story",
		Status:        "planning",
		PriorityLabel: "High",
		Owner:         "alice",
		Developer:     "bob",
		IterationID:   "it1",
	}

	snapshot := buildGitLabIssueFromStory(story)

	if snapshot.EntityType != "story" || snapshot.EntityID != story.ID {
		t.Fatalf("unexpected identity: %+v", snapshot)
	}
	if snapshot.Title != "[TAPD Story] 支持自动续费看板" {
		t.Fatalf("title = %q", snapshot.Title)
	}
	for _, want := range []string{
		"## TAPD 需求",
		"- TAPD: https://tapd.example.com/story",
		"- ID: 1151081496001028684",
		"- Priority: High",
		"- Owner: alice",
		"- Developer: bob",
		"## 背景 / 现状",
		"运营需要展示签约趋势。",
		"## 目标 / 预期",
		"让 PM 快速判断续费表现。",
		"## 需求范围 / 方案",
		"新增趋势图和明细表。",
		"## 验收标准 / 测试要点",
		"能按日期筛选并导出。",
		"## 风险 / 依赖 / 待确认",
		"依赖数据仓库产出。",
		"## 原始补充",
		"补充说明：历史数据只保留 180 天。",
	} {
		if !strings.Contains(snapshot.Description, want) {
			t.Fatalf("description should contain %q, got:\n%s", want, snapshot.Description)
		}
	}
	if strings.Contains(snapshot.Description, "## 描述") {
		t.Fatalf("description should not use raw description section, got:\n%s", snapshot.Description)
	}
	if !snapshot.Ready {
		t.Fatal("story with title and description should be ready")
	}
	if snapshot.Fingerprint == "" {
		t.Fatal("fingerprint should not be empty")
	}
}

func TestBuildGitLabIssueFromStory_PutsUnclassifiedContentInOriginalSupplement(t *testing.T) {
	story := &model.Story{
		ID:          "1151081496001028684",
		Name:        "无模板需求",
		Description: "<p>这是一段没有模板标题的自由描述。</p>",
	}

	snapshot := buildGitLabIssueFromStory(story)

	for _, want := range []string{
		"## TAPD 需求",
		"## 原始补充",
		"这是一段没有模板标题的自由描述。",
	} {
		if !strings.Contains(snapshot.Description, want) {
			t.Fatalf("description should contain %q, got:\n%s", want, snapshot.Description)
		}
	}
	for _, notWant := range []string{"## 背景 / 现状", "## 目标 / 预期", "## 需求范围 / 方案"} {
		if strings.Contains(snapshot.Description, notWant) {
			t.Fatalf("description should not contain empty section %q, got:\n%s", notWant, snapshot.Description)
		}
	}
}

func TestBuildGitLabIssueFromBug_RendersStructuredDescription(t *testing.T) {
	bug := &model.Bug{
		ID:            "1151081496002000001",
		Title:         "保存时报错",
		Description:   "<div>前置条件：测试账号 mid=123，iOS 8.0.0</div><div>复现流程：进入编辑页后点击保存</div><div>实际情况：接口返回 500</div><div>预期情况：保存成功并返回详情页</div><div>影响范围：所有开通充电的 UP 主</div><div>日志 trace_id=abc</div>",
		URL:           "https://tapd.example.com/bug",
		Status:        "new",
		PriorityLabel: "urgent",
		Severity:      "serious",
		CurrentOwner:  "charlie",
		Module:        "charge",
		IterationID:   "it2",
	}

	snapshot := buildGitLabIssueFromBug(bug)

	if snapshot.EntityType != "bug" || snapshot.EntityID != bug.ID {
		t.Fatalf("unexpected identity: %+v", snapshot)
	}
	if snapshot.Title != "[TAPD Bug] 保存时报错" {
		t.Fatalf("title = %q", snapshot.Title)
	}
	for _, want := range []string{
		"## TAPD 缺陷",
		"- TAPD: https://tapd.example.com/bug",
		"- Severity: serious",
		"- Current owner: charlie",
		"- Module: charge",
		"## 复现条件",
		"测试账号 mid=123",
		"## 复现步骤",
		"进入编辑页后点击保存",
		"## 实际结果",
		"接口返回 500",
		"## 预期结果",
		"保存成功并返回详情页",
		"## 影响范围",
		"所有开通充电的 UP 主",
		"## 排查线索",
		"trace\\_id=abc",
	} {
		if !strings.Contains(snapshot.Description, want) {
			t.Fatalf("description should contain %q, got:\n%s", want, snapshot.Description)
		}
	}
	if strings.Contains(snapshot.Description, "## 描述") {
		t.Fatalf("description should not use raw description section, got:\n%s", snapshot.Description)
	}
	if !snapshot.Ready || snapshot.Fingerprint == "" {
		t.Fatalf("bug should be ready with fingerprint: %+v", snapshot)
	}
}

func TestBuildGitLabIssueFromBug_PutsUnclassifiedContentInOriginalSupplement(t *testing.T) {
	bug := &model.Bug{
		ID:          "1151081496002000001",
		Title:       "无模板缺陷",
		Description: "<p>用户反馈页面偶现空白，暂无更多信息。</p>",
	}

	snapshot := buildGitLabIssueFromBug(bug)

	for _, want := range []string{
		"## TAPD 缺陷",
		"## 原始补充",
		"用户反馈页面偶现空白，暂无更多信息。",
	} {
		if !strings.Contains(snapshot.Description, want) {
			t.Fatalf("description should contain %q, got:\n%s", want, snapshot.Description)
		}
	}
	for _, notWant := range []string{"## 复现条件", "## 复现步骤", "## 实际结果"} {
		if strings.Contains(snapshot.Description, notWant) {
			t.Fatalf("description should not contain empty section %q, got:\n%s", notWant, snapshot.Description)
		}
	}
}

func TestBuildGitLabIssueFromStory_NotReadyWithEmptyDescription(t *testing.T) {
	story := &model.Story{Name: "只有标题", Description: "<p><br></p>"}

	snapshot := buildGitLabIssueFromStory(story)

	if snapshot.Ready {
		t.Fatalf("empty TAPD description should not be ready: %+v", snapshot)
	}
}

func TestGitLabSyncMarkerRoundTrip(t *testing.T) {
	marker := gitLabSyncMarker{
		Type:        "story",
		ID:          "1151081496001028684",
		Project:     "go-vas/vas",
		IssueIID:    45,
		Fingerprint: "abc123",
	}

	rendered := renderGitLabSyncComment("https://git.bilibili.co/go-vas/vas/-/issues/45", marker)
	parsed, ok := parseGitLabSyncMarker(rendered, "story", marker.ID, marker.Project)

	if !ok {
		t.Fatalf("marker should parse from comment:\n%s", rendered)
	}
	if parsed.IssueIID != 45 || parsed.Fingerprint != "abc123" {
		t.Fatalf("unexpected parsed marker: %+v", parsed)
	}
}

func TestGitLabSnapshotFingerprintChangesWithStatus(t *testing.T) {
	story := &model.Story{
		ID:          "1",
		Name:        "需求",
		Description: "<p>描述</p>",
		Status:      "open",
	}
	first := buildGitLabIssueFromStory(story)
	story.Status = "done"
	second := buildGitLabIssueFromStory(story)

	if first.Fingerprint == second.Fingerprint {
		t.Fatal("fingerprint should change when status changes")
	}
}

func TestGitLabIssueCreateFromStory_CreatesIssueAndCommentsBack(t *testing.T) {
	resetGitLabFlagsForTest(t)
	oldClient := apiClient
	oldWorkspace := flagWorkspaceID
	t.Cleanup(func() {
		apiClient = oldClient
		flagWorkspaceID = oldWorkspace
	})

	var commentBody string
	tapdSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/stories":
			_, _ = w.Write([]byte(`{"status":1,"data":[{"Story":{"id":"1151081496001028684","name":"需求标题","description":"<p>需求描述</p>","status":"open","priority_label":"High","owner":"alice","developer":"bob"}}]}`))
		case r.Method == http.MethodGet && r.URL.Path == "/users/info":
			_, _ = w.Write([]byte(`{"status":1,"data":{"nick":"tester"}}`))
		case r.Method == http.MethodPost && r.URL.Path == "/comments":
			if err := r.ParseForm(); err != nil {
				t.Fatalf("ParseForm failed: %v", err)
			}
			commentBody = r.PostForm.Get("description")
			_, _ = w.Write([]byte(`{"status":1,"data":{"Comment":{"id":"c1"}}}`))
		default:
			t.Fatalf("unexpected TAPD request %s %s", r.Method, r.URL.Path)
		}
	}))
	defer tapdSrv.Close()

	var gitlabDescription string
	gitlabSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Fatalf("ParseForm failed: %v", err)
		}
		gitlabDescription = r.PostForm.Get("description")
		_, _ = w.Write([]byte(`{"id":123,"iid":45,"web_url":"https://git.bilibili.co/go-vas/vas/-/issues/45","project_id":7}`))
	}))
	defer gitlabSrv.Close()

	apiClient = tapd.NewClientWithBaseURL(tapdSrv.URL, tapdSrv.URL, "tapd-token", "", "")
	flagWorkspaceID = "51081496"
	flagGitLabBaseURL = gitlabSrv.URL
	flagGitLabToken = "gitlab-token"
	flagGitLabProject = "go-vas/vas"
	flagGitLabCommentBack = true

	restore, reader := captureStdout(t)
	err := runGitLabIssueCreateFromStory(nil, []string{"1151081496001028684"})
	restore()
	drainReader(reader)
	if err != nil {
		t.Fatalf("runGitLabIssueCreateFromStory returned error: %v", err)
	}
	if !strings.Contains(gitlabDescription, "需求描述") {
		t.Fatalf("GitLab description should include TAPD story description, got:\n%s", gitlabDescription)
	}
	if !strings.Contains(commentBody, "tapd-gitlab-sync") || !strings.Contains(commentBody, "issues/45") {
		t.Fatalf("comment-back should include sync marker and issue URL, got:\n%s", commentBody)
	}
}

func TestGitLabIssueCreateFromBug_CreatesIssue(t *testing.T) {
	resetGitLabFlagsForTest(t)
	oldClient := apiClient
	oldWorkspace := flagWorkspaceID
	t.Cleanup(func() {
		apiClient = oldClient
		flagWorkspaceID = oldWorkspace
	})

	tapdSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/bugs" {
			t.Fatalf("unexpected TAPD request %s %s", r.Method, r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"status":1,"data":[{"Bug":{"id":"1151081496002000001","title":"缺陷标题","description":"<p>缺陷描述</p>","status":"new","priority_label":"urgent","severity":"serious","current_owner":"alice"}}]}`))
	}))
	defer tapdSrv.Close()

	var gitlabTitle string
	gitlabSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Fatalf("ParseForm failed: %v", err)
		}
		gitlabTitle = r.PostForm.Get("title")
		_, _ = w.Write([]byte(`{"id":124,"iid":46,"web_url":"https://git.bilibili.co/go-vas/vas/-/issues/46","project_id":7}`))
	}))
	defer gitlabSrv.Close()

	apiClient = tapd.NewClientWithBaseURL(tapdSrv.URL, tapdSrv.URL, "tapd-token", "", "")
	flagWorkspaceID = "51081496"
	flagGitLabBaseURL = gitlabSrv.URL
	flagGitLabToken = "gitlab-token"
	flagGitLabProject = "go-vas/vas"

	restore, reader := captureStdout(t)
	err := runGitLabIssueCreateFromBug(nil, []string{"1151081496002000001"})
	restore()
	drainReader(reader)
	if err != nil {
		t.Fatalf("runGitLabIssueCreateFromBug returned error: %v", err)
	}
	if gitlabTitle != "[TAPD Bug] 缺陷标题" {
		t.Fatalf("GitLab title = %q", gitlabTitle)
	}
}

func TestHandleGitLabIssueSyncEvent_CreatesIssueWhenNoMarker(t *testing.T) {
	resetGitLabFlagsForTest(t)
	oldClient := apiClient
	oldWorkspace := flagWorkspaceID
	t.Cleanup(func() {
		apiClient = oldClient
		flagWorkspaceID = oldWorkspace
	})

	var commentBody string
	tapdSrv := newGitLabSyncTAPDServer(t, gitLabSyncTAPDOptions{
		storyDescription: "<p>需求描述</p>",
		commentsJSON:     `[]`,
		onComment: func(desc string) {
			commentBody = desc
		},
	})
	defer tapdSrv.Close()

	var createCalled bool
	gitlabSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		createCalled = true
		if r.URL.EscapedPath() != "/api/v4/projects/go-vas%2Fvas/issues" {
			t.Fatalf("unexpected GitLab path: %s", r.URL.EscapedPath())
		}
		_, _ = w.Write([]byte(`{"id":123,"iid":45,"web_url":"https://git.bilibili.co/go-vas/vas/-/issues/45","project_id":7}`))
	}))
	defer gitlabSrv.Close()

	apiClient = tapd.NewClientWithBaseURL(tapdSrv.URL, tapdSrv.URL, "tapd-token", "", "")
	flagWorkspaceID = "51081496"
	cfg := gitLabSyncConfig{
		options: gitLabOptions{baseURL: gitlabSrv.URL, token: "gitlab-token", project: "go-vas/vas"},
		types:   map[string]bool{"story": true, "bug": true},
	}
	event := `{"id":1,"event":{"event":"story::update","workspace_id":"51081496","story":{"id":"1151081496001028684"}}}`

	handled, err := handleGitLabIssueSyncEvent(context.Background(), event, cfg)
	if err != nil {
		t.Fatalf("handleGitLabIssueSyncEvent returned error: %v", err)
	}
	if !handled || !createCalled {
		t.Fatalf("event should create issue, handled=%v createCalled=%v", handled, createCalled)
	}
	if !strings.Contains(commentBody, "tapd-gitlab-sync") {
		t.Fatalf("sync should write marker comment, got:\n%s", commentBody)
	}
}

func TestHandleGitLabIssueSyncEvent_AppendsNoteWhenFingerprintChanged(t *testing.T) {
	resetGitLabFlagsForTest(t)
	oldClient := apiClient
	oldWorkspace := flagWorkspaceID
	t.Cleanup(func() {
		apiClient = oldClient
		flagWorkspaceID = oldWorkspace
	})

	marker := renderGitLabSyncComment("https://git.bilibili.co/go-vas/vas/-/issues/45", gitLabSyncMarker{
		Type:        "story",
		ID:          "1151081496001028684",
		Project:     "go-vas/vas",
		IssueIID:    45,
		Fingerprint: "old",
	})
	var commentWrites int
	tapdSrv := newGitLabSyncTAPDServer(t, gitLabSyncTAPDOptions{
		storyDescription: "<p>新的需求描述</p>",
		commentsJSON:     `[{"Comment":{"id":"c1","description":` + strconv.Quote(marker) + `}}]`,
		onComment: func(desc string) {
			commentWrites++
		},
	})
	defer tapdSrv.Close()

	var noteCalled bool
	gitlabSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		noteCalled = true
		if r.URL.EscapedPath() != "/api/v4/projects/go-vas%2Fvas/issues/45/notes" {
			t.Fatalf("unexpected GitLab path: %s", r.URL.EscapedPath())
		}
		_, _ = w.Write([]byte(`{"id":900,"body":"ok"}`))
	}))
	defer gitlabSrv.Close()

	apiClient = tapd.NewClientWithBaseURL(tapdSrv.URL, tapdSrv.URL, "tapd-token", "", "")
	flagWorkspaceID = "51081496"
	cfg := gitLabSyncConfig{
		options: gitLabOptions{baseURL: gitlabSrv.URL, token: "gitlab-token", project: "go-vas/vas"},
		types:   map[string]bool{"story": true},
	}
	event := `{"id":2,"event":{"event":"story_update","workspace_id":"51081496","story":{"id":"1151081496001028684"}}}`

	handled, err := handleGitLabIssueSyncEvent(context.Background(), event, cfg)
	if err != nil {
		t.Fatalf("handleGitLabIssueSyncEvent returned error: %v", err)
	}
	if !handled || !noteCalled || commentWrites == 0 {
		t.Fatalf("event should append note and marker, handled=%v noteCalled=%v commentWrites=%d", handled, noteCalled, commentWrites)
	}
}

func TestHandleGitLabIssueSyncEvent_SkipsWhenFingerprintUnchanged(t *testing.T) {
	resetGitLabFlagsForTest(t)
	oldClient := apiClient
	oldWorkspace := flagWorkspaceID
	t.Cleanup(func() {
		apiClient = oldClient
		flagWorkspaceID = oldWorkspace
	})

	story := &model.Story{
		ID:          "1151081496001028684",
		Name:        "需求标题",
		Description: "<p>需求描述</p>",
		Status:      "open",
	}
	snapshot := buildGitLabIssueFromStory(story)
	marker := renderGitLabSyncComment("https://git.bilibili.co/go-vas/vas/-/issues/45", gitLabSyncMarker{
		Type:        "story",
		ID:          story.ID,
		Project:     "go-vas/vas",
		IssueIID:    45,
		Fingerprint: snapshot.Fingerprint,
	})
	tapdSrv := newGitLabSyncTAPDServer(t, gitLabSyncTAPDOptions{
		storyDescription: story.Description,
		commentsJSON:     `[{"Comment":{"id":"c1","description":` + strconv.Quote(marker) + `}}]`,
	})
	defer tapdSrv.Close()

	gitlabSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("GitLab should not be called when fingerprint is unchanged")
	}))
	defer gitlabSrv.Close()

	apiClient = tapd.NewClientWithBaseURL(tapdSrv.URL, tapdSrv.URL, "tapd-token", "", "")
	flagWorkspaceID = "51081496"
	cfg := gitLabSyncConfig{
		options: gitLabOptions{baseURL: gitlabSrv.URL, token: "gitlab-token", project: "go-vas/vas"},
		types:   map[string]bool{"story": true},
	}
	event := `{"id":3,"event":{"event":"story_update","workspace_id":"51081496","story":{"id":"1151081496001028684"}}}`

	handled, err := handleGitLabIssueSyncEvent(context.Background(), event, cfg)
	if err != nil {
		t.Fatalf("handleGitLabIssueSyncEvent returned error: %v", err)
	}
	if handled {
		t.Fatal("unchanged event should be skipped")
	}
}

type gitLabSyncTAPDOptions struct {
	storyDescription string
	commentsJSON     string
	onComment        func(description string)
}

func newGitLabSyncTAPDServer(t *testing.T, opts gitLabSyncTAPDOptions) *httptest.Server {
	t.Helper()
	if opts.commentsJSON == "" {
		opts.commentsJSON = `[]`
	}
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/stories":
			_, _ = w.Write([]byte(`{"status":1,"data":[{"Story":{"id":"1151081496001028684","name":"需求标题","description":` + strconv.Quote(opts.storyDescription) + `,"status":"open"}}]}`))
		case r.Method == http.MethodGet && r.URL.Path == "/comments":
			_, _ = w.Write([]byte(`{"status":1,"data":` + opts.commentsJSON + `}`))
		case r.Method == http.MethodGet && r.URL.Path == "/users/info":
			_, _ = w.Write([]byte(`{"status":1,"data":{"nick":"tester"}}`))
		case r.Method == http.MethodPost && r.URL.Path == "/comments":
			if err := r.ParseForm(); err != nil {
				t.Fatalf("ParseForm failed: %v", err)
			}
			if opts.onComment != nil {
				opts.onComment(r.PostForm.Get("description"))
			}
			_, _ = w.Write([]byte(`{"status":1,"data":{"Comment":{"id":"new-comment"}}}`))
		default:
			t.Fatalf("unexpected TAPD request %s %s", r.Method, r.URL.Path)
		}
	}))
}

func resetGitLabFlagsForTest(t *testing.T) {
	t.Helper()
	resetFlags()
	oldConfig := appConfig
	t.Cleanup(func() {
		appConfig = oldConfig
	})
	appConfig = nil
	flagGitLabBaseURL = ""
	flagGitLabToken = ""
	flagGitLabProject = ""
	flagGitLabLabels = ""
	flagGitLabAssigneeIDs = ""
	flagGitLabDueDate = ""
	flagGitLabConfidential = false
	flagGitLabIssueType = ""
	flagGitLabCommentBack = false
	flagGitLabTypes = ""
}

func assertFormValue(t *testing.T, values url.Values, key, want string) {
	t.Helper()
	if got := values.Get(key); got != want {
		t.Fatalf("%s = %q, want %q", key, got, want)
	}
}
