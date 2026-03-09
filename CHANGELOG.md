# Changelog

All notable changes to the Scout-and-Wave Go engine will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/).

## Version History

| Version | Date | Headline |
|---------|------|----------|
| [0.7.0] | 2026-03-09 | Local model shortcuts — `ollama:` and `lmstudio:` provider prefixes with hardcoded default base URLs |
| [0.9.0] | 2026-03-09 | Agent Observatory — real-time tool call visibility per wave agent via SSE |
| [0.8.0] | 2026-03-09 | Content-mode tool call fallback — handles local models (e.g. Qwen via Ollama) that embed tool calls in response content instead of `tool_calls` array |
| [0.6.0] | 2026-03-09 | OpenAI-compatible API backend + provider-prefix routing — `openai:gpt-4o`, `cli:kimi`, `anthropic:claude-*` prefix dispatch in `newBackendFunc` |
| [0.5.0] | 2026-03-09 | Configurable CLI binary — `BinaryPath` in `backend.Config` allows swapping `claude` for any compatible CLI |
| [0.4.0] | 2026-03-09 | Per-agent model routing — ScoutModel/WaveModel opts, `model:` field in IMPL doc agent sections, per-agent backend dispatch |
| [0.3.0] | 2026-03-08 | Protocol audit fixes — P0: failure_type parsing, multi-gen agent IDs; P1: E22 2-pass scaffold build, cross-repo Repo column; P2: repo field in completion reports |
| [0.2.0] | 2026-03-08 | Engine protocol parity — E17–E23 implemented (context memory, failure routing, stub scan, quality gates, scaffold build verify, per-agent context extraction) |
| [0.1.0] | 2026-03-08 | Initial engine extraction — parser, orchestrator, agent runner, git, worktree management |

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
