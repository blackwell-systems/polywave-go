package protocol

import (
	"os"
	"path/filepath"
	"testing"
)

func TestListPrograms_MultipleFiles(t *testing.T) {
	tmpDir := t.TempDir()

	manifest1 := `title: "Program Alpha"
program_slug: "program-alpha"
state: "PLANNING"
impls:
  - slug: "impl-a"
    title: "Implementation A"
    tier: 1
    status: "pending"
tiers:
  - number: 1
    impls: ["impl-a"]
completion:
  tiers_complete: 0
  tiers_total: 1
  impls_complete: 0
  impls_total: 1
  total_agents: 0
  total_waves: 0
`

	manifest2 := `title: "Program Beta"
program_slug: "program-beta"
state: "TIER_EXECUTING"
impls:
  - slug: "impl-b"
    title: "Implementation B"
    tier: 1
    status: "in-progress"
tiers:
  - number: 1
    impls: ["impl-b"]
completion:
  tiers_complete: 0
  tiers_total: 1
  impls_complete: 0
  impls_total: 1
  total_agents: 0
  total_waves: 0
`

	programDir := filepath.Join(tmpDir, "PROGRAM")
	if err := os.MkdirAll(programDir, 0755); err != nil {
		t.Fatal(err)
	}

	path1 := filepath.Join(programDir, "PROGRAM-alpha.yaml")
	path2 := filepath.Join(programDir, "PROGRAM-beta.yaml")

	if err := os.WriteFile(path1, []byte(manifest1), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path2, []byte(manifest2), 0644); err != nil {
		t.Fatal(err)
	}

	// List programs
	programs, err := ListPrograms(tmpDir)
	if err != nil {
		t.Fatalf("ListPrograms failed: %v", err)
	}

	// Verify two programs returned
	if len(programs) != 2 {
		t.Fatalf("expected 2 programs, got %d", len(programs))
	}

	// Verify first program (alpha - sorted by filename)
	prog1 := programs[0]
	if prog1.Path != path1 {
		t.Errorf("expected path %s, got %s", path1, prog1.Path)
	}
	if prog1.Slug != "program-alpha" {
		t.Errorf("expected slug 'program-alpha', got '%s'", prog1.Slug)
	}
	if prog1.State != ProgramStatePlanning {
		t.Errorf("expected state 'PLANNING', got '%s'", prog1.State)
	}
	if prog1.Title != "Program Alpha" {
		t.Errorf("expected title 'Program Alpha', got '%s'", prog1.Title)
	}

	// Verify second program (beta)
	prog2 := programs[1]
	if prog2.Path != path2 {
		t.Errorf("expected path %s, got %s", path2, prog2.Path)
	}
	if prog2.Slug != "program-beta" {
		t.Errorf("expected slug 'program-beta', got '%s'", prog2.Slug)
	}
	if prog2.State != ProgramStateTierExecuting {
		t.Errorf("expected state 'TIER_EXECUTING', got '%s'", prog2.State)
	}
	if prog2.Title != "Program Beta" {
		t.Errorf("expected title 'Program Beta', got '%s'", prog2.Title)
	}
}

func TestListPrograms_EmptyDir(t *testing.T) {
	tmpDir := t.TempDir()

	// List programs in empty directory
	programs, err := ListPrograms(tmpDir)
	if err != nil {
		t.Fatalf("ListPrograms failed: %v", err)
	}

	// Verify empty slice returned
	if len(programs) != 0 {
		t.Errorf("expected empty slice, got %d programs", len(programs))
	}
}

func TestListPrograms_IgnoresNonProgram(t *testing.T) {
	tmpDir := t.TempDir()

	validProgramManifest := `title: "Valid Program"
program_slug: "valid-program"
state: "PLANNING"
impls:
  - slug: "impl-x"
    title: "Implementation X"
    tier: 1
    status: "pending"
tiers:
  - number: 1
    impls: ["impl-x"]
completion:
  tiers_complete: 0
  tiers_total: 1
  impls_complete: 0
  impls_total: 1
  total_agents: 0
  total_waves: 0
`

	implManifest := `title: "IMPL Feature"
feature_slug: "feature-impl"
verdict: "SUITABLE"
test_command: "go test ./..."
lint_command: "go vet ./..."
file_ownership: []
interface_contracts: []
waves:
  - number: 1
    agents:
      - id: "A"
        task: "Task A"
        files: ["file_a.go"]
`

	otherFile := `some random content
not a manifest
`

	programDir := filepath.Join(tmpDir, "PROGRAM")
	if err := os.MkdirAll(programDir, 0755); err != nil {
		t.Fatal(err)
	}

	programPath := filepath.Join(programDir, "PROGRAM-valid.yaml")
	implPath := filepath.Join(tmpDir, "IMPL-feature.yaml")
	otherPath := filepath.Join(tmpDir, "README.md")

	if err := os.WriteFile(programPath, []byte(validProgramManifest), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(implPath, []byte(implManifest), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(otherPath, []byte(otherFile), 0644); err != nil {
		t.Fatal(err)
	}

	// List programs
	programs, err := ListPrograms(tmpDir)
	if err != nil {
		t.Fatalf("ListPrograms failed: %v", err)
	}

	// Verify only one program (IMPL and README ignored)
	if len(programs) != 1 {
		t.Fatalf("expected 1 program, got %d", len(programs))
	}

	// Verify it's the valid program
	prog := programs[0]
	if prog.Path != programPath {
		t.Errorf("expected path %s, got %s", programPath, prog.Path)
	}
	if prog.Slug != "valid-program" {
		t.Errorf("expected slug 'valid-program', got '%s'", prog.Slug)
	}
}

func TestListPrograms_InvalidYAML(t *testing.T) {
	tmpDir := t.TempDir()

	validManifest := `title: "Valid Program"
program_slug: "valid-program"
state: "PLANNING"
impls:
  - slug: "impl-y"
    title: "Implementation Y"
    tier: 1
    status: "pending"
tiers:
  - number: 1
    impls: ["impl-y"]
completion:
  tiers_complete: 0
  tiers_total: 1
  impls_complete: 0
  impls_total: 1
  total_agents: 0
  total_waves: 0
`

	invalidManifest := `this is not valid YAML
{{{{ broken syntax
`

	programDir := filepath.Join(tmpDir, "PROGRAM")
	if err := os.MkdirAll(programDir, 0755); err != nil {
		t.Fatal(err)
	}

	validPath := filepath.Join(programDir, "PROGRAM-valid.yaml")
	invalidPath := filepath.Join(programDir, "PROGRAM-invalid.yaml")

	if err := os.WriteFile(validPath, []byte(validManifest), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(invalidPath, []byte(invalidManifest), 0644); err != nil {
		t.Fatal(err)
	}

	// List programs
	programs, err := ListPrograms(tmpDir)
	if err != nil {
		t.Fatalf("ListPrograms failed: %v", err)
	}

	// Verify only one program (invalid file skipped)
	if len(programs) != 1 {
		t.Fatalf("expected 1 program, got %d", len(programs))
	}

	// Verify it's the valid one
	prog := programs[0]
	if prog.Path != validPath {
		t.Errorf("expected path %s, got %s", validPath, prog.Path)
	}
	if prog.Slug != "valid-program" {
		t.Errorf("expected slug 'valid-program', got '%s'", prog.Slug)
	}
}

func TestListPrograms_Sorting(t *testing.T) {
	tmpDir := t.TempDir()

	manifest := `title: "Test Program"
program_slug: "test-program"
state: "PLANNING"
impls:
  - slug: "impl-z"
    title: "Implementation Z"
    tier: 1
    status: "pending"
tiers:
  - number: 1
    impls: ["impl-z"]
completion:
  tiers_complete: 0
  tiers_total: 1
  impls_complete: 0
  impls_total: 1
  total_agents: 0
  total_waves: 0
`

	// Create files with names that would sort differently
	programDir := filepath.Join(tmpDir, "PROGRAM")
	if err := os.MkdirAll(programDir, 0755); err != nil {
		t.Fatal(err)
	}

	path1 := filepath.Join(programDir, "PROGRAM-zebra.yaml")
	path2 := filepath.Join(programDir, "PROGRAM-alpha.yaml")
	path3 := filepath.Join(programDir, "PROGRAM-beta.yaml")

	for _, path := range []string{path1, path2, path3} {
		if err := os.WriteFile(path, []byte(manifest), 0644); err != nil {
			t.Fatal(err)
		}
	}

	// List programs
	programs, err := ListPrograms(tmpDir)
	if err != nil {
		t.Fatalf("ListPrograms failed: %v", err)
	}

	// Verify sorted by filename
	if len(programs) != 3 {
		t.Fatalf("expected 3 programs, got %d", len(programs))
	}

	// Filenames should be in alphabetical order
	if filepath.Base(programs[0].Path) != "PROGRAM-alpha.yaml" {
		t.Errorf("expected first filename to be PROGRAM-alpha.yaml, got %s", filepath.Base(programs[0].Path))
	}
	if filepath.Base(programs[1].Path) != "PROGRAM-beta.yaml" {
		t.Errorf("expected second filename to be PROGRAM-beta.yaml, got %s", filepath.Base(programs[1].Path))
	}
	if filepath.Base(programs[2].Path) != "PROGRAM-zebra.yaml" {
		t.Errorf("expected third filename to be PROGRAM-zebra.yaml, got %s", filepath.Base(programs[2].Path))
	}
}
