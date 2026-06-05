// Package cmd 中的 url.go 实现了根据 TAPD URL 查询对应条目详情的通用命令
package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/studyzy/tapd-ai-cli/internal/output"
	"github.com/studyzy/tapd-ai-cli/internal/tapdurl"
)

// urlCmd 是根据 TAPD URL 查询条目详情的命令
var urlCmd = &cobra.Command{
	Use:   "url <url>",
	Short: "根据 TAPD URL 查询对应条目详情（支持需求、缺陷、任务、Wiki）",
	Long: `根据 TAPD URL 自动识别条目类型并查询详情。

支持以下 URL 格式：
  需求详情页:  https://www.tapd.cn/tapd_fe/{workspace_id}/story/detail/{id}
  需求列表页:  https://www.tapd.cn/tapd_fe/{workspace_id}/story/list?...&dialog_preview_id=story_{id}
  需求 view 页: https://www.tapd.cn/{workspace_id}/prong/stories/view/{id}
  缺陷详情页:  https://www.tapd.cn/tapd_fe/{workspace_id}/bug/detail/{id}
  缺陷列表页:  https://www.tapd.cn/tapd_fe/{workspace_id}/bug/list?...&dialog_preview_id=bug_{id}
  缺陷 view 页: https://www.tapd.cn/{workspace_id}/prong/bugs/view/{id}
  任务详情页:  https://www.tapd.cn/tapd_fe/{workspace_id}/task/detail/{id}
  任务看板页:  https://www.tapd.cn/{workspace_id}/prong/tasks?...&dialog_preview_id=task_{id}
  任务 view 页: https://www.tapd.cn/{workspace_id}/prong/tasks/view/{id}
  Wiki 文档:   https://www.tapd.cn/{workspace_id}/markdown_wikis/show/#{id}`,
	Args: cobra.ExactArgs(1),
	RunE: runURLQuery,
}

func init() {
	rootCmd.AddCommand(urlCmd)
}

// runURLQuery 是 url 命令的执行函数
func runURLQuery(cmd *cobra.Command, args []string) error {
	parsed, err := tapdurl.Parse(args[0])
	if err != nil {
		output.PrintError(os.Stderr, "invalid_tapd_url", err.Error(),
			"provide a TAPD URL like https://www.tapd.cn/tapd_fe/{workspace_id}/story/detail/{id}")
		os.Exit(output.ExitParamError)
		return nil
	}

	workspaceID := parsed.WorkspaceID

	switch parsed.EntityType {
	case "story":
		result, err := apiClient.GetStory(context.Background(), workspaceID, parsed.EntityID)
		if err != nil {
			handleAPIError(err)
			return nil
		}
		if err := printDetail(result, "description"); err != nil {
			return err
		}
		printComments(workspaceID, "stories", parsed.EntityID)
		return nil

	case "bug":
		result, err := apiClient.GetBug(context.Background(), workspaceID, parsed.EntityID)
		if err != nil {
			handleAPIError(err)
			return nil
		}
		if err := printDetail(result, "description"); err != nil {
			return err
		}
		printComments(workspaceID, "bug", parsed.EntityID)
		return nil

	case "task":
		result, err := apiClient.GetTask(context.Background(), workspaceID, parsed.EntityID)
		if err != nil {
			handleAPIError(err)
			return nil
		}
		if err := printDetail(result, "description"); err != nil {
			return err
		}
		printComments(workspaceID, "tasks", parsed.EntityID)
		return nil

	case "wiki":
		result, err := apiClient.GetWiki(context.Background(), workspaceID, parsed.EntityID)
		if err != nil {
			handleAPIError(err)
			return nil
		}
		if err := printDetail(result, "markdown_description"); err != nil {
			return err
		}
		printComments(workspaceID, "wiki", parsed.EntityID)
		return nil

	default:
		output.PrintError(os.Stderr, "unsupported_entity_type",
			fmt.Sprintf("unsupported TAPD entity type: %q", parsed.EntityType),
			"supported types: story, bug, task, wiki")
		os.Exit(output.ExitParamError)
		return nil
	}
}

// handleAPIError 统一处理 API 调用错误
func handleAPIError(err error) {
	output.PrintError(os.Stderr, "api_error", err.Error(), "")
	os.Exit(output.ExitAPIError)
}
