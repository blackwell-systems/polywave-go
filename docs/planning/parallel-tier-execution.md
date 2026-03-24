# Parallel Tier Execution — Design Document

**Date:** 2026-03-22
**Status:** Partially Implemented
**Goal:** Execute multiple IMPLs within the same tier simultaneously, not sequentially
**Last reviewed:** 2026-03-24

## Implementation Status Summary

The foundational architecture for parallel tier execution is largely in place. The protocol mandates intra-tier parallelism (E28, P1), and the engine already implements parallel Scout launching and IMPL branch isolation. What remains is the runtime parallel IMPL *wave* execution within a tier and the safety analysis tooling.

| Component | Status | Evidence |
|-----------|--------|----------|
| Protocol rules (E28, P1, P1+, P3) | **Done** | `protocol/execution-rules.md` E28: "IMPLs within the same tier execute in parallel without coordination"; `protocol/program-invariants.md` P1/P1+ enforce disjointness |
| Parallel Scout launching (E31) | **Done** | `pkg/engine/program_parallel_scout.go`: `LaunchParallelScouts` uses goroutines + `sync.WaitGroup` to launch N Scouts concurrently; wired via `program_wire_init.go` |
| Sequential tier loop (E28 framework) | **Done** | `pkg/engine/program_tier_loop.go`: `RunTierLoop` iterates tiers, calls `launchParallelScoutsFunc`, executes IMPL waves, runs tier gates (E29), freezes contracts (E30), auto-replans (E34) |
| IMPL branch isolation (E28B) | **Done** | `RunTierLoop` creates per-IMPL branches via `ProgramBranchName()`, threads `MergeTarget` through `RunWaveFull` so wave merges land on IMPL branches, not main |
| DAG prioritization + ConcurrencyCap | **Done** | `pkg/protocol/program_types.go`: `ConcurrencyCap` on `ProgramTier`; `pkg/engine/program_auto.go`: `ScoreTierIMPLs` returns priority-ordered slugs; callers told to honor `ConcurrencyCap` |
| Web app goroutine-per-IMPL infra | **Done** | `pkg/api/wave_runner.go` launches wave execution in background goroutines; `pkg/api/server.go` has `activeRuns`, `mergingRuns` sync.Maps for concurrency guards; SSE events already per-IMPL scoped |
| Pre-existing IMPL handling (E28A) | **Done** | `PartitionIMPLsByStatus` in `program_tier_loop.go`; `ValidateProgramImportMode` for pre-existing IMPLs |
| `check-tier-conflicts` command | **Not started** | No `tier_conflicts.go`, no `CheckTierConflicts` function, no CLI command |
| `RunTierParallel` function | **Not started** | `RunTierLoop` step 8 iterates `tierSlugs` sequentially in a `for` loop — no goroutines for IMPL wave execution within a tier |
| `MergeLock` / merge serialization | **Not started** | No `merge_lock.go`; web app uses `mergingRuns` sync.Map for per-slug conflict guards but not cross-IMPL merge serialization |
| `ProgramStateLock` | **Not started** | No mutex for serializing PROGRAM manifest writes during concurrent IMPL execution |
| Web app parallel tier endpoint | **Not started** | `program_runner.go` `runProgramTier` iterates IMPLs sequentially; no parallel handler |
| CLI skill prompt for parallel launch | **Not started** | `saw-skill.md` does not reference `check-tier-conflicts` or parallel IMPL execution |

## What "Partially Implemented" Means

The system is architecturally ready for parallel tier execution:

1. **Scouts already run in parallel** within a tier via `LaunchParallelScouts` (goroutine per Scout, `sync.WaitGroup`, `sync.Mutex` for result collection).

2. **IMPL branches provide isolation** so that concurrent IMPL wave execution would not conflict at the git level — each IMPL's waves merge to its own branch, and IMPL branches merge to main only at tier finalization.

3. **ConcurrencyCap is defined and plumbed** through the data model, but no caller yet enforces it at runtime (all callers have "the caller is responsible for honoring ConcurrencyCap" comments).

4. **The web app already runs wave execution in goroutines** with concurrency guards (`sync.Map` for `activeRuns`, `mergingRuns`). Extending this to launch multiple IMPLs concurrently within a tier is incremental.

The gap is specifically in step 8 of `RunTierLoop` (the `for _, slug := range tierSlugs` loop) and the equivalent `for _, implSlug := range tier.Impls` loop in `program_runner.go`. These loops execute IMPLs sequentially. Converting them to parallel execution requires the safety analysis (check-tier-conflicts) and merge serialization (MergeLock) described below.

---

## Remaining Work

### Phase 1: Safety Analysis — `check-tier-conflicts` (low effort)

**New command:** `sawtools check-tier-conflicts <program-manifest> --tier N`

Gatekeeper for parallel execution. Returns a verdict the orchestrator acts on.

#### Verdicts

| Verdict | Meaning | Orchestrator action |
|---------|---------|-------------------|
| `PARALLEL_SAFE` | Fully disjoint — no shared files, no shared packages | Execute IMPLs in parallel |
| `PARALLEL_CAUTIOUS` | Disjoint files but shared packages or dependency files | Execute in parallel, run post-merge build verification after each merge |
| `SEQUENTIAL_ONLY` | Shared files or interface conflicts | Execute sequentially, report why |

#### Static Analysis Checks

**Check 1 — File ownership disjointness:**
Union all `file_ownership` tables across IMPLs in the tier. Any file appearing in multiple IMPLs -> `SEQUENTIAL_ONLY`.

**Check 2 — Package-level overlap:**
Extract the directory (Go package) of each owned file. If two IMPLs modify files in the same package, same-package overlap -> `PARALLEL_CAUTIOUS`.

**Check 3 — Shared dependency files:**
Detect if multiple IMPLs modify `go.mod`, `go.sum`, `package.json`, `package-lock.json`, or similar lockfiles. These are deterministically resolvable post-merge -> `PARALLEL_CAUTIOUS`.

**Check 4 — Interface contract collision:**
If IMPL A's interface contracts define types consumed by IMPL B's agents (or vice versa) -> `SEQUENTIAL_ONLY`.

#### Compilation-based Verification (Phase 2, optional)

Only runs when static analysis returns `PARALLEL_CAUTIOUS`. Dry-run merge test: create temp branch, apply scaffold changes from each IMPL, run `go build ./...`. If build passes -> upgrade to `PARALLEL_SAFE`. If build fails -> downgrade to `SEQUENTIAL_ONLY`.

#### Proposed Types

**File:** `pkg/protocol/tier_conflicts.go`

```go
type TierConflictVerdict string

const (
    VerdictParallelSafe     TierConflictVerdict = "PARALLEL_SAFE"
    VerdictParallelCautious TierConflictVerdict = "PARALLEL_CAUTIOUS"
    VerdictSequentialOnly   TierConflictVerdict = "SEQUENTIAL_ONLY"
)

type TierConflictReport struct {
    TierNumber      int                    `json:"tier_number"`
    IMPLCount       int                    `json:"impl_count"`
    Verdict         TierConflictVerdict    `json:"verdict"`
    FileConflicts   []TierFileConflict     `json:"file_conflicts,omitempty"`
    PackageOverlaps []TierPackageOverlap   `json:"package_overlaps,omitempty"`
    SharedDepFiles  []string               `json:"shared_dep_files,omitempty"`
    ContractCollisions []ContractCollision  `json:"contract_collisions,omitempty"`
}

func CheckTierConflicts(programPath string, tierNum int) (*TierConflictReport, error)
```

### Phase 2: Engine — `RunTierParallel` + merge serialization (medium effort)

#### RunTierParallel

Convert the sequential IMPL loop in `RunTierLoop` step 8 to parallel execution:

1. Run `CheckTierConflicts` — route by verdict
2. For each IMPL in the tier, spawn a goroutine running the full IMPL wave lifecycle
3. Each goroutine manages its own IMPL state independently
4. Merges to main are serialized via `MergeLock` (only one IMPL merges at a time)
5. For `PARALLEL_CAUTIOUS`: run full build verification after each serialized merge
6. Wait for all goroutines to complete
7. Run tier gate

#### MergeLock

```go
// pkg/protocol/merge_lock.go
type MergeLock struct {
    mu sync.Mutex
}

func (ml *MergeLock) WithLock(fn func() error) error {
    ml.mu.Lock()
    defer ml.mu.Unlock()
    return fn()
}
```

#### ProgramStateLock

Program-level state updates (`update-program-impl --status`) need serialization since they modify the same PROGRAM manifest file.

```go
// Add to RunTierOpts
ProgramStateLock *sync.Mutex  // serializes PROGRAM manifest writes
```

#### Baseline Gate Coordination

No code change needed. IMPL branch isolation (E28B) already solves this — each IMPL's waves merge to its own branch, so baseline checks run against the IMPL branch state, not main.

### Phase 3: Web App Integration (low effort)

The web app is well-positioned:
- `wave_runner.go` already launches execution in background goroutines
- SSE events are already per-IMPL scoped (`program_impl_started`, `program_impl_wave_progress`)
- `mergingRuns` sync.Map already guards concurrent merge operations per slug
- UI nested program graph already shows per-IMPL agent status

Changes needed:
1. `program_runner.go`: Convert `runProgramTier` IMPL loop from sequential to parallel (goroutine per IMPL, guarded by `CheckTierConflicts` verdict)
2. Add `MergeLock` to serialize IMPL-branch-to-main merges during `FinalizeTier`
3. Optionally: `?sequential=true` query param for opt-out on existing endpoint

### Phase 4: CLI Integration (low effort)

Update `/saw program execute` flow in `saw-skill.md`:

```
Step 3b: IMPL Execution (parallel within tier)

- Run: sawtools check-tier-conflicts <program> --tier N
- If safe (exit 0):
  - Prepare worktrees for ALL IMPLs in the tier
  - Launch Wave 1 agents from ALL IMPLs simultaneously
  - Finalize each IMPL's wave independently (merges serialized)
  - Proceed wave-by-wave across all IMPLs
- If conflicts (exit 1):
  - Fall back to sequential execution
  - Surface conflict report to user
```

Hybrid agent count threshold: if combined agent count exceeds 6, fall back to sequential (most tiers will not hit this).

## Edge Cases

### Different wave counts across IMPLs
IMPL A has 3 waves, IMPL B has 1 wave. After B completes, A continues alone. No issue — each IMPL runs independently on its own branch.

### Cross-IMPL agent failure
IMPL A's agent fails, IMPL B succeeds. IMPL A enters BLOCKED, IMPL B continues. Tier gate waits for both to complete. User resolves IMPL A independently.

### Merge conflict between IMPLs
`PARALLEL_SAFE` verdict means no file or package overlap — merge conflicts should be impossible. `PARALLEL_CAUTIOUS` means shared packages or dependency files — merge conflicts are possible but recoverable via post-tier reconciliation (`go mod tidy` / `npm install`).

### PARALLEL_CAUTIOUS build failure after merge
Post-merge build verification catches it immediately. Revert the failing IMPL's merge, mark IMPL as BLOCKED, continue with remaining IMPLs.

## Not In Scope

- Cross-tier parallelism (tiers are sequential by design — P3)
- Cross-IMPL agent dependencies (agents only depend on other agents within the same IMPL)
- Dynamic re-tiering during execution (tier assignment is fixed at Scout time)
