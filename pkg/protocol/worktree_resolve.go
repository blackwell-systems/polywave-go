package protocol

import (
	"os"
	"path/filepath"
	"strings"
)

// WorktreeBaseDirs lists the directory names (inside a repo root) where SAW
// worktrees may live. The canonical location is ".claude"; ".claire" is a
// known Claude hallucination that occurs in some sessions.
var WorktreeBaseDirs = []string{".claude", ".claire"}

// ResolveWorktreePath finds an existing worktree by checking all known base
// directories. Returns the first path that exists on disk, or falls back to
// the canonical ".claude" path if none exist (for error messages / creation).
func ResolveWorktreePath(repoDir string, branch string) string {
	for _, base := range WorktreeBaseDirs {
		candidate := filepath.Join(repoDir, base, "worktrees", branch)
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	// Fallback: canonical path (even if it doesn't exist yet).
	return filepath.Join(repoDir, WorktreeBaseDirs[0], "worktrees", branch)
}

// IsWorktreePath returns true if the given absolute path contains a worktree
// directory segment from any of the known base directories.
func IsWorktreePath(absPath string) bool {
	for _, base := range WorktreeBaseDirs {
		if strings.Contains(absPath, base+"/worktrees/") {
			return true
		}
	}
	return false
}
