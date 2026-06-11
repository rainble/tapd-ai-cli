# Project Management Assistant Spec

## 1. Background

The TAPD AI Work Assistant product direction defines role-oriented workflows on top of existing TAPD CLI, MCP, and webhook capabilities. Product requirement assistance is the first product-facing capability. The next role-specific capability should target project management.

Project managers usually need to answer these questions:

- What is the current project or iteration progress?
- Which stories, tasks, or bugs are delayed, stale, or blocked?
- Which owners need follow-up?
- Which requirements are not ready for scheduling or review?
- What should be reported in a daily or weekly project update?
- Which project status transitions are safe to recommend or execute?

This spec defines a project management assistant for progress summarization, blocker detection, and controlled project workflow actions.

## 2. Goal

Build a project management assistant that can:

- Summarize project progress from TAPD stories, tasks, bugs, comments, and iterations.
- Identify blockers, stale items, owner gaps, and schedule risks.
- Generate daily or weekly project reports.
- Produce owner-specific follow-up lists.
- Suggest project or item status transitions based on deterministic rules.
- Optionally write reports back to TAPD comments or send them to Enterprise WeChat when explicitly enabled.

The first implementation should be read-only by default and focus on structured reports. Automatic status transitions should be designed as a later guarded extension.

## 3. Non-Goals

The MVP should not:

- Replace project planning tools or TAPD dashboards.
- Automatically change story, task, bug, iteration, or project status by default.
- Automatically assign owners.
- Automatically modify schedules.
- Depend on Enterprise WeChat group history ingestion.
- Require a web UI.
- Require hidden LLM calls inside the CLI.
- Try to infer business priority beyond available TAPD fields.

## 4. Target Users

### Project Manager

Primary user.

Needs:

- Quickly understand project progress.
- Identify blocked or risky items.
- Generate status updates for stakeholders.
- Know which owners need follow-up.
- Decide whether project status should move forward.

### Product Manager

Secondary user.

Needs:

- See whether requirements are ready for scheduling.
- Understand requirement-side blockers.

### Developer / Tester / Team Lead

Indirect users.

Needs:

- Receive focused follow-up lists instead of broad status meetings.
- Understand which items are blocking project progress.

## 5. User Stories

### US-1: Summarize Iteration Progress

As a project manager, I can summarize an iteration's progress in one command.

Example:

```bash
tapd assistant project progress --iteration-id 12345
```

Expected output:

- Overall progress.
- Story status distribution.
- Task status distribution.
- Bug status distribution.
- Delayed or stale items.
- Owner-level follow-ups.
- Risk summary.
- Suggested next actions.

### US-2: Find Project Blockers

As a project manager, I can list blockers across an iteration or workspace.

Example:

```bash
tapd assistant project blockers --iteration-id 12345
```

Expected output:

- Items with no owner.
- Items with no recent update.
- Bugs blocking stories.
- Stories missing tasks.
- Tasks overdue or stuck.
- Requirements not ready for scheduling.

### US-3: Generate Daily Or Weekly Report

As a project manager, I can generate a report for stakeholders.

Example:

```bash
tapd assistant project report --iteration-id 12345 --period daily
```

Expected output:

- Progress since last report window.
- Newly completed items.
- Newly added risks.
- Current blockers.
- Owner follow-ups.
- Tomorrow or next-week focus.

### US-4: Optional Comment Or Message Delivery

As a project manager, I can explicitly write a report back to TAPD or send it to Enterprise WeChat.

Examples:

```bash
tapd assistant project progress --iteration-id 12345 --comment
tapd assistant project report --iteration-id 12345 --send-qiwei
```

Expected behavior:

- `--comment` writes one structured TAPD comment to the iteration or configured tracking story.
- `--send-qiwei` sends the Markdown report through the existing `qiwei` integration or configured webhook.
- Neither option should be enabled by default.

### US-5: Status Transition Recommendation

As a project manager, I can ask whether a project or iteration should move to the next state.

Example:

```bash
tapd assistant project status-suggest --iteration-id 12345
```

Expected output:

- Current status.
- Suggested next status.
- Evidence supporting or blocking the transition.
- Items that must be resolved first.

MVP should only recommend transitions. Actual transition execution should be a later feature with explicit approval flags.

## 6. Proposed CLI Surface

Add a new role-oriented command group:

```text
tapd assistant project
```

Initial subcommands:

```text
tapd assistant project progress
tapd assistant project blockers
tapd assistant project report
tapd assistant project status-suggest
```

### 6.1 progress

Purpose:

- Summarize project or iteration progress.

Inputs:

- `--iteration-id <id>`: target iteration.
- `--workspace-id <id>`: inherited global workspace flag.
- `--story-filter <filter>`: optional advanced story filters.
- `--task-filter <filter>`: optional advanced task filters.
- `--bug-filter <filter>`: optional advanced bug filters.
- `--stale-days <n>`: item is stale if not updated for n days.
- `--json`: output machine-readable JSON.
- `--comment`: write report to TAPD.

Default behavior:

- Read TAPD data.
- Print Markdown report.
- No TAPD write.

### 6.2 blockers

Purpose:

- Find concrete project blockers.

Inputs:

- `--iteration-id <id>` optional but recommended.
- `--owner <name>` optional owner filter.
- `--stale-days <n>` default 3 or 5.
- `--json`.

Default behavior:

- Print grouped blockers.
- No TAPD write.

### 6.3 report

Purpose:

- Generate stakeholder-facing daily or weekly report.

Inputs:

- `--iteration-id <id>`.
- `--period daily|weekly`.
- `--since <YYYY-MM-DD>` optional explicit start date.
- `--comment`.
- `--send-qiwei`.
- `--qiwei-webhook <url>` or reuse existing Qiwei configuration.
- `--json`.

Default behavior:

- Print Markdown report.
- No external send.

### 6.4 status-suggest

Purpose:

- Recommend whether the project/iteration can move to the next status.

Inputs:

- `--iteration-id <id>`.
- `--target-status <status>` optional.
- `--json`.

Default behavior:

- Print recommendation only.
- No status transition.

Future behavior:

- `--apply` can execute status transitions only after a separate implementation spec.

## 7. Data Sources

MVP should rely on existing TAPD data:

- Iteration detail and list.
- Stories in iteration.
- Tasks in iteration or linked to stories.
- Bugs in iteration or linked to stories.
- Story, task, and bug status.
- Owners/current owners.
- Priority and severity.
- Modified time.
- Comments where needed.

Future sources:

- Enterprise WeChat demand group messages.
- GitLab MR status.
- CI status.
- Launch form status.
- Test case execution data.

## 8. Analysis Rules

### 8.1 Progress Metrics

Calculate:

- Total stories.
- Stories by status.
- Total tasks.
- Tasks by status.
- Total bugs.
- Bugs by status/severity.
- Done ratio for stories and tasks.
- Open bug count and high-severity bug count.

### 8.2 Blocker Rules

Flag an item as blocked or risky if:

- Owner/current owner is empty.
- Modified time is older than `--stale-days`.
- Due date is before today and item is not done.
- Story has no linked tasks.
- Story has open high-severity bugs.
- Bug is assigned to no one.
- Bug has no description.
- Requirement readiness check fails when product assistant rules are available.

### 8.3 Owner Follow-Up Rules

Group follow-ups by owner:

- Stale stories.
- Stale tasks.
- Open bugs.
- Overdue items.
- Items missing required material.

### 8.4 Status Suggestion Rules

Recommend "can move forward" only when:

- Required stories are ready.
- No critical open bugs remain.
- No overdue must-fix tasks remain.
- No high-priority story is ownerless.
- Configured workflow prerequisites are satisfied.

If evidence is incomplete, output "not enough information" rather than guessing.

## 9. Output Format

### 9.1 Markdown Progress Report

Default output:

```markdown
# Project Progress Report

Scope: Iteration 12345
Generated At: 2026-06-09 14:00

## Summary
- Stories: 18 total, 10 done, 5 in progress, 3 blocked
- Tasks: 42 total, 30 done, 8 in progress, 4 overdue
- Bugs: 12 open, 2 high severity

## Key Risks
- Story 1123 has no owner.
- Bug 9988 is high severity and still open.

## Blockers
- ...

## Owner Follow-Ups
- alice: ...
- bob: ...

## Suggested Next Actions
- ...
```

### 9.2 JSON Output

`--json` should emit stable machine-readable data:

```json
{
  "scope": {
    "workspace_id": "20063271",
    "iteration_id": "12345"
  },
  "summary": {
    "stories_total": 18,
    "stories_done": 10,
    "tasks_total": 42,
    "bugs_open": 12,
    "high_severity_bugs_open": 2
  },
  "blockers": [
    {
      "entity_type": "bug",
      "id": "9988",
      "code": "high_severity_open_bug",
      "owner": "alice",
      "message": "High severity bug is still open."
    }
  ],
  "owner_followups": []
}
```

## 10. Architecture

### 10.1 Command Layer

Suggested files:

```text
internal/cmd/assistant_project.go
internal/cmd/assistant_project_progress.go
internal/cmd/assistant_project_report.go
```

Responsibilities:

- Parse flags.
- Resolve workspace and iteration.
- Call collector and analyzer.
- Render Markdown or JSON.
- Optionally write TAPD comments or send Qiwei messages.

### 10.2 Context Collector

Introduce a context type:

```go
type projectContext struct {
    WorkspaceID string
    Iteration   iterationSnapshot
    Stories     []storySnapshot
    Tasks       []taskSnapshot
    Bugs        []bugSnapshot
    Comments    map[string][]commentSnapshot
}
```

MVP can start with stories, tasks, and bugs. Comments can be added only where needed to avoid API cost.

### 10.3 Analyzer

Use deterministic rules first:

- Status distribution.
- Owner missing.
- Stale modified time.
- Due date overdue.
- Bug severity and status.
- Story readiness when available.

No hidden LLM call is required for MVP.

### 10.4 Renderer

Provide:

- Markdown report renderer.
- JSON report renderer.
- Compact comment renderer for TAPD comments.
- Qiwei Markdown renderer if `--send-qiwei` is enabled.

### 10.5 Delivery Layer

Reuse existing capabilities:

- TAPD comment add.
- Qiwei send.
- Existing output package.

## 11. Data Flow

### 11.1 progress

1. User runs `tapd assistant project progress --iteration-id <id>`.
2. CLI loads stories, tasks, and bugs for the iteration.
3. Analyzer calculates progress metrics.
4. Analyzer detects blockers and owner follow-ups.
5. CLI renders Markdown or JSON.
6. If `--comment` is set, CLI writes one report comment.

### 11.2 blockers

1. User runs `tapd assistant project blockers`.
2. CLI loads scoped TAPD items.
3. Analyzer applies blocker rules.
4. CLI groups blockers by severity, entity type, and owner.
5. CLI prints report.

### 11.3 report

1. User runs `tapd assistant project report --period daily`.
2. CLI loads scoped TAPD items and recent changes.
3. Analyzer produces stakeholder-facing report sections.
4. CLI prints report.
5. Optional delivery writes TAPD comment or sends Qiwei message.

### 11.4 status-suggest

1. User runs `tapd assistant project status-suggest`.
2. CLI loads current project/iteration context.
3. Analyzer checks transition prerequisites.
4. CLI prints recommendation and evidence.
5. No status transition happens in MVP.

## 12. Safety And Permissions

Default behavior must be read-only.

Rules:

- No TAPD write unless `--comment` is explicitly set.
- No Enterprise WeChat send unless `--send-qiwei` is explicitly set.
- No status transition in MVP.
- No owner assignment in MVP.
- No schedule modification in MVP.
- Reports must show when data is incomplete.
- Commands should prefer "not enough information" over unsupported inference.

## 13. Error Handling

Expected error cases:

- Missing workspace ID.
- Missing iteration ID where required.
- Invalid `--period`.
- Invalid `--stale-days`.
- TAPD API authentication failure.
- TAPD list API failure.
- Qiwei send failure.
- TAPD comment write failure.

Behavior:

- Read-only report failures should return a structured CLI error.
- Optional delivery failures should not hide the generated report.
- Partial data should be surfaced as warnings when possible.

## 14. MCP Surface

After CLI MVP, expose project assistant tools:

- `tapd_assistant_project_progress`
- `tapd_assistant_project_blockers`
- `tapd_assistant_project_report`
- `tapd_assistant_project_status_suggest`

MCP output should prefer JSON so AI clients can reuse metrics and blockers.

## 15. Configuration

MVP can use CLI flags only.

Future config may live in:

```text
~/.tapd-assistant/project.yaml
./.tapd-assistant/project.yaml
```

Configurable fields:

- Stale-day threshold.
- Done statuses for stories/tasks/bugs.
- Blocked statuses.
- Critical bug severities.
- Report template.
- Qiwei webhook.
- Comment target behavior.
- Status transition prerequisites.

## 16. Testing Strategy

Unit tests:

- Progress metric calculation.
- Blocker detection.
- Stale item detection.
- Owner follow-up grouping.
- Markdown rendering.
- JSON output.
- Status suggestion rules.

Command tests:

- Mock TAPD story/task/bug list responses.
- Run `project progress`.
- Run `project blockers`.
- Run `project report`.
- Verify `--comment` writes one TAPD comment.
- Verify `--send-qiwei` calls delivery only when explicitly set.

No live TAPD or Qiwei dependency should be required in automated tests.

## 17. MVP Acceptance Criteria

The feature is acceptable when:

- `tapd assistant project progress --iteration-id <id>` produces a useful progress report.
- `tapd assistant project blockers --iteration-id <id>` lists concrete blockers.
- `tapd assistant project report --iteration-id <id> --period daily|weekly` produces a stakeholder-ready report.
- `tapd assistant project status-suggest --iteration-id <id>` produces a recommendation without changing TAPD state.
- All commands are read-only by default.
- `--comment` writes exactly one TAPD comment when enabled.
- JSON output is stable enough for MCP reuse.
- Tests cover metrics, blockers, rendering, and command behavior.

## 18. Future Extensions

- Ingest Enterprise WeChat group messages as report context.
- Track delta since last generated report.
- Integrate GitLab MR status and CI status.
- Apply status transitions with explicit `--apply`.
- Create or update project dashboard wiki pages.
- Generate owner-specific Qiwei reminders.
- Support project-level health score.
- Support custom report templates.

## 19. Open Questions

- Should project scope be iteration-first, story-filter-first, or project-wide by default?
- Which statuses count as done, blocked, or in-progress in the target workspace?
- Should reports comment on an iteration, a tracking story, or a project wiki page?
- What stale threshold should be the team default?
- Which bug severities should block project status movement?
- Should status suggestions target TAPD iteration status, launch status, or a custom project status field?

## 20. Recommended First Implementation

Implement in this order:

1. Add project context collector for stories, tasks, and bugs scoped by iteration.
2. Add deterministic progress metrics.
3. Add blocker detection.
4. Add Markdown and JSON renderers.
5. Add `tapd assistant project progress`.
6. Add `tapd assistant project blockers`.
7. Add `tapd assistant project report`.
8. Add `tapd assistant project status-suggest`.
9. Add optional `--comment`.
10. Expose project assistant tools through MCP.
