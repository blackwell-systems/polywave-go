package protocol

import (
	"path/filepath"
	"sort"
)

// ProgramDiscovery represents a lightweight summary of a PROGRAM manifest.
// It contains the essential metadata needed for listing and selection.
type ProgramDiscovery struct {
	Path  string       `json:"path"`
	Slug  string       `json:"slug"`
	State ProgramState `json:"state"`
	Title string       `json:"title"`
}

// ListPrograms scans the specified directory for PROGRAM manifest files and returns
// a summary of each valid PROGRAM document found.
//
// It searches for files matching the pattern:
//   - PROGRAM-*.yaml
//
// For each manifest file found:
//   - Attempts to parse via ParseProgramManifest(path)
//   - If parse fails, skips the file
//   - Builds ProgramDiscovery with path, slug, state, title
//
// Returns:
//   - Slice of ProgramDiscovery with all successfully loaded summaries
//   - Empty slice if dir is invalid or no PROGRAM files found (not an error)
//   - Results are sorted by filename for deterministic output
//
// Note: This function does NOT call ValidateProgram. Discovery is lightweight
// and does not perform full validation.
func ListPrograms(dir string) ([]ProgramDiscovery, error) {
	result := []ProgramDiscovery{}

	// Find all PROGRAM-*.yaml files in dir/PROGRAM/ and dir/PROGRAM/complete/
	pattern := filepath.Join(dir, "PROGRAM", "PROGRAM-*.yaml")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		// Invalid dir or glob pattern — return empty slice
		return result, nil
	}

	// Also scan the archive directory (docs/PROGRAM/complete/)
	archivePattern := filepath.Join(dir, "PROGRAM", "complete", "PROGRAM-*.yaml")
	archiveMatches, _ := filepath.Glob(archivePattern)
	matches = append(matches, archiveMatches...)

	// Process each manifest file
	for _, path := range matches {
		manifest, err := ParseProgramManifest(path)
		if err != nil {
			// Skip files that fail to parse
			continue
		}

		// Build discovery summary
		discovery := ProgramDiscovery{
			Path:  path,
			Slug:  manifest.ProgramSlug,
			State: manifest.State,
			Title: manifest.Title,
		}

		result = append(result, discovery)
	}

	// Sort by filename for deterministic output
	sort.Slice(result, func(i, j int) bool {
		return filepath.Base(result[i].Path) < filepath.Base(result[j].Path)
	})

	return result, nil
}
