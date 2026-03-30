# TODO: Fix Isolation Hook - Allow Orchestrator Commits to Main

## Issue

The SAW isolation hook (`validate_agent_isolation`) incorrectly blocks legitimate Orchestrator commits to the main branch. This forces workarounds using `SAW_ALLOW_MAIN_COMMIT=1`.

## Current Behavior

**Hook blocks:**
- Orchestrator commits (baseline fixes, IMPL state advances, post-wave cleanups)
- Any commit to main from a non-worktree context

**Workaround required:**
```bash
SAW_ALLOW_MAIN_COMMIT=1 git commit -m "..."
```

## Root Cause

The hook cannot distinguish between:
1. **Orchestrator commits** (legitimate) - coordination work, state management
2. **Wave agent commits** (violation) - agents must commit to worktree branches

The hook triggers on any commit attempt to main, regardless of context.

## Proposed Fix

**Option 1: Check for worktree context**
```bash
# In validate_agent_isolation hook
if [[ -f ".git/worktrees" ]] || git worktree list | grep -q "$(pwd)"; then
    # We're in a worktree - block main commits
    exit 1
else
    # Not in a worktree - allow (Orchestrator context)
    exit 0
fi
```

**Option 2: Detect SAW agent context**
Check for `.saw-agent-brief.md` existence:
```bash
if [[ -f ".saw-agent-brief.md" ]]; then
    # Agent context - enforce isolation
    if [[ "$BRANCH" =~ ^(main|master)$ ]]; then
        exit 1
    fi
fi
```

**Option 3: Use SAW state markers**
Check `.saw-state/active-impl` to determine if we're in agent execution:
```bash
if [[ -f ".saw-state/active-wave-agent" ]]; then
    # Agent is executing - enforce isolation
    exit 1
fi
```

## Recommendation

Use **Option 2** (check for `.saw-agent-brief.md`). This file only exists in agent worktrees, making it a reliable signal for agent context.

## Priority

Medium - workaround is available, but it's friction in the automation flow.

## Related

- Session: 2026-03-30 Wave 2 execution
- Commits: 807822b, aa197bf, e996d6a, b7cdabd all required `SAW_ALLOW_MAIN_COMMIT=1`
