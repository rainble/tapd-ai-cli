// Package cmd 中的 watch.go 实现 SSE 长连接订阅 TAPD webhook 事件流。
//
// 服务端：vas/app/upower/interface 内的 tapd 模块（GET /x/upower/tapd/events）。
// 协议：标准 SSE，事件 data 字段是 JSON {"received_at":...,"event":...原始 webhook payload...}。
//
// 当 watch 进程从 stdin 收到 EOF / SIGINT / SIGTERM 时优雅退出；
// 网络中断时自动重连，重连间隔指数退避（1s → 30s 上限）。
package cmd

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"github.com/studyzy/tapd-ai-cli/internal/output"
)

var (
	flagWatchEndpoint string
	flagWatchToken    string
	flagWatchExec     string
	flagWatchPretty   bool
	flagWatchOnce     bool
	flagWatchFilters  []string
	flagWatchNoState  bool

	// watchFilters 是已解析的过滤规则；resolveWatchConfig 之后填充。
	watchFilters []filterRule

	// watchStateRef 是当前 stream 用到的水位状态;emit 成功后回写。
	// 由 runWatch 在解析配置后赋值,stream/streamOnce/emit 共享。
	watchStateRef *watchState

	// watchEventsRef 是当前 stream 的事件缓存;emit 成功后追加到本地 jsonl 文件。
	watchEventsRef *eventCache
)

// watchCmd 订阅服务端 SSE 事件流，把事件转成一行一条紧凑 JSON 写到 stdout。
var watchCmd = &cobra.Command{
	Use:   "watch",
	Short: "订阅 TAPD webhook 事件流（SSE 长连接）",
	Long: `订阅由 vas/app/upower/interface 中转的 TAPD webhook 事件流。

每收到一个事件，输出一行 JSON 到 stdout：
  {"id":12,"received_at":1717000000000000000,"event":{...原始 webhook payload...}}

可选 --exec 指定一个外部命令，每个事件作为 stdin 喂给该命令的一个新进程；
事件量大时建议自己起 worker pool，watch 不做并发控制。

配置来源（优先级从高到低）：
  --endpoint / --token  CLI flag
  环境变量 TAPD_WATCH_ENDPOINT / TAPD_SUBSCRIBE_TOKEN
  ./.tapd.json 或 ~/.tapd.json 中的 watch_endpoint / subscribe_token`,
	RunE: runWatch,
}

func init() {
	watchCmd.Flags().StringVar(&flagWatchEndpoint, "endpoint", "",
		"SSE 端点 URL，覆盖配置文件中的 watch_endpoint")
	watchCmd.Flags().StringVar(&flagWatchToken, "token", "",
		"订阅鉴权 token，覆盖配置文件中的 subscribe_token")
	watchCmd.Flags().StringVar(&flagWatchExec, "exec", "",
		"为每个事件起一个进程执行该命令，事件 JSON 通过 stdin 传入")
	watchCmd.Flags().BoolVar(&flagWatchPretty, "pretty-json", false,
		"输出事件时使用带缩进的 JSON（默认紧凑单行）")
	watchCmd.Flags().BoolVar(&flagWatchOnce, "once", false,
		"收到一个事件后立刻退出，常用于联调")
	watchCmd.Flags().StringSliceVar(&flagWatchFilters, "filter", nil,
		"过滤事件：<path>=<glob>[,<glob>...]，可多次指定（多 filter 间 AND，单 filter 内 OR）。\n"+
			"示例：--filter event.event=story_*,bug_create --filter event.workspace_id=20063271")
	watchCmd.Flags().BoolVar(&flagWatchNoState, "no-state", false,
		"禁用 last_event_id 水位持久化,每次启动都从服务端拉全量历史(测试/调试用)")

	rootCmd.AddCommand(watchCmd)
}

// runWatch 是 watch 命令的入口。watch 不需要 TAPD API 凭据，
// 但 root 的 PersistentPreRunE 会检查；为保持一致，这里手动覆盖端点检查。
func runWatch(cmd *cobra.Command, args []string) error {
	endpoint, token := resolveWatchConfig()
	if endpoint == "" {
		output.PrintError(os.Stderr, "watch_endpoint_missing",
			"watch endpoint not configured",
			"set --endpoint, env TAPD_WATCH_ENDPOINT, or watch_endpoint in .tapd.json")
		os.Exit(output.ExitParamError)
		return nil
	}
	if _, err := url.Parse(endpoint); err != nil {
		output.PrintError(os.Stderr, "watch_endpoint_invalid", err.Error(),
			"endpoint must be a valid URL like https://host/x/upower/tapd/events")
		os.Exit(output.ExitParamError)
		return nil
	}

	rules, err := parseWatchFilters(flagWatchFilters)
	if err != nil {
		output.PrintError(os.Stderr, "watch_filter_invalid", err.Error(),
			"filter format: <path>=<glob>[,<glob>...]")
		os.Exit(output.ExitParamError)
		return nil
	}
	// 通过 TAPD_WATCH_WORKSPACES 环境变量做粗粒度过滤,语法 "id1,id2,id3"。
	// 比 --filter event.workspace_id=... 更直观,适合长期固定关注几个空间的场景。
	if wsList := strings.TrimSpace(os.Getenv("TAPD_WATCH_WORKSPACES")); wsList != "" {
		rules = append(rules, filterRule{
			raw:   "TAPD_WATCH_WORKSPACES=" + wsList,
			path:  []string{"event", "workspace_id"},
			globs: splitGlobs(wsList),
		})
	}
	watchFilters = rules

	if flagWatchNoState {
		watchStateRef = &watchState{}
		watchEventsRef = &eventCache{} // 内存模式
	} else {
		watchStateRef = newWatchState()
		watchEventsRef = newEventCache()
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	stream(ctx, endpoint, token)
	return nil
}

// resolveWatchConfig 按 flag > env > 配置文件的优先级解析端点和 token。
// 注意 appConfig 由 root 的 PersistentPreRunE 加载——但 watch 也允许在
// 未配置 TAPD 凭据时单独使用，因此 fallback 直接读环境变量。
func resolveWatchConfig() (endpoint, token string) {
	endpoint = flagWatchEndpoint
	token = flagWatchToken

	if appConfig != nil {
		if endpoint == "" {
			endpoint = appConfig.WatchEndpoint
		}
		if token == "" {
			token = appConfig.SubscribeToken
		}
	}
	if endpoint == "" {
		endpoint = os.Getenv("TAPD_WATCH_ENDPOINT")
	}
	if token == "" {
		token = os.Getenv("TAPD_SUBSCRIBE_TOKEN")
	}
	return endpoint, token
}

// streamEvent 是 watch 输出到 stdout 和传给 --exec 的事件结构。
type streamEvent struct {
	ID         uint64          `json:"id"`
	ReceivedAt int64           `json:"received_at"`
	Event      json.RawMessage `json:"event"`
}

// stream 维持 SSE 长连接：连上 → 持续读事件 → 断开 → 退避后重连。
// 退出条件：ctx 取消（信号/EOF）或 --once 命中。
func stream(ctx context.Context, endpoint, token string) {
	const (
		minBackoff = time.Second
		maxBackoff = 30 * time.Second
	)
	backoff := minBackoff

	for {
		err := streamOnce(ctx, endpoint, token)
		if ctx.Err() != nil {
			return
		}
		if err == errOnceDone {
			return
		}
		if err != nil {
			fmt.Fprintf(os.Stderr, "watch: connection lost: %v; reconnect in %s\n", err, backoff)
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(backoff):
		}
		backoff *= 2
		if backoff > maxBackoff {
			backoff = maxBackoff
		}
	}
}

// errOnceDone 是 --once 模式正常退出的哨兵错误。
var errOnceDone = fmt.Errorf("watch: once event received")

// streamOnce 建立一次 SSE 连接，读取并处理事件直到连接断开或 ctx 取消。
func streamOnce(ctx context.Context, endpoint, token string) error {
	if watchStateRef == nil {
		// 未走 runWatch 入口（直接被测试或外部调用)时,降级为内存模式,避免空指针
		watchStateRef = &watchState{}
	}
	target, err := injectLastID(endpoint, watchStateRef.LastSeen())
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Cache-Control", "no-cache")
	if token != "" {
		req.Header.Set("X-TAPD-Token", token)
	}
	if v := watchStateRef.LastSeen(); v > 0 {
		// 顺手带上 SSE 标准 header,服务端任一识别即可
		req.Header.Set("Last-Event-ID", strconv.FormatUint(v, 10))
	}

	// 长连接不能给 Client 设 Timeout，否则连接会被周期性 kill；
	// 通过 ctx + 服务端心跳保活。
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<14))
		return fmt.Errorf("status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	return readSSE(resp.Body)
}

// readSSE 按 SSE 规范逐行解析帧，data 字段累积后调用 emit。
// 正常读完返回 io.EOF（上层用它触发重连）；--once 模式 emit 一条后立刻返 errOnceDone。
func readSSE(r io.Reader) error {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 64*1024), 4*1024*1024)

	var dataLines []string
	flush := func() error {
		if len(dataLines) == 0 {
			return nil
		}
		err := emit(strings.Join(dataLines, "\n"))
		dataLines = dataLines[:0]
		if err != nil {
			return err
		}
		if flagWatchOnce {
			return errOnceDone
		}
		return nil
	}
	for scanner.Scan() {
		line := scanner.Text()
		switch {
		case line == "":
			if err := flush(); err != nil {
				return err
			}
		case strings.HasPrefix(line, ":"):
			// 注释/心跳，忽略
		case strings.HasPrefix(line, "data:"):
			dataLines = append(dataLines, strings.TrimSpace(strings.TrimPrefix(line, "data:")))
		default:
			// event:/id:/retry: 字段我们暂时不需要，丢弃
		}
	}
	// 流到尾 flush 剩余 data 并返回 io.EOF（让上层重连）
	if err := flush(); err != nil {
		return err
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	return io.EOF
}

// emit 解析一条事件 JSON，按 --filter 规则过滤后输出到 stdout 并按需触发 --exec。
func emit(data string) error {
	var ev streamEvent
	if err := json.Unmarshal([]byte(data), &ev); err != nil {
		fmt.Fprintf(os.Stderr, "watch: invalid event json: %v\n", err)
		return nil
	}
	if !matchAll(&ev, watchFilters) {
		return nil
	}

	var serialized []byte
	var err error
	if flagWatchPretty {
		serialized, err = json.MarshalIndent(&ev, "", "  ")
	} else {
		serialized, err = json.Marshal(&ev)
	}
	if err != nil {
		return err
	}

	fmt.Fprintln(os.Stdout, string(serialized))

	// 持久化水位供重启去重
	if watchStateRef != nil {
		watchStateRef.Update(ev.ID)
	}

	// 追加到事件缓存供 MCP server 读取
	if watchEventsRef != nil {
		watchEventsRef.Append(&ev)
	}

	if flagWatchExec != "" {
		runExec(flagWatchExec, serialized)
	}
	return nil
}

// runExec 起一个 sh -c 子进程执行 --exec 指定的命令，事件 JSON 从 stdin 喂入。
// 子进程的 stdout/stderr 直通终端，错误只打印不传播——单条事件失败不应影响主循环。
func runExec(command string, payload []byte) {
	cmd := exec.Command("sh", "-c", command)
	cmd.Stdin = strings.NewReader(string(payload))
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "watch: exec failed: %v\n", err)
	}
}

// injectLastID 把当前水位作为 last_id query 拼到 endpoint 上;水位为 0 时返回原 URL。
// 已有 last_id 时覆盖,避免用户在 endpoint 里写死的旧值压住实际进度。
func injectLastID(endpoint string, lastID uint64) (string, error) {
	if lastID == 0 {
		return endpoint, nil
	}
	u, err := url.Parse(endpoint)
	if err != nil {
		return "", err
	}
	q := u.Query()
	q.Set("last_id", strconv.FormatUint(lastID, 10))
	u.RawQuery = q.Encode()
	return u.String(), nil
}
