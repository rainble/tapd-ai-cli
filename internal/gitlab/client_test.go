// Package gitlab 测试 GitLab REST API 客户端行为。
package gitlab

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestClientCreateIssue_EncodesProjectPathAndSendsFields(t *testing.T) {
	var capturedPath string
	var capturedToken string
	var capturedForm url.Values

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedPath = r.URL.EscapedPath()
		capturedToken = r.Header.Get("PRIVATE-TOKEN")
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s, want POST", r.Method)
		}
		if err := r.ParseForm(); err != nil {
			t.Fatalf("ParseForm failed: %v", err)
		}
		capturedForm = r.PostForm
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":123,"iid":45,"web_url":"https://git.example.com/go-vas/vas/-/issues/45","project_id":7}`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "secret")
	issue, err := c.CreateIssue(context.Background(), "go-vas/vas", CreateIssueRequest{
		Title:        "Crash on save",
		Description:  "Steps here",
		Labels:       []string{"bug", "backend"},
		AssigneeIDs:  []int{1001, 1002},
		DueDate:      "2026-06-30",
		Confidential: true,
		IssueType:    "issue",
	})
	if err != nil {
		t.Fatalf("CreateIssue returned error: %v", err)
	}

	if capturedPath != "/api/v4/projects/go-vas%2Fvas/issues" {
		t.Fatalf("path = %q, want encoded project path", capturedPath)
	}
	if capturedToken != "secret" {
		t.Fatalf("PRIVATE-TOKEN = %q, want secret", capturedToken)
	}
	assertFormValue(t, capturedForm, "title", "Crash on save")
	assertFormValue(t, capturedForm, "description", "Steps here")
	assertFormValue(t, capturedForm, "labels", "bug,backend")
	assertFormValue(t, capturedForm, "assignee_ids", "1001,1002")
	assertFormValue(t, capturedForm, "due_date", "2026-06-30")
	assertFormValue(t, capturedForm, "confidential", "true")
	assertFormValue(t, capturedForm, "issue_type", "issue")
	if issue.ID != 123 || issue.IID != 45 || issue.WebURL == "" || issue.ProjectID != 7 {
		t.Fatalf("unexpected issue response: %+v", issue)
	}
}

func TestClientCreateIssueNote_PostsBodyToIssueIID(t *testing.T) {
	var capturedPath string
	var capturedBody string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedPath = r.URL.EscapedPath()
		if err := r.ParseForm(); err != nil {
			t.Fatalf("ParseForm failed: %v", err)
		}
		capturedBody = r.PostForm.Get("body")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":900,"body":"updated snapshot"}`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "secret")
	note, err := c.CreateIssueNote(context.Background(), "go-vas/vas", 45, "updated snapshot")
	if err != nil {
		t.Fatalf("CreateIssueNote returned error: %v", err)
	}

	if capturedPath != "/api/v4/projects/go-vas%2Fvas/issues/45/notes" {
		t.Fatalf("path = %q, want encoded notes path", capturedPath)
	}
	if capturedBody != "updated snapshot" {
		t.Fatalf("body = %q, want updated snapshot", capturedBody)
	}
	if note.ID != 900 || note.Body != "updated snapshot" {
		t.Fatalf("unexpected note response: %+v", note)
	}
}

func TestClientCreateIssue_ReturnsAPIErrorForNon2xx(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"message":"bad token"}`, http.StatusUnauthorized)
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "bad")
	_, err := c.CreateIssue(context.Background(), "go-vas/vas", CreateIssueRequest{Title: "x"})
	if err == nil {
		t.Fatal("CreateIssue should return error")
	}
	var apiErr *APIError
	if !AsAPIError(err, &apiErr) {
		t.Fatalf("error %T should be APIError", err)
	}
	if apiErr.StatusCode != http.StatusUnauthorized || !strings.Contains(apiErr.Body, "bad token") {
		t.Fatalf("unexpected APIError: %+v", apiErr)
	}
}

func TestClientCreateIssue_RejectsMissingToken(t *testing.T) {
	c := NewClient("https://git.example.com", "")
	_, err := c.CreateIssue(context.Background(), "go-vas/vas", CreateIssueRequest{Title: "x"})
	if err == nil || !strings.Contains(err.Error(), "gitlab token is required") {
		t.Fatalf("CreateIssue error = %v, want missing token", err)
	}
}

func TestIssueMarshalForCommandOutput(t *testing.T) {
	issue := Issue{
		ID:        123,
		IID:       45,
		WebURL:    "https://git.example.com/go-vas/vas/-/issues/45",
		ProjectID: 7,
	}
	data, err := json.Marshal(issue)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}
	if !strings.Contains(string(data), `"web_url"`) {
		t.Fatalf("issue JSON should expose web_url, got %s", data)
	}
}

func assertFormValue(t *testing.T, values url.Values, key, want string) {
	t.Helper()
	if got := values.Get(key); got != want {
		t.Fatalf("%s = %q, want %q", key, got, want)
	}
}
