package cmd

import (
	"os"
	"path/filepath"
	"testing"
)

// TestInjectLastID 覆盖三种典型场景:无水位、空 query、已存在 last_id 被覆盖。
func TestInjectLastID(t *testing.T) {
	cases := []struct {
		name     string
		endpoint string
		lastID   uint64
		want     string
	}{
		{
			name:     "zero id keeps url unchanged",
			endpoint: "https://example.com/x/upower/tapd/events",
			lastID:   0,
			want:     "https://example.com/x/upower/tapd/events",
		},
		{
			name:     "appends last_id to bare url",
			endpoint: "https://example.com/x/upower/tapd/events",
			lastID:   42,
			want:     "https://example.com/x/upower/tapd/events?last_id=42",
		},
		{
			name:     "preserves existing query",
			endpoint: "https://example.com/x/upower/tapd/events?token=abc",
			lastID:   7,
			want:     "https://example.com/x/upower/tapd/events?last_id=7&token=abc",
		},
		{
			name:     "overrides stale last_id",
			endpoint: "https://example.com/x/upower/tapd/events?last_id=1",
			lastID:   100,
			want:     "https://example.com/x/upower/tapd/events?last_id=100",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := injectLastID(tc.endpoint, tc.lastID)
			if err != nil {
				t.Fatalf("injectLastID err: %v", err)
			}
			if got != tc.want {
				t.Fatalf("got %q want %q", got, tc.want)
			}
		})
	}
}

// TestWatchStatePersist 验证 newWatchState 能读到上次写的 last_event_id,
// Update 跳过非递增值。
func TestWatchStatePersist(t *testing.T) {
	dir := t.TempDir()
	t.Setenv(watchStateDirEnv, dir)

	s := newWatchState()
	if s.LastSeen() != 0 {
		t.Fatalf("fresh state should be 0, got %d", s.LastSeen())
	}
	s.Update(10)
	s.Update(5) // 非递增,应被忽略
	s.Update(20)
	if got := s.LastSeen(); got != 20 {
		t.Fatalf("LastSeen=%d want 20", got)
	}

	raw, err := os.ReadFile(filepath.Join(dir, watchStateFile))
	if err != nil {
		t.Fatalf("read state file err: %v", err)
	}
	if string(raw) != "20" {
		t.Fatalf("state file content=%q want 20", raw)
	}

	s2 := newWatchState()
	if got := s2.LastSeen(); got != 20 {
		t.Fatalf("reloaded LastSeen=%d want 20", got)
	}
}
