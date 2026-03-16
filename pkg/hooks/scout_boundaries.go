package hooks

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ValidateScoutWrites checks if Scout wrote files outside its boundaries.
// Call this after Scout execution completes.
// Returns error with violation details if any unauthorized writes detected.
func ValidateScoutWrites(repoPath, expectedIMPLPath string, startTime time.Time) error {
	docsDir := filepath.Join(repoPath, "docs")
	if _, err := os.Stat(docsDir); os.IsNotExist(err) {
		// No docs/ directory - no violations possible
		return nil
	}

	var violations []string

	// Walk docs/ tree looking for files modified after startTime
	err := filepath.Walk(docsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}

		// Skip if not modified after Scout started
		if !info.ModTime().After(startTime) {
			return nil
		}

		// Get relative path from repo root
		relPath, _ := filepath.Rel(repoPath, path)

		// Check if it's the expected IMPL doc
		if path == expectedIMPLPath {
			return nil // Allowed
		}

		// Check if it's a valid IMPL doc in docs/IMPL/
		dir := filepath.Dir(relPath)
		base := filepath.Base(relPath)
		if dir == "docs/IMPL" && strings.HasPrefix(base, "IMPL-") && strings.HasSuffix(base, ".yaml") {
			return nil // Allowed
		}

		// Everything else is a violation
		violations = append(violations, relPath)
		return nil
	})

	if err != nil {
		return fmt.Errorf("scout boundaries validation: walk failed: %w", err)
	}

	if len(violations) > 0 {
		return fmt.Errorf(`I6 VIOLATION: Scout wrote files outside permitted boundaries.

Unauthorized writes detected:
%s

Allowed: docs/IMPL/IMPL-<feature-slug>.yaml

Scout's role is reconnaissance (analyze, plan, document).
- Orchestrator writes docs/REQUIREMENTS.md (bootstrap)
- Wave agents write source code
- Tools (sawtools) manage CONTEXT.md and archiving`,
			strings.Join(violations, "\n"))
	}

	return nil
}

// IsValidScoutPath checks if a file path is within Scout's write boundaries.
// Used for pre-execution validation (CLI hooks) and testing.
func IsValidScoutPath(filePath string) bool {
	normalized := filepath.Clean(filePath)
	dir := filepath.Dir(normalized)
	base := filepath.Base(normalized)

	// Allow only docs/IMPL/IMPL-*.yaml (not subdirectories)
	return dir == "docs/IMPL" && strings.HasPrefix(base, "IMPL-") && strings.HasSuffix(base, ".yaml")
}
