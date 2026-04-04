# cmd/sawtools/ — Command Landscape (~75 commands)

## Command Groups

| Group | Key files | Purpose |
|-------|-----------|---------|
| **Wave Lifecycle** | `prepare_wave`, `finalize_wave`, `run_wave_cmd`, `merge_agents`, `prepare_agent` | Prepare/execute/finalize waves and agent worktrees |
| **Scout & Auto** | `run_scout_cmd`, `auto_cmd`, `suggest_wave_structure_cmd` | Run scout analysis, fully-automated orchestration |
| **Validation** | `validate_cmd`, `validate_briefs_cmd`, `validate_scaffold_cmd`, `pre_wave_validate_cmd` | IMPL doc validation, scaffold checks, pre-wave gates |
| **Gates & Quality** | `pre_wave_gate_cmd`, `run_gates_cmd`, `tier_gate_cmd`, `run_critic_cmd`, `run_review_cmd` | Build gates, quality gates, critic verdicts, code review |
| **Analysis** | `analyze_deps_cmd`, `detect_cascades_cmd`, `check_callers_cmd`, `check_type_collisions_cmd`, `detect_shared_types_cmd` | Dependency graphs, type collisions, cascade detection |
| **State & Status** | `set_impl_state_cmd`, `mark_complete_cmd`, `close_impl_cmd`, `update_status`, `check_completion_cmd`, `list_impls` | IMPL lifecycle state transitions and status queries |
| **Build & Retry** | `verify_build`, `diagnose_build_failure_cmd`, `build_retry_context_cmd`, `retry_cmd` | Build verification, failure diagnosis, closed-loop retries |
| **Programs** | `create_program_cmd`, `program_execute_cmd`, `program_status_cmd`, `program_replan_cmd` | Multi-IMPL program coordination |
| **Observability** | `observability_query`, `observability_metrics`, `journal_init`, `debug_journal` | Event store queries, journal ops, metrics |
| **Setup & Hooks** | `init_cmd`, `install_hooks_cmd`, `verify_install_cmd`, `pre_commit_cmd` | Project init, git hook installation, environment checks |
| **Worktrees** | `create_worktrees`, `cleanup_stale_cmd`, `verify_isolation` | Agent worktree creation, cleanup, isolation checks |

## Adding a New Command

1. Create `new_thing_cmd.go` with `func newNewThingCmd() *cobra.Command`
2. Call the corresponding `engine.*` or `pkg/*` function from the command's `RunE`
3. Register in `main.go` via `root.AddCommand(newNewThingCmd())`

## Key Conventions

- **JSON output** — all commands emit `json.MarshalIndent` to stdout; no human-formatted text in data path
- **`--repo-dir`** — global flag on root command, defaults to cwd, threaded to all engine calls
- **`newSawLogger()`** — structured logger from `logger.go`; use for stderr diagnostics
- **Thin wrappers** — commands parse flags, call one `engine.*` function, marshal the result; no business logic in cmd/
