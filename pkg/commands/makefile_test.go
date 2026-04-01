package commands

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMakefileParser_SimpleTargets(t *testing.T) {
	tmpDir := t.TempDir()
	makefilePath := filepath.Join(tmpDir, "Makefile")

	makefileContent := `# Simple Makefile with independent targets
build:
	go build ./...

test:
	go test ./...

lint:
	golangci-lint run

fmt:
	gofmt -w .
`

	if err := os.WriteFile(makefilePath, []byte(makefileContent), 0644); err != nil {
		t.Fatalf("Failed to write Makefile: %v", err)
	}

	parser := &MakefileParser{}
	cmdSet, err := parser.ParseBuildSystem(tmpDir)
	if err != nil {
		t.Fatalf("ParseBuildSystem failed: %v", err)
	}

	if cmdSet == nil {
		t.Fatal("Expected CommandSet, got nil")
	}

	if cmdSet.Toolchain != "make" {
		t.Errorf("Expected toolchain 'make', got '%s'", cmdSet.Toolchain)
	}

	if cmdSet.Commands.Build != "make build" {
		t.Errorf("Expected build command 'make build', got '%s'", cmdSet.Commands.Build)
	}

	if cmdSet.Commands.Test.Full != "make test" {
		t.Errorf("Expected test command 'make test', got '%s'", cmdSet.Commands.Test.Full)
	}

	if cmdSet.Commands.Lint.Check != "make lint" {
		t.Errorf("Expected lint command 'make lint', got '%s'", cmdSet.Commands.Lint.Check)
	}

	if cmdSet.Commands.Format.Fix != "make fmt" {
		t.Errorf("Expected format command 'make fmt', got '%s'", cmdSet.Commands.Format.Fix)
	}
}

func TestMakefileParser_TargetChaining(t *testing.T) {
	tmpDir := t.TempDir()
	makefilePath := filepath.Join(tmpDir, "Makefile")

	makefileContent := `# Makefile with target chaining
.PHONY: test build lint test-unit

test: build lint test-unit

build:
	go build ./...

lint:
	golangci-lint run

test-unit:
	go test ./... -short
`

	if err := os.WriteFile(makefilePath, []byte(makefileContent), 0644); err != nil {
		t.Fatalf("Failed to write Makefile: %v", err)
	}

	parser := &MakefileParser{}
	cmdSet, err := parser.ParseBuildSystem(tmpDir)
	if err != nil {
		t.Fatalf("ParseBuildSystem failed: %v", err)
	}

	if cmdSet == nil {
		t.Fatal("Expected CommandSet, got nil")
	}

	// Should extract leaf targets, not the chained "test" target
	if cmdSet.Commands.Build != "make build" {
		t.Errorf("Expected build command 'make build', got '%s'", cmdSet.Commands.Build)
	}

	if cmdSet.Commands.Lint.Check != "make lint" {
		t.Errorf("Expected lint command 'make lint', got '%s'", cmdSet.Commands.Lint.Check)
	}

	if cmdSet.Commands.Test.Full != "make test-unit" {
		t.Errorf("Expected test command 'make test-unit', got '%s'", cmdSet.Commands.Test.Full)
	}
}

func TestMakefileParser_NoMakefile(t *testing.T) {
	tmpDir := t.TempDir()

	parser := &MakefileParser{}
	cmdSet, err := parser.ParseBuildSystem(tmpDir)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if cmdSet != nil {
		t.Errorf("Expected nil CommandSet when Makefile doesn't exist, got: %+v", cmdSet)
	}
}

func TestMakefileParser_EmptyTargets(t *testing.T) {
	tmpDir := t.TempDir()
	makefilePath := filepath.Join(tmpDir, "Makefile")

	makefileContent := `# Makefile with only comments and no targets

# This is a comment
`

	if err := os.WriteFile(makefilePath, []byte(makefileContent), 0644); err != nil {
		t.Fatalf("Failed to write Makefile: %v", err)
	}

	parser := &MakefileParser{}
	cmdSet, err := parser.ParseBuildSystem(tmpDir)
	if err != nil {
		t.Fatalf("ParseBuildSystem failed: %v", err)
	}

	if cmdSet != nil {
		t.Errorf("Expected nil CommandSet for empty Makefile, got: %+v", cmdSet)
	}
}

func TestMakefileParser_Priority(t *testing.T) {
	parser := &MakefileParser{}
	priority := parser.Priority()

	if priority != 50 {
		t.Errorf("Expected priority 50, got %d", priority)
	}
}

func TestMakefileParser_TabIndentation(t *testing.T) {
	tmpDir := t.TempDir()
	makefilePath := filepath.Join(tmpDir, "Makefile")

	// Use actual tabs (Makefile requirement)
	makefileContent := "build:\n\tgo build ./...\n\ntest:\n\tgo test ./...\n"

	if err := os.WriteFile(makefilePath, []byte(makefileContent), 0644); err != nil {
		t.Fatalf("Failed to write Makefile: %v", err)
	}

	parser := &MakefileParser{}
	cmdSet, err := parser.ParseBuildSystem(tmpDir)
	if err != nil {
		t.Fatalf("ParseBuildSystem failed: %v", err)
	}

	if cmdSet == nil {
		t.Fatal("Expected CommandSet, got nil")
	}

	if cmdSet.Commands.Build != "make build" {
		t.Errorf("Expected build command 'make build', got '%s'", cmdSet.Commands.Build)
	}

	if cmdSet.Commands.Test.Full != "make test" {
		t.Errorf("Expected test command 'make test', got '%s'", cmdSet.Commands.Test.Full)
	}
}

func TestMakefileParser_CommandPrefixes(t *testing.T) {
	tmpDir := t.TempDir()
	makefilePath := filepath.Join(tmpDir, "Makefile")

	makefileContent := `build:
	@go build ./...

test:
	-go test ./...
`

	if err := os.WriteFile(makefilePath, []byte(makefileContent), 0644); err != nil {
		t.Fatalf("Failed to write Makefile: %v", err)
	}

	parser := &MakefileParser{}
	cmdSet, err := parser.ParseBuildSystem(tmpDir)
	if err != nil {
		t.Fatalf("ParseBuildSystem failed: %v", err)
	}

	if cmdSet == nil {
		t.Fatal("Expected CommandSet, got nil")
	}

	// Commands should be extracted even with @ and - prefixes
	if cmdSet.Commands.Build != "make build" {
		t.Errorf("Expected build command 'make build', got '%s'", cmdSet.Commands.Build)
	}

	if cmdSet.Commands.Test.Full != "make test" {
		t.Errorf("Expected test command 'make test', got '%s'", cmdSet.Commands.Test.Full)
	}
}

func TestMakefileParser_TransitiveDependencies(t *testing.T) {
	tmpDir := t.TempDir()
	makefilePath := filepath.Join(tmpDir, "Makefile")

	// 3-level chain: test → integration → test-unit (only test-unit has commands)
	makefileContent := "test: integration\nintegration: test-unit\ntest-unit:\n\tgo test ./... -short\n"

	if err := os.WriteFile(makefilePath, []byte(makefileContent), 0644); err != nil {
		t.Fatalf("Failed to write Makefile: %v", err)
	}

	parser := &MakefileParser{}
	cmdSet, err := parser.ParseBuildSystem(tmpDir)
	if err != nil {
		t.Fatalf("ParseBuildSystem failed: %v", err)
	}

	if cmdSet == nil {
		t.Fatal("Expected CommandSet, got nil")
	}

	// Should resolve transitively to test-unit
	if cmdSet.Commands.Test.Full != "make test-unit" {
		t.Errorf("Expected test command 'make test-unit', got '%s'", cmdSet.Commands.Test.Full)
	}
}

func TestMakefileParser_MultipleTargetsOfSameType(t *testing.T) {
	tmpDir := t.TempDir()
	makefilePath := filepath.Join(tmpDir, "Makefile")

	makefileContent := `build:
	go build ./...

build-prod:
	go build -tags prod ./...

test:
	go test ./...

test-integration:
	go test -tags integration ./...
`

	if err := os.WriteFile(makefilePath, []byte(makefileContent), 0644); err != nil {
		t.Fatalf("Failed to write Makefile: %v", err)
	}

	parser := &MakefileParser{}
	cmdSet, err := parser.ParseBuildSystem(tmpDir)
	if err != nil {
		t.Fatalf("ParseBuildSystem failed: %v", err)
	}

	if cmdSet == nil {
		t.Fatal("Expected CommandSet, got nil")
	}

	// Should pick the first match
	if cmdSet.Commands.Build != "make build" {
		t.Errorf("Expected build command 'make build', got '%s'", cmdSet.Commands.Build)
	}

	if cmdSet.Commands.Test.Full != "make test" {
		t.Errorf("Expected test command 'make test', got '%s'", cmdSet.Commands.Test.Full)
	}
}
