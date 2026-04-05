package scaffold

import (
	"regexp"
	"sort"
	"strings"
	"sync"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/protocol"
)

var typeDefCache sync.Map // map[string][]*regexp.Regexp

func getTypeDefPatterns(typeName string) []*regexp.Regexp {
	if cached, ok := typeDefCache.Load(typeName); ok {
		return cached.([]*regexp.Regexp)
	}
	quoted := regexp.QuoteMeta(typeName)
	templates := []string{
		`(?s)type\s+` + quoted + `\s+struct\s*\{[^}]*\}`,
		`(?s)type\s+` + quoted + `\s+interface\s*\{[^}]*\}`,
		`(?s)interface\s+` + quoted + `\s*\{[^}]*\}`,
		`(?s)class\s+` + quoted + `\s*\{[^}]*\}`,
	}
	patterns := make([]*regexp.Regexp, 0, 4)
	for _, tmpl := range templates {
		re, err := regexp.Compile(tmpl)
		if err != nil {
			continue // defense-in-depth: skip bad patterns
		}
		patterns = append(patterns, re)
	}
	typeDefCache.Store(typeName, patterns)
	return patterns
}

// PreAgentResult is the output of pre-agent scaffold detection.
type PreAgentResult struct {
	ScaffoldsNeeded []ScaffoldCandidate `json:"scaffolds_needed"`
}

// ScaffoldCandidate represents a type that should be extracted to a scaffold file.
type ScaffoldCandidate struct {
	TypeName      string   `json:"type_name"`
	Locations     []string `json:"locations"`      // File path locations referencing this type
	SuggestedFile string   `json:"suggested_file"` // internal/types/<name>.go
	Definition    string   `json:"definition"`     // Full type definition
}

// DetectScaffoldsPreAgent analyzes interface contracts to find types referenced by ≥2 agents.
// It extracts type names from contract definitions and identifies which should be scaffolds.
func DetectScaffoldsPreAgent(contracts []protocol.InterfaceContract) (*PreAgentResult, error) {
	// Build a map of type name -> set of contract locations that reference it
	typeReferences := make(map[string]map[string]bool)
	typeDefinitions := make(map[string]string)

	for _, contract := range contracts {
		// Extract all type names from this contract's definition
		matches := typeNameRe.FindAllStringSubmatch(contract.Definition, -1)

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
			// Extract unique locations
			locs := extractUniqueLocations(locations)

			scaffolds = append(scaffolds, ScaffoldCandidate{
				TypeName:      typeName,
				Locations:     locs,
				SuggestedFile: "internal/types/" + strings.ToLower(typeName) + ".go",
				Definition:    typeDefinitions[typeName],
			})
		}
	}

	sort.Slice(scaffolds, func(i, j int) bool {
		return scaffolds[i].TypeName < scaffolds[j].TypeName
	})

	return &PreAgentResult{
		ScaffoldsNeeded: scaffolds,
	}, nil
}

// extractTypeDefinition extracts the full definition of a type from a contract definition.
// It attempts to find the complete type definition including all fields.
func extractTypeDefinition(definition, typeName string) string {
	// Note: [^}]* patterns do not handle nested struct literals (embedded structs
	// with their own braces). If a type contains embedded struct literals, only the
	// outer brace is matched and extraction may be incomplete. This is a known
	// limitation; use the fallback placeholder for complex nested types.

	for _, pattern := range getTypeDefPatterns(typeName) {
		if match := pattern.FindString(definition); match != "" {
			return strings.TrimSpace(match)
		}
	}

	// Fallback: return a placeholder indicating we found the type but couldn't extract full definition
	return "type " + typeName + " struct { /* see contract definition */ }"
}

// extractUniqueLocations returns a sorted slice of unique location strings.
// Location format is typically "pkg/module/file.go" or similar.
func extractUniqueLocations(locations map[string]bool) []string {
	result := make([]string, 0, len(locations))
	for loc := range locations {
		result = append(result, loc)
	}
	sort.Strings(result)
	return result
}
