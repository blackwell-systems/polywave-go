package protocol

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/result"
)

// handlerPatterns lists path patterns that indicate new API handlers or UI components
// that typically require integration steps.
var handlerPatterns = []string{
	"pkg/api/",
	"web/src/components/",
}

// matchesHandlerPattern returns true if the file path looks like an API handler
// (pkg/api/*_handler.go) or a React component (web/src/components/*.tsx).
func matchesHandlerPattern(file string) bool {
	// Normalize to forward slashes for consistent matching.
	normalized := filepath.ToSlash(file)

	// API handler: path must contain "pkg/api/" and end with "_handler.go"
	if strings.Contains(normalized, "pkg/api/") && strings.HasSuffix(normalized, "_handler.go") {
		return true
	}

	// React component: path must contain "web/src/components/" and end with ".tsx"
	if strings.Contains(normalized, "web/src/components/") && strings.HasSuffix(normalized, ".tsx") {
		return true
	}

	return false
}

// ValidateIntegrationChecklist warns when new API handlers or React components are
// declared in file_ownership but no post_merge_checklist is provided. This is a
// warning (not an error) — the IMPL will still proceed.
//
// Detection heuristic:
//  1. Scan m.FileOwnership for entries with action="new".
//  2. Match file paths against handler/component patterns.
//  3. If any matches found AND post_merge_checklist is nil or empty, emit a warning.
//
// If repoPath is non-empty, each matched file is verified to exist on disk before
// triggering the warning (avoids false positives from typos in the IMPL doc).
func ValidateIntegrationChecklist(m *IMPLManifest, repoPath string) []result.SAWError {
	var matched []string

	for _, fo := range m.FileOwnership {
		if fo.Action != "new" {
			continue
		}
		if !matchesHandlerPattern(fo.File) {
			continue
		}
		// If repoPath is provided, verify the file actually exists.
		if repoPath != "" {
			absPath := filepath.Join(repoPath, fo.File)
			if _, err := os.Stat(absPath); os.IsNotExist(err) {
				// File doesn't exist yet — skip to avoid false positives on typos.
				continue
			}
		}
		matched = append(matched, fo.File)
	}

	if len(matched) == 0 {
		return nil
	}

	// Check whether the manifest has a non-empty post_merge_checklist.
	if m.PostMergeChecklist != nil && len(m.PostMergeChecklist.Groups) > 0 {
		return nil
	}

	return []result.SAWError{
		{
			Code:     result.CodeMissingChecklist,
			Message:  "new handlers/components detected but post_merge_checklist is empty — integration steps may be needed",
			Severity: "warning",
			Field:    "post_merge_checklist",
		},
	}
}
