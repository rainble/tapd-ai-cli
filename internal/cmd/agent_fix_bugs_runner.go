package cmd

import (
	"context"
	"fmt"
	"sync"
)

// bugFixTapdClient 抽象 TAPD 读写，生产实现包装 SDK，测试使用 fake。
type bugFixTapdClient interface {
	GetBugDetail(ctx context.Context, workspaceID, bugID string) (bugFixBugDetail, error)
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
}

func (w *bugFixWorker) handleTarget(ctx context.Context, target bugEventTarget) bugFixResult {
	w.mu.Lock()
	defer w.mu.Unlock()

	result := bugFixResult{WorkspaceID: target.WorkspaceID, BugID: target.BugID, EventID: target.EventID}
	limit := w.outputLimit
	if limit <= 0 {
		limit = 12288
	}

	bug, err := w.tapd.GetBugDetail(ctx, target.WorkspaceID, target.BugID)
	if err != nil {
		return w.fail(ctx, result, "bug_show", err.Error())
	}

	if !w.allowDirty {
		dirty, out, err := gitWorkingTreeDirty(ctx, w.runner, w.repo)
		if err != nil {
			return w.fail(ctx, result, "dirty_check", err.Error())
		}
		if dirty {
			return w.fail(ctx, result, "dirty_repo", out)
		}
	}

	if w.onStartStatus != "" {
		_ = w.tapd.UpdateBugStatus(ctx, bugStatusUpdate{
			WorkspaceID: target.WorkspaceID,
			BugID:       target.BugID,
			Status:      w.onStartStatus,
			CurrentUser: w.currentUser,
		})
	}

	prompt := buildAgentPrompt(bug, w.repo, w.testCmd)
	agent := w.runner.Run(ctx, commandRunConfig{Dir: w.repo, Command: w.agentCmd, Stdin: prompt, Limit: limit})
	if agent.Err != nil || agent.ExitCode != 0 {
		return w.fail(ctx, result, "agent", fmt.Sprintf("stdout:\n%s\n\nstderr:\n%s", agent.Stdout, agent.Stderr))
	}

	test := commandRunResult{}
	verified := false
	if w.testCmd != "" {
		test = w.runner.Run(ctx, commandRunConfig{Dir: w.repo, Command: w.testCmd, Limit: limit})
		if test.Err != nil || test.ExitCode != 0 {
			return w.fail(ctx, result, "test", fmt.Sprintf("stdout:\n%s\n\nstderr:\n%s", test.Stdout, test.Stderr))
		}
		verified = true
	}

	comment := buildSuccessComment(agent, test, verified)
	if err := w.tapd.AddBugComment(ctx, target.WorkspaceID, target.BugID, comment); err != nil {
		return w.failNoComment(result, "comment", err.Error())
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
			result.Detail = err.Error()
			return result
		}
	}

	result.Status = "success"
	result.Verified = verified
	return result
}

func (w *bugFixWorker) fail(ctx context.Context, base bugFixResult, stage, detail string) bugFixResult {
	comment := buildFailureComment(stage, detail)
	_ = w.tapd.AddBugComment(ctx, base.WorkspaceID, base.BugID, comment)
	if w.onFailureStatus != "" {
		_ = w.tapd.UpdateBugStatus(ctx, bugStatusUpdate{
			WorkspaceID: base.WorkspaceID,
			BugID:       base.BugID,
			Status:      w.onFailureStatus,
			CurrentUser: w.currentUser,
		})
	}
	return w.failNoComment(base, stage, detail)
}

func (w *bugFixWorker) failNoComment(base bugFixResult, stage, detail string) bugFixResult {
	base.Status = "failed"
	base.Stage = stage
	base.Detail = truncateOutput(detail, w.outputLimit)
	return base
}

func fallbackString(v, fallback string) string {
	if v != "" {
		return v
	}
	return fallback
}
