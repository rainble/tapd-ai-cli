// Package cmd 中的 removed.go 实现了需求/缺陷/任务的回收站查询命令
package cmd

import (
	"context"
	"os"

	"github.com/spf13/cobra"
	"github.com/studyzy/tapd-ai-cli/internal/output"
	"github.com/studyzy/tapd-sdk-go/model"
)

// storyRemovedCmd 查询回收站需求
var storyRemovedCmd = &cobra.Command{
	Use:   "removed",
	Short: "查询回收站中的需求",
	RunE:  runStoryRemoved,
}

// bugRemovedCmd 查询回收站缺陷
var bugRemovedCmd = &cobra.Command{
	Use:   "removed",
	Short: "查询回收站中的缺陷",
	RunE:  runBugRemoved,
}

// taskRemovedCmd 查询回收站任务
var taskRemovedCmd = &cobra.Command{
	Use:   "removed",
	Short: "查询回收站中的任务",
	RunE:  runTaskRemoved,
}

func init() {
	storyRemovedCmd.Flags().IntVar(&flagLimit, "limit", 0, "返回数量限制（默认 30，最大 200）")
	storyRemovedCmd.Flags().IntVar(&flagPage, "page", 0, "页码")

	bugRemovedCmd.Flags().IntVar(&flagLimit, "limit", 0, "返回数量限制（默认 30，最大 200）")
	bugRemovedCmd.Flags().IntVar(&flagPage, "page", 0, "页码")

	taskRemovedCmd.Flags().IntVar(&flagLimit, "limit", 0, "返回数量限制（默认 30，最大 200）")
	taskRemovedCmd.Flags().IntVar(&flagPage, "page", 0, "页码")

	storyCmd.AddCommand(storyRemovedCmd)
	bugCmd.AddCommand(bugRemovedCmd)
	taskCmd.AddCommand(taskRemovedCmd)
}

func runStoryRemoved(cmd *cobra.Command, args []string) error {
	req := &model.GetRemovedStoriesRequest{
		WorkspaceID: flagWorkspaceID,
		Limit:       flagLimit,
		Page:        flagPage,
	}

	stories, err := apiClient.GetRemovedStories(context.Background(), req)
	if err != nil {
		output.PrintError(os.Stderr, "api_error", err.Error(), "")
		os.Exit(output.ExitAPIError)
		return nil
	}
	return output.PrintJSON(os.Stdout, stories, !flagPretty)
}

func runBugRemoved(cmd *cobra.Command, args []string) error {
	req := &model.GetRemovedBugsRequest{
		WorkspaceID: flagWorkspaceID,
		Limit:       flagLimit,
		Page:        flagPage,
	}

	bugs, err := apiClient.GetRemovedBugs(context.Background(), req)
	if err != nil {
		output.PrintError(os.Stderr, "api_error", err.Error(), "")
		os.Exit(output.ExitAPIError)
		return nil
	}
	return output.PrintJSON(os.Stdout, bugs, !flagPretty)
}

func runTaskRemoved(cmd *cobra.Command, args []string) error {
	req := &model.GetRemovedTasksRequest{
		WorkspaceID: flagWorkspaceID,
		Limit:       flagLimit,
		Page:        flagPage,
	}

	tasks, err := apiClient.GetRemovedTasks(context.Background(), req)
	if err != nil {
		output.PrintError(os.Stderr, "api_error", err.Error(), "")
		os.Exit(output.ExitAPIError)
		return nil
	}
	return output.PrintJSON(os.Stdout, tasks, !flagPretty)
}
