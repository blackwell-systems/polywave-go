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
	"sort"

	"github.com/blackwell-systems/scout-and-wave-go/internal/git"
	"github.com/blackwell-systems/scout-and-wave-go/pkg/protocol"
	"github.com/blackwell-systems/scout-and-wave-go/pkg/result"
)

// RemoveData holds the result payload for a successful Remove operation.
type RemoveData struct {
	RemovedPath string
}

// CleanupData holds the result payload for a CleanupAll operation.
type CleanupData struct {
	RemovedCount int
	RemovedPaths []string
}

// CreateData holds the result payload for a successful Create operation.
type CreateData struct {
	Path   string
	Branch string
}

// Manager tracks and manages git worktrees for SAW wave agents.
// Manager is not safe for concurrent use.
type Manager struct {
	repoPath string
	slug     string            // IMPL feature slug for slug-scoped branch naming
	active   map[string]string // absolute worktree path -> branch name
	logger   *slog.Logger
}

// New creates a new Manager for the git repository at repoPath.
// The slug parameter is the IMPL feature slug used for slug-scoped branch naming.
func New(repoPath, slug string) (*Manager, error) {
	if repoPath == "" {
		return nil, fmt.Errorf("worktree: repoPath must not be empty")
	}
	if slug == "" {
		return nil, fmt.Errorf("worktree: slug must not be empty")
	}
	return &Manager{
		repoPath: repoPath,
		slug:     slug,
		active:   make(map[string]string),
	}, nil
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
// Returns a result.Result[CreateData] with the path and branch on success.
func (m *Manager) Create(wave int, agent string) result.Result[CreateData] {
	branch := protocol.BranchName(m.slug, wave, agent)
	wtPath := protocol.WorktreeDir(m.repoPath, m.slug, wave, agent)
	wtDir := filepath.Dir(wtPath)

	if err := os.MkdirAll(wtDir, 0o755); err != nil {
		return result.NewFailure[CreateData]([]result.SAWError{
			result.NewFatal(result.CodeWorktreeCreateFailed,
				fmt.Sprintf("worktree: failed to create worktree base directory %q: %v", wtDir, err)).WithCause(err),
		})
	}

	if _, err := os.Stat(wtPath); err == nil {
		return result.NewFailure[CreateData]([]result.SAWError{
			result.NewFatal(result.CodeWorktreeCreateFailed,
				fmt.Sprintf("worktree: worktree path %q already exists; remove it or use a different wave/agent combination before retrying", wtPath)),
		})
	}

	if git.BranchExists(m.repoPath, branch) {
		return result.NewFailure[CreateData]([]result.SAWError{
			result.NewFatal(result.CodeBranchExists,
				fmt.Sprintf("worktree: branch %q already exists; remove it before retrying", branch)),
		})
	}

	if err := git.WorktreeAdd(m.repoPath, wtPath, branch); err != nil {
		return result.NewFailure[CreateData]([]result.SAWError{
			result.NewFatal(result.CodeWorktreeCreateFailed,
				fmt.Sprintf("worktree: create worktree for wave %d agent %s: %v", wave, agent, err)).WithCause(err),
		})
	}

	if err := git.InstallHooks(m.repoPath, wtPath); err != nil {
		// Non-fatal: log but do not abort worktree creation.
		m.log().Warn("worktree: could not install pre-commit hook", "path", wtPath, "err", err)
	}

	m.active[wtPath] = branch
	return result.NewSuccess(CreateData{Path: wtPath, Branch: branch})
}

// Remove removes the worktree at the given absolute path and attempts to
// delete its branch. Branch deletion is best-effort: if it fails the failure
// is logged but does not affect the returned result.
// Returns a result.Result[RemoveData] with the removed path on success, or a
// Fatal result if the path is not tracked or the filesystem removal fails.
func (m *Manager) Remove(path string) result.Result[RemoveData] {
	branch, ok := m.active[path]
	if !ok {
		return result.NewFailure[RemoveData]([]result.SAWError{
			result.NewFatal(result.CodeWorktreeRemoveFailed, fmt.Sprintf("worktree: worktree %q is not tracked", path)),
		})
	}

	if err := git.WorktreeRemove(m.repoPath, path); err != nil {
		return result.NewFailure[RemoveData]([]result.SAWError{
			result.NewFatal(result.CodeWorktreeRemoveFailed, fmt.Sprintf("worktree: remove worktree %q: %v", path, err)).WithCause(err),
		})
	}

	if err := git.DeleteBranch(m.repoPath, branch); err != nil {
		// Log but don't fail — the worktree itself is already removed.
		m.log().Warn("worktree: could not delete branch", "branch", branch, "err", err)
	}

	delete(m.active, path)
	return result.NewSuccess(RemoveData{
		RemovedPath: path,
	})
}

// CleanupAll removes all tracked worktrees. It is best-effort: all worktrees
// are attempted even if some fail. Returns Partial if some succeed and some
// fail, Fatal if all removals fail, and Success if all succeed.
func (m *Manager) CleanupAll() result.Result[CleanupData] {
	// Collect paths to avoid mutating the map while iterating.
	paths := make([]string, 0, len(m.active))
	for path := range m.active {
		paths = append(paths, path)
	}

	var removedPaths []string
	var failedErrs []result.SAWError

	for _, path := range paths {
		r := m.Remove(path)
		if r.IsFatal() {
			failedErrs = append(failedErrs, r.Errors...)
			m.log().Warn("worktree: cleanup error", "path", path)
		} else {
			removedPaths = append(removedPaths, path)
		}
	}

	sort.Strings(removedPaths)
	data := CleanupData{
		RemovedCount: len(removedPaths),
		RemovedPaths: removedPaths,
	}

	if len(failedErrs) == 0 {
		return result.NewSuccess(data)
	}

	if len(removedPaths) == 0 {
		// All removals failed — total failure.
		return result.NewFailure[CleanupData](failedErrs)
	}
	// Some succeeded, some failed — partial.
	return result.NewPartial(data, failedErrs)
}

// List returns the absolute paths of all currently tracked worktrees.
func (m *Manager) List() []string {
	paths := make([]string, 0, len(m.active))
	for path := range m.active {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	return paths
}
