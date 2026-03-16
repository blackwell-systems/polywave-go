package protocol

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// IsIntegrationRequired returns true if the export name and category suggest
// this symbol needs wiring into caller files. Uses naming heuristics.
func IsIntegrationRequired(exportName string, category string) bool {
	if category == "none" {
		return false
	}
	if category == "field_init" {
		return true
	}

	// Query-like functions don't need integration wiring
	queryPrefixes := []string{"Is", "Has", "Get", "String", "Format", "Validate"}
	for _, p := range queryPrefixes {
		if strings.HasPrefix(exportName, p) {
			return false
		}
	}

	// Constants and variables are consumed at import time
	if category == "const" || category == "var" {
		return false
	}

	// Action/builder prefixes need wiring
	actionPrefixes := []string{"Build", "Create", "New", "Register", "Setup", "Init", "Run", "Start", "Wire"}
	for _, p := range actionPrefixes {
		if strings.HasPrefix(exportName, p) {
			return true
		}
	}

	// Action/builder suffixes need wiring
	actionSuffixes := []string{"Handler", "Middleware", "Factory", "Builder"}
	for _, s := range actionSuffixes {
		if strings.HasSuffix(exportName, s) {
			return true
		}
	}

	return false
}

// ClassifyExport determines the integration category for an exported symbol.
// kind is the Go AST node kind: "func", "type", "method", "field", "var", "const"
// Returns: "function_call", "type_usage", "field_init", or "none"
func ClassifyExport(exportName string, kind string) string {
	switch kind {
	case "func", "method":
		return "function_call"
	case "type":
		return "type_usage"
	case "field":
		return "field_init"
	case "var", "const":
		return "none"
	default:
		return "none"
	}
}

// goListPackage is a subset of the JSON output from `go list -json`.
type goListPackage struct {
	ImportPath string   `json:"ImportPath"`
	Dir        string   `json:"Dir"`
	Imports    []string `json:"Imports"`
	GoFiles    []string `json:"GoFiles"`
}

// SuggestCallers searches the codebase for files that likely need to call/use
// the given export. Uses import graph (go list) and naming heuristics.
func SuggestCallers(repoPath string, exportPkg string, exportName string) ([]string, error) {
	callers, err := suggestCallersViaGoList(repoPath, exportPkg)
	if err != nil {
		// Fall back to grep-based approach
		return suggestCallersViaGrep(repoPath, exportPkg)
	}
	return callers, nil
}

// suggestCallersViaGoList uses `go list -json ./...` to find files importing exportPkg.
func suggestCallersViaGoList(repoPath string, exportPkg string) ([]string, error) {
	cmd := exec.Command("go", "list", "-json", "./...")
	cmd.Dir = repoPath
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	var callers []string
	dec := json.NewDecoder(strings.NewReader(string(out)))
	for dec.More() {
		var pkg goListPackage
		if err := dec.Decode(&pkg); err != nil {
			continue
		}
		if pkg.ImportPath == exportPkg {
			continue // skip the package itself
		}
		for _, imp := range pkg.Imports {
			if imp == exportPkg {
				// This package imports the export's package — add its Go files
				for _, f := range pkg.GoFiles {
					callers = append(callers, filepath.Join(pkg.Dir, f))
				}
				break
			}
		}
	}
	return callers, nil
}

// suggestCallersViaGrep falls back to scanning import statements for the package path.
func suggestCallersViaGrep(repoPath string, exportPkg string) ([]string, error) {
	var callers []string
	err := filepath.Walk(repoPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // skip errors
		}
		if info.IsDir() {
			name := info.Name()
			if name == "vendor" || name == ".git" || name == "node_modules" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		content := string(data)
		// Check if file imports the target package
		if strings.Contains(content, `"`+exportPkg+`"`) {
			callers = append(callers, path)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return callers, nil
}
