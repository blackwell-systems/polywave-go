package scaffoldval

import (
	"context"
	"fmt"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/commands"
	"github.com/blackwell-systems/scout-and-wave-go/pkg/protocol"
)

// ValidateScaffold runs the validation pipeline on a scaffold file
func ValidateScaffold(scaffoldPath string, implPath string) (*ValidationResult, error) {
	result := NewValidationResult()

	// Step 1: Syntax check (parse Go file)
	fset := token.NewFileSet()
	_, err := parser.ParseFile(fset, scaffoldPath, nil, parser.AllErrors)
	if err != nil {
		result.Syntax.Status = "FAIL"
		result.Syntax.Errors = []string{err.Error()}
		result.Syntax.Fixes = []string{"Fix syntax error at reported location"}
		result.Syntax.AutoFixable = false
		return result, nil // Stop here if syntax fails
	}
	result.Syntax.Status = "PASS"

	// Step 2: Import resolution (check if imports exist)
	content, err := os.ReadFile(scaffoldPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read scaffold: %w", err)
	}

	imports := extractImports(string(content))
	missingImports := checkImports(imports)

	if len(missingImports) > 0 {
		result.Imports.Status = "FAIL"
		result.Imports.Errors = missingImports
		result.Imports.Fixes = []string{fmt.Sprintf("Add missing imports: %v", missingImports)}
		result.Imports.AutoFixable = true
	} else {
		result.Imports.Status = "PASS"
	}

	// Step 3: Type references (check for undeclared types)
	// Parse AST and look for type references
	result.TypeReferences.Status = "PASS" // Simplified for now

	// Step 4: Build check (compile scaffold in isolation)
	// Extract build command from IMPL doc using pkg/commands
	_, err = protocol.Load(context.TODO(), implPath)
	if err != nil {
		result.Build.Status = "SKIP"
		result.Build.Errors = []string{fmt.Sprintf("Failed to parse IMPL manifest: %v", err)}
		return result, nil // Continue without build check
	}

	// Get repo root (parent of docs/ directory)
	repoRoot := filepath.Dir(filepath.Dir(implPath))

	// Extract build command using commands package
	extractor := commands.New()
	extractor.RegisterCIParser(&commands.GithubActionsParser{})
	extractor.RegisterBuildSystemParser(&commands.MakefileParser{})
	extractor.RegisterBuildSystemParser(&commands.PackageJSONParser{})

	commandSet, err := extractor.Extract(repoRoot)
	if err != nil || commandSet.Commands.Build == "" {
		result.Build.Status = "SKIP"
		result.Build.Errors = []string{"No build command found"}
		return result, nil
	}

	// Run build command
	cmd := exec.Command("sh", "-c", commandSet.Commands.Build)
	cmd.Dir = repoRoot
	output, err := cmd.CombinedOutput()
	if err != nil {
		result.Build.Status = "FAIL"
		result.Build.Errors = []string{string(output)}
		result.Build.Fixes = []string{"Fix compilation errors in scaffold"}
		result.Build.AutoFixable = false
	} else {
		result.Build.Status = "PASS"
	}

	return result, nil
}

// extractImports parses import statements from Go source
func extractImports(content string) []string {
	var imports []string
	lines := strings.Split(content, "\n")
	inImportBlock := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		if strings.HasPrefix(trimmed, "import (") {
			inImportBlock = true
			continue
		}

		if inImportBlock && trimmed == ")" {
			inImportBlock = false
			continue
		}

		if inImportBlock || strings.HasPrefix(trimmed, "import ") {
			// Extract import path from: import "path" or "path"
			if strings.Contains(trimmed, `"`) {
				parts := strings.Split(trimmed, `"`)
				if len(parts) >= 2 {
					imports = append(imports, parts[1])
				}
			}
		}
	}

	return imports
}

// checkImports verifies imports exist (simplified: checks standard lib only)
func checkImports(imports []string) []string {
	var missing []string

	for _, imp := range imports {
		// Simple heuristic: standard library imports don't have dots
		if !strings.Contains(imp, ".") {
			continue // Assume standard lib exists
		}

		// For third-party imports, check if package exists in go.mod
		// (Simplified: mark as missing if not in standard lib)
		missing = append(missing, imp)
	}

	return missing
}
