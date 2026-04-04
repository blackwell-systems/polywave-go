# scout-and-wave-go

[![Blackwell Systems™](https://raw.githubusercontent.com/blackwell-systems/blackwell-docs-theme/main/badge-trademark.svg)](https://github.com/blackwell-systems)
[![CI](https://github.com/blackwell-systems/scout-and-wave-go/actions/workflows/ci.yml/badge.svg)](https://github.com/blackwell-systems/scout-and-wave-go/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/blackwell-systems/scout-and-wave-go)](https://github.com/blackwell-systems/scout-and-wave-go/releases)

Go engine and Protocol SDK for Scout-and-Wave (SAW) — a coordination protocol for parallel AI agent development that guarantees merge-conflict-free execution by construction.

---

## The Core Guarantee

**No two agents in the same wave own the same file** (I1: Disjoint File Ownership).

This is not a convention or a preference. It is a hard constraint enforced before any worktree is created. The result: parallel agents can never produce a merge conflict on agent-owned files. The conflict is structurally impossible because the ownership partition is verified and locked before execution begins.

This is distinct from branch-based coordination. Branches prevent concurrent writes to the same commit, but do nothing to prevent two agents from independently modifying the same file on separate branches — which produces a merge conflict you must resolve manually. SAW prevents the conflict from being possible in the first place.

---

## Feature Highlights

- **Merge-conflict-free by construction** — file ownership is partitioned and locked before execution begins; conflicts on agent-owned files are structurally impossible
- **Runs on any LLM: Anthropic, OpenAI, Ollama, or any OpenAI-compatible endpoint. Mix providers per-agent within the same wave.**
- **Full protocol SDK** — importable Go module, no LLM dependencies, deterministic for all inputs
- **75+ CLI commands** — single-purpose with structured JSON output, covering the full wave lifecycle
- **Program layer** — tier-gated execution of multiple IMPLs with shared contract freezing

---

## LLM Backend

The engine abstracts agent execution behind a `Runtime` interface. Provider routing uses model prefix notation:

| Prefix | Backend |
|--------|---------|
| `anthropic:` | Anthropic API (direct) |
| `openai:` | OpenAI-compatible endpoint (OpenAI, Ollama, LM Studio, etc.) |
| `cli:` | Local CLI binary (Claude Code, `SAW_CLI_BINARY`) |
| *(none)* | Auto-detect from environment |

### Per-agent provider routing

Each agent in an IMPL doc carries an optional `model:` field. The orchestrator reads it at launch time and routes that agent to the appropriate backend — with zero changes to the orchestrator itself.

This means a single wave can run heterogeneous workloads: route the broad-context analysis agent to a large-context model, route the mechanical code-generation agents to a fast, cheap model, and route the integration agent to whichever model has the strongest reasoning for your stack. The allocation lives in the IMPL doc, not in orchestrator code.

Example: Agent A on `anthropic:claude-haiku-3` (fast, cheap, high-parallelism), Agent B on `anthropic:claude-opus-4` (complex reasoning, low-parallelism), Agent C on `openai:llama3` via a local Ollama endpoint — all executing concurrently in the same wave, all merging into the same branch when done.

---

## How It Works

SAW decomposes feature work into three phases:

**Scout** — An AI agent analyzes the codebase, runs a suitability gate, designs the file ownership partition, defines interface contracts across agent boundaries, and writes an IMPL doc (a YAML manifest that is the single source of truth for the entire feature).

**Wave** — Parallel AI agents execute concurrently, each in its own git worktree, each owning a disjoint set of files. Agents implement against pre-committed scaffold files; they never coordinate directly. When the wave completes, merging is mechanical — no conflicts on agent-owned files by construction.

**Merge + Verify** — `finalize-wave` runs the full post-wave pipeline: commit verification, stub detection, quality gates (concurrent), merge, build verification, and cleanup. Integration gaps are detected and optionally wired by an Integration Agent.

The IMPL doc flows through the entire lifecycle: Scout writes it, agents append their completion reports to it, the orchestrator reads it to track state, and it becomes the audit trail when the feature closes.

---

## The IMPL Doc as Single Source of Truth

Every piece of protocol state lives in one YAML file (I4):

- Suitability verdict and pre-mortem risk assessment
- File ownership table (per-agent, per-wave, per-repo for cross-repo waves)
- Interface contracts and scaffold file status
- Quality gate configuration
- Agent prompts (9-field format)
- Wave structure and dependency graph
- Completion reports from every agent
- Stub scan results and integration reports

Chat output is not the record. If a completion report is written to chat only, it is a protocol violation — downstream agents and the orchestrator cannot see it.

This makes the protocol observable and auditable. Every SAW session can be reconstructed from the IMPL doc and git history alone.

---

## Interface Contracts and the Freeze Point

Before any worktree is created, the Scout defines all interfaces that cross agent boundaries. A Scaffold Agent materializes them as committed source files on HEAD. Then worktrees branch from that HEAD.

When worktrees are created, interface contracts become immutable (I2, E2). This is the freeze point: wave agents implement against a committed spec, not against each other. An agent cannot discover at runtime that a type it expected does not exist — the type was committed before the agent launched.

Wave N+1 does not launch until Wave N merges and post-merge verification passes (I3). This provides cross-wave coordination without special mechanisms: each successive wave branches from the fully merged codebase of all prior waves.

---

## Program Layer

For multi-feature projects, SAW includes a program layer that coordinates multiple IMPL docs through tier-gated execution.

A **PROGRAM manifest** decomposes a project into:
- **Tiers** — groups of independent IMPLs that execute in parallel
- **Program contracts** — shared types and interfaces consumed by multiple IMPLs, frozen at tier boundaries
- **Tier gates** — quality checks that must pass before the next tier begins

Execution rules:
- **E28** — All IMPLs in a tier are scouted in parallel (one Scout per IMPL)
- **E29** — Tier gate runs after all IMPLs in the tier complete; blocks advancement on failure
- **E30** — Program contracts freeze at tier boundaries; downstream Scouts receive them as immutable inputs
- **E33** — `--auto` mode advances tiers automatically after gate pass; human gate is never skipped on failure
- **E34** — Tier gate failure re-engages the Planner to revise the PROGRAM manifest; human reviews the revised plan before any tier re-executes

The DAG prioritization engine (`sawtools` integrates `PrioritizeIMPLs` / `ScoreTierIMPLs`) scores IMPL execution order within a tier based on dependency depth and downstream unlock count.

---

## Autonomy Layer

Three autonomy levels control how much the orchestrator pauses for human review:

| Level | Behavior |
|-------|----------|
| `gated` | Pauses at every decision point (default) |
| `supervised` | Auto-approves wave advancement and gate retry; pauses for IMPL review |
| `autonomous` | All stages auto-approved; only surfaces gate failures |

The daemon run loop (`sawtools daemon`) enables fully unattended execution across multiple queued IMPLs. Failure classification (E19) routes correctable failures (`transient`, `fixable`) to automatic retry and non-correctable failures (`needs_replan`, `escalate`) to human review, regardless of autonomy level.

The `build-retry-context` command produces structured failure context (error classification, targeted fix suggestions, retry prompt) for re-launching a failed agent with awareness of prior attempts.

---

## sawtools CLI

The `sawtools` binary provides 60+ commands covering the full protocol lifecycle. Commands are single-purpose with structured JSON output.

### Wave lifecycle

| Command | What it does |
|---------|-------------|
| `prepare-wave` | Atomic: baseline gate (E21A) + worktree creation + per-agent brief extraction + journal init |
| `finalize-wave` | Atomic: verify-commits + scan-stubs + run-gates + merge-agents + verify-build + cleanup |
| `create-worktrees` | Create git worktrees for a wave's agents |
| `merge-agents` | Merge wave worktree branches to main |
| `verify-commits` | Verify each agent has commits before merge (I5) |
| `verify-build` | Post-merge build verification |
| `cleanup` | Remove worktrees after merge |

### Validation and verification

| Command | What it does |
|---------|-------------|
| `validate` | E16 manifest validation: required blocks, dep graph grammar, duplicate keys, action enums |
| `check-conflicts` | I1 file ownership conflict detection across agents |
| `validate-scaffolds` | Verify scaffold files are committed before worktree creation (I2) |
| `freeze-check` | Interface contract freeze enforcement (E2) |
| `scan-stubs` | E20 stub detection across agent-owned files |
| `run-gates` | E21/E21A quality gate verification (concurrent, E21B) |
| `predict-conflicts` | E11 hunk-level conflict prediction before merge |
| `check-type-collisions` | Detect duplicate type/const definitions across agent branches |
| `pre-wave-validate` | Combined E16 + E35 + test cascade + wave structure check |
| `finalize-scout` | Consolidates Scout validation: validate + pre-wave-validate + validate-briefs + set-injection-method |
| `validate-integration` | E25 integration gap detection; E35 wiring obligation verification |
| `verify-isolation` | Verify agent is in correct worktree before execution begins |
| `analyze-suitability` | Pre-implementation scanning gate (H1a) |
| `detect-cascades` | Cross-package cascade detection (M2) |

### Agent lifecycle

| Command | What it does |
|---------|-------------|
| `run-scout` | Full Scout pipeline: launch + validate (E16) + auto-correct IDs (M1) + finalize gates (M4) |
| `prepare-agent` | Extract brief + init journal for a single agent |
| `extract-context` | E23 per-agent context extraction from IMPL doc |
| `set-completion` | Register agent completion report |
| `set-critic-verdict` | Set critic report verdict (PASS/ISSUES) without duplicate key risk |
| `set-impl-state` | Atomically transition IMPL state (protocol state machine) |
| `set-injection-method` | Record how Scout received reference files (hook/manual-fallback) |
| `update-status` | Update agent/wave status in manifest |
| `update-agent-prompt` | E8 downstream prompt update after contract revision |
| `build-retry-context` | Structured failure context for agent retry (E19) |
| `assign-agent-ids` | M1 auto-correct agent ID assignment |

### Program layer

| Command | What it does |
|---------|-------------|
| `validate-program` | PROGRAM manifest schema validation + P1 circular dependency check |
| `tier-gate` | E29 tier quality gate verification |
| `freeze-contracts` | E30 program contract freezing at tier boundary |
| `program-status` | E32 full program status report (per-tier, per-IMPL, contract freeze states) |
| `program-replan` | E34 re-engage Planner on tier gate failure |
| `mark-program-complete` | Mark PROGRAM manifest complete + update CONTEXT.md |
| `list-programs` | Discover PROGRAM manifests in repo |

### Setup

| Command | What it does |
|---------|-------------|
| `init` | Auto-detect project language, build/test commands; generate `saw.config.json` |
| `verify-install` | Check prerequisites: sawtools on PATH, skill files, Git version, repo paths |

### Utilities

| Command | What it does |
|---------|-------------|
| `amend-impl` | E36 IMPL amendment: add-wave, redirect-agent, extend-scope |
| `close-impl` | Atomic: E15 SAW:COMPLETE + archive + update CONTEXT.md + git commit |
| `mark-complete` | E15 write SAW:COMPLETE marker + archive to `docs/IMPL/complete/` |
| `update-context` | E18 update `docs/CONTEXT.md` after feature completion |
| `list-impls` | Discover IMPL docs (active and archived) |
| `resume-detect` | Detect interrupted SAW sessions for recovery |
| `journal-init` | Initialize agent tool journal |
| `journal-context` | Generate context summary from tool journal (E23A) |
| `daemon` | Run loop for autonomous multi-IMPL execution |
| `run-review` | AI code review gate on merged diff |
| `diagnose-build-failure` | AI-assisted build failure diagnosis |

### Install

```bash
# Homebrew (macOS/Linux)
brew install blackwell-systems/tap/sawtools

# Or via Go install
go install github.com/blackwell-systems/scout-and-wave-go/cmd/sawtools@latest
```

<details>
<summary>Build from source (for contributors)</summary>

```bash
go build -o sawtools ./cmd/sawtools
cp sawtools ~/.local/bin/sawtools
```
</details>

---

## Protocol SDK

The `pkg/protocol` package is the importable core: pure Go, no LLM dependencies, deterministic for all inputs.

```go
import "github.com/blackwell-systems/scout-and-wave-go/pkg/protocol"

// Load and validate a manifest
manifest, err := protocol.Load(ctx, "docs/IMPL/IMPL-feature.yaml")
errs := protocol.Validate(manifest)

// I1 check: will any agents in wave 1 conflict?
conflicts := protocol.CheckOwnershipConflicts(manifest, 1)

// Query current wave
wave := protocol.CurrentWave(manifest)

// Register agent completion
protocol.SetCompletionReport(manifest, "A", protocol.CompletionReport{
    Status: "complete",
    Commit: "abc123",
    Branch: "saw/my-feature/wave1-agent-A",
    FilesCreated: []string{"pkg/cache/cache.go"},
})
protocol.Save(ctx, manifest, "docs/IMPL/IMPL-feature.yaml")
```

### Invariant enforcement

| Invariant | Rule | Enforcement |
|-----------|------|-------------|
| I1 | No two agents in the same wave own the same file | `Validate()` checks ownership table; `check-conflicts` at pre-launch |
| I2 | Interface contracts defined before agents launch | Scaffold files committed before worktrees created; `freeze-check` enforces |
| I3 | Wave N+1 waits for Wave N merge | `CurrentWave()` returns first incomplete wave; orchestrator controls transitions |
| I4 | IMPL manifest is single source of truth | All state read/written via SDK operations; completion reports written to IMPL doc |
| I5 | Agents commit before reporting | `SetCompletionReport()` requires commit hash; `verify-commits` gates merge |
| I6 | Orchestrator does not perform agent work | Behavioral; enforced by role separation in agent type definitions |

Validation errors are structured (`ValidationError` with code, message, field) — not line-number parse errors.

### Package structure

See [`pkg/README.md`](pkg/README.md) for the full package map (27 packages) with dependency hierarchy and entry points.

Key packages:

```
pkg/
├── protocol/       # Protocol SDK — types, validation, manifest I/O, baseline gates
├── engine/         # High-level entrypoints: RunScout, FinalizeWave, RunReview
├── orchestrator/   # Wave orchestration, SSE events, journal integration
├── agent/          # Agent execution runtime, tool dispatch, LLM backends
├── result/         # result.Result[T] — canonical return type for fallible functions
├── collision/      # Type/const collision detection across agent branches
├── gatecache/      # Baseline gate result caching (E38)
├── observability/  # Event store, metrics rollups, query engine
├── journal/        # Tool journal: append-only execution trace, context recovery
├── pipeline/       # Atomic batching pipeline (prepare-wave, finalize-wave)
├── solver/         # Constraint solver for file ownership optimization
├── worktree/       # Git worktree management
└── ...             # 15 more — see pkg/README.md

internal/
└── git/            # Git operations (commit, branch, merge)
```

---

## Installation

```bash
go get github.com/blackwell-systems/scout-and-wave-go
```

---

## Getting Started

**Using the `/saw` skill (Claude Code):** See the [protocol repository](https://github.com/blackwell-systems/scout-and-wave) for the skill and agent prompts. The `/saw scout <feature>` → `/saw wave` → `/saw wave --auto` workflow is the primary path for Claude Code users.

**Using sawtools directly:**

```bash
# Install the CLI
go install github.com/blackwell-systems/scout-and-wave-go/cmd/sawtools@latest

# Initialize your project (auto-detects language, build, and test commands)
cd your-project
sawtools init

# Validate an IMPL doc
sawtools validate docs/IMPL/IMPL-feature.yaml

# Prepare wave 1 (baseline gate + worktrees + agent briefs)
sawtools prepare-wave docs/IMPL/IMPL-feature.yaml --wave 1 --repo-dir /path/to/repo

# After agents complete, finalize the wave (gates + merge + verify)
sawtools finalize-wave /abs/path/to/IMPL-feature.yaml --wave 1 --repo-dir /path/to/repo
```

**Using the engine programmatically:**

```go
import "github.com/blackwell-systems/scout-and-wave-go/pkg/engine"

opts := engine.RunScoutOpts{
    Prompt:     "Add rate limiting to the API",
    RepoPath:   "/path/to/repo",
    ScoutModel: "claude-sonnet-4-6",
}
manifestPath, err := engine.RunScout(ctx, opts)
```

---

## Architecture

Three repositories with separation of concerns:

| Repository | Purpose |
|-----------|---------|
| [scout-and-wave](https://github.com/blackwell-systems/scout-and-wave) | Protocol specification: invariants (I1–I6), execution rules (E1–E47), agent prompts, `/saw` skill |
| **scout-and-wave-go** (this repo) | Go engine + Protocol SDK + `sawtools` CLI |
| [scout-and-wave-web](https://github.com/blackwell-systems/scout-and-wave-web) | Web UI + HTTP/SSE server (imports this engine) |

The protocol repo is the source of truth for semantics. This repo implements them. The web repo provides the UI layer.

**Design principle:** The SDK handles data deterministically. The engine handles execution. Validation happens at every boundary between the two. Structural operations (manifest parsing, invariant validation, file ownership, wave sequencing) are Go code — pure functions, no LLM, same output for the same input. Creative operations (codebase analysis, implementation, novel error handling) are LLM work, delegated to the appropriate agent type.

---

## Development

```bash
go build ./...
go test ./...
golangci-lint run
```

---

## License

MIT
