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
- 10 CLI commands in `cmd/sawtools/`: `create-worktrees`, `verify-commits`, `scan-stubs`, `merge-agents`, `cleanup`, `verify-build`, `update-status`, `update-context`, `list-impls`, `run-wave`
- Binary output named `sawtools` (directory `cmd/sawtools/`)
- Capstone orchestration: `RunWaveFull()` in `pkg/engine/` — full wave lifecycle in one call
- IMPL doc: `docs/IMPL/IMPL-orchestration-loop-cli.yaml` — 24 agents, 5 waves, SAW:COMPLETE 2026-03-09
- Cross-repo prompt updates: `saw-skill.md` v0.7.0, `saw-merge.md` v0.6.0, `saw-worktree.md` v0.6.0 in scout-and-wave repo

### v0.15.0 — Binary rename to sawtools (2026-03-09)
- Binary output renamed from `saw` to `sawtools`
- `cmd/sawtools/root.go`: Use field updated to `"sawtools"`, Short updated
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
- **h8-scaffold-validation**: completed 2026-03-12, 3 waves, 3 agents
  - IMPL doc: docs/IMPL/complete/IMPL-h8-scaffold-validation.yaml
- **h6-dependency-conflict-detection**: completed 2026-03-12, 3 waves, 6 agents
  - IMPL doc: docs/IMPL/complete/IMPL-h6-dependency-conflict-detection.yaml
- **batch-wave-commands**: completed 2026-03-12, 2 waves, 3 agents
  - IMPL doc: docs/IMPL/complete/IMPL-batch-wave-commands.yaml
- **h7-build-failure-diagnosis**: completed 2026-03-12, 3 waves, 6 agents
  - IMPL doc: docs/IMPL/complete/IMPL-h7-build-failure-diagnosis.yaml
- **dependency-graph-generation**: completed 2026-03-12, 3 waves, 6 agents
  - IMPL doc: docs/IMPL/complete/IMPL-dependency-graph-generation.yaml
- **constraint-solver**: completed 2026-03-12, 3 waves, 4 agents
  - IMPL doc: docs/IMPL/complete/IMPL-constraint-solver.yaml
- **structured-output-parsing**: completed 2026-03-12, 3 waves, 4 agents
  - IMPL doc: docs/IMPL/complete/IMPL-structured-output-parsing.yaml
- **m1-agent-id-assignment**: completed 2026-03-12, 3 waves, 3 agents
  - IMPL doc: docs/IMPL/complete/IMPL-m1-agent-id-assignment.yaml
- **journal-recovery-merge-idempotency**: completed 2026-03-14, 1 waves, 4 agents
  - IMPL doc: docs/IMPL/complete/IMPL-journal-recovery-merge-idempotency.yaml
- **m4-finalize-impl**: completed 2026-03-14, 2 waves, 3 agents
  - IMPL doc: ../scout-and-wave/docs/IMPL/complete/IMPL-m4-finalize-impl.yaml
- **bedrock-tool-loop**: completed 2026-03-14, 1 waves, 2 agents
  - IMPL doc: docs/IMPL/complete/IMPL-bedrock-tool-loop.yaml
- **workshop-constraints**: completed 2026-03-15, 2 waves, 6 agents
  - IMPL doc: docs/IMPL/complete/IMPL-workshop-constraints.yaml
- **impl-schema-validation**: completed 2026-03-15, 2 waves, 6 agents
  - IMPL doc: docs/IMPL/complete/IMPL-impl-schema-validation.yaml
- **bedrock-structured-output**: completed 2026-03-15, 2 waves, 4 agents
  - IMPL doc: docs/IMPL/complete/IMPL-bedrock-structured-output.yaml
- **integration-agent**: completed 2026-03-16, 4 waves, 12 agents
  - IMPL doc: docs/IMPL/complete/IMPL-integration-agent.yaml
- **determinism-automation-v2**: completed 2026-03-16, 2 waves, 6 agents
  - IMPL doc: docs/IMPL/IMPL-determinism-automation-v2.yaml
- **autonomy-layer**: completed 2026-03-17, 4 waves, 9 agents
  - IMPL doc: docs/IMPL/complete/IMPL-autonomy-layer.yaml
- **integration-checklist-m5**: completed 2026-03-18, 2 waves, 4 agents
  - IMPL doc: docs/IMPL/IMPL-integration-checklist-m5.yaml
- **e16-validation-enhancements**: completed 2026-03-18, 2 waves, 6 agents
  - IMPL doc: docs/IMPL/complete/IMPL-e16-validation-enhancements.yaml
- **multi-repo-prepare-wave**: completed 2026-03-19, 1 waves, 2 agents
  - IMPL doc: ../scout-and-wave-web/docs/IMPL/complete/IMPL-multi-repo-prepare-wave.yaml
- **gate-timing-fix**: completed 2026-03-19, 2 waves, 2 agents
  - IMPL doc: ../scout-and-wave-web/docs/IMPL/complete/IMPL-gate-timing-fix.yaml
- **planner-dag-prioritization**: completed 2026-03-19, 2 waves, 3 agents
  - IMPL doc: docs/IMPL/complete/IMPL-planner-dag-prioritization.yaml
- **gate-result-caching**: completed 2026-03-19, 1 waves, 3 agents
  - IMPL doc: ../scout-and-wave/docs/IMPL/complete/IMPL-gate-result-caching.yaml
- **interview-mode**: completed 2026-03-20, 2 waves, 4 agents
  - IMPL doc: ../scout-and-wave/docs/IMPL/complete/IMPL-interview-mode.yaml
- **type-collision-detection**: completed 2026-03-20, 2 waves, 3 agents
  - IMPL doc: ../scout-and-wave/docs/IMPL/complete/IMPL-type-collision-detection.yaml
- **protocol-docs-gaps**: completed 2026-03-20, 1 waves, 3 agents
  - IMPL doc: /Users/dayna.blackwell/code/scout-and-wave/docs/IMPL/complete/IMPL-protocol-docs-gaps.yaml
- **interview-improvements**: completed 2026-03-20, 2 waves, 3 agents
  - IMPL doc: /Users/dayna.blackwell/code/scout-and-wave/docs/IMPL/complete/IMPL-interview-improvements.yaml
- **auto-program-from-impls**: completed 2026-03-21, 2 waves, 3 agents
  - IMPL doc: docs/IMPL/complete/IMPL-auto-program-from-impls.yaml
- **repo-mismatch-hardening**: completed 2026-03-21, 2 waves, 3 agents
  - IMPL doc: docs/IMPL/complete/IMPL-repo-mismatch-hardening.yaml
- **sawtools-cli-gaps**: completed 2026-03-21, 3 waves, 7 agents
  - IMPL doc: docs/IMPL/complete/IMPL-sawtools-cli-gaps.yaml
- **baseline-gate-enforcement**: completed 2026-03-21, 1 waves, 3 agents
  - IMPL doc: docs/IMPL/complete/IMPL-baseline-gate-enforcement.yaml
- **determinism-enforcement**: completed 2026-03-21, 2 waves, 5 agents
  - IMPL doc: docs/IMPL/complete/IMPL-determinism-enforcement.yaml
- **protocol-conformity-gaps**: completed 2026-03-21, 2 waves, 6 agents
  - IMPL doc: docs/IMPL/complete/IMPL-protocol-conformity-gaps.yaml
- **orchestrator-ergonomics**: completed 2026-03-21, 2 waves, 4 agents
  - IMPL doc: docs/IMPL/complete/IMPL-orchestrator-ergonomics.yaml
- **program-disjoint-workflowing**: completed 2026-03-21, 3 waves, 5 agents
  - IMPL doc: docs/IMPL/complete/IMPL-program-disjoint-workflowing.yaml
- **wiring-audit-remaining**: completed 2026-03-22, 2 waves, 3 agents
  - IMPL doc: docs/IMPL/complete/IMPL-wiring-audit-remaining.yaml
- **determinism-remaining**: completed 2026-03-22, 2 waves, 5 agents
  - IMPL doc: docs/IMPL/complete/IMPL-determinism-remaining.yaml

## Features Completed
- Program: Go Engine Enhancements (create-program-cross-repo-and-program-wave-conflict-and-engine-reference-injection) — 2 tiers, 3 IMPLs, 2026-03-25
- Program: engine-hardening (engine-hardening) — 1 tiers, 2 IMPLs, 2026-03-22
- **types-consolidation**: completed 2026-03-22, 3 waves, 5 agents
  - IMPL doc: docs/IMPL/complete/IMPL-types-consolidation.yaml
- **cross-wave-build-continuity**: completed 2026-03-22, 1 waves, 3 agents
  - IMPL doc: docs/IMPL/complete/IMPL-cross-wave-build-continuity.yaml
- **provider-credentials**: completed 2026-03-22, 2 waves, 5 agents
  - IMPL doc: docs/IMPL/complete/IMPL-provider-credentials.yaml
- **hooks-quick-wins**: completed 2026-03-23, 2 waves, 7 agents
  - IMPL doc: docs/IMPL/complete/IMPL-hooks-quick-wins.yaml
- **file-read-dedup**: completed 2026-03-23, 2 waves, 2 agents
  - IMPL doc: docs/IMPL/IMPL-file-read-dedup.yaml
- **unified-state-management**: completed 2026-03-24, 2 waves, 4 agents
  - IMPL doc: docs/IMPL/complete/IMPL-unified-state-management.yaml
- **engine-decomposition**: completed 2026-03-24, 2 waves, 5 agents
  - IMPL doc: docs/IMPL/complete/IMPL-engine-decomposition.yaml
- **unify-config-management**: completed 2026-03-24, 1 waves, 2 agents
  - IMPL doc: docs/IMPL/complete/IMPL-unify-config-management.yaml
- **runner-decomposition**: completed 2026-03-24, 2 waves, 4 agents
  - IMPL doc: docs/IMPL/complete/IMPL-runner-decomposition.yaml
- **m4-pre-commit-gate**: completed 2026-03-24, 2 waves, 4 agents
  - IMPL doc: docs/IMPL/complete/IMPL-m4-pre-commit-gate.yaml
- **agent-complexity-enforcement**: completed 2026-03-25, 2 waves, 4 agents
  - IMPL doc: docs/IMPL/complete/IMPL-agent-complexity-enforcement.yaml
- **logging-slog**: completed 2026-03-25, 3 waves, 6 agents
  - IMPL doc: docs/IMPL/complete/IMPL-logging-slog.yaml
- **logging-injection**: completed 2026-03-25, 3 waves, 6 agents
  - IMPL doc: docs/IMPL/complete/IMPL-logging-injection.yaml
- **webhook-notifications**: completed 2026-03-25, 3 waves, 6 agents
  - IMPL doc: docs/IMPL/complete/IMPL-webhook-notifications.yaml
- **create-program-cross-repo**: completed 2026-03-25, 2 waves, 3 agents
  - IMPL doc: docs/IMPL/complete/IMPL-create-program-cross-repo.yaml
- **engine-reference-injection**: completed 2026-03-25, 2 waves, 2 agents
  - IMPL doc: docs/IMPL/complete/IMPL-engine-reference-injection.yaml
- **program-wave-conflict**: completed 2026-03-25, 2 waves, 4 agents
  - IMPL doc: docs/IMPL/complete/IMPL-program-wave-conflict.yaml
- **protocol-architectural-unification**: completed 2026-03-25, 2 waves, 7 agents
  - IMPL doc: docs/IMPL/complete/IMPL-protocol-architectural-unification.yaml
- **context-injection-observability**: completed 2026-03-25, 3 waves, 4 agents
  - IMPL doc: docs/IMPL/complete/IMPL-context-injection-observability.yaml
- **fullvalidate-severity-filter**: completed 2026-03-25, 1 waves, 1 agents
  - IMPL doc: docs/IMPL/complete/IMPL-fullvalidate-severity-filter.yaml
- **error-code-unification**: completed 2026-03-25, 2 waves, 7 agents
  - IMPL doc: docs/IMPL/complete/IMPL-error-code-unification.yaml
- **go-engine-deduplication**: completed 2026-03-25, 1 waves, 4 agents
  - IMPL doc: docs/IMPL/complete/IMPL-go-engine-deduplication.yaml
- **state-machine-unification**: completed 2026-03-26, 1 waves, 2 agents
  - IMPL doc: docs/IMPL/complete/IMPL-state-machine-unification.yaml
- **os-exit-cleanup**: completed 2026-03-26, 1 waves, 1 agents
  - IMPL doc: docs/IMPL/complete/IMPL-os-exit-cleanup.yaml
- **engine-cleanup-remaining**: completed 2026-03-26, 2 waves, 6 agents
  - IMPL doc: docs/IMPL/complete/IMPL-engine-cleanup-remaining.yaml
- **retry-unification**: completed 2026-03-26, 3 waves, 5 agents
  - IMPL doc: docs/IMPL/complete/IMPL-retry-unification.yaml
