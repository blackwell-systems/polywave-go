# Hooks Determinism Roadmap — Enforcing Protocol via Claude Code Hooks

**Last reviewed**: 2026-03-24
**Goal**: Move protocol enforcement from prompt instructions to deterministic hooks.

**Completed Items** (removed from roadmap):
- H1: SAW tag validation (impl_path.go)
- H2: IMPL path validation (impl_path.go)
- H3: Stub warning (stub_warning.go)
- H4: Branch drift detection (branch_drift.go)
- H5: PreLaunchGate (prelaunch_gate.go + validate_agent_launch hook)
- H6: Context enrichment (extract-context CLI + prepare-agent CLI)
- H7: Journal auto-capture (pkg/journal/ observer pattern)
- H8: Read dedup cache (pkg/agent/dedup/cache.go)

---

## Remaining — Future Considerations

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
**Note**: Stale worktree detection already exists in `cleanup-stale` CLI command and `prepare-wave` pre-flight; this would surface it earlier.

---

## Design Principles

1. **Hooks enforce, prompts guide.** If a rule can be checked by reading data (IMPL doc, file system, git state), it should be a hook. Prompt instructions are for judgment calls that require LLM reasoning.
2. **Fast hooks only.** Every hook fires on every tool call. Budget: <100ms for quick wins, <2s for enrichment. No network calls, no LLM calls from hooks.
3. **Non-blocking where possible.** Prefer `additionalContext` warnings over exit 2 blocks. Let the agent self-correct. Block only for violations that would cause irreversible damage (branch drift, ownership violation).
4. **Per-agent-type behavior.** Use `agent_type` field to scope enforcement. Wave agents get ownership checks. Scouts get write boundary checks. Don't apply wave rules to scouts.
5. **Defense-in-depth, not replacement.** Hooks add a layer on top of existing enforcement (prepare-wave gates, sawtools validate). They don't replace those — they catch what slips through.
