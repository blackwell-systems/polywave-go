# Agent C Brief - Wave 2

**IMPL Doc:** /Users/dayna.blackwell/code/scout-and-wave-go/docs/IMPL/IMPL-planner-dag-prioritization.yaml

## Files Owned

- `pkg/engine/program_auto.go`
- `pkg/engine/program_auto_test.go`


## Task

## Role
Integrate the unblocking potential scorer into the tier execution loop in
pkg/engine/program_auto.go and add tests in pkg/engine/program_auto_test.go.

## Context
Agent A added UnblockingScore and PrioritizeIMPLs to pkg/protocol/program_prioritizer.go.
Agent B added PriorityScore, PriorityReasoning on ProgramIMPL, and ConcurrencyCap
on ProgramTier to pkg/protocol/program_types.go.

## What to implement

### 1. Add ScoreTierIMPLs to pkg/engine/program_auto.go

Implement the function matching the interface contract:
  func ScoreTierIMPLs(manifest *protocol.PROGRAMManifest, tierNumber int) []string

Logic:
1. Find the ProgramTier with Number == tierNumber. If not found, return nil.
2. Collect all IMPL slugs in the tier whose Status is "pending".
3. For each slug, call protocol.UnblockingScore(manifest, slug) to get the score.
4. Build the reasoning string: if score > 0, format as
   "unblocking(%dx+100=+%d), age(+0)" else "unblocking(0), age(+0)".
5. Write PriorityScore and PriorityReasoning back into the matching
   ProgramIMPL entry in manifest.Impls (mutate in place).
6. Call protocol.PrioritizeIMPLs(manifest, pendingSlugs) to get the ordered list.
7. Return the ordered list (highest score first).

### 2. Update AdvanceTierAutomatically to call ScoreTierIMPLs
After the existing step 4 (freeze contracts) and before returning AdvancedToNext=true,
if AdvancedToNext will be true, call ScoreTierIMPLs for the NextTier so that
priority scores are populated in the manifest before the caller launches IMPLs.
Store the ordered slug list in result.ScoredIMPLOrder (new field — add it to
TierAdvanceResult):
  ScoredIMPLOrder []string `json:"scored_impl_order,omitempty"`

This lets the web app and CLI display the launch order before execution starts.

### 3. Update TierAdvanceResult struct
Add:
  ScoredIMPLOrder []string `json:"scored_impl_order,omitempty"`

### 4. Concurrency cap awareness (informational only in this IMPL)
ScoreTierIMPLs already returns slugs in priority order. The caller (web app
or CLI) reads ConcurrencyCap from the tier and launches only the first
ConcurrencyCap slugs (or all if cap == 0). This IMPL does NOT change the
web app launch logic — that is a follow-on task. ScoreTierIMPLs returns the
full priority-ordered list; the caller is responsible for honoring the cap.
Add a comment in ScoreTierIMPLs noting this contract.

## Tests (pkg/engine/program_auto_test.go)

Add tests for ScoreTierIMPLs:
- Single pending IMPL with no downstream deps: score=0, returned slice has 1 element
- Two pending IMPLs where one unblocks the other: higher-unblocking IMPL appears first
- ScoreTierIMPLs mutates manifest.Impls PriorityScore in place (verify via assertion)
- Non-pending IMPLs (status "complete") are excluded from the scored output

Extend or add tests for AdvanceTierAutomatically to verify:
- ScoredIMPLOrder is populated in result when AdvancedToNext=true
- ScoredIMPLOrder is empty when gate fails (RequiresReview=true)

## Verification gate
cd /Users/dayna.blackwell/code/scout-and-wave-go && go build ./... && go vet ./... && go test ./pkg/engine/... -run TestScoreTier -v && go test ./pkg/engine/... -run TestAdvanceTier -v

## Constraints
- Import protocol package (already imported in program_auto.go)
- Do not modify scheduler.go or any other engine file — only program_auto.go and program_auto_test.go
- TierAdvanceResult is defined in program_auto.go — add ScoredIMPLOrder there
- Do not change existing function signatures (AdvanceTierAutomatically signature is unchanged)
- ScoreTierIMPLs must handle missing tier gracefully (return nil, not panic)



## Interface Contracts

### UnblockingScore

BFS over the reverse IMPL dependency graph starting from implSlug.
Returns the count of transitively unblocked IMPLs (those that would
become runnable once implSlug completes), weighted by UNBLOCK_BONUS.
Only considers IMPLs with status "pending" or "blocked" as unblockable.


```
// UnblockingScore computes a priority score for implSlug based on how many
// downstream IMPLs it transitively unblocks in manifest.
//
// Score formula (mirrors Formic's prioritizer):
//   score = unblocking_potential * UnblockBonusPerIMPL + age_bonus
//
// where:
//   unblocking_potential = BFS count of transitively unblocked pending/blocked IMPLs
//   UnblockBonusPerIMPL  = 100 (constant)
//   age_bonus            = min(daysSinceQueued, 10)  — tie-breaker, 0 if no timestamp
//
// Returns 0 if implSlug has no downstream dependents or is not found.
func UnblockingScore(manifest *PROGRAMManifest, implSlug string) int

```

### PrioritizeIMPLs

Re-orders a slice of IMPL slugs by descending UnblockingScore.
Stable sort — equal scores preserve input order (FIFO tiebreak).
Used by the tier execution loop to sequence IMPL launches.


```
// PrioritizeIMPLs returns implSlugs sorted by descending UnblockingScore.
// Stable: equal-scored IMPLs retain their original order.
func PrioritizeIMPLs(manifest *PROGRAMManifest, implSlugs []string) []string

```

### IMPLPriorityScore (struct field on ProgramIMPL)

Optional computed field added to ProgramIMPL. Written by the Planner
(or populated lazily by the engine) so the web UI can display priority
ordering without recomputing on every render.


```
// In ProgramIMPL struct — add these two fields:
PriorityScore        int    `yaml:"priority_score,omitempty" json:"priority_score,omitempty"`
PriorityReasoning    string `yaml:"priority_reasoning,omitempty" json:"priority_reasoning,omitempty"`

```

### ConcurrencyCap (struct field on ProgramTier)

Optional cap on simultaneous IMPL launches within a tier. When 0 or
absent, all pending IMPLs in the tier launch concurrently (current
behavior). When set, the tier execution loop launches at most
ConcurrencyCap IMPLs at a time, in priority order.


```
// In ProgramTier struct — add this field:
ConcurrencyCap int `yaml:"concurrency_cap,omitempty" json:"concurrency_cap,omitempty"`

```

### ScoreTierIMPLs

Computes UnblockingScore for all pending IMPLs in a tier and writes
PriorityScore + PriorityReasoning back into manifest.Impls in place.
Called by the tier execution loop before launching IMPLs.
Returns the ordered list of IMPL slugs (highest score first).


```
// ScoreTierIMPLs scores all pending IMPLs in tierNumber and returns them
// sorted by descending priority. Also updates PriorityScore and
// PriorityReasoning on each ProgramIMPL entry in manifest.Impls.
func ScoreTierIMPLs(manifest *protocol.PROGRAMManifest, tierNumber int) []string

```



## Quality Gates

Level: standard

- **build**: `go build ./...` (required: true)
- **lint**: `go vet ./...` (required: true)
- **test**: `go test ./...` (required: true)

