// Package cmd 中的 workspace.go 实现了工作区管理命令
package cmd

import (
	"context"
	"os"

	"github.com/spf13/cobra"
	"github.com/studyzy/tapd-ai-cli/internal/config"
	"github.com/studyzy/tapd-ai-cli/internal/output"
	"github.com/studyzy/tapd-sdk-go/model"
)

// workspaceCmd 是 workspace 父命令
var workspaceCmd = &cobra.Command{
	Use:   "workspace",
	Short: "工作区管理",
}

// workspaceListCmd 列出用户参与的项目
var workspaceListCmd = &cobra.Command{
	Use:   "list",
	Short: "列出参与的项目",
	RunE:  runWorkspaceList,
}

// workspaceSwitchCmd 切换当前工作区
var workspaceSwitchCmd = &cobra.Command{
	Use:   "switch <workspace_id>",
	Short: "切换当前工作区（写入当前目录 .tapd.json）",
	Args:  cobra.ExactArgs(1),
	RunE:  runWorkspaceSwitch,
}

// workspaceInfoCmd 查看当前工作区详情
var workspaceInfoCmd = &cobra.Command{
	Use:   "info",
	Short: "查看当前工作区详情",
	RunE:  runWorkspaceInfo,
}

// workspaceUsersCmd 查看项目成员
var workspaceUsersCmd = &cobra.Command{
	Use:   "users",
	Short: "查看项目成员列表",
	RunE:  runWorkspaceUsers,
}

// workspaceRolesCmd 查看用户组
var workspaceRolesCmd = &cobra.Command{
	Use:   "roles",
	Short: "查看用户组 ID 对照关系",
	RunE:  runWorkspaceRoles,
}

func init() {
	workspaceCmd.AddCommand(workspaceListCmd)
	workspaceCmd.AddCommand(workspaceSwitchCmd)
	workspaceCmd.AddCommand(workspaceInfoCmd)
	workspaceCmd.AddCommand(workspaceUsersCmd)
	workspaceCmd.AddCommand(workspaceRolesCmd)
	rootCmd.AddCommand(workspaceCmd)
}

func runWorkspaceList(cmd *cobra.Command, args []string) error {
	workspaces, err := apiClient.ListWorkspaces(context.Background(), "", apiClient.GetNick())
	if err != nil {
		output.PrintError(os.Stderr, "api_error", err.Error(), "Check your credentials and network connection.")
		os.Exit(output.ExitAPIError)
		return nil
	}
	resp := &model.ListResponse{
		Items: workspaces,
		Total: len(workspaces),
	}
	return output.PrintJSON(os.Stdout, resp, !flagPretty)
}

func runWorkspaceSwitch(cmd *cobra.Command, args []string) error {
	workspaceID := args[0]

	if err := config.SaveWorkspaceID(workspaceID); err != nil {
		output.PrintError(os.Stderr, "config_error",
			"Failed to save workspace ID: "+err.Error(),
			"Check file system permissions for .tapd.json")
		os.Exit(output.ExitAPIError)
		return nil
	}

	// 更新全局变量供后续命令使用
	flagWorkspaceID = workspaceID

	return output.PrintJSON(os.Stdout, &model.SuccessResponse{
		Success:     true,
		WorkspaceID: workspaceID,
	}, !flagPretty)
}

func runWorkspaceUsers(cmd *cobra.Command, args []string) error {
	if flagWorkspaceID == "" {
		output.PrintError(os.Stderr, "workspace_required",
			"No workspace ID configured",
			"Run 'tapd workspace switch <id>' or use --workspace-id flag.")
		os.Exit(output.ExitParamError)
		return nil
	}

	users, err := apiClient.GetWorkspaceUsers(context.Background(), &model.GetWorkspaceUsersRequest{
		WorkspaceID: flagWorkspaceID,
	})
	if err != nil {
		output.PrintError(os.Stderr, "api_error", err.Error(), "")
		os.Exit(output.ExitAPIError)
		return nil
	}
	resp := &model.ListResponse{
		Items: users,
		Total: len(users),
	}
	return output.PrintJSON(os.Stdout, resp, !flagPretty)
}

func runWorkspaceRoles(cmd *cobra.Command, args []string) error {
	if flagWorkspaceID == "" {
		output.PrintError(os.Stderr, "workspace_required",
			"No workspace ID configured",
			"Run 'tapd workspace switch <id>' or use --workspace-id flag.")
		os.Exit(output.ExitParamError)
		return nil
	}

	roles, err := apiClient.GetRoles(context.Background(), flagWorkspaceID)
	if err != nil {
		output.PrintError(os.Stderr, "api_error", err.Error(), "")
		os.Exit(output.ExitAPIError)
		return nil
	}
	return output.PrintJSON(os.Stdout, roles, !flagPretty)
}

func runWorkspaceInfo(cmd *cobra.Command, args []string) error {
	if flagWorkspaceID == "" {
		output.PrintError(os.Stderr, "workspace_required",
			"No workspace ID configured",
			"Run 'tapd workspace switch <id>' or use --workspace-id flag.")
		os.Exit(output.ExitParamError)
		return nil
	}

	workspace, err := apiClient.GetWorkspaceInfo(context.Background(), flagWorkspaceID)
	if err != nil {
		output.PrintError(os.Stderr, "api_error", err.Error(), "Check workspace ID and try again.")
		os.Exit(output.ExitAPIError)
		return nil
	}
	return output.PrintJSON(os.Stdout, workspace, !flagPretty)
}
