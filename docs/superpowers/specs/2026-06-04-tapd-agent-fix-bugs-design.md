# TAPD Agent Bug Fix Design

## Context

`vas/app/upower/interface` already exposes a TAPD webhook relay:

- TAPD pushes events to `POST /x/upower/tapd/webhook`.
- The service verifies the shared secret, stores recent events through Redis when available, and fans events out through `GET /x/upower/tapd/events`.
- `tapd-ai-cli` already has `tapd watch`, which consumes the SSE stream, filters events, persists `last_event_id`, and can pass events to an external command.

The missing piece is a local, controlled bug-fixing worker. The worker should react to TAPD bug events, let a coding agent inspect and modify the local repository, verify the result, and write the result back to TAPD. The service side must remain a relay and must not receive local repository write permissions.

## Goal

Add a `tapd agent fix-bugs` command to `tapd-ai-cli` that runs locally, listens for TAPD bug events, invokes a configured coding agent to fix the corresponding code, verifies with a configured test command, writes a TAPD comment, and moves the bug to a configured success status. It must not commit, push, create merge requests, or merge code.

## Non-Goals

- Do not run coding agents inside the `upower/interface` service.
- Do not automatically commit, push, create merge requests, deploy, or merge.
- Do not implement a distributed queue in TAPD or Redis.
- Do not attempt to infer every team's workflow status. Status values are explicit configuration.
- Do not process stories, tasks, wiki changes, or non-bug webhook events in this command.

## Recommended Command

```bash
tapd agent fix-bugs \
  --repo /Users/sunruoyu/go/src/vas/app/upower \
  --endpoint https://your-host/x/upower/tapd/events \
  --token <subscribe-token> \
  --workspace-id <workspace-id> \
  --on-start-status in_progress \
  --on-success-status resolved \
  --on-failure-status "" \
  --test-cmd "go test ./..." \
  --agent-cmd "codex exec --full-auto"
```

The command is intentionally configuration-driven. The same binary can run against different repositories and TAPD workflows without hardcoding project-specific status names.

## Architecture

`tapd agent fix-bugs` lives in `tapd-ai-cli`, not in `upower/interface`.

The new command reuses existing CLI capabilities where possible:

- Use the same configuration precedence as other commands for TAPD credentials and default workspace.
- Use the existing SSE protocol and `last_event_id` state model from `tapd watch`.
- Use the SDK client for `bug show`, `bug update`, and `comment add`.
- Use Go `os/exec` to invoke the local agent command and test command.

The command owns the orchestration only:

1. Subscribe to the SSE endpoint.
2. Parse and filter bug events.
3. Fetch current bug detail from TAPD.
4. Guard the local repo state.
5. Move the bug to a start status when configured.
6. Invoke the coding agent with a generated prompt.
7. Run the verification command.
8. Write a result comment.
9. Move the bug to a success status when configured.

## Event Selection

The command processes only bug create and bug update events. It accepts both observed naming styles:

- `bug::create`
- `bug::update`
- `bug_create`
- `bug_update`

The command extracts:

- `workspace_id` from `event.workspace_id`
- bug ID from the first non-empty candidate path:
  - `event.bug.id`
  - `event.object.id`
  - `event.id`
  - `event.data.bug.id`
  - `event.data.id`

If the event has no usable workspace ID or bug ID, the command logs the skip to stderr and does not update `last_event_id` until the event has been safely ignored. Skipped malformed events do not write TAPD comments because there is no reliable target.

Workspace filtering uses the explicit `--workspace-id` value when provided. If no flag is provided, the command falls back to the configured default workspace. Events for other workspaces are skipped.

## Local Execution Safety

The command processes one bug at a time. A single process-level mutex prevents multiple agent runs from modifying the same repository concurrently.

Before invoking the agent, the command runs:

```bash
git status --porcelain
```

inside `--repo`.

If the repository is dirty, the command does not invoke the agent. It writes a TAPD comment explaining that automated fixing was skipped because the working tree contains uncommitted changes. It does not change the bug status unless `--on-failure-status` is configured.

If `--allow-dirty` is passed, this guard is disabled. The default is conservative because the command is allowed to edit code.

The command never runs:

- `git commit`
- `git push`
- `git reset --hard`
- `git checkout --`
- merge or deployment commands

The generated prompt tells the coding agent not to commit, push, create merge requests, deploy, or merge.

## Agent Prompt

The command sends a generated prompt to the configured `--agent-cmd` through stdin.

The prompt contains:

- TAPD workspace ID
- TAPD bug ID
- bug title
- bug status
- current owner
- severity
- priority
- Markdown description
- recent comments when available
- repository path
- verification command
- explicit constraints

The constraints are:

- Modify only code needed for this bug.
- Preserve unrelated user changes.
- Do not commit, push, create merge requests, deploy, or merge.
- Run the requested verification command when possible.
- Return a concise summary of files changed and verification results.

The command captures stdout and stderr from the agent process. To keep TAPD comments readable, stored output is truncated to a configurable byte limit, defaulting to 12 KiB.

## Verification

After the agent command exits successfully, the worker runs `--test-cmd` in `--repo` when configured.

If `--test-cmd` is empty, the command treats agent success as an unverified success and writes that explicitly in the TAPD comment. This is allowed for early adoption but not recommended.

If the test command fails, the command writes a failure comment and does not move the bug to `--on-success-status`.

## TAPD Writeback

The command writes comments using `comment add` semantics with:

- `entry_type=bug`
- `entry_id=<bug_id>`
- `workspace_id=<workspace_id>`

On start, when `--on-start-status` is non-empty, the command updates the bug with:

- `v_status=<on-start-status>`
- `current_user=<configured or authenticated user>`

On success, when `--on-success-status` is non-empty, the command updates the bug with:

- `v_status=<on-success-status>`
- `resolution=<configured resolution or "fixed">`
- `current_user=<configured or authenticated user>`

On failure, when `--on-failure-status` is non-empty, the command updates the bug with:

- `v_status=<on-failure-status>`
- `current_user=<configured or authenticated user>`

If a status update fails but the comment succeeds, the command logs the status update failure and continues. The TAPD comment is the source of truth for what happened.

## Idempotency

The command reuses the `last_event_id` approach from `tapd watch`. A bug event is considered consumed only after the command finishes handling it, including TAPD comment writeback attempts.

The command also keeps a small local processed-run file keyed by:

```text
<workspace_id>:<bug_id>:<event_id>
```

This prevents duplicate local work if the process crashes after agent completion but before `last_event_id` is persisted.

The command does not suppress later events for the same bug with a different event ID. A later bug update might contain new information and should be eligible for processing.

## Logging

Human-readable progress logs go to stderr:

- connection and reconnection status
- skipped events and reasons
- selected bug ID
- start status update result
- agent command exit status
- verification result
- TAPD comment and status update result

Machine-readable result records go to stdout as one JSON line per handled bug:

```json
{"workspace_id":"123","bug_id":"456","event_id":12,"status":"success","verified":true}
```

Failure records use `status="failed"` and include a `stage` field such as `dirty_repo`, `agent`, `test`, `comment`, or `status_update`.

## Configuration

Required:

- `--repo`
- SSE endpoint and token from flags, environment, or `.tapd.json`
- TAPD API credentials from the existing config mechanism

Optional:

- `--workspace-id`
- `--agent-cmd`, default `codex exec --full-auto`
- `--test-cmd`, default empty
- `--on-start-status`, default empty
- `--on-success-status`, default empty
- `--on-failure-status`, default empty
- `--current-user`, default authenticated TAPD user
- `--resolution`, default `fixed`
- `--allow-dirty`, default false
- `--once`, default false
- `--output-limit`, default 12288 bytes

## Error Handling

The command should not exit on a single bug failure. It should write or attempt to write a TAPD failure comment, emit a JSON failure record, then continue listening.

The command exits only when:

- the user sends SIGINT or SIGTERM
- initial configuration is invalid
- `--once` is set and one event has been handled

Network and SSE failures reconnect with the same exponential backoff behavior as `tapd watch`.

## Testing

Unit tests should cover:

- bug event type matching
- bug ID extraction from all supported payload shapes
- workspace filtering
- dirty repository guard
- prompt generation
- command result classification
- success writeback order
- failure writeback behavior
- `--once` handling

Command tests should use fake TAPD SDK clients and fake command runners rather than invoking real agents.

The existing `go test ./...` suite must pass after implementation.

## Rollout

Initial usage should run without status transitions:

```bash
tapd agent fix-bugs \
  --repo /Users/sunruoyu/go/src/vas/app/upower \
  --test-cmd "go test ./..." \
  --on-start-status "" \
  --on-success-status "" \
  --once
```

After comments and local edits look correct, enable `--on-start-status`. Enable `--on-success-status` last, after the TAPD workflow status value has been confirmed with `tapd workflow status-map --system bug --workitem-type-id <id>`.
