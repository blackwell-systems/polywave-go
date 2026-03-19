# Scout Seed: Autonomy Layer

## Feature Summary

Add a configurable autonomy layer to the SAW orchestrator. Three competitive analyses (plan-cascade, Formic, Maestro) consistently identified the same gap: SAW has best-in-class correctness guarantees (I1–I6) but requires manual orchestration at every step. Competitors auto-advance, auto-remediate, and auto-queue — SAW should too, without compromising its structural safety.

**Key design principle: autonomy is a dial, not a switch.** Users configure how much human oversight they want via an `autonomy_level` setting. Every feature in this IMPL respects that setting — the same engine code powers both fully-gated and fully-autonomous flows.

## Target Repos

- **Primary:** scout-and-wave-go (engine/orchestrator, CLI)
- **Secondary:** scout-and-wave-web (API endpoints, SSE events, UI)

## R0: Autonomy Level Configuration

All autonomy features are gated by an `autonomy_level` setting in `saw.config.json`:

```json
{
  "autonomy": {
    "level": "supervised",
    "max_auto_retries": 2,
    "max_queue_depth": 10
  }
}
```

**Three levels:**

| Level | IMPL Review | Wave Advance | Gate Failure | Queue Advance | Use Case |
|-------|------------|-------------|-------------|--------------|----------|
| `gated` | Human approves | Human approves | Human fixes | Manual scout | Current behavior. Nothing changes. Default. |
| `supervised` | Human approves | Auto (with notification) | Auto-fix up to N retries, then pause | Auto-scout next, human approves IMPL | Trust the engine for execution, keep human for planning decisions. |
| `autonomous` | Auto-approve | Auto | Auto-fix up to N retries, then pause | Auto-scout + auto-approve | Full unattended. Only pauses on retry exhaustion. |

**Per-IMPL override:** Queue items can override the global level:
```yaml
# queue/001-dark-mode.yaml
autonomy_override: "gated"   # force human review even in autonomous mode
```

**Implementation:**
- New `pkg/autonomy/` package with `Level` type and `Config` struct
- `autonomy.ShouldAutoApprove(level, stage)` — returns bool for each decision point
- All R1–R4 features call this function instead of hardcoding behavior
- CLI flag: `sawtools daemon --autonomy supervised`
- Web UI: dropdown in settings panel (maps to `saw.config.json`)
- `/saw` skill: `--autonomy` flag on wave/scout commands

**The `gated` level reproduces exactly today's behavior.** No existing workflow changes unless the user opts in.

## Requirements

### R1: Auto-Remediation on Gate Failure

When `finalize-wave` fails at a quality gate (E21) or verify-build step, the orchestrator should automatically attempt to fix the failure instead of stopping and waiting for human intervention.

**Current behavior:** Gate failure → `failure_type: "execute"` → wave goes BLOCKED → human must intervene.

**Desired behavior:** Gate failure → orchestrator creates a retry/fix agent with structured error context → agent fixes the issue → orchestrator re-runs the gate → if fixed, continue; if still failing after N retries (default: 2), escalate to human.

**Existing plumbing that already works:**
- `retryctx.BuildRetryContext()` — classifies errors into 6 categories with per-class fix suggestions
- `retryctx.ClassifyError()` — pattern-matches error output
- `builddiag.DiagnoseError()` — H7 pattern matching for build failures
- `retry.GenerateRetryIMPL()` — creates single-agent retry manifests
- `engine.FinalizeWave()` already returns `BuildDiagnosis` with actionable error info

**What's missing:** The orchestrator doesn't automatically invoke these. The web app's `handleWaveAgentRerun` uses `retryctx.BuildRetryContext` but only on manual rerun button click. The engine needs an `AutoRemediate()` function that:
1. Takes a `FinalizeWaveResult` with `BuildPassed=false`
2. Extracts error context via `retryctx.BuildRetryContext`
3. Launches a fix agent (single-agent wave targeting the failed files)
4. Re-runs `finalize-wave` after the fix agent completes
5. Tracks retry count per wave (max 2 auto-retries before escalation)

### R2: IMPL Queue with Auto-Advance

After an IMPL completes (`mark-complete`), the orchestrator should check for queued IMPLs and auto-trigger the next one.

**Current behavior:** IMPL completes → archived to `docs/IMPL/complete/` → nothing. User must manually run `/saw scout` for the next feature.

**Desired behavior:** IMPL completes → orchestrator checks `docs/IMPL/queue/` (new directory) → if queued IMPLs exist, auto-triggers the next one based on priority and dependency ordering.

**Queue schema:** Each queued item is a simple YAML file in `docs/IMPL/queue/`:
```yaml
# queue/001-dark-mode.yaml
title: "Add dark mode support"
priority: 1          # lower = higher priority
depends_on: []       # slugs of IMPLs that must complete first
feature_description: |
  Add dark mode toggle with system preference detection,
  CSS variable theming, and localStorage persistence.
status: "queued"     # queued | in_progress | complete | blocked
```

**Operations needed:**
- `sawtools queue-add <title> --priority N --description "..."` — add to queue
- `sawtools queue-list` — show queue with status and ordering
- `sawtools queue-next` — return the next eligible item (all deps met, highest priority)
- Engine function `CheckQueue()` called after `MarkIMPLComplete()` — auto-triggers Scout for next item

**Web app integration:**
- `GET /api/queue` — list queued items
- `POST /api/queue` — add item
- SSE event `queue_advanced` when auto-triggering next IMPL
- UI: queue panel in sidebar showing upcoming work

### R3: Closed-Loop Gate Verification

Extend quality gates (E21) to support automatic retry with agent feedback, not just binary pass/fail.

**Current behavior:** Gate fails → wave fails → human reviews error → manually fixes or reruns agent.

**Desired behavior:** Gate fails → gate error output sent to the responsible agent(s) as context → agent fixes → gate re-runs → pass or escalate after N retries.

**This differs from R1:** R1 handles post-merge build failures (whole-wave scope). R3 handles pre-merge gate failures on individual agents (per-agent scope, agents still in worktrees).

**Implementation:**
- New gate mode: `retry_on_fail: true` (default: false for backward compat)
- When a retryable gate fails, `finalize-wave` pauses before merge
- Engine creates a fix prompt with gate output and sends it to the relevant agent
- Agent fixes in its worktree, commits, and the gate re-runs
- Max retries configurable per gate (default: 2)

### R4: Orchestrator Daemon Mode (Engine-Level)

Add an engine-level run loop that processes IMPL queue items and auto-advances through waves without interactive CLI input.

**Current behavior:** CLI orchestrator is interactive — requires human at keyboard for each `/saw scout`, `/saw wave`, review gates.

**Desired behavior:** `sawtools daemon --repo-dir <path>` runs a persistent loop:
1. Check queue for next eligible IMPL
2. Run Scout → validate → auto-approve (or pause if `require_review: true`)
3. Run waves with `--auto` semantics
4. On failure, auto-remediate (R1/R3) up to retry limits
5. On success, mark complete, advance queue (R2)
6. Sleep/poll for new queue items

**This is the web app's `startWave` flow generalized to the engine level.** The web app already does most of this — daemon mode extracts it into a standalone CLI command that doesn't need a browser.

**Safety rails:**
- `require_review: true` per queue item pauses for human approval after Scout
- Max concurrent IMPLs: 1 (sequential by default, configurable)
- Auto-remediation retry limits (R1: 2 retries, R3: 2 retries per gate)
- Failure escalation: after retry exhaustion, daemon pauses and emits notification
- All existing invariants (I1–I6) and execution rules (E1–E26) remain enforced

### R5: Pipeline Visualization

The current web UI is built around single-IMPL-at-a-time workflows. With multi-IMPL queue processing (R2+R4), the UI needs a new top-level view showing all IMPLs across the pipeline.

**Current UI hierarchy:**
1. Sidebar: IMPL list → click one → ReviewScreen or WaveBoard
2. No concept of queue order, throughput, or cross-IMPL status

**Desired UI hierarchy (three layers):**
1. **Pipeline view** (NEW top-level) — all IMPLs across the queue lifecycle
2. **IMPL detail view** (existing ReviewScreen) — drill into a specific feature's waves/agents/contracts
3. **Wave execution view** (existing WaveBoard) — live agent cards during wave execution

**Pipeline view shows:**
- IMPLs in execution order: completed → currently executing → queued
- Per-IMPL status: complete (with timestamp), executing (with wave N/M + active agent), blocked (with reason), queued (with position + dependencies)
- Currently executing IMPL gets a live mini-view: wave progress bar, active agent count, elapsed time
- Blocked IMPLs show the blocking reason and action button (Review / Retry / Escalate)
- Queue items show position, priority, and dependency links (grayed out if deps not met)
- Throughput metrics: IMPLs/hr, avg wave time, queue depth, blocked count

**Pipeline view mockup:**
```
┌─────────────────────────────────────────────────────────┐
│  SAW Pipeline                    ⚙ supervised │ 2/hr   │
├─────────────────────────────────────────────────────────┤
│  ✅ dark-mode          3 waves  2m ago    [View]        │
│  ✅ user-profiles      2 waves  8m ago    [View]        │
│  🔄 notifications      Wave 2/3  Agent B  [Live]        │
│  ⏸️  api-refactor       Gate fail (retry 1/2) [Review]   │
│  ⏳ search-feature     Queued #1                        │
│  ⏳ caching-layer      Queued #2                        │
│  ⏳ admin-panel        Queued #3 (needs: api-refactor)  │
│                                                         │
│  Throughput: 2/hr │ Queue: 3 │ Blocked: 1 │ Done: 2    │
└─────────────────────────────────────────────────────────┘
```

Clicking any row drills into the existing IMPL detail / wave board views.

**API endpoints needed:**
- `GET /api/pipeline` — combined view: completed IMPLs + executing IMPL + queue items, with status and metrics
- SSE events: `pipeline_updated` (fires on any status change across the pipeline)

**Frontend components:**
- `PipelineView.tsx` — new top-level page component
- `PipelineRow.tsx` — per-IMPL row with status indicator, progress, and action buttons
- `PipelineMiniWave.tsx` — inline mini-view of the currently executing wave (compact agent status dots + progress bar)
- `PipelineMetrics.tsx` — throughput bar at the bottom

**Navigation:**
- New route: `/pipeline` as the default landing page when autonomy level is `supervised` or `autonomous`
- Existing `/impl/{slug}` routes remain for detail views
- Pipeline view replaces the sidebar IMPL list as the primary navigation for multi-IMPL workflows
- When autonomy level is `gated`, default landing page stays as the current IMPL list (no behavioral change)

**Data sources:**
- Completed: `sawtools list-impls --dir docs/IMPL/complete` (already exists)
- Executing: current wave state from `useWaveEvents` (already exists)
- Queued: `GET /api/queue` (from R2)
- Metrics: derived client-side from timestamps on completed IMPLs

## Competitive Context

| Feature | SAW (current) | SAW (with autonomy) | Formic | plan-cascade | Maestro |
|---------|--------------|--------------------|---------| -------------|---------|
| Auto-fix on failure | Manual | Auto (R1+R3) | Auto | Manual | None |
| IMPL/task queue | None | Auto-advance (R2) | Auto | None | None |
| Daemon/unattended | No | Yes (R4) | Yes | No | No |
| Pipeline visibility | IMPL list | Pipeline view (R5) | Kanban | None | Session grid |
| Autonomy config | None | 3 levels (R0) | Always auto | None | None |
| Correctness guarantees | I1–I6 | I1–I6 (unchanged) | Optimistic | None | None |

## Architectural Constraints

- **Protocol invariants are untouched.** I1–I6 remain as-is. Auto-remediation operates within the existing state machine.
- **Execution rules extended, not changed.** R1 adds E27 (auto-remediation). R2 adds E28 (queue advance). R3 extends E21 (retryable gates). R4 is orchestrator-only (no new E-rules).
- **Backward compatible.** All new behavior is opt-in. Existing `finalize-wave`, `mark-complete`, and gate commands work identically. Daemon mode is a new command, not a change to existing ones.
- **Web app gets it for free.** R1–R3 are engine-level functions. The web app's `handleWaveFinalize` and `handleWaveMerge` call `engine.FinalizeWave()` — auto-remediation in the engine means the web app auto-remediates too.

## Scope Notes for Scout

This is a large feature. The Scout should assess whether it's suitable for SAW execution (it should be — the components are cleanly separable) and consider multi-wave decomposition:

- **Wave 1:** R0 (autonomy config) + R1 (auto-remediation) + R3 (closed-loop gates) — R0 is the foundation everything checks; R1+R3 are closely related and share retry logic
- **Wave 2:** R2 (IMPL queue) — independent data model and CLI commands
- **Wave 3:** R4 (daemon mode) + R5 (pipeline viz) — R4 depends on R1+R2+R3; R5 is the frontend for R2+R4

The Scout may restructure this differently based on dependency analysis. The seed is guidance, not prescription.
