# Structured GitLab Issue Design

## 背景

当前 `tapd gitlab issue create-from-story`、`create-from-bug` 和 `sync-watch`
共用 `buildGitLabIssueFromStory` / `buildGitLabIssueFromBug` 生成 GitLab Issue。
现有实现会把 TAPD 描述转成 Markdown 后放入 `## 描述`，基本保留原文顺序。
这对同步完整性有利，但对 Codex 阅读和后续处理不够友好。

目标是让 GitLab Issue 变成更结构化的工作文档：保留 TAPD 元信息和回溯链接，
同时把正文拆成更稳定的语义段落，减少原文粘贴感。

## 目标

- 手动生成和监听同步都使用同一套结构化描述。
- Story 和 Bug 使用不同的信息架构。
- 解析必须确定性、可测试，不依赖 LLM、网络或额外服务。
- 不丢失无法识别的 TAPD 内容，归入“原始补充”。
- 保持现有 GitLab 同步 marker、fingerprint 和 comment-back 行为不变。

## 非目标

- 不引入 AI 总结、改写或语义推理。
- 不修改 TAPD 数据模型、GitLab API 客户端或 MCP tool schema。
- 不改变 Issue 标题格式。
- 不新增配置开关；结构化输出成为默认行为。

## 输出结构

### Story

Story 的 GitLab Issue description 输出：

1. `## TAPD 需求`
2. 元信息列表：TAPD URL、ID、Status、Priority、Owner、Developer、Iteration
3. `## 背景 / 现状`
4. `## 目标 / 预期`
5. `## 需求范围 / 方案`
6. `## 验收标准 / 测试要点`
7. `## 风险 / 依赖 / 待确认`
8. `## 原始补充`

### Bug

Bug 的 GitLab Issue description 输出：

1. `## TAPD 缺陷`
2. 元信息列表：TAPD URL、ID、Status、Priority、Severity、Current owner、Module、Iteration
3. `## 问题概述`
4. `## 复现条件`
5. `## 复现步骤`
6. `## 实际结果`
7. `## 预期结果`
8. `## 影响范围`
9. `## 排查线索`
10. `## 原始补充`

空 section 不输出，避免生成空壳文档。若正文完全无法归类，则只输出
`## 原始补充` 并保留清洗后的 Markdown。

## 解析策略

新增一个小型结构化渲染层，输入为清洗后的 TAPD Markdown，输出为 section 列表。

解析分两步：

1. 按 Markdown 标题、中文冒号、常见模板行切分段落。
2. 用关键词归类段落。

Story 关键词示例：

- 背景 / 现状：背景、现状、问题、为什么、痛点、诉求、上下文
- 目标 / 预期：目标、预期、收益、价值、指标、成功标准
- 需求范围 / 方案：范围、方案、怎么做、功能、流程、交互、规则、逻辑
- 验收标准 / 测试要点：验收、测试、验证、case、用例、准入、完成标准
- 风险 / 依赖 / 待确认：风险、依赖、待确认、问题、限制、注意事项

Bug 关键词示例：

- 问题概述：问题、标题、现象、概述
- 复现条件：前置条件、环境、版本、账号、数据、机型
- 复现步骤：复现、步骤、流程、操作
- 实际结果：实际、现状、结果、报错
- 预期结果：预期、应该、期望
- 影响范围：影响、范围、严重性、频率、用户
- 排查线索：日志、接口、curl、trace、截图、线索、分析

归类顺序固定。一个段落只进入第一个命中的 section；无命中的段落进入
`原始补充`。图片、链接、代码块和列表作为段落内容保留。

## 数据流

`buildGitLabIssueFromStory`：

1. 读取 `MarkdownDescription`，为空时退回 `Description`。
2. 使用现有 `normalizedTAPDMarkdown` 做 HTML 到 Markdown 清洗。
3. 调用 `renderStructuredTAPDDescription("story", md)`。
4. 将结构化正文交给现有 GitLab issue snapshot。

`buildGitLabIssueFromBug` 同理，但使用 bug section schema。

`sync-watch` 无需单独改造，因为它已经通过 snapshot description 创建 Issue 或追加 note。

## 错误处理

- 标题或描述为空时继续沿用现有 `Ready=false` 判断。
- 解析器不返回运行时错误；解析失败的输入按原始补充输出。
- 不因为未识别 section 阻止 Issue 创建。

## 测试策略

新增或调整 `internal/cmd/gitlab_test.go` 中的单元测试：

- Story：带“背景、目标、方案、验收、风险”的 TAPD 描述应输出对应 section。
- Bug：带“前置条件、复现流程、实际情况、预期情况、其他”的描述应输出对应 section。
- 未归类内容应进入 `原始补充`。
- HTML 输入经现有清洗后仍能归类。
- 现有 `create-from-story`、`create-from-bug`、`sync-watch` 测试继续通过。

验证命令：

```bash
go test ./internal/cmd -run 'TestBuildGitLabIssue|TestHandleGitLabIssueSyncEvent' -count=1
env -u TAPD_ACCESS_TOKEN -u TAPD_API_USER -u TAPD_API_PASSWORD -u TAPD_WORKSPACE_ID go test ./...
```

## 风险

- TAPD 描述模板不统一，关键词解析无法覆盖所有写法。
- 结构化输出会改变 GitLab description 的 fingerprint，已同步过的 Issue 下次变化时可能追加一条新的结构化 note。
- 过度拆分可能让极短描述显得零散，因此空 section 不输出。

这些风险通过保留原始补充和稳定测试覆盖降低。
