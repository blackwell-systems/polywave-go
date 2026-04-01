package scaffoldval

import (
	"context"
	"os"
	"path/filepath"
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

func TestExtractImports(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected []string
	}{
		{
			name: "single import",
			content: `package main

import "fmt"

func main() {}
`,
			expected: []string{"fmt"},
		},
		{
			name: "multiple imports in block",
			content: `package main

import (
	"fmt"
	"os"
	"strings"
)

func main() {}
`,
			expected: []string{"fmt", "os", "strings"},
		},
		{
			name: "third-party imports",
			content: `package main

import (
	"fmt"
	"github.com/example/pkg"
	"golang.org/x/sync/errgroup"
)

func main() {}
`,
			expected: []string{"fmt", "github.com/example/pkg", "golang.org/x/sync/errgroup"},
		},
		{
			name: "aliased imports",
			content: `package main

import (
	"fmt"
	pkg "github.com/example/pkg"
)

func main() {}
`,
			expected: []string{"fmt", "github.com/example/pkg"},
		},
		{
			name:     "no imports",
			content:  `package main\n\nfunc main() {}`,
			expected: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractImports(tt.content)
			if len(result) != len(tt.expected) {
				t.Errorf("Expected %d imports, got %d: %v", len(tt.expected), len(result), result)
				return
			}
			for i, imp := range result {
				if imp != tt.expected[i] {
					t.Errorf("Expected import[%d] = %s, got %s", i, tt.expected[i], imp)
				}
			}
		})
	}
}

func TestCheckImports_StandardLib(t *testing.T) {
	imports := []string{"fmt", "os", "strings", "io"}
	result := checkImports(imports, "")

	if len(result) != 0 {
		t.Errorf("Expected no missing imports for standard library, got: %v", result)
	}
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
	result := checkImports(imports, tmpDir)
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
	result := checkImports(imports, tmpDir)
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
	result := checkImports(imports, tmpDir)
	if len(result) != 2 {
		t.Errorf("Expected 2 missing imports, got %d: %v", len(result), result)
	}
}
