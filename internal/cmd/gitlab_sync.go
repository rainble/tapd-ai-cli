// Package cmd 中的 gitlab_sync.go 实现 TAPD 事件到 GitLab Issue 的同步逻辑。
package cmd

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/studyzy/tapd-ai-cli/internal/gitlab"
	"github.com/studyzy/tapd-sdk-go/model"
)

func parseGitLabSyncTypes(raw string) map[string]bool {
	if strings.TrimSpace(raw) == "" {
		raw = "story,bug"
	}
	out := map[string]bool{}
	for _, part := range strings.Split(raw, ",") {
		switch strings.TrimSpace(part) {
		case "story":
			out["story"] = true
		case "bug":
			out["bug"] = true
		}
	}
	return out
}

func handleGitLabIssueSyncEvent(ctx context.Context, data string, cfg gitLabSyncConfig) (bool, error) {
	var ev streamEvent
	if err := json.Unmarshal([]byte(data), &ev); err != nil {
		fmt.Fprintf(os.Stderr, "gitlab sync-watch: invalid event json: %v\n", err)
		return false, nil
	}
	target, ok, reason := extractGitLabSyncTarget(&ev)
	if !ok {
		fmt.Fprintf(os.Stderr, "gitlab sync-watch: skip event id=%d reason=%s\n", ev.ID, reason)
		return false, nil
	}
	if len(cfg.types) > 0 && !cfg.types[target.EntityType] {
		return false, nil
	}
	if cfg.workspaceID != "" && target.WorkspaceID != cfg.workspaceID {
		return false, nil
	}
	snapshot, err := loadGitLabSyncSnapshot(ctx, target)
	if err != nil {
		return false, err
	}
	if !snapshot.Ready {
		fmt.Fprintf(os.Stderr, "gitlab sync-watch: skip %s %s reason=not_ready\n", target.EntityType, target.EntityID)
		return false, nil
	}
	marker, hasMarker, err := findGitLabSyncMarker(ctx, snapshot, cfg.options.project)
	if err != nil {
		return false, err
	}
	return syncGitLabSnapshot(ctx, cfg.options, snapshot, marker, hasMarker)
}

func syncGitLabSnapshot(
	ctx context.Context,
	opts gitLabOptions,
	snapshot gitLabIssueSnapshot,
	marker gitLabSyncMarker,
	hasMarker bool,
) (bool, error) {
	client := gitlab.NewClient(opts.baseURL, opts.token)
	if !hasMarker {
		issue, err := client.CreateIssue(ctx, opts.project, gitlab.CreateIssueRequest{
			Title:       snapshot.Title,
			Description: snapshot.Description,
		})
		if err != nil {
			return false, err
		}
		newMarker := gitLabSyncMarker{
			Type:        snapshot.EntityType,
			ID:          snapshot.EntityID,
			Project:     opts.project,
			IssueIID:    issue.IID,
			Fingerprint: snapshot.Fingerprint,
		}
		return true, addGitLabSyncComment(ctx, snapshot, issue.WebURL, newMarker)
	}
	if marker.Fingerprint == snapshot.Fingerprint {
		return false, nil
	}
	if _, err := client.CreateIssueNote(ctx, opts.project, marker.IssueIID, snapshot.Description); err != nil {
		return false, err
	}
	marker.Fingerprint = snapshot.Fingerprint
	return true, addGitLabSyncComment(ctx, snapshot, gitLabIssueWebURL(opts, marker.IssueIID), marker)
}

func gitLabIssueWebURL(opts gitLabOptions, issueIID int) string {
	return fmt.Sprintf("%s/%s/-/issues/%d", strings.TrimRight(opts.baseURL, "/"), opts.project, issueIID)
}

func loadGitLabSyncSnapshot(ctx context.Context, target gitLabSyncTarget) (gitLabIssueSnapshot, error) {
	switch target.EntityType {
	case "story":
		story, err := apiClient.GetStory(ctx, target.WorkspaceID, target.EntityID)
		if err != nil {
			return gitLabIssueSnapshot{}, err
		}
		if story.WorkspaceID == "" {
			story.WorkspaceID = target.WorkspaceID
		}
		return buildGitLabIssueFromStory(story), nil
	case "bug":
		bug, err := apiClient.GetBug(ctx, target.WorkspaceID, target.EntityID)
		if err != nil {
			return gitLabIssueSnapshot{}, err
		}
		if bug.WorkspaceID == "" {
			bug.WorkspaceID = target.WorkspaceID
		}
		return buildGitLabIssueFromBug(bug), nil
	default:
		return gitLabIssueSnapshot{}, fmt.Errorf("unsupported entity type %s", target.EntityType)
	}
}

func findGitLabSyncMarker(ctx context.Context, snapshot gitLabIssueSnapshot, project string) (gitLabSyncMarker, bool, error) {
	entryType := "stories"
	if snapshot.EntityType == "bug" {
		entryType = "bug"
	}
	comments, err := apiClient.ListComments(ctx, &model.ListCommentsRequest{
		WorkspaceID: snapshot.WorkspaceID,
		EntryType:   entryType,
		EntryID:     snapshot.EntityID,
		Limit:       50,
		Order:       "created desc",
	})
	if err != nil {
		return gitLabSyncMarker{}, false, err
	}
	for _, comment := range comments {
		if marker, ok := parseGitLabSyncMarker(comment.Description, snapshot.EntityType, snapshot.EntityID, project); ok {
			return marker, true, nil
		}
	}
	return gitLabSyncMarker{}, false, nil
}

func extractGitLabSyncTarget(ev *streamEvent) (gitLabSyncTarget, bool, string) {
	if ev == nil {
		return gitLabSyncTarget{}, false, "nil_event"
	}
	var payload map[string]interface{}
	decoder := json.NewDecoder(strings.NewReader(string(ev.Event)))
	decoder.UseNumber()
	if err := decoder.Decode(&payload); err != nil {
		return gitLabSyncTarget{}, false, "invalid_payload"
	}
	entityType, ok := gitLabEntityTypeFromEvent(stringifyBugEventValue(payload["event"]))
	if !ok {
		return gitLabSyncTarget{}, false, "unsupported_event"
	}
	workspaceID := stringifyBugEventValue(payload["workspace_id"])
	if workspaceID == "" {
		return gitLabSyncTarget{}, false, "missing_workspace_id"
	}
	entityID := firstBugEventPathString(payload, [][]string{
		{entityType, "id"},
		{"object", "id"},
		{"id"},
		{"data", entityType, "id"},
		{"data", "id"},
	})
	if entityID == "" {
		return gitLabSyncTarget{}, false, "missing_entity_id"
	}
	return gitLabSyncTarget{
		EntityType:  entityType,
		WorkspaceID: workspaceID,
		EntityID:    entityID,
		EventID:     ev.ID,
	}, true, ""
}

func gitLabEntityTypeFromEvent(eventName string) (string, bool) {
	switch eventName {
	case "story::create", "story::update", "story_create", "story_update":
		return "story", true
	case "bug::create", "bug::update", "bug_create", "bug_update":
		return "bug", true
	default:
		return "", false
	}
}

func streamGitLabIssueSync(ctx context.Context, cfg gitLabSyncConfig) {
	const minBackoff = time.Second
	const maxBackoff = 30 * time.Second
	backoff := minBackoff
	for {
		err := streamGitLabIssueSyncOnce(ctx, cfg)
		if ctx.Err() != nil {
			return
		}
		if err != nil {
			fmt.Fprintf(os.Stderr, "gitlab sync-watch: connection lost: %v; reconnect in %s\n", err, backoff)
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(backoff):
		}
		backoff *= 2
		if backoff > maxBackoff {
			backoff = maxBackoff
		}
	}
}

func streamGitLabIssueSyncOnce(ctx context.Context, cfg gitLabSyncConfig) error {
	target, err := injectLastID(cfg.endpoint, watchStateRef.LastSeen())
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "text/event-stream")
	if cfg.token != "" {
		req.Header.Set("X-TAPD-Token", cfg.token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<14))
		return fmt.Errorf("status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return readGitLabIssueSyncSSE(ctx, resp.Body, cfg)
}

func readGitLabIssueSyncSSE(ctx context.Context, r io.Reader, cfg gitLabSyncConfig) error {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 64*1024), 4*1024*1024)
	var dataLines []string
	flush := func() error {
		if len(dataLines) == 0 {
			return nil
		}
		data := strings.Join(dataLines, "\n")
		dataLines = dataLines[:0]
		handled, err := handleGitLabIssueSyncEvent(ctx, data, cfg)
		if err == nil && watchStateRef != nil {
			updateGitLabSyncWatermark(data, handled)
		}
		return err
	}
	for scanner.Scan() {
		line := scanner.Text()
		switch {
		case line == "":
			if err := flush(); err != nil {
				return err
			}
		case strings.HasPrefix(line, ":"):
		case strings.HasPrefix(line, "data:"):
			dataLines = append(dataLines, strings.TrimSpace(strings.TrimPrefix(line, "data:")))
		}
	}
	if err := flush(); err != nil {
		return err
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	return io.EOF
}

func updateGitLabSyncWatermark(data string, handled bool) {
	var ev streamEvent
	if json.Unmarshal([]byte(data), &ev) == nil && (handled || ev.ID > 0) {
		watchStateRef.Update(ev.ID)
	}
}
