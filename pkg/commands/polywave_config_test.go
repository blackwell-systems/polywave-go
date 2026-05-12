package commands

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPolywaveConfigParser_NoFile(t *testing.T) {
	tmpDir := t.TempDir()
	p := &PolywaveConfigParser{}
	r := p.ParseBuildSystem(tmpDir)
	if r.IsFatal() {
		t.Fatalf("expected success when no polywave.config.json, got: %v", r.Errors)
	}
	if r.GetData().CommandSet != nil {
		t.Error("expected nil CommandSet when no polywave.config.json")
	}
}

func TestPolywaveConfigParser_MalformedJSON(t *testing.T) {
	tmpDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmpDir, "polywave.config.json"), []byte("{bad json"), 0644); err != nil {
		t.Fatal(err)
	}
	p := &PolywaveConfigParser{}
	r := p.ParseBuildSystem(tmpDir)
	if r.IsFatal() {
		t.Fatalf("expected graceful skip on malformed JSON, got: %v", r.Errors)
	}
	if r.GetData().CommandSet != nil {
		t.Error("expected nil CommandSet on malformed JSON")
	}
}

func TestPolywaveConfigParser_EmptyCommands(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := `{"repos": [{"name": "test", "path": "/tmp/test"}]}`
	if err := os.WriteFile(filepath.Join(tmpDir, "polywave.config.json"), []byte(cfg), 0644); err != nil {
		t.Fatal(err)
	}
	p := &PolywaveConfigParser{}
	r := p.ParseBuildSystem(tmpDir)
	if r.IsFatal() {
		t.Fatalf("unexpected error: %v", r.Errors)
	}
	if r.GetData().CommandSet != nil {
		t.Error("expected nil CommandSet when no command fields present")
	}
}

func TestPolywaveConfigParser_GoCommands(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := `{
		"buildCommand": "GOWORK=off go build ./...",
		"testCommand":  "GOWORK=off go test ./internal/... ./cmd/...",
		"lintCommand":  "GOWORK=off go vet ./..."
	}`
	if err := os.WriteFile(filepath.Join(tmpDir, "polywave.config.json"), []byte(cfg), 0644); err != nil {
		t.Fatal(err)
	}
	p := &PolywaveConfigParser{}
	r := p.ParseBuildSystem(tmpDir)
	if r.IsFatal() {
		t.Fatalf("unexpected error: %v", r.Errors)
	}
	cmdSet := r.GetData().CommandSet
	if cmdSet == nil {
		t.Fatal("expected non-nil CommandSet")
	}
	if cmdSet.Commands.Build != "GOWORK=off go build ./..." {
		t.Errorf("unexpected build command: %q", cmdSet.Commands.Build)
	}
	if cmdSet.Commands.Test.Full != "GOWORK=off go test ./internal/... ./cmd/..." {
		t.Errorf("unexpected test command: %q", cmdSet.Commands.Test.Full)
	}
	if cmdSet.Commands.Lint.Check != "GOWORK=off go vet ./..." {
		t.Errorf("unexpected lint command: %q", cmdSet.Commands.Lint.Check)
	}
	if cmdSet.Toolchain != "go" {
		t.Errorf("expected toolchain 'go', got %q", cmdSet.Toolchain)
	}
	if len(cmdSet.DetectionSources) != 1 {
		t.Errorf("expected 1 detection source, got %d", len(cmdSet.DetectionSources))
	}
}

func TestPolywaveConfigParser_Priority(t *testing.T) {
	p := &PolywaveConfigParser{}
	if p.Priority() != 200 {
		t.Errorf("expected priority 200, got %d", p.Priority())
	}
}

func TestPolywaveConfigParser_PriorityAboveCI(t *testing.T) {
	// PolywaveConfigParser priority must exceed GithubActionsParser priority.
	pw := &PolywaveConfigParser{}
	ci := &GithubActionsParser{}
	if pw.Priority() <= ci.Priority() {
		t.Errorf("PolywaveConfigParser priority (%d) must be > GithubActionsParser priority (%d)",
			pw.Priority(), ci.Priority())
	}
}

func TestGithubActionsParser_JobLevelEnv(t *testing.T) {
	// Job-level env vars should be prepended to extracted commands.
	tmpDir := t.TempDir()
	workflowsDir := filepath.Join(tmpDir, ".github", "workflows")
	if err := os.MkdirAll(workflowsDir, 0755); err != nil {
		t.Fatal(err)
	}

	workflowYAML := `name: CI
on: [push]
jobs:
  unit-and-smoke:
    runs-on: ubuntu-latest
    env:
      GOWORK: "off"
    steps:
      - name: Build
        run: go build ./...
      - name: Vet
        run: go vet ./...
      - name: Test
        run: go test ./...
`
	if err := os.WriteFile(filepath.Join(workflowsDir, "ci.yml"), []byte(workflowYAML), 0644); err != nil {
		t.Fatal(err)
	}

	parser := &GithubActionsParser{}
	r := parser.ParseCI(tmpDir)
	if r.IsFatal() {
		t.Fatalf("ParseCI failed: %v", r.Errors)
	}
	cmdSet := r.GetData().CommandSet
	if cmdSet == nil {
		t.Fatal("expected non-nil CommandSet")
	}
	// All commands should have GOWORK=off prepended from the job-level env.
	if cmdSet.Commands.Build != "GOWORK=off go build ./..." {
		t.Errorf("build: expected GOWORK=off prefix, got %q", cmdSet.Commands.Build)
	}
	if cmdSet.Commands.Lint.Check != "GOWORK=off go vet ./..." {
		t.Errorf("lint: expected GOWORK=off prefix, got %q", cmdSet.Commands.Lint.Check)
	}
	if cmdSet.Commands.Test.Full != "GOWORK=off go test ./..." {
		t.Errorf("test: expected GOWORK=off prefix, got %q", cmdSet.Commands.Test.Full)
	}
}

func TestGithubActionsParser_StepEnvOverridesJobEnv(t *testing.T) {
	// Step-level env should override job-level env for the same key.
	tmpDir := t.TempDir()
	workflowsDir := filepath.Join(tmpDir, ".github", "workflows")
	if err := os.MkdirAll(workflowsDir, 0755); err != nil {
		t.Fatal(err)
	}

	workflowYAML := `name: CI
on: [push]
jobs:
  build:
    runs-on: ubuntu-latest
    env:
      FOO: "job"
    steps:
      - name: Default env
        run: go build ./...
      - name: Overridden env
        env:
          FOO: "step"
        run: go vet ./...
`
	if err := os.WriteFile(filepath.Join(workflowsDir, "ci.yml"), []byte(workflowYAML), 0644); err != nil {
		t.Fatal(err)
	}

	parser := &GithubActionsParser{}
	r := parser.ParseCI(tmpDir)
	if r.IsFatal() {
		t.Fatalf("ParseCI failed: %v", r.Errors)
	}
	cmdSet := r.GetData().CommandSet
	if cmdSet == nil {
		t.Fatal("expected non-nil CommandSet")
	}
	// Build command gets the job-level FOO=job prefix.
	if cmdSet.Commands.Build != "FOO=job go build ./..." {
		t.Errorf("build: expected FOO=job prefix, got %q", cmdSet.Commands.Build)
	}
	// Lint command gets the step-level FOO=step override.
	if cmdSet.Commands.Lint.Check != "FOO=step go vet ./..." {
		t.Errorf("lint: expected FOO=step override, got %q", cmdSet.Commands.Lint.Check)
	}
}
