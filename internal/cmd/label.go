// Package cmd 中的 label.go 实现了标签管理命令
package cmd

import (
	"context"
	"os"

	"github.com/spf13/cobra"
	"github.com/studyzy/tapd-ai-cli/internal/output"
	"github.com/studyzy/tapd-sdk-go/model"
)

var (
	flagLabelColor string
)

// labelCmd 是 label 父命令
var labelCmd = &cobra.Command{
	Use:   "label",
	Short: "标签管理",
}

var labelListCmd = &cobra.Command{
	Use:   "list",
	Short: "查询标签列表",
	RunE:  runLabelList,
}

var labelAddCmd = &cobra.Command{
	Use:   "add",
	Short: "创建标签",
	RunE:  runLabelAdd,
}

var labelUpdateCmd = &cobra.Command{
	Use:   "update <label_id>",
	Short: "更新标签",
	Args:  cobra.ExactArgs(1),
	RunE:  runLabelUpdate,
}

var labelCountCmd = &cobra.Command{
	Use:   "count",
	Short: "查询标签数量",
	RunE:  runLabelCount,
}

func init() {
	labelListCmd.Flags().StringVar(&flagName, "name", "", "按名称筛选（支持模糊匹配）")
	labelListCmd.Flags().IntVar(&flagLimit, "limit", 30, "返回数量限制（最大 200）")
	labelListCmd.Flags().IntVar(&flagPage, "page", 1, "页码")
	labelListCmd.Flags().StringArrayVar(&flagFilter, "filter", nil, filterFlagDesc)

	labelAddCmd.Flags().StringVar(&flagName, "name", "", "标签名称（必需，不能包含英文竖线）")
	labelAddCmd.Flags().StringVar(&flagLabelColor, "color", "", "颜色标识（1|2|3|4）")

	labelUpdateCmd.Flags().StringVar(&flagLabelColor, "color", "", "颜色标识（1|2|3|4）")

	labelCountCmd.Flags().StringVar(&flagName, "name", "", "按名称筛选（支持模糊匹配）")

	labelCmd.AddCommand(labelListCmd, labelAddCmd, labelUpdateCmd, labelCountCmd)
	rootCmd.AddCommand(labelCmd)
}

func runLabelList(cmd *cobra.Command, args []string) error {
	req := &model.QueryLabelRequest{
		WorkspaceID: flagWorkspaceID,
		Name:        flagName,
		Limit:       flagLimit,
		Page:        flagPage,
	}
	labels, err := listWithFilters[model.LabelPool](cmdContext(cmd), apiClient, "/label_pools", req.ToParams(), flagFilter, "LabelPool")
	if err != nil {
		output.PrintError(os.Stderr, "api_error", err.Error(), "")
		os.Exit(output.ExitAPIError)
		return nil
	}
	countReq := &model.CountLabelRequest{
		WorkspaceID: flagWorkspaceID,
		Name:        flagName,
	}
	total, _ := apiClient.CountLabels(context.Background(), countReq)
	resp := &model.ListResponse{
		Items:   labels,
		Total:   total,
		Page:    flagPage,
		Limit:   flagLimit,
		HasMore: total > flagPage*flagLimit,
	}
	return output.PrintJSON(os.Stdout, resp, !flagPretty)
}

func runLabelAdd(cmd *cobra.Command, args []string) error {
	if flagName == "" {
		output.PrintError(os.Stderr, "missing_parameter",
			"--name is required",
			"Usage: tapd label add --name <name> [--color <1|2|3|4>]")
		os.Exit(output.ExitParamError)
		return nil
	}
	req := &model.AddLabelRequest{
		WorkspaceID: flagWorkspaceID,
		Name:        flagName,
		Color:       flagLabelColor,
	}
	result, err := apiClient.AddLabel(context.Background(), req)
	if err != nil {
		output.PrintError(os.Stderr, "api_error", err.Error(), "")
		os.Exit(output.ExitAPIError)
		return nil
	}
	return output.PrintJSON(os.Stdout, result, !flagPretty)
}

func runLabelUpdate(cmd *cobra.Command, args []string) error {
	req := &model.UpdateLabelRequest{
		WorkspaceID: flagWorkspaceID,
		ID:          args[0],
		Color:       flagLabelColor,
	}
	result, err := apiClient.UpdateLabel(context.Background(), req)
	if err != nil {
		output.PrintError(os.Stderr, "api_error", err.Error(), "")
		os.Exit(output.ExitAPIError)
		return nil
	}
	return output.PrintJSON(os.Stdout, result, !flagPretty)
}

func runLabelCount(cmd *cobra.Command, args []string) error {
	req := &model.CountLabelRequest{
		WorkspaceID: flagWorkspaceID,
		Name:        flagName,
	}
	count, err := apiClient.CountLabels(context.Background(), req)
	if err != nil {
		output.PrintError(os.Stderr, "api_error", err.Error(), "")
		os.Exit(output.ExitAPIError)
		return nil
	}
	return output.PrintJSON(os.Stdout, &model.CountResponse{Count: count}, !flagPretty)
}
