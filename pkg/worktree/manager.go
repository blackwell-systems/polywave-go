// Package worktree manages git worktrees for SAW wave agents. Each agent
// in a wave receives an isolated worktree branched from HEAD so that
// parallel file edits cannot conflict during execution. The Manager
// handles creation, path resolution, and cleanup of worktrees.
package worktree

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/blackwell-systems/scout-and-wave-go/internal/git"
)

// Manager tracks and manages git worktrees for SAW wave agents.
type Manager struct {
	repoPath string
	active   map[string]string // absolute worktree path -> branch name
}

// New creates a new Manager for the git repository at repoPath.
func New(repoPath string) *Manager {
	return &Manager{
		repoPath: repoPath,
		active:   make(map[string]string),
	}
}

// Create creates a new worktree for the given wave number and agent letter.
// The worktree path follows the convention:
//
//	{repoPath}/.claude/worktrees/wave{wave}-agent-{agent}
//
// A new branch with the same name is created from HEAD.
// Returns the absolute path to the created worktree.
func (m *Manager) Create(wave int, agent string) (string, error) {
	branch := fmt.Sprintf("wave%d-agent-%s", wave, agent)
	wtDir := filepath.Join(m.repoPath, ".claude", "worktrees")

	if err := os.MkdirAll(wtDir, 0o755); err != nil {
		return "", fmt.Errorf("failed to create worktree base directory %q: %w", wtDir, err)
	}

	wtPath := filepath.Join(wtDir, branch)

	if err := git.WorktreeAdd(m.repoPath, wtPath, branch); err != nil {
		return "", fmt.Errorf("manager: create worktree for wave %d agent %s: %w", wave, agent, err)
	}

	if err := installPreCommitHook(wtPath); err != nil {
		// Non-fatal: log but do not abort worktree creation.
		fmt.Fprintf(os.Stderr, "manager: warning: could not install pre-commit hook in %q: %v\n", wtPath, err)
	}

	m.active[wtPath] = branch
	return wtPath, nil
}

// preCommitHookScript is the shell script installed into each agent worktree.
// It prevents accidental commits to main/master without SAW_ALLOW_MAIN_COMMIT=1.
const preCommitHookScript = `#!/bin/sh
branch=$(git symbolic-ref --short HEAD 2>/dev/null)
if [ "$branch" = "main" ] || [ "$branch" = "master" ]; then
  if [ -z "$SAW_ALLOW_MAIN_COMMIT" ]; then
    echo "SAW pre-commit guard: commits to '$branch' are blocked in agent worktrees." >&2
    echo "Set SAW_ALLOW_MAIN_COMMIT=1 to override." >&2
    exit 1
  fi
fi
`

// installPreCommitHook writes the SAW pre-commit guard into the worktree's
// .git/hooks/pre-commit file and makes it executable.
func installPreCommitHook(wtPath string) error {
	// Git worktrees have .git as a file containing "gitdir: /path/.git/worktrees/name"
	// We need to read this file to find the actual git directory where hooks live.
	gitFilePath := filepath.Join(wtPath, ".git")
	gitFileContent, err := os.ReadFile(gitFilePath)
	if err != nil {
		return fmt.Errorf("installPreCommitHook: read .git file: %w", err)
	}

	// Parse "gitdir: /path/.git/worktrees/name" to extract the path
	gitdirLine := strings.TrimSpace(string(gitFileContent))
	const prefix = "gitdir: "
	if !strings.HasPrefix(gitdirLine, prefix) {
		return fmt.Errorf("installPreCommitHook: .git file has unexpected format: %q", gitdirLine)
	}
	gitDir := strings.TrimSpace(strings.TrimPrefix(gitdirLine, prefix))

	hooksDir := filepath.Join(gitDir, "hooks")
	if err := os.MkdirAll(hooksDir, 0o755); err != nil {
		return fmt.Errorf("installPreCommitHook: mkdir %q: %w", hooksDir, err)
	}
	hookPath := filepath.Join(hooksDir, "pre-commit")
	if err := os.WriteFile(hookPath, []byte(preCommitHookScript), 0o755); err != nil {
		return fmt.Errorf("installPreCommitHook: write %q: %w", hookPath, err)
	}
	return nil
}

// Remove removes the worktree at the given absolute path and deletes its branch.
func (m *Manager) Remove(path string) error {
	branch, ok := m.active[path]
	if !ok {
		return fmt.Errorf("manager: worktree %q is not tracked", path)
	}

	if err := git.WorktreeRemove(m.repoPath, path); err != nil {
		return fmt.Errorf("manager: remove worktree %q: %w", path, err)
	}

	if err := git.DeleteBranch(m.repoPath, branch); err != nil {
		// Log but don't fail — the worktree itself is already removed.
		fmt.Fprintf(os.Stderr, "manager: warning: could not delete branch %q: %v\n", branch, err)
	}

	delete(m.active, path)
	return nil
}

// CleanupAll removes all tracked worktrees. It is best-effort: all worktrees
// are attempted even if some fail. Returns the last error encountered, if any.
func (m *Manager) CleanupAll() error {
	var lastErr error
	// Collect paths to avoid mutating the map while iterating.
	paths := make([]string, 0, len(m.active))
	for path := range m.active {
		paths = append(paths, path)
	}

	for _, path := range paths {
		if err := m.Remove(path); err != nil {
			fmt.Fprintf(os.Stderr, "manager: cleanup error for %q: %v\n", path, err)
			lastErr = err
		}
	}

	return lastErr
}

// List returns the absolute paths of all currently tracked worktrees.
func (m *Manager) List() []string {
	paths := make([]string, 0, len(m.active))
	for path := range m.active {
		paths = append(paths, path)
	}
	return paths
}
