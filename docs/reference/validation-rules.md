# Validation Rules Reference

## Rule Table

| Code | Name | Severity | Source | Trigger |
|------|------|----------|--------|---------|
| V001_MANIFEST_INVALID | Manifest Invalid | fatal | `FullValidate` | YAML fails to load or parse entirely |
| V002_DISJOINT_OWNERSHIP | Disjoint Ownership | error | `validateI1DisjointOwnership` | Two agents own the same file in the same wave |
| V003_SAME_WAVE_DEPENDENCY | Same-Wave Dependency | error | `validateI2AgentDependencies` | Agent depends on another agent in the same or later wave; also triggered by `file_ownership.depends_on` referencing a same/later-wave agent |
| V004_WAVE_NOT_1INDEXED | Wave Not 1-Indexed | error | `validateI3WaveOrdering` | Waves are not sequentially numbered starting at 1 (e.g., skips from 1 to 3) |
| V005_REQUIRED_FIELDS_MISSING | Required Fields Missing | error | `validateI4RequiredFields`, `validateNestedRequiredFields`, `ValidateReactions` | `title`, `feature_slug`, or `verdict` is blank; nested required fields missing on `FileOwnership`, `Wave`, `Agent`, `InterfaceContract`, `QualityGate`, `ScaffoldFile`, `PreMortemRow`, or `reactions.*` entries |
| V006_FILE_OWNERSHIP_INCOMPLETE | File Ownership Incomplete | error | `validateI5FileOwnershipComplete` | An `agent.files[]` entry has no matching row in `file_ownership` |
| V007_DEPENDENCY_CYCLE | Dependency Cycle | error | `validateI6NoCycles` | Agent dependency graph contains a cycle (detected via DFS) |
| V008_INVALID_STATE | Invalid State | error | `validateSM01StateValid`, `validateProgramState` | `state` field is not one of the valid `ProtocolState` or `ProgramState` constants |
| V009_INVALID_AGENT_ID | Invalid Agent ID | error | `validateAgentIDs` | Agent ID in `waves`, `file_ownership`, or `completion_reports` does not match `^[A-Z][2-9]?$` |
| V010_INVALID_GATE_TYPE | Invalid Gate Type | error | `validateGateTypes` | `quality_gates.gates[].type` is not one of: `build`, `lint`, `test`, `typecheck`, `format`, `custom` |
| V011_INVALID_ACTION_ENUM | Invalid Action Enum | error | `ValidateActionEnums` | `file_ownership[].action` is not one of: `new`, `modify`, `delete` |
| V012_DUPLICATE_KEY | Duplicate Key | error | `ValidateDuplicateKeys` | A top-level YAML key appears more than once in the raw manifest file |
| V013_UNKNOWN_KEY | Unknown Key | error | `DetectUnknownKeys` | A top-level YAML key does not correspond to any known manifest field |
| V014_INVALID_SCAFFOLD_STATUS | Invalid Scaffold Status | error | `validateScaffoldStatuses` | `scaffolds[].status` is not `pending`, does not start with `committed`, and does not start with `FAILED` |
| V015_INVALID_PRE_MORTEM_RISK | Invalid Pre-Mortem Risk | error | `ValidatePreMortemRisk` | `pre_mortem.overall_risk` is not one of: `low`, `medium`, `high` |
| V016_JSONSCHEMA_FAILED | JSON Schema Failed | error | `ValidateManifestJSON` | Manifest fails JSON Schema structural checks (missing required fields, wrong types, invalid enum values) |
| V017_SLUG_MISMATCH | Slug Mismatch | error | (slug cross-check) | `feature_slug` in manifest does not match the expected slug derived from the file name |
| V018_INVALID_SLUG_FORMAT | Invalid Slug Format | error | `validateSlugFormats`, `programGenerator` | `program_slug` or IMPL slug is not kebab-case (`^[a-z0-9]+(-[a-z0-9]+)*$`); note: `feature_slug` format violations in IMPL manifests emit V039, not V018 |
| V019_ORPHAN_FILE | Orphan File | error | `validateI5FileOwnershipComplete` | A file listed in `agent.files[]` has no corresponding `file_ownership` entry |
| V020_INCONSISTENT_REPO | Inconsistent Repo | error | `validateMultiRepoConsistency` | Some `file_ownership` entries have an explicit `repo:` field while others do not; mixing is disallowed |
| V021_KNOWN_ISSUE_MISSING_TITLE | Known Issue Missing Title | error | `validateKnownIssueTitles` | A `known_issues[]` entry has an empty `title` field |
| V022_INVALID_FAILURE_TYPE | Invalid Failure Type | error | `ValidateFailureTypes` | `completion_reports[].failure_type` is not one of: `transient`, `fixable`, `needs_replan`, `escalate`, `timeout` |
| V023_INVALID_MERGE_STATE | Invalid Merge State | error | `validateE9MergeState` | `merge_state` is not one of: `idle`, `in_progress`, `completed`, `failed` |
| V024_PROGRAM_INVALID | Program Invalid | fatal | *(unused — defined but not emitted; `FullValidateProgram` emits V046 for parse failures)* | PROGRAM manifest fails to parse |
| V025_TIER_MISMATCH | Tier Mismatch | error | `validateTierIMPLConsistency` | A `tiers[]` entry references an IMPL slug not defined in `impls[]`, or an IMPL appears in zero or multiple tiers |
| V026_TIER_ORDER_VIOLATION | Tier Order Violation | error | `validateTierOrdering` | IMPL-A depends on IMPL-B but A's tier number is not strictly greater than B's |
| V027_INVALID_CONSUMER | Invalid Consumer | error | `validateProgramContractConsumers` | A `program_contracts[].consumers[].impl` references an IMPL slug not defined in `impls[]` |
| V028_INVALID_DEPENDENCY | Invalid Dependency | error | `validateDependencyValidity` | An IMPL's `depends_on` references a slug not defined in `impls[]` |
| V029_P1_FILE_OVERLAP | P1 File Overlap | error | `ValidateP1FileDisjointness` | Two IMPLs in the same program tier claim ownership of the same file |
| V030_P2_CONTRACT_REDEFINITION | P2 Contract Redefinition | error | `ValidateProgramImportMode` | An IMPL redefines a `program_contract` that is already frozen (its `freeze_at` tier is fully complete) |
| V031_IMPL_FILE_MISSING | IMPL File Missing | error | `ValidateProgramImportMode` | An IMPL with status `reviewed` or `complete` has no corresponding IMPL YAML file on disk |
| V032_IMPL_STATE_MISMATCH | IMPL State Mismatch | error | `ValidateProgramImportMode` | An IMPL's program-level status (`reviewed`/`complete`) disagrees with the state field inside the IMPL doc itself |
| V033_COMPLETION_BOUNDS | Completion Bounds | error | `validateCompletionBounds` | `completion.tiers_complete` exceeds `completion.tiers_total`, or `completion.impls_complete` exceeds `completion.impls_total` |
| V034_IMPLS_TOTAL_MISMATCH | Impls Total Mismatch | error | `validateCompletionBounds` | `completion.impls_total` does not equal the actual count of `impls[]` entries |
| V035_P1_VIOLATION | P1 Violation | error | `validateP1Independence` | An IMPL in a PROGRAM tier depends on another IMPL in the same tier |
| V036_INVALID_ENUM | Invalid Enum | error | `validateAllEnums`, `validateIMPLStatuses`, `ValidateReactions`, `ValidateCompletionStatuses` | Any enum field contains a value not in its allowed set (see per-field details below) |
| V037_INVALID_PATH | Invalid Path | error | `validateFilePaths` | A file path starts with `/`, contains `..`, contains a null byte, or uses backslashes |
| V038_CROSS_FIELD | Cross-Field Inconsistency | error | `validateCrossFieldConsistency` | `file_ownership` agent not in any wave; `file_ownership` wave number does not match any wave; `agent.files[]` entry has no matching `file_ownership` row with same agent+wave; `verdict=NOT_SUITABLE` with non-empty waves/file_ownership/interface_contracts; `completion_reports` key references an agent not in any wave |
| V039_INVALID_FIELD_VALUE | Invalid Field Value | error | `validateI4RequiredFields` | `verdict` is not `SUITABLE`, `NOT_SUITABLE`, or `SUITABLE_WITH_CAVEATS`; also emitted when `feature_slug` is not kebab-case |
| V040_UNSCOPED_GATE | Unscoped Gate | error | `validateMultiRepoConsistency` | A multi-repo IMPL has a `quality_gates.gates[]` entry with no `repo:` field |
| V041_FILE_MISSING | File Missing | error | `ValidateFileExistence` | A `file_ownership` entry with `action=modify` references a file that does not exist on disk (only when `repoPath` is provided) |
| V042_INVALID_WORKTREE_NAME | Invalid Worktree Name | error | `ValidateWorktreeNames` | Completion report `branch` does not match `wave{N}-agent-{ID}` or `saw/{slug}/wave{N}-agent-{ID}`; or `worktree` path does not contain `wave{N}-agent-{ID}` as a path segment. Solo-wave agents (single agent in the wave) are exempt. |
| V043_INVALID_VERIFICATION | Invalid Verification | error | `ValidateVerificationField` | `completion_reports[].verification` is non-empty but does not contain `PASS` or `FAIL` as a standalone word |
| V044_MISSING_CHECKLIST | Missing Checklist | warning | `ValidateIntegrationChecklist` | New API handlers (`pkg/api/*_handler.go`) or React components (`web/src/components/*.tsx`) are declared in `file_ownership` but `post_merge_checklist` is absent or empty |
| V045_REPO_MISMATCH | Repo Mismatch | error | `ValidateFileExistenceMultiRepo` | All `action=modify` files are missing from disk, suggesting the IMPL targets a different repository |
| V046_PARSE_ERROR | Parse Error | error/fatal | `ValidateDuplicateKeys`, `FullValidateProgram` | Raw YAML cannot be parsed at all; severity is `error` from `ValidateDuplicateKeys`, `fatal` from `FullValidateProgram` |
| V047_TRIVIAL_SCOPE | Trivial Scope | error | `CheckAgentComplexity` | IMPL is `SUITABLE` or `SUITABLE_WITH_CAVEATS` but has exactly 1 agent owning exactly 1 file (no parallelization value); exempt for slugs containing `-retry-` |
| V048_AGENT_LOC_BUDGET | Agent LOC Budget | error | `CheckAgentLOCBudget` | An agent's total `action=modify` file lines exceed 2000 LOC (only when `repoPath` is provided) |
| W001_AGENT_SCOPE_LARGE | Agent Scope Large | warning | `CheckAgentComplexity` | An agent owns more than 8 files, or creates more than 5 new files |
| W002_COMPLETION_VERIFY | Completion Verify | warning | `ValidateCompletionReportClaims` | Commit SHA cannot be verified as reachable from the agent's expected branch |
| P007_WIRING_GAP | Wiring Gap | error | `ValidateWiringDeclarations` | A symbol declared in `wiring[]` is not found as a call expression in its `must_be_called_from` file |
| agent-id | Agent ID (typed block) | error | `ValidateIMPLDoc` | Agent ID in typed block (`impl-file-ownership`, `impl-dep-graph`, `impl-wave-structure`) does not match `^[A-Z][2-9]?$` |
| e16a | Missing Required Block | error | `ValidateIMPLDoc` | IMPL doc has at least one typed block but is missing one of: `impl-file-ownership`, `impl-dep-graph`, `impl-wave-structure` |
| impl-dep-graph | Dep Graph Block Invalid | error | `ValidateIMPLDoc` | `impl-dep-graph` block missing `Wave N` header, has no agent lines `[X]`, or an agent block lacks both `✓ root` and `depends on:` |
| impl-file-ownership | File Ownership Block Invalid | error | `ValidateIMPLDoc` | `impl-file-ownership` block missing header row `\| File \|`, has no data rows, or a data row has fewer than 4 pipe characters |
| impl-wave-structure | Wave Structure Block Invalid | error | `ValidateIMPLDoc` | `impl-wave-structure` block missing `Wave N:` lines or has no agent letter references |
| impl-completion-report | Completion Report Block Invalid | error | `ValidateIMPLDoc` | `impl-completion-report` block missing required fields (`status`, `worktree`, `branch`, `commit`, `files_changed`, `interface_deviations`, `verification`) or `status` is not `complete`, `partial`, or `blocked` |
| warning (E16C) | Out-of-Band Dep Graph | warning | `ValidateIMPLDoc` | A plain fenced code block (no `type=` annotation) contains content that looks like a dep graph (`[A]`-style references and `Wave`) |
| MIGRATION_BOUNDARY_WARNING | Migration Boundary | warning | `ValidateMigrationBoundaries` | Consecutive waves N and N+1 both have files in the same directory |
| G003_COMMIT_MISSING | Commit Missing | error | `validateI5CommitBeforeReport`, `ValidateCompletionReportClaims` | Completion report has empty `commit` or the literal string `uncommitted`; or the commit SHA does not exist in the repository |
| A003_COMPLETION_REPORT_MISSING | Completion Report Missing | fatal | `ValidateCompletionReportClaims` | A `files_created` entry does not exist on disk |
| V002 (cross-check) | Files Changed Ownership | fatal | `ValidateCompletionReportClaims` | A file in `files_changed` is not in the agent's owned files and is not a frozen scaffold path |

---

## Invariants (I-Series)

**I1 — Disjoint Ownership (V002):** Within a single wave, each file may be owned by exactly one agent. The same file may appear in multiple waves (sequential modification by different agents across waves is allowed).

**I2 — Prior-Wave Dependencies (V003):** An agent may only declare dependencies on agents assigned to strictly earlier waves. Dependencies on agents in the same wave or a later wave are forbidden. This applies both to `agent.dependencies[]` in the wave structure and to `file_ownership[].depends_on` entries.

**I3 — Sequential Wave Numbering (V004):** Wave numbers must be 1, 2, 3, … with no gaps or duplicates. The validator checks that `waves[i].number == i + 1` for all indices.

**I4 — Required Top-Level Fields (V005, V039):** `title`, `feature_slug`, and `verdict` must be non-empty. `feature_slug` must be kebab-case (invalid format emits V039). `verdict` must be one of `SUITABLE`, `NOT_SUITABLE`, `SUITABLE_WITH_CAVEATS` (invalid value emits V039). V018 is used only for program-level slug validation.

**I5 — File Ownership Completeness (V006/V019):** Every file listed in any `agent.files[]` must have a corresponding `file_ownership` entry. Agents must also commit before submitting a completion report (V — commit field non-empty and not `"uncommitted"`).

**I6 — No Dependency Cycles (V007):** The agent dependency graph must be acyclic. Detected via DFS with a recursion stack; only the first cycle found is reported.

---

## State Machine (SM01)

The `state` field must be one of these `ProtocolState` constants (V008):

| State | Meaning |
|-------|---------|
| `INTERVIEWING` | Requirements interview in progress |
| `SCOUT_PENDING` | Scout has not yet run |
| `SCOUT_VALIDATING` | Scout output under review |
| `REVIEWED` | Manifest approved for execution |
| `SCAFFOLD_PENDING` | Scaffold step not yet complete |
| `WAVE_PENDING` | Wave ready to execute |
| `WAVE_EXECUTING` | Wave agents running |
| `WAVE_MERGING` | Wave branches being merged |
| `WAVE_VERIFIED` | Wave passed quality gates |
| `BLOCKED` | Manual intervention required |
| `COMPLETE` | All waves done |
| `NOT_SUITABLE` | Feature deemed unfit for SAW |

Empty/omitted `state` is accepted for backward compatibility. An unrecognized value (anything not in the table above) triggers V008.

For PROGRAM manifests, the valid states are: `PLANNING`, `VALIDATING`, `REVIEWED`, `SCAFFOLD`, `TIER_EXECUTING`, `TIER_VERIFIED`, `COMPLETE`, `BLOCKED`, `NOT_SUITABLE`.

---

## Wave Numbering

Waves are validated by two separate checks that work together:

1. **Schema check** (`validateNestedRequiredFields`): `waves[i].number > 0` and `waves[i].agents` is non-empty.
2. **Ordering check** (`validateI3WaveOrdering`): `waves[i].number == i + 1` — the ordinal position must equal the wave number. This prevents gaps, out-of-order definitions, and duplicate numbers.

A manifest with `waves` defined as `[1, 3, 4]` would pass the schema check but fail the ordering check at index 1 (expected 2, got 3).

---

## File Ownership Completeness

Three separate validators work in combination:

- **I5 (V006/V019):** Checks that every file in `waves[].agents[].files[]` has a `file_ownership` entry.
- **Cross-field (V038):** Checks the reverse — that each `file_ownership` entry's agent appears in some wave, and that `file_ownership.wave` matches an actual wave number. Also checks that `agent.files[]` entries match their `file_ownership` counterpart with the correct `agent` and `wave` combination.
- **File existence (V041):** When `repoPath` is provided, files with `action=modify` must actually exist on disk. Files for other repositories (when `repo:` does not match `repoPath` basename) are skipped.

---

## Dependency Cycle Detection

`validateI6NoCycles` builds an adjacency list from `agent.dependencies[]` across all waves and runs DFS. The recursion stack tracks the current path; when a node already on the stack is encountered again, the cycle path is extracted from the saved path slice. Only the first cycle is reported. This operates on agent IDs, not file paths.

---

## Scaffold Validation

`ValidateScaffolds` checks each entry in `scaffolds[]` and returns a `ScaffoldStatus` per entry:

- `status` starting with `"committed"` → valid (includes `"committed (abc123)"` variants)
- `status == "pending"` → invalid, wave execution is blocked
- `status` starting with `"FAILED"` → invalid
- empty `status` or any other value → invalid

`AllScaffoldsCommitted` is the gate: wave execution cannot proceed until all scaffold entries are valid.

---

## Interface Contract Enforcement

`interface_contracts[]` entries are validated by `validateNestedRequiredFields` (V005): each entry must have non-empty `name`, `definition`, and `location`.

At the PROGRAM level, `ValidateProgramImportMode` enforces **P2 (V030)**: if a `program_contracts[]` entry has `freeze_at` pointing to a completed tier (all IMPLs in that tier have status `complete`), then no subsequent IMPL may redefine a contract with the same `name`. A frozen contract is immutable once its tier is complete.

Wiring obligations declared in `wiring[]` are enforced post-merge by `ValidateWiringDeclarations` (P007): each `symbol` must appear as a call expression in the `must_be_called_from` file, verified via Go AST for `.go` files, prop-aware line scan for `.tsx`/`.ts` files, and substring scan for all others.

---

## Pre-Mortem Risk Levels

`pre_mortem.overall_risk` must be `low`, `medium`, or `high` when present (V015). Empty is allowed for backward compatibility.

Individual `pre_mortem.rows[]` have two additional enum fields:

- `likelihood`: `low`, `medium`, `high` (or empty)
- `impact`: `low`, `medium`, `high` (or empty)

Both are validated by `validatePreMortemRowEnums` under V036. Each row must also have non-empty `scenario` and `mitigation` fields (V005).

---

## Reactions Config Validation

`reactions` is an optional block. When present, five named entries may appear: `transient`, `timeout`, `fixable`, `needs_replan`, `escalate`. For each entry that is non-nil:

- `action` is required (V005) and must be one of: `retry`, `send-fix-prompt`, `pause`, `auto-scout` (V036)
- `max_attempts` must be `>= 0` (V005 — despite using the required-fields code, this is a range check)

---

## Multi-Repo Consistency

Two checks apply when `file_ownership` entries use explicit `repo:` fields:

**MR01 (V020):** Either all entries have `repo:` or none do. Mixing explicit and implicit repo tags is disallowed because the web GUI uses the presence of any `repo:` field to detect multi-repo IMPLs.

**MR02 (V040):** When entries span two or more distinct `repo:` values, every `quality_gates.gates[]` entry must also have a `repo:` field. Without `repo:` scoping, gates run against all repositories including docs-only repos that have no build system.

---

## Agent Scope and LOC Budget

**V047 (Trivial Scope):** An IMPL declaring itself `SUITABLE` or `SUITABLE_WITH_CAVEATS` with exactly 1 total agent owning exactly 1 file is rejected. SAW provides no parallelization benefit at this scope; the change should be made directly. Retry IMPLs (slugs containing `-retry-`) are exempt.

**W001 (Scope Large):** Warnings are emitted when any agent owns more than 8 files total or creates more than 5 new files. These are advisory; they do not block execution.

**V048 (LOC Budget):** When `repoPath` is provided, the lines of all `action=modify` files per agent are summed. Agents exceeding 2000 total lines receive a blocking error listing the largest files by LOC. Individual `file_ownership` entries may set `v048_exempt: true` to exclude specific files from the LOC count (e.g., generated files or large configuration files that require only mechanical edits).

---

## Typed Block Validation (E16)

The IMPL Markdown document (not the YAML manifest) is validated separately by `ValidateIMPLDoc`. This validator reads the raw file to preserve line numbers.

**E16A:** Once any `type=`-annotated fenced block is found, all three core blocks must be present: `impl-file-ownership`, `impl-dep-graph`, `impl-wave-structure`.

**E16B — Block structural content:**

| Block type | Required content |
|------------|-----------------|
| `impl-file-ownership` | Header row containing `\| File \|`; at least one data row; each data row ≥ 4 pipe characters; agent IDs must match `^[A-Z][2-9]?$` |
| `impl-dep-graph` | At least one `Wave N` header line; at least one `[X]` agent reference; each agent block must contain `✓ root` or `depends on:` |
| `impl-wave-structure` | At least one `Wave N:` line; at least one `[X]` agent reference; all agent IDs must match `^[A-Z][2-9]?$` |
| `impl-completion-report` | Fields: `status:`, `worktree:`, `branch:`, `commit:`, `files_changed:`, `interface_deviations:`, `verification:`; `status` value must be `complete`, `partial`, or `blocked` (not the template placeholder `complete \| partial \| blocked`) |

**E16C (warning):** A plain fenced block (no `type=` annotation) that contains both `[A]`-style agent references and the word `Wave` is flagged as a likely out-of-band dep graph. Recommendation: convert to `\`\`\`yaml type=impl-dep-graph`.

**Agent ID format (agent-id):** All agent IDs across typed blocks must match `^[A-Z][2-9]?$`. Generation-1 agents use bare letters (`A`, `B`, `C`); multi-generation agents append a digit 2–9 (`A2`, `B3`). Common invalid patterns: `[A1]` (digit 1 is reserved — use `[A]`), `[AB]` (multi-letter), `[a]` (lowercase), `[A10]` (two digits).

---

## Program-Level Rules

| Rule | Code | Description |
|------|------|-------------|
| P1 — IMPL Independence | V035 | No IMPL may depend on another IMPL in the same tier (within `impls[].depends_on`) |
| P1 — File Disjointness | V029 | No two IMPLs in the same tier may claim the same file in their `file_ownership` tables (checked in import mode only) |
| P2 — Frozen Contract | V030 | IMPLs may not redefine a `program_contract` whose `freeze_at` tier has fully completed |
| Tier Ordering | V026 | IMPL-A `depends_on` IMPL-B → A's tier number must be strictly greater than B's |
| Tier Consistency | V025 | Every IMPL slug must appear in exactly one tier; tiers may only reference defined IMPL slugs |
| Dependency Validity | V028 | `depends_on` references must be slugs defined in `impls[]` |
| Consumer Validity | V027 | `program_contracts[].consumers[].impl` must reference a slug defined in `impls[]` |
| Completion Bounds | V033, V034 | `tiers_complete ≤ tiers_total`, `impls_complete ≤ impls_total`, `impls_total == len(impls)` |
| IMPL State Consistency | V032 | IMPL program status `reviewed` requires IMPL doc state of REVIEWED or later; status `complete` requires IMPL doc state COMPLETE |
| IMPL File Presence | V031 | IMPLs with status `reviewed` or `complete` must have a corresponding YAML file on disk |

---

## Completion Report Cross-Validation

`ValidateCompletionReportClaims` (called separately from `FullValidate`) performs live git-backed checks:

- **Commit existence:** `git cat-file -e <sha>` — fatal if commit does not exist in the repo.
- **Branch reachability:** commit must be reachable from the agent's expected branch (`saw/{slug}/wave{N}-agent-{ID}`); failure is a warning, not a fatal.
- **Files changed ownership:** every file in `files_changed` must be in the agent's owned files or in the frozen scaffold paths; violation is fatal (V002).
- **Files created existence:** every file in `files_created` must exist on disk; violation is fatal (A003).
- **Worktree path existence:** if `worktree` is set, the path must exist on disk; absence is a warning (may have been cleaned up).

Gate input validation (`ValidateGateInputs`) separately compares files reported by agents against files actually changed in their worktree branches via `git diff --name-only`.

---

## Auto-Fix (`sawtools validate --fix`)

When `--fix` is passed (or `AutoFix: true` programmatically), `FullValidate` applies two corrections before running validation:

1. **Gate type normalization (`FixGateTypes`):** Any `quality_gates.gates[].type` value not in the valid set (`build`, `lint`, `test`, `typecheck`, `format`, `custom`) is rewritten to `"custom"`. The manifest is saved after correction.
2. **Unknown key stripping (`StripUnknownKeys`):** Unrecognized top-level YAML keys are removed from the raw file. The manifest is re-loaded after stripping.

The `Fixed` field in the output reports how many corrections were applied. After auto-fix, the manifest is re-validated normally; any remaining issues are reported as errors.

The `--fix` flag is also available on `sawtools pre-wave-validate` and is used internally by `sawtools finalize-scout`.
