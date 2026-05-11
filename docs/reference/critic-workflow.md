# Critic Workflow (E37)

The critic is a pre-wave gate that reviews every agent brief in the IMPL doc against the actual codebase before wave execution begins. It is implemented as a single agent launched by `polywave-tools run-critic` (or by `engine.RunCritic` programmatically).

---

## Types

### `CriticData`

Written to `IMPLManifest.CriticReport` (`critic_report` in YAML) by `WriteCriticReview` / `WriteCriticReviewResult` after the critic agent calls `polywave-tools set-critic-review`.

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

When `--skip` is used, the `run-critic` CLI writes a synthetic `CriticData` with `Verdict: "PASS"` and `Summary: "Skipped by operator"`. When `--skip-critic` is used on `prepare-wave`, `SkipCriticForIMPL` writes `Summary: "Skipped by operator (--skip-critic)"`. The `SKIPPED` constant is defined but not written to the manifest in either path; both use `PASS` to allow the gate to proceed.

---

## Severity Values

| Constant | Value | Gate behavior |
|---|---|---|
| `CriticSeverityError` | `"error"` | Blocks the wave unconditionally |
| `CriticSeverityWarning` | `"warning"` | Advisory; behavior depends on mode (see Gate Enforcement) |

---

## Check Categories

The critic agent verifies the following categories. These are set in `CriticIssue.Check`. The authoritative list is in `critic-agent.md` (see `implementations/claude-code/prompts/agents/critic-agent.md` in the Polywave protocol repo).

| Check | What is verified |
|---|---|
| `file_existence` | Files with `action:modify` must exist; files with `action:new` must not yet exist; files with `action:delete` must exist |
| `symbol_accuracy` | Named symbols (functions, types) referenced in briefs exist and match descriptions. Skipped when all agent files are `action:new`. Package-qualified references and deletion-context mentions are filtered out. |
| `pattern_accuracy` | Implementation patterns called out in briefs match actual source patterns |
| `interface_consistency` | Interface contracts are syntactically valid and type-consistent |
| `import_chains` | All packages referenced in briefs are importable from the target module. Only runs for agents with at least one `action:new` file. |
| `side_effect_completeness` | Registration files (init hooks, route tables, etc.) are included in `file_ownership` |
| `complexity_balance` | Advisory: flags agents owning >8 files or >40% of total files. Warning-only, never blocks. |
| `caller_exhaustiveness` | When a brief describes migrating/replacing all callers of a symbol, verifies every non-test call site is in `file_ownership`. Test files produce warnings, not errors. |
| `i1_disjoint_ownership` | No file appears in `file_ownership` with multiple agent IDs for the same wave number (I1 invariant). |
| `result_code_semantics` | Verifies briefs using `result.Result[T]` compare `.Code` only against top-level codes (`SUCCESS`, `PARTIAL`, `FATAL`), not error codes which live in `.Errors[0].Code`. |

---

## E37 Trigger Conditions (`E37Required`)

`protocol.E37Required(manifest) bool` determines whether a critic review is required. The trigger conditions are:

- **Wave 1 has 3 or more agents**, OR
- **`file_ownership` spans 2 or more distinct repositories** (counted from `manifest.Repositories` plus any `repo` field on individual `file_ownership` entries)

If neither condition is met, the critic gate is skipped entirely.

---

## E37 Gate Enforcement (`CriticGatePasses`)

`protocol.CriticGatePasses(manifest, autoMode bool) bool` is the authoritative gate decision. It is called by the orchestrator before launching a wave, but only when `E37Required` returns true.

| Condition | `autoMode=true` | `autoMode=false` |
|---|---|---|
| No `critic_report` present | block | block |
| `Verdict == "PASS"` | proceed | proceed |
| `Verdict == "ISSUES"`, has `error`-severity issue | block | block |
| `Verdict == "ISSUES"`, warnings only | proceed | block (surface to user) |
| Unknown verdict (not `PASS` or `ISSUES`) | block | block |

**Auto mode** is active when the wave was launched via `--auto` (unattended execution) or when called from `prepare-wave` / `prepare-tier`. **Manual mode** applies to interactive `saw wave` invocations (via `engine.Prepare`) where the user can review and decide.

A warnings-only report in auto mode is treated as safe to proceed. In manual mode the gate blocks and the orchestrator surfaces the warnings to the user for a decision.

### `SkipCriticForIMPL`

`protocol.SkipCriticForIMPL(ctx, implPath, manifest) result.Result[bool]` writes a synthetic PASS critic report for a single IMPL if E37 is required and no passing report exists. Returns `true` if a skip was written, `false` if no skip was needed. Used by `prepare-wave --skip-critic` and `prepare-tier` to bypass the gate without launching a critic agent.

---

## How `critic_report` Is Populated

1. The orchestrator calls `polywave-tools run-critic <impl-path>` (or `engine.RunCritic` directly).
2. `engine.BuildCriticPrompt` loads the IMPL doc, resolves repo roots, loads `critic-agent.md` from the Polywave protocol repo with reference injection, and assembles the prompt.
3. `engine.RunCritic` applies a context timeout (default 20 minutes), initializes a backend from the model option, and launches a critic agent via `agent.NewRunner`.
4. The critic agent performs its checks and calls `polywave-tools set-critic-review <impl-path> --verdict <V> --summary <S> --issue-count <N> --agent-reviews <JSON>`.
5. `set-critic-review` parses the JSON array of `AgentCriticReview` objects, builds a `CriticData`, and calls `protocol.WriteCriticReview` to persist it to `IMPLManifest.CriticReport`.
6. `engine.RunCritic` reloads the manifest, reads `CriticReport`, and returns a `RunCriticResult` to the caller.

If the critic agent completes but no `critic_report` was written, `RunCritic` returns an error. This is treated as a gate failure.

---

## `polywave-tools run-critic` CLI

```
polywave-tools run-critic <impl-path> [flags]
```

`<impl-path>` must be an absolute path to an existing IMPL YAML file.

| Flag | Default | Description |
|---|---|---|
| `--model <model>` | `""` (inherits default) | Model override for the critic agent (e.g. `claude-opus-4-6`) |
| `--timeout <minutes>` | `20` | Maximum runtime for the critic agent in minutes |
| `--no-review` | `false` | Skip the review; write a synthetic PASS result immediately |
| `--skip` | `false` | Alias for `--no-review` |
| `--backend <mode>` | `"cli"` | Backend mode: `cli` (default, launches agent subprocess) or `agent-tool` (prints assembled prompt to stdout and exits without launching an agent) |

**Exit codes:**
- `0` — verdict is `PASS` (or skipped)
- `1` — verdict is `ISSUES`; error message directs operator to correct the IMPL doc before proceeding

**Polywave repo resolution** (for loading `critic-agent.md`):
1. `$POLYWAVE_REPO` environment variable
2. `~/code/polywave` (default fallback)

The `critic-agent.md` prompt is loaded from `<saw-repo>/implementations/claude-code/prompts/agents/critic-agent.md` with reference injection via `engine.LoadTypePromptWithRefs`. If the file cannot be loaded, `RunCritic` returns a fatal error.

---

## `polywave-tools set-critic-review` CLI

Used by critic agents to write their output. Not intended for direct human use.

```
polywave-tools set-critic-review <impl-path> \
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

## `polywave-tools set-critic-verdict` CLI

Used by the orchestrator to atomically update the `critic_report.verdict` field in an existing IMPL doc. Typical use case: after correcting issues flagged by the critic, transition the verdict from `ISSUES` to `PASS` without manually editing YAML or re-running the full critic agent.

```
polywave-tools set-critic-verdict <impl-path> --verdict <pass|issues>
```

`--verdict` is required (case-insensitive; normalized to uppercase). Exits 1 if no `critic_report` exists in the IMPL doc.

On success, prints a JSON object with `impl_path`, `old_verdict`, and `new_verdict`.

---

## Validation Rules for Critic Agents

When implementing a critic agent, these rules govern which checks to apply:

| Scenario | Rule |
|---|---|
| File in `file_ownership` with `action:new` | Skip "file does not exist" errors; file will be created |
| File in `file_ownership` with `action:modify` | Enforce existence check |
| File in `file_ownership` with `action:delete` | Enforce existence check (file must exist to be deleted); missing file is warning, not error |
| File not in `file_ownership` (external) | Enforce existence check |
| Symbol defined in a file with `action:new` | Skip "symbol does not exist" errors; symbol will be created |
| Symbol in existing file (`action:modify` or external) | Enforce existence check |
| Cross-file reference to same-wave agent's file | Defer (coordination concern, not verification) |
| Cross-file reference to external file | Enforce existence |

Helper functions `IsNewFile`, `IsSymbolInNewFile`, and `GetAgentNewFiles` in `pkg/protocol/critic.go` implement these rules.
