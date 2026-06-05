package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/studyzy/tapd-ai-cli/internal/mcp"
)

func TestActiveWorkspaceResourceUsesConfiguredDefault(t *testing.T) {
	t.Setenv("TAPD_WORKSPACE_ID", "")
	t.Setenv("TAPD_WATCH_WORKSPACES", "")

	pr, pw := io.Pipe()
	out := &bytes.Buffer{}
	server := mcp.NewServer(pr, out, io.Discard, nil)
	RegisterEventResources(server, "cfg-ws")

	done := make(chan error, 1)
	go func() { done <- server.Run(context.Background()) }()

	_, _ = io.WriteString(pw, `{"jsonrpc":"2.0","id":1,"method":"resources/read","params":{"uri":"tapd://workspaces/active"}}`+"\n")
	pw.Close()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("server did not exit")
	}

	var resp map[string]interface{}
	if err := json.Unmarshal([]byte(strings.TrimSpace(out.String())), &resp); err != nil {
		t.Fatalf("invalid response: %v\n%s", err, out.String())
	}
	result := resp["result"].(map[string]interface{})
	contents := result["contents"].([]interface{})
	text := contents[0].(map[string]interface{})["text"].(string)
	if !strings.Contains(text, `"default_workspace_id":"cfg-ws"`) {
		t.Fatalf("resource text=%s", text)
	}
}
