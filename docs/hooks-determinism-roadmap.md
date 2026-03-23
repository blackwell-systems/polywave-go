# Hooks Determinism Roadmap — Enforcing Protocol via Claude Code Hooks

**Date**: 2026-03-22
**Last Cleaned**: 2026-03-23 (removed H1-H4 completed items)
**Goal**: Move protocol enforcement from prompt instructions to deterministic hooks. Every rule the orchestrator can forget, a hook should enforce.

**Completed Items** (removed from roadmap):
- H1: SAW tag validation (impl_path.go)
- H2: IMPL path validation (impl_path.go)
- H3: Stub warning (stub_warning.go)
- H4: Branch drift detection (branch_drift.go)

---

## Medium Effort, High Impact

### H5: PreToolUse on Agent — full launch validation gate
**Effort**: ~150 lines, 2 days
**Hook**: PreToolUse (matcher: Agent)
**What**: Comprehensive pre-launch validator that reads the IMPL doc and validates everything before the agent starts:
- IMPL doc exists and parses
- Wave number matches current wave state (not re-launching completed wave)
- Agent ID exists in the IMPL doc for that wave
- Worktree path exists and is on correct branch (for multi-agent waves)
- Scaffolds are committed (I2 defense-in-depth)
- File ownership for this agent has no conflicts with running agents
- Critic report exists if E37 threshold met (defense-in-depth behind prepare-wave)
**Enforcement**: Exit 2 on any failure with specific diagnostic message.
**Impact**: Makes it impossible to launch a malformed agent. Catches stale state between prepare-wave and launch.
**Depends on**: H2 (subsumes the basic IMPL path check)

### H6: SubagentStart — context enrichment engine
**Effort**: ~200 lines, 3 days
**Hook**: SubagentStart
**What**: Enriches every SAW agent's context at launch time with fresh data from the IMPL doc:
- For wave agents: latest interface contracts, file ownership table, completion reports from same-wave agents that finished first
- For Scout: codebase metrics (file count, language breakdown, recent commits)
- For integration agents: structured integration gap report
- For all: current wave status, which agents completed/running/failed
**Mechanism**: Returns `additionalContext` with structured data read directly from IMPL doc at launch time.
**Impact**: Eliminates stale briefs. The orchestrator's prompt becomes minimal (IMPL path + wave + agent ID), the hook injects everything else from source of truth. Orchestrator can't mess up agent context because the hook always provides correct data.
**Design note**: This hook calls `sawtools extract-context` or reads the IMPL doc directly. Must be fast (<2s) to not block agent launch.

### H7: PostToolUse on all tools — agent journal auto-capture
**Effort**: ~250 lines, 3 days
**Hook**: PostToolUse (universal, filtered by agent_type)
**What**: Automatically captures every tool call for SAW agents to `.saw-state/journals/wave{N}/agent-{ID}/tool-log.jsonl`:
- Tool name, input summary (file path, command), output summary (success/fail, line count)
- Timestamp, estimated token count
- On agent completion: auto-generates structured summary (files read, files written, commands run, tests passed/failed)
**Mechanism**: Appends to JSONL file on each PostToolUse. The summary feeds into completion reports.
**Impact**: Eliminates "agent didn't write a completion report" failures. The journal IS the report data — agent just adds status assessment. Also provides data for the file read dedup cache (know which files were already read).
**Performance note**: Must be fast — file append only, no parsing. Summary generation happens once at session end.

---

## Future Considerations

### H8: PreToolUse on Read — dedup hint (soft)
**Effort**: ~40 lines, 1 day
**Hook**: PreToolUse (matcher: Read)
**What**: Check `.saw-state/dedup-cache.json` for the file being read. If the file was read by this agent within the last N minutes and the content hash matches, inject `additionalContext: "Note: this file was read {N}min ago and is unchanged ({lines} lines). Consider whether you need to re-read it."` Does NOT block the read — soft hint only.
**Limitation**: Cannot modify Read output for non-MCP tools. The full content still enters context. But the hint may cause the LLM to process it more efficiently.
**Alternative**: MCP-based `saw_read` tool with PostToolUse `updatedMCPToolOutput` for actual dedup (requires MCP server setup).

### H9: PermissionRequest — auto-allow SAW operations
**Effort**: ~50 lines, 1 day
**Hook**: PermissionRequest
**What**: Auto-allow specific tool patterns for SAW agents:
- Wave agents: Write/Edit only to owned files (already covered by H4/check_wave_ownership)
- Scout: Write only to `docs/IMPL/IMPL-*.yaml`
- Integration agents: Write/Edit only to files in their `files` list
**Reduces**: Permission prompts during autonomous execution (--auto mode).
**Risk**: Must be carefully scoped to avoid over-permissioning.

### H10: SessionStart — SAW environment validation
**Effort**: ~30 lines, 0.5 day
**Hook**: SessionStart
**What**: On session start, verify SAW infrastructure:
- sawtools binary exists and is current version
- `.claude/skills/saw/SKILL.md` symlink resolves
- Git version >= 2.20
- No stale worktrees from interrupted sessions (warn if found)
**Mechanism**: Returns `additionalContext` with environment status.
**Impact**: Catches setup issues before the user runs `/saw` and hits pre-flight failures.

---

## Implementation Priority

### Phase 1: Quick enforcement (1-2 days)
- **H1**: SAW tag validation
- **H2**: IMPL path validation
- **H3**: Stub warning at write-time
- **H4**: Branch drift detection

### Phase 2: Smart launch gate (2-3 days)
- **H5**: Full launch validation (subsumes H2)
- **H10**: Session environment check

### Phase 3: Context intelligence (3-5 days)
- **H6**: Context enrichment engine
- **H7**: Auto-journal capture

### Phase 4: Optimization (2 days)
- **H8**: Read dedup hint
- **H9**: Auto-allow SAW operations

---

## Existing Hooks (for reference)

| Hook | File | What |
|------|------|------|
| `check_scout_boundaries` | PreToolUse (Write\|Edit) | I6: Scouts can only write IMPL docs |
| `check_wave_ownership` | PreToolUse (Write\|Edit\|NotebookEdit) | I1: Wave agents can only write owned files |
| `block_claire_paths` | PreToolUse (Write\|Edit\|Bash) | Blocks `.claire` typo paths |
| `validate_impl_on_write` | PostToolUse (Write) | E16: Validates IMPL docs on every write |
| `claudewatch hook` | PostToolUse (all) | Observability: error loops, drift, cost |

---

## Design Principles

1. **Hooks enforce, prompts guide.** If a rule can be checked by reading data (IMPL doc, file system, git state), it should be a hook. Prompt instructions are for judgment calls that require LLM reasoning.
2. **Fast hooks only.** Every hook fires on every tool call. Budget: <100ms for quick wins, <2s for enrichment. No network calls, no LLM calls from hooks.
3. **Non-blocking where possible.** Prefer `additionalContext` warnings over exit 2 blocks. Let the agent self-correct. Block only for violations that would cause irreversible damage (branch drift, ownership violation).
4. **Per-agent-type behavior.** Use `agent_type` field to scope enforcement. Wave agents get ownership checks. Scouts get write boundary checks. Don't apply wave rules to scouts.
5. **Defense-in-depth, not replacement.** Hooks add a layer on top of existing enforcement (prepare-wave gates, sawtools validate). They don't replace those — they catch what slips through.
