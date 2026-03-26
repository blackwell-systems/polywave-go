# Audit: Repeated Functionality in scout-and-wave-go

> Generated 2026-03-26. Covers `pkg/protocol/`, `pkg/engine/`, `pkg/agent/`, `internal/git/`.

---

## Executive Summary

5 unification opportunities remain. Top 5 by impact:

1. **Three retry abstractions** — `RetryConfig`, `RetryResult`, and `RetryAttempt` serve overlapping purposes; `RetryResult` was not merged into `RetryAttempt`
2. **Status/state enum proliferation** — `CompletionReport.Status` is a plain string; typed enums exist elsewhere
3. **YAML load/save inconsistency** — 34 direct yaml calls vs 12 usages of `LoadYAML`/`SaveYAML` in production code
4. **Validation function scatter** — 3+ entry points; unclear which to call; doc.go does not document call order
5. **Error wrapping inconsistency** — ~65 chain-breaking `%s`/`%v` patterns in `pkg/protocol` and `pkg/engine`

---

## Finding 3: Three Retry Abstractions (MEDIUM)

**Locations:**
- `pkg/retry/attempt.go` — `RetryAttempt` (replaces deleted `retryctx.RetryContext`; classifies error, builds retry prompt)
- `pkg/retry/types.go:4-8` — `RetryConfig` (max retries, paths — config only)
- `pkg/retry/types.go:11-18` — `RetryResult` (attempt number, gate results, final state)

**Divergence:**

| Type | Purpose | Stateful? |
|---|---|---|
| `RetryAttempt` | Classify error, build prompt for agent | Yes (per-attempt) |
| `RetryConfig` | Configure E24 loop behavior | No (config) |
| `RetryResult` | Capture attempt outcome | Yes (per-attempt) |

`RetryResult.FinalState` and `CompletionReport.Status` both represent "blocked" but as different string enums. `RetryAttempt` has `GateResults []string` while `RetryResult` has separate `GatePassed bool` and `GateOutput string` — overlapping per-attempt state carriers.

**Fix:**
- Keep `RetryConfig` as-is (configuration-only, narrow scope)
- Merge `RetryResult` fields into `RetryAttempt` to unify per-attempt state:
  ```go
  type RetryAttempt struct {
      Number         int
      AgentID        string
      ErrorClass     ErrorClass
      ErrorExcerpt   string
      SuggestedFixes []string
      GateResults    []string
      FinalState     string
      PromptText     string
  }
  ```

---

## Finding 5: YAML Load/Save Inconsistency (MEDIUM)

**Locations:**
- `pkg/protocol/yaml_io.go:15-38` — `LoadYAML[T]`, `SaveYAML` (generic helpers, exist but rarely used)
- `pkg/protocol/manifest.go:18` — `Load()` (specialized, has duplicate-key detection)
- 34 direct `yaml.Marshal/Unmarshal` + `os.WriteFile/ReadFile` calls in production code vs 12 usages of `LoadYAML`/`SaveYAML`

**Impact:** Error wrapping inconsistent; no centralized YAML parsing behavior (strict mode, unknown field handling); duplicate-key detection not shared.

**Fix:** Audit direct yaml calls; route through `LoadYAML`/`SaveYAML`; add optional strict mode; consolidate duplicate-key detection.

---

## Finding 6: Status/State Enum Proliferation (MEDIUM)

**Locations:**
- `pkg/protocol/types.go:221-256` — `ProtocolState`, `MergeState` (typed enums ✓)
- `pkg/protocol/types.go:103-118` — `CompletionReport.Status` (plain string: "complete"|"partial"|"blocked")
- `pkg/retry/types.go:17` — `RetryResult.FinalState` (plain string: "passed"|"retrying"|"blocked")
- `pkg/retry/attempt.go:22` — `RetryAttempt.FailureType` (plain string)
- `pkg/config/state.go:14-22` — `WaveState` (agent classification)

**Impact:** `CompletionReport.Status` and `RetryResult.FinalState` both represent completion state but can't be compared without string conversion; no compile-time typo checking.

**Fix:**
```go
type CompletionStatus string
const (
    CompletionComplete CompletionStatus = "complete"
    CompletionPartial  CompletionStatus = "partial"
    CompletionBlocked  CompletionStatus = "blocked"
)
```
Update `CompletionReport.Status` from `string` to `CompletionStatus`. Repeat for `FailureType`.

---

## Finding 7: Validation Function Scatter (MEDIUM)

**Locations:** 40+ `ValidateX()` functions across `pkg/protocol`:
- `ValidateIMPLDoc` (validator.go:58) — entry point
- `Validate` (validation.go:25) — main orchestrator
- `FullValidate` (full_validate.go:29) — full + subprocess validation
- `ValidateSchema`, `ValidateProgram`, `ValidateReactions`, `ValidateActionEnums`, `ValidateFileOwnership`, etc.

**Impact:** 3+ entry points; unclear which to call; each reimplements manifest loading; no shared validator registry; test isolation difficult. `doc.go` documents `ValidateInvariants` but not the full entry point hierarchy or call order.

**Fix:** Document single entry point: `Validate(manifest)` in `validation.go`. All sub-validators remain as checkpoints called by main. Add validator call order to `doc.go`.

---

## Finding 10: Error Wrapping Inconsistency (LOW-MEDIUM)

**Scope:** ~65 chain-breaking `fmt.Errorf` calls using `%s`/`%v` (vs `%w`) across `pkg/protocol` + `pkg/engine`. Error message prefixes inconsistent.

**Fix:** Prefer `%w` throughout; use function-name prefixes for traceability; audit `%v` usages in error returns.

---

## Prioritized Action Table

| Finding | Impact | Effort | Priority |
|---|---|---|---|
| 3. Retry abstractions | MEDIUM | MEDIUM | **P1** |
| 5. YAML load/save consistency | MEDIUM | LOW | **P1** |
| 6. Status/state typed enums | MEDIUM | MEDIUM | **P1** |
| 7. Validation function scatter | MEDIUM | HIGH | **P2** |
| 10. Error wrapping patterns | LOW-MEDIUM | LOW | **P3** |

---

## Architecture Boundaries (Recommended)

Document and enforce these ownership boundaries:

| Package | Owns |
|---|---|
| `internal/git/` | All git subprocess operations |
| `pkg/protocol/` | Types, validation, path resolution, YAML I/O |
| `pkg/engine/` | Orchestration, execution, wave lifecycle |
| `pkg/agent/` | Agent lifecycle, completion polling |
| `pkg/retry/` | Retry logic, error classification |
| `pkg/config/` | Configuration types only (no protocol types) |
