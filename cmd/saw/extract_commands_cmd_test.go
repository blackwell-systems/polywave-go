package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/commands"
	"gopkg.in/yaml.v3"
)

// TestExtractCommandsCmd_ValidRepo tests command extraction from a repo with a Makefile.
func TestExtractCommandsCmd_ValidRepo(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a Makefile with standard targets
	makefile := `
.PHONY: build test lint fmt

build:
	go build ./...

test:
	go test ./...

lint:
	golangci-lint run

fmt:
	gofmt -w .
`
	if err := os.WriteFile(filepath.Join(tmpDir, "Makefile"), []byte(makefile), 0644); err != nil {
		t.Fatal(err)
	}

	// Create go.mod to mark it as a Go repo
	goMod := `module github.com/test/repo
go 1.21
`
	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(goMod), 0644); err != nil {
		t.Fatal(err)
	}

	// Capture stdout
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	// Create command
	cmd := newExtractCommandsCmd()
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{tmpDir, "--format", "yaml"})

	// Execute
	if err := cmd.Execute(); err != nil {
		t.Fatalf("command failed: %v\nstderr: %s", err, stderr.String())
	}

	// Parse output
	var result commands.CommandSet
	if err := yaml.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse YAML output: %v\noutput: %s", err, stdout.String())
	}

	// Verify toolchain (Makefile parser sets toolchain to "make")
	if result.Toolchain != "make" {
		t.Errorf("expected toolchain 'make', got %q", result.Toolchain)
	}

	// Verify commands were extracted
	if result.Commands.Build == "" {
		t.Error("expected build command to be extracted")
	}
	if result.Commands.Test.Full == "" {
		t.Error("expected test command to be extracted")
	}

	// Verify detection sources
	if len(result.DetectionSources) == 0 {
		t.Error("expected at least one detection source")
	}
	foundMakefile := false
	for _, src := range result.DetectionSources {
		if strings.Contains(src, "Makefile") {
			foundMakefile = true
			break
		}
	}
	if !foundMakefile {
		t.Errorf("expected Makefile in detection sources, got: %v", result.DetectionSources)
	}
}

// TestExtractCommandsCmd_NoConfigs tests extraction from a repo with only go.mod (defaults).
func TestExtractCommandsCmd_NoConfigs(t *testing.T) {
	tmpDir := t.TempDir()

	// Create only go.mod (no CI config, no Makefile, no package.json)
	goMod := `module github.com/test/minimal
go 1.21
`
	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(goMod), 0644); err != nil {
		t.Fatal(err)
	}

	// Capture stdout
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	// Create command
	cmd := newExtractCommandsCmd()
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{tmpDir, "--format", "yaml"})

	// Execute
	if err := cmd.Execute(); err != nil {
		t.Fatalf("command failed: %v\nstderr: %s", err, stderr.String())
	}

	// Parse output
	var result commands.CommandSet
	if err := yaml.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse YAML output: %v\noutput: %s", err, stdout.String())
	}

	// Verify toolchain defaults to Go
	if result.Toolchain != "go" {
		t.Errorf("expected toolchain 'go', got %q", result.Toolchain)
	}

	// Verify default commands were provided
	if result.Commands.Build == "" {
		t.Error("expected default build command")
	}
	if result.Commands.Test.Full == "" {
		t.Error("expected default test command")
	}

	// Should have "defaults" or similar in detection sources
	if len(result.DetectionSources) == 0 {
		t.Error("expected at least one detection source (defaults)")
	}
}

// TestExtractCommandsCmd_JSONOutput tests JSON output format.
func TestExtractCommandsCmd_JSONOutput(t *testing.T) {
	tmpDir := t.TempDir()

	// Create go.mod
	goMod := `module github.com/test/json
go 1.21
`
	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(goMod), 0644); err != nil {
		t.Fatal(err)
	}

	// Create Makefile
	makefile := `
.PHONY: build

build:
	go build ./...
`
	if err := os.WriteFile(filepath.Join(tmpDir, "Makefile"), []byte(makefile), 0644); err != nil {
		t.Fatal(err)
	}

	// Capture stdout
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	// Create command with JSON format
	cmd := newExtractCommandsCmd()
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{tmpDir, "--format", "json"})

	// Execute
	if err := cmd.Execute(); err != nil {
		t.Fatalf("command failed: %v\nstderr: %s", err, stderr.String())
	}

	// Parse JSON output
	var result commands.CommandSet
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse JSON output: %v\noutput: %s", err, stdout.String())
	}

	// Verify structure
	if result.Toolchain == "" {
		t.Error("expected toolchain to be set")
	}
	if result.Commands.Build == "" {
		t.Error("expected build command to be extracted")
	}
}

// TestExtractCommandsCmd_InvalidRepoRoot tests error handling for non-existent directory.
func TestExtractCommandsCmd_InvalidRepoRoot(t *testing.T) {
	// Use a path that definitely doesn't exist
	nonExistentPath := filepath.Join(t.TempDir(), "does-not-exist", "nested", "path")

	// Capture stdout/stderr
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	// Create command
	cmd := newExtractCommandsCmd()
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{nonExistentPath})

	// Execute - should fail gracefully
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for non-existent repo root, got nil")
	}

	// Verify error is meaningful (should mention the path or indicate failure)
	errMsg := err.Error()
	if !strings.Contains(errMsg, "extract commands") && !strings.Contains(errMsg, "no such file") {
		t.Errorf("expected meaningful error message, got: %v", err)
	}
}

// TestExtractCommandsCmd_DefaultFormat tests that YAML is the default format.
func TestExtractCommandsCmd_DefaultFormat(t *testing.T) {
	tmpDir := t.TempDir()

	// Create go.mod
	goMod := `module github.com/test/default-format
go 1.21
`
	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(goMod), 0644); err != nil {
		t.Fatal(err)
	}

	// Capture stdout
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	// Create command without specifying format (should default to yaml)
	cmd := newExtractCommandsCmd()
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{tmpDir})

	// Execute
	if err := cmd.Execute(); err != nil {
		t.Fatalf("command failed: %v\nstderr: %s", err, stderr.String())
	}

	// Should be parseable as YAML
	var result commands.CommandSet
	if err := yaml.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("default output should be YAML, but failed to parse: %v\noutput: %s", err, stdout.String())
	}

	// Basic validation
	if result.Toolchain == "" {
		t.Error("expected toolchain to be set")
	}
}

// TestExtractCommandsCmd_UnsupportedFormat tests error handling for invalid format flag.
func TestExtractCommandsCmd_UnsupportedFormat(t *testing.T) {
	tmpDir := t.TempDir()

	// Create go.mod
	goMod := `module github.com/test/bad-format
go 1.21
`
	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(goMod), 0644); err != nil {
		t.Fatal(err)
	}

	// Capture stdout/stderr
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	// Create command with invalid format
	cmd := newExtractCommandsCmd()
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{tmpDir, "--format", "xml"})

	// Execute - should fail
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for unsupported format, got nil")
	}

	// Verify error message mentions unsupported format
	errMsg := err.Error()
	if !strings.Contains(errMsg, "unsupported format") {
		t.Errorf("expected 'unsupported format' in error, got: %v", err)
	}
}
