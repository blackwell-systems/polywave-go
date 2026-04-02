package hooks

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/result"
)

// ValidateData holds the output of a successful or partial scout boundary validation.
type ValidateData struct {
	ValidatedPath    string
	UnexpectedWrites []string
	Violations       []result.SAWError
}

// ValidateScoutWrites checks if Scout wrote files outside its boundaries.
// Call this after Scout execution completes.
// Returns:
//   - Success if no unexpected writes found
//   - Partial if unexpected writes exist (non-fatal, reportable)
//   - Fatal on filesystem scan error
func ValidateScoutWrites(repoPath, expectedIMPLPath string, startTime time.Time) result.Result[ValidateData] {
	docsDir := filepath.Join(repoPath, "docs")
	if _, err := os.Stat(docsDir); os.IsNotExist(err) {
		// No docs/ directory - no violations possible
		return result.NewSuccess(ValidateData{ValidatedPath: repoPath})
	}

	var unexpectedWrites []string

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
		relPath, err := filepath.Rel(repoPath, path)
		if err != nil {
			// If we can't compute relative path, use absolute path
			// Validation logic works correctly with absolute paths
			relPath = path
		}

		// Check if it's the expected IMPL doc
		if path == expectedIMPLPath {
			return nil // Allowed
		}

		// Check if it's a valid IMPL doc in docs/IMPL/
		if IsValidScoutPath(relPath) {
			return nil // Allowed
		}

		// Everything else is a violation
		unexpectedWrites = append(unexpectedWrites, relPath)
		return nil
	})

	if err != nil {
		return result.NewFailure[ValidateData]([]result.SAWError{
			result.NewFatal(result.CodeScoutBoundaryViolation, fmt.Sprintf("scout boundaries validation: walk failed: %v", err)),
		})
	}

	if len(unexpectedWrites) > 0 {
		violations := make([]result.SAWError, len(unexpectedWrites))
		for i, path := range unexpectedWrites {
			violations[i] = result.SAWError{
				Code:    result.CodeScoutBoundaryViolation,
				Message: fmt.Sprintf("Scout wrote file outside permitted boundaries: %s", path),
				Severity: "warning",
				File:    path,
				Suggestion: "Allowed: docs/IMPL/IMPL-<feature-slug>.yaml. " +
					"Scout's role is reconnaissance only. " +
					"Orchestrator writes docs/REQUIREMENTS.md (bootstrap); " +
					"Wave agents write source code; " +
					"Tools (sawtools) manage CONTEXT.md and archiving.",
			}
		}

		data := ValidateData{
			ValidatedPath:    repoPath,
			UnexpectedWrites: unexpectedWrites,
			Violations:       violations,
		}
		return result.NewPartial(data, violations)
	}

	return result.NewSuccess(ValidateData{ValidatedPath: repoPath})
}

// IsValidScoutPath checks if a file path is within Scout's write boundaries.
// Used for pre-execution validation (CLI hooks) and testing.
// Accepts both relative ("docs/IMPL/IMPL-x.yaml") and absolute paths.
func IsValidScoutPath(filePath string) bool {
	normalized := filepath.Clean(filePath)
	dir := filepath.Dir(normalized)
	base := filepath.Base(normalized)

	// Validate filename
	if !strings.HasPrefix(base, "IMPL-") || !strings.HasSuffix(base, ".yaml") {
		return false
	}
	// Validate directory: must end with "docs/IMPL" (handles both relative and absolute paths)
	// Use filepath.Base twice to check the last two path components.
	implDir := filepath.Base(dir)              // must be "IMPL"
	docsDir := filepath.Base(filepath.Dir(dir)) // must be "docs"
	return implDir == "IMPL" && docsDir == "docs"
}
