// Package worktree manages git worktrees for SAW wave agents. Each agent
// in a wave receives an isolated worktree branched from HEAD so that
// parallel file edits cannot conflict during execution. The Manager
// handles creation, path resolution, and cleanup of worktrees.
package worktree

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/blackwell-systems/scout-and-wave-go/internal/git"
	"github.com/blackwell-systems/scout-and-wave-go/pkg/protocol"
)

// Manager tracks and manages git worktrees for SAW wave agents.
type Manager struct {
	repoPath string
	slug     string            // IMPL feature slug for slug-scoped branch naming
	active   map[string]string // absolute worktree path -> branch name
	logger   *slog.Logger
}

// New creates a new Manager for the git repository at repoPath.
// The slug parameter is the IMPL feature slug used for slug-scoped branch naming.
func New(repoPath, slug string) *Manager {
	return &Manager{
		repoPath: repoPath,
		slug:     slug,
		active:   make(map[string]string),
	}
}

// SetLogger configures the logger used for non-fatal diagnostic messages.
// If never called, the manager falls back to slog.Default().
func (m *Manager) SetLogger(logger *slog.Logger) {
	m.logger = logger
}

// log returns the configured logger, falling back to slog.Default() if nil.
func (m *Manager) log() *slog.Logger {
	if m.logger == nil {
		return slog.Default()
	}
	return m.logger
}

// Create creates a new worktree for the given wave number and agent letter.
// The worktree path follows the slug-scoped convention:
//
//	{repoPath}/.claude/worktrees/saw/{slug}/wave{wave}-agent-{agent}
//
// A new branch with the slug-scoped name is created from HEAD.
// Returns the absolute path to the created worktree.
func (m *Manager) Create(wave int, agent string) (string, error) {
	branch := protocol.BranchName(m.slug, wave, agent)
	wtPath := protocol.WorktreeDir(m.repoPath, m.slug, wave, agent)
	wtDir := filepath.Dir(wtPath)

	if err := os.MkdirAll(wtDir, 0o755); err != nil {
		return "", fmt.Errorf("failed to create worktree base directory %q: %w", wtDir, err)
	}

	if err := git.WorktreeAdd(m.repoPath, wtPath, branch); err != nil {
		return "", fmt.Errorf("manager: create worktree for wave %d agent %s: %w", wave, agent, err)
	}

	if err := git.InstallHooks(m.repoPath, wtPath); err != nil {
		// Non-fatal: log but do not abort worktree creation.
		m.log().Warn("manager: could not install pre-commit hook", "path", wtPath, "err", err)
	}

	m.active[wtPath] = branch
	return wtPath, nil
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
		m.log().Warn("manager: could not delete branch", "branch", branch, "err", err)
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
			m.log().Warn("manager: cleanup error", "path", path, "err", err)
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
