package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// bugEventTarget 是从 TAPD webhook 事件中解析出的缺陷处理目标。
type bugEventTarget struct {
	WorkspaceID string
	BugID       string
	EventID     uint64
}

// isBugWebhookEvent 判断事件名是否是缺陷创建或更新。
func isBugWebhookEvent(name string) bool {
	switch strings.TrimSpace(name) {
	case "bug::create", "bug::update", "bug_create", "bug_update":
		return true
	default:
		return false
	}
}

// extractBugEventTarget 从 watch 的 streamEvent 中提取 workspace_id 和 bug id。
func extractBugEventTarget(ev *streamEvent) (bugEventTarget, bool, string) {
	if ev == nil {
		return bugEventTarget{}, false, "nil_event"
	}
	var payload map[string]interface{}
	decoder := json.NewDecoder(bytes.NewReader(ev.Event))
	decoder.UseNumber()
	if err := decoder.Decode(&payload); err != nil {
		return bugEventTarget{}, false, "invalid_payload"
	}
	eventName := stringifyBugEventValue(payload["event"])
	if !isBugWebhookEvent(eventName) {
		return bugEventTarget{}, false, "not_bug_event"
	}
	workspaceID := stringifyBugEventValue(payload["workspace_id"])
	if workspaceID == "" {
		return bugEventTarget{}, false, "missing_workspace_id"
	}
	bugID := firstBugEventPathString(payload, [][]string{
		{"bug", "id"},
		{"object", "id"},
		{"id"},
		{"data", "bug", "id"},
		{"data", "id"},
	})
	if bugID == "" {
		return bugEventTarget{}, false, "missing_bug_id"
	}
	return bugEventTarget{WorkspaceID: workspaceID, BugID: bugID, EventID: ev.ID}, true, ""
}

func firstBugEventPathString(root map[string]interface{}, paths [][]string) string {
	for _, path := range paths {
		if v, ok := lookupBugEventPath(root, path); ok {
			if s := stringifyBugEventValue(v); s != "" {
				return s
			}
		}
	}
	return ""
}

func lookupBugEventPath(root map[string]interface{}, path []string) (interface{}, bool) {
	var cur interface{} = root
	for _, part := range path {
		obj, ok := cur.(map[string]interface{})
		if !ok {
			return nil, false
		}
		cur, ok = obj[part]
		if !ok {
			return nil, false
		}
	}
	return cur, true
}

func stringifyBugEventValue(v interface{}) string {
	switch x := v.(type) {
	case string:
		return strings.TrimSpace(x)
	case float64:
		if x == float64(int64(x)) {
			return strconv.FormatInt(int64(x), 10)
		}
		return fmt.Sprintf("%v", x)
	case json.Number:
		return x.String()
	default:
		return ""
	}
}
