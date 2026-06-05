// Package cmd 中的 watch_filter.go 实现 watch 子命令的客户端过滤。
//
// 表达式格式：<点路径>=<glob>[,<glob>...]
//
//	event=story_create                单一精确匹配
//	event=story_*,bug_*               单 filter 内多值 = OR
//	workspace_id=20063271             非字符串字段会被转为字符串再做 glob 匹配
//	event.fields.priority=High        点路径深入嵌套对象
//
// CLI 上多个 --filter 之间是 AND 关系；单个 --filter 内多个 glob 是 OR 关系。
// 路径根永远是 watch emit 的 streamEvent 整体，因此可以匹配 id / received_at / event.* 任意字段。
package cmd

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
)

// filterRule 表示一条 path=glob1,glob2,... 规则。
type filterRule struct {
	raw   string   // 原始表达式，调试/出错时回显
	path  []string // 点路径分段
	globs []string // 至少一项；事件值匹配任一项即视为该规则成立
}

// parseWatchFilters 把 CLI 传入的多个 --filter 表达式解析成 rule 列表。
// 任何一条不合法直接报错，避免 watch 跑起来后才发现规则无效。
func parseWatchFilters(exprs []string) ([]filterRule, error) {
	rules := make([]filterRule, 0, len(exprs))
	for _, expr := range exprs {
		expr = strings.TrimSpace(expr)
		if expr == "" {
			continue
		}
		eq := strings.IndexByte(expr, '=')
		if eq <= 0 || eq == len(expr)-1 {
			return nil, fmt.Errorf("invalid filter %q: expected <path>=<glob>[,<glob>...]", expr)
		}
		pathStr := strings.TrimSpace(expr[:eq])
		valuesStr := strings.TrimSpace(expr[eq+1:])
		if pathStr == "" || valuesStr == "" {
			return nil, fmt.Errorf("invalid filter %q: empty path or value", expr)
		}
		globs := splitGlobs(valuesStr)
		if len(globs) == 0 {
			return nil, fmt.Errorf("invalid filter %q: no glob values", expr)
		}
		// 提前验证每个 glob 的语法
		for _, g := range globs {
			if _, err := filepath.Match(g, ""); err != nil {
				return nil, fmt.Errorf("invalid filter %q: bad glob %q: %v", expr, g, err)
			}
		}
		rules = append(rules, filterRule{
			raw:   expr,
			path:  strings.Split(pathStr, "."),
			globs: globs,
		})
	}
	return rules, nil
}

// splitGlobs 按逗号切分，支持 \, 转义；每段去掉前后空白。
func splitGlobs(s string) []string {
	var out []string
	var cur strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == '\\' && i+1 < len(s) && s[i+1] == ',' {
			cur.WriteByte(',')
			i++
			continue
		}
		if c == ',' {
			out = appendNonEmpty(out, cur.String())
			cur.Reset()
			continue
		}
		cur.WriteByte(c)
	}
	out = appendNonEmpty(out, cur.String())
	return out
}

func appendNonEmpty(ss []string, s string) []string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ss
	}
	return append(ss, s)
}

// matchAll 检查事件是否满足全部规则；任一规则失败即整体失败。
// rules 为空时直接放行，调用方无需自己判空。
func matchAll(ev *streamEvent, rules []filterRule) bool {
	if len(rules) == 0 {
		return true
	}
	root := streamEventToMap(ev)
	for _, r := range rules {
		if !matchOne(root, r) {
			return false
		}
	}
	return true
}

// matchOne 取出 rule.path 指向的字段，转为字符串后用 OR 语义匹配 globs。
func matchOne(root interface{}, r filterRule) bool {
	val, ok := lookupJSON(root, r.path)
	if !ok {
		return false
	}
	candidates := stringify(val)
	for _, cand := range candidates {
		for _, g := range r.globs {
			matched, err := filepath.Match(g, cand)
			if err == nil && matched {
				return true
			}
		}
	}
	return false
}

// streamEventToMap 把 streamEvent 序列化回 map[string]interface{}，
// 这样 path "event.story.id" 这种嵌套查找有统一的根。
func streamEventToMap(ev *streamEvent) map[string]interface{} {
	// 直接用 json 来回一道是最稳的——保证所有字段访问规则与用户看到的 JSON 一致。
	raw, err := json.Marshal(ev)
	if err != nil {
		return nil
	}
	var m map[string]interface{}
	_ = json.Unmarshal(raw, &m)
	return m
}

// lookupJSON 顺着 path 在 JSON 树里取值；任一段不存在或类型不匹配返回 (nil, false)。
// 数组段支持纯数字下标（如 path "event.assignees.0"）。
func lookupJSON(node interface{}, path []string) (interface{}, bool) {
	cur := node
	for _, seg := range path {
		switch v := cur.(type) {
		case map[string]interface{}:
			next, ok := v[seg]
			if !ok {
				return nil, false
			}
			cur = next
		case []interface{}:
			idx, err := strconv.Atoi(seg)
			if err != nil || idx < 0 || idx >= len(v) {
				return nil, false
			}
			cur = v[idx]
		default:
			return nil, false
		}
	}
	return cur, true
}

// stringify 把 JSON 值转成字符串候选列表，供 glob 匹配使用。
//   - 字符串：原样
//   - 数字 / 布尔：fmt.Sprint
//   - 数组：每个元素递归（顶层 OR 即可命中）
//   - 对象：取 JSON 序列化整体作为候选（用户多半不会对对象写 glob，但避免静默匹配失败）
//   - nil：返回 ["null"]，让用户能写 path=null 显式判空
func stringify(v interface{}) []string {
	switch x := v.(type) {
	case nil:
		return []string{"null"}
	case string:
		return []string{x}
	case bool:
		if x {
			return []string{"true"}
		}
		return []string{"false"}
	case float64:
		// JSON 数字默认会 unmarshal 成 float64
		if x == float64(int64(x)) {
			return []string{strconv.FormatInt(int64(x), 10)}
		}
		return []string{strconv.FormatFloat(x, 'f', -1, 64)}
	case []interface{}:
		out := make([]string, 0, len(x))
		for _, e := range x {
			out = append(out, stringify(e)...)
		}
		return out
	case map[string]interface{}:
		raw, _ := json.Marshal(x)
		return []string{string(raw)}
	default:
		return []string{fmt.Sprint(v)}
	}
}
