# SAW Engine — Project Memory

## Completed Features

### v0.5.0 — Configurable CLI binary (2026-03-09)
- `BinaryPath string` in `backend.Config` — optional path to CLI binary; resolution order: `Client.claudePath` → `Config.BinaryPath` → PATH lookup for `"claude"`

### v0.4.0 — Per-agent model routing (2026-03-09)
- `RunScoutOpts.ScoutModel` / `RunWaveOpts.WaveModel` — model overrides at the run level
- `AgentSpec.Model` — per-agent model field parsed from `**model:**` in IMPL doc sections
- `Orchestrator.SetDefaultModel()` + per-agent backend dispatch in `RunWave`
- `--model <model>` flag wired in CLI backend (was silently ignored before)

### v0.6.0 — OpenAI-compatible backend + provider-prefix routing (2026-03-09)
- `pkg/agent/backend/openai/` — new package implementing `backend.Backend` via `net/http` against OpenAI `/v1/chat/completions`; tool-call loop (Bash, Read, Write, Edit, Glob, Grep); streaming SSE for final stop turn
- `backend.Config.APIKey` / `backend.Config.BaseURL` — struct-based config for OpenAI backend; env var fallback (`OPENAI_API_KEY`)
- `BackendConfig.OpenAIKey` / `BackendConfig.BaseURL` — orchestrator-level config
- `parseProviderPrefix("openai:gpt-4o")` → `("openai", "gpt-4o")` — routing prefix parsed in `newBackendFunc`
- Provider dispatch: `"openai:*"` → openai backend; `"cli:*"` → CLI backend (binary from `SAW_CLI_BINARY` env); `"anthropic:*"` → Anthropic API backend; no prefix → existing auto logic

### v0.7.0 — Protocol SDK Phase 2: Orchestration Loop CLI (2026-03-09)
- 9 SDK functions in `pkg/protocol/`: `CreateWorktrees`, `VerifyCommits`, `ScanStubs`, `MergeAgents`, `Cleanup`, `VerifyBuild`, `UpdateStatus`, `UpdateContext`, `ListIMPLs`
- 1 git helper in `internal/git/`: `CommitCount`
- 10 CLI commands in `cmd/saw/`: `create-worktrees`, `verify-commits`, `scan-stubs`, `merge-agents`, `cleanup`, `verify-build`, `update-status`, `update-context`, `list-impls`, `run-wave`
- Binary output named `sawtools` (directory `cmd/saw/` is unchanged)
- Capstone orchestration: `RunWaveFull()` in `pkg/engine/` — full wave lifecycle in one call
- IMPL doc: `docs/IMPL/IMPL-orchestration-loop-cli.yaml` — 24 agents, 5 waves, SAW:COMPLETE 2026-03-09
- Cross-repo prompt updates: `saw-skill.md` v0.7.0, `saw-merge.md` v0.6.0, `saw-worktree.md` v0.6.0 in scout-and-wave repo

### v0.15.0 — Binary rename to sawtools (2026-03-09)
- Binary output renamed from `saw` to `sawtools`
- `cmd/saw/root.go`: Use field updated to `"sawtools"`, Short updated
- Clarifies split: `sawtools` = toolkit (this repo), `saw` = orchestrator (scout-and-wave-web)

## Established Interfaces

### `backend.Backend`
```go
type Backend interface {
    Run(ctx context.Context, systemPrompt, userMessage, workDir string) (string, error)
    RunStreaming(ctx context.Context, systemPrompt, userMessage, workDir string, onChunk ChunkCallback) (string, error)
}
```

### `backend.Config`
```go
type Config struct {
    Model      string
    MaxTokens  int
    MaxTurns   int
    BinaryPath string // CLI binary path override
    APIKey     string // OpenAI-compatible API key (falls back to OPENAI_API_KEY)
    BaseURL    string // OpenAI-compatible endpoint override
}
```

### `openai.New`
```go
func New(cfg backend.Config) *Client
// cfg.APIKey → OPENAI_API_KEY env fallback
// cfg.BaseURL → defaults to "https://api.openai.com/v1"
// cfg.Model   → defaults to "gpt-4o"
```

### `BackendConfig` (orchestrator)
```go
type BackendConfig struct {
    Kind      string // "api" | "cli" | "openai" | "auto" | or inferred from provider prefix
    APIKey    string // Anthropic key
    OpenAIKey string // OpenAI key (falls back to OPENAI_API_KEY)
    BaseURL   string // endpoint override for openai kind
    Model     string // may carry provider prefix: "openai:gpt-4o", "cli:kimi"
    MaxTokens int
    MaxTurns  int
}
```

## Architectural Decisions

- **Tool type is package-local** in both `api/` and `openai/` backends — avoids circular imports; each backend defines its own `tool` struct
- **net/http over openai-go SDK** — OpenAI backend uses raw HTTP; SDK is in go.mod but raw HTTP avoids SDK type churn
- **Provider prefix overrides Kind** — if `parseProviderPrefix(cfg.Model)` returns a non-empty provider, it takes precedence over `cfg.Kind`; this lets per-agent `model:` fields in IMPL docs route to any backend without changing orchestrator config
- **`SAW_CLI_BINARY` env** — custom CLI binary path for the `"cli:*"` dispatch case; complements `BinaryPath` in `backend.Config`
- **structured-output-parsing**: completed 2026-03-10, 3 waves, 4 agents
  - IMPL doc: docs/IMPL/IMPL-structured-output-parsing.yaml
- **constraint-solver**: completed 2026-03-10, 3 waves, 4 agents
  - IMPL doc: docs/IMPL/IMPL-constraint-solver.yaml
- **dependency-graph-generation**: completed 2026-03-11, 3 waves, 6 agents
  - IMPL doc: docs/IMPL/IMPL-dependency-graph-generation.yaml
- **h3-phase2-multi-language**: completed 2026-03-11, 2 waves, 4 agents
  - IMPL doc: docs/IMPL/IMPL-h3-phase2-multi-language.yaml
- **scaffold-detection**: completed 2026-03-12, 1 waves, 3 agents
  - IMPL doc: docs/IMPL/complete/IMPL-scaffold-detection.yaml
- **phase2-determinism-final**: completed 2026-03-12, 1 waves, 4 agents
  - IMPL doc: docs/IMPL/complete/IMPL-phase2-determinism-final.yaml
  - Deliverables: H1a (analyze-suitability), M2 (detect-cascades)
  - New packages: `pkg/suitability/` (pre-implementation scanning), `pkg/analyzer/` (cascade detection)
  - New commands: `sawtools analyze-suitability`, `sawtools detect-cascades`
- **h2-command-extraction**: completed 2026-03-12, 2 waves, 6 agents
  - IMPL doc: docs/IMPL/complete/IMPL-h2-command-extraction.yaml
