package deps

import (
	"context"
	"fmt"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/protocol"
	"github.com/blackwell-systems/scout-and-wave-go/pkg/result"
)

// parsers holds registered lock file parsers
var parsers []LockFileParser

// RegisterParser adds a lock file parser to the registry
func RegisterParser(p LockFileParser) {
	parsers = append(parsers, p)
}

// CheckDeps analyzes IMPL doc and lock files for dependency conflicts
func CheckDeps(implPath string, wave int) result.Result[ConflictReport] {
	// 1. Parse IMPL doc to get file ownership for specified wave
	manifest, err := protocol.Load(context.TODO(), implPath)
	if err != nil {
		return result.NewFailure[ConflictReport]([]result.SAWError{
			result.NewFatal(result.CodeIMPLParseFailed,
				fmt.Sprintf("failed to parse IMPL doc: %v", err)).WithCause(err),
		})
	}

	// Get repo root from IMPL doc path
	repoRoot := filepath.Dir(filepath.Dir(filepath.Dir(implPath))) // docs/IMPL/file.yaml -> repo root

	// F5: Validate repo root contains .git directory
	if _, err := os.Stat(filepath.Join(repoRoot, ".git")); err != nil {
		return result.NewFailure[ConflictReport]([]result.SAWError{
			result.NewFatal("D011_REPO_ROOT_INVALID",
				fmt.Sprintf("inferred repo root %s does not contain .git directory", repoRoot)),
		})
	}

	// 2. Extract files owned by agents in this wave
	ownedFiles := make(map[string]string) // file path -> agent ID
	for _, entry := range manifest.FileOwnership {
		if entry.Wave == wave {
			ownedFiles[entry.File] = entry.Agent
		}
	}

	if len(ownedFiles) == 0 {
		return result.NewSuccess(ConflictReport{}) // no files in this wave, no conflicts
	}

	// 3. Extract external package imports from owned files
	imports, err := extractExternalImports(repoRoot, ownedFiles)
	if err != nil {
		return result.NewFailure[ConflictReport]([]result.SAWError{
			result.NewFatal("D003_MISSING_DEPS",
				fmt.Sprintf("failed to analyze dependencies: %v", err)).WithCause(err),
		})
	}

	// 4. Detect lock files in repo root
	lockFiles, err := DetectLockFiles(repoRoot)
	if err != nil {
		return result.NewFailure[ConflictReport]([]result.SAWError{
			result.NewFatal("D001_LOCK_FILE_OPEN",
				fmt.Sprintf("failed to detect lock files: %v", err)).WithCause(err),
		})
	}

	// 5. Parse lock files using registered parsers
	lockFilePackages := make(map[string]PackageInfo) // package name -> info
	var parseErrors []result.SAWError
	for _, lockFile := range lockFiles {
		for _, parser := range parsers {
			if parser.Detect(lockFile) {
				// Agent B updated Parse signature to return result.Result[[]PackageInfo]
				parseRes := parser.Parse(lockFile)
				if !parseRes.IsSuccess() {
					// F6: Log parser error instead of silently dropping it
					parseErrors = append(parseErrors, parseRes.Errors...)
					continue
				}
				packages := parseRes.GetData()
				for _, pkg := range packages {
					lockFilePackages[pkg.Name] = pkg
				}
				break // found a parser that handles this file
			}
		}
	}

	// 6. Cross-reference imports against lock file contents
	// Parse replace directives so locally-replaced modules are not
	// reported as missing dependencies (false positives).
	localReplace := ParseGoModReplace(filepath.Join(repoRoot, "go.mod"))

	report := &ConflictReport{
		MissingDeps:      []MissingDep{},
		VersionConflicts: []VersionConflict{},
		Recommendations:  []string{},
	}

	// Track which imports come from which agents/files
	importsByPackage := make(map[string]map[string][]string) // package -> agent -> []files
	for file, pkgs := range imports {
		agent := ownedFiles[file]
		for _, pkg := range pkgs {
			if importsByPackage[pkg] == nil {
				importsByPackage[pkg] = make(map[string][]string)
			}
			importsByPackage[pkg][agent] = append(importsByPackage[pkg][agent], file)
		}
	}

	// Check for missing deps and version conflicts
	for pkg, agentFiles := range importsByPackage {
		lockPkg, inLockFile := findLockPackage(lockFilePackages, pkg)

		if !inLockFile {
			// Check if satisfied by a local replace directive.
			// Use longest-prefix match: if any replace key is a prefix of
			// the import path (or an exact match), skip this import.
			if isLocalReplace(localReplace, pkg) {
				continue
			}

			// Missing dependency
			for agent, files := range agentFiles {
				for _, file := range files {
					report.MissingDeps = append(report.MissingDeps, MissingDep{
						Agent:        agent,
						Package:      pkg,
						RequiredBy:   file,
						AvailableVer: "",
					})
				}
			}
		} else {
			// Check if multiple agents need this package (potential conflict)
			if len(agentFiles) > 1 {
				var agents []string
				for agent := range agentFiles {
					agents = append(agents, agent)
				}
				report.VersionConflicts = append(report.VersionConflicts, VersionConflict{
					Agents:           agents,
					Package:          pkg,
					Versions:         []string{lockPkg.Version},
					ResolutionNeeded: false, // same version in lock file, no conflict
				})
			}
		}
	}

	// 7. Generate recommendations
	if len(report.MissingDeps) > 0 {
		report.Recommendations = append(report.Recommendations,
			fmt.Sprintf("Run package manager to install %d missing dependencies", len(report.MissingDeps)))
	}
	if len(report.VersionConflicts) > 0 {
		report.Recommendations = append(report.Recommendations,
			"Review version conflicts and update lock files if needed")
	}
	if len(report.MissingDeps) == 0 && len(report.VersionConflicts) == 0 {
		report.Recommendations = append(report.Recommendations, "No dependency conflicts detected")
	}

	// If we collected parse errors, return as partial success with warnings
	if len(parseErrors) > 0 {
		return result.NewPartial(*report, parseErrors)
	}

	return result.NewSuccess(*report)
}

// isLocalReplace reports whether importPath is satisfied by a local replace
// directive. It uses longest-prefix matching identical to findLockPackage: if
// any key in localReplace is an exact match or a module-path prefix of
// importPath, the import is considered locally replaced.
func isLocalReplace(localReplace map[string]struct{}, importPath string) bool {
	bestLen := 0
	for mod := range localReplace {
		if importPath == mod || strings.HasPrefix(importPath, mod+"/") {
			if len(mod) > bestLen {
				bestLen = len(mod)
			}
		}
	}
	return bestLen > 0
}

// findLockPackage resolves an import path against the lock file packages.
// Go import paths like "github.com/aws/aws-sdk-go-v2/service/bedrockruntime/types"
// are sub-packages of module "github.com/aws/aws-sdk-go-v2" — the lock file only
// records the module root. This function finds the longest matching module prefix.
func findLockPackage(lockFilePackages map[string]PackageInfo, importPath string) (PackageInfo, bool) {
	// Try exact match first
	if pkg, ok := lockFilePackages[importPath]; ok {
		return pkg, true
	}
	// Try prefix matching: find the longest module path that is a prefix of the import
	var bestMatch PackageInfo
	bestLen := 0
	for modPath, pkg := range lockFilePackages {
		if strings.HasPrefix(importPath, modPath+"/") || importPath == modPath {
			if len(modPath) > bestLen {
				bestMatch = pkg
				bestLen = len(modPath)
			}
		}
	}
	return bestMatch, bestLen > 0
}

// DetectLockFiles scans repo root for lock files
func DetectLockFiles(repoRoot string) ([]string, error) {
	var lockFiles []string

	// Common lock file names
	lockFileNames := []string{
		"go.sum",
		"go.mod",
		"package-lock.json",
		"yarn.lock",
		"pnpm-lock.yaml",
		"Cargo.lock",
		"poetry.lock",
		"Pipfile.lock",
		"Gemfile.lock",
	}

	for _, name := range lockFileNames {
		path := filepath.Join(repoRoot, name)
		if _, err := os.Stat(path); err == nil {
			lockFiles = append(lockFiles, path)
		}
	}

	return lockFiles, nil
}

// isStdLib checks if an import path is a Go standard library package
func isStdLib(importPath string) bool {
	// Standard library packages don't contain dots in the first path element
	// Examples: fmt, os, encoding/json (no dot before first slash)
	// Non-stdlib: github.com/..., golang.org/x/... (dot before first slash)

	// Special case: golang.org/x/* are extended stdlib but in go.mod
	if strings.HasPrefix(importPath, "golang.org/x/") {
		return false
	}

	// If there's no slash, check if it contains a dot
	if !strings.Contains(importPath, "/") {
		return !strings.Contains(importPath, ".")
	}

	// If there's a slash, check the first component
	firstComponent := importPath
	if idx := strings.Index(importPath, "/"); idx != -1 {
		firstComponent = importPath[:idx]
	}

	// Stdlib packages have no dot in first component
	return !strings.Contains(firstComponent, ".")
}

// extractExternalImports parses Go files and extracts external package imports
// (third-party packages only, not stdlib or local imports)
func extractExternalImports(repoRoot string, ownedFiles map[string]string) (map[string][]string, error) {
	result := make(map[string][]string) // file -> []external packages

	// Get module path from go.mod to identify local vs external imports
	modulePathResult := getModulePath(repoRoot)
	var modulePath string
	if modulePathResult.IsFatal() {
		// If no go.mod, assume all imports are external
		modulePath = ""
	} else {
		modulePath = modulePathResult.GetData()
	}

	for file := range ownedFiles {
		absPath := filepath.Join(repoRoot, file)

		// Check if file exists
		if _, err := os.Stat(absPath); os.IsNotExist(err) {
			// File doesn't exist yet (new file), skip
			continue
		}

		// Only parse Go files for now
		if filepath.Ext(file) != ".go" {
			continue
		}

		fset := token.NewFileSet()
		node, err := parser.ParseFile(fset, absPath, nil, parser.ImportsOnly)
		if err != nil {
			// Skip files that can't be parsed (may not exist yet)
			continue
		}

		var externalImports []string
		for _, imp := range node.Imports {
			// Remove quotes from import path
			importPath := strings.Trim(imp.Path.Value, `"`)

			// Skip "C" (cgo)
			if importPath == "C" {
				continue
			}

			// Skip standard library packages
			if isStdLib(importPath) {
				continue
			}

			// Determine if this is an external import
			isExternal := true
			if modulePath != "" {
				// If it starts with our module path, it's local
				if strings.HasPrefix(importPath, modulePath) {
					isExternal = false
				}
			}

			// Also check for relative imports (./ or ../)
			if strings.HasPrefix(importPath, ".") {
				isExternal = false
			}

			if isExternal {
				externalImports = append(externalImports, importPath)
			}
		}

		if len(externalImports) > 0 {
			result[file] = externalImports
		}
	}

	return result, nil
}

// getModulePath extracts the module path from go.mod
func getModulePath(repoRoot string) result.Result[string] {
	goModPath := filepath.Join(repoRoot, "go.mod")
	data, err := os.ReadFile(goModPath)
	if err != nil {
		return result.NewFailure[string]([]result.SAWError{
			result.NewFatal("D009_GOMOD_READ",
				fmt.Sprintf("failed to read go.mod: %v", err)).WithCause(err),
		})
	}

	// Parse module line
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "module ") {
			modulePath := strings.TrimSpace(strings.TrimPrefix(line, "module"))
			return result.NewSuccess(modulePath)
		}
	}

	return result.NewFailure[string]([]result.SAWError{
		result.NewFatal("D010_GOMOD_PARSE",
			"no module directive found in go.mod"),
	})
}
