package cmd

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// TestStreamOnce_AuthHeader 验证 token 通过 X-TAPD-Token 头送出，并能解析一条事件。
func TestStreamOnce_AuthHeader(t *testing.T) {
	var gotToken atomic.Value
	gotToken.Store("")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotToken.Store(r.Header.Get("X-TAPD-Token"))
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		f, _ := w.(http.Flusher)
		// 推一条事件，--once 命中后客户端会主动断开
		w.Write([]byte("event: tapd\nid: 1\ndata: {\"id\":1,\"received_at\":111,\"event\":{\"hello\":\"world\"}}\n\n"))
		f.Flush()
		// 阻塞一下保证客户端读到帧
		time.Sleep(200 * time.Millisecond)
	}))
	defer srv.Close()

	flagWatchOnce = true
	flagWatchExec = ""
	flagWatchPretty = false
	t.Cleanup(func() { flagWatchOnce = false })

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	if err := streamOnce(ctx, srv.URL, "tok-xyz"); err != errOnceDone {
		t.Fatalf("expected errOnceDone, got %v", err)
	}
	if got := gotToken.Load().(string); got != "tok-xyz" {
		t.Fatalf("expected token tok-xyz, got %q", got)
	}
}

// TestStreamOnce_Unauthorized 服务端返 401 时返回错误而不是误退出。
func TestStreamOnce_Unauthorized(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte("bad token"))
	}))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err := streamOnce(ctx, srv.URL, "wrong")
	if err == nil {
		t.Fatal("expected error on 401")
	}
	if !strings.Contains(err.Error(), "status=401") {
		t.Fatalf("expected status=401 in err, got %v", err)
	}
}

// TestReadSSE_MultipleFrames 验证多帧 + 心跳注释 + multi-line data 都能正确解析。
// 非 once 模式下，流终止时返回 io.EOF。
func TestReadSSE_MultipleFrames(t *testing.T) {
	body := strings.NewReader(strings.Join([]string{
		":hb 1",
		"",
		"event: tapd",
		"id: 1",
		"data: {\"id\":1,\"received_at\":1,\"event\":{\"a\":1}}",
		"",
		":hb 2",
		"",
		"event: tapd",
		"id: 2",
		"data: {\"id\":2,\"received_at\":2,\"event\":{\"a\":2}}",
		"",
	}, "\n"))

	flagWatchOnce = false
	flagWatchExec = ""

	if err := readSSE(body); err != io.EOF {
		t.Fatalf("expected io.EOF on stream end, got %v", err)
	}
}

// TestResolveWatchConfig 验证 flag > env > appConfig 的优先级。
func TestResolveWatchConfig(t *testing.T) {
	defer func() {
		flagWatchEndpoint = ""
		flagWatchToken = ""
		appConfig = nil
	}()

	t.Setenv("TAPD_WATCH_ENDPOINT", "")
	t.Setenv("TAPD_SUBSCRIBE_TOKEN", "")

	flagWatchEndpoint = "https://flag/events"
	flagWatchToken = "flag-token"
	endpoint, token := resolveWatchConfig()
	if endpoint != "https://flag/events" || token != "flag-token" {
		t.Fatalf("flag should win, got %s/%s", endpoint, token)
	}

	flagWatchEndpoint = ""
	flagWatchToken = ""
	t.Setenv("TAPD_WATCH_ENDPOINT", "https://env/events")
	t.Setenv("TAPD_SUBSCRIBE_TOKEN", "env-token")
	endpoint, token = resolveWatchConfig()
	if endpoint != "https://env/events" || token != "env-token" {
		t.Fatalf("env should win when no flag, got %s/%s", endpoint, token)
	}
}
