# scout-and-wave-go

Go engine and Protocol SDK for the Scout-and-Wave parallel agent coordination system.

## Overview

`scout-and-wave-go` provides two layers:

1. **Protocol SDK** (`pkg/protocol`) — Deterministic data operations for YAML manifests. Types, validation, invariant enforcement. Pure Go, no runtime dependencies. Importable by any tool that needs to read, write, or validate SAW protocol state.

2. **Engine** (`pkg/engine`, `pkg/orchestrator`) — Agent execution runtime, backend abstraction, wave orchestration. Shells out to LLM providers (Anthropic, Bedrock, OpenAI-compatible) for the creative work; uses the Protocol SDK for all structural operations.

**Architecture principle:** The SDK handles data deterministically. The engine handles execution. Validation happens at every boundary between the two.

```
┌─────────────────────────────────────────────┐
│  Orchestrator (Claude via skill, or CLI)    │
│  Decides what to do, handles errors         │
├─────────────────────────────────────────────┤
│  CLI Binary (saw validate, saw extract...)  │
│  Thin wrappers — deterministic I/O          │
├─────────────────────────────────────────────┤
│  Protocol SDK (pkg/protocol)                │
│  Types, validation, invariants              │
│  Pure Go — no LLM, no runtime dependency    │
├─────────────────────────────────────────────┤
│  Agent Execution (Runtime interface)        │
│  LLM providers, tool dispatch, context      │
│  Anthropic SDK / Bedrock / OpenAI           │
└─────────────────────────────────────────────┘
```

## Installation

```bash
go get github.com/blackwell-systems/scout-and-wave-go
```

## Quick Start

### Protocol SDK (data operations)

```go
import "github.com/blackwell-systems/scout-and-wave-go/pkg/protocol"

// Load YAML manifest
manifest, err := protocol.Load("docs/IMPL/IMPL-feature.yaml")

// Validate invariants (I1-I6)
errors := protocol.Validate(manifest)
for _, e := range errors {
    fmt.Printf("%s: %s (field: %s)\n", e.Code, e.Message, e.Field)
}

// Query protocol state
wave := protocol.CurrentWave(manifest)
fmt.Printf("Current wave: %d (%d agents)\n", wave.Number, len(wave.Agents))

// Register agent completion
report := protocol.CompletionReport{
    Status:       "complete",
    Branch:       "wave1-agent-A",
    Commit:       "abc123",
    FilesCreated: []string{"pkg/protocol/manifest.go"},
}
protocol.SetCompletionReport(manifest, "A", report)
protocol.Save(manifest, "docs/IMPL/IMPL-feature.yaml")
```

### Engine (agent execution)

```go
import (
    "github.com/blackwell-systems/scout-and-wave-go/pkg/engine"
    "github.com/blackwell-systems/scout-and-wave-go/pkg/orchestrator"
)

// Run Scout to generate YAML manifest
opts := engine.RunScoutOpts{
    Prompt:     "Add user authentication to the API",
    RepoPath:   "/path/to/repo",
    ScoutModel: "claude-sonnet-4-6",
}
manifestPath, err := engine.RunScout(ctx, opts)

// Orchestrate wave execution
orch := orchestrator.New(orchestrator.Config{
    RepoPath:     "/path/to/repo",
    ManifestPath: manifestPath,
})
err = orch.StartWave(ctx, 1)
```

### Using the `saw` server

See [scout-and-wave-web](https://github.com/blackwell-systems/scout-and-wave-web) for the web UI and HTTP server.

## Package Structure

```
pkg/
├── protocol/       # Protocol SDK — the importable core
│   ├── types.go        # IMPLManifest, Wave, Agent, FileOwnership, etc.
│   ├── manifest.go     # Load, Save, CurrentWave, SetCompletionReport
│   └── validation.go   # I1-I6 invariant enforcement, structured errors
├── agent/          # Agent execution runtime, tool system
├── engine/         # High-level entrypoints (RunScout, RunWave, Chat)
├── orchestrator/   # Wave orchestration, SSE events, verification
└── worktree/       # Git worktree management

internal/
└── git/            # Git operations (commit, branch, merge)
```

### What lives where

| Concern | Package | Deterministic? |
|---------|---------|---------------|
| Manifest types & YAML I/O | `pkg/protocol` | Yes |
| Invariant validation (I1-I6) | `pkg/protocol` | Yes |
| Agent context extraction | `pkg/protocol` | Yes |
| LLM conversation loops | `pkg/agent` | No (LLM) |
| Backend provider routing | `pkg/agent` | No (network) |
| Worktree creation & merge | `pkg/worktree` | Yes (git ops) |
| Wave lifecycle & SSE | `pkg/orchestrator` | Mixed |

## CLI Commands

The `saw` binary wraps SDK operations as shell commands. Each command is single-purpose with structured I/O:

| Command | Input | Output | Exit Code |
|---------|-------|--------|-----------|
| `saw validate <manifest>` | YAML path | Errors (JSON) | 0=valid, 1=invalid |
| `saw extract-context <manifest> <agent>` | Manifest + agent ID | Agent context (JSON) | 0=ok, 1=not found |
| `saw current-wave <manifest>` | YAML path | Wave number | 0=ok, 1=no pending |
| `saw set-completion <manifest> <agent>` | Manifest + stdin (YAML) | Success | 0=ok, 1=failed |
| `saw merge-wave <manifest> <wave>` | Manifest + wave number | Merge status | 0=ok, 1=conflicts |
| `saw render <manifest>` | YAML path | Markdown | 0=ok, 1=failed |
| `saw migrate <impl.md>` | Markdown path | YAML path | 0=ok, 1=failed |

## Design Principles

### Structured data, interactive coordination

The protocol has two halves:

- **Structural work** (deterministic) — manifest parsing, invariant validation, file ownership, wave sequencing. This is Go code. It never calls an LLM. It always produces the same output for the same input.
- **Creative work** (non-deterministic) — analyzing code, writing implementations, handling novel errors, deciding what to do next. This is LLM work. It requires conversation, judgment, and context.

The SDK handles the first. The agent runtime handles the second. The CLI binary sits at the boundary, validating data on the way in and the way out.

### Invariants enforced by code

The SAW protocol defines six invariants. Before the SDK, these were enforced by bash regex and human review. Now they're Go functions:

| Invariant | Rule | Enforcement |
|-----------|------|-------------|
| **I1** | No two agents own the same file in a wave | `Validate()` checks ownership table |
| **I2** | Interface contracts defined before agents launch | Scaffold files committed before worktrees created |
| **I3** | Wave N+1 waits for Wave N merge | `CurrentWave()` returns first incomplete wave |
| **I4** | IMPL manifest is single source of truth | All state read/written via SDK operations |
| **I5** | Agents commit before reporting | `SetCompletionReport()` requires commit hash |
| **I6** | Orchestrator doesn't do agent work | Behavioral (not checkable by code) |

### Validation at boundaries

Every transition between layers validates:

```
Manifest loaded from disk       → Validate()
Agent context extracted          → agent exists, wave is current
Completion report registered     → required fields, status enum, files match ownership
Wave merge requested             → all agents complete, no I1 violations
```

Errors are structured (`ValidationError` with code, message, field) — not "parse error on line 342."

### Why not a framework?

We evaluated Google ADK, Claude Agent SDK, GoAgents, and others (see [Framework Evaluation](docs/proposals/protocol-sdk-migration-v2.md#appendix-framework-evaluation)). The conclusion:

**SAW's value is the coordination protocol** — wave sequencing, disjoint file ownership, interface contracts, merge verification. No framework provides these. Agent execution (LLM conversation loops, tool dispatch) is a commodity — delegated to whatever runtime fits the deployment context.

The SDK defines a `Runtime` interface so the execution backend is swappable:
- **Phase 1:** Claude Code subagents (current model)
- **Future:** Claude Agent SDK, Google ADK, direct API, or any LLM provider

## Documentation

- **[Protocol SDK Migration](docs/proposals/protocol-sdk-migration-v2.md)** — Why YAML manifests replace markdown, architectural decisions, framework evaluation
- **[Architecture Overview](docs/architecture.md)** — Engine flow, package relationships
- **[Backends](docs/backends.md)** — Implementing custom LLM backends
- **[Orchestration](docs/orchestration.md)** — Wave lifecycle, worktree management, merge procedure

## Development

### Build

```bash
go build ./...
```

### Test

```bash
go test ./...
```

### Lint

```bash
golangci-lint run
```

## Related Repositories

| Repository | Purpose |
|-----------|---------|
| [scout-and-wave](https://github.com/blackwell-systems/scout-and-wave) | Protocol specification, skills, prompts |
| [scout-and-wave-web](https://github.com/blackwell-systems/scout-and-wave-web) | Web UI + HTTP server (imports this engine) |

## Protocol Specification

The Scout-and-Wave protocol specification lives in the [scout-and-wave](https://github.com/blackwell-systems/scout-and-wave) repository.

## License

MIT
