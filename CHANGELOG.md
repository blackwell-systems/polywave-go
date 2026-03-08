# Changelog

All notable changes to the Scout-and-Wave Go engine will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/).

## Version History

| Version | Date | Headline |
|---------|------|----------|
| [0.2.0] | 2026-03-08 | Engine protocol parity — E17–E23 implemented (context memory, failure routing, stub scan, quality gates, scaffold build verify, per-agent context extraction) |
| [0.1.0] | 2026-03-08 | Initial engine extraction — parser, orchestrator, agent runner, git, worktree management |

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
