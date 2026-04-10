package protocol

import (
	"path/filepath"
	"regexp"
	"strings"
)

// BriefOwnershipDivergence describes a file mentioned in a brief that is not
// in that agent's file_ownership table.
type BriefOwnershipDivergence struct {
	AgentID     string `json:"agent_id"`
	FilePath    string `json:"file_path"`
	OwningAgent string `json:"owning_agent,omitempty"` // non-empty if owned by another agent
	Message     string `json:"message"`
}

// reBriefFilePath matches file paths in task text:
//   - paths containing "/" (e.g. pkg/protocol/types.go, internal/lsp/manager.go)
//   - paths ending in common source extensions (even without slash)
var reBriefFilePath = regexp.MustCompile(
	`\b((?:[a-zA-Z0-9_.-]+/)+[a-zA-Z0-9_.-]+\.[a-zA-Z]{1,5}|` +
		`[a-zA-Z0-9_.-]+\.(?:go|ts|js|py|rs|md|yaml|yml|sh|json))\b`,
)

// DetectBriefOwnershipDivergence scans each agent's task text for file path
// references and checks them against file_ownership. Returns WARNING-level
// divergences: files mentioned in a brief that have no ownership entry for
// that agent, or that are owned by a different agent.
//
// Cross-mentions in "do not touch" constraints produce the same WARNING as other
// mentions — callers should display these as informational rather than blocking.
func DetectBriefOwnershipDivergence(manifest *IMPLManifest) []BriefOwnershipDivergence {
	if manifest == nil {
		return nil
	}

	// Build ownership index: file -> owning agent ID.
	// Use the relative file path (fo.File) as the key since task text uses relative paths.
	ownerOf := make(map[string]string) // normalized relative path -> agent ID
	for _, fo := range manifest.FileOwnership {
		ownerOf[normalizePath(fo.File)] = fo.Agent
	}

	var divergences []BriefOwnershipDivergence

	for _, wave := range manifest.Waves {
		for _, agent := range wave.Agents {
			// Extract all file path references from the task text.
			paths := extractBriefFilePaths(agent.Task)
			seen := make(map[string]bool)
			for _, p := range paths {
				norm := normalizePath(p)
				if seen[norm] {
					continue
				}
				seen[norm] = true

				owner, exists := ownerOf[norm]
				if !exists {
					// File mentioned but no ownership entry for anyone.
					divergences = append(divergences, BriefOwnershipDivergence{
						AgentID:  agent.ID,
						FilePath: p,
						Message:  "file mentioned in brief but has no file_ownership entry",
					})
				} else if owner != agent.ID {
					// File mentioned but owned by a different agent.
					divergences = append(divergences, BriefOwnershipDivergence{
						AgentID:     agent.ID,
						FilePath:    p,
						OwningAgent: owner,
						Message:     "file mentioned in brief but owned by agent " + owner,
					})
				}
				// owner == agent.ID: OK, no divergence.
			}
		}
	}

	return divergences
}

// extractBriefFilePaths returns all file path references found in taskText.
// Deduplicates results.
func extractBriefFilePaths(taskText string) []string {
	matches := reBriefFilePath.FindAllString(taskText, -1)
	seen := make(map[string]bool)
	var result []string
	for _, m := range matches {
		if !seen[m] {
			seen[m] = true
			result = append(result, m)
		}
	}
	return result
}

// normalizePath cleans a file path for consistent map lookup.
// Strips leading "./" and cleans double slashes.
func normalizePath(p string) string {
	p = filepath.Clean(p)
	p = strings.TrimPrefix(p, "./")
	return p
}
