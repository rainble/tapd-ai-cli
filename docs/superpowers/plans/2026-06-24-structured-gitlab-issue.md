# Structured GitLab Issue Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Generate GitLab Issue descriptions from TAPD story/bug content as structured working documents instead of pasting the TAPD description verbatim.

**Architecture:** Keep the change inside the existing GitLab sync snapshot path. `buildGitLabIssueFromStory` and `buildGitLabIssueFromBug` will keep metadata and title/fingerprint behavior, but delegate body rendering to a deterministic local section classifier. `sync-watch` needs no separate path because it already consumes snapshot descriptions.

**Tech Stack:** Go 1.24, Cobra command package, existing TAPD SDK models, standard-library string parsing, existing `go test` suite.

## Global Constraints

- 手动生成和监听同步都使用同一套结构化描述。
- Story 和 Bug 使用不同的信息架构。
- 解析必须确定性、可测试，不依赖 LLM、网络或额外服务。
- 不丢失无法识别的 TAPD 内容，归入“原始补充”。
- 保持现有 GitLab 同步 marker、fingerprint 和 comment-back 行为不变。
- 不改变 Issue 标题格式。
- 不新增配置开关；结构化输出成为默认行为。
- 空 section 不输出。

---

## File Structure

- Modify `internal/cmd/gitlab_test.go`: add focused tests for structured Story and Bug descriptions while keeping existing sync behavior tests.
- Modify `internal/cmd/gitlab.go`: replace `renderTAPDSnapshot` body rendering with deterministic structured rendering helpers.
- Do not modify `internal/mcp/tools_gitlab.go`; it is already dirty and unrelated to this feature.

---

### Task 1: Add Failing Story Structure Tests

**Files:**
- Modify: `internal/cmd/gitlab_test.go`
- Test: `internal/cmd/gitlab_test.go`

**Interfaces:**
- Consumes: existing `buildGitLabIssueFromStory(story *model.Story) gitLabIssueSnapshot`
- Produces: tests requiring structured Story section output from `snapshot.Description`

- [ ] **Step 1: Update the Story test to require structured sections**

Replace `TestBuildGitLabIssueFromStory_RendersTAPDSnapshot` with:

```go
func TestBuildGitLabIssueFromStory_RendersStructuredDescription(t *testing.T) {
	story := &model.Story{
		ID:            "1151081496001028684",
		Name:          "支持自动续费看板",
		Description:   "<h2>背景</h2><p>运营需要展示签约趋势。</p><h2>目标</h2><p>让 PM 快速判断续费表现。</p><h2>方案</h2><p>新增趋势图和明细表。</p><h2>验收标准</h2><p>能按日期筛选并导出。</p><h2>风险</h2><p>依赖数据仓库产出。</p><p>补充说明：历史数据只保留 180 天。</p>",
		URL:           "https://tapd.example.com/story",
		Status:        "planning",
		PriorityLabel: "High",
		Owner:         "alice",
		Developer:     "bob",
		IterationID:   "it1",
	}

	snapshot := buildGitLabIssueFromStory(story)

	if snapshot.EntityType != "story" || snapshot.EntityID != story.ID {
		t.Fatalf("unexpected identity: %+v", snapshot)
	}
	if snapshot.Title != "[TAPD Story] 支持自动续费看板" {
		t.Fatalf("title = %q", snapshot.Title)
	}
	for _, want := range []string{
		"## TAPD 需求",
		"- TAPD: https://tapd.example.com/story",
		"- ID: 1151081496001028684",
		"- Priority: High",
		"- Owner: alice",
		"- Developer: bob",
		"## 背景 / 现状",
		"运营需要展示签约趋势。",
		"## 目标 / 预期",
		"让 PM 快速判断续费表现。",
		"## 需求范围 / 方案",
		"新增趋势图和明细表。",
		"## 验收标准 / 测试要点",
		"能按日期筛选并导出。",
		"## 风险 / 依赖 / 待确认",
		"依赖数据仓库产出。",
		"## 原始补充",
		"补充说明：历史数据只保留 180 天。",
	} {
		if !strings.Contains(snapshot.Description, want) {
			t.Fatalf("description should contain %q, got:\n%s", want, snapshot.Description)
		}
	}
	if strings.Contains(snapshot.Description, "## 描述") {
		t.Fatalf("description should not use raw description section, got:\n%s", snapshot.Description)
	}
	if !snapshot.Ready {
		t.Fatal("story with title and description should be ready")
	}
	if snapshot.Fingerprint == "" {
		t.Fatal("fingerprint should not be empty")
	}
}
```

- [ ] **Step 2: Add a Story fallback test for unclassified content**

Add this test below the structured Story test:

```go
func TestBuildGitLabIssueFromStory_PutsUnclassifiedContentInOriginalSupplement(t *testing.T) {
	story := &model.Story{
		ID:          "1151081496001028684",
		Name:        "无模板需求",
		Description: "<p>这是一段没有模板标题的自由描述。</p>",
	}

	snapshot := buildGitLabIssueFromStory(story)

	for _, want := range []string{
		"## TAPD 需求",
		"## 原始补充",
		"这是一段没有模板标题的自由描述。",
	} {
		if !strings.Contains(snapshot.Description, want) {
			t.Fatalf("description should contain %q, got:\n%s", want, snapshot.Description)
		}
	}
	for _, notWant := range []string{"## 背景 / 现状", "## 目标 / 预期", "## 需求范围 / 方案"} {
		if strings.Contains(snapshot.Description, notWant) {
			t.Fatalf("description should not contain empty section %q, got:\n%s", notWant, snapshot.Description)
		}
	}
}
```

- [ ] **Step 3: Run Story tests and verify RED**

Run:

```bash
go test ./internal/cmd -run 'TestBuildGitLabIssueFromStory' -count=1
```

Expected: FAIL because the current implementation emits `## 描述` and does not emit structured sections such as `## 背景 / 现状`.

- [ ] **Step 4: Commit only failing tests if executing in a separate branch**

Do not commit on `main` unless the user explicitly wants intermediate commits. If committing, stage only `internal/cmd/gitlab_test.go`:

```bash
git add internal/cmd/gitlab_test.go
git commit -m "test: require structured gitlab story descriptions"
```

---

### Task 2: Add Failing Bug Structure Tests

**Files:**
- Modify: `internal/cmd/gitlab_test.go`
- Test: `internal/cmd/gitlab_test.go`

**Interfaces:**
- Consumes: existing `buildGitLabIssueFromBug(bug *model.Bug) gitLabIssueSnapshot`
- Produces: tests requiring structured Bug section output from `snapshot.Description`

- [ ] **Step 1: Replace the Bug snapshot test with structured expectations**

Replace `TestBuildGitLabIssueFromBug_RendersTAPDSnapshot` with:

```go
func TestBuildGitLabIssueFromBug_RendersStructuredDescription(t *testing.T) {
	bug := &model.Bug{
		ID:            "1151081496002000001",
		Title:         "保存时报错",
		Description:   "<div>前置条件：测试账号 mid=123，iOS 8.0.0</div><div>复现流程：进入编辑页后点击保存</div><div>实际情况：接口返回 500</div><div>预期情况：保存成功并返回详情页</div><div>影响范围：所有开通充电的 UP 主</div><div>日志 trace_id=abc</div>",
		URL:           "https://tapd.example.com/bug",
		Status:        "new",
		PriorityLabel: "urgent",
		Severity:      "serious",
		CurrentOwner:  "charlie",
		Module:        "charge",
		IterationID:   "it2",
	}

	snapshot := buildGitLabIssueFromBug(bug)

	if snapshot.EntityType != "bug" || snapshot.EntityID != bug.ID {
		t.Fatalf("unexpected identity: %+v", snapshot)
	}
	if snapshot.Title != "[TAPD Bug] 保存时报错" {
		t.Fatalf("title = %q", snapshot.Title)
	}
	for _, want := range []string{
		"## TAPD 缺陷",
		"- TAPD: https://tapd.example.com/bug",
		"- Severity: serious",
		"- Current owner: charlie",
		"- Module: charge",
		"## 复现条件",
		"测试账号 mid=123",
		"## 复现步骤",
		"进入编辑页后点击保存",
		"## 实际结果",
		"接口返回 500",
		"## 预期结果",
		"保存成功并返回详情页",
		"## 影响范围",
		"所有开通充电的 UP 主",
		"## 排查线索",
		"trace_id=abc",
	} {
		if !strings.Contains(snapshot.Description, want) {
			t.Fatalf("description should contain %q, got:\n%s", want, snapshot.Description)
		}
	}
	if strings.Contains(snapshot.Description, "## 描述") {
		t.Fatalf("description should not use raw description section, got:\n%s", snapshot.Description)
	}
	if !snapshot.Ready || snapshot.Fingerprint == "" {
		t.Fatalf("bug should be ready with fingerprint: %+v", snapshot)
	}
}
```

- [ ] **Step 2: Add a Bug original supplement test**

Add this test below the structured Bug test:

```go
func TestBuildGitLabIssueFromBug_PutsUnclassifiedContentInOriginalSupplement(t *testing.T) {
	bug := &model.Bug{
		ID:          "1151081496002000001",
		Title:       "无模板缺陷",
		Description: "<p>用户反馈页面偶现空白，暂无更多信息。</p>",
	}

	snapshot := buildGitLabIssueFromBug(bug)

	for _, want := range []string{
		"## TAPD 缺陷",
		"## 原始补充",
		"用户反馈页面偶现空白，暂无更多信息。",
	} {
		if !strings.Contains(snapshot.Description, want) {
			t.Fatalf("description should contain %q, got:\n%s", want, snapshot.Description)
		}
	}
	for _, notWant := range []string{"## 复现条件", "## 复现步骤", "## 实际结果"} {
		if strings.Contains(snapshot.Description, notWant) {
			t.Fatalf("description should not contain empty section %q, got:\n%s", notWant, snapshot.Description)
		}
	}
}
```

- [ ] **Step 3: Run Bug tests and verify RED**

Run:

```bash
go test ./internal/cmd -run 'TestBuildGitLabIssueFromBug' -count=1
```

Expected: FAIL because the current implementation emits `## 描述` and does not classify bug content.

- [ ] **Step 4: Commit only failing tests if executing in a separate branch**

Do not commit on `main` unless the user explicitly wants intermediate commits. If committing, stage only `internal/cmd/gitlab_test.go`:

```bash
git add internal/cmd/gitlab_test.go
git commit -m "test: require structured gitlab bug descriptions"
```

---

### Task 3: Implement Structured TAPD Description Rendering

**Files:**
- Modify: `internal/cmd/gitlab.go`
- Test: `internal/cmd/gitlab_test.go`

**Interfaces:**
- Consumes: tests from Task 1 and Task 2
- Produces:
  - `renderStructuredTAPDDescription(entityType string, md string) string`
  - `structuredSection` type
  - `classifyTAPDParagraph(entityType string, paragraph string) string`
  - `splitTAPDParagraphs(md string) []string`

- [ ] **Step 1: Replace snapshot rendering calls**

In `buildGitLabIssueFromStory`, change:

```go
description := renderTAPDSnapshot("TAPD 需求", story.URL, []string{
```

to:

```go
description := renderTAPDSnapshot("story", "TAPD 需求", story.URL, []string{
```

In `buildGitLabIssueFromBug`, change:

```go
description := renderTAPDSnapshot("TAPD 缺陷", bug.URL, []string{
```

to:

```go
description := renderTAPDSnapshot("bug", "TAPD 缺陷", bug.URL, []string{
```

Update the function signature from:

```go
func renderTAPDSnapshot(kind, tapdURL string, fields []string, md string) string {
```

to:

```go
func renderTAPDSnapshot(entityType, kind, tapdURL string, fields []string, md string) string {
```

- [ ] **Step 2: Replace the raw description section with structured rendering**

Inside `renderTAPDSnapshot`, replace:

```go
b.WriteString("\n## 描述\n\n")
b.WriteString(md)
return b.String()
```

with:

```go
structured := renderStructuredTAPDDescription(entityType, md)
if structured != "" {
	b.WriteString("\n")
	b.WriteString(structured)
}
return b.String()
```

- [ ] **Step 3: Add structured section helpers below `renderTAPDSnapshot`**

Add this code in `internal/cmd/gitlab.go` immediately after `renderTAPDSnapshot`:

```go
type structuredSection struct {
	key      string
	title    string
	keywords []string
}

func renderStructuredTAPDDescription(entityType string, md string) string {
	sections := structuredSectionsFor(entityType)
	if len(sections) == 0 {
		sections = structuredSectionsFor("story")
	}
	grouped := make(map[string][]string, len(sections))
	paragraphs := splitTAPDParagraphs(md)
	for _, paragraph := range paragraphs {
		key := classifyTAPDParagraph(entityType, paragraph)
		if key == "" {
			key = "original"
		}
		grouped[key] = append(grouped[key], trimTAPDParagraphLabel(paragraph))
	}

	var b strings.Builder
	for _, section := range sections {
		items := grouped[section.key]
		if len(items) == 0 {
			continue
		}
		if b.Len() > 0 {
			b.WriteString("\n\n")
		}
		b.WriteString("## ")
		b.WriteString(section.title)
		b.WriteString("\n\n")
		b.WriteString(strings.Join(items, "\n\n"))
	}
	return strings.TrimSpace(b.String())
}

func structuredSectionsFor(entityType string) []structuredSection {
	switch entityType {
	case "bug":
		return []structuredSection{
			{key: "summary", title: "问题概述", keywords: []string{"问题", "标题", "现象", "概述"}},
			{key: "condition", title: "复现条件", keywords: []string{"前置条件", "环境", "版本", "账号", "数据", "机型"}},
			{key: "steps", title: "复现步骤", keywords: []string{"复现", "步骤", "流程", "操作"}},
			{key: "actual", title: "实际结果", keywords: []string{"实际", "现状", "结果", "报错"}},
			{key: "expected", title: "预期结果", keywords: []string{"预期", "应该", "期望"}},
			{key: "impact", title: "影响范围", keywords: []string{"影响", "范围", "严重性", "频率", "用户"}},
			{key: "clue", title: "排查线索", keywords: []string{"日志", "接口", "curl", "trace", "截图", "线索", "分析"}},
			{key: "original", title: "原始补充"},
		}
	default:
		return []structuredSection{
			{key: "background", title: "背景 / 现状", keywords: []string{"背景", "现状", "问题", "为什么", "痛点", "诉求", "上下文"}},
			{key: "goal", title: "目标 / 预期", keywords: []string{"目标", "预期", "收益", "价值", "指标", "成功标准"}},
			{key: "scope", title: "需求范围 / 方案", keywords: []string{"范围", "方案", "怎么做", "功能", "流程", "交互", "规则", "逻辑"}},
			{key: "acceptance", title: "验收标准 / 测试要点", keywords: []string{"验收", "测试", "验证", "case", "用例", "准入", "完成标准"}},
			{key: "risk", title: "风险 / 依赖 / 待确认", keywords: []string{"风险", "依赖", "待确认", "限制", "注意事项"}},
			{key: "original", title: "原始补充"},
		}
	}
}

func classifyTAPDParagraph(entityType string, paragraph string) string {
	head := paragraphClassifyText(paragraph)
	for _, section := range structuredSectionsFor(entityType) {
		if section.key == "original" {
			continue
		}
		for _, keyword := range section.keywords {
			if strings.Contains(head, strings.ToLower(keyword)) {
				return section.key
			}
		}
	}
	return ""
}

func paragraphClassifyText(paragraph string) string {
	paragraph = strings.TrimSpace(paragraph)
	paragraph = strings.TrimLeft(paragraph, "#-*> \t")
	if idx := strings.IndexAny(paragraph, "：:\n"); idx >= 0 {
		paragraph = paragraph[:idx]
	}
	return strings.ToLower(strings.TrimSpace(paragraph))
}

func splitTAPDParagraphs(md string) []string {
	md = strings.TrimSpace(md)
	if md == "" {
		return nil
	}
	lines := strings.Split(md, "\n")
	var paragraphs []string
	var current strings.Builder
	flush := func() {
		text := strings.TrimSpace(current.String())
		if text != "" {
			paragraphs = append(paragraphs, text)
		}
		current.Reset()
	}
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			flush()
			continue
		}
		if strings.HasPrefix(trimmed, "#") {
			flush()
			current.WriteString(trimmed)
			current.WriteString("\n")
			continue
		}
		if looksLikeTAPDLabel(trimmed) {
			flush()
		}
		if current.Len() > 0 {
			current.WriteString("\n")
		}
		current.WriteString(trimmed)
	}
	flush()
	return paragraphs
}

func looksLikeTAPDLabel(line string) bool {
	line = strings.TrimLeft(line, "-*> \t")
	idx := strings.IndexAny(line, "：:")
	if idx <= 0 || idx > 24 {
		return false
	}
	prefix := strings.TrimSpace(line[:idx])
	if prefix == "" {
		return false
	}
	for _, section := range structuredSectionsFor("story") {
		for _, keyword := range section.keywords {
			if strings.Contains(prefix, keyword) {
				return true
			}
		}
	}
	for _, section := range structuredSectionsFor("bug") {
		for _, keyword := range section.keywords {
			if strings.Contains(prefix, keyword) {
				return true
			}
		}
	}
	return false
}

func trimTAPDParagraphLabel(paragraph string) string {
	paragraph = strings.TrimSpace(paragraph)
	lines := strings.SplitN(paragraph, "\n", 2)
	first := strings.TrimSpace(lines[0])
	if strings.HasPrefix(first, "#") {
		first = strings.TrimSpace(strings.TrimLeft(first, "# "))
		if len(lines) == 1 {
			return first
		}
		rest := strings.TrimSpace(lines[1])
		if rest == "" {
			return first
		}
		return rest
	}
	cleaned := strings.TrimLeft(first, "-*> \t")
	if idx := strings.IndexAny(cleaned, "：:"); idx > 0 && idx <= 24 {
		value := strings.TrimSpace(cleaned[idx+len(string([]rune(cleaned[idx:idx+1]))):])
		if value != "" {
			if len(lines) == 1 {
				return value
			}
			return strings.TrimSpace(value + "\n" + lines[1])
		}
	}
	return paragraph
}
```

- [ ] **Step 4: Run focused tests and verify GREEN**

Run:

```bash
go test ./internal/cmd -run 'TestBuildGitLabIssue|TestHandleGitLabIssueSyncEvent' -count=1
```

Expected: PASS.

- [ ] **Step 5: If `trimTAPDParagraphLabel` has byte/rune issues, simplify safely**

If Step 4 fails around Chinese colon slicing, replace this line:

```go
value := strings.TrimSpace(cleaned[idx+len(string([]rune(cleaned[idx:idx+1]))):])
```

with:

```go
value := strings.TrimSpace(strings.TrimLeft(cleaned[idx+1:], "：:"))
```

Then rerun:

```bash
go test ./internal/cmd -run 'TestBuildGitLabIssue|TestHandleGitLabIssueSyncEvent' -count=1
```

Expected: PASS.

- [ ] **Step 6: Commit implementation if executing in a separate branch**

Do not commit on `main` unless the user explicitly wants implementation commits. If committing:

```bash
git add internal/cmd/gitlab.go internal/cmd/gitlab_test.go
git commit -m "feat: structure gitlab issue descriptions"
```

---

### Task 4: Verify Full Local Suite and Manual Output Shape

**Files:**
- No new files
- Test: `internal/cmd/gitlab_test.go`, full Go test suite

**Interfaces:**
- Consumes: structured renderer from Task 3
- Produces: verified local test result and example output confidence

- [ ] **Step 1: Run focused command test**

Run:

```bash
go test ./internal/cmd -run 'TestGitLabIssueCreateFromStory_CreatesIssueAndCommentsBack|TestGitLabIssueCreateFromBug_CreatesIssue|TestHandleGitLabIssueSyncEvent' -count=1
```

Expected: PASS.

- [ ] **Step 2: Run full local suite without TAPD credentials**

Run:

```bash
env -u TAPD_ACCESS_TOKEN -u TAPD_API_USER -u TAPD_API_PASSWORD -u TAPD_WORKSPACE_ID go test ./...
```

Expected: PASS. This avoids running real TAPD integration tests with local credentials.

- [ ] **Step 3: Inspect final diff**

Run:

```bash
git diff -- internal/cmd/gitlab.go internal/cmd/gitlab_test.go
```

Expected: diff only changes GitLab issue description rendering and tests. No changes to MCP schema, GitLab client, TAPD client, or unrelated commands.

- [ ] **Step 4: Commit verification-ready result if executing in a separate branch**

Do not commit on `main` unless the user explicitly wants implementation commits. If committing:

```bash
git add internal/cmd/gitlab.go internal/cmd/gitlab_test.go
git commit -m "test: verify structured gitlab issue sync"
```
