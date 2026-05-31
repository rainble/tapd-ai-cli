// Package cmd 中的 mcp.go 提供 `tapd mcp` 子命令——以 stdio MCP server 模式运行。
//
// AI 客户端（Claude Code / Cursor / Codex）启动 tapd 二进制作为子进程，
// 通过 stdin/stdout 走 JSON-RPC，stderr 用于诊断日志。
package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
	"github.com/studyzy/tapd-ai-cli/internal/mcp"
	"github.com/studyzy/tapd-ai-cli/internal/output"
)

// mcpCmd 启动 stdio MCP server。
var mcpCmd = &cobra.Command{
	Use:   "mcp",
	Short: "以 stdio MCP server 模式运行，供 Claude Code / Cursor 等客户端接入",
	Long: `tapd mcp 启动一个 Model Context Protocol stdio server，
把 TAPD 的核心读写操作以 MCP tool 形式暴露给 AI 客户端。

客户端配置示例（Claude Code: ~/.claude/mcp_servers.json）:

  {
    "mcpServers": {
      "tapd": {
        "command": "tapd",
        "args": ["mcp"]
      }
    }
  }

工作区 ID、API 凭据复用 ~/.tapd.json 与环境变量，无需在 mcp_servers.json 重复配置。`,
	RunE: runMCP,
}

func init() {
	rootCmd.AddCommand(mcpCmd)
}

// runMCP 启动 server。stdin/stdout 走协议，stderr 打日志，绝不能反过来。
func runMCP(cmd *cobra.Command, args []string) error {
	if apiClient == nil {
		output.PrintError(os.Stderr, "authentication_required",
			"MCP server requires TAPD credentials",
			"run 'tapd auth login --access-token <token>' or set TAPD_ACCESS_TOKEN env var")
		os.Exit(output.ExitAuthError)
		return nil
	}

	defaultWorkspace := flagWorkspaceID
	if defaultWorkspace == "" && appConfig != nil {
		defaultWorkspace = appConfig.WorkspaceID
	}

	server := mcp.NewServer(os.Stdin, os.Stdout, os.Stderr, apiClient)
	mcp.RegisterDefaultTools(server, defaultWorkspace)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	fmt.Fprintf(os.Stderr, "tapd-mcp: server started, default workspace=%q\n", defaultWorkspace)
	if err := server.Run(ctx); err != nil && ctx.Err() == nil {
		fmt.Fprintf(os.Stderr, "tapd-mcp: server stopped with err: %v\n", err)
	}
	return nil
}
