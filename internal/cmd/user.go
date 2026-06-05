// Package cmd 中的 user.go 实现了用户信息命令
package cmd

import (
	"context"
	"os"

	"github.com/spf13/cobra"
	"github.com/studyzy/tapd-ai-cli/internal/output"
	"github.com/studyzy/tapd-sdk-go/model"
)

var (
	flagViewType string
)

// userCmd 是 user 父命令
var userCmd = &cobra.Command{
	Use:   "user",
	Short: "用户信息管理",
}

var userInfoCmd = &cobra.Command{
	Use:   "info",
	Short: "获取当前用户信息",
	RunE:  runUserInfo,
}

var userViewsCmd = &cobra.Command{
	Use:   "views",
	Short: "获取用户视图列表",
	RunE:  runUserViews,
}

func init() {
	userViewsCmd.Flags().StringVar(&flagViewType, "type", "", "对象类型（目前只支持 story）")

	userCmd.AddCommand(userInfoCmd, userViewsCmd)
	rootCmd.AddCommand(userCmd)
}

func runUserInfo(cmd *cobra.Command, args []string) error {
	info, err := apiClient.GetUsersInfo(context.Background())
	if err != nil {
		output.PrintError(os.Stderr, "api_error", err.Error(), "")
		os.Exit(output.ExitAPIError)
		return nil
	}
	return output.PrintJSON(os.Stdout, info, !flagPretty)
}

func runUserViews(cmd *cobra.Command, args []string) error {
	req := &model.GetUserViewListRequest{
		WorkspaceID: flagWorkspaceID,
		Type:        flagViewType,
	}
	views, err := apiClient.GetUserViewList(context.Background(), req)
	if err != nil {
		output.PrintError(os.Stderr, "api_error", err.Error(), "")
		os.Exit(output.ExitAPIError)
		return nil
	}
	return output.PrintJSON(os.Stdout, views, !flagPretty)
}
