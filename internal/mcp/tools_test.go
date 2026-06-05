package mcp

import (
	"context"
	"encoding/json"
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
