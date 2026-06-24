// Package cmd 中的 gitlab.go 实现 GitLab issue 创建与同步命令。
package cmd

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"

	"github.com/spf13/cobra"
	"github.com/studyzy/tapd-ai-cli/internal/gitlab"
	"github.com/studyzy/tapd-ai-cli/internal/output"
	"github.com/studyzy/tapd-ai-cli/internal/tapdurl"
	"github.com/studyzy/tapd-sdk-go/model"
)

var (
	flagGitLabBaseURL      string
	flagGitLabToken        string
	flagGitLabProject      string
	flagGitLabLabels       string
	flagGitLabAssigneeIDs  string
	flagGitLabDueDate      string
	flagGitLabConfidential bool
	flagGitLabIssueType    string
	flagGitLabCommentBack  bool
	flagGitLabTypes        string
)

var gitlabCmd = &cobra.Command{
	Use:   "gitlab",
	Short: "GitLab 集成",
}

var gitlabIssueCmd = &cobra.Command{
	Use:   "issue",
	Short: "GitLab Issue 管理",
}

var gitlabIssueCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "创建 GitLab Issue",
	RunE:  runGitLabIssueCreate,
}

var gitlabIssueCreateFromStoryCmd = &cobra.Command{
	Use:   "create-from-story <story_id_or_url>",
	Short: "从 TAPD 需求创建 GitLab Issue",
	Args:  cobra.ExactArgs(1),
	RunE:  runGitLabIssueCreateFromStory,
}

var gitlabIssueCreateFromBugCmd = &cobra.Command{
	Use:   "create-from-bug <bug_id_or_url>",
	Short: "从 TAPD 缺陷创建 GitLab Issue",
	Args:  cobra.ExactArgs(1),
	RunE:  runGitLabIssueCreateFromBug,
}

var gitlabIssueSyncWatchCmd = &cobra.Command{
	Use:   "sync-watch",
	Short: "监听 TAPD 变化并同步 GitLab Issue",
	RunE:  runGitLabIssueSyncWatch,
}

type gitLabOptions struct {
	baseURL string
	token   string
	project string
}

type gitLabIssueSuccess struct {
	Success   bool   `json:"success"`
	ID        int    `json:"id"`
	IID       int    `json:"iid"`
	URL       string `json:"url"`
	Project   string `json:"project"`
	ProjectID int    `json:"project_id,omitempty"`
}

type gitLabIssueSnapshot struct {
	EntityType  string
	EntityID    string
	WorkspaceID string
	Title       string
	Description string
	Fingerprint string
	Ready       bool
}

type gitLabSyncMarker struct {
	Type        string `json:"type"`
	ID          string `json:"id"`
	Project     string `json:"project"`
	IssueIID    int    `json:"issue_iid"`
	Fingerprint string `json:"fingerprint"`
}

type gitLabSyncConfig struct {
	options     gitLabOptions
	types       map[string]bool
	endpoint    string
	token       string
	workspaceID string
}

type gitLabSyncTarget struct {
	EntityType  string
	WorkspaceID string
	EntityID    string
	EventID     uint64
}

func init() {
	for _, c := range []*cobra.Command{
		gitlabIssueCreateCmd,
		gitlabIssueCreateFromStoryCmd,
		gitlabIssueCreateFromBugCmd,
		gitlabIssueSyncWatchCmd,
	} {
		c.Flags().StringVar(&flagGitLabBaseURL, "gitlab-base-url", "", "GitLab 站点地址")
		c.Flags().StringVar(&flagGitLabToken, "gitlab-token", "", "GitLab private token")
		c.Flags().StringVar(&flagGitLabProject, "project", "", "GitLab 项目 ID 或路径")
	}

	for _, c := range []*cobra.Command{
		gitlabIssueCreateCmd,
		gitlabIssueCreateFromStoryCmd,
		gitlabIssueCreateFromBugCmd,
	} {
		c.Flags().StringVar(&flagGitLabLabels, "labels", "", "GitLab labels，逗号分隔")
		c.Flags().StringVar(&flagGitLabAssigneeIDs, "assignee-ids", "", "GitLab assignee user IDs，逗号分隔")
		c.Flags().StringVar(&flagGitLabDueDate, "due-date", "", "GitLab issue 截止日期，格式 2006-01-02")
		c.Flags().BoolVar(&flagGitLabConfidential, "confidential", false, "创建 confidential issue")
		c.Flags().StringVar(&flagGitLabIssueType, "issue-type", "", "GitLab issue_type")
	}
	gitlabIssueCreateCmd.Flags().StringVar(&flagTitle, "title", "", "Issue 标题（必需）")
	gitlabIssueCreateCmd.Flags().StringVar(&flagDescription, "description", "", "Issue 描述")
	gitlabIssueCreateCmd.Flags().StringVar(&flagDescFile, "file", "", "从文件读取 Issue 描述")

	for _, c := range []*cobra.Command{gitlabIssueCreateFromStoryCmd, gitlabIssueCreateFromBugCmd} {
		c.Flags().BoolVar(&flagGitLabCommentBack, "comment-back", false, "创建后写回 TAPD 评论")
	}
	gitlabIssueSyncWatchCmd.Flags().StringVar(&flagGitLabTypes, "types", "story,bug", "同步类型，逗号分隔：story,bug")
	gitlabIssueSyncWatchCmd.Flags().StringVar(&flagWatchEndpoint, "endpoint", "", "SSE 端点 URL，覆盖配置文件中的 watch_endpoint")
	gitlabIssueSyncWatchCmd.Flags().StringVar(&flagWatchToken, "token", "", "订阅鉴权 token，覆盖配置文件中的 subscribe_token")

	gitlabIssueCmd.AddCommand(
		gitlabIssueCreateCmd,
		gitlabIssueCreateFromStoryCmd,
		gitlabIssueCreateFromBugCmd,
		gitlabIssueSyncWatchCmd,
	)
	gitlabCmd.AddCommand(gitlabIssueCmd)
	rootCmd.AddCommand(gitlabCmd)
}

func runGitLabIssueCreate(cmd *cobra.Command, args []string) error {
	opts, err := resolveGitLabOptions()
	if err != nil {
		output.PrintError(os.Stderr, "gitlab_config_error", err.Error(), "")
		os.Exit(output.ExitParamError)
		return nil
	}
	if strings.TrimSpace(flagTitle) == "" {
		output.PrintError(os.Stderr, "missing_parameter", "--title is required", "")
		os.Exit(output.ExitParamError)
		return nil
	}
	description, err := readGitLabDescription()
	if err != nil {
		output.PrintError(os.Stderr, "read_description_failed", err.Error(), "")
		os.Exit(output.ExitParamError)
		return nil
	}
	req, err := buildGitLabCreateIssueRequest(flagTitle, description)
	if err != nil {
		output.PrintError(os.Stderr, "invalid_parameter", err.Error(), "")
		os.Exit(output.ExitParamError)
		return nil
	}
	issue, err := gitlab.NewClient(opts.baseURL, opts.token).CreateIssue(cmdContext(cmd), opts.project, req)
	if err != nil {
		output.PrintError(os.Stderr, "gitlab_api_error", err.Error(), "")
		os.Exit(output.ExitAPIError)
		return nil
	}
	return printGitLabIssueSuccess(issue, opts.project)
}

func resolveGitLabOptions() (gitLabOptions, error) {
	opts := gitLabOptions{
		baseURL: strings.TrimSpace(flagGitLabBaseURL),
		token:   strings.TrimSpace(flagGitLabToken),
		project: strings.TrimSpace(flagGitLabProject),
	}
	if appConfig != nil {
		if opts.baseURL == "" {
			opts.baseURL = appConfig.GitLabBaseURL
		}
		if opts.token == "" {
			opts.token = appConfig.GitLabToken
		}
		if opts.project == "" {
			opts.project = appConfig.GitLabProject
		}
	}
	if opts.baseURL == "" {
		opts.baseURL = "https://gitlab.com"
	}
	if opts.token == "" {
		return opts, fmt.Errorf("GitLab token is required")
	}
	if opts.project == "" {
		return opts, fmt.Errorf("GitLab project is required")
	}
	return opts, nil
}

func readGitLabDescription() (string, error) {
	if flagDescription != "" {
		return flagDescription, nil
	}
	if flagDescFile != "" {
		data, err := os.ReadFile(flagDescFile)
		if err != nil {
			return "", err
		}
		return string(data), nil
	}
	stat, err := os.Stdin.Stat()
	if err == nil && (stat.Mode()&os.ModeCharDevice) == 0 {
		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			return "", err
		}
		return string(data), nil
	}
	return "", nil
}

func buildGitLabCreateIssueRequest(title, description string) (gitlab.CreateIssueRequest, error) {
	assigneeIDs, err := parseGitLabIntCSV(flagGitLabAssigneeIDs)
	if err != nil {
		return gitlab.CreateIssueRequest{}, err
	}
	return gitlab.CreateIssueRequest{
		Title:        strings.TrimSpace(title),
		Description:  description,
		Labels:       splitGitLabCSV(flagGitLabLabels),
		AssigneeIDs:  assigneeIDs,
		DueDate:      strings.TrimSpace(flagGitLabDueDate),
		Confidential: flagGitLabConfidential,
		IssueType:    strings.TrimSpace(flagGitLabIssueType),
	}, nil
}

func parseGitLabIntCSV(raw string) ([]int, error) {
	parts := splitGitLabCSV(raw)
	if len(parts) == 0 {
		return nil, nil
	}
	out := make([]int, 0, len(parts))
	for _, part := range parts {
		v, err := strconv.Atoi(part)
		if err != nil {
			return nil, fmt.Errorf("invalid assignee id %q", part)
		}
		out = append(out, v)
	}
	return out, nil
}

func splitGitLabCSV(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if v := strings.TrimSpace(part); v != "" {
			out = append(out, v)
		}
	}
	return out
}

func printGitLabIssueSuccess(issue *gitlab.Issue, project string) error {
	resp := gitLabIssueSuccess{
		Success:   true,
		ID:        issue.ID,
		IID:       issue.IID,
		URL:       issue.WebURL,
		Project:   project,
		ProjectID: issue.ProjectID,
	}
	return output.PrintJSON(os.Stdout, resp, !flagPretty)
}

func buildGitLabIssueFromStory(story *model.Story) gitLabIssueSnapshot {
	md := normalizedTAPDMarkdown(firstNonEmpty(story.MarkdownDescription, story.Description))
	title := strings.TrimSpace(story.Name)
	description := renderTAPDSnapshot("story", "TAPD 需求", story.URL, []string{
		"ID: " + story.ID,
		"Status: " + story.Status,
		"Priority: " + firstNonEmpty(story.PriorityLabel, story.Priority),
		"Owner: " + story.Owner,
		"Developer: " + story.Developer,
		"Iteration: " + story.IterationID,
	}, md)
	ready := isGitLabSnapshotReady(title, md)
	return gitLabIssueSnapshot{
		EntityType:  "story",
		EntityID:    story.ID,
		WorkspaceID: story.WorkspaceID,
		Title:       "[TAPD Story] " + title,
		Description: description,
		Fingerprint: fingerprintGitLabSnapshot("story", story.ID, title, md, story.Status,
			firstNonEmpty(story.PriorityLabel, story.Priority), story.Owner, story.Developer),
		Ready: ready,
	}
}

func buildGitLabIssueFromBug(bug *model.Bug) gitLabIssueSnapshot {
	md := normalizedTAPDMarkdown(bug.Description)
	title := strings.TrimSpace(bug.Title)
	description := renderTAPDSnapshot("bug", "TAPD 缺陷", bug.URL, []string{
		"ID: " + bug.ID,
		"Status: " + bug.Status,
		"Priority: " + firstNonEmpty(bug.PriorityLabel, bug.Priority),
		"Severity: " + bug.Severity,
		"Current owner: " + bug.CurrentOwner,
		"Module: " + bug.Module,
		"Iteration: " + bug.IterationID,
	}, md)
	ready := isGitLabSnapshotReady(title, md)
	return gitLabIssueSnapshot{
		EntityType:  "bug",
		EntityID:    bug.ID,
		WorkspaceID: bug.WorkspaceID,
		Title:       "[TAPD Bug] " + title,
		Description: description,
		Fingerprint: fingerprintGitLabSnapshot("bug", bug.ID, title, md, bug.Status,
			firstNonEmpty(bug.PriorityLabel, bug.Priority), bug.CurrentOwner, bug.Severity, bug.Module),
		Ready: ready,
	}
}

func normalizedTAPDMarkdown(raw string) string {
	md := htmlToMarkdown(raw)
	md = strings.ReplaceAll(md, "\u00a0", " ")
	return strings.TrimSpace(md)
}

func renderTAPDSnapshot(entityType, kind, tapdURL string, fields []string, md string) string {
	var b strings.Builder
	b.WriteString("## ")
	b.WriteString(kind)
	b.WriteString("\n\n")
	if strings.TrimSpace(tapdURL) != "" {
		b.WriteString("- TAPD: ")
		b.WriteString(strings.TrimSpace(tapdURL))
		b.WriteString("\n")
	}
	for _, field := range fields {
		// 字段格式 "Label: value":拆冒号后判断 value 是否为空,空则跳过整行。
		idx := strings.Index(field, ":")
		if idx >= 0 && strings.TrimSpace(field[idx+1:]) == "" {
			continue
		}
		b.WriteString("- ")
		b.WriteString(field)
		b.WriteString("\n")
	}
	structured := renderStructuredTAPDDescription(entityType, md)
	if structured != "" {
		b.WriteString("\n")
		b.WriteString(structured)
	}
	return b.String()
}

type structuredSection struct {
	key      string
	title    string
	keywords []string
}

func renderStructuredTAPDDescription(entityType string, md string) string {
	sections := structuredSectionsFor(entityType)
	if len(sections) == 0 {
		sections = structuredSectionsFor("story")
	}
	grouped := make(map[string][]string, len(sections))
	paragraphs := splitTAPDParagraphs(md)
	for _, paragraph := range paragraphs {
		key := classifyTAPDParagraph(entityType, paragraph)
		if key == "" {
			key = "original"
			grouped[key] = append(grouped[key], paragraph)
			continue
		}
		grouped[key] = append(grouped[key], trimTAPDParagraphLabel(paragraph))
	}

	var b strings.Builder
	for _, section := range sections {
		items := grouped[section.key]
		if len(items) == 0 {
			continue
		}
		if b.Len() > 0 {
			b.WriteString("\n\n")
		}
		b.WriteString("## ")
		b.WriteString(section.title)
		b.WriteString("\n\n")
		b.WriteString(strings.Join(items, "\n\n"))
	}
	return strings.TrimSpace(b.String())
}

func structuredSectionsFor(entityType string) []structuredSection {
	switch entityType {
	case "bug":
		return []structuredSection{
			{key: "summary", title: "问题概述", keywords: []string{"问题", "标题", "现象", "概述"}},
			{key: "condition", title: "复现条件", keywords: []string{"前置条件", "环境", "版本", "账号", "数据", "机型"}},
			{key: "steps", title: "复现步骤", keywords: []string{"复现", "步骤", "流程", "操作"}},
			{key: "expected", title: "预期结果", keywords: []string{"预期", "应该", "期望"}},
			{key: "actual", title: "实际结果", keywords: []string{"实际", "现状", "结果", "报错"}},
			{key: "impact", title: "影响范围", keywords: []string{"影响", "范围", "严重性", "频率", "用户"}},
			{key: "clue", title: "排查线索", keywords: []string{"日志", "接口", "curl", "trace", "截图", "线索", "分析"}},
			{key: "original", title: "原始补充"},
		}
	default:
		return []structuredSection{
			{key: "background", title: "背景 / 现状", keywords: []string{"背景", "现状", "问题", "为什么", "痛点", "诉求", "上下文"}},
			{key: "goal", title: "目标 / 预期", keywords: []string{"目标", "预期", "收益", "价值", "指标", "成功标准"}},
			{key: "scope", title: "需求范围 / 方案", keywords: []string{"范围", "方案", "怎么做", "功能", "流程", "交互", "规则", "逻辑"}},
			{key: "acceptance", title: "验收标准 / 测试要点", keywords: []string{"验收", "测试", "验证", "case", "用例", "准入", "完成标准"}},
			{key: "risk", title: "风险 / 依赖 / 待确认", keywords: []string{"风险", "依赖", "待确认", "限制", "注意事项"}},
			{key: "original", title: "原始补充"},
		}
	}
}

func classifyTAPDParagraph(entityType string, paragraph string) string {
	head := paragraphClassifyText(paragraph)
	if head == "" {
		return ""
	}
	for _, section := range structuredSectionsFor(entityType) {
		if section.key == "original" {
			continue
		}
		for _, keyword := range section.keywords {
			if strings.Contains(head, strings.ToLower(keyword)) {
				return section.key
			}
		}
	}
	return ""
}

func paragraphClassifyText(paragraph string) string {
	paragraph = strings.TrimSpace(paragraph)
	isHeading := strings.HasPrefix(paragraph, "#")
	paragraph = strings.TrimLeft(paragraph, "#-*> \t")
	if idx := strings.IndexAny(paragraph, "：:\n"); idx >= 0 {
		paragraph = paragraph[:idx]
	} else if !isHeading {
		lower := strings.ToLower(paragraph)
		for _, prefix := range []string{"日志", "trace", "curl", "截图", "接口"} {
			if strings.HasPrefix(lower, strings.ToLower(prefix)) {
				return lower
			}
		}
		return ""
	}
	return strings.ToLower(strings.TrimSpace(paragraph))
}

func splitTAPDParagraphs(md string) []string {
	md = strings.TrimSpace(md)
	if md == "" {
		return nil
	}
	lines := strings.Split(md, "\n")
	var paragraphs []string
	var current strings.Builder
	flush := func() {
		text := strings.TrimSpace(current.String())
		if text != "" {
			paragraphs = append(paragraphs, text)
		}
		current.Reset()
	}
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			if currentContainsOnlyHeading(current.String()) {
				continue
			}
			flush()
			continue
		}
		if strings.HasPrefix(trimmed, "#") {
			flush()
			current.WriteString(trimmed)
			current.WriteString("\n")
			continue
		}
		if looksLikeTAPDLabel(trimmed) {
			flush()
		}
		if current.Len() > 0 {
			current.WriteString("\n")
		}
		current.WriteString(trimmed)
	}
	flush()
	return paragraphs
}

func currentContainsOnlyHeading(text string) bool {
	text = strings.TrimSpace(text)
	return strings.HasPrefix(text, "#") && !strings.Contains(text, "\n")
}

func looksLikeTAPDLabel(line string) bool {
	line = strings.TrimLeft(line, "-*> \t")
	idx := strings.IndexAny(line, "：:")
	if idx <= 0 || idx > 24 {
		return false
	}
	prefix := strings.TrimSpace(line[:idx])
	if prefix == "" {
		return false
	}
	for _, section := range structuredSectionsFor("story") {
		for _, keyword := range section.keywords {
			if strings.Contains(prefix, keyword) {
				return true
			}
		}
	}
	for _, section := range structuredSectionsFor("bug") {
		for _, keyword := range section.keywords {
			if strings.Contains(prefix, keyword) {
				return true
			}
		}
	}
	return false
}

func trimTAPDParagraphLabel(paragraph string) string {
	paragraph = strings.TrimSpace(paragraph)
	lines := strings.SplitN(paragraph, "\n", 2)
	first := strings.TrimSpace(lines[0])
	if strings.HasPrefix(first, "#") {
		first = strings.TrimSpace(strings.TrimLeft(first, "# "))
		if len(lines) == 1 {
			return first
		}
		rest := strings.TrimSpace(lines[1])
		if rest == "" {
			return first
		}
		return rest
	}
	cleaned := strings.TrimLeft(first, "-*> \t")
	if key, value, ok := splitTAPDLabel(cleaned); ok && len([]rune(key)) <= 24 {
		if value != "" {
			if len(lines) == 1 {
				return value
			}
			return strings.TrimSpace(value + "\n" + lines[1])
		}
	}
	return paragraph
}

func splitTAPDLabel(text string) (string, string, bool) {
	for idx, r := range text {
		if r != '：' && r != ':' {
			continue
		}
		key := strings.TrimSpace(text[:idx])
		value := strings.TrimSpace(text[idx+len(string(r)):])
		return key, value, key != ""
	}
	return "", "", false
}

func isGitLabSnapshotReady(title, markdownDescription string) bool {
	title = strings.TrimSpace(title)
	md := strings.TrimSpace(markdownDescription)
	if title == "" || md == "" {
		return false
	}
	emptyValues := map[string]bool{
		"<p><br></p>": true,
		"<br>":        true,
		"<br />":      true,
	}
	return !emptyValues[strings.ToLower(md)]
}

func fingerprintGitLabSnapshot(parts ...string) string {
	normalized := make([]string, 0, len(parts))
	for _, part := range parts {
		normalized = append(normalized, strings.TrimSpace(part))
	}
	sum := sha256.Sum256([]byte(strings.Join(normalized, "\n")))
	return hex.EncodeToString(sum[:])
}

func renderGitLabSyncComment(issueURL string, marker gitLabSyncMarker) string {
	data, _ := json.Marshal(marker)
	return fmt.Sprintf("已同步 GitLab Issue: %s\n\n<!-- tapd-gitlab-sync %s -->", issueURL, data)
}

func parseGitLabSyncMarker(comment, entityType, entityID, project string) (gitLabSyncMarker, bool) {
	const prefix = "<!-- tapd-gitlab-sync "
	start := strings.Index(comment, prefix)
	if start < 0 {
		return gitLabSyncMarker{}, false
	}
	start += len(prefix)
	end := strings.Index(comment[start:], " -->")
	if end < 0 {
		return gitLabSyncMarker{}, false
	}
	var marker gitLabSyncMarker
	if err := json.Unmarshal([]byte(comment[start:start+end]), &marker); err != nil {
		return gitLabSyncMarker{}, false
	}
	if marker.Type != entityType || marker.ID != entityID || marker.Project != project {
		return gitLabSyncMarker{}, false
	}
	return marker, true
}

func runGitLabIssueCreateFromStory(cmd *cobra.Command, args []string) error {
	opts, err := resolveGitLabOptions()
	if err != nil {
		output.PrintError(os.Stderr, "gitlab_config_error", err.Error(), "")
		os.Exit(output.ExitParamError)
		return nil
	}
	workspaceID, storyID, err := resolveGitLabTAPDRef(args[0], "story")
	if err != nil {
		output.PrintError(os.Stderr, "invalid_parameter", err.Error(), "")
		os.Exit(output.ExitParamError)
		return nil
	}
	story, err := apiClient.GetStory(cmdContext(cmd), workspaceID, storyID)
	if err != nil {
		output.PrintError(os.Stderr, "api_error", err.Error(), "")
		os.Exit(output.ExitAPIError)
		return nil
	}
	if story.WorkspaceID == "" {
		story.WorkspaceID = workspaceID
	}
	snapshot := buildGitLabIssueFromStory(story)
	return createGitLabIssueFromSnapshot(cmdContext(cmd), opts, snapshot, flagGitLabCommentBack)
}

func runGitLabIssueCreateFromBug(cmd *cobra.Command, args []string) error {
	opts, err := resolveGitLabOptions()
	if err != nil {
		output.PrintError(os.Stderr, "gitlab_config_error", err.Error(), "")
		os.Exit(output.ExitParamError)
		return nil
	}
	workspaceID, bugID, err := resolveGitLabTAPDRef(args[0], "bug")
	if err != nil {
		output.PrintError(os.Stderr, "invalid_parameter", err.Error(), "")
		os.Exit(output.ExitParamError)
		return nil
	}
	bug, err := apiClient.GetBug(cmdContext(cmd), workspaceID, bugID)
	if err != nil {
		output.PrintError(os.Stderr, "api_error", err.Error(), "")
		os.Exit(output.ExitAPIError)
		return nil
	}
	if bug.WorkspaceID == "" {
		bug.WorkspaceID = workspaceID
	}
	snapshot := buildGitLabIssueFromBug(bug)
	return createGitLabIssueFromSnapshot(cmdContext(cmd), opts, snapshot, flagGitLabCommentBack)
}

func runGitLabIssueSyncWatch(cmd *cobra.Command, args []string) error {
	opts, err := resolveGitLabSyncOptions()
	if err != nil {
		output.PrintError(os.Stderr, "gitlab_config_error", err.Error(), "")
		os.Exit(output.ExitParamError)
		return nil
	}
	endpoint, token := resolveWatchConfig()
	if endpoint == "" {
		output.PrintError(os.Stderr, "watch_endpoint_missing", "watch endpoint not configured", "")
		os.Exit(output.ExitParamError)
		return nil
	}
	cfg := gitLabSyncConfig{
		options:     opts,
		types:       parseGitLabSyncTypes(flagGitLabTypes),
		endpoint:    endpoint,
		token:       token,
		workspaceID: flagWorkspaceID,
	}
	watchStateRef = newWatchState()
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	streamGitLabIssueSync(ctx, cfg)
	return nil
}

func resolveGitLabTAPDRef(raw, wantType string) (workspaceID, entityID string, err error) {
	raw = strings.TrimSpace(raw)
	if strings.HasPrefix(raw, "http://") || strings.HasPrefix(raw, "https://") {
		parsed, err := tapdurl.Parse(raw)
		if err != nil {
			return "", "", err
		}
		if parsed.EntityType != wantType {
			return "", "", fmt.Errorf("expected %s URL, got %s", wantType, parsed.EntityType)
		}
		return parsed.WorkspaceID, parsed.EntityID, nil
	}
	return flagWorkspaceID, expandShortID(raw, flagWorkspaceID), nil
}

func createGitLabIssueFromSnapshot(ctx context.Context, opts gitLabOptions, snapshot gitLabIssueSnapshot, commentBack bool) error {
	if !snapshot.Ready {
		output.PrintError(os.Stderr, "not_ready_for_sync", "TAPD title and description are required", "")
		os.Exit(output.ExitParamError)
		return nil
	}
	req, err := buildGitLabCreateIssueRequest(snapshot.Title, snapshot.Description)
	if err != nil {
		output.PrintError(os.Stderr, "invalid_parameter", err.Error(), "")
		os.Exit(output.ExitParamError)
		return nil
	}
	issue, err := gitlab.NewClient(opts.baseURL, opts.token).CreateIssue(ctx, opts.project, req)
	if err != nil {
		output.PrintError(os.Stderr, "gitlab_api_error", err.Error(), "")
		os.Exit(output.ExitAPIError)
		return nil
	}
	if err := printGitLabIssueSuccess(issue, opts.project); err != nil {
		return err
	}
	if commentBack {
		marker := gitLabSyncMarker{
			Type:        snapshot.EntityType,
			ID:          snapshot.EntityID,
			Project:     opts.project,
			IssueIID:    issue.IID,
			Fingerprint: snapshot.Fingerprint,
		}
		if err := addGitLabSyncComment(ctx, snapshot, issue.WebURL, marker); err != nil {
			output.PrintError(os.Stderr, "comment_back_failed", err.Error(), "")
			os.Exit(output.ExitAPIError)
			return nil
		}
	}
	return nil
}

func addGitLabSyncComment(ctx context.Context, snapshot gitLabIssueSnapshot, issueURL string, marker gitLabSyncMarker) error {
	entryType := "stories"
	if snapshot.EntityType == "bug" {
		entryType = "bug"
	}
	workspaceID := firstNonEmpty(snapshot.WorkspaceID, flagWorkspaceID)
	_, err := apiClient.AddComment(ctx, &model.AddCommentRequest{
		WorkspaceID: workspaceID,
		EntryType:   entryType,
		EntryID:     snapshot.EntityID,
		Description: markdownToHTML(renderGitLabSyncComment(issueURL, marker)),
		Author:      ensureNick(),
	})
	return err
}
