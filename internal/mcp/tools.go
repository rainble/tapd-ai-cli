// Package mcp 中的 tools.go 注册具体的 TAPD 工具到 MCP server。
//
// 设计原则：
//  1. 工具命名一律 tapd_<entity>_<verb>，便于 AI 联想；
//  2. 入参 schema 使用 JSON Schema，必填字段明确标注；
//  3. workspace_id 兜底——若调用方未传，回落到 ~/.tapd.json 中保存的默认；
//  4. 返回原样的 SDK 数据结构，由协议层统一序列化为紧凑 JSON。
package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"github.com/studyzy/tapd-sdk-go/model"
)

// RegisterDefaultTools 把首批工具挂到 server 上。
// defaultWorkspaceID 是 ~/.tapd.json 中保存的工作区，调用方未指定 workspace_id 时使用。
func RegisterDefaultTools(s *Server, defaultWorkspaceID string) {
	ws := func(arg string) string {
		if arg != "" {
			return arg
		}
		return defaultWorkspaceID
	}

	s.Register(toolWorkspaceList(s))
	s.Register(toolURLResolve(s, ws))
	s.Register(toolStoryList(s, ws))
	s.Register(toolStoryShow(s, ws))
	s.Register(toolStoryUpdate(s, ws))
	s.Register(toolBugList(s, ws))
	s.Register(toolBugShow(s, ws))
	s.Register(toolBugCreate(s, ws))
	s.Register(toolTaskList(s, ws))
	s.Register(toolTaskShow(s, ws))
	s.Register(toolIterationList(s, ws))
	s.Register(toolCommentList(s, ws))
	s.Register(toolCommentAdd(s, ws))
}

// schema 把字面量 JSON 转成 RawMessage，避免每个工具都重复写 json.RawMessage([]byte(...))。
func schema(s string) json.RawMessage { return json.RawMessage(s) }

// requireString 从 args 取必填 string 字段；缺失或为空时报错（含字段名，便于 AI 自纠）。
func requireString(args map[string]interface{}, key string) (string, error) {
	v, ok := args[key]
	if !ok {
		return "", fmt.Errorf("missing required argument: %s", key)
	}
	s, ok := v.(string)
	if !ok || s == "" {
		return "", fmt.Errorf("argument %s must be a non-empty string", key)
	}
	return s, nil
}

func optString(args map[string]interface{}, key string) string {
	if v, ok := args[key].(string); ok {
		return v
	}
	return ""
}

func optInt(args map[string]interface{}, key string) int {
	if v, ok := args[key].(float64); ok {
		return int(v)
	}
	return 0
}

// parseArgs 把 RawMessage 解成 map[string]interface{}；空 args 返回空 map 而非 nil，
// 让 optXxx 直接读 zero value 不会 panic。
func parseArgs(raw json.RawMessage) (map[string]interface{}, error) {
	if len(raw) == 0 {
		return map[string]interface{}{}, nil
	}
	var m map[string]interface{}
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, err
	}
	if m == nil {
		m = map[string]interface{}{}
	}
	return m, nil
}

// ─────────────────────────── workspace ───────────────────────────

func toolWorkspaceList(s *Server) *Tool {
	return &Tool{
		Name:        "tapd_workspace_list",
		Description: "List all TAPD workspaces (projects) the current user belongs to. No arguments.",
		InputSchema: schema(`{"type":"object","properties":{},"additionalProperties":false}`),
		Handler: func(ctx context.Context, _ json.RawMessage) (interface{}, error) {
			return s.client.ListWorkspaces(ctx)
		},
	}
}

// ─────────────────────────── url resolve ─────────────────────────

func toolURLResolve(s *Server, ws func(string) string) *Tool {
	return &Tool{
		Name: "tapd_url_resolve",
		Description: "Resolve a TAPD URL (story / bug / task / wiki page) and return the entity " +
			"detail. Useful when the user pastes a TAPD link.",
		InputSchema: schema(`{
			"type":"object",
			"properties":{"url":{"type":"string","description":"a TAPD entity URL"}},
			"required":["url"],
			"additionalProperties":false
		}`),
		Handler: func(ctx context.Context, raw json.RawMessage) (interface{}, error) {
			args, err := parseArgs(raw)
			if err != nil {
				return nil, err
			}
			rawURL, err := requireString(args, "url")
			if err != nil {
				return nil, err
			}
			workspaceID, entityType, entityID, err := parseTapdURL(rawURL)
			if err != nil {
				return nil, err
			}
			workspaceID = orFallback(workspaceID, ws(""))
			switch entityType {
			case "story":
				return s.client.GetStory(ctx, workspaceID, entityID)
			case "bug":
				return s.client.GetBug(ctx, workspaceID, entityID)
			case "task":
				return s.client.GetTask(ctx, workspaceID, entityID)
			case "wiki":
				return s.client.GetWiki(ctx, workspaceID, entityID)
			default:
				return nil, fmt.Errorf("unsupported entity type %q", entityType)
			}
		},
	}
}

// ─────────────────────────── story ───────────────────────────────

func toolStoryList(s *Server, ws func(string) string) *Tool {
	return &Tool{
		Name: "tapd_story_list",
		Description: "List stories in a workspace. Filter by status / owner / iteration / name. " +
			"Defaults to the configured workspace if workspace_id is omitted.",
		InputSchema: schema(`{
			"type":"object",
			"properties":{
				"workspace_id":{"type":"string"},
				"status":{"type":"string"},
				"owner":{"type":"string"},
				"iteration_id":{"type":"string"},
				"name":{"type":"string"},
				"limit":{"type":"integer","minimum":1,"maximum":200},
				"page":{"type":"integer","minimum":1}
			},
			"additionalProperties":false
		}`),
		Handler: func(ctx context.Context, raw json.RawMessage) (interface{}, error) {
			args, err := parseArgs(raw)
			if err != nil {
				return nil, err
			}
			req := &model.ListStoriesRequest{
				WorkspaceID: ws(optString(args, "workspace_id")),
				Status:      optString(args, "status"),
				Owner:       optString(args, "owner"),
				IterationID: optString(args, "iteration_id"),
				Name:        optString(args, "name"),
				Limit:       optInt(args, "limit"),
				Page:        optInt(args, "page"),
			}
			if req.WorkspaceID == "" {
				return nil, fmt.Errorf("workspace_id required (no default configured)")
			}
			return s.client.ListStories(ctx, req)
		},
	}
}

func toolStoryShow(s *Server, ws func(string) string) *Tool {
	return &Tool{
		Name:        "tapd_story_show",
		Description: "Get full detail (with description) of a single story by ID.",
		InputSchema: schema(`{
			"type":"object",
			"properties":{
				"id":{"type":"string"},
				"workspace_id":{"type":"string"}
			},
			"required":["id"],
			"additionalProperties":false
		}`),
		Handler: func(ctx context.Context, raw json.RawMessage) (interface{}, error) {
			args, err := parseArgs(raw)
			if err != nil {
				return nil, err
			}
			id, err := requireString(args, "id")
			if err != nil {
				return nil, err
			}
			workspaceID := ws(optString(args, "workspace_id"))
			if workspaceID == "" {
				return nil, fmt.Errorf("workspace_id required (no default configured)")
			}
			return s.client.GetStory(ctx, workspaceID, id)
		},
	}
}

func toolStoryUpdate(s *Server, ws func(string) string) *Tool {
	return &Tool{
		Name: "tapd_story_update",
		Description: "Update fields of an existing story (status / owner / iteration / priority etc.). " +
			"Only provided fields are touched.",
		InputSchema: schema(`{
			"type":"object",
			"properties":{
				"id":{"type":"string"},
				"workspace_id":{"type":"string"},
				"current_user":{"type":"string","description":"required by TAPD as the change actor; defaults to authenticated user"},
				"name":{"type":"string"},
				"status":{"type":"string"},
				"v_status":{"type":"string"},
				"owner":{"type":"string"},
				"priority_label":{"type":"string"},
				"iteration_id":{"type":"string"},
				"description":{"type":"string"}
			},
			"required":["id"],
			"additionalProperties":false
		}`),
		Handler: func(ctx context.Context, raw json.RawMessage) (interface{}, error) {
			args, err := parseArgs(raw)
			if err != nil {
				return nil, err
			}
			id, err := requireString(args, "id")
			if err != nil {
				return nil, err
			}
			workspaceID := ws(optString(args, "workspace_id"))
			if workspaceID == "" {
				return nil, fmt.Errorf("workspace_id required (no default configured)")
			}
			currentUser := optString(args, "current_user")
			if currentUser == "" {
				currentUser = s.client.GetNick()
				if currentUser == "" {
					_ = s.client.FetchNick(ctx)
					currentUser = s.client.GetNick()
				}
			}
			req := &model.UpdateStoryRequest{
				WorkspaceID:   workspaceID,
				ID:            id,
				CurrentUser:   currentUser,
				Name:          optString(args, "name"),
				Status:        optString(args, "status"),
				VStatus:       optString(args, "v_status"),
				Owner:         optString(args, "owner"),
				PriorityLabel: optString(args, "priority_label"),
				IterationID:   optString(args, "iteration_id"),
				Description:   optString(args, "description"),
			}
			return s.client.UpdateStory(ctx, req)
		},
	}
}

// ─────────────────────────── bug ─────────────────────────────────

func toolBugList(s *Server, ws func(string) string) *Tool {
	return &Tool{
		Name:        "tapd_bug_list",
		Description: "List bugs in a workspace with common filters.",
		InputSchema: schema(`{
			"type":"object",
			"properties":{
				"workspace_id":{"type":"string"},
				"status":{"type":"string"},
				"current_owner":{"type":"string"},
				"priority_label":{"type":"string"},
				"iteration_id":{"type":"string"},
				"title":{"type":"string"},
				"limit":{"type":"integer","minimum":1,"maximum":200},
				"page":{"type":"integer","minimum":1}
			},
			"additionalProperties":false
		}`),
		Handler: func(ctx context.Context, raw json.RawMessage) (interface{}, error) {
			args, err := parseArgs(raw)
			if err != nil {
				return nil, err
			}
			workspaceID := ws(optString(args, "workspace_id"))
			if workspaceID == "" {
				return nil, fmt.Errorf("workspace_id required (no default configured)")
			}
			req := &model.ListBugsRequest{
				WorkspaceID:   workspaceID,
				Status:        optString(args, "status"),
				CurrentOwner:  optString(args, "current_owner"),
				PriorityLabel: optString(args, "priority_label"),
				IterationID:   optString(args, "iteration_id"),
				Title:         optString(args, "title"),
				Limit:         optInt(args, "limit"),
				Page:          optInt(args, "page"),
			}
			return s.client.ListBugs(ctx, req)
		},
	}
}

func toolBugShow(s *Server, ws func(string) string) *Tool {
	return &Tool{
		Name:        "tapd_bug_show",
		Description: "Get full detail of a single bug by ID.",
		InputSchema: schema(`{
			"type":"object",
			"properties":{
				"id":{"type":"string"},
				"workspace_id":{"type":"string"}
			},
			"required":["id"],
			"additionalProperties":false
		}`),
		Handler: func(ctx context.Context, raw json.RawMessage) (interface{}, error) {
			args, err := parseArgs(raw)
			if err != nil {
				return nil, err
			}
			id, err := requireString(args, "id")
			if err != nil {
				return nil, err
			}
			workspaceID := ws(optString(args, "workspace_id"))
			if workspaceID == "" {
				return nil, fmt.Errorf("workspace_id required (no default configured)")
			}
			return s.client.GetBug(ctx, workspaceID, id)
		},
	}
}

func toolBugCreate(s *Server, ws func(string) string) *Tool {
	return &Tool{
		Name:        "tapd_bug_create",
		Description: "Create a new bug. Title is required; other fields optional.",
		InputSchema: schema(`{
			"type":"object",
			"properties":{
				"workspace_id":{"type":"string"},
				"title":{"type":"string"},
				"description":{"type":"string"},
				"priority_label":{"type":"string"},
				"severity":{"type":"string"},
				"current_owner":{"type":"string"},
				"iteration_id":{"type":"string"},
				"reporter":{"type":"string"}
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
			workspaceID := ws(optString(args, "workspace_id"))
			if workspaceID == "" {
				return nil, fmt.Errorf("workspace_id required (no default configured)")
			}
			reporter := optString(args, "reporter")
			if reporter == "" {
				reporter = s.client.GetNick()
				if reporter == "" {
					_ = s.client.FetchNick(ctx)
					reporter = s.client.GetNick()
				}
			}
			req := &model.CreateBugRequest{
				WorkspaceID:   workspaceID,
				Title:         title,
				Description:   optString(args, "description"),
				PriorityLabel: optString(args, "priority_label"),
				Severity:      optString(args, "severity"),
				CurrentOwner:  optString(args, "current_owner"),
				IterationID:   optString(args, "iteration_id"),
				Reporter:      reporter,
			}
			return s.client.CreateBug(ctx, req)
		},
	}
}

// ─────────────────────────── task ────────────────────────────────

func toolTaskList(s *Server, ws func(string) string) *Tool {
	return &Tool{
		Name:        "tapd_task_list",
		Description: "List tasks in a workspace.",
		InputSchema: schema(`{
			"type":"object",
			"properties":{
				"workspace_id":{"type":"string"},
				"status":{"type":"string"},
				"owner":{"type":"string"},
				"iteration_id":{"type":"string"},
				"story_id":{"type":"string"},
				"name":{"type":"string"},
				"limit":{"type":"integer","minimum":1,"maximum":200},
				"page":{"type":"integer","minimum":1}
			},
			"additionalProperties":false
		}`),
		Handler: func(ctx context.Context, raw json.RawMessage) (interface{}, error) {
			args, err := parseArgs(raw)
			if err != nil {
				return nil, err
			}
			workspaceID := ws(optString(args, "workspace_id"))
			if workspaceID == "" {
				return nil, fmt.Errorf("workspace_id required (no default configured)")
			}
			req := &model.ListTasksRequest{
				WorkspaceID: workspaceID,
				Status:      optString(args, "status"),
				Owner:       optString(args, "owner"),
				IterationID: optString(args, "iteration_id"),
				StoryID:     optString(args, "story_id"),
				Name:        optString(args, "name"),
				Limit:       optInt(args, "limit"),
				Page:        optInt(args, "page"),
			}
			return s.client.ListTasks(ctx, req)
		},
	}
}

func toolTaskShow(s *Server, ws func(string) string) *Tool {
	return &Tool{
		Name:        "tapd_task_show",
		Description: "Get full detail of a single task by ID.",
		InputSchema: schema(`{
			"type":"object",
			"properties":{
				"id":{"type":"string"},
				"workspace_id":{"type":"string"}
			},
			"required":["id"],
			"additionalProperties":false
		}`),
		Handler: func(ctx context.Context, raw json.RawMessage) (interface{}, error) {
			args, err := parseArgs(raw)
			if err != nil {
				return nil, err
			}
			id, err := requireString(args, "id")
			if err != nil {
				return nil, err
			}
			workspaceID := ws(optString(args, "workspace_id"))
			if workspaceID == "" {
				return nil, fmt.Errorf("workspace_id required (no default configured)")
			}
			return s.client.GetTask(ctx, workspaceID, id)
		},
	}
}

// ─────────────────────────── iteration ───────────────────────────

func toolIterationList(s *Server, ws func(string) string) *Tool {
	return &Tool{
		Name:        "tapd_iteration_list",
		Description: "List iterations (sprints) in a workspace.",
		InputSchema: schema(`{
			"type":"object",
			"properties":{
				"workspace_id":{"type":"string"},
				"status":{"type":"string"},
				"name":{"type":"string"},
				"limit":{"type":"integer","minimum":1,"maximum":200}
			},
			"additionalProperties":false
		}`),
		Handler: func(ctx context.Context, raw json.RawMessage) (interface{}, error) {
			args, err := parseArgs(raw)
			if err != nil {
				return nil, err
			}
			workspaceID := ws(optString(args, "workspace_id"))
			if workspaceID == "" {
				return nil, fmt.Errorf("workspace_id required (no default configured)")
			}
			req := &model.ListIterationsRequest{
				WorkspaceID: workspaceID,
				Status:      optString(args, "status"),
				Name:        optString(args, "name"),
				Limit:       optInt(args, "limit"),
			}
			return s.client.ListIterations(ctx, req)
		},
	}
}

// ─────────────────────────── comment ─────────────────────────────

func toolCommentList(s *Server, ws func(string) string) *Tool {
	return &Tool{
		Name:        "tapd_comment_list",
		Description: "List comments attached to a story / bug / task. entry_type: stories | bug | tasks.",
		InputSchema: schema(`{
			"type":"object",
			"properties":{
				"workspace_id":{"type":"string"},
				"entry_type":{"type":"string","enum":["stories","bug","tasks"]},
				"entry_id":{"type":"string"}
			},
			"required":["entry_type","entry_id"],
			"additionalProperties":false
		}`),
		Handler: func(ctx context.Context, raw json.RawMessage) (interface{}, error) {
			args, err := parseArgs(raw)
			if err != nil {
				return nil, err
			}
			entryType, err := requireString(args, "entry_type")
			if err != nil {
				return nil, err
			}
			entryID, err := requireString(args, "entry_id")
			if err != nil {
				return nil, err
			}
			workspaceID := ws(optString(args, "workspace_id"))
			if workspaceID == "" {
				return nil, fmt.Errorf("workspace_id required (no default configured)")
			}
			return s.client.ListComments(ctx, &model.ListCommentsRequest{
				WorkspaceID: workspaceID,
				EntryType:   entryType,
				EntryID:     entryID,
			})
		},
	}
}

func toolCommentAdd(s *Server, ws func(string) string) *Tool {
	return &Tool{
		Name:        "tapd_comment_add",
		Description: "Add a comment to a story / bug / task.",
		InputSchema: schema(`{
			"type":"object",
			"properties":{
				"workspace_id":{"type":"string"},
				"entry_type":{"type":"string","enum":["stories","bug","tasks"]},
				"entry_id":{"type":"string"},
				"description":{"type":"string"},
				"author":{"type":"string","description":"defaults to authenticated user"}
			},
			"required":["entry_type","entry_id","description"],
			"additionalProperties":false
		}`),
		Handler: func(ctx context.Context, raw json.RawMessage) (interface{}, error) {
			args, err := parseArgs(raw)
			if err != nil {
				return nil, err
			}
			entryType, err := requireString(args, "entry_type")
			if err != nil {
				return nil, err
			}
			entryID, err := requireString(args, "entry_id")
			if err != nil {
				return nil, err
			}
			description, err := requireString(args, "description")
			if err != nil {
				return nil, err
			}
			workspaceID := ws(optString(args, "workspace_id"))
			if workspaceID == "" {
				return nil, fmt.Errorf("workspace_id required (no default configured)")
			}
			author := optString(args, "author")
			if author == "" {
				author = s.client.GetNick()
				if author == "" {
					_ = s.client.FetchNick(ctx)
					author = s.client.GetNick()
				}
			}
			return s.client.AddComment(ctx, &model.AddCommentRequest{
				WorkspaceID: workspaceID,
				EntryType:   entryType,
				EntryID:     entryID,
				Description: description,
				Author:      author,
			})
		},
	}
}

// ─────────────────────────── helpers ─────────────────────────────

func orFallback(primary, fallback string) string {
	if primary != "" {
		return primary
	}
	return fallback
}

// parseTapdURL 是 cmd/url.go 解析逻辑的精简版——MCP 包独立实现以避免循环依赖。
// 仅支持 detail 路径与 dialog_preview_id；wiki fragment 用 #id 形式。
func parseTapdURL(rawURL string) (workspaceID, entityType, entityID string, err error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "", "", "", fmt.Errorf("invalid URL: %w", err)
	}
	segs := splitNonEmpty(u.Path)
	if len(segs) == 0 {
		return "", "", "", fmt.Errorf("empty URL path")
	}

	// dialog_preview_id 格式：?dialog_preview_id={type}_{id}
	if pv := u.Query().Get("dialog_preview_id"); pv != "" {
		entityType, entityID, err = splitPreviewID(pv)
		if err != nil {
			return "", "", "", err
		}
		workspaceID = extractWorkspaceID(segs)
		return workspaceID, entityType, entityID, nil
	}
	// wiki: /{ws}/markdown_wikis/show/#{id}
	if containsSeg(segs, "markdown_wikis") {
		workspaceID = extractWorkspaceID(segs)
		entityID = strings.TrimSpace(u.Fragment)
		if entityID == "" {
			return "", "", "", fmt.Errorf("wiki URL missing #id")
		}
		return workspaceID, "wiki", entityID, nil
	}
	// detail 路径
	for i, s := range segs {
		if s == "detail" && i > 0 && i+1 < len(segs) {
			t := segs[i-1]
			id := segs[i+1]
			switch t {
			case "story", "bug", "task":
				return extractWorkspaceID(segs), t, id, nil
			}
		}
	}
	return "", "", "", fmt.Errorf("cannot identify entity from URL %q", rawURL)
}

func splitNonEmpty(p string) []string {
	out := []string{}
	for _, s := range strings.Split(p, "/") {
		if s != "" {
			out = append(out, s)
		}
	}
	return out
}

func containsSeg(ss []string, target string) bool {
	for _, s := range ss {
		if s == target {
			return true
		}
	}
	return false
}

func extractWorkspaceID(segs []string) string {
	if len(segs) == 0 {
		return ""
	}
	if segs[0] == "tapd_fe" && len(segs) >= 2 {
		return segs[1]
	}
	return segs[0]
}

func splitPreviewID(pv string) (entityType, entityID string, err error) {
	for _, t := range []string{"story", "bug", "task"} {
		prefix := t + "_"
		if strings.HasPrefix(pv, prefix) {
			return t, strings.TrimPrefix(pv, prefix), nil
		}
	}
	return "", "", fmt.Errorf("unknown preview id %q", pv)
}
