// Package cmd 中的 bug_extras.go 实现了缺陷扩展命令（复制、关联）
package cmd

import (
	"context"
	"os"

	"github.com/spf13/cobra"
	"github.com/studyzy/tapd-ai-cli/internal/output"
	"github.com/studyzy/tapd-sdk-go/model"
)

var (
	flagBugLinkTargetIDs string
	flagBugLinkIDs       string
)

// bugCopyCmd 复制缺陷
var bugCopyCmd = &cobra.Command{
	Use:   "copy <bug_id>",
	Short: "复制缺陷",
	Args:  cobra.ExactArgs(1),
	RunE:  runBugCopy,
}

// bugLinkCmd 是缺陷关联父命令
var bugLinkCmd = &cobra.Command{
	Use:   "link",
	Short: "缺陷关联关系管理",
}

var bugLinkListCmd = &cobra.Command{
	Use:   "list",
	Short: "查询缺陷关联关系",
	RunE:  runBugLinkList,
}

var bugLinkAddCmd = &cobra.Command{
	Use:   "add",
	Short: "创建缺陷关联关系",
	RunE:  runBugLinkAdd,
}

var bugLinkRemoveCmd = &cobra.Command{
	Use:   "remove",
	Short: "取消缺陷关联关系",
	RunE:  runBugLinkRemove,
}

// bugRelatedStoriesCmd 获取缺陷关联的需求
var bugRelatedStoriesCmd = &cobra.Command{
	Use:   "related-stories",
	Short: "查询缺陷关联的需求",
	RunE:  runBugRelatedStories,
}

// bugTemplateCmd 是缺陷模板父命令
var bugTemplateCmd = &cobra.Command{
	Use:   "template",
	Short: "缺陷模板管理",
}

var bugTemplateListCmd = &cobra.Command{
	Use:   "list",
	Short: "查询缺陷模板列表",
	RunE:  runBugTemplateList,
}

func init() {
	bugCopyCmd.Flags().StringVar(&flagCurrentUser, "current-user", "", "操作人")

	bugLinkListCmd.Flags().StringVar(&flagChangeEntityID, "bug-id", "", "缺陷 ID（必需）")
	bugLinkAddCmd.Flags().StringVar(&flagChangeEntityID, "bug-id", "", "缺陷 ID（必需）")
	bugLinkAddCmd.Flags().StringVar(&flagBugLinkTargetIDs, "target-bug-ids", "", "关联缺陷 ID（多个以逗号分隔，必需）")
	bugLinkRemoveCmd.Flags().StringVar(&flagChangeEntityID, "bug-id", "", "缺陷 ID（必需）")
	bugLinkRemoveCmd.Flags().StringVar(&flagBugLinkIDs, "link-ids", "", "link_id（多个以逗号分隔，必需）")

	bugRelatedStoriesCmd.Flags().StringVar(&flagChangeEntityID, "bug-id", "", "缺陷 ID（必需）")

	bugLinkCmd.AddCommand(bugLinkListCmd, bugLinkAddCmd, bugLinkRemoveCmd)
	bugTemplateCmd.AddCommand(bugTemplateListCmd)
	bugCmd.AddCommand(bugCopyCmd, bugLinkCmd, bugRelatedStoriesCmd, bugTemplateCmd)
}

func runBugCopy(cmd *cobra.Command, args []string) error {
	id := expandShortID(args[0], flagWorkspaceID)
	req := &model.CopyBugRequest{
		WorkspaceID:    flagWorkspaceID,
		SrcBugID:       id,
		DstWorkspaceID: flagWorkspaceID,
	}
	result, err := apiClient.CopyBug(context.Background(), req)
	if err != nil {
		output.PrintError(os.Stderr, "api_error", err.Error(), "")
		os.Exit(output.ExitAPIError)
		return nil
	}
	return output.PrintJSON(os.Stdout, result, !flagPretty)
}

func runBugLinkList(cmd *cobra.Command, args []string) error {
	if flagChangeEntityID == "" {
		output.PrintError(os.Stderr, "missing_parameter",
			"--bug-id is required",
			"Usage: tapd bug link list --bug-id <id>")
		os.Exit(output.ExitParamError)
		return nil
	}
	req := &model.GetLinkBugsRequest{
		WorkspaceID: flagWorkspaceID,
		BugID:       flagChangeEntityID,
	}
	links, err := apiClient.GetLinkBugs(context.Background(), req)
	if err != nil {
		output.PrintError(os.Stderr, "api_error", err.Error(), "")
		os.Exit(output.ExitAPIError)
		return nil
	}
	return output.PrintJSON(os.Stdout, links, !flagPretty)
}

func runBugLinkAdd(cmd *cobra.Command, args []string) error {
	if flagChangeEntityID == "" || flagBugLinkTargetIDs == "" {
		output.PrintError(os.Stderr, "missing_parameter",
			"--bug-id and --target-bug-ids are required",
			"Usage: tapd bug link add --bug-id <id> --target-bug-ids <id1,id2>")
		os.Exit(output.ExitParamError)
		return nil
	}
	req := &model.LinkBugsRequest{
		WorkspaceID: flagWorkspaceID,
		BugID:       flagChangeEntityID,
		RelateBugs:  flagBugLinkTargetIDs,
	}
	err := apiClient.LinkBugs(context.Background(), req)
	if err != nil {
		output.PrintError(os.Stderr, "api_error", err.Error(), "")
		os.Exit(output.ExitAPIError)
		return nil
	}
	return output.PrintJSON(os.Stdout, &model.SuccessResponse{Success: true}, !flagPretty)
}

func runBugLinkRemove(cmd *cobra.Command, args []string) error {
	if flagChangeEntityID == "" || flagBugLinkIDs == "" {
		output.PrintError(os.Stderr, "missing_parameter",
			"--bug-id and --link-ids are required",
			"Usage: tapd bug link remove --bug-id <id> --link-ids <link_id1,link_id2>")
		os.Exit(output.ExitParamError)
		return nil
	}
	req := &model.DeleteLinkBugsRequest{
		WorkspaceID: flagWorkspaceID,
		BugID:       flagChangeEntityID,
		LinkIDs:     flagBugLinkIDs,
	}
	err := apiClient.DeleteLinkBugs(context.Background(), req)
	if err != nil {
		output.PrintError(os.Stderr, "api_error", err.Error(), "")
		os.Exit(output.ExitAPIError)
		return nil
	}
	return output.PrintJSON(os.Stdout, &model.SuccessResponse{Success: true}, !flagPretty)
}

func runBugRelatedStories(cmd *cobra.Command, args []string) error {
	if flagChangeEntityID == "" {
		output.PrintError(os.Stderr, "missing_parameter",
			"--bug-id is required",
			"Usage: tapd bug related-stories --bug-id <id>")
		os.Exit(output.ExitParamError)
		return nil
	}
	req := &model.GetBugRelatedStoriesRequest{
		WorkspaceID: flagWorkspaceID,
		BugID:       flagChangeEntityID,
	}
	relations, err := apiClient.GetBugRelatedStories(context.Background(), req)
	if err != nil {
		output.PrintError(os.Stderr, "api_error", err.Error(), "")
		os.Exit(output.ExitAPIError)
		return nil
	}
	return output.PrintJSON(os.Stdout, relations, !flagPretty)
}

func runBugTemplateList(cmd *cobra.Command, args []string) error {
	req := &model.WorkspaceIDRequest{
		WorkspaceID: flagWorkspaceID,
	}
	templates, err := apiClient.ListBugTemplates(context.Background(), req)
	if err != nil {
		output.PrintError(os.Stderr, "api_error", err.Error(), "")
		os.Exit(output.ExitAPIError)
		return nil
	}
	return output.PrintJSON(os.Stdout, templates, !flagPretty)
}
