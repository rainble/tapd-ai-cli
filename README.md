# tapd-ai-cli

面向 AI Agent 的 TAPD 命令行工具，通过 TAPD Open API 实现项目管理核心操作。

## 安装

### 方式一：go install（推荐）

```bash
go install github.com/studyzy/tapd-ai-cli/cmd/tapd@latest
```

### 方式二：从源码构建并安装

```bash
git clone git@github.com:studyzy/tapd-ai-cli.git
cd tapd-ai-cli
make install   # 编译并安装到 $GOPATH/bin
```

### 方式三：仅构建二进制

```bash
git clone git@github.com:studyzy/tapd-ai-cli.git
cd tapd-ai-cli
make build     # 在当前目录生成 ./tapd
```

## 认证

支持两种认证方式：

### Access Token（推荐）

```bash
# 命令行登录
tapd auth login --access-token <your_token>

# 或设置环境变量
export TAPD_ACCESS_TOKEN=<your_token>
```

### API User/Password

```bash
# 命令行登录
tapd auth login --api-user <user> --api-password <password>

# 或设置环境变量
export TAPD_API_USER=<user>
export TAPD_API_PASSWORD=<password>
```

凭据也可以写入配置文件 `~/.tapd.json` 或当前目录的 `.tapd.json`。

**凭据优先级**：CLI flags > 环境变量 > `./.tapd.json` > `~/.tapd.json`

### 自定义 TAPD 站点地址

如需连接非 `tapd.cn` 的 TAPD 部署，可通过环境变量或配置文件指定：

```bash
# 环境变量
export TAPD_API_BASE_URL=https://api.my-tapd.com
export TAPD_BASE_URL=https://www.my-tapd.com
```

或写入配置文件：

```json
{
  "access_token": "your-token",
  "api_base_url": "https://api.my-tapd.com",
  "base_url": "https://www.my-tapd.com"
}
```

| 配置项 | 环境变量 | JSON 字段 | 默认值 |
|--------|----------|-----------|--------|
| API 地址 | `TAPD_API_BASE_URL` | `api_base_url` | `https://api.tapd.cn` |
| 前端地址 | `TAPD_BASE_URL` | `base_url` | `https://www.tapd.cn` |

## 基本用法

```bash
# 查看参与的项目
tapd workspace list

# 切换工作区
tapd workspace switch 12345

# 查询需求列表
tapd story list

# 创建需求
tapd story create --name "新功能需求"

# 更新需求并切换迭代
tapd story update 10001 --iteration-id 12345

# 查询缺陷列表
tapd bug list

# 查询任务列表
tapd task list

# 查看迭代列表
tapd iteration list

# 查询发布评审列表
tapd launch list

# 通过 URL 查询任意条目（需求/缺陷/任务/Wiki）
tapd url https://www.tapd.cn/tapd_fe/51081496/story/detail/1151081496001028684

# 查询 Wiki 文档列表
tapd wiki list

# 订阅 TAPD webhook 事件流（需要服务端中转 SSE，详见下文）
tapd watch --endpoint https://your-host/x/upower/tapd/events --token <subscribe-token>

# 查看所有命令参考（AI 自发现）
tapd --help
```

## 高级过滤（--filter）

所有 `list` 命令支持 `--filter` 标志，可重复使用，直接透传 TAPD OpenAPI 的高级查询语法：

```bash
# 按名称模糊搜索
tapd story list --filter "name=LIKE<登录>"

# 按自定义字段精确匹配
tapd story list --filter "custom_field_one=EQ<高优先级>"

# 按时间范围查询
tapd bug list --filter "created=>2024-01-01" --filter "created=<2024-12-31"

# 组合多个过滤条件
tapd task list --owner zhangsan --filter "status=CONTAINS_OR<开发中|测试中>"

# 多人查询
tapd story list --filter "owner=USER_OR<张三|李四>"
```

支持的操作符：`LIKE`（模糊）、`EQ`（精确）、`NOT_EQ`（不等于）、`LIKE_OR`（多值模糊 OR）、`CONTAINS`（包含所有值 AND）、`CONTAINS_OR`（包含任一值 OR）、`USER_OR`（多人 OR）、`>`/`<`（时间/数值比较）、`~`（时间范围）、`<>`（不等于简写）、`|`（多值 OR）。

适用于所有标准字段和自定义字段（`custom_field_*`），可与已有标志（`--status`、`--owner` 等）组合使用。

## 命令一览

```
tapd
├── auth      login --access-token <token> | --api-user <user> --api-password <pwd> [--local]
├── workspace list | switch <id> | info
├── story     list | show <id> | create | update <id> | count | todo
├── task      list | show <id> | create | update <id> | count | todo
├── bug       list | show <id> | create | update <id> | count | todo
├── wiki      list | show <id> | create | update
├── iteration list | create | update | count
├── comment   list | add | update | count
├── tcase     list | create | batch-create
├── timesheet list | add | update
├── launch    list | count | create | update <id> | templates | fields
├── workflow  transitions | status-map | last-steps
├── relation  bugs | create
├── skill     init
├── url       <tapd-url>
├── watch     [--endpoint <url>] [--token <tok>] [--exec <cmd>] [--once]
├── agent     fix-bugs --repo <path> [--test-cmd <cmd>] [--on-success-status <status>]
├── mcp                                   # 以 stdio MCP server 模式运行
└── ...       release, attachment, image, category, custom-field, story-field, workitem-type, commit-msg, qiwei
```

## AI Coding 工具集成

`tapd skill init` 可一键为主流 AI Coding 工具生成 TAPD CLI 的 SKILL.md 指令文件：

```bash
tapd skill init
```

支持的工具：Claude Code、CodeBuddy、Cursor、Windsurf、Trae、Codex、Gemini CLI、Cline、Roo Code、Augment。

命令会自动检测当前目录下已有的工具配置文件夹并默认选中，交互式确认后生成 SKILL.md。生成的命令参考部分从当前 CLI 版本的命令树动态生成，始终保持同步。

## MCP 集成（tapd mcp）

`tapd mcp` 把 CLI 转成一个 stdio MCP server，让 AI 客户端（Claude Code、Cursor、Codex）
直接通过 [Model Context Protocol](https://modelcontextprotocol.io) 调用 TAPD，
不需要额外部署。

凭据复用 `~/.tapd.json` 与环境变量，客户端配置只指向二进制：

```jsonc
// Claude Code: ~/.claude/mcp_servers.json
// Cursor: ~/.cursor/mcp.json
{
  "mcpServers": {
    "tapd": {
      "command": "tapd",
      "args": ["mcp"]
    }
  }
}
```

首批暴露的工具：

| 工具 | 用途 |
|------|------|
| `tapd_workspace_list` | 列出当前用户的全部项目 |
| `tapd_url_resolve` | 粘 TAPD URL，自动识别 story/bug/task/wiki 并返回详情 |
| `tapd_story_list` / `tapd_story_show` / `tapd_story_update` | 需求查/看/改（状态/迭代/优先级等） |
| `tapd_bug_list` / `tapd_bug_show` / `tapd_bug_create` | 缺陷查/看/建 |
| `tapd_task_list` / `tapd_task_show` | 任务查/看 |
| `tapd_iteration_list` | 迭代列表 |
| `tapd_comment_list` / `tapd_comment_add` | 评论查/写（需要 entry_type + entry_id） |

配置过 `workspace switch <id>` 后，工具的 `workspace_id` 参数可省略——
mcp server 会自动用默认工作区兜底。

## 订阅 webhook 事件流（tapd watch）

`tapd watch` 通过 SSE 长连接订阅 TAPD webhook 事件流。整体架构：

```
TAPD SaaS  ──HTTPS POST──▶  你的中转服务  ──SSE──▶  团队成员的 tapd watch
            (HMAC 签名)     (公网域名)              (本地长连接)
```

中转服务负责接收 TAPD 推送、HMAC 校验、并把事件 fan-out 给所有订阅者。
开源仓库 [`tapd-webhook-relay`](#) 提供了一个最小实现，也可以集成到你已有的内部服务里。

CLI 端配置：

```bash
# 通过 flag
tapd watch \
  --endpoint https://your-host/x/upower/tapd/events \
  --token <subscribe-token>

# 或写到 ~/.tapd.json
{
  "watch_endpoint": "https://your-host/x/upower/tapd/events",
  "subscribe_token": "<subscribe-token>"
}

# 也可以用环境变量
export TAPD_WATCH_ENDPOINT=...
export TAPD_SUBSCRIBE_TOKEN=...
```

每收到一个事件，watch 会写一行紧凑 JSON 到 stdout：

```json
{"id":12,"received_at":1717000000000000000,"event":{...原始 webhook payload...}}
```

可选 `--exec` 把事件喂给外部命令（事件 JSON 通过 stdin 传入）：

```bash
tapd watch --exec './handle.sh'
```

可选 `--filter` 在客户端过滤事件，规则格式 `<点路径>=<glob>[,<glob>...]`。
多个 `--filter` 之间是 AND，单个 `--filter` 内多个 glob 之间是 OR：

```bash
# 只关心 story 创建/更新
tapd watch --filter event.event=story_create,story_update

# 只看某个工作区，且 priority 为 High 的需求事件
tapd watch \
  --filter event.workspace_id=20063271 \
  --filter event.story.priority=High

# 数组里命中任一元素也算匹配
tapd watch --filter event.tags=urgent
```

支持的字段路径根是 watch 输出的整体 JSON（含 `id`、`received_at`、`event.*`）。
glob 用 `*` / `?` / `[abc]` 通配，逗号要转义时写 `\,`。

## 自动修复 TAPD Bug（tapd agent fix-bugs）

`tapd agent fix-bugs` 在本地运行，订阅 TAPD webhook SSE，只处理 bug 创建/更新事件。
命令会拉取 bug 详情，调用本地 coding agent 修改 `--repo` 指向的仓库，运行 `--test-cmd`
验证，然后给 bug 写评论。只有配置了 `--on-success-status` 时才会自动流转状态。

如果希望自动对应到 TAPD 绑定的 GitLab MR，可开启 `--branch-strategy linked-mr`。
该模式会优先读取 bug 关联需求里的 MR 链接；如果关联需求没有 MR，会继续检查父需求；
最后才读取 bug 自身描述/评论里的 MR 链接。找到 MR 后在本地执行
`git fetch <remote> merge-requests/<iid>/head` 并 checkout 到 `tapd-agent/mr-<iid>`。
默认仍不会自动 commit、push、创建 MR、部署或合并。

推荐先用一次性、无状态流转模式试跑：

```bash
tapd agent fix-bugs \
  --repo /Users/sunruoyu/go/src/vas/app/upower \
  --branch-strategy linked-mr \
  --test-cmd "go test ./..." \
  --on-start-status "" \
  --on-success-status "" \
  --once
```

确认评论和本地修改符合预期后，再开启状态流转：

```bash
tapd agent fix-bugs \
  --repo /Users/sunruoyu/go/src/vas/app/upower \
  --branch-strategy linked-mr \
  --test-cmd "go test ./..." \
  --on-start-status in_progress \
  --on-success-status resolved
```

默认要求工作区干净；如果 `git status --porcelain` 有输出，命令会跳过自动修复并写 TAPD 评论。
命令不会自动 commit、push、创建 MR、部署或合并。
可用 `--mr-remote` 指定 Git remote，默认 `origin`；可用 `--mr-branch-prefix` 指定本地分支名前缀，默认 `tapd-agent/mr-`。
如果 bug 描述为空，或 bug 当前处理人不包含当前登录用户，命令会直接跳过该事件，不会切分支、运行 agent 或流转状态。

## 全局标志

| 标志 | 说明 |
|------|------|
| `--workspace-id <id>` | 指定工作区 ID（覆盖本地配置） |
| `--pretty` | 输出格式化 JSON（带缩进，便于人类阅读；默认输出紧凑 JSON 以节省 token） |

## SDK

TAPD Go SDK 已独立为单独的模块，可直接引入使用：

```bash
go get github.com/studyzy/tapd-sdk-go@latest
```

详见 [tapd-sdk-go](https://github.com/studyzy/tapd-sdk-go)。

## 开发

```bash
make build      # 构建
make install    # 安装到 $GOPATH/bin
make test       # 运行测试
make coverage   # 测试覆盖率报告
make lint       # 代码检查
make fmt        # 代码格式化
make clean      # 清理构建产物
```

## 许可证

Apache License 2.0
