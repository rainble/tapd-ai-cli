package cmd

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/studyzy/tapd-ai-cli/internal/output"
	"github.com/studyzy/tapd-sdk-go/model"
)

var (
	flagAssistantProjectIterationID  string
	flagAssistantProjectStaleDays    int
	flagAssistantProjectPeriod       string
	flagAssistantProjectSince        string
	flagAssistantProjectTargetStatus string
	flagAssistantProjectLimit        int
)

var assistantProjectCmd = &cobra.Command{
	Use:   "project",
	Short: "项目管理助手",
}

var assistantProjectProgressCmd = &cobra.Command{
	Use:   "progress",
	Short: "汇总项目或迭代进度",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := runAssistantProjectReport(cmdContext(cmd), "progress"); err != nil {
			output.PrintError(os.Stderr, "assistant_project_error", err.Error(), "")
			os.Exit(output.ExitAPIError)
		}
		return nil
	},
}

var assistantProjectBlockersCmd = &cobra.Command{
	Use:   "blockers",
	Short: "识别项目或迭代卡点",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := runAssistantProjectReport(cmdContext(cmd), "blockers"); err != nil {
			output.PrintError(os.Stderr, "assistant_project_error", err.Error(), "")
			os.Exit(output.ExitAPIError)
		}
		return nil
	},
}

var assistantProjectReportCmd = &cobra.Command{
	Use:   "report",
	Short: "生成项目日报或周报",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := runAssistantProjectReport(cmdContext(cmd), "report"); err != nil {
			output.PrintError(os.Stderr, "assistant_project_error", err.Error(), "")
			os.Exit(output.ExitAPIError)
		}
		return nil
	},
}

var assistantProjectStatusSuggestCmd = &cobra.Command{
	Use:   "status-suggest",
	Short: "给出项目状态流转建议，不自动流转",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := runAssistantProjectReport(cmdContext(cmd), "status-suggest"); err != nil {
			output.PrintError(os.Stderr, "assistant_project_error", err.Error(), "")
			os.Exit(output.ExitAPIError)
		}
		return nil
	},
}

func init() {
	for _, c := range []*cobra.Command{
		assistantProjectProgressCmd,
		assistantProjectBlockersCmd,
		assistantProjectReportCmd,
		assistantProjectStatusSuggestCmd,
	} {
		c.Flags().StringVar(&flagAssistantProjectIterationID, "iteration-id", "", "按迭代 ID 汇总")
		c.Flags().IntVar(&flagAssistantProjectStaleDays, "stale-days", 7, "超过多少天未更新视为停滞")
		c.Flags().IntVar(&flagAssistantProjectLimit, "limit", 200, "每类 TAPD 对象最多读取数量")
		c.Flags().BoolVar(&flagJSON, "json", false, "输出 JSON")
	}
	assistantProjectReportCmd.Flags().StringVar(&flagAssistantProjectPeriod, "period", "daily", "报告周期：daily 或 weekly")
	assistantProjectReportCmd.Flags().StringVar(&flagAssistantProjectSince, "since", "", "报告起始日期，格式 2006-01-02")
	assistantProjectStatusSuggestCmd.Flags().StringVar(&flagAssistantProjectTargetStatus, "target-status", "", "期望流转到的目标状态，仅用于建议")

	assistantProjectCmd.AddCommand(
		assistantProjectProgressCmd,
		assistantProjectBlockersCmd,
		assistantProjectReportCmd,
		assistantProjectStatusSuggestCmd,
	)
	assistantCmd.AddCommand(assistantProjectCmd)
}

type projectManagementOptions struct {
	Mode        string `json:"mode"`
	IterationID string `json:"iteration_id,omitempty"`
	StaleDays   int    `json:"stale_days"`
	Period      string `json:"period,omitempty"`
	Since       string `json:"since,omitempty"`
	TargetState string `json:"target_status,omitempty"`
	Limit       int    `json:"limit,omitempty"`
	Now         time.Time
}

type projectManagementSnapshot struct {
	WorkspaceID  string             `json:"workspace_id,omitempty"`
	IterationID  string             `json:"iteration_id,omitempty"`
	Stories      []projectStoryItem `json:"stories"`
	Tasks        []projectTaskItem  `json:"tasks"`
	Bugs         []projectBugItem   `json:"bugs"`
	LoadedAt     string             `json:"loaded_at,omitempty"`
	LoadedFields string             `json:"loaded_fields,omitempty"`
}

type projectStoryItem struct {
	ID       string `json:"id,omitempty"`
	Name     string `json:"name,omitempty"`
	Status   string `json:"status,omitempty"`
	Owner    string `json:"owner,omitempty"`
	Modified string `json:"modified,omitempty"`
	Due      string `json:"due,omitempty"`
}

type projectTaskItem struct {
	ID       string `json:"id,omitempty"`
	Name     string `json:"name,omitempty"`
	Status   string `json:"status,omitempty"`
	Owner    string `json:"owner,omitempty"`
	StoryID  string `json:"story_id,omitempty"`
	Modified string `json:"modified,omitempty"`
	Due      string `json:"due,omitempty"`
}

type projectBugItem struct {
	ID           string `json:"id,omitempty"`
	Title        string `json:"title,omitempty"`
	Status       string `json:"status,omitempty"`
	CurrentOwner string `json:"current_owner,omitempty"`
	StoryID      string `json:"story_id,omitempty"`
	Modified     string `json:"modified,omitempty"`
	Due          string `json:"due,omitempty"`
}

type projectManagementReport struct {
	WorkspaceID       string                  `json:"workspace_id,omitempty"`
	IterationID       string                  `json:"iteration_id,omitempty"`
	Mode              string                  `json:"mode"`
	Period            string                  `json:"period,omitempty"`
	StoryTotal        int                     `json:"story_total"`
	TaskTotal         int                     `json:"task_total"`
	BugTotal          int                     `json:"bug_total"`
	OpenBugCount      int                     `json:"open_bug_count"`
	ProgressPercent   int                     `json:"progress_percent"`
	StoryStatus       map[string]int          `json:"story_status"`
	TaskStatus        map[string]int          `json:"task_status"`
	BugStatus         map[string]int          `json:"bug_status"`
	Blockers          []projectIssue          `json:"blockers"`
	Risks             []projectIssue          `json:"risks"`
	OwnerFollowUps    []projectOwnerFollowUp  `json:"owner_follow_ups"`
	StatusSuggestion  projectStatusSuggestion `json:"status_suggestion"`
	SuggestedActions  []string                `json:"suggested_actions"`
	Summary           string                  `json:"summary"`
	GeneratedAt       string                  `json:"generated_at"`
	TargetStatus      string                  `json:"target_status,omitempty"`
	ReadOnlyStatement string                  `json:"read_only_statement"`
}

type projectIssue struct {
	Code    string `json:"code"`
	Type    string `json:"type"`
	ID      string `json:"id,omitempty"`
	Title   string `json:"title,omitempty"`
	Owner   string `json:"owner,omitempty"`
	Message string `json:"message"`
}

type projectOwnerFollowUp struct {
	Owner      string `json:"owner"`
	OpenTasks  int    `json:"open_tasks"`
	OpenBugs   int    `json:"open_bugs"`
	RiskItems  int    `json:"risk_items"`
	Suggestion string `json:"suggestion"`
}

type projectStatusSuggestion struct {
	Decision     string   `json:"decision"`
	TargetStatus string   `json:"target_status,omitempty"`
	Reason       string   `json:"reason"`
	Prechecks    []string `json:"prechecks"`
}

func runAssistantProjectReport(ctx context.Context, mode string) error {
	opts := projectManagementOptions{
		Mode:        mode,
		IterationID: strings.TrimSpace(flagAssistantProjectIterationID),
		StaleDays:   flagAssistantProjectStaleDays,
		Period:      strings.TrimSpace(flagAssistantProjectPeriod),
		Since:       strings.TrimSpace(flagAssistantProjectSince),
		TargetState: strings.TrimSpace(flagAssistantProjectTargetStatus),
		Limit:       flagAssistantProjectLimit,
		Now:         time.Now(),
	}
	if opts.StaleDays <= 0 {
		opts.StaleDays = 7
	}
	if opts.Limit <= 0 {
		opts.Limit = 200
	}
	if opts.Period == "" {
		opts.Period = "daily"
	}
	if opts.Period != "daily" && opts.Period != "weekly" {
		return fmt.Errorf("--period must be daily or weekly")
	}

	snapshot, err := loadProjectManagementSnapshot(ctx, opts)
	if err != nil {
		return err
	}
	report := buildProjectManagementReport(snapshot, opts)
	if flagJSON {
		return output.PrintJSON(os.Stdout, report, !flagPretty)
	}
	_, err = fmt.Fprintln(os.Stdout, renderProjectManagementReportMarkdown(report))
	return err
}

func loadProjectManagementSnapshot(ctx context.Context, opts projectManagementOptions) (projectManagementSnapshot, error) {
	fields := "id,name,title,status,owner,current_owner,story_id,modified,due"
	stories, err := apiClient.ListStories(ctx, &model.ListStoriesRequest{
		WorkspaceID: flagWorkspaceID,
		IterationID: opts.IterationID,
		Fields:      "id,name,status,owner,modified,due",
		Limit:       opts.Limit,
		Page:        1,
		Order:       "modified desc",
	})
	if err != nil {
		return projectManagementSnapshot{}, err
	}
	tasks, err := apiClient.ListTasks(ctx, &model.ListTasksRequest{
		WorkspaceID: flagWorkspaceID,
		IterationID: opts.IterationID,
		Fields:      "id,name,status,owner,story_id,modified,due",
		Limit:       opts.Limit,
		Page:        1,
		Order:       "modified desc",
	})
	if err != nil {
		return projectManagementSnapshot{}, err
	}
	bugs, err := apiClient.ListBugs(ctx, &model.ListBugsRequest{
		WorkspaceID: flagWorkspaceID,
		IterationID: opts.IterationID,
		Fields:      "id,title,status,current_owner,story_id,modified,due",
		Limit:       opts.Limit,
		Page:        1,
		Order:       "modified desc",
	})
	if err != nil {
		return projectManagementSnapshot{}, err
	}

	out := projectManagementSnapshot{
		WorkspaceID:  flagWorkspaceID,
		IterationID:  opts.IterationID,
		LoadedAt:     opts.Now.Format(time.RFC3339),
		LoadedFields: fields,
	}
	for _, s := range stories {
		out.Stories = append(out.Stories, projectStoryItem{
			ID:       s.ID,
			Name:     s.Name,
			Status:   s.Status,
			Owner:    s.Owner,
			Modified: s.Modified,
			Due:      s.Due,
		})
	}
	for _, t := range tasks {
		out.Tasks = append(out.Tasks, projectTaskItem{
			ID:       t.ID,
			Name:     t.Name,
			Status:   t.Status,
			Owner:    t.Owner,
			StoryID:  t.StoryID,
			Modified: t.Modified,
			Due:      t.Due,
		})
	}
	for _, b := range bugs {
		out.Bugs = append(out.Bugs, projectBugItem{
			ID:           b.ID,
			Title:        b.Title,
			Status:       b.Status,
			CurrentOwner: b.CurrentOwner,
			StoryID:      b.StoryID,
			Modified:     b.Modified,
			Due:          b.Due,
		})
	}
	return out, nil
}

func buildProjectManagementReport(snapshot projectManagementSnapshot, opts projectManagementOptions) projectManagementReport {
	if opts.Now.IsZero() {
		opts.Now = time.Now()
	}
	if opts.StaleDays <= 0 {
		opts.StaleDays = 7
	}
	report := projectManagementReport{
		WorkspaceID:       snapshot.WorkspaceID,
		IterationID:       firstNonEmpty(opts.IterationID, snapshot.IterationID),
		Mode:              opts.Mode,
		Period:            opts.Period,
		StoryTotal:        len(snapshot.Stories),
		TaskTotal:         len(snapshot.Tasks),
		BugTotal:          len(snapshot.Bugs),
		StoryStatus:       map[string]int{},
		TaskStatus:        map[string]int{},
		BugStatus:         map[string]int{},
		GeneratedAt:       opts.Now.Format(time.RFC3339),
		TargetStatus:      opts.TargetState,
		ReadOnlyStatement: "Read-only report. No TAPD status was changed.",
	}

	completedItems := 0
	totalItems := len(snapshot.Stories) + len(snapshot.Tasks) + len(snapshot.Bugs)
	for _, story := range snapshot.Stories {
		status := normalizeProjectStatus(story.Status)
		report.StoryStatus[status]++
		if isProjectDoneStatus(status) {
			completedItems++
		}
		if !isProjectDoneStatus(status) && strings.TrimSpace(story.Owner) == "" {
			report.Blockers = append(report.Blockers, projectIssue{
				Code:    "missing_story_owner",
				Type:    "story",
				ID:      story.ID,
				Title:   story.Name,
				Message: fmt.Sprintf("Story %s has no owner.", displayProjectItem(story.ID, story.Name)),
			})
		}
		if !isProjectDoneStatus(status) && isProjectOverdue(story.Due, opts.Now) {
			report.Blockers = append(report.Blockers, projectIssue{
				Code:    "overdue_story",
				Type:    "story",
				ID:      story.ID,
				Title:   story.Name,
				Owner:   story.Owner,
				Message: fmt.Sprintf("Story %s is overdue.", displayProjectItem(story.ID, story.Name)),
			})
		}
		if !isProjectDoneStatus(status) && isProjectStale(story.Modified, opts.Now, opts.StaleDays) {
			report.Risks = append(report.Risks, projectIssue{
				Code:    "stale_story",
				Type:    "story",
				ID:      story.ID,
				Title:   story.Name,
				Owner:   story.Owner,
				Message: fmt.Sprintf("Story %s has not been updated for more than %d day(s).", displayProjectItem(story.ID, story.Name), opts.StaleDays),
			})
		}
	}

	for _, task := range snapshot.Tasks {
		status := normalizeProjectStatus(task.Status)
		report.TaskStatus[status]++
		if isProjectDoneStatus(status) {
			completedItems++
		}
		if !isProjectDoneStatus(status) && strings.TrimSpace(task.Owner) == "" {
			report.Blockers = append(report.Blockers, projectIssue{
				Code:    "missing_task_owner",
				Type:    "task",
				ID:      task.ID,
				Title:   task.Name,
				Message: fmt.Sprintf("Task %s has no owner.", displayProjectItem(task.ID, task.Name)),
			})
		}
		if !isProjectDoneStatus(status) && isProjectOverdue(task.Due, opts.Now) {
			report.Blockers = append(report.Blockers, projectIssue{
				Code:    "overdue_task",
				Type:    "task",
				ID:      task.ID,
				Title:   task.Name,
				Owner:   task.Owner,
				Message: fmt.Sprintf("Task %s is overdue.", displayProjectItem(task.ID, task.Name)),
			})
		}
		if !isProjectDoneStatus(status) && isProjectStale(task.Modified, opts.Now, opts.StaleDays) {
			report.Risks = append(report.Risks, projectIssue{
				Code:    "stale_task",
				Type:    "task",
				ID:      task.ID,
				Title:   task.Name,
				Owner:   task.Owner,
				Message: fmt.Sprintf("Task %s has not been updated for more than %d day(s).", displayProjectItem(task.ID, task.Name), opts.StaleDays),
			})
		}
	}

	for _, bug := range snapshot.Bugs {
		status := normalizeProjectStatus(bug.Status)
		report.BugStatus[status]++
		if isProjectDoneStatus(status) {
			completedItems++
		} else {
			report.OpenBugCount++
		}
		if !isProjectDoneStatus(status) && strings.TrimSpace(bug.CurrentOwner) == "" {
			report.Blockers = append(report.Blockers, projectIssue{
				Code:    "missing_bug_owner",
				Type:    "bug",
				ID:      bug.ID,
				Title:   bug.Title,
				Message: fmt.Sprintf("Bug %s has no current owner.", displayProjectItem(bug.ID, bug.Title)),
			})
		}
		if !isProjectDoneStatus(status) && isProjectOverdue(bug.Due, opts.Now) {
			report.Blockers = append(report.Blockers, projectIssue{
				Code:    "overdue_bug",
				Type:    "bug",
				ID:      bug.ID,
				Title:   bug.Title,
				Owner:   bug.CurrentOwner,
				Message: fmt.Sprintf("Bug %s is overdue.", displayProjectItem(bug.ID, bug.Title)),
			})
		}
		if !isProjectDoneStatus(status) && isProjectStale(bug.Modified, opts.Now, opts.StaleDays) {
			report.Risks = append(report.Risks, projectIssue{
				Code:    "stale_bug",
				Type:    "bug",
				ID:      bug.ID,
				Title:   bug.Title,
				Owner:   bug.CurrentOwner,
				Message: fmt.Sprintf("Bug %s has not been updated for more than %d day(s).", displayProjectItem(bug.ID, bug.Title), opts.StaleDays),
			})
		}
	}

	report.ProgressPercent = calculateProjectProgress(totalItems, completedItems)
	report.OwnerFollowUps = buildProjectOwnerFollowUps(snapshot, report.Risks)
	report.StatusSuggestion = buildProjectStatusSuggestion(report, opts)
	report.SuggestedActions = buildProjectSuggestedActions(report)
	report.Summary = buildProjectSummary(report)
	return report
}

func buildProjectOwnerFollowUps(snapshot projectManagementSnapshot, risks []projectIssue) []projectOwnerFollowUp {
	byOwner := map[string]*projectOwnerFollowUp{}
	get := func(owner string) *projectOwnerFollowUp {
		owner = strings.TrimSpace(owner)
		if owner == "" {
			owner = "unassigned"
		}
		if byOwner[owner] == nil {
			byOwner[owner] = &projectOwnerFollowUp{Owner: owner}
		}
		return byOwner[owner]
	}
	for _, task := range snapshot.Tasks {
		if !isProjectDoneStatus(normalizeProjectStatus(task.Status)) {
			get(task.Owner).OpenTasks++
		}
	}
	for _, bug := range snapshot.Bugs {
		if !isProjectDoneStatus(normalizeProjectStatus(bug.Status)) {
			get(bug.CurrentOwner).OpenBugs++
		}
	}
	for _, risk := range risks {
		get(risk.Owner).RiskItems++
	}
	out := make([]projectOwnerFollowUp, 0, len(byOwner))
	for _, item := range byOwner {
		item.Suggestion = buildOwnerFollowUpSuggestion(*item)
		out = append(out, *item)
	}
	sort.Slice(out, func(i, j int) bool {
		li := out[i].OpenTasks + out[i].OpenBugs + out[i].RiskItems
		lj := out[j].OpenTasks + out[j].OpenBugs + out[j].RiskItems
		if li == lj {
			return out[i].Owner < out[j].Owner
		}
		return li > lj
	})
	return out
}

func buildOwnerFollowUpSuggestion(item projectOwnerFollowUp) string {
	parts := []string{}
	if item.OpenTasks > 0 {
		parts = append(parts, fmt.Sprintf("%d open task(s)", item.OpenTasks))
	}
	if item.OpenBugs > 0 {
		parts = append(parts, fmt.Sprintf("%d open bug(s)", item.OpenBugs))
	}
	if item.RiskItems > 0 {
		parts = append(parts, fmt.Sprintf("%d stale/risk item(s)", item.RiskItems))
	}
	if len(parts) == 0 {
		return "No immediate follow-up."
	}
	return "Follow up on " + strings.Join(parts, ", ") + "."
}

func buildProjectStatusSuggestion(report projectManagementReport, opts projectManagementOptions) projectStatusSuggestion {
	target := opts.TargetState
	if target == "" {
		target = "completed"
	}
	prechecks := []string{
		"Confirm all blocking issues are closed.",
		"Confirm stakeholder report has been reviewed.",
		"Confirm no automatic TAPD status transition is performed by this command.",
	}
	if report.StoryTotal+report.TaskTotal+report.BugTotal == 0 {
		return projectStatusSuggestion{
			Decision:     "no_action",
			TargetStatus: target,
			Reason:       "No TAPD items were loaded for this scope.",
			Prechecks:    prechecks,
		}
	}
	if len(report.Blockers) > 0 || report.OpenBugCount > 0 {
		return projectStatusSuggestion{
			Decision:     "keep_in_progress",
			TargetStatus: target,
			Reason:       fmt.Sprintf("There are %d blocker(s) and %d open bug(s).", len(report.Blockers), report.OpenBugCount),
			Prechecks:    prechecks,
		}
	}
	if report.ProgressPercent >= 100 {
		return projectStatusSuggestion{
			Decision:     "ready_to_complete",
			TargetStatus: target,
			Reason:       "All loaded stories, tasks, and bugs are in completed statuses.",
			Prechecks:    prechecks,
		}
	}
	return projectStatusSuggestion{
		Decision:     "continue_in_progress",
		TargetStatus: target,
		Reason:       fmt.Sprintf("Progress is %d%% and no blocking issue was detected.", report.ProgressPercent),
		Prechecks:    prechecks,
	}
}

func buildProjectSuggestedActions(report projectManagementReport) []string {
	actions := []string{}
	if len(report.Blockers) > 0 {
		actions = append(actions, "Resolve blocking items before moving project status forward.")
	}
	if len(report.Risks) > 0 {
		actions = append(actions, "Ask owners to update stale items or add latest comments in TAPD.")
	}
	if report.OpenBugCount > 0 {
		actions = append(actions, "Prioritize open bugs that block story completion.")
	}
	if len(actions) == 0 {
		actions = append(actions, "No immediate blocker found. Keep status and stakeholder report synchronized.")
	}
	return actions
}

func buildProjectSummary(report projectManagementReport) string {
	scope := "workspace"
	if report.IterationID != "" {
		scope = "iteration " + report.IterationID
	}
	return fmt.Sprintf("%s progress is %d%% with %d blocker(s), %d risk item(s), and %d open bug(s).",
		scope, report.ProgressPercent, len(report.Blockers), len(report.Risks), report.OpenBugCount)
}

func renderProjectManagementReportMarkdown(report projectManagementReport) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# %s\n\n", projectReportTitle(report.Mode, report.Period))
	if report.WorkspaceID != "" {
		fmt.Fprintf(&b, "Workspace: %s\n", report.WorkspaceID)
	}
	if report.IterationID != "" {
		fmt.Fprintf(&b, "Iteration: %s\n", report.IterationID)
	}
	fmt.Fprintf(&b, "Progress: %d%%\n", report.ProgressPercent)
	fmt.Fprintf(&b, "Stories: %d\nTasks: %d\nBugs: %d\nOpen Bugs: %d\n\n", report.StoryTotal, report.TaskTotal, report.BugTotal, report.OpenBugCount)
	writeProjectStatusMap(&b, "Story Status", report.StoryStatus)
	writeProjectStatusMap(&b, "Task Status", report.TaskStatus)
	writeProjectStatusMap(&b, "Bug Status", report.BugStatus)
	writeProjectIssuesMarkdown(&b, "Blockers", report.Blockers)
	writeProjectIssuesMarkdown(&b, "Risks", report.Risks)
	if len(report.OwnerFollowUps) > 0 {
		b.WriteString("## Owner Follow-ups\n")
		for _, item := range report.OwnerFollowUps {
			fmt.Fprintf(&b, "- %s: %s\n", item.Owner, item.Suggestion)
		}
		b.WriteString("\n")
	}
	b.WriteString("## Status Suggestion\n")
	fmt.Fprintf(&b, "Decision: %s\nReason: %s\n\n", report.StatusSuggestion.Decision, report.StatusSuggestion.Reason)
	b.WriteString("## Suggested Actions\n")
	for _, action := range report.SuggestedActions {
		fmt.Fprintf(&b, "- %s\n", action)
	}
	fmt.Fprintf(&b, "\n## Summary\n%s\n\n%s\n", report.Summary, report.ReadOnlyStatement)
	return b.String()
}

func projectReportTitle(mode, period string) string {
	switch mode {
	case "blockers":
		return "Project Blockers Report"
	case "report":
		if period == "weekly" {
			return "Project Weekly Report"
		}
		return "Project Daily Report"
	case "status-suggest":
		return "Project Status Suggestion"
	default:
		return "Project Progress Report"
	}
}

func writeProjectStatusMap(b *strings.Builder, title string, values map[string]int) {
	b.WriteString("## " + title + "\n")
	if len(values) == 0 {
		b.WriteString("- None\n\n")
		return
	}
	keys := make([]string, 0, len(values))
	for k := range values {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		fmt.Fprintf(b, "- %s: %d\n", k, values[k])
	}
	b.WriteString("\n")
}

func writeProjectIssuesMarkdown(b *strings.Builder, title string, issues []projectIssue) {
	b.WriteString("## " + title + "\n")
	if len(issues) == 0 {
		b.WriteString("- None\n\n")
		return
	}
	for _, issue := range issues {
		fmt.Fprintf(b, "- [%s] %s\n", issue.Code, issue.Message)
	}
	b.WriteString("\n")
}

func normalizeProjectStatus(status string) string {
	status = strings.TrimSpace(strings.ToLower(status))
	if status == "" {
		return "unknown"
	}
	return status
}

func isProjectDoneStatus(status string) bool {
	switch normalizeProjectStatus(status) {
	case "done", "closed", "resolved", "completed", "complete", "finish", "finished", "已完成", "关闭", "已关闭", "完成":
		return true
	default:
		return false
	}
}

func isProjectOverdue(raw string, now time.Time) bool {
	due, ok := parseProjectTime(raw)
	if !ok {
		return false
	}
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	return due.Before(today)
}

func isProjectStale(raw string, now time.Time, staleDays int) bool {
	modified, ok := parseProjectTime(raw)
	if !ok {
		return false
	}
	return modified.Before(now.AddDate(0, 0, -staleDays))
}

func parseProjectTime(raw string) (time.Time, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}, false
	}
	layouts := []string{
		"2006-01-02 15:04:05",
		"2006-01-02",
		time.RFC3339,
	}
	for _, layout := range layouts {
		if t, err := time.ParseInLocation(layout, raw, time.Local); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

func calculateProjectProgress(total, completed int) int {
	if total <= 0 {
		return 0
	}
	return completed * 100 / total
}

func displayProjectItem(id, title string) string {
	id = strings.TrimSpace(id)
	title = strings.TrimSpace(title)
	switch {
	case id != "" && title != "":
		return id + " " + title
	case id != "":
		return id
	case title != "":
		return title
	default:
		return "(unknown)"
	}
}
