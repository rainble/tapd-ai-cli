// Package cmd 中的 story_extras.go 实现了需求扩展命令（复制、关联）
package cmd

import (
	"context"
	"os"

	"github.com/spf13/cobra"
	"github.com/studyzy/tapd-ai-cli/internal/output"
	"github.com/studyzy/tapd-sdk-go/model"
)

var (
	flagStoryLinkTargetID string
)

// storyCopyCmd 复制需求
var storyCopyCmd = &cobra.Command{
	Use:   "copy <story_id>",
	Short: "复制需求",
	Args:  cobra.ExactArgs(1),
	RunE:  runStoryCopy,
}

// storyLinkCmd 是需求关联父命令
var storyLinkCmd = &cobra.Command{
	Use:   "link",
	Short: "需求关联关系管理",
}

var storyLinkListCmd = &cobra.Command{
	Use:   "list",
	Short: "查询需求关联关系",
	RunE:  runStoryLinkList,
}

var storyLinkAddCmd = &cobra.Command{
	Use:   "add",
	Short: "创建需求关联关系",
	RunE:  runStoryLinkAdd,
}

var storyLinkRemoveCmd = &cobra.Command{
	Use:   "remove",
	Short: "解除需求关联关系",
	RunE:  runStoryLinkRemove,
}

func init() {
	storyCopyCmd.Flags().StringVar(&flagCurrentUser, "current-user", "", "新需求创建人")

	storyLinkListCmd.Flags().StringVar(&flagChangeEntityID, "story-id", "", "需求 ID（必需）")
	storyLinkAddCmd.Flags().StringVar(&flagChangeEntityID, "story-id", "", "源需求 ID（必需）")
	storyLinkAddCmd.Flags().StringVar(&flagStoryLinkTargetID, "target-id", "", "目标需求 ID（必需）")
	storyLinkRemoveCmd.Flags().StringVar(&flagChangeEntityID, "story-id", "", "源需求 ID（必需）")
	storyLinkRemoveCmd.Flags().StringVar(&flagStoryLinkTargetID, "target-id", "", "目标需求 ID（必需）")

	storyLinkCmd.AddCommand(storyLinkListCmd, storyLinkAddCmd, storyLinkRemoveCmd)
	storyCmd.AddCommand(storyCopyCmd, storyLinkCmd)
}

func runStoryCopy(cmd *cobra.Command, args []string) error {
	id := expandShortID(args[0], flagWorkspaceID)
	req := &model.CopyStoryRequest{
		WorkspaceID:    flagWorkspaceID,
		SrcStoryID:     id,
		DstWorkspaceID: flagWorkspaceID,
		NewCreator:     flagCurrentUser,
	}
	result, err := apiClient.CopyStory(context.Background(), req)
	if err != nil {
		output.PrintError(os.Stderr, "api_error", err.Error(), "")
		os.Exit(output.ExitAPIError)
		return nil
	}
	return output.PrintJSON(os.Stdout, result, !flagPretty)
}

func runStoryLinkList(cmd *cobra.Command, args []string) error {
	if flagChangeEntityID == "" {
		output.PrintError(os.Stderr, "missing_parameter",
			"--story-id is required",
			"Usage: tapd story link list --story-id <id>")
		os.Exit(output.ExitParamError)
		return nil
	}
	req := &model.GetLinkStoriesRequest{
		WorkspaceID: flagWorkspaceID,
		StoryID:     flagChangeEntityID,
	}
	relations, err := apiClient.GetLinkStories(context.Background(), req)
	if err != nil {
		output.PrintError(os.Stderr, "api_error", err.Error(), "")
		os.Exit(output.ExitAPIError)
		return nil
	}
	return output.PrintJSON(os.Stdout, relations, !flagPretty)
}

func runStoryLinkAdd(cmd *cobra.Command, args []string) error {
	if flagChangeEntityID == "" || flagStoryLinkTargetID == "" {
		output.PrintError(os.Stderr, "missing_parameter",
			"--story-id and --target-id are required",
			"Usage: tapd story link add --story-id <id> --target-id <id>")
		os.Exit(output.ExitParamError)
		return nil
	}
	req := &model.AddStoryLinkRelationsRequest{
		WorkspaceID:   flagWorkspaceID,
		SrcStoryID:    flagChangeEntityID,
		TargetStoryID: flagStoryLinkTargetID,
	}
	ok, err := apiClient.AddStoryLinkRelations(context.Background(), req)
	if err != nil {
		output.PrintError(os.Stderr, "api_error", err.Error(), "")
		os.Exit(output.ExitAPIError)
		return nil
	}
	return output.PrintJSON(os.Stdout, &model.SuccessResponse{Success: ok}, !flagPretty)
}

func runStoryLinkRemove(cmd *cobra.Command, args []string) error {
	if flagChangeEntityID == "" || flagStoryLinkTargetID == "" {
		output.PrintError(os.Stderr, "missing_parameter",
			"--story-id and --target-id are required",
			"Usage: tapd story link remove --story-id <id> --target-id <id>")
		os.Exit(output.ExitParamError)
		return nil
	}
	req := &model.RemoveStoryLinkRelationRequest{
		WorkspaceID:   flagWorkspaceID,
		SrcStoryID:    flagChangeEntityID,
		TargetStoryID: flagStoryLinkTargetID,
	}
	ok, err := apiClient.RemoveStoryLinkRelation(context.Background(), req)
	if err != nil {
		output.PrintError(os.Stderr, "api_error", err.Error(), "")
		os.Exit(output.ExitAPIError)
		return nil
	}
	return output.PrintJSON(os.Stdout, &model.SuccessResponse{Success: ok}, !flagPretty)
}
