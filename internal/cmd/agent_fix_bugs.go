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
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"github.com/studyzy/tapd-ai-cli/internal/output"
)

var (
	flagAgentRepo            string
	flagAgentCmd             string
	flagAgentTestCmd         string
	flagAgentOnStartStatus   string
	flagAgentOnSuccessStatus string
	flagAgentOnFailureStatus string
	flagAgentCurrentUser     string
	flagAgentResolution      string
	flagAgentAllowDirty      bool
	flagAgentOnce            bool
	flagAgentOutputLimit     int
	flagAgentBranchStrategy  string
	flagAgentMRRemote        string
	flagAgentMRBranchPrefix  string
)

var agentFixBugsBlockedEventID uint64

var agentCmd = &cobra.Command{
	Use:   "agent",
	Short: "本地 AI agent 自动化",
}

var agentFixBugsCmd = &cobra.Command{
	Use:   "fix-bugs",
	Short: "监听 TAPD bug 事件并在本地仓库自动修复",
	RunE:  runAgentFixBugs,
}

type agentFixBugsConfig struct {
	repo            string
	endpoint        string
	token           string
	agentCmd        string
	testCmd         string
	onStartStatus   string
	onSuccessStatus string
	onFailureStatus string
	currentUser     string
	resolution      string
	allowDirty      bool
	once            bool
	outputLimit     int
	workspaceID     string
	branchStrategy  string
	mrRemote        string
	mrBranchPrefix  string
}

func init() {
	agentFixBugsCmd.Flags().StringVar(&flagAgentRepo, "repo", "", "本地仓库路径")
	agentFixBugsCmd.Flags().StringVar(&flagWatchEndpoint, "endpoint", "", "SSE 端点 URL，覆盖配置文件中的 watch_endpoint")
	agentFixBugsCmd.Flags().StringVar(&flagWatchToken, "token", "", "订阅鉴权 token，覆盖配置文件中的 subscribe_token")
	agentFixBugsCmd.Flags().StringVar(&flagAgentCmd, "agent-cmd", "", "本地 coding agent 命令")
	agentFixBugsCmd.Flags().StringVar(&flagAgentTestCmd, "test-cmd", "", "修复后验证命令")
	agentFixBugsCmd.Flags().StringVar(&flagAgentOnStartStatus, "on-start-status", "", "开始处理时流转到的 TAPD bug 状态")
	agentFixBugsCmd.Flags().StringVar(&flagAgentOnSuccessStatus, "on-success-status", "", "修复验证成功后流转到的 TAPD bug 状态")
	agentFixBugsCmd.Flags().StringVar(&flagAgentOnFailureStatus, "on-failure-status", "", "失败后流转到的 TAPD bug 状态")
	agentFixBugsCmd.Flags().StringVar(&flagAgentCurrentUser, "current-user", "", "TAPD 状态流转操作人，默认当前认证用户")
	agentFixBugsCmd.Flags().StringVar(&flagAgentResolution, "resolution", "fixed", "成功流转时写入的 resolution")
	agentFixBugsCmd.Flags().BoolVar(&flagAgentAllowDirty, "allow-dirty", false, "允许在 dirty working tree 中运行 agent")
	agentFixBugsCmd.Flags().BoolVar(&flagAgentOnce, "once", false, "处理一个 bug 事件后退出")
	agentFixBugsCmd.Flags().IntVar(&flagAgentOutputLimit, "output-limit", defaultCommandOutputLimit, "写入 TAPD 评论的单段输出截断字节数")
	agentFixBugsCmd.Flags().StringVar(&flagAgentBranchStrategy, "branch-strategy", "current", "本地分支策略：current 或 linked-mr")
	agentFixBugsCmd.Flags().StringVar(&flagAgentMRRemote, "mr-remote", "origin", "linked-mr 策略使用的 Git remote")
	agentFixBugsCmd.Flags().StringVar(&flagAgentMRBranchPrefix, "mr-branch-prefix", "tapd-agent/mr-", "linked-mr 策略创建的本地分支名前缀")

	agentCmd.AddCommand(agentFixBugsCmd)
	rootCmd.AddCommand(agentCmd)
}

func resolveAgentFixBugsConfig() (agentFixBugsConfig, error) {
	endpoint, token := resolveWatchConfig()
	cfg := agentFixBugsConfig{
		repo:            flagAgentRepo,
		endpoint:        endpoint,
		token:           token,
		agentCmd:        fallbackString(flagAgentCmd, "codex exec --full-auto"),
		testCmd:         flagAgentTestCmd,
		onStartStatus:   flagAgentOnStartStatus,
		onSuccessStatus: flagAgentOnSuccessStatus,
		onFailureStatus: flagAgentOnFailureStatus,
		currentUser:     flagAgentCurrentUser,
		resolution:      fallbackString(flagAgentResolution, "fixed"),
		allowDirty:      flagAgentAllowDirty,
		once:            flagAgentOnce,
		outputLimit:     flagAgentOutputLimit,
		workspaceID:     flagWorkspaceID,
		branchStrategy:  fallbackString(flagAgentBranchStrategy, "current"),
		mrRemote:        fallbackString(flagAgentMRRemote, "origin"),
		mrBranchPrefix:  fallbackString(flagAgentMRBranchPrefix, "tapd-agent/mr-"),
	}
	if cfg.outputLimit <= 0 {
		cfg.outputLimit = defaultCommandOutputLimit
	}
	if strings.TrimSpace(cfg.repo) == "" {
		return cfg, fmt.Errorf("--repo is required")
	}
	if cfg.onSuccessStatus != "" && strings.TrimSpace(cfg.testCmd) == "" {
		return cfg, fmt.Errorf("--test-cmd is required when --on-success-status is set")
	}
	if cfg.branchStrategy != "current" && cfg.branchStrategy != branchStrategyLinkedMR {
		return cfg, fmt.Errorf("--branch-strategy must be current or linked-mr")
	}
	if err := validateGitArg(cfg.mrRemote, "--mr-remote"); err != nil {
		return cfg, err
	}
	if err := validateGitArg(cfg.mrBranchPrefix, "--mr-branch-prefix"); err != nil {
		return cfg, err
	}
	parsed, err := url.Parse(cfg.endpoint)
	if cfg.endpoint == "" || err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return cfg, fmt.Errorf("--endpoint or watch_endpoint config is required")
	}
	return cfg, nil
}

func runAgentFixBugs(cmd *cobra.Command, args []string) error {
	cfg, err := resolveAgentFixBugsConfig()
	if err != nil {
		output.PrintError(os.Stderr, "invalid_agent_config", err.Error(), "provide --repo and SSE endpoint config")
		os.Exit(output.ExitParamError)
		return nil
	}
	currentUser := cfg.currentUser
	if currentUser == "" {
		currentUser = ensureNick()
	}
	worker := &bugFixWorker{
		tapd:            sdkBugFixTapdClient{},
		runner:          shellCommandRunner{},
		repo:            cfg.repo,
		agentCmd:        cfg.agentCmd,
		testCmd:         cfg.testCmd,
		onStartStatus:   cfg.onStartStatus,
		onSuccessStatus: cfg.onSuccessStatus,
		onFailureStatus: cfg.onFailureStatus,
		currentUser:     currentUser,
		resolution:      cfg.resolution,
		allowDirty:      cfg.allowDirty,
		outputLimit:     cfg.outputLimit,
		branchStrategy:  cfg.branchStrategy,
		mrRemote:        cfg.mrRemote,
		mrBranchPrefix:  cfg.mrBranchPrefix,
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	runAgentFixBugsStream(ctx, cfg, worker)
	return nil
}

func runAgentFixBugsStream(ctx context.Context, cfg agentFixBugsConfig, worker *bugFixWorker) {
	const minBackoff = time.Second
	const maxBackoff = 30 * time.Second
	backoff := minBackoff
	for {
		err := agentFixBugsStreamOnce(ctx, cfg, worker)
		if ctx.Err() != nil || err == errOnceDone {
			return
		}
		if err != nil {
			fmt.Fprintf(os.Stderr, "agent fix-bugs: connection lost: %v; reconnect in %s\n", err, backoff)
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

func agentFixBugsStreamOnce(ctx context.Context, cfg agentFixBugsConfig, worker *bugFixWorker) error {
	if watchStateRef == nil {
		watchStateRef = newWatchState()
	}
	target, err := injectLastID(cfg.endpoint, watchStateRef.LastSeen())
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Cache-Control", "no-cache")
	if cfg.token != "" {
		req.Header.Set("X-TAPD-Token", cfg.token)
	}
	if v := watchStateRef.LastSeen(); v > 0 {
		req.Header.Set("Last-Event-ID", strconv.FormatUint(v, 10))
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<14))
		return fmt.Errorf("status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return readAgentFixBugsSSE(ctx, resp.Body, cfg, worker)
}

func readAgentFixBugsSSE(ctx context.Context, r io.Reader, cfg agentFixBugsConfig, worker *bugFixWorker) error {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 64*1024), 4*1024*1024)
	var dataLines []string
	flush := func() error {
		if len(dataLines) == 0 {
			return nil
		}
		data := strings.Join(dataLines, "\n")
		dataLines = dataLines[:0]
		handled, err := handleAgentFixBugsEvent(ctx, data, cfg, worker)
		if err != nil {
			return err
		}
		if handled && cfg.once {
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
		case strings.HasPrefix(line, "data:"):
			dataLines = append(dataLines, strings.TrimSpace(strings.TrimPrefix(line, "data:")))
		}
	}
	if err := flush(); err != nil {
		return err
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	return io.EOF
}

func handleAgentFixBugsEvent(ctx context.Context, data string, cfg agentFixBugsConfig, worker *bugFixWorker) (bool, error) {
	var ev streamEvent
	if err := json.Unmarshal([]byte(data), &ev); err != nil {
		fmt.Fprintf(os.Stderr, "agent fix-bugs: invalid event json: %v\n", err)
		return false, nil
	}
	target, ok, reason := extractBugEventTarget(&ev)
	if !ok {
		fmt.Fprintf(os.Stderr, "agent fix-bugs: skip event id=%d reason=%s\n", ev.ID, reason)
		advanceAgentFixBugsWatermark(ev.ID)
		return false, nil
	}
	if cfg.workspaceID != "" && target.WorkspaceID != cfg.workspaceID {
		fmt.Fprintf(os.Stderr, "agent fix-bugs: skip event id=%d workspace=%s\n", ev.ID, target.WorkspaceID)
		advanceAgentFixBugsWatermark(ev.ID)
		return false, nil
	}
	res := worker.handleTarget(ctx, target)
	_ = output.PrintJSON(os.Stdout, res, true)
	if res.Status == "success" || res.Status == "skipped" {
		advanceAgentFixBugsWatermark(ev.ID)
	} else {
		blockAgentFixBugsWatermark(ev.ID)
	}
	return true, nil
}

func blockAgentFixBugsWatermark(eventID uint64) {
	if eventID == 0 {
		return
	}
	if agentFixBugsBlockedEventID == 0 || eventID < agentFixBugsBlockedEventID {
		agentFixBugsBlockedEventID = eventID
	}
}

func advanceAgentFixBugsWatermark(eventID uint64) {
	if watchStateRef == nil || eventID == 0 {
		return
	}
	if agentFixBugsBlockedEventID == 0 {
		watchStateRef.Update(eventID)
		return
	}
	if eventID < agentFixBugsBlockedEventID {
		watchStateRef.Update(eventID)
		return
	}
	if eventID == agentFixBugsBlockedEventID {
		watchStateRef.Update(eventID)
		agentFixBugsBlockedEventID = 0
	}
}
