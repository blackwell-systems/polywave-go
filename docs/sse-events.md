# Server-Sent Events (SSE)

Last reviewed: 2026-03-24

The web application publishes real-time events via Server-Sent Events for monitoring Scout runs, Wave progress, agent execution, merge operations, program orchestration, and more.

## Endpoints

### Per-Slug Wave Events

```
GET /api/wave/{slug}/events
```

Streams all wave-related events for a specific IMPL slug: agent lifecycle, merge, test, pipeline steps, quality gates, etc. Late-connecting clients receive a snapshot of cached agent lifecycle events on connect.

### Per-Run Scout Events

```
GET /api/scout/{runID}/events
```

Streams Scout agent output for a specific run (used for both standard Scout and Bootstrap).

### Per-Run Chat Events

```
GET /api/impl/{slug}/chat/{runID}/events
```

Streams chat agent output for a specific chat session.

### Per-Run Revise Events

```
GET /api/impl/{slug}/revise/{runID}/events
```

Streams IMPL revision agent output for a specific revise session.

### Per-Run Planner Events

```
GET /api/planner/{runID}/events
```

Streams Planner agent output for a specific planner run.

### Per-Run Interview Events

```
GET /api/interview/{runID}/events
```

Streams interview question/answer events for a specific interview session.

### Global Events

```
GET /api/events
```

Streams global state changes (sidebar refresh, notifications, critic reviews, program updates). All connected clients receive these. Sends an initial `connected` event as a heartbeat.

### Program Events

```
GET /api/program/events
```

Filtered view of global events -- only forwards events with the `program_` prefix. Sends an initial `connected` event as a heartbeat.

### Daemon Events

```
GET /api/daemon/events
```

Streams daemon lifecycle events. Replays recent buffered events on connect. Sends an initial `connected` event as a heartbeat.

## Event Format

```
event: <event_type>
data: <JSON_payload>

```

**Fields:**
- `event` -- Event type (see Event Types below)
- `data` -- JSON payload (schema varies by event type)

---

## Event Types

### Run Lifecycle Events

**Emitted by:** Web API (`wave_runner.go`, `pkg/service/wave_service.go`)
**SSE endpoint:** `/api/wave/{slug}/events`

#### `run_started`

Wave execution loop begins.

```json
{
  "slug": "my-feature",
  "impl_path": "/path/to/repo/docs/IMPL/IMPL-my-feature.yaml"
}
```

When cross-repo targeting is detected:

```json
{
  "slug": "my-feature",
  "impl_path": "/path/to/IMPL-my-feature.yaml",
  "target_repos": ["other-repo"],
  "target_repo": "other-repo"
}
```

#### `run_complete`

All waves finished successfully.

```json
{
  "status": "success",
  "waves": 2,
  "agents": 5
}
```

When all waves were already complete:

```json
{
  "status": "success",
  "waves": 2,
  "agents": 5,
  "note": "all waves already complete"
}
```

#### `run_failed`

Run aborted due to error.

```json
{
  "error": "verify-commits: agents with no commits"
}
```

With pre-wave gate failure details:

```json
{
  "error": "pre-wave gate failed",
  "failed_checks": ["check_name: failure message"]
}
```

#### `waves_skipped`

Earlier waves skipped because completion reports indicate they already ran.

```json
{
  "skipped": 1,
  "reason": "already completed (completion reports present)"
}
```

#### `wave_resumed`

A wave's agents already have branch commits from a previous session; skipping straight to merge.

```json
{
  "wave": 1,
  "reason": "agent branches already have commits from previous session; skipping to merge"
}
```

#### `mark_complete_warning`

Non-fatal warning when marking IMPL as complete fails (e.g. already archived).

```json
{
  "error": "file already in complete directory"
}
```

#### `update_status_failed`

Non-fatal warning when updating agent completion status in the IMPL doc fails.

```json
{
  "wave": "my-feature",
  "error": "write error"
}
```

#### `stale_branches_detected`

Advisory event when leftover branches from previous runs are detected.

```json
{
  "slug": "my-feature",
  "branches": ["saw/my-feature/wave1-agent-A", "saw/my-feature/wave1-agent-B"],
  "count": 2
}
```

---

### Cross-Repo Events

**Emitted by:** Service layer (`pkg/service/wave_service.go`)
**SSE endpoint:** `/api/wave/{slug}/events`

#### `repo_redirected`

IMPL targets a different repo than the server's startup repo. Wave execution is redirected.

```json
{
  "from": "/path/to/server-repo",
  "to": "/path/to/target-repo",
  "target_repo": "other-repo"
}
```

#### `repo_mismatch_warning`

Advisory warning about cross-repo targeting with details.

```json
{
  "slug": "my-feature",
  "server_repo": "/path/to/server-repo",
  "target_repo": "other-repo",
  "target_repo_path": "/path/to/target-repo",
  "message": "IMPL targets repo \"other-repo\" at /path/to/target-repo but server is running from /path/to/server-repo. Redirecting worktree operations."
}
```

#### `stage_scaffold_running`

Scaffold stage is starting (service layer variant, emitted before engine scaffold events).

```json
null
```

---

### Agent Events

**Emitted by:** Engine orchestrator (`pkg/orchestrator/`)
**SSE endpoint:** `/api/wave/{slug}/events`

#### `agent_started`

Agent execution begins in its worktree.

```json
{
  "agent": "A",
  "wave": 1,
  "files": ["src/auth.go", "src/middleware.go"]
}
```

Payload struct: `orchestrator.AgentStartedPayload`

| Field   | Type     | JSON        |
|---------|----------|-------------|
| Agent   | string   | `agent`     |
| Wave    | int      | `wave`      |
| Files   | []string | `files`     |

#### `agent_output`

Agent text output (streamed chunks during execution).

```json
{
  "agent": "A",
  "wave": 1,
  "chunk": "Reading src/auth.go..."
}
```

Payload struct: `orchestrator.AgentOutputPayload`

| Field | Type   | JSON    |
|-------|--------|---------|
| Agent | string | `agent` |
| Wave  | int    | `wave`  |
| Chunk | string | `chunk` |

#### `agent_tool_call`

Agent invokes a tool or receives a tool result.

```json
{
  "agent": "A",
  "wave": 1,
  "tool_id": "toolu_01abc",
  "tool_name": "Read",
  "input": "{\"file_path\":\"src/auth.go\"}",
  "is_result": false,
  "is_error": false,
  "duration_ms": 0
}
```

Payload struct: `orchestrator.AgentToolCallPayload` (mirrored in `api.AgentToolCallPayload`)

| Field      | Type   | JSON          | Notes                                    |
|------------|--------|---------------|------------------------------------------|
| Agent      | string | `agent`       |                                          |
| Wave       | int    | `wave`        |                                          |
| ToolID     | string | `tool_id`     |                                          |
| ToolName   | string | `tool_name`   |                                          |
| Input      | string | `input`       | JSON-encoded tool input                  |
| IsResult   | bool   | `is_result`   | `false` = invocation, `true` = result    |
| IsError    | bool   | `is_error`    | `true` when tool returned an error       |
| DurationMs | int64  | `duration_ms` | Only set when `is_result` is `true`      |

#### `agent_complete`

Agent finished execution successfully.

```json
{
  "agent": "A",
  "wave": 1,
  "status": "complete",
  "branch": "saw/my-feature/wave1-agent-A"
}
```

Payload struct: `orchestrator.AgentCompletePayload`

| Field  | Type   | JSON     |
|--------|--------|----------|
| Agent  | string | `agent`  |
| Wave   | int    | `wave`   |
| Status | string | `status` |
| Branch | string | `branch` |

#### `agent_failed`

Agent execution failed.

```json
{
  "agent": "B",
  "wave": 1,
  "status": "failed",
  "failure_type": "fixable",
  "notes": "",
  "message": "Missing dependency: crypto package not installed"
}
```

Payload struct: `orchestrator.AgentFailedPayload`

| Field       | Type   | JSON           |
|-------------|--------|----------------|
| Agent       | string | `agent`        |
| Wave        | int    | `wave`         |
| Status      | string | `status`       |
| FailureType | string | `failure_type` |
| Notes       | string | `notes`        |
| Message     | string | `message`      |

**Failure types (from E19):** `transient`, `fixable`, `needs_replan`, `escalate`, `timeout`

Also emitted by the web API for rerun/resume failures with `failure_type` values `"rerun"` or `"resume"`.

#### `agent_blocked`

Agent encountered a blocking condition that requires routing (E19).

```json
{
  "agent": "C",
  "wave": 1,
  "status": "blocked",
  "failure_type": "needs_replan",
  "message": "Architecture issue detected"
}
```

Uses the same `AgentFailedPayload` struct.

#### `agent_progress`

Derived progress event emitted by the web API's progress parser when an `agent_tool_call` event is processed.

```json
{
  "agent": "A",
  "wave": 1,
  "current_file": "src/auth.go",
  "current_action": "Writing src/auth.go",
  "percent_done": 50
}
```

Payload struct: `api.AgentProgressPayload`

| Field         | Type   | JSON             |
|---------------|--------|------------------|
| Agent         | string | `agent`          |
| Wave          | int    | `wave`           |
| CurrentFile   | string | `current_file`   |
| CurrentAction | string | `current_action` |
| PercentDone   | int    | `percent_done`   |

#### `agent_prioritized`

Agents in a wave were reordered for optimal scheduling based on dependency graph analysis.

```json
{
  "wave": 1,
  "original_order": ["A", "B", "C"],
  "prioritized_order": ["C", "A", "B"],
  "reordered": true,
  "reason": "critical path optimization"
}
```

Payload struct: `orchestrator.AgentPrioritizedPayload`

| Field             | Type     | JSON                |
|-------------------|----------|---------------------|
| Wave              | int      | `wave`              |
| OriginalOrder     | []string | `original_order`    |
| PrioritizedOrder  | []string | `prioritized_order` |
| Reordered         | bool     | `reordered`         |
| Reason            | string   | `reason`            |

#### `agent_resumed`

Agent re-launched during a resume operation.

```json
{
  "slug": "my-feature",
  "wave": 1,
  "agent": "A",
  "status": "launching"
}
```

---

### Auto-Retry Events (E19)

**Emitted by:** Engine orchestrator (`pkg/orchestrator/`)
**SSE endpoint:** `/api/wave/{slug}/events`

#### `auto_retry_started`

Orchestrator is auto-retrying a failed agent.

```json
{
  "agent": "B",
  "wave": 1,
  "failure_type": "transient",
  "attempt": 2,
  "max_attempts": 3
}
```

Payload struct: `orchestrator.AutoRetryStartedPayload`

| Field       | Type   | JSON           |
|-------------|--------|----------------|
| Agent       | string | `agent`        |
| Wave        | int    | `wave`         |
| FailureType | string | `failure_type` |
| Attempt     | int    | `attempt`      |
| MaxAttempts | int    | `max_attempts` |

#### `auto_retry_exhausted`

All auto-retry attempts exhausted for an agent.

```json
{
  "agent": "B",
  "wave": 1,
  "failure_type": "transient",
  "attempts": 3
}
```

Payload struct: `orchestrator.AutoRetryExhaustedPayload`

| Field       | Type   | JSON           |
|-------------|--------|----------------|
| Agent       | string | `agent`        |
| Wave        | int    | `wave`         |
| FailureType | string | `failure_type` |
| Attempts    | int    | `attempts`     |

---

### Wave Events

**Emitted by:** Engine orchestrator (`pkg/orchestrator/`)
**SSE endpoint:** `/api/wave/{slug}/events`

#### `wave_complete`

All agents in a wave finished (pre-merge).

```json
{
  "wave": 1,
  "merge_status": "pending"
}
```

Payload struct: `orchestrator.WaveCompletePayload`

| Field       | Type   | JSON           |
|-------------|--------|----------------|
| Wave        | int    | `wave`         |
| MergeStatus | string | `merge_status` |

---

### Wave Gate Events

**Emitted by:** Web API (`wave_runner.go`, `pkg/service/wave_service.go`)
**SSE endpoint:** `/api/wave/{slug}/events`

#### `wave_gate_pending`

Paused between waves, awaiting user approval to proceed.

```json
{
  "wave": 1,
  "next_wave": 2,
  "slug": "my-feature"
}
```

#### `wave_gate_resolved`

Gate approved, proceeding to next wave.

```json
{
  "wave": 1,
  "action": "proceed",
  "slug": "my-feature"
}
```

---

### Stage Transition Events

**Emitted by:** Web API (`wave_runner.go`)
**SSE endpoint:** `/api/wave/{slug}/events`

#### `stage_transition`

Execution moved to a new stage or changed status within a stage.

```json
{
  "stage": "wave_execute",
  "status": "running",
  "wave_num": 1,
  "message": ""
}
```

**Stage values:** `scaffold`, `wave_execute`, `wave_merge`, `wave_verify`, `wave_gate`, `complete`

**Status values:** `running`, `complete`, `failed`, `skipped`

---

### Pipeline Step Events

**Emitted by:** Web API (`wave_runner.go`, `recovery_handlers.go`)
**SSE endpoint:** `/api/wave/{slug}/events`

#### `pipeline_step`

A finalization pipeline step changed status.

```json
{
  "slug": "my-feature",
  "wave": 1,
  "step": "verify_commits",
  "status": "running",
  "error": ""
}
```

**Step values:** `verify_commits`, `scan_stubs`, `run_gates`, `validate_integration`, `merge_agents`, `fix_go_mod`, `verify_build`, `code_review`, `integration_agent`, `cleanup`

**Status values:** `pending`, `running`, `complete`, `failed`, `skipped`

#### `step_retry_started`

A failed pipeline step is being retried manually.

```json
{
  "slug": "my-feature",
  "wave": 1,
  "step": "run_gates"
}
```

#### `step_retry_complete`

Pipeline step retry succeeded.

```json
{
  "slug": "my-feature",
  "wave": 1,
  "step": "run_gates"
}
```

With non-fatal warning:

```json
{
  "slug": "my-feature",
  "wave": 1,
  "step": "validate_integration",
  "warning": "some non-fatal error"
}
```

#### `step_retry_failed`

Pipeline step retry failed.

```json
{
  "slug": "my-feature",
  "wave": 1,
  "step": "run_gates",
  "error": "required gate \"build\" failed"
}
```

---

### Quality Gate Events

**Emitted by:** Engine orchestrator + Web API (`wave_runner.go`)
**SSE endpoint:** `/api/wave/{slug}/events`

#### `quality_gate_result`

A quality gate completed. Data is `protocol.GateResult`.

```json
{
  "type": "build",
  "command": "go build ./...",
  "exit_code": 0,
  "stdout": "",
  "stderr": "",
  "required": true,
  "passed": true,
  "skipped": false,
  "skip_reason": "",
  "from_cache": false,
  "parsed_errors": []
}
```

Payload struct: `protocol.GateResult`

| Field        | Type                  | JSON             |
|--------------|-----------------------|------------------|
| Type         | string                | `type`           |
| Command      | string                | `command`        |
| ExitCode     | int                   | `exit_code`      |
| Stdout       | string                | `stdout`         |
| Stderr       | string                | `stderr`         |
| Required     | bool                  | `required`       |
| Passed       | bool                  | `passed`         |
| Skipped      | bool                  | `skipped`        |
| SkipReason   | string                | `skip_reason`    |
| FromCache    | bool                  | `from_cache`     |
| ParsedErrors | []StructuredError     | `parsed_errors`  |

#### `gate_cache_hit`

A quality gate result was served from cache (E38).

```json
{
  "gate_type": "build",
  "command": "go build ./...",
  "wave": 1,
  "sha": "abc123"
}
```

#### `stub_report`

Stub scan (E20) found placeholder patterns in agent-changed files. Data is `protocol.ScanStubsResult`.

```json
{
  "hits": [
    {
      "file": "src/auth.go",
      "line": 42,
      "pattern": "TODO",
      "context": "// TODO: implement token refresh"
    }
  ]
}
```

#### `code_review_result`

AI code review gate completed (requires `codereview` build tag).

```json
{
  "slug": "my-feature",
  "wave": 1,
  "dimensions": { ... },
  "overall": 82,
  "passed": true,
  "summary": "Code quality is good...",
  "model": "claude-sonnet-4-20250514",
  "diff_bytes": 4200
}
```

---

### Wiring Validation Events (E35)

**Emitted by:** Web API (`wave_runner.go`)
**SSE endpoint:** `/api/wave/{slug}/events`

#### `wiring_gap`

A single wiring declaration was not found in the codebase.

```json
{
  "wave": 1,
  "symbol": "HandleAuth",
  "defined_in": "pkg/auth/handler.go",
  "must_be_called_from": "pkg/api/router.go",
  "agent": "A",
  "reason": "call site not found",
  "severity": "warning"
}
```

#### `wiring_gaps_summary`

Summary after all wiring declarations have been checked.

```json
{
  "wave": 1,
  "gap_count": 2,
  "summary": "2 wiring gaps detected"
}
```

---

### Merge Events

**Emitted by:** Web API (`merge_test_handlers.go`, `wave_runner.go`, `pkg/service/wave_service.go`)
**SSE endpoint:** `/api/wave/{slug}/events`

#### `merge_started`

Merge/finalize operation begins.

```json
{
  "slug": "my-feature",
  "wave": 1
}
```

#### `merge_output`

Streaming text output during merge/cleanup/fixup operations.

```json
{
  "slug": "my-feature",
  "wave": 1,
  "chunk": "Merging wave 1 agents...\n"
}
```

#### `merge_complete`

Merge/finalize succeeded.

```json
{
  "slug": "my-feature",
  "wave": 1,
  "status": "success"
}
```

#### `merge_failed`

Merge/finalize failed.

```json
{
  "slug": "my-feature",
  "wave": 1,
  "error": "CONFLICT (content): Merge conflict in src/auth.go",
  "conflicting_files": ["src/auth.go"]
}
```

Note: `conflicting_files` is only present when the failure is a merge conflict.

---

### Conflict Resolution Events

**Emitted by:** Web API (`merge_test_handlers.go`)
**SSE endpoint:** `/api/wave/{slug}/events`

#### `conflict_resolving`

AI is resolving a conflict in a specific file.

```json
{
  "slug": "my-feature",
  "wave": 1,
  "file": "src/auth.go"
}
```

#### `conflict_resolved`

AI successfully resolved a conflict in a specific file.

```json
{
  "slug": "my-feature",
  "wave": 1,
  "file": "src/auth.go"
}
```

#### `conflict_resolution_failed`

AI conflict resolution failed.

```json
{
  "slug": "my-feature",
  "wave": 1,
  "error": "unable to resolve conflict in src/auth.go"
}
```

---

### Fix Build Events

**Emitted by:** Web API (`merge_test_handlers.go`)
**SSE endpoint:** `/api/wave/{slug}/events`

#### `fix_build_started`

AI build-fix agent launched.

```json
{
  "slug": "my-feature",
  "wave": 1,
  "gate": "build"
}
```

#### `fix_build_output`

Streaming text output from the build-fix agent.

```json
{
  "slug": "my-feature",
  "wave": 1,
  "chunk": "Analyzing build error..."
}
```

#### `fix_build_tool_call`

Tool call from the build-fix agent.

```json
{
  "slug": "my-feature",
  "wave": 1,
  "tool_id": "toolu_01abc",
  "tool_name": "Edit",
  "input": "...",
  "is_result": false,
  "is_error": false,
  "duration_ms": 0
}
```

#### `fix_build_complete`

Build-fix agent finished successfully.

```json
{
  "slug": "my-feature",
  "wave": 1
}
```

#### `fix_build_failed`

Build-fix agent failed.

```json
{
  "slug": "my-feature",
  "wave": 1,
  "error": "unable to fix build failure"
}
```

---

### Test Events

**Emitted by:** Web API (`merge_test_handlers.go`)
**SSE endpoint:** `/api/wave/{slug}/events`

#### `test_started`

Test command execution begins.

```json
{
  "slug": "my-feature",
  "wave": 1
}
```

#### `test_output`

Streaming test output line-by-line.

```json
{
  "slug": "my-feature",
  "wave": 1,
  "chunk": "--- PASS: TestAuthenticate (0.01s)\n"
}
```

#### `test_complete`

Test command passed.

```json
{
  "slug": "my-feature",
  "wave": 1,
  "status": "pass"
}
```

#### `test_failed`

Test command failed.

```json
{
  "slug": "my-feature",
  "wave": 1,
  "status": "fail",
  "output": "FAIL: TestAuthenticate (0.01s)\n    auth_test.go:42: expected nil, got error"
}
```

---

### Resume Events

**Emitted by:** Web API (`resume_action.go`)
**SSE endpoint:** `/api/wave/{slug}/events`

#### `resume_started`

Interrupted session resume begins.

```json
{
  "slug": "my-feature",
  "wave": 1
}
```

#### `resume_complete`

Resume operation finished.

```json
{
  "slug": "my-feature",
  "wave": 1,
  "status": "success"
}
```

---

### Scaffold Events

**Emitted by:** Engine (`pkg/engine/runner.go`) and Web API (`scaffold_handler.go`)
**SSE endpoint:** `/api/wave/{slug}/events`

#### `scaffold_started`

Scaffold agent begins generating shared type definitions.

```json
{
  "impl_path": "/path/to/repo/docs/IMPL/IMPL-my-feature.yaml"
}
```

#### `scaffold_output`

Scaffold agent text output (streamed chunks).

```json
{
  "chunk": "Creating shared interface..."
}
```

#### `scaffold_complete`

Scaffold agent finished successfully.

```json
{
  "impl_path": "/path/to/repo/docs/IMPL/IMPL-my-feature.yaml"
}
```

#### `scaffold_failed`

Scaffold agent failed.

```json
{
  "error": "failed to create scaffold files"
}
```

#### `scaffold_cancelled`

Scaffold agent was cancelled by the user.

```json
{
  "run_id": "1710900000000000000",
  "slug": "my-feature"
}
```

---

### Scout Events

**Emitted by:** Web API (`scout.go`, `bootstrap_handler.go`)
**SSE endpoint:** `/api/scout/{runID}/events`

#### `scout_output`

Scout agent text output (streamed chunks).

```json
{
  "run_id": "1710900000000000000",
  "chunk": "Analyzing codebase structure..."
}
```

#### `scout_finalize`

IMPL doc post-processing (M4: populate verification gates).

Running:
```json
{
  "run_id": "1710900000000000000",
  "status": "running"
}
```

Complete:
```json
{
  "run_id": "1710900000000000000",
  "status": "complete",
  "agents_updated": "5"
}
```

Warning:
```json
{
  "run_id": "1710900000000000000",
  "status": "warning",
  "message": "Verification gates not fully populated (H2 data unavailable or validation issues)"
}
```

#### `scout_complete`

Scout agent finished IMPL doc generation.

```json
{
  "run_id": "1710900000000000000",
  "slug": "my-feature",
  "impl_path": "/path/to/repo/docs/IMPL/IMPL-my-feature.yaml"
}
```

#### `scout_failed`

Scout agent failed.

```json
{
  "run_id": "1710900000000000000",
  "error": "context deadline exceeded"
}
```

#### `scout_cancelled`

Scout agent was cancelled by the user.

```json
{
  "run_id": "1710900000000000000"
}
```

---

### Chat Events

**Emitted by:** Web API (`chat_handler.go`)
**SSE endpoint:** `/api/impl/{slug}/chat/{runID}/events`

#### `chat_output`

Chat agent text output (streamed chunks).

```json
{
  "run_id": "1710900000000000000",
  "chunk": "Based on the IMPL doc..."
}
```

#### `chat_complete`

Chat agent finished.

```json
{
  "run_id": "1710900000000000000",
  "slug": "my-feature"
}
```

#### `chat_failed`

Chat agent failed.

```json
{
  "run_id": "1710900000000000000",
  "error": "cancelled"
}
```

---

### IMPL Revision Events

**Emitted by:** Web API (`impl_edit.go`)
**SSE endpoint:** `/api/impl/{slug}/revise/{runID}/events`

#### `revise_output`

Revision agent text output (streamed chunks).

```json
{
  "run_id": "1710900000000000000",
  "chunk": "Reading IMPL doc..."
}
```

#### `revise_complete`

Revision agent finished.

```json
{
  "run_id": "1710900000000000000",
  "slug": "my-feature"
}
```

#### `revise_failed`

Revision agent failed.

```json
{
  "run_id": "1710900000000000000",
  "error": "failed to write IMPL doc"
}
```

#### `revise_cancelled`

Revision agent was cancelled by the user.

```json
{
  "run_id": "1710900000000000000"
}
```

---

### IMPL Approval Events

**Emitted by:** Service layer (`pkg/service/impl_service.go`)
**SSE endpoint:** `/api/wave/{slug}/events` (published to the slug channel)

#### `plan_approved`

User approved the IMPL plan.

```json
{
  "slug": "my-feature"
}
```

#### `plan_rejected`

User rejected the IMPL plan.

```json
{
  "slug": "my-feature"
}
```

---

### Planner Events

**Emitted by:** Web API (`planner.go`)
**SSE endpoint:** `/api/planner/{runID}/events`

#### `planner_output`

Planner agent text output (streamed chunks).

```json
{
  "run_id": "1710900000000000000",
  "chunk": "Analyzing project requirements..."
}
```

#### `planner_complete`

Planner agent finished.

```json
{
  "run_id": "1710900000000000000",
  "slug": "my-project",
  "program_path": "/path/to/repo/docs/PROGRAM-my-project.yaml"
}
```

#### `planner_failed`

Planner agent failed.

```json
{
  "run_id": "1710900000000000000",
  "error": "context deadline exceeded"
}
```

#### `planner_cancelled`

Planner agent was cancelled by the user.

```json
{
  "run_id": "1710900000000000000"
}
```

---

### Interview Events

**Emitted by:** Web API (`interview_handlers.go`)
**SSE endpoint:** `/api/interview/{runID}/events`

#### `question`

A new interview question is ready for the user.

```json
{
  "phase": "discovery",
  "question_num": 1,
  "max_questions": 10,
  "text": "What problem does this feature solve?",
  "hint": "Describe the user pain point"
}
```

Payload struct: `api.InterviewQuestionEvent`

| Field        | Type   | JSON             |
|--------------|--------|------------------|
| Phase        | string | `phase`          |
| QuestionNum  | int    | `question_num`   |
| MaxQuestions | int    | `max_questions`  |
| Text         | string | `text`           |
| Hint         | string | `hint`           |

#### `answer_recorded`

User's answer was successfully recorded.

```json
{
  "status": "recorded"
}
```

#### `complete`

Interview finished. All questions answered.

```json
{
  "requirements_path": "/path/to/requirements.md",
  "slug": "my-feature"
}
```

#### `error`

Interview error or cancellation.

```json
{
  "message": "interview cancelled"
}
```

---

### Program Events

**Emitted by:** Web API (`program_runner.go`, `program_handler.go`, `pkg/service/program_service.go`)
**SSE endpoint:** `/api/program/events` (global, filtered), `/api/events` (global)

#### `program_tier_started`

A program tier begins execution.

```json
{
  "program_slug": "my-project",
  "tier": 1
}
```

#### `program_impl_started`

An IMPL within a tier begins execution.

```json
{
  "program_slug": "my-project",
  "impl_slug": "add-auth"
}
```

#### `program_impl_wave_progress`

Wave progress update for an IMPL within a tier.

```json
{
  "program_slug": "my-project",
  "impl_slug": "add-auth",
  "current_wave": 1,
  "total_waves": 2
}
```

#### `program_impl_complete`

An IMPL within a tier finished execution.

```json
{
  "program_slug": "my-project",
  "impl_slug": "add-auth"
}
```

#### `program_contract_frozen`

A contract was frozen at a tier boundary.

```json
{
  "program_slug": "my-project",
  "contract_name": "AuthService",
  "tier": 1
}
```

#### `program_tier_complete`

A program tier finished execution.

```json
{
  "program_slug": "my-project",
  "tier": 1
}
```

#### `program_tier_failed`

A program tier failed.

```json
{
  "program_slug": "my-project",
  "tier": 1,
  "error": "tier gates failed for tier 1"
}
```

#### `program_complete`

All tiers in the program completed successfully.

```json
{
  "program_slug": "my-project"
}
```

Engine-emitted variant (via `orchestrator.ProgramCompletePayload`):

```json
{
  "total_tiers": 3,
  "total_impls": 8,
  "final_state": "complete"
}
```

#### `program_blocked`

Program execution is blocked (IMPL not found, gate failure, contract freeze failure, shutdown, etc.).

```json
{
  "program_slug": "my-project",
  "tier": 1,
  "reason": "tier gates failed"
}
```

With IMPL context:

```json
{
  "program_slug": "my-project",
  "impl_slug": "add-auth",
  "reason": "IMPL doc not found: ..."
}
```

#### `program_replan_complete`

Program replan finished.

```json
{
  "program_slug": "my-project",
  "validation_passed": true,
  "changes_summary": "Added tier 3 for monitoring"
}
```

#### `program_replan_failed`

Program replan failed.

```json
{
  "program_slug": "my-project",
  "error": "failed to parse revised manifest"
}
```

---

### IMPL Branch Events (E28)

**Emitted by:** Web API (`program_handler.go`)
**SSE endpoint:** `/api/events` (global)

These events track IMPL branch lifecycle during program-tier execution.
Each IMPL in a tier runs on an isolated branch; waves merge to that branch
instead of main. The branch is merged to main after all waves complete.

#### `impl_branch_created`

An IMPL branch was created for program-tier execution.

```json
{
  "program_slug": "my-project",
  "tier": 1,
  "impl_slug": "add-auth",
  "branch": "saw/program/my-project/tier1/add-auth"
}
```

#### `impl_branch_merged`

An IMPL branch was merged back to main after successful completion.

```json
{
  "program_slug": "my-project",
  "tier": 1,
  "impl_slug": "add-auth",
  "branch": "saw/program/my-project/tier1/add-auth"
}
```

---

### Queue Events

**Emitted by:** Engine (`pkg/engine/queue_advance.go`)

#### `queue_advanced`

The IMPL queue advanced to the next item.

```json
{
  "advanced": true,
  "next_slug": "add-auth",
  "next_title": "Add user authentication",
  "reason": "triggered"
}
```

Payload struct: `engine.CheckQueueResult`

| Field     | Type   | JSON         |
|-----------|--------|--------------|
| Advanced  | bool   | `advanced`   |
| NextSlug  | string | `next_slug`  |
| NextTitle | string | `next_title` |
| Reason    | string | `reason`     |

**Reason values:** `no_items`, `deps_unmet`, `autonomy_blocked`, `triggered`

---

### Critic Events

**Emitted by:** Web API (`critic_handler.go`)
**SSE endpoint:** `/api/events` (global)

#### `critic_review_started`

Critic review process initiated for an IMPL.

```json
{
  "slug": "my-feature"
}
```

#### `critic_output`

Streaming text output from the critic process.

```json
{
  "slug": "my-feature",
  "chunk": "Analyzing IMPL structure...\n"
}
```

#### `critic_review_complete`

Critic review (E37) completed for an IMPL.

```json
{
  "slug": "my-feature",
  "result": { ... }
}
```

#### `critic_review_failed`

Critic review failed.

```json
{
  "slug": "my-feature",
  "error": "critic timed out after 5 minutes"
}
```

#### `critic_autofix_started`

Critic autofix process initiated for an IMPL (applies critic suggestions automatically).

```json
{
  "slug": "my-feature"
}
```

#### `critic_autofix_complete`

Critic autofix finished.

```json
{
  "slug": "my-feature",
  ...
}
```

#### `impl_updated`

An IMPL doc was updated (e.g. by critic autofix applying changes).

```json
{
  "slug": "my-feature"
}
```

---

### Daemon Events

**Emitted by:** Web API (`daemon_handler.go`)
**SSE endpoint:** `/api/daemon/events`

#### `daemon_event`

A daemon lifecycle event (IMPL started, completed, queue checked, etc.). Payload is `engine.Event`.

```json
{
  "type": "impl_started",
  "slug": "add-auth",
  "timestamp": "2026-03-22T10:00:00Z",
  "message": "Starting IMPL execution"
}
```

#### `daemon_stopped`

The daemon process stopped. Payload is `engine.DaemonState`.

```json
{
  "running": false
}
```

---

### Global Broadcast Events

**Emitted by:** Web API (various handlers)
**SSE endpoint:** `/api/events`

These events are broadcast to all connected clients, not per-slug.

#### `connected`

Sent on initial connection as a heartbeat.

```json
{}
```

#### `impl_list_updated`

The IMPL document list changed (new IMPL created, archived, deleted, execution started/stopped). Frontend should re-fetch the IMPL list. Also triggered by filesystem watcher (fsnotify) on IMPL directory changes.

```json
{}
```

#### `program_list_updated`

The program list changed (new program created, tier completed). Frontend should re-fetch the program list.

With context:
```json
{
  "slug": "my-project"
}
```

Without context (simple broadcast):
```json
{}
```

#### `notification`

User-facing notification (toast/browser notification). Published by the `NotificationBus`.

```json
{
  "type": "wave_complete",
  "slug": "my-feature",
  "title": "Wave 1 Complete",
  "message": "Wave finalized successfully and merged to main",
  "severity": "success"
}
```

Payload struct: `api.NotificationEvent`

| Field    | Type   | JSON       |
|----------|--------|------------|
| Type     | string | `type`     |
| Slug     | string | `slug`     |
| Title    | string | `title`    |
| Message  | string | `message`  |
| Severity | string | `severity` |

**Notification types:** `wave_complete`, `agent_failed`, `merge_complete`, `merge_failed`, `scaffold_complete`, `build_verify_pass`, `build_verify_fail`, `impl_complete`, `run_failed`

**Severity values:** `info`, `success`, `warning`, `error`

---

## Engine Event Types (Orchestrator)

The engine/orchestrator defines additional event types in `pkg/orchestrator/events.go` that are emitted via `OrchestratorEvent` structs during program-level tier execution. These are consumed by the web API's engine event bridge and mapped to SSE events. The following are defined as constants but emitted by the engine tier loop (not the web API directly):

| Constant                          | Event String                  | Payload Struct                        |
|-----------------------------------|-------------------------------|---------------------------------------|
| `EventProgramTierStarted`        | `program_tier_started`        | `ProgramTierStartedPayload`           |
| `EventProgramScoutLaunched`      | `program_scout_launched`      | `ProgramScoutLaunchedPayload`         |
| `EventProgramScoutComplete`      | `program_scout_complete`      | `ProgramScoutCompletePayload`         |
| `EventProgramIMPLComplete`       | `program_impl_complete`       | `ProgramIMPLCompletePayload`          |
| `EventProgramTierGateStarted`    | `program_tier_gate_started`   | `ProgramTierGateStartedPayload`       |
| `EventProgramTierGateResult`     | `program_tier_gate_result`    | `ProgramTierGateResultPayload`        |
| `EventProgramContractsFrozen`    | `program_contracts_frozen`    | `ProgramContractsFrozenPayload`       |
| `EventProgramTierAdvanced`       | `program_tier_advanced`       | `ProgramTierAdvancedPayload`          |
| `EventProgramReplanTriggered`    | `program_replan_triggered`    | `ProgramReplanTriggeredPayload`       |
| `EventProgramComplete`           | `program_complete`            | `ProgramCompletePayload`              |

Note: The web API emits its own versions of some program events (e.g. `program_tier_started`, `program_impl_complete`) with slightly different payloads. The engine versions include richer metadata (e.g. `ProgramTierStartedPayload` includes `total_tiers` and `impls` fields). Which version a client receives depends on whether the execution is driven by the web API's `program_runner.go` or the engine's `RunTierLoop`.

---

## SSE Broker Architecture

### Per-Slug Broker

**Implementation:** `pkg/api/wave.go` -- `sseBroker` type

The per-slug broker manages SSE subscriptions keyed by slug (or broker key like `"scout-{runID}"`, `"chat-{runID}"`, `"revise-{runID}"`, `"planner-{runID}"`, `"interview-{runID}"`). Each slug has a list of subscriber channels. Events are published with non-blocking sends; slow clients have events dropped.

Agent lifecycle events (`agent_started`, `agent_complete`, `agent_failed`, `auto_retry_started`, `auto_retry_exhausted`) are cached in an `agentSnapshot` map so late-connecting clients (e.g. page reload mid-execution) receive a snapshot of current agent states on connect.

### Global Broker

**Implementation:** `pkg/api/global_events.go` -- `globalBroker` type

The global broker broadcasts events to all connected clients. Supports two formats:
- `broadcast(event)` -- simple event name with empty `{}` data
- `broadcastJSON(eventType, data)` -- event with JSON payload

A filesystem watcher (`fsnotify`) monitors IMPL directories across all configured repos and broadcasts `impl_list_updated` when `.yaml` files are created, renamed, or removed.

### SSE Publisher (Service Bridge)

**Implementation:** `pkg/api/sse_publisher.go` -- `SSEPublisher` type

Bridges the service layer's `EventPublisher` interface to the concrete SSE brokers. Routes events by channel name:
- `"global"` or `"global:*"` channels use `globalBroker.broadcastJSON`
- `"wave:{slug}"` channels use `sseBroker.Publish` with the slug as key
- `"scout:{runID}"` channels use `sseBroker.Publish` with `"scout-"+runID` as key
- All other channels use `sseBroker.Publish` with the channel name as key

### Daemon Event Broker

**Implementation:** `pkg/api/daemon_handler.go` -- `daemonEventBroker` type

Separate broker for daemon events. Buffers the last 100 events and replays them to newly-connected clients.

### Notification Bus

**Implementation:** `pkg/api/notification_bus.go` -- `NotificationBus` type

Central hub for user-facing notifications. Handlers call `Notify()` with a `NotificationEvent`, which is broadcast to all SSE clients as a `"notification"` event via `globalBroker.broadcastJSON`. Thread-safe.

### Engine Event Bridge

**Implementation:** `pkg/orchestrator/events.go` -- `OrchestratorEvent` type

The engine/orchestrator emits `OrchestratorEvent` structs. The web API maps these to `SSEEvent` structs via `makeEnginePublisher()` and `makePublisher()` in `wave_runner.go`.

**Publishing (web API):**

```go
// Direct publish
s.broker.Publish(slug, SSEEvent{Event: "plan_approved", Data: map[string]string{"slug": slug}})

// Via publisher closure (wave execution)
publish := s.makePublisher(slug)
publish("run_started", map[string]string{"slug": slug, "impl_path": implPath})

// Engine events bridged to SSE
enginePublisher := s.makeEnginePublisher(slug)
engine.RunSingleWave(ctx, opts, waveNum, enginePublisher)
```

**Subscribing (web client):**

```javascript
// Wave events for a specific IMPL
const waveEvents = new EventSource('/api/wave/my-feature/events')
waveEvents.addEventListener('agent_started', (e) => {
  const data = JSON.parse(e.data)
  console.log(`Agent ${data.agent} started wave ${data.wave}`)
})

// Scout events for a specific run
const scoutEvents = new EventSource('/api/scout/1710900000000000000/events')
scoutEvents.addEventListener('scout_output', (e) => {
  const data = JSON.parse(e.data)
  console.log(data.chunk)
})

// Global events (all clients)
const globalEvents = new EventSource('/api/events')
globalEvents.addEventListener('impl_list_updated', () => {
  refreshImplList()
})
globalEvents.addEventListener('notification', (e) => {
  const data = JSON.parse(e.data)
  showToast(data.title, data.message, data.severity)
})

// Daemon events
const daemonEvents = new EventSource('/api/daemon/events')
daemonEvents.addEventListener('daemon_event', (e) => {
  const data = JSON.parse(e.data)
  console.log(data.type, data.message)
})

// Interview events
const interviewEvents = new EventSource('/api/interview/interview-123/events')
interviewEvents.addEventListener('question', (e) => {
  const data = JSON.parse(e.data)
  showQuestion(data.text, data.hint)
})
```
