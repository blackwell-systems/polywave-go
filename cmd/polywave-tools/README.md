# cmd/polywave-tools/ — Command Landscape (~75 commands)

## Command Groups

| Group | Key files | Purpose |
|-------|-----------|---------|
| **Wave Lifecycle** | `prepare_wave`, `finalize_wave`, `run_wave_cmd`, `merge_agents`, `prepare_agent` | Prepare/execute/finalize waves and agent worktrees |
| **Scout & Auto** | `run_scout_cmd`, `auto_cmd`, `suggest_wave_structure_cmd` | Run scout analysis, fully-automated orchestration |
| **Validation** | `validate_cmd`, `validate_briefs_cmd`, `validate_scaffold_cmd`, `pre_wave_validate_cmd`, `finalize_scout_cmd` | IMPL doc validation, scaffold checks, pre-wave gates, Scout finalization |
| **Gates & Quality** | `pre_wave_gate_cmd`, `run_gates_cmd`, `tier_gate_cmd`, `run_critic_cmd`, `run_review_cmd` | Build gates, quality gates, critic verdicts, code review |
| **Analysis** | `analyze_deps_cmd`, `detect_cascades_cmd`, `check_callers_cmd`, `check_type_collisions_cmd`, `detect_shared_types_cmd` | Dependency graphs, type collisions, cascade detection |
| **State & Status** | `set_impl_state_cmd`, `mark_complete_cmd`, `close_impl_cmd`, `update_status`, `check_completion_cmd`, `list_impls` | IMPL lifecycle state transitions and status queries |
| **Build & Retry** | `verify_build`, `diagnose_build_failure_cmd`, `build_retry_context_cmd`, `retry_cmd` | Build verification, failure diagnosis, closed-loop retries |
| **Programs** | `create_program_cmd`, `program_execute_cmd`, `program_status_cmd`, `program_replan_cmd` | Multi-IMPL program coordination |
| **Observability** | `observability_query`, `observability_metrics`, `journal_init`, `debug_journal` | Event store queries, journal ops, metrics |
| **Setup & Hooks** | `init_cmd`, `install_hooks_cmd`, `verify_install_cmd`, `pre_commit_cmd` | Project init, git hook installation, environment checks |
| **Worktrees** | `create_worktrees`, `cleanup_stale_cmd`, `verify_isolation` | Agent worktree creation, cleanup, isolation checks |

## Batching Commands

Several commands consolidate multi-step workflows into single invocations. These replaced
manual sequences of 5-11 individual commands that were error-prone when run by agents.

| Batching Command | Replaces | Steps |
|-----------------|----------|-------|
| `prepare-wave` | `create-worktrees` → `prepare-agent` (per agent) → baseline gates → hook install | Resume detection, I1 check, baseline gates, cross-repo baseline, worktree creation, workspace setup, brief extraction (~12 steps) |
| `finalize-wave` | `verify-commits` → `scan-stubs` → `run-gates` → `merge-agents` → `verify-build` → `cleanup` | 6 steps + type collision check + integration validation + workspace restore |
| `finalize-scout` | `validate --fix` → `pre-wave-validate` → `validate-briefs` → `set-injection-method` | 4 checks in sequence with structured JSON output per step |
| `pre-wave-validate` | `validate --fix` → E35 detection → test cascade check → wave structure check | 4 validation passes with combined result |
| `close-impl` | `set-impl-state COMPLETE` → archive to `complete/` → `update-context` → git commit | Atomic: state + archive + CONTEXT.md + commit in one |

The primitive commands still exist for debugging and programmatic use — batching commands
call them internally via `pkg/engine` and `pkg/protocol` functions.

## Adding a New Command

1. Create `new_thing_cmd.go` with `func newNewThingCmd() *cobra.Command`
2. Call the corresponding `engine.*` or `pkg/*` function from the command's `RunE`
3. Register in `main.go` via `root.AddCommand(newNewThingCmd())`

## Key Conventions

- **JSON output** — all commands emit `json.MarshalIndent` to stdout; no human-formatted text in data path
- **`--repo-dir`** — global flag on root command, defaults to cwd, threaded to all engine calls
- **`newPolywaveLogger()`** — structured logger from `logger.go`; use for stderr diagnostics
- **Thin wrappers** — commands parse flags, call one `engine.*` function, marshal the result; no business logic in cmd/
