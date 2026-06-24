// Package mcp 中的 tools_gitlab.go 注册 GitLab Issue 相关 MCP tools。
package mcp

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"unicode"

	htmltomarkdown "github.com/JohannesKaufmann/html-to-markdown/v2"
	"github.com/studyzy/tapd-ai-cli/internal/config"
	"github.com/studyzy/tapd-ai-cli/internal/gitlab"
	"github.com/studyzy/tapd-ai-cli/internal/tapdurl"
	"github.com/studyzy/tapd-sdk-go/model"
)

type gitLabToolOptions struct {
	BaseURL string
	Token   string
	Project string
}

type gitLabIssueToolResponse struct {
	Success   bool   `json:"success"`
	ID        int    `json:"id"`
	IID       int    `json:"iid"`
	URL       string `json:"url"`
	Project   string `json:"project"`
	ProjectID int    `json:"project_id,omitempty"`
	Warning   string `json:"warning,omitempty"`
}

type gitLabToolSnapshot struct {
	EntityType  string
	EntityID    string
	WorkspaceID string
	Title       string
	Description string
	Fingerprint string
	Ready       bool
}

type gitLabToolSyncMarker struct {
	Type        string `json:"type"`
	ID          string `json:"id"`
	Project     string `json:"project"`
	IssueIID    int    `json:"issue_iid"`
	Fingerprint string `json:"fingerprint"`
}

func toolGitLabIssueCreate(s *Server) *Tool {
	return &Tool{
		Name:        "tapd_gitlab_issue_create",
		Description: "Create a GitLab issue. Uses gitlab_base_url/gitlab_token/gitlab_project config or GITLAB_* env vars.",
		AllowNoTAPD: true,
		InputSchema: schema(`{
			"type":"object",
			"properties":{
				"gitlab_base_url":{"type":"string"},
				"gitlab_token":{"type":"string"},
				"project":{"type":"string","description":"GitLab project ID or path, e.g. go-vas/vas"},
				"title":{"type":"string"},
				"description":{"type":"string"},
				"labels":{"type":"string","description":"comma-separated labels"},
				"assignee_ids":{"type":"string","description":"comma-separated GitLab numeric user IDs"},
				"due_date":{"type":"string","description":"YYYY-MM-DD"},
				"confidential":{"type":"boolean"},
				"issue_type":{"type":"string"}
			},
			"required":["title"],
			"additionalProperties":false
		}`),
		Handler: func(ctx context.Context, raw json.RawMessage) (interface{}, error) {
			args, err := parseArgs(raw)
			if err != nil {
				return nil, err
			}
			title, err := requireString(args, "title")
			if err != nil {
				return nil, err
			}
			opts, err := resolveGitLabToolOptions(args)
			if err != nil {
				return nil, err
			}
			req, err := gitLabToolCreateIssueRequest(args, title, optString(args, "description"))
			if err != nil {
				return nil, err
			}
			issue, err := gitlab.NewClient(opts.BaseURL, opts.Token).CreateIssue(ctx, opts.Project, req)
			if err != nil {
				return nil, err
			}
			return gitLabToolResponse(issue, opts.Project), nil
		},
	}
}

func toolGitLabIssueCreateFromStory(s *Server, ws func(string) string) *Tool {
	return &Tool{
		Name:        "tapd_gitlab_issue_create_from_story",
		Description: "Create a GitLab issue from a TAPD story. Can optionally comment the GitLab issue link back to TAPD.",
		InputSchema: schema(`{
			"type":"object",
			"properties":{
				"id":{"type":"string","description":"TAPD story ID or URL"},
				"workspace_id":{"type":"string"},
				"gitlab_base_url":{"type":"string"},
				"gitlab_token":{"type":"string"},
				"project":{"type":"string"},
				"labels":{"type":"string"},
				"assignee_ids":{"type":"string"},
				"due_date":{"type":"string"},
				"confidential":{"type":"boolean"},
				"issue_type":{"type":"string"},
				"comment_back":{"type":"boolean"}
			},
			"required":["id"],
			"additionalProperties":false
		}`),
		Handler: func(ctx context.Context, raw json.RawMessage) (interface{}, error) {
			args, err := parseArgs(raw)
			if err != nil {
				return nil, err
			}
			rawID, err := requireString(args, "id")
			if err != nil {
				return nil, err
			}
			workspaceID, storyID, err := resolveMCPGitLabTAPDRef(rawID, "story", ws(optString(args, "workspace_id")))
			if err != nil {
				return nil, err
			}
			story, err := s.client.GetStory(ctx, workspaceID, storyID)
			if err != nil {
				return nil, err
			}
			if story.WorkspaceID == "" {
				story.WorkspaceID = workspaceID
			}
			return createGitLabIssueFromMCP(ctx, s, args, buildMCPGitLabStorySnapshot(story))
		},
	}
}

func toolGitLabIssueCreateFromBug(s *Server, ws func(string) string) *Tool {
	return &Tool{
		Name:        "tapd_gitlab_issue_create_from_bug",
		Description: "Create a GitLab issue from a TAPD bug. Can optionally comment the GitLab issue link back to TAPD.",
		InputSchema: schema(`{
			"type":"object",
			"properties":{
				"id":{"type":"string","description":"TAPD bug ID or URL"},
				"workspace_id":{"type":"string"},
				"gitlab_base_url":{"type":"string"},
				"gitlab_token":{"type":"string"},
				"project":{"type":"string"},
				"labels":{"type":"string"},
				"assignee_ids":{"type":"string"},
				"due_date":{"type":"string"},
				"confidential":{"type":"boolean"},
				"issue_type":{"type":"string"},
				"comment_back":{"type":"boolean"}
			},
			"required":["id"],
			"additionalProperties":false
		}`),
		Handler: func(ctx context.Context, raw json.RawMessage) (interface{}, error) {
			args, err := parseArgs(raw)
			if err != nil {
				return nil, err
			}
			rawID, err := requireString(args, "id")
			if err != nil {
				return nil, err
			}
			workspaceID, bugID, err := resolveMCPGitLabTAPDRef(rawID, "bug", ws(optString(args, "workspace_id")))
			if err != nil {
				return nil, err
			}
			bug, err := s.client.GetBug(ctx, workspaceID, bugID)
			if err != nil {
				return nil, err
			}
			if bug.WorkspaceID == "" {
				bug.WorkspaceID = workspaceID
			}
			return createGitLabIssueFromMCP(ctx, s, args, buildMCPGitLabBugSnapshot(bug))
		},
	}
}

func createGitLabIssueFromMCP(ctx context.Context, s *Server, args map[string]interface{}, snapshot gitLabToolSnapshot) (interface{}, error) {
	if !snapshot.Ready {
		return nil, fmt.Errorf("TAPD title and description are required before creating GitLab issue")
	}
	opts, err := resolveGitLabToolOptions(args)
	if err != nil {
		return nil, err
	}
	req, err := gitLabToolCreateIssueRequest(args, snapshot.Title, snapshot.Description)
	if err != nil {
		return nil, err
	}
	issue, err := gitlab.NewClient(opts.BaseURL, opts.Token).CreateIssue(ctx, opts.Project, req)
	if err != nil {
		return nil, err
	}
	resp := gitLabToolResponse(issue, opts.Project)
	if optBool(args, "comment_back") {
		marker := gitLabToolSyncMarker{
			Type:        snapshot.EntityType,
			ID:          snapshot.EntityID,
			Project:     opts.Project,
			IssueIID:    issue.IID,
			Fingerprint: snapshot.Fingerprint,
		}
		// comment_back 失败不能让整个工具调用失败:GitLab issue 已经创建,
		// 必须把 IID/URL 透传给调用方,否则 LLM 看不到结果会重试导致重复创建。
		if err := addMCPGitLabSyncComment(ctx, s, snapshot, issue.WebURL, marker); err != nil {
			resp.Warning = "comment_back_failed: " + err.Error()
		}
	}
	return resp, nil
}

func resolveGitLabToolOptions(args map[string]interface{}) (gitLabToolOptions, error) {
	cfg, cfgErr := config.LoadConfig()
	opts := gitLabToolOptions{
		BaseURL: strings.TrimSpace(optString(args, "gitlab_base_url")),
		Token:   strings.TrimSpace(optString(args, "gitlab_token")),
		Project: strings.TrimSpace(optString(args, "project")),
	}
	if cfg != nil {
		if opts.BaseURL == "" {
			opts.BaseURL = cfg.GitLabBaseURL
		}
		if opts.Token == "" {
			opts.Token = cfg.GitLabToken
		}
		if opts.Project == "" {
			opts.Project = cfg.GitLabProject
		}
	}
	if opts.BaseURL == "" {
		opts.BaseURL = "https://gitlab.com"
	}
	if opts.Token == "" {
		if cfgErr != nil {
			return opts, fmt.Errorf("GitLab token is required (config load failed: %w)", cfgErr)
		}
		return opts, fmt.Errorf("GitLab token is required")
	}
	if opts.Project == "" {
		if cfgErr != nil {
			return opts, fmt.Errorf("GitLab project is required (config load failed: %w)", cfgErr)
		}
		return opts, fmt.Errorf("GitLab project is required")
	}
	return opts, nil
}

func gitLabToolCreateIssueRequest(args map[string]interface{}, title, description string) (gitlab.CreateIssueRequest, error) {
	assigneeIDs, err := parseGitLabToolIntCSV(optString(args, "assignee_ids"))
	if err != nil {
		return gitlab.CreateIssueRequest{}, err
	}
	return gitlab.CreateIssueRequest{
		Title:        strings.TrimSpace(title),
		Description:  description,
		Labels:       splitGitLabToolCSV(optString(args, "labels")),
		AssigneeIDs:  assigneeIDs,
		DueDate:      strings.TrimSpace(optString(args, "due_date")),
		Confidential: optBool(args, "confidential"),
		IssueType:    strings.TrimSpace(optString(args, "issue_type")),
	}, nil
}

func gitLabToolResponse(issue *gitlab.Issue, project string) gitLabIssueToolResponse {
	return gitLabIssueToolResponse{
		Success:   true,
		ID:        issue.ID,
		IID:       issue.IID,
		URL:       issue.WebURL,
		Project:   project,
		ProjectID: issue.ProjectID,
	}
}

func resolveMCPGitLabTAPDRef(raw, wantType, defaultWorkspace string) (workspaceID, entityID string, err error) {
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
	if defaultWorkspace == "" {
		return "", "", fmt.Errorf("workspace_id required (no default configured)")
	}
	return defaultWorkspace, expandMCPShortID(raw, defaultWorkspace), nil
}

// expandMCPShortID 镜像 cmd.expandShortID:对纯数字、长度 ≤ 9 的短 ID 左补零到 9 位,前缀 "11" + workspaceID。
// 与 cmd 包同名函数同步维护(MCP 不能反向依赖 cmd 包)。
func expandMCPShortID(id, workspaceID string) string {
	if id == "" || workspaceID == "" {
		return id
	}
	for _, c := range id {
		if !unicode.IsDigit(c) {
			return id
		}
	}
	if len(id) > 9 {
		return id
	}
	return "11" + workspaceID + fmt.Sprintf("%09s", id)
}

func buildMCPGitLabStorySnapshot(story *model.Story) gitLabToolSnapshot {
	md := normalizedGitLabToolMarkdown(firstNonEmptyMCP(story.MarkdownDescription, story.Description))
	title := strings.TrimSpace(story.Name)
	description := renderGitLabToolSnapshot("story", "TAPD 需求", story.URL, []string{
		"ID: " + story.ID,
		"Status: " + story.Status,
		"Priority: " + firstNonEmptyMCP(story.PriorityLabel, story.Priority),
		"Owner: " + story.Owner,
		"Developer: " + story.Developer,
		"Iteration: " + story.IterationID,
	}, md)
	return gitLabToolSnapshot{
		EntityType:  "story",
		EntityID:    story.ID,
		WorkspaceID: story.WorkspaceID,
		Title:       "[TAPD Story] " + title,
		Description: description,
		Fingerprint: fingerprintGitLabToolSnapshot("story", story.ID, title, md, story.Status,
			firstNonEmptyMCP(story.PriorityLabel, story.Priority), story.Owner, story.Developer),
		Ready: isGitLabToolSnapshotReady(title, md),
	}
}

func buildMCPGitLabBugSnapshot(bug *model.Bug) gitLabToolSnapshot {
	md := normalizedGitLabToolMarkdown(bug.Description)
	title := strings.TrimSpace(bug.Title)
	description := renderGitLabToolSnapshot("bug", "TAPD 缺陷", bug.URL, []string{
		"ID: " + bug.ID,
		"Status: " + bug.Status,
		"Priority: " + firstNonEmptyMCP(bug.PriorityLabel, bug.Priority),
		"Severity: " + bug.Severity,
		"Current owner: " + bug.CurrentOwner,
		"Module: " + bug.Module,
		"Iteration: " + bug.IterationID,
	}, md)
	return gitLabToolSnapshot{
		EntityType:  "bug",
		EntityID:    bug.ID,
		WorkspaceID: bug.WorkspaceID,
		Title:       "[TAPD Bug] " + title,
		Description: description,
		Fingerprint: fingerprintGitLabToolSnapshot("bug", bug.ID, title, md, bug.Status,
			firstNonEmptyMCP(bug.PriorityLabel, bug.Priority), bug.CurrentOwner, bug.Severity, bug.Module),
		Ready: isGitLabToolSnapshotReady(title, md),
	}
}

func addMCPGitLabSyncComment(ctx context.Context, s *Server, snapshot gitLabToolSnapshot, issueURL string, marker gitLabToolSyncMarker) error {
	entryType := "stories"
	if snapshot.EntityType == "bug" {
		entryType = "bug"
	}
	author := s.client.GetNick()
	if author == "" {
		_ = s.client.FetchNick(ctx)
		author = s.client.GetNick()
	}
	_, err := s.client.AddComment(ctx, &model.AddCommentRequest{
		WorkspaceID: snapshot.WorkspaceID,
		EntryType:   entryType,
		EntryID:     snapshot.EntityID,
		Description: markdownToHTML(renderMCPGitLabSyncComment(issueURL, marker)),
		Author:      author,
	})
	return err
}

func renderMCPGitLabSyncComment(issueURL string, marker gitLabToolSyncMarker) string {
	data, _ := json.Marshal(marker)
	return fmt.Sprintf("已同步 GitLab Issue: %s\n\n<!-- tapd-gitlab-sync %s -->", issueURL, data)
}

func normalizedGitLabToolMarkdown(raw string) string {
	md, err := htmltomarkdown.ConvertString(raw)
	if err != nil {
		md = raw
	}
	md = strings.ReplaceAll(md, "\u00a0", " ")
	return strings.TrimSpace(md)
}

func renderGitLabToolSnapshot(entityType, kind, tapdURL string, fields []string, md string) string {
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
	structured := renderStructuredGitLabToolDescription(entityType, md)
	if structured != "" {
		b.WriteString("\n")
		b.WriteString(structured)
	}
	return b.String()
}

type gitLabToolStructuredSection struct {
	key      string
	title    string
	keywords []string
}

func renderStructuredGitLabToolDescription(entityType string, md string) string {
	sections := gitLabToolStructuredSectionsFor(entityType)
	grouped := make(map[string][]string, len(sections))
	paragraphs := splitGitLabToolParagraphs(md)
	for _, paragraph := range paragraphs {
		key := classifyGitLabToolParagraph(entityType, paragraph)
		if key == "" {
			key = "original"
			grouped[key] = append(grouped[key], paragraph)
			continue
		}
		grouped[key] = append(grouped[key], trimGitLabToolParagraphLabel(paragraph))
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

func gitLabToolStructuredSectionsFor(entityType string) []gitLabToolStructuredSection {
	switch entityType {
	case "bug":
		return []gitLabToolStructuredSection{
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
		return []gitLabToolStructuredSection{
			{key: "background", title: "背景 / 现状", keywords: []string{"背景", "现状", "问题", "为什么", "痛点", "诉求", "上下文"}},
			{key: "goal", title: "目标 / 预期", keywords: []string{"目标", "预期", "收益", "价值", "指标", "成功标准"}},
			{key: "scope", title: "需求范围 / 方案", keywords: []string{"范围", "方案", "怎么做", "功能", "流程", "交互", "规则", "逻辑"}},
			{key: "acceptance", title: "验收标准 / 测试要点", keywords: []string{"验收", "测试", "验证", "case", "用例", "准入", "完成标准"}},
			{key: "risk", title: "风险 / 依赖 / 待确认", keywords: []string{"风险", "依赖", "待确认", "限制", "注意事项"}},
			{key: "original", title: "原始补充"},
		}
	}
}

func classifyGitLabToolParagraph(entityType string, paragraph string) string {
	head := gitLabToolParagraphClassifyText(paragraph)
	if head == "" {
		return ""
	}
	for _, section := range gitLabToolStructuredSectionsFor(entityType) {
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

func gitLabToolParagraphClassifyText(paragraph string) string {
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

func splitGitLabToolParagraphs(md string) []string {
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
			if currentGitLabToolContainsOnlyHeading(current.String()) {
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
		if looksLikeGitLabToolLabel(trimmed) {
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

func currentGitLabToolContainsOnlyHeading(text string) bool {
	text = strings.TrimSpace(text)
	return strings.HasPrefix(text, "#") && !strings.Contains(text, "\n")
}

func looksLikeGitLabToolLabel(line string) bool {
	line = strings.TrimLeft(line, "-*> \t")
	idx := strings.IndexAny(line, "：:")
	if idx <= 0 || idx > 24 {
		return false
	}
	prefix := strings.TrimSpace(line[:idx])
	if prefix == "" {
		return false
	}
	for _, section := range gitLabToolStructuredSectionsFor("story") {
		for _, keyword := range section.keywords {
			if strings.Contains(prefix, keyword) {
				return true
			}
		}
	}
	for _, section := range gitLabToolStructuredSectionsFor("bug") {
		for _, keyword := range section.keywords {
			if strings.Contains(prefix, keyword) {
				return true
			}
		}
	}
	return false
}

func trimGitLabToolParagraphLabel(paragraph string) string {
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
	if key, value, ok := splitGitLabToolLabel(cleaned); ok && len([]rune(key)) <= 24 {
		if value != "" {
			if len(lines) == 1 {
				return value
			}
			return strings.TrimSpace(value + "\n" + lines[1])
		}
	}
	return paragraph
}

func splitGitLabToolLabel(text string) (string, string, bool) {
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

func isGitLabToolSnapshotReady(title, markdownDescription string) bool {
	title = strings.TrimSpace(title)
	md := strings.TrimSpace(markdownDescription)
	return title != "" && md != "" && md != "<p><br></p>"
}

func fingerprintGitLabToolSnapshot(parts ...string) string {
	normalized := make([]string, 0, len(parts))
	for _, part := range parts {
		normalized = append(normalized, strings.TrimSpace(part))
	}
	sum := sha256.Sum256([]byte(strings.Join(normalized, "\n")))
	return hex.EncodeToString(sum[:])
}

func parseGitLabToolIntCSV(raw string) ([]int, error) {
	parts := splitGitLabToolCSV(raw)
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

func splitGitLabToolCSV(raw string) []string {
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

func optBool(args map[string]interface{}, key string) bool {
	if v, ok := args[key].(bool); ok {
		return v
	}
	return false
}

func firstNonEmptyMCP(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
