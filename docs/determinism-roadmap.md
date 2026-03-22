# Determinism Roadmap — Reducing AI Reliance in the Engine

**Date**: 2026-03-20 (last audited: 2026-03-22)
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

## LOW — Style/consistency improvements

### L1: Missing agent prompt files should error, not fallback
**Files**: `pkg/engine/runner.go` (line 68-69)
**Issue**: If `scout.md` is missing, a hardcoded one-line fallback prompt is used silently (no warning, no error). The fallback is: `"You are a Scout agent. Analyze the codebase and produce an IMPL doc."`
**Risk**: Scout runs with severely degraded instructions. Output quality degrades silently and is hard to diagnose.
**Fix**: Make missing prompt files an error: `return fmt.Errorf("scout.md not found at %s", path)`.
**Effort**: Low (0.5 day)

### L2: Standardize validation error context
**Files**: All validation files in `pkg/protocol/`
**Issue**: `ValidationError` struct has `Code`, `Message`, `Field`, `Line` but no `Slug`, `Wave`, or `AgentID`. Usage is inconsistent — some callers populate `Field`, many don't. Messages vary between "agent X not found" and "agent X (wave Y) not found" with no standard format.
**Risk**: Debugging validation failures requires manual cross-referencing of error messages with IMPL doc context.
**Fix**: Add `Slug`, `Wave`, `AgentID` fields to `ValidationError`. Audit all call sites to populate consistently.
**Effort**: Low-Medium (1-2 days)

### L3: Consolidate multi-repo resolution logic
**Files**: `pkg/protocol/worktree.go`, `pkg/protocol/cleanup.go`, `cmd/saw/prepare_wave.go`
**Issue**: Two separate functions resolve agent-to-repo mapping: `determineAgentRepo()` (pkg/protocol, returns repo name string from file_ownership) and `resolveAgentRepoRoot()` (cmd/saw, returns absolute path using repo registry + fallback). The CLI version has strictly more logic (registry lookup, fallback to projectRoot) that the protocol version lacks.
**Fix**: Extract a unified `ResolveAgentRepo()` into `pkg/protocol/multi_repo.go` that handles both the name lookup and path resolution, so the CLI doesn't need its own copy.
**Effort**: Low (1 day)

### L4: Remove raw YAML fallback for IntegrationConnectors
**Files**: `pkg/engine/constraints.go` (lines 84, 143-169)
**Issue**: `IntegrationConnectors` field exists on `IMPLManifest` in `pkg/protocol/types.go:45`, but `BuildIntegratorConstraints()` still calls `loadIntegrationConnectors()` which re-reads and re-parses the YAML file to extract the same field. A stale comment at line 84 says "manifest.IntegrationConnectors is not yet on the typed struct" — but it is.
**Fix**: Remove `loadIntegrationConnectors()`. Use `manifest.IntegrationConnectors` directly in `BuildIntegratorConstraints()`. Remove the stale comment.
**Effort**: Low (0.5 day)

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

### Phase 5: Cleanup
- **L1-L4**: Prompt enforcement, error standardization, repo resolution consolidation, raw YAML removal
