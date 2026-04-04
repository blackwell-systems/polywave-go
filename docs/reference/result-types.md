# Result Types Reference

Package: `pkg/result`

---

## Result[T]

```go
type Result[T any] struct {
    Data   *T         // nil on FATAL; non-nil on SUCCESS and PARTIAL
    Errors []SAWError // empty on SUCCESS; warnings on PARTIAL; one or more errors on FATAL
    Code   string     // "SUCCESS" | "PARTIAL" | "FATAL"
}
```

`Result[T]` is the single return type for all engine operations. It replaces 68 previously distinct `*Result` types and eliminates inconsistent success-checking patterns (`IsSuccess` vs `Success` vs `Ok` vs `Error==nil`).

### Status codes

| Code | Data | Errors | Meaning |
|------|------|--------|---------|
| `SUCCESS` | present | empty | Operation completed without issues. |
| `PARTIAL` | present | warnings | Operation produced usable output; warnings require caller attention but do not block. |
| `FATAL` | nil | one or more errors | Operation failed; no usable output was produced. |

### Methods

| Method | Returns | Notes |
|--------|---------|-------|
| `IsSuccess() bool` | `true` when `Code == "SUCCESS"` and `len(Errors) == 0` | Both conditions must hold; a `Code == "SUCCESS"` with stray errors returns `false`. |
| `IsPartial() bool` | `true` when `Code == "PARTIAL"` | Data is present but warnings in `Errors` should be surfaced to the caller. |
| `IsFatal() bool` | `true` when `Code == "FATAL"` | `Data` is nil; do not call `GetData()` without first checking. |
| `HasErrors() bool` | `true` when `len(Errors) > 0` | True for both `PARTIAL` and `FATAL`. Does not distinguish severity. |
| `GetData() T` | `*Data` or zero value of `T` | Returns the zero value when `Data == nil`. Always check `IsSuccess()` or `IsPartial()` before using the return value. |

### Constructors

```go
// Full success — no warnings.
result.NewSuccess(data T) Result[T]

// Partial success — data is usable, warnings are non-empty.
result.NewPartial(data T, warnings []SAWError) Result[T]

// Failure — data is nil, operation did not complete.
result.NewFailure(errors []SAWError) Result[T]
```

`NewFailure` always sets `Code` to `"FATAL"` regardless of the severity values on the supplied errors, because any call to `NewFailure` signals that the operation did not produce output.

### JSON serialisation

`Data`, `Errors`, and `Code` serialise as-is. `Data` is omitted when nil (`omitempty`). `Errors` is omitted when the slice is empty. `Code` is always present.

### Utility

```go
// Converts []SAWError to []error for errors.Join or standard range loops.
result.ToErrors(errs []SAWError) []error
```

---

## SAWError

```go
type SAWError struct {
    Code       string            // structured error code, e.g. "V001_MANIFEST_INVALID"
    Message    string            // human-readable description
    Severity   string            // "fatal" | "error" | "warning" | "info"
    File       string            // source file path, if applicable
    Line       int               // line number within File, if applicable
    Field      string            // field name for validation errors
    Tool       string            // tool name for tool/parse errors
    Suggestion string            // optional remediation hint for the caller
    Context    map[string]string // free-form key-value pairs: slug, wave, agent_id, rule, column
    Cause      error             // wrapped error; excluded from JSON; used by errors.Is/As
}
```

`SAWError` implements `error`. Its `Error()` string format is:

```
[<severity>] <code>: <message>
```

or, when severity is empty:

```
[<code>] <message>
```

### Severity levels

| Value | Meaning |
|-------|---------|
| `"fatal"` | Operation cannot continue; caller must treat as hard failure. `IsFatal()` returns `true`. |
| `"error"` | A significant problem that caused the operation to fail or produce incomplete output. |
| `"warning"` | Advisory condition; operation output is still usable. Used in `PARTIAL` results. |
| `"info"` | Informational annotation; never blocks execution. |

Note: the severity on a `SAWError` and the `Code` on `Result[T]` are independent. A `FATAL` result can contain errors with severity `"error"` (not `"fatal"`) because `NewFailure` sets `Code` to `"FATAL"` unconditionally.

### Methods

```go
// Reports whether Severity == "fatal".
(e SAWError) IsFatal() bool

// Returns a copy with an additional Context key-value pair.
(e SAWError) WithContext(key, value string) SAWError

// Returns a copy with Cause set to the given error.
(e SAWError) WithCause(cause error) SAWError

// Returns Cause for errors.Is/As chain traversal.
(e SAWError) Unwrap() error
```

### Constructors

```go
result.NewError(code, message string) SAWError   // Severity: "error"
result.NewFatal(code, message string) SAWError   // Severity: "fatal"
result.NewWarning(code, message string) SAWError // Severity: "warning"
```

### Attaching context

```go
err := result.NewError(result.CodeMergeConflict, "conflict in main.go").
    WithContext("agent_id", "A1").
    WithContext("wave", "2").
    WithCause(underlyingErr)
```

---

## Error code domains

All `SAWError.Code` values are defined as constants in `pkg/result/codes.go`. Each code begins with a domain prefix and a three-digit number. Active domains: V, W, B, G, A, N, O, P, T, Z (plus S, C, K, I, D, E, X, Q, R, J for internal subsystems).

### V — Validation (V001–V099)

IMPL doc structural validation: schema correctness, field presence, ownership coverage, dependency graph rules, and state consistency. Failures here mean the IMPL doc cannot be acted upon.

Selected codes:

| Code | Constant | Meaning |
|------|----------|---------|
| `V001_MANIFEST_INVALID` | `CodeManifestInvalid` | Top-level manifest structure is malformed. |
| `V002_DISJOINT_OWNERSHIP` | `CodeDisjointOwnership` | File ownership sets are disjoint across agents when they must overlap or vice-versa. |
| `V003_SAME_WAVE_DEPENDENCY` | `CodeSameWaveDependency` | Agent declares a dependency on another agent in the same wave. |
| `V004_WAVE_NOT_1INDEXED` | `CodeWaveNotOneIndexed` | Waves are not numbered starting at 1 with no gaps. |
| `V005_REQUIRED_FIELDS_MISSING` | `CodeRequiredFieldsMissing` | One or more required fields absent. |
| `V006_FILE_OWNERSHIP_INCOMPLETE` | `CodeFileOwnershipIncomplete` | Files declared in scope are not fully assigned to agents. |
| `V007_DEPENDENCY_CYCLE` | `CodeDependencyCycle` | Agent dependency graph contains a cycle. |
| `V008_INVALID_STATE` | `CodeInvalidState` | IMPL state field holds an unrecognised value. |
| `V016_JSONSCHEMA_FAILED` | `CodeJSONSchemaFailed` | Document failed JSON Schema validation. |
| `V036_INVALID_ENUM` | `CodeInvalidEnum` | Field value is not a member of the allowed enum set. |
| `V046_PARSE_ERROR` | `CodeParseError` | YAML or JSON could not be parsed. |
| `V047_TRIVIAL_SCOPE` | `CodeTrivialScope` | IMPL has only one agent owning one file; SAW provides no parallelisation value. |

### W — Warnings (W001–W099)

Advisory conditions. Severity is always `"warning"`. These codes never appear in a `FATAL` result and never block execution.

| Code | Constant | Meaning |
|------|----------|---------|
| `W001_AGENT_SCOPE_LARGE` | `CodeAgentScopeLarge` | Agent owns more than 8 files or creates more than 5 new files. |
| `W002_COMPLETION_VERIFY` | `CodeCompletionVerificationWarning` | Agent has no commits on its wave branch after execution. |

### B — Build and gate (B001–B099)

Compilation, test, lint, and format-check failures emitted by gate execution. Also covers gate configuration problems.

| Code | Constant | Meaning |
|------|----------|---------|
| `B001_BUILD_FAILED` | `CodeBuildFailed` | Compilation failed. |
| `B002_TEST_FAILED` | `CodeTestFailed` | Test suite failed. |
| `B003_LINT_FAILED` | `CodeLintFailed` | Lint check failed. |
| `B004_FORMAT_CHECK_FAILED` | `CodeFormatCheckFailed` | Formatter check failed. |
| `B005_GATE_TIMEOUT` | `CodeGateTimeout` | Gate command exceeded its time limit. |
| `B006_GATE_COMMAND_MISSING` | `CodeGateCommandMissing` | Gate references a command that does not exist. |
| `B007_STUB_DETECTED` | `CodeStubDetected` | Gate output indicates unimplemented stub code. |
| `B008_GATE_INPUT_INVALID` | `CodeGateInputInvalid` | Gate input validation failed. |

### G — Git (G001–G099)

Worktree lifecycle, merge, and commit errors.

| Code | Constant | Meaning |
|------|----------|---------|
| `G001_WORKTREE_CREATE_FAILED` | `CodeWorktreeCreateFailed` | Could not create git worktree. |
| `G002_MERGE_CONFLICT` | `CodeMergeConflict` | Merge produced conflicts that were not auto-resolved. |
| `G003_COMMIT_MISSING` | `CodeCommitMissing` | Expected commit is absent from the branch. |
| `G004_BRANCH_EXISTS` | `CodeBranchExists` | Target branch already exists and cannot be overwritten. |
| `G005_DIRTY_WORKTREE` | `CodeDirtyWorktree` | Worktree has uncommitted changes where a clean state is required. |
| `G006_HOOK_INSTALL_FAILED` | `CodeHookInstallFailed` | Git hook installation failed. |
| `G007_WORKTREE_CLEANUP` | `CodeWorktreeCleanup` | Worktree cleanup after operation failed. |

### A — Agent (A001–A099)

Agent lifecycle and output verification errors.

| Code | Constant | Meaning |
|------|----------|---------|
| `A001_AGENT_TIMEOUT` | `CodeAgentTimeout` | Agent did not complete within the configured timeout. |
| `A002_STUB_DETECTED` | `CodeAgentStubDetected` | Agent output contains unimplemented stub code. |
| `A003_COMPLETION_REPORT_MISSING` | `CodeCompletionReportMissing` | Agent did not write a completion report. |
| `A004_VERIFICATION_FAILED` | `CodeVerificationFailed` | Post-run verification of agent output failed. |
| `A005_AGENT_LAUNCH_FAILED` | `CodeAgentLaunchFailed` | Agent process could not be started. |
| `A006_BRIEF_EXTRACT_FAIL` | `CodeBriefExtractFail` | Could not extract agent brief from IMPL doc. |
| `A007_JOURNAL_INIT_FAIL` | `CodeJournalInitFail` | Agent journal could not be initialised. |

### N — Engine (N001–N099)

Orchestration and state-machine errors. Covers prepare/finalize wave, scout/planner/wave execution, merge, verification, configuration, and all other engine subsystems (N018–N098).

Selected codes:

| Code | Constant | Meaning |
|------|----------|---------|
| `N001_PREPARE_WAVE_FAILED` | `CodePrepareWaveFailed` | Wave preparation step failed. |
| `N002_FINALIZE_WAVE_FAILED` | `CodeFinalizeWaveFailed` | Wave finalization step failed. |
| `N003_SCOUT_FAILED` | `CodeScoutFailed` | Scout operation failed. |
| `N005_IMPL_NOT_FOUND` | `CodeIMPLNotFound` | IMPL doc could not be located. |
| `N006_IMPL_PARSE_FAILED` | `CodeIMPLParseFailed` | IMPL doc could not be parsed. |
| `N007_WAVE_NOT_READY` | `CodeWaveNotReady` | Wave dependencies are not yet satisfied. |
| `N008_STATE_TRANSITION` | `CodeStateTransition` | State machine transition failed. |
| `N013_CONFIG_NOT_FOUND` | `CodeConfigNotFound` | Configuration file not found. |
| `N014_CONFIG_INVALID` | `CodeConfigInvalid` | Configuration file is present but invalid. |

N018–N098 are fine-grained engine operation codes. All follow the `Nxxx_DESCRIPTION` naming pattern (e.g. `N020_SCOUT_RUN_FAILED`, `N025_WAVE_FAILED`, `N094_MANIFEST_SAVE_FAILED`). Consult `pkg/result/codes.go` for the full list.

### P — Protocol (P001–P099)

Invariant and execution-rule violations detected by the protocol layer.

| Code | Constant | Meaning |
|------|----------|---------|
| `P001_STATE_TRANSITION_INVALID` | `CodeStateTransitionInvalid` | A state transition was attempted that violates the protocol state machine. |
| `P002_PROGRAM_VALIDATION_FAILED` | `CodeProgramValidationFailed` | Program-level validation failed. |
| `P003_MIGRATION_BOUNDARY_UNSAFE` | `CodeMigrationBoundaryUnsafe` | Migration would cross an unsafe boundary. |
| `P004_DEPS_NOT_MET` | `CodeDepsNotMet` | Declared dependencies are not satisfied. |
| `P005_INVARIANT_VIOLATION` | `CodeInvariantViolation` | A protocol invariant was violated. |
| `P006_EXECUTION_RULE` | `CodeExecutionRule` | An execution rule was broken. |
| `P007_WIRING_GAP` | `CodeWiringGap` | A wiring gap between components was detected. |

### O — Observability (O001–O099)

Errors from `pkg/observability` when recording or querying events.

| Code | Constant | Meaning |
|------|----------|---------|
| `O001_OBS_EMIT_FAILED` | `CodeObsEmitFailed` | `EmitSync` failed to record an event. |
| `O002_OBS_QUERY_FAILED` | `CodeObsQueryFailed` | An observability query failed. |

### T — Tool/parse (T001–T099)

Errors from the tool runner and `errparse` subsystem.

| Code | Constant | Meaning |
|------|----------|---------|
| `T001_TOOL_ERROR` | `CodeToolError` | Tool execution returned an error. |
| `T002_PARSE_PANIC` | `CodeParsePanic` | Parser panicked; recovered but output is invalid. |
| `T003_TOOL_NOT_FOUND` | `CodeToolNotFound` | Required tool binary is not present on PATH. |
| `T004_TOOL_TIMEOUT` | `CodeToolTimeout` | Tool execution exceeded its time limit. |

### Z — Analyzer (Z001–Z099)

Errors from `pkg/analyzer` during dependency graph construction, cascade detection, import resolution, and wiring analysis.

| Code | Constant | Meaning |
|------|----------|---------|
| `Z001_PARSE_FAILED` | `CodeAnalyzeParseFailed` | A source file could not be parsed. |
| `Z002_GOMOD_READ_FAILED` | `CodeAnalyzeGomodReadFailed` | `go.mod` could not be read. |
| `Z003_MODULE_NOT_FOUND` | `CodeAnalyzeModuleNotFound` | Module path could not be resolved. |
| `Z004_IMPORT_RESOLVE_FAILED` | `CodeAnalyzeImportResolveFailed` | An import path could not be resolved to a directory. |
| `Z005_CYCLE_DETECTED` | `CodeAnalyzeCycleDetected` | Circular dependency detected in the file graph. |
| `Z006_UNSUPPORTED_LANGUAGE` | `CodeAnalyzeUnsupportedLang` | File set contains an unsupported language extension. |
| `Z007_NODE_MISSING` | `CodeAnalyzeNodeMissing` | A referenced file node is absent from the graph. |
| `Z008_JS_PARSER_MISSING` | `CodeAnalyzeJSParserMissing` | `js-parser.js` helper script not found. |
| `Z009_PYTHON_MISSING` | `CodeAnalyzePythonMissing` | Python parser script not found. |
| `Z010_RUST_PARSER_MISSING` | `CodeAnalyzeRustParserMissing` | Rust parser helper binary not found. |
| `Z011_MANIFEST_NIL` | `CodeAnalyzeManifestNil` | Nil manifest passed to `DetectSharedTypes` or `DetectWiring`. |
| `Z012_CIRCULAR_AGENT_DEP` | `CodeAnalyzeCircularAgentDep` | Circular agent dependency detected during wiring analysis. |
| `Z013_WALK_FAILED` | `CodeAnalyzeWalkFailed` | `filepath.Walk` failed during cascade detection. |

---

## Caller handling guide

### Checking outcomes

```go
r := someOperation()

switch {
case r.IsSuccess():
    use(r.GetData())

case r.IsPartial():
    use(r.GetData())
    for _, w := range r.Errors {
        log.Warn(w.Error())
    }

case r.IsFatal():
    for _, e := range r.Errors {
        log.Error(e.Error())
        if e.Suggestion != "" {
            log.Info("suggestion: " + e.Suggestion)
        }
    }
    return r // propagate or wrap
}
```

### Propagating failures

When a callee returns `FATAL`, the caller typically wraps it in a new `FATAL` using a higher-level code:

```go
if r.IsFatal() {
    return result.NewFailure[MyType](append(
        []result.SAWError{result.NewFatal(result.CodePrepareWaveFailed, "prepare wave failed")},
        r.Errors...,
    ))
}
```

### Using errors.Is / errors.As

`SAWError` participates in the standard `errors` chain via `Unwrap()`. Attach the original error with `WithCause`, then use `errors.Is` / `errors.As` as normal:

```go
err := result.NewFatal(result.CodeMergeConflict, "merge failed").
    WithCause(gitErr)

errors.Is(err, gitErr) // true
```

### Converting to []error

```go
errs := result.ToErrors(r.Errors)
combined := errors.Join(errs...)
```

### Accessing structured context

Inspect `SAWError.Context` for operational metadata. Common keys populated by the engine:

| Key | Set by |
|-----|--------|
| `slug` | IMPL identification |
| `wave` | Current wave number |
| `agent_id` | Agent identifier (e.g. `A1`, `A2`) |
| `rule` | Validation rule that triggered |
| `column` | Column number within a file |
