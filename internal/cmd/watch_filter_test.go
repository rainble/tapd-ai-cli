package cmd

import (
	"encoding/json"
	"testing"
)

func TestParseFilters_Valid(t *testing.T) {
	cases := []struct {
		name  string
		expr  string
		path  []string
		globs []string
	}{
		{"single value", "event=story_create", []string{"event"}, []string{"story_create"}},
		{"multi values OR", "event=story_*,bug_*", []string{"event"}, []string{"story_*", "bug_*"}},
		{"nested path", "event.story.priority=High", []string{"event", "story", "priority"}, []string{"High"}},
		{"escaped comma", `event=a\,b,c`, []string{"event"}, []string{"a,b", "c"}},
		{"trim whitespace", "  event = story_*  ,  bug_*  ", []string{"event"}, []string{"story_*", "bug_*"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rules, err := parseFilters([]string{tc.expr})
			if err != nil {
				t.Fatalf("parseFilters err: %v", err)
			}
			if len(rules) != 1 {
				t.Fatalf("want 1 rule, got %d", len(rules))
			}
			r := rules[0]
			if !equalSlice(r.path, tc.path) {
				t.Fatalf("path mismatch: want %v got %v", tc.path, r.path)
			}
			if !equalSlice(r.globs, tc.globs) {
				t.Fatalf("globs mismatch: want %v got %v", tc.globs, r.globs)
			}
		})
	}
}

func TestParseFilters_Invalid(t *testing.T) {
	bads := []string{
		"",            // 空被跳过，但 ParseFilters 至少要解析一条；用 invalidPair 单独覆盖
		"no_equals",   // 无等号
		"=value",      // 空 path
		"key=",        // 空 value
		"key=[bad",    // 非法 glob
		"key=,,",      // 全部空 glob
	}
	for _, expr := range bads {
		t.Run(expr, func(t *testing.T) {
			_, err := parseFilters([]string{expr})
			// 空字符串不算错（被跳过），但其他必须错
			if expr == "" {
				if err != nil {
					t.Fatalf("empty filter should be skipped, got err %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("expected error for %q", expr)
			}
		})
	}
}

func TestMatchAll_AND(t *testing.T) {
	ev := mustEvent(`{
		"id": 1,
		"received_at": 1700000000,
		"event": {"event":"story_create", "workspace_id":"20063271", "story":{"priority":"High"}}
	}`)

	rules, err := parseFilters([]string{
		"event.event=story_*",
		"event.workspace_id=20063271",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !matchAll(ev, rules) {
		t.Fatal("expected match (both rules satisfied)")
	}

	// 改 workspace_id 不在白名单
	rules2, _ := parseFilters([]string{
		"event.event=story_*",
		"event.workspace_id=99999999",
	})
	if matchAll(ev, rules2) {
		t.Fatal("expected NOT match (workspace_id rule fails)")
	}
}

func TestMatchAll_NoRulesPasses(t *testing.T) {
	ev := mustEvent(`{"id":1,"received_at":1,"event":{"x":1}}`)
	if !matchAll(ev, nil) {
		t.Fatal("no rules should pass")
	}
}

func TestMatchAll_MissingFieldFails(t *testing.T) {
	ev := mustEvent(`{"id":1,"received_at":1,"event":{"event":"story_create"}}`)
	rules, _ := parseFilters([]string{"event.workspace_id=20063271"})
	if matchAll(ev, rules) {
		t.Fatal("missing field should make rule fail")
	}
}

func TestMatchAll_ArrayIndex(t *testing.T) {
	ev := mustEvent(`{"id":1,"received_at":1,"event":{"assignees":["alice","bob"]}}`)
	rules, _ := parseFilters([]string{"event.assignees.1=bob"})
	if !matchAll(ev, rules) {
		t.Fatal("array index lookup failed")
	}
}

func TestMatchAll_ArrayContains(t *testing.T) {
	// 不带索引时数组的每个元素都是候选
	ev := mustEvent(`{"id":1,"received_at":1,"event":{"tags":["urgent","backend","p0"]}}`)
	rules, _ := parseFilters([]string{"event.tags=urgent"})
	if !matchAll(ev, rules) {
		t.Fatal("array element OR match failed")
	}
}

func TestMatchAll_NumericFieldStringified(t *testing.T) {
	ev := mustEvent(`{"id":42,"received_at":1700000000,"event":{"x":1}}`)
	rules, _ := parseFilters([]string{"id=42"})
	if !matchAll(ev, rules) {
		t.Fatal("numeric stringify match failed")
	}
}

func mustEvent(s string) *streamEvent {
	var ev streamEvent
	if err := json.Unmarshal([]byte(s), &ev); err != nil {
		panic(err)
	}
	return &ev
}

func equalSlice(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
