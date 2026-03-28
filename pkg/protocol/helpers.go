package protocol

import (
	"os"
	"path/filepath"
)

// IsSoloWave returns true if the wave has exactly 1 agent.
// Returns false if the wave is nil or has 0 or 2+ agents.
func IsSoloWave(wave *Wave) bool {
	return wave != nil && len(wave.Agents) == 1
}

// IsWaveComplete returns true if all agents in the wave have status "complete".
// Returns false if any agent is missing a completion report or has a status other than "complete".
func IsWaveComplete(wave *Wave, reports map[string]CompletionReport) bool {
	if wave == nil {
		return false
	}

	for _, agent := range wave.Agents {
		report, exists := reports[agent.ID]
		if !exists || report.Status != StatusComplete {
			return false
		}
	}

	return true
}

// IsFinalWave returns true if this is the last wave in the manifest.
// Waves are 1-indexed, so the final wave number equals the total wave count.
// Returns false if waveNumber is out of bounds or manifest has no waves.
func IsFinalWave(manifest *IMPLManifest, waveNumber int) bool {
	if manifest == nil || len(manifest.Waves) == 0 {
		return false
	}
	return waveNumber == len(manifest.Waves)
}

// FindWave returns a pointer to the Wave whose Number equals waveNum,
// or nil if no such wave exists.
func (m *IMPLManifest) FindWave(waveNum int) *Wave {
	for i := range m.Waves {
		if m.Waves[i].Number == waveNum {
			return &m.Waves[i]
		}
	}
	return nil
}

// FindAgent returns a pointer to the Agent whose ID equals agentID
// within the wave, or nil if not found.
func (w *Wave) FindAgent(agentID string) *Agent {
	for i := range w.Agents {
		if w.Agents[i].ID == agentID {
			return &w.Agents[i]
		}
	}
	return nil
}

// FindWaveWithAgent returns the Wave containing agentID and a pointer
// to the Agent, or (nil, nil) if not found.
func (m *IMPLManifest) FindWaveWithAgent(agentID string) (*Wave, *Agent) {
	for i := range m.Waves {
		for j := range m.Waves[i].Agents {
			if m.Waves[i].Agents[j].ID == agentID {
				return &m.Waves[i], &m.Waves[i].Agents[j]
			}
		}
	}
	return nil, nil
}

// TargetsRepo returns true if this IMPL targets the given repository.
// Used by pre-commit-check to skip cross-repo IMPLs.
//
// Logic:
// 1. If IMPL has explicit Repository field, check if it matches repoDir path
// 2. If IMPL has Repositories list, check if any path matches repoDir
// 3. Otherwise check FileOwnership:
//    - Returns true if ANY FileOwnership entry has Repo == "" (same-repo default)
//      OR Repo == extracted repo name
//    - Returns true if FileOwnership is empty (no file constraints)
//    - Returns false if ALL FileOwnership entries specify different repos
func (m *IMPLManifest) TargetsRepo(repoDir string) bool {
	// Normalize repoDir to absolute path for comparison
	absRepoDir, err := filepath.Abs(repoDir)
	if err != nil {
		absRepoDir = repoDir
	}

	// Check explicit Repository field (single-repo IMPL)
	if m.Repository != "" {
		absImplRepo, err := filepath.Abs(m.Repository)
		if err != nil {
			absImplRepo = m.Repository
		}
		return absImplRepo == absRepoDir
	}

	// Check Repositories list (multi-repo IMPL)
	if len(m.Repositories) > 0 {
		for _, repo := range m.Repositories {
			absImplRepo, err := filepath.Abs(repo)
			if err != nil {
				absImplRepo = repo
			}
			if absImplRepo == absRepoDir {
				return true
			}
		}
		return false
	}

	// Fallback: Check FileOwnership repo field
	if len(m.FileOwnership) == 0 {
		return true // No file ownership = no repo restriction
	}

	// Extract repo name from path for FileOwnership.Repo matching
	repoName := filepath.Base(absRepoDir)

	// Track if we found any matches and if all entries have empty Repo fields
	hasMatch := false
	allEmptyRepoFields := true

	for _, fo := range m.FileOwnership {
		if fo.Repo != "" {
			allEmptyRepoFields = false
			if fo.Repo == repoName {
				hasMatch = true
			}
		} else {
			// Empty Repo field - check if file exists in current repo
			filePath := filepath.Join(absRepoDir, fo.File)
			if _, err := os.Stat(filePath); err == nil {
				hasMatch = true
			}
		}
	}

	// If all Repo fields are empty and no files exist in current repo,
	// this is likely a cross-repo IMPL stored in the wrong location
	if allEmptyRepoFields && !hasMatch {
		return false
	}

	return hasMatch
}
