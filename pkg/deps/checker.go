package deps

import (
	"fmt"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/protocol"
)

// parsers holds registered lock file parsers
var parsers []LockFileParser

// RegisterParser adds a lock file parser to the registry
func RegisterParser(p LockFileParser) {
	parsers = append(parsers, p)
}

// CheckDeps analyzes IMPL doc and lock files for dependency conflicts
func CheckDeps(implPath string, wave int) (*ConflictReport, error) {
	// 1. Parse IMPL doc to get file ownership for specified wave
	manifest, err := protocol.Load(implPath)
	if err != nil {
		return nil, fmt.Errorf("failed to parse IMPL doc: %w", err)
	}

	// Get repo root from IMPL doc path
	repoRoot := filepath.Dir(filepath.Dir(filepath.Dir(implPath))) // docs/IMPL/file.yaml -> repo root

	// 2. Extract files owned by agents in this wave
	ownedFiles := make(map[string]string) // file path -> agent ID
	for _, entry := range manifest.FileOwnership {
		if entry.Wave == wave {
			ownedFiles[entry.File] = entry.Agent
		}
	}

	if len(ownedFiles) == 0 {
		return &ConflictReport{}, nil // no files in this wave, no conflicts
	}

	// 3. Extract external package imports from owned files
	imports, err := extractExternalImports(repoRoot, ownedFiles)
	if err != nil {
		return nil, fmt.Errorf("failed to analyze dependencies: %w", err)
	}

	// 4. Detect lock files in repo root
	lockFiles, err := DetectLockFiles(repoRoot)
	if err != nil {
		return nil, fmt.Errorf("failed to detect lock files: %w", err)
	}

	// 5. Parse lock files using registered parsers
	lockFilePackages := make(map[string]PackageInfo) // package name -> info
	for _, lockFile := range lockFiles {
		for _, parser := range parsers {
			if parser.Detect(lockFile) {
				packages, err := parser.Parse(lockFile)
				if err != nil {
					// Log but don't fail on parse errors
					continue
				}
				for _, pkg := range packages {
					lockFilePackages[pkg.Name] = pkg
				}
				break // found a parser that handles this file
			}
		}
	}

	// 6. Cross-reference imports against lock file contents
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
		lockPkg, inLockFile := lockFilePackages[pkg]

		if !inLockFile {
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

	return report, nil
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

// NormalizePackageName normalizes package names across different ecosystems
func NormalizePackageName(pkg string) string {
	// Remove version suffixes like @v1.2.3
	if idx := strings.Index(pkg, "@"); idx != -1 {
		pkg = pkg[:idx]
	}
	// Trim common prefixes
	pkg = strings.TrimPrefix(pkg, "github.com/")
	pkg = strings.TrimPrefix(pkg, "golang.org/x/")
	return strings.ToLower(pkg)
}

// extractExternalImports parses Go files and extracts external package imports
// (stdlib and third-party packages, not local relative imports)
func extractExternalImports(repoRoot string, ownedFiles map[string]string) (map[string][]string, error) {
	result := make(map[string][]string) // file -> []external packages

	// Get module path from go.mod to identify local vs external imports
	modulePath, err := getModulePath(repoRoot)
	if err != nil {
		// If no go.mod, assume all imports are external
		modulePath = ""
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
func getModulePath(repoRoot string) (string, error) {
	goModPath := filepath.Join(repoRoot, "go.mod")
	data, err := os.ReadFile(goModPath)
	if err != nil {
		return "", err
	}

	// Parse module line
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "module ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "module")), nil
		}
	}

	return "", fmt.Errorf("no module directive found in go.mod")
}
