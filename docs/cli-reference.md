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
| `validate-program` | Validation | Validate a YAML PROGRAM manifest against schema rules |
| `validate-integration` | Validation | Detect unconnected exports after a wave completes (E25) |
| `solve` | Validation | Compute optimal wave assignments from dependency graph |
| `extract-context` | Context | Extract per-agent context payload from manifest |
| `list-impls` | Context | List IMPL manifests in a directory |
| `list-programs` | Context | List PROGRAM manifests in a directory |
| `extract-commands` | Context | Extract build/test/lint/format commands from CI configs |
| `analyze-deps` | Context | Analyze Go file dependencies and compute wave structure |
| `analyze-suitability` | Context | Scan codebase for pre-implemented requirements |
| `detect-scaffolds` | Context | Detect shared types that should be scaffold files |
| `detect-cascades` | Context | Detect files affected by type renames |
| `create-worktrees` | Execution | Create git worktrees for agents in a wave |
| `prepare-agent` | Execution | Prepare agent environment (extract brief, init journal) |
| `prepare-wave` | Execution | Prepare all agents in a wave (atomic batch operation) |
| `run-wave` | Execution | Execute full wave lifecycle end-to-end |
| `run-scout` | Execution | Automated Scout execution with validation (I3) |
| `run-critic` | Execution | Run critic agent to review briefs against codebase (E37) |
| `finalize-wave` | Execution | Finalize wave: verify, scan, gate, merge, build, cleanup |
| `finalize-impl` | Execution | Finalize IMPL doc: validate, populate gates, validate again |
| `verify-commits` | Execution | Verify agent branches have commits (I5) |
| `merge-agents` | Execution | Merge all agent branches for a wave |
| `verify-build` | Execution | Run test and lint commands from manifest |
| `cleanup` | Execution | Remove worktrees and branches after merge |
| `verify-isolation` | Execution | Verify agent is in correct isolated worktree (E12) |
| `verify-hook-installed` | Execution | Verify pre-commit hook is installed in worktree (E4) |
| `update-status` | Status | Update agent status in manifest |
| `update-context` | Status | Update project CONTEXT.md (E18) |
| `set-completion` | Status | Set completion report for an agent |
| `set-critic-review` | Status | Write critic review result to IMPL doc (E37) |
| `mark-complete` | Status | Write completion marker and archive IMPL manifest |
| `program-status` | Status | Display full program status report |
| `mark-program-complete` | Status | Mark a PROGRAM manifest as complete |
| `scan-stubs` | Quality | Scan files for stub/TODO patterns (E20) |
| `run-gates` | Quality | Run quality gates from manifest |
| `run-review` | Quality | Run AI code review on the current diff |
| `check-conflicts` | Quality | Detect file ownership conflicts |
| `check-deps` | Quality | Detect dependency conflicts before wave execution |
| `validate-scaffolds` | Quality | Validate scaffold files are committed |
| `validate-scaffold` | Quality | Validate a single scaffold file before committing |
| `freeze-check` | Quality | Check manifest for freeze violations |
| `update-agent-prompt` | Quality | Update an agent's prompt/task in manifest |
| `populate-integration-checklist` | Quality | Auto-generate post-merge checklist from file patterns (M5) |
| `assign-agent-ids` | Determinism | Generate agent IDs following the `^[A-Z][2-9]?$` pattern |
| `diagnose-build-failure` | Determinism | Pattern-match build errors and suggest fixes (H7) |
| `amend-impl` | Amendment | Amend a living IMPL doc (add wave, redirect agent, extend scope) |
| `retry` | Recovery | Generate retry IMPL doc for failed quality gate (E24) |
| `build-retry-context` | Recovery | Build structured retry context for a failed agent |
| `resume-detect` | Recovery | Detect interrupted SAW sessions in the repository |
| `journal-init` | Journal | Initialize journal directory for a wave agent |
| `journal-context` | Journal | Generate context.md from journal entries for agent recovery |
| `debug-journal` | Journal | Inspect journal contents for debugging failed agents |
| `daemon` | Automation | Run the SAW daemon loop (process queue items continuously) |
| `queue` | Automation | Manage the IMPL execution queue (add, list, next) |
| `tier-gate` | Program | Verify tier gate for a PROGRAM manifest |
| `freeze-contracts` | Program | Freeze program contracts at a tier boundary |
| `program-replan` | Program | Re-engage Planner agent to revise a PROGRAM manifest |

---

## Validation

### validate

Validate a YAML IMPL manifest against protocol invariants and E16 typed-block rules.

```
sawtools validate <manifest-path> [flags]
```

**Arguments:**
- `manifest-path` -- path to YAML IMPL manifest (required)

**Flags:**
- `--solver` -- use CSP solver for wave assignment validation (default: false)
- `--fix` -- auto-correct fixable issues (e.g. invalid gate types to custom, unknown keys stripped) (default: false)

**Output:** JSON object with `valid` (bool), `error_count` (int), `errors` (array of `{code, message, field?, line?}`).

**Exit codes:** 0 if valid, 1 if any errors found.

**Example:**
```bash
sawtools validate docs/IMPL/my-feature.yaml
sawtools validate docs/IMPL/my-feature.yaml --solver
sawtools validate docs/IMPL/my-feature.yaml --fix
```

---

### validate-program

Validate a YAML PROGRAM manifest against schema rules.

```
sawtools validate-program <program-manifest>
```

**Arguments:**
- `program-manifest` -- path to YAML PROGRAM manifest (required)

**Output:** JSON validation result.

**Exit codes:** 0 if valid, 1 if any errors found.

**Example:**
```bash
sawtools validate-program docs/PROGRAM.yaml
```

---

### validate-integration

Scan a completed wave for unconnected exports using Go AST analysis. Detects heuristic integration gaps and optionally checks wiring declarations (E35 Layer 3B). Persists reports back to the manifest.

```
sawtools validate-integration <manifest-path> --wave <n> [flags]
```

**Arguments:**
- `manifest-path` -- path to YAML IMPL manifest (required)

**Flags:**
- `--wave` -- wave number (required)
- `--wiring` -- enable wiring declaration checking (default: true)

**Output:** Combined JSON report of integration gaps.

**Exit codes:** 0 if no gaps found, 1 if gaps are detected.

**Example:**
```bash
sawtools validate-integration docs/IMPL/my-feature.yaml --wave 1
sawtools validate-integration docs/IMPL/my-feature.yaml --wave 2 --wiring=false
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

List IMPL manifests found in a directory. Excludes completed manifests by default. Prints JSON summaries with path, feature slug, verdict, current wave, and total waves.

```
sawtools list-impls [flags]
```

**Flags:**
- `--dir` -- directory to scan (default: `docs/IMPL`)
- `--include-complete` -- include completed/archived IMPL docs (default: false)

**Output:** JSON array of manifest summaries. Empty array is valid.

**Exit codes:** 0 always (empty list is not an error).

**Example:**
```bash
sawtools list-impls
sawtools list-impls --dir /path/to/impls
sawtools list-impls --include-complete
```

---

### list-programs

List PROGRAM manifests found in a directory.

```
sawtools list-programs [flags]
```

**Flags:**
- `--dir` -- directory to scan (default: `docs/`)

**Output:** JSON array of program manifest summaries.

**Exit codes:** 0 always.

**Example:**
```bash
sawtools list-programs
sawtools list-programs --dir /path/to/programs
```

---

### extract-commands

Extract build, test, lint, and format commands from CI configuration files (GitHub Actions, GitLab CI, CircleCI) and build system files (Makefile, package.json). Uses priority-based resolution and falls back to language defaults when no config files are present.

```
sawtools extract-commands <repo-root> [flags]
```

**Arguments:**
- `repo-root` -- path to the repository root (required)

**Flags:**
- `--format` -- output format: `yaml` or `json` (default: `yaml`)

**Output:** Command specification matching the Scout IMPL doc schema.

**Exit codes:** 0 on success, 1 on error.

**Example:**
```bash
sawtools extract-commands .
sawtools extract-commands /path/to/repo --format json
```

---

### analyze-deps

Analyze Go source files to extract import dependencies, detect cycles, compute topological sort, and assign wave structure for parallel agent execution.

```
sawtools analyze-deps <repo-root> --files <file-list> [flags]
```

**Arguments:**
- `repo-root` -- path to the repository root (required)

**Flags:**
- `--files` -- comma-separated list of Go files to analyze (required)
- `--format` -- output format: `yaml` or `json` (default: `yaml`)

**Output:** Dependency graph matching Scout IMPL doc schema.

**Exit codes:** 0 on success, 1 on error.

**Example:**
```bash
sawtools analyze-deps . --files "pkg/foo.go,pkg/bar.go"
sawtools analyze-deps /path/to/repo --files "cmd/main.go" --format json
```

---

### analyze-suitability

Scan a codebase to determine which requirements are already implemented (DONE), partially implemented (PARTIAL), or not yet implemented (TODO). Reads a structured markdown requirements document with `Location:` fields.

```
sawtools analyze-suitability [flags]
```

**Flags:**
- `--requirements` -- path to requirements/audit doc in markdown format (required)
- `--repo-root` -- repository root directory (default: `.`)
- `--output` -- output format (default: `json`)

**Output:** JSON with status, test coverage, and time savings estimates per requirement.

**Exit codes:** 0 on success, 1 on error.

**Example:**
```bash
sawtools analyze-suitability --requirements docs/audit.md --repo-root /path/to/repo
```

---

### detect-scaffolds

Analyze IMPL document to detect types that should be extracted to scaffold files. Pre-agent mode finds types referenced by two or more agents. Post-agent mode detects duplicate type definitions.

```
sawtools detect-scaffolds <impl-doc-path> --stage <stage>
```

**Arguments:**
- `impl-doc-path` -- path to IMPL document (required)

**Flags:**
- `--stage` -- detection stage: `pre-agent` or `post-agent` (required)

**Output:** JSON with detected scaffold candidates.

**Exit codes:** 0 on success, 1 on error.

**Example:**
```bash
sawtools detect-scaffolds docs/IMPL/my-feature.yaml --stage pre-agent
sawtools detect-scaffolds docs/IMPL/my-feature.yaml --stage post-agent
```

---

### detect-cascades

Detect files affected by type renames in a repository. Outputs cascade candidates with severity levels and reasons.

```
sawtools detect-cascades <repo-root> --renames <json>
```

**Arguments:**
- `repo-root` -- path to the repository root (required)

**Flags:**
- `--renames` -- JSON array of rename objects, e.g. `[{"old":"AuthToken","new":"SessionToken","scope":"pkg/auth"}]` (required)

**Output:** YAML matching the CascadeResult schema.

**Exit codes:** 0 on success, 1 on error.

**Example:**
```bash
sawtools detect-cascades . --renames '[{"old":"AuthToken","new":"SessionToken","scope":"pkg/auth"}]'
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

### prepare-agent

Prepare an agent's execution environment by extracting the agent's brief and initializing the journal observer. For worktree-based agents, writes brief to worktree root. For solo agents, writes to `.saw-state/`.

```
sawtools prepare-agent <manifest-path> --wave <n> --agent <id> [flags]
```

**Arguments:**
- `manifest-path` -- path to YAML IMPL manifest (required)

**Flags:**
- `--wave` -- wave number (required)
- `--agent` -- agent ID (required)
- `--no-worktree` -- solo agent mode (write brief to .saw-state instead of worktree) (default: false)

**Output:** JSON with prepared agent details.

**Exit codes:** 0 on success, 1 on error.

**Example:**
```bash
sawtools prepare-agent docs/IMPL/my-feature.yaml --wave 1 --agent A
sawtools prepare-agent docs/IMPL/my-feature.yaml --wave 1 --agent A --no-worktree
```

---

### prepare-wave

Atomic batch command that prepares all agents in a wave for parallel execution. Combines check-deps, create-worktrees, and prepare-agent into a single operation.

```
sawtools prepare-wave <manifest-path> --wave <n>
```

**Arguments:**
- `manifest-path` -- path to YAML IMPL manifest (required)

**Flags:**
- `--wave` -- wave number (required)

**Output:** JSON with worktree paths and agent metadata. If dependency conflicts are detected, exits with code 1 and prints a JSON report.

**Exit codes:** 0 on success, 1 on dependency conflict or error.

**Note:** For solo agents (1 agent in wave), use `prepare-agent --no-worktree` instead. `prepare-wave` always creates worktrees, which is unnecessary overhead for single-agent waves.

**Example:**
```bash
sawtools prepare-wave docs/IMPL/my-feature.yaml --wave 1
```

---

### run-wave

Execute the full wave lifecycle: create worktrees, verify commits, merge agents, verify build, and cleanup.

```
sawtools run-wave <manifest-path> --wave <n> [flags]
```

**Arguments:**
- `manifest-path` -- path to YAML IMPL manifest (required)

**Flags:**
- `--wave` -- wave number (required)
- `--no-prioritize` -- disable agent launch prioritization, use declaration order (default: false)

**Output:** JSON object with per-step results. Partial results are printed even on failure.

**Exit codes:** 0 if all steps succeed, 1 if any step fails.

**Example:**
```bash
sawtools run-wave docs/IMPL/my-feature.yaml --wave 2
sawtools run-wave docs/IMPL/my-feature.yaml --wave 1 --no-prioritize
```

---

### run-scout

Fully automated Scout workflow (I3 integration). Launches Scout agent to analyze codebase, creates IMPL doc, validates it (E16), auto-corrects agent IDs (M1), and optionally runs a critic review (E37).

```
sawtools run-scout <feature-description> [flags]
```

**Arguments:**
- `feature-description` -- description of the feature to implement (required)

**Flags:**
- `--repo-dir` -- target repository path (default: current directory)
- `--saw-repo` -- Scout-and-Wave protocol repo path (default: `$SAW_REPO` or `~/code/scout-and-wave`)
- `--scout-model` -- Scout model override (e.g., `claude-opus-4-6`)
- `--critic-model` -- critic agent model override (e.g., `claude-opus-4-6`)
- `--no-critic` -- skip critic gate even if agent count threshold is met (default: false)
- `--program` -- path to PROGRAM manifest (Scout receives frozen contracts as input)
- `--timeout` -- timeout in minutes (default: 10)

**Output:** Validated IMPL doc at `docs/IMPL/IMPL-<slug>.yaml`, ready for wave execution.

**Exit codes:** 0 on success, 1 on error.

**Example:**
```bash
sawtools run-scout "Add audit logging to auth module"
sawtools run-scout "Add audit logging" --repo-dir /path/to/project
sawtools run-scout "Add audit logging" --scout-model claude-opus-4-6
```

---

### run-critic

Launch a critic agent (E37) that reviews every agent brief in the IMPL doc against the actual codebase before wave execution begins. Verifies file existence, symbol accuracy, pattern accuracy, interface consistency, import chains, and side-effect completeness.

```
sawtools run-critic <impl-path> [flags]
```

**Arguments:**
- `impl-path` -- path to IMPL document (required)

**Flags:**
- `--model` -- model override for critic agent (e.g., `claude-opus-4-6`)
- `--no-review` -- skip critic review, write PASS result with "Skipped by operator" summary (default: false)
- `--skip` -- alias for `--no-review`
- `--timeout` -- timeout in minutes (default: 20)

**Output:** Writes CriticResult to the IMPL doc `critic_report` field.

**Exit codes:** 0 if verdict is PASS, 1 if verdict is ISSUES.

**Example:**
```bash
sawtools run-critic docs/IMPL/IMPL-feature.yaml
sawtools run-critic docs/IMPL/IMPL-feature.yaml --model claude-opus-4-6
sawtools run-critic docs/IMPL/IMPL-feature.yaml --skip
```

---

### finalize-wave

Atomic batch command combining the post-agent verification and merge workflow. Stops on first failure and returns partial results. Automatically diagnoses build failures using H7 pattern matching.

Execution order:
1. VerifyCommits (I5 trip wire)
2. ScanStubs (E20)
3. RunPreMergeGates (structural gates, E21)
4. ValidateIntegration (E25, informational)
5. MergeAgents
6. VerifyBuild (test + lint)
7. RunPostMergeGates (content/integration gates, E21)
8. Cleanup

```
sawtools finalize-wave <manifest-path> --wave <n>
```

**Arguments:**
- `manifest-path` -- path to YAML IMPL manifest (required)

**Flags:**
- `--wave` -- wave number (required)

**Output:** JSON object with per-step results.

**Exit codes:** 0 if all steps succeed, 1 if any step fails.

**Example:**
```bash
sawtools finalize-wave docs/IMPL/my-feature.yaml --wave 1
```

---

### finalize-impl

Atomic batch command that validates an IMPL doc, extracts build/test/lint commands (H2), populates verification gate blocks for all agents, and validates again. Transactional (rolls back on failure) and idempotent. Supports multi-repo IMPLs.

```
sawtools finalize-impl <manifest-path> [flags]
```

**Arguments:**
- `manifest-path` -- path to YAML IMPL manifest (required)

**Flags:**
- `--repo-root` -- repository root directory (default: `.`)

**Output:** JSON with validation results and gate population stats.

**Exit codes:** 0 on success, 1 on error.

**Example:**
```bash
sawtools finalize-impl docs/IMPL/IMPL-feature-x.yaml
sawtools finalize-impl docs/IMPL/IMPL-feature-x.yaml --repo-root /path/to/repo
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

### verify-hook-installed

Verify that the pre-commit hook exists in a worktree's `.git/hooks` directory, contains isolation check logic, and is executable. Layer 0 of worktree isolation enforcement (E4).

```
sawtools verify-hook-installed <worktree-path> [flags]
```

**Arguments:**
- `worktree-path` -- path to the worktree to check (required)

**Flags:**
- `--wave` -- wave number (for context in error messages)

**Output:** JSON with hook validation result.

**Exit codes:** 0 if hook is valid, 1 if hook is missing or broken.

**Example:**
```bash
sawtools verify-hook-installed /tmp/saw-worktrees/wave1-agent-A
sawtools verify-hook-installed /tmp/saw-worktrees/wave1-agent-A --wave 1
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

### set-critic-review

Write critic review result to IMPL doc's `critic_report` field. Called by critic agents after completing their review. Not intended for direct human use.

```
sawtools set-critic-review <impl-path> --verdict <verdict> --summary <text> [flags]
```

**Arguments:**
- `impl-path` -- path to IMPL document (required)

**Flags:**
- `--verdict` -- overall verdict: `PASS` or `ISSUES` (required)
- `--summary` -- human-readable summary of findings (required)
- `--issue-count` -- total number of issues found across all agents
- `--agent-reviews` -- JSON array of AgentCriticReview objects

**Output:** JSON confirmation.

**Exit codes:** 0 on success, 1 on error.

**Example:**
```bash
sawtools set-critic-review docs/IMPL/IMPL-feature.yaml \
  --verdict PASS --summary "All briefs verified" \
  --agent-reviews '[{"agent_id":"A","verdict":"PASS","issues":[]}]'
```

---

### mark-complete

Write the final completion marker to an IMPL manifest, recording the completion date, and archive it to the `complete/` subdirectory.

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

### program-status

Display comprehensive status report for a PROGRAM manifest, including current tier, per-tier IMPL statuses, contract freeze states, and completion tracking.

```
sawtools program-status <program-manifest>
```

**Arguments:**
- `program-manifest` -- path to YAML PROGRAM manifest (required)

**Output:** Human-readable status report.

**Exit codes:** 0 always (status is informational), 2 on parse error.

**Example:**
```bash
sawtools program-status docs/PROGRAM.yaml
```

---

### mark-program-complete

Mark a PROGRAM manifest as complete. Verifies all tiers are complete, updates state to PROGRAM_COMPLETE, sets completion date, writes the `SAW:PROGRAM:COMPLETE` marker, updates CONTEXT.md, and commits both files.

```
sawtools mark-program-complete <program-manifest> [flags]
```

**Arguments:**
- `program-manifest` -- path to YAML PROGRAM manifest (required)

**Flags:**
- `--date` -- completion date in `YYYY-MM-DD` format (default: today)
- `--repo-dir` -- repository directory (default: current directory)

**Output:** JSON confirmation.

**Exit codes:** 0 on success, 1 if not all tiers complete, 2 on parse error.

**Example:**
```bash
sawtools mark-program-complete docs/PROGRAM.yaml
sawtools mark-program-complete docs/PROGRAM.yaml --date 2026-03-15
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

Run the quality gates declared in the manifest. Gates can be marked as required or optional. Supports gate result caching (E38).

```
sawtools run-gates <manifest-path> [flags]
```

**Arguments:**
- `manifest-path` -- path to YAML IMPL manifest (required)

**Flags:**
- `--wave` -- wave number (default: 0)
- `--no-cache` -- disable gate result caching (default: false)

**Output:** JSON array of gate results, each with `passed` (bool) and `required` (bool).

**Exit codes:** 0 if all required gates pass, 1 if any required gate fails.

**Example:**
```bash
sawtools run-gates docs/IMPL/my-feature.yaml --wave 1
sawtools run-gates docs/IMPL/my-feature.yaml --wave 1 --no-cache
```

---

### run-review

Run AI code review on the current diff. Used as a post-merge quality gate.

```
sawtools run-review [flags]
```

**Flags:**
- `--model` -- Anthropic model to use (default: `claude-haiku-4-5`)
- `--threshold` -- minimum overall score (0-100) to pass (default: 70)
- `--blocking` -- exit code 1 on failing review (default: false)

**Output:** JSON review result with scores and feedback.

**Exit codes:** 0 on pass (or non-blocking mode), 1 if blocking and review fails.

**Example:**
```bash
sawtools run-review
sawtools run-review --blocking --threshold 80
sawtools run-review --model claude-sonnet-4-20250514
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

### check-deps

Detect dependency conflicts before wave execution. Analyzes IMPL doc file ownership and lock files for missing dependencies and version conflicts.

```
sawtools check-deps <impl-doc> [flags]
```

**Arguments:**
- `impl-doc` -- path to IMPL document (required)

**Flags:**
- `--wave` -- wave number to check (0 = all waves, default: 0)

**Output:** JSON report of dependency conflicts.

**Exit codes:** 0 if no conflicts, 1 if conflicts found.

**Example:**
```bash
sawtools check-deps docs/IMPL/my-feature.yaml
sawtools check-deps docs/IMPL/my-feature.yaml --wave 1
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

### validate-scaffold

Validate a single scaffold file before committing. Runs a pipeline: syntax check (Go AST), import resolution, type reference check, and partial build.

```
sawtools validate-scaffold <scaffold-file> [flags]
```

**Arguments:**
- `scaffold-file` -- path to scaffold file to validate (required)

**Flags:**
- `--impl-doc` -- path to IMPL doc for build command extraction

**Output:** Structured YAML with pass/fail status per validation step.

**Exit codes:** 0 if all checks pass, 1 if any fail.

**Example:**
```bash
sawtools validate-scaffold pkg/types/scaffold.go
sawtools validate-scaffold pkg/types/scaffold.go --impl-doc docs/IMPL/my-feature.yaml
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

### populate-integration-checklist

Determinism tool (M5) that scans `file_ownership` for integration-requiring patterns and populates `post_merge_checklist` groups. Detects new API handlers, React components, CLI commands, and background services. Idempotent.

```
sawtools populate-integration-checklist <manifest-path> [flags]
```

**Arguments:**
- `manifest-path` -- path to YAML IMPL manifest (required)

**Flags:**
- `--repo-root` -- repository root for file parsing (default: `.`)

**Output:** Updated IMPL manifest with populated post_merge_checklist.

**Exit codes:** 0 on success, 1 on error.

**Example:**
```bash
sawtools populate-integration-checklist docs/IMPL/my-feature.yaml
sawtools populate-integration-checklist docs/IMPL/my-feature.yaml --repo-root /path/to/repo
```

---

## Determinism & Analysis

### assign-agent-ids

Generate agent IDs following the `^[A-Z][2-9]?$` pattern. Supports sequential mode (A-Z, then A2-Z2, etc.) and grouped mode with category tags.

```
sawtools assign-agent-ids --count <n> [flags]
```

**Flags:**
- `--count` -- number of agents (required)
- `--grouping` -- JSON array of category tags for grouped assignment (optional)

**Output:** Space-separated agent IDs.

**Exit codes:** 0 on success, 1 on error.

**Example:**
```bash
sawtools assign-agent-ids --count 5
sawtools assign-agent-ids --count 9 --grouping '[["data"],["data"],["data"],["api"],["api"],["ui"],["ui"],["ui"],["ui"]]'
```

---

### diagnose-build-failure

Pattern-match build error logs against known catalogs and suggest fixes. Supports Go, Rust, JavaScript, TypeScript, and Python.

```
sawtools diagnose-build-failure <error-log> --language <lang>
```

**Arguments:**
- `error-log` -- path to error log file (required)

**Flags:**
- `--language` -- language: `go`, `rust`, `js`, `ts`, `python` (required)

**Output:** Structured YAML with diagnosis, confidence, fix, rationale, and auto_fixable flag.

**Exit codes:** 0 on success, 1 on error.

**Example:**
```bash
sawtools diagnose-build-failure /tmp/build-errors.log --language go
```

---

## Amendment & Recovery

### amend-impl

Mutate a living IMPL doc. Supports three modes: add a new wave, redirect an agent, or extend scope. Exactly one mode flag must be provided.

```
sawtools amend-impl <manifest-path> <mode-flag> [flags]
```

**Arguments:**
- `manifest-path` -- path to YAML IMPL manifest (required)

**Flags (exactly one required):**
- `--add-wave` -- append a new empty wave skeleton to the manifest
- `--redirect-agent <id>` -- re-queue an agent: update its task and clear its completion report
- `--extend-scope` -- print instructions for re-engaging Scout with this IMPL as context

**Additional flags:**
- `--wave` -- wave number for `--redirect-agent` (required with `--redirect-agent`)
- `--new-task` -- replacement task text for `--redirect-agent` (reads from stdin if omitted)

**Output:** JSON.

**Exit codes:** 0 on success, 1 on error.

**Example:**
```bash
sawtools amend-impl docs/IMPL/my-feature.yaml --add-wave
sawtools amend-impl docs/IMPL/my-feature.yaml --redirect-agent B --wave 1 --new-task "Fix the handler"
sawtools amend-impl docs/IMPL/my-feature.yaml --extend-scope
```

---

### retry

Generate a single-agent retry IMPL doc targeting files that failed a quality gate. Implements the E24 verification loop. Does NOT execute the retry agent -- only generates the IMPL doc.

```
sawtools retry <impl-doc> --wave <n> --gate-type <type> [flags]
```

**Arguments:**
- `impl-doc` -- path to IMPL document (required)

**Flags:**
- `--wave` -- wave number that failed (required)
- `--gate-type` -- type of gate that failed: `build`, `test`, `lint` (required)
- `--max-retries` -- maximum retry attempts before transitioning to blocked (default: 2)
- `--repo-root` -- repository root directory (default: inferred from `--repo-dir` or impl path)

**Output:** JSON with retry IMPL doc path and state (`retrying` or `blocked`).

**Exit codes:** 0 on success, 1 on error.

**Example:**
```bash
sawtools retry docs/IMPL/my-feature.yaml --wave 1 --gate-type build
sawtools retry docs/IMPL/my-feature.yaml --wave 2 --gate-type test --max-retries 3
```

---

### build-retry-context

Build structured retry context for a failed agent attempt. Reads the completion report, classifies the error type, and outputs actionable retry context.

```
sawtools build-retry-context <manifest-path> --agent <id> [flags]
```

**Arguments:**
- `manifest-path` -- path to YAML IMPL manifest (required)

**Flags:**
- `--agent` -- agent ID to build retry context for (required)
- `--attempt` -- retry attempt number (default: 1)

**Output:** JSON with `attempt_number`, `agent_id`, `error_class`, `error_excerpt`, `gate_results`, `suggested_fixes`, `prior_notes`, and `prompt_text`.

**Exit codes:** 0 on success, 1 if agent has no completion report or manifest cannot be loaded.

**Example:**
```bash
sawtools build-retry-context docs/IMPL/my-feature.yaml --agent B
sawtools build-retry-context docs/IMPL/my-feature.yaml --agent B --attempt 2
```

---

### resume-detect

Detect interrupted SAW sessions in the repository. Scans `docs/IMPL/` for manifests that are not complete or unsuitable, inspects completion reports and git worktrees.

```
sawtools resume-detect
```

**Output:** JSON array of interrupted session details. Empty array if none found.

**Exit codes:** 0 always.

**Example:**
```bash
sawtools resume-detect
```

---

## Journal & Debugging

### journal-init

Initialize the journal directory structure (`.saw-state/journals/wave<N>/agent-<ID>/`) and cursor file for tracking Claude Code session log position.

```
sawtools journal-init <manifest-path> --wave <n> --agent <id>
```

**Arguments:**
- `manifest-path` -- path to YAML IMPL manifest (required)

**Flags:**
- `--wave` -- wave number (required)
- `--agent` -- agent ID (required)

**Output:** JSON confirmation.

**Exit codes:** 0 on success, 1 on error.

**Example:**
```bash
sawtools journal-init docs/IMPL/my-feature.yaml --wave 1 --agent A
```

---

### journal-context

Sync the journal from Claude Code session logs and generate a markdown summary of the agent's execution history. The generated `context.md` can be prepended to the agent's prompt after context compaction.

```
sawtools journal-context <manifest-path> --wave <n> --agent <id> [flags]
```

**Arguments:**
- `manifest-path` -- path to YAML IMPL manifest (required)

**Flags:**
- `--wave` -- wave number (required)
- `--agent` -- agent ID (required)
- `--max-entries` -- maximum entries to include (0 = all, default: 0)
- `--output` -- output path for context.md (default: `<journal-dir>/context.md`)

**Output:** Markdown context file written to disk.

**Exit codes:** 0 on success, 1 on error.

**Example:**
```bash
sawtools journal-context docs/IMPL/my-feature.yaml --wave 1 --agent A
sawtools journal-context docs/IMPL/my-feature.yaml --wave 1 --agent A --max-entries 50
```

---

### debug-journal

Inspect tool execution journal for a specific agent. Supports multiple output modes.

```
sawtools debug-journal <agent-path> [flags]
```

**Arguments:**
- `agent-path` -- agent path format: `wave1/agent-A` or `wave2-agent-B` (required)

**Flags:**
- `--summary` -- show human-readable summary (default: full JSONL dump)
- `--failures-only` -- show only failed tool calls
- `--last <n>` -- show last N entries only
- `--export <path>` -- export HTML timeline to file
- `--force` -- overwrite export file if it exists

**Output:** JSONL (default), human-readable summary, or HTML timeline.

**Exit codes:** 0 on success, 1 on error.

**Example:**
```bash
sawtools debug-journal wave1/agent-A
sawtools debug-journal wave1/agent-A --summary
sawtools debug-journal wave1/agent-A --failures-only
sawtools debug-journal wave1/agent-A --last 20
sawtools debug-journal wave1/agent-A --export timeline.html
```

---

## Automation

### daemon

Run the SAW daemon loop that continuously monitors the IMPL queue, runs Scout/Wave cycles, auto-remediates failures, and advances the queue.

```
sawtools daemon [flags]
```

**Flags:**
- `--autonomy` -- override autonomy level: `gated`, `supervised`, or `autonomous`
- `--model` -- chat model to use
- `--poll-interval` -- how often to check the queue (default: `30s`)

**Output:** JSON lines streamed to stdout.

**Exit codes:** 0 on clean shutdown, 1 on error.

**Example:**
```bash
sawtools daemon
sawtools daemon --autonomy supervised --poll-interval 10s
sawtools daemon --model claude-opus-4-6
```

---

### queue

Manage the IMPL execution queue. Has three subcommands: `add`, `list`, and `next`.

#### queue add

Add an item to the execution queue.

```
sawtools queue add [flags]
```

#### queue list

List all items in the execution queue.

```
sawtools queue list [flags]
```

#### queue next

Get the next item from the execution queue.

```
sawtools queue next [flags]
```

**Example:**
```bash
sawtools queue add
sawtools queue list
sawtools queue next
```

---

## Program Layer

### tier-gate

Verify a tier gate for a PROGRAM manifest. Checks that all IMPLs in the specified tier are complete and all required quality gates pass.

```
sawtools tier-gate <program-manifest> --tier <n> [flags]
```

**Arguments:**
- `program-manifest` -- path to YAML PROGRAM manifest (required)

**Flags:**
- `--tier` -- tier number to verify (required)
- `--repo-dir` -- repository directory (default: current directory)

**Output:** JSON with gate pass/fail result.

**Exit codes:** 0 if tier gate passed, 1 if failed (incomplete IMPLs or gates failed), 2 on parse error or tier not found.

**Example:**
```bash
sawtools tier-gate docs/PROGRAM.yaml --tier 1
sawtools tier-gate docs/PROGRAM.yaml --tier 2 --repo-dir /path/to/repo
```

---

### freeze-contracts

Freeze program contracts at a tier boundary. Verifies that contract source files exist and are committed to HEAD, then updates the manifest's contract state.

```
sawtools freeze-contracts <program-manifest> --tier <n> [flags]
```

**Arguments:**
- `program-manifest` -- path to YAML PROGRAM manifest (required)

**Flags:**
- `--tier` -- tier number (required)
- `--repo-dir` -- repository directory (default: current directory)

**Output:** JSON with freeze results.

**Exit codes:** 0 on success, 1 if contracts missing or uncommitted, 2 on parse error or tier not found.

**Example:**
```bash
sawtools freeze-contracts docs/PROGRAM.yaml --tier 1
sawtools freeze-contracts docs/PROGRAM.yaml --tier 2 --repo-dir /path/to/repo
```

---

### program-replan

Re-engage the Planner agent to revise a PROGRAM manifest. Used when a tier gate fails or a user explicitly requests re-planning.

```
sawtools program-replan <program-manifest> --reason <text> [flags]
```

**Arguments:**
- `program-manifest` -- path to YAML PROGRAM manifest (required)

**Flags:**
- `--reason` -- why re-planning was triggered (required)
- `--tier` -- tier number that failed (0 if user-initiated, default: 0)
- `--model` -- model override for the Planner agent

**Output:** JSON with revised manifest path and validation result.

**Exit codes:** 0 on success, 1 if re-planning or validation failed, 2 on parse error.

**Example:**
```bash
sawtools program-replan docs/PROGRAM.yaml --reason "Tier 2 gate failed: integration tests failing"
sawtools program-replan docs/PROGRAM.yaml --reason "User-initiated replan" --tier 0
sawtools program-replan docs/PROGRAM.yaml --reason "Blocked IMPL" --tier 3 --model claude-opus-4-6
```

---

## Global Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--repo-dir` | `.` | Repository root directory. Inherited by all subcommands that perform git operations. |
