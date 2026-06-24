# GitLab Issue Sync Design

## Goal

Add GitLab issue creation and TAPD-to-GitLab synchronization to `tapd-ai-cli`.
The CLI should support manual GitLab issue creation, manual creation from a TAPD story or bug, and an eventual watcher that creates or updates GitLab issues when TAPD stories or bugs become ready.

## Scope

The first implementation should include:

- `tapd gitlab issue create`
- `tapd gitlab issue create-from-story <story_id_or_url>`
- `tapd gitlab issue create-from-bug <bug_id_or_url>`
- `tapd gitlab issue sync-watch`
- Optional TAPD comment-back for manual create-from commands with `--comment-back`
- Mandatory TAPD sync marker comments for `sync-watch`

The feature should support self-hosted GitLab instances. For the current expected environment:

- GitLab base URL: `https://git.bilibili.co`
- GitLab project path: `go-vas/vas`
- API project identifier: URL-encoded project path, `go-vas%2Fvas`

Out of scope for the first implementation:

- Updating GitLab issue title or description after initial creation
- Closing GitLab issues from TAPD state
- GitLab assignee auto-mapping from TAPD users
- Deleting GitLab issues if TAPD comment-back fails
- Bidirectional GitLab-to-TAPD synchronization

## Configuration

Extend `.tapd.json` with GitLab-specific fields:

```json
{
  "gitlab_base_url": "https://git.bilibili.co",
  "gitlab_token": "...",
  "gitlab_project": "go-vas/vas"
}
```

Environment variables override file config:

- `GITLAB_BASE_URL`
- `GITLAB_TOKEN`
- `GITLAB_PROJECT`

Command flags override environment and file config:

- `--gitlab-base-url`
- `--gitlab-token`
- `--project`

`gitlab_base_url` defaults to `https://gitlab.com` when omitted. `gitlab_project` has no default unless configured.

## Commands

### Manual GitLab Issue Creation

```bash
tapd gitlab issue create \
  --project go-vas/vas \
  --title "Issue title" \
  --description "Issue description"
```

Description input follows the existing TAPD rich-text input pattern:

- `--description`
- `--file`
- stdin when piped

Supported optional GitLab fields:

- `--labels`, comma-separated
- `--assignee-ids`, comma-separated GitLab numeric user IDs
- `--due-date`, `YYYY-MM-DD`
- `--confidential`
- `--issue-type`

This command only needs GitLab credentials. It must not require TAPD credentials or a TAPD workspace ID.

### Create From TAPD Story

```bash
tapd gitlab issue create-from-story <story_id_or_url> \
  --project go-vas/vas \
  --comment-back
```

The command loads the TAPD story, converts the TAPD HTML description to Markdown, creates a GitLab issue, and optionally writes the GitLab issue URL back to TAPD as a comment.

GitLab title:

```text
[TAPD Story] <story name>
```

GitLab description includes:

- TAPD story URL
- Story ID
- Status
- Priority
- Owner
- Developer
- Iteration ID
- Story description in Markdown

### Create From TAPD Bug

```bash
tapd gitlab issue create-from-bug <bug_id_or_url> \
  --project go-vas/vas \
  --comment-back
```

The command loads the TAPD bug, converts the TAPD HTML description to Markdown, creates a GitLab issue, and optionally writes the GitLab issue URL back to TAPD as a comment.

GitLab title:

```text
[TAPD Bug] <bug title>
```

GitLab description includes:

- TAPD bug URL
- Bug ID
- Status
- Priority
- Severity
- Current owner
- Module
- Iteration ID
- Bug description in Markdown

### Watch And Sync

```bash
tapd gitlab issue sync-watch \
  --project go-vas/vas \
  --types story,bug
```

`sync-watch` reuses the existing TAPD SSE configuration used by `tapd watch`:

- `--endpoint`
- `--token`
- `TAPD_WATCH_ENDPOINT`
- `TAPD_SUBSCRIBE_TOKEN`
- `watch_endpoint`
- `subscribe_token`

It listens for TAPD story and bug create/update events. It does not create GitLab issues immediately on every create event. Instead, every event triggers a fresh TAPD detail load, readiness check, idempotency lookup, and fingerprint comparison.

## Sync State Machine

```text
Receive TAPD story/bug create or update event
  -> Load current TAPD detail
  -> Check whether content is ready
  -> Read TAPD comments and search for sync marker
  -> If no marker exists, create GitLab issue
  -> If marker exists and fingerprint changed, append GitLab issue note
  -> If marker exists and fingerprint is unchanged, skip
  -> Write TAPD sync marker comment
```

## Readiness Check

A TAPD story or bug is ready for GitLab sync when:

- Title is non-empty after trimming whitespace
- Markdown description is non-empty after converting from HTML
- Markdown description is not a known empty placeholder

Known empty placeholders include:

- Empty string
- `<p><br></p>` after HTML normalization
- Markdown that becomes empty after trimming whitespace

This prevents empty TAPD shells from creating low-quality GitLab issues. A later update event can create the GitLab issue once the description is filled in.

## Idempotency

Use TAPD comments as the shared synchronization record. This avoids local state drift when multiple machines run `sync-watch`.

After a GitLab issue is created by `sync-watch`, write a TAPD comment containing a human-readable line and a machine-readable marker:

```md
已同步 GitLab Issue: https://git.bilibili.co/go-vas/vas/-/issues/123

<!-- tapd-gitlab-sync {"type":"story","id":"1151081496001028684","project":"go-vas/vas","issue_iid":123,"fingerprint":"<sha256>"} -->
```

The marker fields are:

- `type`: `story` or `bug`
- `id`: TAPD long ID
- `project`: GitLab project path
- `issue_iid`: GitLab project-local issue IID
- `fingerprint`: SHA-256 fingerprint of the synced TAPD content

On later events, the sync code reads TAPD comments and searches for this marker. If found, it uses `issue_iid` to append GitLab notes instead of creating another issue.

For manual `create-from-story` and `create-from-bug`, the marker is only written when `--comment-back` is set. Manual commands without `--comment-back` intentionally behave as direct creation commands and do not provide deduplication across repeated invocations.

## Fingerprint

Compute fingerprint from normalized content that matters to the GitLab issue:

```text
sha256(type + "\n" + id + "\n" + title + "\n" + markdown_description + "\n" + status + "\n" + priority + "\n" + owner_fields)
```

For stories, `owner_fields` includes owner and developer.

For bugs, `owner_fields` includes current owner, severity, and module.

If the fingerprint is unchanged, `sync-watch` skips the event.

If the fingerprint changed, `sync-watch` posts a GitLab issue note with the current TAPD snapshot. The first implementation should not edit the existing GitLab issue description; appending notes preserves history and avoids destructive updates.

## GitLab API

Use the GitLab REST API with the standard library `net/http`. Do not introduce a GitLab SDK dependency for the first implementation.

Authentication:

```text
PRIVATE-TOKEN: <gitlab_token>
```

Create issue:

```text
POST /api/v4/projects/:project/issues
```

Append issue note:

```text
POST /api/v4/projects/:project/issues/:issue_iid/notes
```

`project` may be a numeric project ID or URL-encoded project path. For `go-vas/vas`, use `go-vas%2Fvas`.

## Output

Successful issue creation prints compact JSON:

```json
{"success":true,"id":123,"iid":45,"url":"https://git.bilibili.co/go-vas/vas/-/issues/45","project":"go-vas/vas"}
```

If GitLab issue creation succeeds but TAPD comment-back fails:

- stdout still prints the GitLab issue JSON
- stderr prints a structured `comment_back_failed` error
- exit code is non-zero
- the GitLab issue is not deleted

## Error Handling

Missing GitLab token:

- Error code: `gitlab_auth_required`
- Exit code: auth error

Missing GitLab project:

- Error code: `missing_parameter`
- Exit code: parameter error

TAPD detail load failure:

- Error code: `api_error`
- Exit code: API error

GitLab API non-2xx response:

- Error code: `gitlab_api_error`
- Include HTTP status and a bounded response body snippet
- Exit code: API error

Not ready for sync:

- Manual `create-from-story` and `create-from-bug`: return parameter error with a clear message
- `sync-watch`: skip and log a concise message to stderr

## Testing

Add focused tests with `httptest.Server` for GitLab API behavior:

- Creates issue with URL-encoded project path
- Sends `PRIVATE-TOKEN`
- Sends title, description, labels, assignee IDs, due date, confidential, and issue type
- Parses GitLab response into compact success output
- Appends issue note to existing issue IID
- Handles non-2xx GitLab responses

Add command tests:

- `gitlab issue create` does not require TAPD workspace or TAPD credentials
- Missing GitLab token fails
- Missing project fails when no config or env is present
- Project is read from flag, env, or config
- Description is read from flag, file, or stdin
- `create-from-story` builds issue content from TAPD story data
- `create-from-bug` builds issue content from TAPD bug data
- `--comment-back` writes TAPD comments with the sync marker
- Existing marker causes GitLab note append instead of issue creation
- Unchanged fingerprint skips GitLab calls in `sync-watch`

Run:

```bash
go test ./internal/cmd ./internal/gitlab
go test ./...
```
