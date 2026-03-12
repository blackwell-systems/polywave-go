package analyzer

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

// parseRustFiles parses Rust files using an external rust-parser binary.
// It executes rust-parser for each file and parses the JSON output.
// Returns a map of file path -> list of absolute paths to local dependencies.
// Filters out stdlib (std::, core::, alloc::) and external crates.
//
// The rust-parser binary is expected to output JSON in the format:
// {"file": "path.rs", "imports": ["crate::foo", "std::vec", "super::bar"]}
func parseRustFiles(repoRoot string, files []string) (map[string][]string, error) {
	// Check if rust-parser binary exists
	parserPath, err := exec.LookPath("rust-parser")
	if err != nil {
		return nil, fmt.Errorf("rust-parser binary not found in PATH: %w", err)
	}

	fileImports := make(map[string][]string)

	for _, file := range files {
		// Execute rust-parser for this file
		cmd := exec.Command(parserPath, file)
		cmd.Dir = repoRoot

		output, err := cmd.CombinedOutput()
		if err != nil {
			return nil, fmt.Errorf("rust-parser failed for %s: %w (output: %s)", file, err, string(output))
		}

		// Parse JSON output
		var result struct {
			File    string   `json:"file"`
			Imports []string `json:"imports"`
		}

		if err := json.Unmarshal(output, &result); err != nil {
			return nil, fmt.Errorf("failed to parse rust-parser JSON output for %s: %w", file, err)
		}

		// Filter and resolve imports
		var localDeps []string
		for _, imp := range result.Imports {
			// Skip stdlib imports
			if isRustStdlib(imp) {
				continue
			}

			// Skip external crate imports (they don't start with crate::, super::, or self::)
			if !isLocalRustImport(imp) {
				continue
			}

			// Resolve local import to absolute file path
			resolved, err := resolveRustImport(repoRoot, file, imp, files)
			if err != nil {
				// Skip unresolvable imports (might be in different files not in our list)
				continue
			}

			localDeps = append(localDeps, resolved)
		}

		fileImports[file] = localDeps
	}

	return fileImports, nil
}

// isRustStdlib returns true if the import path is a Rust standard library import.
// Standard library imports start with std::, core::, or alloc::.
func isRustStdlib(importPath string) bool {
	return strings.HasPrefix(importPath, "std::") ||
		strings.HasPrefix(importPath, "core::") ||
		strings.HasPrefix(importPath, "alloc::")
}

// isLocalRustImport returns true if the import is a local crate import.
// Local imports start with crate::, super::, or self::.
func isLocalRustImport(importPath string) bool {
	return strings.HasPrefix(importPath, "crate::") ||
		strings.HasPrefix(importPath, "super::") ||
		strings.HasPrefix(importPath, "self::")
}

// resolveRustImport converts a Rust import path to an absolute file path.
// This is a simplified implementation that handles basic cases:
// - crate::module -> repoRoot/src/module.rs or repoRoot/src/module/mod.rs
// - super::module -> parent_dir/module.rs
// - self::module -> same_dir/module.rs
//
// Returns the absolute path if found in the files list, otherwise returns an error.
func resolveRustImport(repoRoot, currentFile, importPath string, allFiles []string) (string, error) {
	currentDir := filepath.Dir(currentFile)

	var targetPath string

	switch {
	case strings.HasPrefix(importPath, "crate::"):
		// crate:: refers to the crate root (typically src/)
		// Extract the module path after crate::
		modulePath := strings.TrimPrefix(importPath, "crate::")
		moduleParts := strings.Split(modulePath, "::")

		// Try src/module.rs first
		srcDir := filepath.Join(repoRoot, "src")
		targetPath = filepath.Join(srcDir, strings.Join(moduleParts, string(filepath.Separator))+".rs")

		// If not found, try src/module/mod.rs
		if !fileExists(targetPath, allFiles) {
			targetPath = filepath.Join(srcDir, strings.Join(moduleParts, string(filepath.Separator)), "mod.rs")
		}

	case strings.HasPrefix(importPath, "super::"):
		// super:: refers to the parent module
		modulePath := strings.TrimPrefix(importPath, "super::")
		moduleParts := strings.Split(modulePath, "::")

		parentDir := filepath.Dir(currentDir)
		targetPath = filepath.Join(parentDir, strings.Join(moduleParts, string(filepath.Separator))+".rs")

		if !fileExists(targetPath, allFiles) {
			targetPath = filepath.Join(parentDir, strings.Join(moduleParts, string(filepath.Separator)), "mod.rs")
		}

	case strings.HasPrefix(importPath, "self::"):
		// self:: refers to the current module
		modulePath := strings.TrimPrefix(importPath, "self::")
		moduleParts := strings.Split(modulePath, "::")

		targetPath = filepath.Join(currentDir, strings.Join(moduleParts, string(filepath.Separator))+".rs")

		if !fileExists(targetPath, allFiles) {
			targetPath = filepath.Join(currentDir, strings.Join(moduleParts, string(filepath.Separator)), "mod.rs")
		}

	default:
		return "", fmt.Errorf("unsupported import path format: %s", importPath)
	}

	// Check if the resolved path is in our files list
	if fileExists(targetPath, allFiles) {
		return targetPath, nil
	}

	return "", fmt.Errorf("resolved import path %s not found in files list", targetPath)
}

// fileExists checks if a file path exists in the given list of files.
func fileExists(path string, files []string) bool {
	for _, f := range files {
		if f == path {
			return true
		}
	}
	return false
}
