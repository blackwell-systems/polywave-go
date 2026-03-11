# Binary Architecture: saw vs sawtools

Scout-and-Wave provides **two separate binaries** from **two separate repositories**. They serve different purposes and target different users.

## Overview

| Binary | Source Repo | Purpose | Primary Users |
|--------|-------------|---------|---------------|
| **sawtools** | scout-and-wave-go | Protocol SDK toolkit | CLI orchestrators, CI/CD, power users |
| **saw** | scout-and-wave-web | Web UI + orchestration | Feature developers, code reviewers |

---

## sawtools (SDK Toolkit)

**Source:** `scout-and-wave-go/cmd/saw` → renamed to `sawtools` at install time

**Size:** 11 MB

**Installation:**
```bash
cd scout-and-wave-go
go build -o sawtools ./cmd/saw
cp sawtools ~/.local/bin/sawtools
```

**Purpose:**
- Full protocol SDK command-line toolkit
- Operator utilities for CI/CD pipelines
- Low-level protocol operations

**Commands (23):**
- Worktree operations: `create-worktrees`, `verify-isolation`, `cleanup`
- Merge operations: `merge-agents`, `verify-commits`, `verify-build`
- Protocol operations: `validate`, `extract-context`, `solve` (dependency solver)
- Quality gates: `run-gates`, `check-conflicts`, `freeze-check`, `validate-scaffolds`
- Manifest operations: `set-completion`, `mark-complete`, `update-agent-prompt`, `update-status`, `update-context`
- Journal operations: `debug-journal`, `journal-init`, `journal-context`
- Wave execution: `run-wave` (programmatic orchestration)
- Discovery: `list-impls`, `scan-stubs`

**Target Audience:**
1. **CLI Orchestrators** (e.g., `/saw` skill in Claude Code) — need `create-worktrees`, `merge-agents`, `verify-commits` because they can't import Go packages
2. **CI/CD Pipelines** — automated validation, quality gates, conflict detection
3. **Power Users** — dependency solver, journal debugging, protocol-level operations

**Key Distinction:** sawtools provides protocol-level operations that orchestrators need when they can't directly import the Go SDK.

---

## saw (Web UI + Orchestration)

**Source:** `scout-and-wave-web/cmd/saw`

**Size:** 20 MB (includes embedded React bundle via `//go:embed all:dist`)

**Installation:**
```bash
cd scout-and-wave-web
make build  # or: go build -o saw ./cmd/saw
./saw serve
```

**Purpose:**
- Interactive web UI for IMPL review and wave monitoring
- HTTP API (42 endpoints) for programmatic access
- High-level orchestration commands

**Primary Feature:** `serve` command (HTTP server on port 7432)

**Commands (18):**
- **Web server:** `serve` (THE killer feature)
- **Orchestration:** `scout`, `scaffold`, `wave`, `merge`, `merge-wave`, `current-wave`, `status`
- **Format conversion:** `render` (YAML→Markdown), `migrate` (Markdown→YAML)
- **Validation:** `validate`, `check-conflicts`, `run-gates`, `freeze-check`, `validate-scaffolds`
- **Manifest operations:** `extract-context`, `set-completion`, `mark-complete`, `update-agent-prompt`

**Target Audience:**
1. **Feature Developers** — use web UI to run Scout → Wave → Merge workflow
2. **Code Reviewers** — approve/reject IMPL docs via web UI
3. **Team Leads** — monitor wave execution, manage worktrees

**Key Distinction:** saw is a user-facing application with an embedded web UI. Users never run low-level protocol commands manually.

---

## Architecture: How They Relate

```
┌─────────────────────────────────────────────────────────┐
│ scout-and-wave-go (SDK/Engine Repo)                     │
│                                                          │
│  pkg/engine/      ◄──┐  Scout, Wave, Merge execution    │
│  pkg/protocol/    ◄──┤  YAML parsing, validation        │
│  pkg/agent/       ◄──┤  Claude API client, backends     │
│  internal/git/    ◄──┤  Git helpers                     │
│                      │                                   │
│  cmd/saw/  ────► sawtools (23 commands)                 │
│                   Protocol toolkit binary                │
└──────────────────────┬───────────────────────────────────┘
                       │
                       │ Go package imports
                       │
┌──────────────────────▼───────────────────────────────────┐
│ scout-and-wave-web (Web App Repo)                        │
│                                                           │
│  pkg/api/         HTTP server, SSE broker, routes        │
│    │                                                      │
│    ├─► Imports: scout-and-wave-go/pkg/engine            │
│    ├─► Imports: scout-and-wave-go/pkg/protocol          │
│    │                                                      │
│    └─► 42 API endpoints (direct Go function calls)      │
│                                                           │
│  web/             React UI (embedded in binary)          │
│                                                           │
│  cmd/saw/  ────► saw (18 commands)                       │
│                   Orchestration CLI + web UI             │
└───────────────────────────────────────────────────────────┘
```

**Important:** The web app (`saw`) imports `pkg/engine` and `pkg/protocol` as **Go packages**. It does NOT shell out to the `sawtools` binary. The two binaries are independent.

---

## Command Overlap

**9-11 commands exist in BOTH binaries:**
- `validate`, `extract-context`, `set-completion`, `mark-complete`
- `run-gates`, `check-conflicts`, `validate-scaffolds`, `freeze-check`, `update-agent-prompt`

**Why the overlap?**
Both tools need these operations:
- sawtools: For CI/CD validation pipelines
- saw: For web UI validation panels

This is intentional duplication, not a design flaw.

---

## Execution Models

### CLI Orchestration (uses sawtools)
**Context:** Inside Claude Code session (Max plan or Bedrock)

**How it works:**
1. Orchestrator (Claude) launches agents via Agent tool
2. Orchestrator calls `sawtools create-worktrees` to set up git isolation
3. Agents run in worktrees
4. Orchestrator calls `sawtools merge-agents` to merge results

**Why sawtools?** The orchestrator is a running LLM session. It can't import Go packages, so it needs CLI commands.

### Programmatic Orchestration (uses saw web app)
**Context:** User in web UI or native app

**How it works:**
1. User clicks "Run Scout" in web UI
2. Web app calls `engine.RunScout()` (direct Go function call)
3. Web app calls `engine.RunWave()` to execute agents
4. Results streamed to UI via SSE

**Why saw?** The web app is a Go application. It imports the engine as a library. No CLI commands needed.

---

## When to Use Which

### Use sawtools when:
- Orchestrating from CLI (e.g., `/saw` skill in Claude Code)
- Running in CI/CD pipelines
- Debugging protocol operations (dependency solver, journal inspection)
- Need low-level worktree operations
- Can't import Go packages

### Use saw when:
- Want interactive web UI for IMPL review
- Need real-time wave execution monitoring
- Running as a local HTTP server
- Building workflows around the HTTP API
- Want a single-binary deployable with embedded UI

---

## Installation Recommendations

**For end users:** Install `saw` from scout-and-wave-web
```bash
git clone https://github.com/blackwell-systems/scout-and-wave-web.git
cd scout-and-wave-web
make build
./saw serve
```

**For power users / CI/CD:** Install `sawtools` from scout-and-wave-go
```bash
git clone https://github.com/blackwell-systems/scout-and-wave-go.git
cd scout-and-wave-go
go build -o sawtools ./cmd/saw
cp sawtools ~/.local/bin/sawtools
```

**For developers:** Install both
- Use `saw serve` for development workflow
- Use `sawtools` for testing protocol operations

---

## FAQ

**Q: Why are there two binaries?**
A: They serve different execution models. sawtools is for CLI orchestration (when you can't import Go packages). saw is for end users who want a web UI.

**Q: Can I use saw for CI/CD?**
A: Yes, but you'll be missing toolkit commands like `solve`, `debug-journal`, and `verify-isolation`. Use sawtools for full protocol operations.

**Q: Does the web app shell out to sawtools?**
A: No. The web app imports `pkg/engine` and `pkg/protocol` as Go packages. It never executes CLI commands (except for git queries and user test commands).

**Q: Why is saw larger (20MB vs 11MB)?**
A: saw embeds the entire React UI via `//go:embed all:dist`. This makes it a single-file deployable.

**Q: Which binary should I use for the `/saw` skill?**
A: sawtools. The skill orchestrates from CLI and needs commands like `create-worktrees` and `merge-agents`.
