// Package cmd 中的 mcp_resources.go 负责注册 MCP resources,
// 把本地事件缓存暴露给 AI。
//
// 设计原则:
//  1. description 要明确告诉 AI "查询需求/缺陷进展时优先读这里"
//  2. 暴露 workspace context 给 AI,避免它误用默认或随机的 workspace
//  3. 提供按 workspace 过滤的 resource,适配多空间场景
package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/studyzy/tapd-ai-cli/internal/mcp"
)

// RegisterEventResources 注册 tapd://events/* 系列 resources 与 tapd://workspaces/active 上下文资源。
func RegisterEventResources(server *mcp.Server) {
	cache := newEventCache()

	// tapd://workspaces/active - 暴露当前用户关注的 workspace 列表
	// AI 在不确定用哪个 workspace 时应优先读这个,避免乱选
	server.RegisterResource(&mcp.Resource{
		URI:  "tapd://workspaces/active",
		Name: "Active TAPD Workspaces",
		Description: "用户当前关注的 TAPD workspace 列表(从 TAPD_WATCH_WORKSPACES 或 TAPD_WORKSPACE_ID 环境变量读取)。" +
			"⚠️ 任何 TAPD 查询(列 story/bug、查需求详情等)在没有明确 workspace_id 时,必须先读这个 resource 取默认值,不要自行猜测。",
		MimeType: "application/json",
		Handler: func(ctx context.Context, uri string) (interface{}, error) {
			watchList := splitNonEmpty(os.Getenv("TAPD_WATCH_WORKSPACES"))
			defaultWS := strings.TrimSpace(os.Getenv("TAPD_WORKSPACE_ID"))
			result := map[string]interface{}{
				"watched_workspace_ids": watchList,
				"default_workspace_id":  defaultWS,
				"hint": "调用 tapd 工具(如 list_stories)时,如果用户没指明空间,优先用 default_workspace_id; " +
					"如果它也为空,从 watched_workspace_ids 里选;两者都空才提示用户提供。",
			}
			data, _ := json.Marshal(result)
			return string(data), nil
		},
	})

	// tapd://events/recent - 返回最近 N 条事件
	server.RegisterResource(&mcp.Resource{
		URI:  "tapd://events/recent",
		Name: "Recent TAPD Events",
		Description: "本地缓存的 TAPD webhook 事件流(story/bug/task 创建、更新、指派、评论、关联等),最多保留 100 条。" +
			"⚠️ 用户问\"某需求/缺陷的最近进展\"\"最近发生了什么\"\"我的项目动态\"时,务必先读这个 resource," +
			"不要直接调用 TAPD API 查询。本地缓存包含完整 payload 和时间戳,响应更快且涵盖了用户实际关心的事件。" +
			"\n\n## 事件类型说明" +
			"\n- story::* - 需求事件(create/update/bug_link/story_link),`event.old_parent_id` 非空表示子需求" +
			"\n- bug::* - 缺陷事件(create/update),通常没有父子关系" +
			"\n- task::* - 任务事件(create/update),`event.old_story_id` 表示关联的需求 ID;一个需求下面可以有多个任务" +
			"\n- *_comment::add - 评论事件(story_comment::add / bug_comment::add),`event.entry_id` 是被评论实体的 ID" +
			"\n\n## 默认过滤规则" +
			"\n用户问\"需求进展\"\"项目动态\"等宽泛问题时,默认只展示父级实体的核心变更,屏蔽以下噪音:" +
			"\n1. story 事件:`event.old_parent_id` 非空(不是空字符串/null/0)的子需求,默认过滤" +
			"\n2. task 事件:默认归并到对应需求(`old_story_id`)下展示;用户明确问\"任务\"\"task\"时才单独列出" +
			"\n3. *_link 事件(story::bug_link / story::story_link):一般是关联关系建立,默认折叠" +
			"\n用户问\"全部变更\"\"包括子任务/子需求\"时,保留全部不过滤。",
		MimeType: "application/json",
		Handler: func(ctx context.Context, uri string) (interface{}, error) {
			events, err := cache.ReadAll()
			if err != nil {
				return nil, fmt.Errorf("read events cache: %w", err)
			}
			if len(events) == 0 {
				return map[string]interface{}{
					"events": []interface{}{},
					"count":  0,
					"hint":   "本地暂无缓存事件,请确认 'tapd watch' 已在后台运行。如需立即拉取数据可改用 tapd 工具(list_stories 等)。",
				}, nil
			}
			summary := map[string]interface{}{
				"events": events,
				"count":  len(events),
				"hint":   "events 已按时间倒序排列(最新在前);每条 event 字段是原始 webhook payload,含 workspace_id / story_id / bug_id 等。",
			}
			data, _ := json.Marshal(summary)
			return string(data), nil
		},
	})

	// tapd://events/summary - 事件类型 + workspace 维度的统计摘要
	server.RegisterResource(&mcp.Resource{
		URI:  "tapd://events/summary",
		Name: "TAPD Events Summary",
		Description: "本地缓存事件的统计摘要:按事件类型 + workspace 双维度分组计数。" +
			"用于快速了解最近哪些空间最活跃、哪类事件最多。token 消耗远小于 tapd://events/recent。",
		MimeType: "application/json",
		Handler: func(ctx context.Context, uri string) (interface{}, error) {
			events, err := cache.ReadAll()
			if err != nil {
				return nil, fmt.Errorf("read events cache: %w", err)
			}
			eventTypeStats := make(map[string]int)
			workspaceStats := make(map[string]int)
			for _, ev := range events {
				var payload struct {
					Event       string      `json:"event"`
					WorkspaceID interface{} `json:"workspace_id"`
				}
				if err := json.Unmarshal(ev.Event, &payload); err != nil {
					continue
				}
				if payload.Event != "" {
					eventTypeStats[payload.Event]++
				}
				if ws := stringifyAny(payload.WorkspaceID); ws != "" {
					workspaceStats[ws]++
				}
			}
			summary := map[string]interface{}{
				"total":           len(events),
				"event_types":     eventTypeStats,
				"workspace_stats": workspaceStats,
			}
			data, _ := json.Marshal(summary)
			return string(data), nil
		},
	})
}

// splitNonEmpty 按逗号切分字符串,去除空白和空项。
func splitNonEmpty(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if v := strings.TrimSpace(p); v != "" {
			out = append(out, v)
		}
	}
	return out
}

// stringifyAny 把任意 JSON 值转字符串,用于 workspace_id(可能是字符串或数字)。
func stringifyAny(v interface{}) string {
	switch x := v.(type) {
	case nil:
		return ""
	case string:
		return x
	case float64:
		return fmt.Sprintf("%.0f", x)
	default:
		return fmt.Sprint(x)
	}
}
