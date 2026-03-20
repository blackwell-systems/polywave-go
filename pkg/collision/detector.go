package collision

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"sort"
	"strings"

	igit "github.com/blackwell-systems/scout-and-wave-go/internal/git"
	"github.com/blackwell-systems/scout-and-wave-go/pkg/protocol"
)

// DetectCollisions scans all agent branches for the given wave, extracts new type
// declarations via AST parsing, and detects collisions (same type name in same
// package across 2+ branches).
//
// Algorithm:
// 1. Load IMPL manifest and identify all agents in target wave
// 2. For each agent, determine branch name (saw/{slug}/waveN-agent-{ID} or legacy waveN-agent-{ID})
// 3. Run git diff main...branch --name-only to get changed .go files
// 4. For each .go file, parse AST and extract type declarations
// 5. Group types by package path
// 6. Detect collisions: any type name in 2+ agent branches within same package
// 7. Generate resolution suggestions (keep alphabetically first agent)
//
// Returns CollisionReport with Valid=false if any collisions detected.
func DetectCollisions(manifestPath string, waveNum int, repoPath string) (CollisionReport, error) {
	// Load IMPL manifest
	manifest, err := protocol.Load(manifestPath)
	if err != nil {
		return CollisionReport{}, fmt.Errorf("load manifest: %w", err)
	}

	// Find target wave
	if waveNum < 1 || waveNum > len(manifest.Waves) {
		return CollisionReport{}, fmt.Errorf("invalid wave number %d (manifest has %d waves)", waveNum, len(manifest.Waves))
	}
	wave := manifest.Waves[waveNum-1]

	// Extract slug from manifest (if present)
	slug := manifest.FeatureSlug

	// Map: agentID -> []TypeDeclaration
	agentTypes := make(map[string][]TypeDeclaration)

	// Scan each agent's branch for type declarations
	for _, agent := range wave.Agents {
		branchName := buildBranchName(slug, waveNum, agent.ID)

		// Try slug-scoped branch first, fall back to legacy
		if !igit.BranchExists(repoPath, branchName) && slug != "" {
			// Try legacy format
			branchName = fmt.Sprintf("wave%d-agent-%s", waveNum, agent.ID)
			if !igit.BranchExists(repoPath, branchName) {
				// Branch doesn't exist yet — skip this agent
				continue
			}
		}

		// Get changed .go files in this branch
		changedFiles, err := getChangedGoFiles(repoPath, branchName)
		if err != nil {
			return CollisionReport{}, fmt.Errorf("get changed files for agent %s: %w", agent.ID, err)
		}

		// Extract type declarations from changed files
		types, err := extractTypesFromFiles(repoPath, branchName, changedFiles)
		if err != nil {
			return CollisionReport{}, fmt.Errorf("extract types for agent %s: %w", agent.ID, err)
		}

		agentTypes[agent.ID] = types
	}

	// Detect collisions: same type name in same package across multiple agents
	collisions := detectCollisionsInTypes(agentTypes)

	// Sort collisions deterministically
	sort.Slice(collisions, func(i, j int) bool {
		if collisions[i].Package != collisions[j].Package {
			return collisions[i].Package < collisions[j].Package
		}
		return collisions[i].TypeName < collisions[j].TypeName
	})

	return CollisionReport{
		Collisions: collisions,
		Valid:      len(collisions) == 0,
	}, nil
}

// buildBranchName constructs the branch name for a given slug, wave, and agent.
// Returns slug-scoped format if slug is non-empty, otherwise legacy format.
func buildBranchName(slug string, waveNum int, agentID string) string {
	if slug != "" {
		return fmt.Sprintf("saw/%s/wave%d-agent-%s", slug, waveNum, agentID)
	}
	return fmt.Sprintf("wave%d-agent-%s", waveNum, agentID)
}

// getChangedGoFiles returns the list of .go files changed in the given branch
// relative to main. Uses three-dot diff to find changes introduced by the branch.
func getChangedGoFiles(repoPath, branchName string) ([]string, error) {
	// Use three-dot range to get changes introduced by branch
	out, err := igit.Run(repoPath, "diff", "main..."+branchName, "--name-only", "--", "*.go")
	if err != nil {
		return nil, fmt.Errorf("git diff: %w", err)
	}

	trimmed := strings.TrimSpace(out)
	if trimmed == "" {
		return []string{}, nil
	}

	files := strings.Split(trimmed, "\n")
	var goFiles []string
	for _, f := range files {
		f = strings.TrimSpace(f)
		if f != "" && strings.HasSuffix(f, ".go") && !strings.HasSuffix(f, "_test.go") {
			goFiles = append(goFiles, f)
		}
	}

	return goFiles, nil
}

// extractTypesFromFiles checks out each file from the branch and extracts
// type declarations using AST parsing.
func extractTypesFromFiles(repoPath, branchName string, files []string) ([]TypeDeclaration, error) {
	var allTypes []TypeDeclaration

	for _, file := range files {
		// Get file content from branch
		content, err := igit.Run(repoPath, "show", branchName+":"+file)
		if err != nil {
			// File might have been deleted or is binary — skip
			continue
		}

		// Parse AST
		types, err := extractTypeDecls(file, content)
		if err != nil {
			// Error parsing — fail fast per constraints
			return nil, fmt.Errorf("parse %s: %w", file, err)
		}

		allTypes = append(allTypes, types...)
	}

	return allTypes, nil
}

// extractTypeDecls parses a Go source file and extracts type declarations.
// Returns TypeDeclaration structs for each type defined in the file.
func extractTypeDecls(filePath, content string) ([]TypeDeclaration, error) {
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, filePath, content, parser.ParseComments)
	if err != nil {
		return nil, err
	}

	// Derive package path from file path (e.g., "pkg/service/handler.go" → "pkg/service")
	pkgPath := filepath.Dir(filePath)

	var decls []TypeDeclaration
	ast.Inspect(node, func(n ast.Node) bool {
		switch t := n.(type) {
		case *ast.TypeSpec:
			kind := "alias"
			switch t.Type.(type) {
			case *ast.StructType:
				kind = "struct"
			case *ast.InterfaceType:
				kind = "interface"
			}
			decls = append(decls, TypeDeclaration{
				Name:    t.Name.Name,
				Package: pkgPath,
				Kind:    kind,
			})
		}
		return true
	})

	return decls, nil
}

// detectCollisionsInTypes finds type name collisions across agents.
// A collision occurs when 2+ agents define the same type name in the same package.
func detectCollisionsInTypes(agentTypes map[string][]TypeDeclaration) []TypeCollision {
	// Map: "package/TypeName" -> []agentID
	typeOccurrences := make(map[string][]string)

	for agentID, types := range agentTypes {
		for _, t := range types {
			key := t.Package + "/" + t.Name
			typeOccurrences[key] = append(typeOccurrences[key], agentID)
		}
	}

	var collisions []TypeCollision
	for key, agents := range typeOccurrences {
		if len(agents) < 2 {
			continue
		}

		// Sort agents alphabetically for determinism
		sort.Strings(agents)

		// Parse key back into package and type name
		lastSlash := strings.LastIndex(key, "/")
		pkg := key[:lastSlash]
		typeName := key[lastSlash+1:]

		// Generate resolution: keep first agent alphabetically
		resolution := fmt.Sprintf("Keep %s, remove from %s", agents[0], strings.Join(agents[1:], " and "))

		collisions = append(collisions, TypeCollision{
			TypeName:   typeName,
			Package:    pkg,
			Agents:     agents,
			Resolution: resolution,
		})
	}

	return collisions
}
