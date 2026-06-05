// Package tapdurl 把 TAPD URL 解析逻辑收敛到一处，供 CLI 子命令与 MCP 工具共用。
//
// 支持的 URL 格式：
//
//  1. detail 路径：/tapd_fe/{ws}/{type}/detail/{id}
//  2. view 路径：  /{ws}/prong/{type}s/view/{id}              ← TAPD 新版 UI
//  3. dialog_preview_id 查询：?dialog_preview_id={type}_{id}
//  4. Wiki fragment：       /{ws}/markdown_wikis/show/#{id}
//
// 实体类型：story / bug / task / wiki。
// view 路径里类型是复数（stories/bugs/tasks），解析时统一归一为单数。
package tapdurl

import (
	"fmt"
	"net/url"
	"strings"
)

// Parsed 是一次解析结果。EntityType 取值：story / bug / task / wiki。
type Parsed struct {
	WorkspaceID string
	EntityType  string
	EntityID    string
}

// Parse 解析 TAPD URL；不依赖网络，只看 URL 形态。
func Parse(rawURL string) (*Parsed, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("invalid URL: %w", err)
	}
	if u.Hostname() == "" {
		return nil, fmt.Errorf("not a valid TAPD URL: missing host")
	}
	segs := splitNonEmpty(u.Path)

	// 1. dialog_preview_id 查询参数（优先）
	if previewID := u.Query().Get("dialog_preview_id"); previewID != "" {
		entityType, entityID, err := splitPreviewID(previewID)
		if err != nil {
			return nil, err
		}
		ws, err := extractWorkspaceID(segs)
		if err != nil {
			return nil, err
		}
		return &Parsed{WorkspaceID: ws, EntityType: entityType, EntityID: entityID}, nil
	}

	// 2. wiki fragment
	if containsSeg(segs, "markdown_wikis") {
		ws, err := extractWorkspaceID(segs)
		if err != nil {
			return nil, err
		}
		id := strings.TrimSpace(u.Fragment)
		if id == "" {
			return nil, fmt.Errorf("wiki URL missing #id")
		}
		return &Parsed{WorkspaceID: ws, EntityType: "wiki", EntityID: id}, nil
	}

	// 3. detail 路径：.../{type}/detail/{id}
	if t, id, ok := matchDetailPath(segs); ok {
		ws, err := extractWorkspaceID(segs)
		if err != nil {
			return nil, err
		}
		return &Parsed{WorkspaceID: ws, EntityType: t, EntityID: id}, nil
	}

	// 4. view 路径：/{ws}/prong/{type}s/view/{id}
	if t, id, ok := matchProngViewPath(segs); ok {
		ws, err := extractWorkspaceID(segs)
		if err != nil {
			return nil, err
		}
		return &Parsed{WorkspaceID: ws, EntityType: t, EntityID: id}, nil
	}

	return nil, fmt.Errorf("cannot identify TAPD entity from URL %q", rawURL)
}

// splitNonEmpty 按 "/" 切分并丢弃空段。
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

// extractWorkspaceID 兼容 /tapd_fe/{ws}/... 与 /{ws}/... 两种前缀。
func extractWorkspaceID(segs []string) (string, error) {
	if len(segs) == 0 {
		return "", fmt.Errorf("URL path is empty, cannot extract workspace ID")
	}
	if segs[0] == "tapd_fe" {
		if len(segs) < 2 {
			return "", fmt.Errorf("URL path too short to extract workspace ID")
		}
		return segs[1], nil
	}
	return segs[0], nil
}

// splitPreviewID 解析 "{type}_{id}" 形式的预览 ID。
func splitPreviewID(pv string) (entityType, entityID string, err error) {
	for _, t := range []string{"story", "bug", "task"} {
		prefix := t + "_"
		if strings.HasPrefix(pv, prefix) {
			return t, strings.TrimPrefix(pv, prefix), nil
		}
	}
	return "", "", fmt.Errorf("unsupported entity in dialog_preview_id %q", pv)
}

// matchDetailPath 找到 ".../{type}/detail/{id}"；type 必须是单数 story/bug/task。
func matchDetailPath(segs []string) (entityType, entityID string, ok bool) {
	for i, s := range segs {
		if s != "detail" || i == 0 || i+1 >= len(segs) {
			continue
		}
		t := segs[i-1]
		if t == "story" || t == "bug" || t == "task" {
			return t, segs[i+1], true
		}
	}
	return "", "", false
}

// matchProngViewPath 找到 "{ws}/prong/{type}s/view/{id}"；
// type 是复数（stories/bugs/tasks），归一为单数。
func matchProngViewPath(segs []string) (entityType, entityID string, ok bool) {
	for i, s := range segs {
		if s != "view" || i < 2 || i+1 >= len(segs) {
			continue
		}
		// 例：[20063271, prong, stories, view, 1120063271004942331]
		// segs[i-2] 应为 "prong"，segs[i-1] 应为 stories/bugs/tasks
		if segs[i-2] != "prong" {
			continue
		}
		switch segs[i-1] {
		case "stories":
			return "story", segs[i+1], true
		case "bugs":
			return "bug", segs[i+1], true
		case "tasks":
			return "task", segs[i+1], true
		}
	}
	return "", "", false
}
