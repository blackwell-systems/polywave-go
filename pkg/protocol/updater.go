package protocol

import (
	"fmt"
	"os"
	"strings"
)

// UpdateIMPLStatus reads the IMPL doc at path, ticks the Status table
// checkboxes for all agents whose letter appears in completedAgents,
// and writes the result back to path.
//
// It is idempotent: rows already showing "DONE" are left unchanged.
// Returns nil if no Status table section is found (non-fatal).
// Returns an error if the file cannot be read or written.
//
// Expected Status table row format:
//
//	| <wave> | <letter> | <description> | TO-DO |
//
// After update:
//
//	| <wave> | <letter> | <description> | DONE  |
//
// The function matches rows where:
//   - The row contains "| TO-DO |" (case-sensitive)
//   - The agent letter cell matches one of completedAgents (exact, single char)
func UpdateIMPLStatus(path string, completedAgents []string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("UpdateIMPLStatus: cannot read %q: %w", path, err)
	}

	updated := UpdateIMPLStatusBytes(data, completedAgents)

	if err := os.WriteFile(path, updated, 0644); err != nil {
		return fmt.Errorf("UpdateIMPLStatus: cannot write %q: %w", path, err)
	}

	return nil
}

// UpdateIMPLStatusBytes is the pure functional core of UpdateIMPLStatus.
// It takes file bytes and returns updated bytes. Used in tests to avoid file I/O.
func UpdateIMPLStatusBytes(content []byte, completedAgents []string) []byte {
	// Build a set for O(1) lookup.
	agentSet := make(map[string]bool, len(completedAgents))
	for _, a := range completedAgents {
		agentSet[a] = true
	}

	lines := strings.Split(string(content), "\n")

	inStatusSection := false

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Detect section boundaries by ### headers.
		if strings.HasPrefix(trimmed, "### ") {
			if trimmed == "### Status" {
				inStatusSection = true
			} else {
				inStatusSection = false
			}
			continue
		}

		// Only process table rows inside the Status section.
		if !inStatusSection {
			continue
		}

		// Only process rows containing "| TO-DO |".
		if !strings.Contains(line, "| TO-DO |") {
			continue
		}

		// Parse the agent letter: second pipe-delimited column (index 2 in split).
		// Example: "| 1 | A | Multi-wave loop in runWave | TO-DO |"
		// Split gives: ["", " 1 ", " A ", " Multi-wave loop in runWave ", " TO-DO ", ""]
		parts := strings.Split(line, "|")
		if len(parts) < 4 {
			continue
		}
		agentLetter := strings.TrimSpace(parts[2])

		if agentSet[agentLetter] {
			// Replace "| TO-DO |" with "| DONE  |" preserving column alignment.
			// "TO-DO" is 5 chars, "DONE " is 5 chars.
			lines[i] = strings.Replace(line, "| TO-DO |", "| DONE  |", 1)
		}
	}

	return []byte(strings.Join(lines, "\n"))
}

// UpdateAgentPrompt updates the task/prompt text for a specific agent in a YAML manifest.
// Returns ErrAgentNotFound if the agent ID doesn't exist in any wave.
func UpdateAgentPrompt(m *IMPLManifest, agentID string, newPrompt string) error {
	if agentID == "" {
		return fmt.Errorf("agent ID cannot be empty")
	}

	// Search all waves for the agent with matching ID
	for i := range m.Waves {
		for j := range m.Waves[i].Agents {
			if m.Waves[i].Agents[j].ID == agentID {
				// Update the agent's Task field with newPrompt
				m.Waves[i].Agents[j].Task = newPrompt
				return nil
			}
		}
	}

	// Agent not found in any wave
	return fmt.Errorf("%w: %s", ErrAgentNotFound, agentID)
}
