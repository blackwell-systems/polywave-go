# SAW Error Code Reference

Error codes appear in the `code` field of every `SAWError` object returned by sawtools commands. They identify the exact failure condition so agents and users can diagnose and fix problems without reading source.

## Where codes appear

- `sawtools validate` — structural and schema validation of IMPL docs (V-series, W-series)
- `sawtools prepare-wave` — pre-wave setup: worktree creation, isolation, baseline gates (G-series, N-series, B-series)
- `sawtools finalize-wave` — post-wave merge, gate execution, completion validation (B-series, A-series, N-series)
- `sawtools validate-program` — PROGRAM manifest validation (V-series subset, N-series)
- All commands — engine and protocol errors (P-series, T-series)

## Prefix system

| Prefix | Domain | Severity |
|--------|--------|---------|
| V | Validation (IMPL/PROGRAM structure, schema, fields) | error (blocking) |
| W | Warnings (advisory, never block execution) | warning |
| B | Build and gate failures | error or fatal |
| G | Git errors (worktree, merge, commit) | error or fatal |
| A | Agent errors (launch, timeout, completion) | error or fatal |
| N | Engine/orchestration errors | error or fatal |
| P | Protocol invariant violations | error or fatal |
| T | Tool/parse errors | error or fatal |

---

## V — Validation

Emitted by `sawtools validate` and pre-flight checks inside `prepare-wave` and `finalize-wave`. All V codes are blocking errors unless noted.

| Code | Description | Fix |
|------|-------------|-----|
| `V001_MANIFEST_INVALID` | IMPL doc fails structural parsing or has an unrecoverable format error. | Check YAML syntax and top-level structure. The error message identifies the specific field or section. |
| `V002_DISJOINT_OWNERSHIP` | A file is owned by more than one agent in the same wave, or a completion report lists a file not in the agent's owned set. | Each file may only appear once per wave in `file_ownership`. Move conflicting files to separate waves or assign them to a single agent. |
| `V003_SAME_WAVE_DEPENDENCY` | An agent depends on another agent in the same wave, or on an unknown agent. | Agent dependencies must reference agents in prior (lower-numbered) waves only. Move the dependency target to an earlier wave or remove the dependency. |
| `V004_WAVE_NOT_1INDEXED` | Wave numbers are not sequential starting from 1 (e.g., gap or out-of-order). | Renumber waves as 1, 2, 3, … with no gaps. |
| `V005_REQUIRED_FIELDS_MISSING` | A required field is absent or empty. The `field` property identifies which field. | Add the missing field. Required top-level fields include `title`, `feature_slug`, `verdict`, and others identified in the error message. |
| `V006_FILE_OWNERSHIP_INCOMPLETE` | An agent's `files` list contains a path not present in `file_ownership`. | Every file listed under a wave agent must also have a corresponding `file_ownership` entry. Add the missing row. |
| `V007_DEPENDENCY_CYCLE` | A cycle exists in the dependency graph between agents or IMPLs. | Remove or restructure dependencies to eliminate the cycle. The error message names the agents involved. |
| `V008_INVALID_STATE` | The IMPL `state` field contains a value not in the allowed state machine. | Set `state` to one of: `SCOUT_PENDING`, `SCOUT_VALIDATING`, `REVIEWED`, `SCAFFOLD_PENDING`, `WAVE_PENDING`, `WAVE_EXECUTING`, `WAVE_MERGING`, `WAVE_VERIFIED`, `BLOCKED`, `COMPLETE`, `NOT_SUITABLE`. |
| `V009_INVALID_AGENT_ID` | An agent ID does not match the required pattern (one uppercase letter, optionally followed by a digit 2–9, e.g. `A`, `B2`). | Use single uppercase letters (`A`–`Z`) as agent IDs, or append a digit 2–9 for agents beyond the 26-letter limit (e.g. `A2`, `B3`). |
| `V010_INVALID_GATE_TYPE` | A quality gate `type` field is not one of the allowed values. | Use one of: `build`, `lint`, `test`, `typecheck`, `format`, `custom`. |
| `V011_INVALID_ACTION_ENUM` | A `file_ownership` entry has an invalid `action` value. | Use one of: `new`, `modify`, `delete`. |
| `V012_DUPLICATE_KEY` | The YAML source contains a duplicate key within the same mapping. | Remove the duplicate key. YAML silently drops one value; this error prevents silent data loss. |
| `V013_UNKNOWN_KEY` | The YAML source contains a key that is not part of the IMPL schema. | Remove or correct the unrecognized key. Common cause: typos in field names (e.g. `vertict` instead of `verdict`). |
| `V014_INVALID_SCAFFOLD_STATUS` | A scaffold entry has an invalid `status` field. | Check the scaffold status value against allowed enum values listed in the error message. |
| `V015_INVALID_PRE_MORTEM_RISK` | `pre_mortem.overall_risk` is not one of the allowed values. | Set `overall_risk` to `low`, `medium`, or `high`. |
| `V016_JSONSCHEMA_FAILED` | The IMPL doc failed JSON Schema validation. | The error message and `field` identify the violating path. Fix the field value or structure to comply with the schema. |
| `V017_SLUG_MISMATCH` | The `feature_slug` does not match the expected value derived from the IMPL filename or program context. | The IMPL filename must match `IMPL-{feature_slug}.yaml`. Rename the file or correct `feature_slug` so they agree. |
| `V018_INVALID_SLUG_FORMAT` | A slug (IMPL or program) contains characters not allowed by the kebab-case format. | Slugs must be lowercase letters, digits, and hyphens only. No leading, trailing, or consecutive hyphens. |
| `V019_ORPHAN_FILE` | An agent's `files` list contains a path that is not in the `file_ownership` table. | Add a `file_ownership` row for the orphaned file, or remove it from the agent's `files` list. |
| `V020_INCONSISTENT_REPO` | Some `file_ownership` entries have an explicit `repo:` field and others do not, in the same IMPL. | When any entry uses `repo:`, all entries must have an explicit `repo:` field. Add `repo:` to every row that is missing it. |
| `V021_KNOWN_ISSUE_MISSING_TITLE` | A `known_issues` entry has an empty `title` field. | Add a non-empty `title` to every `known_issues` entry. |
| `V022_INVALID_FAILURE_TYPE` | A completion report `failure_type` is not one of the allowed enum values. | Set `failure_type` to one of: `transient`, `fixable`, `needs_replan`, `escalate`, `timeout`. Empty is valid when `status` is `complete`. |
| `V023_INVALID_MERGE_STATE` | The IMPL `merge_state` field contains an invalid value. | Check the allowed values for `merge_state` listed in the error message. |
| `V024_PROGRAM_INVALID` | A PROGRAM manifest has structural or semantic errors. | The error message identifies the specific violation. Check the `impls`, `tiers`, and `completion` sections. |
| `V025_TIER_MISMATCH` | A tier references an IMPL slug not defined in the `impls` section, or an IMPL is not assigned to any tier, or is assigned to more than one tier. | Every IMPL slug must appear in exactly one tier. Ensure `tiers[*].impls` entries match defined `impls[*].slug` values. |
| `V026_TIER_ORDER_VIOLATION` | An IMPL depends on another IMPL in a later (higher-numbered) tier. | Dependencies must only reference IMPLs in lower-numbered tiers. Move the dependency target to an earlier tier or restructure the dependency. |
| `V027_INVALID_CONSUMER` | A consumer reference in an interface contract is invalid. | Check the consumer value format and ensure it references a valid IMPL slug or agent. |
| `V028_INVALID_DEPENDENCY` | An IMPL `depends_on` references a slug that is not defined in the program. | Ensure all `depends_on` entries reference valid IMPL slugs defined in the `impls` section. |
| `V029_P1_FILE_OVERLAP` | Two IMPLs in the same tier both claim ownership of the same file. | IMPLs executing in the same tier must have disjoint file sets (P1 invariant). Move the file to one IMPL or push one IMPL to a later tier. |
| `V030_P2_CONTRACT_REDEFINITION` | An IMPL redefines an interface contract that was frozen by a previous tier. | Interface contracts frozen in prior tiers cannot be redefined. Create a new contract or extend the existing one from a later tier. |
| `V031_IMPL_FILE_MISSING` | A PROGRAM references an IMPL slug with status `reviewed` or `complete`, but the corresponding `IMPL-{slug}.yaml` file cannot be found on disk. | Create the IMPL file at `docs/IMPL/IMPL-{slug}.yaml` (or `docs/IMPL/complete/` for completed IMPLs) or correct the slug in the PROGRAM manifest. |
| `V032_IMPL_STATE_MISMATCH` | An IMPL's status in the PROGRAM manifest does not match the `state` field inside the IMPL doc itself. | Synchronize the two: if the PROGRAM says `complete`, the IMPL doc must have `state: COMPLETE`. Update whichever is out of date. |
| `V033_COMPLETION_BOUNDS` | A PROGRAM `completion` counter (`tiers_complete` or `impls_complete`) exceeds its corresponding total. | Reduce the counter or increase the total to reflect actual progress. |
| `V034_IMPLS_TOTAL_MISMATCH` | `completion.impls_total` does not equal the number of entries in the `impls` section. | Set `impls_total` to equal the actual count of `impls` entries. |
| `V035_P1_VIOLATION` | An IMPL within a tier depends on another IMPL in the same tier (P1 independence rule). | IMPLs in the same tier must be fully independent. Move the dependency target to a prior tier, or merge the two IMPLs. |
| `V036_INVALID_ENUM` | A field contains a value not in its allowed enum set. The `field` property identifies the exact location. | Replace the value with one of the allowed values listed in the error message. Common locations: `completion_reports[*].status` (must be `complete`, `partial`, or `blocked`), `impls[*].status`, gate `type` fields. |
| `V037_INVALID_PATH` | A file path field contains an invalid format (e.g., absolute path where relative is required, or illegal characters). | Use relative paths from the repo root. The error message identifies the offending path. |
| `V038_CROSS_FIELD` | Two or more fields are inconsistent with each other (e.g., a file_ownership agent does not appear in any wave, or a NOT_SUITABLE verdict has non-empty waves). | The error message describes which fields conflict. Align them so they are mutually consistent. |
| `V039_INVALID_FIELD_VALUE` | A field value is syntactically invalid for its type (e.g., `feature_slug` is not kebab-case, `verdict` is not a recognized value). | The error message identifies the field and the allowed values. |
| `V040_UNSCOPED_GATE` | A multi-repo IMPL has a quality gate with no `repo:` field. Without scoping, the gate command runs in every repo, including those that may not support that build system. | Add `repo: <repo-name>` to every gate in `quality_gates.gates` when the IMPL touches two or more repos. |
| `V041_FILE_MISSING` | A `file_ownership` entry with `action: modify` references a file that does not exist on disk in the resolved repo directory. | Either change `action` to `new` if the file is being created, or ensure the file exists at the specified path before running validate. |
| `V042_INVALID_WORKTREE_NAME` | A completion report's `branch` or `worktree` field does not follow the required naming pattern. | Branches must follow `wave{N}-agent-{ID}` or `saw/{slug}/wave{N}-agent-{ID}` format. Worktree paths must contain `wave{N}-agent-{ID}` as a path segment. Solo-wave agents are exempt. |
| `V043_INVALID_VERIFICATION` | A completion report `verification` field does not contain the word `PASS` or `FAIL` as a standalone word. | Write a verification string that includes `PASS` or `FAIL`, e.g. `PASS — all tests green` or `FAIL (build error in pkg/api)`. |
| `V044_MISSING_CHECKLIST` | New API handlers (`pkg/api/*_handler.go`) or React components (`web/src/components/*.tsx`) are declared in `file_ownership` but `post_merge_checklist` is absent or empty. | Add a `post_merge_checklist` section listing integration steps needed after merge. This is a warning — it does not block execution. |
| `V045_REPO_MISMATCH` | Every single `action: modify` file in the IMPL is missing from disk. This indicates the IMPL is being validated against the wrong repository directory. | Run validate with the correct `--repo-dir` pointing to the repository this IMPL targets. |
| `V046_PARSE_ERROR` | The IMPL YAML cannot be parsed (syntax error, invalid encoding, or duplicate key detected during raw parsing). | Fix the YAML syntax. The error message includes the line or key causing the failure. |
| `V047_TRIVIAL_SCOPE` | The IMPL is marked `SUITABLE` but has exactly 1 agent owning exactly 1 file. SAW provides no parallelization benefit at this scope. | Make the change directly without using SAW. Delete the IMPL doc and edit the file manually. If SAW is genuinely needed (e.g. for the quality gate infrastructure), split the work into multiple agents or mark it `NOT_SUITABLE`. |

---

## W — Warnings

Advisory only. Never block execution.

| Code | Description | Fix |
|------|-------------|-----|
| `W001_AGENT_SCOPE_LARGE` | An agent owns more than 8 files total, or creates more than 5 new files. Large agent scope increases the risk of conflicts and agent context exhaustion. | Consider splitting the agent's work into two agents across two waves. This is advisory — the IMPL will still execute. |
| `W002_COMPLETION_VERIFY` | An agent has no commits on its wave branch after the wave completed. Cross-repo agents are exempt if their completion report includes a commit SHA. | Verify the agent actually committed its work. If the agent ran against a different repo (cross-repo pattern), ensure the completion report's `commit` field is populated. |

---

## B — Build and Gate

Emitted during gate execution in `prepare-wave` (baseline gates) and `finalize-wave` (pre-merge and post-merge gates). Also appears in `parsed_errors` within gate results when the errparse package extracts structured errors from compiler/linter output.

| Code | Description | Fix |
|------|-------------|-----|
| `B001_BUILD_FAILED` | A build gate command exited non-zero. Appears in `parsed_errors` when the output contains structured build errors. | Fix the compilation errors identified in `parsed_errors`. Re-run the gate after fixing. |
| `B002_TEST_FAILED` | A test gate command exited non-zero. | Fix the failing tests. The `parsed_errors` field lists individual test failures with file and line references. |
| `B003_LINT_FAILED` | A lint gate command exited non-zero. | Fix the lint violations listed in `parsed_errors`. |
| `B004_FORMAT_CHECK_FAILED` | A format gate command exited non-zero (file formatting does not match expected). | Run the formatter locally (e.g. `gofmt -w .`, `prettier --write .`) and commit the result. |
| `B005_GATE_TIMEOUT` | A gate command exceeded its configured timeout. | The command ran too long. Optimize the command, increase the timeout in `quality_gates.gates[*].timeout`, or split large test suites. |
| `B006_GATE_COMMAND_MISSING` | A gate entry has no `command` field or an empty command. | Add a `command` to every gate in `quality_gates.gates`. |
| `B007_STUB_DETECTED` | A stub or placeholder implementation was detected in the code under review. | Replace the stub with a real implementation before finalizing the wave. |
| `B008_GATE_INPUT_INVALID` | The inputs to gate validation are inconsistent (e.g. the requested wave number does not exist in the manifest). | Ensure the wave number passed to `finalize-wave` matches a wave defined in the IMPL. Check the IMPL has not been manually edited to remove the wave. |

---

## G — Git

Emitted during worktree operations, merge, and commit verification.

| Code | Description | Fix |
|------|-------------|-----|
| `G001_WORKTREE_CREATE_FAILED` | Failed to create a git worktree for an agent. The error message includes the underlying git error. | Common causes: missing parent directory, git version too old, or a corrupt git repo state. Run `git worktree prune` and retry. Check disk space and git version. |
| `G002_MERGE_CONFLICT` | A merge conflict was detected when merging an agent's branch into the base. | Resolve the conflict manually in the agent's worktree, commit the resolution, and re-run `finalize-wave`. |
| `G003_COMMIT_MISSING` | An agent's completion report references a commit SHA that does not exist in the repository, or the validation step found no commits on the agent's wave branch. | Ensure the agent committed its work and the SHA in the completion report is correct. If the branch was cleaned up, the agent must re-run. |
| `G004_BRANCH_EXISTS` | The agent's wave branch already exists and has not been merged into HEAD. | Delete the stale branch manually (`git branch -D <branch>`) or merge it before re-running `prepare-wave`. |
| `G005_DIRTY_WORKTREE` | The worktree has uncommitted changes that prevent the operation. | Commit or stash changes in the affected worktree before proceeding. |
| `G006_HOOK_INSTALL_FAILED` | Installing a git hook failed. | Check permissions on `.git/hooks/` and ensure the hook file is writable. |
| `G007_WORKTREE_CLEANUP` | A worktree cleanup operation failed (stale worktree removal could not complete). | Run `git worktree prune` manually. If the worktree directory still exists, remove it with `git worktree remove --force <path>`. |

---

## A — Agent

Emitted during wave execution when monitoring agent lifecycle or validating completion reports.

| Code | Description | Fix |
|------|-------------|-----|
| `A001_AGENT_TIMEOUT` | An agent exceeded the configured execution time limit. | Increase the timeout if the task is legitimately long. Otherwise, investigate why the agent stalled (context overflow, tool failures, infinite loop). |
| `A002_STUB_DETECTED` | A stub or `TODO`/placeholder was detected in the agent's output. | The agent did not complete the implementation. Re-run the agent with instructions to finish the stubbed sections. |
| `A003_COMPLETION_REPORT_MISSING` | An agent completed but did not write a completion report, or a `files_created` entry in the completion report does not exist on disk. | Ensure the agent writes a completion report with accurate `files_created` and `files_changed` fields. Verify all listed files were actually created at the stated paths. |
| `A004_VERIFICATION_FAILED` | The agent's self-reported verification step failed (the `verification` field contains `FAIL`). | Read the agent's `verification` field for details. The agent identified a problem — review the failure and either fix it or re-run the agent. |
| `A005_AGENT_LAUNCH_FAILED` | The agent process could not be started. | Check the agent command, environment variables, and working directory. The error message includes the underlying launch failure. |
| `A006_BRIEF_EXTRACT_FAIL` | The agent brief could not be extracted from the IMPL doc for agent launch. | Verify the IMPL doc is valid and the agent ID exists in the wave structure. Run `sawtools validate` to rule out structural issues. |
| `A007_JOURNAL_INIT_FAIL` | The agent journal file could not be initialized. | Check write permissions in the repo's `.claude/` directory. |

---

## N — Engine

Emitted by the saw orchestration engine during wave preparation, execution, and finalization.

| Code | Description | Fix |
|------|-------------|-----|
| `N001_PREPARE_WAVE_FAILED` | The `prepare-wave` step failed at the engine level. | The error message describes which sub-step failed. Common causes: validation failure, worktree creation error, or baseline gate failure. Fix the underlying cause and re-run `prepare-wave`. |
| `N002_FINALIZE_WAVE_FAILED` | The `finalize-wave` step failed at the engine level (e.g. gate population or checklist injection failed). | The error message identifies the failing sub-step. Check gate configuration and re-run `finalize-wave`. |
| `N003_SCOUT_FAILED` | The scout phase failed. | The error message describes the failure. Check the scout agent output and the resulting IMPL doc for structural issues. |
| `N004_ISOLATION_VERIFY_FAILED` | An agent's isolation check failed: the agent is not running in the expected worktree branch, or the worktree path does not match the known pattern. | Ensure the agent is invoked from its designated worktree directory (`.claude/worktrees/saw/{slug}/wave{N}-agent-{ID}`). Do not run agents from the main repository checkout. |
| `N005_IMPL_NOT_FOUND` | The IMPL document could not be located at the expected path. | Verify the IMPL file exists at `docs/IMPL/IMPL-{slug}.yaml`. Pass the correct `--impl` or slug argument to the command. |
| `N006_IMPL_PARSE_FAILED` | The IMPL YAML file exists but could not be parsed. | Fix YAML syntax errors in the IMPL doc. Run `sawtools validate` for detailed error output. |
| `N007_WAVE_NOT_READY` | The requested wave cannot execute because the IMPL is not in the correct state. | The IMPL state must be `WAVE_PENDING` or `REVIEWED` before a wave can start. Use `sawtools set-state` or advance the IMPL through the correct state sequence. |
| `N008_STATE_TRANSITION` | An IMPL state transition was attempted that is not allowed by the state machine. | Valid transitions are: `SCOUT_PENDING/SCOUT_VALIDATING` → `REVIEWED` or `NOT_SUITABLE`; `REVIEWED` → `SCAFFOLD_PENDING`, `WAVE_PENDING`, `WAVE_EXECUTING`, `BLOCKED`, or `COMPLETE`; `SCAFFOLD_PENDING` → `WAVE_PENDING`, `WAVE_EXECUTING`, or `BLOCKED`; `WAVE_PENDING` → `WAVE_EXECUTING` or `BLOCKED`; `WAVE_EXECUTING` → `WAVE_MERGING`, `WAVE_VERIFIED`, `BLOCKED`, or `COMPLETE`; `WAVE_MERGING` → `WAVE_VERIFIED` or `BLOCKED`; `WAVE_VERIFIED` → `WAVE_PENDING`, `WAVE_EXECUTING`, `COMPLETE`, or `BLOCKED`; `BLOCKED` → `REVIEWED`, `WAVE_EXECUTING`, or `WAVE_PENDING`; `COMPLETE` and `NOT_SUITABLE` are terminal. |
| `N009_CONTEXT_ERROR` | An error occurred updating or reading the shared execution context (wave context file). | Check file permissions in the `.claude/` directory. If the context file is corrupt, delete it and re-run `prepare-wave`. |
| `N010_BASELINE_ERROR` | Baseline gate verification (E21A or E21B) failed before the wave started. The codebase does not pass its own quality gates from a clean state. | Fix the pre-existing failures in the codebase (build errors, test failures, etc.) before attempting a new wave. The error message lists which gates failed. |
| `N011_STALE_WORKTREE` | A stale worktree was detected but could not be cleaned up automatically. | Run `git worktree prune` and manually remove the stale directory if needed, then retry. |
| `N012_FREEZE_ERROR` | A program tier freeze operation failed (the tier's IMPLs could not be frozen for P2 contract enforcement). | Check that all IMPLs in the tier are in `COMPLETE` state before freezing. The error message identifies which tier and what failed. |
| `N013_CONFIG_NOT_FOUND` | The SAW configuration file (`saw.config.json` or `.saw/config.json`) was not found. | Create a `saw.config.json` in the repository root or specify the config path explicitly. |
| `N014_CONFIG_INVALID` | The SAW configuration file exists but contains invalid content. | Fix the JSON syntax or field values in `saw.config.json`. |
| `N015_STATUS_UPDATE_FAILED` | An IMPL status mutation could not be written to disk or committed. | Check file permissions on the IMPL doc and that the git working tree is clean enough to accept a commit. |
| `N016_TIER_GATE_FAILED` | A program-level tier gate failed — either the tier was not found or not all IMPLs in the tier are complete. | Ensure all IMPLs in the tier have `status: complete` before running the tier gate. The error message specifies the tier number and which IMPLs are incomplete. |
| `N017_PROGRAM_STATUS_FAILED` | Computing the PROGRAM status failed. | The error message describes the underlying failure. This is typically a parse or I/O issue with one of the IMPL docs referenced by the PROGRAM. |

---

## P — Protocol

Emitted when protocol invariants or execution rules are violated at the engine level. These are distinct from V-series validation errors: P codes fire during execution, not at validate time.

| Code | Description | Fix |
|------|-------------|-----|
| `P001_STATE_TRANSITION_INVALID` | An invalid state transition was attempted at the protocol layer (distinct from N008 which fires at the manifest-write layer). | Review the allowed state machine transitions documented under N008. Ensure the command sequence follows the protocol. |
| `P002_PROGRAM_VALIDATION_FAILED` | A PROGRAM manifest failed validation at the engine level. | Run `sawtools validate-program` for detailed errors, then fix the PROGRAM manifest. |
| `P003_MIGRATION_BOUNDARY_UNSAFE` | A migration boundary was unsafe to cross (e.g., attempting an operation during an active wave that would corrupt in-flight state). | Complete or abort the active wave before performing the migration. |
| `P004_DEPS_NOT_MET` | An IMPL's dependencies have not yet completed before it was scheduled for execution. | Wait for the dependency IMPLs to reach `COMPLETE` status before starting this IMPL. |
| `P005_INVARIANT_VIOLATION` | A protocol invariant was violated during execution. | The error message identifies the invariant. This is generally an internal consistency error — report it as a bug if it cannot be attributed to a user action. |
| `P006_EXECUTION_RULE` | An execution rule was violated (e.g., attempting to run a wave out of sequence). | Follow the prescribed execution order. Waves must be run in sequence; tiers in a PROGRAM must complete before the next tier begins. |
| `P007_WIRING_GAP` | A wiring gap was detected — a component or gate is defined but not connected to the execution graph. | The error message identifies what is unwired. Ensure all gates, hooks, and handlers are properly registered and referenced. |

---

## T — Tool/Parse

Emitted by the `errparse` package when parsing gate output, and by the tool runner.

| Code | Description | Fix |
|------|-------------|-----|
| `T001_TOOL_ERROR` | A compiler, linter, or test tool reported a structured error. This code appears inside `parsed_errors` on gate results — it carries the file, line, and message from the tool's output. | Fix the error at the location identified by `file` and `line` in the error object. |
| `T002_PARSE_PANIC` | The error parser itself panicked while processing tool output. | This is an internal parser bug. File an issue with the raw tool output that triggered it. |
| `T003_TOOL_NOT_FOUND` | A required external tool (e.g. for H2 command extraction) was not found. Emitted when `finalize-wave` cannot locate any valid toolchain in the target repos. | Run `sawtools extract-commands` before `finalize-wave`. Ensure the required toolchain (Go, Node, etc.) is installed and the project has the expected marker files (`go.mod`, `package.json`). |
| `T004_TOOL_TIMEOUT` | An external tool timed out. | Increase the timeout for the gate or tool invocation. If the tool is consistently slow, investigate why (e.g., large codebase, missing cache). |
