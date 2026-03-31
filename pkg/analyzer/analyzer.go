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
// Returns error if file cannot be read or contains syntax errors.
// Respects context cancellation before performing I/O.
func (a *Analyzer) ParseFile(ctx context.Context, path string) (*ast.File, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return parser.ParseFile(a.fset, path, nil, parser.ImportsOnly)
}

// ExtractImports returns all local import paths from a parsed AST.
// Filters out stdlib imports (no slash in path, or starts with known stdlib prefix).
// For local imports like "github.com/user/repo/pkg/foo", resolves to absolute file path.
// Respects context cancellation before performing I/O.
func (a *Analyzer) ExtractImports(ctx context.Context, file *ast.File, repoRoot string) ([]string, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	var imports []string

	// Get module path from go.mod
	modulePath, err := getModulePath(ctx, repoRoot)
	if err != nil {
		return nil, fmt.Errorf("failed to get module path: %w", err)
	}

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
		resolved, err := ResolveImportPath(importPath, repoRoot, modulePath)
		if err != nil {
			return nil, err
		}

		imports = append(imports, resolved)
	}

	return imports, nil
}

// IsStdlib returns true if the import path is a Go stdlib package.
// Heuristic: no slash OR starts with known stdlib prefix (encoding/, net/, etc).
func IsStdlib(importPath string) bool {
	// No slash means it's a stdlib package (e.g., "fmt", "os")
	if !strings.Contains(importPath, "/") {
		return true
	}

	// Known stdlib prefixes
	stdlibPrefixes := []string{
		"archive/",
		"bufio/",
		"builtin/",
		"bytes/",
		"cmp/",
		"compress/",
		"container/",
		"context/",
		"crypto/",
		"database/",
		"debug/",
		"embed/",
		"encoding/",
		"errors/",
		"expvar/",
		"flag/",
		"fmt/",
		"go/",
		"hash/",
		"html/",
		"image/",
		"index/",
		"io/",
		"iter/",
		"log/",
		"maps/",
		"math/",
		"mime/",
		"net/",
		"os/",
		"path/",
		"plugin/",
		"reflect/",
		"regexp/",
		"runtime/",
		"slices/",
		"sort/",
		"strconv/",
		"strings/",
		"sync/",
		"syscall/",
		"testing/",
		"text/",
		"time/",
		"unicode/",
		"unsafe/",
	}

	for _, prefix := range stdlibPrefixes {
		if strings.HasPrefix(importPath, prefix) {
			return true
		}
	}

	return false
}

// ResolveImportPath converts a local import path to an absolute file path.
// Example: "github.com/blackwell-systems/scout-and-wave-go/pkg/protocol"
// -> "/Users/.../scout-and-wave-go/pkg/protocol" (directory, not file — caller scans .go files)
func ResolveImportPath(importPath, repoRoot, modulePath string) (string, error) {
	// Strip the module prefix
	if !strings.HasPrefix(importPath, modulePath) {
		return "", fmt.Errorf("import path %q does not start with module path %q", importPath, modulePath)
	}

	relPath := strings.TrimPrefix(importPath, modulePath)
	relPath = strings.TrimPrefix(relPath, "/")

	// Construct absolute path
	absPath := filepath.Join(repoRoot, relPath)

	// Verify directory exists
	info, err := os.Stat(absPath)
	if err != nil {
		return "", fmt.Errorf("import path resolves to non-existent directory: %w", err)
	}

	if !info.IsDir() {
		return "", fmt.Errorf("import path resolves to file, not directory: %s", absPath)
	}

	return absPath, nil
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
