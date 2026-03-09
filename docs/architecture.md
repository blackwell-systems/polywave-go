# Architecture Overview

## High-Level Flow

```
User Request
    ↓
RunScout (pkg/engine/runner.go)
    → Backend.RunStreaming with Scout prompt
    → Writes IMPL doc to docs/IMPL/
    ↓
StartWave (pkg/engine/runner.go)
    → Orchestrator.StartWave (pkg/orchestrator/orchestrator.go)
    → Per-agent: worktree checkout, agent runner launch
    → Agent.ExecuteStreaming (pkg/agent/runner.go)
        → Backend.RunStreamingWithTools (tool call loop)
        → Writes completion report to IMPL doc
    → Quality gates execution
    → Merge to main
    ↓
Verification (pkg/orchestrator/orchestrator.go)
    → Build verification
    → Test suite
    → Protocol invariants
```

## Package Responsibilities

### `pkg/engine`

**Purpose:** High-level entrypoints for Scout, Wave, Scaffold, and Chat operations.

**Key files:**
- `runner.go` — `RunScout`, `StartWave`, `RunScaffold`
- `chat.go` — `RunChat` (standalone chat without IMPL doc)

**Dependencies:** Uses `pkg/agent`, `pkg/orchestrator`, `pkg/protocol`

### `pkg/agent`

**Purpose:** Agent execution runtime. Handles tool system, backend abstraction, and agent-tool-LLM loop.

**Key files:**
- `runner.go` — `ExecuteStreaming` (main agent loop)
- `tools.go` — Tool definitions (Read, Write, Edit, Bash, Glob, Grep)
- `backend/` — Backend implementations

**Tool Call Loop:**
1. Backend sends messages + tools to LLM
2. LLM responds with text or tool_use
3. If tool_use: execute tool, append result, loop
4. If text: return final answer

### `pkg/orchestrator`

**Purpose:** Wave orchestration, git worktree management, SSE event publishing, verification pipeline.

**Key files:**
- `orchestrator.go` — Wave lifecycle, agent launching, merge
- `sse.go` — Event broker and publishing
- `verification.go` — Build + test verification
- `quality_gates.go` — Custom quality gate execution
- `failure.go` — Failure type routing (E19)
- `context.go` — Project memory (CONTEXT.md) updates

**Event Flow:**
- `wave_started` → `agent_started` → `agent_output` (chunks) → `agent_completed` → `wave_completed`
- Error path: `agent_blocked` with failure routing decision

### `pkg/protocol`

**Purpose:** IMPL doc parsing, validation, and extraction.

**Key files:**
- `parser.go` — `ParseIMPLDoc`, `ParseCompletionReport`, `ParseQualityGates`
- `validator.go` — Protocol invariant checks (I1–I6)
- `extract.go` — Per-agent context extraction (E23)

**Parser State Machine:**
- Line-by-line scan with section header detection
- Wave/agent structure: `## Wave N` → `### Agent X`
- Tables: File ownership, interface contracts
- YAML blocks: Completion reports
- Fenced blocks: Scaffolds

### `pkg/types`

**Purpose:** Shared protocol types used across packages.

**Key types:**
- `IMPLDoc` — Parsed IMPL document structure
- `Wave`, `Agent` — Wave/agent specifications
- `CompletionReport` — Agent completion report
- `FileOwnershipInfo` — File ownership table row
- `InterfaceContract` — Shared type/interface contract

### `pkg/worktree`

**Purpose:** Git worktree lifecycle management.

**Operations:**
- Create: `git worktree add .claude/worktrees/wave1-agent-A wave1-agent-A`
- Cleanup: `git worktree remove --force`
- Branch tracking: Associate worktrees with agent IDs

### `internal/git`

**Purpose:** Low-level git operations.

**Operations:**
- Commit, branch creation/deletion
- Merge (with conflict detection)
- Diff, status, log parsing

## Design Decisions

### Why Backend Abstraction?

The `backend.Backend` interface decouples the engine from specific LLM providers:

```go
type Backend interface {
    Run(ctx, system, user, model string) (string, error)
    RunStreaming(ctx, system, user, model string, onChunk ChunkCallback) (string, error)
    RunStreamingWithTools(ctx, system, user, model string, onChunk ChunkCallback, onToolCall ToolCallCallback) (string, error)
}
```

**Supported backends:**
- **API** (`pkg/agent/backend/api`) — Anthropic Messages API
- **Bedrock** (`pkg/agent/backend/bedrock`) — AWS Bedrock with AWS SDK v2
- **OpenAI-compatible** (`pkg/agent/backend/openai`) — OpenAI API, Groq, Ollama, LM Studio
- **CLI** (`pkg/agent/backend/cli`) — Claude Code CLI subprocess

Each backend serializes tools into its native format (Anthropic `tools` array, OpenAI `functions`, etc.) and deserializes responses back into the unified tool call loop.

### Why Worktrees?

Git worktrees provide true filesystem isolation for parallel agents:

- Each agent works in `.claude/worktrees/wave1-agent-A/` on branch `wave1-agent-A`
- No file locking needed — agents cannot conflict at the filesystem level
- Merge is explicit and auditable (main never receives uncommitted changes)
- Cleanup is safe even if agent crashes (worktree + branch are isolated)

### Why SSE for Events?

Server-Sent Events (SSE) provide real-time streaming with simple HTTP:

- One-way server → client (sufficient for read-only event monitoring)
- Automatic reconnection on disconnect
- Native browser EventSource API
- Lower overhead than WebSocket for unidirectional streams

Events are fire-and-forget — the orchestrator publishes, subscribers receive, no acknowledgment required.

### Why Line-by-Line Parser?

The protocol parser uses a line-by-line state machine rather than a grammar-based parser (e.g., PEG):

**Trade-off:** Brittle to format variations, but simple and debuggable.

**Mitigation:** Protocol spec enforces strict format. Future: structured output with Claude's `output_config` to bypass parser entirely for API backend (see ROADMAP.md).

## Cross-Repo Architecture

The engine is split across two repos:

- **scout-and-wave-go** (this repo) — Engine, protocol, orchestrator
- **scout-and-wave-web** — Web UI, HTTP server, `saw` CLI

`scout-and-wave-web` imports the engine as a Go module:

```go
import (
    "github.com/blackwell-systems/scout-and-wave-go/pkg/engine"
    "github.com/blackwell-systems/scout-and-wave-go/pkg/orchestrator"
)
```

The `saw serve` command in `scout-and-wave-web` wraps the engine with HTTP handlers and an SSE broker.

## Extension Points

1. **Custom Backends** — Implement `backend.Backend` for new LLM providers
2. **Custom Tools** — Register tools via `Workshop` (after refactoring, see ROADMAP.md)
3. **Middleware** — Wrap tool execution (logging, timing, validation, permissions)
4. **Quality Gates** — Add custom verification scripts to IMPL doc `## Quality Gates` section
5. **SSE Consumers** — Subscribe to orchestrator events for custom monitoring/logging

See `docs/backends.md` and `docs/tools.md` for implementation guides.
