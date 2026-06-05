// Package cmd 中的 report.go 实现了项目报告管理命令
package cmd

import (
	"context"
	"os"

	"github.com/spf13/cobra"
	"github.com/studyzy/tapd-ai-cli/internal/output"
	"github.com/studyzy/tapd-sdk-go/model"
)

// reportCmd 是项目报告父命令
var reportCmd = &cobra.Command{
	Use:   "report",
	Short: "项目报告管理",
}

var reportListCmd = &cobra.Command{
	Use:   "list",
	Short: "查询项目报告列表",
	RunE:  runReportList,
}

func init() {
	reportListCmd.Flags().IntVar(&flagLimit, "limit", 30, "返回数量限制")
	reportListCmd.Flags().IntVar(&flagPage, "page", 1, "页码")

	reportCmd.AddCommand(reportListCmd)
	rootCmd.AddCommand(reportCmd)
}

func runReportList(cmd *cobra.Command, args []string) error {
	req := &model.GetWorkspaceReportsRequest{
		WorkspaceID: flagWorkspaceID,
		Limit:       flagLimit,
		Page:        flagPage,
	}
	data, err := apiClient.GetWorkspaceReports(context.Background(), req)
	if err != nil {
		output.PrintError(os.Stderr, "api_error", err.Error(), "")
		os.Exit(output.ExitAPIError)
		return nil
	}
	resp := &model.ListResponse{
		Items: data,
		Total: len(data),
		Page:  flagPage,
		Limit: flagLimit,
	}
	return output.PrintJSON(os.Stdout, resp, !flagPretty)
}
