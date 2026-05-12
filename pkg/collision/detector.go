package collision

import (
	"context"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"log/slog"
	"path/filepath"
	"sort"
	"strings"

	igit "github.com/blackwell-systems/polywave-go/internal/git"
	"github.com/blackwell-systems/polywave-go/pkg/protocol"
	"github.com/blackwell-systems/polywave-go/pkg/result"
)

// DetectCollisions scans all agent branches for the given wave, extracts new type
// declarations via AST parsing, and detects collisions (same type name in same
// package across 2+ branches).
//
// Algorithm:
// 1. Load IMPL manifest and identify all agents in target wave
// 2. For each agent, determine branch name (polywave/{slug}/waveN-agent-{ID} or legacy waveN-agent-{ID})
// 3. Run git diff <merge-base HEAD branch>..branch --name-only to get changed .go files
// 4. For each .go file, parse AST and extract type declarations
// 5. Group types by package path
// 6. Detect collisions: any type name in 2+ agent branches within same package
// 7. Generate resolution suggestions (keep alphabetically first agent)
//
// Returns CollisionReport with Valid=false if any collisions detected.
func DetectCollisions(ctx context.Context, manifestPath string, waveNum int, repoPath string) result.Result[CollisionReport] {
	if err := ctx.Err(); err != nil {
		return result.NewFailure[CollisionReport]([]result.PolywaveError{
			result.NewFatal(result.CodeCollisionContextCancelled, err.Error()).WithCause(err),
		})
	}
	// Load IMPL manifest
	manifest, err := protocol.Load(ctx, manifestPath)
	if err != nil {
		return result.NewFailure[CollisionReport]([]result.PolywaveError{
			result.NewFatal(result.CodeCollisionLoadManifestFailed, fmt.Sprintf("load manifest: %s", err.Error())).WithCause(err),
		})
	}

	// Find target wave
	if waveNum < 1 || waveNum > len(manifest.Waves) {
		return result.NewFailure[CollisionReport]([]result.PolywaveError{
			result.NewFatal(result.CodeCollisionInvalidWave, fmt.Sprintf("invalid wave number %d (manifest has %d waves)", waveNum, len(manifest.Waves))),
		})
	}
	wave := manifest.Waves[waveNum-1]

	// Extract slug from manifest (if present)
	slug := manifest.FeatureSlug

	// Map: agentID -> []TypeDeclaration
	agentTypes := make(map[string][]TypeDeclaration)

	// Scan each agent's branch for type declarations
	for _, agent := range wave.Agents {
		if err := ctx.Err(); err != nil {
			return result.NewFailure[CollisionReport]([]result.PolywaveError{
				result.NewFatal(result.CodeCollisionContextCancelled, err.Error()).WithCause(err),
			})
		}
		branchName := buildBranchName(slug, waveNum, agent.ID)

		// Try slug-scoped branch first, fall back to legacy format, then skip if absent.
		if !igit.BranchExists(repoPath, branchName) {
			if slug != "" {
				// Slug-scoped branch missing; try legacy format.
				branchName = fmt.Sprintf("wave%d-agent-%s", waveNum, agent.ID)
			}
			if !igit.BranchExists(repoPath, branchName) {
				// Branch doesn't exist yet — skip this agent.
				continue
			}
		}

		// Get changed .go files in this branch
		changedFilesResult := getChangedGoFiles(ctx, repoPath, branchName)
		if changedFilesResult.IsFatal() {
			return result.NewFailure[CollisionReport]([]result.PolywaveError{
				result.NewFatal(result.CodeCollisionGetFilesFailed, fmt.Sprintf("get changed files for agent %s: %s", agent.ID, changedFilesResult.Errors[0].Message)).WithCause(changedFilesResult.Errors[0]),
			})
		}
		changedFiles := changedFilesResult.GetData()

		// Extract type declarations from changed files
		typesResult := extractTypesFromFiles(ctx, repoPath, branchName, changedFiles)
		if typesResult.IsFatal() {
			return result.NewFailure[CollisionReport]([]result.PolywaveError{
				result.NewFatal(result.CodeCollisionExtractTypesFailed, fmt.Sprintf("extract types for agent %s: %s", agent.ID, typesResult.Errors[0].Message)).WithCause(typesResult.Errors[0]),
			})
		}
		types := typesResult.GetData()

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

	return result.NewSuccess(CollisionReport{
		Collisions: collisions,
		Valid:      len(collisions) == 0,
	})
}

// buildBranchName constructs the branch name for a given slug, wave, and agent.
// Returns slug-scoped format if slug is non-empty, otherwise legacy format.
func buildBranchName(slug string, waveNum int, agentID string) string {
	if slug != "" {
		return fmt.Sprintf("polywave/%s/wave%d-agent-%s", slug, waveNum, agentID)
	}
	return fmt.Sprintf("wave%d-agent-%s", waveNum, agentID)
}

// getChangedGoFiles returns the list of .go files changed in the given branch
// relative to its merge base with HEAD. Uses merge-base to find the actual
// divergence point, so the diff is correct regardless of whether HEAD is on
// main, develop, or a feature branch.
func getChangedGoFiles(ctx context.Context, repoPath, branchName string) result.Result[[]string] {
	if err := ctx.Err(); err != nil {
		return result.NewFailure[[]string]([]result.PolywaveError{
			result.NewFatal(result.CodeCollisionContextCancelled, err.Error()).WithCause(err),
		})
	}
	// Compute the actual fork point between the current working branch and the
	// agent branch. This handles main, develop, and arbitrary feature branches
	// without hardcoding a branch name.
	mergeBaseOut, mbErr := igit.Run(repoPath, "merge-base", "HEAD", branchName)
	var diffBase string
	if mbErr != nil || strings.TrimSpace(mergeBaseOut) == "" {
		// Fallback: diff against main if merge-base fails (e.g., no common ancestor).
		diffBase = "main"
	} else {
		diffBase = strings.TrimSpace(mergeBaseOut)
	}
	out, err := igit.Run(repoPath, "diff", diffBase+".."+branchName, "--name-only", "--", "*.go")
	if err != nil {
		return result.NewFailure[[]string]([]result.PolywaveError{
			result.NewFatal(result.CodeCollisionGitDiffFailed, fmt.Sprintf("git diff: %s", err.Error())).WithCause(err),
		})
	}

	trimmed := strings.TrimSpace(out)
	if trimmed == "" {
		return result.NewSuccess([]string{})
	}

	files := strings.Split(trimmed, "\n")
	var goFiles []string
	for _, f := range files {
		f = strings.TrimSpace(f)
		if f != "" && strings.HasSuffix(f, ".go") && !strings.HasSuffix(f, "_test.go") {
			goFiles = append(goFiles, f)
		}
	}

	return result.NewSuccess(goFiles)
}

// extractTypesFromFiles checks out each file from the branch and extracts
// type declarations using AST parsing.
func extractTypesFromFiles(ctx context.Context, repoPath, branchName string, files []string) result.Result[[]TypeDeclaration] {
	var allTypes []TypeDeclaration

	for _, file := range files {
		if err := ctx.Err(); err != nil {
			return result.NewFailure[[]TypeDeclaration]([]result.PolywaveError{
				result.NewFatal(result.CodeCollisionContextCancelled, err.Error()).WithCause(err),
			})
		}
		// Get file content from branch
		content, err := igit.Run(repoPath, "show", branchName+":"+file)
		if err != nil {
			// File was deleted in this branch — no types to extract, not a collision.
			if strings.Contains(err.Error(), "exists on disk, but not in") {
				continue
			}
			return result.NewFailure[[]TypeDeclaration]([]result.PolywaveError{
				result.NewFatal(result.CodeCollisionGitShowFailed, fmt.Sprintf("git show failed for %s on branch %s: %s", file, branchName, err.Error())).WithCause(err),
			})
		}

		// Parse AST
		typesResult := extractTypeDecls(file, content)
		if typesResult.IsFatal() {
			return result.NewFailure[[]TypeDeclaration]([]result.PolywaveError{
				result.NewFatal(result.CodeCollisionParseFailed, fmt.Sprintf("parse %s: %s", file, typesResult.Errors[0].Message)).WithCause(typesResult.Errors[0]),
			})
		}
		types := typesResult.GetData()

		allTypes = append(allTypes, types...)
	}

	return result.NewSuccess(allTypes)
}

// extractTypeDecls parses a Go source file and extracts type declarations.
// Returns TypeDeclaration structs for each type defined in the file.
func extractTypeDecls(filePath, content string) result.Result[[]TypeDeclaration] {
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, filePath, content, parser.ParseComments)
	if err != nil {
		return result.NewFailure[[]TypeDeclaration]([]result.PolywaveError{
			result.NewFatal(result.CodeCollisionParseFailed, fmt.Sprintf("parse file %s: %s", filePath, err.Error())).WithCause(err),
		})
	}

	// Derive package path from file path (e.g., "pkg/service/handler.go" → "pkg/service")
	pkgPath := filepath.Dir(filePath)
	if pkgPath == "." {
		pkgPath = ""
	}

	// Only inspect top-level declarations (node.Decls), not recursively.
	// ast.Inspect with return true would visit function bodies and catch
	// local type aliases (e.g., "type Alias T" inside MarshalJSON methods),
	// producing false-positive collisions.
	var decls []TypeDeclaration
	for _, decl := range node.Decls {
		genDecl, ok := decl.(*ast.GenDecl)
		if !ok || genDecl.Tok != token.TYPE {
			continue
		}
		for _, spec := range genDecl.Specs {
			typeSpec, ok := spec.(*ast.TypeSpec)
			if !ok {
				continue
			}
			kind := "alias"
			switch typeSpec.Type.(type) {
			case *ast.StructType:
				kind = "struct"
			case *ast.InterfaceType:
				kind = "interface"
			}
			decls = append(decls, TypeDeclaration{
				Name:    typeSpec.Name.Name,
				Package: pkgPath,
				Kind:    kind,
			})
		}
	}

	return result.NewSuccess(decls)
}

// detectCollisionsInTypes finds type name collisions across agents.
// A collision occurs when 2+ agents define the same type name in the same package.
func detectCollisionsInTypes(agentTypes map[string][]TypeDeclaration) []TypeCollision {
	// Map: "package/TypeName" -> []agentID
	typeOccurrences := make(map[string][]string)

	for agentID, types := range agentTypes {
		for _, t := range types {
			if t.Name == "" {
				continue
			}
			key := t.Package + "/" + t.Name
			// NOT goroutine-safe: this map is written from a single goroutine.
			// Do not parallelize this loop without protecting typeOccurrences with a sync.Mutex.
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
		if lastSlash == -1 {
			slog.Warn("invalid type key format (missing slash separator)", "key", key)
			continue
		}
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
