package analyzer

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/protocol"
)

// SharedTypeCandidate represents a type that should be scaffolded because
// multiple agents reference it.
type SharedTypeCandidate struct {
	TypeName          string   `json:"type_name" yaml:"type_name"`
	DefiningAgent     string   `json:"defining_agent" yaml:"defining_agent"`
	DefiningFile      string   `json:"defining_file" yaml:"defining_file"`
	ReferencingAgents []string `json:"referencing_agents" yaml:"referencing_agents"`
	ReferencingFiles  []string `json:"referencing_files" yaml:"referencing_files"`
	Reason            string   `json:"reason" yaml:"reason"`
}

// DetectSharedTypes scans an IMPL doc's file_ownership, interface_contracts,
// and agent task prompts to find types that multiple agents reference.
// Returns scaffold candidates with metadata for Scout to review.
//
// Detection heuristics:
// 1. Agent A owns file X, Agent B's task says "import Type from X"
// 2. Type appears in interface_contracts AND 2+ agents reference it in tasks
// 3. Same struct name mentioned in multiple agents' "Interfaces to implement"
//
// Does NOT trigger for:
// - Types imported from external packages (stdlib, third-party deps)
// - Types in files not owned by any agent (existing codebase infrastructure)
// - Types mentioned in only one agent's task
func DetectSharedTypes(manifest *protocol.IMPLManifest, repoRoot string) ([]SharedTypeCandidate, error) {
	if manifest == nil {
		return []SharedTypeCandidate{}, fmt.Errorf("manifest cannot be nil")
	}

	// Build a map of file path -> (agent, wave) ownership
	fileOwnershipMap := make(map[string]struct {
		agent string
		wave  int
		repo  string
	})
	for _, fo := range manifest.FileOwnership {
		// Normalize file path
		filePath := fo.File
		fileOwnershipMap[filePath] = struct {
			agent string
			wave  int
			repo  string
		}{agent: fo.Agent, wave: fo.Wave, repo: fo.Repo}
	}

	// Build a map of type -> references (agent IDs + file paths)
	typeReferences := make(map[string]*typeRefData)

	// Scan all agents' task prompts for import patterns
	for _, wave := range manifest.Waves {
		for _, agent := range wave.Agents {
			refs := extractTypeReferences(agent.Task, agent.ID)
			for _, ref := range refs {
				if typeReferences[ref.typeName] == nil {
					typeReferences[ref.typeName] = &typeRefData{
						typeName:          ref.typeName,
						importedFile:      ref.importedFile,
						referencingAgents: []string{},
						referencingFiles:  []string{},
					}
				}
				// Add this agent to the referencing list if not already present
				if !containsString(typeReferences[ref.typeName].referencingAgents, agent.ID) {
					typeReferences[ref.typeName].referencingAgents = append(
						typeReferences[ref.typeName].referencingAgents, agent.ID)
				}
				// Track which files this agent owns that reference the type
				for _, ownedFile := range agent.Files {
					if !containsString(typeReferences[ref.typeName].referencingFiles, ownedFile) {
						typeReferences[ref.typeName].referencingFiles = append(
							typeReferences[ref.typeName].referencingFiles, ownedFile)
					}
				}
			}
		}
	}

	// Filter to types that need scaffolding
	// A type needs scaffolding if 2+ agents reference it (not counting the defining agent)
	var candidates []SharedTypeCandidate

	for typeName, refData := range typeReferences {
		if len(refData.referencingAgents) < 2 {
			continue
		}

		// Find which agent owns the defining file
		importedFile := refData.importedFile
		ownership, isOwned := fileOwnershipMap[importedFile]

		// If exact match fails, try matching by basename or suffix
		if !isOwned {
			for ownedPath, owner := range fileOwnershipMap {
				// Try suffix match (handles cases like "./types" -> "src/types.ts")
				if strings.HasSuffix(ownedPath, importedFile) ||
				   strings.HasSuffix(ownedPath, "/"+importedFile) ||
				   filepath.Base(ownedPath) == filepath.Base(importedFile) {
					ownership = owner
					importedFile = ownedPath // Use the full path from file_ownership
					isOwned = true
					break
				}
			}
		}

		if !isOwned {
			// Type is in a file not owned by any agent (existing codebase infrastructure)
			continue
		}

		// Check if type already exists in codebase
		typeExists := false
		reason := ""
		if repoRoot != "" {
			absPath := filepath.Join(repoRoot, importedFile)
			if fileExists(absPath) {
				typeExists = true
				reason = fmt.Sprintf("Type exists in %s but not scaffolded; agents %v reference it",
					importedFile, refData.referencingAgents)
			} else {
				reason = fmt.Sprintf("Agents %v reference type from %s (owned by agent %s)",
					refData.referencingAgents, importedFile, ownership.agent)
			}
		} else {
			reason = fmt.Sprintf("Agents %v reference type from %s (owned by agent %s)",
				refData.referencingAgents, importedFile, ownership.agent)
		}

		if typeExists {
			// Don't skip — still emit the candidate but mark with "exists" reason
			reason = fmt.Sprintf("Type exists in %s but agents %v still reference it; verify imports are correct",
				importedFile, refData.referencingAgents)
		}

		candidates = append(candidates, SharedTypeCandidate{
			TypeName:          typeName,
			DefiningAgent:     ownership.agent,
			DefiningFile:      importedFile,
			ReferencingAgents: refData.referencingAgents,
			ReferencingFiles:  refData.referencingFiles,
			Reason:            reason,
		})
	}

	// Check for circular dependencies
	circularDeps := detectCircularDeps(manifest)
	if len(circularDeps) > 0 {
		return nil, fmt.Errorf("circular dependency detected: %s", strings.Join(circularDeps, "; "))
	}

	return candidates, nil
}

type typeRefData struct {
	typeName          string
	importedFile      string
	referencingAgents []string
	referencingFiles  []string
}

type importRef struct {
	typeName     string
	importedFile string
}

// extractTypeReferences parses agent task text for import patterns
func extractTypeReferences(taskText, agentID string) []importRef {
	var refs []importRef

	// Go: import "path/to/package" or import Type from "path"
	goImportPattern := regexp.MustCompile(`import\s+(?:"([^"]+)"|(\w+)\s+from\s+"([^"]+)")`)
	matches := goImportPattern.FindAllStringSubmatch(taskText, -1)
	for _, match := range matches {
		if len(match) > 1 && match[1] != "" {
			// Simple import "path"
			refs = append(refs, importRef{typeName: "", importedFile: match[1]})
		} else if len(match) > 3 && match[2] != "" && match[3] != "" {
			// import Type from "path"
			refs = append(refs, importRef{typeName: match[2], importedFile: match[3]})
		}
	}

	// Rust: use crate::module::Type or import Type from crate::module
	rustUsePattern := regexp.MustCompile(`(?:use|import)\s+(?:crate::)?([a-zA-Z0-9_/:]+)(?:::(\w+))?`)
	rustFromPattern := regexp.MustCompile(`(?:import|use)\s+(\w+)\s+from\s+crate::([a-zA-Z0-9_:]+)`)

	matches = rustUsePattern.FindAllStringSubmatch(taskText, -1)
	for _, match := range matches {
		if len(match) > 2 && match[2] != "" {
			// use crate::module::Type
			modulePath := strings.ReplaceAll(match[1], "::", "/")
			refs = append(refs, importRef{typeName: match[2], importedFile: "src/" + modulePath + ".rs"})
		}
	}

	matches = rustFromPattern.FindAllStringSubmatch(taskText, -1)
	for _, match := range matches {
		if len(match) > 2 {
			// import Type from crate::module
			modulePath := strings.ReplaceAll(match[2], "::", "/")
			refs = append(refs, importRef{typeName: match[1], importedFile: "src/" + modulePath + ".rs"})
		}
	}

	// TypeScript: import { Type } from "./module"
	tsImportPattern := regexp.MustCompile(`import\s+\{\s*(\w+)\s*\}\s+from\s+["']([^"']+)["']`)
	matches = tsImportPattern.FindAllStringSubmatch(taskText, -1)
	for _, match := range matches {
		if len(match) > 2 {
			filePath := match[2]
			// Normalize TypeScript paths: "./types" -> need to add .ts extension
			// and resolve relative paths if possible
			if strings.HasPrefix(filePath, "./") || strings.HasPrefix(filePath, "../") {
				// Relative import - strip leading ./ and add .ts if no extension
				filePath = strings.TrimPrefix(filePath, "./")
				if !strings.Contains(filePath, ".") {
					filePath += ".ts"
				}
			}
			refs = append(refs, importRef{typeName: match[1], importedFile: filePath})
		}
	}

	// Python: from module import Type
	pyImportPattern := regexp.MustCompile(`from\s+([a-zA-Z0-9_.]+)\s+import\s+(\w+)`)
	matches = pyImportPattern.FindAllStringSubmatch(taskText, -1)
	for _, match := range matches {
		if len(match) > 2 {
			modulePath := strings.ReplaceAll(match[1], ".", "/")
			refs = append(refs, importRef{typeName: match[2], importedFile: modulePath + ".py"})
		}
	}

	// Generic pattern: "reference Type from file.ext"
	genericPattern := regexp.MustCompile(`(?:reference|import|use)\s+(\w+)\s+from\s+(?:crate::|package::|module::)?([a-zA-Z0-9_/.:-]+)`)
	matches = genericPattern.FindAllStringSubmatch(taskText, -1)
	for _, match := range matches {
		if len(match) > 2 {
			refs = append(refs, importRef{typeName: match[1], importedFile: match[2]})
		}
	}

	return refs
}

// detectCircularDeps checks for circular import dependencies
func detectCircularDeps(manifest *protocol.IMPLManifest) []string {
	// Build adjacency list: agent -> agents it depends on
	deps := make(map[string][]string)
	for _, wave := range manifest.Waves {
		for _, agent := range wave.Agents {
			deps[agent.ID] = agent.Dependencies
		}
	}

	// DFS to detect cycles
	visited := make(map[string]bool)
	recStack := make(map[string]bool)
	var cycles []string

	var dfs func(agent string, path []string) bool
	dfs = func(agent string, path []string) bool {
		visited[agent] = true
		recStack[agent] = true
		path = append(path, agent)

		for _, dep := range deps[agent] {
			if !visited[dep] {
				if dfs(dep, path) {
					return true
				}
			} else if recStack[dep] {
				// Found a cycle
				cycle := append(path, dep)
				cycles = append(cycles, fmt.Sprintf("Agent %s -> %s", strings.Join(cycle, " -> "), dep))
				return true
			}
		}

		recStack[agent] = false
		return false
	}

	for agent := range deps {
		if !visited[agent] {
			dfs(agent, []string{})
		}
	}

	return cycles
}

func containsString(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
