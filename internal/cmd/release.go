// Package cmd 中的 release.go 实现了发布计划管理命令
package cmd

import (
	"context"
	"os"

	"github.com/spf13/cobra"
	"github.com/studyzy/tapd-ai-cli/internal/output"
	"github.com/studyzy/tapd-sdk-go/model"
)

// releaseCmd 是 release 父命令
var releaseCmd = &cobra.Command{
	Use:   "release",
	Short: "发布计划管理",
}

var releaseListCmd = &cobra.Command{
	Use:   "list",
	Short: "查询发布计划列表",
	RunE:  runReleaseList,
}

var releaseCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "创建发布计划",
	RunE:  runReleaseCreate,
}

var releaseUpdateCmd = &cobra.Command{
	Use:   "update <release_id>",
	Short: "更新发布计划",
	Args:  cobra.ExactArgs(1),
	RunE:  runReleaseUpdate,
}

var releaseCountCmd = &cobra.Command{
	Use:   "count",
	Short: "查询发布计划数量",
	RunE:  runReleaseCount,
}

func init() {
	releaseListCmd.Flags().StringArrayVar(&flagFilter, "filter", nil, filterFlagDesc)

	releaseCreateCmd.Flags().StringVar(&flagName, "name", "", "发布计划名称（必需）")
	releaseCreateCmd.Flags().StringVar(&flagStartDate, "startdate", "", "开始日期（格式：2006-01-02）")
	releaseCreateCmd.Flags().StringVar(&flagEndDate, "enddate", "", "结束日期（格式：2006-01-02）")
	releaseCreateCmd.Flags().StringVar(&flagDescription, "description", "", "描述")

	releaseUpdateCmd.Flags().StringVar(&flagName, "name", "", "发布计划名称")
	releaseUpdateCmd.Flags().StringVar(&flagStatus, "status", "", "状态")
	releaseUpdateCmd.Flags().StringVar(&flagStartDate, "startdate", "", "开始日期（格式：2006-01-02）")
	releaseUpdateCmd.Flags().StringVar(&flagEndDate, "enddate", "", "结束日期（格式：2006-01-02）")

	releaseCmd.AddCommand(releaseListCmd, releaseCreateCmd, releaseUpdateCmd, releaseCountCmd)
	rootCmd.AddCommand(releaseCmd)
}

func runReleaseList(cmd *cobra.Command, args []string) error {
	req := &model.WorkspaceIDRequest{
		WorkspaceID: flagWorkspaceID,
	}

	releases, err := listWithFilters[model.Release](cmdContext(cmd), apiClient, "/releases", req.ToParams(), flagFilter, "Release")
	if err != nil {
		output.PrintError(os.Stderr, "api_error", err.Error(), "")
		os.Exit(output.ExitAPIError)
		return nil
	}
	return output.PrintJSON(os.Stdout, releases, !flagPretty)
}

func runReleaseCreate(cmd *cobra.Command, args []string) error {
	if flagName == "" {
		output.PrintError(os.Stderr, "missing_parameter",
			"--name is required",
			"Usage: tapd release create --name <name> --startdate <date> --enddate <date>")
		os.Exit(output.ExitParamError)
		return nil
	}

	req := &model.CreateReleaseRequest{
		WorkspaceID: flagWorkspaceID,
		Name:        flagName,
		Description: flagDescription,
		StartDate:   flagStartDate,
		EndDate:     flagEndDate,
	}

	release, err := apiClient.CreateRelease(context.Background(), req)
	if err != nil {
		output.PrintError(os.Stderr, "api_error", err.Error(), "")
		os.Exit(output.ExitAPIError)
		return nil
	}
	return output.PrintJSON(os.Stdout, release, !flagPretty)
}

func runReleaseUpdate(cmd *cobra.Command, args []string) error {
	releaseID := expandShortID(args[0], flagWorkspaceID)

	req := &model.UpdateReleaseRequest{
		WorkspaceID: flagWorkspaceID,
		ID:          releaseID,
		Name:        flagName,
		Status:      flagStatus,
		StartDate:   flagStartDate,
		EndDate:     flagEndDate,
	}

	release, err := apiClient.UpdateRelease(context.Background(), req)
	if err != nil {
		output.PrintError(os.Stderr, "api_error", err.Error(), "")
		os.Exit(output.ExitAPIError)
		return nil
	}
	return output.PrintJSON(os.Stdout, release, !flagPretty)
}

func runReleaseCount(cmd *cobra.Command, args []string) error {
	req := &model.CountReleasesRequest{
		WorkspaceID: flagWorkspaceID,
	}

	count, err := apiClient.CountReleases(context.Background(), req)
	if err != nil {
		output.PrintError(os.Stderr, "api_error", err.Error(), "")
		os.Exit(output.ExitAPIError)
		return nil
	}
	return output.PrintJSON(os.Stdout, &model.CountResponse{Count: count}, !flagPretty)
}
