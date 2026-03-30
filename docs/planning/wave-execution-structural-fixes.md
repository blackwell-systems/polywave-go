# Wave Execution Structural Fixes

**Date:** 2026-03-30
**Context:** Post-mortem from logging-injection-remaining Wave 1 execution failures

## Issues Identified

### 1. Wrong Agent Naming Scheme (Observability Violation)
**What happened:** Agents launched with `Wave1-Agent-A` instead of `[SAW:wave1:agent-A]`
**Impact:** Monitoring tools (claudewatch) cannot detect SAW agent runs
**Root cause:** Orchestrator manually constructed agent names instead of using protocol-defined format

**Structural fix required:**
- Add `agentName(wave, id, slug)` helper function to orchestrator that enforces SAW tag format
- Update Agent tool invocation to use helper: `name: agentName(1, "A", "logging-injection")`
- Add validation in `sawtools prepare-wave` that checks agent name format in briefs
- **Blocker until:** Helper function implemented in orchestrator skill

### 2. Agent Branch Isolation Violation (I1)
**What happened:** Agent B committed to Agent D's branch (`wave1-agent-D` instead of `wave1-agent-B`)
**Impact:** I1 (disjoint ownership) violated; merge confusion; branch doesn't exist
**Root cause:** Agent tool doesn't set working directory context for worktrees; agents ran in wrong locations

**Structural fix required:**
- **Option A (preferred):** Agent tool enhancement to support explicit working directory parameter
  - Add `cwd: "/path/to/worktree"` parameter to Agent tool
  - Update orchestrator to pass worktree path from prepare-wave output
- **Option B (workaround):** Pre-flight cd command in agent prompt
  - Already tried this — agents ignored it or couldn't execute
- **Option C (E42 hook enhancement):** SubagentStop hook detects branch mismatch
  - Check: agent brief says branch X, actual git branch is Y → block completion
  - Add to `validate_agent_completion` hook
- **Blocker until:** Agent tool cwd parameter implemented OR E42 enhancement deployed

### 3. Isolation Verification False Negatives
**What happened:** `sawtools verify-isolation` failed for all agents despite correct worktree setup
**Impact:** Agents A, B, C blocked; manual recovery required
**Root cause:** Verification tool checks orchestrator's cwd instead of agent's actual location

**Structural fix required:**
- Fix `sawtools verify-isolation` to detect actual agent working directory
  - Read cwd from agent environment, not from caller
  - Verify worktree registration via `git worktree list` lookup
- Remove "MANDATORY FIRST STEP - If fails, STOP" from agent briefs
  - Replace with: "Verify isolation. If check fails but `git branch --show-current` shows correct branch, proceed (known false negative)"
- **Blocker until:** sawtools verify-isolation bug fixed

### 4. E35 Violations (Unowned Call Sites)
**What happened:** Agent C defined `CreateProgramWorktrees(logger)` but didn't own 2 call sites in same package
**Impact:** Post-merge build failure; manual fixup required
**Root cause:** Scout assigned function ownership without checking same-package callers

**Structural fix required:**
- **Scout enhancement:** Same-package caller detection
  - When agent owns function F in file X, scan package for calls to F
  - If calls exist in files not owned by agent, flag as E35 violation
  - Add to suitability assessment: "Agent C owns CreateProgramWorktrees but not its 2 callers"
- **Option A:** Auto-extend ownership to include callers (risky - may bloat scope)
- **Option B:** Require Scout to explicitly document E35 gaps in pre_mortem section
- **Option C:** Add `sawtools detect-e35-gaps` post-scout validation
- **Blocker until:** Scout same-package analysis implemented OR detect-e35-gaps command added

### 5. Redundant Helper Definitions (Pre-mortem Predicted)
**What happened:** 3 agents independently defined `loggerFrom` helper in same package
**Impact:** Build failure "redeclared in this block"; manual consolidation required
**Root cause:** Agents don't share code; no coordination on shared helpers

**Structural fix (low priority - pre-mortem predicted this):**
- Scaffold agent could pre-commit shared helpers if multiple agents need them
- Scout could detect pattern: "3+ agents in pkg/protocol use loggerFrom" → scaffold it
- Not blocking - this is expected behavior for parallel waves

## Priority Order

1. **P0 (Blocking):** Agent tool cwd parameter OR E42 branch mismatch detection
   - Without this, agents will continue committing to wrong branches
2. **P0 (Blocking):** Agent naming helper function
   - Simple fix, high observability impact
3. **P1 (High):** Scout same-package caller detection for E35
   - Prevents post-merge build failures
4. **P2 (Medium):** Fix sawtools verify-isolation bug
   - Currently worked around via brief wording
5. **P3 (Low):** Shared helper scaffolding
   - Pre-mortem already catches this; manual cleanup is acceptable

## Implementation Plan

### Phase 1: Immediate (This Session)
- [x] Document issues (this file)
- [ ] Update orchestrator to use proper SAW tag format for agent names
- [ ] Update agent briefs to soften isolation verification failure handling

### Phase 2: Next Session (Tooling)
- [ ] Add cwd parameter to Agent tool (request from Claude Code team)
- [ ] Fix `sawtools verify-isolation` to detect agent cwd correctly
- [ ] Implement E42 branch mismatch detection in SubagentStop hook

### Phase 3: Scout Enhancement (Week)
- [ ] Add same-package caller detection to Scout analysis
- [ ] Update suitability gate to flag E35 gaps
- [ ] Add `sawtools detect-e35-gaps` validation command

## Lessons for Protocol

**What worked:**
- E7 (all agents complete) enforcement caught issues before merge
- Pre-mortem correctly predicted loggerFrom duplication
- Manual recovery was possible because files were disjoint (I1 partially held)

**What failed:**
- Agent tool doesn't enforce worktree isolation (I1 violation possible)
- Scout doesn't detect same-package E35 gaps
- Verification tool has false negatives (agents proceeded anyway - correct behavior)

**Protocol changes needed:**
- Add E44: "Orchestrator must use SAW tag format for agent names"
- Update E35: "Scout must detect same-package callers for ownership analysis"
- Update I1: "SubagentStop hook must verify agent committed to correct branch"
