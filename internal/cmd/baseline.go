// Package cmd 中的 baseline.go 实现了基线管理命令
package cmd

import (
	"context"
	"os"

	"github.com/spf13/cobra"
	"github.com/studyzy/tapd-ai-cli/internal/output"
	"github.com/studyzy/tapd-sdk-go/model"
)

var (
	flagVersionID string
)

// baselineCmd 是基线管理父命令
var baselineCmd = &cobra.Command{
	Use:   "baseline",
	Short: "基线管理",
}

var baselineListCmd = &cobra.Command{
	Use:   "list",
	Short: "查询基线列表",
	RunE:  runBaselineList,
}

var baselineCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "创建基线",
	RunE:  runBaselineCreate,
}

var baselineUpdateCmd = &cobra.Command{
	Use:   "update <baseline_id>",
	Short: "更新基线",
	Args:  cobra.ExactArgs(1),
	RunE:  runBaselineUpdate,
}

var baselineCountCmd = &cobra.Command{
	Use:   "count",
	Short: "查询基线数量",
	RunE:  runBaselineCount,
}

func init() {
	baselineListCmd.Flags().StringVar(&flagName, "name", "", "按名称筛选")
	baselineListCmd.Flags().IntVar(&flagLimit, "limit", 30, "返回数量限制")
	baselineListCmd.Flags().IntVar(&flagPage, "page", 1, "页码")

	baselineCreateCmd.Flags().StringVar(&flagName, "name", "", "基线名称")
	baselineCreateCmd.Flags().StringVar(&flagVersionID, "version-id", "", "关联版本 ID")
	baselineCreateCmd.Flags().StringVar(&flagDescription, "description", "", "基线描述")

	baselineUpdateCmd.Flags().StringVar(&flagName, "name", "", "新名称")
	baselineUpdateCmd.Flags().StringVar(&flagDescription, "description", "", "新描述")

	baselineCmd.AddCommand(baselineListCmd, baselineCreateCmd, baselineUpdateCmd, baselineCountCmd)
	rootCmd.AddCommand(baselineCmd)
}

func runBaselineList(cmd *cobra.Command, args []string) error {
	req := &model.GetBaselinesRequest{
		WorkspaceID: flagWorkspaceID,
		Name:        flagName,
		Limit:       flagLimit,
		Page:        flagPage,
	}
	baselines, err := apiClient.GetBaselines(context.Background(), req)
	if err != nil {
		output.PrintError(os.Stderr, "api_error", err.Error(), "")
		os.Exit(output.ExitAPIError)
		return nil
	}
	total, _ := apiClient.CountBaselines(context.Background(), &model.CountBaselinesRequest{
		WorkspaceID: flagWorkspaceID,
	})
	resp := &model.ListResponse{
		Items:   baselines,
		Total:   total,
		Page:    flagPage,
		Limit:   flagLimit,
		HasMore: total > flagPage*flagLimit,
	}
	return output.PrintJSON(os.Stdout, resp, !flagPretty)
}

func runBaselineCreate(cmd *cobra.Command, args []string) error {
	req := &model.CreateBaselineRequest{
		WorkspaceID: flagWorkspaceID,
		Name:        flagName,
		VersionID:   flagVersionID,
		Description: flagDescription,
	}
	baseline, err := apiClient.CreateBaseline(context.Background(), req)
	if err != nil {
		output.PrintError(os.Stderr, "api_error", err.Error(), "")
		os.Exit(output.ExitAPIError)
		return nil
	}
	return printSuccessResponse(baseline.ID, "", baseline.WorkspaceID)
}

func runBaselineUpdate(cmd *cobra.Command, args []string) error {
	req := &model.UpdateBaselineRequest{
		WorkspaceID: flagWorkspaceID,
		ID:          args[0],
		Name:        flagName,
		Description: flagDescription,
	}
	baseline, err := apiClient.UpdateBaseline(context.Background(), req)
	if err != nil {
		output.PrintError(os.Stderr, "api_error", err.Error(), "")
		os.Exit(output.ExitAPIError)
		return nil
	}
	return printSuccessResponse(baseline.ID, "", baseline.WorkspaceID)
}

func runBaselineCount(cmd *cobra.Command, args []string) error {
	req := &model.CountBaselinesRequest{
		WorkspaceID: flagWorkspaceID,
	}
	count, err := apiClient.CountBaselines(context.Background(), req)
	if err != nil {
		output.PrintError(os.Stderr, "api_error", err.Error(), "")
		os.Exit(output.ExitAPIError)
		return nil
	}
	return output.PrintJSON(os.Stdout, &model.CountResponse{Count: count}, !flagPretty)
}
