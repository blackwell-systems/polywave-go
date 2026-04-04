package analyzer

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

// jsParserOutput represents the JSON output from js-parser.js
type jsParserOutput struct {
	File    string   `json:"file"`
	Imports []string `json:"imports"`
}

// parseJavaScriptFiles parses JavaScript/TypeScript files using the js-parser.js Node.js script.
// It returns a map of file paths to their local dependencies (absolute paths).
// External npm packages are filtered out.
//
// The function requires:
// - node binary in PATH
// - js-parser.js script in the same directory as this file (or discoverable path)
//
// Returns error if node is not found or parsing fails.
func parseJavaScriptFiles(repoRoot string, files []string) (map[string][]string, error) {
	// Check if node binary exists
	nodePath, err := exec.LookPath("node")
	if err != nil {
		return nil, fmt.Errorf("node binary not found: %w (JavaScript parsing requires Node.js)", err)
	}

	// Locate js-parser.js script via PATH lookup.
	parserScript, err := exec.LookPath("js-parser.js")
	if err != nil {
		return nil, fmt.Errorf("js-parser.js not found in PATH: required for JavaScript parsing")
	}

	result := make(map[string][]string)

	for _, file := range files {
		// Execute node js-parser.js <file>
		cmd := exec.Command(nodePath, parserScript, file)
		cmd.Dir = repoRoot

		output, err := cmd.CombinedOutput()
		if err != nil {
			// If the parser script doesn't exist, return a more helpful error
			if strings.Contains(string(output), "Cannot find module") || strings.Contains(string(output), "ENOENT") {
				return nil, fmt.Errorf("js-parser.js not found: %w (required for JavaScript parsing)", err)
			}
			return nil, fmt.Errorf("failed to parse %s: %w (output: %s)", file, err, string(output))
		}

		// Parse JSON output
		var parsed jsParserOutput
		if err := json.Unmarshal(output, &parsed); err != nil {
			return nil, fmt.Errorf("failed to parse JSON output for %s: %w", file, err)
		}

		// Filter and resolve imports
		var localDeps []string
		for _, imp := range parsed.Imports {
			// Skip npm packages (no ./ or ../ prefix)
			if !strings.HasPrefix(imp, "./") && !strings.HasPrefix(imp, "../") {
				continue
			}

			// Resolve relative import to absolute path
			absPath, err := resolveJSImport(file, imp, repoRoot)
			if err != nil {
				// Skip unresolvable imports (e.g., missing files)
				continue
			}

			localDeps = append(localDeps, absPath)
		}

		result[file] = localDeps
	}

	return result, nil
}

// resolveJSImport resolves a relative JavaScript import to an absolute file path.
// Handles:
// - ./utils -> ./utils.js or ./utils/index.js
// - ../types -> ../types.ts or ../types/index.ts
// - ./helpers -> ./helpers.js
//
// JavaScript/TypeScript resolution rules:
// 1. Try exact path with common extensions (.js, .ts, .jsx, .tsx)
// 2. Try path/index with common extensions
// 3. Return error if not found
func resolveJSImport(importer, importPath, repoRoot string) (string, error) {
	importerDir := filepath.Dir(importer)
	basePath := filepath.Join(importerDir, importPath)

	// Extensions to try
	extensions := []string{".js", ".ts", ".jsx", ".tsx", ".mjs", ".cjs"}

	// Try exact path with extensions
	for _, ext := range extensions {
		candidate := basePath + ext
		if fileExists(candidate) {
			return candidate, nil
		}
	}

	// Try index files
	for _, ext := range extensions {
		candidate := filepath.Join(basePath, "index"+ext)
		if fileExists(candidate) {
			return candidate, nil
		}
	}

	// Import cannot be resolved to a known file.
	return "", fmt.Errorf("cannot resolve JS import: %s", importPath)
}

