package protocol

import (
	"fmt"
	"regexp"
	"strings"
)

// StaleConstraintWarning is emitted when an agent's task text references a
// file path that is owned by a different agent in the manifest.
type StaleConstraintWarning struct {
	AgentID       string `json:"agent_id"`
	MentionedFile string `json:"mentioned_file"`
	OwnedByAgent  string `json:"owned_by_agent"`
	Message       string `json:"message"`
}

// filePathRegex matches file path references in task text.
// It requires recognized extensions and boundary anchors to minimize false positives.
// It does NOT match URLs (containing "://") or version-like filenames.
var filePathRegex = regexp.MustCompile(
	`(?:^|[\s` + "`" + `'"(])([a-zA-Z0-9_\-./]+\.(?:go|md|sh|yaml|yml|ts|js|py|rs|toml|json|txt))(?:$|[\s` + "`" + `'")\]:,.])`,
)

// DetectStaleConstraints scans each agent's task text for file path references
// that appear in another agent's file_ownership, emitting a warning for each match.
// Returns nil if no stale references are found.
func DetectStaleConstraints(m *IMPLManifest) []StaleConstraintWarning {
	if m == nil {
		return nil
	}

	// Build two lookup maps from FileOwnership:
	//   ownerMap: full path → agent ID  (e.g. "pkg/engine/manager.go" → "A")
	//   basenameMap: basename → agent ID (e.g. "manager.go" → "A")
	// The basename map is only used when the full-path lookup fails, enabling
	// detection of bare-name references like "manager.go" when the ownership
	// entry uses the full path "pkg/engine/manager.go".
	// If multiple agents own files with the same basename, the basename map
	// entry for that name is cleared (ambiguous — skip to avoid false positives).
	ownerMap := make(map[string]string, len(m.FileOwnership))
	basenameMap := make(map[string]string, len(m.FileOwnership))
	for _, fo := range m.FileOwnership {
		ownerMap[fo.File] = fo.Agent
		base := fo.File
		if idx := strings.LastIndex(fo.File, "/"); idx >= 0 {
			base = fo.File[idx+1:]
		}
		if existing, conflict := basenameMap[base]; conflict && existing != fo.Agent {
			// Ambiguous basename — mark as unresolvable by setting to empty string.
			basenameMap[base] = ""
		} else if !conflict {
			basenameMap[base] = fo.Agent
		}
	}

	if len(ownerMap) == 0 {
		return nil
	}

	var warnings []StaleConstraintWarning

	for _, wave := range m.Waves {
		for _, agent := range wave.Agents {
			// Extract all file path references from this agent's task text.
			paths := extractFilePaths(agent.Task)

			for path := range paths {
				// Look up ownership: try full path first, then basename fallback.
				owner, exists := ownerMap[path]
				if !exists {
					base := path
					if idx := strings.LastIndex(path, "/"); idx >= 0 {
						base = path[idx+1:]
					}
					owner = basenameMap[base]
					exists = owner != ""
				}
				if !exists {
					// File not owned by anyone — no warning (avoids false positives).
					continue
				}
				if owner == agent.ID {
					// Agent references its own file — not stale.
					continue
				}
				warnings = append(warnings, StaleConstraintWarning{
					AgentID:       agent.ID,
					MentionedFile: path,
					OwnedByAgent:  owner,
					Message: fmt.Sprintf(
						"Agent %s task mentions `%s` which is owned by Agent %s — possible stale constraint text",
						agent.ID, path, owner,
					),
				})
			}
		}
	}

	if len(warnings) == 0 {
		return nil
	}
	return warnings
}

// extractFilePaths extracts deduplicated file path candidates from text.
// It applies the file path regex, strips leading "./" and filters out
// URLs and version-like basenames.
func extractFilePaths(text string) map[string]struct{} {
	matches := filePathRegex.FindAllStringSubmatch(text, -1)
	result := make(map[string]struct{})
	for _, m := range matches {
		if len(m) < 2 {
			continue
		}
		path := m[1]

		// Reject URLs.
		if strings.Contains(path, "://") {
			continue
		}

		// Reject semver-like basenames: base component starts with a digit
		// followed by only digits and dots (e.g. "1.2.3" in "v1.2.3").
		base := path
		if idx := strings.LastIndex(path, "/"); idx >= 0 {
			base = path[idx+1:]
		}
		// Strip leading 'v' for version check (e.g. "v1.0.go")
		checkBase := base
		if strings.HasPrefix(checkBase, "v") {
			checkBase = checkBase[1:]
		}
		// If what remains before the extension is all digits and dots, skip.
		extIdx := strings.LastIndex(checkBase, ".")
		if extIdx > 0 {
			stem := checkBase[:extIdx]
			if isSemverStem(stem) {
				continue
			}
		}

		// Normalize: strip leading "./".
		path = strings.TrimPrefix(path, "./")

		result[path] = struct{}{}
	}
	return result
}

// isSemverStem returns true if s consists only of digits and dots,
// indicating a version number stem like "1", "1.2", "1.2.3".
func isSemverStem(s string) bool {
	if s == "" {
		return false
	}
	for _, ch := range s {
		if ch != '.' && (ch < '0' || ch > '9') {
			return false
		}
	}
	return true
}
