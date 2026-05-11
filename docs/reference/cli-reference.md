# polywave-tools CLI Reference

`polywave-tools` is the Polywave Protocol SDK command-line toolkit. All commands accept a global `--repo-dir` flag (default `.`) specifying the repository root.

```
polywave-tools [command] [args] [flags]
polywave-tools --repo-dir /path/to/repo [command] ...
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
| `detect-shared-types` | Context | Alias for detect-scaffolds (legacy compatibility) |
| `detect-cascades` | Context | Detect files affected by type renames |
| `detect-wiring` | Context | Detect cross-agent function calls and generate wiring declarations |
| `resolve-impl` | Context | Resolve IMPL doc path from slug, filename, or auto-select |
| `interview` | Context | Conduct a structured requirements interview |
| `init` | Setup | Zero-config project initialization |
| `install-hooks` | Setup | Install Polywave git hooks in repository |
| `pre-commit-check` | Quality | Pre-commit validation check (called by hooks) |
| `set-injection-method` | Execution | Set agent injection method for an IMPL |
| `create-worktrees` | Execution | Create git worktrees for agents in a wave |
| `prepare-agent` | Execution | Prepare agent environment (extract brief, init journal) |
| `prepare-wave` | Execution | Prepare all agents in a wave (atomic batch operation) |
| `pre-wave-gate` | Execution | Run pre-wave readiness checks on an IMPL manifest |
| `run-wave` | Execution | Execute full wave lifecycle end-to-end |
| `auto` | Execution | Scout + confirm + wave: the full Polywave flow in one command |
| `run-scout` | Execution | Automated Scout execution with validation (I3) |
| `run-critic` | Execution | Run critic agent to review briefs against codebase (E37) |
| `run-integration-agent` | Execution | Launch integration agent to wire integration gaps (E26) |
| `run-integration-wave` | Execution | Execute planned integration wave (E27) |
| `pre-wave-validate` | Execution | Run E16 validation + E35 gap detection before wave execution |
| `finalize-wave` | Execution | Finalize wave: verify, scan, gate, merge, build, apply-cascade-hotfix, cleanup |
| `finalize-impl` | Execution | Finalize IMPL doc: validate, populate gates, validate again |
| `close-impl` | Execution | Close an IMPL: mark complete, archive, update context, clean worktrees |
| `verify-commits` | Execution | Verify agent branches have commits (I5) |
| `merge-agents` | Execution | Merge all agent branches for a wave |
| `verify-build` | Execution | Run test and lint commands from manifest |
| `cleanup` | Execution | Remove worktrees and branches after merge |
| `cleanup-stale` | Execution | Detect and remove stale Polywave worktrees |
| `verify-isolation` | Execution | Verify agent is in correct isolated worktree (E12) |
| `verify-hook-installed` | Execution | Verify pre-commit hook is installed in worktree (E4) |
| `verify-install` | Execution | Check that all Polywave prerequisites are met |
| `update-status` | Status | Update agent status in manifest |
| `update-context` | Status | Update project CONTEXT.md (E18) |
| `set-completion` | Status | Set completion report for an agent |
| `check-completion` | Status | Check if an agent has a completion report |
| `set-impl-state` | Status | Atomically transition an IMPL manifest to a new protocol state |
| `set-critic-review` | Status | Write critic review result to IMPL doc (E37) |
| `set-critic-verdict` | Status | Update `critic_report.verdict` in an existing IMPL doc |
| `mark-complete` | Status | Write completion marker and archive IMPL manifest |
| `program-status` | Status | Display full program status report |
| `mark-program-complete` | Status | Mark a PROGRAM manifest as complete |
| `scan-stubs` | Quality | Scan files for stub/TODO patterns (E20) |
| `run-gates` | Quality | Run quality gates from manifest |
| `run-review` | Quality | Run AI code review on the current diff |
| `check-conflicts` | Quality | Detect file ownership conflicts |
| `predict-conflicts` | Quality | Predict merge conflicts using hunk-level diff analysis (E11) |
| `check-deps` | Quality | Detect dependency conflicts before wave execution |
| `check-type-collisions` | Quality | Detect type name collisions across agent branches |
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
| `resume-detect` | Recovery | Detect interrupted Polywave sessions in the repository |
| `journal-init` | Journal | Initialize journal directory for a wave agent |
| `journal-context` | Journal | Generate context.md from journal entries for agent recovery |
| `debug-journal` | Journal | Inspect journal contents for debugging failed agents |
| `daemon` | Automation | Run the Polywave daemon loop (process queue items continuously) |
| `queue` | Automation | Manage the IMPL execution queue (add, list, next) |
| `metrics` | Observability | Show metrics for an IMPL (cost, duration, success rate) |
| `query events` | Observability | Query observability events with filters |
| `tier-gate` | Program | Verify tier gate for a PROGRAM manifest |
| `freeze-contracts` | Program | Freeze program contracts at a tier boundary |
| `program-replan` | Program | Re-engage Planner agent to revise a PROGRAM manifest |
| `program-execute` | Program | Execute a PROGRAM manifest through the tier loop |
| `create-program` | Program | Auto-generate a PROGRAM manifest from existing IMPL docs |
| `check-impl-conflicts` | Program | Check file ownership conflicts across IMPL docs |
| `check-program-conflicts` | Program | Detect file conflicts across IMPLs in a program tier |
| `import-impls` | Program | Import existing IMPL docs into a PROGRAM manifest |
| `update-program-impl` | Program | Update IMPL status in a PROGRAM manifest |
| `update-program-state` | Program | Update state field of a PROGRAM manifest |
| `create-program-worktrees` | Program | Create branches/worktrees for all IMPLs in a program tier |
| `prepare-tier` | Program | Prepare a program tier: check conflicts, validate, create branches |
| `finalize-tier` | Program | Finalize a program tier: merge IMPL branches and run tier gate |

---

## Validation

### validate

Validate a YAML IMPL manifest against protocol invariants and E16 typed-block rules.

```
polywave-tools validate <manifest-path> [flags]
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
polywave-tools validate docs/IMPL/my-feature.yaml
polywave-tools validate docs/IMPL/my-feature.yaml --solver
polywave-tools validate docs/IMPL/my-feature.yaml --fix
```

**`file_ownership` entry fields:**

Each entry in the `file_ownership` array supports the following fields:

- `file` (string, required): File path owned by this agent.
- `agent` (string, required): Agent ID that owns this file.
- `wave` (int, required): Wave number in which this file is owned.
- `action` (string, optional): `new`, `modify`, or `delete`.
- `depends_on` ([]string, optional): Other files this file depends on.
- `repo` (string, optional): Repo identifier for cross-repo waves.
- `v048_exempt` (bool, optional): When `true`, skips the V048_AGENT_LOC_BUDGET
  line-count check for this file. Use for large documentation files (e.g., files
  exceeding 2000 lines) that cannot be split. All other validators still apply.
  Default: `false` (omitted from YAML when false).

---

### validate-program

Validate a YAML PROGRAM manifest against schema rules.

```
polywave-tools validate-program <program-manifest>
```

**Arguments:**
- `program-manifest` -- path to YAML PROGRAM manifest (required)

**Output:** JSON validation result.

**Exit codes:** 0 if valid, 1 if any errors found.

**Example:**
```bash
polywave-tools validate-program docs/PROGRAM.yaml
```

---

### validate-integration

Scan a completed wave for unconnected exports using Go AST analysis. Detects heuristic integration gaps and optionally checks wiring declarations (E35 Layer 3B). Persists reports back to the manifest.

```
polywave-tools validate-integration <manifest-path> --wave <n> [flags]
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
polywave-tools validate-integration docs/IMPL/my-feature.yaml --wave 1
polywave-tools validate-integration docs/IMPL/my-feature.yaml --wave 2 --wiring=false
```

---

### solve

Compute optimal wave assignments from dependency declarations using topological sort. Rewrites the manifest in-place with corrected wave numbers.

```
polywave-tools solve <manifest-path> [--dry-run]
```

**Arguments:**
- `manifest-path` -- path to YAML IMPL manifest (required)

**Flags:**
- `--dry-run` -- print changes without writing (default: false)

**Output:** Human-readable text. Prints each reassignment, then a summary line. No JSON output.

**Exit codes:** 0 on success or no changes needed, 1 if the dependency graph cannot be solved (e.g., cycles).

**Example:**
```bash
polywave-tools solve docs/IMPL/my-feature.yaml --dry-run
polywave-tools solve docs/IMPL/my-feature.yaml
```

---

## Context & Discovery

### extract-context

Extract the per-agent context payload from a YAML IMPL manifest. Used to build agent prompts (E23).

```
polywave-tools extract-context <manifest-path> --agent <agent-id>
```

**Arguments:**
- `manifest-path` -- path to YAML IMPL manifest (required)

**Flags:**
- `--agent` -- agent ID to extract context for (required)

**Output:** JSON object containing the agent's task, files, dependencies, and `impl_doc_path`.

**Exit codes:** 0 on success, 1 on error.

**Example:**
```bash
polywave-tools extract-context docs/IMPL/my-feature.yaml --agent A
```

---

### list-impls

List IMPL manifests found in a directory. Excludes completed manifests by default. Prints JSON summaries with path, feature slug, verdict, current wave, and total waves.

```
polywave-tools list-impls [flags]
```

**Flags:**
- `--dir` -- directory to scan (default: `docs/IMPL`)
- `--include-complete` -- include completed/archived IMPL docs (default: false)

**Output:** JSON array of manifest summaries. Empty array is valid.

**Exit codes:** 0 always (empty list is not an error).

**Example:**
```bash
polywave-tools list-impls
polywave-tools list-impls --dir /path/to/impls
polywave-tools list-impls --include-complete
```

---

### list-programs

List PROGRAM manifests found in a directory.

```
polywave-tools list-programs [flags]
```

**Flags:**
- `--dir` -- directory to scan (default: `docs/`)

**Output:** JSON array of program manifest summaries.

**Exit codes:** 0 always.

**Example:**
```bash
polywave-tools list-programs
polywave-tools list-programs --dir /path/to/programs
```

---

### extract-commands

Extract build, test, lint, and format commands from CI configuration files (GitHub Actions, GitLab CI, CircleCI) and build system files (Makefile, package.json). Uses priority-based resolution and falls back to language defaults when no config files are present.

```
polywave-tools extract-commands <repo-root> [flags]
```

**Arguments:**
- `repo-root` -- path to the repository root (required)

**Flags:**
- `--format` -- output format: `yaml` or `json` (default: `yaml`)

**Output:** Command specification matching the Scout IMPL doc schema.

**Exit codes:** 0 on success, 1 on error.

**Example:**
```bash
polywave-tools extract-commands .
polywave-tools extract-commands /path/to/repo --format json
```

---

### analyze-deps

Analyze Go source files to extract import dependencies, detect cycles, compute topological sort, and assign wave structure for parallel agent execution.

```
polywave-tools analyze-deps <repo-root> --files <file-list> [flags]
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
polywave-tools analyze-deps . --files "pkg/foo.go,pkg/bar.go"
polywave-tools analyze-deps /path/to/repo --files "cmd/main.go" --format json
```

---

### analyze-suitability

Scan a codebase to determine which requirements are already implemented (DONE), partially implemented (PARTIAL), or not yet implemented (TODO). Reads a structured markdown requirements document with `Location:` fields.

```
polywave-tools analyze-suitability [flags]
```

**Flags:**
- `--requirements` -- path to requirements/audit doc in markdown format (required)
- `--repo-root` -- repository root directory (default: `.`)
- `--output` -- output format (default: `json`)

**Output:** JSON with status, test coverage, and time savings estimates per requirement.

**Exit codes:** 0 on success, 1 on error.

**Example:**
```bash
polywave-tools analyze-suitability --requirements docs/audit.md --repo-root /path/to/repo
```

---

### detect-scaffolds

Analyze IMPL document to detect types that should be extracted to scaffold files. Pre-agent mode finds types referenced by two or more agents. Post-agent mode detects duplicate type definitions.

```
polywave-tools detect-scaffolds <impl-doc-path> --stage <stage>
```

**Arguments:**
- `impl-doc-path` -- path to IMPL document (required)

**Flags:**
- `--stage` -- detection stage: `pre-agent` or `post-agent` (required)

**Output:** JSON with detected scaffold candidates.

**Exit codes:** 0 on success, 1 on error.

**Example:**
```bash
polywave-tools detect-scaffolds docs/IMPL/my-feature.yaml --stage pre-agent
polywave-tools detect-scaffolds docs/IMPL/my-feature.yaml --stage post-agent
```

---

### detect-shared-types

Alias for `detect-scaffolds`. Maintained for backward compatibility with older IMPL documents and scripts.

```
polywave-tools detect-shared-types <impl-doc-path> --stage <stage>
```

See `detect-scaffolds` for full documentation.

---

### detect-wiring

Analyze IMPL document agent task prompts for cross-agent function calls and generate wiring declarations. Detects patterns such as "calls FunctionName()", "uses pkg.FunctionName", "delegates to X", and "invokes FunctionName".

```
polywave-tools detect-wiring <impl-doc-path> [flags]
```

**Arguments:**
- `impl-doc-path` -- path to IMPL document (required)

**Flags:**
- `--format` -- output format: `yaml` or `json` (default: `yaml`)

**Output:** Wiring declarations (YAML or JSON) under a `wiring` key. The declarations specify which functions must be called from which files to satisfy cross-agent dependencies.

**Exit codes:** 0 if analysis succeeds (even if no wiring declarations found), 1 if IMPL doc is malformed or repo root cannot be determined.

**Example:**
```bash
polywave-tools detect-wiring docs/IMPL/my-feature.yaml
polywave-tools detect-wiring docs/IMPL/my-feature.yaml --format json
```

---

### detect-cascades

Detect files affected by type renames in a repository. Outputs cascade candidates with severity levels and reasons.

```
polywave-tools detect-cascades <repo-root> --renames <json>
```

**Arguments:**
- `repo-root` -- path to the repository root (required)

**Flags:**
- `--renames` -- JSON array of rename objects, e.g. `[{"old":"AuthToken","new":"SessionToken","scope":"pkg/auth"}]` (required)

**Output:** YAML matching the CascadeResult schema.

**Exit codes:** 0 on success, 1 on error.

**Example:**
```bash
polywave-tools detect-cascades . --renames '[{"old":"AuthToken","new":"SessionToken","scope":"pkg/auth"}]'
```

---

### resolve-impl

Resolve an `--impl` flag value (slug, filename, or path) to an absolute IMPL doc path. Supports auto-selection when exactly one pending IMPL exists. Used by orchestrators to canonicalize IMPL targeting across all commands.

```
polywave-tools resolve-impl [flags]
```

**Flags:**
- `--impl` -- IMPL slug, filename (e.g. `IMPL-feature.yaml`), or path (absolute or relative). Omit to auto-select when exactly one pending IMPL exists.

**Resolution order:**
1. Absolute path → used directly if file exists
2. Relative path (contains `/`) → resolved from cwd
3. Filename (`*.yaml` / `*.yml`) → looked up in `<repo-dir>/docs/IMPL/`
4. Slug → scanned against `feature_slug` in pending IMPLs
5. Omitted → auto-selected if exactly 1 pending IMPL exists

**Output:** JSON object with `impl_path` (string), `slug` (string), `resolution_method` (`auto-select` | `explicit-slug` | `explicit-filename` | `explicit-path`), and `pending_count` (int).

**Exit codes:** 0 on success, 1 if the IMPL cannot be resolved (file missing, slug not found, or multiple pending IMPLs when auto-selecting).

**Example:**
```bash
polywave-tools resolve-impl
polywave-tools resolve-impl --impl my-feature
polywave-tools resolve-impl --impl IMPL-my-feature.yaml
polywave-tools resolve-impl --impl /abs/path/to/IMPL-feature.yaml
```

---

### interview

Conduct a structured requirements interview that produces a REQUIREMENTS.md file suitable for `/polywave bootstrap` or `/polywave scout`. Walks through 6 phases: overview, scope, requirements, interfaces, stories, and review.

```
polywave-tools interview "<description>" [flags]
```

**Arguments:**
- `description` -- description of the feature/project (required unless `--resume` is used)

**Flags:**
- `--mode` -- question mode: `deterministic` or `llm` (default: `deterministic`)
- `--max-questions` -- soft cap on total questions (default: 18)
- `--project-path` -- optional path to existing project for context
- `--resume` -- path to an existing `INTERVIEW-<slug>.yaml` to resume
- `--output` -- path for output REQUIREMENTS.md (default: `docs/REQUIREMENTS.md`)
- `--docs-dir` -- directory to write INTERVIEW doc (default: `docs`)
- `--non-interactive` -- read answers from stdin without prompts, for testing/piping (default: false)

**Output:** Interactive question-answer loop. On completion, writes `REQUIREMENTS.md` and saves interview state as `INTERVIEW-<slug>.yaml`.

**Exit codes:** 0 on completion, 1 on error, 2 if stdin closes before interview is complete (state is saved for resume).

**Example:**
```bash
polywave-tools interview "Build a REST API for user management"
polywave-tools interview "Add OAuth2 support" --max-questions 12
polywave-tools interview --resume docs/INTERVIEW-my-feature.yaml
echo "answers..." | polywave-tools interview "test" --non-interactive
```

---

## Wave Execution

### create-worktrees

Create git worktrees for all agents in a given wave. Each agent gets an isolated branch and working directory.

```
polywave-tools create-worktrees <manifest-path> --wave <n>
```

**Arguments:**
- `manifest-path` -- path to YAML IMPL manifest (required)

**Flags:**
- `--wave` -- wave number (required)

**Output:** JSON object with worktree paths and branch names per agent.

**Exit codes:** 0 on success, 1 on error.

**Example:**
```bash
polywave-tools create-worktrees docs/IMPL/my-feature.yaml --wave 1
```

---

### prepare-agent

Prepare an agent's execution environment by extracting the agent's brief and initializing the journal observer. For worktree-based agents, writes brief to worktree root. For solo agents, writes to `.polywave-state/`.

```
polywave-tools prepare-agent <manifest-path> --wave <n> --agent <id> [flags]
```

**Arguments:**
- `manifest-path` -- path to YAML IMPL manifest (required)

**Flags:**
- `--wave` -- wave number (required)
- `--agent` -- agent ID (required)
- `--no-worktree` -- solo agent mode (write brief to .polywave-state instead of worktree) (default: false)

**Output:** JSON with prepared agent details.

**Exit codes:** 0 on success, 1 on error.

**Example:**
```bash
polywave-tools prepare-agent docs/IMPL/my-feature.yaml --wave 1 --agent A
polywave-tools prepare-agent docs/IMPL/my-feature.yaml --wave 1 --agent A --no-worktree
```

---

### prepare-wave

Atomic batch command that prepares all agents in a wave for parallel execution. Combines check-deps, create-worktrees, and prepare-agent into a single operation.

```
polywave-tools prepare-wave <manifest-path> --wave <n>
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
polywave-tools prepare-wave docs/IMPL/my-feature.yaml --wave 1
```

---

### pre-wave-gate

Run pre-wave readiness checks on an IMPL manifest. Checks validation, critic review status (E37), scaffold commits, and IMPL state.

```
polywave-tools pre-wave-gate <manifest-path>
```

**Arguments:**
- `manifest-path` -- path to YAML IMPL manifest (required)

**Output:** JSON object with `ready` (bool) and per-check results.

**Exit codes:** 0 if all checks pass (ready=true), 1 if any check fails (ready=false).

**Example:**
```bash
polywave-tools pre-wave-gate docs/IMPL/my-feature.yaml
```

---

### pre-wave-validate

Combined pre-wave check that runs E16 validation (invariants, gates, contracts) followed by E35 same-package caller detection. E35 detects when an agent owns a function definition but does not own all call sites in the same package, preventing post-merge build failures caused by signature drift.

```
polywave-tools pre-wave-validate <manifest-path> --wave <n> [flags]
```

**Arguments:**
- `manifest-path` -- path to YAML IMPL manifest (required)

**Flags:**
- `--wave` -- wave number to validate (required)
- `--fix` -- auto-correct fixable E16 validation issues (default: false)

**Output:** JSON object with two top-level keys: `validation` (E16 result with `valid`, `error_count`, `errors`) and `e35_gaps` (with `passed` bool and `gaps` array).

**Exit codes:** 0 if both E16 validation and E35 gap detection pass, 1 if either fails.

**Example:**
```bash
polywave-tools pre-wave-validate docs/IMPL/my-feature.yaml --wave 1
polywave-tools pre-wave-validate docs/IMPL/my-feature.yaml --wave 2 --fix
```

---

### run-wave

Execute the full wave lifecycle: create worktrees, verify commits, merge agents, verify build, and cleanup.

```
polywave-tools run-wave <manifest-path> --wave <n> [flags]
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
polywave-tools run-wave docs/IMPL/my-feature.yaml --wave 2
polywave-tools run-wave docs/IMPL/my-feature.yaml --wave 1 --no-prioritize
```

---

### run-scout

Fully automated Scout workflow (I3 integration). Launches Scout agent to analyze codebase, creates IMPL doc, validates it (E16), auto-corrects agent IDs (M1), and optionally runs a critic review (E37).

```
polywave-tools run-scout <feature-description> [flags]
```

**Arguments:**
- `feature-description` -- description of the feature to implement (required)

**Flags:**
- `--repo-dir` -- target repository path (default: current directory)
- `--saw-repo` -- Polywave protocol repo path (default: `$POLYWAVE_REPO` or `~/code/polywave`)
- `--scout-model` -- Scout model override (e.g., `claude-opus-4-6`)
- `--critic-model` -- critic agent model override (e.g., `claude-opus-4-6`)
- `--no-critic` -- skip critic gate even if agent count threshold is met (default: false)
- `--program` -- path to PROGRAM manifest (Scout receives frozen contracts as input)
- `--timeout` -- timeout in minutes (default: 10)

**Output:** Validated IMPL doc at `docs/IMPL/IMPL-<slug>.yaml`, ready for wave execution.

**Exit codes:** 0 on success, 1 on error.

**Example:**
```bash
polywave-tools run-scout "Add audit logging to auth module"
polywave-tools run-scout "Add audit logging" --repo-dir /path/to/project
polywave-tools run-scout "Add audit logging" --scout-model claude-opus-4-6
```

---

### run-critic

Launch a critic agent (E37) that reviews every agent brief in the IMPL doc against the actual codebase before wave execution begins. Verifies file existence, symbol accuracy, pattern accuracy, interface consistency, import chains, and side-effect completeness.

```
polywave-tools run-critic <impl-path> [flags]
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
polywave-tools run-critic docs/IMPL/IMPL-feature.yaml
polywave-tools run-critic docs/IMPL/IMPL-feature.yaml --model claude-opus-4-6
polywave-tools run-critic docs/IMPL/IMPL-feature.yaml --skip
```

---

### run-integration-agent

Automated integration agent workflow (E26). Runs `validate-integration` (or uses an existing integration report from the manifest), and if gaps are found, launches an integration agent to wire the exports. Verifies the build after the agent completes.

```
polywave-tools run-integration-agent <manifest-path> --wave <n>
```

**Arguments:**
- `manifest-path` -- path to YAML IMPL manifest (required)

**Flags:**
- `--wave` -- wave number to check for integration gaps (required)

**Output:** JSON object with `success` (bool), `gap_count` (int), `agent_launched` (bool), `build_passed` (bool), and `completion_report`. When no gaps are found, `agent_launched` is false and the command exits 0 immediately.

**Exit codes:** 0 on success or when no gaps are found, 1 if the integration agent fails.

**Example:**
```bash
polywave-tools run-integration-agent docs/IMPL/IMPL-feature.yaml --wave 1
polywave-tools run-integration-agent docs/IMPL/IMPL-feature.yaml --wave 2
```

---

### run-integration-wave

Execute a planned integration wave (E27) where Scout pre-assigned the wiring work in the IMPL doc. Verifies that the target wave has `type: integration`, extracts agent briefs, and outputs metadata for the orchestrator to launch agents.

```
polywave-tools run-integration-wave <manifest-path> --wave <n>
```

**Arguments:**
- `manifest-path` -- path to YAML IMPL manifest (required)

**Flags:**
- `--wave` -- wave number to execute (required; must be a wave with `type: integration`)

**Output:** JSON object with `success` (bool), `wave` (int), and `agents` array. Each agent entry contains `id`, `files`, and `brief` path. Agent briefs are written to `.polywave-agent-brief.md` in the repo root. Diagnostic messages go to stderr.

**Exit codes:** 0 on success, 1 if the wave is not found or is not type `integration`.

**Example:**
```bash
polywave-tools run-integration-wave docs/IMPL/IMPL-feature.yaml --wave 2
```

---

### auto

Collapse the three-step scout → review → wave flow into a single command. Runs Scout, displays the IMPL summary, prompts for confirmation, then executes all waves in sequence. The human confirmation checkpoint is preserved by default.

Behavior by Scout verdict:
- `NOT_SUITABLE` — shows reason and suggests running `/polywave interview` first, exits 0
- `SUITABLE` — shows IMPL summary, prompts for confirmation, then executes waves
- `SUITABLE_WITH_CAVEATS` — shows IMPL summary and caveats, prompts for confirmation, then executes waves

```
polywave-tools auto <feature-description> [flags]
```

**Arguments:**
- `feature-description` -- description of the feature to implement (required)

**Flags:**
- `--saw-repo` -- Polywave protocol repo path (default: `$POLYWAVE_REPO` or `~/code/polywave`)
- `--scout-model` -- Scout model override (e.g., `claude-opus-4-6`)
- `--wave-model` -- wave model override (reserved for future use)
- `--timeout` -- Scout timeout in minutes (default: 10)
- `--skip-confirm` -- skip the confirmation prompt; proceed directly to wave execution (expert/CI use only, default: false)

**Output:** Scout streaming output, IMPL summary, and wave execution progress to stdout. Final line confirms completion and prints the IMPL path.

**Exit codes:** 0 on success or when verdict is `NOT_SUITABLE`, 1 if Scout or any wave fails.

**Example:**
```bash
polywave-tools auto "Add audit logging to auth module"
polywave-tools auto "Add caching layer" --repo-dir /path/to/project
polywave-tools auto "Refactor storage layer" --skip-confirm
polywave-tools auto "Add OAuth support" --scout-model claude-opus-4-6 --timeout 20
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
6a. apply-cascade-hotfix (E47): if CallerCascadeOnly, auto-launch hotfix agent
7. RunPostMergeGates (content/integration gates, E21)
8. Cleanup

```
polywave-tools finalize-wave <manifest-path> --wave <n>
```

**Arguments:**
- `manifest-path` -- path to YAML IMPL manifest (required)

**Flags:**
- `--wave` -- wave number (required)
- `--dry-run` -- show what cascade errors would be hotfixed without running the agent; outputs JSON with `step`, `dry_run`, `error_count`, `files`, `errors`; exits 0

**Output:** JSON object with per-step results. When CallerCascadeOnly errors are detected and hotfix runs, an additional JSON object appears: `{"step":"apply-cascade-hotfix","status":"success","files_fixed":[...],"commit":"<sha>","build_passed":true}`

**Exit codes:** 0 if all steps succeed, 1 if any step fails.

**Example:**
```bash
polywave-tools finalize-wave docs/IMPL/my-feature.yaml --wave 1
```

---

### apply-cascade-hotfix (finalize-wave step)

Named step within `finalize-wave` (E47). Automatically triggered when
`VerifyBuild` fails and `CallerCascadeOnly=true` — meaning ALL errors are
in future-wave-owned or unowned files, not in the current wave's own files.

**When it runs:** Immediately after step 6 (VerifyBuild), before step 7
(RunPostMergeGates). Not invoked by the Orchestrator directly.

**What it does:**
1. Launches an integration-style agent restricted to files listed in
   `CallerCascadeErrors`
2. Agent applies minimal caller fixes: result.Result[T] unwrapping,
   ctx param additions, deleted symbol replacements
3. Agent commits as:
   `[PW:wave{N}:integration-hotfix] fix caller cascade after wave N signature changes`
4. Re-runs `go build ./... && go vet ./...` to confirm clean

**Output fields:** `step` (string), `status` ("success"), `files_fixed`
([]string), `commit` (string SHA), `build_passed` (bool).

**Dry-run mode:** Pass `--dry-run` to `finalize-wave` to see what would
be hotfixed without running the agent.

**Failure:** If the agent cannot fix all errors, `finalize-wave` exits 1
with `"apply-cascade-hotfix: build still fails after hotfix"`. Route
through E7/E8 as a genuine build failure.

---

### finalize-impl

Atomic batch command that validates an IMPL doc, extracts build/test/lint commands (H2), populates verification gate blocks for all agents, and validates again. Transactional (rolls back on failure) and idempotent. Supports multi-repo IMPLs.

```
polywave-tools finalize-impl <manifest-path> [flags]
```

**Arguments:**
- `manifest-path` -- path to YAML IMPL manifest (required)

**Flags:**
- `--repo-root` -- repository root directory (default: `.`)

**Output:** JSON with validation results and gate population stats.

**Exit codes:** 0 on success, 1 on error.

**Example:**
```bash
polywave-tools finalize-impl docs/IMPL/IMPL-feature-x.yaml
polywave-tools finalize-impl docs/IMPL/IMPL-feature-x.yaml --repo-root /path/to/repo
```

---

### close-impl

Batch command that combines the full IMPL close lifecycle into one operation: write completion marker, archive to `complete/`, update CONTEXT.md, and clean stale worktrees for this IMPL.

```
polywave-tools close-impl <manifest-path> [flags]
```

**Arguments:**
- `manifest-path` -- path to YAML IMPL manifest (required)

**Flags:**
- `--date` -- completion date in `YYYY-MM-DD` format (default: today)

**Output:** JSON object with `marked` (bool), `date`, `archived_path`, `context_updated` (bool), `worktrees_cleaned` (int), and `state_cleaned` (int).

**Exit codes:** 0 on success, 1 on error.

**Example:**
```bash
polywave-tools close-impl docs/IMPL/IMPL-feature.yaml
polywave-tools close-impl docs/IMPL/IMPL-feature.yaml --date 2026-03-22
```

---

### verify-commits

Verify that each agent branch in a wave has at least one commit. Implements the I5 trip wire check.

```
polywave-tools verify-commits <manifest-path> --wave <n>
```

**Arguments:**
- `manifest-path` -- path to YAML IMPL manifest (required)

**Flags:**
- `--wave` -- wave number (required)

**Output:** JSON object with `all_valid` (bool) and per-agent results.

**Exit codes:** 0 if all agents have commits, 1 if any agent branch is empty.

**Example:**
```bash
polywave-tools verify-commits docs/IMPL/my-feature.yaml --wave 1
```

---

### merge-agents

Merge all agent branches for a wave into the integration branch.

```
polywave-tools merge-agents <manifest-path> --wave <n>
```

**Arguments:**
- `manifest-path` -- path to YAML IMPL manifest (required)

**Flags:**
- `--wave` -- wave number (required)

**Output:** JSON object with `success` (bool) and per-agent merge results.

**Exit codes:** 0 if all merges succeed, 1 if any merge fails.

**Example:**
```bash
polywave-tools merge-agents docs/IMPL/my-feature.yaml --wave 1
```

---

### verify-build

Run the test and lint commands declared in the manifest.

```
polywave-tools verify-build <manifest-path>
```

**Arguments:**
- `manifest-path` -- path to YAML IMPL manifest (required)

**Output:** JSON object with `test_passed` (bool), `lint_passed` (bool), and command output.

**Exit codes:** 0 if both tests and lint pass, 1 if either fails.

**Example:**
```bash
polywave-tools verify-build docs/IMPL/my-feature.yaml
```

---

### cleanup

Remove worktrees and branches for a wave after a successful merge.

```
polywave-tools cleanup <manifest-path> --wave <n>
```

**Arguments:**
- `manifest-path` -- path to YAML IMPL manifest (required)

**Flags:**
- `--wave` -- wave number (required)

**Output:** JSON object with cleanup results per agent.

**Exit codes:** 0 on success, 1 on error.

**Example:**
```bash
polywave-tools cleanup docs/IMPL/my-feature.yaml --wave 1
```

---

### cleanup-stale

Detect and remove stale Polywave worktrees (completed IMPLs, orphaned branches, merged-but-not-cleaned).

```
polywave-tools cleanup-stale [flags]
```

**Flags:**
- `--slug` -- only clean stale worktrees matching this IMPL slug
- `--all` -- clean stale worktrees across all slugs
- `--dry-run` -- report what would be cleaned without acting (default: false)
- `--force` -- skip safety checks for uncommitted changes (default: false)

**Note:** Exactly one of `--slug` or `--all` must be provided.

**Output:** JSON object with `detected` (int), `cleaned` (array), `skipped` (array), and `errors` (array).

**Exit codes:** 0 on success, 1 on error.

**Example:**
```bash
polywave-tools cleanup-stale --all --dry-run
polywave-tools cleanup-stale --slug my-feature
polywave-tools cleanup-stale --all --force
```

---

### verify-install

Check that all Polywave prerequisites are met: polywave-tools binary on PATH, Git version >= 2.20, skill directory and files, config file, and configured repo paths.

```
polywave-tools verify-install [flags]
```

**Flags:**
- `--human` -- print human-readable output instead of JSON (default: false)

**Output:** JSON object with `checks` (array of per-check results), `verdict` (`PASS`, `PARTIAL`, or `FAIL`), and `summary`.

**Exit codes:** 0 always (verdict is informational).

**Example:**
```bash
polywave-tools verify-install
polywave-tools verify-install --human
```

---

### init

Initialize a new Polywave-managed repository with zero configuration. Creates `docs/IMPL/`, `docs/IMPL/complete/`, and `polywave.config.json` with sensible defaults.

```
polywave-tools init [flags]
```

**Flags:**
- `--repo-dir` -- target repository path (default: current directory)

**Output:** JSON confirmation of created files and directories.

**Exit codes:** 0 on success, 1 on error.

**Example:**
```bash
polywave-tools init
polywave-tools init --repo-dir /path/to/new/repo
```

---

### install-hooks

Install Polywave git hooks in a repository. Installs pre-commit hook for worktree isolation enforcement (E43) and other validation checks.

```
polywave-tools install-hooks [flags]
```

**Flags:**
- `--repo-dir` -- target repository path (default: current directory)
- `--force` -- overwrite existing hooks (default: false)

**Output:** JSON confirmation of installed hooks.

**Exit codes:** 0 on success, 1 on error.

**Example:**
```bash
polywave-tools install-hooks
polywave-tools install-hooks --repo-dir /path/to/repo --force
```

---

### pre-commit-check

Run pre-commit validation checks. Called automatically by the pre-commit hook. Validates worktree isolation, file ownership, and other protocol invariants.

```
polywave-tools pre-commit-check [flags]
```

**Flags:**
- `--repo-dir` -- repository root path (default: current directory)

**Output:** JSON validation result with `ok` (bool) and diagnostic messages.

**Exit codes:** 0 if all checks pass, 1 if any check fails.

**Example:**
```bash
polywave-tools pre-commit-check
```

**Note:** This command is typically called by the pre-commit hook and not invoked manually.

---

### set-injection-method

Set the agent injection method for an IMPL manifest. Controls how agent prompts receive context (e.g., `full`, `incremental`, `minimal`).

```
polywave-tools set-injection-method <manifest-path> --method <method>
```

**Arguments:**
- `manifest-path` -- path to YAML IMPL manifest (required)

**Flags:**
- `--method` -- injection method: `full` | `incremental` | `minimal` (required)

**Output:** JSON confirmation.

**Exit codes:** 0 on success, 1 on error.

**Example:**
```bash
polywave-tools set-injection-method docs/IMPL/my-feature.yaml --method incremental
```

---

### verify-isolation

Verify that the current working directory is an isolated agent worktree on the expected branch. Enforces Field 0 / E12.

```
polywave-tools verify-isolation --branch <branch-name>
```

**Flags:**
- `--branch` -- expected branch name, e.g. `wave1-agent-A` (required)

**Output:** JSON object with `ok` (bool), actual branch, and expected branch.

**Exit codes:** 0 if isolation is correct, 1 if the branch does not match.

**Example:**
```bash
polywave-tools verify-isolation --branch wave1-agent-A
```

---

### verify-hook-installed

Verify that the pre-commit hook exists in a worktree's `.git/hooks` directory, contains isolation check logic, and is executable. Layer 0 of worktree isolation enforcement (E4).

```
polywave-tools verify-hook-installed <worktree-path> [flags]
```

**Arguments:**
- `worktree-path` -- path to the worktree to check (required)

**Flags:**
- `--wave` -- wave number (for context in error messages)

**Output:** JSON with hook validation result.

**Exit codes:** 0 if hook is valid, 1 if hook is missing or broken.

**Example:**
```bash
polywave-tools verify-hook-installed /tmp/saw-worktrees/wave1-agent-A
polywave-tools verify-hook-installed /tmp/saw-worktrees/wave1-agent-A --wave 1
```

---

## Status & Reporting

### update-status

Update an agent's status field in the manifest.

```
polywave-tools update-status <manifest-path> --wave <n> --agent <id> --status <status>
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
polywave-tools update-status docs/IMPL/my-feature.yaml --wave 1 --agent A --status complete
```

---

### update-context

Update the project's `CONTEXT.md` file from the manifest. Implements E18.

```
polywave-tools update-context <manifest-path> [--project-root <path>]
```

**Arguments:**
- `manifest-path` -- path to YAML IMPL manifest (required)

**Flags:**
- `--project-root` -- project root directory (default: `.`)

**Output:** JSON object confirming the update.

**Exit codes:** 0 on success, 1 on error.

**Example:**
```bash
polywave-tools update-context docs/IMPL/my-feature.yaml --project-root /path/to/project
```

---

### check-completion

Check if an agent has a completion report in the manifest. Used by the SubagentStop hook for wave agent validation.

```
polywave-tools check-completion <manifest-path> --agent <id>
```

**Arguments:**
- `manifest-path` -- path to YAML IMPL manifest (required)

**Flags:**
- `--agent` -- agent ID to check (required)

**Output:** JSON object with `found` (bool), `agent_id`, `status`, `has_commit` (bool), and `files_changed` (array).

**Exit codes:** 0 if completion report found, 1 if not found or status is empty.

**Example:**
```bash
polywave-tools check-completion docs/IMPL/my-feature.yaml --agent A
```

---

### set-impl-state

Atomically transition an IMPL manifest to a new protocol state. Validates the transition against the protocol state machine before writing.

```
polywave-tools set-impl-state <manifest-path> --state <state> [flags]
```

**Arguments:**
- `manifest-path` -- path to YAML IMPL manifest (required)

**Flags:**
- `--state` -- target state (required). Valid states: `INTERVIEWING`, `SCOUT_PENDING`, `SCOUT_VALIDATING`, `REVIEWED`, `SCAFFOLD_PENDING`, `WAVE_PENDING`, `WAVE_EXECUTING`, `WAVE_MERGING`, `WAVE_VERIFIED`, `BLOCKED`, `COMPLETE`, `NOT_SUITABLE`
- `--commit` -- git commit the state change (default: false)
- `--commit-msg` -- commit message override

**Output:** JSON with `previous_state`, `new_state`, `committed`, `commit_sha`.

**Exit codes:** 0 on success, 1 on invalid transition or error.

**Example:**
```bash
polywave-tools set-impl-state docs/IMPL/my-feature.yaml --state REVIEWED
polywave-tools set-impl-state docs/IMPL/my-feature.yaml --state WAVE_PENDING --commit
```

---

### set-completion

Set a completion report for an agent in the manifest. Records commit, files changed/created, tests added, and verification results.

```
polywave-tools set-completion <manifest-path> --agent <id> --status <status> --commit <sha> [flags]
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
polywave-tools set-completion docs/IMPL/my-feature.yaml \
  --agent A --status complete --commit abc1234 \
  --files-changed "pkg/foo.go,pkg/bar.go" \
  --tests-added "pkg/foo_test.go"
```

---

### set-critic-review

Write critic review result to IMPL doc's `critic_report` field. Called by critic agents after completing their review. Not intended for direct human use.

```
polywave-tools set-critic-review <impl-path> --verdict <verdict> --summary <text> [flags]
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
polywave-tools set-critic-review docs/IMPL/IMPL-feature.yaml \
  --verdict PASS --summary "All briefs verified" \
  --agent-reviews '[{"agent_id":"A","verdict":"PASS","issues":[]}]'
```

---

### set-critic-verdict

Atomically updates `critic_report.verdict` in an existing IMPL doc.
Use after the Orchestrator corrects IMPL issues flagged by the critic,
to transition the verdict from `ISSUES` to `PASS` without manually editing YAML.
This avoids the duplicate-key YAML error that can occur with manual edits.

```
polywave-tools set-critic-verdict <impl-path> --verdict <pass|issues>
```

**Arguments:**
- `impl-path` -- path to IMPL document (required)

**Flags:**
- `--verdict` (required): New verdict value. Accepted values: `pass`, `issues`
  (case-insensitive; stored as uppercase: PASS, ISSUES).

**Output:** JSON object:
```json
{"impl_path": "...", "old_verdict": "ISSUES", "new_verdict": "PASS"}
```

**Exit codes:** 0 on success. 1 if `critic_report` key does not exist in the IMPL.

**Note:** `set-critic-review` is used by critic agents to write the full structured
review result. `set-critic-verdict` is a lightweight Orchestrator-facing command
to update just the verdict field after manual corrections are applied.

---

### mark-complete

Write the final completion marker to an IMPL manifest, recording the completion date, and archive it to the `complete/` subdirectory.

```
polywave-tools mark-complete <manifest-path> [--date <YYYY-MM-DD>]
```

**Arguments:**
- `manifest-path` -- path to YAML IMPL manifest (required)

**Flags:**
- `--date` -- completion date in `YYYY-MM-DD` format (default: today)

**Output:** JSON object with `marked` (bool), `date`, and `path`.

**Exit codes:** 0 on success, 1 on error.

**Example:**
```bash
polywave-tools mark-complete docs/IMPL/my-feature.yaml
polywave-tools mark-complete docs/IMPL/my-feature.yaml --date 2026-03-10
```

---

### program-status

Display comprehensive status report for a PROGRAM manifest, including current tier, per-tier IMPL statuses, contract freeze states, and completion tracking.

```
polywave-tools program-status <program-manifest>
```

**Arguments:**
- `program-manifest` -- path to YAML PROGRAM manifest (required)

**Output:** Human-readable status report.

**Exit codes:** 0 always (status is informational), 2 on parse error.

**Example:**
```bash
polywave-tools program-status docs/PROGRAM.yaml
```

---

### mark-program-complete

Mark a PROGRAM manifest as complete. Verifies all tiers are complete, updates state to PROGRAM_COMPLETE, sets completion date, writes the `polywave:program:complete` marker, updates CONTEXT.md, and commits both files.

```
polywave-tools mark-program-complete <program-manifest> [flags]
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
polywave-tools mark-program-complete docs/PROGRAM.yaml
polywave-tools mark-program-complete docs/PROGRAM.yaml --date 2026-03-15
```

---

## Quality & Safety

### scan-stubs

Scan source files for stub/TODO patterns. Implements E20 enforcement. Optionally appends the stub report to a manifest.

```
polywave-tools scan-stubs <file> [file...] [--append-impl <path>] [--wave <n>]
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
polywave-tools scan-stubs pkg/foo.go pkg/bar.go
polywave-tools scan-stubs pkg/*.go --append-impl docs/IMPL/my-feature.yaml --wave 2
```

---

### run-gates

Run the quality gates declared in the manifest. Gates can be marked as required or optional. Supports gate result caching (E38).

```
polywave-tools run-gates <manifest-path> [flags]
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
polywave-tools run-gates docs/IMPL/my-feature.yaml --wave 1
polywave-tools run-gates docs/IMPL/my-feature.yaml --wave 1 --no-cache
```

---

### run-review

Run AI code review on the current diff. Used as a post-merge quality gate.

```
polywave-tools run-review [flags]
```

**Flags:**
- `--model` -- Anthropic model to use (default: `claude-haiku-4-5`)
- `--threshold` -- minimum overall score (0-100) to pass (default: 70)
- `--blocking` -- exit code 1 on failing review (default: false)

**Output:** JSON review result with scores and feedback.

**Exit codes:** 0 on pass (or non-blocking mode), 1 if blocking and review fails.

**Example:**
```bash
polywave-tools run-review
polywave-tools run-review --blocking --threshold 80
polywave-tools run-review --model claude-sonnet-4-20250514
```

---

### check-conflicts

Detect file ownership conflicts across agent completion reports. Flags files touched by multiple agents.

```
polywave-tools check-conflicts <manifest-path>
```

**Arguments:**
- `manifest-path` -- path to YAML IMPL manifest (required)

**Output:** JSON object with `conflict_count` (int) and `conflicts` array.

**Exit codes:** 0 if no conflicts, 1 if any conflicts found.

**Example:**
```bash
polywave-tools check-conflicts docs/IMPL/my-feature.yaml
```

---

### predict-conflicts

Predict merge conflicts for a wave using hunk-level diff analysis (E11). For each file touched by multiple agents, runs `git diff --unified=0 mergeBase..branch -- file` per agent, parses `@@` line ranges, and checks whether any two agents' modified line ranges overlap. Non-overlapping edits (e.g., cascade patches where each agent modifies a different function) produce exit 0 — only true line-range overlaps are flagged.

```
polywave-tools predict-conflicts <manifest-path> --wave <n>
```

**Arguments:**
- `manifest-path` -- path to YAML IMPL manifest (required)

**Flags:**
- `--wave` -- wave number to check (required)

**Output:** JSON object with `conflicts_detected` (int), `conflicts` (array of conflict predictions per file), and optional `warnings` (array of non-fatal issues encountered during analysis).

**Exit codes:** 0 if no overlapping hunks found (safe to merge), 1 if overlapping hunks are detected (merge conflict likely).

**Example:**
```bash
polywave-tools predict-conflicts docs/IMPL/my-feature.yaml --wave 1
polywave-tools predict-conflicts docs/IMPL/my-feature.yaml --wave 2
```

---

### check-deps

Detect dependency conflicts before wave execution. Analyzes IMPL doc file ownership and lock files for missing dependencies and version conflicts.

```
polywave-tools check-deps <impl-doc> [flags]
```

**Arguments:**
- `impl-doc` -- path to IMPL document (required)

**Flags:**
- `--wave` -- wave number to check (0 = all waves, default: 0)

**Output:** JSON report of dependency conflicts.

**Exit codes:** 0 if no conflicts, 1 if conflicts found.

**Example:**
```bash
polywave-tools check-deps docs/IMPL/my-feature.yaml
polywave-tools check-deps docs/IMPL/my-feature.yaml --wave 1
```

---

### check-type-collisions

Detect type name collisions across agent branches in a wave. Uses AST parsing to extract type names from git diffs (base..branch) and reports duplicate declarations in the same package.

```
polywave-tools check-type-collisions <manifest-path> --wave <n> [flags]
```

**Arguments:**
- `manifest-path` -- path to YAML IMPL manifest (required)

**Flags:**
- `--wave` -- wave number (required)
- `--repo-dir` -- repository root directory (default: `.`)

**Output:** JSON collision report with `valid` (bool) and collision details.

**Exit codes:** 0 if no collisions, 1 if collisions found.

**Example:**
```bash
polywave-tools check-type-collisions docs/IMPL/my-feature.yaml --wave 1
```

---

### validate-scaffolds

Validate that all scaffold files declared in the manifest are committed to the repository.

```
polywave-tools validate-scaffolds <manifest-path>
```

**Arguments:**
- `manifest-path` -- path to YAML IMPL manifest (required)

**Output:** JSON object with `all_committed` (bool), `scaffold_count` (int), and `statuses` array.

**Exit codes:** 0 if all scaffolds are committed, 1 if any are missing.

**Example:**
```bash
polywave-tools validate-scaffolds docs/IMPL/my-feature.yaml
```

---

### validate-scaffold

Validate a single scaffold file before committing. Runs a pipeline: syntax check (Go AST), import resolution, type reference check, and partial build.

```
polywave-tools validate-scaffold <scaffold-file> [flags]
```

**Arguments:**
- `scaffold-file` -- path to scaffold file to validate (required)

**Flags:**
- `--impl-doc` -- path to IMPL doc for build command extraction

**Output:** Structured YAML with pass/fail status per validation step.

**Exit codes:** 0 if all checks pass, 1 if any fail.

**Example:**
```bash
polywave-tools validate-scaffold pkg/types/scaffold.go
polywave-tools validate-scaffold pkg/types/scaffold.go --impl-doc docs/IMPL/my-feature.yaml
```

---

### freeze-check

Check whether a manifest is frozen (worktrees have been created) and detect any freeze violations.

```
polywave-tools freeze-check <manifest-path>
```

**Arguments:**
- `manifest-path` -- path to YAML IMPL manifest (required)

**Output:** JSON object with `frozen` (bool), `violation_count` (int), and `violations` array.

**Exit codes:** 0 if no violations, 1 if any freeze violations found.

**Example:**
```bash
polywave-tools freeze-check docs/IMPL/my-feature.yaml
```

---

### update-agent-prompt

Update an agent's prompt/task text in the manifest. Saves the manifest in-place.

```
polywave-tools update-agent-prompt <manifest-path> --agent <id> --prompt <text>
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
polywave-tools update-agent-prompt docs/IMPL/my-feature.yaml \
  --agent B --prompt "Implement the HTTP handler for /api/widgets"
```

---

### populate-integration-checklist

Determinism tool (M5) that scans `file_ownership` for integration-requiring patterns and populates `post_merge_checklist` groups. Detects new API handlers, React components, CLI commands, and background services. Idempotent.

```
polywave-tools populate-integration-checklist <manifest-path> [flags]
```

**Arguments:**
- `manifest-path` -- path to YAML IMPL manifest (required)

**Flags:**
- `--repo-root` -- repository root for file parsing (default: `.`)

**Output:** Updated IMPL manifest with populated post_merge_checklist.

**Exit codes:** 0 on success, 1 on error.

**Example:**
```bash
polywave-tools populate-integration-checklist docs/IMPL/my-feature.yaml
polywave-tools populate-integration-checklist docs/IMPL/my-feature.yaml --repo-root /path/to/repo
```

---

## Determinism & Analysis

### assign-agent-ids

Generate agent IDs following the `^[A-Z][2-9]?$` pattern. Supports sequential mode (A-Z, then A2-Z2, etc.) and grouped mode with category tags.

```
polywave-tools assign-agent-ids --count <n> [flags]
```

**Flags:**
- `--count` -- number of agents (required)
- `--grouping` -- JSON array of category tags for grouped assignment (optional)

**Output:** Space-separated agent IDs.

**Exit codes:** 0 on success, 1 on error.

**Example:**
```bash
polywave-tools assign-agent-ids --count 5
polywave-tools assign-agent-ids --count 9 --grouping '[["data"],["data"],["data"],["api"],["api"],["ui"],["ui"],["ui"],["ui"]]'
```

---

### diagnose-build-failure

Pattern-match build error logs against known catalogs and suggest fixes. Supports Go, Rust, JavaScript, TypeScript, and Python.

```
polywave-tools diagnose-build-failure <error-log> --language <lang>
```

**Arguments:**
- `error-log` -- path to error log file (required)

**Flags:**
- `--language` -- language: `go`, `rust`, `js`, `ts`, `python` (required)

**Output:** Structured YAML with diagnosis, confidence, fix, rationale, and auto_fixable flag.

**Exit codes:** 0 on success, 1 on error.

**Example:**
```bash
polywave-tools diagnose-build-failure /tmp/build-errors.log --language go
```

---

## Amendment & Recovery

### amend-impl

Mutate a living IMPL doc. Supports three modes: add a new wave, redirect an agent, or extend scope. Exactly one mode flag must be provided.

```
polywave-tools amend-impl <manifest-path> <mode-flag> [flags]
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
polywave-tools amend-impl docs/IMPL/my-feature.yaml --add-wave
polywave-tools amend-impl docs/IMPL/my-feature.yaml --redirect-agent B --wave 1 --new-task "Fix the handler"
polywave-tools amend-impl docs/IMPL/my-feature.yaml --extend-scope
```

---

### retry

Generate a single-agent retry IMPL doc targeting files that failed a quality gate. Implements the E24 verification loop. Does NOT execute the retry agent -- only generates the IMPL doc.

```
polywave-tools retry <impl-doc> --wave <n> --gate-type <type> [flags]
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
polywave-tools retry docs/IMPL/my-feature.yaml --wave 1 --gate-type build
polywave-tools retry docs/IMPL/my-feature.yaml --wave 2 --gate-type test --max-retries 3
```

---

### build-retry-context

Build structured retry context for a failed agent attempt. Reads the completion report, classifies the error type, and outputs actionable retry context.

```
polywave-tools build-retry-context <manifest-path> --agent <id> [flags]
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
polywave-tools build-retry-context docs/IMPL/my-feature.yaml --agent B
polywave-tools build-retry-context docs/IMPL/my-feature.yaml --agent B --attempt 2
```

---

### resume-detect

Detect interrupted Polywave sessions in the repository. Scans `docs/IMPL/` for manifests that are not complete or unsuitable, inspects completion reports and git worktrees.

```
polywave-tools resume-detect
```

**Output:** JSON array of interrupted session details. Empty array if none found.

**Exit codes:** 0 always.

**Example:**
```bash
polywave-tools resume-detect
```

---

## Journal & Debugging

### journal-init

Initialize the journal directory structure (`.polywave-state/journals/wave<N>/agent-<ID>/`) and cursor file for tracking Claude Code session log position.

```
polywave-tools journal-init <manifest-path> --wave <n> --agent <id>
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
polywave-tools journal-init docs/IMPL/my-feature.yaml --wave 1 --agent A
```

---

### journal-context

Manually sync the journal from Claude Code session logs and generate a markdown context summary for an agent. As of `journal-integration`, context generation runs **automatically** during `prepare-wave` and `prepare-agent` — this command is now primarily for manual inspection or regeneration.

```
polywave-tools journal-context <manifest-path> --wave <n> --agent <id> [flags]
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

**Automatic context generation:** `prepare-wave` and `prepare-agent` now call `Sync()` + `GenerateContext()` automatically for each agent after cursor initialization. The result is surfaced in their JSON output:
- `journal_context_available` (bool) — `true` when prior session history exists
- `journal_context_file` (string) — absolute path to the generated `context.md`

The Orchestrator reads these fields and prepends the context file contents to the agent's launch prompt when `journal_context_available` is `true` (see step 6 in `polywave-skill.md`).

**Example:**
```bash
polywave-tools journal-context docs/IMPL/my-feature.yaml --wave 1 --agent A
polywave-tools journal-context docs/IMPL/my-feature.yaml --wave 1 --agent A --max-entries 50
```

---

### debug-journal

Inspect tool execution journal for a specific agent. Supports multiple output modes.

```
polywave-tools debug-journal <agent-path> [flags]
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
polywave-tools debug-journal wave1/agent-A
polywave-tools debug-journal wave1/agent-A --summary
polywave-tools debug-journal wave1/agent-A --failures-only
polywave-tools debug-journal wave1/agent-A --last 20
polywave-tools debug-journal wave1/agent-A --export timeline.html
```

---

## Automation

### daemon

Run the Polywave daemon loop that continuously monitors the IMPL queue, runs Scout/Wave cycles, auto-remediates failures, and advances the queue.

```
polywave-tools daemon [flags]
```

**Flags:**
- `--autonomy` -- override autonomy level: `gated`, `supervised`, or `autonomous`
- `--model` -- chat model to use
- `--poll-interval` -- how often to check the queue (default: `30s`)

**Output:** JSON lines streamed to stdout.

**Exit codes:** 0 on clean shutdown, 1 on error.

**Example:**
```bash
polywave-tools daemon
polywave-tools daemon --autonomy supervised --poll-interval 10s
polywave-tools daemon --model claude-opus-4-6
```

---

### queue

Manage the IMPL execution queue. Has three subcommands: `add`, `list`, and `next`.

#### queue add

Add an item to the execution queue.

```
polywave-tools queue add [flags]
```

**Flags:**
- `--title` -- item title (required)
- `--priority` -- item priority, lower = higher priority (default: 50)
- `--description` -- feature description

**Output:** JSON with `added` (bool), `slug`, and `path`.

#### queue list

List all items in the execution queue, sorted by priority.

```
polywave-tools queue list
```

**Output:** JSON array of queue items with `slug`, `title`, `priority`, and `status`.

#### queue next

Get the next eligible item from the execution queue.

```
polywave-tools queue next
```

**Output:** JSON with next item's `slug`, `title`, `priority`, or `{"next": null}` if queue is empty.

**Example:**
```bash
polywave-tools queue add --title "Add audit logging" --priority 10 --description "Add logging to auth module"
polywave-tools queue list
polywave-tools queue next
```

---

## Observability

### metrics

Show cost and performance metrics for an IMPL from the observability store.

```
polywave-tools metrics <impl-slug> [flags]
```

**Arguments:**
- `impl-slug` -- IMPL slug to query metrics for (required)

**Flags:**
- `--program` -- show program-level summary instead of IMPL metrics
- `--breakdown` -- show per-agent cost breakdown (default: false)
- `--store` -- store DSN (default: `~/.polywave/observability.db`)

**Output:** Human-readable table with cost, duration, success rate, and agent stats. With `--breakdown`, includes per-agent cost table.

**Exit codes:** 0 on success, 1 on error.

**Example:**
```bash
polywave-tools metrics my-feature
polywave-tools metrics my-feature --breakdown
polywave-tools metrics --program my-program
```

---

### query

Query observability data with various subcommands and filters.

**Subcommands:**
- `events` -- Query observability events

```
polywave-tools query events [flags]
```

**Flags:**
- `--type` -- event type filter (`cost`, `agent_performance`, `activity`)
- `--impl` -- IMPL slug filter
- `--program` -- program slug filter
- `--agent` -- agent ID filter
- `--since` -- time range (e.g., `24h`, `7d`, `30d`)
- `--format` -- output format: `table`, `json`, or `csv` (default: `table`)
- `--limit` -- max results to return (default: 100)
- `--store` -- store DSN (default: `~/.polywave/observability.db`)

**Output:** Events in the selected format (table, JSON, or CSV).

**Exit codes:** 0 on success, 1 on error.

**Example:**
```bash
polywave-tools query events --type cost --since 7d
polywave-tools query events --impl my-feature --format json
polywave-tools query events --agent A --since 24h --limit 50
```

---

## Program Layer

### tier-gate

Verify a tier gate for a PROGRAM manifest. Checks that all IMPLs in the specified tier are complete and all required quality gates pass.

```
polywave-tools tier-gate <program-manifest> --tier <n> [flags]
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
polywave-tools tier-gate docs/PROGRAM.yaml --tier 1
polywave-tools tier-gate docs/PROGRAM.yaml --tier 2 --repo-dir /path/to/repo
```

---

### freeze-contracts

Freeze program contracts at a tier boundary. Verifies that contract source files exist and are committed to HEAD, then updates the manifest's contract state.

```
polywave-tools freeze-contracts <program-manifest> --tier <n> [flags]
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
polywave-tools freeze-contracts docs/PROGRAM.yaml --tier 1
polywave-tools freeze-contracts docs/PROGRAM.yaml --tier 2 --repo-dir /path/to/repo
```

---

### program-replan

Re-engage the Planner agent to revise a PROGRAM manifest. Used when a tier gate fails or a user explicitly requests re-planning.

```
polywave-tools program-replan <program-manifest> --reason <text> [flags]
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
polywave-tools program-replan docs/PROGRAM.yaml --reason "Tier 2 gate failed: integration tests failing"
polywave-tools program-replan docs/PROGRAM.yaml --reason "User-initiated replan" --tier 0
polywave-tools program-replan docs/PROGRAM.yaml --reason "Blocked IMPL" --tier 3 --model claude-opus-4-6
```

---

### program-execute

Execute a PROGRAM manifest through the tier loop (E28-E34). Launches parallel Scouts for pending IMPLs, executes waves, runs tier gates, freezes contracts, and advances through tiers. Events are streamed as JSON lines for observability.

```
polywave-tools program-execute <program-manifest> [flags]
```

**Arguments:**
- `program-manifest` -- path to YAML PROGRAM manifest (required)

**Flags:**
- `--auto` -- enable auto-advancement through tiers (default: false)
- `--model` -- model override for Scout/Planner agents

**Output:** JSON lines streamed to stdout during execution, then a final JSON result object.

**Exit codes:** 0 if program complete or paused awaiting review, 1 on execution failure, 2 on parse error.

**Example:**
```bash
polywave-tools program-execute docs/PROGRAM.yaml
polywave-tools program-execute docs/PROGRAM.yaml --auto
polywave-tools program-execute docs/PROGRAM.yaml --auto --model claude-opus-4-6
```

---

### create-program

Auto-generate a PROGRAM manifest from existing IMPL docs. Uses cross-IMPL conflict detection to automatically assign tiers so that IMPLs with overlapping file ownership are placed in separate tiers.

```
polywave-tools create-program [flags]
```

**Flags:**
- `--from-impls` -- IMPL slugs to include (required, at least 2; repeatable)
- `--slug` -- override program slug (auto-derived if empty)
- `--title` -- override program title (auto-derived if empty)

**Output:** JSON with generated program details and conflict report.

**Exit codes:** 0 on success, 1 on error.

**Example:**
```bash
polywave-tools create-program --from-impls feature-a --from-impls feature-b
polywave-tools create-program --from-impls feature-a --from-impls feature-b --slug my-program --title "My Program"
```

---

### check-impl-conflicts

Check for file ownership conflicts across IMPL docs. Loads IMPL docs by slug, extracts `file_ownership` entries, and detects overlapping files.

```
polywave-tools check-impl-conflicts [flags]
```

**Flags:**
- `--impls` -- IMPL slugs to check for conflicts (required; repeatable)

**Output:** JSON conflict report.

**Exit codes:** 0 if all disjoint, 1 if conflicts found.

**Example:**
```bash
polywave-tools check-impl-conflicts --impls feature-a --impls feature-b
```

---

### check-program-conflicts

Detect file ownership conflicts across IMPLs in a specific program tier.

```
polywave-tools check-program-conflicts <program-manifest> --tier <n>
```

**Arguments:**
- `program-manifest` -- path to YAML PROGRAM manifest (required)

**Flags:**
- `--tier` -- tier number to check (required)

**Output:** JSON conflict report.

**Exit codes:** 0 if no conflicts, 1 if conflicts found.

**Example:**
```bash
polywave-tools check-program-conflicts docs/PROGRAM.yaml --tier 1
```

---

### import-impls

Import existing IMPL documents into a PROGRAM manifest for tiered execution. Supports both explicit import and auto-discovery. Computes tier assignments based on file ownership overlap.

```
polywave-tools import-impls [flags]
```

**Flags:**
- `--program` -- path to PROGRAM manifest; created if missing (required)
- `--from-impls` -- explicit IMPL doc paths to import (repeatable)
- `--discover` -- auto-discover IMPL docs in `docs/IMPL/` (default: false)
- `--repo-dir` -- repository root directory (default: cwd)

**Note:** Either `--from-impls` or `--discover` must be specified.

**Output:** JSON with imported IMPLs, tier assignments, and P1/P2 conflict detection.

**Exit codes:** 0 on success, 1 on error.

**Example:**
```bash
polywave-tools import-impls --program PROGRAM-my-feature.yaml --discover
polywave-tools import-impls --program PROGRAM-my-feature.yaml --from-impls IMPL-a.yaml --from-impls IMPL-b.yaml
```

---

### update-program-impl

Update the status of a specific IMPL entry in a PROGRAM manifest. The IMPL is identified by its slug field.

```
polywave-tools update-program-impl <program-manifest> --impl <slug> --status <status>
```

**Arguments:**
- `program-manifest` -- path to YAML PROGRAM manifest (required)

**Flags:**
- `--impl` -- IMPL slug to update (required)
- `--status` -- new status value, e.g. `complete`, `in_progress`, `blocked` (required)

**Output:** JSON with `updated` (bool), `manifest_path`, `impl_slug`, `previous_status`, `new_status`.

**Exit codes:** 0 on success, 1 if IMPL not found or write error, 2 on parse error.

**Example:**
```bash
polywave-tools update-program-impl docs/PROGRAM.yaml --impl my-feature --status complete
```

---

### update-program-state

Update the state field of a PROGRAM manifest.

```
polywave-tools update-program-state <program-manifest> --state <state>
```

**Arguments:**
- `program-manifest` -- path to YAML PROGRAM manifest (required)

**Flags:**
- `--state` -- new state value, e.g. `REVIEWED`, `TIER_EXECUTING` (required)

**Output:** JSON with `updated` (bool), `manifest_path`, `previous_state`, `new_state`.

**Exit codes:** 0 on success, 1 on update or write error, 2 on parse error.

**Example:**
```bash
polywave-tools update-program-state docs/PROGRAM.yaml --state REVIEWED
polywave-tools update-program-state docs/PROGRAM.yaml --state TIER_EXECUTING
```

---

### create-program-worktrees

Create IMPL branches and worktrees for all IMPLs in a program tier. Branch naming follows `polywave/program/{program-slug}/tier{N}-impl-{impl-slug}`. These branches serve as merge targets for all wave executions within each IMPL.

```
polywave-tools create-program-worktrees <program-manifest> --tier <n>
```

**Arguments:**
- `program-manifest` -- path to YAML PROGRAM manifest (required)

**Flags:**
- `--tier` -- tier number (required)

**Output:** JSON with created worktree paths and branch details.

**Exit codes:** 0 if all worktrees created, 1 if one or more failed, 2 on parse error or tier not found.

**Example:**
```bash
polywave-tools create-program-worktrees docs/PROGRAM.yaml --tier 1
polywave-tools create-program-worktrees program.yaml --tier 2 --repo-dir /path/to/repo
```

---

### prepare-tier

Prepare a program tier for execution. Checks for file ownership conflicts across IMPLs, validates each IMPL doc (with auto-corrections), and creates IMPL branches for all IMPLs in the tier. Counterpart to `finalize-tier`.

```
polywave-tools prepare-tier <program-manifest> --tier <n>
```

**Arguments:**
- `program-manifest` -- path to YAML PROGRAM manifest (required)

**Flags:**
- `--tier` -- tier number to prepare (required)

**Output:** JSON with per-step results and overall `success` (bool).

**Exit codes:** 0 if all steps succeeded, 1 on step failure, 2 on fatal error (manifest/tier not found).

**Example:**
```bash
polywave-tools prepare-tier docs/PROGRAM.yaml --tier 1
polywave-tools prepare-tier program.yaml --tier 2 --repo-dir /path/to/repo
```

---

### finalize-tier

Finalize a program tier by merging all IMPL branches into main in sequence, then running tier-level quality gates. Stops on the first merge failure. With `--auto`, automatically advances to the next tier after the gate passes.

```
polywave-tools finalize-tier <program-manifest> --tier <n> [flags]
```

**Arguments:**
- `program-manifest` -- path to YAML PROGRAM manifest (required)

**Flags:**
- `--tier` -- tier number to finalize (required)
- `--auto` -- automatically advance to next tier after gate passes (default: false)

**Output:** JSON with merge results and tier gate outcome.

**Exit codes:** 0 if all merges succeeded and tier gate passed, 1 on merge or gate failure, 2 on parse error or tier not found.

**Example:**
```bash
polywave-tools finalize-tier docs/PROGRAM.yaml --tier 1
polywave-tools finalize-tier program.yaml --tier 2 --repo-dir /path/to/repo
polywave-tools finalize-tier program.yaml --tier 1 --auto
```

---

## Global Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--repo-dir` | `.` | Repository root directory. Inherited by all subcommands that perform git operations. |

---

Last reviewed: 2026-03-28
