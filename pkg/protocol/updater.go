package protocol

import (
	"fmt"
	"os"
	"strings"

	"github.com/blackwell-systems/polywave-go/pkg/result"
)

// IMPLStatusData holds the result of an UpdateIMPLStatus operation.
type IMPLStatusData struct {
	UpdatedPath     string
	CompletedAgents []string
	NewState        string
}

// UpdatePromptData holds the result of an UpdateAgentPrompt operation.
type UpdatePromptData struct {
	AgentID       string
	PromptUpdated bool
}

// UpdateIMPLStatus reads the IMPL doc at path, ticks the Status table
// checkboxes for all agents whose letter appears in completedAgents,
// and writes the result back to path.
//
// It is idempotent: rows already showing "DONE" are left unchanged.
// Returns Fatal if the file cannot be read or written.
// Returns Success with UpdateStatusData on success (including when no Status section is found).
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
func UpdateIMPLStatus(path string, completedAgents []string) result.Result[IMPLStatusData] {
	data, err := os.ReadFile(path)
	if err != nil {
		return result.NewFailure[IMPLStatusData]([]result.PolywaveError{
			result.NewFatal(result.CodeStatusUpdateFailed,
				fmt.Sprintf("UpdateIMPLStatus: cannot read %q: %v", path, err)),
		})
	}

	updated := UpdateIMPLStatusBytes(data, completedAgents)

	if err := os.WriteFile(path, updated, 0644); err != nil {
		return result.NewFailure[IMPLStatusData]([]result.PolywaveError{
			result.NewFatal(result.CodeStatusUpdateFailed,
				fmt.Sprintf("UpdateIMPLStatus: cannot write %q: %v", path, err)),
		})
	}

	return result.NewSuccess(IMPLStatusData{
		UpdatedPath:     path,
		CompletedAgents: completedAgents,
		NewState:        "DONE",
	})
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
// Returns Fatal wrapping ErrAgentNotFound if the agent ID doesn't exist in any wave.
func UpdateAgentPrompt(m *IMPLManifest, agentID string, newPrompt string) result.Result[UpdatePromptData] {
	if agentID == "" {
		return result.NewFailure[UpdatePromptData]([]result.PolywaveError{
			result.NewFatal("PROMPT_UPDATE_FAILED", "agent ID cannot be empty"),
		})
	}

	// Search all waves for the agent with matching ID
	for i := range m.Waves {
		for j := range m.Waves[i].Agents {
			if m.Waves[i].Agents[j].ID == agentID {
				// Update the agent's Task field with newPrompt
				m.Waves[i].Agents[j].Task = newPrompt
				return result.NewSuccess(UpdatePromptData{
					AgentID:       agentID,
					PromptUpdated: true,
				})
			}
		}
	}

	// Agent not found in any wave
	return result.NewFailure[UpdatePromptData]([]result.PolywaveError{
		result.NewFatal("PROMPT_UPDATE_FAILED",
			fmt.Sprintf("%s: %s", ErrAgentNotFound, agentID)),
	})
}
