# Determinism Roadmap — Reducing AI Reliance in the Engine

**Date**: 2026-03-20 (last audited: 2026-03-22, L1-L4 confirmed complete)
**Last Cleaned**: 2026-03-23 — removed all completed features
**Goal**: Move validation from post-execution to runtime/pre-execution. Every protocol invariant should be enforced by code, not by trusting agents to follow instructions.

---

## Completed

All CRITICAL (C1-C6) and HIGH (H1-H4) items have been implemented. See git history for details:
- Runtime constraint enforcement: `pkg/tools/constraint_enforcer.go`, `pkg/tools/role_middleware.go`
- Completion report validation: `pkg/protocol/completion_validator.go`
- Pre-merge commit verification: `pkg/engine/finalize.go`
- File ownership coverage: `pkg/protocol/ownership_coverage.go`
- Dependency availability: `pkg/protocol/dependency_verifier.go`
- Gate input validation: `pkg/protocol/gate_input_validator.go`
- Pre-merge ownership check: `pkg/protocol/merge_agents.go`
- **M4: Wave base commit validation** — Superseded by `RunBaselineGates()` (E21A) in `pkg/protocol/baseline_gates.go`, called by `prepare-wave` before worktree creation. Merge flow in `pkg/orchestrator/merge.go` also records base commit and verifies agent commits against it.
- **L1: Missing agent prompt files error** — `pkg/engine/runner.go:65-69` now returns fatal error with `L1: no fallback` comment.
- **L2: Standardize validation error context** — `ValidationError` in `pkg/protocol/types.go:208-229` has `Slug`, `Wave`, `AgentID` fields + `WithContext()` helper. Call sites audited.
- **L3: Consolidate multi-repo resolution** — Unified `ResolveAgentRepo()` in `pkg/protocol/multi_repo.go`. Old `determineAgentRepo()` and `resolveAgentRepoRoot()` removed.
- **L4: Remove raw YAML fallback** — `loadIntegrationConnectors()` removed from `pkg/engine/constraints.go`. Uses `manifest.IntegrationConnectors` directly.

---

## MEDIUM — Non-deterministic flows that should be structured

### M1: Atomic RunWave transaction wrapper
**Files**: `pkg/orchestrator/orchestrator.go`, `pkg/engine/run_wave_full.go`
**Issue**: `RunWaveFull()` and `FinalizeWave()` advance state via sequential steps with early return on error but no rollback. If a step fails partway (e.g., merge completes but state transition fails), the IMPL doc state may be inconsistent with reality.
**Risk**: Partial-failure state inconsistency. Manual recovery requires understanding state machine.
**Fix**: Create `RunWaveTransaction` wrapper. All state mutations go through transaction. Only commit state after ALL steps succeed. On failure, roll back to start state.
**New file**: `pkg/orchestrator/run_wave_atomic.go`
**Effort**: Medium (2-3 days)

### M2: Integration validation enforcement (E25)
**Files**: `pkg/engine/finalize.go` (lines 119-129)
**Issue**: `ValidateIntegration()` in `FinalizeWave` Step 3.5 is explicitly non-fatal ("informational, does not block"). Errors are logged to stderr but never block merge or build.
**Risk**: Integration breaks silently. Orphaned exports left by agents.
**Fix**: Add `EnforceIntegrationValidation bool` to `FinalizeWaveOpts`. When true, unconnected exports fail the wave before merge.
**Effort**: Low (1 day)

### M3: Mandatory stub detection (E20)
**Files**: `pkg/engine/finalize.go` (lines 83-101)
**Issue**: `ScanStubs()` in `FinalizeWave` Step 2 is explicitly non-fatal ("informational, does not block"). Stub scan errors are logged to stderr and results stored, but never block merge.
**Risk**: Incomplete implementations merged. Later waves find stubs where they expect complete code.
**Fix**: Add `RequireNoStubs bool` to `FinalizeWaveOpts`. When true, any TODO/FIXME in changed files fails the wave before merge.
**Effort**: Low (0.5 day)

---

## LOW — Remaining items

### M4: Pre-commit quality gate (language-agnostic)
**Files**: `cmd/saw/pre_commit_cmd.go` (new), `pkg/protocol/gate_discovery.go` (new)
**Issue**: E21A baseline gates only run at `prepare-wave` time. Bad code committed directly to main (by agents or humans) passes unchecked until the next wave attempt. This was hit in practice: a `fmt.Fprintf` type mismatch in `close_impl_cmd.go` was committed to main and only caught when E21A blocked `prepare-wave`.
**Risk**: High friction. Every direct commit is a potential baseline failure that wastes agent work.
**Fix**: New `sawtools pre-commit-check --repo-dir .` command that:
1. Discovers active IMPL doc(s) for the repo (or falls back to `saw.config.json`)
2. Reads the `quality_gates` section and extracts the `lint` gate command
3. Runs the lint command — blocks commit on failure
4. If no active IMPL or config, passes silently (no config = no gate)
Language-agnostic: whatever lint command the Scout wrote (`go vet`, `npm run lint`, `cargo clippy`, `ruff check`) is what runs. New `sawtools install-hooks` subcommand wires it into `.git/hooks/pre-commit`.
**New files**: `cmd/saw/pre_commit_cmd.go`, `cmd/saw/install_hooks_cmd.go`, `pkg/protocol/gate_discovery.go`
**Integration points**:
- `sawtools install-hooks` subcommand wires `pre-commit-check` into `.git/hooks/pre-commit`
- `prepare-wave` pre-flight: check if hooks installed, warn if not (non-blocking)
- `/saw` skill: mention `install-hooks` during bootstrap flow
- `close-impl`: no change (hooks persist across IMPLs)
**Documentation** (protocol repo):
- `docs/hooks.md`: document new `pre-commit-check` hook (trigger, behavior, fallback chain)
- `docs/cli-reference.md`: add `pre-commit-check` and `install-hooks` commands
- `/saw` skill prompt: add `install-hooks` to sawtools command list
**Effort**: Medium (2-3 days)

---

## Implementation Priority

### Phase 4: Transactional execution & configurable gates
- **M1**: Atomic `RunWaveTransaction` wrapper
- **M2**: Configurable integration enforcement
- **M3**: Configurable stub detection gate
- **M4**: Pre-commit quality gate

### Phase 5: ~~Cleanup~~ COMPLETE
- ~~**L1-L4**~~: All implemented. See Completed section.
