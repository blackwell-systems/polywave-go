package commands

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGithubActionsParser_ValidWorkflow(t *testing.T) {
	// Create temp directory with a valid GitHub Actions workflow
	tmpDir := t.TempDir()
	workflowsDir := filepath.Join(tmpDir, ".github", "workflows")
	if err := os.MkdirAll(workflowsDir, 0755); err != nil {
		t.Fatalf("failed to create workflows dir: %v", err)
	}

	workflowYAML := `name: CI
on: [push, pull_request]
jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      - name: Build
        run: go build ./...
      - name: Test
        run: go test -v ./...
      - name: Lint
        run: go vet ./...
      - name: Format check
        run: gofmt -l .
`

	workflowPath := filepath.Join(workflowsDir, "ci.yml")
	if err := os.WriteFile(workflowPath, []byte(workflowYAML), 0644); err != nil {
		t.Fatalf("failed to write workflow file: %v", err)
	}

	parser := &GithubActionsParser{}
	cmdSet, err := parser.ParseCI(tmpDir)
	if err != nil {
		t.Fatalf("ParseCI failed: %v", err)
	}

	if cmdSet == nil {
		t.Fatal("expected non-nil CommandSet")
	}

	// Verify build command
	if cmdSet.Commands.Build == "" {
		t.Error("expected build command to be extracted")
	}
	if cmdSet.Commands.Build != "go build ./..." {
		t.Errorf("expected build command 'go build ./...', got '%s'", cmdSet.Commands.Build)
	}

	// Verify test command
	if cmdSet.Commands.Test.Full == "" {
		t.Error("expected test command to be extracted")
	}

	// Verify lint command
	if cmdSet.Commands.Lint.Check == "" {
		t.Error("expected lint command to be extracted")
	}

	// Verify format command
	if cmdSet.Commands.Format.Check == "" {
		t.Error("expected format command to be extracted")
	}

	// Verify toolchain detection
	if cmdSet.Toolchain != "go" {
		t.Errorf("expected toolchain 'go', got '%s'", cmdSet.Toolchain)
	}

	// Verify detection sources
	if len(cmdSet.DetectionSources) != 1 {
		t.Errorf("expected 1 detection source, got %d", len(cmdSet.DetectionSources))
	}
}

func TestGithubActionsParser_MatrixBuild(t *testing.T) {
	// Create temp directory with a matrix build workflow
	tmpDir := t.TempDir()
	workflowsDir := filepath.Join(tmpDir, ".github", "workflows")
	if err := os.MkdirAll(workflowsDir, 0755); err != nil {
		t.Fatalf("failed to create workflows dir: %v", err)
	}

	workflowYAML := `name: Matrix CI
on: [push]
jobs:
  test:
    strategy:
      matrix:
        os: [ubuntu-latest, macos-latest, windows-latest]
        go: ["1.21", "1.22"]
    runs-on: ${{ matrix.os }}
    steps:
      - uses: actions/checkout@v3
      - name: Setup Go
        uses: actions/setup-go@v4
        with:
          go-version: ${{ matrix.go }}
      - name: Build
        run: go build -v ./...
      - name: Test
        run: go test -race -coverprofile=coverage.txt ./...
`

	workflowPath := filepath.Join(workflowsDir, "matrix.yml")
	if err := os.WriteFile(workflowPath, []byte(workflowYAML), 0644); err != nil {
		t.Fatalf("failed to write workflow file: %v", err)
	}

	parser := &GithubActionsParser{}
	cmdSet, err := parser.ParseCI(tmpDir)
	if err != nil {
		t.Fatalf("ParseCI failed: %v", err)
	}

	if cmdSet == nil {
		t.Fatal("expected non-nil CommandSet")
	}

	// Should extract commands from the matrix job (matching current OS)
	if cmdSet.Commands.Build == "" {
		t.Error("expected build command to be extracted from matrix job")
	}
	if cmdSet.Commands.Test.Full == "" {
		t.Error("expected test command to be extracted from matrix job")
	}
}

func TestGithubActionsParser_NoWorkflows(t *testing.T) {
	// Create temp directory without .github/workflows
	tmpDir := t.TempDir()

	parser := &GithubActionsParser{}
	cmdSet, err := parser.ParseCI(tmpDir)
	if err != nil {
		t.Fatalf("ParseCI failed: %v", err)
	}

	// Should return nil (not error) when directory doesn't exist
	if cmdSet != nil {
		t.Errorf("expected nil CommandSet when workflows directory absent, got %+v", cmdSet)
	}
}

func TestGithubActionsParser_MalformedYAML(t *testing.T) {
	// Create temp directory with malformed YAML
	tmpDir := t.TempDir()
	workflowsDir := filepath.Join(tmpDir, ".github", "workflows")
	if err := os.MkdirAll(workflowsDir, 0755); err != nil {
		t.Fatalf("failed to create workflows dir: %v", err)
	}

	malformedYAML := `name: Broken CI
on: [push
jobs:
  build:
    steps:
      - name: Build
        run: go build
      - this is not valid yaml at all
`

	workflowPath := filepath.Join(workflowsDir, "broken.yml")
	if err := os.WriteFile(workflowPath, []byte(malformedYAML), 0644); err != nil {
		t.Fatalf("failed to write workflow file: %v", err)
	}

	parser := &GithubActionsParser{}
	cmdSet, err := parser.ParseCI(tmpDir)

	// Should not fail - malformed files are skipped
	if err != nil {
		t.Fatalf("ParseCI should not fail on malformed YAML: %v", err)
	}

	// Should return nil since no valid workflows found
	if cmdSet != nil {
		t.Error("expected nil CommandSet when only malformed workflows present")
	}
}

func TestGithubActionsParser_Priority(t *testing.T) {
	parser := &GithubActionsParser{}
	priority := parser.Priority()

	if priority != 100 {
		t.Errorf("expected priority 100, got %d", priority)
	}
}

func TestGithubActionsParser_LintFixMode(t *testing.T) {
	// Test distinguishing between check and fix modes for linting
	tmpDir := t.TempDir()
	workflowsDir := filepath.Join(tmpDir, ".github", "workflows")
	if err := os.MkdirAll(workflowsDir, 0755); err != nil {
		t.Fatalf("failed to create workflows dir: %v", err)
	}

	workflowYAML := `name: Lint CI
on: [push]
jobs:
  lint:
    runs-on: ubuntu-latest
    steps:
      - name: Lint check
        run: cargo clippy -- -D warnings
      - name: Lint fix
        run: cargo clippy --fix
      - name: ESLint check
        run: eslint .
      - name: ESLint fix
        run: eslint --fix .
`

	workflowPath := filepath.Join(workflowsDir, "lint.yml")
	if err := os.WriteFile(workflowPath, []byte(workflowYAML), 0644); err != nil {
		t.Fatalf("failed to write workflow file: %v", err)
	}

	parser := &GithubActionsParser{}
	cmdSet, err := parser.ParseCI(tmpDir)
	if err != nil {
		t.Fatalf("ParseCI failed: %v", err)
	}

	if cmdSet == nil {
		t.Fatal("expected non-nil CommandSet")
	}

	// Verify check mode
	if cmdSet.Commands.Lint.Check == "" {
		t.Error("expected lint check command to be extracted")
	}

	// Verify fix mode
	if cmdSet.Commands.Lint.Fix == "" {
		t.Error("expected lint fix command to be extracted")
	}

	// Check should not contain --fix
	if cmdSet.Commands.Lint.Check != "" && contains(cmdSet.Commands.Lint.Check, "--fix") {
		t.Errorf("check command should not contain --fix: %s", cmdSet.Commands.Lint.Check)
	}

	// Fix should contain --fix
	if cmdSet.Commands.Lint.Fix != "" && !contains(cmdSet.Commands.Lint.Fix, "--fix") {
		t.Errorf("fix command should contain --fix: %s", cmdSet.Commands.Lint.Fix)
	}
}

func TestGithubActionsParser_FormatCheckMode(t *testing.T) {
	// Test distinguishing between check and fix modes for formatting
	tmpDir := t.TempDir()
	workflowsDir := filepath.Join(tmpDir, ".github", "workflows")
	if err := os.MkdirAll(workflowsDir, 0755); err != nil {
		t.Fatalf("failed to create workflows dir: %v", err)
	}

	workflowYAML := `name: Format CI
on: [push]
jobs:
  format:
    runs-on: ubuntu-latest
    steps:
      - name: Format check
        run: gofmt -l .
      - name: Format fix
        run: gofmt -w .
      - name: Prettier check
        run: prettier --check .
      - name: Prettier fix
        run: prettier --write .
`

	workflowPath := filepath.Join(workflowsDir, "format.yml")
	if err := os.WriteFile(workflowPath, []byte(workflowYAML), 0644); err != nil {
		t.Fatalf("failed to write workflow file: %v", err)
	}

	parser := &GithubActionsParser{}
	cmdSet, err := parser.ParseCI(tmpDir)
	if err != nil {
		t.Fatalf("ParseCI failed: %v", err)
	}

	if cmdSet == nil {
		t.Fatal("expected non-nil CommandSet")
	}

	// Verify check mode
	if cmdSet.Commands.Format.Check == "" {
		t.Error("expected format check command to be extracted")
	}

	// Verify fix mode
	if cmdSet.Commands.Format.Fix == "" {
		t.Error("expected format fix command to be extracted")
	}
}

func TestGithubActionsParser_MultilineCommands(t *testing.T) {
	// Test parsing multi-line run commands
	tmpDir := t.TempDir()
	workflowsDir := filepath.Join(tmpDir, ".github", "workflows")
	if err := os.MkdirAll(workflowsDir, 0755); err != nil {
		t.Fatalf("failed to create workflows dir: %v", err)
	}

	workflowYAML := `name: Multi-line CI
on: [push]
jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - name: Multi-line build and test
        run: |
          go build ./...
          go test ./...
          # This is a comment and should be ignored
          go vet ./...
`

	workflowPath := filepath.Join(workflowsDir, "multiline.yml")
	if err := os.WriteFile(workflowPath, []byte(workflowYAML), 0644); err != nil {
		t.Fatalf("failed to write workflow file: %v", err)
	}

	parser := &GithubActionsParser{}
	cmdSet, err := parser.ParseCI(tmpDir)
	if err != nil {
		t.Fatalf("ParseCI failed: %v", err)
	}

	if cmdSet == nil {
		t.Fatal("expected non-nil CommandSet")
	}

	// Should extract all non-comment commands
	if cmdSet.Commands.Build == "" {
		t.Error("expected build command from multi-line run")
	}
	if cmdSet.Commands.Test.Full == "" {
		t.Error("expected test command from multi-line run")
	}
	if cmdSet.Commands.Lint.Check == "" {
		t.Error("expected vet command from multi-line run")
	}
}

func TestGithubActionsParser_ToolchainDetection(t *testing.T) {
	tests := []struct {
		name       string
		workflow   string
		wantTool   string
	}{
		{
			name: "rust toolchain",
			workflow: `name: Rust CI
jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      - run: cargo build
      - run: cargo test
      - run: cargo clippy
`,
			wantTool: "rust",
		},
		{
			name: "node toolchain",
			workflow: `name: Node CI
jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      - run: npm install
      - run: npm test
      - run: npm run build
`,
			wantTool: "node",
		},
		{
			name: "python toolchain",
			workflow: `name: Python CI
jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      - run: python -m pytest
      - run: python setup.py build
`,
			wantTool: "python",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			workflowsDir := filepath.Join(tmpDir, ".github", "workflows")
			if err := os.MkdirAll(workflowsDir, 0755); err != nil {
				t.Fatalf("failed to create workflows dir: %v", err)
			}

			workflowPath := filepath.Join(workflowsDir, "ci.yml")
			if err := os.WriteFile(workflowPath, []byte(tt.workflow), 0644); err != nil {
				t.Fatalf("failed to write workflow file: %v", err)
			}

			parser := &GithubActionsParser{}
			cmdSet, err := parser.ParseCI(tmpDir)
			if err != nil {
				t.Fatalf("ParseCI failed: %v", err)
			}

			if cmdSet == nil {
				t.Fatal("expected non-nil CommandSet")
			}

			if cmdSet.Toolchain != tt.wantTool {
				t.Errorf("expected toolchain '%s', got '%s'", tt.wantTool, cmdSet.Toolchain)
			}
		})
	}
}

// Helper function
func contains(s, substr string) bool { return strings.Contains(s, substr) }
