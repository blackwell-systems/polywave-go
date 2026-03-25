# Logging Architecture — Remaining Work

The slog migration is substantially complete. This document records only what is **not yet done**.

---

## What has been implemented

- `cmd/sawtools/logger.go`: `newSawLogger()` constructs a `*slog.Logger` with level from `SAW_LOG_LEVEL` env var (default: WARN).
- `Logger *slog.Logger` field added to all engine opts structs: `RunScoutOpts`, `FinalizeWaveOpts`, `RunWaveTransactionOpts`, `RunWaveFullOpts`, `RunWaveAtomicOpts`, `RunIntegrationOpts`, and the step types.
- `Orchestrator` has a `logger *slog.Logger` field, `SetLogger()`/`log()` methods.
- `pkg/worktree/Manager`, `pkg/journal/JournalObserver`, `pkg/agent/backend/bedrock/Client` all have `SetLogger()`/`log()` patterns.
- All `pkg/` packages have zero `fmt.Fprintf(os.Stderr, ...)` calls (only one remains in `pkg/observability/emitter.go` — the emergency fallback, deliberately left).
- `log.Printf` in `pkg/protocol/cleanup.go` converted to `slog.Default().Warn`.
- `newSawLogger()` is wired at call sites in `cmd/sawtools` (prepare-wave, finalize-wave, run-scout, close-impl).

---

## Remaining work

### 1. `pkg/protocol/*` — Logger not injected; uses `slog.Default()` directly

All four protocol files (`worktree.go`, `merge_agents.go`, `program_worktree.go`, `gates.go`, `cleanup.go`) call `slog.Default()` directly. The protocol package has no opts structs or constructor parameters to receive a logger, so callers (including `scout-and-wave-web`) cannot control what handler these calls go to.

**What is needed:** Add a `Logger *slog.Logger` parameter to the relevant protocol functions, or introduce a package-level options struct for the functions that are called with many arguments. Replace all `slog.Default()` calls with the injected logger. Wire it at the `pkg/engine` and `cmd/sawtools` call sites.

Affected files and approximate call counts:
- `pkg/protocol/worktree.go` — 7 calls
- `pkg/protocol/merge_agents.go` — 2 calls
- `pkg/protocol/program_worktree.go` — 2 calls
- `pkg/protocol/gates.go` — 1 call
- `pkg/protocol/cleanup.go` — 1 call

### 2. `pkg/orchestrator/stubs.go` and `journal_integration.go` — bypasses `o.log()`

The `Orchestrator` type has a proper `log()` method, but `stubs.go` (4 calls) and `journal_integration.go` (1 call) call `slog.Default()` directly instead of routing through `o.log()`. These are package-level functions, not methods on `Orchestrator`, so they do not have access to the struct's logger field.

**What is needed:** Either thread the logger as a parameter to `RunStubScan` and the journal integration function, or make them methods on `Orchestrator` so they can use `o.log()`.

### 3. `pkg/resume/worktree_status.go` — no logger injection

`ClassifyWorktrees` calls `slog.Default().Warn(...)` directly. The function has no options struct or constructor; the logger cannot be overridden by callers.

**What is needed:** Add a `logger *slog.Logger` parameter to `ClassifyWorktrees` (or a `ClassifyWorktreesOpts` struct if it grows). Wire it at the call site in the engine.

---

## Non-goals (do not change)

- `cmd/sawtools` `fmt.Fprintf(os.Stderr, ...)` progress messages — correct as-is; these are intentional CLI UX output, not diagnostics.
- `pkg/observability/emitter.go:35` — the one stderr call is an emergency fallback for store write failure. Leave it.
