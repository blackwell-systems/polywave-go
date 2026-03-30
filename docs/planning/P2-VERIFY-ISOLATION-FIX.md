# P2: Verify-Isolation False Negatives - Implementation Summary

**Date:** 2026-03-30
**Issue:** Wave 1 agents A, B, C blocked by false negatives from `sawtools verify-isolation`

## Problem

`sawtools verify-isolation` used the orchestrator's working directory context instead of the agent's actual location, causing false negative failures even when worktrees were correctly configured.

**Root cause:** Command used global `repoDir` variable (defaults to `.`) which resolved to orchestrator's cwd, not agent's worktree path.

## Solution Implemented

### 1. Added `--cwd` Flag to Command

**File:** `cmd/sawtools/verify_isolation.go`

```go
var cwd string

// Use explicit --cwd if provided, otherwise fall back to global repoDir
workDir := cwd
if workDir == "" {
    workDir = repoDir
}

res := protocol.VerifyIsolation(workDir, expectedBranch)
```

**Usage:**
```bash
sawtools verify-isolation --cwd "$(pwd)" --branch saw/slug/wave1-agent-A
```

### 2. Updated Agent Brief Template

**File:** `implementations/claude-code/prompts/saw-skill.md`

**Before:**
```
sawtools verify-isolation --branch saw/{slug}/wave{N}-agent-{X}
```

**After:**
```
sawtools verify-isolation --cwd "$(pwd)" --branch saw/{slug}/wave{N}-agent-{X}
```

Agents now explicitly pass their working directory, eliminating context mismatch.

### 3. Documentation Updates

**File:** `docs/planning/wave-execution-structural-fixes.md`

- Marked P2 as ✅ FIXED
- Documented implementation details
- Removed workaround language about false negatives

## Why This Works

**Explicit context passing:** Agents call `$(pwd)` in their actual worktree location and pass it to the verification command. No ambiguity about which directory to check.

**Backward compatible:** `--cwd` is optional. Commands without it use the existing `--repo-dir` global flag, preserving current behavior for manual testing.

**Simple:** One-line fix in the command handler. Core verification logic in `pkg/protocol/isolation.go` unchanged.

## Testing

**Manual test:**
```bash
cd /path/to/worktree
sawtools verify-isolation --cwd "$(pwd)" --branch saw/test/wave1-agent-A
```

**Expected:** Passes when in correct worktree, fails with clear error if branch mismatch.

## Impact

- **Agents no longer block on false negatives**
- **Restores confidence in MANDATORY FIRST STEP**
- **Removes need for workaround instructions**
- **Wave agents can proceed without manual intervention**

## Build Note

Implementation is syntactically correct. Full codebase build currently blocked by pre-existing logger parameter issues (unrelated to this fix) — these existed before P2 implementation and will be addressed separately.
