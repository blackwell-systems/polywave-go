package analyzer

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// TestDetectCascades_SyntaxImport tests detection of old type names in import statements.
func TestDetectCascades_SyntaxImport(t *testing.T) {
	tmpDir := t.TempDir()

	// Create go.mod
	goMod := `module github.com/test/cascade
go 1.21
`
	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(goMod), 0644); err != nil {
		t.Fatal(err)
	}

	// Create a file that imports a package with renamed type
	pkgDir := filepath.Join(tmpDir, "pkg", "user")
	os.MkdirAll(pkgDir, 0755)
	userFile := filepath.Join(pkgDir, "user.go")
	code := `package user

import "github.com/test/cascade/pkg/auth"

func GetUser() {
	// Uses auth package
}
`
	if err := os.WriteFile(userFile, []byte(code), 0644); err != nil {
		t.Fatal(err)
	}

	// Define a rename for AuthToken in pkg/auth
	renames := []RenameInfo{
		{
			Old:   "AuthToken",
			New:   "SessionToken",
			Scope: "pkg/auth",
		},
	}

	result, err := DetectCascades(context.Background(), tmpDir, renames)
	if err != nil {
		t.Fatalf("DetectCascades failed: %v", err)
	}

	// Should detect import statement
	if len(result.CascadeCandidates) == 0 {
		t.Fatal("expected cascade candidates, got none")
	}

	found := false
	for _, c := range result.CascadeCandidates {
		if c.File == userFile && c.CascadeType == "syntax" && c.Severity == "high" {
			found = true
			if c.Line != 3 {
				t.Errorf("expected line 3, got %d", c.Line)
			}
		}
	}

	if !found {
		t.Error("expected to find syntax cascade in import statement")
	}
}

// TestDetectCascades_SyntaxTypeDecl tests detection of type declarations using old type names.
func TestDetectCascades_SyntaxTypeDecl(t *testing.T) {
	tmpDir := t.TempDir()

	// Create go.mod
	goMod := `module github.com/test/cascade
go 1.21
`
	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(goMod), 0644); err != nil {
		t.Fatal(err)
	}

	// Create a file with variable declaration using old type name
	pkgDir := filepath.Join(tmpDir, "pkg", "handler")
	os.MkdirAll(pkgDir, 0755)
	handlerFile := filepath.Join(pkgDir, "handler.go")
	code := `package handler

type AuthToken struct {
	Token string
}

var token AuthToken
`
	if err := os.WriteFile(handlerFile, []byte(code), 0644); err != nil {
		t.Fatal(err)
	}

	// Define rename
	renames := []RenameInfo{
		{
			Old:   "AuthToken",
			New:   "SessionToken",
			Scope: "pkg/auth",
		},
	}

	result, err := DetectCascades(context.Background(), tmpDir, renames)
	if err != nil {
		t.Fatalf("DetectCascades failed: %v", err)
	}

	// Should detect both type declaration and variable declaration
	if len(result.CascadeCandidates) < 2 {
		t.Fatalf("expected at least 2 cascade candidates, got %d", len(result.CascadeCandidates))
	}

	foundTypeDecl := false
	foundVarDecl := false

	for _, c := range result.CascadeCandidates {
		if c.File == handlerFile {
			if c.Line == 3 && c.CascadeType == "syntax" && c.Severity == "high" {
				foundTypeDecl = true
			}
			if c.Line == 7 && c.CascadeType == "syntax" && c.Severity == "medium" {
				foundVarDecl = true
			}
		}
	}

	if !foundTypeDecl {
		t.Error("expected to find type declaration cascade")
	}
	if !foundVarDecl {
		t.Error("expected to find variable declaration cascade")
	}
}

// TestDetectCascades_SemanticComment tests detection of old type names in comments.
func TestDetectCascades_SemanticComment(t *testing.T) {
	tmpDir := t.TempDir()

	// Create go.mod
	goMod := `module github.com/test/cascade
go 1.21
`
	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(goMod), 0644); err != nil {
		t.Fatal(err)
	}

	// Create a file with comment mentioning old type name
	pkgDir := filepath.Join(tmpDir, "pkg", "service")
	os.MkdirAll(pkgDir, 0755)
	serviceFile := filepath.Join(pkgDir, "service.go")
	code := `package service

// GetSession retrieves the AuthToken for the user.
// The AuthToken is used for authentication.
func GetSession() {
	// Implementation
}
`
	if err := os.WriteFile(serviceFile, []byte(code), 0644); err != nil {
		t.Fatal(err)
	}

	// Define rename
	renames := []RenameInfo{
		{
			Old:   "AuthToken",
			New:   "SessionToken",
			Scope: "pkg/auth",
		},
	}

	result, err := DetectCascades(context.Background(), tmpDir, renames)
	if err != nil {
		t.Fatalf("DetectCascades failed: %v", err)
	}

	// Should detect comments mentioning AuthToken
	if len(result.CascadeCandidates) == 0 {
		t.Fatal("expected cascade candidates, got none")
	}

	semanticCount := 0
	for _, c := range result.CascadeCandidates {
		if c.File == serviceFile && c.CascadeType == "semantic" && c.Severity == "low" {
			semanticCount++
		}
	}

	if semanticCount < 2 {
		t.Errorf("expected at least 2 semantic cascade candidates from comments, got %d", semanticCount)
	}
}

// TestDetectCascades_SemanticString tests detection of old type names in string literals.
func TestDetectCascades_SemanticString(t *testing.T) {
	tmpDir := t.TempDir()

	// Create go.mod
	goMod := `module github.com/test/cascade
go 1.21
`
	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(goMod), 0644); err != nil {
		t.Fatal(err)
	}

	// Create a file with string literal mentioning old type name
	pkgDir := filepath.Join(tmpDir, "pkg", "logger")
	os.MkdirAll(pkgDir, 0755)
	loggerFile := filepath.Join(pkgDir, "logger.go")
	code := `package logger

import "fmt"

func LogAuth() {
	fmt.Println("Using AuthToken for authentication")
	message := "AuthToken is required"
	_ = message
}
`
	if err := os.WriteFile(loggerFile, []byte(code), 0644); err != nil {
		t.Fatal(err)
	}

	// Define rename
	renames := []RenameInfo{
		{
			Old:   "AuthToken",
			New:   "SessionToken",
			Scope: "pkg/auth",
		},
	}

	result, err := DetectCascades(context.Background(), tmpDir, renames)
	if err != nil {
		t.Fatalf("DetectCascades failed: %v", err)
	}

	// Should detect string literals mentioning AuthToken
	if len(result.CascadeCandidates) == 0 {
		t.Fatal("expected cascade candidates, got none")
	}

	semanticCount := 0
	for _, c := range result.CascadeCandidates {
		if c.File == loggerFile && c.CascadeType == "semantic" && c.Severity == "low" {
			semanticCount++
		}
	}

	if semanticCount < 2 {
		t.Errorf("expected at least 2 semantic cascade candidates from strings, got %d", semanticCount)
	}
}

// TestDetectCascades_MultipleRenames tests handling of multiple type renames in single scan.
func TestDetectCascades_MultipleRenames(t *testing.T) {
	tmpDir := t.TempDir()

	// Create go.mod
	goMod := `module github.com/test/cascade
go 1.21
`
	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(goMod), 0644); err != nil {
		t.Fatal(err)
	}

	// Create a file using multiple old type names
	pkgDir := filepath.Join(tmpDir, "pkg", "multi")
	os.MkdirAll(pkgDir, 0755)
	multiFile := filepath.Join(pkgDir, "multi.go")
	code := `package multi

type AuthToken struct {
	Token string
}

type UserRole struct {
	Role string
}

var token AuthToken
var role UserRole
`
	if err := os.WriteFile(multiFile, []byte(code), 0644); err != nil {
		t.Fatal(err)
	}

	// Define multiple renames
	renames := []RenameInfo{
		{
			Old:   "AuthToken",
			New:   "SessionToken",
			Scope: "pkg/auth",
		},
		{
			Old:   "UserRole",
			New:   "AccountRole",
			Scope: "pkg/user",
		},
	}

	result, err := DetectCascades(context.Background(), tmpDir, renames)
	if err != nil {
		t.Fatalf("DetectCascades failed: %v", err)
	}

	// Should detect cascades for both renames
	if len(result.CascadeCandidates) < 4 {
		t.Fatalf("expected at least 4 cascade candidates (2 types + 2 vars), got %d", len(result.CascadeCandidates))
	}

	authCount := 0
	roleCount := 0

	for _, c := range result.CascadeCandidates {
		if c.File == multiFile {
			if c.Match == "AuthToken" || (c.CascadeType == "syntax" && c.Line == 3) {
				authCount++
			}
			if c.Match == "UserRole" || (c.CascadeType == "syntax" && c.Line == 7) {
				roleCount++
			}
		}
	}

	if authCount < 2 {
		t.Errorf("expected at least 2 cascades for AuthToken, got %d", authCount)
	}
	if roleCount < 2 {
		t.Errorf("expected at least 2 cascades for UserRole, got %d", roleCount)
	}
}

// TestDetectCascades_NoMatches tests behavior when no files reference old names.
func TestDetectCascades_NoMatches(t *testing.T) {
	tmpDir := t.TempDir()

	// Create go.mod
	goMod := `module github.com/test/cascade
go 1.21
`
	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(goMod), 0644); err != nil {
		t.Fatal(err)
	}

	// Create a file that doesn't reference the renamed type
	pkgDir := filepath.Join(tmpDir, "pkg", "other")
	os.MkdirAll(pkgDir, 0755)
	otherFile := filepath.Join(pkgDir, "other.go")
	code := `package other

type Something struct {
	Value string
}

func DoSomething() {
	// Implementation here
}
`
	if err := os.WriteFile(otherFile, []byte(code), 0644); err != nil {
		t.Fatal(err)
	}

	// Define rename
	renames := []RenameInfo{
		{
			Old:   "AuthToken",
			New:   "SessionToken",
			Scope: "pkg/auth",
		},
	}

	result, err := DetectCascades(context.Background(), tmpDir, renames)
	if err != nil {
		t.Fatalf("DetectCascades failed: %v", err)
	}

	// Should return empty list
	if len(result.CascadeCandidates) != 0 {
		t.Errorf("expected 0 cascade candidates, got %d", len(result.CascadeCandidates))
		for i, c := range result.CascadeCandidates {
			t.Logf("  [%d] File=%s Line=%d Match=%q Type=%s Severity=%s Reason=%s",
				i, c.File, c.Line, c.Match, c.CascadeType, c.Severity, c.Reason)
		}
	}
}

// TestDetectCascades_SkipsTestFiles tests that test files are excluded from scan.
func TestDetectCascades_SkipsTestFiles(t *testing.T) {
	tmpDir := t.TempDir()

	// Create go.mod
	goMod := `module github.com/test/cascade
go 1.21
`
	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(goMod), 0644); err != nil {
		t.Fatal(err)
	}

	// Create a test file with old type name
	pkgDir := filepath.Join(tmpDir, "pkg", "auth")
	os.MkdirAll(pkgDir, 0755)
	testFile := filepath.Join(pkgDir, "auth_test.go")
	code := `package auth

type AuthToken struct {
	Token string
}

func TestAuth() {
	var token AuthToken
}
`
	if err := os.WriteFile(testFile, []byte(code), 0644); err != nil {
		t.Fatal(err)
	}

	// Define rename
	renames := []RenameInfo{
		{
			Old:   "AuthToken",
			New:   "SessionToken",
			Scope: "pkg/auth",
		},
	}

	result, err := DetectCascades(context.Background(), tmpDir, renames)
	if err != nil {
		t.Fatalf("DetectCascades failed: %v", err)
	}

	// Should skip test file
	for _, c := range result.CascadeCandidates {
		if c.File == testFile {
			t.Errorf("test file should be skipped, but found cascade: %+v", c)
		}
	}
}

// TestDetectCascades_Determinism tests that output is sorted deterministically.
func TestDetectCascades_Determinism(t *testing.T) {
	tmpDir := t.TempDir()

	// Create go.mod
	goMod := `module github.com/test/cascade
go 1.21
`
	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(goMod), 0644); err != nil {
		t.Fatal(err)
	}

	// Create multiple files with cascades
	files := []string{"a.go", "b.go", "c.go"}
	for _, fname := range files {
		pkgDir := filepath.Join(tmpDir, "pkg")
		os.MkdirAll(pkgDir, 0755)
		fpath := filepath.Join(pkgDir, fname)
		code := `package pkg

type AuthToken struct {
	Token string
}
`
		if err := os.WriteFile(fpath, []byte(code), 0644); err != nil {
			t.Fatal(err)
		}
	}

	// Define rename
	renames := []RenameInfo{
		{
			Old:   "AuthToken",
			New:   "SessionToken",
			Scope: "pkg/auth",
		},
	}

	// Run twice and verify identical order
	result1, err := DetectCascades(context.Background(), tmpDir, renames)
	if err != nil {
		t.Fatalf("DetectCascades run 1 failed: %v", err)
	}

	result2, err := DetectCascades(context.Background(), tmpDir, renames)
	if err != nil {
		t.Fatalf("DetectCascades run 2 failed: %v", err)
	}

	if len(result1.CascadeCandidates) != len(result2.CascadeCandidates) {
		t.Errorf("different number of candidates: %d vs %d", len(result1.CascadeCandidates), len(result2.CascadeCandidates))
	}

	// Verify order is identical
	for i := range result1.CascadeCandidates {
		if i >= len(result2.CascadeCandidates) {
			break
		}
		c1 := result1.CascadeCandidates[i]
		c2 := result2.CascadeCandidates[i]

		if c1.File != c2.File || c1.Line != c2.Line {
			t.Errorf("different order at index %d: %s:%d vs %s:%d", i, c1.File, c1.Line, c2.File, c2.Line)
		}
	}

	// Verify sorted by file, then line
	for i := 1; i < len(result1.CascadeCandidates); i++ {
		prev := result1.CascadeCandidates[i-1]
		curr := result1.CascadeCandidates[i]

		if prev.File > curr.File {
			t.Errorf("not sorted by file: %s comes after %s", prev.File, curr.File)
		}
		if prev.File == curr.File && prev.Line > curr.Line {
			t.Errorf("not sorted by line: %d comes after %d in %s", prev.Line, curr.Line, prev.File)
		}
	}
}
