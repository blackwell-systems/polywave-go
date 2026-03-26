package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/protocol"
)

// buildMinimalIMPLManifest creates a minimal IMPL manifest YAML with one wave and one agent,
// writes it to dir/IMPL.yaml, and returns the path. Used to satisfy
// protocol.Load / protocol.SetCompletionReport validation.
func buildMinimalIMPLManifest(t *testing.T, dir string, agentID string) string {
	t.Helper()
	implPath := filepath.Join(dir, "IMPL.yaml")
	content := fmt.Sprintf(`title: test
feature_slug: test
verdict: SUITABLE
test_command: go test ./...
lint_command: go vet ./...
file_ownership: []
interface_contracts: []
waves:
  - number: 1
    agents:
      - id: %s
        task: do work
        files: []
quality_gates:
  level: standard
  gates: []
`, agentID)
	if err := os.WriteFile(implPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write minimal IMPL manifest: %v", err)
	}
	return implPath
}

// --- TestRunWaveAgentStructured_APIBackend ----------------------------------------

// TestRunWaveAgentStructured_APIBackend verifies that runWaveAgentStructured
// correctly uses the API backend seam, unmarshals the JSON completion report, and
// saves it to the manifest when the API returns a valid structured response.
//
// We replace the runWaveAgentStructuredAPI var with a stub that returns a
// hard-coded JSON completion report, avoiding real HTTP calls.
func TestRunWaveAgentStructured_APIBackend(t *testing.T) {
	// Build a completion report JSON the stub will return.
	report := protocol.CompletionReport{
		Status:       "complete",
		Commit:       "abc1234",
		FilesChanged: []string{"pkg/foo/bar.go"},
		FilesCreated: []string{"pkg/foo/baz.go"},
		Notes:        "all done",
	}
	reportJSON, err := json.Marshal(report)
	if err != nil {
		t.Fatalf("marshal report: %v", err)
	}

	// Stub the API function to return the JSON without real HTTP calls.
	origFn := runWaveAgentStructuredAPI
	runWaveAgentStructuredAPI = func(_ context.Context, _, _, _ string, _ map[string]any, onChunk func(string)) (string, error) {
		if onChunk != nil {
			onChunk(string(reportJSON))
		}
		return string(reportJSON), nil
	}
	t.Cleanup(func() { runWaveAgentStructuredAPI = origFn })

	dir := t.TempDir()
	implPath := buildMinimalIMPLManifest(t, dir, "A")

	opts := RunWaveOpts{
		IMPLPath:  implPath,
		WaveModel: "claude-sonnet-4-5", // non-bedrock model → API path
	}
	agentSpec := protocol.Agent{ID: "A", Task: "do work"}

	got, err := runWaveAgentStructured(context.Background(), opts, agentSpec, dir, nil)
	if err != nil {
		t.Fatalf("runWaveAgentStructured returned error: %v", err)
	}
	if got == nil {
		t.Fatal("expected non-nil CompletionReport, got nil")
	}
	if got.Status != "complete" {
		t.Errorf("report.Status = %q, want %q", got.Status, "complete")
	}
	if len(got.FilesChanged) != 1 || got.FilesChanged[0] != "pkg/foo/bar.go" {
		t.Errorf("report.FilesChanged = %v, want [pkg/foo/bar.go]", got.FilesChanged)
	}
	if len(got.FilesCreated) != 1 || got.FilesCreated[0] != "pkg/foo/baz.go" {
		t.Errorf("report.FilesCreated = %v, want [pkg/foo/baz.go]", got.FilesCreated)
	}

	// Verify the report was persisted to the manifest.
	manifest, loadErr := protocol.Load(implPath)
	if loadErr != nil {
		t.Fatalf("failed to reload manifest: %v", loadErr)
	}
	saved, ok := manifest.CompletionReports["A"]
	if !ok {
		t.Fatal("completion report for agent A not saved in manifest")
	}
	if saved.Status != "complete" {
		t.Errorf("saved report.Status = %q, want %q", saved.Status, "complete")
	}
	if saved.Notes != "all done" {
		t.Errorf("saved report.Notes = %q, want %q", saved.Notes, "all done")
	}
}

// --- TestRunWaveAgentStructured_BedrockBackend -----------------------------------

// TestRunWaveAgentStructured_BedrockBackend verifies that runWaveAgentStructured
// routes to the Bedrock path when the model is prefixed with "bedrock:".
// We replace the Bedrock seam with a stub returning a valid JSON report.
func TestRunWaveAgentStructured_BedrockBackend(t *testing.T) {
	report := protocol.CompletionReport{
		Status: "complete",
		Commit: "def5678",
		Notes:  "bedrock agent done",
	}
	reportJSON, err := json.Marshal(report)
	if err != nil {
		t.Fatalf("marshal report: %v", err)
	}

	// Stub the Bedrock function.
	origFn := runWaveAgentStructuredBedrock
	runWaveAgentStructuredBedrock = func(_ context.Context, model, _, _ string, _ map[string]any, onChunk func(string)) (string, error) {
		// Verify it received the "bedrock:" prefix
		if !strings.HasPrefix(model, "bedrock:") {
			return "", fmt.Errorf("expected bedrock: prefix in model, got %q", model)
		}
		if onChunk != nil {
			onChunk(string(reportJSON))
		}
		return string(reportJSON), nil
	}
	t.Cleanup(func() { runWaveAgentStructuredBedrock = origFn })

	dir := t.TempDir()
	implPath := buildMinimalIMPLManifest(t, dir, "A")

	opts := RunWaveOpts{
		IMPLPath:  implPath,
		WaveModel: "bedrock:claude-sonnet-4-5",
	}
	agentSpec := protocol.Agent{ID: "A", Task: "do work"}

	got, err := runWaveAgentStructured(context.Background(), opts, agentSpec, dir, nil)
	if err != nil {
		t.Fatalf("runWaveAgentStructured with Bedrock stub returned error: %v", err)
	}
	if got == nil {
		t.Fatal("expected non-nil CompletionReport, got nil")
	}
	if got.Status != "complete" {
		t.Errorf("report.Status = %q, want complete", got.Status)
	}

	// Verify persistence.
	manifest, loadErr := protocol.Load(implPath)
	if loadErr != nil {
		t.Fatalf("failed to reload manifest: %v", loadErr)
	}
	if _, ok := manifest.CompletionReports["A"]; !ok {
		t.Fatal("completion report for agent A not saved in manifest (Bedrock path)")
	}
}

// --- TestRunWaveAgentStructured_InvalidJSON --------------------------------------

// TestRunWaveAgentStructured_InvalidJSON verifies that runWaveAgentStructured
// returns an error containing "unmarshal" when the backend returns malformed JSON.
func TestRunWaveAgentStructured_InvalidJSON(t *testing.T) {
	// Stub API to return invalid JSON.
	origFn := runWaveAgentStructuredAPI
	runWaveAgentStructuredAPI = func(_ context.Context, _, _, _ string, _ map[string]any, _ func(string)) (string, error) {
		return "{ not valid json !!!", nil
	}
	t.Cleanup(func() { runWaveAgentStructuredAPI = origFn })

	dir := t.TempDir()
	implPath := buildMinimalIMPLManifest(t, dir, "A")

	opts := RunWaveOpts{
		IMPLPath:  implPath,
		WaveModel: "claude-sonnet-4-5",
	}
	agentSpec := protocol.Agent{ID: "A", Task: "do work"}

	_, err := runWaveAgentStructured(context.Background(), opts, agentSpec, dir, nil)
	if err == nil {
		t.Fatal("expected error for invalid JSON response, got nil")
	}
	if !strings.Contains(err.Error(), "unmarshal") {
		t.Errorf("expected unmarshal error, got: %v", err)
	}
}

// --- TestRunWaveAgentStructured_SchemaGeneration --------------------------------

// TestRunWaveAgentStructured_SchemaGeneration verifies that
// protocol.GenerateCompletionReportSchema returns a non-empty schema that
// contains the expected "status" and "files_changed" fields, confirming it
// matches the CompletionReport struct.
func TestRunWaveAgentStructured_SchemaGeneration(t *testing.T) {
	schema, err := protocol.GenerateCompletionReportSchema()
	if err != nil {
		t.Fatalf("GenerateCompletionReportSchema returned error: %v", err)
	}
	if len(schema) == 0 {
		t.Fatal("GenerateCompletionReportSchema returned empty schema")
	}

	// The schema should be a JSON Schema object.
	schemaBytes, err := json.Marshal(schema)
	if err != nil {
		t.Fatalf("failed to marshal schema: %v", err)
	}
	schemaStr := string(schemaBytes)

	// Verify "status" field is described (it's a required field in CompletionReport).
	if !strings.Contains(schemaStr, "status") {
		t.Errorf("schema does not mention 'status' field; got: %s", schemaStr)
	}
	// Verify "files_changed" is present.
	if !strings.Contains(schemaStr, "files_changed") {
		t.Errorf("schema does not mention 'files_changed' field; got: %s", schemaStr)
	}
}

// --- TestRunWaveAgentStructured_PerAgentModelOverride ----------------------------

// TestRunWaveAgentStructured_PerAgentModelOverride verifies that per-agent model
// overrides are forwarded correctly to the backend seam.
func TestRunWaveAgentStructured_PerAgentModelOverride(t *testing.T) {
	var capturedModel string

	origFn := runWaveAgentStructuredAPI
	runWaveAgentStructuredAPI = func(_ context.Context, model, _, _ string, _ map[string]any, _ func(string)) (string, error) {
		capturedModel = model
		report := protocol.CompletionReport{Status: "complete", Commit: "ghi9012"}
		b, _ := json.Marshal(report)
		return string(b), nil
	}
	t.Cleanup(func() { runWaveAgentStructuredAPI = origFn })

	dir := t.TempDir()
	implPath := buildMinimalIMPLManifest(t, dir, "A")

	opts := RunWaveOpts{
		IMPLPath:  implPath,
		WaveModel: "claude-haiku-4-5", // default model
	}
	agentSpec := protocol.Agent{
		ID:    "A",
		Task:  "do work",
		Model: "claude-opus-4-6", // per-agent override
	}

	_, err := runWaveAgentStructured(context.Background(), opts, agentSpec, dir, nil)
	if err != nil {
		t.Fatalf("runWaveAgentStructured returned error: %v", err)
	}
	if capturedModel != "claude-opus-4-6" {
		t.Errorf("backend received model %q, want per-agent override %q", capturedModel, "claude-opus-4-6")
	}
}

// --- TestRunWaveAgentStructured_EmptyStatusDefaultsToComplete -------------------

// TestRunWaveAgentStructured_EmptyStatusDefaultsToComplete verifies that when the
// JSON response omits the status field, runWaveAgentStructured defaults it to "complete".
func TestRunWaveAgentStructured_EmptyStatusDefaultsToComplete(t *testing.T) {
	// Return a JSON object without the status field.
	origFn := runWaveAgentStructuredAPI
	runWaveAgentStructuredAPI = func(_ context.Context, _, _, _ string, _ map[string]any, _ func(string)) (string, error) {
		return `{"files_changed":["a.go"],"commit":"jkl3456"}`, nil
	}
	t.Cleanup(func() { runWaveAgentStructuredAPI = origFn })

	dir := t.TempDir()
	implPath := buildMinimalIMPLManifest(t, dir, "A")

	opts := RunWaveOpts{IMPLPath: implPath, WaveModel: "claude-sonnet-4-5"}
	agentSpec := protocol.Agent{ID: "A", Task: "do work"}

	got, err := runWaveAgentStructured(context.Background(), opts, agentSpec, dir, nil)
	if err != nil {
		t.Fatalf("runWaveAgentStructured returned error: %v", err)
	}
	if got.Status != "complete" {
		t.Errorf("empty status should default to 'complete', got %q", got.Status)
	}
}
