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
