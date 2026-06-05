// Package cmd 中的 workspace_extras.go 实现了工作区扩展命令
package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/studyzy/tapd-ai-cli/internal/output"
	"github.com/studyzy/tapd-sdk-go/model"
)

var (
	flagMemberNick    string
	flagMemberRoleIDs string
	flagSettingType   string
	flagDocLimit      string
	flagDocPage       string
)

// workspaceSubWorkspacesCmd 获取子项目
var workspaceSubWorkspacesCmd = &cobra.Command{
	Use:   "sub-workspaces",
	Short: "获取子项目信息",
	RunE:  runWorkspaceSubWorkspaces,
}

// workspaceDocumentsCmd 获取项目文档
var workspaceDocumentsCmd = &cobra.Command{
	Use:   "documents",
	Short: "获取项目文档列表",
	RunE:  runWorkspaceDocuments,
}

// workspaceMembersCmd 是成员管理父命令
var workspaceMembersCmd = &cobra.Command{
	Use:   "members",
	Short: "项目成员管理",
}

// workspaceMembersAddCmd 添加项目成员
var workspaceMembersAddCmd = &cobra.Command{
	Use:   "add",
	Short: "添加项目成员",
	RunE:  runWorkspaceMembersAdd,
}

// workspaceSettingsCmd 获取项目配置
var workspaceSettingsCmd = &cobra.Command{
	Use:   "settings",
	Short: "获取项目配置开关",
	RunE:  runWorkspaceSettings,
}

func init() {
	workspaceDocumentsCmd.Flags().StringVar(&flagDocLimit, "limit", "", "返回数量限制（默认 30，最大 200）")
	workspaceDocumentsCmd.Flags().StringVar(&flagDocPage, "page", "", "页码")

	workspaceMembersAddCmd.Flags().StringVar(&flagMemberNick, "nick", "", "用户昵称（必需）")
	workspaceMembersAddCmd.Flags().StringVar(&flagMemberRoleIDs, "role-ids", "", "用户组 ID（逗号分隔）")

	workspaceSettingsCmd.Flags().StringVar(&flagSettingType, "type", "", "配置名称（必需，如 is_enabled_story_category / workspace_metrology）")

	workspaceMembersCmd.AddCommand(workspaceMembersAddCmd)
	workspaceCmd.AddCommand(workspaceSubWorkspacesCmd, workspaceDocumentsCmd, workspaceMembersCmd, workspaceSettingsCmd)
}

func runWorkspaceSubWorkspaces(cmd *cobra.Command, args []string) error {
	req := &model.GetSubWorkspacesRequest{
		WorkspaceID: flagWorkspaceID,
	}
	ws, err := apiClient.GetSubWorkspaces(context.Background(), req)
	if err != nil {
		output.PrintError(os.Stderr, "api_error", err.Error(), "")
		os.Exit(output.ExitAPIError)
		return nil
	}
	return output.PrintJSON(os.Stdout, ws, !flagPretty)
}

func runWorkspaceDocuments(cmd *cobra.Command, args []string) error {
	req := &model.GetWorkspaceDocumentsRequest{
		WorkspaceID: flagWorkspaceID,
		Limit:       flagDocLimit,
		Page:        flagDocPage,
	}
	docs, err := apiClient.GetWorkspaceDocuments(context.Background(), req)
	if err != nil {
		output.PrintError(os.Stderr, "api_error", err.Error(), "")
		os.Exit(output.ExitAPIError)
		return nil
	}
	resp := &model.ListResponse{
		Items: docs,
		Total: len(docs),
	}
	return output.PrintJSON(os.Stdout, resp, !flagPretty)
}

func runWorkspaceMembersAdd(cmd *cobra.Command, args []string) error {
	if flagMemberNick == "" {
		output.PrintError(os.Stderr, "missing_parameter", "--nick is required",
			"Usage: tapd workspace members add --nick <nick> [--role-ids <ids>]")
		os.Exit(output.ExitParamError)
		return nil
	}
	req := &model.AddWorkspaceMemberRequest{
		WorkspaceID: flagWorkspaceID,
		Nick:        flagMemberNick,
		RoleIDs:     flagMemberRoleIDs,
	}
	result, err := apiClient.AddWorkspaceMember(context.Background(), req)
	if err != nil {
		output.PrintError(os.Stderr, "api_error", err.Error(), "")
		os.Exit(output.ExitAPIError)
		return nil
	}
	return output.PrintJSON(os.Stdout, result, !flagPretty)
}

func runWorkspaceSettings(cmd *cobra.Command, args []string) error {
	if flagSettingType == "" {
		output.PrintError(os.Stderr, "missing_parameter", "--type is required",
			fmt.Sprintf("Usage: tapd workspace settings --type <type>\nValid types: is_enabled_story_category, workspace_metrology"))
		os.Exit(output.ExitParamError)
		return nil
	}
	req := &model.GetWorkspaceSettingRequest{
		WorkspaceID: flagWorkspaceID,
		Type:        flagSettingType,
	}
	setting, err := apiClient.GetWorkspaceSetting(context.Background(), req)
	if err != nil {
		output.PrintError(os.Stderr, "api_error", err.Error(), "")
		os.Exit(output.ExitAPIError)
		return nil
	}
	return output.PrintJSON(os.Stdout, setting, !flagPretty)
}
