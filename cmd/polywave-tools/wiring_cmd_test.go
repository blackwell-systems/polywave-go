package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/blackwell-systems/polywave-go/pkg/engine"
	"github.com/blackwell-systems/polywave-go/pkg/protocol"
)

// TestCombinedReportJSON verifies that combinedReport marshals to expected JSON keys.
func TestCombinedReportJSON(t *testing.T) {
	r := &combinedReport{
		HeuristicReport: &protocol.IntegrationReport{Wave: 1, Valid: true, Gaps: []protocol.IntegrationGap{}},
		WiringReport:    &protocol.WiringValidationData{Valid: true, Gaps: []protocol.WiringGap{}},
		Valid:           true,
	}

	b, err := json.Marshal(r)
	if err != nil {
		t.Fatalf("failed to marshal combinedReport: %v", err)
	}

	s := string(b)
	if !strings.Contains(s, `"heuristic_report"`) {
		t.Error("expected heuristic_report key in JSON")
	}
	if !strings.Contains(s, `"wiring_report"`) {
		t.Error("expected wiring_report key in JSON")
	}
	if !strings.Contains(s, `"valid"`) {
		t.Error("expected valid key in JSON")
	}
}

// TestCombinedReportOmitEmpty verifies that nil reports are omitted from JSON output.
func TestCombinedReportOmitEmpty(t *testing.T) {
	r := &combinedReport{
		Valid: true,
		// HeuristicReport and WiringReport are nil
	}

	b, err := json.Marshal(r)
	if err != nil {
		t.Fatalf("failed to marshal combinedReport: %v", err)
	}

	s := string(b)
	if strings.Contains(s, `"heuristic_report"`) {
		t.Error("expected heuristic_report to be omitted when nil")
	}
	if strings.Contains(s, `"wiring_report"`) {
		t.Error("expected wiring_report to be omitted when nil")
	}
}

// TestAppendWiringReport_CreatesKey verifies appendWiringReport creates the
// wiring_validation_reports key when it does not exist.
func TestAppendWiringReport_CreatesKey(t *testing.T) {
	// Write a minimal manifest YAML without wiring_validation_reports
	dir := t.TempDir()
	manifestPath := filepath.Join(dir, "IMPL.yaml")
	content := `title: test
feature_slug: test-feature
verdict: SUITABLE
test_command: go test ./...
lint_command: go vet ./...
file_ownership: []
interface_contracts: []
waves: []
`
	if err := os.WriteFile(manifestPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write manifest: %v", err)
	}

	result := &protocol.WiringValidationData{
		Valid:   true,
		Gaps:    []protocol.WiringGap{},
		Summary: "no gaps",
	}

	if err := appendWiringReport(manifestPath, "wave1", result); err != nil {
		t.Fatalf("appendWiringReport failed: %v", err)
	}

	// Read back and verify key was created
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("failed to read manifest: %v", err)
	}

	if !strings.Contains(string(data), "wiring_validation_reports") {
		t.Error("expected wiring_validation_reports key to be present after append")
	}
	if !strings.Contains(string(data), "wave1") {
		t.Error("expected wave1 key under wiring_validation_reports")
	}
}

// TestAppendWiringReport_UpdatesExistingKey verifies appendWiringReport updates
// an existing wave key rather than creating a duplicate.
func TestAppendWiringReport_UpdatesExistingKey(t *testing.T) {
	dir := t.TempDir()
	manifestPath := filepath.Join(dir, "IMPL.yaml")
	content := `title: test
feature_slug: test-feature
verdict: SUITABLE
test_command: go test ./...
lint_command: go vet ./...
file_ownership: []
interface_contracts: []
waves: []
wiring_validation_reports:
  wave1:
    valid: false
    summary: "old result"
    gaps: []
`
	if err := os.WriteFile(manifestPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write manifest: %v", err)
	}

	updated := &protocol.WiringValidationData{
		Valid:   true,
		Gaps:    []protocol.WiringGap{},
		Summary: "updated result",
	}

	if err := appendWiringReport(manifestPath, "wave1", updated); err != nil {
		t.Fatalf("appendWiringReport failed: %v", err)
	}

	data, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("failed to read manifest: %v", err)
	}

	s := string(data)
	if strings.Contains(s, "old result") {
		t.Error("expected old summary to be replaced")
	}
	if !strings.Contains(s, "updated result") {
		t.Error("expected new summary to appear in manifest")
	}
}

// TestFinalizeWaveResultHasWiringReport verifies FinalizeWaveResult has the
// WiringReport field and it serializes correctly.
func TestFinalizeWaveResultHasWiringReport(t *testing.T) {
	result := &engine.FinalizeWaveResult{
		Wave:    1,
		Success: true,
		WiringReport: &protocol.WiringValidationData{
			Valid:   false,
			Summary: "1 wiring gap",
			Gaps: []protocol.WiringGap{
				{
					Declaration: protocol.WiringDeclaration{
						Symbol:           "NewFoo",
						DefinedIn:        "pkg/foo/foo.go",
						MustBeCalledFrom: "cmd/polywave-tools/main.go",
					},
					Reason:   "symbol not found in caller file",
					Severity: "error",
				},
			},
		},
	}

	b, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("failed to marshal FinalizeWaveResult: %v", err)
	}

	s := string(b)
	if !strings.Contains(s, `"wiring_report"`) {
		t.Error("expected wiring_report key in FinalizeWaveResult JSON")
	}
	if !strings.Contains(s, "1 wiring gap") {
		t.Error("expected wiring gap summary in JSON")
	}
}
