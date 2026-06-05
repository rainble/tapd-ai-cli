// Package cmd 中的 change.go 实现了变更历史管理命令
package cmd

import (
	"context"
	"os"

	"github.com/spf13/cobra"
	"github.com/studyzy/tapd-ai-cli/internal/output"
	"github.com/studyzy/tapd-sdk-go/model"
)

var (
	flagChangeType     string
	flagChangeEntityID string
)

// changeCmd 是 change 父命令
var changeCmd = &cobra.Command{
	Use:   "change",
	Short: "变更历史管理",
}

var changeListCmd = &cobra.Command{
	Use:   "list",
	Short: "查询变更历史",
	RunE:  runChangeList,
}

var changeCountCmd = &cobra.Command{
	Use:   "count",
	Short: "查询变更数量",
	RunE:  runChangeCount,
}

func init() {
	changeListCmd.Flags().StringVar(&flagChangeType, "type", "", "实体类型（story|bug|task|iteration，必需）")
	changeListCmd.Flags().StringVar(&flagChangeEntityID, "entity-id", "", "实体 ID（迭代变更时必需）")
	changeListCmd.Flags().IntVar(&flagLimit, "limit", 10, "返回数量限制")
	changeListCmd.Flags().IntVar(&flagPage, "page", 1, "页码")
	changeListCmd.Flags().StringArrayVar(&flagFilter, "filter", nil, filterFlagDesc)

	changeCountCmd.Flags().StringVar(&flagChangeType, "type", "", "实体类型（story|bug|task，必需）")
	changeCountCmd.Flags().StringVar(&flagChangeEntityID, "entity-id", "", "实体 ID")

	changeCmd.AddCommand(changeListCmd, changeCountCmd)
	rootCmd.AddCommand(changeCmd)
}

func runChangeList(cmd *cobra.Command, args []string) error {
	if flagChangeType == "" {
		output.PrintError(os.Stderr, "missing_parameter",
			"--type is required (story|bug|task|iteration)",
			"Usage: tapd change list --type story [--entity-id <id>]")
		os.Exit(output.ExitParamError)
		return nil
	}

	ctx := cmdContext(cmd)
	switch flagChangeType {
	case "story":
		return listStoryChanges(ctx)
	case "bug":
		return listBugChanges(ctx)
	case "task":
		return listTaskChanges(ctx)
	case "iteration":
		return listIterationChanges(ctx)
	default:
		output.PrintError(os.Stderr, "invalid_parameter",
			"--type must be one of: story, bug, task, iteration",
			"")
		os.Exit(output.ExitParamError)
		return nil
	}
}

func listStoryChanges(ctx context.Context) error {
	req := &model.GetStoryChangesRequest{
		WorkspaceID: flagWorkspaceID,
		StoryID:     flagChangeEntityID,
		Limit:       flagLimit,
		Page:        flagPage,
	}
	changes, err := listWithFilters[model.WorkitemChange](ctx, apiClient, "/story_changes", req.ToParams(), flagFilter, "WorkitemChange")
	if err != nil {
		output.PrintError(os.Stderr, "api_error", err.Error(), "")
		os.Exit(output.ExitAPIError)
		return nil
	}
	countReq := &model.CountStoryChangesRequest{
		WorkspaceID: flagWorkspaceID,
		StoryID:     flagChangeEntityID,
	}
	total, _ := apiClient.CountStoryChanges(ctx, countReq)
	resp := &model.ListResponse{
		Items:   changes,
		Total:   total,
		Page:    flagPage,
		Limit:   flagLimit,
		HasMore: total > flagPage*flagLimit,
	}
	return output.PrintJSON(os.Stdout, resp, !flagPretty)
}

func listBugChanges(ctx context.Context) error {
	req := &model.GetBugChangesRequest{
		WorkspaceID: flagWorkspaceID,
		BugID:       flagChangeEntityID,
		Limit:       flagLimit,
		Page:        flagPage,
	}
	changes, err := listWithFilters[model.BugChange](ctx, apiClient, "/bug_changes", req.ToParams(), flagFilter, "BugChange")
	if err != nil {
		output.PrintError(os.Stderr, "api_error", err.Error(), "")
		os.Exit(output.ExitAPIError)
		return nil
	}
	countReq := &model.CountBugChangesRequest{
		WorkspaceID: flagWorkspaceID,
		BugID:       flagChangeEntityID,
	}
	total, _ := apiClient.CountBugChanges(ctx, countReq)
	resp := &model.ListResponse{
		Items:   changes,
		Total:   total,
		Page:    flagPage,
		Limit:   flagLimit,
		HasMore: total > flagPage*flagLimit,
	}
	return output.PrintJSON(os.Stdout, resp, !flagPretty)
}

func listTaskChanges(ctx context.Context) error {
	req := &model.GetTaskChangesRequest{
		WorkspaceID: flagWorkspaceID,
		TaskID:      flagChangeEntityID,
		Limit:       flagLimit,
		Page:        flagPage,
	}
	changes, err := listWithFilters[model.WorkitemChange](ctx, apiClient, "/task_changes", req.ToParams(), flagFilter, "WorkitemChange")
	if err != nil {
		output.PrintError(os.Stderr, "api_error", err.Error(), "")
		os.Exit(output.ExitAPIError)
		return nil
	}
	countReq := &model.CountTaskChangesRequest{
		WorkspaceID: flagWorkspaceID,
		TaskID:      flagChangeEntityID,
	}
	total, _ := apiClient.CountTaskChanges(ctx, countReq)
	resp := &model.ListResponse{
		Items:   changes,
		Total:   total,
		Page:    flagPage,
		Limit:   flagLimit,
		HasMore: total > flagPage*flagLimit,
	}
	return output.PrintJSON(os.Stdout, resp, !flagPretty)
}

func listIterationChanges(ctx context.Context) error {
	if flagChangeEntityID == "" {
		output.PrintError(os.Stderr, "missing_parameter",
			"--entity-id is required for iteration changes",
			"Usage: tapd change list --type iteration --entity-id <iteration_id>")
		os.Exit(output.ExitParamError)
		return nil
	}
	req := &model.GetIterationChangesRequest{
		WorkspaceID: flagWorkspaceID,
		IterationID: flagChangeEntityID,
		Limit:       flagLimit,
		Page:        flagPage,
	}
	changes, err := listWithFilters[model.IterationChange](ctx, apiClient, "/iteration_changes", req.ToParams(), flagFilter, "IterationChange")
	if err != nil {
		output.PrintError(os.Stderr, "api_error", err.Error(), "")
		os.Exit(output.ExitAPIError)
		return nil
	}
	resp := &model.ListResponse{
		Items: changes,
		Total: len(changes),
		Page:  flagPage,
		Limit: flagLimit,
	}
	return output.PrintJSON(os.Stdout, resp, !flagPretty)
}

func runChangeCount(cmd *cobra.Command, args []string) error {
	if flagChangeType == "" {
		output.PrintError(os.Stderr, "missing_parameter",
			"--type is required (story|bug|task)",
			"Usage: tapd change count --type story [--entity-id <id>]")
		os.Exit(output.ExitParamError)
		return nil
	}

	ctx := cmdContext(cmd)
	var count int
	var err error

	switch flagChangeType {
	case "story":
		count, err = apiClient.CountStoryChanges(ctx, &model.CountStoryChangesRequest{
			WorkspaceID: flagWorkspaceID,
			StoryID:     flagChangeEntityID,
		})
	case "bug":
		count, err = apiClient.CountBugChanges(ctx, &model.CountBugChangesRequest{
			WorkspaceID: flagWorkspaceID,
			BugID:       flagChangeEntityID,
		})
	case "task":
		count, err = apiClient.CountTaskChanges(ctx, &model.CountTaskChangesRequest{
			WorkspaceID: flagWorkspaceID,
			TaskID:      flagChangeEntityID,
		})
	default:
		output.PrintError(os.Stderr, "invalid_parameter",
			"--type must be one of: story, bug, task",
			"")
		os.Exit(output.ExitParamError)
		return nil
	}

	if err != nil {
		output.PrintError(os.Stderr, "api_error", err.Error(), "")
		os.Exit(output.ExitAPIError)
		return nil
	}
	return output.PrintJSON(os.Stdout, &model.CountResponse{Count: count}, !flagPretty)
}
