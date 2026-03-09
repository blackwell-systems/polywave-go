# Server-Sent Events (SSE)

The orchestrator publishes real-time events via Server-Sent Events for monitoring Scout runs, Wave progress, and agent execution.

## Endpoint

```
GET /api/events?session_id=<session_id>
```

**Query Parameters:**
- `session_id` (optional) — Filter events for a specific session. If omitted, receives all events across all sessions.

**Response Format:** SSE stream (`text/event-stream`)

## Event Format

```
event: <event_type>
data: <JSON_payload>
id: <event_id>

```

**Fields:**
- `event` — Event type (see Event Types below)
- `data` — JSON payload (schema varies by event type)
- `id` — Monotonically increasing event ID for replay

## Event Types

### Scout Events

#### `scout_started`

Scout agent begins IMPL doc generation.

```json
{
  "session_id": "abc123",
  "prompt": "Add user authentication",
  "repo_path": "/path/to/repo",
  "timestamp": "2026-03-09T10:00:00Z"
}
```

#### `scout_output`

Scout agent text output (streamed chunks).

```json
{
  "session_id": "abc123",
  "chunk": "Analyzing codebase structure...",
  "timestamp": "2026-03-09T10:00:01Z"
}
```

#### `scout_completed`

Scout agent finished IMPL doc generation.

```json
{
  "session_id": "abc123",
  "impl_path": "/path/to/repo/docs/IMPL/IMPL-add-user-auth.md",
  "status": "complete",
  "verdict": "SUITABLE",
  "wave_count": 2,
  "agent_count": 5,
  "timestamp": "2026-03-09T10:02:30Z"
}
```

---

### Wave Events

#### `wave_started`

Wave execution begins (all agents launching).

```json
{
  "session_id": "abc123",
  "impl_path": "/path/to/repo/docs/IMPL/IMPL-feature.md",
  "wave_number": 1,
  "agents": ["A", "B", "C"],
  "timestamp": "2026-03-09T10:05:00Z"
}
```

#### `wave_completed`

Wave execution finished (all agents completed, quality gates passed).

```json
{
  "session_id": "abc123",
  "wave_number": 1,
  "status": "complete",
  "completed_agents": ["A", "B", "C"],
  "timestamp": "2026-03-09T10:15:00Z"
}
```

#### `wave_merged`

Wave branches merged to main.

```json
{
  "session_id": "abc123",
  "wave_number": 1,
  "status": "merged",
  "timestamp": "2026-03-09T10:16:00Z"
}
```

---

### Agent Events

#### `agent_started`

Agent execution begins in its worktree.

```json
{
  "session_id": "abc123",
  "wave_number": 1,
  "agent_id": "A",
  "worktree_path": ".claude/worktrees/wave1-agent-A",
  "branch": "wave1-agent-A",
  "timestamp": "2026-03-09T10:05:01Z"
}
```

#### `agent_output`

Agent text output (streamed chunks during execution).

```json
{
  "session_id": "abc123",
  "wave_number": 1,
  "agent_id": "A",
  "chunk": "Reading src/auth.go...",
  "timestamp": "2026-03-09T10:05:02Z"
}
```

#### `agent_tool_call`

Agent invokes a tool (Read, Write, Edit, Bash, etc.).

```json
{
  "session_id": "abc123",
  "wave_number": 1,
  "agent_id": "A",
  "tool_name": "Read",
  "tool_input": {
    "path": "src/auth.go"
  },
  "timestamp": "2026-03-09T10:05:03Z"
}
```

> **Note:** Tool call events are published by the Agent Observatory (as of v0.9.0). Requires middleware integration (see ROADMAP.md Tool System Refactoring).

#### `agent_tool_result`

Agent receives tool execution result.

```json
{
  "session_id": "abc123",
  "wave_number": 1,
  "agent_id": "A",
  "tool_name": "Read",
  "tool_result": "package auth\n\nfunc Authenticate(...) { ... }",
  "duration_ms": 15,
  "timestamp": "2026-03-09T10:05:03Z"
}
```

#### `agent_completed`

Agent finishes execution and writes completion report.

```json
{
  "session_id": "abc123",
  "wave_number": 1,
  "agent_id": "A",
  "status": "complete",
  "files_modified": ["src/auth.go", "src/middleware.go"],
  "timestamp": "2026-03-09T10:10:00Z"
}
```

**Status values:**
- `complete` — Agent finished successfully
- `partial` — Agent completed part of the work (see `failure_type`)
- `blocked` — Agent cannot proceed (see `failure_type`)

#### `agent_blocked`

Agent failed with failure type routing decision.

```json
{
  "session_id": "abc123",
  "wave_number": 1,
  "agent_id": "B",
  "status": "blocked",
  "failure_type": "fixable",
  "action": "retry",
  "reason": "Missing dependency: crypto package not installed",
  "timestamp": "2026-03-09T10:12:00Z"
}
```

**Failure types (from E19):**
- `transient` — Temporary failure (network timeout, API rate limit) → action: `retry`
- `fixable` — Agent can fix the issue itself → action: `relaunch`
- `needs_replan` — Architecture issue, Scout must replan → action: `escalate_scout`
- `escalate` — Human intervention required → action: `escalate_human`
- `timeout` — Agent exceeded max turns → action: `escalate_human`

**Actions:**
- `retry` — Re-run the same agent immediately
- `relaunch` — Launch a new agent to fix the issue
- `escalate_scout` — Re-run Scout to replan
- `escalate_human` — Notify user, block wave

---

### Verification Events

#### `verification_started`

Verification suite begins (build, tests, invariants).

```json
{
  "session_id": "abc123",
  "impl_path": "/path/to/repo/docs/IMPL/IMPL-feature.md",
  "timestamp": "2026-03-09T10:16:30Z"
}
```

#### `verification_gate`

A verification gate completes (build, test, invariant check).

```json
{
  "session_id": "abc123",
  "gate": "build",
  "status": "passed",
  "timestamp": "2026-03-09T10:16:45Z"
}
```

**Failure example:**

```json
{
  "session_id": "abc123",
  "gate": "tests",
  "status": "failed",
  "output": "FAIL: TestAuthenticate (0.01s)\n    auth_test.go:42: expected nil, got error",
  "timestamp": "2026-03-09T10:17:00Z"
}
```

**Gate types:**
- `build` — Compilation succeeds
- `tests` — Test suite passes
- `invariants` — Protocol invariants (I1–I6) hold
- `quality_gates` — Custom gates from IMPL doc `## Quality Gates` section

#### `verification_completed`

Verification suite finished.

```json
{
  "session_id": "abc123",
  "status": "passed",
  "timestamp": "2026-03-09T10:17:30Z"
}
```

---

### Quality Gate Events

#### `quality_gate_started`

Custom quality gate begins execution.

```json
{
  "session_id": "abc123",
  "gate_name": "lint",
  "command": "golangci-lint run",
  "timestamp": "2026-03-09T10:17:00Z"
}
```

#### `quality_gate_completed`

Custom quality gate finishes.

```json
{
  "session_id": "abc123",
  "gate_name": "lint",
  "status": "passed",
  "duration_ms": 2300,
  "timestamp": "2026-03-09T10:17:02Z"
}
```

**Failure example:**

```json
{
  "session_id": "abc123",
  "gate_name": "security_scan",
  "status": "failed",
  "output": "gosec: G104: Unchecked error return in auth.go:45",
  "duration_ms": 1500,
  "timestamp": "2026-03-09T10:17:04Z"
}
```

---

## Event Lifecycle Example

A complete Scout + Wave 1 run produces this event sequence:

```
scout_started
scout_output (multiple chunks)
scout_completed
↓
wave_started (wave=1)
  agent_started (agent=A)
  agent_started (agent=B)
  agent_started (agent=C)
  ↓ (parallel)
  agent_output (agent=A, multiple chunks)
  agent_tool_call (agent=A, tool=Read)
  agent_tool_result (agent=A, tool=Read)
  agent_tool_call (agent=A, tool=Edit)
  agent_tool_result (agent=A, tool=Edit)
  agent_completed (agent=A, status=complete)
  ↓
  agent_output (agent=B, multiple chunks)
  agent_tool_call (agent=B, tool=Write)
  agent_tool_result (agent=B, tool=Write)
  agent_completed (agent=B, status=complete)
  ↓
  agent_output (agent=C, multiple chunks)
  agent_blocked (agent=C, failure_type=fixable, action=relaunch)
  ↓
wave_completed (wave=1, status=complete)
quality_gate_started (gate=build)
quality_gate_completed (gate=build, status=passed)
wave_merged (wave=1)
verification_started
verification_gate (gate=build, status=passed)
verification_gate (gate=tests, status=passed)
verification_gate (gate=invariants, status=passed)
verification_completed (status=passed)
```

---

## SSE Broker Implementation

**Engine:** `pkg/orchestrator/sse.go` provides the `SSEBroker` type.

**Web Server:** `scout-and-wave-web/pkg/api/events_handler.go` wraps the broker and serves `/api/events`.

**Publishing:**

```go
broker.Publish(orchestrator.Event{
    Type: "agent_completed",
    Data: map[string]interface{}{
        "session_id":  sessionID,
        "wave_number": 1,
        "agent_id":    "A",
        "status":      "complete",
    },
})
```

**Subscribing (web client):**

```javascript
const eventSource = new EventSource('/api/events?session_id=abc123')

eventSource.addEventListener('agent_output', (e) => {
  const data = JSON.parse(e.data)
  console.log(`Agent ${data.agent_id}: ${data.chunk}`)
})

eventSource.addEventListener('agent_completed', (e) => {
  const data = JSON.parse(e.data)
  console.log(`Agent ${data.agent_id} completed with status ${data.status}`)
})
```

---

## Future Enhancements

- **Event replay** — Use `Last-Event-ID` header to resume from a specific event
- **Event filtering** — Filter by wave number, agent ID, event type
- **Retention** — Archive events to disk for post-run analysis
- **Metrics aggregation** — Total tool calls, average agent duration, token usage per wave
