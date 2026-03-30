package analyzer

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/protocol"
)

// DetectWiring analyzes an IMPL manifest's agent task prompts for cross-agent
// function calls and returns WiringDeclaration entries. Patterns detected:
// - "calls `FunctionName()`"
// - "uses `pkg.FunctionName`"
// - "delegates to `X`"
// - "invokes `FunctionName`"
// Cross-references function names against file_ownership to find defining agent.
// Only emits wiring declarations when caller agent ≠ definer agent.
func DetectWiring(manifest *protocol.IMPLManifest, repoRoot string) ([]protocol.WiringDeclaration, error) {
	if manifest == nil {
		return nil, fmt.Errorf("manifest is nil")
	}

	if len(manifest.FileOwnership) == 0 {
		return nil, fmt.Errorf("manifest file_ownership is empty")
	}

	// Build a map of function names to their definitions from interface contracts
	contractMap := make(map[string]*protocol.InterfaceContract)
	for i := range manifest.InterfaceContracts {
		contract := &manifest.InterfaceContracts[i]
		// Extract function name from contract.Name
		// Name might be "FunctionName" or "pkg.FunctionName"
		funcName := extractFunctionName(contract.Name)
		contractMap[funcName] = contract
	}

	// Compile regex patterns for function call detection
	// Pattern 1: "calls `FunctionName()`"
	// Pattern 2: "uses `pkg.FunctionName`" or "uses `FunctionName()`"
	// Pattern 3: "delegates to `FunctionName`"
	// Pattern 4: "invokes `FunctionName`"
	// All patterns allow optional () at the end
	patterns := []*regexp.Regexp{
		regexp.MustCompile(`calls\s+` + "`" + `([A-Za-z0-9_.]+)(?:\(\))?` + "`"),
		regexp.MustCompile(`uses\s+` + "`" + `([A-Za-z0-9_.]+)(?:\(\))?` + "`"),
		regexp.MustCompile(`delegates\s+to\s+` + "`" + `([A-Za-z0-9_.]+)(?:\(\))?` + "`"),
		regexp.MustCompile(`invokes\s+` + "`" + `([A-Za-z0-9_.]+)(?:\(\))?` + "`"),
	}

	var declarations []protocol.WiringDeclaration

	// Iterate through all waves and agents
	for _, wave := range manifest.Waves {
		for _, agent := range wave.Agents {
			// Find function calls in this agent's task
			functionCalls := make(map[string]bool) // deduplicate within agent
			for _, pattern := range patterns {
				matches := pattern.FindAllStringSubmatch(agent.Task, -1)
				for _, match := range matches {
					if len(match) > 1 {
						rawName := match[1]
						funcName := extractFunctionName(rawName)
						functionCalls[funcName] = true
					}
				}
			}

			// For each function call, check if it's cross-agent
			for funcName := range functionCalls {
				// Look up the defining agent/file
				definedIn, definingAgent, _, found := findDefiningAgent(funcName, contractMap, manifest.FileOwnership, agent.ID)

				if !found {
					// Function not in contracts and not in file ownership
					// Likely stdlib or external function - skip
					continue
				}

				// Check if it's a cross-agent call
				if definingAgent == agent.ID {
					// Same agent calling its own function - no wiring needed
					continue
				}

				// Get the calling agent's first file
				callerFile := getAgentFirstFile(agent.ID, wave.Number, manifest.FileOwnership)
				if callerFile == "" {
					// Agent has no files (shouldn't happen)
					continue
				}

				// Emit wiring declaration
				declarations = append(declarations, protocol.WiringDeclaration{
					Symbol:             funcName,
					DefinedIn:          definedIn,
					MustBeCalledFrom:   callerFile,
					Agent:              agent.ID,
					Wave:               wave.Number,
					IntegrationPattern: "call",
				})
			}
		}
	}

	return declarations, nil
}

// extractFunctionName strips package prefixes, backticks, and trailing parens
// from a function reference to get the bare function name.
// Examples:
//   - "pkg.FunctionName" -> "FunctionName"
//   - "FunctionName()" -> "FunctionName"
//   - "FunctionName" -> "FunctionName"
func extractFunctionName(raw string) string {
	// Remove backticks
	name := strings.ReplaceAll(raw, "`", "")
	// Remove trailing parens
	name = strings.TrimSuffix(name, "()")
	// Strip package prefix (take last component after .)
	parts := strings.Split(name, ".")
	if len(parts) > 1 {
		return parts[len(parts)-1]
	}
	return name
}

// findDefiningAgent searches for the file that defines a given function.
// It first checks interface_contracts, then falls back to file_ownership heuristic.
// Returns (definedIn file path, defining agent ID, defining wave number, found bool).
func findDefiningAgent(funcName string, contractMap map[string]*protocol.InterfaceContract, fileOwnership []protocol.FileOwnership, callerAgentID string) (string, string, int, bool) {
	// Check interface contracts first
	if contract, ok := contractMap[funcName]; ok {
		if contract.Location != "" {
			// Find the agent that owns this location
			for _, fo := range fileOwnership {
				if fo.File == contract.Location {
					return contract.Location, fo.Agent, fo.Wave, true
				}
			}
			// Contract has location but no matching ownership entry
			// Use location anyway, leave agent empty (will fail validation later)
			return contract.Location, "", 0, true
		}
	}

	// Fallback heuristic: assume function is in the first file owned by
	// the earliest-wave agent that is NOT the calling agent
	var earliestAgent string
	var earliestWave int = 99999
	var earliestFile string

	for _, fo := range fileOwnership {
		if fo.Agent == callerAgentID {
			continue // skip caller's own files
		}
		if fo.Wave < earliestWave {
			earliestWave = fo.Wave
			earliestAgent = fo.Agent
			earliestFile = fo.File
		}
	}

	if earliestAgent != "" {
		return earliestFile, earliestAgent, earliestWave, true
	}

	return "", "", 0, false
}

// getAgentFirstFile returns the first file owned by the given agent in the given wave.
func getAgentFirstFile(agentID string, waveNum int, fileOwnership []protocol.FileOwnership) string {
	for _, fo := range fileOwnership {
		if fo.Agent == agentID && fo.Wave == waveNum {
			return fo.File
		}
	}
	return ""
}
