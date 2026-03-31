# IMPL Manifest Schema Reference

The IMPL manifest is a YAML document parsed into `protocol.IMPLManifest`. It is the single source of truth for a SAW feature: scout output, wave/agent definitions, quality gates, wiring declarations, and runtime reports all live here.

---

## Top-Level Fields

| Field | YAML Key | Type | Required | Description |
|-------|----------|------|----------|-------------|
| Title | `title` | string | yes | Human-readable name of the feature |
| FeatureSlug | `feature_slug` | string | yes | Machine-safe identifier used for branch/worktree names (e.g. `add-cache-api`) |
| Feature | `feature` | string | no | One-line summary written by the Scout; informational only |
| Repository | `repository` | string | no | Absolute path to repo root (single-repo waves) |
| Repositories | `repositories` | []string | no | Absolute paths for multi-repo waves; omit when single-repo |
| PlanReference | `plan_reference` | string | no | Path to the original plan document, if any |
| Verdict | `verdict` | string | yes | Scout suitability decision: `SUITABLE`, `NOT_SUITABLE`, or `SUITABLE_WITH_CAVEATS` |
| SuitabilityAssessment | `suitability_assessment` | string | no | Short prose summary of the suitability verdict |
| SuitabilityReasoning | `suitability_reasoning` | string | no | Detailed reasoning behind the verdict |
| TestCommand | `test_command` | string | yes | Command used to run the test suite (e.g. `go test ./...`) |
| LintCommand | `lint_command` | string | yes | Command used to run the linter |
| FileOwnership | `file_ownership` | []FileOwnership | yes | Which agent owns which file in which wave |
| InterfaceContracts | `interface_contracts` | []InterfaceContract | yes | Expected interfaces between agents/systems |
| Waves | `waves` | []Wave | yes | Ordered list of execution phases |
| QualityGates | `quality_gates` | QualityGates | no | Checks that must pass before a wave is complete |
| PostMergeChecklist | `post_merge_checklist` | PostMergeChecklist | no | Manual verification steps after wave merge |
| Scaffolds | `scaffolds` | []ScaffoldFile | no | Shared type files committed before wave agents run |
| Wiring | `wiring` | []WiringDeclaration | no | Explicit symbol→caller wiring contracts |
| WiringValidationReports | `wiring_validation_reports` | map[string]WiringValidationData | no | Per-wave wiring validation results; keys are wave labels (e.g. `wave1`) |
| CompletionReports | `completion_reports` | map[string]CompletionReport | no | Agent completion reports; keys are agent IDs |
| StubReports | `stub_reports` | map[string]ScanStubsData | no | Per-wave stub scan results; keys are wave labels |
| IntegrationReports | `integration_reports` | map[string]IntegrationReport | no | Per-wave integration gap reports (E25); keys are wave labels |
| IntegrationConnectors | `integration_connectors` | []IntegrationConnector | no | Files the integration agent is permitted to modify |
| PreMortem | `pre_mortem` | PreMortem | no | Risk analysis written by Scout |
| Reactions | `reactions` | ReactionsConfig | no | Per-failure-type routing overrides for agent retry behavior |
| KnownIssues | `known_issues` | []KnownIssue | no | Issues identified during scout with status and workarounds |
| IntegrationGapSeverityThreshold | `integration_gap_severity_threshold` | string | no | Minimum severity treated as a gap: `error`, `warning`, or `info`. Defaults to `warning` |
| CriticReport | `critic_report` | CriticData | no | Output of a critic-agent review run, if performed |
| State | `state` | ProtocolState | no | Current state machine position (see [Protocol States](#protocol-states)) |
| MergeState | `merge_state` | MergeState | no | Merge operation status for the current wave |
| InjectionMethod | `injection_method` | InjectionMethod | no | How the Scout received reference file content |
| CompletionDate | `completion_date` | string | no | ISO8601 date when the IMPL reached `COMPLETE` |

---

## Wave

Agents within a wave run in parallel. Waves execute sequentially.

| Field | YAML Key | Type | Required | Description |
|-------|----------|------|----------|-------------|
| Number | `number` | int | yes | 1-based wave index |
| Type | `type` | string | no | `standard` (default) or `integration` (wiring-only, no worktree) |
| Agents | `agents` | []Agent | yes | Agents that execute concurrently in this wave |
| AgentLaunchOrder | `agent_launch_order` | []string | no | Explicit agent ID ordering for serial launch when needed |
| BaseCommit | `base_commit` | string | no | Commit SHA recorded when worktrees are created; used for post-merge verification |

---

## Agent

| Field | YAML Key | Type | Required | Description |
|-------|----------|------|----------|-------------|
| ID | `id` | string | yes | Unique identifier within the wave (e.g. `A`, `B2`) |
| Task | `task` | string | yes | Natural-language description of what this agent implements |
| Files | `files` | []string | yes | Files this agent owns (absolute or repo-relative paths) |
| Dependencies | `dependencies` | []string | no | Agent IDs from a previous wave this agent depends on |
| Model | `model` | string | no | Override the default model for this agent |
| ContextSource | `context_source` | ContextSource | no | How the orchestrator delivered context to this agent (written at launch time) |

**ContextSource values:** `prepared-brief` (agent brief file used), `fallback-full-context` (brief inaccessible; full IMPL passed inline), `cross-repo-full` (cross-repo agent, full context payload).

---

## FileOwnership

| Field | YAML Key | Type | Required | Description |
|-------|----------|------|----------|-------------|
| File | `file` | string | yes | File path |
| Agent | `agent` | string | yes | Agent ID that owns this file |
| Wave | `wave` | int | yes | Wave number in which this file is owned |
| Action | `action` | string | no | `new`, `modify`, or `delete` |
| DependsOn | `depends_on` | []string | no | Other files this file depends on |
| Repo | `repo` | string | no | Repo identifier for cross-repo waves |

---

## InterfaceContract

Defines an expected API or interface that agents must respect. Frozen at worktree creation time; changes after that require explicit amendment.

| Field | YAML Key | Type | Required | Description |
|-------|----------|------|----------|-------------|
| Name | `name` | string | yes | Short contract identifier (e.g. `UserStore`) |
| Description | `description` | string | no | Human-readable explanation of the interface |
| Definition | `definition` | string | yes | The actual interface definition (code block, function signature, etc.) |
| Location | `location` | string | yes | File path where this interface lives or will be created |

---

## Scaffold

Scaffold files are committed to the repo before wave agents launch, allowing parallel agents to import shared types without race conditions.

| Field | YAML Key | Type | Required | Description |
|-------|----------|------|----------|-------------|
| FilePath | `file_path` | string | yes | Destination file path for the scaffold |
| Contents | `contents` | string | no | Full file contents to write |
| ImportPath | `import_path` | string | no | Go import path (or equivalent) for this scaffold |
| Status | `status` | string | no | `pending` or `committed` |
| Commit | `commit` | string | no | Commit SHA after the scaffold was committed |

---

## QualityGates

```yaml
quality_gates:
  level: standard        # "quick" | "standard" | "full"
  gates:
    - type: test
      command: go test ./...
      required: true
      timing: pre-merge
      phase: VALIDATION
      parallel_group: checks
```

### QualityGates Container

| Field | YAML Key | Type | Required | Description |
|-------|----------|------|----------|-------------|
| Level | `level` | string | yes | Gate set level: `quick`, `standard`, or `full` |
| Gates | `gates` | []QualityGate | yes | Individual gate definitions |

### QualityGate

| Field | YAML Key | Type | Required | Description |
|-------|----------|------|----------|-------------|
| Type | `type` | string | yes | `build`, `lint`, `test`, `typecheck`, `format`, or `custom` |
| Command | `command` | string | yes | Shell command to execute |
| Required | `required` | bool | yes | If true, failure blocks the wave |
| Description | `description` | string | no | Human-readable explanation |
| Repo | `repo` | string | no | If set, gate only runs in this repo |
| Fix | `fix` | bool | no | Enables fix mode for `format` gates (e.g. `gofmt -w`) |
| Timing | `timing` | string | no | `pre-merge` (default) runs before agent merge; `post-merge` runs after |
| Phase | `phase` | GatePhase | no | Execution phase: `PRE_VALIDATION` (sequential, auto-fix), `VALIDATION` (parallel, default), or `POST_VALIDATION` (parallel, review) |
| ParallelGroup | `parallel_group` | string | no | Gates sharing the same group run in parallel within their phase; empty means sequential |

---

## WiringDeclaration

Declares that a symbol must be called from a specific file. Enforced at `prepare-wave` pre-flight and `validate-integration` post-merge. Injected into agent briefs.

| Field | YAML Key | Type | Required | Description |
|-------|----------|------|----------|-------------|
| Symbol | `symbol` | string | yes | Function or type name that must be wired |
| DefinedIn | `defined_in` | string | yes | File where the symbol is defined |
| MustBeCalledFrom | `must_be_called_from` | string | yes | File that must contain a call/reference to the symbol |
| Agent | `agent` | string | yes | Agent ID responsible for the caller file |
| Wave | `wave` | int | yes | Wave in which wiring must be verified |
| IntegrationPattern | `integration_pattern` | string | no | Guidance on how to wire the symbol (e.g. `register in init()`) |

---

## Reactions

Per-failure-type routing overrides. When absent, the orchestrator uses E19 hardcoded defaults.

```yaml
reactions:
  transient:
    action: retry
    max_attempts: 3
  timeout:
    action: retry
    max_attempts: 2
  fixable:
    action: send-fix-prompt
    max_attempts: 2
  needs_replan:
    action: auto-scout
  escalate:
    action: pause
```

### ReactionsConfig

| Field | YAML Key | Type | Description |
|-------|----------|------|-------------|
| Transient | `transient` | ReactionEntry | Network/infra transient failures |
| Timeout | `timeout` | ReactionEntry | Agent timeout failures |
| Fixable | `fixable` | ReactionEntry | Failures with a correctable code issue |
| NeedsReplan | `needs_replan` | ReactionEntry | Failures requiring a new scout pass |
| Escalate | `escalate` | ReactionEntry | Failures requiring human intervention |

### ReactionEntry

| Field | YAML Key | Type | Required | Description |
|-------|----------|------|----------|-------------|
| Action | `action` | string | yes | One of: `retry`, `send-fix-prompt`, `pause`, `auto-scout` |
| MaxAttempts | `max_attempts` | int | no | Maximum launch attempts including the first. `0` uses the E19 default |

---

## PostMergeChecklist

Manual verification groups shown to the operator after wave merge.

```yaml
post_merge_checklist:
  groups:
    - title: Smoke Tests
      items:
        - description: Run the server and hit /health
          command: curl localhost:8080/health
```

| Type | Field | YAML Key | Description |
|------|-------|----------|-------------|
| PostMergeChecklist | groups | `groups` | Ordered list of ChecklistGroup |
| ChecklistGroup | title | `title` | Section heading |
| ChecklistGroup | items | `items` | List of ChecklistItem |
| ChecklistItem | description | `description` | What to verify |
| ChecklistItem | command | `command` | Optional shell command to run |

---

## CompletionReport

Written by each wave agent after it finishes. Keyed by agent ID in `completion_reports`. Full details are covered in the completion report reference doc.

| Field | YAML Key | Type | Description |
|-------|----------|------|-------------|
| Status | `status` | CompletionStatus | `complete`, `partial`, or `blocked` |
| Worktree | `worktree` | string | Worktree path used |
| Branch | `branch` | string | Branch name |
| Commit | `commit` | string | Final commit SHA |
| FilesChanged | `files_changed` | []string | Modified files |
| FilesCreated | `files_created` | []string | Newly created files |
| InterfaceDeviations | `interface_deviations` | []InterfaceDeviation | Deviations from planned contracts |
| OutOfScopeDeps | `out_of_scope_deps` | []string | Dependencies outside the agent's assigned scope |
| TestsAdded | `tests_added` | []string | Test files or test names added |
| Verification | `verification` | string | How the agent verified its work |
| FailureType | `failure_type` | string | Failure category if status is `partial` or `blocked` |
| Notes | `notes` | string | Free-form notes |
| Repo | `repo` | string | Repo identifier for cross-repo waves |

### InterfaceDeviation

| Field | YAML Key | Type | Description |
|-------|----------|------|-------------|
| Description | `description` | string | What changed from the contract |
| DownstreamActionRequired | `downstream_action_required` | bool | Whether downstream agents must adapt |
| Affects | `affects` | []string | Agent IDs affected by the deviation |

---

## Frozen Contract Fields

These fields are written by the orchestrator at worktree creation time and used to detect unauthorized changes to shared contracts.

| Field | YAML Key | Type | Description |
|-------|----------|------|-------------|
| WorktreesCreatedAt | `worktrees_created_at` | time.Time | Timestamp when worktrees were created |
| FrozenContractsHash | `frozen_contracts_hash` | string | SHA256 hash of `interface_contracts` at freeze time |
| FrozenScaffoldsHash | `frozen_scaffolds_hash` | string | SHA256 hash of `scaffolds` at freeze time |

If either hash changes between freeze and finalize, `finalize-wave` rejects the manifest with a freeze violation error.

---

## Protocol States

Values for the `state` field:

| Value | Description |
|-------|-------------|
| `SCOUT_PENDING` | Scout has not run yet |
| `SCOUT_VALIDATING` | Scout output is being validated |
| `REVIEWED` | Manifest has been reviewed and is ready for wave execution |
| `SCAFFOLD_PENDING` | Scaffold files need to be committed |
| `WAVE_PENDING` | Wave is ready to launch |
| `WAVE_EXECUTING` | Wave agents are running |
| `WAVE_MERGING` | Wave is in the merge/finalize phase |
| `WAVE_VERIFIED` | Wave passed quality gates and verification |
| `BLOCKED` | Wave is blocked; human intervention required |
| `COMPLETE` | All waves finished successfully |
| `NOT_SUITABLE` | Scout determined this feature is not suitable for parallel implementation |

---

## Other Runtime Fields

### PreMortem

Written by Scout to capture anticipated failure modes.

| Field | YAML Key | Type | Description |
|-------|----------|------|-------------|
| OverallRisk | `overall_risk` | string | `low`, `medium`, or `high` |
| Rows | `rows` | []PreMortemRow | Individual risk scenarios |

**PreMortemRow:** `scenario` (string), `likelihood` (string), `impact` (string), `mitigation` (string).

### KnownIssue

| Field | YAML Key | Type | Description |
|-------|----------|------|-------------|
| Title | `title` | string | Short label |
| Description | `description` | string | Full description of the issue |
| Status | `status` | string | Current status (e.g. `open`, `resolved`) |
| Workaround | `workaround` | string | Steps to work around the issue |

### CriticReport (CriticData)

Written by the critic agent after reviewing agent briefs. See `pkg/protocol/critic.go` for full type definitions.

| Field | YAML Key | Type | Description |
|-------|----------|------|-------------|
| Verdict | `verdict` | string | `PASS`, `ISSUES`, or `SKIPPED` |
| AgentReviews | `agent_reviews` | map[string]AgentCriticReview | Per-agent verdict keyed by agent ID |
| Summary | `summary` | string | Human-readable summary of findings |
| ReviewedAt | `reviewed_at` | string | ISO8601 timestamp of review completion |
| IssueCount | `issue_count` | int | Total issues found across all agents |

**AgentCriticReview:** `agent_id`, `verdict` (`PASS`/`ISSUES`), `issues` ([]CriticIssue).

**CriticIssue:** `check` (category), `severity` (`error`/`warning`), `description`, `file` (optional), `symbol` (optional).

### StubReports

Map of wave label → `ScanStubsData`. Written by `finalize-wave` after stub scanning.

**ScanStubsData:** `hits` ([]StubHit). Each **StubHit** has `file`, `line`, `pattern` (e.g. `TODO`), `context` (trimmed source line).

### IntegrationReports

Map of wave label → `IntegrationReport`. Written by `validate-integration` (E25).

**IntegrationReport:** `wave` (int), `gaps` ([]IntegrationGap), `valid` (bool), `summary` (string).

**IntegrationGap:** `export_name`, `file_path`, `agent_id`, `category` (`function_call`/`type_usage`/`field_init`), `severity` (`error`/`warning`/`info`), `reason`, `suggested_fix`, `search_results` ([]string).

### IntegrationConnectors

Files the integration agent is allowed to modify when wiring gaps.

| Field | YAML Key | Type | Description |
|-------|----------|------|-------------|
| File | `file` | string | File path (e.g. `pkg/server/orchestrator.go`) |
| Reason | `reason` | string | Why this file needs wiring |

### WiringValidationReports

Map of wave label → `WiringValidationData`. Written by `validate-integration --wiring`.

**WiringValidationData:** `gaps` ([]WiringGap), `valid` (bool), `summary` (string).

**WiringGap:** `declaration` (WiringDeclaration), `reason` (string), `severity` (always `error`).

### InjectionMethod

Records how the Scout received reference files. Written by the Scout before completing.

| Value | Description |
|-------|-------------|
| `hook` | `validate_agent_launch` hook injected references via `updatedInput` |
| `manual-fallback` | Scout read reference files explicitly (hook absent or failed) |
| `unknown` | Scout predates this field |
