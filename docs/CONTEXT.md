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
- Program: Auto-generated PROGRAM: Batch E37 failures in prepare-tier and add --skip-critic flag, CLI Backend Stderr Capture and Claude Code Context Detection (prepare-tier-batching-and-run-critic-stderr) — 1 tiers, 2 IMPLs, 2026-03-31
- Program: Auto-generated PROGRAM: pkg/config Code Review Hardening, Fix pkg/collision code review issues, Fix pkg/commands and gate_populator code review issues (config-hardening-and-collision-fixes-and-commands-fixes) — 1 tiers, 3 IMPLs, 2026-03-31
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
- **gate-result-consolidation**: completed 2026-03-26, 1 waves, 1 agents
  - IMPL doc: docs/IMPL/complete/IMPL-gate-result-consolidation.yaml
- **completion-report-unification**: completed 2026-03-26, 2 waves, 5 agents
  - IMPL doc: docs/IMPL/complete/IMPL-completion-report-unification.yaml
- **git-subprocess-unification**: completed 2026-03-26, 2 waves, 5 agents
  - IMPL doc: docs/IMPL/complete/IMPL-git-subprocess-unification.yaml
- **audit-findings-unification**: completed 2026-03-26, 2 waves, 7 agents
  - IMPL doc: docs/IMPL/complete/IMPL-audit-findings-unification.yaml
- **engine-unification**: completed 2026-03-30, 3 waves, 6 agents
  - IMPL doc: docs/IMPL/complete/IMPL-engine-unification.yaml
- **cli-thin-adapter**: completed 2026-03-30, 1 waves, 8 agents
  - IMPL doc: docs/IMPL/complete/IMPL-cli-thin-adapter.yaml
- **detect-wiring**: completed 2026-03-30, 2 waves, 4 agents
  - IMPL doc: docs/IMPL/complete/IMPL-detect-wiring.yaml
- **logging-injection-remaining**: completed 2026-03-30, 2 waves, 8 agents
  - IMPL doc: docs/IMPL/complete/IMPL-logging-injection-remaining.yaml
- **result-type-migration**: completed 2026-03-30, 3 waves, 20 agents
  - IMPL doc: docs/IMPL/complete/IMPL-result-type-migration.yaml
- **saw-system-hardening**: completed 2026-03-30, 1 waves, 5 agents
  - IMPL doc: docs/IMPL/complete/IMPL-saw-system-hardening.yaml
- **post-migration-cleanup**: completed 2026-03-30, 1 waves, 4 agents
  - IMPL doc: docs/IMPL/complete/IMPL-post-migration-cleanup.yaml
- **observability-improvements**: completed 2026-03-30, 1 waves, 3 agents
  - IMPL doc: docs/IMPL/complete/IMPL-observability-improvements.yaml
- **context-propagation**: completed 2026-03-31, 6 waves, 31 agents
  - IMPL doc: docs/IMPL/complete/IMPL-context-propagation.yaml
- **crash-fixes-and-error-code-string**: completed 2026-03-31, 1 waves, 1 agents
  - IMPL doc: docs/IMPL/complete/IMPL-crash-fixes-and-error-code-string.yaml
- **engine-error-code-catalog**: completed 2026-03-31, 1 waves, 1 agents
  - IMPL doc: docs/IMPL/complete/IMPL-engine-error-code-catalog.yaml
- **engine-structural-debt**: completed 2026-03-31, 1 waves, 1 agents
  - IMPL doc: docs/IMPL/complete/IMPL-engine-structural-debt.yaml
- **context-propagation**: completed 2026-03-31, 1 waves, 2 agents
  - IMPL doc: docs/IMPL/complete/IMPL-context-propagation.yaml
- **protocol-backend-cleanup**: completed 2026-03-31, 1 waves, 2 agents
  - IMPL doc: docs/IMPL/complete/IMPL-protocol-backend-cleanup.yaml
- **miscellaneous-cleanup**: completed 2026-03-31, 1 waves, 3 agents
  - IMPL doc: docs/IMPL/complete/IMPL-miscellaneous-cleanup.yaml
- **final-cleanup**: completed 2026-03-31, 1 waves, 2 agents
  - IMPL doc: docs/IMPL/complete/IMPL-final-cleanup.yaml
- **critic-gate-fixes**: completed 2026-03-31, 1 waves, 3 agents
  - IMPL doc: docs/IMPL/complete/IMPL-critic-gate-fixes.yaml
- **program-status-sync**: completed 2026-03-31, 1 waves, 1 agents
  - IMPL doc: docs/IMPL/complete/IMPL-program-status-sync.yaml
- **config-hardening**: completed 2026-03-31, 1 waves, 3 agents
  - IMPL doc: docs/IMPL/complete/IMPL-config-hardening.yaml
- **collision-fixes**: completed 2026-03-31, 1 waves, 1 agents
  - IMPL doc: docs/IMPL/complete/IMPL-collision-fixes.yaml
- **commands-fixes**: completed 2026-03-31, 1 waves, 2 agents
  - IMPL doc: docs/IMPL/complete/IMPL-commands-fixes.yaml
- **prepare-tier-batching**: completed 2026-03-31, 1 waves, 2 agents
  - IMPL doc: docs/IMPL/complete/IMPL-prepare-tier-batching.yaml
- **run-critic-stderr**: completed 2026-03-31, 1 waves, 2 agents
  - IMPL doc: docs/IMPL/complete/IMPL-run-critic-stderr.yaml
- **prepare-tier-batching**: completed 2026-03-31, 1 waves, 2 agents
  - IMPL doc: docs/IMPL/complete/IMPL-prepare-tier-batching.yaml
- **run-critic-stderr**: completed 2026-03-31, 1 waves, 2 agents
  - IMPL doc: docs/IMPL/complete/IMPL-run-critic-stderr.yaml
- **idgen-bug-fixes**: completed 2026-03-31, 1 waves, 1 agents
  - IMPL doc: docs/IMPL/complete/IMPL-idgen-bug-fixes.yaml
- **suitability-bug-fixes**: completed 2026-03-31, 1 waves, 2 agents
  - IMPL doc: docs/IMPL/complete/IMPL-suitability-bug-fixes.yaml
- **scaffold-bug-fixes**: completed 2026-03-31, 2 waves, 2 agents
  - IMPL doc: docs/IMPL/complete/IMPL-scaffold-bug-fixes.yaml
- **worktree-bug-fixes**: completed 2026-03-31, 1 waves, 1 agents
  - IMPL doc: docs/IMPL/complete/IMPL-worktree-bug-fixes.yaml
- **hooks-bug-fixes**: completed 2026-03-31, 1 waves, 2 agents
  - IMPL doc: docs/IMPL/complete/IMPL-hooks-bug-fixes.yaml
- **scaffoldval-bug-fixes**: completed 2026-03-31, 1 waves, 2 agents
  - IMPL doc: docs/IMPL/complete/IMPL-scaffoldval-bug-fixes.yaml
- **result-bug-fixes**: completed 2026-03-31, 1 waves, 2 agents
  - IMPL doc: docs/IMPL/complete/IMPL-result-bug-fixes.yaml
- **resume-bug-fixes**: completed 2026-03-31, 2 waves, 3 agents
  - IMPL doc: docs/IMPL/complete/IMPL-resume-bug-fixes.yaml
- **notify-bug-fixes**: completed 2026-03-31, 2 waves, 3 agents
  - IMPL doc: docs/IMPL/complete/IMPL-notify-bug-fixes.yaml
- **sawtools-ux-bugs**: completed 2026-03-31, 1 waves, 2 agents
  - IMPL doc: docs/IMPL/complete/IMPL-sawtools-ux-bugs.yaml
- **gowork-lsp-setup**: completed 2026-04-01, 1 waves, 2 agents
  - IMPL doc: docs/IMPL/complete/IMPL-gowork-lsp-setup.yaml
- **interview-bug-fixes**: completed 2026-04-01, 2 waves, 2 agents
  - IMPL doc: docs/IMPL/complete/IMPL-interview-bug-fixes.yaml
- **journal-bug-fixes**: completed 2026-04-01, 1 waves, 4 agents
  - IMPL doc: docs/IMPL/complete/IMPL-journal-bug-fixes.yaml
- **tools-bug-fixes**: completed 2026-04-01, 1 waves, 3 agents
  - IMPL doc: docs/IMPL/complete/IMPL-tools-bug-fixes.yaml
- **engine-doc**: completed 2026-04-01, 1 waves, 3 agents
  - IMPL doc: docs/IMPL/complete/IMPL-engine-doc.yaml
- **workspace-manager**: completed 2026-04-01, 3 waves, 6 agents
  - IMPL doc: docs/IMPL/complete/IMPL-workspace-manager.yaml
- **codereview-bug-fixes**: completed 2026-04-01, 1 waves, 2 agents
  - IMPL doc: docs/IMPL/complete/IMPL-codereview-bug-fixes.yaml
- **errparse-deep-review-fixes**: completed 2026-04-01, 2 waves, 4 agents
  - IMPL doc: docs/IMPL/complete/IMPL-errparse-deep-review-fixes.yaml
- **agent-deep-review-fixes**: completed 2026-04-01, 2 waves, 7 agents
  - IMPL doc: docs/IMPL/complete/IMPL-agent-deep-review-fixes.yaml
- **notify-deep-review-fixes**: completed 2026-04-01, 1 waves, 3 agents
  - IMPL doc: docs/IMPL/complete/IMPL-notify-deep-review-fixes.yaml
- **suitability-review-fixes**: completed 2026-04-01, 1 waves, 3 agents
  - IMPL doc: docs/IMPL/complete/IMPL-suitability-review-fixes.yaml
- **worktree-review-fixes**: completed 2026-04-01, 2 waves, 4 agents
  - IMPL doc: docs/IMPL/complete/IMPL-worktree-review-fixes.yaml
- **queue-review-fixes**: completed 2026-04-01, 2 waves, 2 agents
  - IMPL doc: docs/IMPL/complete/IMPL-queue-review-fixes.yaml
- **pkg-deps-review-fixes**: completed 2026-04-01, 1 waves, 8 agents
  - IMPL doc: docs/IMPL/complete/IMPL-deps-review-fixes.yaml
- **idgen-review-fixes**: completed 2026-04-01, 1 waves, 2 agents
  - IMPL doc: docs/IMPL/complete/IMPL-idgen-review-fixes.yaml
- **hooks-review-fixes**: completed 2026-04-01, 1 waves, 4 agents
  - IMPL doc: docs/IMPL/complete/IMPL-hooks-review-fixes.yaml
- **collision-review-findings**: completed 2026-04-01, 1 waves, 3 agents
  - IMPL doc: docs/IMPL/complete/IMPL-collision-review-findings.yaml
- **engine-review-fixes**: completed 2026-04-03, 5 waves, 21 agents
  - IMPL doc: docs/IMPL/complete/IMPL-engine-review-fixes.yaml
- **orchestrator-review-fixes**: completed 2026-04-03, 2 waves, 6 agents
  - IMPL doc: docs/IMPL/complete/IMPL-orchestrator-review-fixes.yaml
- **protocol-review-fixes**: completed 2026-04-03, 4 waves, 10 agents
  - IMPL doc: docs/IMPL/complete/IMPL-protocol-review-fixes.yaml
- **pipeline-review-fixes**: completed 2026-04-03, 2 waves, 3 agents
  - IMPL doc: docs/IMPL/complete/IMPL-pipeline-review-fixes.yaml
- **observability-review-fixes**: completed 2026-04-03, 2 waves, 6 agents
  - IMPL doc: docs/IMPL/complete/IMPL-observability-review-fixes.yaml
- **analyzer-review-fixes**: completed 2026-04-03, 4 waves, 10 agents
  - IMPL doc: docs/IMPL/complete/IMPL-analyzer-review-fixes.yaml
- **between-wave-integration-hotfix**: completed 2026-04-03, 3 waves, 7 agents
  - IMPL doc: docs/IMPL/complete/IMPL-between-wave-integration-hotfix.yaml
- **journal-integration**: completed 2026-04-03, 1 waves, 4 agents
  - IMPL doc: docs/IMPL/complete/IMPL-journal-integration.yaml
- **sawtools-ux-improvements**: completed 2026-04-03, 2 waves, 3 agents
  - IMPL doc: docs/IMPL/complete/IMPL-sawtools-ux-improvements.yaml
- **roadmap-hardening**: completed 2026-04-04, 2 waves, 4 agents
  - IMPL doc: docs/IMPL/complete/IMPL-roadmap-hardening.yaml
- **wire-manager-lifecycle**: completed 2026-04-04, 1 waves, 2 agents
  - IMPL doc: docs/IMPL/complete/IMPL-wire-manager-lifecycle.yaml
- **solver-graph-wiring**: completed 2026-04-04, 1 waves, 3 agents
  - IMPL doc: docs/IMPL/complete/IMPL-solver-graph-wiring.yaml
