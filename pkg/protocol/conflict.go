package protocol

import "context"

// OwnershipConflict represents a file that was modified by multiple agents in the same wave,
// or a file that was modified by an agent but not declared in the file ownership table.
type OwnershipConflict struct {
	File       string   `json:"file"`
	Agents     []string `json:"agents"`
	WaveNumber int      `json:"wave_number"`
	Repo       string   `json:"repo,omitempty"`
}

// DetectOwnershipConflicts cross-references completion reports to find files
// modified by multiple agents in the same wave. It also detects undeclared modifications
// (files touched by an agent but not listed in the file ownership table).
//
// The function checks:
// 1. Same-wave conflicts: multiple agents in the same wave modifying the same file
// 2. Cross-wave detection: same file in different waves is OK (no conflict)
// 3. Undeclared modifications: agent modified a file outside its ownership list
//
// Returns an empty slice if no conflicts are found.
func DetectOwnershipConflicts(ctx context.Context, manifest *IMPLManifest, reports map[string]CompletionReport) []OwnershipConflict {
	_ = ctx
	if manifest == nil || reports == nil {
		return nil
	}

	var conflicts []OwnershipConflict

	// Build ownership map from manifest for undeclared modification checks
	// Map structure: file → agent → wave number
	declaredOwnership := make(map[string]map[string]int)
	for _, fo := range manifest.FileOwnership {
		if declaredOwnership[fo.File] == nil {
			declaredOwnership[fo.File] = make(map[string]int)
		}
		declaredOwnership[fo.File][fo.Agent] = fo.Wave
	}

	// Process each wave independently
	for _, wave := range manifest.Waves {
		// Build map of file → []agentID for this wave
		fileToAgents := make(map[string][]string)
		fileToRepo := make(map[string]string)

		for _, agent := range wave.Agents {
			report, exists := reports[agent.ID]
			if !exists {
				continue
			}

			// Collect all files touched by this agent
			allFiles := append([]string{}, report.FilesChanged...)
			allFiles = append(allFiles, report.FilesCreated...)

			for _, file := range allFiles {
				fileToAgents[file] = append(fileToAgents[file], agent.ID)
				if report.Repo != "" {
					fileToRepo[file] = report.Repo
				}
			}

			// Check for undeclared modifications
			for _, file := range allFiles {
				owners, declared := declaredOwnership[file]
				if !declared {
					// File not in ownership table at all
					conflicts = append(conflicts, OwnershipConflict{
						File:       file,
						Agents:     []string{agent.ID},
						WaveNumber: wave.Number,
						Repo:       report.Repo,
					})
				} else if _, agentIsDeclaredOwner := owners[agent.ID]; !agentIsDeclaredOwner {
					// File exists in ownership table but this agent is not the declared owner
					// Find the declared owner for this wave
					var declaredOwner string
					for owner, waveNum := range owners {
						if waveNum == wave.Number {
							declaredOwner = owner
							break
						}
					}
					if declaredOwner != "" {
						// Report as conflict: [declaredOwner, violatingAgent]
						conflicts = append(conflicts, OwnershipConflict{
							File:       file,
							Agents:     []string{declaredOwner, agent.ID},
							WaveNumber: wave.Number,
							Repo:       report.Repo,
						})
					}
				}
			}
		}

		// Detect same-wave conflicts (2+ agents modifying same file)
		for file, agents := range fileToAgents {
			if len(agents) > 1 {
				conflicts = append(conflicts, OwnershipConflict{
					File:       file,
					Agents:     agents,
					WaveNumber: wave.Number,
					Repo:       fileToRepo[file],
				})
			}
		}
	}

	return conflicts
}
