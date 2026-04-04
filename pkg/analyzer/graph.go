package analyzer

import (
	"context"
	"fmt"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/result"
)

// detectLanguage auto-detects the language from file extensions.
// Returns "go", "rust", "javascript", or "python" based on the most common extension.
// Returns error if unsupported extension or mixed languages detected.
func detectLanguage(files []string) (string, error) {
	extCounts := make(map[string]int)
	for _, f := range files {
		ext := filepath.Ext(f)
		extCounts[ext]++
	}

	var maxExt string
	var maxCount int
	for ext, count := range extCounts {
		if count > maxCount {
			maxExt = ext
			maxCount = count
		}
	}

	switch maxExt {
	case ".go":
		return "go", nil
	case ".rs":
		return "rust", nil
	case ".js", ".jsx", ".ts", ".tsx", ".mjs":
		return "javascript", nil
	case ".py":
		return "python", nil
	default:
		return "", fmt.Errorf("unsupported file extension: %s", maxExt)
	}
}

// parseGoFiles parses Go files and extracts their dependencies.
// Extracted from BuildGraph Step 1 logic.
// Uses Analyzer.ParseFile() and ExtractImports() with context propagation (P0-3 fix).
// Agent C migrates ParseFile/ExtractImports to result.Result[T]; after merge the
// call sites here will use .IsSuccess()/.GetData() unwrap pattern.
func parseGoFiles(ctx context.Context, repoRoot string, files []string) (map[string][]string, error) {
	a := New()
	fileImports := make(map[string][]string)

	for _, f := range files {
		// P0-3: propagate ctx instead of context.Background()
		astFile, err := a.ParseFile(ctx, f)
		if err != nil {
			return nil, fmt.Errorf("parse %s: %w", f, err)
		}

		// P0-3: propagate ctx instead of context.Background()
		imports, err := a.ExtractImports(ctx, astFile, repoRoot)
		if err != nil {
			return nil, fmt.Errorf("extract imports %s: %w", f, err)
		}

		// Resolve package directories to actual .go files
		resolvedImports, err := resolvePackageDeps(f, imports, files)
		if err != nil {
			return nil, fmt.Errorf("resolve deps for %s: %w", f, err)
		}

		fileImports[f] = resolvedImports
	}

	return fileImports, nil
}

// BuildGraph constructs a file-level dependency graph for the given files.
// Algorithm:
// 0. Detect language from file extensions
// 1. Parse each file with language-specific parser, extract imports
// 2. Build adjacency map: file -> deps (local imports only)
// 3. Detect cycles using DFS (reuse pkg/solver/graph.go pattern)
// 4. Compute topological sort depth: files with no deps = depth 0, files depending on depth N = depth N+1
// 5. Assign waves: wave N = all files at depth N
// 6. Detect cascade candidates: scan repoRoot for files importing modified files but not in files list
func BuildGraph(ctx context.Context, repoRoot string, files []string) result.Result[*DepGraph] {
	// Step 0: Detect language
	lang, err := detectLanguage(files)
	if err != nil {
		return result.NewFailure[*DepGraph]([]result.SAWError{
			result.NewFatal(result.CodeAnalyzeUnsupportedLang, err.Error()),
		})
	}

	// Step 1: Parse files with language-specific parser
	var fileImports map[string][]string
	switch lang {
	case "go":
		fileImports, err = parseGoFiles(ctx, repoRoot, files)
	case "rust":
		fileImports, err = parseRustFiles(repoRoot, files)
	case "javascript":
		fileImports, err = parseJavaScriptFiles(repoRoot, files)
	case "python":
		fileImports, err = parsePythonFiles(repoRoot, files)
	default:
		return result.NewFailure[*DepGraph]([]result.SAWError{
			result.NewFatal(result.CodeAnalyzeUnsupportedLang, fmt.Sprintf("unsupported language: %s", lang)),
		})
	}
	if err != nil {
		return result.NewFailure[*DepGraph]([]result.SAWError{
			result.NewFatal(result.CodeAnalyzeParseFailed, err.Error()),
		})
	}

	// Step 2: Build adjacency map and reverse map
	adj := make(map[string][]string)
	revAdj := make(map[string][]string)
	for file, imports := range fileImports {
		adj[file] = imports
		for _, imp := range imports {
			revAdj[imp] = append(revAdj[imp], file)
		}
	}

	// Sort all adjacency lists for determinism
	for file := range adj {
		sort.Strings(adj[file])
	}
	for file := range revAdj {
		sort.Strings(revAdj[file])
	}

	// Step 3: Detect cycles
	if cycles := detectCycles(adj, files); cycles != nil {
		return result.NewFailure[*DepGraph]([]result.SAWError{
			result.NewFatal(result.CodeAnalyzeCycleDetected, fmt.Sprintf("circular dependency detected: %s", formatCycle(cycles[0]))),
		})
	}

	// Step 4: Topological sort depth (Kahn's algorithm)
	// Pass revAdj to avoid O(n²) inner loop recomputation (P2-1 fix)
	depth := computeDepth(adj, revAdj, files)

	// Step 5: Assign waves
	waves := assignWaves(depth)

	// Step 6: Detect cascade candidates
	cascades, err := detectCascades(ctx, repoRoot, files, revAdj)
	if err != nil {
		return result.NewFailure[*DepGraph]([]result.SAWError{
			result.NewFatal(result.CodeAnalyzeWalkFailed, fmt.Sprintf("detect cascades: %v", err)),
		})
	}

	// Step 7: Build DepGraph
	nodes := make([]FileNode, 0, len(files))
	for _, f := range files {
		nodes = append(nodes, FileNode{
			File:          f,
			DependsOn:     adj[f],
			DependedBy:    revAdj[f],
			WaveCandidate: depth[f],
		})
	}

	// Sort nodes by file path for determinism
	sort.Slice(nodes, func(i, j int) bool {
		return nodes[i].File < nodes[j].File
	})

	return result.NewSuccess(&DepGraph{
		Nodes:             nodes,
		Waves:             waves,
		CascadeCandidates: cascades,
	})
}

// resolvePackageDeps converts package directories to actual file dependencies.
// For each package directory imported, finds which files in the 'files' list belong to that package.
func resolvePackageDeps(importer string, pkgDirs []string, allFiles []string) ([]string, error) {
	var deps []string
	pkgDirSet := make(map[string]bool)
	for _, dir := range pkgDirs {
		pkgDirSet[dir] = true
	}

	// For each file in allFiles, check if its directory is in pkgDirs
	for _, f := range allFiles {
		if f == importer {
			continue // Don't depend on self
		}
		fileDir := filepath.Dir(f)
		if pkgDirSet[fileDir] {
			deps = append(deps, f)
		}
	}

	sort.Strings(deps)
	return deps, nil
}

// detectCycles uses DFS to find cycles in the adjacency map.
// Returns first cycle found as []string, or nil if no cycles.
func detectCycles(adj map[string][]string, files []string) [][]string {
	// Sort files for deterministic iteration
	sorted := make([]string, len(files))
	copy(sorted, files)
	sort.Strings(sorted)

	var cycles [][]string
	visited := make(map[string]bool)
	inStack := make(map[string]bool)
	path := make([]string, 0)

	var dfs func(id string)
	dfs = func(id string) {
		visited[id] = true
		inStack[id] = true
		path = append(path, id)

		deps := adj[id]
		for _, dep := range deps {
			if inStack[dep] {
				// Found a cycle — extract the cycle path from the recursion stack
				cycleStart := -1
				for i, p := range path {
					if p == dep {
						cycleStart = i
						break
					}
				}
				if cycleStart >= 0 {
					cycle := make([]string, len(path[cycleStart:]))
					copy(cycle, path[cycleStart:])
					cycle = append(cycle, dep) // close the cycle
					cycles = append(cycles, cycle)
				}
			} else if !visited[dep] {
				if _, exists := adj[dep]; exists {
					dfs(dep)
				}
			}
		}

		path = path[:len(path)-1]
		inStack[id] = false
	}

	for _, id := range sorted {
		if !visited[id] {
			dfs(id)
		}
	}

	if len(cycles) == 0 {
		return nil
	}
	return cycles
}

// formatCycle formats a cycle path for error messages.
func formatCycle(cycle []string) string {
	// Use base names for readability
	names := make([]string, len(cycle))
	for i, path := range cycle {
		names[i] = filepath.Base(path)
	}
	return strings.Join(names, " -> ")
}

// computeDepth assigns topological depth to each file.
// Depth 0 = no dependencies, Depth N = depends on files at depth N-1.
// Uses Kahn's algorithm for deterministic ordering.
// revAdj is passed in to avoid O(n²) inner loop recomputation (P2-1 fix).
func computeDepth(adj map[string][]string, revAdj map[string][]string, files []string) map[string]int {
	depth := make(map[string]int)
	inDegree := make(map[string]int)

	// Initialize in-degree for all files
	for _, f := range files {
		inDegree[f] = len(adj[f])
	}

	// Find roots (files with no dependencies)
	var queue []string
	for _, f := range files {
		if inDegree[f] == 0 {
			queue = append(queue, f)
			depth[f] = 0
		}
	}
	sort.Strings(queue)

	// Process in topological order using revAdj to find dependents
	for len(queue) > 0 {
		// Sort for determinism
		sort.Strings(queue)
		current := queue[0]
		queue = queue[1:]

		// Use revAdj to find files that depend on current (O(1) lookup vs O(n²) inner loop)
		for _, dependent := range revAdj[current] {
			// dependent depends on current, so dependent's depth = max(dependent's depth, current's depth + 1)
			newDepth := depth[current] + 1
			if newDepth > depth[dependent] {
				depth[dependent] = newDepth
			}
			inDegree[dependent]--
			if inDegree[dependent] == 0 {
				queue = append(queue, dependent)
			}
		}
	}

	return depth
}

// assignWaves groups files by depth into waves.
// Wave 1 = depth 0 files (no dependencies).
func assignWaves(depth map[string]int) map[int][]string {
	waves := make(map[int][]string)
	for file, d := range depth {
		waveNum := d + 1 // waves are 1-indexed
		waves[waveNum] = append(waves[waveNum], file)
	}

	// Sort each wave's files
	for wave := range waves {
		sort.Strings(waves[wave])
	}

	return waves
}

// detectCascades scans repoRoot for .go files that import modified files
// but are not in the files list. These are cascade candidates.
// ctx is propagated to getModulePath and used for cancellation checks.
func detectCascades(ctx context.Context, repoRoot string, modifiedFiles []string, revAdj map[string][]string) ([]CascadeCandidate, error) {
	// Build set of modified files for quick lookup
	modifiedSet := make(map[string]bool)
	for _, f := range modifiedFiles {
		modifiedSet[f] = true
	}

	// Get all .go files that import modified files (from revAdj)
	var cascades []CascadeCandidate
	seen := make(map[string]bool)

	for modFile := range modifiedSet {
		importers := revAdj[modFile]
		for _, importer := range importers {
			if !modifiedSet[importer] && !seen[importer] {
				seen[importer] = true
				cascades = append(cascades, CascadeCandidate{
					File:        importer,
					Line:        0,
					Match:       "",
					CascadeType: "semantic",
					Severity:    "medium",
					Reason:      fmt.Sprintf("imports modified file %s", filepath.Base(modFile)),
				})
			}
		}
	}

	// Also scan for files in the repo that import modified packages but aren't in our graph.
	// P0-4: propagate ctx instead of context.Background()
	modulePath, err := getModulePath(ctx, repoRoot)
	if err != nil {
		return cascades, nil // Non-fatal
	}

	err = filepath.Walk(repoRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Check for context cancellation
		if ctx.Err() != nil {
			return ctx.Err()
		}

		// Skip vendor, .git, and hidden directories
		if info.IsDir() {
			base := filepath.Base(path)
			if base == "vendor" || base == ".git" || strings.HasPrefix(base, ".") {
				return filepath.SkipDir
			}
			return nil
		}

		// Only process .go files
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}

		// Skip if already in modified set or already seen
		if modifiedSet[path] || seen[path] {
			return nil
		}

		// Parse and check imports
		fset := token.NewFileSet()
		astFile, err := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
		if err != nil {
			return nil // Skip unparseable files
		}

		// Check if this file imports any modified packages
		for _, imp := range astFile.Imports {
			importPath := strings.Trim(imp.Path.Value, `"`)
			if !strings.HasPrefix(importPath, modulePath) {
				continue
			}

			// Resolve import to directory.
			// Agent C migrates ResolveImportPath to accept ctx and return result.Result[string];
			// after merge, this becomes: ResolveImportPath(ctx, importPath, repoRoot, modulePath)
			resolved, err := ResolveImportPath(importPath, repoRoot, modulePath)
			if err != nil {
				continue
			}

			// Check if any modified file is in this package
			for modFile := range modifiedSet {
				if filepath.Dir(modFile) == resolved {
					if !seen[path] {
						seen[path] = true
						cascades = append(cascades, CascadeCandidate{
							File:        path,
							Line:        0,
							Match:       "",
							CascadeType: "semantic",
							Severity:    "medium",
							Reason:      fmt.Sprintf("imports package containing modified file %s", filepath.Base(modFile)),
						})
					}
					break
				}
			}
		}

		return nil
	})

	if err != nil {
		return cascades, nil // Non-fatal, return what we found
	}

	// Sort cascades by file path
	sort.Slice(cascades, func(i, j int) bool {
		return cascades[i].File < cascades[j].File
	})

	return cascades, nil
}
