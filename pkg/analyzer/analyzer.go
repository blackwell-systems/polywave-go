package analyzer

import (
	"context"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/result"
)

// Analyzer parses Go source files and extracts import dependencies.
type Analyzer struct {
	fset *token.FileSet
}

// New creates a new Analyzer instance.
func New() *Analyzer {
	return &Analyzer{fset: token.NewFileSet()}
}

// ParseFile parses a Go source file at the given path and returns its AST.
// Returns result.Result failure if file cannot be read or contains syntax errors.
// Respects context cancellation before performing I/O.
func (a *Analyzer) ParseFile(ctx context.Context, path string) result.Result[*ast.File] {
	if err := ctx.Err(); err != nil {
		return result.NewFailure[*ast.File]([]result.SAWError{
			result.NewFatal(result.CodeAnalyzeParseFailed, err.Error()),
		})
	}
	file, err := parser.ParseFile(a.fset, path, nil, parser.ImportsOnly)
	if err != nil {
		return result.NewFailure[*ast.File]([]result.SAWError{
			result.NewFatal(result.CodeAnalyzeParseFailed, fmt.Sprintf("failed to parse %s: %s", path, err.Error())),
		})
	}
	return result.NewSuccess(file)
}

// ExtractImports returns all local import paths from a parsed AST.
// Filters out stdlib imports (no dot in first path element).
// For local imports like "github.com/user/repo/pkg/foo", resolves to absolute file path.
// Respects context cancellation before performing I/O.
func (a *Analyzer) ExtractImports(ctx context.Context, file *ast.File, repoRoot string) result.Result[[]string] {
	if err := ctx.Err(); err != nil {
		return result.NewFailure[[]string]([]result.SAWError{
			result.NewFatal(result.CodeAnalyzeParseFailed, err.Error()),
		})
	}

	// Get module path from go.mod
	modulePath, err := getModulePath(ctx, repoRoot)
	if err != nil {
		return result.NewFailure[[]string]([]result.SAWError{
			result.NewFatal(result.CodeAnalyzeGomodReadFailed, fmt.Sprintf("failed to get module path: %s", err.Error())),
		})
	}

	var imports []string
	for _, imp := range file.Imports {
		// Remove quotes from import path
		importPath := strings.Trim(imp.Path.Value, `"`)

		// Skip stdlib imports
		if IsStdlib(importPath) {
			continue
		}

		// Only process imports that belong to this module
		if !strings.HasPrefix(importPath, modulePath) {
			continue
		}

		// Resolve to absolute path
		resolved := ResolveImportPath(ctx, importPath, repoRoot, modulePath)
		if !resolved.IsSuccess() {
			return result.NewFailure[[]string](resolved.Errors)
		}

		imports = append(imports, resolved.GetData())
	}

	return result.NewSuccess(imports)
}

// IsStdlib returns true if the import path is a Go stdlib package.
// Heuristic: first path element has no dot (e.g. "fmt", "os", "go/ast").
func IsStdlib(importPath string) bool {
	// New: first path element has no dot = stdlib
	parts := strings.SplitN(importPath, "/", 2)
	return !strings.Contains(parts[0], ".")
}

// ResolveImportPath converts a local import path to an absolute file path.
// Example: "github.com/blackwell-systems/scout-and-wave-go/pkg/protocol"
// -> "/Users/.../scout-and-wave-go/pkg/protocol" (directory, not file — caller scans .go files)
func ResolveImportPath(ctx context.Context, importPath, repoRoot, modulePath string) result.Result[string] {
	// Strip the module prefix
	if !strings.HasPrefix(importPath, modulePath) {
		return result.NewFailure[string]([]result.SAWError{
			result.NewFatal(result.CodeAnalyzeImportResolveFailed,
				fmt.Sprintf("import path %q does not start with module path %q", importPath, modulePath)),
		})
	}

	relPath := strings.TrimPrefix(importPath, modulePath)
	relPath = strings.TrimPrefix(relPath, "/")

	// Construct absolute path
	absPath := filepath.Join(repoRoot, relPath)

	// Verify directory exists
	info, err := os.Stat(absPath)
	if err != nil {
		return result.NewFailure[string]([]result.SAWError{
			result.NewFatal(result.CodeAnalyzeImportResolveFailed,
				fmt.Sprintf("import path resolves to non-existent directory: %s", err.Error())),
		})
	}

	if !info.IsDir() {
		return result.NewFailure[string]([]result.SAWError{
			result.NewFatal(result.CodeAnalyzeImportResolveFailed,
				fmt.Sprintf("import path resolves to file, not directory: %s", absPath)),
		})
	}

	return result.NewSuccess(absPath)
}

// getModulePath reads the go.mod file and extracts the module path.
// Respects context cancellation before performing file I/O.
func getModulePath(ctx context.Context, repoRoot string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	goModPath := filepath.Join(repoRoot, "go.mod")
	content, err := os.ReadFile(goModPath)
	if err != nil {
		return "", fmt.Errorf("failed to read go.mod: %w", err)
	}

	lines := strings.Split(string(content), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "module ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "module")), nil
		}
	}

	return "", fmt.Errorf("module directive not found in go.mod")
}
