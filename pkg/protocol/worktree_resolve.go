package protocol

import (
	"os"
	"path/filepath"
	"strings"
)

// WorktreeBaseDirs lists the directory names (inside a repo root) where Polywave
// worktrees may live. The canonical location is ".claude"; ".claire" is a
// known Claude hallucination that occurs in some sessions.
var WorktreeBaseDirs = []string{".claude", ".claire"}

// ResolveWorktreePath finds an existing worktree by checking all known base
// directories. It first checks slug-scoped paths (saw/{slug}/...) then falls
// back to legacy flat paths for backward compatibility.
//
// Returns the first path that exists on disk, or falls back to the canonical
// ".claude" path if none exist (for error messages / creation).
//
// If slug is non-empty, the canonical fallback uses the slug-scoped layout.
// If slug is empty, it falls back to the legacy flat layout.
func ResolveWorktreePath(repoDir string, branch string) string {
	// Extract slug from the branch name to determine path layout
	slug := ExtractSlug(branch)
	// The directory-local part of the branch (wave{N}-agent-{ID})
	localPart := branch
	if slug != "" {
		// Strip "polywave/{slug}/" prefix to get local directory name
		localPart = LegacyBranchName(0, "") // placeholder; parse properly
		if w, a, ok := ParseBranch(branch); ok {
			localPart = LegacyBranchName(w, a)
		}
	}

	// Check slug-scoped paths first (if slug is known)
	if slug != "" {
		for _, base := range WorktreeBaseDirs {
			candidate := filepath.Join(repoDir, base, "worktrees", "polywave", slug, localPart)
			if _, err := os.Stat(candidate); err == nil {
				return candidate
			}
		}
	}

	// Check legacy flat paths (backward compat)
	for _, base := range WorktreeBaseDirs {
		candidate := filepath.Join(repoDir, base, "worktrees", localPart)
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}

	// Fallback: canonical path (slug-scoped if slug is available).
	if slug != "" {
		return filepath.Join(repoDir, WorktreeBaseDirs[0], "worktrees", "polywave", slug, localPart)
	}
	return filepath.Join(repoDir, WorktreeBaseDirs[0], "worktrees", localPart)
}

// ResolveWorktreePathWithSlug finds an existing worktree by checking slug-scoped
// paths first, then falling back to legacy flat paths. Use this when you have
// the slug available separately (e.g., from the manifest).
func ResolveWorktreePathWithSlug(repoDir, slug string, waveNum int, agentID string) string {
	localPart := LegacyBranchName(waveNum, agentID)

	// Check slug-scoped paths first
	if slug != "" {
		for _, base := range WorktreeBaseDirs {
			candidate := filepath.Join(repoDir, base, "worktrees", "polywave", slug, localPart)
			if _, err := os.Stat(candidate); err == nil {
				return candidate
			}
		}
	}

	// Check legacy flat paths (backward compat)
	for _, base := range WorktreeBaseDirs {
		candidate := filepath.Join(repoDir, base, "worktrees", localPart)
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}

	// Fallback: canonical slug-scoped path
	if slug != "" {
		return filepath.Join(repoDir, WorktreeBaseDirs[0], "worktrees", "polywave", slug, localPart)
	}
	return filepath.Join(repoDir, WorktreeBaseDirs[0], "worktrees", localPart)
}

// IsWorktreePath returns true if the given absolute path contains a worktree
// directory segment from any of the known base directories. Handles both
// legacy flat layout (.claude/worktrees/wave1-agent-A) and slug-scoped
// layout (.claude/worktrees/saw/{slug}/wave1-agent-A).
func IsWorktreePath(absPath string) bool {
	for _, base := range WorktreeBaseDirs {
		if strings.Contains(absPath, base+"/worktrees/") {
			return true
		}
	}
	return false
}
