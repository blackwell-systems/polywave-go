package protocol

import "fmt"

// OwnershipCoverageResult is the outcome of checking that every file an agent
// reported changing is contained within its declared file ownership table.
type OwnershipCoverageResult struct {
	Valid      bool                 `json:"valid"`
	Violations []OwnershipViolation `json:"violations,omitempty"`
}

// OwnershipViolation records a single agent's set of files that were reported
// changed but are absent from the file ownership table.
type OwnershipViolation struct {
	Agent        string   `json:"agent"`
	UnownedFiles []string `json:"unowned_files"`
}

// ValidateFileOwnershipCoverage verifies that every file reported as changed
// or created by an agent in the given wave is a subset of that agent's owned
// files (from the file_ownership table) or a frozen scaffold path.
//
// A file reported in FilesChanged or FilesCreated that does not appear in the
// agent's owned files and is not a scaffold is recorded as a violation.
//
// Returns an error only for system-level failures (e.g. wave not found).
func ValidateFileOwnershipCoverage(
	manifest *IMPLManifest,
	waveNum int,
) (*OwnershipCoverageResult, error) {
	// Find the target wave.
	var targetWave *Wave
	for i := range manifest.Waves {
		if manifest.Waves[i].Number == waveNum {
			targetWave = &manifest.Waves[i]
			break
		}
	}
	if targetWave == nil {
		return nil, fmt.Errorf("wave %d not found in manifest", waveNum)
	}

	// Build owned-files set per agent.
	agentOwned := make(map[string]map[string]bool) // agentID -> set of owned file paths
	for _, fo := range manifest.FileOwnership {
		if fo.Wave == waveNum {
			if agentOwned[fo.Agent] == nil {
				agentOwned[fo.Agent] = make(map[string]bool)
			}
			agentOwned[fo.Agent][fo.File] = true
		}
	}

	// Frozen scaffold paths — allowed for any agent.
	frozenPaths := make(map[string]bool)
	for _, sc := range manifest.Scaffolds {
		if sc.FilePath != "" {
			frozenPaths[sc.FilePath] = true
		}
	}

	result := &OwnershipCoverageResult{Valid: true}

	for _, agent := range targetWave.Agents {
		report, hasReport := manifest.CompletionReports[agent.ID]
		if !hasReport {
			// No completion report yet — nothing to validate.
			continue
		}

		owned := agentOwned[agent.ID] // may be nil if agent has no file_ownership entries
		var unowned []string

		check := func(files []string) {
			for _, f := range files {
				if frozenPaths[f] {
					continue
				}
				if owned != nil && owned[f] {
					continue
				}
				unowned = append(unowned, f)
			}
		}

		check(report.FilesChanged)
		check(report.FilesCreated)

		if len(unowned) > 0 {
			result.Valid = false
			result.Violations = append(result.Violations, OwnershipViolation{
				Agent:        agent.ID,
				UnownedFiles: unowned,
			})
		}
	}

	return result, nil
}
