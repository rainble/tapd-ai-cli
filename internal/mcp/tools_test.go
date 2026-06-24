package mcp

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	tapd "github.com/studyzy/tapd-sdk-go"
)

func TestToolStoryCreateConvertsMarkdownDescription(t *testing.T) {
	var captured url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		captured = r.PostForm
		w.Write([]byte(`{"status":1,"data":{"Story":{"id":"10001","name":"Test","url":"http://test/story/10001"}}}`))
	}))
	defer srv.Close()

	client := tapd.NewClientWithBaseURL(srv.URL, srv.URL, "test-token", "", "")
	client.SetNick("agent")
	server := NewServer(nil, nil, nil, client)
	tool := toolStoryCreate(server, func(string) string { return "12345" })

	_, err := tool.Handler(context.Background(), json.RawMessage(`{"name":"Test","description":"## 需求背景"}`))
	if err != nil {
		t.Fatal(err)
	}
	description := captured.Get("description")
	if !strings.Contains(description, "<h2") || !strings.Contains(description, "需求背景") {
		t.Fatalf("description was not converted to HTML: %q", description)
	}
}

func TestGitLabIssueCreateToolCreatesIssueWithoutTAPDClient(t *testing.T) {
	var captured url.Values
	gitlabSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.EscapedPath() != "/api/v4/projects/go-vas%2Fvas/issues" {
			t.Fatalf("path = %q", r.URL.EscapedPath())
		}
		if r.Header.Get("PRIVATE-TOKEN") != "secret" {
			t.Fatalf("PRIVATE-TOKEN = %q", r.Header.Get("PRIVATE-TOKEN"))
		}
		if err := r.ParseForm(); err != nil {
			t.Fatalf("ParseForm failed: %v", err)
		}
		captured = r.PostForm
		_, _ = w.Write([]byte(`{"id":123,"iid":45,"web_url":"https://git.bilibili.co/go-vas/vas/-/issues/45","project_id":7}`))
	}))
	defer gitlabSrv.Close()
	t.Setenv("GITLAB_BASE_URL", gitlabSrv.URL)
	t.Setenv("GITLAB_TOKEN", "secret")
	t.Setenv("GITLAB_PROJECT", "go-vas/vas")

	server := NewServer(nil, nil, nil, nil)
	tool := toolGitLabIssueCreate(server)
	result, err := tool.Handler(context.Background(), json.RawMessage(`{
		"title":"Issue 标题",
		"description":"Issue 描述",
		"labels":"bug,backend",
		"assignee_ids":"1001,1002"
	}`))
	if err != nil {
		t.Fatalf("toolGitLabIssueCreate returned error: %v", err)
	}
	if !tool.AllowNoTAPD {
		t.Fatal("pure GitLab create tool should be callable without TAPD client")
	}
	if captured.Get("title") != "Issue 标题" || captured.Get("description") != "Issue 描述" {
		t.Fatalf("unexpected form: %v", captured)
	}
	resp := result.(gitLabIssueToolResponse)
	if !resp.Success || resp.IID != 45 || resp.Project != "go-vas/vas" {
		t.Fatalf("unexpected result: %+v", resp)
	}
}

func TestGitLabIssueCreateFromStoryToolUsesTAPDAndCommentsBack(t *testing.T) {
	var commentBody string
	tapdSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/stories":
			_, _ = w.Write([]byte(`{"status":1,"data":[{"Story":{"id":"1151081496001028684","workspace_id":"51081496","name":"需求标题","description":"<p>需求描述</p>","status":"open","priority_label":"High","owner":"alice","developer":"bob"}}]}`))
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
	t.Setenv("GITLAB_BASE_URL", gitlabSrv.URL)
	t.Setenv("GITLAB_TOKEN", "secret")
	t.Setenv("GITLAB_PROJECT", "go-vas/vas")

	client := tapd.NewClientWithBaseURL(tapdSrv.URL, tapdSrv.URL, "tapd-token", "", "")
	server := NewServer(nil, nil, nil, client)
	tool := toolGitLabIssueCreateFromStory(server, func(string) string { return "51081496" })
	result, err := tool.Handler(context.Background(), json.RawMessage(`{
		"id":"1151081496001028684",
		"comment_back":true
	}`))
	if err != nil {
		t.Fatalf("toolGitLabIssueCreateFromStory returned error: %v", err)
	}
	if !strings.Contains(gitlabDescription, "需求描述") {
		t.Fatalf("GitLab description missing TAPD content: %s", gitlabDescription)
	}
	if !strings.Contains(commentBody, "tapd-gitlab-sync") {
		t.Fatalf("comment-back missing sync marker: %s", commentBody)
	}
	resp := result.(gitLabIssueToolResponse)
	if resp.IID != 45 {
		t.Fatalf("unexpected result: %+v", resp)
	}
}

func TestGitLabIssueCreateToolThroughServerWithoutTAPDCredentials(t *testing.T) {
	gitlabSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"id":123,"iid":45,"web_url":"https://git.example.com/go-vas/vas/-/issues/45","project_id":7}`))
	}))
	defer gitlabSrv.Close()
	t.Setenv("GITLAB_BASE_URL", gitlabSrv.URL)
	t.Setenv("GITLAB_TOKEN", "secret")
	t.Setenv("GITLAB_PROJECT", "go-vas/vas")

	pr, pw := io.Pipe()
	out := &strings.Builder{}
	server := NewServer(pr, out, io.Discard, nil)
	server.Register(toolGitLabIssueCreate(server))
	done := make(chan error, 1)
	go func() { done <- server.Run(context.Background()) }()
	io.WriteString(pw, `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"tapd_gitlab_issue_create","arguments":{"title":"x"}}}`+"\n")
	pw.Close()
	<-done

	var resp map[string]interface{}
	if err := json.Unmarshal([]byte(strings.TrimSpace(out.String())), &resp); err != nil {
		t.Fatalf("invalid response: %v\n%s", err, out.String())
	}
	if resp["error"] != nil {
		t.Fatalf("should not be rpc error: %v", resp["error"])
	}
	if result := resp["result"].(map[string]interface{}); result["isError"] == true {
		t.Fatalf("GitLab tool should run without TAPD credentials: %v", result)
	}
}

func TestGitLabToolsRegisteredByDefault(t *testing.T) {
	server := NewServer(nil, nil, nil, nil)
	RegisterDefaultTools(server, "")
	for _, name := range []string{
		"tapd_gitlab_issue_create",
		"tapd_gitlab_issue_create_from_story",
		"tapd_gitlab_issue_create_from_bug",
	} {
		if server.tools[name] == nil {
			t.Fatalf("%s should be registered", name)
		}
	}
}
