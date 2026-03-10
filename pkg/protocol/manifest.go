package protocol

import (
	"errors"
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// ErrAgentNotFound is returned when SetCompletionReport is called with an unknown agent ID.
var ErrAgentNotFound = errors.New("agent not found in manifest")

// Load reads a YAML IMPL manifest from the specified path and parses it into an IMPLManifest.
// Returns an error if the file cannot be read or the YAML is invalid.
func Load(path string) (*IMPLManifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read manifest file: %w", err)
	}

	var manifest IMPLManifest
	if err := yaml.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("failed to parse manifest YAML: %w", err)
	}

	// Initialize maps if nil
	if manifest.CompletionReports == nil {
		manifest.CompletionReports = make(map[string]CompletionReport)
	}

	return &manifest, nil
}

// Save writes an IMPLManifest to the specified path as YAML.
// Returns an error if the file cannot be written or the manifest cannot be marshaled.
func Save(m *IMPLManifest, path string) error {
	data, err := yaml.Marshal(m)
	if err != nil {
		return fmt.Errorf("failed to marshal manifest to YAML: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write manifest file: %w", err)
	}

	return nil
}

// CurrentWave returns the first wave that has incomplete agents (agents without completion reports).
// If all waves are complete, returns nil.
// An agent is considered incomplete if its completion report is missing or has status != "complete".
func CurrentWave(m *IMPLManifest) *Wave {
	if m.CompletionReports == nil {
		m.CompletionReports = make(map[string]CompletionReport)
	}

	for i := range m.Waves {
		wave := &m.Waves[i]
		for _, agent := range wave.Agents {
			report, exists := m.CompletionReports[agent.ID]
			if !exists || report.Status != "complete" {
				return wave
			}
		}
	}

	return nil
}

// SetCompletionReport registers a completion report for the specified agent.
// Returns an error if the agent ID is not found in any wave or if the agent ID is empty.
func SetCompletionReport(m *IMPLManifest, agentID string, report CompletionReport) error {
	if agentID == "" {
		return fmt.Errorf("agent ID cannot be empty")
	}

	// Verify agent exists in manifest
	found := false
	for _, wave := range m.Waves {
		for _, agent := range wave.Agents {
			if agent.ID == agentID {
				found = true
				break
			}
		}
		if found {
			break
		}
	}

	if !found {
		return fmt.Errorf("%w: %s", ErrAgentNotFound, agentID)
	}

	// Initialize map if nil
	if m.CompletionReports == nil {
		m.CompletionReports = make(map[string]CompletionReport)
	}

	// Store report
	m.CompletionReports[agentID] = report

	return nil
}
