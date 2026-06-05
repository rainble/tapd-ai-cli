// Package cmd 中的 story_template.go 实现了需求模板管理命令
package cmd

import (
	"context"
	"os"

	"github.com/spf13/cobra"
	"github.com/studyzy/tapd-ai-cli/internal/output"
	"github.com/studyzy/tapd-sdk-go/model"
)

var (
	flagStoryTemplateWorkitemTypeID string
)

// storyTemplateCmd 是需求模板父命令
var storyTemplateCmd = &cobra.Command{
	Use:   "template",
	Short: "需求模板管理",
}

var storyTemplateListCmd = &cobra.Command{
	Use:   "list",
	Short: "查询需求模板列表",
	RunE:  runStoryTemplateList,
}

func init() {
	storyTemplateListCmd.Flags().StringVar(&flagStoryTemplateWorkitemTypeID, "workitem-type-id", "", "需求类别 ID")

	storyTemplateCmd.AddCommand(storyTemplateListCmd)
	storyCmd.AddCommand(storyTemplateCmd)
}

func runStoryTemplateList(cmd *cobra.Command, args []string) error {
	req := &model.GetStoryTemplateListRequest{
		WorkspaceID:    flagWorkspaceID,
		WorkitemTypeID: flagStoryTemplateWorkitemTypeID,
	}
	templates, err := apiClient.GetStoryTemplateList(context.Background(), req)
	if err != nil {
		output.PrintError(os.Stderr, "api_error", err.Error(), "")
		os.Exit(output.ExitAPIError)
		return nil
	}
	return output.PrintJSON(os.Stdout, templates, !flagPretty)
}
