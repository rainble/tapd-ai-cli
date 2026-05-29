// Package cmd 中的 workflow.go 实现了工作流状态管理命令
package cmd

import (
	"context"
	"os"

	"github.com/spf13/cobra"
	"github.com/studyzy/tapd-ai-cli/internal/output"
	"github.com/studyzy/tapd-sdk-go/model"
)

var (
	flagSystem         string
	flagWorkitemTypeID string
)

// workflowCmd 是 workflow 父命令
var workflowCmd = &cobra.Command{
	Use:   "workflow",
	Short: "工作流状态管理",
}

var workflowTransitionsCmd = &cobra.Command{
	Use:   "transitions",
	Short: "获取状态流转规则",
	RunE:  runWorkflowTransitions,
}

var workflowStatusMapCmd = &cobra.Command{
	Use:   "status-map",
	Short: "获取状态中英文映射",
	RunE:  runWorkflowStatusMap,
}

var workflowLastStepsCmd = &cobra.Command{
	Use:   "last-steps",
	Short: "获取结束状态",
	RunE:  runWorkflowLastSteps,
}

var workflowFirstStepCmd = &cobra.Command{
	Use:   "first-step",
	Short: "获取工作流起始状态",
	RunE:  runWorkflowFirstStep,
}

var workflowListCmd = &cobra.Command{
	Use:   "list",
	Short: "获取工作流列表",
	RunE:  runWorkflowList,
}

func init() {
	workflowTransitionsCmd.Flags().StringVar(&flagSystem, "system", "", "系统名（story|bug，必需）")
	workflowTransitionsCmd.Flags().StringVar(&flagWorkitemTypeID, "workitem-type-id", "", "需求类别 ID（必需）")

	workflowStatusMapCmd.Flags().StringVar(&flagSystem, "system", "", "系统名（story|bug，必需）")
	workflowStatusMapCmd.Flags().StringVar(&flagWorkitemTypeID, "workitem-type-id", "", "需求类别 ID（必需）")

	workflowLastStepsCmd.Flags().StringVar(&flagSystem, "system", "", "系统名（story|bug，必需）")
	workflowLastStepsCmd.Flags().StringVar(&flagWorkitemTypeID, "workitem-type-id", "", "需求类别 ID")

	workflowFirstStepCmd.Flags().StringVar(&flagSystem, "system", "", "系统名（story|bug，必需）")
	workflowFirstStepCmd.Flags().StringVar(&flagWorkitemTypeID, "workitem-type-id", "", "需求类别 ID")

	workflowListCmd.Flags().StringVar(&flagSystem, "system", "", "系统名（story|bug，必需）")
	workflowListCmd.Flags().StringVar(&flagWorkitemTypeID, "workitem-type-id", "", "需求类别 ID")

	workflowCmd.AddCommand(workflowTransitionsCmd, workflowStatusMapCmd, workflowLastStepsCmd, workflowFirstStepCmd, workflowListCmd)
	rootCmd.AddCommand(workflowCmd)
}

func runWorkflowTransitions(cmd *cobra.Command, args []string) error {
	if flagSystem == "" || flagWorkitemTypeID == "" {
		output.PrintError(os.Stderr, "missing_parameter",
			"--system and --workitem-type-id are required",
			"Usage: tapd workflow transitions --system <story|bug> --workitem-type-id <id>")
		os.Exit(output.ExitParamError)
		return nil
	}

	req := &model.WorkflowRequest{
		WorkspaceID:    flagWorkspaceID,
		System:         flagSystem,
		WorkitemTypeID: flagWorkitemTypeID,
	}

	data, err := apiClient.GetWorkflowTransitions(context.Background(), req)
	if err != nil {
		output.PrintError(os.Stderr, "api_error", err.Error(), "")
		os.Exit(output.ExitAPIError)
		return nil
	}
	return output.PrintJSON(os.Stdout, data, !flagPretty)
}

func runWorkflowStatusMap(cmd *cobra.Command, args []string) error {
	if flagSystem == "" || flagWorkitemTypeID == "" {
		output.PrintError(os.Stderr, "missing_parameter",
			"--system and --workitem-type-id are required",
			"Usage: tapd workflow status-map --system <story|bug> --workitem-type-id <id>")
		os.Exit(output.ExitParamError)
		return nil
	}

	req := &model.WorkflowRequest{
		WorkspaceID:    flagWorkspaceID,
		System:         flagSystem,
		WorkitemTypeID: flagWorkitemTypeID,
	}

	data, err := apiClient.GetWorkflowStatusMap(context.Background(), req)
	if err != nil {
		output.PrintError(os.Stderr, "api_error", err.Error(), "")
		os.Exit(output.ExitAPIError)
		return nil
	}
	return output.PrintJSON(os.Stdout, data, !flagPretty)
}

func runWorkflowLastSteps(cmd *cobra.Command, args []string) error {
	if flagSystem == "" {
		output.PrintError(os.Stderr, "missing_parameter",
			"--system is required",
			"Usage: tapd workflow last-steps --system <story|bug>")
		os.Exit(output.ExitParamError)
		return nil
	}

	req := &model.WorkflowRequest{
		WorkspaceID:    flagWorkspaceID,
		System:         flagSystem,
		WorkitemTypeID: flagWorkitemTypeID,
	}

	data, err := apiClient.GetWorkflowLastSteps(context.Background(), req)
	if err != nil {
		output.PrintError(os.Stderr, "api_error", err.Error(), "")
		os.Exit(output.ExitAPIError)
		return nil
	}
	return output.PrintJSON(os.Stdout, data, !flagPretty)
}

func runWorkflowFirstStep(cmd *cobra.Command, args []string) error {
	if flagSystem == "" {
		output.PrintError(os.Stderr, "missing_parameter",
			"--system is required",
			"Usage: tapd workflow first-step --system <story|bug>")
		os.Exit(output.ExitParamError)
		return nil
	}

	req := &model.WorkflowRequest{
		WorkspaceID:    flagWorkspaceID,
		System:         flagSystem,
		WorkitemTypeID: flagWorkitemTypeID,
	}

	data, err := apiClient.GetWorkflowFirstStep(context.Background(), req)
	if err != nil {
		output.PrintError(os.Stderr, "api_error", err.Error(), "")
		os.Exit(output.ExitAPIError)
		return nil
	}
	return output.PrintJSON(os.Stdout, data, !flagPretty)
}

func runWorkflowList(cmd *cobra.Command, args []string) error {
	if flagSystem == "" {
		output.PrintError(os.Stderr, "missing_parameter",
			"--system is required",
			"Usage: tapd workflow list --system <story|bug>")
		os.Exit(output.ExitParamError)
		return nil
	}

	req := &model.WorkflowRequest{
		WorkspaceID:    flagWorkspaceID,
		System:         flagSystem,
		WorkitemTypeID: flagWorkitemTypeID,
	}

	data, err := apiClient.GetWorkflows(context.Background(), req)
	if err != nil {
		output.PrintError(os.Stderr, "api_error", err.Error(), "")
		os.Exit(output.ExitAPIError)
		return nil
	}
	return output.PrintJSON(os.Stdout, data, !flagPretty)
}
