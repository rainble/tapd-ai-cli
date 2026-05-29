// Package cmd 中的 module.go 实现了模块管理命令
package cmd

import (
	"context"
	"os"

	"github.com/spf13/cobra"
	"github.com/studyzy/tapd-ai-cli/internal/output"
	"github.com/studyzy/tapd-sdk-go/model"
)

// moduleCmd 是 module 父命令
var moduleCmd = &cobra.Command{
	Use:   "module",
	Short: "模块管理",
}

var moduleListCmd = &cobra.Command{
	Use:   "list",
	Short: "查询模块列表",
	RunE:  runModuleList,
}

var moduleCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "创建模块",
	RunE:  runModuleCreate,
}

var moduleUpdateCmd = &cobra.Command{
	Use:   "update <module_id>",
	Short: "更新模块",
	Args:  cobra.ExactArgs(1),
	RunE:  runModuleUpdate,
}

var moduleCountCmd = &cobra.Command{
	Use:   "count",
	Short: "查询模块数量",
	RunE:  runModuleCount,
}

func init() {
	moduleListCmd.Flags().StringVar(&flagName, "name", "", "按名称筛选")
	moduleListCmd.Flags().IntVar(&flagLimit, "limit", 30, "返回数量限制")
	moduleListCmd.Flags().IntVar(&flagPage, "page", 1, "页码")
	moduleListCmd.Flags().StringArrayVar(&flagFilter, "filter", nil, filterFlagDesc)

	moduleCreateCmd.Flags().StringVar(&flagName, "name", "", "模块名称（必需）")
	moduleCreateCmd.Flags().StringVar(&flagDescription, "description", "", "模块描述")
	moduleCreateCmd.Flags().StringVar(&flagOwner, "owner", "", "负责人")

	moduleUpdateCmd.Flags().StringVar(&flagName, "name", "", "新名称")
	moduleUpdateCmd.Flags().StringVar(&flagDescription, "description", "", "新描述")
	moduleUpdateCmd.Flags().StringVar(&flagOwner, "owner", "", "新负责人")

	moduleCmd.AddCommand(moduleListCmd, moduleCreateCmd, moduleUpdateCmd, moduleCountCmd)
	rootCmd.AddCommand(moduleCmd)
}

func runModuleList(cmd *cobra.Command, args []string) error {
	req := &model.GetModulesRequest{
		WorkspaceID: flagWorkspaceID,
		Name:        flagName,
		Limit:       flagLimit,
		Page:        flagPage,
	}
	modules, err := listWithFilters[model.Module](cmdContext(cmd), apiClient, "/modules", req.ToParams(), flagFilter, "Module")
	if err != nil {
		output.PrintError(os.Stderr, "api_error", err.Error(), "")
		os.Exit(output.ExitAPIError)
		return nil
	}
	countReq := &model.CountModulesRequest{
		WorkspaceID: flagWorkspaceID,
		Name:        flagName,
	}
	total, _ := apiClient.CountModules(context.Background(), countReq)
	resp := &model.ListResponse{
		Items:   modules,
		Total:   total,
		Page:    flagPage,
		Limit:   flagLimit,
		HasMore: total > flagPage*flagLimit,
	}
	return output.PrintJSON(os.Stdout, resp, !flagPretty)
}

func runModuleCreate(cmd *cobra.Command, args []string) error {
	if flagName == "" {
		output.PrintError(os.Stderr, "missing_parameter",
			"--name is required",
			"Usage: tapd module create --name <name>")
		os.Exit(output.ExitParamError)
		return nil
	}
	req := &model.CreateModuleRequest{
		WorkspaceID: flagWorkspaceID,
		Name:        flagName,
		Description: flagDescription,
		Owner:       flagOwner,
	}
	result, err := apiClient.CreateModule(context.Background(), req)
	if err != nil {
		output.PrintError(os.Stderr, "api_error", err.Error(), "")
		os.Exit(output.ExitAPIError)
		return nil
	}
	return output.PrintJSON(os.Stdout, result, !flagPretty)
}

func runModuleUpdate(cmd *cobra.Command, args []string) error {
	req := &model.UpdateModuleRequest{
		WorkspaceID: flagWorkspaceID,
		ID:          args[0],
		Name:        flagName,
		Description: flagDescription,
		Owner:       flagOwner,
	}
	result, err := apiClient.UpdateModule(context.Background(), req)
	if err != nil {
		output.PrintError(os.Stderr, "api_error", err.Error(), "")
		os.Exit(output.ExitAPIError)
		return nil
	}
	return output.PrintJSON(os.Stdout, result, !flagPretty)
}

func runModuleCount(cmd *cobra.Command, args []string) error {
	req := &model.CountModulesRequest{
		WorkspaceID: flagWorkspaceID,
	}
	count, err := apiClient.CountModules(context.Background(), req)
	if err != nil {
		output.PrintError(os.Stderr, "api_error", err.Error(), "")
		os.Exit(output.ExitAPIError)
		return nil
	}
	return output.PrintJSON(os.Stdout, &model.CountResponse{Count: count}, !flagPretty)
}
