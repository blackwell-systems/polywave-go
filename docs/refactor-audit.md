# Go Engine Refactor Audit

## Summary

58 issues found across 6 categories. Estimated scope: **large**. Reclassifications since initial audit: Issue 3 (StartWave/startWaveWithGate divergence) promoted to Critical; Issue 8 (CLI backend constraint bypass) promoted to High with expanded fix options; Issue 16 (ScaffoldFile tag mismatch) promoted to Critical.

The codebase is in good structural shape — `pkg/result`, `pkg/protocol/types.go`, and the step-function architecture are well-designed. The debt is concentrated in three clusters: (1) `context.Context` was added as a parameter to `LoadYAML`/`SaveYAML` but was never threaded through the 40+ internal call sites that still pass `context.TODO()`; (2) the engine has two nearly-identical wave-loop implementations (`StartWave` and `startWaveWithGate`) that diverged and are not being kept in sync; and (3) the engine uses 45+ ad-hoc `"ENGINE_*"` error codes that do not appear in `pkg/result/codes.go`, defeating the purpose of the catalog.

---

## Decisions Required Before Scouting

~~Two decisions need human input before a Scout can produce accurate IMPL docs.~~ **Both decisions made — Scout can proceed on all IMPLs.**

### Decision 1: ctx in LoadYAML/SaveYAML (Issue 11) — ✅ Choice A

**Chosen:** Remove the `ctx` parameter entirely. `LoadYAML`/`SaveYAML` do synchronous disk I/O via `os.ReadFile`/`os.WriteFile`; the ctx parameter is a no-op today and misleads callers into thinking cancellation is active. Removing it eliminates ~40 `context.TODO()` call sites and honestly reflects the implementation. Scope: ~40 call site cleanups.

### Decision 2: CLI backend architectural stance (Issue 8) — ✅ Choice A

**Chosen:** Post-hoc enforcement via event stream inspection. The CLI backend already uses `--output-format stream-json` and `RunStreamingWithTools` already parses `tool_use` content blocks extracting `file_path` from Write/Edit inputs. Implement a post-hoc enforcement pass that replays each recorded Write/Edit invocation against I1/I2/I6 logic from `pkg/tools/constraint_enforcer.go` (`OwnedFiles`, `FrozenPaths`+`FreezeTime`, `AllowedPathPrefixes`) and returns structured violations. Limitation: post-hoc cannot prevent the write, only detect and report it. Scope: new enforcement pass in `pkg/agent/backend/cli/`, plus plumbing to pass `Constraints` into the CLI client (~3 packages).

---

## Critical (blocking correctness or consistency)

### 1. Nil `ObsEmitter` panic in `RunScout` — `pkg/engine/runner.go:177`

**Problem:** `opts.ObsEmitter.Emit(...)` is called unconditionally at line 177 (`// E40: Emit scout_complete on success`). The preceding `EmitSync` at line 68 is correctly nil-guarded (`if opts.ObsEmitter != nil`), but the success-path call at 177 is not. The `ObsEmitter` field is documented as optional in `RunScoutOpts`. Any caller that omits it (e.g., the CLI's `run_scout_cmd.go`) will panic on a successful Scout run.

**Fix:** Wrap line 177 with `if opts.ObsEmitter != nil { ... }`.

**Scope:** 1 file, 1 line.

---

### 2. Nil `ObsEmitter` panics in `TierLoop` — `pkg/engine/program_tier_loop.go:246,280,318,335`

**Problem:** `TierLoopOpts.ObsEmitter` is documented as `// optional`, but four bare `opts.ObsEmitter.Emit(...)` calls in the tier loop body (lines 246, 280, 318, 335) have no nil guard. If a caller constructs `TierLoopOpts{}` without setting `ObsEmitter`, these calls will panic.

**Fix:** Either add `if opts.ObsEmitter != nil` guards or use a `nilSafeEmit` helper (same pattern as `loggerFrom`).

**Scope:** 1 file, 4 lines.

---

### 3. Duplicate wave-loop logic between `StartWave` and `startWaveWithGate` — `pkg/engine/runner.go:402–637` and `1422–1541`

**Problem:** `startWaveWithGate` (line 1422) is a 120-line private function that reimplements the wave loop from `StartWave` (line 402). Both implement the same sequence: `RunWave → MergeWave → RunVerification → E25/E26 integration validation → UpdateIMPLStatus`. The two copies have diverged: `StartWave` handles worktree creation via `protocol.CreateWorktrees`, emits an E18 context update, and handles the scaffold pre-step — none of which appear in `startWaveWithGate`. The E25/E26 integration blocks (lines 555–589 and 1470–1505) are line-for-line copies.

Beyond the current duplication, this is a feature-maintenance trap: any future change to integration gap detection, wave sequencing logic, or post-wave hooks must be made in two places. As the engine grows, the two copies will continue to diverge in ways that are hard to detect. The E25/E26 copy alone is 60 lines duplicated twice — the next person to add a wave-level invariant will either miss `startWaveWithGate` or introduce a subtle behavioral difference.

**Fix:** Refactor the per-wave body into a shared helper `runOneWave(ctx, orch, opts, waveNum, gateCh)`. `StartWave` and `startWaveWithGate` become thin wrappers around it. The duplicate E25/E26 block (60 lines, duplicated twice) disappears.

**Scope:** `pkg/engine/runner.go` (1 file, ~180 lines of dead duplicate).

---

### 4. Legacy error code string `"E16_REPO_MISMATCH_SUSPECTED"` hardcoded in engine — `pkg/engine/prepare.go:255`

**Problem:** `PrepareWave` checks `if w.Code == "E16_REPO_MISMATCH_SUSPECTED"`. This is the old pre-migration error code. The current catalog defines `result.CodeRepoMismatch = "V045_REPO_MISMATCH"`. The check will silently miss the error if `ValidateFileExistenceMultiRepo` is ever updated to emit the canonical code.

**Fix:** Replace the string literal with `result.CodeRepoMismatch`.

**Scope:** 1 file, 1 line.

---

### 16. `ScaffoldFile` has a YAML/JSON field name mismatch — `pkg/protocol/types.go:197`

**Problem:**
```go
type ScaffoldFile struct {
    FilePath   string `yaml:"file" json:"file_path"`
```
The YAML field is `file` but the JSON field is `file_path`. Any YAML↔JSON roundtrip of a `ScaffoldFile` will silently drop the field value — YAML serializes it as `file:`, JSON expects `file_path:`. This affects structured Scout output (which uses JSON) and IMPL manifest files (which use YAML). A scaffold that appears to load successfully from YAML will have an empty `FilePath` when the same data is read via JSON, causing a wave to launch with broken scaffold state.

**Confirmed:** There are no roundtrip tests for `ScaffoldFile` serialization. `pkg/protocol/scaffold_validation_test.go` tests only validation logic with pre-constructed in-memory structs — no test marshals a `ScaffoldFile` to JSON and unmarshals as YAML, or vice versa. The fix scope therefore includes adding roundtrip tests.

**Fix:** Pick one canonical name. `file_path` is used in `integration_types.go` and `scaffold_validation.go`. Align YAML tag to `yaml:"file_path"` (or keep `yaml:"file"` and add migration logic in `LoadYAML`). Add a `TestScaffoldFileRoundtrip` test that marshals to JSON, unmarshals as YAML (and vice versa), and asserts `FilePath` is preserved.

**Scope:** `pkg/protocol/types.go` (1 line), any YAML fixtures in test files that use `file:`, plus new roundtrip test.

---

## High (significant debt, worth fixing before new features)

### 5. `context.TODO()` inside functions that already receive a `ctx` — multiple files

**Problem:** The most impactful propagation gaps (caller already has a live `ctx` but discards it):

| File | Line(s) | Function | Gap |
|---|---|---|---|
| `pkg/protocol/state_transition.go` | 47 | `SetImplState` | `Load(context.TODO(), ...)` — no ctx param on the function |
| `pkg/protocol/status_update.go` | 32 | `UpdateStatus` | `Load(context.TODO(), ...)` — no ctx param |
| `pkg/protocol/verify_build.go` | 36 | `VerifyBuild` | `Load(context.TODO(), ...)` — no ctx param, runs shell commands |
| `pkg/protocol/merge_agents.go` | 167 | `MergeAgents` | `Load(context.TODO(), ...)` — no ctx param |
| `pkg/protocol/worktree.go` | 48, 92 | `CreateWorktrees` | `Load/Save(context.TODO(), ...)` — no ctx param |
| `pkg/protocol/stubs.go` | 100, 111 | `PersistStubReport` | `Load/Save(context.TODO(), ...)` — no ctx param |
| `pkg/protocol/cleanup.go` | 97 | `Cleanup` | already accepts ctx, passes `context.TODO()` to `Load` |
| `pkg/protocol/commit_verify.go` | 50 | `VerifyCommits` | already accepts ctx, passes `context.TODO()` to `Load` |
| `pkg/protocol/gate_populator.go` | 286, 347, 443 | `PopulateGates` | no ctx param, runs `extractor.Extract` |
| `pkg/engine/runner.go` | 806, 1250, 1292 | `runScaffoldBuildVerification`, `runScoutAutomation` | calls `exec.CommandContext(context.Background(), ...)` discarding parent ctx |
| `pkg/protocol/program_generator.go` | 80, 257 | `GenerateProgramManifest` | no ctx param |
| `pkg/retry/loop.go` | 50 | `RetryLoop.Run` | `ctx = context.Background()` assigned when nil — silently drops caller's deadline |

`MergeAgents`, `VerifyBuild`, `CreateWorktrees`, `SetImplState`, and `UpdateStatus` are the most impactful because they run git operations or shell commands that can block indefinitely with no cancellation path.

**Fix:** Add `ctx context.Context` as the first parameter to these functions. Their callers (engine functions, sawtools commands) already carry a `ctx` and can pass it directly.

**Scope:** 8–12 files in `pkg/protocol/`, 3 in `pkg/engine/`, ~30 sawtools call sites.

---

### 6. 45+ ad-hoc `"ENGINE_*"` error codes not in `pkg/result/codes.go` — `pkg/engine/`

**Problem:** The engine emits structured `result.SAWError` values, but nearly all of its codes are ad-hoc strings (`"ENGINE_SCOUT_FAILED"`, `"ENGINE_WAVE_FAILED"`, `"ENGINE_HOOK_VERIFY_FAILED"`, `"CONTEXT_CANCELLED"`, `"ENGINE_WAVE_SEQUENCING_FAILED"`, etc.) that do not appear in `pkg/result/codes.go`. This means no programmatic error identification, no IDE autocomplete, no grep-ability by canonical code. The `codes.go` catalog already has an `N001–N017` range for engine codes — the engine simply isn't using it.

Distinct ad-hoc codes found (partial list):
- `"ENGINE_SCOUT_INVALID_OPTS"`, `"ENGINE_SCOUT_FAILED"`, `"ENGINE_SCOUT_BOUNDARY_VIOLATION"`
- `"ENGINE_PLANNER_INVALID_OPTS"`, `"ENGINE_PLANNER_FAILED"`
- `"ENGINE_WAVE_INVALID_OPTS"`, `"ENGINE_WAVE_FAILED"`, `"ENGINE_WAVE_SEQUENCING_FAILED"`
- `"ENGINE_HOOK_VERIFY_FAILED"`, `"ENGINE_SCAFFOLD_FAILED"`, `"ENGINE_AGENT_FAILED"`
- `"ENGINE_MERGE_FAILED"`, `"ENGINE_MARK_COMPLETE_FAILED"`, `"ENGINE_VERIFY_TIERS_INCOMPLETE"`
- `"CONTEXT_CANCELLED"` (used in 14 places across pkg/engine and pkg/notify with no code constant)

**Fix:** Add constants to `pkg/result/codes.go` under the existing `N*`/`A*` ranges. Replace inline string literals with the constants throughout `pkg/engine/`.

**Scope:** `pkg/result/codes.go` (additions), ~20 engine files (replacements).

---

### 7. `executeTool` duplicated across three backends — `pkg/agent/backend/api/client.go:173`, `pkg/agent/backend/bedrock/tools.go:27`

**Problem:** The function `executeTool(ctx, workshop, name, input, workDir)` is defined twice with nearly identical signatures and logic: once in `pkg/agent/backend/api/` and once in `pkg/agent/backend/bedrock/`. The OpenAI backend has a similar `execTool` at `pkg/agent/backend/openai/client.go:120`. All three perform: look up tool by name → deserialize input → call executor → return (output, isError).

**Fix:** Move `executeTool` to `pkg/agent/backend/` (the shared package) or to `pkg/tools/` and import it from all three backends.

**Scope:** 3 files (`api/client.go`, `bedrock/tools.go`, `openai/client.go`), plus a new shared location.

---

### 8. `pkg/tools` constraint enforcement not applied in CLI backend — `pkg/agent/backend/cli/client.go`

**Problem:** The API backend (`api/client.go:89–93`) and Bedrock backend (`bedrock/client.go:260–264`) both apply `tools.WithConstraints(w, *c.cfg.Constraints)` when `cfg.Constraints != nil`. The CLI backend (`cli/client.go`) does not reference `Constraints` at all — it uses `--allowedTools Bash,Read,Write,Edit,Glob,Grep` hardcoded at the process level and never applies the SAW ownership middleware (I1), freeze middleware (I2), or role-path middleware (I6). Any CLI-backend agent bypasses all constraint enforcement silently.

This is not a documentation gap — it is a trust model hole. The protocol's main selling point is enforcement-based correctness. An agent running via the CLI backend can write to any path it chooses with no violation emitted and no record in the protocol state. Silent bypass is worse than explicit unsupport.

**Assessed feasibility of post-hoc enforcement:** The CLI backend (`cli/client.go`) already uses `--output-format stream-json` and the `RunStreamingWithTools` method already parses structured JSON events in real time, extracting `tool_use` content blocks with tool name and `file_path` from Write/Edit inputs (see `extractToolInput`). A post-hoc enforcement pass is technically feasible: after the subprocess completes, replay each recorded Write/Edit tool invocation against the I1/I2/I6 logic from `pkg/tools/constraint_enforcer.go` — check `OwnedFiles[filePath]`, `FrozenPaths[filePath] && FreezeTime != nil`, and `AllowedPathPrefixes`. Violations would be reported as structured errors after the fact. The limitation: post-hoc enforcement cannot prevent the write from happening, only detect and report it.

**Fix (Decision 2 — Choice A):** Implement a post-hoc enforcement pass in `pkg/agent/backend/cli/`. Thread `cfg.Constraints` into the CLI client config. After `RunStreamingWithTools` completes, inspect the accumulated tool-use events and flag any Write/Edit to an unowned, frozen, or out-of-role path against the I1/I2/I6 logic in `pkg/tools/constraint_enforcer.go`. Return structured constraint violation errors that cause the wave agent result to be marked as blocked. Note: post-hoc cannot prevent the write from occurring, only detect and report it after the fact.

**Scope:** `pkg/agent/backend/cli/client.go`, `pkg/agent/backend/backend.go`, `pkg/agent/backend/cli/` (new enforcement pass, ~3 files).

---

### 9. `implSlugFromPath` and `implSlugFromIMPLPath` are duplicated — `pkg/engine/finalize.go:703` and `pkg/engine/runner.go:188`

**Problem:** Two private functions with different names do the same thing: strip the `IMPL-` prefix and `.yaml` extension from a path to get the slug. `pkg/protocol/impl_slug.go` already exports `ExtractIMPLSlug(implPath, manifest)` which does the same. The engine functions predate `ExtractIMPLSlug` but were never cleaned up.

**Fix:** Delete `implSlugFromPath` and `implSlugFromIMPLPath`; replace their call sites with `protocol.ExtractIMPLSlug(path, nil)` (nil manifest falls through to path parsing, which is the same behavior).

**Scope:** 2 engine files, ~4 call sites.

---

### 10. `finalize.go` `firstRepo` copy-paste anti-pattern — `pkg/engine/finalize.go`

**Problem:** The `FinalizeWave` function contains 21 occurrences of the pattern:
```go
firstRepo := opts.RepoPath
for _, rp := range repos {
    firstRepo = rp
    break
}
repoOpts := opts
repoOpts.RepoPath = firstRepo
```
This is copy-pasted 9 times for each manifest-level step (steps 1.1, 1.2, 1.3, 2, 3.5, 3.6, 4.2, 5). The pattern is never extracted into a helper.

**Fix:** Extract `firstRepoOpts(opts FinalizeWaveOpts, repos map[string]string) FinalizeWaveOpts` and use it.

**Scope:** `pkg/engine/finalize.go`, ~80 lines of reduction.

---

### 11. `pkg/protocol/yaml_io.go`: `ctx` is reserved but never forwarded — `pkg/protocol/yaml_io.go:48,63`

**Problem:** `LoadYAML` and `SaveYAML` both accept `ctx context.Context` but immediately execute `_ = ctx // reserved for future cancellation support`. This means every `context.TODO()` call into these functions is technically correct today but represents a design debt: the comment promises future cancellation that won't happen unless the internal `os.ReadFile`/`os.WriteFile` is replaced with cancellation-aware I/O (e.g., reading into a goroutine and selecting on ctx.Done). With 40+ `context.TODO()` call sites, the practical impact of making ctx real would be significant.

**Fix (Decision 1 — Choice A):** Remove the `ctx context.Context` parameter from `LoadYAML` and `SaveYAML`. Replace all ~40 call sites (`context.TODO()` and `context.Background()` passed to these functions) with direct calls without a ctx argument. Update `Load`, `Save`, and all wrapper functions that forward ctx to these primitives.

**Scope:** `pkg/protocol/yaml_io.go` + all 40+ call sites in `pkg/protocol/`, `pkg/engine/`, `pkg/queue/`, `pkg/interview/`, `pkg/deps/`, `pkg/scaffold/`, `pkg/scaffoldval/`, `pkg/retry/`.

---

## Medium (cleanup, consistency)

### 12. `map[string]interface{}` for pipeline `State.Values` — `pkg/pipeline/types.go:33`

**Problem:** `State.Values map[string]interface{}` is the pipeline's generic state bag. In practice, callers will need to type-assert everything they store and retrieve. The existing `pkg/pipeline/saw_steps.go` registers the engine-specific steps but has to work around this untyped map. A typed extension point (e.g., `Values map[string]any` with a generic accessor, or a dedicated `SAWPipelineState` embedding `State`) would be safer.

**Fix:** Replace with typed fields for known SAW-specific data (manifest, wave result, etc.) or use `any` with a typed accessor helper.

**Scope:** `pkg/pipeline/types.go`, `pkg/pipeline/saw_steps.go`.

---

### 13. `StepResult.Data` is `interface{}` with repetitive type assertions — `pkg/engine/finalize.go`

**Problem:** `StepResult.Data interface{}` forces the caller to type-assert the result at every use site. In `FinalizeWave`, there are 12+ type assertions of the form:
```go
if verifyData, ok := verifyBuildStepResult.Data.(protocol.VerifyBuildData); ok { ... }
if cleanupData, ok := cleanupStepResult.Data.(protocol.CleanupData); ok { ... }
```
The step functions (`StepVerifyBuild`, `StepCleanup`, etc.) already know the concrete type — the `interface{}` is an unnecessary indirection.

**Fix:** Make `StepResult` generic: `type StepResult[T any] struct { ... Data *T }`. Each step function returns its typed data. Callers no longer need type assertions.

**Scope:** `pkg/engine/step_types.go`, `pkg/engine/finalize_steps.go`, `pkg/engine/finalize.go` (~12 assertions removed).

---

### 14. `pkg/protocol` functions missing `ctx` that run shell commands — multiple files

**Problem:** Several protocol functions run `exec.Command` without a ctx, meaning they cannot be cancelled:

- `pkg/protocol/verify_build.go:127` — `runCommand` calls `exec.Command("sh", "-c", ...)` with no context.
- `pkg/protocol/gates.go` — gate execution likely similar (same `runCommand` pattern).
- `pkg/protocol/cleanup.go` — runs git commands via `internal/git` without forwarding caller ctx.

These are in protocol-layer functions that callers (engine, sawtools) invoke with real contexts that have deadlines and cancellations.

**Fix:** Add `ctx context.Context` to `runCommand`, `VerifyBuild`, and update all git call sites to use `exec.CommandContext(ctx, ...)`.

**Scope:** ~5 files in `pkg/protocol/`.

---

### 15. `fmt.Errorf` without `%w` (error chain broken) — 229 instances in `pkg/`

**Problem:** 229 `fmt.Errorf` calls in `pkg/` omit `%w` and instead format the error as a plain string with `%v` or `%s`. This breaks `errors.Is`/`errors.As` chains. The most impactful locations are in backend clients and protocol primitives where callers may want to inspect the underlying error type.

Notable examples:
- `pkg/agent/backend/cli/client.go:155,322` — `fmt.Errorf("cli backend: claude exited with code %d", exitErr.ExitCode())` — discards the `*exec.ExitError`.
- `pkg/retry/loop.go:152` — `fmt.Errorf("%s", saveRes.Errors[0].Message)` — discards the SAWError.
- `pkg/protocol/e35_detection.go:35,38` — plain string errors with no wrapping.
- `pkg/journal/observer.go:173,179,184` — all three error paths use `%s` instead of `%w`.

**Fix:**

- **Primary:** Add `errcheck` linter (with `-asserts` flag) to CI/pre-commit to enforce `%w` going forward. This stops the bleeding without a historical fixup pass.
- **Secondary:** Run the 229-site replacement as its own standalone IMPL with a single solo wave agent, AFTER all structural refactors are merged, to avoid merge conflicts across every file. Mechanical: replace `fmt.Errorf("...: %s", err.Error())` with `fmt.Errorf("...: %w", err)` and `fmt.Errorf("...: %v", err)` with `fmt.Errorf("...: %w", err)` throughout. Prioritize `pkg/protocol/`, `pkg/engine/`, `pkg/agent/backend/` first.

**Scope:** ~229 sites across all packages (secondary pass). Linter addition is 1 config file.

---

### 17. Legacy branch name fallback duplicated in 5 protocol files — `pkg/protocol/`

**Problem:** The dual-lookup pattern (try slug-scoped branch first, fall back to `LegacyBranchName`) appears independently in:
- `pkg/protocol/merge_agents.go:295–331` (two copies: `mergeAgentsSingleRepo` and `mergeAgentsMultiRepo`)
- `pkg/protocol/commit_verify.go:185–203`
- `pkg/protocol/worktree.go:157–172`
- `pkg/protocol/cleanup.go:177–202`
- `pkg/protocol/completion_validator.go:78`

Each implements its own try-slug/try-legacy logic rather than calling a shared `resolveAgentBranch(slug, wave, agent, repoDir)` helper.

**Fix:** Extract `func resolveAgentBranch(featureSlug, agentID string, waveNum int, repoDir string) (branchName string, isLegacy bool)` into `pkg/protocol/branchname.go` and use it everywhere.

**Scope:** 5 files, `pkg/protocol/branchname.go`.

---

### 18. `retry.Loop.Run` silently replaces nil `ctx` with `context.Background()` — `pkg/retry/loop.go:50`

**Problem:**
```go
if rl.cfg.Ctx == nil {
    ctx = context.Background()
}
```
Callers that forget to set `cfg.Ctx` get an uncancellable background context, silently losing deadline propagation from the parent. The field should be required or validated at construction time.

**Fix:** Either make `Ctx` required (return error from `NewLoop` if nil) or use `context.TODO()` with a prominent comment that the caller must pass a real context.

**Scope:** `pkg/retry/loop.go`, `pkg/retry/types.go`.

---

### 19. `Event.Data` typed as `interface{}` in the engine event bus — `pkg/engine/engine.go:44`

**Problem:** `Event.Data interface{}` is used for all engine events. The `publish` closures in `StartWave` and `startWaveWithGate` use `map[string]string{"error": msg}` and `map[string]interface{}{"wave": waveNum, ...}` inconsistently. Web callers that try to type-assert `Event.Data` will see different types for the same event name.

**Fix:** Define a typed union or per-event data structs (similar to how `orchestrator.OrchestratorEvent` already works). Replace `interface{}` with typed payloads.

**Scope:** `pkg/engine/engine.go`, `pkg/engine/runner.go`, `pkg/engine/runner_data_types.go`.

---

### 20. `pkg/orchestrator/setters.go` is a dead file — `pkg/orchestrator/setters.go`

**Problem:** The comment says "this setter is no longer needed. The file is retained for backward compatibility of the package structure." The file contains only a `SetLogger` method. "Package structure backward compatibility" is not a real constraint for an internal-only package. The file is misleading noise.

**Fix:** Move `SetLogger` to `orchestrator.go` and delete `setters.go`.

**Scope:** 2 files.

---

## Low (nice to have)

### 21. `pkg/protocol/program_status.go` swallows yaml errors silently

**Problem:** The yaml_io.go comment lists `pkg/protocol/program_status.go` as exempt because it "swallows errors; anonymous struct". Silent error swallowing in a status computation function means incorrect program status is surfaced as "unknown" with no diagnostic.

**Fix:** Log the error at least; ideally propagate it.

**Scope:** `pkg/protocol/program_status.go`.

---

### 22. `pkg/pipeline/State.Errors` uses `[]error` while the rest of the codebase uses `[]result.SAWError`

**Problem:** `State.Errors []error` means pipeline steps that emit SAWErrors must wrap them in `fmt.Errorf`, losing the structured Code/Severity/Context fields. When pipeline errors surface in a `result.Result`, the rich metadata is gone.

**Fix:** Change `State.Errors []error` to `State.Errors []result.SAWError` and update `pipeline.go` to accumulate typed errors.

**Scope:** `pkg/pipeline/types.go`, `pkg/pipeline/pipeline.go`, `pkg/pipeline/saw_steps.go`.

---

### 23. Duplicate `runCommand` / shell execution logic between `verify_build.go` and `gates.go`

**Problem:** `pkg/protocol/verify_build.go:126` defines `runCommand(command, repoDir string) (bool, string)` and `pkg/protocol/gates.go` has its own equivalent shell execution. The verify_build.go comment says "Follows the exact pattern from gates.go" — confirming the duplication is known.

**Fix:** Extract a shared `runShellCommand(ctx, command, repoDir) (bool, string)` into a `pkg/protocol/exec.go` internal helper.

**Scope:** 2 files.

---

### 24. `docs/error-codes.md` may be stale relative to `pkg/result/codes.go`

**Problem:** `pkg/result/codes.go` is the authoritative code catalog. `docs/error-codes.md` exists as a doc but may not be kept in sync (no automated check). If they diverge, users reading the docs get wrong information.

**Fix:** Either generate `docs/error-codes.md` from `codes.go` via a script, or add a CI check that compares them.

**Scope:** `docs/error-codes.md`, `pkg/result/codes.go`.

---

### 25. `pkg/notify` uses `map[string]interface{}` for Slack/Discord payloads

**Problem:** `pkg/notify/slack.go`, `pkg/notify/discord.go` build their API payloads as nested `map[string]interface{}` maps. These are untyped and bypass compile-time checking. Dedicated struct types with JSON tags (matching the Slack/Discord Block Kit schema) would catch field name typos at compile time.

**Fix:** Define typed structs for each notification format or use the official SDKs.

**Scope:** `pkg/notify/slack.go`, `pkg/notify/discord.go`.

---

## Scout IMPL Notes

The issues group into 6 IMPLs. IMPLs 2 and 3 are fully independent and run in parallel. IMPL 4 is gated on the ctx decision (Decision 1 above). IMPL 5 can partially overlap with IMPL 4 for non-ctx items. IMPL 6 runs last to avoid merge conflicts.

**IMPL 1: Crash fixes + legacy code string (immediate, solo wave or 2 agents) — ✅ COMPLETE (2026-03-31, commit 47af66c)**
- Issue 1: nil `ObsEmitter` panic in `runner.go:177`
- Issue 2: nil `ObsEmitter` panics in `program_tier_loop.go:246,280,318,335`
- Issue 4: hardcoded legacy error code string in `prepare.go:255`

Fully independent. Fix first before any other work. Single wave agent owns `pkg/engine/runner.go`, `pkg/engine/program_tier_loop.go`, `pkg/engine/prepare.go`.

---

**IMPL 2: Error code catalog (parallel with IMPL 3) — ✅ COMPLETE (2026-03-31)**
- Issue 6: Add `ENGINE_*` and `CONTEXT_CANCELLED` constants to `pkg/result/codes.go`, replace all inline strings in `pkg/engine/`

Independent of all other IMPLs. Single wave agent. Explicit file ownership:
- `pkg/result/codes.go` (add constants)
- `pkg/engine/chat.go`
- `pkg/engine/debug_journal.go`
- `pkg/engine/finalize.go`
- `pkg/engine/fix_build.go`
- `pkg/engine/integration_runner.go`
- `pkg/engine/mark_program_complete.go`
- `pkg/engine/merge_cleanup.go`
- `pkg/engine/prepare.go`
- `pkg/engine/program_progress.go`
- `pkg/engine/resolve_conflicts.go`
- `pkg/engine/run_wave_atomic.go`
- `pkg/engine/runner.go`
- `pkg/engine/scout_correction_loop.go`
- `pkg/engine/test_runner.go`

Note: IMPL 3 also owns `runner.go` and `finalize.go` for the StartWave merge. Run IMPL 2 and IMPL 3 in parallel — their changes to those files are to different code regions (error code strings vs. wave loop logic). Scout must declare this as a cascade patch contract and set `finalize-wave --skip-merge` if E11 prediction fires on the shared files.

---

**IMPL 3: Engine structural debt (parallel with IMPL 2) — ✅ COMPLETE (2026-03-31)**
- Issue 3 (Critical): Merge `StartWave`/`startWaveWithGate` via shared `runOneWave` helper
- Issue 9: Replace duplicate `implSlugFromPath`/`implSlugFromIMPLPath` with `protocol.ExtractIMPLSlug`
- Issue 10: Extract `firstRepoOpts` helper in `finalize.go`
- Issue 13: Type `StepResult.Data` generically

Independent of the ctx decision. One agent owns `runner.go`, `finalize.go`, `step_types.go`, `finalize_steps.go`.

**Interface contract for Issue 13 (StepResult generics):** Do NOT make `StepResult` a generic struct. `PrepareWaveResult.Steps []StepResult` requires a single concrete type — making it `StepResult[T]` would break the slice. Instead, add a typed second return value to each step function alongside `*StepResult`:

```go
// Before:
func StepVerifyBuild(ctx context.Context, opts StepVerifyBuildOpts, onEvent OnStepEvent) *StepResult

// After:
func StepVerifyBuild(ctx context.Context, opts StepVerifyBuildOpts, onEvent OnStepEvent) (*StepResult, *protocol.VerifyBuildData)
func StepCleanup(ctx context.Context, opts StepCleanupOpts, onEvent OnStepEvent) (*StepResult, *protocol.CleanupData)
// etc. for each step in finalize_steps.go
```

The `StepResult.Data interface{}` field and `PrepareWaveResult.Steps []StepResult` remain unchanged. Call sites in `finalize.go` switch from type assertions to direct typed returns. Agent must update all step function signatures in `finalize_steps.go` and all call sites in `finalize.go` atomically in a single commit.

---

**Architectural decision gate (human decision before IMPL 4)**
- Issue 11: ✅ Choice A — remove ctx from `LoadYAML`/`SaveYAML`, clean up ~40 call sites
- Issue 8: ✅ Choice A — post-hoc event stream enforcement in CLI backend (~3 packages)

---

**IMPL 4: Context propagation in `pkg/protocol/` — ✅ COMPLETE (2026-03-31)**
- Issues 5, 11, 14: Remove ctx from `LoadYAML`/`SaveYAML` (Issue 11, Choice A); add ctx to `SetImplState`, `UpdateStatus`, `VerifyBuild`, `MergeAgents`, `CreateWorktrees`, `ScanStubs`, `runCommand`
- Scope: ~40 call site cleanups for Issue 11 + ctx additions for Issues 5 and 14

Two agents with explicit file ownership:

**Agent A — `LoadYAML`/`SaveYAML` removal + all call sites (Issue 11)**

Remove `ctx context.Context` parameter from `LoadYAML` and `SaveYAML` in `yaml_io.go`. Update all call sites to call without a ctx argument.

Owned files:
- `pkg/protocol/yaml_io.go` (remove ctx param, update Load/Save wrappers)
- `pkg/protocol/duplicate_key_validator.go`
- `pkg/protocol/integration.go`
- `pkg/protocol/manifest.go`
- `pkg/protocol/marker.go`
- `pkg/protocol/memory.go`
- `pkg/protocol/program_generator.go`
- `pkg/protocol/program_parser.go`
- `pkg/protocol/program_status.go`
- `pkg/protocol/schema_unknown_keys.go`
- `pkg/protocol/solver_integration.go`
- `pkg/protocol/validation.go`
- `pkg/protocol/wiring_validation.go`
- `pkg/analyzer/output.go`
- `pkg/commands/github_actions.go`
- `pkg/engine/import_impls.go`
- `pkg/engine/program_progress.go`
- `pkg/interview/deterministic.go`
- `pkg/queue/manager.go`
- `cmd/sawtools/detect_cascades_cmd.go`
- `cmd/sawtools/diagnose_build_failure_cmd.go`
- `cmd/sawtools/extract_commands_cmd.go`
- `cmd/sawtools/update_program_impl_cmd.go`
- `cmd/sawtools/update_program_state_cmd.go`
- `cmd/sawtools/validate_integration.go`
- `cmd/sawtools/validate_scaffold_cmd.go`

Do NOT touch `runner.go`, `finalize.go` (IMPL 3 owns those), or any ctx-addition files (Agent B owns those).

**Agent B — Add `ctx context.Context` to core protocol functions (Issues 5 and 14)**

Add `ctx context.Context` as first parameter to functions that run git/shell commands without one.

Owned files:
- `pkg/protocol/state_transition.go` (add ctx to `SetImplState`)
- `pkg/protocol/status_update.go` (add ctx to `UpdateStatus`)
- `pkg/protocol/verify_build.go` (add ctx to `VerifyBuild`, `runCommand`)
- `pkg/protocol/merge_agents.go` (add ctx to `MergeAgents`)
- `pkg/protocol/worktree.go` (add ctx to `CreateWorktrees`)
- `pkg/protocol/stubs.go` (add ctx to `PersistStubReport`)
- `pkg/protocol/gate_populator.go` (add ctx to `PopulateGates`)
- Engine callers that invoke the above functions and already carry a ctx (search for call sites in `pkg/engine/` after Agent A completes, excluding `runner.go` and `finalize.go` which are IMPL 3 territory)

Do NOT touch `runner.go`, `finalize.go`, or `program_progress.go` (owned by IMPL 3 / Agent A).

---

**IMPL 5: Protocol + backend cleanup (can parallel with IMPL 4)**
- Issue 7: Move `executeTool` to shared backend package
- Issue 8 (High): CLI backend post-hoc constraint enforcement (Choice A — ~3 files in `pkg/agent/backend/cli/`)
- Issue 16 (Critical): Fix `ScaffoldFile` YAML/JSON tag mismatch + add `TestScaffoldFileRoundtrip`
- Issue 17: Extract `resolveAgentBranch` helper
- Issue 22: `State.Errors []result.SAWError`

All items are independent of the ctx decision. Two agents: (a) backend cleanup (`api/`, `bedrock/`, `openai/`, `cli/`); (b) protocol cleanup (`branchname.go`, `types.go`, pipeline).

---

**IMPL 6: %w sweep (standalone, after all structural work merged)**
- Issue 15 only. Single solo-wave agent. Run after IMPLs 1–5 are complete and merged to avoid merge conflicts across every file.
- Alternative: skip the historical fixup entirely and add `errcheck` linter to CI instead. Linter stops the bleeding going forward without touching 229 sites.
