package protocol

import (
	"os"
	"path/filepath"
	"testing"
)

func TestListIMPLs_MultipleFiles(t *testing.T) {
	// Create temp directory with two valid IMPL manifests
	tmpDir := t.TempDir()

	manifest1 := `title: "Test Feature A"
feature_slug: "feature-a"
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
  - number: 2
    agents:
      - id: "B"
        task: "Task B"
        files: ["file_b.go"]
completion_reports:
  A:
    status: "complete"
    commit: "abc123"
`

	manifest2 := `title: "Test Feature B"
feature_slug: "feature-b"
verdict: "SUITABLE_WITH_CAVEATS"
test_command: "npm test"
lint_command: "npm run lint"
file_ownership: []
interface_contracts: []
waves:
  - number: 1
    agents:
      - id: "X"
        task: "Task X"
        files: ["file_x.go"]
      - id: "Y"
        task: "Task Y"
        files: ["file_y.go"]
`

	path1 := filepath.Join(tmpDir, "IMPL-feature-a.yaml")
	path2 := filepath.Join(tmpDir, "IMPL-feature-b.yml")

	if err := os.WriteFile(path1, []byte(manifest1), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path2, []byte(manifest2), 0644); err != nil {
		t.Fatal(err)
	}

	// List IMPLs
	result, err := ListIMPLs(tmpDir)
	if err != nil {
		t.Fatalf("ListIMPLs failed: %v", err)
	}

	// Verify two summaries returned
	if len(result.IMPLs) != 2 {
		t.Fatalf("expected 2 summaries, got %d", len(result.IMPLs))
	}

	// Verify first summary (feature-a)
	summary1 := result.IMPLs[0]
	if summary1.Path != path1 {
		t.Errorf("expected path %s, got %s", path1, summary1.Path)
	}
	if summary1.FeatureSlug != "feature-a" {
		t.Errorf("expected feature_slug 'feature-a', got '%s'", summary1.FeatureSlug)
	}
	if summary1.Verdict != "SUITABLE" {
		t.Errorf("expected verdict 'SUITABLE', got '%s'", summary1.Verdict)
	}
	if summary1.CurrentWave != 2 {
		t.Errorf("expected current_wave 2, got %d", summary1.CurrentWave)
	}
	if summary1.TotalWaves != 2 {
		t.Errorf("expected total_waves 2, got %d", summary1.TotalWaves)
	}

	// Verify second summary (feature-b)
	summary2 := result.IMPLs[1]
	if summary2.Path != path2 {
		t.Errorf("expected path %s, got %s", path2, summary2.Path)
	}
	if summary2.FeatureSlug != "feature-b" {
		t.Errorf("expected feature_slug 'feature-b', got '%s'", summary2.FeatureSlug)
	}
	if summary2.Verdict != "SUITABLE_WITH_CAVEATS" {
		t.Errorf("expected verdict 'SUITABLE_WITH_CAVEATS', got '%s'", summary2.Verdict)
	}
	if summary2.CurrentWave != 1 {
		t.Errorf("expected current_wave 1, got %d", summary2.CurrentWave)
	}
	if summary2.TotalWaves != 1 {
		t.Errorf("expected total_waves 1, got %d", summary2.TotalWaves)
	}
}

func TestListIMPLs_EmptyDir(t *testing.T) {
	// Create empty temp directory
	tmpDir := t.TempDir()

	// List IMPLs
	result, err := ListIMPLs(tmpDir)
	if err != nil {
		t.Fatalf("ListIMPLs failed: %v", err)
	}

	// Verify empty list returned
	if len(result.IMPLs) != 0 {
		t.Errorf("expected empty list, got %d summaries", len(result.IMPLs))
	}
}

func TestListIMPLs_InvalidDir(t *testing.T) {
	// Use a path that doesn't exist
	invalidDir := "/nonexistent/path/to/dir"

	// List IMPLs
	result, err := ListIMPLs(invalidDir)
	if err != nil {
		t.Fatalf("ListIMPLs failed: %v", err)
	}

	// Verify empty list returned (not an error)
	if len(result.IMPLs) != 0 {
		t.Errorf("expected empty list, got %d summaries", len(result.IMPLs))
	}
}

func TestListIMPLs_InvalidYAML(t *testing.T) {
	// Create temp directory with one valid and one invalid YAML file
	tmpDir := t.TempDir()

	validManifest := `title: "Valid Feature"
feature_slug: "valid-feature"
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

	invalidManifest := `this is not valid YAML
{{{{ broken syntax
`

	validPath := filepath.Join(tmpDir, "IMPL-valid.yaml")
	invalidPath := filepath.Join(tmpDir, "IMPL-invalid.yaml")

	if err := os.WriteFile(validPath, []byte(validManifest), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(invalidPath, []byte(invalidManifest), 0644); err != nil {
		t.Fatal(err)
	}

	// List IMPLs
	result, err := ListIMPLs(tmpDir)
	if err != nil {
		t.Fatalf("ListIMPLs failed: %v", err)
	}

	// Verify only one summary (invalid file should be skipped)
	if len(result.IMPLs) != 1 {
		t.Fatalf("expected 1 summary, got %d", len(result.IMPLs))
	}

	// Verify it's the valid one
	summary := result.IMPLs[0]
	if summary.Path != validPath {
		t.Errorf("expected path %s, got %s", validPath, summary.Path)
	}
	if summary.FeatureSlug != "valid-feature" {
		t.Errorf("expected feature_slug 'valid-feature', got '%s'", summary.FeatureSlug)
	}
}

func TestListIMPLs_CompleteFeature(t *testing.T) {
	// Test a feature where all agents are complete (CurrentWave returns nil)
	tmpDir := t.TempDir()

	manifest := `title: "Complete Feature"
feature_slug: "complete-feature"
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
  - number: 2
    agents:
      - id: "B"
        task: "Task B"
        files: ["file_b.go"]
completion_reports:
  A:
    status: "complete"
    commit: "abc123"
  B:
    status: "complete"
    commit: "def456"
`

	path := filepath.Join(tmpDir, "IMPL-complete.yaml")
	if err := os.WriteFile(path, []byte(manifest), 0644); err != nil {
		t.Fatal(err)
	}

	// List IMPLs
	result, err := ListIMPLs(tmpDir)
	if err != nil {
		t.Fatalf("ListIMPLs failed: %v", err)
	}

	// Verify one summary returned
	if len(result.IMPLs) != 1 {
		t.Fatalf("expected 1 summary, got %d", len(result.IMPLs))
	}

	// Verify current_wave equals total_waves (all complete)
	summary := result.IMPLs[0]
	if summary.CurrentWave != 2 {
		t.Errorf("expected current_wave 2, got %d", summary.CurrentWave)
	}
	if summary.TotalWaves != 2 {
		t.Errorf("expected total_waves 2, got %d", summary.TotalWaves)
	}
}

func TestListIMPLs_Sorting(t *testing.T) {
	// Test that results are sorted by path
	tmpDir := t.TempDir()

	manifest := `title: "Test"
feature_slug: "test"
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

	// Create files with names that would sort differently
	path1 := filepath.Join(tmpDir, "IMPL-zebra.yaml")
	path2 := filepath.Join(tmpDir, "IMPL-alpha.yaml")
	path3 := filepath.Join(tmpDir, "IMPL-beta.yml")

	for _, path := range []string{path1, path2, path3} {
		if err := os.WriteFile(path, []byte(manifest), 0644); err != nil {
			t.Fatal(err)
		}
	}

	// List IMPLs
	result, err := ListIMPLs(tmpDir)
	if err != nil {
		t.Fatalf("ListIMPLs failed: %v", err)
	}

	// Verify sorted order
	if len(result.IMPLs) != 3 {
		t.Fatalf("expected 3 summaries, got %d", len(result.IMPLs))
	}

	// Paths should be in alphabetical order
	if result.IMPLs[0].Path != path2 { // alpha
		t.Errorf("expected first path to be %s, got %s", path2, result.IMPLs[0].Path)
	}
	if result.IMPLs[1].Path != path3 { // beta
		t.Errorf("expected second path to be %s, got %s", path3, result.IMPLs[1].Path)
	}
	if result.IMPLs[2].Path != path1 { // zebra
		t.Errorf("expected third path to be %s, got %s", path1, result.IMPLs[2].Path)
	}
}
