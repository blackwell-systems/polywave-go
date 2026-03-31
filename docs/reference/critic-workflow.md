# Critic Workflow (E37)

The critic is a pre-wave gate that reviews every agent brief in the IMPL doc against the actual codebase before wave execution begins. It is implemented as a single agent launched by `sawtools run-critic` (or by `engine.RunCritic` programmatically).

---

## Types

### `CriticData`

Written to `IMPLManifest.CriticReport` (`critic_report` in YAML) by `WriteCriticReview` / `WriteCriticReviewResult` after the critic agent calls `sawtools set-critic-review`.

| Field | Type | YAML key | Description |
|---|---|---|---|
| `Verdict` | `string` | `verdict` | Overall decision: `PASS` or `ISSUES` |
| `AgentReviews` | `map[string]AgentCriticReview` | `agent_reviews` | Per-agent verdicts, keyed by agent ID (e.g. `"A"`, `"B2"`) |
| `Summary` | `string` | `summary` | Human-readable summary of overall findings |
| `ReviewedAt` | `string` | `reviewed_at` | ISO 8601 timestamp of when the review completed |
| `IssueCount` | `int` | `issue_count` | Total number of issues found across all agents |

### `AgentCriticReview`

One entry per agent in the wave.

| Field | Type | YAML key | Description |
|---|---|---|---|
| `AgentID` | `string` | `agent_id` | Agent identifier matching the wave agent ID (e.g. `"A"`, `"B2"`) |
| `Verdict` | `string` | `verdict` | `PASS` or `ISSUES` for this specific agent |
| `Issues` | `[]CriticIssue` | `issues` | Problems found; omitted when `Verdict` is `PASS` |

### `CriticIssue`

One entry per discrete problem found within an agent's brief.

| Field | Type | YAML key | Description |
|---|---|---|---|
| `Check` | `string` | `check` | Verification category (see Check Categories below) |
| `Severity` | `string` | `severity` | `error` or `warning` |
| `Description` | `string` | `description` | Human-readable explanation of the problem |
| `File` | `string` | `file` | Specific file referenced in the brief, if applicable |
| `Symbol` | `string` | `symbol` | Specific function or type name that failed verification, if applicable |

---

## Verdict Values

| Constant | Value | Meaning |
|---|---|---|
| `CriticVerdictPass` | `"PASS"` | No blocking issues found across any agent brief |
| `CriticVerdictIssues` | `"ISSUES"` | One or more issues found in at least one agent brief |
| `CriticVerdictSkipped` | `"SKIPPED"` | Operator explicitly skipped the review via `--skip` / `--no-review` |

When `--skip` is used, the engine writes a synthetic `CriticData` with `Verdict: "PASS"` and `Summary: "Skipped by operator"`. The `SKIPPED` constant is defined but not written to the manifest in the current implementation; the skip path uses `PASS` to allow the gate to proceed.

---

## Severity Values

| Constant | Value | Gate behavior |
|---|---|---|
| `CriticSeverityError` | `"error"` | Blocks the wave unconditionally |
| `CriticSeverityWarning` | `"warning"` | Advisory; behavior depends on mode (see Gate Enforcement) |

---

## Check Categories

The critic agent verifies the following categories. These are set in `CriticIssue.Check`.

| Check | What is verified |
|---|---|
| `file_existence` | Files with `action:modify` must exist; files with `action:new` must not yet exist |
| `symbol_accuracy` | Named symbols (functions, types) referenced in briefs exist and match descriptions |
| `pattern_accuracy` | Implementation patterns called out in briefs match actual source patterns |
| `interface_consistency` | Interface contracts are syntactically valid and type-consistent |
| `import_chains` | All packages referenced in briefs are importable from the target module |
| `side_effect_completeness` | Registration files (init hooks, route tables, etc.) are included in `file_ownership` |

---

## E37 Gate Enforcement (`CriticGatePasses`)

`protocol.CriticGatePasses(manifest, autoMode bool) bool` is the authoritative gate decision. It is called by the orchestrator before launching a wave.

| Condition | `autoMode=true` | `autoMode=false` |
|---|---|---|
| No `critic_report` present | block | block |
| `Verdict == "PASS"` | proceed | proceed |
| `Verdict == "ISSUES"`, has `error`-severity issue | block | block |
| `Verdict == "ISSUES"`, warnings only | proceed | block (surface to user) |
| Unknown verdict (not `PASS` or `ISSUES`) | block | block |

**Auto mode** is active when the wave was launched via `--auto` (unattended execution). **Manual mode** applies to interactive `saw wave` invocations where the user can review and decide.

A warnings-only report in auto mode is treated as safe to proceed. In manual mode the gate blocks and the orchestrator surfaces the warnings to the user for a decision.

---

## How `critic_report` Is Populated

1. The orchestrator calls `sawtools run-critic <impl-path>` (or `engine.RunCritic` directly).
2. `engine.RunCritic` loads the IMPL doc, resolves repo roots, and launches a critic agent with the `critic-agent.md` prompt injected with the IMPL doc path and repo roots.
3. The critic agent performs its checks and calls `sawtools set-critic-review <impl-path> --verdict <V> --summary <S> --issue-count <N> --agent-reviews <JSON>`.
4. `set-critic-review` parses the JSON array of `AgentCriticReview` objects, builds a `CriticData`, and calls `protocol.WriteCriticReview` to persist it to `IMPLManifest.CriticReport`.
5. `engine.RunCritic` reloads the manifest, reads `CriticReport`, and returns a `RunCriticResult` to the caller.

If the critic agent completes but no `critic_report` was written, `RunCritic` returns an error. This is treated as a gate failure.

---

## `sawtools run-critic` CLI

```
sawtools run-critic <impl-path> [flags]
```

`<impl-path>` must be an absolute path to an existing IMPL YAML file.

| Flag | Default | Description |
|---|---|---|
| `--model <model>` | `""` (inherits default) | Model override for the critic agent (e.g. `claude-opus-4-6`) |
| `--timeout <minutes>` | `20` | Maximum runtime for the critic agent in minutes |
| `--no-review` | `false` | Skip the review; write a synthetic PASS result immediately |
| `--skip` | `false` | Alias for `--no-review` |

**Exit codes:**
- `0` — verdict is `PASS` (or skipped)
- `1` — verdict is `ISSUES`; error message directs operator to correct the IMPL doc before proceeding

**SAW repo resolution** (for loading `critic-agent.md`):
1. `--saw-repo` option (if present)
2. `$SAW_REPO` environment variable
3. `~/code/scout-and-wave` (default fallback)

If `critic-agent.md` cannot be loaded, a minimal inline prompt is used as a fallback.

---

## `sawtools set-critic-review` CLI

Used by critic agents to write their output. Not intended for direct human use.

```
sawtools set-critic-review <impl-path> \
  --verdict <PASS|ISSUES> \
  --summary <text> \
  [--issue-count <N>] \
  [--agent-reviews <JSON>]
```

`--verdict` and `--summary` are required. `--agent-reviews` accepts a JSON array of `AgentCriticReview` objects:

```json
[
  { "agent_id": "A", "verdict": "PASS", "issues": [] },
  {
    "agent_id": "B",
    "verdict": "ISSUES",
    "issues": [
      {
        "check": "symbol_accuracy",
        "severity": "error",
        "description": "Function Foo does not exist in pkg/bar/bar.go",
        "file": "pkg/bar/bar.go",
        "symbol": "Foo"
      }
    ]
  }
]
```

On success, prints a JSON confirmation object with `verdict`, `issue_count`, `reviewed_at`, and `saved: true`.

---

## Validation Rules for Critic Agents

When implementing a critic agent, these rules govern which checks to apply:

| Scenario | Rule |
|---|---|
| File in `file_ownership` with `action:new` | Skip "file does not exist" errors; file will be created |
| File in `file_ownership` with `action:modify` | Enforce existence check |
| File not in `file_ownership` (external) | Enforce existence check |
| Symbol defined in a file with `action:new` | Skip "symbol does not exist" errors; symbol will be created |
| Symbol in existing file (`action:modify` or external) | Enforce existence check |
| Cross-file reference to same-wave agent's file | Defer (coordination concern, not verification) |
| Cross-file reference to external file | Enforce existence |

Helper functions `IsNewFile`, `IsSymbolInNewFile`, and `GetAgentNewFiles` in `pkg/protocol/critic.go` implement these rules.
