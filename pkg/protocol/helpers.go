package protocol

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
		if !exists || report.Status != "complete" {
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
