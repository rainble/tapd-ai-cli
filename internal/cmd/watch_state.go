// Package cmd 中的 watch_state.go 负责把 watch 已消费的最大事件 ID 持久化到本地，
// 让 watch 进程重启后能在 SSE 订阅请求里带上 ?last_id=N，
// 让服务端 replay 兜底跳过那些已经处理过的历史事件，避免重启重复消费。
//
// 文件位置默认 ~/.tapd-ai-cli/last_event_id；用 os.Rename 做原子覆盖，
// 防止写入过程中崩溃导致读到半截文件。
package cmd

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
)

// watchStateDirEnv 允许测试或多 watch 并行时覆盖默认目录。
const watchStateDirEnv = "TAPD_WATCH_STATE_DIR"

// watchStateFile 是 last_event_id 文件名（位于 stateDir 下）。
const watchStateFile = "last_event_id"

// watchState 维护 watch 进程在内存里的最后 ID,并在变化时刷盘。
// path 为空表示禁用持久化（解析路径失败时降级为内存模式）。
type watchState struct {
	mu   sync.Mutex
	path string
	last uint64
}

// newWatchState 解析持久化路径并加载已有水位；解析或读取失败均降级为内存模式。
// 调用方拿到的对象始终非 nil，可以无脑用。
func newWatchState() *watchState {
	s := &watchState{}
	dir, err := watchStateDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "watch: cannot resolve state dir: %v; running without persistence\n", err)
		return s
	}
	s.path = filepath.Join(dir, watchStateFile)
	if v, err := readLastID(s.path); err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			fmt.Fprintf(os.Stderr, "watch: read last_event_id err: %v; starting from 0\n", err)
		}
	} else {
		s.last = v
	}
	return s
}

// LastSeen 返回当前已知的最大事件 ID;0 表示尚无水位。
func (s *watchState) LastSeen() uint64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.last
}

// Update 在 ID 单调推进时把新水位写盘;非递增直接忽略。
// 写盘失败只 warn，不阻断 watch 主循环。
func (s *watchState) Update(id uint64) {
	if id == 0 {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if id <= s.last {
		return
	}
	s.last = id
	if s.path == "" {
		return
	}
	if err := writeLastID(s.path, id); err != nil {
		fmt.Fprintf(os.Stderr, "watch: persist last_event_id err: %v\n", err)
	}
}

// watchStateDir 解析持久化目录。优先用 TAPD_WATCH_STATE_DIR 环境变量,
// 其次 ~/.tapd-ai-cli;两者都失败则返回错误。
func watchStateDir() (string, error) {
	if v := strings.TrimSpace(os.Getenv(watchStateDirEnv)); v != "" {
		if err := os.MkdirAll(v, 0o755); err != nil {
			return "", err
		}
		return v, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, ".tapd-ai-cli")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return dir, nil
}

// readLastID 读取并解析 last_event_id 文件,文件不存在返回 os.ErrNotExist。
func readLastID(path string) (uint64, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	s := strings.TrimSpace(string(raw))
	if s == "" {
		return 0, nil
	}
	return strconv.ParseUint(s, 10, 64)
}

// writeLastID 用 tmp + Rename 原子写,避免写一半被读到。
func writeLastID(path string, id uint64) error {
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, []byte(strconv.FormatUint(id, 10)), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
