// Package cmd 定义了 tapd-ai-cli 的所有 Cobra 命令
package cmd

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/studyzy/tapd-ai-cli/internal/config"
	"github.com/studyzy/tapd-ai-cli/internal/output"
	tapd "github.com/studyzy/tapd-sdk-go"
	"github.com/studyzy/tapd-sdk-go/model"
)

var (
	// 全局标志
	flagWorkspaceID string
	flagPretty      bool
	flagJSON        bool
	flagNoComments  bool
	flagAccessToken string
	flagAPIUser     string
	flagAPIPassword string

	// 全局共享的客户端和配置
	apiClient  *tapd.Client
	appConfig  *config.Config
	rawBaseURL string // listWithFilters 使用的 API 基础地址，与 apiClient 保持同步

	// list 命令共享的 filter 标志
	flagFilter []string

	// filterFlagDesc 是 --filter 标志的描述文本，统一常量避免 13 处重复
	filterFlagDesc = "高级过滤条件（可重复，格式：field=OP<value>，支持 LIKE/EQ/CONTAINS 等 OpenAPI 特殊查询语法）"
)

// rootCmd 是 CLI 的根命令
var rootCmd = &cobra.Command{
	Use:   "tapd",
	Short: "面向 AI Agent 的 TAPD 命令行工具",
	Long:  "tapd-ai-cli 是一个面向 AI Agent 的 TAPD 命令行工具，通过 TAPD Open API 实现项目管理核心操作。",
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// auth login 命令不需要预加载配置和客户端
		if cmd.Name() == "login" || cmd.Name() == "init" {
			return nil
		}
		// --version 不需要认证
		if v, _ := cmd.Flags().GetBool("version"); v {
			return nil
		}
		return initClientAndConfig(cmd)
	},
	SilenceUsage:  true,
	SilenceErrors: true,
}

// Execute 执行根命令
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	// 根命令自定义 help：输出紧凑参考卡（原 spec 子命令的功能）
	defaultHelp := rootCmd.HelpFunc()
	rootCmd.SetHelpFunc(func(cmd *cobra.Command, args []string) {
		if cmd != rootCmd {
			defaultHelp(cmd, args)
			return
		}
		lines := buildSpecLines(rootCmd)
		printSpecOutput(os.Stdout, rootCmd, lines)
	})

	rootCmd.PersistentFlags().StringVar(&flagWorkspaceID, "workspace-id", "", "指定工作区 ID（覆盖本地配置）")
	rootCmd.PersistentFlags().BoolVar(&flagPretty, "pretty", false, "输出带缩进的 JSON，仅供人类阅读，AI Agent 不应使用（浪费 token）")
	rootCmd.PersistentFlags().BoolVar(&flagJSON, "json", false, "强制 JSON 输出（列表默认已是 JSON，详情默认 Markdown 更省 token；仅在需要从详情提取字段时使用）")
	rootCmd.PersistentFlags().StringVar(&flagAccessToken, "access-token", "", "TAPD Access Token")
	rootCmd.PersistentFlags().StringVar(&flagAPIUser, "api-user", "", "TAPD API 用户名")
	rootCmd.PersistentFlags().StringVar(&flagAPIPassword, "api-password", "", "TAPD API 密码")
	rootCmd.PersistentFlags().BoolVar(&flagNoComments, "no-comments", false, "不展示评论")
}

// initClientAndConfig 初始化配置和 API 客户端
func initClientAndConfig(cmd *cobra.Command) error {
	// 加载配置文件
	cfg, err := config.LoadConfig()
	if err != nil {
		return err
	}
	appConfig = cfg

	// 命令行标志覆盖配置
	accessToken := flagAccessToken
	apiUser := flagAPIUser
	apiPassword := flagAPIPassword

	if accessToken == "" {
		accessToken = cfg.AccessToken
	}
	if apiUser == "" {
		apiUser = cfg.APIUser
	}
	if apiPassword == "" {
		apiPassword = cfg.APIPassword
	}

	// 检查是否有有效凭据
	if accessToken == "" && (apiUser == "" || apiPassword == "") {
		output.PrintError(os.Stderr, "authentication_required",
			"No valid credentials found",
			"Run 'tapd auth login --access-token <token>' or 'tapd auth login --api-user <user> --api-password <password>'. "+
				"You can also set TAPD_ACCESS_TOKEN or TAPD_API_USER/TAPD_API_PASSWORD environment variables.")
		os.Exit(output.ExitAuthError)
	}

	apiClient = tapd.NewClientWithBaseURL(cfg.APIBaseURL, cfg.BaseURL, accessToken, apiUser, apiPassword)
	rawBaseURL = cfg.APIBaseURL

	// workspace-id 标志覆盖配置
	if flagWorkspaceID == "" {
		flagWorkspaceID = cfg.WorkspaceID
	}

	// 需要 workspace_id 的命令检查
	// 以下命令不需要 workspace_id：
	// - workspace list：列出所有工作区
	// - auth 子命令：认证操作
	// - url 命令：从 URL 中提取 workspace ID
	skipWorkspace := map[string]bool{"auth": true, "workspace": true}
	parentName := ""
	if cmd.Parent() != nil {
		parentName = cmd.Parent().Name()
	}
	needsWorkspace := !skipWorkspace[parentName] &&
		cmd.Name() != "url" &&
		!(cmd.Name() == "list" && parentName == "workspace")
	if needsWorkspace && flagWorkspaceID == "" {
		output.PrintError(os.Stderr, "workspace_required",
			"No workspace ID configured",
			"Run 'tapd workspace switch <id>' or use --workspace-id flag.")
		os.Exit(output.ExitParamError)
	}

	return nil
}

// cmdContext 安全获取命令的 context，当 cmd 为 nil 或 context 未设置时回退到 context.Background()
func cmdContext(cmd *cobra.Command) context.Context {
	if cmd != nil {
		defer func() { recover() }()
		if ctx := cmd.Context(); ctx != nil {
			return ctx
		}
	}
	return context.Background()
}

// ensureNick 按需获取当前用户昵称，仅在首次调用时发起 HTTP 请求
func ensureNick() string {
	if apiClient.GetNick() == "" {
		apiClient.FetchNick(context.Background())
	}
	return apiClient.GetNick()
}

// useJSONOutput 判断是否应使用 JSON 格式输出，--pretty 隐含 --json
func useJSONOutput() bool {
	return flagJSON || flagPretty
}

// printDetail 输出单条详情，默认 Markdown 格式，--json/--pretty 时输出 JSON
// bodyField 指定作为 Markdown body 的字段名（JSON tag 名称）
func printDetail(data interface{}, bodyField string) error {
	if useJSONOutput() {
		return output.PrintJSON(os.Stdout, data, !flagPretty)
	}
	return output.PrintMarkdown(os.Stdout, data, bodyField)
}

// printSuccessResponse 输出创建/更新操作的精简成功响应，节省 AI Agent token 消耗
func printSuccessResponse(id, url, workspaceID string) error {
	resp := &model.SuccessResponse{
		Success:     true,
		ID:          id,
		URL:         url,
		WorkspaceID: workspaceID,
	}
	return output.PrintJSON(os.Stdout, resp, !flagPretty)
}

// printComments 获取并输出条目的评论列表
// entryType 取值：stories|bug|tasks，entryID 为条目 ID
// 当 --no-comments 标志启用或获取失败时静默跳过
func printComments(workspaceID, entryType, entryID string) {
	if flagNoComments {
		return
	}
	comments, err := apiClient.ListComments(context.Background(), &model.ListCommentsRequest{
		WorkspaceID: workspaceID,
		EntryType:   entryType,
		EntryID:     entryID,
	})
	if err != nil || len(comments) == 0 {
		return
	}
	if useJSONOutput() {
		fmt.Fprintln(os.Stdout)
		output.PrintJSON(os.Stdout, map[string]interface{}{
			"comments": comments,
			"count":    len(comments),
		}, !flagPretty)
		return
	}
	fmt.Fprintf(os.Stdout, "\n## 评论 (%d)\n\n", len(comments))
	for i := range comments {
		comments[i].Description = htmlToMarkdown(comments[i].Description)
	}
	for _, c := range comments {
		fmt.Fprintf(os.Stdout, "**%s** (%s):\n%s\n\n", c.Author, c.Created, c.Description)
	}
}

// parseCustomFields 将 ["key1=value1","key2=value2"] 形式的切片解析为 map[string]string
// 用于支持 --custom-field key=value 可重复 flag
func parseCustomFields(fields []string) map[string]string {
	if len(fields) == 0 {
		return nil
	}
	m := make(map[string]string, len(fields))
	for _, f := range fields {
		k, v, ok := strings.Cut(f, "=")
		if !ok || k == "" {
			continue
		}
		m[k] = v
	}
	return m
}

// parseFilters 解析 --filter 参数为 map，格式无效时打印警告到 stderr
func parseFilters(fields []string) map[string]string {
	if len(fields) == 0 {
		return nil
	}
	m := make(map[string]string, len(fields))
	for _, f := range fields {
		k, v, ok := strings.Cut(f, "=")
		if !ok || k == "" {
			fmt.Fprintf(os.Stderr, "warning: skipping invalid filter %q (expected format: key=value)\n", f)
			continue
		}
		m[k] = v
	}
	return m
}

// rawHTTPClient 是 listWithFilters 用于发送原始 HTTP 请求的客户端（懒初始化）
var rawHTTPClient = &http.Client{Timeout: 30 * time.Second}

// rawGet 直接向 TAPD API 发送 GET 请求并返回 data 字段。
// 用于绕过 SDK 封装以支持 --filter 高级参数。
func rawGet(ctx context.Context, endpoint string, params map[string]string) (json.RawMessage, error) {
	baseURL := rawBaseURL
	if baseURL == "" {
		baseURL = tapd.DefaultAPIURL
	}
	baseURL = strings.TrimRight(baseURL, "/")

	reqURL, err := url.Parse(baseURL + endpoint)
	if err != nil {
		return nil, fmt.Errorf("failed to parse URL: %w", err)
	}
	if len(params) > 0 {
		q := reqURL.Query()
		for k, v := range params {
			q.Set(k, v)
		}
		reqURL.RawQuery = q.Encode()
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// 构建 Authorization 头，使用与 SDK 相同的凭据优先级
	accessToken := flagAccessToken
	apiUser := flagAPIUser
	apiPassword := flagAPIPassword
	if appConfig != nil {
		if accessToken == "" {
			accessToken = appConfig.AccessToken
		}
		if apiUser == "" {
			apiUser = appConfig.APIUser
		}
		if apiPassword == "" {
			apiPassword = appConfig.APIPassword
		}
	}
	if accessToken != "" {
		req.Header.Set("Authorization", "Bearer "+accessToken)
	} else {
		encoded := base64.StdEncoding.EncodeToString([]byte(apiUser + ":" + apiPassword))
		req.Header.Set("Authorization", "Basic "+encoded)
	}

	resp, err := rawHTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, truncateString(string(bodyBytes), 200))
	}

	var tapdResp model.TAPDResponse
	if err := json.Unmarshal(bodyBytes, &tapdResp); err != nil {
		return nil, fmt.Errorf("failed to parse response JSON: %w", err)
	}
	if tapdResp.Status != 1 {
		return nil, fmt.Errorf("TAPD API error: %s", tapdResp.Info)
	}
	return tapdResp.Data, nil
}

// parseListResponse 解析 TAPD 列表响应格式 [{"Key": {...}}, ...]
func parseListResponse[T any](data json.RawMessage, key string) ([]T, error) {
	var rawList []map[string]json.RawMessage
	if err := json.Unmarshal(data, &rawList); err != nil {
		return nil, fmt.Errorf("failed to parse list response: %w", err)
	}
	results := make([]T, 0, len(rawList))
	for i, item := range rawList {
		raw, ok := item[key]
		if !ok {
			continue
		}
		var v T
		if err := json.Unmarshal(raw, &v); err != nil {
			return nil, fmt.Errorf("failed to parse %s at index %d: %w", key, i, err)
		}
		results = append(results, v)
	}
	return results, nil
}

// truncateString 将字符串截断到指定最大字符数
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen]
}

// listWithFilters 是一个泛型辅助函数，用于 list 命令支持 --filter 参数。
// 它将 request struct 的 ToParams() 结果与 --filter 解析出的额外参数合并到新 map，
// 然后通过本地 HTTP 客户端直接发送请求，再解析 TAPD 包装响应。
// 注意：不会修改传入的 params map；filter 值会覆盖同名 params 键。
func listWithFilters[T any](ctx context.Context, _ *tapd.Client, endpoint string, params map[string]string, filters []string, wrapperKey string) ([]T, error) {
	merged := make(map[string]string, len(params)+len(filters))
	for k, v := range params {
		merged[k] = v
	}
	for k, v := range parseFilters(filters) {
		merged[k] = v
	}
	data, err := rawGet(ctx, endpoint, merged)
	if err != nil {
		return nil, err
	}
	return parseListResponse[T](data, wrapperKey)
}
