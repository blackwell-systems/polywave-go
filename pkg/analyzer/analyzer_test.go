package analyzer

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseFile_ValidGo(t *testing.T) {
	// Create temp directory with a valid Go file
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "main.go")

	content := `package main

import (
	"fmt"
	"github.com/blackwell-systems/scout-and-wave-go/pkg/protocol"
)

func main() {
	fmt.Println("hello")
}
`
	if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	a := New()
	file, err := a.ParseFile(context.Background(), testFile)
	if err != nil {
		t.Fatalf("ParseFile() error = %v", err)
	}

	if file == nil {
		t.Fatal("ParseFile() returned nil file")
	}

	if file.Name.Name != "main" {
		t.Errorf("expected package name 'main', got %q", file.Name.Name)
	}

	if len(file.Imports) != 2 {
		t.Errorf("expected 2 imports, got %d", len(file.Imports))
	}
}

func TestParseFile_SyntaxError(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "invalid.go")

	// Syntax error in package declaration (before imports)
	content := `package 123invalid

import "fmt"

func main() {}
`
	if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	a := New()
	_, err := a.ParseFile(context.Background(), testFile)
	if err == nil {
		t.Fatal("ParseFile() expected error for syntax error, got nil")
	}
}

func TestExtractImports_LocalOnly(t *testing.T) {
	// Create temp repo structure
	tmpDir := t.TempDir()
	pkgDir := filepath.Join(tmpDir, "pkg", "protocol")
	if err := os.MkdirAll(pkgDir, 0755); err != nil {
		t.Fatalf("failed to create pkg dir: %v", err)
	}

	// Create go.mod
	goModContent := `module github.com/blackwell-systems/scout-and-wave-go

go 1.25.0
`
	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(goModContent), 0644); err != nil {
		t.Fatalf("failed to write go.mod: %v", err)
	}

	// Create test file with mixed imports
	testFile := filepath.Join(tmpDir, "main.go")
	content := `package main

import (
	"fmt"
	"net/http"
	"github.com/blackwell-systems/scout-and-wave-go/pkg/protocol"
)

func main() {}
`
	if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	a := New()
	file, err := a.ParseFile(context.Background(), testFile)
	if err != nil {
		t.Fatalf("ParseFile() error = %v", err)
	}

	imports, err := a.ExtractImports(context.Background(), file, tmpDir)
	if err != nil {
		t.Fatalf("ExtractImports() error = %v", err)
	}

	// Should only have local import, stdlib filtered
	if len(imports) != 1 {
		t.Fatalf("expected 1 local import, got %d: %v", len(imports), imports)
	}

	expectedPath := filepath.Join(tmpDir, "pkg", "protocol")
	if imports[0] != expectedPath {
		t.Errorf("expected import path %q, got %q", expectedPath, imports[0])
	}
}

func TestExtractImports_NoImports(t *testing.T) {
	tmpDir := t.TempDir()

	// Create go.mod
	goModContent := `module github.com/blackwell-systems/scout-and-wave-go

go 1.25.0
`
	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(goModContent), 0644); err != nil {
		t.Fatalf("failed to write go.mod: %v", err)
	}

	testFile := filepath.Join(tmpDir, "noimports.go")
	content := `package main

func main() {}
`
	if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	a := New()
	file, err := a.ParseFile(context.Background(), testFile)
	if err != nil {
		t.Fatalf("ParseFile() error = %v", err)
	}

	imports, err := a.ExtractImports(context.Background(), file, tmpDir)
	if err != nil {
		t.Fatalf("ExtractImports() error = %v", err)
	}

	if len(imports) != 0 {
		t.Errorf("expected 0 imports, got %d: %v", len(imports), imports)
	}
}

func TestIsStdlib_StdlibPackages(t *testing.T) {
	tests := []struct {
		name       string
		importPath string
		want       bool
	}{
		{"simple stdlib", "fmt", true},
		{"stdlib with slash", "net/http", true},
		{"encoding package", "encoding/json", true},
		{"crypto package", "crypto/sha256", true},
		{"go package", "go/ast", true},
		{"testing package", "testing", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsStdlib(tt.importPath)
			if got != tt.want {
				t.Errorf("IsStdlib(%q) = %v, want %v", tt.importPath, got, tt.want)
			}
		})
	}
}

func TestIsStdlib_LocalPackages(t *testing.T) {
	tests := []struct {
		name       string
		importPath string
		want       bool
	}{
		{"github package", "github.com/user/repo/pkg", false},
		{"gitlab package", "gitlab.com/user/repo", false},
		{"custom domain", "example.com/pkg/foo", false},
		{"local relative", "./pkg/local", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsStdlib(tt.importPath)
			if got != tt.want {
				t.Errorf("IsStdlib(%q) = %v, want %v", tt.importPath, got, tt.want)
			}
		})
	}
}

func TestResolveImportPath_Success(t *testing.T) {
	tmpDir := t.TempDir()
	pkgDir := filepath.Join(tmpDir, "pkg", "protocol")
	if err := os.MkdirAll(pkgDir, 0755); err != nil {
		t.Fatalf("failed to create pkg dir: %v", err)
	}

	modulePath := "github.com/blackwell-systems/scout-and-wave-go"
	importPath := "github.com/blackwell-systems/scout-and-wave-go/pkg/protocol"

	resolved, err := ResolveImportPath(importPath, tmpDir, modulePath)
	if err != nil {
		t.Fatalf("ResolveImportPath() error = %v", err)
	}

	expectedPath := pkgDir
	if resolved != expectedPath {
		t.Errorf("ResolveImportPath() = %q, want %q", resolved, expectedPath)
	}
}

func TestResolveImportPath_NonExistentDir(t *testing.T) {
	tmpDir := t.TempDir()
	modulePath := "github.com/blackwell-systems/scout-and-wave-go"
	importPath := "github.com/blackwell-systems/scout-and-wave-go/pkg/nonexistent"

	_, err := ResolveImportPath(importPath, tmpDir, modulePath)
	if err == nil {
		t.Fatal("ResolveImportPath() expected error for non-existent directory, got nil")
	}

	if !strings.Contains(err.Error(), "non-existent directory") {
		t.Errorf("expected 'non-existent directory' in error, got: %v", err)
	}
}

func TestResolveImportPath_WrongModule(t *testing.T) {
	tmpDir := t.TempDir()
	modulePath := "github.com/blackwell-systems/scout-and-wave-go"
	importPath := "github.com/other/repo/pkg/foo"

	_, err := ResolveImportPath(importPath, tmpDir, modulePath)
	if err == nil {
		t.Fatal("ResolveImportPath() expected error for wrong module, got nil")
	}

	if !strings.Contains(err.Error(), "does not start with module path") {
		t.Errorf("expected 'does not start with module path' in error, got: %v", err)
	}
}

func TestGetModulePath(t *testing.T) {
	tmpDir := t.TempDir()

	goModContent := `module github.com/blackwell-systems/scout-and-wave-go

go 1.25.0

require (
	github.com/spf13/cobra v1.10.2
)
`
	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(goModContent), 0644); err != nil {
		t.Fatalf("failed to write go.mod: %v", err)
	}

	modulePath, err := getModulePath(context.Background(), tmpDir)
	if err != nil {
		t.Fatalf("getModulePath() error = %v", err)
	}

	expected := "github.com/blackwell-systems/scout-and-wave-go"
	if modulePath != expected {
		t.Errorf("getModulePath() = %q, want %q", modulePath, expected)
	}
}

func TestNew(t *testing.T) {
	a := New()
	if a == nil {
		t.Fatal("New() returned nil")
	}
	if a.fset == nil {
		t.Error("New() created Analyzer with nil fset")
	}
}

// Helper to verify AST structure
func TestParseFile_ImportsStructure(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.go")

	content := `package test

import (
	"fmt"
	"net/http"
)
`
	if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	a := New()
	file, err := a.ParseFile(context.Background(), testFile)
	if err != nil {
		t.Fatalf("ParseFile() error = %v", err)
	}

	// Verify import structure
	for _, imp := range file.Imports {
		if imp.Path == nil {
			t.Error("import has nil Path")
		}
		if imp.Path.Value == "" {
			t.Error("import has empty Path.Value")
		}
		// Should have quotes
		if !strings.HasPrefix(imp.Path.Value, `"`) {
			t.Errorf("import path missing quotes: %s", imp.Path.Value)
		}
	}
}

// Test edge case: file with C imports
func TestExtractImports_CgoImport(t *testing.T) {
	tmpDir := t.TempDir()

	goModContent := `module github.com/blackwell-systems/scout-and-wave-go

go 1.25.0
`
	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(goModContent), 0644); err != nil {
		t.Fatalf("failed to write go.mod: %v", err)
	}

	testFile := filepath.Join(tmpDir, "cgo.go")
	content := `package main

import "C"

func main() {}
`
	if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	a := New()
	file, err := a.ParseFile(context.Background(), testFile)
	if err != nil {
		t.Fatalf("ParseFile() error = %v", err)
	}

	imports, err := a.ExtractImports(context.Background(), file, tmpDir)
	if err != nil {
		t.Fatalf("ExtractImports() error = %v", err)
	}

	// C import should be filtered as stdlib
	if len(imports) != 0 {
		t.Errorf("expected 0 imports (C filtered), got %d: %v", len(imports), imports)
	}
}

func TestParseFile_CancelledContext(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "main.go")

	content := `package main

func main() {}
`
	if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel before calling ParseFile

	a := New()
	_, err := a.ParseFile(ctx, testFile)
	if err == nil {
		t.Fatal("ParseFile() expected error for cancelled context, got nil")
	}
}

func TestExtractImports_CancelledContext(t *testing.T) {
	tmpDir := t.TempDir()
	pkgDir := filepath.Join(tmpDir, "pkg", "protocol")
	if err := os.MkdirAll(pkgDir, 0755); err != nil {
		t.Fatalf("failed to create pkg dir: %v", err)
	}

	goModContent := `module github.com/blackwell-systems/scout-and-wave-go

go 1.25.0
`
	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(goModContent), 0644); err != nil {
		t.Fatalf("failed to write go.mod: %v", err)
	}

	testFile := filepath.Join(tmpDir, "main.go")
	content := `package main

import "fmt"

func main() { _ = fmt.Sprintf("") }
`
	if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	a := New()
	file, err := a.ParseFile(context.Background(), testFile)
	if err != nil {
		t.Fatalf("ParseFile() error = %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel before calling ExtractImports

	_, err = a.ExtractImports(ctx, file, tmpDir)
	if err == nil {
		t.Fatal("ExtractImports() expected error for cancelled context, got nil")
	}
}
