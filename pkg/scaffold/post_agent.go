package scaffold

import (
	"regexp"
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
			typeNames := extractTypeDefinitions(agent.Task)

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
	var conflicts []TypeConflict
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

			// Convert files set to sorted list
			var files []string
			for file := range filesSet {
				files = append(files, file)
			}

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

	return &PostAgentResult{
		Conflicts: conflicts,
	}, nil
}

// extractTypeDefinitions searches for type definitions in agent task text.
// It matches patterns like:
//   - type Name struct
//   - interface Name
//   - enum Name
//   - class Name
func extractTypeDefinitions(taskText string) []string {
	var typeNames []string
	seen := make(map[string]bool)

	// Pattern 1: Go-style type definitions (type Name struct|interface)
	goTypeRe := regexp.MustCompile(`type\s+(\w+)\s+(struct|interface)`)
	matches := goTypeRe.FindAllStringSubmatch(taskText, -1)
	for _, match := range matches {
		if len(match) > 1 {
			typeName := match[1]
			if !seen[typeName] {
				typeNames = append(typeNames, typeName)
				seen[typeName] = true
			}
		}
	}

	// Pattern 2: Interface definitions (interface Name)
	interfaceRe := regexp.MustCompile(`interface\s+(\w+)`)
	matches = interfaceRe.FindAllStringSubmatch(taskText, -1)
	for _, match := range matches {
		if len(match) > 1 {
			typeName := match[1]
			if !seen[typeName] {
				typeNames = append(typeNames, typeName)
				seen[typeName] = true
			}
		}
	}

	// Pattern 3: Enum definitions (enum Name)
	enumRe := regexp.MustCompile(`enum\s+(\w+)`)
	matches = enumRe.FindAllStringSubmatch(taskText, -1)
	for _, match := range matches {
		if len(match) > 1 {
			typeName := match[1]
			if !seen[typeName] {
				typeNames = append(typeNames, typeName)
				seen[typeName] = true
			}
		}
	}

	// Pattern 4: Class definitions (class Name)
	classRe := regexp.MustCompile(`class\s+(\w+)`)
	matches = classRe.FindAllStringSubmatch(taskText, -1)
	for _, match := range matches {
		if len(match) > 1 {
			typeName := match[1]
			if !seen[typeName] {
				typeNames = append(typeNames, typeName)
				seen[typeName] = true
			}
		}
	}

	return typeNames
}
