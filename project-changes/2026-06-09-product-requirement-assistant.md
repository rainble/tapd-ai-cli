# Product Requirement Assistant Spec

## 1. Background

The overall TAPD AI Work Assistant direction defines role-oriented workflows on top of the existing TAPD CLI, MCP, and webhook capabilities. The first role-specific feature should target product managers because requirement submission is a frequent, process-heavy workflow with clear value and low execution risk.

Product managers often need to answer these questions before creating or submitting a TAPD story:

- What is the correct requirement submission process?
- What materials must be prepared before review?
- Is the current requirement description complete enough?
- Which fields, attachments, acceptance criteria, dependencies, or risks are missing?
- Is the requirement ready for review or should it be improved first?

This spec defines the first product-facing assistant: a requirement submission assistant that helps product managers turn rough input or existing TAPD stories into review-ready requirement material.

## 2. Goal

Build a product requirement assistant that can:

- Explain the team's requirement submission checklist.
- Generate a structured requirement draft from free-form input.
- Check an existing TAPD story for missing materials.
- Produce review-readiness output with concrete improvement suggestions.
- Optionally write results back to TAPD only when explicitly enabled.

The first implementation should be conservative: read-only by default, structured output first, TAPD writes later or behind explicit flags.

## 3. Non-Goals

The MVP should not:

- Replace the TAPD story creation page.
- Automatically submit requirements for review without explicit user action.
- Automatically schedule meetings.
- Automatically approve or reject requirements.
- Depend on Enterprise WeChat history ingestion.
- Require a web UI.
- Require a custom LLM provider integration inside the CLI.

The CLI should prepare context and structured prompts/results. AI clients can call it through MCP or users can use the command output directly.

## 4. Target Users

### Product Manager

Primary user.

Needs:

- Convert an idea into a structured requirement.
- Check whether an existing requirement is ready for review.
- Understand what is missing before asking reviewers or developers.

### Project Manager

Secondary user.

Needs:

- Quickly identify requirements that are not ready for scheduling.
- See why a requirement is blocked before entering planning.

### Developer / Tester

Indirect users.

Needs:

- Receive clearer requirements with acceptance criteria, scope, risks, and dependencies.
- Avoid back-and-forth caused by missing requirement material.

## 5. User Stories

### US-1: Draft Requirement From Rough Input

As a product manager, I can provide a rough idea or local Markdown file and get a structured requirement draft.

Example:

```bash
tapd assistant product draft-story \
  --input "希望支持主播付费视频自动续费签约数据看板"
```

Expected output:

- Background
- Goal
- User value
- Scope
- Non-scope
- Functional requirements
- Acceptance criteria
- Data requirements
- Risk and dependency list
- Open questions

### US-2: Check Existing TAPD Story

As a product manager, I can check an existing story before review.

Example:

```bash
tapd assistant product check-story 1120063271004942331
```

Expected output:

- Overall readiness result
- Missing required sections
- Missing TAPD fields
- Missing attachments or links
- Ambiguous acceptance criteria
- Suggested next edits

### US-3: Review-Ready Gate

As a product manager, I can ask whether a story is ready for requirement review.

Example:

```bash
tapd assistant product review-ready 1120063271004942331
```

Expected output:

- `ready`: true or false
- Blocking issues
- Non-blocking suggestions
- Suggested review summary

### US-4: Optional TAPD Comment

As a product manager, I can explicitly write the checklist result back to TAPD as a comment.

Example:

```bash
tapd assistant product check-story 1120063271004942331 --comment
```

Expected behavior:

- The assistant adds one structured TAPD comment.
- No status transition happens unless a later explicit flag is designed.

## 6. Proposed CLI Surface

Add a new role-oriented command group:

```text
tapd assistant product
```

Initial subcommands:

```text
tapd assistant product draft-story
tapd assistant product check-story <story_id_or_url>
tapd assistant product review-ready <story_id_or_url>
```

### 6.1 draft-story

Purpose:

- Convert rough input into a structured requirement draft.

Inputs:

- `--input <text>`: direct idea text.
- `--file <path>`: local Markdown/text file.
- `--template <name>`: optional requirement template name.
- `--json`: output machine-readable JSON.

Default behavior:

- Print Markdown to stdout.
- No TAPD write.

Future behavior:

- `--create`: create a TAPD story after explicit opt-in.
- `--parent-id`, `--iteration-id`, `--owner`, `--priority`: fields used only when creating.

### 6.2 check-story

Purpose:

- Check whether an existing TAPD story has enough material for review.

Inputs:

- `<story_id_or_url>`: TAPD story ID or URL.
- `--template <name>`: optional checklist template.
- `--comment`: write checklist result as TAPD comment.
- `--json`: output machine-readable JSON.

Context loaded:

- Story detail.
- Story comments.
- Attachments if available.
- Child tasks if available.
- Linked bugs if available.
- Iteration and owner fields.

Default behavior:

- Print readiness report.
- No TAPD write.

### 6.3 review-ready

Purpose:

- Provide a stricter go/no-go readiness gate before review.

Inputs:

- `<story_id_or_url>`: TAPD story ID or URL.
- `--comment`: write result as TAPD comment.
- `--json`: output machine-readable JSON.

Default behavior:

- Print a concise review decision.
- No TAPD write.

## 7. Requirement Readiness Checklist

The MVP checklist should be explicit and configurable later.

### Required Content Sections

A story is not review-ready if any of these are missing:

- Background: why this requirement exists.
- Goal: what success looks like.
- User or business value: who benefits and how.
- Scope: what is included.
- Acceptance criteria: how reviewers/testers can verify completion.
- Owner: who owns the requirement.

### Strongly Recommended Sections

Missing these should produce warnings, not blockers:

- Non-scope: what is intentionally excluded.
- Data requirements: metrics, data sources, dashboards, logs, or reporting needs.
- Dependencies: upstream/downstream systems, people, or approvals.
- Risks: product, technical, compliance, launch, or data risks.
- Rollout plan: gray release, experiment, monitoring, fallback.
- Test focus: key test scenarios.

### TAPD Field Checks

The checker should inspect available TAPD fields:

- `name`
- `description`
- `owner`
- `developer`
- `priority_label`
- `iteration_id`
- `category_id`
- `module`
- `parent_id`
- custom fields when requested by configuration

MVP should only treat `name`, `description`, and `owner` as hard blockers unless a template config says otherwise.

## 8. Output Format

### 8.1 Markdown Output

Default output should be human-readable:

```markdown
# Requirement Readiness Report

Story: <id> <name>
Ready: No

## Blocking Issues
- Missing acceptance criteria.
- Missing requirement owner.

## Suggestions
- Add non-scope to clarify whether historical data migration is included.
- Add data source and metric definition for dashboard fields.

## Suggested Review Summary
...
```

### 8.2 JSON Output

`--json` should produce stable machine-readable output:

```json
{
  "story_id": "1120063271004942331",
  "ready": false,
  "score": 68,
  "blocking_issues": [
    {
      "code": "missing_acceptance_criteria",
      "message": "Acceptance criteria are missing."
    }
  ],
  "warnings": [],
  "suggestions": [],
  "summary": "..."
}
```

## 9. Architecture

### 9.1 Command Layer

New files should follow existing `internal/cmd` Cobra patterns.

Suggested structure:

```text
internal/cmd/assistant.go
internal/cmd/assistant_product.go
internal/cmd/assistant_product_check.go
internal/cmd/assistant_product_draft.go
```

Responsibilities:

- Parse flags.
- Resolve story ID or URL.
- Call assistant service.
- Print Markdown or JSON.
- Optionally write comments when `--comment` is set.

### 9.2 Context Collector

Introduce a small product requirement context type:

```go
type productRequirementContext struct {
    Story       storySnapshot
    Comments    []commentSnapshot
    Tasks       []taskSnapshot
    Bugs        []bugSnapshot
    Attachments []attachmentSnapshot
}
```

MVP can start with story detail and comments only. The type should leave room for tasks, bugs, and attachments.

### 9.3 Rule Engine

Use deterministic checks first.

Examples:

- Description is empty.
- Owner is empty.
- Acceptance criteria section is missing.
- Background section is missing.
- Goal section is missing.

Do not require an LLM for MVP readiness scoring. AI clients can use the structured context and report for deeper rewriting.

### 9.4 Draft Generator

For `draft-story`, MVP can generate a structured Markdown skeleton from provided input.

It should not pretend to know unavailable details. Missing areas should be marked as open questions.

Example sections:

- Background
- Goal
- User Story
- Scope
- Non-Scope
- Functional Requirements
- Acceptance Criteria
- Data Requirements
- Dependencies
- Risks
- Rollout Plan
- Open Questions

## 10. Data Flow

### 10.1 check-story

1. User runs `tapd assistant product check-story <story_id_or_url>`.
2. CLI resolves story ID and workspace ID.
3. CLI loads story detail.
4. CLI loads story comments.
5. Rule engine checks required sections and fields.
6. CLI builds readiness report.
7. CLI prints Markdown or JSON.
8. If `--comment` is set, CLI adds a TAPD comment with the report.

### 10.2 draft-story

1. User runs `tapd assistant product draft-story --input ...` or `--file ...`.
2. CLI reads input.
3. Draft generator maps rough input into the standard template.
4. Missing details are placed under Open Questions.
5. CLI prints Markdown or JSON.
6. Future `--create` can create a TAPD story after explicit opt-in.

### 10.3 review-ready

1. User runs `tapd assistant product review-ready <story_id_or_url>`.
2. CLI reuses check-story context and rule engine.
3. CLI applies stricter readiness gating.
4. CLI prints a concise decision and blockers.
5. Optional `--comment` writes the decision.

## 11. Safety And Permissions

Default behavior must be read-only.

Rules:

- `draft-story` prints output only.
- `check-story` prints output only.
- `review-ready` prints output only.
- `--comment` is required to write TAPD comments.
- No story creation in MVP unless separately designed.
- No status transition in MVP.
- No meeting reservation in MVP.
- No hidden AI network call inside CLI.

## 12. Error Handling

Expected error cases:

- Invalid story ID or URL.
- Story not found.
- Missing workspace ID.
- TAPD API authentication failure.
- TAPD comments API failure.
- Both `--input` and `--file` missing for `draft-story`.
- Both `--input` and `--file` provided for `draft-story`.
- Local input file cannot be read.

Behavior:

- Return structured CLI errors using existing output conventions.
- For `check-story`, if comments fail to load, still check story detail and include a warning.
- For `--comment`, if comment writing fails, return a write failure but keep the generated report visible.

## 13. MCP Surface

After CLI MVP, expose matching MCP tools:

- `tapd_assistant_product_draft_story`
- `tapd_assistant_product_check_story`
- `tapd_assistant_product_review_ready`

MCP outputs should prefer JSON so AI clients can reason over blockers and suggestions.

## 14. Template Configuration

MVP can ship one built-in template named `default`.

Future template config can be stored in:

```text
~/.tapd-assistant/templates/product-requirement.yaml
./.tapd-assistant/templates/product-requirement.yaml
```

Configuration should allow teams to define:

- Required sections.
- Recommended sections.
- Required TAPD fields.
- Custom field labels.
- Readiness scoring weights.
- Comment format.

Do not build the full template system in the first implementation unless needed. Keep the built-in template deterministic.

## 15. Testing Strategy

Unit tests:

- Story URL and ID resolution.
- Draft skeleton generation.
- Required section detection.
- Readiness report scoring.
- Markdown output.
- JSON output.
- `--comment` write behavior.
- Error cases for missing input and invalid story.

Integration-style command tests:

- Mock TAPD story detail and comments.
- Run `check-story`.
- Verify blockers and suggestions.
- Run `review-ready`.
- Verify decision.
- Run `draft-story`.
- Verify expected sections.

No live TAPD dependency should be required in automated tests.

## 16. MVP Acceptance Criteria

The feature is acceptable when:

- A user can draft a structured requirement from free-form text.
- A user can check an existing TAPD story and see clear blockers.
- The command is read-only by default.
- `--comment` writes one TAPD comment with the readiness report.
- Output is available in Markdown and JSON.
- Tests cover the rule engine and command behavior.
- Existing TAPD CRUD, MCP, watch, and bug auto-fix commands continue to pass tests.

## 17. Future Extensions

- Create TAPD story from draft with explicit `--create`.
- Reserve requirement review or launch review.
- Attach generated checklist to TAPD custom fields.
- Read team-specific templates from config.
- Use AI clients through MCP for rewriting requirement text.
- Ingest Enterprise WeChat group discussion as additional context.
- Generate impact analysis for developers and testers after requirement changes.

## 18. Open Questions

- What is the team's official product requirement template?
- Which TAPD fields are mandatory before review in the current workflow?
- Should `owner` mean product owner, TAPD story owner, or current handler?
- Should the assistant write checklist comments by default in team mode, or always require `--comment`?
- Does review readiness require launch form or review meeting integration?

## 19. Recommended First Implementation

Implement in this order:

1. Add `tapd assistant product draft-story` as a local-only command.
2. Add deterministic readiness rule engine.
3. Add `tapd assistant product check-story`.
4. Add `tapd assistant product review-ready`.
5. Add optional `--comment`.
6. Expose the three capabilities through MCP.

This order gives product managers value quickly while keeping the first version safe and testable.
