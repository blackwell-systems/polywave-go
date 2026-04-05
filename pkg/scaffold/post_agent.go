package scaffold

import (
	"sort"
	"strings"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/protocol"
)

// PostAgentResult is the output of post-agent scaffold detection.
type PostAgentResult struct {
	Conflicts []TypeConflict `json:"conflicts"`
}

// TypeConflict represents a type defined by multiple agents.
type TypeConflict struct {
	TypeName   string   `json:"type_name"`
	Agents     []string `json:"agents"`      // Agent IDs that define it
	Files      []string `json:"files"`       // Files where it's defined
	Resolution string   `json:"resolution"`  // Suggested scaffold file
}

// DetectScaffoldsPostAgent parses agent task fields to detect duplicate type definitions.
// It searches for type definitions across all agent tasks and identifies conflicts where
// the same type name is defined by multiple agents.
func DetectScaffoldsPostAgent(manifest *protocol.IMPLManifest) (*PostAgentResult, error) {
	// Map: type name -> list of (agent ID, files)
	typeDefinitions := make(map[string]map[string][]string) // typeName -> agentID -> files

	// Parse all agent tasks
	for _, wave := range manifest.Waves {
		for _, agent := range wave.Agents {
			// Extract type definitions from task text
			typeNames := extractTypeNames(agent.Task)

			// Record each type with this agent's files
			for _, typeName := range typeNames {
				if typeDefinitions[typeName] == nil {
					typeDefinitions[typeName] = make(map[string][]string)
				}

				// Get files from agent's Files list (ownership)
				typeDefinitions[typeName][agent.ID] = append(typeDefinitions[typeName][agent.ID], agent.Files...)
			}
		}
	}

	// Find duplicates (types defined by ≥2 agents)
	conflicts := []TypeConflict{}
	for typeName, agentMap := range typeDefinitions {
		if len(agentMap) >= 2 {
			// Collect agent IDs and files
			var agents []string
			filesSet := make(map[string]bool)

			for agentID, agentFiles := range agentMap {
				agents = append(agents, agentID)
				for _, file := range agentFiles {
					filesSet[file] = true
				}
			}
			sort.Strings(agents)

			// Convert files set to sorted list
			var files []string
			for file := range filesSet {
				files = append(files, file)
			}
			sort.Strings(files)

			// Generate resolution suggestion
			resolution := "Extract to internal/types/" + strings.ToLower(typeName) + ".go"

			conflicts = append(conflicts, TypeConflict{
				TypeName:   typeName,
				Agents:     agents,
				Files:      files,
				Resolution: resolution,
			})
		}
	}
	sort.Slice(conflicts, func(i, j int) bool {
		return conflicts[i].TypeName < conflicts[j].TypeName
	})

	return &PostAgentResult{
		Conflicts: conflicts,
	}, nil
}

