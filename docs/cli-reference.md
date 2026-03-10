# sawtools CLI Reference

`sawtools` is the SAW Protocol SDK command-line toolkit. All commands accept a global `--repo-dir` flag (default `.`) specifying the repository root.

```
sawtools [command] [args] [flags]
sawtools --repo-dir /path/to/repo [command] ...
```

## Quick Reference

| Command | Category | Description |
|---------|----------|-------------|
| `validate` | Validation | Validate YAML IMPL manifest against protocol invariants |
| `solve` | Validation | Compute optimal wave assignments from dependency graph |
| `extract-context` | Context | Extract per-agent context payload from manifest |
| `list-impls` | Context | List all IMPL manifests in a directory |
| `create-worktrees` | Execution | Create git worktrees for agents in a wave |
| `run-wave` | Execution | Execute full wave lifecycle end-to-end |
| `verify-commits` | Execution | Verify agent branches have commits (I5) |
| `merge-agents` | Execution | Merge all agent branches for a wave |
| `verify-build` | Execution | Run test and lint commands from manifest |
| `cleanup` | Execution | Remove worktrees and branches after merge |
| `verify-isolation` | Execution | Verify agent is in correct isolated worktree (E12) |
| `update-status` | Status | Update agent status in manifest |
| `update-context` | Status | Update project CONTEXT.md (E18) |
| `set-completion` | Status | Set completion report for an agent |
| `mark-complete` | Status | Write completion marker to manifest |
| `scan-stubs` | Quality | Scan files for stub/TODO patterns (E20) |
| `run-gates` | Quality | Run quality gates from manifest |
| `check-conflicts` | Quality | Detect file ownership conflicts |
| `validate-scaffolds` | Quality | Validate scaffold files are committed |
| `freeze-check` | Quality | Check manifest for freeze violations |
| `update-agent-prompt` | Quality | Update an agent's prompt/task in manifest |

---

## Validation

### validate

Validate a YAML IMPL manifest against protocol invariants and E16 typed-block rules.

```
sawtools validate <manifest-path> [--solver]
```

**Arguments:**
- `manifest-path` -- path to YAML IMPL manifest (required)

**Flags:**
- `--solver` -- use CSP solver for wave assignment validation (default: false)

**Output:** JSON object with `valid` (bool), `error_count` (int), `errors` (array of `{code, message, field?, line?}`).

**Exit codes:** 0 if valid, 1 if any errors found.

**Example:**
```bash
sawtools validate docs/IMPL/my-feature.yaml
sawtools validate docs/IMPL/my-feature.yaml --solver
```

---

### solve

Compute optimal wave assignments from dependency declarations using topological sort. Rewrites the manifest in-place with corrected wave numbers.

```
sawtools solve <manifest-path> [--dry-run]
```

**Arguments:**
- `manifest-path` -- path to YAML IMPL manifest (required)

**Flags:**
- `--dry-run` -- print changes without writing (default: false)

**Output:** Human-readable text. Prints each reassignment, then a summary line. No JSON output.

**Exit codes:** 0 on success or no changes needed, 1 if the dependency graph cannot be solved (e.g., cycles).

**Example:**
```bash
sawtools solve docs/IMPL/my-feature.yaml --dry-run
sawtools solve docs/IMPL/my-feature.yaml
```

---

## Context & Discovery

### extract-context

Extract the per-agent context payload from a YAML IMPL manifest. Used to build agent prompts (E23).

```
sawtools extract-context <manifest-path> --agent <agent-id>
```

**Arguments:**
- `manifest-path` -- path to YAML IMPL manifest (required)

**Flags:**
- `--agent` -- agent ID to extract context for (required)

**Output:** JSON object containing the agent's task, files, dependencies, and `impl_doc_path`.

**Exit codes:** 0 on success, 1 on error.

**Example:**
```bash
sawtools extract-context docs/IMPL/my-feature.yaml --agent A
```

---

### list-impls

List all IMPL manifests found in a directory. Prints JSON summaries with path, feature slug, verdict, current wave, and total waves.

```
sawtools list-impls [--dir <path>]
```

**Flags:**
- `--dir` -- directory to scan (default: `docs/IMPL`)

**Output:** JSON array of manifest summaries. Empty array is valid.

**Exit codes:** 0 always (empty list is not an error).

**Example:**
```bash
sawtools list-impls
sawtools list-impls --dir /path/to/impls
```

---

## Wave Execution

### create-worktrees

Create git worktrees for all agents in a given wave. Each agent gets an isolated branch and working directory.

```
sawtools create-worktrees <manifest-path> --wave <n>
```

**Arguments:**
- `manifest-path` -- path to YAML IMPL manifest (required)

**Flags:**
- `--wave` -- wave number (required)

**Output:** JSON object with worktree paths and branch names per agent.

**Exit codes:** 0 on success, 1 on error.

**Example:**
```bash
sawtools create-worktrees docs/IMPL/my-feature.yaml --wave 1
```

---

### run-wave

Execute the full wave lifecycle: create worktrees, verify commits, merge agents, verify build, and cleanup.

```
sawtools run-wave <manifest-path> --wave <n>
```

**Arguments:**
- `manifest-path` -- path to YAML IMPL manifest (required)

**Flags:**
- `--wave` -- wave number (required)

**Output:** JSON object with per-step results. Partial results are printed even on failure.

**Exit codes:** 0 if all steps succeed, 1 if any step fails.

**Example:**
```bash
sawtools run-wave docs/IMPL/my-feature.yaml --wave 2
```

---

### verify-commits

Verify that each agent branch in a wave has at least one commit. Implements the I5 trip wire check.

```
sawtools verify-commits <manifest-path> --wave <n>
```

**Arguments:**
- `manifest-path` -- path to YAML IMPL manifest (required)

**Flags:**
- `--wave` -- wave number (required)

**Output:** JSON object with `all_valid` (bool) and per-agent results.

**Exit codes:** 0 if all agents have commits, 1 if any agent branch is empty.

**Example:**
```bash
sawtools verify-commits docs/IMPL/my-feature.yaml --wave 1
```

---

### merge-agents

Merge all agent branches for a wave into the integration branch.

```
sawtools merge-agents <manifest-path> --wave <n>
```

**Arguments:**
- `manifest-path` -- path to YAML IMPL manifest (required)

**Flags:**
- `--wave` -- wave number (required)

**Output:** JSON object with `success` (bool) and per-agent merge results.

**Exit codes:** 0 if all merges succeed, 1 if any merge fails.

**Example:**
```bash
sawtools merge-agents docs/IMPL/my-feature.yaml --wave 1
```

---

### verify-build

Run the test and lint commands declared in the manifest.

```
sawtools verify-build <manifest-path>
```

**Arguments:**
- `manifest-path` -- path to YAML IMPL manifest (required)

**Output:** JSON object with `test_passed` (bool), `lint_passed` (bool), and command output.

**Exit codes:** 0 if both tests and lint pass, 1 if either fails.

**Example:**
```bash
sawtools verify-build docs/IMPL/my-feature.yaml
```

---

### cleanup

Remove worktrees and branches for a wave after a successful merge.

```
sawtools cleanup <manifest-path> --wave <n>
```

**Arguments:**
- `manifest-path` -- path to YAML IMPL manifest (required)

**Flags:**
- `--wave` -- wave number (required)

**Output:** JSON object with cleanup results per agent.

**Exit codes:** 0 on success, 1 on error.

**Example:**
```bash
sawtools cleanup docs/IMPL/my-feature.yaml --wave 1
```

---

### verify-isolation

Verify that the current working directory is an isolated agent worktree on the expected branch. Enforces Field 0 / E12.

```
sawtools verify-isolation --branch <branch-name>
```

**Flags:**
- `--branch` -- expected branch name, e.g. `wave1-agent-A` (required)

**Output:** JSON object with `ok` (bool), actual branch, and expected branch.

**Exit codes:** 0 if isolation is correct, 1 if the branch does not match.

**Example:**
```bash
sawtools verify-isolation --branch wave1-agent-A
```

---

## Status & Reporting

### update-status

Update an agent's status field in the manifest.

```
sawtools update-status <manifest-path> --wave <n> --agent <id> --status <status>
```

**Arguments:**
- `manifest-path` -- path to YAML IMPL manifest (required)

**Flags:**
- `--wave` -- wave number (required)
- `--agent` -- agent ID (required)
- `--status` -- one of: `complete`, `partial`, `blocked` (required)

**Output:** JSON object confirming the update.

**Exit codes:** 0 on success, 1 on error.

**Example:**
```bash
sawtools update-status docs/IMPL/my-feature.yaml --wave 1 --agent A --status complete
```

---

### update-context

Update the project's `CONTEXT.md` file from the manifest. Implements E18.

```
sawtools update-context <manifest-path> [--project-root <path>]
```

**Arguments:**
- `manifest-path` -- path to YAML IMPL manifest (required)

**Flags:**
- `--project-root` -- project root directory (default: `.`)

**Output:** JSON object confirming the update.

**Exit codes:** 0 on success, 1 on error.

**Example:**
```bash
sawtools update-context docs/IMPL/my-feature.yaml --project-root /path/to/project
```

---

### set-completion

Set a completion report for an agent in the manifest. Records commit, files changed/created, tests added, and verification results.

```
sawtools set-completion <manifest-path> --agent <id> --status <status> --commit <sha> [flags]
```

**Arguments:**
- `manifest-path` -- path to YAML IMPL manifest (required)

**Flags:**
- `--agent` -- agent ID (required)
- `--status` -- one of: `complete`, `partial`, `blocked` (required)
- `--commit` -- commit SHA (required)
- `--worktree` -- worktree path
- `--branch` -- branch name
- `--files-changed` -- comma-separated list of changed files
- `--files-created` -- comma-separated list of created files
- `--tests-added` -- comma-separated list of tests added
- `--verification` -- verification result text

**Output:** JSON object with `agent`, `status`, and `saved` fields.

**Exit codes:** 0 on success, 1 on error.

**Example:**
```bash
sawtools set-completion docs/IMPL/my-feature.yaml \
  --agent A --status complete --commit abc1234 \
  --files-changed "pkg/foo.go,pkg/bar.go" \
  --tests-added "pkg/foo_test.go"
```

---

### mark-complete

Write the final completion marker to an IMPL manifest, recording the completion date.

```
sawtools mark-complete <manifest-path> [--date <YYYY-MM-DD>]
```

**Arguments:**
- `manifest-path` -- path to YAML IMPL manifest (required)

**Flags:**
- `--date` -- completion date in `YYYY-MM-DD` format (default: today)

**Output:** JSON object with `marked` (bool), `date`, and `path`.

**Exit codes:** 0 on success, 1 on error.

**Example:**
```bash
sawtools mark-complete docs/IMPL/my-feature.yaml
sawtools mark-complete docs/IMPL/my-feature.yaml --date 2026-03-10
```

---

## Quality & Safety

### scan-stubs

Scan source files for stub/TODO patterns. Implements E20 enforcement. Optionally appends the stub report to a manifest.

```
sawtools scan-stubs <file> [file...] [--append-impl <path>] [--wave <n>]
```

**Arguments:**
- `file` -- one or more file paths to scan (at least one required)

**Flags:**
- `--append-impl` -- append stub report to the manifest at this path
- `--wave` -- wave number for the stub report key (used with `--append-impl`, default: 0)

**Output:** JSON object with stub/TODO findings per file.

**Exit codes:** 0 always.

**Example:**
```bash
sawtools scan-stubs pkg/foo.go pkg/bar.go
sawtools scan-stubs pkg/*.go --append-impl docs/IMPL/my-feature.yaml --wave 2
```

---

### run-gates

Run the quality gates declared in the manifest. Gates can be marked as required or optional.

```
sawtools run-gates <manifest-path> [--wave <n>]
```

**Arguments:**
- `manifest-path` -- path to YAML IMPL manifest (required)

**Flags:**
- `--wave` -- wave number (default: 0)

**Output:** JSON array of gate results, each with `passed` (bool) and `required` (bool).

**Exit codes:** 0 if all required gates pass, 1 if any required gate fails.

**Example:**
```bash
sawtools run-gates docs/IMPL/my-feature.yaml --wave 1
```

---

### check-conflicts

Detect file ownership conflicts across agent completion reports. Flags files touched by multiple agents.

```
sawtools check-conflicts <manifest-path>
```

**Arguments:**
- `manifest-path` -- path to YAML IMPL manifest (required)

**Output:** JSON object with `conflict_count` (int) and `conflicts` array.

**Exit codes:** 0 if no conflicts, 1 if any conflicts found.

**Example:**
```bash
sawtools check-conflicts docs/IMPL/my-feature.yaml
```

---

### validate-scaffolds

Validate that all scaffold files declared in the manifest are committed to the repository.

```
sawtools validate-scaffolds <manifest-path>
```

**Arguments:**
- `manifest-path` -- path to YAML IMPL manifest (required)

**Output:** JSON object with `all_committed` (bool), `scaffold_count` (int), and `statuses` array.

**Exit codes:** 0 if all scaffolds are committed, 1 if any are missing.

**Example:**
```bash
sawtools validate-scaffolds docs/IMPL/my-feature.yaml
```

---

### freeze-check

Check whether a manifest is frozen (worktrees have been created) and detect any freeze violations.

```
sawtools freeze-check <manifest-path>
```

**Arguments:**
- `manifest-path` -- path to YAML IMPL manifest (required)

**Output:** JSON object with `frozen` (bool), `violation_count` (int), and `violations` array.

**Exit codes:** 0 if no violations, 1 if any freeze violations found.

**Example:**
```bash
sawtools freeze-check docs/IMPL/my-feature.yaml
```

---

### update-agent-prompt

Update an agent's prompt/task text in the manifest. Saves the manifest in-place.

```
sawtools update-agent-prompt <manifest-path> --agent <id> --prompt <text>
```

**Arguments:**
- `manifest-path` -- path to YAML IMPL manifest (required)

**Flags:**
- `--agent` -- agent ID (required)
- `--prompt` -- new prompt/task text (required)

**Output:** JSON object with `agent` and `updated` (bool).

**Exit codes:** 0 on success, 1 on error.

**Example:**
```bash
sawtools update-agent-prompt docs/IMPL/my-feature.yaml \
  --agent B --prompt "Implement the HTTP handler for /api/widgets"
```

---

## Global Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--repo-dir` | `.` | Repository root directory. Inherited by all subcommands that perform git operations. |
