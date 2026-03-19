package protocol

import (
	"fmt"
	"os"
	"path/filepath"
)

// ValidateFileExistence warns when file_ownership entries with action=modify
// reference files that do not exist on disk.
//
// If repoPath is empty, all checks are skipped (allows offline/schema-only
// validation). If a FileOwnership entry has a non-empty Repo field that does
// not match the basename of repoPath, the entry belongs to a different
// repository and is skipped.
func ValidateFileExistence(m *IMPLManifest, repoPath string) []ValidationError {
	// Skip entirely when no repoPath provided
	if repoPath == "" {
		return nil
	}

	repoBasename := filepath.Base(repoPath)

	var errs []ValidationError
	for i, fo := range m.FileOwnership {
		// Only check "modify" entries
		if fo.Action != "modify" {
			continue
		}

		// Skip entries that belong to a different repository
		if fo.Repo != "" && fo.Repo != repoBasename {
			continue
		}

		fullPath := filepath.Join(repoPath, fo.File)
		if _, err := os.Stat(fullPath); os.IsNotExist(err) {
			errs = append(errs, ValidationError{
				Code:    "E16_FILE_NOT_FOUND",
				Message: fmt.Sprintf("file '%s' marked action=modify but does not exist", fo.File),
				Field:   fmt.Sprintf("file_ownership[%d]", i),
			})
		}
	}

	return errs
}
