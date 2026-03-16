# Agent A Brief - Wave 1

**IMPL Doc:** /Users/dayna.blackwell/code/scout-and-wave/docs/IMPL/IMPL-prepare-wave-workflow-fixes.yaml

## Files Owned

- `internal/git/commands.go`
- `internal/git/commands_test.go`
- `pkg/protocol/cleanup_test.go`


## Task

## What to Implement

Fix TWO bugs in `internal/git/commands.go`:

**Bug #3 (No Hook Installation):** Add `git.InstallHooks()` function that copies the SAW pre-commit hook
from the main repository's `.git/hooks/pre-commit` to a worktree's hooks directory.

**Bug #1 (Incomplete Cleanup):** Change `git branch -d` to `git branch -D` in `DeleteBranch()` to
force-delete agent branches after merge.

### Part 1: Add InstallHooks() Function

**Context:** When `git worktree add` creates a worktree, it creates an empty `hooks/` directory in
`.git/worktrees/<name>/hooks/`. The H10 pre-commit hook exists in the main repo's `.git/hooks/pre-commit`,
but worktrees don't inherit it automatically. This function bridges that gap.

Add to `internal/git/commands.go`:

```go
// InstallHooks copies the SAW pre-commit hook from the main repository to a worktree.
// It reads the hook from repoPath/.git/hooks/pre-commit and writes it to the worktree's
// hooks directory, making it executable. Creates the hooks directory if it doesn't exist.
//
// For worktrees, the hook path is: .git/worktrees/<name>/hooks/pre-commit
// For regular repos, the hook path is: .git/hooks/pre-commit
//
// Returns an error if:
// - The source hook doesn't exist in the main repo
// - The worktree path is invalid or doesn't exist
// - File I/O operations fail
func InstallHooks(repoPath, worktreePath string) error
```

**Implementation requirements:**
1. Read hook content from `{repoPath}/.git/hooks/pre-commit`
2. Determine target hook path:
   - Read `{worktreePath}/.git` file to parse `gitdir:` pointer
   - Extract worktree git directory (e.g., `/path/to/repo/.git/worktrees/wave1-agent-A`)
   - Target path is `{worktree_git_dir}/hooks/pre-commit`
3. Create `hooks/` directory if it doesn't exist (`os.MkdirAll`, mode 0755)
4. Write hook content to target path (mode 0755 for executable)
5. Return descriptive errors for each failure case

**Edge cases:**
- Source hook missing: return error "pre-commit hook not found in main repo"
- Worktree `.git` file malformed: return error with file content
- Hooks directory creation fails: return wrapped error

### Part 2: Fix DeleteBranch() Force Delete

**Context:** The current implementation at line 114 uses `git branch -d` (safe delete), which
refuses to delete branches that haven't been merged. This causes `prepare-wave` to fail on
subsequent runs with "fatal: a branch named 'wave1-agent-A' already exists" because cleanup
successfully removed the worktree but left the orphaned branch.

**Root cause:** After a wave completes, branches are merged to main but may not be fast-forward
merged. Git's `-d` flag refuses to delete non-fast-forward branches even after merge. The `-D`
flag (force delete) is safe here because cleanup only runs AFTER the orchestrator has verified
and merged the wave's work.

Modify `internal/git/commands.go` line 114:

```go
// Before:
_, err := Run(repoPath, "branch", "-d", branch)

// After:
_, err := Run(repoPath, "branch", "-D", branch)
```

Update the function comment:

```go
// DeleteBranch deletes the named branch from the repository at repoPath.
// Uses -D (force delete) because this is only called during cleanup after
// successful merge, where the branch may not be fast-forward mergeable but
// is known to be safe to delete.
func DeleteBranch(repoPath, branch string) error
```

## Interfaces to Call

Use standard library:
- `os.ReadFile()` to read source hook
- `os.WriteFile()` to write target hook with mode 0755
- `os.MkdirAll()` to create hooks directory
- `filepath.Join()` for path construction

## Tests to Write

Add to `internal/git/commands_test.go`:

**InstallHooks tests:**
1. `TestInstallHooks_Success` — creates temp repo + worktree, installs hook, verifies:
   - Hook file exists at correct path
   - Hook content matches source
   - Hook is executable (stat.Mode() & 0111 != 0)

2. `TestInstallHooks_MissingSourceHook` — temp repo with no hook in `.git/hooks/`:
   - Returns error containing "pre-commit hook not found"

3. `TestInstallHooks_InvalidWorktreePath` — nonexistent worktree path:
   - Returns error (wrapped os.IsNotExist error)

4. `TestInstallHooks_CreatesHooksDirectory` — worktree without `hooks/` dir:
   - Creates directory and installs hook successfully

Add to `pkg/protocol/cleanup_test.go`:

**DeleteBranch tests:**
5. `TestCleanup_ForcesDeleteUnmergedBranches` — simulates post-merge state:
   - Create temp repo with main branch
   - Create worktree branch `wave1-agent-A` with commits
   - Merge branch to main with `--no-ff` (non-fast-forward)
   - Run cleanup
   - Verify branch is deleted despite non-fast-forward merge

6. `TestCleanup_IdempotentBranchDeletion` — branch already deleted:
   - Create temp repo, create and delete branch manually
   - Run cleanup
   - Verify BranchDeleted = true (idempotent success)

## Verification Gate

```bash
go build -o /tmp/sawtools ./cmd/saw
go vet ./internal/git/... ./pkg/protocol/...
go test ./internal/git/... -v -run TestInstallHooks
go test ./pkg/protocol/... -v -run TestCleanup
```

## Constraints

- Do NOT modify `cmd/saw/verify_hook_installed.go` — hook verification logic is correct
- Do NOT add a CLI command — InstallHooks is library code called by prepare-wave
- Do NOT change `pkg/protocol/cleanup.go` logic — only modify `internal/git/commands.go`
- Use `filepath.Join()` for all path construction (cross-platform compatibility)
- Hook file must be executable (mode 0755) — non-executable hooks are silently ignored by git
- Force delete (`-D`) is safe because cleanup only runs AFTER orchestrator merge
- Preserve idempotent behavior: treat "branch not found" errors as success
- Error messages must be actionable (include paths, suggest fixes)



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

