# Wave Execution Structural Fixes

**Date:** 2026-03-30
**Context:** Post-mortem from logging-injection-remaining Wave 1 execution failures

## Issues Identified

### 1. Wrong Agent Naming Scheme (Observability Violation) ✅ FIXED
**What happened:** Agents launched with `Wave1-Agent-A` instead of `[SAW:wave1:agent-A]`
**Impact:** Monitoring tools (claudewatch) cannot detect SAW agent runs
**Root cause:** Orchestrator manually constructed agent names instead of using protocol-defined format

**Structural fix implemented:**
- ✅ `sawtools prepare-agent` writes `saw_name` field to brief frontmatter (commit 71e820b)
- ✅ Orchestrator reads `saw_name` from brief metadata per E44 section (commit abf8c5c)
- ✅ `auto_format_saw_agent_names` PreToolUse hook validates and provides fallback (commit 06b5527)
- ✅ Three-layer architecture: metadata (primary) → orchestrator (secondary) → hook (fallback)
- ✅ Documentation updated in saw-skill.md, wave-agent-contracts.md, hooks/README.md

### 2. Agent Branch Isolation Violation (I1) ✅ FIXED
**What happened:** Agent B committed to Agent D's branch (`wave1-agent-D` instead of `wave1-agent-B`)
**Impact:** I1 (disjoint ownership) violated; merge confusion; branch doesn't exist
**Root cause:** Agent tool doesn't set working directory context for worktrees; agents ran in wrong locations

**Structural fix implemented:**
- ✅ **Option C selected:** E42 branch verification added to `validate_agent_completion` hook (commit 1124c8a)
- ✅ Hook verifies agent committed to expected `saw/{slug}/wave{N}-agent-{ID}` branch
- ✅ Blocks completion if branch mismatch detected
- ✅ Prevents future I1 violations like Agent B→D confusion
- **Note:** Option A (Agent tool cwd parameter) would be ideal long-term enhancement but requires external changes

### 3. Isolation Verification False Negatives ✅ FIXED
**What happened:** `sawtools verify-isolation` failed for all agents despite correct worktree setup
**Impact:** Agents A, B, C blocked; manual recovery required
**Root cause:** Verification tool checks orchestrator's cwd instead of agent's actual location

**Structural fix implemented:**
- ✅ Added `--cwd` flag to `sawtools verify-isolation` command (agents explicitly pass working directory)
- ✅ Created `validate_agent_isolation` SubagentStart hook for automatic enforcement
- ✅ Hook blocks agent from starting with exit 2 if isolation check fails (E12 enforcement)
- ✅ Removed manual verification step from agent briefs (now automatic via hook)
- ✅ Documentation updated in saw-skill.md, hooks/README.md, install.sh
- ✅ Three-layer enforcement: command accepts `--cwd`, hook validates at start, exit 2 blocks execution

### 4. E35 Violations (Unowned Call Sites) ✅ FIXED
**What happened:** Agent C defined `CreateProgramWorktrees(logger)` but didn't own 2 call sites in same package
**Impact:** Post-merge build failure; manual fixup required
**Root cause:** Scout assigned function ownership without checking same-package callers

**Structural fix implemented:**
- ✅ **Option C selected:** `sawtools pre-wave-validate` command with integrated E35 detection
- ✅ Go AST-based analysis detects same-package function definitions and call sites
- ✅ Runs after E16 validation, before E37 critic gate
- ✅ Reports gaps with file:line references in JSON output
- ✅ Blocks wave execution if gaps detected (exit 1)
- ✅ Three remediation strategies documented: reassign ownership, create wiring entry, defer to integration
- ✅ Implementation: `pkg/protocol/e35_detection.go` + `cmd/sawtools/pre_wave_validate_cmd.go`
- ✅ Documentation updated in saw-skill.md and pre-wave-validation.md

### 5. Redundant Helper Definitions (Pre-mortem Predicted)
**What happened:** 3 agents independently defined `loggerFrom` helper in same package
**Impact:** Build failure "redeclared in this block"; manual consolidation required
**Root cause:** Agents don't share code; no coordination on shared helpers

**Structural fix (low priority - pre-mortem predicted this):**
- Scaffold agent could pre-commit shared helpers if multiple agents need them
- Scout could detect pattern: "3+ agents in pkg/protocol use loggerFrom" → scaffold it
- Not blocking - this is expected behavior for parallel waves

## Priority Order

1. ~~**P0 (Blocking):** Agent tool cwd parameter OR E42 branch mismatch detection~~ ✅ FIXED
   - ~~Without this, agents will continue committing to wrong branches~~
2. ~~**P0 (Blocking):** Agent naming helper function~~ ✅ FIXED
   - ~~Simple fix, high observability impact~~
3. ~~**P1 (High):** Scout same-package caller detection for E35~~ ✅ FIXED
   - ~~Prevents post-merge build failures~~
4. ~~**P2 (Medium):** Fix sawtools verify-isolation bug~~ ✅ FIXED
   - ~~Currently worked around via brief wording~~
5. **P3 (Low):** Shared helper scaffolding
   - Pre-mortem already catches this; manual cleanup is acceptable

## Implementation Plan

### Phase 1: Immediate (This Session) ✅ COMPLETE
- [x] Document issues (this file)
- [x] Update orchestrator to use proper SAW tag format for agent names (E44)
- [x] Implement E42 branch mismatch detection in SubagentStop hook
- [x] Add `sawtools pre-wave-validate` with E35 detection

### Phase 2: Future (Low Priority)
- [ ] Add cwd parameter to Agent tool (request from Claude Code team) — nice-to-have, E42 hook covers this
- [ ] Fix `sawtools verify-isolation` to detect agent cwd correctly — workaround acceptable

## Lessons for Protocol

**What worked:**
- E7 (all agents complete) enforcement caught issues before merge
- Pre-mortem correctly predicted loggerFrom duplication
- Manual recovery was possible because files were disjoint (I1 partially held)
- Hook-based enforcement (E42, E44) prevents protocol violations automatically

**What was fixed:**
- ~~Agent tool doesn't enforce worktree isolation (I1 violation possible)~~ → E42 hook validates branch
- ~~Scout doesn't detect same-package E35 gaps~~ → pre-wave-validate detects at planning time
- ~~Orchestrator manually constructed agent names~~ → metadata-driven E44 compliance
- Verification tool has false negatives (agents proceeded anyway - correct behavior, still acceptable)

**Protocol changes implemented:**
- ✅ E44: "Orchestrator must use SAW tag format for agent names" (metadata + hook)
- ✅ E35: "pre-wave-validate must detect same-package callers for ownership analysis" (AST-based)
- ✅ I1: "SubagentStop hook must verify agent committed to correct branch" (E42 enforcement)
