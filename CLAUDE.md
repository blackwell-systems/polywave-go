# Scout-and-Wave Go Engine — Claude Instructions

## Docs Contract

When adding or modifying a feature, check `docs/reference/` for an existing doc
covering it. If one exists, update it as part of the same wave. If none exists
and the feature is user-facing or protocol-level, note a new reference doc is
needed in the completion report.

Reference docs to check per area:
- IMPL manifest fields → `docs/reference/manifest-schema.md`
- PROGRAM manifest fields → `docs/reference/program-schema.md`
- Protocol rules (E-numbers) → `docs/reference/protocol-rules.md`
- Validation invariants → `docs/reference/validation-rules.md`
- CLI commands → `docs/reference/cli-reference.md`
- Wiring declarations → `docs/reference/wiring-declarations.md`
- Conflict prediction (E11) → `docs/reference/conflict-prediction.md`
- Quality gates → `docs/reference/quality-gates.md`
- Result types / error codes → `docs/reference/result-types.md`, `docs/reference/error-codes.md`
- Autonomy levels → `docs/reference/autonomy-levels.md`
- Critic workflow (E37) → `docs/reference/critic-workflow.md`
- Completion reports → `docs/reference/completion-reports.md`
- Reactions config → `docs/reference/reactions-config.md`
- Observability / event querying → `docs/reference/observability.md`
- Architecture overview → `docs/reference/architecture.md`
- Orchestration flow → `docs/reference/orchestration.md`
- API endpoints → `docs/reference/api-endpoints.md`
- SSE events → `docs/reference/sse-events.md`
- Backend configs → `docs/reference/backends.md`
- Binary layout → `docs/reference/binaries.md`

## Repo Role

This is the **Go engine** — the SDK and CLI layer for Scout-and-Wave. It is one
of three repos:

1. `scout-and-wave` — protocol spec (POSITION.md) and skill definitions
2. `scout-and-wave-go` ← **this repo** — Go implementation (pkg/, cmd/sawtools/)
3. `scout-and-wave-web` — web UI (imports the Go engine via `pkg/engine`)

**Cross-repo update order**: protocol spec first → Go engine second → web last.
Changes to pkg/protocol/types.go that add fields must be reflected in the
manifest-schema.md reference doc and may require web UI updates.

## Design Principles

### sawtools (cmd/sawtools/)
- Each command is a thin adapter: parse flags → call pkg/ → emit JSON → exit
- No orchestration logic in cmd/ — all logic lives in pkg/
- Exit 0 = success, exit 1 = partial/blocked, exit 2 = fatal/invalid input
- JSON output on stdout; human-readable errors on stderr
- See `cmd/sawtools/README.md` for the full command landscape

### Key packages (pkg/)

| Package | Role |
|---------|------|
| `protocol` | Pure data transformation — no I/O except git and file reads. Result[T] return type. |
| `engine` | Orchestration — delegates protocol logic to pkg/protocol. Step functions accept context.Context. |
| `orchestrator` | Agent launch, wave execution, tier management |
| `agent` | Agent prompt building, completion report parsing |
| `pipeline` | Multi-step pipeline composition |
| `hooks` | Claude Code hook integration |
| `journal` | Session journal capture and context recovery |
| `queue` | Queued IMPL execution |
| `resume` | Session resume detection |
| `retry` | Build-failure retry with context |
| `interview` | Structured requirements gathering |
| `solver` | Constraint-based wave structure solver |
| `scaffold` / `scaffoldval` | Scaffold detection and validation |
| `codereview` | Review agent logic |
| `autonomy` | Autonomy level enforcement |
| `analyzer` / `deps` / `collision` | Static analysis, dependency graphs, type collision detection |
| `suitability` | Codebase suitability analysis |
| `builddiag` / `errparse` | Build failure diagnosis and error parsing |
| `worktree` | Git worktree management |
| `config` | Configuration loading |
| `format` | Output formatting |
| `gatecache` | Gate result caching |
| `idgen` | Agent/wave ID generation |
| `notify` | Notification delivery |
| `observability` | Event logging and querying |
| `result` | Result[T] generic type |
| `tools` | Shared tool utilities |
| `commands` | Command registration helpers |

See `pkg/README.md` for additional navigation guidance.

## Pre-wave Checks

Before executing any wave on this repo, `sawtools pre-wave-validate` should
pass. E35 (same-package caller gaps) and E16 (manifest invariants) are both
blocking.

## Build

```bash
cd cmd/sawtools && go build -o ../../sawtools .
go test ./...
go vet ./...
```
