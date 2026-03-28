# Scout-and-Wave Engine Roadmap

> Last audited: 2026-03-28. Completed items removed; see CHANGELOG.md for shipped features.

This is the engine/SDK roadmap. See also:
- **scout-and-wave-web** roadmap for UI/UX features
- **scout-and-wave** (protocol repo) roadmap for protocol enhancements

**Current version:** v1.2.3

---

## Phase 1: Observability

### Agent Progress Tracking API

**Why:** Wave execution is a black box. SSE events contain file writes, command executions, and tool calls but aren't structured for progress tracking. The Agent Observatory (v0.9.0) provides raw tool call visibility, but not structured progress summaries.

**Scope:**
1. Parse agent SSE stream for:
   - File write events: `writing to {path}`
   - Command execution: `running: {cmd}`
   - Tool calls: `calling tool: {name}`
2. Emit structured SSE events: `agent_progress` with `{agent: "A", current_file: "api.go", current_action: "write"}`
3. Add `/api/wave/{slug}/status` endpoint returning per-agent progress summary
4. Track commit count vs expected file count (progress percentage)

**Implementation:**
- `pkg/api/sse.go` — add progress event emitter
- `pkg/orchestrator/progress.go` — new file for progress tracking
- `pkg/api/wave.go` — add status endpoint

**Effort:** 2-3 days
**Value:** Makes wave execution observable in real-time

---

### Lease Visualization (Informational)

**Why:** Worktree isolation prevents conflicts, but file contention is invisible. Showing which agents work on which files aids debugging.

**Scope:**
1. Track per-agent file activity: `{agentID: {files: []string, status: string}}`
2. Parse agent logs for file write events
3. Emit SSE event: `agent_file_activity` with `{agent: "A", file: "api.go", action: "write"}`
4. **Informational only** — no lease enforcement (worktree isolation is the enforcement)

**Implementation:**
- `pkg/orchestrator/activity.go` — file activity tracker
- `pkg/api/sse.go` — file activity event emitter

**Effort:** 2-3 days
**Value:** Makes file contention visible during execution

---

## Phase 2: Intelligence & Learning

### Advanced Memory System (Reflection Agent)

**Why:** Basic project memory (E17/E18 context read/write) is shipped. Agents still repeat mistakes across features because there is no automated pattern extraction from completion reports.

**Scope:**
1. After wave completes, run reflection agent:
   - Prompt: "Analyze completion reports from Wave {N}. Extract patterns, pitfalls, preferences."
   - Output: YAML entries with `type` (pattern/pitfall/preference), `content`, `relevance_tags`, `source_wave`
2. Append entries to project memory (chronological order)
3. Before Scout runs, score memories by tag overlap with feature description
4. Prepend top-5 relevant memories to Scout prompt as `## Past Experience` section

**Implementation:**
- `pkg/memory/reflector.go` — reflection agent runner
- `pkg/memory/scorer.go` — relevance scoring (tag overlap + recency decay)
- `pkg/memory/store.go` — CRUD for memory entries
- `pkg/engine/runner.go` — inject scored memories into Scout prompt

**Effort:** 5-6 days
**Value:** Eliminates repetitive mistakes, compounds learning across features

---

## Phase 3: Production Hardening

### Observability Stack

- OpenTelemetry tracing (spans per wave/agent/tool call)
- Structured logging (`slog` with context propagation)
- Cost tracking per IMPL doc (token usage, model costs)
- Prometheus metrics endpoint

### Sandboxed Execution

- Agents run in isolated containers (Docker/Podman)
- Filesystem restrictions (read-only except worktree)
- Network policies (block outbound except API)
- Resource limits (CPU/memory per agent)

---

## Current Focus

**Now:** Agent Progress Tracking API (observability)

**Next:** Advanced Memory System (reflection agent)
