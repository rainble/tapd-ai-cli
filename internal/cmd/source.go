// Package cmd 中的 source.go 实现了源码提交关联管理命令
package cmd

import (
	"context"
	"os"

	"github.com/spf13/cobra"
	"github.com/studyzy/tapd-ai-cli/internal/output"
	"github.com/studyzy/tapd-sdk-go/model"
)

var (
	flagMessage  string
	flagWebURL   string
	flagRef      string
	flagObjectID string
	flagCommitID string
)

// sourceCmd 是 source 父命令
var sourceCmd = &cobra.Command{
	Use:   "source",
	Short: "源码提交关联管理",
}

var sourceAddCmd = &cobra.Command{
	Use:   "add",
	Short: "保存 Commit 提交数据",
	RunE:  runSourceAdd,
}

var sourceListCmd = &cobra.Command{
	Use:   "list",
	Short: "获取 GIT 关联提交数据",
	RunE:  runSourceList,
}

var sourceObjectsCmd = &cobra.Command{
	Use:   "objects",
	Short: "获取指定 commit 关联的 TAPD 业务对象",
	RunE:  runSourceObjects,
}

func init() {
	sourceAddCmd.Flags().StringVar(&flagMessage, "message", "", "提交信息（必需）")
	sourceAddCmd.Flags().StringVar(&flagWebURL, "web-url", "", "仓库链接")
	sourceAddCmd.Flags().StringVar(&flagRef, "ref", "", "分支引用")

	sourceListCmd.Flags().StringVar(&flagObjectID, "object-id", "", "TAPD 业务对象 ID（必需）")
	sourceListCmd.Flags().StringVar(&flagEntityType, "type", "", "业务对象类型 story/bug/task（必需）")

	sourceObjectsCmd.Flags().StringVar(&flagCommitID, "commit-id", "", "提交 ID（必需，支持逗号分隔多个）")
	sourceObjectsCmd.Flags().StringVar(&flagEntityType, "type", "", "业务对象类型 story/bug/task（必需）")

	sourceCmd.AddCommand(sourceAddCmd, sourceListCmd, sourceObjectsCmd)
	rootCmd.AddCommand(sourceCmd)
}

func runSourceAdd(cmd *cobra.Command, args []string) error {
	if flagMessage == "" {
		output.PrintError(os.Stderr, "missing_parameter",
			"--message is required",
			"Usage: tapd source add --message <msg> --web-url <url>")
		os.Exit(output.ExitParamError)
		return nil
	}

	req := &model.AddCodeCommitInfoRequest{
		WorkspaceID: flagWorkspaceID,
		Message:     flagMessage,
		Ref:         flagRef,
		RepoURL:     flagWebURL,
	}

	data, err := apiClient.AddCodeCommitInfo(context.Background(), req)
	if err != nil {
		output.PrintError(os.Stderr, "api_error", err.Error(), "")
		os.Exit(output.ExitAPIError)
		return nil
	}
	return output.PrintJSON(os.Stdout, data, !flagPretty)
}

func runSourceList(cmd *cobra.Command, args []string) error {
	if flagObjectID == "" || flagEntityType == "" {
		output.PrintError(os.Stderr, "missing_parameter",
			"--object-id and --type are required",
			"Usage: tapd source list --object-id <id> --type <story|bug|task>")
		os.Exit(output.ExitParamError)
		return nil
	}

	req := &model.GetCodeCommitInfosRequest{
		WorkspaceID: flagWorkspaceID,
		ObjectID:    flagObjectID,
		Type:        flagEntityType,
	}

	data, err := apiClient.GetCodeCommitInfos(context.Background(), req)
	if err != nil {
		output.PrintError(os.Stderr, "api_error", err.Error(), "")
		os.Exit(output.ExitAPIError)
		return nil
	}
	return output.PrintJSON(os.Stdout, data, !flagPretty)
}

func runSourceObjects(cmd *cobra.Command, args []string) error {
	if flagCommitID == "" || flagEntityType == "" {
		output.PrintError(os.Stderr, "missing_parameter",
			"--commit-id and --type are required",
			"Usage: tapd source objects --commit-id <id> --type <story|bug|task>")
		os.Exit(output.ExitParamError)
		return nil
	}

	req := &model.GetCodeCommitObjectsRequest{
		WorkspaceID: flagWorkspaceID,
		CommitID:    flagCommitID,
		EntityType:  flagEntityType,
	}

	data, err := apiClient.GetCodeCommitObjects(context.Background(), req)
	if err != nil {
		output.PrintError(os.Stderr, "api_error", err.Error(), "")
		os.Exit(output.ExitAPIError)
		return nil
	}
	return output.PrintJSON(os.Stdout, data, !flagPretty)
}
