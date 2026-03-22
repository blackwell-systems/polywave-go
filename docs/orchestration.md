# Orchestration

The orchestrator (`pkg/orchestrator`) drives SAW protocol wave execution: it
advances an 11-state machine, creates per-agent git worktrees, launches agents
concurrently via pluggable backends, merges completed worktrees, runs
post-merge verification, and updates the IMPL doc status table.

The engine layer (`pkg/engine`) wraps the orchestrator with higher-level
entrypoints (Scout, Wave, Merge, Finalize, Chat, Daemon) and provides the
program-level tier loop for multi-IMPL orchestration.

> **Migration note (2026-03):** The orchestrator is being migrated from
> `types.IMPLDoc` to `protocol.IMPLManifest` (type-unification IMPL). The
> target state uses `protocol.IMPLManifest` as the canonical type.
> During migration, the engine layer bridges the two via `manifestToIMPLDoc()`
> in `pkg/engine/engine.go`. The orchestrator still holds `*types.IMPLDoc`
> internally; this will be replaced with `*protocol.IMPLManifest` once all
> callers are migrated. Document below describes the target state.

---

## Table of Contents

1. [State Machine](#state-machine)
2. [Orchestrator Lifecycle](#orchestrator-lifecycle)
3. [Set\*Func Injection Pattern](#setfunc-injection-pattern)
4. [Event System](#event-system)
5. [Agent Launch Flow](#agent-launch-flow)
6. [Multi-Backend Agent Support](#multi-backend-agent-support)
7. [Failure Handling (E19)](#failure-handling-e19)
8. [Merge](#merge)
9. [Verification and Quality Gates](#verification-and-quality-gates)
10. [FinalizeWave Pipeline](#finalizewave-pipeline)
11. [Auto-Remediation](#auto-remediation)
12. [Closed-Loop Gate Retry (R3)](#closed-loop-gate-retry-r3)
13. [Context Memory (E18)](#context-memory-e18)
14. [Journal Integration](#journal-integration)
15. [Pipeline Package](#pipeline-package)
16. [Queue / Daemon System](#queue--daemon-system)
17. [Program-Level Orchestration (RunTierLoop)](#program-level-orchestration-runtierloop)
18. [Autonomy Levels](#autonomy-levels)
19. [Agent Scheduling](#agent-scheduling)
20. [Cross-Repo Orchestration](#cross-repo-orchestration)
21. [See Also](#see-also)

---

## State Machine

The orchestrator tracks an 11-state protocol state machine defined in
`pkg/protocol`. All mutations go through `Orchestrator.TransitionTo()`, which
validates the transition against `validTransitions` in `transitions.go`.

```
ScoutPending ──→ ScoutValidating ──→ Reviewed
     │                                   │
     ├──→ Reviewed                        ├──→ ScaffoldPending ──→ WavePending
     ├──→ NotSuitable                     └──→ WavePending
     └──→ Blocked                                  │
                                                   ▼
                                            WaveExecuting
                                                   │
                                                   ▼
                                             WaveMerging
                                                   │
                                                   ▼
                                            WaveVerified
                                              │       │
                                              ▼       ▼
                                          Complete  WavePending (next wave)

    Blocked ──→ (any state)     // recovery from any blocked state
    NotSuitable ──→ (terminal)
    Complete ──→ (terminal)
```

States: `ScoutPending`, `ScoutValidating`, `Reviewed`, `ScaffoldPending`,
`WavePending`, `WaveExecuting`, `WaveMerging`, `WaveVerified`, `Blocked`,
`NotSuitable`, `Complete`.

---

## Orchestrator Lifecycle

```
New(repoPath, implDocPath)          // parse IMPL, initial state = ScoutPending
  │
  ├─ SetEventPublisher(pub)         // inject SSE event sink
  ├─ SetDefaultModel(model)         // fallback model for wave agents
  ├─ SetWorktreePaths(paths)        // pre-computed multi-repo worktree paths
  │
  ▼
RunWave(waveNum)                    // launch all agents concurrently
  │
  ├─ ValidateInvariants (I1)
  ├─ PrioritizeAgents (critical path scheduling)
  ├─ For each agent (errgroup):
  │    └─ launchAgent() or launchAgentStructured()
  │         ├─ Create/reuse worktree
  │         ├─ Extract E23 context payload
  │         ├─ ExecuteStreamingWithTools()
  │         ├─ Poll/synthesize completion report
  │         ├─ Persist report to manifest
  │         └─ E19 failure routing → executeRetryLoop() if applicable
  │
  └─ Publish "wave_complete" event
  │
  ▼
MergeWave(waveNum)                  // merge all agent branches to main
  │
  ▼
RunVerification(testCommand)        // go vet + test command
  │
  ▼
UpdateIMPLStatus(waveNum)           // tick status checkboxes for complete agents
```

### Constructor

```go
orch, err := orchestrator.New(repoPath, implDocPath)
```

`New` loads the IMPL doc via `parseIMPLDocFunc` (injected by the engine layer)
and initializes the state to `ScoutPending`. The constructor takes two string
arguments (not a config struct):

- `repoPath` -- absolute path to the repository root
- `implDocPath` -- absolute path to the IMPL YAML manifest

---

## Set\*Func Injection Pattern

The orchestrator avoids circular imports between `pkg/orchestrator`,
`pkg/protocol`, and `pkg/engine` by using **package-level function variables**
with `Set*Func` injection. Each function variable has a no-op or stub default
so the orchestrator compiles independently.

| Function Variable | Injected By | Purpose |
|---|---|---|
| `parseIMPLDocFunc` | `engine.init()` via `SetParseIMPLDocFunc` | Load IMPL manifest → `types.IMPLDoc` |
| `validateInvariantsFunc` | `orchestrator.init()` | I1 disjoint file ownership check |
| `prioritizeAgentsFunc` | `engine.init()` via `SetPrioritizeAgentsFunc` | Critical-path agent scheduling |
| `runWaveAgentStructuredFunc` | `engine.init()` via `SetRunWaveAgentStructuredFunc` | Structured output agent execution |
| `mergeWaveFunc` | `merge.go init()` | Merge implementation |
| `runVerificationFunc` | `verification.go init()` | Post-merge verification |
| `newBackendFunc` | Package-level default | Backend construction (seam for tests) |
| `newRunnerFunc` | Package-level default | Agent runner construction (seam for tests) |
| `worktreeCreatorFunc` | Package-level default | Worktree creation (seam for tests) |
| `waitForCompletionFunc` | Package-level default | Completion report polling (seam for tests) |

**Why this pattern exists:** The orchestrator needs protocol parsing and
engine-level functions, but importing those packages would create a cycle
(`orchestrator -> engine -> orchestrator`). The function-variable seam breaks
the cycle: the orchestrator declares variables with stub defaults, and the
engine/protocol packages inject real implementations in their `init()`
functions.

---

## Event System

The orchestrator publishes events via `OrchestratorEvent` and an
`EventPublisher` callback. The API layer maps these to SSE events without the
orchestrator importing `pkg/api`.

```go
type OrchestratorEvent struct {
    Event string      // event name (e.g. "agent_started")
    Data  interface{} // typed payload struct
}

type EventPublisher func(ev OrchestratorEvent)
```

### Wave-Level Events

| Event | Payload Type | When |
|---|---|---|
| `agent_prioritized` | `AgentPrioritizedPayload` | Before agent launch; shows reordering |
| `agent_started` | `AgentStartedPayload` | After worktree ready, before execution |
| `agent_output` | `AgentOutputPayload` | Per text chunk during agent execution |
| `agent_tool_call` | `AgentToolCallPayload` | Per tool invocation and result |
| `agent_complete` | `AgentCompletePayload` | Agent finished (status: complete/partial/blocked) |
| `agent_failed` | `AgentFailedPayload` | Agent execution error |
| `agent_blocked` | `AgentBlockedPayload` | E19 failure routing triggered |
| `auto_retry_started` | `AutoRetryStartedPayload` | E19 automatic retry begins |
| `auto_retry_exhausted` | `AutoRetryExhaustedPayload` | All retry attempts used |
| `wave_complete` | `WaveCompletePayload` | All agents in wave finished |
| `run_complete` | `RunCompletePayload` | Full run finished |

### Program-Level Events (E40)

| Event | Payload Type |
|---|---|
| `program_tier_started` | `ProgramTierStartedPayload` |
| `program_scout_launched` | `ProgramScoutLaunchedPayload` |
| `program_scout_complete` | `ProgramScoutCompletePayload` |
| `program_impl_complete` | `ProgramIMPLCompletePayload` |
| `program_tier_gate_started` | `ProgramTierGateStartedPayload` |
| `program_tier_gate_result` | `ProgramTierGateResultPayload` |
| `program_contracts_frozen` | `ProgramContractsFrozenPayload` |
| `program_tier_advanced` | `ProgramTierAdvancedPayload` |
| `program_replan_triggered` | `ProgramReplanTriggeredPayload` |
| `program_complete` | `ProgramCompletePayload` |

---

## Agent Launch Flow

`launchAgent()` in `orchestrator.go` executes a single agent. Called from
`RunWave()` inside an errgroup goroutine for each agent.

### Step-by-step

1. **Resolve worktree path.** If `SetWorktreePaths()` provided a pre-computed
   path (multi-repo), use it. Otherwise, create on demand via
   `worktreeCreatorFunc`. Reuses existing worktrees if present.

2. **Capture base SHA.** Records `HEAD` before the agent runs to detect
   agent-committed work later.

3. **Publish `agent_started`.**

4. **Extract E23 context.** Loads `protocol.IMPLManifest` and calls
   `protocol.ExtractAgentContextFromManifest()` to produce a per-agent JSON
   payload. This replaces the full IMPL doc prompt with only what the agent
   needs.

5. **Inject retry prefix (GAP-4 fix).** If this is a retry via
   `executeRetryLoop()`, the retry context prefix is read from
   `retryPrefixMap` and prepended to the agent prompt *after* E23 extraction
   to avoid being overwritten.

6. **Execute via backend.** Calls `runner.ExecuteStreamingWithTools()` with
   `onChunk` and `onToolCall` callbacks that publish SSE events.

7. **Poll for completion report.** After execution returns, briefly polls
   (5s) for a report the agent may have written to the worktree IMPL doc.

8. **Auto-commit and synthesize report.** If no report found (API/Bedrock
   agents don't write protocol reports), the orchestrator:
   - Checks for existing partial/blocked reports (BUG-4 fix)
   - Auto-stages and commits all changes
   - Synthesizes a `status: complete` report with commit SHA and file list

9. **Persist report.** Writes completion report to the main-branch IMPL
   manifest under `reportMu` lock (serializes concurrent agent writes).

10. **Publish `agent_complete`.** Suppressed if auto-retry will follow
    (BUG-5 fix).

11. **E19 failure routing.** If status is `partial` or `blocked`, calls
    `RouteFailure()` and either retries automatically or escalates.

### Structured Output Path

`launchAgentStructured()` in `structured_wave.go` is an alternative launch
path used when `UseStructuredOutput = true`. It delegates to
`runWaveAgentStructuredFunc` (injected by the engine) and skips the
poll/synthesize step since structured output produces the completion report
directly. CLI backends fall back to the standard `launchAgent()` path.

---

## Multi-Backend Agent Support

The orchestrator supports multiple LLM providers via `newBackendFunc()` and
the `BackendConfig.Kind` field. Agents can also use a **provider prefix** in
their `model:` field (e.g., `openai:gpt-4o`, `bedrock:claude-sonnet-4-5`).

### Provider Routing

```
model string ──→ parseProviderPrefix()
                    │
                    ├─ "openai:gpt-4o"      → provider="openai",  model="gpt-4o"
                    ├─ "bedrock:claude-..."   → provider="bedrock", model="claude-..."
                    ├─ "cli:kimi"            → provider="cli",     model="kimi"
                    ├─ "anthropic:claude-..." → provider="anthropic", model="claude-..."
                    └─ "gpt-4o" (no colon)   → provider="",        model="gpt-4o"
```

Explicit prefix overrides `BackendConfig.Kind`.

### Supported Backends

| Kind | Backend | Notes |
|---|---|---|
| `api` / `anthropic` | `apiclient` (Anthropic Messages API) | Default; uses `ANTHROPIC_API_KEY` |
| `openai` | `openaibackend` (OpenAI-compatible) | Uses `OpenAIKey` or `OPENAI_API_KEY`; supports `BaseURL` override |
| `bedrock` | `bedrockbackend` (AWS Bedrock SDK) | Auto-expands short model names to Bedrock inference profile IDs |
| `cli` | `cliclient` (shells out to Claude CLI) | Uses `SAW_CLI_BINARY` env var |
| `ollama` | `openaibackend` with localhost:11434 | OpenAI-compatible API, no API key |
| `lmstudio` | `openaibackend` with localhost:1234 | OpenAI-compatible API |
| `auto` / `""` | Anthropic API if key present, else CLI | Fallback logic |

### Per-Agent Model Override

Each agent in the IMPL manifest can specify a `model:` field. During
`RunWave()`, the orchestrator compares the agent's model to `defaultModel`
and creates a separate backend+runner if they differ. This allows Scout to
assign different agents to different models (e.g., Opus for complex logic,
Haiku for simple transforms).

### Bedrock Model Expansion

`expandBedrockModelID()` maps short names to full Bedrock inference profile
IDs:

- `claude-sonnet-4-5` -> `us.anthropic.claude-sonnet-4-5-20250929-v1:0`
- `claude-opus-4-6` -> `us.anthropic.claude-opus-4-6-v1`
- `claude-haiku-4-5` -> `us.anthropic.claude-haiku-4-5-20251001-v1:0`

Already-expanded IDs (containing `.anthropic.`) pass through unchanged.

### Model Name Validation

`validateModelName()` rejects model strings over 200 characters or containing
characters outside `[a-zA-Z0-9\-._:/]` to prevent injection attacks.

---

## Failure Handling (E19)

When an agent reports `partial` or `blocked` status in its completion report,
the orchestrator routes the failure through a decision tree.

### Failure Types and Actions

| Failure Type | Action | Behavior |
|---|---|---|
| `transient` | `ActionRetry` | Retry up to 2 times (network timeout, rate limit) |
| `fixable` | `ActionApplyAndRelaunch` | Apply fix from notes, relaunch up to 2 times |
| `timeout` | `ActionRetryWithScope` | Retry once with scope-reduction note |
| `needs_replan` | `ActionReplan` | Re-engage Scout; surfaces to human |
| `escalate` | `ActionEscalate` | Human intervention required |

### Retry Loop

`executeRetryLoop()` handles automatic retries for transient, fixable, and
timeout failures:

1. Check retry count against max (`retryCountMap` keyed by
   `<slug>:<wave>:<agent>`).
2. Build retry context via `retryctx.BuildRetryContext()` for enriched prompt
   with fix guidance.
3. Publish `auto_retry_started` event.
4. Store retry prefix in `retryPrefixMap` (consumed by `launchAgent` after
   E23 extraction).
5. Re-call `launchAgent()` recursively. Retry count prevents infinite loops.

If all retries exhaust, publishes `auto_retry_exhausted` and returns error.

### Reactions Override

`RouteFailureWithReactions()` consults the IMPL manifest's `reactions:` block
before falling back to E19 defaults. This allows per-IMPL customization of
failure behavior (e.g., `action: "pause"` for timeout instead of auto-retry).

---

## Merge

`MergeWave()` delegates to `executeMergeWave()` (in `merge.go`).

### Steps

1. **Find wave** in IMPL doc.
2. **Load manifest** and check completion reports. Aborts if any agent is
   `partial` or `blocked`.
3. **Record base commit** (`HEAD` before merges).
4. **Load merge log** for idempotency (E9). Agents already merged are skipped.
5. **Verify agent commits.** Checks each pending agent has commits beyond
   base. Cross-repo agents verified in their own repo.
6. **Predict conflicts.** Cross-references `files_changed` and
   `files_created` across agents. Returns error if any file appears in
   multiple agents' lists (excluding `docs/IMPL/`).
7. **Merge each agent.** `git merge --no-ff` with merge message. Records
   merge SHA in merge log after each agent.
8. **Cleanup.** Remove worktree and delete branch after successful merge.

### Idempotency (E9)

Merge log (`protocol.LoadMergeLog` / `SaveMergeLog`) tracks which agents have
been merged. Re-running `MergeWave` skips already-merged agents, making the
operation safe to retry.

### No-Op Agents

Agents that produced no file changes are recorded in the merge log as
`"no-op"` and their worktrees/branches are cleaned up without merging.

---

## Verification and Quality Gates

### Post-Merge Verification

`RunVerification()` (in `verification.go`) runs:

1. `go vet ./...` (skipped if no `go.mod` present)
2. The user-specified test command (split on whitespace, executed in repo dir)

### Quality Gates (E21)

`RunQualityGates()` (in `quality_gates.go`) executes configured gates from
the IMPL manifest's `quality_gates:` section:

- Each gate runs with a 5-minute timeout.
- Output is truncated to 2000 characters.
- If `gates.Level == "quick"`, all gates are skipped.
- Required gates that fail produce a blocking error. Non-required gates
  report failures but continue.

---

## FinalizeWave Pipeline

`engine.FinalizeWave()` (in `pkg/engine/finalize.go`) is the engine-level
post-agent finalization pipeline. It is the Go equivalent of the CLI's
`sawtools finalize-wave` command.

### Pipeline Steps

1. **VerifyCommits (I5)** -- fatal if any agent has no commits.
2. **ScanStubs (E20)** -- scan changed files for TODO/FIXME markers
   (informational, non-blocking).
3. **RunGates (E21)** -- execute quality gates with caching
   (`gatecache.New`). Required gate failure is fatal.
4. **ValidateIntegration (E25)** -- scan for unconnected exports
   (informational, non-blocking). Persists report to manifest.
5. **MergeAgents** -- merge agent branches into target branch. Fatal on
   conflict.
6. **FixGoModReplacePaths** -- defense-in-depth worktree artifact cleanup.
7. **VerifyBuild** -- run `test_command` and `lint_command`. Both must pass.
8. **Cleanup** -- remove worktrees and branches (best-effort, non-fatal).

On build failure, auto-diagnoses using H7 pattern matching
(`builddiag.DiagnoseError`).

### MarkIMPLComplete

`engine.MarkIMPLComplete()` handles post-wave completion:

1. **E15:** Write completion marker to the manifest.
2. **E18:** Update `docs/CONTEXT.md` with new contracts.
3. **Archive:** Move IMPL from `docs/IMPL/` to `docs/IMPL/complete/`.

---

## Auto-Remediation

`engine.AutoRemediate()` (in `pkg/engine/auto_remediate.go`) handles automatic
fix attempts when `FinalizeWave` fails on build verification.

### Loop

1. Extract error log and determine failed gate type from `FinalizeWaveResult`.
2. Call `FixBuildFailure()` -- AI agent that reads error output and applies
   fixes.
3. Re-run `VerifyBuild()`.
4. If build passes, return `Fixed=true`. Otherwise loop.
5. After `MaxRetries` attempts, return `Fixed=false`.

Used by the daemon when autonomy permits (`StageGateFailure`).

---

## Closed-Loop Gate Retry (R3)

`engine.ClosedLoopGateRetry()` (in `pkg/engine/closed_loop_gate.go`) handles
**pre-merge per-agent** gate failures, distinct from post-merge
auto-remediation (R1).

### Flow

1. Classify the error using `retryctx.ClassifyError()`.
2. Generate fix suggestions via `retryctx.SuggestFixes()`.
3. Build a structured fix prompt with error output, classification, and
   suggestions.
4. Launch a fix agent in the agent's **worktree** (not main repo).
5. Re-run the gate command in the worktree.
6. If gate passes, return `Fixed=true`. Otherwise loop up to `MaxRetries`.

The fix agent is instructed to apply minimal targeted fixes and not commit --
the orchestrator handles the gate re-run.

---

## Context Memory (E18)

`UpdateContextMD()` (in `context.go`) creates or updates `docs/CONTEXT.md`
after final wave verification:

- Creates the file with canonical schema if it doesn't exist.
- Appends a `features_completed` entry with slug, IMPL doc path, wave/agent
  counts, and date.
- Commits the change: `chore: update docs/CONTEXT.md for {slug}`.

This ensures future Scout runs avoid proposing types/interfaces that already
exist.

---

## Journal Integration

`journal_integration.go` integrates with `pkg/journal` for agent session
recovery:

- **`PrepareAgentContext()`** -- loads journal history from
  `.saw-state/wave{N}/agent-{ID}/index.jsonl` and generates context markdown
  for agent recovery. Returns empty string on first launch.
- **`WriteJournalEntry()`** -- appends tool use/result entries to the agent's
  journal in JSONL format. Creates directory structure on first write.

---

## Pipeline Package

`pkg/pipeline` provides a reusable, ordered step-execution framework.

### Core Types

```go
type Pipeline struct { name string; steps []Step }
type Step struct {
    Name          string
    Func          StepFunc  // func(ctx, *State) error
    ErrorStrategy ErrorStrategy  // "fail" | "continue" | "retry"
    MaxRetries    int
    Condition     string  // "" | "always" | "on_success" | "on_failure"
}
type State struct {
    RepoPath string
    IMPLPath string
    WaveNum  int
    Values   map[string]interface{}
    Errors   []error
}
```

### Execution Semantics

- Steps run sequentially. Context cancellation checked before each step.
- `ErrorFail` (default): abort pipeline on error.
- `ErrorContinue`: append error to `state.Errors`, proceed.
- `ErrorRetry`: retry up to `MaxRetries` additional attempts; final failure
  treated as `ErrorFail`.
- Conditions: `"on_success"` runs only when `state.Errors` is empty;
  `"on_failure"` runs only when non-empty.

### Registry

`pipeline.Registry` maps step names to `StepFunc` implementations.
`DefaultRegistry()` pre-populates the standard SAW steps:
`validate_invariants`, `create_worktrees`, `run_quality_gates`,
`merge_agents`, `verify_build`, `cleanup`.

### WavePipeline

`WavePipeline(waveNum)` builds the canonical wave pipeline:

```
validate_invariants → create_worktrees → run_quality_gates →
merge_agents → verify_build → cleanup
```

Step implementations are stubs in `saw_steps.go` that validate required state
keys; real implementations are wired via the registry by the engine layer.

---

## Queue / Daemon System

### Queue Manager (`pkg/queue`)

`queue.Manager` manages the IMPL queue directory (`docs/IMPL/queue/`).

```go
type Item struct {
    Title              string   // human-readable feature title
    Priority           int      // lower number = higher priority
    DependsOn          []string // slugs that must be "complete" first
    FeatureDescription string
    Status             string   // queued | in_progress | complete | blocked
    AutonomyOverride   string   // per-item autonomy level override
    Slug               string   // auto-generated from title if empty
}
```

**Key operations:**

- `Add(item)` -- writes YAML to `{priority:03d}-{slug}.yaml`
- `List()` -- reads all items sorted by priority
- `Next()` -- returns highest-priority item with status `"queued"` whose
  `depends_on` slugs are all `"complete"` (checks both queue items and
  `docs/IMPL/complete/` directory)
- `UpdateStatus(slug, status)` -- updates in-place

### Daemon (`pkg/engine/daemon.go`)

`RunDaemon()` is a long-running loop that processes the IMPL queue:

```
loop:
  1. CheckQueue() → find next eligible item (autonomy-gated)
  2. If empty → sleep PollInterval (default 30s) → loop
  3. Process item:
     a. RunScout() → generate IMPL doc
     b. If supervised autonomy: pause for human review
     c. Load wave list from IMPL
     d. For each wave:
        - CreateWorktrees()
        - FinalizeWave() (verify + merge + build)
        - On failure: AutoRemediate() if autonomy permits
     e. MarkIMPLComplete()
  4. Update queue status (complete/blocked)
```

`CheckQueue()` (in `queue_advance.go`) respects autonomy levels: it only
marks items as `"in_progress"` if `ShouldAutoApprove(level, StageQueueAdvance)`
returns true.

---

## Program-Level Orchestration (RunTierLoop)

`engine.RunTierLoop()` (in `pkg/engine/program_tier_loop.go`) implements
E28-E34 for multi-IMPL programs organized into dependency tiers.

### Flow

```
1. Parse PROGRAM manifest
2. Loop:
   a. Find current tier (lowest with incomplete IMPLs)
   b. If all complete → return ProgramComplete
   c. Partition IMPLs: needsScout vs preExisting (E28A)
   d. Launch parallel Scouts for pending IMPLs
   e. Validate pre-existing IMPLs (program import mode)
   f. If not autoMode → return RequiresReview
   g. Execute waves for each IMPL in the tier (RunWaveFull)
   h. Run tier gate (E29)
   i. If gate fails:
      - AutoMode: trigger replan (E34) → re-parse manifest → retry tier
      - Manual: return gate_failed
   j. Freeze contracts (E30)
   k. If final tier → return Complete
   l. Advance to next tier (E33) → loop
```

### Key Types

```go
type TierLoopOpts struct {
    ManifestPath string
    RepoPath     string
    AutoMode     bool    // true = fully autonomous; false = pause for review
    Model        string
    OnEvent      func(TierLoopEvent)
}

type TierLoopResult struct {
    TiersExecuted   int
    TiersRemaining  int
    ProgramComplete bool
    FinalState      string  // "complete", "cancelled", "awaiting_review", "gate_failed", etc.
    RequiresReview  bool
    Errors          []string
}
```

### Parallel Scout

Scouts within a tier launch in parallel via `launchParallelScoutsFunc`
(injected as a function variable for compilation independence).

### Replan (E34)

`AutoTriggerReplan()` fires when a tier gate fails in auto mode. It constructs
a reason string from gate failure details and calls `ReplanProgram()` to
re-engage the Planner agent.

---

## Autonomy Levels

`pkg/autonomy` controls how much the orchestrator can do without human
approval.

### Levels

| Level | Description |
|---|---|
| `gated` | All stages require human approval |
| `supervised` | IMPL review requires human; wave advance, gate failure, queue advance are auto-approved |
| `autonomous` | All stages auto-approved |

### Decision Stages

| Stage | What it gates |
|---|---|
| `impl_review` | Whether to proceed after Scout produces IMPL |
| `wave_advance` | Whether to advance to next wave automatically |
| `gate_failure` | Whether to auto-remediate failed gates |
| `queue_advance` | Whether to auto-pick next item from queue |

### Per-Item Override

Queue items can specify `autonomy_override` to use a different level than the
global config. `EffectiveLevel()` checks the override first, then falls back
to `Config.Level`.

---

## Agent Scheduling

`engine.PrioritizeAgents()` (in `pkg/engine/scheduler.go`) determines optimal
agent launch order within a wave based on dependency graph analysis.

### Algorithm

1. Build dependency graph from `FileOwnership` entries.
2. Calculate critical path depth for each agent (longest chain of dependents).
3. Sort by:
   - Critical path depth (descending) -- deeper paths launch first to
     unblock downstream work earlier
   - File count (ascending) -- fewer files = lower implementation risk
   - Original declaration order (stable sort for ties)

Single-agent waves skip sorting. Disabled via `SAW_NO_PRIORITIZE=1` env var.

The orchestrator publishes an `agent_prioritized` event showing original vs.
reordered launch sequence.

---

## Cross-Repo Orchestration

For waves that span multiple repositories, the orchestrator supports
pre-computed worktree paths via `SetWorktreePaths()`.

### Flow

1. The engine layer calls `protocol.CreateWorktrees()` which creates
   worktrees in each target repo based on `FileOwnership.Repo` fields.
2. Engine passes the resulting path map to `orch.SetWorktreePaths()`.
3. During `launchAgent()`, if a pre-computed path exists for the agent letter,
   it is used directly. Otherwise, single-repo worktree creation runs as
   fallback.
4. During merge, `executeMergeWave()` builds a per-agent repo map from
   `FileOwnership` and merges each agent's branch in its own repo.

---

## See Also

- [Architecture Overview](architecture.md) -- Orchestrator role in the engine
- [SSE Events](sse-events.md) -- Event types and lifecycle
- [Protocol Parsing](protocol-parsing.md) -- IMPL doc structure
- [Backends](backends.md) -- Per-agent backend routing
