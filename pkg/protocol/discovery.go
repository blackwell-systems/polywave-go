package protocol

import (
	"os"
	"path/filepath"
	"sort"
)

// IMPLSummary represents a lightweight summary of an IMPL document.
// It contains the essential metadata needed for listing and selection.
type IMPLSummary struct {
	Path        string `json:"path"`
	FeatureSlug string `json:"feature_slug"`
	Verdict     string `json:"verdict"`
	CurrentWave int    `json:"current_wave"`
	TotalWaves  int    `json:"total_waves"`
}

// ListIMPLsResult contains the results of scanning a directory for IMPL documents.
type ListIMPLsResult struct {
	IMPLs []IMPLSummary `json:"impls"`
}

// ListIMPLs scans the specified directory for IMPL manifest files and returns
// a summary of each valid IMPL document found.
//
// It searches for files matching the patterns:
//   - IMPL-*.yaml
//   - IMPL-*.yml
//
// For each manifest file found:
//   - Attempts to load via Load(path)
//   - If load fails, skips the file
//   - Computes CurrentWave using CurrentWave(manifest)
//   - If CurrentWave returns nil (all waves complete), uses len(Waves)
//   - Builds IMPLSummary with path, feature_slug, verdict, current_wave, total_waves
//
// Returns:
//   - ListIMPLsResult with all successfully loaded summaries
//   - Empty list if dir is invalid or no IMPL files found (not an error)
//   - Results are sorted by path for deterministic output
func ListIMPLs(dir string) (*ListIMPLsResult, error) {
	result := &ListIMPLsResult{
		IMPLs: []IMPLSummary{},
	}

	// Scan both active and complete directories
	dirs := []string{
		dir,
		filepath.Join(dir, "complete"),
	}

	var allMatches []string
	for _, scanDir := range dirs {
		// Find all IMPL-*.yaml and IMPL-*.yml files
		yamlPattern := filepath.Join(scanDir, "IMPL-*.yaml")
		ymlPattern := filepath.Join(scanDir, "IMPL-*.yml")

		yamlMatches, err := filepath.Glob(yamlPattern)
		if err != nil {
			// Invalid dir or glob pattern — skip this directory
			continue
		}

		ymlMatches, err := filepath.Glob(ymlPattern)
		if err != nil {
			// Invalid dir or glob pattern — skip this directory
			continue
		}

		// Combine matches from this directory
		allMatches = append(allMatches, yamlMatches...)
		allMatches = append(allMatches, ymlMatches...)
	}

	// Process each manifest file
	for _, path := range allMatches {
		manifest, err := Load(path)
		if err != nil {
			// Skip files that fail to load
			continue
		}

		// Compute current wave number
		currentWave := CurrentWave(manifest)
		currentWaveNum := len(manifest.Waves)
		if currentWave != nil {
			currentWaveNum = currentWave.Number
		}

		// Build summary
		summary := IMPLSummary{
			Path:        path,
			FeatureSlug: manifest.FeatureSlug,
			Verdict:     manifest.Verdict,
			CurrentWave: currentWaveNum,
			TotalWaves:  len(manifest.Waves),
		}

		result.IMPLs = append(result.IMPLs, summary)
	}

	// Sort by path for deterministic output
	sort.Slice(result.IMPLs, func(i, j int) bool {
		return result.IMPLs[i].Path < result.IMPLs[j].Path
	})

	return result, nil
}

// ArchiveIMPL moves an IMPL doc from docs/IMPL/ to docs/IMPL/complete/.
// Returns the new path if successful.
func ArchiveIMPL(manifestPath string) (string, error) {
	// Get directory and filename
	dir := filepath.Dir(manifestPath)
	filename := filepath.Base(manifestPath)

	// Check if already in complete directory
	if filepath.Base(dir) == "complete" {
		return manifestPath, nil // already archived
	}

	// Ensure complete directory exists
	completeDir := filepath.Join(dir, "complete")
	if err := os.MkdirAll(completeDir, 0755); err != nil {
		return "", err
	}

	// Move file
	destPath := filepath.Join(completeDir, filename)
	if err := os.Rename(manifestPath, destPath); err != nil {
		return "", err
	}

	return destPath, nil
}
