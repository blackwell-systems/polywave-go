package scaffold

import (
	"regexp"
	"strings"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/protocol"
)

// PreAgentResult is the output of pre-agent scaffold detection.
type PreAgentResult struct {
	ScaffoldsNeeded []ScaffoldCandidate `json:"scaffolds_needed"`
}

// ScaffoldCandidate represents a type that should be extracted to a scaffold file.
type ScaffoldCandidate struct {
	TypeName      string   `json:"type_name"`
	ReferencedBy  []string `json:"referenced_by"`  // Agent IDs
	SuggestedFile string   `json:"suggested_file"` // internal/types/<name>.go
	Definition    string   `json:"definition"`     // Full type definition
}

// DetectScaffoldsPreAgent analyzes interface contracts to find types referenced by ≥2 agents.
// It extracts type names from contract definitions and identifies which should be scaffolds.
func DetectScaffoldsPreAgent(contracts []protocol.InterfaceContract) (*PreAgentResult, error) {
	// Build a map of type name -> set of contract locations that reference it
	typeReferences := make(map[string]map[string]bool)
	typeDefinitions := make(map[string]string)

	// Regex to extract type definitions from Go/Rust/JS/Python
	// Matches: "type TypeName struct", "type TypeName interface", "struct TypeName", "interface TypeName", "class TypeName", etc.
	typePattern := regexp.MustCompile(`(?m)^\s*(?:type|struct|interface|class|enum)\s+(\w+)\s+(?:struct|interface|enum|class|\{)`)

	for _, contract := range contracts {
		// Extract all type names from this contract's definition
		matches := typePattern.FindAllStringSubmatch(contract.Definition, -1)

		for _, match := range matches {
			if len(match) > 1 {
				typeName := match[1]

				// Initialize the reference set if this is the first time we see this type
				if typeReferences[typeName] == nil {
					typeReferences[typeName] = make(map[string]bool)
				}

				// Add this contract's location to the reference set
				typeReferences[typeName][contract.Location] = true

				// Store the full definition (we'll use the first occurrence)
				if typeDefinitions[typeName] == "" {
					// Extract the full type definition from the contract
					typeDefinitions[typeName] = extractTypeDefinition(contract.Definition, typeName)
				}
			}
		}
	}

	// Build the result - only types referenced by multiple locations (implying multiple agents)
	scaffolds := []ScaffoldCandidate{}
	for typeName, locations := range typeReferences {
		if len(locations) >= 2 {
			// Extract unique agent IDs from locations
			agents := extractAgentsFromLocations(locations)

			scaffolds = append(scaffolds, ScaffoldCandidate{
				TypeName:      typeName,
				ReferencedBy:  agents,
				SuggestedFile: "internal/types/" + strings.ToLower(typeName) + ".go",
				Definition:    typeDefinitions[typeName],
			})
		}
	}

	return &PreAgentResult{
		ScaffoldsNeeded: scaffolds,
	}, nil
}

// extractTypeDefinition extracts the full definition of a type from a contract definition.
// It attempts to find the complete type definition including all fields.
func extractTypeDefinition(definition, typeName string) string {
	// Try to extract the complete type definition
	// Look for "type TypeName struct { ... }" or similar patterns

	// Pattern to match the type declaration and its body
	patterns := []string{
		// Go struct/interface
		`(?s)type\s+` + typeName + `\s+struct\s*\{[^}]*\}`,
		`(?s)type\s+` + typeName + `\s+interface\s*\{[^}]*\}`,
		// Interface/class patterns for other languages
		`(?s)interface\s+` + typeName + `\s*\{[^}]*\}`,
		`(?s)class\s+` + typeName + `\s*\{[^}]*\}`,
	}

	for _, patternStr := range patterns {
		pattern := regexp.MustCompile(patternStr)
		if match := pattern.FindString(definition); match != "" {
			return strings.TrimSpace(match)
		}
	}

	// Fallback: return a placeholder indicating we found the type but couldn't extract full definition
	return "type " + typeName + " struct { /* see contract definition */ }"
}

// extractAgentsFromLocations extracts agent IDs from location strings.
// Location format is typically "pkg/module/file.go" or similar.
// For now, we deduplicate based on unique locations, as the actual agent mapping
// requires cross-referencing with file ownership, which is the caller's responsibility.
func extractAgentsFromLocations(locations map[string]bool) []string {
	agents := make([]string, 0, len(locations))
	for location := range locations {
		// For pre-agent analysis, locations represent different contracts,
		// which implies different agents will implement them.
		// We return location strings as proxy for agent IDs.
		// The actual agent mapping happens at CLI level by cross-referencing
		// with file_ownership table.
		agents = append(agents, location)
	}
	return agents
}
