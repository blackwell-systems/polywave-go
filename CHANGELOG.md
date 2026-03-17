# Changelog

All notable changes to the Scout-and-Wave Go engine will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/).

## Version History

| Version | Date | Headline |
|---------|------|----------|
| [0.57.0] | 2026-03-16 | Batch command integration — gate caching in `finalize-wave`, resume detection in `prepare-wave`, error classification in `retry` agent task |
| [0.56.0] | 2026-03-16 | Failure recovery UX — gate caching (`pkg/gatecache/`), classified retry context (`pkg/retryctx/`), resume detection (`pkg/resume/`), FixBuildFailure maxTurns 20→50 |
| [0.55.0] | 2026-03-16 | E27: Wave.Type field + validator support — `type: "integration"` on Wave struct, schema_unknown_keys accepts `type` for wave objects |
| [0.54.0] | 2026-03-16 | AI-powered build failure fixer — FixBuildFailure engine function uses SDK backend with full tool use (Read/Edit/Bash) to diagnose and fix test/gate failures post-merge, streams progress via OnOutput/OnToolCall callbacks |
| [0.53.0] | 2026-03-16 | Streaming conflict resolution — RunStreaming replaces blocking Run in resolve_conflicts, enables real-time SSE output during AI merge conflict resolution |
| [0.52.0] | 2026-03-16 | go.mod replace path enforcement — pre-commit hook blocks deep relative paths, post-merge auto-fixup in FinalizeWave |
| [0.51.0] | 2026-03-16 | Stale branch auto-cleanup + worktree lifecycle fixes — BranchExists/IsAncestor cleanup in CreateWorktrees, cleanup runs after merge regardless of build result, mark-complete in early-return path |
| [0.50.0] | 2026-03-16 | Worktree reuse + timeout failure type — rerun agents reuse existing worktrees, maxTurns emits failure_type "timeout" |
| [0.49.0] | 2026-03-16 | Integration Agent engine (E25/E26) — 4-wave implementation: validation engine, heuristics, CLI, runner, constraints, manifest types, engine wiring |
| [0.48.0] | 2026-03-15 | MR01 multi-repo consistency + list-impls state field — validator catches mixed repo: tags, list-impls adds state field, filters completed IMPLs by default (--include-complete to show) |
| [0.47.0] | 2026-03-15 | Workshop constraints — tool-level SAW protocol enforcement (I1/I2/I5/I6 middleware), H6 dep checker prefix matching fix, orchestrator wiring |
| [0.46.0] | 2026-03-14 | Validate --fix + .claire worktree resolution — auto-correct invalid gate types, shared worktree resolver for .claude/.claire fallback |
| [0.45.0] | 2026-03-14 | API agent parity — auto-commit, synthetic completion reports, mutex-serialized writes, no-op agent handling, wave-skip on re-run |
| [0.44.0] | 2025-03-14 | Cross-repo finalize-wave — all 6 pipeline steps run per-repo, aggregates results from multiple repositories |
| [0.43.0] | 2025-03-14 | InstallHooks template embedding — hook generated from code instead of copied from main repo, eliminates manual setup |
| [0.42.0] | 2025-03-14 | Multi-repo finalize-impl — gate_populator extracts H2 data from all repos in file_ownership, applies repo-specific gates to each agent |
| [0.41.0] | 2025-03-14 | H10 pre-commit hook verification — verify-hook-installed command, integrated into prepare-wave Step 1.5 |
| [0.40.0] | 2025-03-14 | E23A journal recovery + E9 merge idempotency — orchestrator journal integration, merge-log tracking, crash-resistant finalize-wave |
| [0.39.0] | 2026-03-14 | Scout automation integration — runScoutAutomation() integrates H1a-H4 tools into engine.RunScout(), inject results into Scout prompts |
| [0.38.0] | 2026-03-12 | H7 build failure diagnosis — pattern-matching engine with 27 error patterns across 4 languages (Go, Rust, JS/TS, Python) |
| [0.37.0] | 2026-03-12 | Batch wave commands — prepare-wave and finalize-wave reduce orchestrator overhead from 11 commands to 3 (23% faster execution) |
| [0.36.0] | 2026-03-12 | H6 dependency conflict detection + journal graceful degradation — check-deps CLI, 4 lock file parsers, empty session handling |
| [0.35.0] | 2026-03-12 | State machine conformance — fixed SCOUT_VALIDATING self-loop, removed direct SCOUT_PENDING→REVIEWED bypass, aligned validation guards |
| [0.34.0] | 2026-03-12 | Markdown system removal — deprecated markdown IMPL parsers removed, cross-repo wave prevention fixes added |
| [0.33.0] | 2026-03-11 | mark-complete simplification — always archives to complete/ directory, removed --archive flag |
| [0.32.0] | 2026-03-11 | Multi-language dependency analysis — Rust, JavaScript/TypeScript, Python parsers added to analyze-deps (H3 Phase 2 complete) |
| [0.31.0] | 2026-03-10 | Agent launch prioritization — critical path scheduling reduces wave completion time 10-20% |
| [0.30.0] | 2026-03-10 | Engine roadmap — verification loop (E24), agent launch prioritization, wave timeout enforcement, persistent memory system |
| [0.29.0] | 2026-03-10 | LLM orchestrator journal integration — journal-init and journal-context CLI commands for saw-skill.md |
| [0.28.0] | 2026-03-10 | Multi-repo wave support — merge-agents and verify-commits auto-detect cross-repo waves from file ownership table |
| [0.27.0] | 2026-03-10 | Tool Journaling Wave 1 — Core observer, context generator, checkpoint system, archive policy (4 agents, 53 tests) |
| [0.26.0] | 2026-03-10 | Scaffold agent ID validation — validator accepts "Scaffold" for wave 0 file ownership entries |
| [0.25.0] | 2026-03-10 | mark-complete preservation fix — text-based YAML editing preserves all 1600+ lines of IMPL doc structure instead of compacting to 50 lines |
| [0.24.0] | 2026-03-10 | Cross-repo worktree support — `create-worktrees` resolves agent repos from FileOwnership table, creates worktrees in correct sibling directories |
| [0.23.0] | 2026-03-10 | Hybrid IMPL doc support — `create-worktrees` parses markdown/YAML manifests via `ParseIMPLDoc()` instead of pure YAML `Load()` |
| [0.22.0] | 2026-03-10 | E5/E10 validator hardening — solo-wave exemption, lenient verification format, CLI reference docs |
| [0.21.0] | 2026-03-10 | Constraint solver — Kahn's topological sort, cycle detection, `SolveManifest`, `sawtools solve` command |
| [0.20.0] | 2026-03-10 | Tool middleware wired — TimingMiddleware feeds Observatory SSE, PermissionMiddleware gives Scout read-only tools, `ReadOnlyTools()` factory |
| [0.19.0] | 2026-03-10 | Tool system refactoring — unified `pkg/tools` Workshop wired into both Anthropic and OpenAI backends; 7 standard tools, 3 old duplicated tool files deleted |
| [0.18.0] | 2026-03-10 | E16A/E16C validator tests — 5 new tests for required block presence and out-of-band dep graph warning in `validator_test.go` |
| [0.17.0] | 2026-03-10 | Structured output parsing — JSON schema-constrained Scout output via Anthropic API `output_config.format`; eliminates brittle markdown parser path |
| [0.16.0] | 2026-03-10 | YAML-mode CLI commands — 9 missing commands: `validate`, `extract-context`, `set-completion`, `mark-complete`, `run-gates`, `check-conflicts`, `validate-scaffolds`, `freeze-check`, `update-agent-prompt` |
| [0.15.0] | 2026-03-09 | Binary rename — `sawtools` replaces `saw` as the protocol toolkit CLI name |
| [0.14.0] | 2026-03-09 | Protocol gap closures — `verify-isolation` command, `scan-stubs --append-impl`, `merge-agents` auto-status-update after successful merge |
| [0.13.0] | 2026-03-09 | Cobra CLI migration — all 10 subcommands converted from flag.FlagSet to cobra.Command; fixes arg-order bug in create-worktrees |
| [0.12.0] | 2026-03-09 | Protocol SDK conformance — 44-gap remediation: state machine, freeze enforcement, conflict detection, quality gates, failure routing, scaffold/enum validation, project memory |
| [0.11.0] | 2026-03-09 | AWS Bedrock backend — real AWS SDK integration with inference profile IDs, replaces fake Bedrock routing |
| [0.10.0] | 2026-03-09 | Model name validation — input sanitization to prevent command injection via provider-prefixed model names |
| [0.9.0] | 2026-03-09 | Agent Observatory — real-time tool call visibility per wave agent via SSE |
| [0.8.0] | 2026-03-09 | Content-mode tool call fallback — handles local models (e.g. Qwen via Ollama) that embed tool calls in response content instead of `tool_calls` array |
| [0.7.0] | 2026-03-09 | Local model shortcuts — `ollama:` and `lmstudio:` provider prefixes with hardcoded default base URLs |
| [0.6.0] | 2026-03-09 | OpenAI-compatible API backend + provider-prefix routing — `openai:gpt-4o`, `cli:kimi`, `anthropic:claude-*` prefix dispatch in `newBackendFunc` |
| [0.5.0] | 2026-03-09 | Configurable CLI binary — `BinaryPath` in `backend.Config` allows swapping `claude` for any compatible CLI |
| [0.4.0] | 2026-03-09 | Per-agent model routing — ScoutModel/WaveModel opts, `model:` field in IMPL doc agent sections, per-agent backend dispatch |
| [0.3.0] | 2026-03-08 | Protocol audit fixes — P0: failure_type parsing, multi-gen agent IDs; P1: E22 2-pass scaffold build, cross-repo Repo column; P2: repo field in completion reports |
| [0.2.0] | 2026-03-08 | Engine protocol parity — E17–E23 implemented (context memory, failure routing, stub scan, quality gates, scaffold build verify, per-agent context extraction) |

---

## [0.53.0] - 2026-03-16

### Changed

- **Streaming conflict resolution** — `resolve_conflicts.go` switched from blocking `b.Run()` to `b.RunStreaming()` with an `OnOutput` chunk callback. AI conflict resolution now streams model output in real-time via SSE instead of returning a single response after completion.
- **`ResolveConflictsOpts.OnOutput`** — New `func(chunk string)` callback field in `engine.go` for streaming text chunks during conflict resolution.

---

## [0.50.0] - 2026-03-16

### Fixed

- **Worktree reuse for agent reruns** — `launchAgent` in `pkg/orchestrator/orchestrator.go` now checks `os.Stat(wtPath)` before creating worktrees. Existing worktrees are reused instead of failing with "branch already exists" error. Enables rerunning individual failed agents after server restart.
- **maxTurns failure type** — Agents exceeding the turn limit now emit `failure_type: "timeout"` instead of generic `"execute"`, enabling proper E19 failure routing (timeout → retry automatically vs execute → escalate).

---

## [0.49.0] - 2026-03-16

### Added

- **E25 integration validation engine** — `pkg/protocol/integration.go` with `ValidateIntegration()` that performs AST-based scanning to detect unexported symbols, unregistered routes, missing wire-up, and unused scaffold types across Go source files.
- **Integration heuristics** — `pkg/protocol/integration_heuristics.go` with `ClassifyExport()`, `IsIntegrationRequired()`, `SuggestCallers()` for intelligent classification of exports by action prefix/suffix patterns and context-aware caller suggestion.
- **`sawtools check-integration` CLI** — `cmd/saw/check_integration.go` surfaces E25 validation results as structured JSON for orchestrator consumption.
- **Integration agent runner** — `pkg/engine/runner.go` wired with `IntegrationModel` support; both E26 call sites use dedicated model with fallback to `WaveModel`.
- **`IntegrationModel` in engine opts** — `pkg/engine/engine.go` `RunWaveOpts.IntegrationModel` field for per-role model configuration.
- **Integration constraint types** — `pkg/protocol/manifest_types.go` extended with integration-specific constraint and validation result types.

### Fixed

- **Bedrock `maxTurns` default** — Changed from 50 to 200 to match Anthropic API behavior; prevents premature agent termination on complex tasks.

---

## [0.46.0] - 2026-03-14

### Added

- **`sawtools validate --fix`** — auto-corrects fixable validation errors before reporting. Currently fixes invalid gate types (e.g. `build-vite` → `custom`). Output JSON includes `"fixed": N` count. Eliminates retry cycles for mechanically correctable Scout output.
- **`ValidGateTypes` exported map** — shared gate type allowlist used by both validator and fixer.
- **`FixGateTypes(m)`** — rewrites unrecognized quality gate types to `"custom"`, returns count of corrections.
- **`.claire` worktree resolution** — `ResolveWorktreePath()` and `IsWorktreePath()` in `pkg/protocol/worktree_resolve.go`. Checks `.claude` first, falls back to `.claire`, defaults to `.claude` if neither exists. Defense-in-depth for Claude's occasional `.claire` worktree creation bug.

### Changed

- **Isolation check** uses `IsWorktreePath()` instead of hardcoded `.claude/worktrees/` string.
- **Cleanup** uses `ResolveWorktreePath()` to find worktrees in either `.claude` or `.claire`.
- **Merge** uses `ResolveWorktreePath()` for worktree path resolution.

---

## [0.45.0] - 2026-03-14

### Added

- **Auto-commit for API agents** — `autoCommitWorktree()` in orchestrator detects uncommitted changes after agent execution, stages with `git add -A`, and commits with `--no-verify`. Bridges the gap between CLI agents (SAW-protocol-aware, self-commit) and API/Bedrock agents (vanilla Claude, write files but never commit).
- **Synthetic completion reports** — When an API agent doesn't write a completion report, the orchestrator synthesizes one from the auto-commit results (SHA, branch, files changed) and writes it to the IMPL doc. The merge pipeline sees identical data regardless of agent backend.
- **No-op agent handling** — Agents that produce no file changes (e.g. routes already registered) get a completion report with `notes: "no changes produced"`. Merge pipeline skips diff check and merge for these agents, but still cleans up worktrees.
- **Git helpers** — `StatusPorcelain()`, `AddAll()`, `Commit()`, `ChangedFilesSinceRef()` added to `internal/git/commands.go`

### Fixed

- **Completion report race condition** — Parallel agents in the same wave could clobber each other's reports (concurrent Load→Set→Save on same IMPL doc file). Added `sync.Mutex` (`reportMu`) to serialize writes.
- **Branch name mismatch** — Synthetic reports used `saw/wave{N}-agent-{ID}` prefix but worktree manager creates `wave{N}-agent-{ID}`. Fixed to match worktree convention.
- **verifyAgentCommits false positive** — No-op agents with zero files changed were flagged as isolation failures. Now skips diff check when `FilesChanged` and `FilesCreated` are both empty.

### Files

- `internal/git/commands.go` — 4 new functions
- `pkg/orchestrator/orchestrator.go` — `autoCommitWorktree()`, `reportMu`, updated `launchAgent` step (d)
- `pkg/orchestrator/merge.go` — no-op agent skip in `verifyAgentCommits` and `executeMergeWave`

---

## [0.44.0] - 2025-03-14

### Added

- **Cross-Repo finalize-wave** — Automated wave finalization for multi-repo IMPLs
  - `cmd/saw/finalize_wave.go`: Enhanced to run all 6 steps per-repo
    - `FinalizeWaveResult`: Changed from single values to maps (repo → result)
    - `extractReposFromManifest()`: Extracts unique repos from file_ownership, resolves relative paths
    - Step 1 (VerifyCommits): Runs per-repo, checks all agents in that repo have commits
    - Step 2 (ScanStubs): Aggregated across all repos (stubs are file-level, not repo-specific)
    - Step 3 (RunGates): Runs per-repo with repo-specific quality gates
    - Step 4 (MergeAgents): Runs per-repo, merges agent branches in their home repo
    - Step 5 (VerifyBuild): Runs per-repo, tests post-merge code in each repo
    - Step 6 (Cleanup): Runs per-repo, removes worktrees and branches from each repo
  - Backward compatible: Single-repo IMPLs work identically (one entry in result maps)
  - Eliminates manual per-repo orchestration (previous workaround: separate merge commands per repo)
  - Tested: IMPL-remove-markdown-impl-support (scout-and-wave-web + scout-and-wave-go)

### Implementation

- Direct implementation (no SAW wave execution)
- Files modified: 1 (`finalize_wave.go`)
- Lines changed: +156, -78 (total 234 lines modified)
- Completes cross-repo automation trilogy: prepare-wave (v0.37.0), finalize-impl (v0.42.0), finalize-wave (v0.44.0)

---

## [0.43.0] - 2025-03-14

### Fixed

- **InstallHooks dependency on main repo hook** — Hook is now embedded as template in code
  - `internal/git/commands.go`: Added `preCommitHookTemplate` constant (31 lines)
    - Worktree isolation logic: blocks commits to main/master unless `SAW_ALLOW_MAIN_COMMIT=1`
    - Contains required markers for verify-hook-installed validation
  - `InstallHooks()`: Removed `os.ReadFile(sourceHookPath)` — generates hook from template instead
  - Root cause: Previous implementation copied hook from main repo `.git/hooks/pre-commit`, which may not exist or may contain wrong hook (I6 Scout boundaries instead of worktree isolation)
  - Impact: prepare-wave now works without manual hook setup in target repos
  - Tested: Cross-repo IMPL (scout-and-wave-web + scout-and-wave-go) worktree creation succeeded

### Implementation

- Direct fix (no SAW wave execution)
- Files modified: 1 (`internal/git/commands.go`)
- Lines added: 35, Lines removed: 11
- Eliminates prepare-wave failure mode: "hook verification failed: hook does not contain SAW isolation logic"

---

## [0.42.0] - 2025-03-14

### Added

- **Multi-Repo finalize-impl** — Automatic verification gate population for cross-repo IMPLs
  - `pkg/protocol/gate_populator.go`: Enhanced `PopulateVerificationGates()` to accept map of repo paths to command sets
    - `buildAgentRepoMap()`: Creates agent ID → repo path mapping from file_ownership table
    - `extractUniqueRepos()`: Extracts all unique repos from file_ownership, resolves relative paths to absolute
    - Resolves relative repo names (e.g., `scout-and-wave-web`) to absolute paths by checking sibling directories
    - Maps each agent to its repo using file_ownership, applies repo-specific H2 data (build/test/lint commands)
  - `pkg/protocol/gate_populator.go`: Updated `FinalizeIMPL()` to extract H2 data from all repos
    - Builds repo mapping: relative names → absolute paths for resolution
    - Extracts command sets from each unique repo (parallel H2 extraction)
    - Skips repos without valid toolchains (agents may have manually-specified gates)
    - Reports comma-separated toolchains in JSON output for multi-repo cases
  - `cmd/saw/finalize_impl_cmd.go`: Updated documentation to explain multi-repo usage
    - Single-repo: `sawtools finalize-impl docs/IMPL/IMPL-*.yaml --repo-root /path/to/repo`
    - Multi-repo: Specify `repo:` field in file_ownership, finalize-impl auto-detects and extracts from each
  - Enables automated gate population for cross-repo refactors, library+consumer updates, monorepo work
  - Zero breaking changes - single-repo IMPLs work identically (backward compatible)

### Implementation

- Direct implementation (no SAW wave execution)
- Files modified: 2 (`gate_populator.go`, `finalize_impl_cmd.go`)
- Agents updated: 0 (no verification gates existed before, all agents get gates now)
- Multi-repo test case: IMPL-remove-markdown-impl-support (Agent A in scout-and-wave-web, Agent B in scout-and-wave-go)

---

## [0.41.0] - 2025-03-14

### Added

- **H10: Pre-Commit Hook Verification** — Layer 0 isolation enforcement catches silent hook removal
  - `cmd/saw/verify_hook_installed.go`: New command `sawtools verify-hook-installed <worktree-path>`
    - Verifies hook file exists in worktree git directory (handles both regular repos and worktrees)
    - Checks hook is executable (mode & 0111)
    - Validates hook contains SAW isolation logic markers (`SAW_ALLOW_MAIN_COMMIT` or `SAW pre-commit guard`)
    - Returns JSON with `valid`, `reason`, `hook_path`, `executable`, `has_logic` fields
    - Exit code 0 if valid, 1 if missing/broken
  - `cmd/saw/prepare_wave.go`: Integrated into Step 1.5 (after worktree creation, before agent launch)
    - Verifies all hooks after `protocol.CreateWorktrees()` returns
    - Blocks wave execution if any hook is missing or invalid
    - Clear error message with fix instructions: "hook verification failed for agent X: pre-commit hook file does not exist"
  - Prevents agents from launching in worktrees with broken isolation (Layer 0 protection lost)
  - Completes H10 from protocol enhancement roadmap (Priority 3)

### Implementation

- Direct implementation (no SAW wave execution)
- Files created: 1 (`verify_hook_installed.go`)
- Files modified: 2 (`main.go`, `prepare_wave.go`)
- Zero test overhead - verification happens once per wave during prepare-wave
- Transparent to agents and orchestrator (no prompt updates needed)

---

## [0.40.0] - 2025-03-14

### Added

- **E23A: Tool Journal Recovery** — Agents can recover execution history across retries, context compaction, and process restarts
  - `pkg/orchestrator/journal_integration.go`: New module with 2 functions
    - `PrepareAgentContext()`: Loads journal history and generates context.md for agent recovery (defaults to last 50 entries per E23A spec)
    - `WriteJournalEntry()`: Appends tool use/result entries to `.saw-state/journals/wave{N}/agent-{ID}/index.jsonl`
  - `pkg/journal/observer.go`: Completed journal recovery implementation
    - `LoadJournal()`: Reads all entries from index.jsonl with JSONL parsing, error handling for invalid lines
    - `GenerateContext()`: Delegates to `journal.GenerateContext()` to produce markdown context summary
  - Enables automatic failure remediation (E7a, E19) by preserving agent execution history
  - 11 new tests (6 in orchestrator, 5 in journal observer)
  - Completed by: Agent A (orchestrator integration), Agent B (observer wiring)

- **E9: Merge Idempotency** — finalize-wave is now crash-resistant with merge-log tracking
  - `pkg/protocol/merge_log.go`: New module with MergeLog persistence (117 lines)
    - `MergeLog` type with Wave and Merges[] fields, persists to `.saw-state/wave{N}/merge-log.json`
    - `LoadMergeLog()`: Reads merge-log.json, returns empty log if file doesn't exist (first merge attempt)
    - `SaveMergeLog()`: Writes merge-log.json after successful agent merge, creates directories as needed
    - `IsMerged()`: Case-insensitive agent merge status check
    - `GetMergeSHA()`: Returns merge commit SHA for agent
  - Idempotency checks integrated into merge pipeline:
    - `cmd/saw/finalize_wave.go`: Updated comment to document E9 idempotency
    - `pkg/protocol/merge_agents.go`: Load merge-log before merging, skip already-merged agents, save after each merge
    - `pkg/orchestrator/merge.go`: Check merge-log in executeMergeWave(), record merge entries with SHA
  - Crashed merges can resume without duplicate commits (running finalize-wave twice produces identical git history)
  - 12 new tests (9 in merge_log, 3 in merge_agents) including TestMergeAgents_IdempotentOnCrash
  - Completed by: Agent C (merge-log persistence), Agent D (finalize-wave integration)

### Fixed

- **Go stdlib false positives in dependency checker** — `sawtools check-deps` no longer reports stdlib packages as missing
  - `pkg/deps/checker.go`: Added `isStdLib()` helper that identifies stdlib packages by checking if first path component contains a dot
  - Stdlib packages like `fmt`, `os`, `encoding/json` have no dots before first slash, third-party packages like `github.com/...` do
  - Fixes prepare-wave blocking on 30+ false positive "missing dependencies" (all stdlib packages)
  - Commit: 7942790

- **Test compilation error after merge** — Fixed `time.Time` zero value usage in merge_agents_test.go
  - Added missing `time` import
  - Changed `Timestamp: nil` to `Timestamp: time.Time{}` (structs can't be nil)
  - Commit: 9af9ba9

- **Test logic bug in TestMergeAgents_IdempotentOnCrash** — Test now correctly simulates crashed merge
  - Previous version merged all 3 agents in first call, then expected C to not be skipped in second call
  - Fixed to manually merge A+B and record in merge-log (simulating crash before C merged), then verify MergeAgents skips A+B and merges C
  - Commit: 9af9ba9

### Implementation

- **Combined IMPL** — E23A and E9 implemented together (disjoint file ownership, fully parallel execution)
- **Wave structure**: Single wave, 4 agents (A: orchestrator integration, B: observer wiring, C: merge-log persistence, D: finalize-wave integration)
- **Files created**: 6 (journal_integration.go, journal_integration_test.go, merge_log.go, merge_log_test.go, 2 test files modified)
- **Files modified**: 4 (observer.go, observer_test.go, finalize_wave.go, merge_agents.go, merge.go, merge_agents_test.go, checker.go)
- **Tests added**: 28 total (11 E23A, 12 E9, 5 auxiliary)
- **All tests passing**: Build ✅, Tests ✅, Lint ✅

---

## [0.39.0] - 2026-03-14

### Added

- **Scout automation integration** — Scout agents now receive pre-execution automation analysis before launching
  - `pkg/engine/runner.go`: New `runScoutAutomation()` function orchestrates H2, H1a, H3 tool execution
  - H2 (extract-commands): Detects build/test/lint commands from CI configs/manifests
  - H1a (analyze-suitability): Conditional requirements file analysis (triggers when .md/.txt path detected)
  - H3 (analyze-deps): Dependency graph analysis using targetFiles from H1a or full repo scan
  - Results injected as "Automation Analysis Results" markdown section in Scout prompt
  - Best-effort execution model: tool failures logged but don't block Scout launch
  - `pkg/suitability/wrapper.go`: Engine-compatible wrappers for suitability analysis
    - `AnalyzeSuitability()`: Wrapper that calls internal `ScanPreImplementation()`
    - `ParseRequirements()`: Markdown requirements parser supporting audit.md format
  - `pkg/engine/runner_automation_test.go`: 4 comprehensive tests
    - `TestRunScout_AutomationIntegration`: Verifies automation results in Scout prompt
    - `TestRunScout_AutomationFailure`: Verifies Scout launches despite tool failures
    - `TestDetectRequirementsFile`: Tests requirements file path detection heuristic
    - `TestRunScoutAutomation_WithRequirementsFile`: Tests H1a conditional execution

### Changed

- **Scout prompt structure** — Automation results section now inserted after feature description, before scout.md contents
- **Requirements detection** — Heuristic based on file extensions (.md, .txt) in feature description string

### Fixed

- **Test failures after Wave 1 merge** — Empty grouping array handling in idgen package
  - `pkg/idgen/generator.go`: Removed `len(grouping) > 0` check in validation
  - `cmd/saw/assign_agent_ids_cmd.go`: CLI converts `--grouping "[]"` to nil (sequential mode)
  - Fixed `TestAssignAgentIDs_EmptyGrouping` expectations

### Implementation

- **Wave 2 Agent C** — SDK automation integration
- **Files modified**: `pkg/engine/runner.go`
- **Files created**: `pkg/engine/runner_automation_test.go`, `pkg/suitability/wrapper.go`
- **Tests added**: 4 (all passing)

## [0.38.0] - 2026-03-12

### Added

- **H7: Build failure diagnosis** — Phase 3 determinism tool that turns cryptic build errors into actionable fixes
  - `pkg/builddiag/` package with pattern-matching diagnosis engine
  - `ErrorPattern` struct (name, regex, fix, rationale, auto_fixable, confidence)
  - `DiagnoseError(errorLog, language)` function matches error logs against language-specific catalogs
  - 4 language catalogs with 27 total patterns:
    - Go: 6 patterns (missing_package, missing_go_sum_entry, undefined_identifier, type_mismatch, import_cycle, syntax_error)
    - Rust: 5 patterns (E0425, E0277, E0308, E0432, macro errors)
    - JavaScript/TypeScript: 5 patterns (module_not_found, property_not_exist, syntax_error, type_any_implicit, import_path_invalid)
    - Python: 6 patterns (ModuleNotFoundError, NameError, SyntaxError, ImportError, IndentationError, TypeError)
  - `sawtools diagnose-build-failure <error-log> --language <lang>` CLI command
  - Outputs structured YAML with diagnosis, confidence, fix recommendation, rationale, and auto-fixability
  - 42 tests passing across all agents (7 core engine, 35 language-specific)
  - Completed via SAW: 3 waves, 6 agents (A solo, B-E parallel, F solo)

### Fixed

- **Test isolation bug** in `pkg/builddiag/diagnose_test.go`
  - Tests were clearing global `catalogs` map without restoring it, causing failures when language pattern `init()` functions ran before tests
  - Added save/restore pattern using defer to all 7 test functions
  - Pattern: `originalCatalogs := catalogs; defer func() { catalogs = originalCatalogs }()`
  - All 42 tests now pass in full suite (previously only passed in isolation)

## [0.37.0] - 2026-03-12

### Added

- **Batch wave commands** — Two new commands reduce orchestrator overhead from 11 commands per wave to 3
  - `sawtools prepare-wave` — Combines `create-worktrees` + N×`prepare-agent` into single atomic operation
    - Creates worktrees for all agents in wave
    - Extracts agent briefs to `.saw-agent-brief.md` in each worktree
    - Initializes journal observers for all agents
    - Returns JSON with worktree paths, brief paths, and agent metadata
    - 200 lines in `cmd/saw/prepare_wave.go`
  - `sawtools finalize-wave` — Combines 6-step post-wave pipeline into single atomic operation
    - Executes: verify-commits → scan-stubs → run-gates → merge-agents → verify-build → cleanup
    - Stops on first failure (no partial merges)
    - Returns comprehensive JSON with all verification results
    - Exit code 1 if `Success: false`
    - 147 lines in `cmd/saw/finalize_wave.go`
  - Benefits: 23% faster wave execution (~7 min savings), atomic operations with stop-on-failure semantics
  - Completed via SAW: 2 waves, 2 agents parallel (A+B in Wave 1), Wave 2 registration done by agents during Wave 1

### Changed

- **Orchestrator prompt updated** — `/saw` skill (saw-skill.md) now uses batch commands instead of 11-command flow
  - Step 3: `prepare-wave` replaces `create-worktrees` + loop over `prepare-agent`
  - Step 7: `finalize-wave` replaces `verify-commits` + `scan-stubs` + `run-gates` + `merge-agents` + `verify-build` + `cleanup`
  - Net reduction: 11 lines (38% reduction in wave execution section)
  - Mental model simplified: orchestrator tracks 3 atomic phases instead of 11 distinct operations

### Fixed

- **Test compilation failures** resolved in pkg/protocol and pkg/orchestrator
  - Renamed test helper functions to avoid collision with production code (`testContains`/`testContainsMiddle` in `validation_test.go`)
  - Fixed type mismatch in orchestrator test mocks (`*protocol.CompletionReport` instead of `*types.CompletionReport`)
  - Synchronized state transition tables across `protocol/manifest.go` and `orchestrator/transitions.go`
    - Added SCOUT_PENDING→REVIEWED shortcut
    - Added WAVE_EXECUTING→WAVE_MERGING→WAVE_VERIFIED path
    - Fixed test cases to match protocol state machine (SM-02)
  - All tests now passing: pkg/protocol (4.8s), pkg/orchestrator (1.0s), cmd/saw (0.4s)

## [0.36.0] - 2026-03-12

### Added

- **H6: Dependency conflict detection** — Phase 3 determinism tool for pre-wave dependency scanning
  - `pkg/deps/` package with `CheckDeps()` function and `LockFileParser` interface
  - 4 lock file parsers: `GoSumParser` (go.sum), `PackageLockParser` (package-lock.json), `CargoLockParser` (Cargo.lock), `PoetryLockParser` (poetry.lock)
  - `sawtools check-deps` CLI command with JSON output
  - Detects missing dependencies (imports not in lock files) and version conflicts (agents requiring different versions)
  - 46 tests passing (8 in checker, 6+8+7+6 in parsers, 5 in CLI)
  - Completed via SAW: 3 waves, 6 agents (A solo, B-E parallel, F solo)

### Fixed

- **Journal graceful degradation** — `journal-context` no longer fails with noisy errors when no session files exist (fresh sessions with no prior tool execution)
  - `observer.go`: `findLatestSessionFile()` returns empty string (not error) when no session files found
  - `observer.go`: `Sync()` returns empty `SyncResult` when `sessionFile == ""`
  - `debug_journal.go`: `loadJournalEntries()` returns empty slice when `index.jsonl` doesn't exist
  - Generated `context.md` now shows clear message: "No tool activity recorded yet."
- **Test fix** — `analyze-suitability` validates repo root exists before calling `ScanPreImplementation`, fixing `TestAnalyzeSuitabilityCmd_InvalidRepoRoot`


### Fixed

- **State machine conformance** — Fixed two critical state transition violations identified in AUDIT-state-machine.md:
  1. Added SCOUT_VALIDATING self-loop transition (enables E16 validation retry loop)
  2. Removed direct SCOUT_PENDING → REVIEWED bypass (forces validation through SCOUT_VALIDATING)
- **Aligned transition guards** — Synchronized `pkg/orchestrator/transitions.go` and `pkg/protocol/manifest.go` state machines to enforce consistent rules

### Changed

- **State transitions** — Scout validation now requires explicit SCOUT_VALIDATING state; cannot skip validation gate
- **Validation retry** — SCOUT_VALIDATING → SCOUT_VALIDATING self-loop allows retrying validation up to retry limit before blocking

## [0.34.0] - 2026-03-12

### Changed

- **`sawtools mark-complete` always archives** — Removed `--archive` flag. Command now always moves completed IMPL docs from `docs/IMPL/` to `docs/IMPL/complete/`. There is no use case for marking complete without archiving, so the optional flag created unnecessary complexity.

### Rationale

If an IMPL is complete, it should be archived. If it's not ready to archive, it's not actually complete. The flag created a half-state that the protocol doesn't need. Simplified both code (7 fewer lines, no conditional logic) and user mental model (one command, one outcome).

---

## [0.32.0] - 2026-03-11

### Added

- **Multi-language dependency analysis (H3 Phase 2)** — `sawtools analyze-deps` now supports Rust, JavaScript/TypeScript, and Python in addition to Go. Coverage expanded from ~40% to ~90% of SAW projects.
- **Rust parser** (`pkg/analyzer/rust.go`) — Parses Rust source files via external `rust-parser` helper binary. Extracts `use` statements, filters stdlib imports (std::, core::, alloc::), resolves local crate imports (crate::, super::, self::) to absolute file paths. Tests gracefully skip when rust-parser binary unavailable.
- **JavaScript/TypeScript parser** (`pkg/analyzer/javascript.go`) — Parses JS/TS files via external `js-parser.js` Node.js script. Handles ES6 imports, CommonJS require(), and TypeScript imports. Filters npm packages, resolves relative imports (./, ../) to absolute file paths. Supports .js, .jsx, .ts, .tsx, .mjs, .cjs extensions.
- **Python parser** (`pkg/analyzer/python.go`) — Parses Python files via external `python-parser.py` script. Handles `import` and `from X import Y` statements. Filters stdlib modules (40+ hardcoded), resolves relative imports (., .., .module) and absolute imports to file paths.
- **Language auto-detection** (`pkg/analyzer/graph.go:detectLanguage()`) — Analyzes file extensions to determine project language, routes to appropriate parser. Returns error for unsupported extensions or mixed-language projects.
- **Refactored Go parser** (`pkg/analyzer/graph.go:parseGoFiles()`) — Extracted existing BuildGraph Step 1 logic into standalone function for consistency with other language parsers.
- **Test fixtures** (`pkg/analyzer/testdata/{rust,javascript,python}/`) — Language-specific test scenarios: simple files, import patterns, stdlib filtering, relative/absolute resolution.

### Changed

- **BuildGraph language dispatch** — Modified `BuildGraph()` to detect language first (Step 0), then dispatch to language-specific parser (parseGoFiles, parseRustFiles, parseJavaScriptFiles, parsePythonFiles) before graph building (Steps 2-7 unchanged).

### Fixed

- **Name collision resolution** — Renamed `fileExists()` to `containsFile()` in `rust.go` to resolve conflict with `javascript.go`'s filesystem-checking `fileExists()` function.

### Testing

- **68 tests total** (42 from Phase 1 + 26 new in Phase 2)
  - 6 Rust parser tests (simple, with imports, stdlib filtering, binary missing, local import detection, resolve import)
  - 6 JavaScript parser tests (ES6, CommonJS, TypeScript, binary missing, multiple files, resolve import)
  - 8 Python parser tests (simple, absolute imports, relative imports, binary missing, parser script missing, stdlib detection, resolve relative, resolve absolute)
  - 7 integration tests (detectLanguage for Go/Rust/JS/Python, mixed error, unsupported error, multi-language BuildGraph)

### Documentation

- **IMPL doc** — `docs/IMPL/IMPL-h3-phase2-multi-language.yaml` marked SAW:COMPLETE
- **Implementation time** — <15 minutes (Wave 1: ~8 min parallel, Wave 2: ~3 min solo) vs. 30-45 hour roadmap estimate = 180x faster
- **Helper binary approach** — External language-specific parsers (rust-parser, js-parser.js, python-parser.py) exec'd from Go, output JSON. Tests gracefully skip when helpers unavailable, documented as Phase 2 limitation.

## [0.29.0] - 2026-03-10

### Added

- **LLM orchestrator journal integration** — `sawtools journal-init` and `sawtools journal-context` CLI commands enable saw-skill.md (LLM orchestrator) to use tool journaling for agent recovery after context compaction. These commands provide the same functionality that the Go orchestrator (web app) gets automatically via `pkg/engine` integration.
- **`sawtools journal-init`** (`cmd/saw/journal_init.go`) — Creates journal directory structure (`.saw-state/journals/wave<N>/agent-<ID>/`) and initializes cursor file. Called by LLM orchestrator before launching wave agents. Output: JSON with status, journal_dir, cursor_path, index_path, results_dir.
- **`sawtools journal-context`** (`cmd/saw/journal_context.go`) — Syncs journal from Claude Code session logs, loads entries from index.jsonl, generates context.md markdown summary via `journal.GenerateContext()`. Output: JSON with sync stats, context file path, length, availability. Supports `--max-entries` flag to limit context size.
- **Dual orchestrator support** — Journal recovery now works in both execution modes: (1) Go orchestrator (web app) uses automatic `JournalIntegration` in runner.go, (2) LLM orchestrator (saw-skill.md) calls `journal-init` + `journal-context` before/during wave execution.

### Changed

- **Agent J's saw-skill.md changes restored** — Originally reverted because commands didn't exist. After implementing the CLI commands, restored Agent J's orchestrator instructions for journal integration (step 4: init/context, step 5: prepend context to prompts).

---

## [0.34.0] - 2026-03-12

### Removed

- **Markdown IMPL parsing system deprecated** — Complete removal of markdown-based IMPL doc parsing in favor of YAML-only manifests. Protocol v0.7.0+ mandates YAML format; markdown support was a migration shim that is no longer needed.
- **`protocol.ParseIMPLDoc()` function removed** (1,184 lines) — Full markdown IMPL doc parser eliminated from `pkg/protocol/parser.go`. Replaced by YAML-only `protocol.Load()` function.
- **`protocol.ParseCompletionReport()` removed** — Markdown completion report parser eliminated. Completion reports now read from manifest's `completion_reports` YAML map.
- **`protocol.ExtractAgentContext()` and `FormatAgentContextPayload()` removed** — Markdown-specific context extraction helpers eliminated from `pkg/protocol/extract.go`. Replaced by `ExtractAgentContextFromManifest()` for YAML manifests.
- **`protocol.writeCompletionMarkerMarkdown()` removed** — Markdown completion marker writer eliminated from `pkg/protocol/marker.go`. Only YAML path remains.
- **15 markdown parsing helper functions removed** — `isAgentHeader`, `extractAgentLetter`, `parseFileOwnershipRow`, `readUntilClosingFence`, `extractTypedBlock`, `parsePreMortemSection`, `parseKnownIssuesSection`, `parseScaffoldsDetailSection`, `parseInterfaceContractsSection`, `parseDependencyGraphSection`, `parsePostMergeChecklistSection`, `parseWaveStructureBlock`, `classifyAction` all eliminated.
- **Markdown test coverage removed** (510 lines) — `TestParseIMPLDocMinimal`, `TestParseCompletionReportNotFound`, `TestExtractAgentContextFound`, `TestExtractAgentContextNotFound`, `TestFormatAgentContextPayload`, `TestWriteCompletionMarker_Markdown*` all deleted from test files.
- **Web API markdown handlers removed** (scout-and-wave-web) — Dual-format branching removed from `handleListImpls`, `handleGetImpl`, `handleDeleteImpl`, `handleArchiveImpl`. All API endpoints now use `protocol.Load()` exclusively.
- **Migration tool deleted** — `cmd/saw/migrate.go` (206 lines) removed from scout-and-wave-web as markdown-to-YAML migration is complete.

### Added

- **Base commit tracking for post-merge verification** (Prevention fix #1) — `Wave.BaseCommit` field records HEAD when worktrees are created. `CreateWorktrees()` saves this to manifest; `VerifyCommits()` uses it instead of current HEAD. Prevents false "0 commits" reports after branches are already merged.
- **Duplicate completion report detection** (Prevention fix #2) — `protocol.Load()` enhanced to detect duplicate YAML keys (agents appending instead of replacing reports) with error message "completion report written twice". Validates all completion report keys match existing agents. Prevents YAML parse failures from duplicate keys.
- **Compatibility shims in pkg/engine** — `ParseIMPLDoc()` and `ParseCompletionReport()` functions re-implemented as converters from `protocol.IMPLManifest` to legacy `types.IMPLDoc` format, maintaining backward compatibility for scout-and-wave-web's `cmd/saw/commands.go` callers.

### Changed

- **`pkg/orchestrator` migrated to YAML-only** — `merge.go` and `orchestrator.go` now use `protocol.Load()` and convert completion reports from protocol types to engine types. No longer call removed markdown parsers.
- **`pkg/agent/completion.go`** — `WaitForCompletion()` updated to read from `manifest.CompletionReports` map instead of calling removed `ParseCompletionReport()`.
- **`pkg/agent/runner.go`** — Removed `ParseCompletionReport()` wrapper method and unused protocol import.
- **Cross-repo wave file ownership** — IMPL manifests now use absolute paths (`/Users/.../scout-and-wave-go`) instead of relative paths (`scout-and-wave-go`) in `FileOwnership.Repo` field to fix worktree creation and verification across repositories.

### Fixed

- **Cross-repo verify-commits bug** — Fixed issue where `verify-commits` reported 0 commits for all agents after merge because it compared `HEAD..branchName` when branches were already merged into HEAD. Now uses recorded `Wave.BaseCommit` from worktree creation time.
- **Agent completion report idempotency** — Agents writing completion reports twice (appending to YAML) now caught immediately with clear error instead of silent YAML parse failure.

### Metrics

- **Total lines removed**: ~2,500 across both scout-and-wave-go and scout-and-wave-web repositories
- **Cross-repo wave execution**: Wave 1 (Agents A & B in parallel across 2 repos), Wave 2 (Agent C cleanup), manual fixes for pkg/orchestrator and pkg/engine
- **Test coverage maintained**: All YAML-related tests preserved; only markdown-specific tests removed

---
| [0.1.0] | 2026-03-08 | Initial engine extraction — parser, orchestrator, agent runner, git, worktree management |
## [0.28.0] - 2026-03-10

### Added

- **Multi-repo wave support** — `MergeAgents` and `VerifyCommits` now automatically detect cross-repo waves by reading `file_ownership.repo` and `completion_reports.repo` fields. Single-repo waves use optimized path; multi-repo waves group agents by repository and execute git operations per-repo. Fixes false reporting in Wave 2 execution where Agent G (scout-and-wave-web) was incorrectly reported as merged/verified in scout-and-wave-go.
- **`mergeAgentsMultiRepo()`** (`pkg/protocol/merge_agents.go`) — groups agents by repository, resolves relative paths from manifest directory, performs git merge operations in each repo separately. Error messages include repo context for debugging.
- **Repository resolution logic** — checks `FileOwnership.Repo` first (wave-specific files), falls back to `CompletionReport.Repo` (agent-level override), defaults to CLI-provided `--repo-dir` when both empty.
- **`VerifyCommits` repo-awareness** (`pkg/protocol/commit_verify.go`) — each agent's commit count checked in its correct repository using per-repo base commit (HEAD). Prevents false "no commits" reports when agent worked in different repo than CLI default.

### Changed

- **`MergeAgents` function signature unchanged** — backward compatible; single `--repo-dir` parameter still accepted, used as default for agents without explicit repo specification.

---

## [0.27.0] - 2026-03-10

### Added

- **Tool Journaling Wave 1** — Complete foundation for external log observer pattern (4 agents, 4605 lines, 53 tests passing):
  - **pkg/journal/observer.go** (430 lines) — Core `JournalObserver` that tails Claude Code session logs (`~/.claude/projects/<project>/*.jsonl`), extracts tool_use/tool_result blocks, maintains cursor for incremental reads, appends to index.jsonl, caches last 30 events in recent.json
  - **pkg/journal/context.go** (602 lines) — Context generator analyzes journal entries and produces markdown summaries for agent recovery after compaction. Extracts files modified, test results, git commits, scaffold imports, verification gates, completion report status
  - **pkg/journal/checkpoint.go** (277 lines) — Checkpoint system creates immutable snapshots of journal state at milestones. Supports list, restore, delete operations. Preserves checkpoints during restore
  - **pkg/journal/archive.go** (388 lines) — Archive policy compresses journal directories to .tar.gz with gzip, achieves 10:1 compression ratio, automatic retention cleanup, metadata tracking
  - **pkg/journal/types.go** (66 lines) — Shared types: `SessionCursor`, `ToolEntry`, `SyncResult`
  - **pkg/journal/doc.go** (67 lines) — Package documentation explaining external observer architecture

### Fixed

- **JournalObserver field visibility** — Capitalized private fields (`CursorPath`, `IndexPath`, `RecentPath`, `ResultsDir`) to allow checkpoint and archive methods to access them across files
- **Merge conflict resolution** — Removed stub `JournalObserver` declarations from archive.go and placeholder methods from observer.go after merging Wave 1 agents
- **Test field references** — Updated all test files to use capitalized field names after struct visibility changes

### Changed

- **53 tests passing** — Comprehensive test coverage: observer (9 tests), context (19 tests), checkpoint (12 tests), archive (13 tests)
- **Build verification** — `go build ./pkg/journal/...` and `go vet ./pkg/journal/...` pass cleanly

---

## [0.26.0] - 2026-03-10

### Fixed

- **Scaffold agent ID validation** (`pkg/protocol/validation.go`) — `validateAgentIDs` now accepts `"Scaffold"` as a special agent ID for wave 0 entries in the file ownership table. Previously rejected with `DC04_INVALID_AGENT_ID` because it didn't match the protocol pattern `^[A-Z][2-9]?$` (single uppercase letter, optionally followed by 2-9). Scaffold files are created before Wave 1 by the Scaffold Agent and need to appear in the file ownership table for complete tracking, but aren't owned by any Wave agent (A, B, C, etc.). The validator now special-cases `agent: "Scaffold"` with `wave: 0` to allow this pattern while maintaining strict validation for all Wave agents.

---

## [0.25.0] - 2026-03-10

### Fixed

- **`mark-complete` preservation bug** (`pkg/protocol/marker.go`) — `writeCompletionMarkerYAML` was using `map[string]interface{}` round-trip which destroyed IMPL doc structure. When `yaml.Unmarshal` parses into a map, it only preserves fields present in the YAML. When `yaml.Marshal` writes from the map, it has no knowledge of the `IMPLManifest` struct tags, so it only outputs what's in the map with default formatting. This compacted a 1613-line IMPL doc (with detailed agent prompts, completion reports, interface contracts, quality gates) down to 50 lines of minimal YAML.
- **Line-based YAML editing** — Rewrote `writeCompletionMarkerYAML` to use text-based line manipulation (`bufio.Scanner`, `strings.HasPrefix`) to find and update the `completion_date:` field in place, preserving 100% of original formatting, comments, block scalars, and indentation. Validates the modified YAML with `yaml.Unmarshal` before writing to catch errors. This is the correct approach for single-field updates when preservation is required — YAML libraries prioritize correctness over formatting.

---

## [0.24.0] - 2026-03-10

### Added

- **Cross-repo worktree support** (`pkg/protocol/worktree.go`) — `CreateWorktrees` now looks up each agent's repo from the FileOwnership table's Repo column. For cross-repo waves, worktrees are created in sibling directories (e.g., if orchestrating from scout-and-wave, Agent I worktree goes in scout-and-wave-go, Agent J in scout-and-wave-web). Falls back to single-repo behavior when Repo column is empty.
- **`determineAgentRepo` helper** — Scans FileOwnership map for agent's first file and returns its Repo field, enabling repo resolution without adding fields to Agent type.

---

## [0.23.0] - 2026-03-10

### Fixed

- **Hybrid IMPL doc parsing in `create-worktrees`** (`pkg/protocol/worktree.go`) — Changed from `Load()` (pure YAML unmarshaling) to `ParseIMPLDoc()` (hybrid markdown/YAML parser). IMPL docs started as pure markdown, then evolved to include typed YAML blocks (`type=impl-wave-structure`, `type=impl-quality-gates`) for machine-readable structure while keeping agent prompts in readable markdown. `create-worktrees` was calling the wrong parser (`Load()` expects 100% YAML) and failing with "wave N not found". Now uses `ParseIMPLDoc()` which handles the hybrid format.
- **Wave structure typed-block extraction** (`pkg/protocol/parser.go`) — Added `parseWaveStructureBlock()` to extract wave numbers and agent IDs from `type=impl-wave-structure` blocks. Parser now merges typed-block wave structure with markdown agent prompts, supporting the hybrid format throughout the toolchain.

### Changed

- **Wave/Agent type references** (`pkg/protocol/worktree.go`) — Updated to use `types.Wave` and `types.AgentSpec` with `Letter` field instead of deprecated `Agent.ID`, aligning with the hybrid format's agent identification scheme.

---

## [0.22.0] - 2026-03-10

### Fixed

- **E5 solo-wave exemption** (`pkg/protocol/fieldvalidation.go`) — `ValidateWorktreeNames` now builds a `waveSize` map and skips branch/worktree pattern checks for single-agent waves. Solo agents commit directly to main/develop per protocol; the `wave{N}-agent-{ID}` pattern only applies to multi-agent waves that use worktree isolation.
- **E10 lenient verification** (`pkg/protocol/fieldvalidation.go`) — Verification regex relaxed from `^(PASS|FAIL)(\s+\(.*\))?$` to `\b(PASS|FAIL)\b`. Accepts natural agent phrasing like `"go build, go test — all 18 tests PASS"` while still rejecting text without a PASS/FAIL keyword.

### Added

- **CLI reference docs** (`docs/cli-reference.md`) — All 21 `sawtools` commands documented with usage, flags, exit codes, and examples. Grouped into 5 categories: Validation, Context & Discovery, Wave Execution, Status & Reporting, Quality & Safety.
- 4 new validator tests: `SoloWaveExempt`, `SoloWaveStillFailsMultiAgent`, `DescriptivePass` (3 subtests). 4 existing tests updated to use multi-agent wave fixtures.

---

## [0.21.0] - 2026-03-10

### Added

- **Constraint solver** (`pkg/solver/`) — Kahn's algorithm (BFS topological sort) assigns agents to waves respecting dependency order. Protocol-independent: stdlib only, no imports from `pkg/protocol`.
  - `solver.go` — `Solve(nodes []DepNode) (*SolveResult, error)`: topological sort with wave-level assignment
  - `graph.go` — `DetectCycles`, `TransitiveDeps`, `CriticalPath`, `ValidateRefs`: graph analysis utilities
  - `types.go` — `DepNode`, `Assignment`, `SolveResult` type definitions
- **`SolveManifest`** (`pkg/protocol/solver_integration.go`) — Full pipeline: extract nodes from manifest → solve → compare computed vs declared waves → apply corrections → return rewritten manifest + change descriptions.
- **`sawtools solve`** (`cmd/saw/solve_cmd.go`) — CLI command: reads IMPL manifest, runs constraint solver, reports corrections. `--dry-run` shows changes without writing. `--json` for machine-readable output.
- **`--solver` flag on `sawtools validate`** (`cmd/saw/validate_cmd.go`) — Runs constraint solver as additional validation pass alongside E5/E10/E16 checks.
- 42 tests across solver (10), graph (14), integration (18).

---

## [0.20.0] - 2026-03-10

### Added

- **TimingMiddleware** (`pkg/tools/middleware.go`) — wraps tool execution with wall-clock timing. Emits `ToolCallEvent` (tool name, duration in ms, error status) after each call. Tool name is baked in at wrap time, not resolved from runtime metadata.
- **PermissionMiddleware** (`pkg/tools/middleware.go`) — blocks execution of tools not in an allowed set. Returns a denial message to the model (not a Go error) so it can adjust its approach. Used to enforce read-only mode for Scout agents.
- **`WithTiming(w Workshop, onCall)` / `WithPermissions(w Workshop, allowed)`** (`pkg/tools/middleware.go`) — convenience functions that return a new Workshop with middleware applied to every tool's executor. Original Workshop is not modified.
- **`ReadOnlyTools(workDir string) Workshop`** (`pkg/tools/middleware.go`) — factory for Scout agents. All 7 tools registered (model sees them) but `write_file` and `edit_file` blocked at execution time. `bash` permitted for codebase analysis.
- **`ReadOnlyAllowed` permission set** — `read_file`, `list_directory`, `glob`, `grep`, `bash` permitted; `write_file`, `edit_file` denied.
- **`OnToolCall ToolCallCallback`** on `backend.Config` (`pkg/agent/backend/backend.go`) — optional callback for tool timing events. When set, backends wrap the Workshop with `WithTiming` and bridge `tools.ToolCallEvent` → `backend.ToolCallEvent`.
- **`ReadOnly bool`** on `backend.Config` — when true, backends use `ReadOnlyTools()` instead of `StandardTools()`. Used for Scout agent backends.
- **`buildWorkshop(workDir)` method** on both `api.Client` and `openai.Client` — applies timing and permission middleware based on Config fields, replacing direct `tools.StandardTools()` calls.

## [0.19.0] - 2026-03-10

### Added

- **Unified tool system wiring** — `pkg/tools.Workshop` now powers both the Anthropic API backend (`pkg/agent/backend/api/client.go`) and the OpenAI-compatible backend (`pkg/agent/backend/openai/client.go`). Both backends call `tools.StandardTools(workDir)` to get a Workshop, iterate `Workshop.All()` for serialization, and call `tool.Executor.Execute(ctx, execCtx, input)` for execution. Eliminates 3 duplicated tool files.
- **3 new tool executors** (`pkg/tools/executors.go`) — `EditExecutor` (search-and-replace in files), `GlobExecutor` (file pattern matching), `GrepExecutor` (content search via ripgrep with line-scan fallback). All implement `ToolExecutor` interface.
- **7 standard tools** (`pkg/tools/standard.go`) — `read_file`, `write_file`, `list_directory`, `bash`, `edit_file`, `glob`, `grep`. Tool names use underscores (OpenAI function name compatible). Up from 4 tools in the old system.
- **Absolute path support** — all file executors use `resolvePath()` which passes absolute paths through unchanged and joins relative paths with `workDir`. Replaces the old relative-only + traversal-check pattern.

### Removed

- **`pkg/agent/tools.go`** — superseded by `pkg/tools/standard.go` + `pkg/tools/executors.go`
- **`pkg/agent/backend/api/tools.go`** — duplicated tool definitions for Anthropic backend, superseded by Workshop
- **`pkg/agent/backend/openai/tools.go`** — duplicated tool definitions for OpenAI backend (6 tools with local `tool` struct), superseded by Workshop

## [0.18.0] - 2026-03-10

### Added

- **E16A/E16C validator tests** (`pkg/protocol/validator_test.go`) — 5 new tests covering required block presence enforcement (E16A: missing blocks, all blocks present, no typed blocks) and out-of-band dep graph detection (E16C: warns on plain fenced block, no false positive on typed block). Validator logic for E16A and E16C was already implemented; tests verify correctness and prevent regressions.

## [0.17.0] - 2026-03-10

### Added

- **`GenerateScoutSchema()`** (`pkg/protocol/schema.go`) — reflects `scoutOutputSchema` (a stripped `IMPLManifest` without runtime-only fields) into a JSON Schema using `invopop/jsonschema` with `DoNotReference: true` to flatten `$ref` pointers as required by the Anthropic API
- **`scoutOutputSchema`** (unexported, `pkg/protocol/schema.go`) — mirrors `IMPLManifest` but omits `completion_reports`, `stub_reports`, `merge_state`, `worktrees_created_at`, `frozen_contracts_hash`, `frozen_scaffolds_hash`; Scout writes none of these
- **`WithOutputConfig(schema map[string]any) *Client`** (`pkg/agent/backend/api/client.go`) — sets `anthropic.OutputConfigParam` on the API client; wired into both `Run` and `RunStreaming`
- **`UseStructuredOutput bool`** and **`OutputSchemaOverride map[string]any`** on `RunScoutOpts` (`pkg/engine/engine.go`) — opt-in flags for structured output path
- **`runScoutStructured()`** (unexported, `pkg/engine/runner.go`) — calls API backend with output config, unmarshals JSON response directly into `*protocol.IMPLManifest`, saves to disk as YAML; activated when `UseStructuredOutput: true`
- **`SuitabilityAssessment string`** and **`CompletionDate string`** fields on `IMPLManifest` (`pkg/protocol/types.go`) — required by Scout structured output schema

### Dependencies

- Added `github.com/invopop/jsonschema v0.13.0` for Go struct → JSON Schema reflection

## [0.16.0] - 2026-03-10

### Added

- **9 YAML-mode CLI commands** completing the YAML manifest interaction layer:
  - `sawtools validate` — E16 invariant + typed-block validation; JSON output with error codes; exits 1 on failures
  - `sawtools extract-context --agent <ID>` — E23 per-agent context extraction; outputs `AgentContextJSONPayload` JSON with agent task, file ownership, interface contracts, scaffolds, quality gates
  - `sawtools set-completion --agent <ID> --status <complete|partial|blocked> --commit <sha>` — I4/I5 completion report registration; writes to manifest and saves
  - `sawtools mark-complete [--date YYYY-MM-DD]` — E15 completion marker; writes `completion_date` to manifest
  - `sawtools run-gates [--wave <N>]` — E21 quality gate execution; exits 1 if any required gate fails
  - `sawtools check-conflicts` — I1 file ownership conflict detection from completion reports; exits 1 if conflicts found
  - `sawtools validate-scaffolds` — I2 scaffold commit verification; exits 1 if any scaffold not committed
  - `sawtools freeze-check` — I2 interface contract freeze enforcement; exits 1 if violations found
  - `sawtools update-agent-prompt --agent <ID> --prompt <text>` — E8 downstream prompt updates
- **`ExtractAgentContextFromManifest`** (`pkg/protocol/extract.go`) — new YAML-mode SDK function; YAML equivalent of the existing markdown `ExtractAgentContext`; returns `*AgentContextJSONPayload` with typed fields importable by web server
- **`AgentContextJSONPayload`** struct in `pkg/protocol/extract.go` — structured output type for `extract-context`; JSON-serializable; includes `impl_doc_path`, `agent_id`, `agent_task`, `file_ownership`, `interface_contracts`, `scaffolds`, `quality_gates`
- **Fixed `.gitignore`** — anchored `saw` and `sawtools` patterns to `/saw` and `/sawtools` to prevent shadowing `cmd/saw/` source directory

## [0.15.0] - 2026-03-09

### Changed

- **Binary renamed from `saw` to `sawtools`** — clarifies the architectural split between `sawtools` (scout-and-wave-go: protocol toolkit, git/manifest operations, no agent execution) and `saw` (scout-and-wave-web: orchestration loop, runs agents, drives the full protocol lifecycle). All CLI subcommand names are unchanged; only the top-level binary name changes. Install: `go build -o sawtools ./cmd/saw` and copy to PATH.

## [0.14.0] - 2026-03-09

### Added

- **`saw verify-isolation`** (`cmd/saw/verify_isolation.go`, `pkg/protocol/isolation.go`) — new command agents call in Field 0 to confirm they are on the expected branch and registered in git's worktree list. Returns JSON `{"ok": true/false, "branch": "...", "errors": [...]}` and exits 1 on failure. Implements E12 isolation verification as a deterministic script rather than inline bash.
- **`scan-stubs --append-impl <path> [--wave N]`** (`cmd/saw/scan_stubs.go`, `pkg/protocol/stubs.go`) — appends stub scan results directly into the IMPL manifest under `stub_reports.wave{N}`, eliminating the manual copy-paste step from the orchestrator flow.
- **`merge-agents` auto-status-update** (`pkg/protocol/merge_agents.go`) — after each successful agent merge, automatically calls `UpdateStatus` to mark the agent `complete` in the manifest. `MergeStatus` now includes `status_updated` field in JSON output.
- **`StubReports` field on `IMPLManifest`** (`pkg/protocol/types.go`) — `map[string]*ScanStubsResult` keyed by `"wave{N}"`, stored under `stub_reports` in YAML.

## [0.13.0] - 2026-03-09

### Added

- **Cobra CLI migration** (`cmd/saw/`) — all 10 subcommands converted from hand-rolled `flag.NewFlagSet` + switch to `cobra.Command`. `newRootCmd()` in `root.go` owns `--repo-dir` as a persistent flag; each subcommand owns its local flags. Auto-generates `completion` and `help` subcommands. Fixes the arg-order bug where `saw create-worktrees /path --wave 1` failed because `flag.FlagSet` stops at the first non-flag positional argument.
- **`root.go`** (`cmd/saw/root.go`) — new file defining `newRootCmd()` and the package-level `repoDir` var bound to `--repo-dir`.
- **`github.com/spf13/cobra v1.10.2`** added to `go.mod`/`go.sum`.
- **`IMPL-cobra-cli-migration.yaml`** (`docs/IMPL/`) — first production YAML manifest generated by Scout v0.6.0.

## [0.12.0] - 2026-03-09

### Added

- **Protocol state machine** (`pkg/protocol/types.go`, `pkg/protocol/manifest.go`) — 11 `ProtocolState` constants (SCOUT_PENDING → COMPLETE), 4 `MergeState` constants, `State`/`MergeState` fields on `IMPLManifest`. `TransitionTo(m, target)` enforces SM-02 transition guards with structured `ValidationError` output. `ValidateSM02TransitionGuards()` implements full adjacency list matching protocol spec.
- **Interface freeze enforcement** (`pkg/protocol/freeze.go`) — `CheckFreeze(manifest)` detects post-worktree modifications to interface contracts and scaffolds via SHA256 hash comparison. `SetFreezeTimestamp(m, t)` records the freeze point. `FreezeViolation` struct for structured reporting. Implements E2/I2-02.
- **Quality gates runner** (`pkg/protocol/gates.go`) — `RunGates(manifest, waveNumber, repoDir)` executes quality gate commands via `os/exec`, captures stdout/stderr/exit code. `GateResult` struct with pass/fail, required flag. Implements E21.
- **Completion marker writer** (`pkg/protocol/marker.go`) — `WriteCompletionMarker(implDocPath, date)` inserts `<!-- SAW:COMPLETE date -->` after `# IMPL:` title. Handles both `.md` and `.yaml` files. Implements E15.
- **Ownership conflict detection** (`pkg/protocol/conflict.go`) — `DetectOwnershipConflicts(manifest, reports)` cross-references agent file lists to detect same-wave conflicts and undeclared modifications. `OwnershipConflict` struct. Implements E11.
- **Failure type decision tree** (`pkg/protocol/failure.go`) — `FailureTypeEnum` with 5 constants (transient, fixable, needs_replan, escalate, timeout). `ShouldRetry()`, `MaxRetries()`, `ActionRequired()` decision helpers. Implements E19.
- **Solo wave and completion helpers** (`pkg/protocol/helpers.go`) — `IsSoloWave(wave)`, `IsWaveComplete(wave, reports)`, `IsFinalWave(manifest, waveNumber)`. Implements SM-03.
- **Field format validation** (`pkg/protocol/fieldvalidation.go`) — `ValidateWorktreeNames(m)` (E5 branch naming regex), `ValidateVerificationField(m)` (E10 verification format).
- **Scaffold validation** (`pkg/protocol/scaffold_validation.go`) — `ValidateScaffolds(m)` returns per-file `ScaffoldStatus`, `AllScaffoldsCommitted(m)` boolean check. Implements SKILL-04.
- **Enum validation** (`pkg/protocol/enumvalidation.go`) — `ValidateCompletionStatuses(m)` (DC-02), `ValidateFailureTypes(m)` (DC-03), `ValidatePreMortemRisk(m)` (DC-06).
- **Project memory helpers** (`pkg/protocol/memory.go`) — `ProjectMemory` type with nested types, `LoadProjectMemory(path)`, `SaveProjectMemory(path, pm)`, `AddCompletedFeature(pm, feature)`. Implements E17/E18.
- **Agent prompt updater** (`pkg/protocol/updater.go`) — `UpdateAgentPrompt(m, agentID, newPrompt)` for E8 downstream prompt propagation.

### Changed

- **`Validate()` function wired** (`pkg/protocol/validation.go`) — all new validators (ValidateWorktreeNames, ValidateVerificationField, ValidateCompletionStatuses, ValidateFailureTypes, ValidatePreMortemRisk) now called from the main `Validate()` entrypoint. Previously only I1–I6 core validators were wired.

### Implementation

Delivered via SAW protocol: 3 waves, 12 agents (A–L). Wave 1 (5 agents): state machine, gates, freeze, prompt updater, enum validation. Wave 2 (4 agents): conflict detection, failure routing, helpers, field validation. Wave 3 (3 agents): scaffold validation, enum validation, project memory. Post-wave gap closure: wired all validators into `Validate()`, added `TransitionTo()`. Conformance audit: 91% coverage, zero critical gaps.

---

## [0.11.0] - 2026-03-09

### Added

- **Real AWS Bedrock backend** (`pkg/agent/backend/bedrock/`) — new package using AWS SDK v2 `bedrockruntime` service for authentic AWS Bedrock API calls. Authenticates via default credential chain (~/.aws/credentials, AWS env vars, IAM roles). Supports streaming with `InvokeModelWithResponseStream`. Uses Anthropic Messages API format (`bedrock-2023-05-31`).
- **Bedrock inference profile ID mapping** (`pkg/orchestrator/orchestrator.go`, `pkg/engine/chat.go`) — `expandBedrockModelID()` now maps short names to inference profile IDs required by Bedrock on-demand throughput: `claude-sonnet-4-5` → `us.anthropic.claude-sonnet-4-5-20250929-v1:0`. Profiles queried from `aws bedrock list-inference-profiles`.
- **AWS SDK dependencies** (`go.mod`) — added `github.com/aws/aws-sdk-go-v2`, `github.com/aws/aws-sdk-go-v2/config`, `github.com/aws/aws-sdk-go-v2/service/bedrockruntime`.

### Changed

- **`bedrock:` prefix behavior** — previously called Anthropic API with Bedrock-formatted model IDs (broken). Now creates real Bedrock backend using AWS SDK. **Breaking change**: requires AWS credentials instead of `ANTHROPIC_API_KEY`.

### Fixed

- **Chat backend missing bedrock case** — `pkg/engine/chat.go` provider routing had no `bedrock` case, causing `bedrock:` prefix to fall through to default CLI backend. Added `bedrock` case with `chatExpandBedrockModelID()` helper.

---

---

## [0.8.0] - 2026-03-09

### Added

- **Content-mode tool call fallback** (`pkg/agent/backend/openai/client.go`) — local models such as Qwen2.5-Coder via Ollama return tool calls as a JSON string in `content` with `finish_reason: "stop"` rather than in the `tool_calls` array. `parseContentToolCall` detects this pattern (valid JSON with `name` + `arguments`, where `name` is a registered tool) and routes it through the same execution path. False positives are prevented by requiring the tool name to exist in the tool map — a legitimate JSON final answer with an unknown key passes through as normal. The tool result is sent back as a user message (`"Function result:\n<result>"`) and the loop continues. Applied in both `Run` and `RunStreaming`.

---

## [0.7.0] - 2026-03-09

### Added

- **`"ollama"` provider prefix** (`pkg/orchestrator/orchestrator.go`) — `"ollama:granite3.1-dense:8b"` routes to the OpenAI-compatible backend with `BaseURL` defaulting to `"http://localhost:11434/v1"`. No API key required. `BaseURL` can be overridden via `BackendConfig.BaseURL` for non-default Ollama ports.
- **`"lmstudio"` provider prefix** (`pkg/orchestrator/orchestrator.go`) — `"lmstudio:phi-4"` routes to the OpenAI-compatible backend with `BaseURL` defaulting to `"http://localhost:1234/v1"`. No API key required.

Both prefixes alias into the existing `openaibackend` — no new package. Local model usage example in `saw.config.json`:
```json
{ "agent": { "wave_model": "ollama:granite3.1-dense:8b" } }
```

---

## [0.6.0] - 2026-03-09

### Added

- **`pkg/agent/backend/openai/` package** — new backend implementing `backend.Backend` via `net/http` against any OpenAI-compatible `POST /v1/chat/completions` endpoint. Supports the full tool-call loop (Bash, Read, Write, Edit, Glob, Grep) and streaming SSE for the final stop turn. Default model: `"gpt-4o"`. Constructor: `openai.New(cfg backend.Config) *Client`.
- **`backend.Config.APIKey string`** (`pkg/agent/backend/backend.go`) — API key for the OpenAI-compatible backend; falls back to `OPENAI_API_KEY` env var if empty.
- **`backend.Config.BaseURL string`** (`pkg/agent/backend/backend.go`) — optional endpoint override (e.g. `"https://api.groq.com/openai/v1"` for Groq, `"http://localhost:11434/v1"` for Ollama). Defaults to the official OpenAI endpoint.
- **`BackendConfig.OpenAIKey string`** (`pkg/orchestrator/orchestrator.go`) — orchestrator-level OpenAI key; falls back to `OPENAI_API_KEY`.
- **`BackendConfig.BaseURL string`** (`pkg/orchestrator/orchestrator.go`) — endpoint override forwarded to the OpenAI backend.
- **`parseProviderPrefix(model string) (provider, bareModel string)`** (`pkg/orchestrator/orchestrator.go`) — splits `"openai:gpt-4o"` → `("openai", "gpt-4o")`; no-colon input returns `("", model)`.
- **Provider-prefix routing in `newBackendFunc`** (`pkg/orchestrator/orchestrator.go`) — prefix parsed from `cfg.Model` overrides `cfg.Kind`; new dispatch cases:
  - `"openai"` → `openaibackend.New(backend.Config{...APIKey, BaseURL})`
  - `"anthropic"` → `apiclient.New(apiKey, backend.Config{...})`
  - `"cli"` → `cliclient.New(binaryPath, backend.Config{...})` where `binaryPath` comes from `SAW_CLI_BINARY` env
  - existing `"api"` / `"auto"` / `""` cases unchanged

### Changed

- `backend.Config` doc comment updated: `Model` is provider-agnostic; any model identifier accepted by the target backend is valid.

---

## [0.5.0] - 2026-03-09

### Added

- **`BinaryPath string` in `backend.Config`** (`pkg/agent/backend/backend.go`) — optional path to the CLI binary used by the CLI backend. When set, takes priority over the `claudePath` field on `Client` and over PATH lookup. Allows swapping `claude` for any compatible CLI binary (e.g. a future Kimi CLI, a local proxy, or an absolute path to a pinned version).
- **CLI binary resolution order** (`pkg/agent/backend/cli/client.go`) — updated to: `Client.claudePath` → `Config.BinaryPath` → PATH lookup for `"claude"`. Empty string at each step falls through to the next, preserving full backward compatibility.

### Changed

- `backend.Config.Model` doc comment updated to reflect that it is no longer Claude-specific — any model identifier the target CLI accepts is valid.

---

## [0.4.0] - 2026-03-09

### Added

**Per-agent model routing** — each wave agent can now run on a different LLM model (e.g. Scout on Opus, wave agents on Haiku), configured at two levels:

- **`RunScoutOpts.ScoutModel string`** (`pkg/engine/engine.go`) — model override for the Scout agent. Empty string uses the CLI/API default.
- **`RunWaveOpts.WaveModel string`** (`pkg/engine/engine.go`) — default model for all wave agents in the run. Per-agent `model:` field overrides this.
- **`AgentSpec.Model string`** (`pkg/types/types.go`) — per-agent model field parsed from the IMPL doc. Scout can write `**model:** claude-haiku-4-5` in any agent section to route that agent to a specific model.
- **`**model:**` parser** (`pkg/protocol/parser.go`) — parser now extracts `**model:** <id>` from agent sections, same pattern as the existing `**wave:**` metadata. Value is trimmed and stored in `AgentSpec.Model`.
- **`Orchestrator.SetDefaultModel(model string)`** (`pkg/orchestrator/orchestrator.go`) — sets the fallback model for agents that do not have a per-agent `model:` field.
- **Per-agent backend dispatch** (`pkg/orchestrator/orchestrator.go`) — `RunWave` now creates a separate backend instance for agents whose `AgentSpec.Model` differs from the wave default, enabling true per-agent provider routing. Agents without a model override share the default runner (zero extra backend construction).

### Fixed

- **`cli/client.go` silently ignored `Config.Model`** (`pkg/agent/backend/cli/client.go`) — `--model <model>` was never passed to the claude CLI even when `Config.Model` was set. Fixed by appending `--model` to args when the field is non-empty.

---

## [0.3.0] - 2026-03-08

### Fixed

**Protocol audit — 6 gaps identified by cross-referencing engine against protocol spec v0.14.5**

- **P0: `failure_type` not parsed in `ParseCompletionReport`** (`pkg/protocol/parser.go`) — The `raw` anonymous struct used for YAML unmarshaling had no `failure_type` field. `report.FailureType` was always empty string, routing every partial/blocked agent to `ActionEscalate` via `RouteFailure`. Added `FailureType string \`yaml:"failure_type"\`` to `raw` struct; result assigned to `report.FailureType`.

- **P0: Agent ID format `[A-Z][2-9]?` not supported** (`pkg/protocol/parser.go`, `pkg/protocol/validator.go`) — `isAgentHeader` checked `rest[1]` for `:`, ` `, or `—` only, so multi-char IDs like `A2` silently failed to parse (wave returned zero agents, `StartWave` exited immediately with no error). `extractAgentLetter` returned only `string(rest[0])`. Validator regexes `agentLineRe` and `agentRefRe` captured only `[A-Z]`. All fixed to support `[A-Z][2-9]?`.

- **P1: E22 scaffold build single-pass, no dependency resolution** (`pkg/engine/runner.go`) — `runScaffoldBuildVerification` ran only `go build ./...`. Protocol spec v0.14.2/v0.14.3 requires three steps: (1) dependency resolution (`go get ./...` + `go mod tidy`), (2) scaffold-package-only build (Pass 1), (3) full project build (Pass 2). Added dependency resolution step before build. Added scaffold-package-only Pass 1 using the scaffold file paths from `doc.ScaffoldsDetail` to derive the package set. Added language detection to handle Rust (`cargo fetch` + `cargo build -p <crate>`) and Node (`npm install` + `tsc --noEmit`); Python deferred.

- **P1: Cross-repo `Repo` column silently dropped** (`pkg/types/types.go`, `pkg/protocol/parser.go`, `pkg/orchestrator/orchestrator.go`) — `FileOwnershipInfo` had no `Repo` field. `parseFileOwnershipRow` handled 4-column tables only; a 5-column cross-repo table had the `Repo` column silently ignored. `ValidateInvariants` grouped file conflicts by `file` path alone, producing false I1 violations when the same filename existed in different repos. Fixed: added `Repo string` to `FileOwnershipInfo`; `parseFileOwnershipRow` detects and parses 5-column tables; `ValidateInvariants` now groups by `repo+file` composite key.

- **P2: `repo` field in completion reports not parsed** (`pkg/types/types.go`, `pkg/protocol/parser.go`) — `CompletionReport` had no `Repo` field; `raw` struct in `ParseCompletionReport` had no `repo` field. Added `Repo string \`yaml:"repo,omitempty"\`` to both.

- **P2: E19 auto-remediation not wired** — `RouteFailure` correctly computes and publishes the action as SSE but takes no automatic retry/relaunch action. Full auto-remediation requires significant orchestrator logic and LLM session management; deferred to a future release. Noted in orchestrator.go comment.

---

## [0.2.0] - 2026-03-08

### Added

- **E17 — Scout reads project memory:** `RunScout` in `pkg/engine/runner.go` reads `docs/CONTEXT.md` before constructing the scout prompt. If present, prepends it as `## Project Memory` so Scout avoids proposing types/interfaces that already exist.
- **E18 — Orchestrator writes project memory:** `UpdateContextMD` in `pkg/orchestrator/context.go` creates or appends to `docs/CONTEXT.md` after the final wave completes and verification passes. Commits the update automatically.
- **E19 — Failure type routing decision tree:** `RouteFailure` in `pkg/orchestrator/failure.go` maps `types.FailureType` values (`transient`, `fixable`, `needs_replan`, `escalate`, `timeout`) to `OrchestratorAction` constants. Wired into `launchAgent` in `orchestrator.go`: publishes `agent_blocked` event with routed action when completion report shows `partial` or `blocked` status.
- **E20 — Post-wave stub scan execution:** `RunStubScan` in `pkg/orchestrator/stubs.go` collects files from wave completion reports, invokes `scan-stubs.sh` from the SAW repo, and appends `## Stub Report — Wave {N}` section to the IMPL doc. Always returns nil (informational only). Wired into `StartWave` before merge step.
- **E21 — Post-wave quality gates:** `RunQualityGates` in `pkg/orchestrator/quality_gates.go` executes gates from the IMPL doc `## Quality Gates` section after wave agents complete and before merge. Required gate failures block merge. 5-minute per-gate timeout via `exec.CommandContext`. Wired into `StartWave`.
- **E22 — Scaffold build verification:** `runScaffoldBuildVerification` in `pkg/engine/runner.go` runs `go build ./...` in the repo after the scaffold agent completes. On failure, returns error and blocks wave launch.
- **E23 — Per-agent context extraction:** `ExtractAgentContext` and `FormatAgentContextPayload` in `pkg/protocol/extract.go` parse the IMPL doc and produce a trimmed per-agent payload containing only the agent's 9-field section, Interface Contracts, File Ownership, Scaffolds, and Quality Gates. Wired into `launchAgent` before `ExecuteStreaming`; falls back to full prompt on extraction error.
- **`ParseQualityGates`** added to `pkg/protocol/parser.go`; `ParseIMPLDoc` now populates `doc.QualityGates` when `## Quality Gates` section is present.
- **`types.FailureType`** string type with five constants (`transient`, `fixable`, `needs_replan`, `escalate`, `timeout`) added to `pkg/types/types.go`.
- **`types.QualityGate` and `types.QualityGates`** structs added to `pkg/types/types.go`.
- **`FailureType` field** added to `types.CompletionReport` (`yaml:"failure_type,omitempty"`).
- **`QualityGates` field** (`*types.QualityGates`) added to `types.IMPLDoc`.
- **`AgentBlockedPayload`** struct defined in `pkg/orchestrator/orchestrator.go` for `agent_blocked` SSE events.

### Implementation

Delivered via 2-wave SAW run (6 agents). Wave 1: new types + new isolated files. Wave 2: wiring into existing entrypoints. All tests green post-merge.

---

## [0.1.0] - 2026-03-08

### Added

- Initial engine extraction from `scout-and-wave-web`.
- `pkg/protocol/parser.go` — IMPL doc parser (wave/agent structure, completion reports, typed blocks).
- `pkg/orchestrator/` — wave orchestration: `RunWave`, `MergeWave`, `RunVerification`, `launchAgent`, SSE event publishing.
- `pkg/engine/runner.go` — `RunScout`, `RunScaffold`, `StartWave` entrypoints.
- `pkg/agent/runner.go` — `ExecuteStreaming` with API and CLI backends.
- `pkg/agent/backend/` — API backend (Claude API) and CLI backend (Claude Code subprocess).
- `pkg/types/types.go` — shared protocol types: `IMPLDoc`, `Wave`, `Agent`, `CompletionReport`, status constants.
- `internal/git/` — git operations used by orchestrator.
- `pkg/worktree/` — git worktree management.
- Go module: `github.com/blackwell-systems/scout-and-wave-go`.
