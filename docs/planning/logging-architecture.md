# Logging Architecture

## Current State

### What exists

**Observability system (`pkg/observability`):** A purpose-built SQLite-backed event system with three event types — `CostEvent`, `AgentPerformanceEvent`, and `ActivityEvent`. The `Emitter` is nil-safe and non-blocking (writes happen in a goroutine). It is threaded via **struct field injection**: callers receive a `*observability.Emitter` and store it in an `ObsEmitter` field on their options struct (e.g. `RunScoutOpts.ObsEmitter`, `FinalizeWaveOpts.ObsEmitter`, `RunWaveTransactionOpts.ObsEmitter`). The Emitter is not passed through `context.Context`. It is wired at the call site in `cmd/sawtools` and propagated downward through engine functions.

**Standard `log` package:** Used in exactly one place — `pkg/protocol/cleanup.go:183` — as `log.Printf(...)`. This is an isolated outlier, not a pattern.

**No structured logger:** There is no `slog`, `zap`, `zerolog`, or `logrus` import anywhere in the codebase. No `Logger` interface, no `log.go` abstraction file exists.

**282 `fmt.Fprintf(os.Stderr, ...)` calls across 34 non-worktree files:** These are the entire de facto logging system for everything that is not a domain event.

### What is missing

There is no structured, leveled, filterable logging for internal engine/library diagnostics. Every warning, debug trace, and non-fatal error goes to raw stderr with no level, no caller context, and no ability to suppress noise in production or amplify it in debug mode.

---

## Categorization of the 282 stderr calls

### Category 1 — Already covered by Emitter: do not duplicate (~10 calls)

These log lines fire at the same moment as an `Emitter.Emit()` call and carry the same information. They are redundant once the Emitter is wired. Examples:

- `engine.FinalizeWave: auto-corrected go.mod replace paths` (accompanies a wave lifecycle event)
- `close-impl: marked complete` / `close-impl: archived to` (accompany `impl_complete` events)
- `prepare-wave: pre-wave gate passed ✓` (accompanies gate events)

These should simply be removed when the Emitter is confirmed wired.

### Category 2 — Legitimate user-facing CLI progress output (~80 calls, all in `cmd/sawtools/`)

These are operator-visible progress messages printed during interactive CLI use. They are not internal diagnostic logs — they are the CLI's status channel. Examples:

- `prepare-wave: critic review passed ✓`
- `prepare-wave: E21B baseline FAILED in repo %s — fix before launching agents`
- `finalize-wave: closed-loop retry fixed gate '%s' for agent %s after %d attempt(s)`
- `close-impl: cleaned %d stale worktree(s)`
- `Warning: %d orphaned worktree(s) detected for %s (wave %d)`

These should **stay as `fmt.Fprintf(os.Stderr, ...)`**. Stderr is the correct channel for CLI progress; stdout is reserved for machine-readable output (JSON). These calls do not need a logger — they are intentional user communication, not diagnostics.

### Category 3 — Internal engine/library warnings and non-fatal errors (~120 calls, spread across `pkg/`)

These are the primary target for migration. They appear inside library packages that have no business writing directly to stderr. Examples:

- `pkg/orchestrator/orchestrator.go`: `orchestrator: E23 context extraction failed for agent %s`
- `pkg/orchestrator/merge.go`: `executeMergeWave: warning: failed to save merge-log`
- `pkg/engine/runner.go`: `scaffold build: go get ./... (non-fatal)`
- `pkg/engine/runner.go`: `engine.RunSingleAgent: retry context (best-effort)`
- `pkg/protocol/worktree.go`: `Warning: failed to install hooks for agent %s`
- `pkg/protocol/merge_agents.go`: `warning: failed to save merge-log`
- `pkg/worktree/manager.go`: `manager: warning: could not install pre-commit hook`
- `pkg/resume/worktree_status.go`: `resume: ClassifyWorktrees: git status failed`
- `pkg/journal/observer.go`: `Warning: skipping malformed JSONL line`
- `pkg/agent/backend/bedrock/client.go`: `bedrock: end_turn at turn %d` (debug trace)

These should route to **`slog`**.

### Category 4 — Debug/trace messages (~70 calls)

Low-signal operational traces that are currently always-on noise:

- `bedrock tool [turn %d]: %s(%s) → error=%v result=%s`
- `orchestrator: reusing existing worktree for agent %s at %s`
- `gate [%s]: skipped (cached at SHA %s)`
- `scaffold build verification: unrecognized project type, skipping`

These should become `slog.Debug(...)` so they are off by default and opt-in when `SLOG_LEVEL=DEBUG`.

---

## Recommended Approach

### Single unified path: `slog` for diagnostics, `Emitter` for domain events

Use **two channels, never three**:

| Signal type | Where it goes | Why |
|---|---|---|
| Domain lifecycle events (scout_launch, wave_merge, impl_complete, gate_passed/failed) | `observability.Emitter` | Queryable, stored in SQLite, drives dashboards |
| Engine/library warnings, non-fatal errors, debug traces | `slog.Logger` | Structured, leveled, filterable, stdlib — no new dependency |
| CLI operator progress messages | `fmt.Fprintf(os.Stderr, ...)` | Intentional UX, not diagnostics — leave as-is |

Do **not** create a custom `Logger` interface or wrapper. Go 1.21+ `log/slog` is already available (the module declares `go 1.25.0`) and provides everything needed: levels, key-value attributes, pluggable handlers, context propagation.

### Threading the logger

Use **struct field injection**, matching the existing `ObsEmitter` pattern. Do not use `context.Context` as a logger carrier — context is for cancellation and request-scoped values, not ambient configuration. The engine already injects `ObsEmitter` via options structs; add a `Logger *slog.Logger` field alongside it.

```
// Example pattern — add to existing options structs
type RunScoutOpts struct {
    // ... existing fields ...
    ObsEmitter *observability.Emitter  // already present
    Logger     *slog.Logger            // add this
}
```

Library packages (`pkg/engine`, `pkg/orchestrator`, `pkg/protocol`, `pkg/worktree`) receive the logger via their opts struct or (for packages without opts structs) as a constructor parameter. They must not call `slog.Default()` — that is a global and defeats injection.

Call sites in `cmd/sawtools` construct the logger once at startup and pass it down:

```
// In cmd/sawtools main or command setup
logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
    Level: resolveLogLevel(), // reads SAW_LOG_LEVEL env var
}))
```

A nil-safe wrapper is not needed — `slog.Logger` is safe to use as a zero value if initialized with `slog.New(slog.DiscardHandler)` for callers that opt out.

### What not to do

- Do not introduce a `Logger` interface. `*slog.Logger` is already an interface-backed type; wrapping it adds indirection with no benefit.
- Do not put the logger in `context.Context`. This pattern is contentious in the Go community, makes call sites verbose, and the codebase does not do this for `ObsEmitter` either.
- Do not route internal diagnostics through `Emitter`. The Emitter is for domain events that drive business intelligence (cost, performance, lifecycle). Engine warnings do not belong in SQLite.
- Do not use the global `slog.Default()` or `log.Printf` in library packages. These are uncontrollable from outside.

---

## Migration Priority Order

Prioritize by **impact on debuggability** (packages with the most signal-to-noise problems) and by **architectural harm** (library packages writing directly to stderr is a layering violation).

### Priority 1 — `pkg/orchestrator` (18 calls across 4 files)

Highest impact. The orchestrator is the hot path for all wave execution. Its stderr calls include both important warnings (`failed to save merge-log`, `auto-commit failed`) and low-value traces (`reusing existing worktree`). Mix of warn-level and debug-level. The orchestrator already receives context through function parameters — add `Logger *slog.Logger` to the relevant internal structs.

Files: `orchestrator.go` (8 calls), `merge.go` (10 calls), `stubs.go` (4 calls), `journal_integration.go` (1 call).

### Priority 2 — `pkg/engine` (16 calls across 3 files)

Engine is the public API surface. Library callers (scout-and-wave-web) cannot suppress these. Engine opts structs already have `ObsEmitter` — adding `Logger` is a one-line change per struct.

Files: `runner.go` (8 calls), `finalize.go` (7 calls), `integration_runner.go` (1 call).

### Priority 3 — `pkg/protocol` (12 calls across 4 files)

Protocol is a pure library — it should never write to stderr. These are the clearest layering violations.

Files: `worktree.go` (8 calls), `merge_agents.go` (2 calls), `program_worktree.go` (2 calls), `gates.go` (1 call).

### Priority 4 — `pkg/worktree`, `pkg/resume`, `pkg/journal`, `pkg/agent/backend/bedrock`

Smaller surface. The bedrock debug traces are the most useful to make level-gated (they are very noisy during development but valuable when debugging Bedrock API behavior).

### Priority 5 — `cmd/sawtools` non-progress calls

Some `cmd/sawtools` calls are not user-facing progress — they are internal errors that happen to be in CLI files (e.g. `validate-integration: failed to persist wiring report`). These can be migrated last since they are at least in the correct binary and not in a library.

### Do not migrate

- `cmd/sawtools` progress messages (Categories 2 above) — these are correct as-is.
- `pkg/observability/emitter.go:35` — the one stderr call in the emitter is an emergency fallback for when the store itself fails. It cannot use the store it just failed to write to, and using `slog` there would require injecting a logger into the emitter, adding complexity for an extremely rare path. Leave it.
- `pkg/protocol/cleanup.go` `log.Printf` — convert to `slog.Warn` in Priority 3 pass.

---

## Summary

The codebase needs one new thing: a `*slog.Logger` field added to engine and orchestrator options structs, constructed once in `cmd/sawtools`, and passed down. This replaces ~150 stderr calls in library packages without touching the CLI progress messages or the observability system. The two existing channels (`Emitter` for events, stderr for CLI output) remain unchanged. No new abstractions, no third-party dependencies, no parallel logging system.
