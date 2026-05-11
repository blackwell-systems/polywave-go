package protocol

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/blackwell-systems/polywave-go/pkg/result"
)

// IMPLSummary represents a lightweight summary of an IMPL document.
// It contains the essential metadata needed for listing and selection.
type IMPLSummary struct {
	Path        string `json:"path"`
	FeatureSlug string `json:"feature_slug"`
	Verdict     string `json:"verdict"`
	State       string `json:"state"`
	CurrentWave int    `json:"current_wave"`
	TotalWaves  int    `json:"total_waves"`
}

// ListIMPLsData contains the data of scanning a directory for IMPL documents.
type ListIMPLsData struct {
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
//   - ListIMPLsData with all successfully loaded summaries
//   - Empty list if dir is invalid or no IMPL files found (not an error)
//   - Results are sorted by path for deterministic output
// ListIMPLsOpts controls filtering behavior for ListIMPLs.
type ListIMPLsOpts struct {
	IncludeComplete bool // If false (default), exclude IMPLs with state COMPLETE or in complete/ directory
}

func ListIMPLs(ctx context.Context, dir string, opts ...ListIMPLsOpts) result.Result[*ListIMPLsData] {
	var o ListIMPLsOpts
	if len(opts) > 0 {
		o = opts[0]
	}
	data := &ListIMPLsData{
		IMPLs: []IMPLSummary{},
	}

	// Scan both active and complete directories
	dirs := []string{
		dir,
		filepath.Join(dir, "complete"),
	}

	var allMatches []string
	for _, scanDir := range dirs {
		// Check for cancellation before scanning each directory
		if err := ctx.Err(); err != nil {
			return result.NewFailure[*ListIMPLsData]([]result.PolywaveError{
				{Code: result.CodeContextCancelled, Message: "context cancelled during directory scan", Severity: "fatal"},
			})
		}

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
		// Check for cancellation before processing each file
		if err := ctx.Err(); err != nil {
			return result.NewFailure[*ListIMPLsData]([]result.PolywaveError{
				{Code: result.CodeContextCancelled, Message: "context cancelled during manifest processing", Severity: "fatal"},
			})
		}

		manifest, err := Load(ctx, path)
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
			State:       string(manifest.State),
			CurrentWave: currentWaveNum,
			TotalWaves:  len(manifest.Waves),
		}

		// Filter completed IMPLs unless requested
		if !o.IncludeComplete {
			if manifest.State == StateComplete || strings.Contains(path, "/complete/") {
				continue
			}
		}

		data.IMPLs = append(data.IMPLs, summary)
	}

	// Sort by path for deterministic output
	sort.Slice(data.IMPLs, func(i, j int) bool {
		return data.IMPLs[i].Path < data.IMPLs[j].Path
	})

	return result.NewSuccess(data)
}

// ArchiveData holds the result of an archive operation.
type ArchiveData struct {
	NewPath string `json:"new_path"`
}

// ArchiveIMPL moves an IMPL doc from docs/IMPL/ to docs/IMPL/complete/.
// Returns the new path if successful.
func ArchiveIMPL(ctx context.Context, manifestPath string) result.Result[ArchiveData] {
	if err := ctx.Err(); err != nil {
		return result.NewFailure[ArchiveData]([]result.PolywaveError{
			{Code: result.CodeContextCancelled, Message: "context cancelled", Severity: "fatal"},
		})
	}
	// Get directory and filename
	dir := filepath.Dir(manifestPath)
	filename := filepath.Base(manifestPath)

	// Check if already in complete directory
	if filepath.Base(dir) == "complete" {
		return result.NewSuccess(ArchiveData{NewPath: manifestPath}) // already archived
	}

	// Ensure complete directory exists
	completeDir := filepath.Join(dir, "complete")
	if err := os.MkdirAll(completeDir, 0755); err != nil {
		return result.NewFailure[ArchiveData]([]result.PolywaveError{
			{Code: "ARCHIVE_MKDIR_FAILED", Message: err.Error(), Severity: "fatal"},
		})
	}

	// Move file
	destPath := filepath.Join(completeDir, filename)
	if err := os.Rename(manifestPath, destPath); err != nil {
		return result.NewFailure[ArchiveData]([]result.PolywaveError{
			{Code: "ARCHIVE_RENAME_FAILED", Message: err.Error(), Severity: "fatal"},
		})
	}

	return result.NewSuccess(ArchiveData{NewPath: destPath})
}

// ArchiveProgram moves a PROGRAM manifest from docs/PROGRAM/ to docs/PROGRAM/complete/.
// Returns the new path. Idempotent — returns the existing path if already archived.
func ArchiveProgram(ctx context.Context, manifestPath string) result.Result[ArchiveData] {
	if err := ctx.Err(); err != nil {
		return result.NewFailure[ArchiveData]([]result.PolywaveError{
			{Code: result.CodeContextCancelled, Message: "context cancelled", Severity: "fatal"},
		})
	}
	dir := filepath.Dir(manifestPath)
	filename := filepath.Base(manifestPath)

	// Check if already in complete directory
	if filepath.Base(dir) == "complete" {
		return result.NewSuccess(ArchiveData{NewPath: manifestPath})
	}

	// Archive to docs/PROGRAM/complete/
	completeDir := filepath.Join(dir, "complete")
	if err := os.MkdirAll(completeDir, 0755); err != nil {
		return result.NewFailure[ArchiveData]([]result.PolywaveError{
			{Code: "ARCHIVE_MKDIR_FAILED", Message: err.Error(), Severity: "fatal"},
		})
	}

	destPath := filepath.Join(completeDir, filename)
	if err := os.Rename(manifestPath, destPath); err != nil {
		return result.NewFailure[ArchiveData]([]result.PolywaveError{
			{Code: "ARCHIVE_RENAME_FAILED", Message: err.Error(), Severity: "fatal"},
		})
	}

	return result.NewSuccess(ArchiveData{NewPath: destPath})
}
