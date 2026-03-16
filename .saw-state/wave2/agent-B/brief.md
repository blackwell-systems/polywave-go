# Agent B Brief - Wave 2

**IMPL Doc:** /Users/dayna.blackwell/code/scout-and-wave/docs/IMPL/IMPL-prepare-wave-workflow-fixes.yaml

## Files Owned

- `pkg/protocol/worktree.go`
- `pkg/protocol/worktree_test.go`


## Task

## What to Implement

Fix Bug #2 and #3: Auto-install hooks during worktree creation by calling `git.InstallHooks()`
immediately after each `git.WorktreeAdd()` call in `pkg/protocol/worktree.go`.

**Context:** `prepare_wave.go` lines 85-93 verify hooks after worktree creation, but worktrees
don't inherit hooks from the main repo. The verification correctly finds the hook location
(`.git/worktrees/wave{N}-agent-{ID}/hooks/`), but nothing puts the hook there. This change
makes worktree creation automatically install hooks, eliminating the chicken-and-egg problem.

**Root cause:** `git worktree add` creates an empty `hooks/` directory but doesn't copy hooks
from the main repo. The H10 pre-commit hook exists in `.git/hooks/pre-commit` but must be
explicitly copied to each worktree.

## Interfaces to Implement

Modify `pkg/protocol/worktree.go` in the `CreateWorktrees()` function around line 123:

```go
// Before:
if err := git.WorktreeAdd(agentRepoDir, worktreePath, branchName); err != nil {
    return nil, fmt.Errorf("failed to create worktree for agent %s: %w", agent.ID, err)
}

// After:
if err := git.WorktreeAdd(agentRepoDir, worktreePath, branchName); err != nil {
    return nil, fmt.Errorf("failed to create worktree for agent %s: %w", agent.ID, err)
}

// Install pre-commit hook (H10 isolation enforcement)
if err := git.InstallHooks(agentRepoDir, worktreePath); err != nil {
    // Log warning but don't fail — hook verification in prepare-wave will catch this
    // and provide actionable error message
    fmt.Fprintf(os.Stderr, "Warning: failed to install hooks for agent %s: %v\n", agent.ID, err)
}
```

Add import for `os` package at the top of the file (needed for `os.Stderr`).

## Interfaces to Call

From Agent A:
- `git.InstallHooks(repoPath, worktreePath string) error`

Existing functions:
- `git.WorktreeAdd()` (already called)
- `fmt.Fprintf()` for warning messages

## Tests to Write

Add to `pkg/protocol/worktree_test.go`:

1. `TestCreateWorktrees_InstallsHooks` — verifies hooks are installed:
   - Create temp repo with IMPL manifest (1 wave, 1 agent)
   - Create main repo hook at `.git/hooks/pre-commit` (test content)
   - Call `CreateWorktrees()`
   - Verify hook exists at `.git/worktrees/wave1-agent-A/hooks/pre-commit`
   - Verify hook content matches main repo hook
   - Verify hook is executable

2. `TestCreateWorktrees_ContinuesOnHookInstallFailure` — missing source hook:
   - Create temp repo with no hook in `.git/hooks/`
   - Call `CreateWorktrees()`
   - Verify worktree is created despite hook install failure
   - Verify warning is printed (capture stderr)

Update existing `TestCreateWorktrees_*` tests if they check for hook absence:
- Update assertions to expect hooks to be present

## Verification Gate

```bash
go build -o /tmp/sawtools ./cmd/saw
go vet ./pkg/protocol/...
go test ./pkg/protocol/... -v -run TestCreateWorktrees
```

## Constraints

- Do NOT fail worktree creation if hook install fails — hook verification in `prepare-wave`
  will catch it and provide clear error message
- Print warning to stderr (not stdout) — stdout is reserved for JSON result
- Do NOT modify `cmd/saw/prepare_wave.go` hook verification logic — it's already correct
- Do NOT modify `cmd/saw/verify_hook_installed.go` — verification logic is correct
- Preserve all existing `CreateWorktrees()` behavior (base commit recording, cross-repo support)
- Hook installation is best-effort — orchestrator will catch missing hooks before agent launch



## Interface Contracts

### InstallHooks

Copies the SAW pre-commit hook from the main repository to a worktree's hooks directory.
Creates hooks directory if it doesn't exist. Makes hook executable (chmod +x).


```
func InstallHooks(repoPath, worktreePath string) error

```



## Quality Gates

Level: standard

- **build**: `go build -o /tmp/sawtools ./cmd/saw` (required: true)
- **test**: `go test ./pkg/protocol/... ./internal/git/...` (required: true)
- **lint**: `go vet ./...` (required: false)
  Check for common Go mistakes

