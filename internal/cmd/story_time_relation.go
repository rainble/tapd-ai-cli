// Package cmd 中的 story_time_relation.go 实现了需求前后置关系管理命令
package cmd

import (
	"context"
	"os"

	"github.com/spf13/cobra"
	"github.com/studyzy/tapd-ai-cli/internal/output"
	"github.com/studyzy/tapd-sdk-go/model"
)

var (
	flagTimeRelStoryID   string
	flagTimeRelRelatedID string
)

// storyTimeRelationCmd 是需求前后置关系父命令
var storyTimeRelationCmd = &cobra.Command{
	Use:   "time-relation",
	Short: "需求前后置关系管理",
}

var storyTimeRelationListCmd = &cobra.Command{
	Use:   "list",
	Short: "查询需求前后置关系",
	RunE:  runStoryTimeRelationList,
}

var storyTimeRelationDeleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "删除需求前后置关系",
	RunE:  runStoryTimeRelationDelete,
}

func init() {
	storyTimeRelationListCmd.Flags().StringVar(&flagTimeRelStoryID, "story-id", "", "需求 ID（必需）")

	storyTimeRelationDeleteCmd.Flags().StringVar(&flagTimeRelStoryID, "story-id", "", "起点需求 ID（必需）")
	storyTimeRelationDeleteCmd.Flags().StringVar(&flagTimeRelRelatedID, "related-id", "", "终点需求 ID（必需）")
	storyTimeRelationDeleteCmd.Flags().StringVar(&flagCurrentUser, "current-user", "", "操作人（必需）")

	storyTimeRelationCmd.AddCommand(storyTimeRelationListCmd, storyTimeRelationDeleteCmd)
	storyCmd.AddCommand(storyTimeRelationCmd)
}

func runStoryTimeRelationList(cmd *cobra.Command, args []string) error {
	if flagTimeRelStoryID == "" {
		output.PrintError(os.Stderr, "missing_parameter", "--story-id is required",
			"Usage: tapd story time-relation list --story-id <id>")
		os.Exit(output.ExitParamError)
		return nil
	}
	req := &model.GetTimeRelativeStoriesRequest{
		WorkspaceID: flagWorkspaceID,
		StoryID:     flagTimeRelStoryID,
	}
	relations, err := apiClient.GetTimeRelativeStories(context.Background(), req)
	if err != nil {
		output.PrintError(os.Stderr, "api_error", err.Error(), "")
		os.Exit(output.ExitAPIError)
		return nil
	}
	return output.PrintJSON(os.Stdout, relations, !flagPretty)
}

func runStoryTimeRelationDelete(cmd *cobra.Command, args []string) error {
	if flagTimeRelStoryID == "" || flagTimeRelRelatedID == "" {
		output.PrintError(os.Stderr, "missing_parameter", "--story-id and --related-id are required",
			"Usage: tapd story time-relation delete --story-id <id> --related-id <id> --current-user <user>")
		os.Exit(output.ExitParamError)
		return nil
	}
	if flagCurrentUser == "" {
		output.PrintError(os.Stderr, "missing_parameter", "--current-user is required",
			"Usage: tapd story time-relation delete --story-id <id> --related-id <id> --current-user <user>")
		os.Exit(output.ExitParamError)
		return nil
	}
	req := &model.DeleteTimeRelationsRequest{
		WorkspaceID: flagWorkspaceID,
		CurrentUser: flagCurrentUser,
		Relations: []model.DeleteTimeRelationItem{
			{
				WorkitemID:    flagTimeRelStoryID,
				DstWorkitemID: flagTimeRelRelatedID,
			},
		},
	}
	num, err := apiClient.DeleteTimeRelations(context.Background(), req)
	if err != nil {
		output.PrintError(os.Stderr, "api_error", err.Error(), "")
		os.Exit(output.ExitAPIError)
		return nil
	}
	return output.PrintJSON(os.Stdout, &model.CountResponse{Count: num}, !flagPretty)
}
