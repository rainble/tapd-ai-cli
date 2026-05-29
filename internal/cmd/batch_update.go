// Package cmd 中的 batch_update.go 实现了需求/缺陷/任务的批量更新命令
package cmd

import (
	"context"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/studyzy/tapd-ai-cli/internal/output"
	"github.com/studyzy/tapd-sdk-go/model"
)

var flagBatchIDs string

// storyBatchUpdateCmd 批量更新需求
var storyBatchUpdateCmd = &cobra.Command{
	Use:   "batch-update",
	Short: "批量更新需求",
	RunE:  runStoryBatchUpdate,
}

// bugBatchUpdateCmd 批量更新缺陷
var bugBatchUpdateCmd = &cobra.Command{
	Use:   "batch-update",
	Short: "批量更新缺陷",
	RunE:  runBugBatchUpdate,
}

// taskBatchUpdateCmd 批量更新任务
var taskBatchUpdateCmd = &cobra.Command{
	Use:   "batch-update",
	Short: "批量更新任务",
	RunE:  runTaskBatchUpdate,
}

func init() {
	storyBatchUpdateCmd.Flags().StringVar(&flagBatchIDs, "ids", "", "需求 ID 列表（逗号分隔，必需）")
	storyBatchUpdateCmd.Flags().StringVar(&flagStatus, "status", "", "目标状态")
	storyBatchUpdateCmd.Flags().StringVar(&flagOwner, "owner", "", "处理人")
	storyBatchUpdateCmd.Flags().StringVar(&flagPriority, "priority", "", "优先级")
	storyBatchUpdateCmd.Flags().StringVar(&flagCurrentUser, "current-user", "", "变更人")

	bugBatchUpdateCmd.Flags().StringVar(&flagBatchIDs, "ids", "", "缺陷 ID 列表（逗号分隔，必需）")
	bugBatchUpdateCmd.Flags().StringVar(&flagStatus, "status", "", "目标状态")
	bugBatchUpdateCmd.Flags().StringVar(&flagOwner, "owner", "", "处理人")
	bugBatchUpdateCmd.Flags().StringVar(&flagCurrentUser, "current-user", "", "变更人")

	taskBatchUpdateCmd.Flags().StringVar(&flagBatchIDs, "ids", "", "任务 ID 列表（逗号分隔，必需）")
	taskBatchUpdateCmd.Flags().StringVar(&flagStatus, "status", "", "目标状态")
	taskBatchUpdateCmd.Flags().StringVar(&flagOwner, "owner", "", "处理人")
	taskBatchUpdateCmd.Flags().StringVar(&flagCurrentUser, "current-user", "", "变更人")

	storyCmd.AddCommand(storyBatchUpdateCmd)
	bugCmd.AddCommand(bugBatchUpdateCmd)
	taskCmd.AddCommand(taskBatchUpdateCmd)
}

func runStoryBatchUpdate(cmd *cobra.Command, args []string) error {
	if flagBatchIDs == "" {
		output.PrintError(os.Stderr, "missing_parameter",
			"--ids is required",
			"Usage: tapd story batch-update --ids <id1,id2,...>")
		os.Exit(output.ExitParamError)
		return nil
	}

	ids := strings.Split(flagBatchIDs, ",")
	items := make([]model.BatchUpdateStoryItem, 0, len(ids))
	for _, id := range ids {
		item := model.BatchUpdateStoryItem{
			ID:          strings.TrimSpace(id),
			Status:      flagStatus,
			Owner:       flagOwner,
			Priority:    flagPriority,
			CurrentUser: flagCurrentUser,
		}
		items = append(items, item)
	}

	req := &model.BatchUpdateStoryRequest{
		WorkspaceID: flagWorkspaceID,
		Workitems:   items,
	}

	msg, err := apiClient.BatchUpdateStory(context.Background(), req)
	if err != nil {
		output.PrintError(os.Stderr, "api_error", err.Error(), "")
		os.Exit(output.ExitAPIError)
		return nil
	}
	return output.PrintJSON(os.Stdout, map[string]string{"message": msg}, !flagPretty)
}

func runBugBatchUpdate(cmd *cobra.Command, args []string) error {
	if flagBatchIDs == "" {
		output.PrintError(os.Stderr, "missing_parameter",
			"--ids is required",
			"Usage: tapd bug batch-update --ids <id1,id2,...>")
		os.Exit(output.ExitParamError)
		return nil
	}

	ids := strings.Split(flagBatchIDs, ",")
	items := make([]model.BatchUpdateBugItem, 0, len(ids))
	for _, id := range ids {
		item := model.BatchUpdateBugItem{
			ID:           strings.TrimSpace(id),
			Status:       flagStatus,
			CurrentOwner: flagOwner,
			CurrentUser:  flagCurrentUser,
		}
		items = append(items, item)
	}

	req := &model.BatchUpdateBugRequest{
		WorkspaceID: flagWorkspaceID,
		Workitems:   items,
	}

	err := apiClient.BatchUpdateBug(context.Background(), req)
	if err != nil {
		output.PrintError(os.Stderr, "api_error", err.Error(), "")
		os.Exit(output.ExitAPIError)
		return nil
	}
	return output.PrintJSON(os.Stdout, map[string]string{"message": "batch update success"}, !flagPretty)
}

func runTaskBatchUpdate(cmd *cobra.Command, args []string) error {
	if flagBatchIDs == "" {
		output.PrintError(os.Stderr, "missing_parameter",
			"--ids is required",
			"Usage: tapd task batch-update --ids <id1,id2,...>")
		os.Exit(output.ExitParamError)
		return nil
	}

	ids := strings.Split(flagBatchIDs, ",")
	items := make([]model.BatchUpdateTaskItem, 0, len(ids))
	for _, id := range ids {
		item := model.BatchUpdateTaskItem{
			"id": strings.TrimSpace(id),
		}
		if flagStatus != "" {
			item["status"] = flagStatus
		}
		if flagOwner != "" {
			item["owner"] = flagOwner
		}
		if flagCurrentUser != "" {
			item["current_user"] = flagCurrentUser
		}
		items = append(items, item)
	}

	req := &model.BatchUpdateTaskRequest{
		WorkspaceID: flagWorkspaceID,
		Workitems:   items,
	}

	msg, err := apiClient.BatchUpdateTask(context.Background(), req)
	if err != nil {
		output.PrintError(os.Stderr, "api_error", err.Error(), "")
		os.Exit(output.ExitAPIError)
		return nil
	}
	return output.PrintJSON(os.Stdout, map[string]string{"message": msg}, !flagPretty)
}
