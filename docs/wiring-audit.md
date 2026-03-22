# Wiring Audit: Unwired Code in scout-and-wave-go

**Date:** 2026-03-22
**Scope:** All exported functions, types, and patterns in `pkg/` and `cmd/saw/`
**Method:** Static analysis of callers across production code (excluding `_test.go` and `.claude/worktrees/`)

## Summary

| Pattern | Gaps Found |
|---|---|
| 1. Set\*Func injection setters never called | 1 critical |
| 2. New\* constructors with no production callers | 5 |
| 3. Register\* functions not called from init/startup | 0 |
| 4. CLI commands defined but not in main.go AddCommand | 4 commands (1 parent + 3 sub) |
| 5. Exported functions with zero production callers | 30+ |
| 6. Interface implementations not registered | 1 (observability Store has no impl) |

---

## Critical Gaps (implemented but never runs in production)

### C1. `SetPrioritizeAgentsFunc` never called -- agent scheduling is a no-op

**File:** `pkg/orchestrator/orchestrator.go:138`
**Impact:** The scheduler in `pkg/engine/scheduler.go` is implemented but never injected into the orchestrator. `prioritizeAgentsFunc` stays as the default no-op, meaning agents always run in YAML declaration order regardless of dependency analysis.
**Fix:** Add `orchestrator.SetPrioritizeAgentsFunc(PrioritizeAgents)` to `pkg/engine/engine.go` `init()`, alongside the existing `SetParseIMPLDocFunc` and `SetRunWaveAgentStructuredFunc` calls.
**Status:** Known; fix in progress.

### C2. `ClosedLoopGateRetry` never called from any CLI command or engine workflow

**File:** `pkg/engine/closed_loop_gate.go`
**Impact:** R3 pre-merge per-agent gate retry is fully implemented (creates an agent runner, builds retry context, re-runs the agent in its worktree) but nothing invokes it. When a per-agent gate fails, the system reports failure instead of auto-retrying.
**Fix:** Wire into `finalize-wave` or `run-gates` when an individual agent gate fails. The `ClosedLoopRetryOpts` struct is ready to use.

### C3. `FinalizeIMPLEngine` never called from any CLI or web handler

**File:** `pkg/engine/finalize_impl.go:21`
**Impact:** The engine-level wrapper around `protocol.FinalizeIMPL` (designed for webapp automation with context cancellation and SSE streaming compatibility) is dead code. The CLI's `finalize-impl` command calls `protocol.FinalizeIMPL` directly, bypassing context support.
**Fix:** Web app's Scout completion handler should call `engine.FinalizeIMPLEngine` instead of (or in addition to) `protocol.FinalizeIMPL`. CLI is fine as-is.

### C4. `AdvanceTierAutomatically` never called from CLI or web

**File:** `pkg/engine/program_auto.go:48`
**Impact:** Automatic tier advancement (check gate, freeze contracts, advance to next tier, score IMPLs) is implemented but nothing triggers it. Program tier progression requires manual intervention when it should be automatable after wave completion.
**Fix:** Wire into `finalize-tier` CLI command or call from daemon's post-wave-merge handler.

### C5. `AutoTriggerReplan` never called from CLI or web

**File:** `pkg/engine/program_tier_loop.go`
**Impact:** Automatic replan triggering (detects when too many IMPLs in a tier have failed and triggers a program replan) is dead code. Failed programs accumulate without automated recovery.
**Fix:** Wire into daemon loop or `program-execute` command's post-wave handler.

### C6. `SyncProgramStatusFromDisk` never called from CLI or web

**File:** `pkg/engine/program_progress.go`
**Impact:** Disk-to-memory status synchronization for programs is implemented but never runs. Program status tracking may drift from on-disk reality.
**Fix:** Call from `program-status` CLI command and/or daemon startup.

### C7. Four CLI commands defined but never added to `main.go`

**Files and commands:**
- `cmd/saw/update_program_impl_cmd.go` -- `newUpdateProgramImplCmd()` (update-program-impl)
- `cmd/saw/update_program_state_cmd.go` -- `newUpdateProgramStateCmd()` (update-program-state)
- `cmd/saw/pre_wave_gate_cmd.go` -- `newPreWaveGateCmd()` (pre-wave-gate)
- `cmd/saw/queue_cmd.go` -- `newQueueCmd()` (queue, with subcommands add/list/next)

**Impact:** These commands compile into the binary but are unreachable. Users and orchestrators cannot invoke them.
**Fix:** Add to `main.go` `rootCmd.AddCommand(...)` block.

### C8. Observability subsystem has no `Store` implementation

**File:** `pkg/observability/store.go:10`
**Impact:** The `Store` interface defines `RecordEvent`, `QueryEvents`, and `ComputeRollup` but no concrete implementation exists anywhere in the codebase. This means:
- `NewEmitter` is dead code (needs a Store to function)
- All `New*Event` constructors work but events can never be persisted
- `GetAgentHistory`, `GetFailurePatterns`, all `Compute*Rollup` functions, and `ComputeTrend` require a Store and are uncallable
- The `metrics` and `query` CLI commands likely fail at runtime

**Fix:** Implement a concrete Store (e.g., SQLite-backed or JSON-file-backed) and wire it into the engine/daemon startup.

### C9. `ScoutCorrectionLoop` never called

**File:** `pkg/engine/scout_correction_loop.go`
**Impact:** The Scout self-correction loop (re-runs Scout with feedback when IMPL validation fails) is implemented but nothing invokes it. Scout validation failures require manual re-runs.
**Fix:** Wire into `run-scout` command when validation step fails.

### C10. `RunSingleWave` and `RunSingleAgent` never called

**File:** `pkg/engine/runner.go`
**Impact:** Engine-level functions for running a single wave or single agent (designed for web app granular control) are dead code. Not called from CLI (which uses different paths) or web app.
**Fix:** These are likely intended for web app consumption. Verify `scout-and-wave-web` imports them, or wire into CLI for testing.

---

## Advisory Gaps (exported, tested, but no production caller in this repo)

These may be intentionally exported for `scout-and-wave-web` or future use.

### Observability query/rollup functions (no Store impl blocks all of these)
- `GetAgentHistory` -- `pkg/observability/query.go`
- `GetFailurePatterns` -- `pkg/observability/query.go`
- `ComputeCostRollup` -- `pkg/observability/rollups.go`
- `ComputeSuccessRateRollup` -- `pkg/observability/rollups.go`
- `ComputeRetryRollup` -- `pkg/observability/rollups.go`
- `ComputeTrend` -- `pkg/observability/rollups.go`

### Protocol helper functions (exported API surface)
- `ValidateCompletionReportClaims` -- completion report cross-validation (15 tests, 0 production callers)
- `ValidateGateInputs` -- gate input validation
- `LoadProjectMemory` / `SaveProjectMemory` / `AddCompletedFeature` -- project memory CRUD
- `GenerateManifestSchema` / `ValidateManifestJSON` -- JSON Schema validation
- `SetFreezeTimestamp` -- freeze timestamp setter
- `IsSoloWave` / `IsWaveComplete` / `IsFinalWave` -- wave helper predicates
- `LookupErrorCode` / `AllErrorCodes` / `MigrateErrorCode` -- error code utilities
- `ShouldRetry` / `MaxRetriesWithReactions` / `ShouldRetryWithReactions` / `ValidFailureType` -- failure routing
- `ValidateBytes` -- raw YAML validation entry point

### Orchestrator functions
- `RouteFailureWithReactions` / `MaxAttemptsFor` -- failure routing with reaction support
- `WriteJournalEntry` -- journal integration helper

### Tools package (SDK surface for external consumers)
- `NewAnthropicAdapter` / `NewOpenAIAdapter` / `NewBedrockAdapter` -- tool format adapters
- `PermissionMiddleware` / `WithPermissions` -- tool permission middleware
- `ConstrainedTools` -- constrained tool workshop

### Pipeline package
- `DefaultRegistry` / `WavePipeline` -- pipeline registry and wave pipeline definition

### Other
- `NewPoller` -- git activity poller (0 callers, 0 tests -- fully dead)
- `NewGateResultCache` -- gate result cache (tests only)
- `NewJournalIntegration` -- journal integration (tests only)
- `CleanupExpired` / `ListArchives` -- journal archive management
- `WriteRequirementsFile` -- interview requirements writer
- `ValidateRequiredField` / `HandleBackCommand` -- interview helpers
- `SupportedLanguages` -- build diagnostics language list
- `StopDaemon` -- daemon stop helper (0 callers, 0 tests)
- `BuildWaveConstraints` / `BuildIntegratorConstraints` -- constraint builders (tests only)

---

## False Positives (look unwired but are fine)

### Called internally within same package (different file)
- `ValidateSM02TransitionGuards` -- called from `manifest.go` transition logic
- `UpdateIMPLStatusBytes` -- called from `updater.go` public function
- `ValidateP1FileDisjointness` -- called from `program_validation.go` validator
- `DetectLockFiles` / `NormalizePackageName` -- called from `checker.go` internals
- `ProgramWorktreeDir` -- called from `CreateProgramWorktrees`
- `ResolveTargetRepos` -- called internally from repo resolution logic
- `PopulateVerificationGates` -- called from `FinalizeIMPL`
- `PreMergeValidation` -- called from merge functions
- `ScoreTierIMPLs` -- called from `AdvanceTierAutomatically` (which is itself unwired, see C4)
- `FormatVerificationBlock` / `DetermineFocusedTestPattern` -- called from gate populator

### Register\* functions properly wired via init()
- `RegisterPatterns` -- called from `init()` in go\_patterns.go, js\_patterns.go, rust\_patterns.go, python\_patterns.go
- `RegisterCompiler` -- called from `init()` in compiler.go
- `RegisterParser` -- called from `init()` in gosum.go, cargolock.go, packagelock.go, poetrylock.go

### Set\*Func properly wired
- `SetValidateInvariantsFunc` -- called from `pkg/orchestrator/orchestrator.go` `init()`
- `SetParseIMPLDocFunc` -- called from `pkg/engine/engine.go` `init()`
- `SetRunWaveAgentStructuredFunc` -- called from `pkg/engine/engine.go` `init()`

### Queue subcommands
- `newQueueAddCmd` / `newQueueListCmd` / `newQueueNextCmd` -- added to parent `newQueueCmd()` internally. But the parent `newQueueCmd()` itself is not in main.go (see C7).

---

## Recommended Fix Priority

1. **C7** (missing CLI commands) -- trivial 4-line fix in main.go, unlocks queue/program-state/pre-wave-gate commands
2. **C1** (SetPrioritizeAgentsFunc) -- 1-line fix, already in progress
3. **C8** (Store implementation) -- medium effort, unblocks entire observability subsystem
4. **C2** (ClosedLoopGateRetry) -- wire into finalize-wave, enables auto-retry
5. **C4 + C5 + C6** (program automation) -- wire into daemon/CLI, enables automated program tier progression
6. **C9** (ScoutCorrectionLoop) -- wire into run-scout, enables self-healing Scout
7. **C3** (FinalizeIMPLEngine) -- web app integration, low urgency for CLI-only usage
8. **C10** (RunSingleWave/Agent) -- web app integration
