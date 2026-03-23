# Parallel Tier Execution — Design Document

**Date:** 2026-03-22
**Status:** Planning
**Goal:** Execute multiple IMPLs within the same tier simultaneously, not sequentially

## Current State

Tiers group IMPLs that have no cross-dependencies. Today, IMPLs within a tier execute sequentially (IMPL A waves → IMPL B waves). This wastes time when IMPLs are file-disjoint.

## Architecture

### Three execution layers need changes:

```
┌─────────────────────────────────────────────────┐
│  Orchestrator (CLI / Web App)                    │
│  - Decides what to launch and when               │
│  - CLI: single session, context-bound            │
│  - Web: goroutine per IMPL, unlimited            │
├─────────────────────────────────────────────────┤
│  Go Engine (pkg/engine, pkg/protocol)            │
│  - RunTierLoop, FinalizeWave, MergeAgents        │
│  - Must handle concurrent IMPL state mutations   │
│  - Must validate cross-IMPL file disjointness    │
├─────────────────────────────────────────────────┤
│  Git Layer (worktrees, branches, merges)          │
│  - Already namespaced by slug                    │
│  - Merges to main must be serialized             │
│  - Baseline verification needs coordination      │
└─────────────────────────────────────────────────┘
```

## Pre-flight: Cross-IMPL Disjointness Check

Before parallel execution, validate that no file appears in file_ownership across multiple IMPLs in the same tier.

## Safety Analysis: check-tier-conflicts

**New command:** `sawtools check-tier-conflicts <program-manifest> --tier N`

This is the gatekeeper for parallel execution. It performs multi-level analysis and returns a verdict that the orchestrator acts on automatically.

### Verdicts

| Verdict | Meaning | Orchestrator action |
|---------|---------|-------------------|
| `PARALLEL_SAFE` | Fully disjoint — no shared files, no shared packages | Execute IMPLs in parallel |
| `PARALLEL_CAUTIOUS` | Disjoint files but shared packages or dependency files | Execute in parallel, run post-merge build verification after each merge |
| `SEQUENTIAL_ONLY` | Shared files or interface conflicts | Execute sequentially, report why |

### Phase 1: Static Analysis (fast, no compilation)

Runs in <1 second. Catches 90% of conflicts.

**Check 1 — File ownership disjointness:**
Union all `file_ownership` tables across IMPLs in the tier. Any file appearing in multiple IMPLs → `SEQUENTIAL_ONLY`.

**Check 2 — Package-level overlap:**
Extract the directory (Go package) of each owned file. If two IMPLs modify files in the same package (e.g., IMPL A owns `pkg/engine/runner.go`, IMPL B owns `pkg/engine/finalize.go`), they could break each other's builds even with disjoint files. Same-package overlap → `PARALLEL_CAUTIOUS`.

**Check 3 — Shared dependency files:**
Detect if multiple IMPLs are likely to modify `go.mod`, `go.sum`, `package.json`, `package-lock.json`, or similar lockfiles. These are deterministically resolvable post-merge (`go mod tidy`, `npm install`), so they escalate to `PARALLEL_CAUTIOUS` not `SEQUENTIAL_ONLY`.

**Check 4 — Interface contract collision:**
If IMPL A's interface contracts define types consumed by IMPL B's agents (or vice versa), the contracts could conflict at merge. Cross-IMPL contract references → `SEQUENTIAL_ONLY`.

### Phase 2: Compilation-based Verification (slow, definitive)

Only runs when Phase 1 returns `PARALLEL_CAUTIOUS`. Takes 30-60 seconds.

**Dry-run merge test:**
1. Create a temp branch from current HEAD
2. Apply IMPL A's scaffold/interface changes (if any)
3. Apply IMPL B's scaffold/interface changes (if any)
4. Run `go build ./...` (or project's build command)
5. If build passes → upgrade to `PARALLEL_SAFE`
6. If build fails → downgrade to `SEQUENTIAL_ONLY`
7. Clean up temp branch

This is expensive but gives a definitive answer. Most real programs won't need it — either IMPLs are clearly disjoint (different packages) or clearly overlapping (same package).

### Implementation

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

type TierFileConflict struct {
    File   string   `json:"file"`
    IMPLs  []string `json:"impls"`
}

type TierPackageOverlap struct {
    Package string   `json:"package"`    // directory path
    IMPLs   []string `json:"impls"`
    Files   []string `json:"files"`      // all files in this package across IMPLs
}

type ContractCollision struct {
    ContractName string   `json:"contract_name"`
    IMPLs        []string `json:"impls"`
}

func CheckTierConflicts(programPath string, tierNum int) (*TierConflictReport, error)
```

### Verdict Derivation Logic

```
if len(fileConflicts) > 0 || len(contractCollisions) > 0:
    → SEQUENTIAL_ONLY

else if len(packageOverlaps) > 0 || len(sharedDepFiles) > 0:
    → PARALLEL_CAUTIOUS

else:
    → PARALLEL_SAFE
```

## Go Engine Changes

### 1. RunTierParallel (new function)

```go
// pkg/engine/program_tier_loop.go
func RunTierParallel(ctx context.Context, opts RunTierOpts) (*TierResult, error)
```

Orchestrates parallel IMPL execution within a tier:
1. Run `CheckTierConflicts` — route by verdict:
   - `SEQUENTIAL_ONLY` → fall back to existing sequential `RunTierLoop`
   - `PARALLEL_SAFE` or `PARALLEL_CAUTIOUS` → proceed with parallel execution
2. For each IMPL in the tier, spawn a goroutine running the full IMPL lifecycle (scout → waves → merge)
3. Each goroutine manages its own IMPL state independently
4. Merges to main are serialized via a mutex (only one IMPL merges at a time)
5. For `PARALLEL_CAUTIOUS`: run full build verification after each serialized merge (not just at tier gate)
6. Wait for all goroutines to complete
7. For all dependency file conflicts: run `go mod tidy` / `npm install` as post-tier reconciliation
8. Run tier gate

**Key constraint:** `MergeAgents` must acquire a merge lock. Two IMPLs cannot merge simultaneously because `git merge` to main is not concurrent-safe.

### 2. Merge Serialization

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

Passed to each IMPL executor. `FinalizeWave` wraps its merge step with `mergeLock.WithLock(...)`.

### 3. Baseline Gate Coordination

E21A baseline verification runs before each wave. When IMPLs execute in parallel:
- IMPL A's Wave 1 merge changes main
- IMPL B's Wave 1 baseline check must see IMPL A's merged state

**Solution:** After each merge (serialized by lock), the next IMPL to run baseline gates gets a fresh HEAD. No code change needed — `prepare-wave` already reads current HEAD.

### 4. State Tracking

Each IMPL tracks its own state in its own IMPL doc. The PROGRAM manifest tracks aggregate status. IMPL-level state mutations are independent (different YAML files, no contention).

Program-level state updates (`update-program-impl --status`) need serialization since they modify the same PROGRAM manifest file.

```go
// Add to RunTierOpts
ProgramStateLock *sync.Mutex  // serializes PROGRAM manifest writes
```

## Web App Changes

### Already well-positioned

The web app already runs wave execution in goroutines (`pkg/api/wave_runner.go`). Parallel tier execution means:

1. **wave_runner.go:** Add `RunTierParallel` handler that spawns one goroutine per IMPL
2. **SSE events:** Already per-IMPL scoped (`program_impl_started`, `program_impl_wave_progress`). No changes needed.
3. **UI:** The nested program graph already shows per-IMPL agent status. During parallel execution, you'd see both IMPL containers animating simultaneously.

### New API endpoint

`POST /api/program/{slug}/tier/{n}/execute-parallel`

Same as existing `execute` but uses `RunTierParallel` instead of sequential loop.

Or: make the existing endpoint parallel by default when `check-tier-conflicts` passes. Add `?sequential=true` query param for opt-out.

## CLI Flow Changes

### Context window constraint

The CLI orchestrator runs inside a single Claude session. Launching agents from two IMPLs simultaneously means more concurrent agents competing for context.

**Options:**

1. **Interleaved execution (recommended):** Prepare all worktrees for both IMPLs upfront. Launch all Wave 1 agents across both IMPLs in one batch. As agents complete, merge per-IMPL (serialized). This is what we'd do naturally — it's just launching more agents at once.

2. **Separate sessions:** Each IMPL gets its own Claude session (requires multi-session orchestration, not supported in Claude Code CLI today).

3. **Hybrid:** If combined agent count exceeds threshold (e.g., 6), fall back to sequential. Most tiers won't have enough agents across IMPLs to hit this.

### Skill prompt update

The `/saw program execute` flow in saw-skill.md needs:

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

## Edge Cases

### Different wave counts across IMPLs
IMPL A has 3 waves, IMPL B has 1 wave. After B completes, A continues alone. No issue — each IMPL runs independently.

### Cross-IMPL agent failure
IMPL A's agent fails, IMPL B succeeds. IMPL A enters BLOCKED, IMPL B continues. Tier gate waits for both to complete. User resolves IMPL A independently.

### Merge conflict between IMPLs
`PARALLEL_SAFE` verdict means no file or package overlap — merge conflicts should be impossible.

`PARALLEL_CAUTIOUS` verdict means shared packages or dependency files. Merge conflicts are possible but recoverable:
- **Same package, different files:** Usually merges clean. Post-merge build verification catches compilation failures.
- **Dependency files (go.mod, go.sum, lockfiles):** Deterministically resolvable. Post-tier reconciliation runs `go mod tidy` / `npm install` to regenerate.
- **If merge fails:** The serialized merge lock means only one IMPL's merge fails. That IMPL enters BLOCKED. The other IMPL's merge already succeeded. User resolves the conflict manually (or re-runs with `SEQUENTIAL_ONLY` forced).

### PARALLEL_CAUTIOUS → build failure after merge
If Phase 1 says `PARALLEL_CAUTIOUS` and Phase 2 is skipped (or passes), but the actual merge causes a build failure:
1. The post-merge build verification catches it immediately (before the next IMPL merges)
2. Revert the failing IMPL's merge (`git revert`)
3. Mark that IMPL as BLOCKED
4. Continue with remaining IMPLs
5. Surface the failure: "IMPL X broke the build after merge. Re-run sequentially or fix the conflict."

## Implementation Priority

### Phase 1: Validation (low effort)
- `sawtools check-tier-conflicts` command
- Wire into `prepare-wave` as informational check

### Phase 2: Engine support (medium effort)
- `MergeLock` type
- `RunTierParallel` function
- Program state serialization

### Phase 3: Web app integration (low effort)
- Parallel tier handler using `RunTierParallel`
- SSE already supports this

### Phase 4: CLI integration (low effort)
- Skill prompt update for parallel agent launching
- Agent count threshold check

## Not In Scope

- Cross-tier parallelism (tiers are sequential by design — tier N+1 depends on tier N contracts)
- Cross-IMPL agent dependencies (agents only depend on other agents within the same IMPL)
- Dynamic re-tiering during execution (tier assignment is fixed at Scout time)
