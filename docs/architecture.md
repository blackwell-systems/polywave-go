# Architecture Overview

Last reviewed: 2026-03-24

## Module

```
module github.com/blackwell-systems/scout-and-wave-go
```

Go engine and SDK for the Scout-and-Wave protocol. Consumed as a library by
`scout-and-wave-web` (HTTP/SSE web application) and directly via the `sawtools`
CLI binary (80 commands as of March 2026, including `help` and `completion`).

## High-Level Flow

```
User Request
    |
RunScout (pkg/engine/runner.go)
    -> Backend.RunStreaming with Scout prompt
    -> Writes YAML IMPL manifest to docs/IMPL/
    -> FinalizeIMPL: validate (E16) + populate gates (M4) + correct agent IDs (M1)
    |
StartWave (pkg/engine/runner.go | run_wave_full.go)
    -> PrepareWave: check deps (H6) + create worktrees + extract briefs + init journals
    -> Per-agent: worktree checkout, agent runner launch
    -> Agent.ExecuteStreaming (pkg/agent/runner.go)
        -> Backend.RunStreamingWithTools (tool call loop)
        -> Writes completion report to IMPL manifest
    -> Quality gates execution (E21)
    -> Merge to main
    |
Verification (pkg/orchestrator)
    -> Build verification
    -> Test suite
    -> Protocol invariants
    |
[Repeat for each wave]
    |
MarkComplete -> archive to docs/IMPL/complete/
```

### Program Layer (multi-IMPL orchestration)

```
PROGRAM manifest (docs/PROGRAM-*.yaml)
    -> Tiers: ordered groups of IMPL docs
    -> TierLoop (pkg/engine/program_tier_loop.go)
        -> For each tier: Scout all IMPLs -> execute waves -> tier gate
        -> Freeze contracts at tier boundary
    -> ParallelScout: run Scouts concurrently within a tier
    -> ProgramComplete: mark done, update CONTEXT.md
```

### Daemon / Queue System

```
Daemon (pkg/engine/daemon.go)
    -> Polls docs/IMPL/queue/ for queued Items
    -> Respects autonomy level (gated | supervised | autonomous)
    -> Dequeues next item -> runs Scout -> runs waves -> marks complete
    -> Continuous loop with configurable poll interval
```

## Package Map

The module contains 31 packages under `pkg/`, 1 under `internal/`, and 1 binary
under `cmd/`. Listed below by functional layer.

### Core Engine Layer

#### `pkg/engine`

High-level entrypoints for all operations. Orchestrates the other packages.

Key files:
- `runner.go` -- `RunScout`, `StartWave`, `RunScaffold`
- `chat.go` -- `RunChat` (standalone chat, no IMPL doc)
- `run_wave_full.go` -- `RunWaveFull` (create + verify + merge + build + cleanup)
- `daemon.go` -- `RunDaemon` (continuous queue processing loop)
- `program_tier_loop.go` -- `RunTierLoop` (multi-IMPL tiered execution)
- `program_parallel_scout.go` -- parallel Scout execution within a tier
- `program_auto.go` -- automated program execution
- `finalize_impl.go` -- `FinalizeIMPL` (validate + populate gates + re-validate)
- `finalize.go` -- `FinalizeWave` (verify + scan stubs + gates + merge + build + cleanup)
- `scout_correction_loop.go` -- Scout output correction (M1 agent ID fixing)
- `auto_remediate.go` -- automatic remediation for common failures
- `closed_loop_gate.go` -- closed-loop gate retry
- `fix_build.go` -- build failure repair
- `resolve_conflicts.go` -- merge conflict resolution
- `gate_cache.go` -- gate result caching integration
- `queue_advance.go` -- `CheckQueue` (advance daemon queue)
- `scheduler.go` -- wave scheduling
- `integration_runner.go` -- integration validation runner
- `constraints.go` -- engine-level constraint enforcement

**Imports:** `pkg/agent`, `pkg/orchestrator`, `pkg/protocol`, `pkg/result`,
`pkg/analyzer`, `pkg/commands`, `pkg/hooks`, `pkg/journal`, `pkg/observability`,
`pkg/retryctx`, `pkg/suitability`, `pkg/autonomy`, `pkg/queue`

#### `pkg/orchestrator`

Wave lifecycle management: launching agents, tracking completion, merging
branches, running verification, publishing SSE events.

Key files:
- `orchestrator.go` -- core wave lifecycle, agent launching
- `structured_wave.go` -- structured wave execution with state tracking
- `state.go` -- orchestrator state management
- `transitions.go` -- wave state transitions
- `merge.go` -- branch merging
- `events.go` -- SSE event broker and publishing
- `quality_gates.go` -- quality gate execution (E21)
- `failure.go` -- failure type routing (E19)
- `context.go` -- project memory (CONTEXT.md) updates (E18)
- `verification.go` -- build + test verification
- `stubs.go` -- stub/TODO scanning (E20)
- `journal_integration.go` -- journal observer integration
- `setters.go` -- completion report setters

Event flow:
`wave_started` -> `agent_started` -> `agent_output` (chunks) -> `agent_completed` -> `wave_completed`
Error path: `agent_blocked` with failure routing decision

#### `pkg/agent`

Agent execution runtime. Handles the tool-call loop, backend abstraction,
and streaming output.

Key files:
- `runner.go` -- `ExecuteStreaming` (main agent loop)
- `tools.go` -- tool definitions (Read, Write, Edit, Bash, Glob, Grep)
- `backend/` -- backend implementations (see below)

Tool call loop:
1. Backend sends messages + tools to LLM
2. LLM responds with text or tool_use
3. If tool_use: execute tool, append result, loop
4. If text (stop): return final answer

#### `pkg/agent/backend`

LLM provider abstraction. Defines the `Backend` interface:

```go
type Backend interface {
    Run(ctx, system, user, model string) (string, error)
    RunStreaming(ctx, system, user, model string, onChunk ChunkCallback) (string, error)
    RunStreamingWithTools(ctx, system, user, model string, onChunk ChunkCallback, onToolCall ToolCallCallback) (string, error)
}
```

**Implementations:**

| Sub-package | Provider | Notes |
|---|---|---|
| `backend/api` | Anthropic Messages API | Default backend |
| `backend/bedrock` | AWS Bedrock | AWS SDK v2 credentials |
| `backend/openai` | OpenAI-compatible | Also Groq, Ollama, LM Studio |
| `backend/cli` | Claude Code CLI | Subprocess, local dev |

Provider routing via model prefix:
- `anthropic:claude-opus-4-6` -> api
- `bedrock:claude-sonnet-4-5` -> bedrock
- `openai:gpt-4o` -> openai
- `ollama:qwen2.5-coder:32b` -> openai (localhost:11434)
- `lmstudio:phi-4` -> openai (localhost:1234)
- `cli:claude-sonnet-4-6` -> cli
- `claude-sonnet-4-6` -> api (default, no prefix)

### Protocol Layer

#### `pkg/protocol`

YAML IMPL manifest parsing, validation, mutation, and extraction. The largest
package (~157 files including tests). All IMPL documents are YAML manifests;
markdown parsing has been removed.

Key areas:
- **Load/Save:** `manifest.go` -- `Load()`, `Save()` for YAML manifests
- **Validation:** `validator.go`, `validation.go` -- protocol invariants (I1-I6); `schema_validation.go`, `schema_cross_field.go`, `schema_enum_validation.go`, `schema_unknown_keys.go` -- structural schema validation; `fieldvalidation.go`, `enumvalidation.go` -- field-level checks
- **Extraction:** `extract.go` -- per-agent context extraction (E23)
- **State machine:** `state_transition.go` -- `SetImplState()` with allowed transition map (`SCOUT_PENDING` -> `REVIEWED` -> `WAVE_EXECUTING` -> `COMPLETE`, etc.)
- **Mutation:** `amend.go` -- add waves, redirect agents; `updater.go`, `status_update.go` -- update agent status/completion; `marker.go` -- mark complete
- **Gates:** `gates.go`, `gate_populator.go` -- quality gate parsing and population (M4); `baseline_gates.go` -- default gate generation; `pre_wave_gate.go` -- pre-wave checks; `gate_input_validator.go` -- gate input validation
- **Worktrees:** `worktree.go`, `worktree_resolve.go` -- worktree creation and resolution; `stale_worktree.go` -- stale worktree detection
- **Merging:** `merge_agents.go` -- agent branch merging; `merge_log.go` -- merge audit log
- **Conflict detection:** `conflict.go`, `conflict_predict.go` -- file ownership conflicts
- **Build verification:** `verify_build.go` -- test/lint command execution; `commit_verify.go` -- commit verification (I5)
- **Stubs:** `stubs.go` -- stub/TODO scanning (E20)
- **Scaffolds:** `scaffold_validation.go` -- scaffold file validation
- **Integration:** `integration.go`, `integration_heuristics.go`, `integration_tsx.go` -- post-wave integration validation; `integration_checklist_validator.go`, `checklist_populator.go` -- M5 integration checklists; `wiring_injection.go`, `wiring_validation.go`, `wiring_types.go` -- wiring injection
- **Program layer:** `program_types.go` -- `PROGRAMManifest`, `ProgramTier`, `ProgramIMPL`; `program_parser.go` -- load/save programs; `program_validation.go` -- program schema validation; `program_generator.go` -- auto-generate programs from IMPLs; `program_prioritizer.go` -- IMPL prioritization; `program_conflict.go` -- cross-IMPL conflict detection; `program_freeze.go` -- contract freezing at tier boundaries; `program_tier_finalize.go`, `program_tier_gate.go` -- tier completion; `program_status.go` -- program status reporting; `program_discovery.go` -- discover IMPLs; `program_worktree.go` -- program-level worktree management
- **Freeze:** `freeze.go` -- contract/scaffold freeze with SHA-256 checksums after worktree creation
- **Misc:** `branchname.go` -- branch naming conventions; `cleanup.go` -- post-merge cleanup; `context_update.go` -- CONTEXT.md updates; `critic.go` -- critic agent review storage; `dependency_verifier.go` -- dependency verification (H6); `discovery.go` -- IMPL discovery; `error_codes.go` -- structured error codes; `failure.go` -- failure type definitions; `gomod_fixup.go` -- go.mod repair; `helpers.go` -- shared utilities; `isolation.go` -- worktree isolation verification (E12); `memory.go` -- agent memory; `ownership_coverage.go` -- ownership coverage checks; `repo_resolve.go` -- repo path resolution; `solver_integration.go` -- solver integration; `types_sections.go` -- section type definitions

#### `pkg/result`

Unified `Result[T]` generic type for consistent error handling across the
engine. Replaces ad-hoc result types with a single wrapper supporting full
success, partial success, and total failure signaling.

Key files:
- `result.go` -- `Result[T]` generic wrapper with `SUCCESS`, `PARTIAL`, `FATAL` codes
- `codes.go` -- structured error code constants by domain (V=validation, B=build, G=git, A=agent, N=engine, P=protocol, T=tool)

#### `pkg/tools`

Extensible tool system with Workshop registry, middleware, and backend adapters.

Components:
- `workshop.go` -- `Workshop` interface, `DefaultWorkshop` (thread-safe registry)
- `workshop_constrained.go` -- constrained workshop with role-based filtering
- `standard.go` -- standard tool set (Read, Write, Edit, Bash, Glob, Grep)
- `executors.go` -- tool executor implementations
- `middleware.go` -- composable middleware (logging, timing, validation)
- `role_middleware.go` -- role-based tool access control
- `adapters.go` -- backend-specific serializers (Anthropic, OpenAI, Bedrock)
- `constraint_enforcer.go` -- tool constraint enforcement
- `constraints.go` -- constraint definitions
- `types.go` -- `Tool`, `ToolExecutor`, `ExecutionContext`, `Middleware`, `Workshop` types

Namespaces: `file:read`, `file:write`, `bash`, `git:commit`, etc. Scout agents
receive read-only tools via namespace filtering; wave agents get the full set.

### Analysis and Planning

#### `pkg/analyzer`

Go source file dependency analysis. Parses AST to extract import relationships
and compute wave structure.

- `analyzer.go` -- `ParseFile`, `ExtractImports` (Go AST-based)
- `deps.go` -- dependency graph construction
- `cascade.go` -- detect files affected by type renames

#### `pkg/solver`

Compute optimal wave assignments from dependency declarations using topological
sort (Kahn's algorithm variant). Wave numbers are 1-based. Detects cycles and
missing references.

- `solver.go` -- `Solve(nodes []DepNode) SolveResult`
- `graph.go` -- directed graph representation
- `types.go` -- `DepNode`, `SolveResult`, `WaveAssignment`

#### `pkg/suitability`

Pre-implementation scanning. Analyzes repository files against requirements to
classify which items are already implemented, partially done, or missing.

- `scanner.go` -- `ScanPreImplementation`
- `types.go` -- `Requirement`, `SuitabilityResult`, `ItemStatus`
- `wrapper.go` -- CLI wrapper

#### `pkg/collision`

Type name collision detection across agent branches within the same wave.
Parses Go AST on each branch, groups by package, detects same-name types
declared by different agents.

- `detector.go` -- `DetectCollisions(manifestPath, waveNum, repoPath)`
- `types.go` -- `CollisionReport`, `Collision`

#### `pkg/deps`

Dependency conflict detection before wave execution. Parses lock files
(go.mod, Cargo.lock, etc.) and checks for conflicting version requirements
across agents.

- `checker.go` -- `CheckDeps(implPath, wave)`
- `gomod.go` -- go.mod parser
- `cargolock.go` -- Cargo.lock parser

#### `pkg/scaffold`

Shared type detection for scaffold extraction. Two modes:
- **Pre-agent:** analyzes interface contracts to find types referenced by 2+ agents
- **Post-agent:** parses agent tasks to detect duplicate type definitions

- `pre_agent.go`, `post_agent.go` -- detection modes
- `doc.go` -- design documentation

#### `pkg/scaffoldval`

Scaffold file validation pipeline. Checks syntax (Go AST parse), build
verification, and consistency with IMPL manifest.

- `validator.go` -- `ValidateScaffold(scaffoldPath, implPath)`
- `types.go` -- `ValidationResult`, `ValidationStep`

#### `pkg/interview`

Structured requirements interview state machine. CLI-driven multi-turn
conversation that produces a REQUIREMENTS.md.

Two modes:
- **deterministic:** fixed question set, no LLM required (default)
- **llm:** LLM-driven contextual questions

State persisted to `docs/INTERVIEW-<slug>.yaml`. Resumable across restarts.

- `deterministic.go` -- fixed question flow
- `compiler.go` -- compile answers into REQUIREMENTS.md

### Execution Support

#### `pkg/worktree`

Git worktree lifecycle management. Creates/removes worktrees for agent isolation.

Operations:
- Create: `git worktree add .claude/worktrees/wave1-agent-A wave1-agent-A`
- Cleanup: `git worktree remove --force`
- Branch tracking: associate worktrees with agent IDs

#### `pkg/journal`

External log observer for Claude Code sessions. Tails
`~/.claude/projects/<project>/*.jsonl` to track tool executions without
modifying the agent backend.

File structure per agent:
```
.saw-state/wave{N}/agent-{ID}/
+-- cursor.json          # read position
+-- index.jsonl          # append-only tool entries
+-- recent.json          # last 30 entries
+-- tool-results/
    +-- toolu_abc123.txt # full tool output
```

Key files:
- `context.go` -- generate context.md from journal entries for agent recovery
- `checkpoint.go` -- named snapshots at milestones
- `archive.go` -- compress old journals after wave completion

#### `pkg/retry`

Generate retry IMPL docs for failed quality gates (E24 retry loop). Creates
minimal single-wave, single-agent manifests targeting failed files.

- `impl_generator.go` -- `GenerateRetryIMPL`
- `loop.go` -- retry loop orchestration
- `types.go` -- retry types

#### `pkg/retryctx`

Structured failure context for agent retries. Classifies errors from prior
attempts (import_error, type_error, test_failure, build_error, lint_error) and
builds retry prompts with specific fix suggestions.

- `retryctx.go` -- `BuildRetryContext`, error classification

#### `pkg/resume`

Detection of interrupted SAW sessions. Scans IMPL docs and git state to
determine what was in progress and recommends how to resume.

- `detect.go` -- `DetectSessions`
- `sessions.go` -- session state analysis
- `worktree_status.go` -- worktree health checks (file content hashes, etc.)

#### `pkg/gatecache`

Quality gate result caching. Keys on HEAD commit + staged/unstaged stat +
command string. Default TTL: 5 minutes. Avoids re-running expensive gates
when repo state hasn't changed.

- `cache.go` -- `CacheKey`, `CacheEntry`, lookup/store

#### `pkg/hooks`

Scout boundary enforcement. Validates that Scout did not write files outside
its allowed boundaries (docs/IMPL/ only).

- `scout_boundaries.go` -- `ValidateScoutWrites`

#### `pkg/idgen`

Agent ID generation following the `^[A-Z][2-9]?$` pattern. Supports sequential
mode (A-Z, A2-Z2, ...) and grouped mode (category-based multi-generation IDs).

- `generator.go` -- `AssignAgentIDs(count, grouping)`

### Build and Error Diagnostics

#### `pkg/builddiag`

Build failure pattern matching. Language-specific error catalogs (Go, JS, etc.)
with regex patterns, confidence scores, and fix suggestions.

- `diagnose.go` -- `DiagnoseError(errorLog, language)`
- `go_patterns.go` -- Go compiler error patterns
- `js_patterns.go` -- JavaScript/TypeScript error patterns

#### `pkg/errparse`

Structured error parsing for CI tool output. Extracts file, line, column, and
message from linter/formatter/compiler output.

- `format_parsers.go` -- gofmt, prettier, ruff, cargo-fmt parsers
- `go_parsers.go` -- Go compiler/vet parsers
- `js_parsers.go` -- ESLint, TypeScript parsers

#### `pkg/commands`

Build/test/lint/format command extraction from CI configs and build system files.
Priority-based resolution across multiple parsers.

- `extractor.go` -- `Extractor` with CI and build system parser registry
- `defaults.go` -- default commands by language
- `github_actions.go` -- GitHub Actions workflow parser

#### `pkg/format`

Project formatter detection. Inspects marker files (go.mod, package.json,
pyproject.toml, Cargo.toml) to determine the appropriate formatter and
generate check/fix commands.

- `detect.go` -- `DetectFormatter(projectRoot) FormatConfig`

### Observability

#### `pkg/observability`

Event types and storage abstractions for cost tracking, agent performance, and
orchestrator activity.

**Status: interface-only.** The `Store` interface and event types are defined.
The `Emitter` provides nil-safe async event recording. No concrete store
implementation (SQLite, PostgreSQL) exists in this repo yet.

Components:
- `store.go` -- `Store` interface (`RecordEvent`, `QueryEvents`, `GetRollup`, `Close`)
- `events.go` -- `Event` interface, `CostEvent`, `AgentPerformanceEvent`, `ActivityEvent`
- `emitter.go` -- `Emitter` (nil-safe, non-blocking wrapper around Store)

Query/rollup support: `QueryFilters` (by event type, IMPL slug, program slug,
agent ID, time range) and `RollupRequest` (cost, success_rate, retry_count
grouped by agent/wave/impl/program/model).

### Orchestration Infrastructure

#### `pkg/pipeline`

Generic step-based pipeline framework. Named sequences of steps with conditions,
error strategies (fail/continue), and context cancellation.

- `pipeline.go` -- `Pipeline`, `Step`, `Run`
- `registry.go` -- named pipeline registry
- `saw_steps.go` -- SAW-specific pipeline step implementations
- `types.go` -- `StepFunc`, `ErrorStrategy`, `State`

#### `pkg/queue`

IMPL queue management. Manages `docs/IMPL/queue/` directory for the daemon
system. Items have priority, dependencies, status, and optional autonomy
overrides.

- `manager.go` -- `Manager` (enqueue, dequeue, list, status updates)
- `types.go` -- `Item` (title, priority, depends_on, status, autonomy_override)

#### `pkg/autonomy`

Autonomy level configuration for orchestrator decision-making. Three levels
control how much human approval is required.

- `autonomy.go` -- level checking and decision logic
- `config.go` -- configuration loading from `saw.config.json`

Levels: `gated` (human approves everything), `supervised` (auto-advance with
review points), `autonomous` (fully automatic).

Stages: `impl_review`, `wave_advance`, `gate_failure`, `queue_advance`.

#### `pkg/codereview`

AI-powered code review on post-merge diffs. Uses Claude (haiku by default) to
score code across dimensions and produce a summary.

- `reviewer.go` -- `RunReview` (calls Anthropic API with diff)
- `types.go` -- `DimensionScore`, review response types

### Internal Packages

#### `internal/git`

Low-level git operations used by `pkg/protocol`, `pkg/orchestrator`, and
`pkg/collision`. Provides commit, branch creation/deletion, merge (with conflict
detection), diff, status, and log parsing. Not importable outside this module.

### CLI Binary

#### `cmd/saw` (sawtools)

Cobra-based CLI binary with 80 commands (including `help` and `completion`).
Each command wraps one or more engine/protocol functions. Commands are organized
by workflow phase:

**Scout phase:**
`run-scout`, `finalize-impl`, `validate`, `analyze-suitability`, `interview`

**Wave preparation:**
`prepare-wave`, `create-worktrees`, `extract-context`, `prepare-agent`,
`journal-init`, `check-deps`, `check-type-collisions`, `detect-scaffolds`,
`validate-scaffold`, `validate-scaffolds`, `verify-hook-installed`,
`verify-isolation`, `freeze-contracts`, `freeze-check`, `pre-wave-gate`

**Wave execution:**
`run-wave`, `run-critic`, `build-retry-context`, `retry`

**Wave finalization:**
`finalize-wave`, `merge-agents`, `run-gates`, `scan-stubs`, `verify-commits`,
`verify-build`, `cleanup`, `cleanup-stale`, `mark-complete`, `close-impl`

**Program layer:**
`create-program`, `validate-program`, `import-impls`, `list-programs`,
`program-execute`, `program-replan`, `program-status`, `finalize-tier`,
`tier-gate`, `check-program-conflicts`, `mark-program-complete`,
`create-program-worktrees`, `prepare-tier`, `update-program-impl`,
`update-program-state`

**Daemon / queue:**
`daemon`, `list-impls`, `queue`

**Analysis / diagnostics:**
`analyze-deps`, `detect-cascades`, `check-conflicts`, `check-impl-conflicts`,
`diagnose-build-failure`, `solve`, `debug-journal`, `journal-context`,
`validate-integration`, `resume-detect`, `extract-commands`, `metrics`, `query`,
`check-completion`

**Mutation:**
`amend-impl`, `assign-agent-ids`, `set-completion`, `set-critic-review`,
`set-impl-state`, `update-agent-prompt`, `update-context`, `update-status`,
`populate-integration-checklist`, `baseline-output`

**Misc:**
`run-review`, `verify-install`

## IMPL Document Type

`protocol.IMPLManifest` is the single authoritative type for IMPL documents.
It is a YAML-native struct with tags supporting round-trip `Load`/`Save`,
protocol state transitions, program-layer fields, freeze checksums, and
structured completion reports. The legacy `types.IMPLDoc` (markdown-era) has
been fully removed.

## Protocol State Machine

IMPL manifests track lifecycle state via `protocol.ProtocolState`:

```
SCOUT_PENDING -> REVIEWED -> SCAFFOLD_PENDING -> WAVE_PENDING -> WAVE_EXECUTING
                    |                                                |
                    v                                                v
              NOT_SUITABLE                                    WAVE_MERGING
                                                                   |
                                                                   v
                                              WAVE_VERIFIED -> COMPLETE
                                                   |
                                                   v
                                            (next wave: WAVE_EXECUTING)

Any active state -> BLOCKED (recoverable)
```

State transitions are enforced by `protocol.SetImplState()` with a static
allowed-transitions map. Invalid transitions return an error.

## Cross-Repo Architecture

The engine is split across three repos:

- **scout-and-wave** -- protocol specification (invariants, execution rules, agent prompts)
- **scout-and-wave-go** (this repo) -- engine, protocol SDK, CLI
- **scout-and-wave-web** -- web UI, HTTP server, `saw serve` binary

`scout-and-wave-web` imports the engine as a Go module:

```go
import (
    "github.com/blackwell-systems/scout-and-wave-go/pkg/engine"
    "github.com/blackwell-systems/scout-and-wave-go/pkg/orchestrator"
    "github.com/blackwell-systems/scout-and-wave-go/pkg/protocol"
)
```

The `saw serve` command in `scout-and-wave-web` wraps the engine with HTTP
handlers and an SSE broker.

## Design Decisions

### Why Backend Abstraction?

The `backend.Backend` interface decouples the engine from specific LLM
providers. Each backend serializes tools into its native format (Anthropic
`tools` array, OpenAI `functions`, etc.) and deserializes responses back into
the unified tool call loop. Adding a new provider requires only implementing
three methods.

### Why Worktrees?

Git worktrees provide true filesystem isolation for parallel agents:
- Each agent works in its own directory on its own branch
- No file locking needed -- agents cannot conflict at the filesystem level
- Merge is explicit and auditable (main never receives uncommitted changes)
- Cleanup is safe even if agent crashes (worktree + branch are isolated)

### Why SSE for Events?

Server-Sent Events provide real-time streaming with simple HTTP:
- One-way server -> client (sufficient for read-only monitoring)
- Automatic reconnection on disconnect
- Native browser EventSource API
- Lower overhead than WebSocket for unidirectional streams

### Why YAML Manifests?

IMPL documents migrated from markdown to YAML for:
- Deterministic round-trip parsing (no ambiguous markdown edge cases)
- Native Go struct mapping via `yaml` tags
- Machine-writable without parser fragility
- Schema validation via JSON Schema cross-compilation
- Clean `Load`/`Save` cycle

### Why the Pipeline Package?

The `pkg/pipeline` framework provides a reusable step-based execution model
that the batching commands (run-scout, prepare-wave, finalize-wave,
finalize-impl) use internally. Steps can have conditions, error strategies
(fail-fast or continue), and the pipeline respects context cancellation.

## Extension Points

1. **Custom backends** -- implement `backend.Backend` for new LLM providers
2. **Custom tools** -- register via `Workshop` with namespace and executor
3. **Middleware** -- wrap tool execution (logging, timing, validation, permissions)
4. **Quality gates** -- add custom verification commands to IMPL manifest
5. **SSE consumers** -- subscribe to orchestrator events for monitoring
6. **Observability stores** -- implement `observability.Store` for custom backends
7. **Pipeline steps** -- register custom steps in the pipeline registry
8. **Queue items** -- add IMPL requests to `docs/IMPL/queue/` for daemon processing
