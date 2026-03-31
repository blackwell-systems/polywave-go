# Completion Reports

Completion reports are the structured records that wave agents write at the end of their work. They are stored in `IMPLManifest.CompletionReports` (YAML key: `completion_reports`), a `map[string]CompletionReport` keyed by agent ID.

---

## `CompletionReport` Schema

| Field | Type | YAML key | Required | Description |
|---|---|---|---|---|
| `Status` | `CompletionStatus` | `status` | yes | Outcome of the agent's work: `complete`, `partial`, or `blocked` |
| `Worktree` | `string` | `worktree` | no | Absolute path to the agent's git worktree |
| `Branch` | `string` | `branch` | no | Branch name used for the agent's work |
| `Commit` | `string` | `commit` | no | SHA of the final commit in the agent's branch |
| `FilesChanged` | `[]string` | `files_changed` | no | Absolute or repo-relative paths of existing files modified |
| `FilesCreated` | `[]string` | `files_created` | no | Absolute or repo-relative paths of new files created |
| `InterfaceDeviations` | `[]InterfaceDeviation` | `interface_deviations` | no | Deviations from planned interface contracts; omit when none |
| `OutOfScopeDeps` | `[]string` | `out_of_scope_deps` | no | Packages or symbols consumed that were not in the agent's brief scope |
| `TestsAdded` | `[]string` | `tests_added` | no | Test function names or file paths added during this agent's work |
| `Verification` | `string` | `verification` | no | Free-text record of what the agent ran to verify its work (commands, test output snippets) |
| `FailureType` | `string` | `failure_type` | no | Populated when `Status` is `partial` or `blocked`; classifies the failure mode |
| `Notes` | `string` | `notes` | no | Free-text notes for the orchestrator or human reviewer |
| `DedupStats` | `*DedupStats` | `dedup_stats` | no | File-read deduplication metrics for the agent's session |
| `Repo` | `string` | `repo` | no | Repository root for cross-repo waves; omit for single-repo |

---

## `Status` Values (`CompletionStatus`)

| Value | Constant | Meaning |
|---|---|---|
| `"complete"` | `StatusComplete` | Agent finished all assigned work successfully |
| `"partial"` | `StatusPartial` | Agent completed some but not all assigned work |
| `"blocked"` | `StatusBlocked` | Agent could not make progress; work was not completed |

---

## `files_changed` vs `files_created`

| Field | When to populate |
|---|---|
| `files_changed` | Files that existed before the agent started and were modified |
| `files_created` | Files that did not exist before the agent started and were created by the agent |

Agents should not include files in both lists for the same path. A file that was created and then immediately modified within the same agent session belongs in `files_created` only.

---

## `InterfaceDeviation` Schema

Each entry in `interface_deviations` represents one deviation from a planned `interface_contracts` entry in the IMPL doc.

| Field | Type | YAML key | Description |
|---|---|---|---|
| `Description` | `string` | `description` | What changed and why, relative to the contract |
| `DownstreamActionRequired` | `bool` | `downstream_action_required` | `true` when other agents or consumers must update their usage due to this deviation |
| `Affects` | `[]string` | `affects` | Agent IDs or file paths that are downstream of this deviation |

The `downstream_action_required` flag is the primary signal used by the orchestrator and human reviewers to determine whether a deviation requires follow-up work in subsequent waves.

---

## `failure_type` Values

`failure_type` is a free-form string but the following values are used by convention and are recognized by the E19 reactions system:

| Value | Meaning |
|---|---|
| `transient` | Transient infrastructure failure (rate limit, timeout, network error) |
| `timeout` | Agent exceeded its allotted execution time |
| `fixable` | Agent hit a correctable code or logic error; a fix prompt may resolve it |
| `needs_replan` | Task requirements are unclear or contradictory; Scout-level replanning needed |
| `escalate` | Problem requires human intervention beyond automated retry |

The `ReactionsConfig` on the IMPL manifest can override how the orchestrator responds to each `failure_type` value. Absent a `reactions` block, E19 hardcoded defaults apply.

---

## `out_of_scope_deps`

Lists packages or symbols that the agent consumed but that were not listed in the agent brief's scope. Agents should populate this when they discover they need to import or call something outside their assigned files. This field is informational; it signals to the Scout or orchestrator that the IMPL doc's scope may need adjustment.

---

## `tests_added`

Lists test function names or test file paths added during the agent's work. Convention is to use the fully qualified test function name (e.g. `TestFoo_BarCase`) or the file path relative to the repo root (e.g. `pkg/engine/foo_test.go`). Mixing both forms in one report is acceptable.

---

## `verification`

Free-text block recording how the agent verified its work. Agents should include:

- Commands run (e.g. `go build ./...`, `go test ./pkg/engine/...`)
- Summary of test output (pass/fail counts)
- Any lint or typecheck invocations

This field is not machine-parsed; it exists for human review and audit.

---

## `DedupStats` Schema

File-read deduplication metrics recorded by the agent's session.

| Field | Type | YAML key | Description |
|---|---|---|---|
| `Hits` | `int` | `hits` | Number of file reads served from cache (duplicate reads avoided) |
| `Misses` | `int` | `misses` | Number of file reads that required a fresh disk read |
| `TokensSavedEstimate` | `int` | `tokens_saved_estimate` | Estimated token count saved by deduplication |

`DedupStats` is populated automatically by the agent runner infrastructure. Wave agents do not need to populate it manually.

---

## Agent Population Expectations

Wave agents are expected to populate the following fields before completing:

| Field | Expected |
|---|---|
| `status` | Always; must be one of the three valid values |
| `files_changed` | When any existing file was modified |
| `files_created` | When any new file was created |
| `interface_deviations` | When the agent deviated from any `interface_contracts` entry |
| `verification` | Always; include at minimum the commands run and their outcome |
| `failure_type` | When `status` is `partial` or `blocked` |
| `notes` | When there is context a reviewer needs but that does not fit other fields |
| `out_of_scope_deps` | When the agent consumed packages or symbols outside its scope |
| `tests_added` | When the agent added new test functions or test files |

The `worktree`, `branch`, `commit`, and `dedup_stats` fields are populated by the orchestrator infrastructure, not by the agent itself. Agents should not set these fields.

---

## Location in IMPL Doc

Completion reports are stored under `completion_reports` at the top level of the IMPL manifest, keyed by agent ID:

```yaml
completion_reports:
  A:
    status: complete
    files_changed:
      - pkg/engine/foo.go
    files_created: []
    tests_added:
      - TestFoo_HappyPath
    verification: "go test ./pkg/engine/... — PASS (12 tests)"
    dedup_stats:
      hits: 4
      misses: 2
      tokens_saved_estimate: 1800
  B:
    status: blocked
    failure_type: fixable
    notes: "Cannot resolve import cycle between pkg/a and pkg/b"
    files_changed: []
    files_created: []
    verification: "go build ./... — FAIL"
```

The `ErrReportNotFound` sentinel (`protocol.ErrReportNotFound`) is returned by `ParseCompletionReport` when the requested agent's key is absent from the map.
