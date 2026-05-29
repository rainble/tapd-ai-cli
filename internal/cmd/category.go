// Package cmd 中的 category.go 实现了需求分类管理命令
package cmd

import (
	"context"
	"os"

	"github.com/spf13/cobra"
	"github.com/studyzy/tapd-ai-cli/internal/output"
	"github.com/studyzy/tapd-sdk-go/model"
)

var flagCategoryName string

// categoryCmd 是 category 父命令
var categoryCmd = &cobra.Command{
	Use:   "category",
	Short: "需求分类管理",
}

var categoryListCmd = &cobra.Command{
	Use:   "list",
	Short: "查询需求分类列表",
	RunE:  runCategoryList,
}

var categoryCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "创建需求分类",
	RunE:  runCategoryCreate,
}

var categoryUpdateCmd = &cobra.Command{
	Use:   "update <category_id>",
	Short: "更新需求分类",
	Args:  cobra.ExactArgs(1),
	RunE:  runCategoryUpdate,
}

var categoryCountCmd = &cobra.Command{
	Use:   "count",
	Short: "查询需求分类数量",
	RunE:  runCategoryCount,
}

func init() {
	categoryListCmd.Flags().StringVar(&flagCategoryName, "name", "", "按名称筛选（支持模糊匹配，如 %搜索词%）")
	categoryListCmd.Flags().StringArrayVar(&flagFilter, "filter", nil, filterFlagDesc)

	categoryCreateCmd.Flags().StringVar(&flagName, "name", "", "分类名称（必需）")
	categoryCreateCmd.Flags().StringVar(&flagParentID, "parent-id", "", "父分类 ID")
	categoryCreateCmd.Flags().StringVar(&flagDescription, "description", "", "分类描述")

	categoryUpdateCmd.Flags().StringVar(&flagName, "name", "", "分类名称")
	categoryUpdateCmd.Flags().StringVar(&flagDescription, "description", "", "分类描述")

	categoryCmd.AddCommand(categoryListCmd, categoryCreateCmd, categoryUpdateCmd, categoryCountCmd)
	rootCmd.AddCommand(categoryCmd)
}

func runCategoryList(cmd *cobra.Command, args []string) error {
	params := map[string]string{
		"workspace_id": flagWorkspaceID,
	}
	addOptionalParam(params, "name", flagCategoryName)

	categories, err := listWithFilters[model.Category](cmdContext(cmd), apiClient, "/story_categories", params, flagFilter, "Category")
	if err != nil {
		output.PrintError(os.Stderr, "api_error", err.Error(), "")
		os.Exit(output.ExitAPIError)
		return nil
	}

	resp := &model.ListResponse{
		Items: categories,
		Total: len(categories),
	}
	return output.PrintJSON(os.Stdout, resp, !flagPretty)
}

func runCategoryCreate(cmd *cobra.Command, args []string) error {
	if flagName == "" {
		output.PrintError(os.Stderr, "missing_parameter",
			"--name is required",
			"Usage: tapd category create --name <name>")
		os.Exit(output.ExitParamError)
		return nil
	}

	req := &model.CreateStoryCategoryRequest{
		WorkspaceID: flagWorkspaceID,
		Name:        flagName,
		Description: flagDescription,
		ParentID:    flagParentID,
	}

	category, err := apiClient.CreateStoryCategory(context.Background(), req)
	if err != nil {
		output.PrintError(os.Stderr, "api_error", err.Error(), "")
		os.Exit(output.ExitAPIError)
		return nil
	}
	return output.PrintJSON(os.Stdout, category, !flagPretty)
}

func runCategoryUpdate(cmd *cobra.Command, args []string) error {
	categoryID := expandShortID(args[0], flagWorkspaceID)

	req := &model.UpdateStoryCategoryRequest{
		WorkspaceID: flagWorkspaceID,
		ID:          categoryID,
		Name:        flagName,
		Description: flagDescription,
	}

	category, err := apiClient.UpdateStoryCategory(context.Background(), req)
	if err != nil {
		output.PrintError(os.Stderr, "api_error", err.Error(), "")
		os.Exit(output.ExitAPIError)
		return nil
	}
	return output.PrintJSON(os.Stdout, category, !flagPretty)
}

func runCategoryCount(cmd *cobra.Command, args []string) error {
	req := &model.CountStoryCategoriesRequest{
		WorkspaceID: flagWorkspaceID,
	}

	count, err := apiClient.CountStoryCategories(context.Background(), req)
	if err != nil {
		output.PrintError(os.Stderr, "api_error", err.Error(), "")
		os.Exit(output.ExitAPIError)
		return nil
	}
	return output.PrintJSON(os.Stdout, &model.CountResponse{Count: count}, !flagPretty)
}
