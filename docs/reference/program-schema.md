# PROGRAM Schema Reference

PROGRAM manifests coordinate multiple IMPL docs into tiered, parallel execution.
Files live at `docs/PROGRAM/PROGRAM-<slug>.yaml`.

---

## PROGRAMManifest

The root document.

| YAML key | Type | Required | Description |
|---|---|---|---|
| `title` | string | **required** | Human-readable name for the program. |
| `program_slug` | string | **required** | Kebab-case identifier (`[a-z0-9]+(-[a-z0-9]+)*`). Used in the filename and as a stable reference. |
| `state` | string | **required** | Current lifecycle state. See [Program State Machine](#program-state-machine). |
| `created` | string | optional | RFC 3339 timestamp of when the manifest was first generated. |
| `updated` | string | optional | RFC 3339 timestamp of the last modification. |
| `requirements` | string | optional | Freeform requirements or goals for the program. Not parsed by the engine. |
| `program_contracts` | `[]ProgramContract` | optional | Cross-IMPL interface contracts that must be frozen at tier boundaries. |
| `impls` | `[]ProgramIMPL` | **required** | Flat list of all IMPLs in the program. Every IMPL must appear in exactly one tier. |
| `tiers` | `[]ProgramTier` | **required** | Ordered list of execution tiers. IMPLs within a tier run in parallel. |
| `tier_gates` | `[]QualityGate` | optional | Quality gates applied at tier boundaries, in addition to per-IMPL gates. |
| `completion` | `ProgramCompletion` | **required** | Progress counters. Must satisfy `impls_total == len(impls)`. |
| `pre_mortem` | `[]PreMortemRow` | optional | Risk register entries identifying potential failure modes. |

### Minimal example

```yaml
title: My Feature Program
program_slug: my-feature
state: REVIEWED
impls:
  - slug: api-layer
    title: API Layer
    tier: 1
    status: pending
  - slug: ui-layer
    title: UI Layer
    tier: 2
    depends_on: [api-layer]
    status: pending
tiers:
  - number: 1
    impls: [api-layer]
  - number: 2
    impls: [ui-layer]
completion:
  tiers_complete: 0
  tiers_total: 2
  impls_complete: 0
  impls_total: 2
  total_agents: 0
  total_waves: 0
```

---

## ProgramTier

Groups IMPLs that execute in parallel within the same tier.

| YAML key | Type | Required | Description |
|---|---|---|---|
| `number` | int | **required** | 1-indexed tier number. Tiers execute in ascending order. |
| `impls` | `[]string` | **required** | IMPL slugs belonging to this tier. Every slug must also appear in the root `impls` list. |
| `description` | string | optional | Human-readable explanation of what this tier accomplishes. |
| `concurrency_cap` | int | optional | Maximum number of IMPLs to run concurrently within this tier. `0` means unlimited. |

**Invariants enforced by validation:**
- Every IMPL slug in `tiers[n].impls` must be defined in the root `impls` list.
- Every IMPL in the root `impls` list must appear in exactly one tier (not zero, not multiple).
- IMPLs within the same tier must have no dependency on each other (P1 independence).
- An IMPL that depends on another must be in a strictly later tier.

```yaml
tiers:
  - number: 1
    description: Foundation — shared types and database layer
    impls:
      - data-model
      - migrations
  - number: 2
    description: Business logic — depends on tier 1
    concurrency_cap: 3
    impls:
      - api-handlers
      - background-jobs
      - notifications
```

---

## ProgramContract

Declares a cross-IMPL interface that must not change after a tier boundary completes.
Contracts allow agents in later tiers to depend on a stable API surface.

| YAML key | Type | Required | Description |
|---|---|---|---|
| `name` | string | **required** | Stable identifier for the contract. Referenced by `consumers[].impl` and `freeze_at`. |
| `description` | string | optional | What the contract represents. |
| `definition` | string | **required** | Canonical interface definition (type signature, OpenAPI fragment, schema, etc.). |
| `consumers` | `[]ProgramContractConsumer` | optional | IMPLs that consume this contract. Validated — all referenced slugs must exist. |
| `location` | string | **required** | Repo-relative path to the file that implements this contract (e.g. `pkg/api/types.go`). |
| `freeze_at` | string | optional | IMPL slug. When that IMPL's tier completes, the file at `location` is verified to be committed and treated as frozen. Subsequent IMPLs must not redefine this contract. |

```yaml
program_contracts:
  - name: UserAPI
    description: Public HTTP handler surface for the user service
    definition: |
      GET /users/{id} -> UserResponse
      POST /users     -> UserResponse
    location: pkg/api/user_handlers.go
    freeze_at: api-layer
    consumers:
      - impl: ui-layer
        usage: Calls GET /users/{id} to populate the profile page
      - impl: notifications
        usage: Calls POST /users to register webhook recipients
```

---

## ProgramContractConsumer

Identifies an IMPL that depends on a `ProgramContract`.

| YAML key | Type | Required | Description |
|---|---|---|---|
| `impl` | string | **required** | IMPL slug. Must exist in the root `impls` list. |
| `usage` | string | **required** | Free-text explanation of how this IMPL uses the contract. |

---

## ProgramIMPL

One entry per IMPL participating in the program. Mirrors high-level metadata from the IMPL doc itself.

| YAML key | Type | Required | Description |
|---|---|---|---|
| `slug` | string | **required** | Kebab-case IMPL slug. Must be unique within the program. Used to locate the IMPL doc at `docs/IMPL/IMPL-<slug>.yaml`. |
| `abs_path` | string | optional | Absolute filesystem path to the IMPL doc. Set automatically for cross-repo refs. Overrides the slug-based lookup. |
| `title` | string | **required** | Human-readable IMPL title. |
| `tier` | int | **required** | Tier number this IMPL belongs to. Must match its position in `tiers[].impls`. |
| `depends_on` | `[]string` | optional | Slugs of IMPLs this IMPL depends on. All referenced slugs must exist in `impls`. Dependencies must be in a strictly earlier tier. |
| `estimated_agents` | int | optional | Expected agent count across all waves, derived from the IMPL doc. |
| `estimated_waves` | int | optional | Expected wave count, derived from the IMPL doc. |
| `key_outputs` | `[]string` | optional | Named outputs produced by this IMPL (e.g. function names, type names, API endpoints). Populated from IMPL `interface_contracts`. |
| `status` | string | **required** | Current execution status. One of: `pending`, `scouting`, `reviewed`, `executing`, `complete`. |
| `priority_score` | int | optional | Numeric priority score (higher = more urgent). Set by the prioritizer. |
| `priority_reasoning` | string | optional | Explanation for the priority score. Set by the prioritizer. |
| `serial_waves` | `[]int` | optional | Wave numbers that must not execute concurrently with the same-numbered wave of any other IMPL in the same tier. Empty means all waves can run in parallel. Populated automatically by `create-program` via wave-level conflict detection. |

**IMPL status values:**

| Status | Meaning |
|---|---|
| `pending` | Not yet scouted. |
| `scouting` | Scout agent is running. |
| `reviewed` | IMPL doc exists and is in a reviewed-or-later protocol state. |
| `executing` | Wave agents are running. |
| `complete` | All waves done; IMPL doc is in COMPLETE state. |

```yaml
impls:
  - slug: data-model
    title: Core data model and database schema
    tier: 1
    estimated_agents: 4
    estimated_waves: 2
    key_outputs:
      - UserRecord
      - SessionRecord
      - CreateSchema
    status: complete

  - slug: api-handlers
    title: REST API handlers
    tier: 2
    depends_on:
      - data-model
    estimated_agents: 6
    estimated_waves: 3
    serial_waves: [2]
    status: pending
```

---

## ProgramCompletion

Progress counters for the overall program. Updated as tiers and IMPLs complete.

| YAML key | Type | Required | Description |
|---|---|---|---|
| `tiers_complete` | int | **required** | Number of tiers where all IMPLs have `status: complete`. |
| `tiers_total` | int | **required** | Total number of tiers. Must equal `len(tiers)`. |
| `impls_complete` | int | **required** | Number of IMPLs with `status: complete`. |
| `impls_total` | int | **required** | Total number of IMPLs. Must equal `len(impls)`. Validated. |
| `total_agents` | int | **required** | Sum of `estimated_agents` across all IMPLs. |
| `total_waves` | int | **required** | Sum of `estimated_waves` across all IMPLs. |

**Validation rules:**
- `tiers_complete` must not exceed `tiers_total`.
- `impls_complete` must not exceed `impls_total`.
- `impls_total` must equal `len(impls)` exactly.

```yaml
completion:
  tiers_complete: 1
  tiers_total: 2
  impls_complete: 2
  impls_total: 4
  total_agents: 18
  total_waves: 8
```

---

## ImportedIMPL

Returned by the `import-impls` operation. Describes a single IMPL that was imported into a PROGRAM manifest. Not written to the YAML file; used in API/CLI responses only.

| JSON key | Type | Description |
|---|---|---|
| `slug` | string | IMPL slug. |
| `title` | string | IMPL title. |
| `status` | string | Computed status at import time. |
| `assigned_tier` | int | Tier the IMPL was placed in. |
| `agent_count` | int | Number of agents across all waves. |
| `wave_count` | int | Number of waves. |

---

## QualityGate (tier_gates)

When used in `tier_gates`, quality gates run at tier boundaries before advancing to the next tier. The same type is used for per-IMPL wave gates.

| YAML key | Type | Required | Description |
|---|---|---|---|
| `type` | string | **required** | Gate category: `build`, `lint`, `test`, `typecheck`, `format`, or `custom`. |
| `command` | string | **required** | Shell command to execute. Run via `sh -c`. |
| `required` | bool | **required** | If `true`, a non-zero exit code blocks progression. Advisory gates are recorded but do not block. |
| `description` | string | optional | Human-readable label for the gate. |
| `repo` | string | optional | If set, the gate only runs when the working directory's base name matches this value. Enables multi-repo programs to scope gates. |
| `fix` | bool | optional | For `format` gates only. If `true`, runs the formatter in write mode. Must be paired with `phase: PRE_VALIDATION`. |
| `timing` | string | optional | `pre-merge` (default when empty) or `post-merge`. Controls when within finalize-wave the gate runs. |
| `phase` | string | optional | `PRE_VALIDATION`, `VALIDATION` (default), or `POST_VALIDATION`. Controls ordering and parallelization within gate execution. |
| `parallel_group` | string | optional | Gates sharing the same non-empty group name run concurrently within their phase. Empty means sequential. |

**Phase execution order:** `PRE_VALIDATION` → `VALIDATION` → `POST_VALIDATION`

**Constraint:** Format gates with `fix: true` must use `phase: PRE_VALIDATION`. Using any other phase (or omitting `phase`) with `fix: true` is a validation error.

```yaml
tier_gates:
  - type: build
    command: go build ./...
    required: true
    phase: VALIDATION
    parallel_group: checks

  - type: test
    command: go test ./...
    required: true
    phase: VALIDATION
    parallel_group: checks

  - type: format
    command: gofmt -l .
    required: false
    fix: true
    phase: PRE_VALIDATION
```

---

## PreMortemRow (pre_mortem)

Risk register entry. Each row describes one potential failure mode identified during planning.

| YAML key | Type | Required | Description |
|---|---|---|---|
| `scenario` | string | **required** | Description of the failure scenario. |
| `likelihood` | string | **required** | Subjective probability: `low`, `medium`, or `high`. |
| `impact` | string | **required** | Severity if it occurs: `low`, `medium`, or `high`. |
| `mitigation` | string | **required** | Planned response or preventive measure. |

```yaml
pre_mortem:
  - scenario: Agent produces incompatible interface for api-handlers tier
    likelihood: medium
    impact: high
    mitigation: Freeze the UserAPI contract at end of tier 1 via program_contracts.freeze_at
  - scenario: Wave 2 of both tier-2 IMPLs write to the same file
    likelihood: low
    impact: medium
    mitigation: serial_waves populated by create-program conflict detection
```

---

## Program State Machine

`state` in `PROGRAMManifest` tracks the overall program lifecycle.

| State | Constant | Meaning |
|---|---|---|
| `PLANNING` | `ProgramStatePlanning` | Being designed; tiers and IMPLs not yet finalized. |
| `VALIDATING` | `ProgramStateValidating` | Schema validation running; not yet approved for execution. |
| `REVIEWED` | `ProgramStateReviewed` | Structure approved. IMPLs may still be in `pending` status. Initial state produced by `create-program`. |
| `SCAFFOLD` | `ProgramStateScaffold` | Scaffold files are being committed before wave execution. |
| `TIER_EXECUTING` | `ProgramStateTierExecuting` | One or more IMPLs in the current tier are running waves. |
| `TIER_VERIFIED` | `ProgramStateTierVerified` | All IMPLs in the current tier are complete; tier gate checks passed. |
| `COMPLETE` | `ProgramStateComplete` | All tiers complete. `completion.tiers_complete == completion.tiers_total`. |
| `BLOCKED` | `ProgramStateBlocked` | Execution halted due to a gate failure, merge conflict, or unresolvable dependency. |
| `NOT_SUITABLE` | `ProgramStateNotSuitable` | Planning determined the feature is not appropriate for PROGRAM-level coordination. |

**Typical forward path:**

```
PLANNING → VALIDATING → REVIEWED → SCAFFOLD → TIER_EXECUTING → TIER_VERIFIED → (next tier) TIER_EXECUTING → … → COMPLETE
```

**Blocking and recovery:**

```
TIER_EXECUTING → BLOCKED → (manual resolution) → TIER_EXECUTING
```

---

## Validation Rules Summary

The engine runs these checks on every `PROGRAMManifest`:

| Rule | Scope | Error code |
|---|---|---|
| `title`, `program_slug`, `state` are non-empty | root | `REQUIRED_FIELDS_MISSING` |
| `state` is a valid `ProgramState` constant | root | `INVALID_STATE` |
| `program_slug` and all IMPL `slug` values are kebab-case | root, impls | `INVALID_SLUG_FORMAT` |
| Every IMPL `status` is one of the five valid values | impls | `INVALID_ENUM` |
| No IMPL in a tier depends on another IMPL in the same tier | impls | `P1_VIOLATION` |
| Every slug in `tiers[n].impls` exists in `impls` | tiers | `TIER_MISMATCH` |
| Every IMPL appears in exactly one tier | tiers | `TIER_MISMATCH` |
| All `depends_on` slugs exist in `impls` | impls | `INVALID_DEPENDENCY` |
| Dependent IMPLs are in a strictly later tier than their dependencies | impls | `TIER_ORDER_VIOLATION` |
| All `program_contracts[].consumers[].impl` slugs exist | program_contracts | `INVALID_CONSUMER` |
| `completion.tiers_complete ≤ completion.tiers_total` | completion | `COMPLETION_BOUNDS` |
| `completion.impls_complete ≤ completion.impls_total` | completion | `COMPLETION_BOUNDS` |
| `completion.impls_total == len(impls)` | completion | `IMPLS_TOTAL_MISMATCH` |

**Import-mode additional checks** (`ValidateProgramImportMode`):

| Rule | Error code |
|---|---|
| IMPLs with `status: reviewed` or `status: complete` must have a readable IMPL doc on disk | `IMPL_FILE_MISSING` |
| IMPL doc state must be consistent with program-level status | `IMPL_STATE_MISMATCH` |
| No two IMPLs in the same tier may own overlapping files (P1 file disjointness) | `P1_FILE_OVERLAP` |
| An IMPL doc may not redefine a frozen program contract | `P2_CONTRACT_REDEFINITION` |
