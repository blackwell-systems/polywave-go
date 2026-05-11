# Parallel Tier Execution — Design Document

**Date:** 2026-03-22
**Status:** Partially Implemented
**Goal:** Execute multiple IMPLs within the same tier simultaneously, not sequentially
**Last reviewed:** 2026-03-25

## What Remains

The foundational architecture is in place. Scouts run in parallel (`LaunchParallelScouts`), IMPL branches provide wave isolation (`ProgramBranchName`, `RunWaveFull`), `ConcurrencyCap` is defined in `ProgramTier`, `ScoreTierIMPLs` returns priority-ordered slugs, and the web app already runs wave execution in goroutines with `activeRuns`/`mergingRuns` sync.Maps. `PartitionIMPLsByStatus` and `ValidateProgramImportMode` handle pre-existing IMPLs.

The gap is step 8 of `RunTierLoop` (`pkg/engine/program_tier_loop.go`) and the equivalent `for _, implSlug := range tier.Impls` loop in `pkg/api/program_runner.go` (web). Both execute IMPLs sequentially. Parallelizing them requires the three items below.

---

## Remaining Work

### 1. `check-tier-conflicts` command (low effort)

**New command:** `polywave-tools check-tier-conflicts <program-manifest> --tier N`

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
Extract the directory (Go package) of each owned file. If two IMPLs modify files in the same package -> `PARALLEL_CAUTIOUS`.

**Check 3 — Shared dependency files:**
Detect if multiple IMPLs modify `go.mod`, `go.sum`, `package.json`, `package-lock.json`, or similar lockfiles -> `PARALLEL_CAUTIOUS`.

**Check 4 — Interface contract collision:**
If IMPL A's interface contracts define types consumed by IMPL B's agents (or vice versa) -> `SEQUENTIAL_ONLY`.

#### Compilation-based Verification (optional)

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

---

### 2. `RunTierParallel` + merge serialization (medium effort)

Convert step 8 of `RunTierLoop` (`pkg/engine/program_tier_loop.go`) from the sequential `for _, slug := range tierSlugs` loop to parallel goroutines:

1. Run `CheckTierConflicts` — route by verdict
2. For each IMPL in the tier, spawn a goroutine running the full IMPL wave lifecycle
3. Each goroutine manages its own IMPL state independently
4. Merges to main are serialized via `MergeLock` (one IMPL merges at a time)
5. For `PARALLEL_CAUTIOUS`: run full build verification after each serialized merge
6. Wait for all goroutines to complete
7. Run tier gate

#### MergeLock

New type, no existing implementation. `mergingRuns` in the web app is per-slug, not a cross-IMPL merge serializer.

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

`update-program-impl --status` writes to the same PROGRAM manifest file. Needs serialization during concurrent IMPL execution.

```go
// Add to RunTierOpts
ProgramStateLock *sync.Mutex  // serializes PROGRAM manifest writes
```

Note: IMPL branch isolation already handles baseline gate coordination — each IMPL's waves merge to its own branch, so no code change needed there.

---

### 3. Web app + CLI integration (low effort)

**Web app (`pkg/api/program_runner.go`):** Convert the `for _, implSlug := range tier.Impls` loop in `runProgramTier` to parallel (goroutine per IMPL, guarded by `CheckTierConflicts` verdict). Add `MergeLock` for IMPL-branch-to-main merges during `FinalizeTier`. Optional: `?sequential=true` query param for opt-out.

**CLI skill (`polywave-skill.md`):** Update `/polywave program execute` flow:

```
Step 3b: IMPL Execution (parallel within tier)

- Run: polywave-tools check-tier-conflicts <program> --tier N
- If safe (exit 0):
  - Prepare worktrees for ALL IMPLs in the tier
  - Launch Wave 1 agents from ALL IMPLs simultaneously
  - Finalize each IMPL's wave independently (merges serialized)
  - Proceed wave-by-wave across all IMPLs
- If conflicts (exit 1):
  - Fall back to sequential execution
  - Surface conflict report to user
```

Hybrid agent count threshold: if combined agent count exceeds 6, fall back to sequential.

---

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
