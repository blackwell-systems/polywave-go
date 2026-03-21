# Determinism Roadmap — Reducing AI Reliance in the Engine

**Date**: 2026-03-20
**Goal**: Move validation from post-execution to runtime/pre-execution. Every protocol invariant should be enforced by code, not by trusting agents to follow instructions.

---

## CRITICAL — System correctness depends on AI behavior

### C1: Runtime I1 constraint enforcement in tool layer
**Files**: `pkg/engine/constraints.go`, `pkg/tools/workshop.go`
**Issue**: File ownership constraints (I1) are built from the manifest and passed to agents, but tools don't reject writes to non-owned files. Violations only caught at finalize.
**Risk**: Agent commits files outside ownership scope → entire wave rolled back.
**Fix**: Add `ConstraintEnforcer` to `file.write` tool. Before executing write, check `constraint.OwnedFiles[filePath]`. Return error if not owned.
**New file**: `pkg/tools/constraint_enforcer.go`
**Effort**: Medium (2-3 days)

### C2: Runtime I2 freeze enforcement
**Files**: `pkg/engine/constraints.go`, `pkg/protocol/freeze.go`
**Issue**: Frozen paths (scaffolds, contracts) are tracked in constraints but tools don't reject writes to them. `ValidateFreeze()` only runs post-execution.
**Risk**: Agent edits scaffold or interface contract → breaks I2 before detection → wave fails validation.
**Fix**: In tool layer, before `file.write`: `if constraint.FrozenPaths[filePath] { return error }`.
**Effort**: Low (same PR as C1)

### C3: Cross-validate completion reports against git state
**Files**: `pkg/protocol/manifest.go` (SetCompletionReport), `pkg/orchestrator/orchestrator.go`
**Issue**: `SetCompletionReport()` accepts agent-provided `Commit`, `FilesChanged`, `FilesCreated`, `Worktree`, `Branch` without checking they match reality.
**Risk**: Agent reports fake SHA → CI gates reference wrong commit. Agent reports fake file changes → gate logic sees wrong changed-file set. Agent reports wrong worktree → cleanup removes wrong directory.
**Fix**: New `ValidateCompletionReportClaims()` function:
- Verify commit SHA exists and is on agent's branch
- Verify `FilesChanged ⊆ (owned files ∪ frozen paths)`
- Verify `FilesCreated` actually exist
- Verify worktree path exists (cross-repo)
- Call in `SetCompletionReport` or immediately after agent completes
**New file**: `pkg/protocol/completion_validator.go`
**Effort**: Medium (2 days)

### C4: Pre-merge commit verification gate (I5)
**Files**: `pkg/protocol/commit_verify.go`, `pkg/engine/finalize.go`
**Issue**: `VerifyCommits()` runs AFTER agents complete at finalize stage. If agent writes code but forgets `git commit`, verification fails too late (after merge attempt).
**Risk**: Agent's work lost in worktree. Wave finalization fails. Manual intervention required.
**Fix**: Move `VerifyCommits` before `MergeAgents()` as a mandatory pre-merge gate. Make it fatal — if any agent has no commits, stop before merge.
**Effort**: Low (1 day)

### C5: Scout write boundaries at runtime (I6)
**Files**: `pkg/engine/runner.go`, `pkg/engine/constraints.go`
**Issue**: I6 (Scout writes only to `docs/IMPL/`) validated AFTER Scout completes via hooks. If validation fails, Scout's work is already committed.
**Risk**: Scout writes to `src/` or other files. Violation undetected until post-execution. Manifest corrupt.
**Fix**: Add `AllowedPathPrefixes: ["docs/IMPL/IMPL-"]` constraint with `EnforceAtRuntime: true` flag for scout role. Tools reject writes outside prefix.
**Effort**: Low (same constraint enforcer as C1/C2)

### C6: Schema validation on agent output
**Files**: `pkg/engine/runner_wave_structured.go`, `pkg/orchestrator/orchestrator.go`
**Issue**: When agents write completion reports, minimal verification of reported file lists, commit counts, or other claims. Structure is validated but content claims are not.
**Risk**: False `FilesChanged`/`FilesCreated` → false ScanStubs results → incorrect gate results → integration validation errors.
**Fix**: Add deterministic post-execution validation layer. Verify every claim field against git state before accepting the report.
**Effort**: Medium (included in C3 implementation)

---

## HIGH — Missing validation that could cause silent failures

### H1: File ownership coverage validation
**Files**: `pkg/protocol/validation.go`
**Issue**: `validateI1DisjointOwnership()` checks no file is owned by 2+ agents, but does NOT check: are all agent-changed files actually in the ownership table? Are there dead ownership entries for non-existent files?
**Risk**: Agents leave unowned "bonus" files. Gates and integration validation can't interpret them. Silent correctness bug.
**Fix**: New `ValidateFileOwnershipCoverage()` — for each agent, verify `created-files ⊆ owned-files ∪ frozen-paths`. Run in `validate-impl` or `prepare-wave`.
**Effort**: Low (1 day)

### H2: Dynamic dependency availability verification
**Files**: `pkg/protocol/validation.go`
**Issue**: `validateI2AgentDependencies()` checks DAG statically but does NOT verify at wave execution time that dependency outputs actually exist and are available.
**Risk**: Agent declares dependency on agent B, but B's wave hasn't produced outputs. Agent hallucinates or uses stale data. No error raised.
**Fix**: New `VerifyDependenciesAvailable()` — for each agent in wave N, verify each dependency has completion report and produced output files. Call in `prepare-wave` pre-flight.
**New file**: `pkg/protocol/dependency_verifier.go`
**Effort**: Low-Medium (1-2 days)

### H3: Gate input validation against actual changes
**Files**: Gate execution code, `pkg/engine/finalize.go`
**Issue**: Gates (E21) execute post-merge but no validation that gate input (changed files) matches agent's actual changes. Gate may receive stale or fabricated file list.
**Risk**: Gate passes when it shouldn't (or vice versa). Bad code merged because gate data was wrong.
**Fix**: New `ValidateGateInputs()` — get reported `FilesChanged` from completion report, compare against actual `git diff base..merge-commit`. Pass actual file list to gate, not reported list.
**Effort**: Low (1 day)

### H4: Pre-merge file ownership consistency check
**Files**: `pkg/protocol/merge_agents.go`
**Issue**: When merging, if conflicts occur, system uses file ownership to auto-resolve. But does NOT verify ownership table is correct before using it. If ownership is wrong, auto-resolution accepts bad merges.
**Risk**: Silent inconsistency between merge result and manifest.
**Fix**: New `PreMergeValidation()` — verify every file_ownership entry references an existing agent in the current wave. Call at start of `MergeAgents()`.
**Effort**: Low (1 day)

---

## MEDIUM — Non-deterministic flows that should be structured

### M1: Atomic RunWave transaction wrapper
**Files**: `pkg/orchestrator/orchestrator.go`, `pkg/engine/run_wave_full.go`
**Issue**: `RunWave()` advances state manually at multiple points. If a step fails partway, IMPL doc state may be inconsistent with reality (e.g., merge completes but state transition fails).
**Risk**: Partial-failure state inconsistency. Manual recovery requires understanding state machine.
**Fix**: Create `RunWaveTransaction` wrapper. All state mutations go through transaction. Only commit state after ALL steps succeed. On failure, roll back to start state.
**New file**: `pkg/orchestrator/run_wave_atomic.go`
**Effort**: Medium (2-3 days)

### M2: Integration validation enforcement (E25)
**Files**: `pkg/protocol/integration.go`, `pkg/engine/finalize.go`
**Issue**: `ValidateIntegration()` checks for unconnected exports but is non-fatal. Errors logged but don't block merge or build.
**Risk**: Integration breaks silently. Orphaned exports left by agents.
**Fix**: Make integration validation a configurable gate: `FinalizeWaveOpts.EnforceIntegrationValidation bool`. When true, unconnected exports fail the wave.
**Effort**: Low (1 day)

### M3: Mandatory stub detection (E20)
**Files**: `pkg/protocol/stubs.go`, `pkg/engine/finalize.go`
**Issue**: `ScanStubs()` looks for TODO/FIXME but is non-fatal. Agents can ship incomplete code.
**Risk**: Incomplete implementations merged. Later waves find stubs where they expect complete code.
**Fix**: `FinalizeWaveOpts.RequireNoStubs bool`. When true, any TODO/FIXME fails the wave.
**Effort**: Low (0.5 day)

### M4: Wave base commit validation
**Files**: `pkg/protocol/worktree.go`
**Issue**: Base commit recorded when creating worktrees but no verification it's on main branch or that all worktrees use the same base. Fallback logic uses HEAD if base missing.
**Risk**: Base commit detached or on wrong branch. Commit counts off. Verification uses wrong baseline.
**Fix**: New `ValidateWaveBaseCommit()` — verify commit exists and is ancestor of main. Call in `prepare-wave` or `finalize-wave`.
**Effort**: Low (0.5 day)

---

## LOW — Style/consistency improvements

### L1: Missing agent prompt files should error, not fallback
**Files**: `pkg/engine/runner.go` (lines 60-91)
**Issue**: If `scout.md` or `planner.md` are missing, hardcoded minimal fallback prompt used. No warning logged.
**Risk**: Scout runs with degraded instructions. Output quality degrades silently.
**Fix**: Make missing prompt files an error: `return fmt.Errorf("scout.md not found at %s", path)`.
**Effort**: Low (0.5 day)

### L2: Standardize validation error messages
**Files**: All validation files in `pkg/protocol/`
**Issue**: Inconsistent error context. Some say "agent X not found", others "agent X (wave Y) not found".
**Fix**: Standardize `ValidationError` to always include IMPL slug, wave number, agent ID, and field name.
**Effort**: Low-Medium (1-2 days)

### L3: Consolidate constraint building
**Files**: `pkg/engine/constraints.go`, `pkg/tools/constraints.go`
**Issue**: Constraints built in engine but tools expect `tools.Constraints`. Mapping logic split.
**Fix**: Single `BuildConstraints()` function in `pkg/protocol/constraints_builder.go`.
**Effort**: Low (1 day)

### L4: Consolidate multi-repo resolution logic
**Files**: `pkg/protocol/commit_verify.go`, `pkg/protocol/merge_agents.go`, `pkg/protocol/worktree.go`
**Issue**: Same "resolve agent repo from file_ownership" logic replicated in 3+ files.
**Fix**: Extract `ResolveAgentRepo()` into `pkg/protocol/multi_repo.go`.
**Effort**: Low (1 day)

### L5: Typed manifest vs raw YAML inconsistency
**Files**: `pkg/engine/constraints.go` (lines 143-169)
**Issue**: Integration connectors loaded separately via raw YAML because they're not yet on `IMPLManifest` struct. Two sources of truth.
**Fix**: Add `IntegrationConnectors` to `IMPLManifest` and remove raw YAML load.
**Effort**: Low (0.5 day)

---

## Implementation Priority

### Phase 1: Runtime constraint enforcement (Week 1)
- **C1 + C2 + C5**: Single PR — `constraint_enforcer.go` in tool layer
- Covers I1, I2, I6 at runtime
- Highest-impact change: prevents entire wave failures from agent writes

### Phase 2: Completion report validation (Week 1-2)
- **C3 + C6**: `completion_validator.go` — cross-validate all agent claims against git
- **C4**: Move `VerifyCommits` to pre-merge gate
- **H1**: File ownership coverage check

### Phase 3: Pre-execution verification (Week 2)
- **H2**: Dependency availability verification in `prepare-wave`
- **H3**: Gate input validation against actual `git diff`
- **H4**: Pre-merge ownership consistency check
- **M4**: Base commit validation

### Phase 4: Transactional execution (Week 2-3)
- **M1**: Atomic `RunWaveTransaction` wrapper
- **M2**: Configurable integration enforcement
- **M3**: Configurable stub detection gate

### Phase 5: Cleanup (Week 3)
- **L1-L5**: Error standardization, deduplication, prompt enforcement
