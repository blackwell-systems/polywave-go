# Research: Completion Report Unification (Finding 8)

> Generated 2026-03-26. Deep research by Explore agent before IMPL design.

---

## 1. CompletionReport Struct

**Location:** `pkg/protocol/types.go:101-118`

| Field | Type | JSON/YAML Tag | Valid Values | Required? |
|---|---|---|---|---|
| Status | string | `status` | "complete", "partial", "blocked" | Required |
| Worktree | string | `worktree,omitempty` | any path | Optional |
| Branch | string | `branch,omitempty` | any git branch name | Optional |
| Commit | string | `commit,omitempty` | git SHA | Optional |
| FilesChanged | []string | `files_changed,omitempty` | file paths | Optional |
| FilesCreated | []string | `files_created,omitempty` | file paths | Optional |
| InterfaceDeviations | []InterfaceDeviation | `interface_deviations,omitempty` | structured | Optional |
| OutOfScopeDeps | []string | `out_of_scope_deps,omitempty` | dependency names | Optional |
| TestsAdded | []string | `tests_added,omitempty` | test names | Optional |
| Verification | string | `verification,omitempty` | "PASS", "FAIL", etc | Optional |
| FailureType | string | `failure_type,omitempty` | "transient", "fixable", "needs_replan", "escalate", "timeout" | Optional |
| Notes | string | `notes,omitempty` | free text | Optional |
| DedupStats | *DedupStats | `dedup_stats,omitempty` | structured | Optional |
| Repo | string | `repo,omitempty` | absolute repo path | Optional (cross-repo) |

**Embedding:** `IMPLManifest.CompletionReports map[string]CompletionReport` (YAML key: `completion_reports`)

---

## 2. Write Sites Map

| File | Function | Fields Written | When | Notes |
|---|---|---|---|---|
| `pkg/orchestrator/orchestrator.go:876-890` | `launchAgent()` post-completion | status, commit, worktree, branch, filesChanged, filesCreated, testsAdded, verification, dedupStats, repo | After agent completes | Uses `SetCompletionReport()` + `Save()` behind `reportMu` |
| `pkg/engine/runner_wave_structured.go:82-133` | `runWaveAgentStructured()` | All fields from JSON schema | During wave execution | Unmarshals from agent's structured output; behind `reportWaveMu` |
| `cmd/sawtools/set_completion_cmd.go:28-108` | `set-completion` CLI command | status, commit, worktree, branch, filesChanged, filesCreated, testsAdded, verification | Manual CLI invocation | Validates status enum; splits CSV; calls `SetCompletionReport()` + `Save()` |
| `pkg/protocol/manifest.go:106-140` | `SetCompletionReport()` | Replaces entire report in map | Called by all writers | Low-level map update; no lock management |

---

## 3. Read Sites Map

| File | Function | Purpose |
|---|---|---|
| `pkg/agent/completion.go:10-47` | `WaitForCompletion()` | Poll manifest until completion report appears |
| `pkg/protocol/completion_validator.go:19-154` | `ValidateCompletionReportClaims()` | Post-hoc: commit SHA exists, files match ownership, worktree exists |
| `pkg/engine/finalize_steps.go:382-412` | `StepVerifyCompletionReports()` (I4) | Check all wave agents have reports |
| `pkg/engine/finalize_steps.go:414-443` | `StepCheckAgentStatuses()` (E7) | Block on partial/blocked status |
| `pkg/engine/finalize_steps.go:445-458` | `StepPredictConflicts()` (E11) | Detect file-level conflicts using report files |
| `pkg/resume/detect.go:137-223` | `DetectResumeState()` | Classify agents as completed/failed/pending |
| `pkg/orchestrator/merge.go:20-40` | `executeMergeWave()` | Extract reports to prepare merge context |
| `scout-and-wave-web/pkg/api/wave_runner.go:527` | Wave status display | Read CompletionReports for UI display only |

---

## 4. sawtools set-completion

**Location:** `cmd/sawtools/set_completion_cmd.go:28-108`

- **Required flags:** `--agent`, `--status` (enum), `--commit`
- **Optional flags:** `--worktree`, `--branch`, `--files-changed`, `--files-created`, `--tests-added`, `--verification`
- **Status validation:** Accepts only "complete", "partial", "blocked"
- **CSV parsing:** `--files-changed` and `--files-created` split on commas, whitespace trimmed
- **No commit:** Writes YAML only; does NOT git commit
- **No early validation:** Commit SHA, file ownership, worktree existence all checked post-hoc by `ValidateCompletionReportClaims()` during finalize-wave — too late

---

## 5. Missing / Inconsistent Fields

| Issue | Field(s) | Impact |
|---|---|---|
| `Repo` not populated | `repo` | Cross-repo waves can't identify which repo a report came from |
| `FailureType` not always set | `failure_type` | E19 retry logic falls back to "blocked"; can't distinguish transient vs fixable |
| `DedupStats` only in orchestrator path | `dedup_stats` | Engine path (`runner_wave_structured.go`) never sets it |
| `InterfaceDeviations` never wired | `interface_deviations` | Struct exists; never populated by agents or engine |
| `OutOfScopeDeps` never wired | `out_of_scope_deps` | Struct exists; never populated |
| No timestamp | N/A | Can't detect stale reports; recovery can't tell when agent finished |

---

## 6. Write Ordering Analysis

**Orchestrator path (most common):**
```
1. Agent calls set-completion in its worktree IMPL doc
2. WaitForCompletion() polls → detects report → returns
3. Orchestrator reads report from worktree
4. Orchestrator ALSO writes to main branch (SetCompletionReport + Save behind reportMu)
5. finalize-wave reads reports from main branch (I4 check)
```

**Engine path (runner_wave_structured):**
```
1. Agent generates JSON completion report
2. runWaveAgentStructured() unmarshals JSON → CompletionReport
3. Lock reportWaveMu
4. Load manifest
5. SetCompletionReport()
6. Save()
7. Unlock
```

**Race condition window:**
```
Agent A: Load() → SetCompletion(A) → Save()
                   ↑ B reads here   → sees stale manifest
Agent B:           Load() → SetCompletion(B) → Save()
Finalize:                                        → may see only A
```

`reportMu` (orchestrator) and `reportWaveMu` (engine) are independent — no cross-path coordination. Load-Set-Save is not atomic; crash between Set and Save loses the report silently.

---

## 7. Builder Design Recommendation

**New file:** `pkg/protocol/completion_report.go`

```go
type CompletionReportBuilder struct {
    report      *CompletionReport
    agentID     string
    manifest    *IMPLManifest
    implDocPath string
    errors      []error
}

func NewCompletionReport(agentID string, manifest *IMPLManifest) *CompletionReportBuilder
func (b *CompletionReportBuilder) WithStatus(status string) *CompletionReportBuilder
func (b *CompletionReportBuilder) WithCommit(sha string) *CompletionReportBuilder
func (b *CompletionReportBuilder) WithFiles(changed, created []string) *CompletionReportBuilder
func (b *CompletionReportBuilder) WithVerification(result string) *CompletionReportBuilder
func (b *CompletionReportBuilder) WithFailureType(ft string) *CompletionReportBuilder
func (b *CompletionReportBuilder) WithWorktree(path string) *CompletionReportBuilder
func (b *CompletionReportBuilder) WithBranch(name string) *CompletionReportBuilder
func (b *CompletionReportBuilder) WithRepo(path string) *CompletionReportBuilder
func (b *CompletionReportBuilder) Validate() error          // early validation
func (b *CompletionReportBuilder) AppendToManifest() error  // atomic in-memory update
```

**Key design decisions:**
- `Validate()` checks status enum, commit format, and semantic rules (e.g., "blocked requires failure_type") before any mutation
- `AppendToManifest()` updates manifest in-memory; caller controls `Save()` timing
- Unify `reportMu` / `reportWaveMu` into a single lock owned by `pkg/protocol/manifest.go`
- "blocked requires failure_type" enforced at build time, not post-hoc

---

## 8. Cross-Repo Scope

- **Web app** (`scout-and-wave-web/pkg/api/wave_runner.go:527`) reads `CompletionReports` for display only
- Web app does NOT write completion reports
- All writes go through orchestrator or engine (both in scout-and-wave-go)
- **No cross-repo write scope needed** — builder lives in scout-and-wave-go; web auto-inherits via module upgrade

---

## 9. pkg/agent vs pkg/engine Boundary

| Aspect | pkg/agent | pkg/engine |
|---|---|---|
| Writes reports | No | Yes (runner_wave_structured.go) |
| Reads reports | Yes (WaitForCompletion polls) | Yes (finalize-wave steps) |
| Validates reports | No | Yes (StepCheckAgentStatuses, etc) |
| Retry logic | No | Yes (E19 in orchestrator.go) |

**Boundary is clean.** `pkg/agent` is read-only (polling); `pkg/engine` owns writes. No duplication. The only issue is the split lock between orchestrator.go and engine path.

---

## 10. Finding 6 Intersection (Status Enums)

```go
// pkg/protocol/failure.go — Typed enum ✓ (already exists)
type FailureTypeEnum string
const (
    FailureTransient   = "transient"
    FailureFixable     = "fixable"
    FailureNeedsReplan = "needs_replan"
    FailureEscalate    = "escalate"
    FailureTimeout     = "timeout"
)

// CompletionReport.Status — plain string ✗ (needs typed enum)
// RetryResult.FinalState — plain string ✗ (needs typed enum)
```

`CompletionReport.Status = "blocked"` + `FailureType = "transient"` means: blocked but will retry.
`RetryResult.FinalState = "blocked"` means: terminal failure. Same string, different semantics.

**Builder should enforce semantic rules today** (without waiting for Finding 6):
- `WithStatus("blocked")` without `WithFailureType(...)` = validation error
- `WithStatus("complete")` with `WithFailureType(...)` = validation error (makes no sense)

---

## 11. Prioritized Gaps for IMPL

**Critical:**
1. **Lock fragmentation** — `reportMu` (orchestrator) and `reportWaveMu` (engine) uncoordinated → unify into single lock in `pkg/protocol/manifest.go`
2. **No early validation** — `set-completion` writes invalid data that only fails at finalize-wave → builder enforces constraints at write time
3. **Non-atomic write** — Load-Set-Save can lose report on crash → builder returns error on Save failure with clear recovery path

**High:**
4. **FailureType not populated in engine path** — `runner_wave_structured.go` doesn't unmarshal it from JSON
5. **DedupStats only in orchestrator** — engine path never sets it
6. **Repo not populated** — cross-repo waves have no way to identify report origin

**Medium (separate IMPL ok):**
7. **No timestamp** — add `WrittenAt *time.Time` to CompletionReport
8. **InterfaceDeviations/OutOfScopeDeps unused** — document or remove
9. **Status/FailureType typed enums** — Finding 6 unification

---

## IMPL Scope (Recommended)

1. Create `pkg/protocol/completion_report.go` — builder with `NewCompletionReport()`, `WithX()` methods, `Validate()`, `AppendToManifest()`
2. Unify `reportMu` / `reportWaveMu` into single lock in `pkg/protocol/manifest.go`
3. Update `pkg/orchestrator/orchestrator.go` write path to use builder
4. Update `pkg/engine/runner_wave_structured.go` write path to use builder
5. Update `cmd/sawtools/set_completion_cmd.go` to use builder (replaces manual field-by-field construction)
6. Populate missing fields consistently: `FailureType` (engine path), `Repo` (both paths)
7. Add `WrittenAt` timestamp to struct and all write paths
8. Tests for builder validation rules
