// Package cmd 中的 watch_events.go 负责把 watch 收到的事件持久化到本地滚动文件,
// 供 tapd mcp 读取后暴露给 AI。
//
// 文件位置 ~/.tapd-ai-cli/events.jsonl,每行一条事件 JSON(newline-delimited JSON)。
// 保留最近 100 条,超出后从头截断(FIFO)。
//
// 和 last_event_id 的区别:
// - last_event_id:单个数字,仅用于 watch 重启时的去重水位
// - events.jsonl:完整事件流,供 MCP server 暴露给 AI,让 AI 感知项目动态
package cmd

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

const (
	eventsFile      = "events.jsonl"
	eventsCapacity  = 100 // 保留最近 N 条
	eventsFilePerms = 0o644
)

// eventCache 维护本地事件缓存文件,支持 append(滚动窗口)和 read(全量读)。
type eventCache struct {
	mu   sync.Mutex
	path string
}

// newEventCache 构造事件缓存,path 为空表示禁用持久化(内存模式)。
func newEventCache() *eventCache {
	dir, err := watchStateDir() // 复用 watch_state.go 里的目录解析逻辑
	if err != nil {
		return &eventCache{} // 降级为内存模式
	}
	return &eventCache{path: filepath.Join(dir, eventsFile)}
}

// Append 把一条事件追加到缓存;满了就截断头部保留最近 N 条。
// 失败只 warn 不阻断 watch 主循环。
func (c *eventCache) Append(ev *streamEvent) {
	if c.path == "" {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()

	// 读现有全部事件
	existing, _ := c.readUnlocked()

	// 追加新事件
	existing = append(existing, ev)

	// 超出容量时截断头部
	if len(existing) > eventsCapacity {
		existing = existing[len(existing)-eventsCapacity:]
	}

	// 重写整个文件(原子写用 tmp+rename)
	if err := c.writeUnlocked(existing); err != nil {
		// 失败只打 stderr,不阻断主流程
		return
	}
}

// ReadAll 读取缓存里全部事件,按时间升序(最老的在前)。
func (c *eventCache) ReadAll() ([]*streamEvent, error) {
	if c.path == "" {
		return nil, nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.readUnlocked()
}

// readUnlocked 内部读取逻辑,调用方需持锁。
func (c *eventCache) readUnlocked() ([]*streamEvent, error) {
	f, err := os.Open(c.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // 文件不存在等价于空缓存
		}
		return nil, err
	}
	defer f.Close()

	var events []*streamEvent
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var ev streamEvent
		if err := json.Unmarshal(scanner.Bytes(), &ev); err != nil {
			continue // 单行损坏不影响其他行
		}
		events = append(events, &ev)
	}
	if err := scanner.Err(); err != nil {
		return events, err // 返回已解析的部分
	}
	return events, nil
}

// writeUnlocked 原子覆盖文件,调用方需持锁。
func (c *eventCache) writeUnlocked(events []*streamEvent) error {
	tmp := c.path + ".tmp"
	f, err := os.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, eventsFilePerms)
	if err != nil {
		return err
	}
	for _, ev := range events {
		line, err := json.Marshal(ev)
		if err != nil {
			f.Close()
			os.Remove(tmp)
			return err
		}
		if _, err := f.Write(append(line, '\n')); err != nil {
			f.Close()
			os.Remove(tmp)
			return err
		}
	}
	if err := f.Close(); err != nil {
		os.Remove(tmp)
		return err
	}
	return os.Rename(tmp, c.path)
}
