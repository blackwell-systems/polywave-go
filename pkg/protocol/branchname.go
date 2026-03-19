package protocol

import (
	"fmt"
	"path/filepath"
	"regexp"
)

// BranchName returns the slug-scoped branch name for an agent.
// Format: saw/<slug>/wave{N}-agent-{ID}
//
// This is the primary format for all new branches created after v0.39.0.
// The slug prefix prevents collision across different IMPL documents.
func BranchName(slug string, waveNum int, agentID string) string {
	return fmt.Sprintf("saw/%s/wave%d-agent-%s", slug, waveNum, agentID)
}

// LegacyBranchName returns the old-format branch name for backward compatibility.
// Format: wave{N}-agent-{ID}
//
// Used by code that needs to detect or clean up branches created before v0.39.0.
func LegacyBranchName(waveNum int, agentID string) string {
	return fmt.Sprintf("wave%d-agent-%s", waveNum, agentID)
}

// WorktreeDir returns the slug-scoped worktree directory path.
// Format: {repoDir}/.claude/worktrees/saw/{slug}/wave{N}-agent-{ID}
//
// This matches the branch naming structure and prevents worktree path collisions
// when multiple IMPL documents are active in the same repository.
func WorktreeDir(repoDir, slug string, waveNum int, agentID string) string {
	return filepath.Join(repoDir, ".claude", "worktrees", "saw", slug,
		fmt.Sprintf("wave%d-agent-%s", waveNum, agentID))
}

// ScopedBranchRegex matches both old and new branch name formats.
// Group 1: wave number, Group 2: agent ID
//
// Accepts:
//   - "wave1-agent-A" (legacy format, pre-v0.39.0)
//   - "saw/my-slug/wave1-agent-A" (new slug-scoped format)
//
// The optional slug prefix allows backward compatibility during transition.
var ScopedBranchRegex = regexp.MustCompile(
	`^(?:saw/[a-z0-9][-a-z0-9]*/)?wave(\d+)-agent-([A-Z][2-9]?)$`)

// ParseBranch extracts wave number and agent ID from either format.
// Returns (wave, agentID, ok).
//
// Examples:
//   - "wave1-agent-A" -> (1, "A", true)
//   - "saw/my-slug/wave2-agent-B3" -> (2, "B3", true)
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
//   - "saw/my-slug/wave1-agent-A" -> "my-slug"
//   - "wave1-agent-A" -> ""
func ExtractSlug(branch string) string {
	// Pattern: saw/<slug>/wave...-agent-...
	if len(branch) > 4 && branch[:4] == "saw/" {
		rest := branch[4:]
		for i, c := range rest {
			if c == '/' {
				return rest[:i]
			}
		}
	}
	return ""
}
