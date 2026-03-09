# scout-and-wave-go

Go implementation of the Scout-and-Wave protocol engine.

## Overview

`scout-and-wave-go` is the core engine for orchestrating multi-agent parallel code generation using the Scout-and-Wave protocol. It provides:

- **Protocol parser** for IMPL documents (wave structure, agent assignments, completion reports)
- **Agent execution runtime** with tool support (Read, Write, Edit, Bash, Glob, Grep)
- **Backend abstraction** for multiple LLM providers (Anthropic API, AWS Bedrock, OpenAI-compatible, CLI)
- **Wave orchestrator** with git worktree isolation and SSE event streaming
- **Verification pipeline** (build gates, quality gates, invariant validation)

## Installation

```bash
go get github.com/blackwell-systems/scout-and-wave-go
```

## Quick Start

### As a library

```go
import (
    "github.com/blackwell-systems/scout-and-wave-go/pkg/engine"
    "github.com/blackwell-systems/scout-and-wave-go/pkg/orchestrator"
)

// Run Scout to generate IMPL doc
opts := engine.RunScoutOpts{
    Prompt:     "Add user authentication to the API",
    RepoPath:   "/path/to/repo",
    ScoutModel: "claude-sonnet-4-6",
}
implPath, err := engine.RunScout(ctx, opts)

// Parse IMPL doc
doc, err := protocol.ParseIMPLDoc(implPath)

// Run Wave 1
orch := orchestrator.New(orchestrator.Config{
    RepoPath: "/path/to/repo",
    IMPLPath: implPath,
})
err = orch.StartWave(ctx, 1)
```

### Using the `saw` server

See [scout-and-wave-web](https://github.com/blackwell-systems/scout-and-wave-web) for the web UI and HTTP server that wraps this engine.

## Documentation

- **[Architecture Overview](docs/architecture.md)** — Engine flow, package relationships, design decisions
- **[API Endpoints](docs/api-endpoints.md)** — HTTP API reference (implemented in scout-and-wave-web)
- **[SSE Events](docs/sse-events.md)** — Server-Sent Events schema and lifecycle
- **[Tool System](docs/tools.md)** — Tool architecture, registration, middleware, custom tools
- **[Backends](docs/backends.md)** — Implementing custom LLM backends
- **[Protocol Parsing](docs/protocol-parsing.md)** — IMPL doc format and parser internals
- **[Orchestration](docs/orchestration.md)** — Wave lifecycle, worktree management, merge procedure

## Examples

- **[Custom Backend](examples/custom-backend/)** — Implement `backend.Backend` for a new LLM provider
- **[Custom Tool](examples/custom-tool/)** — Register custom tools with middleware
- **[Library Usage](examples/library-usage/)** — Use the engine programmatically

## Package Structure

```
pkg/
├── agent/          # Agent execution runtime, tool system, backends
├── engine/         # High-level entrypoints (RunScout, RunWave, Chat)
├── orchestrator/   # Wave orchestration, SSE events, verification
├── protocol/       # IMPL doc parser and types
├── types/          # Shared protocol types
└── worktree/       # Git worktree management

internal/
└── git/            # Git operations (commit, branch, merge)
```

## Development

### Build

```bash
go build -o saw ./cmd/saw
```

### Test

```bash
go test ./...
```

### Lint

```bash
golangci-lint run
```

## Protocol Specification

The Scout-and-Wave protocol specification lives in the [scout-and-wave](https://github.com/blackwell-systems/scout-and-wave) repository. This implementation tracks protocol version **0.14.x**.

## License

MIT
