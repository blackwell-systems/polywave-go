# API Endpoints

> **Note:** The HTTP API is implemented in [scout-and-wave-web](https://github.com/blackwell-systems/scout-and-wave-web), which imports the engine (`scout-and-wave-go`) as a library. This document describes the full API surface.

## Base URL

```
http://localhost:7432/api
```

## Authentication

**Current:** None. The API is intended for localhost development only.

**Future:** API key or OAuth for remote deployments (see `scout-and-wave-web` roadmap).

## Error Responses

All endpoints return standard error JSON on failure:

```json
{
  "error": "descriptive error message"
}
```

Common HTTP status codes:
- `400` — Bad request (invalid input, validation failure)
- `404` — Not found (IMPL doc, wave, agent, program)
- `409` — Conflict (concurrent operation already in progress)
- `500` — Internal server error (engine crash, git failure)

---

## SSE Streams

### `GET /api/events`

Global SSE stream for server-wide state changes (IMPL list updates, config changes, program updates, pipeline changes).

**Response:** `text/event-stream`

**Events:**
- `connected` — Initial heartbeat on connection (`data: {}`)
- `impl_list_updated` — IMPL directory changed (file created/renamed/removed, execution started/stopped)
- `program_list_updated` — Program manifest created or updated (`data: {"slug":"..."}`)
- `pipeline_updated` — Pipeline state changed
- `critic_review_complete` — Critic review written (`data: {"slug":"...","result":{...}}`)
- `program_*` — All program lifecycle events are also broadcast here

**Keepalive:** `: ping` comment every 30 seconds.

---

### `GET /api/wave/{slug}/events`

Per-IMPL SSE stream for wave execution, merge, test, scaffold, and agent lifecycle events.

**Path params:**
- `slug` (string) — IMPL slug

**Response:** `text/event-stream`

**Events (partial list):**
- `agent_started`, `agent_complete`, `agent_failed` — Agent lifecycle
- `agent_tool_call` — Per-tool invocation/result
- `agent_progress` — Parsed progress updates
- `wave_gate_pending`, `wave_complete`, `run_complete` — Wave lifecycle
- `scaffold_started`, `scaffold_output`, `scaffold_complete`, `scaffold_failed`, `scaffold_cancelled`
- `merge_started`, `merge_output`, `merge_complete`, `merge_failed`
- `test_started`, `test_output`, `test_complete`, `test_failed`
- `fix_build_started`, `fix_build_output`, `fix_build_tool_call`, `fix_build_complete`, `fix_build_failed`
- `conflict_resolving`, `conflict_resolved`, `conflict_resolution_failed`
- `step_retry_started`, `step_retry_complete`, `step_retry_failed`
- `program_tier_started`, `program_tier_complete`, `program_blocked`, `program_tier_failed`

**Replay:** On connect, cached agent lifecycle events are replayed so late-connecting clients see current state.

**Keepalive:** `: ping` every 30 seconds.

---

### `GET /api/scout/{runID}/events`

SSE stream for a Scout agent run (also used by Bootstrap runs).

**Path params:**
- `runID` (string) — Run ID returned by `POST /api/scout/run` or `POST /api/bootstrap/run`

**Events:**
- `scout_output` — `{"run_id":"...","chunk":"..."}`
- `scout_finalize` — `{"run_id":"...","status":"running|warning|complete",...}`
- `scout_complete` — `{"run_id":"...","slug":"...","impl_path":"..."}`
- `scout_failed` — `{"run_id":"...","error":"..."}`
- `scout_cancelled` — `{"run_id":"..."}`

---

### `GET /api/impl/{slug}/chat/{runID}/events`

SSE stream for an IMPL chat session.

**Path params:**
- `slug` (string), `runID` (string)

**Events:**
- `chat_output` — `{"run_id":"...","chunk":"..."}`
- `chat_complete` — `{"run_id":"...","slug":"..."}`
- `chat_failed` — `{"run_id":"...","error":"..."}`

---

### `GET /api/impl/{slug}/revise/{runID}/events`

SSE stream for an IMPL revision session.

**Path params:**
- `slug` (string), `runID` (string)

**Events:**
- `revise_output` — `{"run_id":"...","chunk":"..."}`
- `revise_complete` — `{"run_id":"...","slug":"..."}`
- `revise_failed` — `{"run_id":"...","error":"..."}`
- `revise_cancelled` — `{"run_id":"..."}`

---

### `GET /api/planner/{runID}/events`

SSE stream for a Planner agent run.

**Path params:**
- `runID` (string)

**Events:**
- `planner_output` — `{"run_id":"...","chunk":"..."}`
- `planner_complete` — `{"run_id":"...","slug":"...","program_path":"..."}`
- `planner_failed` — `{"run_id":"...","error":"..."}`
- `planner_cancelled` — `{"run_id":"..."}`

---

### `GET /api/program/events`

SSE stream filtered to program lifecycle events only (`program_*`).

**Events:**
- `connected` — Initial heartbeat
- `program_tier_started`, `program_tier_complete`, `program_tier_failed`
- `program_impl_started`, `program_impl_complete`
- `program_contract_frozen`
- `program_complete`, `program_blocked`
- `program_replan_complete`, `program_replan_failed`

---

### `GET /api/daemon/events`

SSE stream for daemon lifecycle events.

**Events:**
- `connected` — Initial heartbeat
- `daemon_event` — JSON-encoded `engine.Event` payload
- `daemon_stopped` — `{"running":false}`

**Replay:** Recent events (up to 100) are replayed on connect.

---

## IMPL Management

### `GET /api/impl`

List all IMPL documents across all configured repos.

**Response:** `200 OK`
```json
[
  {
    "slug": "add-user-auth",
    "repo": "my-project",
    "repo_path": "/abs/path/to/my-project",
    "doc_status": "active",
    "wave_count": 2,
    "agent_count": 5,
    "is_multi_repo": false,
    "involved_repos": [],
    "is_executing": false
  }
]
```

Scans `docs/IMPL/` and `docs/IMPL/complete/` in every repo from `saw.config.json`.

---

### `GET /api/impl/{slug}`

Get a parsed IMPL document with full detail.

**Path params:**
- `slug` (string)

**Response:** `200 OK` — `IMPLDocResponse`
```json
{
  "slug": "add-user-auth",
  "repo": "my-project",
  "repo_path": "/abs/path",
  "doc_status": "active",
  "completed_at": "",
  "suitability": { "verdict": "suitable", "rationale": "..." },
  "file_ownership": [{ "file": "src/auth.go", "agent": "A", "wave": 1, "action": "new", "depends_on": "", "repo": "" }],
  "waves": [{ "number": 1, "agents": ["A","B"], "dependencies": [], "status": "pending" }],
  "scaffold": { "required": true, "committed": false, "files": ["types.go"], "contracts": [] },
  "pre_mortem": { "overall_risk": "medium", "rows": [...] },
  "reactions": null,
  "known_issues": [],
  "scaffolds_detail": [{ "file_path": "types.go", "contents": "...", "import_path": "" }],
  "interface_contracts_text": "...",
  "dependency_graph_text": "...",
  "agent_prompts": [{ "wave": 1, "agent": "A", "prompt": "..." }],
  "quality_gates": { "level": "standard", "gates": [...] },
  "post_merge_checklist": { "groups": [...] },
  "stub_report_text": "",
  "known_issues_structured": [],
  "wiring": [{ "symbol": "...", "defined_in": "...", "must_be_called_from": "...", "agent": "A", "wave": 1, "status": "declared" }]
}
```

**Errors:** `404` if not found, `500` on parse error.

---

### `GET /api/impl/{slug}/raw`

Get the raw IMPL document (YAML) as plain text.

**Response:** `200 OK` — `text/plain; charset=utf-8` (raw YAML content)

**Errors:** `404` if not found.

---

### `PUT /api/impl/{slug}/raw`

Replace the IMPL document content (atomic write via temp file + rename).

**Request:** Raw body (`text/plain` or `application/octet-stream`), max 10 MB.

**Response:** `200 OK` (empty body)

**Errors:** `400` if body is empty.

---

### `DELETE /api/impl/{slug}`

Delete an IMPL document from disk.

**Response:** `204 No Content`

**Errors:** `404` if not found.

---

### `POST /api/impl/{slug}/archive`

Move an IMPL from `docs/IMPL/` to `docs/IMPL/complete/`.

**Response:** `200 OK` (empty body)

**Errors:** `404` if not found in active directory.

---

### `POST /api/impl/{slug}/approve`

Approve an IMPL plan (publishes `plan_approved` SSE event). May auto-trigger critic gate (E37) if threshold is met.

**Response:** `202 Accepted`

---

### `POST /api/impl/{slug}/reject`

Reject an IMPL plan (publishes `plan_rejected` SSE event).

**Response:** `202 Accepted`

---

### `POST /api/impl/{slug}/amend`

Amend an IMPL document (add wave, redirect agent, or extend scope).

**Request:**
```json
{
  "operation": "add-wave",
  "agent_id": "",
  "wave_num": 0,
  "new_task": ""
}
```
- `operation` (string, required) — `"add-wave"` | `"redirect-agent"` | `"extend-scope"`
- `agent_id` (string) — Required for `redirect-agent`
- `wave_num` (int) — Required for `redirect-agent`
- `new_task` (string) — Required for `redirect-agent`

**Response:** `200 OK`
```json
{
  "success": true,
  "operation": "add-wave",
  "new_wave_number": 3,
  "agent_id": "",
  "warnings": [],
  "error": ""
}
```

**Errors:** `400` invalid operation, `404` IMPL not found, `409` amend blocked (active execution), `500` system error.

---

### `POST /api/impl/{slug}/revise`

Launch a Claude agent to revise the IMPL doc based on feedback.

**Request:**
```json
{ "feedback": "Add error handling to the auth flow" }
```

**Response:** `202 Accepted`
```json
{ "run_id": "1234567890" }
```

Subscribe to `GET /api/impl/{slug}/revise/{runID}/events` for progress.

---

### `POST /api/impl/{slug}/revise/{runID}/cancel`

Cancel a running IMPL revision.

**Response:** `204 No Content`

---

### `GET /api/impl/{slug}/diff/{agent}`

Get a unified diff of an agent's branch changes for a specific file.

**Query params:**
- `wave` (int, default 1) — Wave number
- `file` (string, required, URL-encoded) — Relative file path

**Response:** `200 OK`
```json
{
  "agent": "A",
  "file": "src/auth.go",
  "branch": "saw/add-user-auth/wave1-agent-A",
  "diff": "--- a/src/auth.go\n+++ b/src/auth.go\n..."
}
```

---

### `GET /api/impl/{slug}/agent/{letter}/context`

Get the agent's context payload (task prompt, file ownership, contracts).

**Path params:**
- `slug` (string), `letter` (string, e.g. `"A"`)

**Response:** `200 OK`
```json
{
  "slug": "add-user-auth",
  "agent": "A",
  "wave": 1,
  "context_text": "Implement the auth middleware..."
}
```

---

### `GET /api/impl/{slug}/critic-review`

Get the critic review result for an IMPL.

**Response:** `200 OK` — `protocol.CriticResult` JSON

**Errors:** `404` if no IMPL found or no critic review exists, `500` on parse error.

---

### `POST /api/impl/{slug}/run-critic`

Trigger an async critic review via `sawtools run-critic`.

**Response:** `202 Accepted`

Publishes `critic_review_complete` via global SSE when done.

---

## Scout

### `POST /api/scout/run`

Launch a Scout agent to generate an IMPL document.

**Request:**
```json
{
  "feature": "Add user authentication to the API",
  "repo": ""
}
```
- `feature` (string, required) — Feature description
- `repo` (string, optional) — Override repo path

**Response:** `202 Accepted`
```json
{ "run_id": "1234567890" }
```

Subscribe to `GET /api/scout/{runID}/events` for progress.

---

### `POST /api/scout/{slug}/rerun`

Re-run Scout for an existing IMPL slug, reusing the feature title from the manifest.

**Response:** `202 Accepted`
```json
{ "run_id": "1234567890" }
```

---

### `POST /api/scout/{runID}/cancel`

Cancel a running Scout agent.

**Response:** `204 No Content`

---

### `POST /api/bootstrap/run`

Launch a bootstrap Scout agent for greenfield project initialization.

**Request:**
```json
{
  "description": "Build a REST API with Go and React",
  "repo": ""
}
```

**Response:** `202 Accepted`
```json
{ "run_id": "1234567890" }
```

Events published under `scout-{runID}` — subscribe via `GET /api/scout/{runID}/events`.

---

## Wave Execution & Recovery

### `POST /api/wave/{slug}/start`

Start wave execution (launches all agents across all waves sequentially, with gates between waves).

**Response:** `202 Accepted`

**Errors:** `409` if already running, `404` if IMPL not found.

---

### `POST /api/wave/{slug}/gate/proceed`

Signal the inter-wave gate to proceed to the next wave.

**Response:** `202 Accepted`

**Errors:** `404` if no gate is pending for this slug.

---

### `POST /api/wave/{slug}/agent/{letter}/rerun`

Re-run a single agent within a wave.

**Request:**
```json
{
  "wave": 1,
  "scope_hint": "Focus on fixing the auth middleware"
}
```
- `wave` (int, required, >= 1)
- `scope_hint` (string, optional) — Additional guidance for the retry

**Response:** `202 Accepted`

**Errors:** `400` invalid body or wave, `404` IMPL not found.

---

### `POST /api/wave/{slug}/merge`

Merge a wave's completed agent branches to main.

**Request:**
```json
{ "wave": 1 }
```

**Response:** `202 Accepted`

**SSE events:** `merge_started`, `merge_output`, `merge_complete` or `merge_failed` (with `conflicting_files`).

**Errors:** `400` invalid body, `404` IMPL not found, `409` merge already in progress.

---

### `POST /api/wave/{slug}/merge-abort`

Abort a conflicted merge state (`git merge --abort`).

**Response:** `200 OK` — `"merge aborted successfully"`

**Errors:** `404` IMPL not found, `500` if git merge --abort fails.

---

### `POST /api/wave/{slug}/resolve-conflicts`

Use AI to resolve merge conflicts.

**Request:**
```json
{ "wave": 1 }
```

**Response:** `202 Accepted`

**SSE events:** `conflict_resolving`, `conflict_resolved`, `conflict_resolution_failed`, `merge_complete`.

**Errors:** `409` merge/resolution already in progress.

---

### `POST /api/wave/{slug}/fix-build`

Use AI to diagnose and fix a build/test/gate failure after merge.

**Request:**
```json
{
  "wave": 1,
  "error_log": "error: undefined reference to `authenticate`",
  "gate_type": "build"
}
```
- `wave` (int, required, >= 1)
- `error_log` (string, required)
- `gate_type` (string, optional) — e.g. `"build"`, `"test"`, `"lint"`

**Response:** `202 Accepted`

**SSE events:** `fix_build_started`, `fix_build_output`, `fix_build_tool_call`, `fix_build_complete` or `fix_build_failed`.

---

### `POST /api/wave/{slug}/test`

Run the IMPL's `test_command` and stream output.

**Request:**
```json
{ "wave": 1 }
```

**Response:** `202 Accepted`

**SSE events:** `test_started`, `test_output`, `test_complete` (status=pass) or `test_failed` (status=fail, output=...).

**Errors:** `409` test already in progress.

---

### `POST /api/wave/{slug}/finalize`

Run the full finalization pipeline for a wave (verify commits, scan stubs, run gates, validate integration, merge agents, fix go.mod, verify build, integration agent, cleanup).

**Request:**
```json
{ "wave": 1 }
```

**Response:** `202 Accepted`

**Errors:** `400` invalid body, `404` IMPL not found, `409` merge/finalize already in progress.

---

### `POST /api/wave/{slug}/step/{step}/retry`

Retry a specific finalization pipeline step independently.

**Path params:**
- `step` — One of: `verify_commits`, `scan_stubs`, `run_gates`, `validate_integration`, `merge_agents`, `fix_gomod`, `verify_build`, `integration_agent`, `cleanup`

**Request:**
```json
{ "wave": 1 }
```

**Response:** `202 Accepted`

**SSE events:** `step_retry_started`, `step_retry_complete` or `step_retry_failed`.

**Errors:** `400` unknown step or wave < 1, `409` concurrent operation.

---

### `POST /api/wave/{slug}/step/{step}/skip`

Skip a non-required finalization step.

**Request:**
```json
{
  "wave": 1,
  "reason": "not applicable"
}
```

**Response:** `200 OK`
```json
{ "status": "skipped", "step": "scan_stubs", "reason": "not applicable" }
```

**Skippable steps:** `scan_stubs`, `validate_integration`, `integration_agent`, `cleanup`, `fix_gomod`.

**Errors:** `400` if step is not skippable.

---

### `POST /api/wave/{slug}/mark-complete`

Force-mark an IMPL as complete regardless of pipeline state. Clears stale pipeline state.

**Response:** `200 OK`
```json
{ "status": "complete" }
```

**Errors:** `404` IMPL not found, `500` on failure.

---

### `GET /api/wave/{slug}/state`

Get the stage state machine for an active wave execution.

**Response:** `200 OK` — Array of stage entries with status, wave number, timestamps.

**Errors:** `404` if no state exists for this slug.

---

### `GET /api/wave/{slug}/status`

Get real-time progress for all agents in the active wave (in-memory tracker).

**Response:** `200 OK`
```json
[
  {
    "agent": "A",
    "wave": 1,
    "current_file": "src/auth.go",
    "current_action": "writing",
    "percent_done": 45
  }
]
```

---

### `GET /api/wave/{slug}/disk-status`

Reconstruct wave/agent status from disk (survives server restarts).

**Response:** `200 OK`
```json
{
  "slug": "add-user-auth",
  "current_wave": 1,
  "total_waves": 2,
  "scaffold_status": "committed",
  "agents": [
    {
      "agent": "A",
      "wave": 1,
      "status": "complete",
      "branch": "saw/add-user-auth/wave1-agent-A",
      "commit": "abc1234",
      "files": ["src/auth.go"],
      "failure_type": "",
      "message": ""
    }
  ],
  "waves_merged": [1]
}
```

Agent `status` values: `"complete"`, `"partial"`, `"blocked"`, `"failed"`, `"pending"`.

---

### `GET /api/wave/{slug}/pipeline`

Get the finalization pipeline state for a slug.

**Response:** `200 OK` — Pipeline state JSON with per-step status.

**Errors:** `404` if no pipeline state, `503` if tracker not initialized.

---

### `GET /api/wave/{slug}/review/{wave}`

Get the cached code review result for a wave (polling fallback for SSE).

**Response:** `404` — Not yet implemented (review data delivered via SSE `code_review_result` events).

---

### `POST /api/wave/{slug}/resume`

Resume an interrupted wave execution. Detects which agents need re-launch.

**Response:** `202 Accepted`

**Errors:** `404` no interrupted session found, `409` already running or cross-repo conflict.

---

### `POST /api/impl/{slug}/scaffold/rerun`

Re-run the scaffold agent for an IMPL.

**Response:** `202 Accepted`
```json
{ "run_id": "1234567890" }
```

Events published to `GET /api/wave/{slug}/events` (scaffold_started, scaffold_output, etc.).

---

## Session Recovery

### `GET /api/sessions/interrupted`

Scan all configured repos for interrupted SAW sessions.

**Response:** `200 OK`
```json
[
  {
    "slug": "add-user-auth",
    "wave": 1,
    "agents": ["A", "B"],
    "repo_path": "/abs/path"
  }
]
```

---

## File Operations

### `GET /api/files/tree`

Get a recursive directory tree for a repository.

**Query params:**
- `repo` (string, required) — Repo name from `saw.config.json`
- `path` (string, optional) — Relative path within repo (default: repo root)

**Response:** `200 OK`
```json
{
  "repo": "my-project",
  "root": {
    "name": "my-project",
    "path": ".",
    "isDir": true,
    "children": [
      { "name": "main.go", "path": "main.go", "isDir": false, "gitStatus": "M" }
    ]
  }
}
```

Skips `.git`, `node_modules`, `dist`, `build`, `target`, `.next`, `.claude`, `.claire`.

---

### `GET /api/files/read`

Read a file's contents with language detection.

**Query params:**
- `repo` (string, required)
- `path` (string, required) — Relative file path

**Response:** `200 OK`
```json
{
  "repo": "my-project",
  "path": "src/main.go",
  "content": "package main\n...",
  "language": "go",
  "size": 1234
}
```

**Errors:** `400` path escapes repo root or is a directory, `404` not found, `413` file > 1 MB, `415` binary file.

---

### `GET /api/files/diff`

Get git diff (unstaged + staged) for a file.

**Query params:**
- `repo` (string, required)
- `path` (string, required)

**Response:** `200 OK`
```json
{
  "repo": "my-project",
  "path": "src/main.go",
  "diff": "--- a/src/main.go\n..."
}
```

---

### `GET /api/files/status`

Get git status for all files in a repository.

**Query params:**
- `repo` (string, required)

**Response:** `200 OK`
```json
{
  "repo": "my-project",
  "files": [
    { "path": "src/main.go", "status": "M" },
    { "path": "src/new.go", "status": "U" }
  ]
}
```

Status codes: `"M"` (modified), `"A"` (added), `"U"` (untracked), `"D"` (deleted).

---

## Chat & Revision

### `POST /api/impl/{slug}/chat`

Start a chat session with Claude using the IMPL doc as context.

**Request:**
```json
{
  "message": "How should I handle JWT refresh tokens?",
  "history": [
    { "role": "user", "content": "What auth strategy should I use?" },
    { "role": "assistant", "content": "I recommend JWT with..." }
  ]
}
```

**Response:** `202 Accepted`
```json
{ "run_id": "1234567890" }
```

Subscribe to `GET /api/impl/{slug}/chat/{runID}/events` for streaming response.

---

## Worktree Management

### `GET /api/impl/{slug}/worktrees`

List all SAW-managed git worktrees for an IMPL.

**Response:** `200 OK`
```json
{
  "worktrees": [
    {
      "branch": "saw/add-user-auth/wave1-agent-A",
      "path": "/abs/path/.claude/worktrees/...",
      "status": "merged",
      "has_unsaved": false,
      "last_commit_age": "2 hours ago"
    }
  ]
}
```

Status values: `"merged"`, `"unmerged"`, `"stale"` (unmerged + last commit > 24h old).

---

### `DELETE /api/impl/{slug}/worktrees/{branch}`

Delete a single worktree and its branch.

**Query params:**
- `force` (string, optional) — `"true"` to delete even if unmerged

**Response:** `204 No Content`

**Errors:** `409` if branch is unmerged and `force` is not set:
```json
{ "error": "branch is unmerged", "branch": "..." }
```

---

### `POST /api/impl/{slug}/worktrees/cleanup`

Batch delete worktrees and branches.

**Request:**
```json
{
  "branches": ["saw/slug/wave1-agent-A", "saw/slug/wave1-agent-B"],
  "force": false
}
```

**Response:** `200 OK`
```json
{
  "results": [
    { "branch": "saw/slug/wave1-agent-A", "deleted": true, "error": "" }
  ],
  "deleted_count": 2,
  "failed_count": 0
}
```

**Errors:** `409` if `force=false` and unmerged branches exist:
```json
{ "error": "unmerged branches exist", "unmerged": ["..."] }
```

---

## Manifest

### `GET /api/manifest/{slug}`

Load and return the parsed YAML manifest as JSON.

**Response:** `200 OK` — Full `protocol.IMPLManifest` as JSON.

**Errors:** `400` missing slug, `500` on parse error.

---

### `POST /api/manifest/{slug}/validate`

Validate an IMPL manifest and return structured errors.

**Response:** `200 OK`
```json
{
  "valid": true,
  "errors": []
}
```

Or with validation errors:
```json
{
  "valid": false,
  "errors": [
    { "field": "waves[0].agents[0].files", "message": "file not in file_ownership" }
  ]
}
```

---

### `GET /api/manifest/{slug}/wave/{number}`

Get a specific wave from the manifest.

**Path params:**
- `number` (int) — 1-based wave number

**Response:** `200 OK` — `protocol.Wave` as JSON.

**Errors:** `400` invalid number, `404` wave not found.

---

### `POST /api/manifest/{slug}/completion/{agentID}`

Set the completion report for an agent and save the manifest.

**Request:** `protocol.CompletionReport` JSON body.

**Response:** `200 OK`
```json
{ "status": "ok" }
```

---

## Configuration & System

### `GET /api/config`

Get the current SAW configuration.

**Response:** `200 OK`
```json
{
  "repos": [{ "name": "my-project", "path": "/abs/path" }],
  "agent": {
    "scout_model": "claude-sonnet-4-6",
    "wave_model": "claude-sonnet-4-6",
    "chat_model": "claude-sonnet-4-6",
    "scaffold_model": "",
    "integration_model": "",
    "planner_model": "",
    "review_model": ""
  },
  "quality": {
    "require_tests": false,
    "require_lint": false,
    "block_on_failure": false,
    "code_review": { "enabled": false, "blocking": false, "model": "", "threshold": 0 }
  },
  "appearance": { "theme": "system", "color_theme": "", "color_theme_dark": "", "color_theme_light": "" }
}
```

Falls back to defaults if `saw.config.json` does not exist.

---

### `POST /api/config`

Save SAW configuration (atomic write).

**Request:** `SAWConfig` JSON body (same shape as GET response).

**Response:** `200 OK` (empty body)

**Validation:** Model names validated with `^[a-zA-Z0-9:._/-]+$` (max 200 chars).

---

### `POST /api/config/validate-repo`

Validate that a path is an existing git repository with commits.

**Request:**
```json
{ "path": "/abs/path/to/repo" }
```

**Response:** `200 OK`
```json
{ "valid": true }
```

Or on failure:
```json
{
  "valid": false,
  "error": "Not a git repository (run `git init` first)",
  "error_code": "not_git"
}
```

Error codes: `"not_found"`, `"not_git"`, `"no_commits"`.

---

### `GET /api/browse`

Browse filesystem directories (server-side directory listing for repo picker).

**Query params:**
- `path` (string, optional) — Absolute directory path (default: `$HOME`)

**Response:** `200 OK`
```json
{
  "path": "/Users/me/code",
  "parent": "/Users/me",
  "entries": [
    { "name": "my-project", "is_dir": true }
  ]
}
```

Only returns directories; hides dotfiles.

---

### `GET /api/browse/native`

Open the OS-native folder picker dialog (macOS only via `osascript`).

**Query params:**
- `prompt` (string, optional) — Dialog prompt text

**Response:** `200 OK`
```json
{ "path": "/Users/me/code/my-project" }
```

**Errors:** `204 No Content` if user cancels, `501` on non-macOS platforms.

---

### `GET /api/context`

Get the contents of `docs/CONTEXT.md`.

**Response:** `200 OK` — `text/plain; charset=utf-8`

**Errors:** `404` if file does not exist.

---

### `PUT /api/context`

Save `docs/CONTEXT.md` (atomic write, max 10 MB body).

**Request:** Plain text body.

**Response:** `200 OK` (empty body)

---

### `GET /api/notifications/preferences`

Get notification preferences from `saw.config.json`.

**Response:** `200 OK`
```json
{
  "enabled": true,
  "muted_types": null,
  "browser_notify": true,
  "toast_notify": true
}
```

---

### `POST /api/notifications/preferences`

Save notification preferences (preserves all other config fields).

**Request:**
```json
{
  "enabled": true,
  "muted_types": ["run_failed"],
  "browser_notify": true,
  "toast_notify": false
}
```

**Response:** `200 OK` (empty body)

---

## Journal / Observatory

### `GET /api/journal/{wave}/{agent}`

Get full tool journal entries for an agent.

**Path params:**
- `wave` (string, e.g. `"wave1"`)
- `agent` (string, e.g. `"agent-A"`)

**Response:** `200 OK`
```json
{
  "entries": [
    { "tool": "Read", "input": "main.go", "output": "...", "timestamp": "..." }
  ]
}
```

**Errors:** `404` if journal not found.

---

### `GET /api/journal/{wave}/{agent}/summary`

Get context markdown generated from the tool journal.

**Response:** `200 OK`
```json
{ "markdown": "## Session Context...\n..." }
```

---

### `GET /api/journal/{wave}/{agent}/checkpoints`

List available journal checkpoints.

**Response:** `200 OK`
```json
{
  "checkpoints": [
    { "name": "pre-refactor", "timestamp": "..." }
  ]
}
```

---

### `POST /api/journal/{wave}/{agent}/restore`

Restore journal state to a named checkpoint.

**Request:**
```json
{ "checkpoint_name": "pre-refactor" }
```

**Response:** `200 OK`
```json
{ "status": "success", "message": "restored to checkpoint \"pre-refactor\"" }
```

**Errors:** `400` if checkpoint not found or name contains invalid characters.

---

## Planner

### `POST /api/planner/run`

Launch a Planner agent to produce a PROGRAM manifest.

**Request:**
```json
{
  "description": "Build a full-stack e-commerce platform",
  "repo": ""
}
```

**Response:** `202 Accepted`
```json
{ "run_id": "1234567890" }
```

---

### `POST /api/planner/{runID}/cancel`

Cancel a running Planner agent.

**Response:** `204 No Content`

---

## Program

### `GET /api/programs`

List all PROGRAM manifests across configured repos.

**Response:** `200 OK`
```json
{
  "programs": [
    { "slug": "ecommerce", "title": "E-commerce Platform", "path": "/abs/path/docs/PROGRAM-ecommerce.yaml", "tier_count": 3, "impl_count": 8 }
  ]
}
```

---

### `GET /api/program/{slug}`

Get comprehensive status for a PROGRAM manifest.

**Response:** `200 OK`
```json
{
  "program_slug": "ecommerce",
  "title": "E-commerce Platform",
  "state": "executing",
  "current_tier": 2,
  "tier_statuses": [...],
  "contract_statuses": [...],
  "completion": { "total_impls": 8, "completed_impls": 3, "percent": 37 },
  "is_executing": true,
  "validation_errors": []
}
```

---

### `GET /api/program/{slug}/tier/{n}`

Get status for a single tier.

**Path params:**
- `n` (int, >= 1)

**Response:** `200 OK` — `protocol.TierStatusDetail` JSON.

---

### `POST /api/program/{slug}/tier/{n}/execute`

Launch tier execution in background.

**Request (optional):**
```json
{ "auto": false }
```

**Response:** `202 Accepted`

**Errors:** `409` if tier already executing.

---

### `GET /api/program/{slug}/contracts`

Get program contracts with freeze status.

**Response:** `200 OK` — `[]protocol.ContractStatus` JSON.

---

### `POST /api/program/{slug}/replan`

Launch the Planner agent to revise the PROGRAM manifest.

**Request (optional):**
```json
{
  "reason": "Tier 2 failed due to missing auth contracts",
  "failed_tier": 2
}
```

**Response:** `202 Accepted`

---

## Autonomy / Pipeline

### `GET /api/pipeline`

Get a combined pipeline view of all IMPLs (completed, executing, queued) with metrics.

**Query params:**
- `include_completed` (string, optional) — `"true"` to include completed IMPLs in entries

**Response:** `200 OK`
```json
{
  "entries": [
    {
      "slug": "add-user-auth",
      "title": "Add User Auth",
      "status": "executing",
      "repo": "my-project",
      "wave_progress": "",
      "active_agent": "",
      "blocked_reason": "",
      "queue_position": 0,
      "depends_on": [],
      "completed_at": "",
      "elapsed_seconds": 0,
      "program_slug": "ecommerce",
      "program_title": "E-commerce Platform",
      "program_tier": 1,
      "program_tiers_total": 3
    }
  ],
  "metrics": {
    "impls_per_hour": 0,
    "avg_wave_seconds": 0,
    "queue_depth": 2,
    "blocked_count": 0,
    "completed_count": 5
  },
  "autonomy_level": "gated"
}
```

Entry `status` values: `"complete"`, `"executing"`, `"queued"`, `"blocked"`.

---

### `GET /api/queue`

List all queue items sorted by priority.

**Response:** `200 OK` — `[]queue.Item` JSON array.

---

### `POST /api/queue`

Add a new item to the execution queue.

**Request:**
```json
{
  "title": "Add payment processing",
  "priority": 100,
  "feature_description": "Integrate Stripe for payments",
  "depends_on": ["add-user-auth"],
  "autonomy_override": "",
  "require_review": true
}
```

**Response:** `201 Created` — The created `queue.Item` JSON.

---

### `DELETE /api/queue/{slug}`

Remove a queue item.

**Response:** `204 No Content`

**Errors:** `404` if not found.

---

### `PUT /api/queue/{slug}/priority`

Update the priority of a queue item.

**Request:**
```json
{ "priority": 50 }
```

**Response:** `200 OK` — Updated `queue.Item` JSON.

**Errors:** `404` if not found.

---

### `GET /api/autonomy`

Get the current autonomy configuration.

**Response:** `200 OK` — `autonomy.Config` JSON.

Returns defaults (`"gated"` level) if no config exists.

---

### `PUT /api/autonomy`

Save autonomy configuration.

**Request:** `autonomy.Config` JSON body.

**Response:** `200 OK` (empty body)

**Errors:** `400` if invalid autonomy level.

---

## Daemon

### `POST /api/daemon/start`

Start the autonomous execution daemon.

**Response:** `200 OK`
```json
{ "running": true }
```

**Errors:** `409` if already running, `500` if config load fails.

---

### `POST /api/daemon/stop`

Stop the running daemon.

**Response:** `200 OK` (empty body)

---

### `GET /api/daemon/status`

Get current daemon state.

**Response:** `200 OK`
```json
{ "running": false }
```

---

## Model Name Format

Model names support provider-prefix routing:

| Prefix | Provider | Example |
|--------|----------|---------|
| `anthropic:` | Anthropic API | `anthropic:claude-opus-4-6` |
| `bedrock:` | AWS Bedrock | `bedrock:claude-sonnet-4-5` |
| `openai:` | OpenAI | `openai:gpt-4o` |
| `ollama:` | Ollama (local) | `ollama:qwen2.5-coder:32b` |
| `lmstudio:` | LM Studio (local) | `lmstudio:phi-4` |
| `cli:` | Claude Code CLI | `cli:claude-sonnet-4-6` |
| (none) | Default (Anthropic API) | `claude-sonnet-4-6` |
