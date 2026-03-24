package protocol

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
)

// ValidateMigrationBoundaries detects cross-wave migration boundaries where
// wave N modifies files in a directory and wave N+1 also has files in that
// same directory. Returns warnings (not errors) since the Scout may have
// intentionally used a bridge pattern.
//
// Only consecutive wave pairs are checked. Directory-level granularity is used
// to keep the check language-agnostic (no import parsing).
func ValidateMigrationBoundaries(m *IMPLManifest) []ValidationError {
	if m == nil {
		return nil
	}

	// Build map: wave number -> set of directories
	waveDirs := make(map[int]map[string]bool)
	for _, fo := range m.FileOwnership {
		dir := filepath.Dir(fo.File)
		if waveDirs[fo.Wave] == nil {
			waveDirs[fo.Wave] = make(map[string]bool)
		}
		waveDirs[fo.Wave][dir] = true
	}

	// Collect and sort wave numbers
	waves := make([]int, 0, len(waveDirs))
	for w := range waveDirs {
		waves = append(waves, w)
	}
	sort.Ints(waves)

	var warnings []ValidationError

	// Check consecutive wave pairs
	for i := 0; i < len(waves)-1; i++ {
		waveN := waves[i]
		waveN1 := waves[i+1]

		// Only check truly consecutive waves (N and N+1)
		if waveN1 != waveN+1 {
			continue
		}

		dirsN := waveDirs[waveN]
		dirsN1 := waveDirs[waveN1]

		// Find overlapping directories
		overlapping := make([]string, 0)
		for dir := range dirsN {
			if dirsN1[dir] {
				overlapping = append(overlapping, dir)
			}
		}
		sort.Strings(overlapping)

		for _, dir := range overlapping {
			warnings = append(warnings, ValidationError{
				Code:    "MIGRATION_BOUNDARY_WARNING",
				Message: fmt.Sprintf("Wave %d modifies files in %s, wave %d has callers — verify re-export bridge or consolidate into one wave", waveN, dir, waveN1),
				Field:   "file_ownership",
			})
		}
	}

	return warnings
}

// DiagnoseMigrationFailure inspects a failed BaselineData's gate output for
// type/import mismatch patterns (e.g., "cannot use X as Y", "undefined: X",
// "imported and not used"). Returns a human-readable suggestion string if a
// migration boundary issue is detected, or empty string if the failure is
// unrelated.
func DiagnoseMigrationFailure(result *BaselineData) string {
	if result == nil || result.Passed {
		return ""
	}

	migrationPatterns := []string{
		"cannot use",
		"undefined:",
		"imported and not used",
		"has no field or method",
		"not enough arguments",
		"too many arguments",
		"cannot convert",
		"incompatible type",
	}

	for _, gr := range result.GateResults {
		if gr.Passed {
			continue
		}
		output := gr.Stderr + "\n" + gr.Stdout
		for _, pattern := range migrationPatterns {
			if strings.Contains(output, pattern) {
				return "Baseline broken at wave boundary. Consider: (1) consolidate callers into the same wave as the signature change, or (2) add re-export aliases in the old package to forward to the new signatures."
			}
		}
	}

	return ""
}
