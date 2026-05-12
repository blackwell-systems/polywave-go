# Binary Architecture: polywave-tools vs polywave

## Do Not Merge These Binaries

Polywave ships **two separate binaries** from **two separate repositories**. This is a deliberate architectural decision, not an accident or technical debt.

**Why two binaries exist:**

1. **polywave-tools** is a protocol engine CLI. It exposes every low-level protocol operation as a command. Its consumers are the `/polywave` skill (Claude Code orchestrator), CI/CD pipelines, and the Go SDK. It has zero web dependencies -- no embedded assets, no HTTP server, no React. Adding a web dependency would bloat every CI job and skill invocation with 4+ MB of unused React bundle.

2. **polywave-web** is a web application binary. It embeds a full React UI via `//go:embed` and runs an HTTP server with ~112 API endpoints. Its consumers are developers using the web UI to run Scout/Wave/Merge workflows interactively. It imports `pkg/engine` and `pkg/protocol` from polywave-go as Go libraries -- it never shells out to polywave-tools.

3. **They share the Go engine, not each other.** Both binaries depend on `polywave-go/pkg/` packages. Neither binary depends on the other. The web app does not call polywave-tools. polywave-tools does not know the web app exists.

Merging them would mean: every `/polywave` skill invocation and CI pipeline embeds an unused React bundle; every web app user gets 77 CLI commands they never run; the web app repo loses independent deployability. There is no benefit.

---

## Overview

| Binary | Source Repo | Commands | Size | Purpose |
|--------|-------------|----------|------|---------|
| **polywave-tools** | polywave-go | 77 | ~26 MB | Protocol engine CLI for skill/CI/SDK consumers |
| **polywave-web** | polywave-web | 23 | ~24 MB (includes embedded React) | Web application binary with HTTP API |

---

## polywave-tools (Protocol Engine CLI)

**Source:** `polywave-go/cmd/polywave-tools`

**Installation:**
```bash
# Homebrew (macOS/Linux)
brew install blackwell-systems/tap/polywave-tools

# Or via Go install
go install github.com/blackwell-systems/polywave-go/cmd/polywave-tools@latest
```

<details>
<summary>Build from source</summary>

```bash
cd polywave-go
go build -o polywave-tools ./cmd/polywave-tools
cp polywave-tools ~/.local/bin/polywave-tools
```
</details>

**Target audience:**
- **CLI Orchestrators** (the `/polywave` skill in Claude Code) -- need `create-worktrees`, `merge-agents`, `prepare-wave`, `finalize-wave` because they cannot import Go packages
- **CI/CD Pipelines** -- validation, quality gates, conflict detection, build verification
- **Power Users** -- dependency solver, journal debugging, protocol-level operations
- **Program Execution** -- multi-IMPL orchestration via `program-execute`, `finalize-tier`, `tier-gate`

### Commands (77)

**Worktree & Isolation:**
- `create-worktrees` -- create git worktrees for all agents in a wave
- `verify-isolation` -- verify agent is running in correct isolated worktree (E12)
- `cleanup` -- remove worktrees and branches after merge
- `cleanup-stale` -- detect and remove stale Polywave worktrees
- `verify-hook-installed` -- verify pre-commit hook is installed in worktree

**Merge & Build Verification:**
- `merge-agents` -- merge all agent branches for a wave
- `verify-commits` -- verify each agent branch has commits (I5)
- `verify-build` -- run test and lint commands from manifest
- `diagnose-build-failure` -- pattern-match build errors and suggest fixes

**Validation:**
- `validate` -- validate YAML IMPL manifest against protocol invariants
- `validate-program` -- validate PROGRAM manifest against schema rules
- `validate-scaffold` -- validate a single scaffold file before committing
- `validate-scaffolds` -- validate that all scaffold files are committed
- `validate-integration` -- validate integration gaps after wave completion
- `freeze-check` -- check manifest for freeze violations
- `freeze-contracts` -- freeze program contracts at a tier boundary
- `check-conflicts` -- detect file ownership conflicts in completion reports
- `check-deps` -- detect dependency conflicts before wave execution
- `check-impl-conflicts` -- check file ownership conflicts across IMPL docs
- `check-program-conflicts` -- detect file ownership conflicts across IMPLs in a program tier
- `check-type-collisions` -- detect type name collisions across agent branches

**Quality Gates:**
- `run-gates` -- run quality gates from manifest
- `run-review` -- run AI code review on current diff (post-merge gate)
- `run-critic` -- run critic agent to review agent briefs (E37)
- `scan-stubs` -- scan files for stub/TODO patterns (E20)
- `tier-gate` -- verify tier gate: all IMPLs complete + quality gates
- `pre-wave-gate` -- run pre-wave readiness checks on an IMPL manifest

**Batching / Lifecycle:**
- `run-scout` -- automated Scout execution with validation and ID correction (I3)
- `prepare-wave` -- prepare all agents in a wave (deps + worktrees + briefs + journals)
- `prepare-agent` -- prepare single agent environment (extract brief, init journal)
- `finalize-wave` -- finalize wave: verify, scan stubs, gates, merge, build, cleanup
- `finalize-impl` -- finalize IMPL doc: validate, populate gates, validate again
- `finalize-tier` -- finalize program tier: merge all IMPL branches and run tier gate
- `run-wave` -- execute full wave lifecycle (create, verify, merge, build, cleanup)
- `close-impl` -- close IMPL: mark complete, update CONTEXT.md, archive, and clean worktrees

**Manifest Operations:**
- `set-completion` -- set completion report for an agent
- `check-completion` -- check if an agent has a completion report in the manifest
- `mark-complete` -- write completion marker and archive to `complete/`
- `update-agent-prompt` -- update an agent's prompt/task in manifest
- `update-status` -- update agent status in manifest
- `update-context` -- update project CONTEXT.md (E18)
- `set-critic-review` -- write critic review result to IMPL doc
- `set-impl-state` -- atomically transition IMPL manifest to new protocol state
- `amend-impl` -- amend a living IMPL doc (add wave, redirect agent, extend scope)
- `populate-integration-checklist` -- auto-generate post_merge_checklist (M5)

**Context & Extraction:**
- `extract-context` -- extract per-agent context payload from manifest (E23)
- `extract-commands` -- extract build/test/lint/format commands from CI configs
- `build-retry-context` -- build structured retry context for a failed agent

**Journal Operations:**
- `journal-init` -- initialize journal directory structure for a wave agent
- `journal-context` -- generate context.md from journal entries for agent recovery
- `debug-journal` -- inspect journal contents for debugging failed agents

**Analysis & Detection:**
- `analyze-deps` -- analyze Go file dependencies and compute wave structure
- `analyze-suitability` -- scan codebase for pre-implemented requirements
- `detect-scaffolds` -- detect shared types that should be extracted to scaffold files
- `detect-cascades` -- detect files affected by type renames
- `solve` -- compute optimal wave assignments from dependency declarations
- `assign-agent-ids` -- generate agent IDs following the `^[A-Z][2-9]?$` pattern

**Program Operations:**
- `create-program` -- auto-generate PROGRAM manifest from existing IMPL docs
- `create-program-worktrees` -- create IMPL branches and worktrees for all IMPLs in a program tier
- `import-impls` -- import existing IMPL docs into a PROGRAM manifest
- `program-execute` -- execute PROGRAM manifest through the tier loop
- `program-replan` -- re-engage Planner agent to revise a PROGRAM manifest
- `program-status` -- display full program status report
- `validate-program` -- validate PROGRAM manifest against schema rules
- `list-programs` -- list PROGRAM manifests in a directory
- `mark-program-complete` -- mark PROGRAM as complete and update CONTEXT.md
- `prepare-tier` -- prepare a program tier: check conflicts, validate IMPLs, create branches
- `update-program-impl` -- update the status of a specific IMPL entry in a PROGRAM manifest
- `update-program-state` -- update the state field of a PROGRAM manifest

**Queue Operations:**
- `queue` -- manage IMPL queue (parent command)
  - `queue add` -- add an item to the IMPL queue
  - `queue list` -- list all IMPL queue items
  - `queue next` -- return the next eligible IMPL queue item

**Discovery & Observability:**
- `list-impls` -- list IMPL manifests in a directory
- `metrics` -- show metrics for an IMPL (cost, duration, success rate)
- `query` -- query observability data
  - `query events` -- query observability events
- `resume-detect` -- detect interrupted Polywave sessions in the repository

**Recovery:**
- `retry` -- generate retry IMPL doc for a failed quality gate (E24)
- `interview` -- conduct a structured requirements interview

**Daemon:**
- `daemon` -- run the Polywave daemon loop (processes queue items continuously)

**System:**
- `verify-install` -- check that all Polywave prerequisites are met

---

## polywave (Web Application)

**Source:** `polywave-web/cmd/polywave-web`

**Installation:**
```bash
cd polywave-web
make build  # or: cd web && npm run build && cd .. && go build -o polywave-web ./cmd/polywave-web
./polywave-web serve
```

**Target audience:**
- **Feature Developers** -- use web UI to run Scout, review IMPLs, execute waves, merge
- **Code Reviewers** -- approve/reject IMPL docs, view diffs, run critic reviews
- **Team Leads** -- monitor wave execution, manage worktrees, view observability dashboards

### Commands (23)

**Web Server:**
- `serve` -- start HTTP server on port 7432 (the primary command; ~112 API endpoints + embedded React UI)

**High-Level Orchestration:**
- `scout` -- run a Scout agent to generate an IMPL doc
- `scaffold` -- run a Scaffold agent to set up worktrees
- `wave` -- execute agents for a wave
- `merge` -- merge agent worktrees for a completed wave
- `merge-wave` -- check if a wave is ready to merge (outputs JSON status)
- `current-wave` -- return the wave number of the first incomplete wave
- `status` -- show current wave/agent status from an IMPL doc

**Format Conversion:**
- `render` -- render YAML IMPL manifest as markdown

**Shared with polywave-tools** (14 commands):
- `validate`, `extract-context`, `set-completion`, `mark-complete`
- `run-gates`, `check-conflicts`, `validate-scaffolds`, `freeze-check`
- `update-agent-prompt`, `analyze-deps`, `analyze-suitability`
- `detect-cascades`, `detect-scaffolds`, `extract-commands`

---

## Architecture: How They Relate

```
┌──────────────────────────────────────────────────────────────┐
│ polywave-go (SDK/Engine Repo)                          │
│                                                               │
│  pkg/engine/      ◄──┐  Scout, Wave, Merge execution         │
│  pkg/protocol/    ◄──┤  YAML parsing, validation             │
│  pkg/agent/       ◄──┤  Claude API client, backends          │
│  internal/git/    ◄──┤  Git helpers                          │
│                       │                                       │
│  cmd/polywave-tools/  ────► polywave-tools (77 commands)                      │
│                   Protocol engine CLI                         │
└───────────────────────┬──────────────────────────────────────┘
                        │
                        │ Go package imports (library dependency)
                        │ NOT a binary dependency — polywave never calls polywave-tools
                        │
┌───────────────────────▼──────────────────────────────────────┐
│ polywave-web (Web App Repo)                             │
│                                                               │
│  pkg/api/         HTTP server, SSE broker, routes            │
│    │                                                          │
│    ├─► Imports: polywave-go/pkg/engine                 │
│    ├─► Imports: polywave-go/pkg/protocol               │
│    ├─► Imports: polywave-go/pkg/{agent,analyzer,       │
│    │       autonomy,codereview,commands,config,gatecache,     │
│    │       interview,journal,observability,queue,result,      │
│    │       resume,retryctx,scaffold,suitability}             │
│    │                                                          │
│    └─► ~112 API endpoints (direct Go function calls)         │
│                                                               │
│  web/             React UI (embedded in binary via go:embed) │
│                                                               │
│  cmd/polywave-web/ ──► polywave-web (23 commands)                      │
│                   Web UI + orchestration CLI                  │
└──────────────────────────────────────────────────────────────┘
```

**The web app imports `pkg/engine` and `pkg/protocol` as Go packages. It does NOT shell out to polywave-tools. The two binaries are independent.**

---

## Command Overlap

**14 commands exist in both binaries:**

`analyze-deps`, `analyze-suitability`, `check-conflicts`, `detect-cascades`, `detect-scaffolds`, `extract-commands`, `extract-context`, `freeze-check`, `mark-complete`, `run-gates`, `set-completion`, `update-agent-prompt`, `validate`, `validate-scaffolds`

**Why the overlap?** Both tools need these operations. polywave-tools exposes them for CI/CD and CLI orchestration. polywave exposes them as convenience commands for users who also have the web UI. Both implementations call the same underlying `pkg/` functions.

**63 commands are polywave-tools-only** (protocol engine, worktree management, program execution, journals, daemon, recovery, queue). These are the operations that CLI orchestrators and CI pipelines need but web UI users do not run manually.

**9 commands are polywave-web-only** (serve, scout, scaffold, wave, merge, merge-wave, current-wave, status, render). These are high-level orchestration commands and the web server itself.

---

## Execution Models

### CLI Orchestration (uses polywave-tools)
**Context:** Inside a Claude Code session (Max plan or Bedrock)

1. Orchestrator (Claude via `/polywave` skill) launches agents via Agent tool
2. Orchestrator calls `polywave-tools prepare-wave` to set up worktrees, briefs, journals
3. Agents run in isolated worktrees
4. Orchestrator calls `polywave-tools finalize-wave` to merge, verify, clean up

**Why polywave-tools?** The orchestrator is a running LLM session. It cannot import Go packages, so it needs CLI commands.

### Web Orchestration (uses polywave-web)
**Context:** User in browser at `localhost:7432`

1. User clicks "Run Scout" in web UI
2. Web app calls `engine.RunScout()` (direct Go function call)
3. Web app calls `engine.RunWave()` to execute agents
4. Results streamed to UI via SSE (~112 API endpoints)

**Why polywave-web?** The web app is a Go application. It imports the engine as a library. No CLI shelling-out needed.

---

## When to Use Which

### Use polywave-tools when:
- Orchestrating from CLI (e.g., `/polywave` skill in Claude Code)
- Running in CI/CD pipelines
- Executing multi-IMPL programs (`program-execute`, `finalize-tier`)
- Debugging protocol internals (dependency solver, journal inspection, stub scanning)
- Need low-level worktree or merge operations
- Cannot import Go packages

### Use polywave when:
- Want interactive web UI for IMPL review and wave monitoring
- Need real-time SSE event streaming
- Running as a local HTTP server
- Building workflows around the HTTP API
- Want a single-binary deployable with embedded UI

---

## Installation

**For end users:**
```bash
git clone https://github.com/blackwell-systems/polywave-web.git
cd polywave-web
make build
./polywave serve
```

**For power users / CI/CD:**
```bash
git clone https://github.com/blackwell-systems/polywave-go.git
cd polywave-go
go build -o polywave-tools ./cmd/polywave-tools
cp polywave-tools ~/.local/bin/polywave-tools
```

**For developers:** Install both. Use `polywave-web serve` for the development workflow. Use `polywave-tools` for testing protocol operations.

---

## FAQ

**Q: Why not merge them into one binary?**
A: Because they serve different execution models with different dependency profiles. polywave-tools has zero web dependencies (no embedded React, no HTTP server). Merging would add ~4 MB of unused web assets to every CI job and skill invocation, bloat the command list for web users with 63 commands they never run, and couple the web app's release cycle to the protocol engine's. The shared code lives in `pkg/` -- that is the integration point, not a combined binary.

**Q: Does the web app shell out to polywave-tools?**
A: No. The web app imports `pkg/engine` and `pkg/protocol` as Go packages. It calls Go functions directly. The only external commands it runs are git and user-specified test/lint commands.

**Q: Why is polywave larger than polywave-tools?**
A: Despite having fewer commands, polywave embeds the entire React UI via `//go:embed all:dist`, adding ~4 MB of web assets to the binary.

**Q: Which binary should I use for the `/polywave` skill?**
A: polywave-tools. The skill orchestrates from CLI and needs commands like `prepare-wave`, `finalize-wave`, and `merge-agents`.

**Q: Can I use polywave for CI/CD?**
A: You would be missing 63 commands including `prepare-wave`, `finalize-wave`, `solve`, `verify-commits`, `verify-build`, `scan-stubs`, and all program execution commands. Use polywave-tools.

---

*Last reviewed: 2026-03-24*
