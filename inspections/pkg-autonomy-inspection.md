# Code Quality Inspection — `pkg/autonomy`

**Inspector version:** 0.2.0
**Date:** 2026-04-05
**Repo root:** `/Users/dayna.blackwell/workspace/code/scout-and-wave-go`
**Area audited:** `/Users/dayna.blackwell/workspace/code/scout-and-wave-go/pkg/autonomy`
**Files:** `autonomy.go`, `config.go`, `autonomy_test.go`, `config_test.go`

---

## Summary

- **Audited:** `pkg/autonomy` (4 files, ~230 lines of implementation, ~200 lines of tests)
- **Layer map:**
  ```
  cmd/sawtools → pkg/engine → pkg/autonomy → pkg/result
  pkg/config   → pkg/result                              (peer; no import of pkg/autonomy)
  pkg/autonomy → pkg/result                              (leaf dependency only)
  ```
  Boundary: `pkg/autonomy` must not import `pkg/engine`, `pkg/config`, or `cmd/`. It does not.
  `pkg/config` independently replicates the autonomy config shape (`AutonomyConfig` struct) without importing `pkg/autonomy`.
- **Highest severity:** warning
- **Signal:** The package is small, well-tested, and correctly isolated. Two structural issues require attention: `SaveConfig` is a dead export (no non-test callers exist anywhere, including the web app), and `pkg/config.AutonomyConfig` duplicates `pkg/autonomy.Config` as parallel types serving the same domain, creating a consistency maintenance risk.

---

## `pkg/autonomy/autonomy.go`

**dead_symbol** · warning · confidence: reduced
`pkg/autonomy/autonomy.go:23` · [LSP unavailable — Grep fallback, reduced confidence]
**Symbol:** `StageWaveAdvance Stage = "wave_advance"`
What: `StageWaveAdvance` is defined and included in the `ShouldAutoApprove` switch arm for `LevelSupervised` (line 61), but no non-test caller in `pkg/engine`, `cmd/sawtools`, or `scout-and-wave-web` ever passes it to `ShouldAutoApprove`. The doc comment on `autonomy-levels.md` explicitly states it is "Reserved: will fire between waves when per-wave gating is added." Its presence in the live switch arm means it is reachable code that currently produces no behavior — the daemon iterates waves unconditionally and never calls `ShouldAutoApprove(_, StageWaveAdvance)`.
Fix: No code removal needed until the feature is implemented. However, the `ShouldAutoApprove` doc comment should clarify that `StageWaveAdvance` is currently unused by any caller so a reader of the switch arm does not assume coverage is being exercised.

---

**duplicate_semantics** · warning · confidence: high
`pkg/autonomy/autonomy.go:29` and `pkg/config/config.go:57`
[LSP unavailable — Grep fallback, reduced confidence]
**Symbols:** `autonomy.Config` vs `pkg/config.AutonomyConfig`
What: Two structs represent the same autonomy configuration concept and map to identical JSON keys (`level`, `max_auto_retries`, `max_queue_depth`). `pkg/autonomy.Config` uses typed `Level` for its `Level` field; `pkg/config.AutonomyConfig` uses untyped `string`. The web app handler (`pkg/api/autonomy_handler.go`) manually converts between them on every request:
```go
cfg := autonomy.Config{
    Level:          autonomy.Level(sawCfg.Autonomy.Level),
    MaxAutoRetries: sawCfg.Autonomy.MaxAutoRetries,
    MaxQueueDepth:  sawCfg.Autonomy.MaxQueueDepth,
}
```
and `pkg/engine/daemon.go` (`daemon_handler.go` line 98-102) does the same. If a new field is added to one struct, it must be manually added to the other and to every conversion site. Callers cannot statically verify that the conversion is exhaustive.

Web app impact:
- Affected exported symbols: `autonomy.Config`, `autonomy.Level`, `autonomy.MaxAutoRetries`, `autonomy.MaxQueueDepth` (struct fields)
- Web app files with the manual conversion: `/Users/dayna.blackwell/workspace/code/scout-and-wave-web/pkg/api/autonomy_handler.go` (line 16–20), `/Users/dayna.blackwell/workspace/code/scout-and-wave-web/pkg/api/daemon_handler.go` (line 98–102)
- Fix requires signature change: No — the fix is additive (embed or alias), not a signature removal
- Blast radius: 2 web app files would need updating if fields change; currently 0 if just aliasing

Fix: `pkg/config.AutonomyConfig` should embed or type-alias `autonomy.Config` so the conversion sites disappear and a field addition in one place propagates automatically. Alternatively, `pkg/autonomy` could import `pkg/config.AutonomyConfig` — but the cleaner direction given the layer map is for `pkg/config` to use `autonomy.Config` directly (since `pkg/config` is a peer that does not need to be below `pkg/autonomy` in the hierarchy).

---

## `pkg/autonomy/config.go`

**dead_symbol** · error · confidence: reduced
`pkg/autonomy/config.go:83` · [LSP unavailable — Grep fallback, reduced confidence]
**Symbol:** `func SaveConfig(repoPath string, cfg Config) result.Result[SaveConfigData]`
What: `SaveConfig` has zero callers outside of its own test file in both this repo and `scout-and-wave-web`. The web app's `handleSaveAutonomy` uses `config.Save` (the unified config package) instead. `cmd/sawtools/daemon_cmd.go` uses `autonomy.LoadConfig` but never `SaveConfig`. A search across all non-worktree Go files in both repos confirms no production caller exists.

Web app impact:
- Affected exported symbol: `autonomy.SaveConfig`
- Web app files calling it: none (confirmed by Grep across `/Users/dayna.blackwell/workspace/code/scout-and-wave-web`)
- Fix requires signature change: No — this is removal, not modification
- Blast radius: 0 web app files affected by removal

Fix: Remove `SaveConfig` from the package if `pkg/config.Save` is the intended write path. If `SaveConfig` is retained for CLI use cases, add a `sawtools set-autonomy` command or document which binary is expected to call it — and add a non-test caller. Without one, the function imposes a maintenance obligation with no payoff.

---

**coverage_gap** · warning · confidence: high
`pkg/autonomy/config.go:32–77`
What: `LoadConfig` treats an invalid `autonomy.Level` value in the config file as a success. If `saw.config.json` contains `{"autonomy": {"level": "turbo"}}`, `LoadConfig` returns `result.NewSuccess` with `Config.Level = "turbo"`. The caller then passes this `Config` to `ShouldAutoApprove`, which hits the `default: return false` arm — silently behaving as `gated`. The invalid value is never surfaced to the operator.
Reachability: Normal input path. Any operator who misconfigures the level string gets silent fallback behavior with no diagnostic.
Fix: After successfully unmarshalling `cfg` at line 66, call `ParseLevel(string(cfg.Level))` and return a fatal result with `CONFIG_LOAD_FAILED` if it fails. This matches the documented behavior in `autonomy-levels.md` ("Invalid JSON in the file causes a fatal CONFIG_LOAD_FAILED error") which implies invalid values should also be fatal, not silently coerced.

---

**doc_drift** · warning · confidence: high
`pkg/autonomy/config.go:32`
What: The doc comment for `LoadConfig` states "Returns fatal result on JSON parse errors." The `autonomy-levels.md` reference doc states "Invalid JSON in the file causes a fatal `CONFIG_LOAD_FAILED` error; other top-level keys are preserved when the autonomy section is written back." Neither the function doc nor the reference doc mentions that an invalid `level` string is silently accepted. The doc is not wrong, but it is incomplete in a way that misleads callers into assuming all invalid configurations are rejected at load time.
Fix: Add a sentence to the `LoadConfig` doc comment: "An unrecognized `level` string is NOT validated at load time; callers should call `ParseLevel(string(cfg.Level))` if strict validation is required." This is a low-effort fix that makes the current behavior explicit rather than surprising.

---

**cross_field_consistency** · warning · confidence: high
`pkg/autonomy/autonomy.go:29–33`
**Fields:** `MaxAutoRetries` and `Level`
What: `MaxAutoRetries` is only meaningful when `Level` is `supervised` or `autonomous` (specifically when `ShouldAutoApprove(level, StageGateFailure)` returns true). When `Level` is `gated`, `MaxAutoRetries` has no effect. No validation enforces this relationship and no documentation warns the operator. This is not a runtime error, but it creates a latent confusion: an operator who sets `max_auto_retries: 5` with `level: "gated"` will observe that retries never happen, with no indication why.
The `MaxQueueDepth` field has an analogous issue: it is documented as "Reserved for future queue depth limiting; not currently enforced" (line 32 comment) but the comment is only visible in source code, not in the JSON config schema or the reference doc.
Fix: Add a note to `autonomy-levels.md` (and optionally to the `Config` struct comment) explicitly stating that `max_auto_retries` is only operative when `gate_failure` is auto-approved and `max_queue_depth` is currently unenforced.

---

## All Findings

| Severity | Confidence | Check Type | Finding | Location |
|----------|------------|------------|---------|----------|
| error | reduced | dead_symbol | `SaveConfig` has zero production callers in both repos | `pkg/autonomy/config.go:83` |
| warning | high | duplicate_semantics | `autonomy.Config` and `pkg/config.AutonomyConfig` represent the same domain with manual conversion at every call site | `pkg/autonomy/autonomy.go:29`, `pkg/config/config.go:57` |
| warning | high | coverage_gap | `LoadConfig` silently accepts invalid `level` strings; no fatal error returned for unrecognized values | `pkg/autonomy/config.go:66` |
| warning | high | doc_drift | `LoadConfig` doc comment implies all invalid config rejected; invalid level string is silently accepted | `pkg/autonomy/config.go:32` |
| warning | high | cross_field_consistency | `MaxAutoRetries` is silently inoperative under `gated` level; `MaxQueueDepth` is unenforced but not noted in reference doc | `pkg/autonomy/autonomy.go:31–32` |
| warning | reduced | dead_symbol | `StageWaveAdvance` in switch arm is never passed by any caller; reserved per docs but creates false coverage impression | `pkg/autonomy/autonomy.go:23` |

---

## Not Checked — Out of Scope

- `pkg/engine/daemon.go` and `pkg/engine/queue_advance.go` — these are callers of `pkg/autonomy`, not part of the audited package. Their use of `autonomy.EffectiveLevel` and `autonomy.ShouldAutoApprove` was read only to validate call sites for dead_symbol analysis.
- `pkg/config/config.go` — read only to characterize the duplicate_semantics finding. Full audit of `pkg/config` was not requested.
- `scout-and-wave-web` files read only to assess blast radius for exported symbol changes; no audit of web app logic was performed.

## Not Checked — Tooling Constraints

- **LSP findReferences** — LSP tool was not available in this environment. All symbol reference analysis was performed via Grep across both repos (`scout-and-wave-go` and `scout-and-wave-web`). All findings that depend on reference counts are annotated `[LSP unavailable — Grep fallback, reduced confidence]`. Grep may miss aliased calls or interface implementations, but in this package (pure functions, no interfaces, no reflection) the risk of missed references is low.
- **LSP hover** — Not used; signatures were read directly from source and verified adequate for the checks performed.
- `StageWaveAdvance` dead_symbol finding carries reduced confidence because LSP was unavailable; manual Grep across the full repo and web app returned zero non-test callers.
