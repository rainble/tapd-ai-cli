# GitLab Issue Sync Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build GitLab issue creation plus TAPD story/bug to GitLab issue synchronization, including watcher-driven idempotent sync.

**Architecture:** Add a small internal GitLab REST client, extend config loading with GitLab fields, and add a `gitlab issue` Cobra command group. TAPD-to-GitLab conversion and sync marker logic live in focused command helpers so manual commands and watcher reuse the same code.

**Tech Stack:** Go standard library `net/http`, existing Cobra command patterns, existing TAPD SDK client, existing `tapd watch` SSE parsing helpers, existing Markdown conversion helpers.

---

### Task 1: GitLab Client

**Files:**
- Create: `internal/gitlab/client.go`
- Create: `internal/gitlab/client_test.go`

- [ ] **Step 1: Write failing tests**

Create tests for `CreateIssue`, `CreateIssueNote`, project path escaping, `PRIVATE-TOKEN`, and non-2xx errors using `httptest.Server`.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/gitlab`

Expected: fail because `internal/gitlab` does not exist.

- [ ] **Step 3: Implement client**

Add request/response structs and a `Client` with `CreateIssue` and `CreateIssueNote`.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/gitlab`

Expected: pass.

### Task 2: Config And Root Command Gating

**Files:**
- Modify: `internal/config/config.go`
- Modify: `internal/config/config_test.go`
- Modify: `internal/cmd/root.go`

- [ ] **Step 1: Write failing config tests**

Cover `GITLAB_BASE_URL`, `GITLAB_TOKEN`, `GITLAB_PROJECT`, and local `.tapd.json` loading.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/config`

Expected: fail because GitLab fields are missing.

- [ ] **Step 3: Implement config fields**

Add GitLab fields and env/file merge behavior.

- [ ] **Step 4: Update root gating**

Allow `tapd gitlab issue create` to run without TAPD credentials or workspace. Keep TAPD initialization for `create-from-story`, `create-from-bug`, and `sync-watch`.

- [ ] **Step 5: Run tests**

Run: `go test ./internal/config ./internal/cmd`

Expected: pass.

### Task 3: Manual GitLab Issue Command

**Files:**
- Create: `internal/cmd/gitlab.go`
- Create: `internal/cmd/gitlab_test.go`

- [ ] **Step 1: Write failing command tests**

Cover flag/env/config project resolution, missing token, `--description`, `--file`, optional fields, and compact JSON output.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/cmd -run TestGitLab`

Expected: fail because command/helpers are missing.

- [ ] **Step 3: Implement command**

Add `tapd gitlab issue create` and shared GitLab option resolution.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/cmd -run TestGitLab`

Expected: pass.

### Task 4: TAPD Conversion And Comment-Back

**Files:**
- Modify: `internal/cmd/gitlab.go`
- Modify: `internal/cmd/gitlab_test.go`

- [ ] **Step 1: Write failing conversion tests**

Cover story/bug title and Markdown description rendering, readiness checks, sync marker parsing, fingerprint changes, and comment-back body.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/cmd -run TestGitLab`

Expected: fail because conversion/sync helpers are missing.

- [ ] **Step 3: Implement conversion helpers**

Add `buildIssueFromStory`, `buildIssueFromBug`, `isReady`, `fingerprint`, marker parse/render, and comment-back helpers.

- [ ] **Step 4: Implement create-from commands**

Add `create-from-story` and `create-from-bug`.

- [ ] **Step 5: Run test to verify it passes**

Run: `go test ./internal/cmd -run TestGitLab`

Expected: pass.

### Task 5: Watch Sync

**Files:**
- Modify: `internal/cmd/gitlab.go`
- Modify: `internal/cmd/gitlab_test.go`

- [ ] **Step 1: Write failing sync tests**

Cover create/update event selection, skip when not ready, create when no marker, append note when marker fingerprint changes, and skip when unchanged.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/cmd -run TestGitLab`

Expected: fail because sync watcher helpers are missing.

- [ ] **Step 3: Implement sync helpers and command**

Add `sync-watch`, event parsing, event filters, detail loading, comment marker lookup, GitLab create/note calls, and mandatory TAPD marker comments.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/cmd -run TestGitLab`

Expected: pass.

### Task 6: Full Verification

**Files:**
- Modify as needed from previous tasks

- [ ] **Step 1: Format**

Run: `gofmt -w internal/gitlab internal/cmd/gitlab.go internal/cmd/gitlab_test.go internal/config/config.go internal/config/config_test.go internal/cmd/root.go`

- [ ] **Step 2: Focused tests**

Run: `go test ./internal/gitlab ./internal/config ./internal/cmd -run 'TestGitLab|TestLoadConfig'`

- [ ] **Step 3: Full tests**

Run: `go test ./...`

- [ ] **Step 4: Commit**

Commit implementation after all tests pass.
