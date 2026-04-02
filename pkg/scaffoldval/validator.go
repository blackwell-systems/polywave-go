package scaffoldval

import (
	"context"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/commands"
	"github.com/blackwell-systems/scout-and-wave-go/pkg/protocol"
	"github.com/blackwell-systems/scout-and-wave-go/pkg/result"
)

// ValidateScaffold runs the validation pipeline on a scaffold file.
// ctx is used to cancel the build step. repoRoot, if non-empty, is used directly;
// if empty, the function walks upward from implPath looking for go.mod.
func ValidateScaffold(ctx context.Context, scaffoldPath string, implPath string, repoRoot string) result.Result[*ValidationResult] {
	vr := NewValidationResult()

	// Step 1: Syntax check (parse Go file)
	fset := token.NewFileSet()
	astFile, err := parser.ParseFile(fset, scaffoldPath, nil, parser.AllErrors)
	if err != nil {
		vr.Syntax.Status = "FAIL"
		vr.Syntax.Errors = []string{err.Error()}
		vr.Syntax.Fixes = []string{"Fix syntax error at reported location"}
		vr.Syntax.AutoFixable = false
		return result.NewSuccess(vr) // Stop here if syntax fails
	}
	vr.Syntax.Status = "PASS"

	// Step 2: Import resolution (check if imports exist)
	// Determine repo root before checking imports
	root := repoRoot
	if root == "" {
		root = findRepoRoot(filepath.Dir(implPath))
	}

	imports := importsFromAST(astFile)
	missingImports := checkImports(imports, root)

	if len(missingImports) > 0 {
		vr.Imports.Status = "FAIL"
		vr.Imports.Errors = missingImports
		vr.Imports.Fixes = []string{fmt.Sprintf("Add missing imports: %v", missingImports)}
		vr.Imports.AutoFixable = true
	} else {
		vr.Imports.Status = "PASS"
	}

	// Step 3: Type references (check for undeclared types)
	// Type reference checking is not yet implemented. Return SKIP (honest signaling)
	// rather than a false PASS. The parsed AST (astFile) is available for future use.
	// astFile is available here for future type-reference checking
	_ = astFile
	vr.TypeReferences.Status = "SKIP"

	// Step 4: Build check (compile scaffold in isolation)
	// Extract build command from IMPL doc using pkg/commands
	_, err = protocol.Load(ctx, implPath)
	if err != nil {
		vr.Build.Status = "SKIP"
		vr.Build.Errors = []string{fmt.Sprintf("Failed to parse IMPL manifest: %v", err)}
		return result.NewSuccess(vr) // Continue without build check
	}

	// Extract build command using commands package
	extractor := commands.New()
	extractor.RegisterCIParser(&commands.GithubActionsParser{})
	extractor.RegisterBuildSystemParser(&commands.MakefileParser{})
	extractor.RegisterBuildSystemParser(&commands.PackageJSONParser{})

	r := extractor.Extract(ctx, root)
	if r.IsFatal() {
		vr.Build.Status = "SKIP"
		vr.Build.Errors = []string{"No build command found"}
		return result.NewSuccess(vr)
	}
	commandSet := r.GetData().CommandSet
	if commandSet == nil || commandSet.Commands.Build == "" {
		vr.Build.Status = "SKIP"
		vr.Build.Errors = []string{"No build command found"}
		return result.NewSuccess(vr)
	}

	// Run build command with cancellable context
	cmd := exec.CommandContext(ctx, "sh", "-c", commandSet.Commands.Build)
	cmd.Dir = root
	output, err := cmd.CombinedOutput()
	if err != nil {
		vr.Build.Status = "FAIL"
		vr.Build.Errors = []string{string(output)}
		vr.Build.Fixes = []string{"Fix compilation errors in scaffold"}
		vr.Build.AutoFixable = false
	} else {
		vr.Build.Status = "PASS"
	}

	return result.NewSuccess(vr)
}

// findRepoRoot walks upward from startDir looking for go.mod.
// Returns the directory containing go.mod if found.
// Falls back to filepath.Dir(startDir) if go.mod is not found.
func findRepoRoot(startDir string) string {
	dir := startDir
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			// Reached filesystem root without finding go.mod
			return filepath.Dir(startDir) // fallback
		}
		dir = parent
	}
}

// importsFromAST extracts import paths from a parsed *ast.File.
func importsFromAST(f *ast.File) []string {
	var imports []string
	for _, imp := range f.Imports {
		// imp.Path.Value is a quoted string like `"fmt"`
		path := strings.Trim(imp.Path.Value, `"`)
		imports = append(imports, path)
	}
	return imports
}

// extractImports parses import statements from Go source.
// Kept for backward compatibility with TestExtractImports.
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

// parseGoMod parses a go.mod file and returns the module name and all required module paths.
func parseGoMod(goModPath string) (string, []string) {
	data, err := os.ReadFile(goModPath)
	if err != nil {
		return "", nil
	}
	var moduleName string
	var requires []string
	lines := strings.Split(string(data), "\n")
	inRequire := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "module ") {
			moduleName = strings.TrimSpace(strings.TrimPrefix(trimmed, "module "))
			continue
		}
		if trimmed == "require (" {
			inRequire = true
			continue
		}
		if inRequire && trimmed == ")" {
			inRequire = false
			continue
		}
		if inRequire {
			// line is like: github.com/foo/bar v1.2.3 or github.com/foo/bar v1.2.3 // indirect
			parts := strings.Fields(trimmed)
			if len(parts) >= 2 {
				requires = append(requires, parts[0])
			}
			continue
		}
		if strings.HasPrefix(trimmed, "require ") {
			// single-line: require github.com/foo/bar v1.2.3
			rest := strings.TrimPrefix(trimmed, "require ")
			parts := strings.Fields(rest)
			if len(parts) >= 1 {
				requires = append(requires, parts[0])
			}
		}
	}
	return moduleName, requires
}

// checkImports verifies imports exist by consulting go.mod.
// repoRoot is the directory containing go.mod.
func checkImports(imports []string, repoRoot string) []string {
	moduleName, requires := parseGoMod(filepath.Join(repoRoot, "go.mod"))
	var missing []string
	for _, imp := range imports {
		if !strings.Contains(imp, ".") {
			continue // standard library
		}
		if moduleName != "" && strings.HasPrefix(imp, moduleName) {
			continue // local package
		}
		found := false
		for _, req := range requires {
			if strings.HasPrefix(imp, req) {
				found = true
				break
			}
		}
		if !found {
			missing = append(missing, imp)
		}
	}
	return missing
}
