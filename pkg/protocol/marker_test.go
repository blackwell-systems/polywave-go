package protocol

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestWriteCompletionMarker_YAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.yaml")

	// Create a YAML file
	content := map[string]interface{}{
		"title":        "Test Feature",
		"feature_slug": "test-feature",
	}
	data, err := yaml.Marshal(content)
	if err != nil {
		t.Fatalf("failed to marshal test YAML: %v", err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	// Write completion marker
	date := "2026-03-09"
	if err := WriteCompletionMarker(path, date); err != nil {
		t.Fatalf("WriteCompletionMarker failed: %v", err)
	}

	// Read back and verify
	resultData, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}

	var result map[string]interface{}
	if err := yaml.Unmarshal(resultData, &result); err != nil {
		t.Fatalf("failed to unmarshal result: %v", err)
	}

	// Check completion_date field
	completionDate, ok := result["completion_date"]
	if !ok {
		t.Fatal("completion_date field not found in YAML")
	}
	if completionDate != date {
		t.Errorf("expected completion_date '%s', got '%v'", date, completionDate)
	}

	// Verify original fields are preserved
	if result["title"] != "Test Feature" {
		t.Errorf("expected title 'Test Feature', got '%v'", result["title"])
	}
	if result["feature_slug"] != "test-feature" {
		t.Errorf("expected feature_slug 'test-feature', got '%v'", result["feature_slug"])
	}
}

func TestWriteCompletionMarker_YAML_Idempotent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.yaml")

	// Create a YAML file
	content := map[string]interface{}{
		"title": "Test Feature",
	}
	data, err := yaml.Marshal(content)
	if err != nil {
		t.Fatalf("failed to marshal test YAML: %v", err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	// Write completion marker twice with different dates
	date1 := "2026-03-09"
	date2 := "2026-03-10"
	if err := WriteCompletionMarker(path, date1); err != nil {
		t.Fatalf("first WriteCompletionMarker failed: %v", err)
	}
	if err := WriteCompletionMarker(path, date2); err != nil {
		t.Fatalf("second WriteCompletionMarker failed: %v", err)
	}

	// Read back and verify - second write should have updated the date
	resultData, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}

	var result map[string]interface{}
	if err := yaml.Unmarshal(resultData, &result); err != nil {
		t.Fatalf("failed to unmarshal result: %v", err)
	}

	// Should have the second date
	completionDate, ok := result["completion_date"]
	if !ok {
		t.Fatal("completion_date field not found in YAML")
	}
	if completionDate != date2 {
		t.Errorf("expected completion_date '%s', got '%v'", date2, completionDate)
	}
}

func TestWriteCompletionMarker_UnsupportedExtension(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.txt")

	// Create a text file
	if err := os.WriteFile(path, []byte("some content"), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	// Write completion marker - should fail
	date := "2026-03-09"
	err := WriteCompletionMarker(path, date)
	if err == nil {
		t.Fatal("expected error for unsupported file type, got nil")
	}
	if !strings.Contains(err.Error(), "unsupported file type") {
		t.Errorf("expected error about unsupported file type, got: %v", err)
	}
}
