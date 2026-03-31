# Quality Gates

Quality gates are shell commands executed by the engine during wave finalization
to verify that agent work meets structural and content standards before and after
merging agent branches. They are declared in the IMPL manifest under
`quality_gates` and are evaluated by `pkg/protocol/gates.go`.

---

## Table of Contents

1. [YAML Schema](#yaml-schema)
2. [QualityGate Fields](#qualitygate-fields)
3. [Gate Types](#gate-types)
4. [GatePhase Values](#gatephase-values)
5. [Timing Field](#timing-field)
6. [Parallel Groups](#parallel-groups)
7. [Required vs Advisory Gates](#required-vs-advisory-gates)
8. [Auto-Fix Gates](#auto-fix-gates)
9. [Gate Execution in finalize-wave](#gate-execution-in-finalize-wave)
10. [Caching](#caching)
11. [Docs-Only Wave Skipping](#docs-only-wave-skipping)
12. [Build System Skipping](#build-system-skipping)
13. [Example Configurations](#example-configurations)
14. [See Also](#see-also)

---

## YAML Schema

```yaml
quality_gates:
  level: standard           # "quick" | "standard" | "full"
  gates:
    - type: format
      command: ""           # empty = auto-detect formatter
      required: true
      fix: true
      phase: PRE_VALIDATION
      description: "auto-format changed files"

    - type: build
      command: go build ./...
      required: true
      timing: pre-merge
      phase: VALIDATION
      parallel_group: main

    - type: test
      command: go test ./...
      required: true
      timing: pre-merge
      phase: VALIDATION
      parallel_group: main

    - type: lint
      command: golangci-lint run
      required: false
      timing: pre-merge
      phase: VALIDATION

    - type: custom
      command: go vet ./...
      required: true
      timing: post-merge
      phase: POST_VALIDATION
      description: "vet after merge"
      repo: scout-and-wave-go
```

---

## QualityGate Fields

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `type` | string | yes | — | Gate category. Controls auto-detection behavior for `format` gates and build-system skipping for known toolchain commands. |
| `command` | string | yes | — | Shell command executed via `sh -c`. For `format` gates, empty string triggers auto-detection. |
| `required` | bool | yes | — | When `true`, a non-zero exit code blocks the wave. When `false`, failure is recorded but execution continues. |
| `description` | string | no | `""` | Human-readable description shown in CLI output and logs. |
| `repo` | string | no | `""` | If set, gate runs only when the target repo basename matches this value. Used to scope gates in cross-repo IMPLs. |
| `fix` | bool | no | `false` | When `true` on a `format` gate, runs the formatter's fix/rewrite command instead of the check-only command. Must be used with `phase: PRE_VALIDATION`. |
| `timing` | string | no | `""` (= `pre-merge`) | Controls which finalization step executes the gate. `"pre-merge"` or `""` = step 3; `"post-merge"` = step 5.5. |
| `phase` | GatePhase | no | `""` (= `VALIDATION`) | Execution ordering within a timing bucket. See [GatePhase Values](#gatephase-values). |
| `parallel_group` | string | no | `""` | When non-empty, gates sharing the same value execute concurrently within their phase. |

### QualityGates (container)

| Field | Type | Description |
|-------|------|-------------|
| `level` | string | Informational label: `"quick"`, `"standard"`, or `"full"`. Not enforced by the engine; used by Scouts to communicate intent. |
| `gates` | []QualityGate | Ordered list of gate definitions. |

---

## Gate Types

| Type | Description | Build-system detection |
|------|-------------|----------------------|
| `build` | Compilation check | Inferred from command prefix (`go`, `cargo`, `npm`, etc.) |
| `test` | Test suite execution | Same as build |
| `lint` | Static analysis | Same as build |
| `format` | Code formatting | Auto-detects formatter when `command` is empty |
| `typecheck` | Type-only check (e.g. `tsc --noEmit`) | Same as build |
| `custom` | Any command not fitting above categories | Never skipped by build-system detection |

The `format` type receives special handling: when `command` is empty, the engine
calls `format.DetectFormatter()` to discover the project's formatter
(`gofmt`, `prettier`, `rustfmt`, etc.) and selects either its check or fix
command based on the `fix` field. `custom` gates always run regardless of build
system detection.

---

## GatePhase Values

Phase controls the relative ordering of gate execution within a timing bucket
(`pre-merge` or `post-merge`). Gates are grouped into three ordered phases and
executed sequentially across phases.

| Constant | YAML value | Execution order | Typical use |
|----------|-----------|-----------------|-------------|
| `GatePhasePre` | `PRE_VALIDATION` | 1st | Auto-fix gates (`format --fix`, `lint --fix`). Runs sequentially before all validation gates so that fix-rewrites land before checks read files. |
| `GatePhaseMain` | `VALIDATION` | 2nd | Independent structural checks: build, typecheck, test, lint. Gates in this phase may run in parallel via `parallel_group`. Empty `phase` defaults to this value. |
| `GatePhasePost` | `POST_VALIDATION` | 3rd | Review or integration gates that depend on all validation checks having passed. |

Phase execution order: `PRE_VALIDATION` → `VALIDATION` → `POST_VALIDATION`.

Within a phase, gates run sequentially unless they share a `parallel_group`
(see [Parallel Groups](#parallel-groups)).

### Validation constraint

Fix gates (`fix: true`) **must** declare `phase: PRE_VALIDATION`. Placing a
fix gate in `VALIDATION` or `POST_VALIDATION` (or leaving `phase` empty) is a
schema error. `ValidateQualityGate()` returns a fatal error for this condition
and blocks execution before any gate runs.

---

## Timing Field

The `timing` field selects which finalization step runs the gate. It is
independent of `phase`, which controls ordering within that step.

| Value | Finalization step | When it runs |
|-------|-------------------|--------------|
| `""` (empty) | Step 3 (pre-merge) | Before `MergeAgents`. Sees the per-agent worktree state. |
| `"pre-merge"` | Step 3 (pre-merge) | Same as empty. Explicit alias. |
| `"post-merge"` | Step 5.5 (post-merge) | After `MergeAgents` completes and `VerifyBuild` runs. Sees the merged state. |

Pre-merge gates run with optional result caching (keyed on HEAD commit SHA).
Post-merge gates always run without cache because the merged state is always
fresh.

---

## Parallel Groups

By default, gates within a phase execute sequentially in declaration order.
Setting the same `parallel_group` string on multiple gates causes them to run
concurrently within that phase using goroutines.

Rules:

- Gates with an empty `parallel_group` always run sequentially (one gate per
  execution slot), in declaration order, before any parallel groups.
- Gates sharing a non-empty `parallel_group` run concurrently as a unit. All
  gates in the group must complete before the next sequential gate or group
  begins.
- `parallel_group` is scoped to the gate's phase. Two gates in different phases
  with the same `parallel_group` string do not run concurrently.
- Fix gates in `PRE_VALIDATION` should not use `parallel_group`; they modify
  files and must not race with each other.

```yaml
# build and test run concurrently; lint runs sequentially after both finish
gates:
  - type: build
    command: go build ./...
    required: true
    phase: VALIDATION
    parallel_group: fast-checks

  - type: test
    command: go test -short ./...
    required: true
    phase: VALIDATION
    parallel_group: fast-checks

  - type: lint
    command: golangci-lint run
    required: false
    phase: VALIDATION
    # no parallel_group — runs sequentially after fast-checks group
```

---

## Required vs Advisory Gates

| `required` value | Failure behavior |
|-----------------|-----------------|
| `true` | Wave finalization stops immediately. The merge step does not run. The failed gate's type and command are included in the error returned to the caller. |
| `false` | Failure is recorded in `GateResult` and included in the finalization result, but execution continues to the next gate and ultimately to the merge step. |

All required gates must pass for the wave to advance to `MergeAgents`. Advisory
gates (required=false) are surfaced in output and observability events but do
not block progress.

---

## Auto-Fix Gates

Format and lint gates can rewrite files automatically before check gates run.

### Format gates

```yaml
- type: format
  command: ""        # empty = auto-detect (gofmt, prettier, rustfmt, etc.)
  required: true
  fix: true
  phase: PRE_VALIDATION
```

When `fix: true`:

1. The engine runs the formatter's rewrite command (e.g. `gofmt -w .`).
2. After the command exits, the gate cache is **invalidated** so that
   subsequent gates in `VALIDATION` phase see the reformatted files.
3. A zero exit code is treated as pass regardless of whether files were
   modified.

When `fix: false` (default):

- The engine runs the formatter's check command (e.g. `gofmt -l .`).
- Non-zero exit or any output indicates unformatted files.

If `command` is empty and no formatter is detected in the project directory,
the gate is skipped with `Passed: true` and `SkipReason: "no formatter
detected for project type"`.

### Lint fix gates

Standard `lint` gates do not receive special `fix` handling from the engine;
the fix command must be embedded in `command` directly. Only `format` type
gates use auto-detection and fix-mode cache invalidation.

---

## Gate Execution in finalize-wave

The `FinalizeWave` pipeline (defined in `pkg/engine/finalize.go`) runs gates at
two points:

```
Step 1   VerifyCommits
Step 1.1 VerifyCompletionReports
Step 1.2 CheckAgentStatuses
Step 1.3 PredictConflicts
Step 2   ScanStubs
Step 3   RunPreMergeGates  ← quality_gates with timing="" or timing="pre-merge"
Step 3.3 CheckTypeCollisions (opt-in)
Step 3.5 ValidateIntegration
Step 3.6 CheckWiringDeclarations
Step 4   MergeAgents
Step 4.2 PopulateIntegrationChecklist
Step 4.5 FixGoMod
Step 5   VerifyBuild (test_command + lint_command)
Step 5.5 RunPostMergeGates ← quality_gates with timing="post-merge"
Step 6   Cleanup
```

Pre-merge gates run per-repo for cross-repo IMPLs. Post-merge gates also run
per-repo. Within each repo, the phase ordering (`PRE_VALIDATION` →
`VALIDATION` → `POST_VALIDATION`) is applied independently.

### Failure behavior in finalize-wave

- If any **required** pre-merge gate fails, `FinalizeWave` returns immediately
  with an error. Steps 3.3 through 6 do not run.
- If the closed-loop retry option is enabled (`ClosedLoopRetryEnabled: true`),
  the engine attempts to fix the failing gate via `ClosedLoopGateRetry` before
  giving up.
- If any **required** post-merge gate fails, cleanup still runs before
  `FinalizeWave` returns an error.
- Advisory gates (required=false) never stop the pipeline.

### Baseline gates

Before worktree creation, `RunBaselineGates()` (called by the `prepare-wave`
step) runs pre-merge gates against the current HEAD to verify the baseline is
healthy. Baseline gate results are cached by HEAD commit SHA and shared with
the later finalize-wave gate run to avoid redundant execution.

---

## Caching

Pre-merge gates use a result cache stored under `.saw/` in the repo directory.
The cache key combines the HEAD commit SHA and the gate command string.

- Cache TTL: 5 minutes (configurable via `gatecache.DefaultTTL`).
- A cache hit returns the stored `Passed`, `ExitCode`, `Stdout`, and `Stderr`
  without re-running the command.
- Fix-mode format gates always invalidate the cache after running because they
  modify files.
- Post-merge gates never use the cache.

---

## Docs-Only Wave Skipping

When every file owned by the wave has a documentation or configuration
extension (`.md`, `.yaml`, `.yml`, `.txt`, `.rst`), the engine automatically
skips all source-code gate types (`build`, `test`, `tests`, `lint`, `format`)
with `Skipped: true` and `Passed: true`. Custom gates always run.

---

## Build System Skipping

Before running a gate, the engine infers the required build system from the
first token of the gate command:

| Command prefix | Inferred build system | Marker file |
|---------------|----------------------|-------------|
| `go`, `golangci-lint`, `staticcheck` | go | `go.mod` |
| `npm`, `yarn`, `pnpm`, `npx`, `node` | node | `package.json` |
| `cargo` | rust | `Cargo.toml` |
| `python`, `python3`, `pip`, `pytest`, `ruff`, `mypy`, `uv` | python | `pyproject.toml` or `setup.py` |
| `mvn` | maven | `pom.xml` |
| `gradle`, `gradlew`, `./gradlew` | gradle | `build.gradle` or `build.gradle.kts` |
| `custom` type, or unrecognized prefix | — | never skipped |

If the inferred build system's marker file is absent from the repo directory,
the gate is skipped with `Passed: true`. This prevents Go-specific gates from
failing in a JavaScript-only repo that happens to share a cross-repo IMPL.

---

## Example Configurations

### Minimal Go project

```yaml
quality_gates:
  level: standard
  gates:
    - type: format
      command: ""
      required: true
      fix: true
      phase: PRE_VALIDATION

    - type: build
      command: go build ./...
      required: true

    - type: test
      command: go test ./...
      required: true

    - type: lint
      command: golangci-lint run
      required: false
```

### Parallel build + test with sequential lint

```yaml
quality_gates:
  level: full
  gates:
    - type: format
      command: gofmt -w .
      required: true
      fix: true
      phase: PRE_VALIDATION

    - type: build
      command: go build ./...
      required: true
      phase: VALIDATION
      parallel_group: checks

    - type: test
      command: go test -race ./...
      required: true
      phase: VALIDATION
      parallel_group: checks

    - type: lint
      command: golangci-lint run --timeout 5m
      required: true
      phase: VALIDATION
      # sequential — runs after the parallel "checks" group finishes
```

### Post-merge integration check

```yaml
quality_gates:
  level: standard
  gates:
    - type: build
      command: go build ./...
      required: true
      timing: pre-merge

    - type: custom
      command: ./scripts/check-wiring.sh
      required: true
      timing: post-merge
      description: "verify exported symbols are wired after merge"
```

### Cross-repo gate scoping

```yaml
quality_gates:
  level: standard
  gates:
    - type: test
      command: go test ./...
      required: true
      repo: scout-and-wave-go    # only runs in this repo

    - type: test
      command: npm test
      required: true
      repo: scout-and-wave-web   # only runs in this repo
```

---

## See Also

- `pkg/protocol/types.go` — `QualityGate`, `QualityGates`, `GatePhase` definitions
- `pkg/protocol/gates.go` — `RunGatesWithCache`, `RunPreMergeGates`, `RunPostMergeGates`
- `pkg/protocol/baseline_gates.go` — `RunBaselineGates` (pre-worktree baseline check)
- `pkg/engine/finalize.go` — `FinalizeWave` pipeline showing gate placement
- `pkg/engine/finalize_steps.go` — `StepRunGates` and closed-loop retry integration
- `pkg/gatecache` — gate result cache implementation
- `docs/reference/orchestration.md` — FinalizeWave pipeline overview
