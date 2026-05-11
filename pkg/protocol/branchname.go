package protocol

import (
	"fmt"
	"path/filepath"
	"regexp"

	"github.com/blackwell-systems/polywave-go/internal/git"
)

// BranchName returns the slug-scoped branch name for an agent.
// Format: polywave/<slug>/wave{N}-agent-{ID}
//
// This is the primary format for all new branches created after v0.39.0.
// The slug prefix prevents collision across different IMPL documents.
func BranchName(slug string, waveNum int, agentID string) string {
	return fmt.Sprintf("polywave/%s/wave%d-agent-%s", slug, waveNum, agentID)
}

// LegacyBranchName returns the old-format branch name for backward compatibility.
// Format: wave{N}-agent-{ID}
//
// Used by code that needs to detect or clean up branches created before v0.39.0.
func LegacyBranchName(waveNum int, agentID string) string {
	return fmt.Sprintf("wave%d-agent-%s", waveNum, agentID)
}

// WorktreeDir returns the slug-scoped worktree directory path.
// Format: {repoDir}/.claude/worktrees/polywave/{slug}/wave{N}-agent-{ID}
//
// This matches the branch naming structure and prevents worktree path collisions
// when multiple IMPL documents are active in the same repository.
func WorktreeDir(repoDir, slug string, waveNum int, agentID string) string {
	return filepath.Join(repoDir, ".claude", "worktrees", "polywave", slug,
		fmt.Sprintf("wave%d-agent-%s", waveNum, agentID))
}

// ScopedBranchRegex matches both old and new branch name formats.
// Group 1: wave number, Group 2: agent ID
//
// Accepts:
//   - "wave1-agent-A" (legacy format, pre-v0.39.0)
//   - "polywave/my-slug/wave1-agent-A" (new slug-scoped format)
//
// The optional slug prefix allows backward compatibility during transition.
var ScopedBranchRegex = regexp.MustCompile(
	`^(?:polywave/[a-z0-9][-a-z0-9]*/)?wave(\d+)-agent-([A-Z][2-9]?)$`)

// ParseBranch extracts wave number and agent ID from either format.
// Returns (wave, agentID, ok).
//
// Examples:
//   - "wave1-agent-A" -> (1, "A", true)
//   - "polywave/my-slug/wave2-agent-B3" -> (2, "B3", true)
//   - "invalid" -> (0, "", false)
func ParseBranch(branch string) (int, string, bool) {
	m := ScopedBranchRegex.FindStringSubmatch(branch)
	if m == nil {
		return 0, "", false
	}
	var wave int
	fmt.Sscanf(m[1], "%d", &wave)
	return wave, m[2], true
}

// ExtractSlug extracts the slug from a scoped branch name.
// Returns empty string for legacy format branches.
//
// Examples:
//   - "polywave/my-slug/wave1-agent-A" -> "my-slug"
//   - "wave1-agent-A" -> ""
func ExtractSlug(branch string) string {
	// Pattern: polywave/<slug>/wave...-agent-...
	const prefix = "polywave/"
	if len(branch) > len(prefix) && branch[:len(prefix)] == prefix {
		rest := branch[len(prefix):]
		for i, c := range rest {
			if c == '/' {
				return rest[:i]
			}
		}
	}
	return ""
}

// resolveAgentBranch returns the active branch name for the given agent by
// checking whether the slug-scoped branch exists in repoDir via git.BranchExists.
// If the slug-scoped branch does not exist, falls back to LegacyBranchName.
// isLegacy is true when the legacy fallback was used.
// If neither branch exists, branchName is the slug-scoped form (BranchName result)
// and isLegacy is false.
func resolveAgentBranch(featureSlug, agentID string, waveNum int, repoDir string) (branchName string, isLegacy bool) {
	slug := BranchName(featureSlug, waveNum, agentID)
	if git.BranchExists(repoDir, slug) {
		return slug, false
	}
	legacy := LegacyBranchName(waveNum, agentID)
	if git.BranchExists(repoDir, legacy) {
		return legacy, true
	}
	return slug, false // neither exists; return slug-scoped form
}
