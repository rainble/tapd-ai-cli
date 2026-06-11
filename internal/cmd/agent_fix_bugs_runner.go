package cmd

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/studyzy/tapd-sdk-go/model"
)

// bugFixTapdClient 抽象 TAPD 读写，生产实现包装 SDK，测试使用 fake。
type bugFixTapdClient interface {
	GetBugDetail(ctx context.Context, workspaceID, bugID string) (bugFixBugDetail, error)
	GetStoryDetail(ctx context.Context, workspaceID, storyID string) (bugFixStoryDetail, error)
	AddBugComment(ctx context.Context, workspaceID, bugID, description string) error
	UpdateBugStatus(ctx context.Context, update bugStatusUpdate) error
}

// bugStatusUpdate 描述一次 TAPD 缺陷状态流转。
type bugStatusUpdate struct {
	WorkspaceID string
	BugID       string
	Status      string
	CurrentUser string
	Resolution  string
}

// bugFixResult 是 stdout 输出的一行机器可读处理结果。
type bugFixResult struct {
	WorkspaceID string `json:"workspace_id"`
	BugID       string `json:"bug_id"`
	EventID     uint64 `json:"event_id"`
	Status      string `json:"status"`
	Stage       string `json:"stage,omitempty"`
	Verified    bool   `json:"verified"`
	Detail      string `json:"detail,omitempty"`
}

// bugFixWorker 负责单个缺陷的本地修复流程。
type bugFixWorker struct {
	mu              sync.Mutex
	tapd            bugFixTapdClient
	runner          commandRunner
	repo            string
	agentCmd        string
	testCmd         string
	onStartStatus   string
	onSuccessStatus string
	onFailureStatus string
	currentUser     string
	resolution      string
	allowDirty      bool
	outputLimit     int
	branchStrategy  string
	mrRemote        string
	mrBranchPrefix  string
}

func (w *bugFixWorker) handleTarget(ctx context.Context, target bugEventTarget) bugFixResult {
	w.mu.Lock()
	defer w.mu.Unlock()

	result := bugFixResult{WorkspaceID: target.WorkspaceID, BugID: target.BugID, EventID: target.EventID}
	limit := w.normalizedOutputLimit()

	bug, err := w.tapd.GetBugDetail(ctx, target.WorkspaceID, target.BugID)
	if err != nil {
		return w.fail(ctx, result, "bug_show", err.Error(), limit)
	}
	if skipped, stage, detail := w.shouldSkipBug(bug); skipped {
		return w.skip(result, stage, detail, limit)
	}

	if !w.allowDirty {
		dirty, out, err := gitWorkingTreeDirty(ctx, w.runner, w.repo, limit)
		if err != nil {
			return w.fail(ctx, result, "dirty_check", err.Error(), limit)
		}
		if dirty {
			return w.fail(ctx, result, "dirty_repo", out, limit)
		}
	}

	branch, err := w.prepareBranch(ctx, bug, limit)
	if err != nil {
		return w.fail(ctx, result, "branch_prepare", err.Error(), limit)
	}

	if w.onStartStatus != "" {
		err := w.tapd.UpdateBugStatus(ctx, bugStatusUpdate{
			WorkspaceID: target.WorkspaceID,
			BugID:       target.BugID,
			Status:      w.onStartStatus,
			CurrentUser: w.currentUser,
		})
		if err != nil {
			return w.failNoComment(result, "status_update", statusUpdateFailureDetail(w.onStartStatus, err, "", ""), limit)
		}
	}

	prompt := buildAgentPrompt(bug, w.repo, w.testCmd)
	agent := w.runner.Run(ctx, commandRunConfig{Dir: w.repo, Command: w.agentCmd, Stdin: prompt, Limit: limit})
	if agent.Err != nil || agent.ExitCode != 0 {
		return w.fail(ctx, result, "agent", fmt.Sprintf("stdout:\n%s\n\nstderr:\n%s", agent.Stdout, agent.Stderr), limit)
	}

	test := commandRunResult{}
	verified := false
	if w.testCmd != "" {
		test = w.runner.Run(ctx, commandRunConfig{Dir: w.repo, Command: w.testCmd, Limit: limit})
		if test.Err != nil || test.ExitCode != 0 {
			return w.fail(ctx, result, "test", fmt.Sprintf("stdout:\n%s\n\nstderr:\n%s", test.Stdout, test.Stderr), limit)
		}
		verified = true
	}

	comment := buildSuccessComment(agent, test, verified, branch)
	if err := w.tapd.AddBugComment(ctx, target.WorkspaceID, target.BugID, comment); err != nil {
		result.Verified = verified
		if statusErr := w.updateFailureStatus(ctx, result); statusErr != nil {
			return w.failNoComment(result, "status_update", statusUpdateFailureDetail(w.onFailureStatus, statusErr, "comment", err.Error()), limit)
		}
		return w.failNoComment(result, "comment", err.Error(), limit)
	}

	if w.onSuccessStatus != "" {
		err := w.tapd.UpdateBugStatus(ctx, bugStatusUpdate{
			WorkspaceID: target.WorkspaceID,
			BugID:       target.BugID,
			Status:      w.onSuccessStatus,
			CurrentUser: w.currentUser,
			Resolution:  fallbackString(w.resolution, "fixed"),
		})
		if err != nil {
			result.Status = "failed"
			result.Stage = "status_update"
			result.Verified = verified
			result.Detail = truncateOutput(statusUpdateFailureDetail(w.onSuccessStatus, err, "", ""), limit)
			return result
		}
	}

	result.Status = "success"
	result.Verified = verified
	return result
}

func (w *bugFixWorker) shouldSkipBug(bug bugFixBugDetail) (bool, string, string) {
	if strings.TrimSpace(bug.Description) == "" {
		return true, "empty_content", "bug description is empty"
	}
	if bugHasAgentProcessedComment(bug.Comments) {
		return true, "already_processed", "bug already has an AI agent result comment"
	}
	user := strings.TrimSpace(w.currentUser)
	if user == "" && strings.TrimSpace(bug.CurrentOwner) != "" {
		return true, "owner_unknown", fmt.Sprintf("bug current_owner is %q, but current user is unknown", bug.CurrentOwner)
	}
	if user != "" && !ownerContainsUser(bug.CurrentOwner, user) {
		return true, "owner_mismatch", fmt.Sprintf("bug current_owner %q does not contain current user %q", bug.CurrentOwner, user)
	}
	return false, "", ""
}

func bugHasAgentProcessedComment(comments []bugFixComment) bool {
	for _, comment := range comments {
		if strings.Contains(comment.Description, "AI agent run completed.") ||
			strings.Contains(comment.Description, "AI agent bug fix failed.") {
			return true
		}
	}
	return false
}

func ownerContainsUser(currentOwner, currentUser string) bool {
	currentUser = strings.TrimSpace(currentUser)
	if currentUser == "" {
		return true
	}
	for _, owner := range strings.FieldsFunc(currentOwner, func(r rune) bool {
		return r == ';' || r == ',' || r == '，' || r == '、' || r == ' ' || r == '\t' || r == '\n' || r == '\r'
	}) {
		if strings.TrimSpace(owner) == currentUser {
			return true
		}
	}
	return false
}

func (w *bugFixWorker) fail(ctx context.Context, base bugFixResult, stage, detail string, limit int) bugFixResult {
	comment := buildFailureComment(stage, detail)
	_ = w.tapd.AddBugComment(ctx, base.WorkspaceID, base.BugID, comment)
	if err := w.updateFailureStatus(ctx, base); err != nil {
		return w.failNoComment(base, "status_update", statusUpdateFailureDetail(w.onFailureStatus, err, stage, detail), limit)
	}
	return w.failNoComment(base, stage, detail, limit)
}

func (w *bugFixWorker) skip(base bugFixResult, stage, detail string, limit int) bugFixResult {
	base.Status = "skipped"
	base.Stage = stage
	base.Detail = truncateOutput(detail, limit)
	return base
}

func (w *bugFixWorker) updateFailureStatus(ctx context.Context, base bugFixResult) error {
	if w.onFailureStatus == "" {
		return nil
	}
	return w.tapd.UpdateBugStatus(ctx, bugStatusUpdate{
		WorkspaceID: base.WorkspaceID,
		BugID:       base.BugID,
		Status:      w.onFailureStatus,
		CurrentUser: w.currentUser,
	})
}

func (w *bugFixWorker) failNoComment(base bugFixResult, stage, detail string, limit int) bugFixResult {
	base.Status = "failed"
	base.Stage = stage
	base.Detail = truncateOutput(detail, limit)
	return base
}

func (w *bugFixWorker) prepareBranch(ctx context.Context, bug bugFixBugDetail, limit int) (*bugFixBranchContext, error) {
	strategy := fallbackString(w.branchStrategy, "current")
	switch strategy {
	case "current":
		return nil, nil
	case "linked-mr":
		return prepareLinkedMRBranch(ctx, w.tapd, w.runner, w.repo, bug, bugFixBranchOptions{
			Remote:       fallbackString(w.mrRemote, "origin"),
			BranchPrefix: fallbackString(w.mrBranchPrefix, "tapd-agent/mr-"),
			Limit:        limit,
		})
	default:
		return nil, fmt.Errorf("unsupported branch strategy %q", strategy)
	}
}

func (w *bugFixWorker) normalizedOutputLimit() int {
	if w.outputLimit > 0 {
		return w.outputLimit
	}
	return defaultCommandOutputLimit
}

func fallbackString(v, fallback string) string {
	if v != "" {
		return v
	}
	return fallback
}

func statusUpdateFailureDetail(status string, err error, originalStage, originalDetail string) string {
	detail := fmt.Sprintf("status %q update failed: %s", status, err)
	if originalStage != "" || originalDetail != "" {
		detail += fmt.Sprintf("\n\nOriginal stage: %s\n\nOriginal detail:\n%s", originalStage, originalDetail)
	}
	return detail
}

// sdkBugFixTapdClient 把 tapd-sdk-go Client 适配成 bugFixTapdClient。
type sdkBugFixTapdClient struct{}

func (sdkBugFixTapdClient) GetBugDetail(ctx context.Context, workspaceID, bugID string) (bugFixBugDetail, error) {
	bug, err := apiClient.GetBug(ctx, workspaceID, bugID)
	if err != nil {
		return bugFixBugDetail{}, err
	}
	comments, _ := apiClient.ListComments(ctx, &model.ListCommentsRequest{
		WorkspaceID: workspaceID,
		EntryType:   "bug",
		EntryID:     bugID,
	})
	mapped := bugFixBugDetail{
		WorkspaceID:  workspaceID,
		ID:           bug.ID,
		Title:        bug.Title,
		StoryID:      bug.StoryID,
		Status:       bug.Status,
		CurrentOwner: bug.CurrentOwner,
		Severity:     bug.Severity,
		Priority:     bug.PriorityLabel,
		Description:  htmlToMarkdown(bug.Description),
	}
	for _, c := range comments {
		mapped.Comments = append(mapped.Comments, bugFixComment{
			Author:      c.Author,
			Created:     c.Created,
			Description: htmlToMarkdown(c.Description),
		})
	}
	return mapped, nil
}

func (sdkBugFixTapdClient) GetStoryDetail(ctx context.Context, workspaceID, storyID string) (bugFixStoryDetail, error) {
	story, err := apiClient.GetStory(ctx, workspaceID, storyID)
	if err != nil {
		return bugFixStoryDetail{}, err
	}
	comments, _ := apiClient.ListComments(ctx, &model.ListCommentsRequest{
		WorkspaceID: workspaceID,
		EntryType:   "stories",
		EntryID:     storyID,
	})
	mapped := bugFixStoryDetail{
		WorkspaceID: workspaceID,
		ID:          story.ID,
		ParentID:    story.ParentID,
		Title:       story.Name,
		Description: htmlToMarkdown(story.Description),
	}
	for _, c := range comments {
		mapped.Comments = append(mapped.Comments, bugFixComment{
			Author:      c.Author,
			Created:     c.Created,
			Description: htmlToMarkdown(c.Description),
		})
	}
	return mapped, nil
}

func (sdkBugFixTapdClient) AddBugComment(ctx context.Context, workspaceID, bugID, description string) error {
	author := ensureNick()
	_, err := apiClient.AddComment(ctx, &model.AddCommentRequest{
		WorkspaceID: workspaceID,
		EntryType:   "bug",
		EntryID:     bugID,
		Description: description,
		Author:      author,
	})
	return err
}

func (sdkBugFixTapdClient) UpdateBugStatus(ctx context.Context, update bugStatusUpdate) error {
	currentUser := update.CurrentUser
	if currentUser == "" {
		currentUser = ensureNick()
	}
	_, err := apiClient.UpdateBug(ctx, &model.UpdateBugRequest{
		WorkspaceID: update.WorkspaceID,
		ID:          update.BugID,
		VStatus:     update.Status,
		CurrentUser: currentUser,
		Resolution:  update.Resolution,
	})
	return err
}
