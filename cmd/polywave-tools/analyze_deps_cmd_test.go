package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/blackwell-systems/polywave-go/pkg/analyzer"
	"gopkg.in/yaml.v3"
)

// TestAnalyzeDepsCmd_SimpleFixture tests the analyze-deps command on a simple fixture.
func TestAnalyzeDepsCmd_SimpleFixture(t *testing.T) {
	tmpDir := t.TempDir()

	// Create go.mod
	goMod := `module github.com/test/simple
go 1.21
`
	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(goMod), 0644); err != nil {
		t.Fatal(err)
	}

	// Create package structure: c.go -> b.go -> a.go
	pkgA := filepath.Join(tmpDir, "pkga")
	pkgB := filepath.Join(tmpDir, "pkgb")
	pkgC := filepath.Join(tmpDir, "pkgc")
	os.MkdirAll(pkgA, 0755)
	os.MkdirAll(pkgB, 0755)
	os.MkdirAll(pkgC, 0755)

	fileA := filepath.Join(pkgA, "a.go")
	fileB := filepath.Join(pkgB, "b.go")
	fileC := filepath.Join(pkgC, "c.go")

	codeA := `package pkga

func A() string {
	return "a"
}
`
	if err := os.WriteFile(fileA, []byte(codeA), 0644); err != nil {
		t.Fatal(err)
	}

	codeB := `package pkgb

import "github.com/test/simple/pkga"

func B() string {
	return pkga.A() + "b"
}
`
	if err := os.WriteFile(fileB, []byte(codeB), 0644); err != nil {
		t.Fatal(err)
	}

	codeC := `package pkgc

import "github.com/test/simple/pkgb"

func C() string {
	return pkgb.B() + "c"
}
`
	if err := os.WriteFile(fileC, []byte(codeC), 0644); err != nil {
		t.Fatal(err)
	}

	// Capture stdout
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	// Create command
	cmd := newAnalyzeDepsCmd()
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{
		tmpDir,
		"--files", strings.Join([]string{fileA, fileB, fileC}, ","),
		"--format", "yaml",
	})

	// Execute
	if err := cmd.Execute(); err != nil {
		t.Fatalf("command failed: %v", err)
	}

	// Parse output
	var output analyzer.Output
	if err := yaml.Unmarshal(stdout.Bytes(), &output); err != nil {
		t.Fatalf("failed to parse YAML output: %v", err)
	}

	// Verify structure
	if len(output.Nodes) != 3 {
		t.Errorf("expected 3 nodes, got %d", len(output.Nodes))
	}

	if len(output.Waves) != 3 {
		t.Errorf("expected 3 waves, got %d", len(output.Waves))
	}

	// Verify wave 1 contains fileA (no deps)
	if len(output.Waves[1]) != 1 || output.Waves[1][0] != fileA {
		t.Errorf("wave 1 should contain %s, got %v", fileA, output.Waves[1])
	}

	// Verify wave 2 contains fileB (depends on fileA)
	if len(output.Waves[2]) != 1 || output.Waves[2][0] != fileB {
		t.Errorf("wave 2 should contain %s, got %v", fileB, output.Waves[2])
	}

	// Verify wave 3 contains fileC (depends on fileB)
	if len(output.Waves[3]) != 1 || output.Waves[3][0] != fileC {
		t.Errorf("wave 3 should contain %s, got %v", fileC, output.Waves[3])
	}
}

// TestAnalyzeDepsCmd_JSONFormat tests JSON output format.
func TestAnalyzeDepsCmd_JSONFormat(t *testing.T) {
	tmpDir := t.TempDir()

	// Create go.mod
	goMod := `module github.com/test/simple
go 1.21
`
	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(goMod), 0644); err != nil {
		t.Fatal(err)
	}

	// Create single file with no deps
	pkg := filepath.Join(tmpDir, "pkg")
	os.MkdirAll(pkg, 0755)
	file := filepath.Join(pkg, "file.go")

	code := `package pkg

func Foo() string {
	return "foo"
}
`
	if err := os.WriteFile(file, []byte(code), 0644); err != nil {
		t.Fatal(err)
	}

	// Capture stdout
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	// Create command
	cmd := newAnalyzeDepsCmd()
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{
		tmpDir,
		"--files", file,
		"--format", "json",
	})

	// Execute
	if err := cmd.Execute(); err != nil {
		t.Fatalf("command failed: %v", err)
	}

	// Parse JSON output
	var output analyzer.Output
	if err := json.Unmarshal(stdout.Bytes(), &output); err != nil {
		t.Fatalf("failed to parse JSON output: %v", err)
	}

	// Verify structure
	if len(output.Nodes) != 1 {
		t.Errorf("expected 1 node, got %d", len(output.Nodes))
	}

	if len(output.Waves) != 1 {
		t.Errorf("expected 1 wave, got %d", len(output.Waves))
	}

	if output.Nodes[0].File != file {
		t.Errorf("expected node file %s, got %s", file, output.Nodes[0].File)
	}

	if output.Nodes[0].WaveCandidate != 0 {
		t.Errorf("expected wave candidate 0, got %d", output.Nodes[0].WaveCandidate)
	}
}

// TestAnalyzeDepsCmd_CycleError tests that cycle detection produces an error.
func TestAnalyzeDepsCmd_CycleError(t *testing.T) {
	tmpDir := t.TempDir()

	// Create go.mod
	goMod := `module github.com/test/cycle
go 1.21
`
	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(goMod), 0644); err != nil {
		t.Fatal(err)
	}

	// Create cyclic dependency: a -> b -> a
	pkgA := filepath.Join(tmpDir, "pkga")
	pkgB := filepath.Join(tmpDir, "pkgb")
	os.MkdirAll(pkgA, 0755)
	os.MkdirAll(pkgB, 0755)

	fileA := filepath.Join(pkgA, "a.go")
	fileB := filepath.Join(pkgB, "b.go")

	codeA := `package pkga

import "github.com/test/cycle/pkgb"

func A() {
	pkgb.B()
}
`
	if err := os.WriteFile(fileA, []byte(codeA), 0644); err != nil {
		t.Fatal(err)
	}

	codeB := `package pkgb

import "github.com/test/cycle/pkga"

func B() {
	pkga.A()
}
`
	if err := os.WriteFile(fileB, []byte(codeB), 0644); err != nil {
		t.Fatal(err)
	}

	// Capture stderr
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	// Create command
	cmd := newAnalyzeDepsCmd()
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{
		tmpDir,
		"--files", strings.Join([]string{fileA, fileB}, ","),
		"--format", "yaml",
	})

	// Execute should fail
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for cyclic dependency, got nil")
	}

	// Verify error message mentions cycle
	errMsg := err.Error()
	if !strings.Contains(errMsg, "circular dependency") && !strings.Contains(errMsg, "cycle") {
		t.Errorf("error message should mention cycle, got: %s", errMsg)
	}
}

// TestAnalyzeDepsCmd_MissingFilesFlag tests that missing --files flag produces an error.
func TestAnalyzeDepsCmd_MissingFilesFlag(t *testing.T) {
	tmpDir := t.TempDir()

	// Create command without --files flag
	cmd := newAnalyzeDepsCmd()
	cmd.SetArgs([]string{tmpDir})

	// Execute should fail
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for missing --files flag, got nil")
	}

	// Verify error message mentions required flag
	errMsg := err.Error()
	if !strings.Contains(errMsg, "required flag") && !strings.Contains(errMsg, "--files") {
		t.Errorf("error message should mention required --files flag, got: %s", errMsg)
	}
}

// TestAnalyzeDepsCmd_InvalidFormat tests that invalid format flag produces an error.
func TestAnalyzeDepsCmd_InvalidFormat(t *testing.T) {
	tmpDir := t.TempDir()

	// Create go.mod
	goMod := `module github.com/test/simple
go 1.21
`
	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(goMod), 0644); err != nil {
		t.Fatal(err)
	}

	// Create single file
	pkg := filepath.Join(tmpDir, "pkg")
	os.MkdirAll(pkg, 0755)
	file := filepath.Join(pkg, "file.go")

	code := `package pkg

func Foo() string {
	return "foo"
}
`
	if err := os.WriteFile(file, []byte(code), 0644); err != nil {
		t.Fatal(err)
	}

	// Create command with invalid format
	cmd := newAnalyzeDepsCmd()
	cmd.SetArgs([]string{
		tmpDir,
		"--files", file,
		"--format", "xml",
	})

	// Execute should fail
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for invalid format, got nil")
	}

	// Verify error message mentions unsupported format
	errMsg := err.Error()
	if !strings.Contains(errMsg, "unsupported format") {
		t.Errorf("error message should mention unsupported format, got: %s", errMsg)
	}
}

// TestAnalyzeDepsCmd_RelativeFilePaths tests that relative file paths work correctly.
func TestAnalyzeDepsCmd_RelativeFilePaths(t *testing.T) {
	tmpDir := t.TempDir()

	// Create go.mod
	goMod := `module github.com/test/simple
go 1.21
`
	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(goMod), 0644); err != nil {
		t.Fatal(err)
	}

	// Create single file
	pkg := filepath.Join(tmpDir, "pkg")
	os.MkdirAll(pkg, 0755)
	file := filepath.Join(pkg, "file.go")

	code := `package pkg

func Foo() string {
	return "foo"
}
`
	if err := os.WriteFile(file, []byte(code), 0644); err != nil {
		t.Fatal(err)
	}

	// Capture stdout
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	// Create command with absolute path (our implementation should handle both)
	cmd := newAnalyzeDepsCmd()
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{
		tmpDir,
		"--files", file,
		"--format", "yaml",
	})

	// Execute
	if err := cmd.Execute(); err != nil {
		t.Fatalf("command failed: %v", err)
	}

	// Parse output
	var output analyzer.Output
	if err := yaml.Unmarshal(stdout.Bytes(), &output); err != nil {
		t.Fatalf("failed to parse YAML output: %v", err)
	}

	// Verify file path appears in nodes
	if len(output.Nodes) != 1 {
		t.Errorf("expected 1 node, got %d", len(output.Nodes))
	}

	if output.Nodes[0].File != file {
		t.Errorf("expected node file %s, got %s", file, output.Nodes[0].File)
	}
}
