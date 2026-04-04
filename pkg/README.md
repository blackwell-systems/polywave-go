# pkg/ — Package Landscape

## Package Groups

| Group | Packages | Purpose |
|-------|----------|---------|
| **Result & Codes** | `result` | Leaf layer: `Result[T]` type, `SAWError`, domain-prefixed error codes |
| **Protocol** | `protocol`, `solver`, `commands`, `format`, `errparse`, `gatecache` | IMPL manifest types, YAML parsing, wave solver, language toolchains |
| **Agent** | `agent`, `orchestrator`, `idgen`, `worktree`, `autonomy`, `tools` | Agent lifecycle: launch, poll, worktree isolation, tool constraints |
| **Engine** | `engine`, `pipeline`, `hooks`, `retry`, `resume` | Orchestration core: prepare/finalize waves, closed-loop gates, retries |
| **Analysis** | `analyzer`, `collision`, `scaffold`, `scaffoldval`, `deps`, `suitability`, `builddiag` | Static analysis: import graphs, type collisions, cascade detection |
| **Ops** | `config`, `journal`, `observability`, `notify`, `queue` | Configuration, event storage, notifications, feature queue |
| **Specialty** | `codereview`, `interview` | AI-powered review scoring, structured requirements gathering |

## Dependency Direction

```
cmd/sawtools          ← CLI shell, imports engine + most pkg/
  └─ engine           ← orchestration hub, imports ~25 packages
       ├─ orchestrator, agent, protocol, result
       ├─ analyzer, collision, builddiag, hooks
       └─ journal, observability, config
            └─ protocol   ← IMPL types, imports solver/result/commands
                 └─ result ← true leaf, zero pkg/ imports
```

## Entry Points

| Task | Start here |
|------|-----------|
| Add a sawtools command | `cmd/sawtools/` + `pkg/engine/` |
| Add an error code | `pkg/result/codes.go` (pick domain prefix, add constant) |
| Add an execution rule | `pkg/protocol/` types + `pkg/engine/` logic |
| Understand wave lifecycle | `pkg/engine/prepare.go` → `runner.go` → `finalize.go` |
| Add a new agent type | `pkg/agent/` + `pkg/orchestrator/` |

## Recurring Patterns

- **`result.Result[T]`** — canonical return type for fallible public functions (not `(T, error)`)
- **Domain-prefixed error codes** — `V`alidation, `B`uild, `G`it, `A`gent, `N`engine, `P`rotocol, etc. in `pkg/result/codes.go`
- **`onEvent func(Event)`** — callback-based progress reporting threaded through `pkg/engine` functions
- **`protocol.IMPLManifest`** — central data structure; nearly every engine/orchestrator function takes or loads one
- **`pipeline.Pipeline`** — composable step sequences used by engine for scout, wave, and gate flows
- **`result.SAWError` accumulation** — functions collect `[]SAWError` rather than fail-fast, enabling batch diagnostics
