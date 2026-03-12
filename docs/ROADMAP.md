# Scout-and-Wave Engine Roadmap

This is the engine/SDK roadmap. See also:
- **scout-and-wave-web** roadmap for UI/UX features
- **scout-and-wave** (protocol repo) roadmap for protocol enhancements

**Current version:** v0.34.0 (markdown system removal complete, base commit tracking, duplicate report detection)

---

## Phase 1: Self-Healing & Observability

### v0.35.0 — Verification Loop with Auto-Retry (E24)

**Why:** Quality gates currently report failures but don't trigger recovery. Automated retry with context preservation eliminates manual debugging cycles.

**Scope:**
1. Add `E24: Verification Loop` to protocol execution rules
2. After wave merge, run quality gates via `orchestrator.RunQualityGates()`
3. On failure:
   - Create retry IMPL doc: `{feature-slug}-fix-wave{N}.yaml`
   - Include failure output, blocked tasks, safe point SHA
   - Single wave with 1 agent assigned to failed files
   - Limit to 2 retry waves; 3rd failure → `blocked` state
4. Store retry chain metadata in manifest: `parent_wave_sha`, `retry_count`

**Implementation:**
- `pkg/engine/verifier.go` — new file for verification loop logic
- `pkg/engine/runner.go` — call verifier after merge
- `pkg/protocol/manifest.go` — add `RetryMetadata` struct
- `pkg/orchestrator/retry.go` — retry IMPL doc generation

**Effort:** 3-4 days
**Value:** Eliminates 80% of manual quality gate failure debugging

---

### v0.36.0 — Wave Timeout Enforcement

**Why:** Hung agents block waves indefinitely. Automatic timeout + recovery prevents gridlock.

**Scope:**
1. Add `agent_timeout_minutes: 30` to IMPL manifest (per-wave, optional, defaults to 30)
2. In `orchestrator.RunWave()`, wrap each agent in `context.WithTimeout()`
3. On timeout:
   - Kill agent process (via context cancellation)
   - Write completion report: `status: timeout`, `error: "Agent exceeded {N} minutes"`
   - Continue wave (don't block other agents)
4. Quality gates detect missing implementations from timed-out agents

**Implementation:**
- `pkg/orchestrator/orchestrator.go` — add timeout context wrapper
- `pkg/protocol/manifest.go` — add `AgentTimeoutMinutes` field

**Effort:** 2-3 days
**Value:** Prevents wave gridlock, enables unattended execution

---

## Phase 2: Intelligence & Learning

### v0.37.0 — Persistent Memory System

**Why:** Agents repeat mistakes across waves (e.g., forgetting cross-repo dependencies, missing common pitfalls). Memory system learns patterns from completion reports and injects them into future Scout/Wave agents.

**Scope:**
1. Add `docs/MEMORY.md` to protocol structure (peer to `CONTEXT.md`)
2. After wave completes, run reflection agent:
   - Prompt: "Analyze completion reports from Wave {N}. Extract patterns, pitfalls, preferences."
   - Output: YAML entries with `type` (pattern/pitfall/preference), `content`, `relevance_tags`, `source_wave`
3. Append entries to `docs/MEMORY.md` (chronological order)
4. Before Scout runs, score memories by tag overlap with feature description
5. Prepend top-5 relevant memories to Scout prompt as `## Past Experience` section

**Implementation:**
- `pkg/memory/reflector.go` — reflection agent runner
- `pkg/memory/scorer.go` — relevance scoring (tag overlap + recency decay)
- `pkg/memory/store.go` — CRUD for MEMORY.md
- `pkg/engine/runner.go` — inject memories into Scout prompt

**Effort:** 5-6 days
**Value:** Eliminates repetitive mistakes, compounds learning across features

---

### v0.38.0 — Agent Progress Tracking API

**Why:** Wave execution is a black box. SSE events contain file writes, command executions, and tool calls but aren't structured for progress tracking.

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

## Phase 3: Multi-Provider & Scale

### v0.39.0 — Multi-Provider Backends

**Why:** Claude-only execution limits deployment scenarios (air-gapped, cost optimization, alternative reasoning models).

**Providers:**
- OpenAI (GPT-4o, o3, o4-mini)
- LiteLLM (100+ models via proxy)
- Ollama (local inference, air-gapped)
- Google Gemini (Vertex AI / AI Studio)
- Kimi (Moonshot AI)
- Any OpenAI-compatible endpoint

**Interface:**
- `--backend claude|openai|litellm|ollama|gemini|kimi` flag
- Auto-detection from env vars
- Per-agent model override (Scout on Opus, Wave on Haiku)

**Implementation:**
- `pkg/agent/backend/` — provider abstraction layer
- `pkg/agent/backend/normalize.go` — tool-use format translation
- `pkg/agent/backend/retry.go` — provider-specific backoff

**Effort:** 1-2 weeks
**Value:** Unlocks air-gapped deployments, reduces costs by 60-80%

---

### v0.40.0 — Lease Visualization (Informational)

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

## Phase 4: Production Hardening

### v0.45.0 — Observability Stack

- OpenTelemetry tracing (spans per wave/agent/tool call)
- Structured logging (`slog` with context propagation)
- Cost tracking per IMPL doc (token usage, model costs)
- Prometheus metrics endpoint

### v0.46.0 — Sandboxed Execution

- Agents run in isolated containers (Docker/Podman)
- Filesystem restrictions (read-only except worktree)
- Network policies (block outbound except API)
- Resource limits (CPU/memory per agent)

---

## Current Focus

**Now:** v0.35.0 — Verification Loop (E24 implementation)

**Next:** v0.36.0 — Wave Timeout Enforcement
