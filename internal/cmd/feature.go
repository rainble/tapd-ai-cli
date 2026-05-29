// Package cmd 中的 feature.go 实现了特性管理命令
package cmd

import (
	"context"
	"os"

	"github.com/spf13/cobra"
	"github.com/studyzy/tapd-ai-cli/internal/output"
	"github.com/studyzy/tapd-sdk-go/model"
)

// featureCmd 是特性管理父命令
var featureCmd = &cobra.Command{
	Use:   "feature",
	Short: "特性管理",
}

var featureListCmd = &cobra.Command{
	Use:   "list",
	Short: "查询特性列表",
	RunE:  runFeatureList,
}

var featureCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "创建特性",
	RunE:  runFeatureCreate,
}

var featureUpdateCmd = &cobra.Command{
	Use:   "update <feature_id>",
	Short: "更新特性",
	Args:  cobra.ExactArgs(1),
	RunE:  runFeatureUpdate,
}

var featureCountCmd = &cobra.Command{
	Use:   "count",
	Short: "查询特性数量",
	RunE:  runFeatureCount,
}

func init() {
	featureListCmd.Flags().StringVar(&flagName, "name", "", "按名称筛选")
	featureListCmd.Flags().IntVar(&flagLimit, "limit", 30, "返回数量限制")
	featureListCmd.Flags().IntVar(&flagPage, "page", 1, "页码")

	featureCreateCmd.Flags().StringVar(&flagName, "name", "", "特性名称（必需）")
	featureCreateCmd.Flags().StringVar(&flagDescription, "description", "", "特性描述")

	featureUpdateCmd.Flags().StringVar(&flagName, "name", "", "新名称")
	featureUpdateCmd.Flags().StringVar(&flagDescription, "description", "", "新描述")
	featureUpdateCmd.Flags().StringVar(&flagOwner, "owner", "", "新负责人")

	featureCmd.AddCommand(featureListCmd, featureCreateCmd, featureUpdateCmd, featureCountCmd)
	rootCmd.AddCommand(featureCmd)
}

func runFeatureList(cmd *cobra.Command, args []string) error {
	req := &model.GetFeaturesRequest{
		WorkspaceID: flagWorkspaceID,
		Name:        flagName,
		Limit:       flagLimit,
		Page:        flagPage,
	}
	features, err := apiClient.GetFeatures(context.Background(), req)
	if err != nil {
		output.PrintError(os.Stderr, "api_error", err.Error(), "")
		os.Exit(output.ExitAPIError)
		return nil
	}
	total, _ := apiClient.CountFeatures(context.Background(), &model.CountFeaturesRequest{
		WorkspaceID: flagWorkspaceID,
	})
	resp := &model.ListResponse{
		Items:   features,
		Total:   total,
		Page:    flagPage,
		Limit:   flagLimit,
		HasMore: total > flagPage*flagLimit,
	}
	return output.PrintJSON(os.Stdout, resp, !flagPretty)
}

func runFeatureCreate(cmd *cobra.Command, args []string) error {
	if flagName == "" {
		output.PrintError(os.Stderr, "missing_parameter", "--name is required", "Usage: tapd feature create --name <name>")
		os.Exit(output.ExitParamError)
		return nil
	}
	req := &model.CreateFeatureRequest{
		WorkspaceID: flagWorkspaceID,
		Name:        flagName,
		Description: flagDescription,
	}
	feature, err := apiClient.CreateFeature(context.Background(), req)
	if err != nil {
		output.PrintError(os.Stderr, "api_error", err.Error(), "")
		os.Exit(output.ExitAPIError)
		return nil
	}
	return printSuccessResponse(feature.ID, "", feature.WorkspaceID)
}

func runFeatureUpdate(cmd *cobra.Command, args []string) error {
	req := &model.UpdateFeatureRequest{
		WorkspaceID: flagWorkspaceID,
		ID:          args[0],
		Name:        flagName,
		Description: flagDescription,
		Owner:       flagOwner,
	}
	feature, err := apiClient.UpdateFeature(context.Background(), req)
	if err != nil {
		output.PrintError(os.Stderr, "api_error", err.Error(), "")
		os.Exit(output.ExitAPIError)
		return nil
	}
	return printSuccessResponse(feature.ID, "", feature.WorkspaceID)
}

func runFeatureCount(cmd *cobra.Command, args []string) error {
	req := &model.CountFeaturesRequest{
		WorkspaceID: flagWorkspaceID,
	}
	count, err := apiClient.CountFeatures(context.Background(), req)
	if err != nil {
		output.PrintError(os.Stderr, "api_error", err.Error(), "")
		os.Exit(output.ExitAPIError)
		return nil
	}
	return output.PrintJSON(os.Stdout, &model.CountResponse{Count: count}, !flagPretty)
}
