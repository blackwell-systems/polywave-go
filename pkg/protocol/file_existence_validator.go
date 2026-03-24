package protocol

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/result"
)

// ValidateFileExistence warns when file_ownership entries with action=modify
// reference files that do not exist on disk.
//
// If repoPath is empty, all checks are skipped (allows offline/schema-only
// validation). If a FileOwnership entry has a non-empty Repo field that does
// not match the basename of repoPath, the entry belongs to a different
// repository and is skipped.
func ValidateFileExistence(m *IMPLManifest, repoPath string) []result.SAWError {
	// Skip entirely when no repoPath provided
	if repoPath == "" {
		return nil
	}

	repoBasename := filepath.Base(repoPath)

	var errs []result.SAWError
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
			errs = append(errs, result.SAWError{
				Code:     "E16_FILE_NOT_FOUND",
				Message:  fmt.Sprintf("file '%s' marked action=modify but does not exist", fo.File),
				Severity: "error",
				Field:    fmt.Sprintf("file_ownership[%d]", i),
			})
		}
	}

	return errs
}

// ValidateFileExistenceMultiRepo is an enhanced version of ValidateFileExistence
// that resolves files across multiple repos using the configRepos registry.
// It checks that action=modify files exist in their resolved target repos.
// If ALL action=modify entries are missing, it also returns a
// E16_REPO_MISMATCH_SUSPECTED error.
func ValidateFileExistenceMultiRepo(m *IMPLManifest, primaryRepoPath string, configRepos []RepoEntry) []result.SAWError {
	if primaryRepoPath == "" {
		return nil
	}

	// Build config lookup
	configLookup := make(map[string]string, len(configRepos))
	for _, r := range configRepos {
		configLookup[r.Name] = r.Path
	}

	var errs []result.SAWError
	modifyCount := 0
	missingCount := 0

	for i, fo := range m.FileOwnership {
		if fo.Action != "modify" {
			continue
		}
		modifyCount++

		// Resolve which repo this file belongs to
		repoPath := primaryRepoPath
		if fo.Repo != "" {
			if p, ok := configLookup[fo.Repo]; ok {
				repoPath = p
			} else {
				// Try sibling directory
				siblingPath := filepath.Join(filepath.Dir(primaryRepoPath), fo.Repo)
				if info, err := os.Stat(siblingPath); err == nil && info.IsDir() {
					repoPath = siblingPath
				}
				// If we can't resolve the repo, check in primaryRepoPath anyway
			}
		}

		fullPath := filepath.Join(repoPath, fo.File)
		if _, err := os.Stat(fullPath); os.IsNotExist(err) {
			missingCount++
			errs = append(errs, result.SAWError{
				Code:     "E16_FILE_NOT_FOUND",
				Message:  fmt.Sprintf("file '%s' marked action=modify but does not exist in repo '%s'", fo.File, filepath.Base(repoPath)),
				Severity: "error",
				Field:    fmt.Sprintf("file_ownership[%d]", i),
			})
		}
	}

	// If ALL modify entries are missing, suspect a repo mismatch
	if modifyCount > 0 && missingCount == modifyCount {
		errs = append(errs, result.SAWError{
			Code:     "E16_REPO_MISMATCH_SUSPECTED",
			Message:  fmt.Sprintf("all %d action=modify files are missing - this IMPL may target a different repository", modifyCount),
			Severity: "error",
			Field:    "file_ownership",
		})
	}

	return errs
}
