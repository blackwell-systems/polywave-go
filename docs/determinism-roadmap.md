# Determinism Roadmap — Reducing AI Reliance in the Engine

**Last reviewed**: 2026-03-24
**Goal**: Move validation from post-execution to runtime/pre-execution. Every protocol invariant should be enforced by code, not by trusting agents to follow instructions.

---

## Completed

All CRITICAL (C1-C6), HIGH (H1-H4), and MEDIUM (M1-M3) items have been implemented. See git history for details:

- Runtime constraint enforcement: `pkg/tools/constraint_enforcer.go`, `pkg/tools/role_middleware.go`
- Completion report validation: `pkg/protocol/completion_validator.go`
- Pre-merge commit verification: `pkg/engine/finalize.go`
- File ownership coverage: `pkg/protocol/ownership_coverage.go`
- Dependency availability: `pkg/protocol/dependency_verifier.go`
- Gate input validation: `pkg/protocol/gate_input_validator.go`
- Pre-merge ownership check: `pkg/protocol/merge_agents.go`
- Wave base commit validation (E21A): `pkg/protocol/baseline_gates.go`, `pkg/orchestrator/merge.go`
- L1-L4 cleanup: All implemented (prompt file errors, ValidationError context, multi-repo consolidation, raw YAML removal)
- **M1: Atomic RunWave transaction wrapper** — `pkg/engine/run_wave_atomic.go` with rollback on partial failure. Tests in `run_wave_atomic_test.go`.
- **M2: Integration validation enforcement (E25)** — `EnforceIntegrationValidation` field on `FinalizeWaveOpts` in `pkg/engine/finalize.go`. When true, unconnected exports block merge. Tests in `finalize_test.go`.
- **M3: Mandatory stub detection (E20)** — `RequireNoStubs` field on `FinalizeWaveOpts` in `pkg/engine/finalize.go`. When true, TODO/FIXME stubs block merge. Tests in `finalize_test.go`.

---

## Remaining

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
