// Package cmd 中的 version_mgmt.go 实现了版本管理命令
package cmd

import (
	"context"
	"os"

	"github.com/spf13/cobra"
	"github.com/studyzy/tapd-ai-cli/internal/output"
	"github.com/studyzy/tapd-sdk-go/model"
)

var (
	flagVersionModifier string
)

// appVersionCmd 是版本管理父命令
var appVersionCmd = &cobra.Command{
	Use:   "app-version",
	Short: "版本管理",
}

var appVersionListCmd = &cobra.Command{
	Use:   "list",
	Short: "查询版本列表",
	RunE:  runAppVersionList,
}

var appVersionCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "创建版本",
	RunE:  runAppVersionCreate,
}

var appVersionUpdateCmd = &cobra.Command{
	Use:   "update <version_id>",
	Short: "更新版本",
	Args:  cobra.ExactArgs(1),
	RunE:  runAppVersionUpdate,
}

var appVersionCountCmd = &cobra.Command{
	Use:   "count",
	Short: "查询版本数量",
	RunE:  runAppVersionCount,
}

func init() {
	appVersionListCmd.Flags().StringVar(&flagName, "name", "", "按名称筛选")
	appVersionListCmd.Flags().StringVar(&flagStatus, "status", "", "按状态筛选（Closed/Unclosed）")
	appVersionListCmd.Flags().IntVar(&flagLimit, "limit", 30, "返回数量限制")
	appVersionListCmd.Flags().IntVar(&flagPage, "page", 1, "页码")

	appVersionCreateCmd.Flags().StringVar(&flagName, "name", "", "版本名称（必需）")
	appVersionCreateCmd.Flags().StringVar(&flagCreator, "creator", "", "创建人（必需）")
	appVersionCreateCmd.Flags().StringVar(&flagDescription, "description", "", "版本描述")

	appVersionUpdateCmd.Flags().StringVar(&flagVersionModifier, "modifier", "", "当前处理人（必需）")
	appVersionUpdateCmd.Flags().StringVar(&flagName, "name", "", "新名称")
	appVersionUpdateCmd.Flags().StringVar(&flagDescription, "description", "", "新描述")

	appVersionCmd.AddCommand(appVersionListCmd, appVersionCreateCmd, appVersionUpdateCmd, appVersionCountCmd)
	rootCmd.AddCommand(appVersionCmd)
}

func runAppVersionList(cmd *cobra.Command, args []string) error {
	req := &model.GetVersionsRequest{
		WorkspaceID: flagWorkspaceID,
		Name:        flagName,
		Status:      flagStatus,
		Limit:       flagLimit,
		Page:        flagPage,
	}
	versions, err := apiClient.GetVersions(context.Background(), req)
	if err != nil {
		output.PrintError(os.Stderr, "api_error", err.Error(), "")
		os.Exit(output.ExitAPIError)
		return nil
	}
	total, _ := apiClient.CountVersions(context.Background(), &model.CountVersionsRequest{
		WorkspaceID: flagWorkspaceID,
	})
	resp := &model.ListResponse{
		Items:   versions,
		Total:   total,
		Page:    flagPage,
		Limit:   flagLimit,
		HasMore: total > flagPage*flagLimit,
	}
	return output.PrintJSON(os.Stdout, resp, !flagPretty)
}

func runAppVersionCreate(cmd *cobra.Command, args []string) error {
	if flagName == "" {
		output.PrintError(os.Stderr, "missing_parameter", "--name is required", "Usage: tapd app-version create --name <name> --creator <user>")
		os.Exit(output.ExitParamError)
		return nil
	}
	if flagCreator == "" {
		output.PrintError(os.Stderr, "missing_parameter", "--creator is required", "Usage: tapd app-version create --name <name> --creator <user>")
		os.Exit(output.ExitParamError)
		return nil
	}
	req := &model.CreateVersionRequest{
		WorkspaceID: flagWorkspaceID,
		Name:        flagName,
		Creator:     flagCreator,
		Description: flagDescription,
	}
	version, err := apiClient.CreateVersion(context.Background(), req)
	if err != nil {
		output.PrintError(os.Stderr, "api_error", err.Error(), "")
		os.Exit(output.ExitAPIError)
		return nil
	}
	return printSuccessResponse(version.ID, "", version.WorkspaceID)
}

func runAppVersionUpdate(cmd *cobra.Command, args []string) error {
	if flagVersionModifier == "" {
		output.PrintError(os.Stderr, "missing_parameter", "--modifier is required", "Usage: tapd app-version update <id> --modifier <user>")
		os.Exit(output.ExitParamError)
		return nil
	}
	req := &model.UpdateVersionRequest{
		WorkspaceID: flagWorkspaceID,
		ID:          args[0],
		Modifier:    flagVersionModifier,
		Name:        flagName,
		Description: flagDescription,
	}
	version, err := apiClient.UpdateVersion(context.Background(), req)
	if err != nil {
		output.PrintError(os.Stderr, "api_error", err.Error(), "")
		os.Exit(output.ExitAPIError)
		return nil
	}
	return printSuccessResponse(version.ID, "", version.WorkspaceID)
}

func runAppVersionCount(cmd *cobra.Command, args []string) error {
	req := &model.CountVersionsRequest{
		WorkspaceID: flagWorkspaceID,
	}
	count, err := apiClient.CountVersions(context.Background(), req)
	if err != nil {
		output.PrintError(os.Stderr, "api_error", err.Error(), "")
		os.Exit(output.ExitAPIError)
		return nil
	}
	return output.PrintJSON(os.Stdout, &model.CountResponse{Count: count}, !flagPretty)
}
