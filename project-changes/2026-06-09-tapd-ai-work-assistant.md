# TAPD AI Work Assistant Spec

## 1. Background

`tapd-ai-cli` started as a command line tool for AI agents to operate TAPD through Open API. It already supports core TAPD entities, URL resolution, MCP integration, webhook watching, and a concrete automation flow for bug auto-fixing.

The next product direction is to position the tool as a TAPD assistant for all project roles, not only for coding agents. The assistant should help product managers, project managers, developers, testers, and other roles finish common TAPD workflows with less manual lookup and less context switching.

## 2. Product Positioning

`tapd-ai-cli` should become a role-oriented TAPD work assistant:

> A CLI, MCP, and webhook based assistant that uses TAPD as the system of record, understands role-specific workflows, and can safely query, summarize, recommend, and execute TAPD actions.

The product should not replace the TAPD web UI. It should focus on repeatable, context-heavy workflows where AI and automation provide leverage:

- Collecting TAPD context across stories, tasks, bugs, comments, iterations, workflows, and reviews.
- Summarizing project state, blockers, and missing materials.
- Generating or updating structured TAPD content.
- Triggering safe automation from TAPD webhook events.
- Connecting TAPD records with local repositories, merge requests, and coding agents.

## 3. Existing Capabilities

The current repository already provides the foundation:

- TAPD CRUD commands for stories, tasks, bugs, comments, iterations, wiki pages, launch forms, workflows, test cases, and related resources.
- `tapd url <url>` for resolving TAPD links into entity details.
- `--filter` support for advanced TAPD OpenAPI querying.
- `tapd mcp` for exposing TAPD tools to AI clients such as Codex, Claude Code, and Cursor.
- `tapd watch` for subscribing to TAPD webhook events through SSE.
- `tapd agent fix-bugs` for developer automation:
  - listens to bug create/update events,
  - loads bug detail and comments,
  - skips empty bugs and bugs not owned by the current user,
  - finds MR links from the linked story, parent story, or bug,
  - checks out the local MR branch,
  - runs a coding agent and verification command,
  - writes TAPD comments and optionally transitions bug status.
- `tapd qiwei send` for sending Enterprise WeChat messages.
- `tapd skill init` for generating AI coding tool skill instructions.

## 4. Target Roles And Scenarios

### 4.1 Product Manager

Needs:

- Understand the demand submission process.
- Know which materials are required before submitting a story.
- Convert rough ideas into structured TAPD requirements.
- Check whether a story is ready for review.
- Schedule or prepare requirement and launch reviews.

Candidate capabilities:

- `tapd assistant product draft-story`
  - Generate a story draft from free-form input.
  - Fill background, goal, scope, acceptance criteria, risks, dependencies, and rollout notes.
- `tapd assistant product check-story <story_id>`
  - Check whether required fields and materials are complete.
  - Report missing information and suggested fixes.
- `tapd assistant product review-ready <story_id>`
  - Decide whether a story is ready for review.
  - Optionally add a TAPD comment with the checklist result.

### 4.2 Project Manager

Needs:

- Understand project progress from TAPD stories, tasks, bugs, and iterations.
- Identify blockers and stale work.
- Summarize demand group messages together with TAPD status.
- Automatically transition project status or notify owners.

Candidate capabilities:

- `tapd assistant project progress --iteration-id <id>`
  - Summarize story/task/bug progress for an iteration.
  - Highlight delayed items, missing owners, stale updates, and blocking bugs.
- `tapd assistant project blockers`
  - Find items that have not moved for a configured time window.
  - Group blockers by owner, module, iteration, or status.
- `tapd assistant project report`
  - Generate daily or weekly project reports.
  - Optionally send the report to Enterprise WeChat.
- `tapd agent project-watch`
  - Listen to TAPD story/task/bug events.
  - Apply configured status transition or notification rules.

### 4.3 Developer

Needs:

- Split development tasks from requirements.
- Keep task changes synchronized when requirements change.
- Associate TAPD items with merge requests and local branches.
- Automatically fix bugs when enough context is available.

Candidate capabilities:

- `tapd assistant dev split-task <story_id>`
  - Analyze a story and create development task drafts.
  - Support dry-run first, then confirmed TAPD task creation.
- `tapd assistant dev sync-change <story_id>`
  - Summarize requirement changes.
  - Identify impacted tasks, bugs, and merge requests.
- `tapd agent fix-bugs`
  - Continue as the main developer automation flow.
  - Extend later with optional commit/push support guarded by explicit flags.

### 4.4 Tester

Needs:

- Quickly find bugs waiting for verification.
- Transition bug status according to test results.
- Generate or update test cases from stories.
- Track testing impact after requirement changes.

Candidate capabilities:

- `tapd assistant qa todo`
  - List current user's pending bugs, test cases, and stories.
- `tapd assistant qa verify-bug <bug_id>`
  - Record verification result.
  - Transition bug status when explicitly allowed.
- `tapd assistant qa gen-cases <story_id>`
  - Generate test case drafts from story content.
  - Support dry-run and confirmed TAPD test case creation.
- `tapd agent bug-flow`
  - Listen to bug events and apply safe status/comment automation.

## 5. Product Shape

The assistant should expose four entry styles.

### 5.1 Manual CLI

For explicit user-driven actions:

```bash
tapd story show <id>
tapd bug update <id> --status resolved
tapd assistant product check-story <story_id>
```

### 5.2 MCP Tools

For AI clients that need structured TAPD tools:

```bash
tapd mcp
```

The MCP surface should gradually expose assistant-level tools, not only low-level CRUD.

### 5.3 Event Agents

For unattended or semi-attended automation:

```bash
tapd agent fix-bugs
tapd agent project-watch
tapd agent bug-flow
```

Agents should use explicit safety flags for every write, transition, or local command execution.

### 5.4 Role-Oriented Assistant Commands

For productized workflows:

```bash
tapd assistant product ...
tapd assistant project ...
tapd assistant dev ...
tapd assistant qa ...
```

This should become the primary user-facing layer for non-developer roles.

## 6. Architecture

### 6.1 TAPD API Layer

Responsibilities:

- Wrap story, task, bug, comment, workflow, iteration, launch, test case, and relation APIs.
- Normalize entity IDs, URL resolution, comments, markdown/html conversion, and pagination.
- Preserve advanced filtering for power users and agents.

### 6.2 Context Collector Layer

Responsibilities:

- Build rich context packages from TAPD entities.
- Resolve linked stories, parent stories, child tasks, bugs, comments, iterations, workflows, owners, and status history where available.
- Optionally enrich context with Git/MR data, Enterprise WeChat messages, and local repository state.

### 6.3 Role Assistant Layer

Responsibilities:

- Implement role-specific workflows.
- Convert low-level TAPD operations into user-facing actions such as "check story readiness", "summarize project progress", "split tasks", and "verify bug".
- Produce structured outputs that can be read by humans and AI clients.

### 6.4 Policy And Guardrail Layer

Responsibilities:

- Enforce safe defaults.
- Decide whether a command may write TAPD data, transition status, execute local commands, modify files, or send messages.
- Support dry-run, confirmation, owner checks, dirty workspace checks, and explicit allow flags.

### 6.5 Execution Layer

Responsibilities:

- Execute TAPD updates, comments, status transitions, local shell commands, Git commands, and Enterprise WeChat sends.
- Emit machine-readable JSON results.
- Record enough detail for auditing and debugging.

## 7. Core Data Flow Examples

### 7.1 Bug Auto-Fix

1. `tapd agent fix-bugs` receives a TAPD bug event from SSE.
2. The agent loads bug detail and comments.
3. The guardrail layer skips unsafe cases:
   - bug description is empty,
   - current owner does not include the logged-in user,
   - local repo is dirty,
   - linked MR cannot be found.
4. The context collector resolves MR links in this order:
   - linked story,
   - parent story chain,
   - bug description/comments.
5. The execution layer fetches and checks out the local MR branch.
6. The coding agent runs with a generated prompt.
7. The verification command runs.
8. The agent writes a TAPD comment and optionally transitions status.

### 7.2 Project Progress Summary

1. User runs `tapd assistant project progress --iteration-id <id>`.
2. The context collector loads stories, tasks, bugs, comments, and iteration metadata.
3. The assistant groups work by status, owner, module, and risk.
4. The output includes:
   - completion summary,
   - delayed items,
   - stale items,
   - blockers,
   - owner-specific follow-ups,
   - suggested next actions.
5. Optional execution sends the report to Enterprise WeChat or comments on TAPD.

### 7.3 Story Readiness Check

1. User runs `tapd assistant product check-story <story_id>`.
2. The context collector loads story detail, comments, attachments, linked tasks, linked bugs, and review status when available.
3. The assistant checks configured readiness rules:
   - background exists,
   - goal is clear,
   - scope and non-scope are described,
   - acceptance criteria exist,
   - owner and iteration are set,
   - dependencies and risks are declared.
4. The command prints a readiness report.
5. Optional execution writes the checklist result as a TAPD comment.

## 8. Safety Principles

The assistant must remain conservative by default.

- Read-only commands should be easy to run.
- Write operations should support `--dry-run`.
- Status transitions must require explicit flags or confirmation.
- Local command execution must be explicit.
- Automatic code modification must require a clean repository.
- Bug automation should skip events without enough context.
- Owner mismatch should skip rather than modify.
- Commit, push, merge, deploy, or release actions must not happen unless explicitly implemented and enabled.
- All automation should emit structured JSON results.

## 9. MVP Scope

Recommended first product milestone:

1. Keep improving `tapd agent fix-bugs`.
   - This is already the most concrete end-to-end automation.
   - It proves webhook, TAPD context, Git/MR mapping, local agent execution, verification, and TAPD feedback.

2. Add `tapd assistant product check-story`.
   - Low risk because it can be read-only first.
   - Useful for product managers immediately.

3. Add `tapd assistant project progress`.
   - Builds on existing list/filter APIs.
   - Useful for project managers and team leads.

4. Add `tapd assistant dev split-task`.
   - Start with dry-run task drafts.
   - Add confirmed TAPD task creation later.

## 10. Non-Goals For MVP

- No web UI.
- No automatic commit or push.
- No automatic merge, deploy, or release.
- No broad autonomous project management without explicit rules.
- No hidden TAPD writes.
- No dependency on Enterprise WeChat message history until a stable source is available.

## 11. Open Questions

- Which role command should be implemented first after bug auto-fix: product readiness check or project progress summary?
- Where should workflow rules live: config file, TAPD custom fields, or CLI flags?
- Which TAPD fields are mandatory for story readiness in the target team?
- How should Enterprise WeChat group messages be ingested: manual paste, exported file, bot callback, or API integration?
- Should role assistants be exposed as MCP tools in the same milestone as CLI commands?

## 12. Success Criteria

The product direction is successful when:

- Product managers can validate story readiness without asking another person for the process checklist.
- Project managers can get a useful progress and blocker summary from TAPD data in one command.
- Developers can let the agent handle eligible TAPD bugs with safe skip behavior.
- Testers can query and transition common bug workflows with fewer manual TAPD steps.
- AI clients can use MCP tools to perform the same workflows with structured inputs and outputs.

## 13. Suggested Next Change Specs

- `product-story-readiness-check`
- `project-progress-summary`
- `dev-task-split-dry-run`
- `qa-bug-flow-status-transition`
- `agent-fix-bugs-gitlab-api-source-branch`
