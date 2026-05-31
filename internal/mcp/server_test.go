package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"strings"
	"testing"
	"time"
)

// TestInitializeHandshake 验证 initialize 返回协议版本和 server info。
func TestInitializeHandshake(t *testing.T) {
	resps := runOnce(t, []string{
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`,
	})
	if len(resps) != 1 {
		t.Fatalf("expected 1 response, got %d", len(resps))
	}
	r := resps[0]
	if r["error"] != nil {
		t.Fatalf("initialize returned error: %v", r["error"])
	}
	res := r["result"].(map[string]interface{})
	if res["protocolVersion"] != protocolVersion {
		t.Fatalf("protocolVersion=%v want=%s", res["protocolVersion"], protocolVersion)
	}
	caps, ok := res["capabilities"].(map[string]interface{})
	if !ok || caps["tools"] == nil {
		t.Fatalf("capabilities.tools missing: %+v", res)
	}
}

// TestNotificationsInitialized 验证通知不返回响应。
func TestNotificationsInitialized(t *testing.T) {
	resps := runOnce(t, []string{
		`{"jsonrpc":"2.0","method":"notifications/initialized"}`,
		`{"jsonrpc":"2.0","id":2,"method":"ping"}`,
	})
	// 只应有 ping 一条响应，notifications/initialized 不回
	if len(resps) != 1 {
		t.Fatalf("expected 1 response, got %d: %+v", len(resps), resps)
	}
	if id, _ := resps[0]["id"].(float64); id != 2 {
		t.Fatalf("expected ping reply id=2, got %v", resps[0]["id"])
	}
}

// TestToolsList_StableOrder 注册三个工具后列出，验证按字典序输出。
func TestToolsList_StableOrder(t *testing.T) {
	server, in, out, done := newTestServer()
	defer cleanup(t, in, done)

	for _, name := range []string{"zeta", "alpha", "mu"} {
		name := name
		server.Register(&Tool{
			Name:        name,
			Description: "stub",
			InputSchema: schema(`{"type":"object"}`),
			Handler: func(_ context.Context, _ json.RawMessage) (interface{}, error) {
				return name, nil
			},
		})
	}

	mustWrite(t, in, `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`)
	resp := mustReadOne(t, out)
	tools := resp["result"].(map[string]interface{})["tools"].([]interface{})
	got := []string{}
	for _, x := range tools {
		got = append(got, x.(map[string]interface{})["name"].(string))
	}
	want := []string{"alpha", "mu", "zeta"}
	if !equalStringSlice(got, want) {
		t.Fatalf("tools/list order: got=%v want=%v", got, want)
	}
}

// TestToolsCall_Success 验证 handler 返回值被包装成 content 数组。
func TestToolsCall_Success(t *testing.T) {
	server, in, out, done := newTestServer()
	defer cleanup(t, in, done)

	server.Register(&Tool{
		Name:        "echo",
		Description: "stub",
		InputSchema: schema(`{"type":"object"}`),
		Handler: func(_ context.Context, args json.RawMessage) (interface{}, error) {
			return map[string]interface{}{"echo": json.RawMessage(args)}, nil
		},
	})

	mustWrite(t, in,
		`{"jsonrpc":"2.0","id":7,"method":"tools/call","params":{"name":"echo","arguments":{"hello":"world"}}}`)
	resp := mustReadOne(t, out)
	res := resp["result"].(map[string]interface{})
	content := res["content"].([]interface{})
	if len(content) != 1 {
		t.Fatalf("want 1 content item, got %d", len(content))
	}
	first := content[0].(map[string]interface{})
	if first["type"] != "text" {
		t.Fatalf("content type=%v", first["type"])
	}
	if !strings.Contains(first["text"].(string), `"hello":"world"`) {
		t.Fatalf("text missing args: %v", first["text"])
	}
	if res["isError"] != nil {
		t.Fatalf("isError should not be set on success: %v", res["isError"])
	}
}

// TestToolsCall_HandlerError 验证 handler 返回错误时回包成 isError=true，而不是 RPC error。
func TestToolsCall_HandlerError(t *testing.T) {
	server, in, out, done := newTestServer()
	defer cleanup(t, in, done)

	server.Register(&Tool{
		Name:        "boom",
		Description: "stub",
		InputSchema: schema(`{"type":"object"}`),
		Handler: func(_ context.Context, _ json.RawMessage) (interface{}, error) {
			return nil, io.ErrUnexpectedEOF
		},
	})

	mustWrite(t, in, `{"jsonrpc":"2.0","id":8,"method":"tools/call","params":{"name":"boom","arguments":{}}}`)
	resp := mustReadOne(t, out)
	if resp["error"] != nil {
		t.Fatalf("expected isError content, got rpc error: %v", resp["error"])
	}
	res := resp["result"].(map[string]interface{})
	if res["isError"] != true {
		t.Fatalf("isError should be true: %v", res)
	}
}

// TestToolsCall_UnknownTool 走 method-not-found 路径。
func TestToolsCall_UnknownTool(t *testing.T) {
	resps := runOnce(t, []string{
		`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"nope","arguments":{}}}`,
	})
	if len(resps) != 1 {
		t.Fatalf("expected 1 response, got %d", len(resps))
	}
	errObj, ok := resps[0]["error"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected error obj, got %+v", resps[0])
	}
	if int(errObj["code"].(float64)) != errMethodNotFound {
		t.Fatalf("code=%v want=%d", errObj["code"], errMethodNotFound)
	}
}

// TestParseError 输入非法 JSON 时回 parse error。
func TestParseError(t *testing.T) {
	resps := runOnce(t, []string{`not json`})
	if len(resps) != 1 {
		t.Fatalf("expected 1 response, got %d", len(resps))
	}
	errObj := resps[0]["error"].(map[string]interface{})
	if int(errObj["code"].(float64)) != errParseError {
		t.Fatalf("expected parse error, got %v", errObj)
	}
}

// ─── helpers ───

func newTestServer() (*Server, io.WriteCloser, *bytes.Buffer, chan error) {
	pr, pw := io.Pipe()
	out := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	server := NewServer(pr, syncBuffer{out}, stderr, nil)
	done := make(chan error, 1)
	go func() {
		done <- server.Run(context.Background())
	}()
	return server, pw, out, done
}

// syncBuffer 让 server.Run 在写时触发 buffer 写入；正常 *bytes.Buffer 已经满足 io.Writer，
// 但通过 wrapper 提醒读者：这里没有并发保护，测试中是单写者单读者。
type syncBuffer struct{ *bytes.Buffer }

func cleanup(t *testing.T, in io.WriteCloser, done chan error) {
	t.Helper()
	in.Close()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("server did not exit after stdin close")
	}
}

func mustWrite(t *testing.T, in io.Writer, line string) {
	t.Helper()
	if _, err := io.WriteString(in, line+"\n"); err != nil {
		t.Fatalf("write: %v", err)
	}
}

// mustReadOne 等到 out 里出现一行 JSON 后解析；最多等 500ms。
func mustReadOne(t *testing.T, out *bytes.Buffer) map[string]interface{} {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for {
		s := out.String()
		if idx := strings.Index(s, "\n"); idx >= 0 {
			line := s[:idx]
			out.Next(idx + 1)
			var m map[string]interface{}
			if err := json.Unmarshal([]byte(line), &m); err != nil {
				t.Fatalf("invalid response json: %v\nline: %s", err, line)
			}
			return m
		}
		if time.Now().After(deadline) {
			t.Fatalf("timeout waiting for response; buffer: %q", out.String())
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// runOnce 写一组请求，关闭 stdin，读所有响应。便于"一发一收"风格测试。
func runOnce(t *testing.T, lines []string) []map[string]interface{} {
	t.Helper()
	pr, pw := io.Pipe()
	out := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	server := NewServer(pr, syncBuffer{out}, stderr, nil)

	done := make(chan error, 1)
	go func() {
		done <- server.Run(context.Background())
	}()

	for _, l := range lines {
		if _, err := io.WriteString(pw, l+"\n"); err != nil {
			t.Fatalf("write: %v", err)
		}
	}
	pw.Close()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("server did not exit")
	}

	resps := []map[string]interface{}{}
	for _, line := range strings.Split(strings.TrimRight(out.String(), "\n"), "\n") {
		if line == "" {
			continue
		}
		var m map[string]interface{}
		if err := json.Unmarshal([]byte(line), &m); err != nil {
			t.Fatalf("invalid response json: %v\nline: %s", err, line)
		}
		resps = append(resps, m)
	}
	return resps
}

func equalStringSlice(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
