package analyzer

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// parsePythonFiles parses Python files using the python-parser.py helper script.
// Returns a map of file path -> list of absolute paths to local dependencies.
// Filters out stdlib imports and resolves relative imports to absolute paths.
func parsePythonFiles(repoRoot string, files []string) (map[string][]string, error) {
	// Check if python3 binary exists
	pythonPath, err := exec.LookPath("python3")
	if err != nil {
		return nil, fmt.Errorf("python3 not found: %w (install Python 3 or skip Python analysis)", err)
	}

	// Locate python-parser.py via PATH lookup.
	parserScript, err := exec.LookPath("python-parser.py")
	if err != nil {
		return nil, fmt.Errorf("python-parser.py not found in PATH: required for Python parsing")
	}

	// Verify the script exists on disk.
	if _, statErr := os.Stat(parserScript); os.IsNotExist(statErr) {
		return nil, fmt.Errorf("python parser script not found: %s", parserScript)
	}

	fileImports := make(map[string][]string)

	// Parse each file
	for _, file := range files {
		// Execute python-parser.py <file>
		cmd := exec.Command(pythonPath, parserScript, file)
		cmd.Dir = repoRoot
		output, err := cmd.Output()
		if err != nil {
			return nil, fmt.Errorf("failed to parse %s: %w", file, err)
		}

		// Parse JSON output: {"file": "path.py", "imports": ["internal.types", ".utils", "json"]}
		var result struct {
			File    string   `json:"file"`
			Imports []string `json:"imports"`
		}
		if err := json.Unmarshal(output, &result); err != nil {
			return nil, fmt.Errorf("failed to parse JSON from python-parser.py for %s: %w", file, err)
		}

		// Filter and resolve imports
		var localDeps []string
		for _, imp := range result.Imports {
			// Skip stdlib imports
			if isPythonStdlib(imp) {
				continue
			}

			// Resolve relative imports (., .., .utils, etc.)
			if strings.HasPrefix(imp, ".") {
				resolved, err := resolvePythonRelativeImport(file, imp, repoRoot, files)
				if err != nil {
					// Skip unresolvable relative imports
					continue
				}
				if resolved != "" {
					localDeps = append(localDeps, resolved)
				}
				continue
			}

			// For absolute imports, try to resolve to local files
			resolved := resolvePythonAbsoluteImport(imp, repoRoot, files)
			if resolved != "" {
				localDeps = append(localDeps, resolved)
			}
		}

		fileImports[file] = localDeps
	}

	return fileImports, nil
}

// isPythonStdlib returns true if the import is a Python stdlib module.
// Hardcoded list of common stdlib modules.
func isPythonStdlib(importPath string) bool {
	// Extract the top-level module name
	parts := strings.Split(importPath, ".")
	topLevel := parts[0]

	// Common Python stdlib modules
	stdlibModules := map[string]bool{
		"abc":         true,
		"ast":         true,
		"asyncio":     true,
		"base64":      true,
		"collections": true,
		"concurrent":  true,
		"contextlib":  true,
		"copy":        true,
		"csv":         true,
		"dataclasses": true,
		"datetime":    true,
		"decimal":     true,
		"enum":        true,
		"functools":   true,
		"glob":        true,
		"hashlib":     true,
		"http":        true,
		"importlib":   true,
		"io":          true,
		"itertools":   true,
		"json":        true,
		"logging":     true,
		"math":        true,
		"multiprocessing": true,
		"operator":    true,
		"os":          true,
		"pathlib":     true,
		"pickle":      true,
		"platform":    true,
		"queue":       true,
		"random":      true,
		"re":          true,
		"shutil":      true,
		"signal":      true,
		"socket":      true,
		"sqlite3":     true,
		"string":      true,
		"subprocess":  true,
		"sys":         true,
		"tempfile":    true,
		"threading":   true,
		"time":        true,
		"traceback":   true,
		"typing":      true,
		"unittest":    true,
		"urllib":      true,
		"uuid":        true,
		"warnings":    true,
		"weakref":     true,
		"xml":         true,
		"zipfile":     true,
	}

	return stdlibModules[topLevel]
}

// resolvePythonRelativeImport resolves a relative import (., .., .utils) to an absolute file path.
// Example: file="pkg/foo/bar.py", imp=".utils" -> "pkg/foo/utils.py"
// Example: file="pkg/foo/bar.py", imp="." -> "pkg/foo/__init__.py"
// Returns empty string if the import cannot be resolved to a file in the files list.
func resolvePythonRelativeImport(file, imp, repoRoot string, allFiles []string) (string, error) {
	fileDir := filepath.Dir(file)

	// Count leading dots to determine how many levels to go up
	dots := 0
	for i := 0; i < len(imp) && imp[i] == '.'; i++ {
		dots++
	}

	// Navigate up the directory tree
	targetDir := fileDir
	for i := 1; i < dots; i++ {
		targetDir = filepath.Dir(targetDir)
	}

	// Get the module name after the dots
	moduleName := strings.TrimPrefix(imp, strings.Repeat(".", dots))

	// If no module name, it's importing the package itself (from . import something)
	if moduleName == "" {
		// Look for __init__.py in targetDir
		initFile := filepath.Join(targetDir, "__init__.py")
		for _, f := range allFiles {
			if f == initFile {
				return f, nil
			}
		}
		return "", nil // Package import, not a specific file
	}

	// Convert module path to file path (e.g., "utils.types" -> "utils/types.py")
	modulePath := strings.ReplaceAll(moduleName, ".", string(filepath.Separator))

	// Try two possibilities: module.py or module/__init__.py
	candidates := []string{
		filepath.Join(targetDir, modulePath+".py"),
		filepath.Join(targetDir, modulePath, "__init__.py"),
	}

	for _, candidate := range candidates {
		for _, f := range allFiles {
			if f == candidate {
				return f, nil
			}
		}
	}

	return "", nil // Not found in files list
}

// resolvePythonAbsoluteImport tries to resolve an absolute import to a local file.
// Example: "internal.types" -> "internal/types.py" or "internal/types/__init__.py"
// Returns empty string if the import cannot be resolved to a file in the files list.
func resolvePythonAbsoluteImport(imp, repoRoot string, allFiles []string) string {
	// Convert import path to file path
	modulePath := strings.ReplaceAll(imp, ".", string(filepath.Separator))

	// Try two possibilities: module.py or module/__init__.py
	candidates := []string{
		filepath.Join(repoRoot, modulePath+".py"),
		filepath.Join(repoRoot, modulePath, "__init__.py"),
	}

	for _, candidate := range candidates {
		for _, f := range allFiles {
			if f == candidate {
				return f
			}
		}
	}

	return "" // Not a local import or not in files list
}
