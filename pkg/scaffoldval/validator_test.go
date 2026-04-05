package scaffoldval

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateScaffold_ValidScaffold(t *testing.T) {
	// Create a temporary directory for test files
	tmpDir := t.TempDir()

	// Create a valid Go scaffold
	scaffoldPath := filepath.Join(tmpDir, "scaffold.go")
	scaffoldContent := `package main

import "fmt"

func Hello() {
	fmt.Println("Hello")
}
`
	if err := os.WriteFile(scaffoldPath, []byte(scaffoldContent), 0644); err != nil {
		t.Fatalf("Failed to create scaffold: %v", err)
	}

	// Create a minimal IMPL manifest
	implPath := filepath.Join(tmpDir, "docs", "IMPL", "test.yaml")
	if err := os.MkdirAll(filepath.Dir(implPath), 0755); err != nil {
		t.Fatalf("Failed to create docs/IMPL directory: %v", err)
	}
	implContent := `impl: test-feature
description: Test feature
waves: []
`
	if err := os.WriteFile(implPath, []byte(implContent), 0644); err != nil {
		t.Fatalf("Failed to create IMPL manifest: %v", err)
	}

	// Create go.mod for language defaults
	goModPath := filepath.Join(tmpDir, "go.mod")
	goModContent := `module example.com/test

go 1.21
`
	if err := os.WriteFile(goModPath, []byte(goModContent), 0644); err != nil {
		t.Fatalf("Failed to create go.mod: %v", err)
	}

	res := ValidateScaffold(context.Background(), scaffoldPath, implPath, "")
	if res.IsFatal() {
		t.Fatalf("ValidateScaffold returned fatal result: %v", res.Errors)
	}

	result := res.GetData()

	if result.Syntax.Status != "PASS" {
		t.Errorf("Expected syntax check to PASS, got %s: %v", result.Syntax.Status, result.Syntax.Errors)
	}

	if result.Imports.Status != "PASS" {
		t.Errorf("Expected import check to PASS, got %s: %v", result.Imports.Status, result.Imports.Errors)
	}

	if result.TypeReferences.Status != "SKIP" && result.TypeReferences.Status != "PASS" {
		t.Errorf("Expected type reference check to be SKIP or PASS, got %s: %v", result.TypeReferences.Status, result.TypeReferences.Errors)
	}

	// Build check will run (may pass or fail depending on environment)
	if result.Build.Status != "PASS" && result.Build.Status != "FAIL" && result.Build.Status != "SKIP" {
		t.Errorf("Expected build check to have valid status, got %s", result.Build.Status)
	}
}

func TestValidateScaffold_SyntaxError(t *testing.T) {
	// Create a temporary directory for test files
	tmpDir := t.TempDir()

	// Create a scaffold with syntax error
	scaffoldPath := filepath.Join(tmpDir, "scaffold.go")
	scaffoldContent := `package main

import "fmt"

func Hello() {
	fmt.Println("Hello"
}  // Missing closing parenthesis
`
	if err := os.WriteFile(scaffoldPath, []byte(scaffoldContent), 0644); err != nil {
		t.Fatalf("Failed to create scaffold: %v", err)
	}

	// Create a minimal IMPL manifest
	implPath := filepath.Join(tmpDir, "docs", "IMPL", "test.yaml")
	if err := os.MkdirAll(filepath.Dir(implPath), 0755); err != nil {
		t.Fatalf("Failed to create docs/IMPL directory: %v", err)
	}
	implContent := `impl: test-feature
description: Test feature
waves: []
`
	if err := os.WriteFile(implPath, []byte(implContent), 0644); err != nil {
		t.Fatalf("Failed to create IMPL manifest: %v", err)
	}

	res := ValidateScaffold(context.Background(), scaffoldPath, implPath, "")
	if res.IsFatal() {
		t.Fatalf("ValidateScaffold returned fatal result: %v", res.Errors)
	}

	result := res.GetData()

	if result.Syntax.Status != "FAIL" {
		t.Errorf("Expected syntax check to FAIL, got %s", result.Syntax.Status)
	}

	if len(result.Syntax.Errors) == 0 {
		t.Error("Expected syntax errors to be reported")
	}

	if result.Syntax.AutoFixable {
		t.Error("Expected syntax error to not be auto-fixable")
	}

	// Other checks should be SKIP after syntax failure
	if result.Imports.Status != "SKIP" {
		t.Errorf("Expected imports check to be SKIP after syntax failure, got %s", result.Imports.Status)
	}
}

func TestValidateScaffold_MissingImport(t *testing.T) {
	// Create a temporary directory for test files
	tmpDir := t.TempDir()

	// Create a scaffold with third-party import
	scaffoldPath := filepath.Join(tmpDir, "scaffold.go")
	scaffoldContent := `package main

import (
	"fmt"
	"github.com/example/missing"
)

func Hello() {
	fmt.Println("Hello")
	missing.DoSomething()
}
`
	if err := os.WriteFile(scaffoldPath, []byte(scaffoldContent), 0644); err != nil {
		t.Fatalf("Failed to create scaffold: %v", err)
	}

	// Create a minimal IMPL manifest
	implPath := filepath.Join(tmpDir, "docs", "IMPL", "test.yaml")
	if err := os.MkdirAll(filepath.Dir(implPath), 0755); err != nil {
		t.Fatalf("Failed to create docs/IMPL directory: %v", err)
	}
	implContent := `impl: test-feature
description: Test feature
waves: []
`
	if err := os.WriteFile(implPath, []byte(implContent), 0644); err != nil {
		t.Fatalf("Failed to create IMPL manifest: %v", err)
	}

	// Create go.mod that does NOT list github.com/example/missing
	goModPath := filepath.Join(tmpDir, "go.mod")
	goModContent := `module example.com/test

go 1.21
`
	if err := os.WriteFile(goModPath, []byte(goModContent), 0644); err != nil {
		t.Fatalf("Failed to create go.mod: %v", err)
	}

	res := ValidateScaffold(context.Background(), scaffoldPath, implPath, "")
	if res.IsFatal() {
		t.Fatalf("ValidateScaffold returned fatal result: %v", res.Errors)
	}

	result := res.GetData()

	if result.Syntax.Status != "PASS" {
		t.Errorf("Expected syntax check to PASS, got %s", result.Syntax.Status)
	}

	if result.Imports.Status != "FAIL" {
		t.Errorf("Expected import check to FAIL, got %s", result.Imports.Status)
	}

	if len(result.Imports.Errors) == 0 {
		t.Error("Expected import errors to be reported")
	}

	if !result.Imports.AutoFixable {
		t.Error("Expected import error to be auto-fixable")
	}

	// Should contain the missing import
	foundMissing := false
	for _, err := range result.Imports.Errors {
		if err == "github.com/example/missing" {
			foundMissing = true
			break
		}
	}
	if !foundMissing {
		t.Errorf("Expected missing import 'github.com/example/missing' in errors, got: %v", result.Imports.Errors)
	}
}

func TestValidateScaffold_BuildFail(t *testing.T) {
	// Create a temporary directory for test files
	tmpDir := t.TempDir()

	// Create a scaffold with compilation error (undefined variable)
	scaffoldPath := filepath.Join(tmpDir, "scaffold.go")
	scaffoldContent := `package main

import "fmt"

func Hello() {
	fmt.Println(undefinedVariable)
}
`
	if err := os.WriteFile(scaffoldPath, []byte(scaffoldContent), 0644); err != nil {
		t.Fatalf("Failed to create scaffold: %v", err)
	}

	// Create a minimal IMPL manifest
	implPath := filepath.Join(tmpDir, "docs", "IMPL", "test.yaml")
	if err := os.MkdirAll(filepath.Dir(implPath), 0755); err != nil {
		t.Fatalf("Failed to create docs/IMPL directory: %v", err)
	}
	implContent := `impl: test-feature
description: Test feature
waves: []
`
	if err := os.WriteFile(implPath, []byte(implContent), 0644); err != nil {
		t.Fatalf("Failed to create IMPL manifest: %v", err)
	}

	// Create go.mod for language defaults
	goModPath := filepath.Join(tmpDir, "go.mod")
	goModContent := `module example.com/test

go 1.21
`
	if err := os.WriteFile(goModPath, []byte(goModContent), 0644); err != nil {
		t.Fatalf("Failed to create go.mod: %v", err)
	}

	res := ValidateScaffold(context.Background(), scaffoldPath, implPath, "")
	if res.IsFatal() {
		t.Fatalf("ValidateScaffold returned fatal result: %v", res.Errors)
	}

	result := res.GetData()

	if result.Syntax.Status != "PASS" {
		t.Errorf("Expected syntax check to PASS, got %s", result.Syntax.Status)
	}

	// Build should fail (or skip if no build command)
	if result.Build.Status == "FAIL" {
		if len(result.Build.Errors) == 0 {
			t.Error("Expected build errors to be reported")
		}
		if result.Build.AutoFixable {
			t.Error("Expected build error to not be auto-fixable")
		}
	}
}

func TestCheckImports_StandardLib(t *testing.T) {
	imports := []string{"fmt", "os", "strings", "io"}
	result, err := checkImports(imports, "")
	// With empty repoRoot, parseGoMod will fail to read go.mod
	if err == nil {
		// If no error, all std lib imports should pass
		if len(result) != 0 {
			t.Errorf("Expected no missing imports for standard library, got: %v", result)
		}
	}
	// Error is acceptable — go.mod at "/" won't exist
}

func TestCheckImports_ThirdParty(t *testing.T) {
	tmpDir := t.TempDir()
	goModContent := "module example.com/test\n\ngo 1.21\n"
	goModPath := filepath.Join(tmpDir, "go.mod")
	if err := os.WriteFile(goModPath, []byte(goModContent), 0644); err != nil {
		t.Fatalf("Failed to write go.mod: %v", err)
	}
	imports := []string{
		"fmt",
		"github.com/example/pkg",
		"golang.org/x/sync/errgroup",
	}
	result, err := checkImports(imports, tmpDir)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if len(result) != 2 {
		t.Errorf("Expected 2 missing imports, got %d: %v", len(result), result)
	}

	expectedMissing := map[string]bool{
		"github.com/example/pkg":     true,
		"golang.org/x/sync/errgroup": true,
	}

	for _, imp := range result {
		if !expectedMissing[imp] {
			t.Errorf("Unexpected missing import: %s", imp)
		}
	}
}

func TestCheckImports_ThirdPartyInGoMod(t *testing.T) {
	tmpDir := t.TempDir()
	goModContent := "module example.com/test\n\ngo 1.21\n\nrequire (\n\tgithub.com/example/pkg v1.0.0\n\tgolang.org/x/sync v0.1.0\n)\n"
	goModPath := filepath.Join(tmpDir, "go.mod")
	if err := os.WriteFile(goModPath, []byte(goModContent), 0644); err != nil {
		t.Fatalf("Failed to write go.mod: %v", err)
	}
	imports := []string{
		"fmt",
		"github.com/example/pkg",
		"golang.org/x/sync/errgroup",
	}
	result, err := checkImports(imports, tmpDir)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if len(result) != 0 {
		t.Errorf("Expected no missing imports when all third-party are in go.mod, got: %v", result)
	}
}

func TestCheckImports_Mixed(t *testing.T) {
	tmpDir := t.TempDir()
	goModContent := "module example.com/test\n\ngo 1.21\n"
	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(goModContent), 0644); err != nil {
		t.Fatalf("Failed to write go.mod: %v", err)
	}
	imports := []string{
		"fmt",
		"github.com/example/pkg",
		"os",
		"strings",
		"golang.org/x/sync/errgroup",
	}
	result, err := checkImports(imports, tmpDir)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if len(result) != 2 {
		t.Errorf("Expected 2 missing imports, got %d: %v", len(result), result)
	}
}

func TestFindRepoRoot_NoGoMod(t *testing.T) {
	// Create a temp dir with no go.mod anywhere in ancestry
	tmpDir := t.TempDir()
	subDir := filepath.Join(tmpDir, "a", "b", "c")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatalf("Failed to create subdirectory: %v", err)
	}

	result, err := findRepoRoot(subDir)
	if err == nil {
		t.Errorf("Expected error when no go.mod found, got root: %s", result)
	}
	if result != "" {
		t.Errorf("Expected empty string when no go.mod found, got: %s", result)
	}
	if err != nil && !strings.Contains(err.Error(), "could not locate go.mod") {
		t.Errorf("Expected error to mention 'could not locate go.mod', got: %v", err)
	}
}

func TestFindRepoRoot_GoModFound(t *testing.T) {
	tmpDir := t.TempDir()
	// Create go.mod at tmpDir
	goModPath := filepath.Join(tmpDir, "go.mod")
	if err := os.WriteFile(goModPath, []byte("module test\n\ngo 1.21\n"), 0644); err != nil {
		t.Fatalf("Failed to create go.mod: %v", err)
	}
	// Search from a subdirectory
	subDir := filepath.Join(tmpDir, "pkg", "foo")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatalf("Failed to create subdirectory: %v", err)
	}

	result, err := findRepoRoot(subDir)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if result != tmpDir {
		t.Errorf("Expected root %s, got %s", tmpDir, result)
	}
}

func TestParseGoMod_FileNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	goModPath := filepath.Join(tmpDir, "go.mod") // does not exist

	moduleName, requires, err := parseGoMod(goModPath)
	if err == nil {
		t.Error("Expected error for missing go.mod file")
	}
	if moduleName != "" {
		t.Errorf("Expected empty module name, got: %s", moduleName)
	}
	if requires != nil {
		t.Errorf("Expected nil requires, got: %v", requires)
	}
}

func TestParseGoMod_MalformedFile(t *testing.T) {
	tmpDir := t.TempDir()
	goModPath := filepath.Join(tmpDir, "go.mod")
	// Malformed: no module directive, just garbage
	if err := os.WriteFile(goModPath, []byte("this is not a valid go.mod\n"), 0644); err != nil {
		t.Fatalf("Failed to create go.mod: %v", err)
	}

	moduleName, requires, err := parseGoMod(goModPath)
	if err != nil {
		t.Fatalf("Unexpected error for readable but malformed go.mod: %v", err)
	}
	// File is readable but has no module directive — empty module name is expected
	if moduleName != "" {
		t.Errorf("Expected empty module name for malformed go.mod, got: %s", moduleName)
	}
	if len(requires) != 0 {
		t.Errorf("Expected no requires for malformed go.mod, got: %v", requires)
	}
}
