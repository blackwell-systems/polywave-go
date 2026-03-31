# Reference Documentation

Complete reference for the Scout-and-Wave Go engine. All docs are derived
directly from source — if behavior differs from what's described here, the
code is authoritative and the doc should be updated.

## Authoring a manifest

| Doc | What it covers |
|-----|---------------|
| [manifest-schema.md](manifest-schema.md) | Every field in the IMPL manifest (IMPLManifest, Wave, Agent, FileOwnership, InterfaceContract, Scaffold, QualityGate, WiringDeclaration, Reactions, CompletionReport, etc.) |
| [program-schema.md](program-schema.md) | PROGRAM manifest fields (PROGRAMManifest, ProgramTier, ProgramContract, ProgramIMPL, state machine) |
| [validation-rules.md](validation-rules.md) | All 50+ validation invariants — what makes a manifest invalid, error codes, severity |
| [quality-gates.md](quality-gates.md) | QualityGate schema, GatePhase enum, timing, parallel_group, auto-fix gates |
| [wiring-declarations.md](wiring-declarations.md) | WiringDeclaration schema, E35/E26/E27 enforcement, integration_connectors, when to use wiring vs. reassign |
| [reactions-config.md](reactions-config.md) | Per-failure-type routing overrides, action values, E19 defaults table |

## Protocol rules

| Doc | What it covers |
|-----|---------------|
| [protocol-rules.md](protocol-rules.md) | All 32 E-rules (E10–E44) — phase, enforcement type, what each checks |
| [conflict-prediction.md](conflict-prediction.md) | E11 hunk-level merge safety: two-pass algorithm, cascade patch pattern, HunkRange semantics |
| [critic-workflow.md](critic-workflow.md) | E37 critic gate: CriticData schema, verdict matrix, auto vs manual mode, run-critic CLI |
| [completion-reports.md](completion-reports.md) | CompletionReport schema, status values, InterfaceDeviation, failure_type, agent vs orchestrator fields |

## CLI reference

| Doc | What it covers |
|-----|---------------|
| [cli-reference.md](cli-reference.md) | All sawtools commands — usage, flags, exit codes, examples |
| [binaries.md](binaries.md) | sawtools vs saw binary split, build instructions, release workflow |

## SDK / engine internals

| Doc | What it covers |
|-----|---------------|
| [result-types.md](result-types.md) | Result[T] and SAWError — constructors, methods, error code domains (V/W/B/G/A/N/P/T) |
| [error-codes.md](error-codes.md) | Error code registry with severity and descriptions |
| [autonomy-levels.md](autonomy-levels.md) | Gated/supervised/autonomous levels, Stage enum, approval matrix, saw.config.json |
| [observability.md](observability.md) | OrchestratorEvent types, SQLite store, query filters, rollups, sawtools query CLI |

## Architecture and integrations

| Doc | What it covers |
|-----|---------------|
| [architecture.md](architecture.md) | System overview, three-repo structure, package dependency graph |
| [orchestration.md](orchestration.md) | Engine orchestration flow, step sequence, E19 failure routing |
| [api-endpoints.md](api-endpoints.md) | saw server REST API |
| [sse-events.md](sse-events.md) | Server-sent events emitted by the saw server |
| [backends.md](backends.md) | LLM backend configuration (Anthropic, Bedrock, etc.) |
| [bedrock-tool-use-api.md](bedrock-tool-use-api.md) | AWS Bedrock tool use specifics |

---

> **Docs contract**: When adding or modifying a feature, update the relevant
> doc(s) above. If no doc exists for the feature, note a new one is needed in
> the completion report. See [CLAUDE.md](../../CLAUDE.md) for the full contract.
