# Code Quality Audit: pkg/builddiag

**Inspector version:** 0.2.0
**Date:** 2026-04-05
**Repo root:** `/Users/dayna.blackwell/workspace/code/scout-and-wave-go`
**Audited area:** `pkg/builddiag` (all files)

---

## Summary

- **Audited:** `pkg/builddiag/diagnose.go`, `types.go`, `go_patterns.go`, `js_patterns.go`, `python_patterns.go`, `rust_patterns.go`, `diagnose_test.go`, `types_test.go`, `go_patterns_test.go`, `js_patterns_test.go`, `python_patterns_test.go`, `rust_patterns_test.go`
- **Layer map:** `cmd/sawtools` → `pkg/engine` → `pkg/builddiag` (no upward imports; `builddiag` is a leaf package). `pkg/engine/finalize.go` and `cmd/sawtools/diagnose_build_failure_cmd.go` are the only non-test callers. The web app (`scout-and-wave-web`) has zero direct references to `builddiag` types — they surface only transitively via `FinalizeWaveResult.BuildDiagnosis` in `pkg/engine`.
- **Highest severity:** error
- **Signal:** The package is well-tested and structurally simple; three concrete defects exist — a globally misleading doc comment, a silent confidence-ordering violation in the Rust catalog that contradicts the dispatch invariant, and a permanently-vacuous test assertion that provides false assurance of regex correctness.

---

## Findings

### Finding 1: doc_drift — `RegisterPatterns` doc says "adds" but implementation replaces

**doc_drift** · error · confidence: reduced
`pkg/builddiag/diagnose.go:11–13` · [LSP unavailable — Grep fallback, reduced confidence]

**What:** The doc comment on `RegisterPatterns` reads "adds patterns for a language to the catalog." The body executes `catalogs[strings.ToLower(language)] = patterns`, which is a full replacement of any existing slice for that key, not an append. A caller invoking `RegisterPatterns` twice for the same language (e.g., a plugin adding supplemental patterns) will silently discard the first registration.

The doc drift is elevated to error because it describes incorrect behavior for an error condition: callers attempting to extend an existing catalog will lose patterns without any diagnostic, which can cause downstream `DiagnoseError` calls to silently miss matches they expect to be present. The `cmd/sawtools/diagnose_build_failure_cmd_test.go` tests work around this by re-registering the full pattern set, suggesting the replacement behavior has been discovered empirically but never documented.

**Exported symbol affected:** `RegisterPatterns`
**Web app blast radius:** 0 files — the web app does not call `RegisterPatterns` directly.
**Fix requires signature change:** No. The fix is either (a) update the doc comment to say "replaces the pattern catalog for a language" and add a note that callers wanting to extend should read existing patterns first, or (b) change the implementation to append and document that behavior instead.

---

### Finding 2: coverage_gap — Rust patterns violate the confidence-descending dispatch invariant

**coverage_gap** · error · confidence: high
`pkg/builddiag/rust_patterns.go:33–44` · [LSP unavailable — Grep fallback, reduced confidence]

**What:** `DiagnoseError` in `diagnose.go` (line 24 comment: "Try each pattern in order (highest confidence first)") assumes patterns are ordered by descending confidence. The Rust catalog breaks this:

```
cannot_find_value       0.90  (index 0)
trait_bound_not_satisfied 0.90  (index 1)
mismatched_types        0.85  (index 2)  ← drops
unresolved_import       0.90  (index 3)  ← rises again
macro_undefined         0.85  (index 4)
```

`unresolved_import` (0.90) appears after `mismatched_types` (0.85), violating the invariant. If an error log simultaneously matches both `mismatched_types` and `unresolved_import`, the lower-confidence pattern is returned first. This is a reachable code path — a "mismatched types" error in a missing Cargo.toml dependency can legitimately match both patterns.

The `TestGoPatterns_ConfidenceLevels` test enforces ordering for the Go catalog but no analogous test exists for Rust, Python, or JS catalogs. The gap means this ordering violation passes the full test suite.

**Exported symbol affected:** None directly; affects the `Diagnosis` value returned by `DiagnoseError` for Rust errors.
**Web app blast radius:** 0 files (no direct call path from web app to builddiag).
**Fix requires signature change:** No.
**Fix:** Move `unresolved_import` to index 0 or 1 (alongside the other 0.90-confidence patterns). Add confidence-ordering assertion tests for all four language catalogs, mirroring `TestGoPatterns_ConfidenceLevels`.

---

### Finding 3: coverage_gap — `containsErrorCode` helper is permanently vacuous

**coverage_gap** · error · confidence: high
`pkg/builddiag/rust_patterns_test.go:248–253` · [LSP unavailable — Grep fallback, reduced confidence]

**What:** `TestRustPatterns_ErrorCodeMatching` is described in its comment (line 238) as confirming "our regex properly escapes square brackets." It delegates this check to `containsErrorCode`. The helper is:

```go
func containsErrorCode(errorLog, code string) bool {
    expectedFormat := "[" + code + "]"
    return len(errorLog) > 0 && len(expectedFormat) > 0
}
```

This function returns `true` for every non-empty input — it never checks whether `expectedFormat` actually appears in `errorLog`. Because `len("[" + code + "]")` is always > 0 for any non-empty code string, the branch `!containsErrorCode(tt.errorLog, tt.wantCode)` at line 240 can never be true, so `t.Errorf` on line 241 can never fire.

The stated test goal — verifying that the regex escapes `[` and `]` in Rust error codes — is entirely unverified. A regex like `error\[E0432\]` and a broken regex like `error[E0432]` (unescaped) would produce the same test outcome.

**Exported symbol affected:** None (test-internal helper). However, it provides false assurance about regex correctness for the exported `DiagnoseError` function.
**Web app blast radius:** 0 files.
**Fix requires signature change:** No.
**Fix:** Replace the body with `strings.Contains(errorLog, "["+code+"]")`. This is the actual check the comment describes.

---

### Finding 4: init_side_effects — all four `init()` functions mutate a package-level global

**init_side_effects** · warning · confidence: high
`pkg/builddiag/go_patterns.go:3`, `js_patterns.go:3`, `python_patterns.go:3`, `rust_patterns.go:3` · [LSP unavailable — Grep fallback, reduced confidence]

**What:** All four language files use `func init()` to call `RegisterPatterns`, which mutates the package-level `catalogs` map. This is global state mutation at import time. Practically this creates one concrete problem:

Tests that need a controlled `catalogs` state (e.g., `TestDiagnoseError_KnownPattern`) save and restore the map at the top of each test:
```go
originalCatalogs := catalogs
defer func() { catalogs = originalCatalogs }()
catalogs = make(map[string][]ErrorPattern)
```
This pattern works correctly for sequential tests but would be racy under `t.Parallel()`. The `rust_patterns_test.go` file compounds this by introducing `ensureRustPatternsRegistered()`, a workaround that re-registers Rust patterns because other tests clear the global. This guard function is itself a symptom of the init-based global design.

The side effects cannot fail at import time (they are pure in-memory writes), so severity is warning rather than error.

**Exported symbol affected:** `RegisterPatterns` (the mutation mechanism).
**Web app blast radius:** 0 files.
**Fix requires signature change:** No.
**Fix:** The conventional Go mitigation is to keep `init()` for the default catalog setup but accept this as the package's design. The concrete actionable fix is to add `t.Parallel()` guards to tests that mutate `catalogs`, or refactor tests to use `DiagnoseError` against a locally-scoped catalog passed as a parameter (which would require a function signature change to `DiagnoseError`). The lower-cost immediate fix is to document the non-parallelizability of the tests with a comment.

---

### Finding 5: doc_drift — `DiagnoseError` comment does not describe the "no match" return behavior

**doc_drift** · warning · confidence: reduced
`pkg/builddiag/diagnose.go:16–18` · [LSP unavailable — Grep fallback, reduced confidence]

**What:** The doc comment for `DiagnoseError` states: "Returns nil for unsupported languages." This is accurate but incomplete. When a supported language is provided but no pattern matches, the function returns a non-nil `*Diagnosis` with `Pattern: "unknown"` and `Confidence: 0.0`. This is a distinct return case that callers must handle differently from the nil case, but it is not documented.

The caller in `cmd/sawtools/diagnose_build_failure_cmd.go` (line 40) guards only for nil: `if diagnosis == nil { return fmt.Errorf("unsupported language") }`. A caller that wants to detect "no pattern matched" versus "language supported with a match" must read the implementation to discover the `Pattern == "unknown"` sentinel. This is drift between the documented contract and the actual behavior.

**Exported symbol affected:** `DiagnoseError`
**Web app blast radius:** 0 files.
**Fix requires signature change:** No.
**Fix:** Extend the doc comment: "Returns nil for unsupported languages. Returns a Diagnosis with Pattern 'unknown' and Confidence 0.0 when no registered pattern matches."

---

### Finding 6: test_coverage — `SupportedLanguages` has no test for non-deterministic ordering

**test_coverage** · warning · confidence: reduced
`pkg/builddiag/diagnose.go:53–58` · [LSP unavailable — Grep fallback, reduced confidence]

**What:** `SupportedLanguages` iterates over a Go map and returns the keys as a `[]string`. Go map iteration order is deliberately randomized. The existing test `TestSupportedLanguages` (line 181) checks `len(langs) == 3` and that each language is present — it does not test ordering. This is correct defensive test design.

However, callers that display or process the language list in sorted order will see non-deterministic output across runs. The function does not document that the returned slice is unordered, and the CLI help text in `diagnose_build_failure_cmd.go` hardcodes language order in the `Long` description independently of the programmatic catalog. If a new language is registered via `init()` in a new file, the hardcoded help text will not update automatically.

The finding is warning-level because callers cannot currently be broken by this (they either use it for display or check membership), but the undocumented non-determinism and the divergence between catalog and CLI help text is a maintenance hazard.

**Exported symbol affected:** `SupportedLanguages`
**Web app blast radius:** 0 files.
**Fix requires signature change:** No.
**Fix:** Document the return order as undefined in the function comment. Consider sorting the returned slice for stable output: `sort.Strings(langs)`.

---

## All Findings

| Severity | Confidence | Check Type | Finding | Location |
|----------|------------|------------|---------|----------|
| error | reduced | doc_drift | `RegisterPatterns` doc says "adds" but implementation replaces, silently discarding any previous registration for the same language | `pkg/builddiag/diagnose.go:11` |
| error | high | coverage_gap | Rust catalog violates the confidence-descending ordering invariant (`unresolved_import` at 0.90 appears after `mismatched_types` at 0.85) | `pkg/builddiag/rust_patterns.go:33` |
| error | high | coverage_gap | `containsErrorCode` helper is always-true; the test it backs never verifies regex bracket escaping | `pkg/builddiag/rust_patterns_test.go:248` |
| warning | high | init_side_effects | All four `init()` functions mutate package-level `catalogs` map, requiring each test to save/restore global state; unsafe under `t.Parallel()` | `pkg/builddiag/go_patterns.go:3`, `js_patterns.go:3`, `python_patterns.go:3`, `rust_patterns.go:3` |
| warning | reduced | doc_drift | `DiagnoseError` doc does not document the `Pattern: "unknown"` fallback return for supported-language / no-match case | `pkg/builddiag/diagnose.go:16` |
| warning | reduced | test_coverage | `SupportedLanguages` return order is non-deterministic (map iteration); function and CLI help text diverge on language enumeration | `pkg/builddiag/diagnose.go:53` |

---

## Web App Impact Summary

All three exported functions (`DiagnoseError`, `RegisterPatterns`, `SupportedLanguages`) and both exported types (`ErrorPattern`, `Diagnosis`) are used exclusively within the Go engine repo. The web app (`scout-and-wave-web`) has **zero direct references** to any `builddiag` symbol. The `Diagnosis` type appears in the web app's view of data only as an opaque JSON field `build_diagnosis` serialized by `FinalizeWaveResult`.

**Blast radius for any signature change to any builddiag exported symbol: 0 web app files.**

The only non-test callers within the Go repo are:
- `pkg/engine/finalize.go` — embeds `*builddiag.Diagnosis` in `FinalizeWaveResult`
- `pkg/engine/finalize_steps.go` — calls `builddiag.DiagnoseError` in `StepVerifyBuild`
- `cmd/sawtools/diagnose_build_failure_cmd.go` — calls `builddiag.DiagnoseError` directly
- `cmd/sawtools/diagnose_build_failure_cmd_test.go` — calls `builddiag.RegisterPatterns` in test setup

---

## Not Checked — Out of Scope

- Correctness of the regex patterns themselves (e.g., whether `cannot use .* \(type .*\) as type` matches all real Go type-mismatch messages) — this is domain content validation, outside the structural check taxonomy.
- `pkg/errparse` — referenced in CLAUDE.md as a sibling to `builddiag`, but not included in the audit scope.

## Not Checked — Tooling Constraints

- **LSP `findReferences` and `hover`** — the `LSP` tool was not available as a callable tool in this session. All symbol-level checks used Grep as fallback. Findings marked `confidence: reduced` reflect this. A Grep search across the full repo was performed for each exported symbol to compensate; all callers were identified. The reduced-confidence annotation should be interpreted as "cross-file aliasing and interface-dispatch call sites may be missed by Grep" — for this package (no interfaces, no function types assigned to variables), the practical risk is low.
- **Cross-repo worktree copies** — `.claude/worktrees/` contains multiple in-progress copies of `pkg/engine/finalize.go` that also embed `builddiag.Diagnosis`. These are agent worktrees, not the main branch; they were excluded from the blast-radius count.
