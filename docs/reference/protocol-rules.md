# Protocol Rules Reference

E-rules are numbered execution constraints checked by the engine at specific lifecycle phases. Some are structural (enforced by the YAML validator), some are behavioral (enforced at runtime by `prepare-wave`, `finalize-wave`, or the orchestrator), and some are informational (surfaced as warnings without blocking).

Rules from the I-series (I1–I6) are invariants summarized at the end of this document. See the canonical `protocol/invariants.md` for full formal statements and enforcement details, and `docs/reference/error-codes.md` for the V-series codes that map to invariant violations.

---

## Summary Table

| Rule | Name / Summary | Phase | Enforcement | What It Checks |
|------|---------------|-------|-------------|----------------|
| E2 | Interface Freeze | pre-wave | blocking | Interface contracts and scaffolds must not change after worktrees are created |
| E10 | Verification Field Format | validation | blocking | `verification:` field must match expected format |
| E11 | Conflict Prediction | pre-merge | informational (warning) | Files touched by multiple agents in the same wave predicted to conflict |
| E12 | Agent Isolation Verification | wave (agent start) | blocking | Agent must be running in its assigned worktree on the correct branch |
| E15 | Completion Marker | post-wave | blocking | Completion marker written to archive IMPL doc after final wave |
| E16 | IMPL Validation | scout / pre-wave | blocking | IMPL manifest structural integrity (all invariants, schema, enums) |
| E17 | Project Memory Injection | scout / planner | automatic | `docs/CONTEXT.md` prepended to Scout and Planner prompts if present |
| E18 | Context Update | post-wave (final) | non-fatal | `docs/CONTEXT.md` updated after final wave completes |
| E19 | Failure Type Routing | orchestrator | behavioral | Agent partial/blocked reports are routed to retry, relaunch, replan, or escalate |
| E20 | Stub Scan | pre-merge | informational / opt-in blocking | Changed files scanned for TODO/FIXME/stub markers |
| E21 | Quality Gate | pre-merge / post-merge | blocking | Build, test, lint, format gates from `quality_gates` in the IMPL doc |
| E21A | Baseline Gate | pre-wave | blocking | Quality gates run against the baseline (before worktrees) to verify a clean starting state |
| E21B | Cross-Repo Baseline Gate | pre-wave | blocking | Baseline gates run in each repo of a multi-repo IMPL |
| E22 | Scaffold Build Verification | post-scaffold | blocking | Scaffold files are compiled after creation to catch errors before agent launch |
| E23 | Per-Agent Context Extraction | wave (agent start) | automatic | Each agent receives its own extracted context payload instead of the full IMPL doc |
| E24 | Verification Retry Loop | post-merge | behavioral | When quality gates fail after merge, a single-agent retry IMPL is generated (up to MaxRetries) |
| E25 | Integration Gap Detection | pre-merge | informational / opt-in blocking | Exported symbols with no call-sites outside their defining file |
| E26 | Integration Agent | post-wave | opt-in | Dedicated integration agent launched to wire exports when E25 gaps are found |
| E27 | Type Collision Detection | pre-merge | opt-in blocking | Two agent branches in the same wave declare the same type name in the same package |
| E28 | Program Tier Execution | program / tier loop | blocking | IMPL partitioning and wave execution within a program tier (E28A = partition, E28 = merge target checkout) |
| E29 | Tier Gate | program / tier boundary | blocking | Verification gates run across all IMPLs before advancing to the next tier |
| E30 | Contract Freeze at Tier Boundary | program / tier boundary | blocking | Interface contracts frozen (hash-locked) after a tier gate passes |
| E31 | Parallel Scout Launch | program / scout phase | automatic | Multiple Scout agents launched in parallel for all pending IMPLs in a tier |
| E32 | Program Progress Sync | program | automatic | Program manifest completion counters recalculated after each IMPL wave completes |
| E33 | Tier Advance | program / tier boundary | automatic | Program tier index advanced after tier gate passes and contracts are frozen |
| E34 | Auto Replan | program / tier gate fail | automatic | Planner re-engaged automatically when a tier gate fails |
| E35 | Wiring Declaration Validation | pre-merge | non-fatal | Functions defined by one agent but called from files not owned by that agent (same-package call gaps) |
| E37 | Critic Gate | pre-wave | blocking (threshold: 3 agents or multi-repo) | Pre-wave critic review required; verdict must be PASS or explicitly skipped |
| E38 | Gate Result Cache | wave / finalize | automatic | Gate results cached by HEAD commit + command hash to avoid redundant re-runs |
| E40 | Observability Events | scout / tier loop | automatic | Lifecycle events (`scout_launch`, `scout_complete`, `tier_gate_passed/failed`) emitted to observability |
| E41 | Type Collision Pre-Wave | pre-wave (prepare) | blocking | Type collision check (same as E27) run during `prepare-wave` before worktrees are created |
| E42 | SubagentStop Validation | wave (agent stop) | blocking | Agent protocol obligations verified at session close: I1 ownership, I5 commits, completion report |
| E43 | Hook-Based Isolation Enforcement | wave (agent lifecycle) | blocking | Four-hook defense-in-depth: env injection, cd auto-injection, write path validation, compliance verification |
| E44 | Context Injection Observability | scout / pre-wave | non-fatal (warning) | `injection_method` and `context_source` recorded on IMPL doc for observability audit |
| E45 | Shared Data Structure Scaffold Detection | scout | automatic | Scout detects structs/enums/types referenced by 2+ agents and adds them to Scaffolds section |
| E46 | Test File Cascade Detection | scout / pre-wave | blocking (pre-wave) | Test files referencing changed interfaces detected and assigned to interface-changing agent |
| E47 | Between-Wave Caller Cascade Hotfix | finalize-wave | automatic | Caller cascade compile errors from signature changes auto-fixed by hotfix agent inline |

---

## Rule Details

### E2 — Interface Freeze

After worktrees are created (`sawtools create-worktrees`), the `interface_contracts` and `scaffolds` sections of the IMPL doc are frozen. A SHA-256 hash of each section is written to `frozen_contracts_hash` and `frozen_scaffolds_hash` on the manifest at freeze time.

On subsequent reads, `CheckFreeze` recomputes the hashes and returns a `FreezeViolation` for any section that changed. The pre-wave gate does not re-run CheckFreeze directly, but worktree creation calls `SetFreezeTimestamp`, and any mutation to contracts after that point is detectable.

Workaround when a genuine interface change is needed: delete worktrees, update the IMPL doc, recreate worktrees. The freeze timestamp is reset on the next `create-worktrees` call.

**Implementation:** `pkg/protocol/freeze.go` — `SetFreezeTimestamp`, `CheckFreeze`.

---

### E10 — Verification Field Format

The `verification` field on the IMPL manifest must conform to the expected format. An invalid format emits `V043_INVALID_VERIFICATION` (formerly `E10_INVALID_VERIFICATION`) during validation.

**Implementation:** `pkg/protocol/fieldvalidation.go`, `pkg/result/codes.go` (`CodeInvalidVerification`).

---

### E11 — Conflict Prediction

Before merging agent branches, `PredictConflictsFromReports` cross-references each agent's `files_changed` and `files_created` lists. Any non-IMPL-state file appearing in two or more agents' reports is flagged as a conflict risk.

Two passes reduce false positives:
1. **Identical content:** If all agents that touched a file produced the same file hash, git will auto-resolve (skipped).
2. **Non-overlapping hunks:** If the line ranges in each agent's diff do not overlap, a 3-way merge is safe (skipped).

The result is `Partial` (warnings) rather than `Fatal`. The finalize pipeline logs the warnings but does not block merge by default. The caller (orchestrator or CLI) decides how to handle conflict predictions.

IMPL doc files (`docs/IMPL/`) and state files (`.saw-state/`) are exempt from conflict prediction because multiple agents are expected to update them.

**Implementation:** `pkg/protocol/conflict_predict.go` — `PredictConflictsFromReports`.

---

### E12 — Agent Isolation Verification

At the start of a wave agent's work (Field 0), `VerifyIsolation` checks:
- The current git directory is inside a known worktree path (contains `.claude/worktrees/` or `.claire/worktrees/` in the resolved absolute path).
- The current branch matches the expected branch name for this agent (`saw/{slug}/waveN-agent-{ID}` or legacy `waveN-agent-{ID}`).
- At least one worktree is registered with git (guards against running on main).

A violation returns `CodeIsolationVerifyFailed` (fatal). The agent must not proceed if isolation verification fails.

**Implementation:** `pkg/protocol/isolation.go` — `VerifyIsolation`.

---

### E15 — Completion Marker

After the final wave of an IMPL completes successfully, `WriteCompletionMarker` writes a marker file and `ArchiveIMPL` moves the IMPL doc from `docs/IMPL/` to `docs/IMPL/complete/`. These two operations together constitute the E15 completion contract.

The marker write uses a date string (`YYYY-MM-DD`). If `MarkIMPLCompleteOpts.Date` is empty, the current date is used.

**Implementation:** `pkg/engine/finalize.go` — `MarkIMPLComplete`, which calls `protocol.WriteCompletionMarker` and `protocol.ArchiveIMPL`.

---

### E16 — IMPL Validation

`Validate` runs all structural invariant checks on a parsed `IMPLManifest`. Called at multiple lifecycle points:
- After Scout generates the IMPL doc (scout correction loop validates before accepting output).
- Before wave execution (`sawtools finalize-impl`, `FinalizeIMPL`).
- As part of the pre-wave gate.

Failures are `SAWError` slices using V-series codes (formerly E16_* variants). Common checks triggered by E16: disjoint ownership (V002), same-wave dependencies (V003), required fields (V005), invalid state (V008), unknown YAML keys (V013), file existence for `action=modify` (V041), repo mismatch when all modify-files are absent (V045).

Auto-fix: `sawtools validate --fix` can correct some common issues (invalid gate types rewritten to `custom`).

**Implementation:** `pkg/protocol/validation.go` — `Validate`, `ValidateBytes`, `FullValidate`.

---

### E17 — Project Memory Injection

When `docs/CONTEXT.md` exists in the repository, its contents are prepended to the Scout and Planner agent prompts under a `## Project Memory` heading. This is automatic and non-blocking. If the file is absent, the prompt is unchanged.

`readContextMD` reads the file; any read error is silently ignored (non-fatal by design — CONTEXT.md is optional).

**Implementation:** `pkg/engine/runner.go` — `readContextMD` called in `RunScout` and `RunPlanner`.

---

### E18 — Context Update

After the final wave of an IMPL completes, `UpdateContext` (via `protocol.UpdateContextMD`) appends a record of the completed feature to `docs/CONTEXT.md`. The format is a `ContextMDEntry` with the feature title, slug, completion date, and a brief description.

This step is non-fatal: if it fails, `MarkIMPLComplete` logs a warning and continues to archive the IMPL. A missing `docs/CONTEXT.md` is created automatically.

**Implementation:** `pkg/orchestrator/context.go` — `UpdateContextMD`; `pkg/engine/finalize.go` — called in `MarkIMPLComplete`.

---

### E19 — Failure Type Routing

When an agent reports `partial` or `blocked` completion status, the orchestrator classifies the failure by its `failure_type` field and routes to one of five actions:

| `failure_type` | Action | Max Retries |
|---|---|---|
| `transient` | `ActionRetry` (plain retry) | 2 |
| `fixable` | `ActionApplyAndRelaunch` (apply fix prompt, relaunch) | 2 |
| `timeout` | `ActionRetryWithScope` (retry once with scope-reduction note) | 1 |
| `needs_replan` | `ActionReplan` (re-engage Scout) | 0 |
| `escalate` | `ActionEscalate` (surface to human) | 0 |
| _(absent)_ | `ActionEscalate` | 0 |

The `reactions:` block in the IMPL doc can override the default action and `max_attempts` for each failure type via `RouteFailureWithReactions`. If no matching reaction entry is found, E19 defaults apply.

**Implementation:** `pkg/orchestrator/failure.go` — `RouteFailure`, `RouteFailureWithReactions`; `pkg/orchestrator/orchestrator.go` — `executeRetryLoop`.

---

### E20 — Stub Scan

After agents complete and before merge, `ScanStubs` searches changed files for `TODO`, `FIXME`, and similar placeholder markers. By default the result is informational (logged but not blocking).

When `FinalizeWaveOpts.RequireNoStubs` is `true` (the M3 mode flag), any detected stubs cause the finalize pipeline to fail with a fatal error. This mode is opt-in; the default is informational.

The file list scanned is derived from each agent's `files_changed` and `files_created` in their completion reports.

**Implementation:** `pkg/engine/finalize_steps.go` — `StepScanStubs`.

---

### E21 — Quality Gate

Quality gates are shell commands defined in `quality_gates.gates` of the IMPL doc. They run in two phases:
- **Pre-merge** (`RunPreMergeGates`): gates with `timing: ""` or `timing: "pre-merge"`. Run after agents complete, before `MergeAgents`. A failing required gate blocks merge.
- **Post-merge** (`RunPostMergeGates`): gates with `timing: "post-merge"`. Run after merge completes. Results inform the post-merge verification step.

Gates are grouped by `phase` (PRE → VALIDATION → POST) and within each phase by `parallel_group` for concurrent execution. Source-code gates (build, test, lint) are auto-skipped for docs-only waves. Gates whose build system is absent in the repo are also auto-skipped.

**Implementation:** `pkg/protocol/gates.go` — `RunGatesWithCache`, `RunPreMergeGates`, `RunPostMergeGates`.

---

### E21A — Baseline Gate

Before creating worktrees, `RunBaselineGates` executes the IMPL's quality gates against the current codebase (the baseline). A failing required gate blocks wave launch — the problem is pre-existing, not agent-introduced.

Results are optionally cached by HEAD commit SHA to avoid re-running on repeated `prepare-wave` invocations against the same commit.

**Implementation:** `pkg/protocol/baseline_gates.go` — `RunBaselineGates`; `pkg/engine/prepare.go`.

---

### E21B — Cross-Repo Baseline Gate

For multi-repo IMPLs, `RunCrossRepoBaselineGates` runs baseline checks in each target repository. Repos with explicit `build_command` / `test_command` in the SAW config use those directly; others fall back to the IMPL's `quality_gates`.

Returns early on first repo failure for fast feedback. Errors are fatal and block wave launch.

**Implementation:** `pkg/protocol/baseline_gates.go` — `RunCrossRepoBaselineGates`.

---

### E22 — Scaffold Build Verification

After scaffold files are committed and before agent launch, `runScaffoldBuildVerification` compiles the project to verify the scaffolds are syntactically valid. A build failure at this point indicates a problem in the scaffold design, not agent work.

This runs once per IMPL, keyed to the scaffold step in the prepare-wave pipeline.

**Implementation:** `pkg/engine/runner.go` — `runScaffoldBuildVerification`, `runScaffoldBuildVerificationWithDoc`.

---

### E23 — Per-Agent Context Extraction

Instead of passing the entire IMPL doc as each agent's prompt, `ExtractAgentContextFromManifest` extracts a targeted context payload containing only the fields relevant to that agent (owned files, task, dependencies, verification gate). The payload is marshaled to JSON and set as `agentSpec.Task`.

If context extraction fails (e.g., the manifest cannot be loaded), the orchestrator falls back to the pre-existing task string already set on the `agentSpec` from the IMPL doc parse.

Checkpoint names within the agent's work sequence follow the E23A sub-protocol: `001-isolation`, `002-first-edit`, `003-tests`, `004-pre-report`.

**Implementation:** `pkg/orchestrator/orchestrator.go` (in `launchAgent`).

---

### E24 — Verification Retry Loop

When quality gates fail after wave merge and the C2 closed-loop retry is enabled, `RetryLoop.Run` generates a minimal single-agent IMPL doc (`IMPL-{slug}-retry-{N}.yaml`) targeting the failed files. The agent task includes the gate failure output so the retry agent knows exactly what to fix.

State transitions:
- Attempt < `MaxRetries` → `FinalState = "retrying"` (caller launches the retry agent).
- Attempt ≥ `MaxRetries` → `FinalState = "blocked"` (no IMPL saved; escalation required).

Default `MaxRetries` is 2. Retry IMPLs are excluded from V047_TRIVIAL_SCOPE validation (single-agent single-file is expected for retries).

**Implementation:** `pkg/retry/loop.go` — `RetryLoop`.

---

### E25 — Integration Gap Detection

After wave agents complete, `ValidateIntegration` scans changed `.go` and `.tsx` files for exported symbols (functions, types, methods) that have no call-sites outside their defining file. Such gaps indicate that the new code was written but not wired into the rest of the system.

Severity classification:
- `error`: functions with constructor-style prefixes (`New`, `Build`, `Register`, `Run`, `Start`, `Init`).
- `warning`: function calls, React prop passes.
- `info`: types, struct fields with JSON tags (configurable via `integration_gap_severity_threshold`).

By default E25 is informational. When `FinalizeWaveOpts.EnforceIntegrationValidation` is `true` (the M2 mode flag), gaps at or above the threshold severity block the finalize pipeline.

**Implementation:** `pkg/protocol/integration.go` — `ValidateIntegration`; `pkg/engine/finalize_steps.go` — `StepValidateIntegration`.

---

### E26 — Integration Agent

When E25 gap detection finds unconnected exports and the IMPL doc has `integration_connectors` defined, an Integration Agent can be launched automatically to wire the gaps.

Preconditions (both required):
- **E26-P1**: `integration_reports` must contain persisted gap data for the wave (E25 must have run first).
- **E26-P2**: `integration_connectors` entries must list the files the agent is permitted to edit.

The Integration Agent receives a prompt summarizing the gap report and the connector file list. It operates within the same repo as the wave agents.

`IntegrationModel` on the engine opts selects the model for the integration agent; if unset, falls back to `WaveModel`.

**Implementation:** `pkg/engine/integration_runner.go` — `RunIntegrationAgent`.

---

### E27 — Type Collision Detection (Pre-Merge)

During the finalize-wave pipeline (step 3.3, opt-in), `CheckTypeCollisions` scans agent branches for type declarations using Go AST. Any type name appearing in two or more branches within the same Go package is a collision.

When a collision is detected, the step returns `fatal` (not informational). The recommended resolution is to keep the alphabetically-first agent's declaration and have other agents import it.

**Implementation:** `pkg/collision/detector.go` — `DetectCollisions`; `pkg/engine/finalize.go`.

---

### E28 — Program Tier Execution

In the PROGRAM workflow, `RunTierLoop` partitions the IMPLs at the current tier into two groups (E28A):
- IMPLs needing a Scout run (status `pending` or `scout_pending`).
- IMPLs with pre-existing IMPL docs (status already `reviewed` or later).

Scouts are launched in parallel for the first group (E31), then wave execution proceeds sequentially per IMPL. The merge target checkout within `MergeAgents` also references E28 — each agent branch is merged into the IMPL-scoped branch, not directly to `main`, when running under a PROGRAM tier.

**Implementation:** `pkg/engine/program_tier_loop.go` — `RunTierLoop`, `PartitionIMPLsByStatus`.

---

### E29 — Tier Gate

After all IMPLs in a tier complete their final wave, a tier gate verification step runs. The gate checks that the integrated state of all completed IMPLs satisfies cross-IMPL acceptance criteria.

Events emitted: `tier_gate_started`, `tier_gate_passed`, `tier_gate_failed` (all part of the E40 event stream).

A failing tier gate triggers E34 (auto replan) or surfaces to the human depending on configuration.

**Implementation:** `pkg/engine/program_tier_loop.go` step 9; `pkg/orchestrator/events.go` — `TierGateStartPayload`, `TierGateResultPayload`.

---

### E30 — Contract Freeze at Tier Boundary

When a tier gate passes, `SetFreezeTimestamp` is called on the PROGRAM manifest to hash-lock the interface contracts. This prevents downstream tiers from observing a moving contract target.

The freeze hash covers the `interface_contracts` section. Any subsequent mutation to contracts while downstream tiers are executing is detectable via `CheckFreeze`.

**Implementation:** `pkg/engine/program_tier_loop.go` step 10; `pkg/protocol/freeze.go`.

---

### E31 — Parallel Scout Launch

For all pending IMPLs in a tier, `LaunchParallelScouts` starts N Scout goroutines simultaneously. Each Scout receives the feature description from the PROGRAM manifest plus the manifest path so it can read frozen contract data.

Results are collected via a `sync.WaitGroup`. Failed scouts are recorded in `ParallelScoutResult.Failed`; the tier loop proceeds only if all scouts succeed.

**Implementation:** `pkg/engine/program_parallel_scout.go` — `LaunchParallelScouts`.

---

### E32 — Program Progress Sync

After each IMPL's wave completes, `SyncProgramProgress` re-reads the IMPL manifest, updates the PROGRAM manifest's per-IMPL status, and recalculates `completed_impls` and `total_impls` counters.

This is automatic and non-blocking. A failure to sync logs a warning but does not abort the tier loop.

**Implementation:** `pkg/engine/program_progress.go`.

---

### E33 — Tier Advance

After a tier gate passes and contracts are frozen (E29, E30), the PROGRAM manifest's `current_tier` is incremented to point to the next tier. In auto mode, `AdvanceTierAutomatically` handles the state mutation and commit.

**Implementation:** `pkg/engine/program_tier_loop.go` step 12.

---

### E34 — Auto Replan

When a tier gate fails, `triggerAutoReplan` re-engages the Planner agent with context from the gate failure. The replan constructs a reason string summarizing which gates failed and which IMPLs were involved.

The `replan_triggered` event is emitted (E40 stream). The tier loop halts until the replan completes and the operator re-runs the tier.

**Implementation:** `pkg/engine/program_tier_loop.go` step 11, `triggerAutoReplan`; `pkg/orchestrator/events.go` — `ReplanStartPayload`.

---

### E35 — Wiring Declaration Validation (Same-Package Call Gap)

The IMPL doc's `wiring:` block declares explicit wiring obligations: `symbol` (a function name), `defined_in` (the file that defines it), `must_be_called_from` (the file that must call it), and `agent` (the agent that owns `must_be_called_from`).

`ValidateWiringDeclarations` checks that each declared symbol appears as a call expression in `must_be_called_from`. For `.go` files, Go AST is used; for `.tsx`/`.ts` files, a TSX-prop-aware scan is used; all others fall back to a line scan.

`DetectE35Gaps` is a broader static analysis that finds all functions defined in agent-owned files but called from files not owned by the same agent (in the same Go package). This is run as an informational step in the finalize pipeline (non-fatal).

`CheckWiringOwnership` verifies that the `must_be_called_from` file is actually owned by the declared agent in the wiring entry.

**Implementation:** `pkg/protocol/e35_detection.go` — `DetectE35Gaps`; `pkg/protocol/wiring_validation.go` — `ValidateWiringDeclarations`, `CheckWiringOwnership`.

---

### E37 — Critic Gate

A pre-wave critic review is required when either:
- Wave 1 has 3 or more agents, **or**
- The IMPL spans 2 or more repositories.

If neither threshold is met, the gate passes automatically with `"critic review not required"`.

When required, the gate fails if `critic_report` is absent on the manifest. After the critic runs, `CriticGatePasses` evaluates the verdict:
- `PASS` → always proceed.
- `ISSUES` with any `error`-severity finding → always block.
- `ISSUES` with warnings only → proceed in auto mode; block in manual mode.
- `SKIPPED` (operator-explicit) → proceed.
- Any other verdict → block.

**Implementation:** `pkg/protocol/critic_gate.go` — `CriticGatePasses`; `pkg/protocol/pre_wave_gate.go` — `checkCriticReview`.

---

### E38 — Gate Result Cache

`GateResultCache` (engine-level) and `gatecache.Cache` (protocol-level) store gate pass/fail results keyed by HEAD commit SHA + gate command string. Repeated `prepare-wave` or `finalize-wave` invocations against the same commit reuse cached results.

Format gates with `fix: true` invalidate the cache after running (file content has changed). The protocol-level cache uses a 5-minute TTL; the engine-level cache is per-execution (in-memory).

**Implementation:** `pkg/engine/gate_cache.go`; `pkg/gatecache/cache.go`.

---

### E40 — Observability Events

Key lifecycle transitions emit structured events to the observability subsystem:
- `scout_launch` — emitted after Scout validation passes, before agent execution.
- `scout_complete` — emitted after Scout execution succeeds.
- `tier_gate_passed` / `tier_gate_failed` — emitted at each program tier boundary.
- `impl_complete` — emitted after `MarkIMPLComplete` archives the IMPL.

These events are non-blocking: emit failures are logged as warnings and do not halt execution.

**Implementation:** `pkg/engine/runner.go`; `pkg/engine/program_tier_loop.go`; `pkg/observability/events.go`.

---

### E41 — Type Collision Pre-Wave

During `prepare-wave` (before worktrees are created), `DetectCollisions` runs the same AST-based type collision check as E27 (finalize-wave step 3.3). A collision detected at this stage is fatal and blocks worktree creation.

The rationale: detecting collisions before launch is cheaper than discovering them during merge. E27 catches any collisions that appear during agent execution; E41 catches collisions that were already present in the planned file assignments.

**Implementation:** `pkg/engine/prepare.go` — step `type_collision`.

---

### E42 — SubagentStop Validation

At the SubagentStop lifecycle event, a hook validates that the completing agent has fulfilled its protocol obligations before the agent session closes. The hook identifies SAW agents by parsing the `[SAW:...]` tag from `agent_description` and runs agent-type-specific checks. Non-SAW agents pass through immediately (exit 0).

Validation matrix by agent type:
- **Wave agents:** I1 ownership verification (`git diff --name-only` vs `.saw-ownership.json`), I5 commit verification (at least 1 commit ahead of merge base), completion report presence in IMPL doc.
- **Critic agents:** `critic_report:` field present with `verdict`, `agents_reviewed`, and `issues` keys.
- **Scout agents:** IMPL doc exists at expected path and passes `sawtools validate`.
- **Scaffold agents:** All scaffold entries have `status: committed (...)`.

Exit codes: 0 = pass, 2 = block (unfulfilled obligations). The active IMPL path is read from `.saw-state/active-impl` (written by `prepare-wave`), falling back to extraction from `agent_description`.

**Implementation:** Claude Code hook script (`validate_agent_stop`); `pkg/protocol/ownership_check.go`, `pkg/protocol/commit_check.go`.

---

### E43 — Hook-Based Isolation Enforcement

Orchestrator ensures Claude Code lifecycle hooks are installed and active before launching wave agents. Hook-based enforcement supersedes instruction-based isolation (agents following written protocol).

Four-hook defense-in-depth:
1. **SubagentStart (`inject_worktree_env`):** Sets `SAW_AGENT_WORKTREE`, `SAW_AGENT_ID`, `SAW_WAVE_NUMBER`, `SAW_IMPL_PATH`, `SAW_BRANCH` environment variables. Non-blocking.
2. **PreToolUse:Bash (`inject_bash_cd`):** Prepends `cd $SAW_AGENT_WORKTREE &&` to every bash command. Fires only when `SAW_AGENT_WORKTREE` is non-empty (skips solo waves). Non-blocking.
3. **PreToolUse:Write/Edit (`validate_write_paths`):** Blocks relative paths and paths outside worktree boundaries (exit 2). Fires only when `SAW_AGENT_WORKTREE` is non-empty.
4. **SubagentStop (`verify_worktree_compliance`):** Checks completion report exists (E42/I4) and commits exist on branch (I5). Non-blocking (warnings logged to stderr for audit trail).

E43 enforces E4 mechanically. Before E43, agents followed written protocol and violations were possible via agent error or context compaction loss. After E43, relative paths and out-of-bounds writes are blocked at the tool boundary.

**Implementation:** Claude Code hook scripts in `implementations/claude-code/hooks/`.

---

### E44 — Context Injection Observability

**Scout obligation:** Before completing, the Scout calls `sawtools set-injection-method <impl-doc-path> --method <value>` to record how reference files were received. Valid values: `hook`, `manual-fallback`, `unknown`.

**Orchestrator obligation:** `sawtools prepare-agent` automatically writes `context_source` to each agent entry when extracting the brief. Valid values: `prepared-brief`, `cross-repo-full`. The orchestrator may write `fallback-full-context` manually when the fallback prompt path was used.

**Enforcement:** `sawtools validate` warns (non-blocking) when `injection_method` is absent on an active IMPL, and warns when `context_source` is absent on wave agents in `WAVE_EXECUTING`/`WAVE_MERGING`/`WAVE_VERIFIED` state.

**Implementation:** `pkg/protocol/fieldvalidation.go` (injection_method/context_source checks); `cmd/sawtools/set_injection_method.go`.

---

### E45 — Shared Data Structure Scaffold Detection

During the Scout phase, after defining agent tasks but before finalizing the IMPL doc, the Scout scans agent task prompts, file_ownership, and interface_contracts to detect data structures (structs, enums, type aliases, traits) referenced by 2+ agents. For each detected shared type, an entry is added to the Scaffolds section of the IMPL doc.

Detection heuristics:
- Agent A owns file X, Agent B's task says "import TypeName from X"
- Type appears in interface_contracts AND 2+ agent tasks reference it
- Same struct/enum name mentioned in multiple agents' "Interfaces to implement"

Does NOT trigger for types from external packages, types in existing codebase files not owned by any agent, or types mentioned in only one agent's task.

**Automated tool:** `sawtools detect-shared-types <impl-doc>` automates this detection. Scout invokes it after writing agent prompts and merges output into the Scaffolds section.

**Implementation:** `pkg/protocol/shared_type_detection.go`; `cmd/sawtools/detect_shared_types.go`.

---

### E46 — Test File Cascade Detection

When an interface contract involves signature changes, test files referencing the interface must be detected and assigned to an agent in the same wave.

Detection layers:
1. **Scout-time (primary):** During dependency analysis, Scout scans for `*_test.go` files that reference changed interfaces using `sawtools check-callers`. Unowned test files are assigned to the interface-changing agent.
2. **Pre-wave validation (E35 extension):** `sawtools pre-wave-validate` runs E35 detection including `detectTestCascades()`. Reports orphaned test files as E35Gap entries. Also runs `check-test-cascade` for a whole-repo scan; exits 1 if any orphaned test callers are found.

Rationale: Interface signature changes break test files, but test files are often not included in file_ownership because Scout focuses on implementation files. E46 prevents the 30+ minute manual post-merge fix cycle.

**Implementation:** `pkg/protocol/e35_detection.go` — `detectTestCascades`; `cmd/sawtools/check_test_cascade.go`.

---

### E47 — Between-Wave Caller Cascade Hotfix

When `finalize-wave`'s verify-build step completes with `CallerCascadeOnly=true` — meaning ALL build errors are in future-wave-owned or unowned files (caller cascade side-effects, not genuine wave N failures) — the hotfix step runs automatically.

`finalize-wave` detects `CallerCascadeOnly=true` and runs `apply-cascade-hotfix` inline (step 6a). The hotfix agent is restricted to files in `CallerCascadeErrors` and applies minimal caller fixes: `result.Result[T]` unwrapping, ctx param additions, deleted symbol replacements. Commits as `[SAW:wave{N}:integration-hotfix]`.

**Distinction from E26:** E26 wires unconnected exports (logical gaps, no compile error). E47 fixes compile errors in callers caused by signature changes in the wave that just completed.

**Debugging:** Pass `--dry-run` to `finalize-wave` to see what cascade errors would be hotfixed without running the agent.

**Implementation:** `pkg/engine/finalize.go` — step `apply_cascade_hotfix`; `pkg/engine/cascade_hotfix.go`.

---

## Invariants (I1–I6)

The canonical definitions live in `protocol/invariants.md`. This section summarizes each invariant and its enforcement mechanism for quick reference.

### I1 — Disjoint File Ownership

No two agents in the same wave own the same file. Enforced by six defense-in-depth layers: E43 hooks (Layer 0, primary), E3 pre-launch validation (Layer 1), E37 Critic gate (Layer 2), PrepareWave runtime check (Layer 3), E11 conflict prediction at merge (Layer 4), and E42 SubagentStop ownership audit (Layer 5). Cross-repo scope: I1 applies per-repository.

**Amendments:** Integration Agent (E26) is exempt (runs after merge, sole writer). Identical edits producing byte-identical content are non-blocking at merge time. Post-hoc undeclared-modification detection flags files modified outside declared ownership even when no other agent owns that file.

### I2 — Interface Contracts Precede Parallel Implementation

The Scout defines all interfaces that cross agent boundaries in the IMPL doc, including shared data structures (structs, enums, type aliases, traits) referenced by 2+ agents. The Scaffold Agent materializes them as committed type scaffold files before any Wave Agent launches. Contracts are frozen when worktrees are created (E2). Detection of shared types is covered by E45.

### I3 — Wave Sequencing

Wave N+1 does not launch until Wave N has been merged and post-merge verification has passed. `PrepareWave` enforces this at the execution layer: when `WaveNum > 1`, it verifies all agents in wave `WaveNum - 1` have completion reports with `status: complete` before creating worktrees.

### I4 — IMPL Doc is the Single Source of Truth

Completion reports, interface contract updates, and status are written to the IMPL doc. Chat output is not the record. The tool journal (E23A) complements the IMPL doc for execution history without violating I4. E42 enforces I4 at agent completion time by verifying completion reports exist in the IMPL doc.

### I5 — Agents Commit Before Reporting

Each agent commits its changes to its worktree branch before writing a completion report. Uncommitted state at report time is a protocol deviation. E42 performs post-hoc I5 commit verification at SubagentStop time. E43's `verify_worktree_compliance` hook creates an audit trail. Cross-repo agents may commit directly to the target repo's default branch.

### I6 — Role Separation

The Orchestrator does not perform Scout, Scaffold Agent, Wave Agent, or Integration Agent duties. Enforced via orchestrator prompt instructions and agent type restrictions, not via SDK validators or lifecycle hooks. Solo wave work must still be launched as an asynchronous agent — executing inline violates I6 regardless of wave size.
