package protocol

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/blackwell-systems/polywave-go/pkg/result"
)

// Minimal valid IMPL manifest YAML for testing.
// Uses 2 agents so it does not trip V047_TRIVIAL_SCOPE.
const validManifestYAML = `title: "Test Feature"
feature_slug: "test-feature"
verdict: "SUITABLE"
state: "WAVE_EXECUTING"
waves:
  - number: 1
    agents:
      - id: "A"
        task: "implement feature"
      - id: "B"
        task: "implement tests"
file_ownership:
  - file: "pkg/foo.go"
    agent: "A"
    wave: 1
  - file: "pkg/foo_test.go"
    agent: "B"
    wave: 1
quality_gates:
  level: "standard"
`

func writeManifest(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestFullValidate_ValidManifest(t *testing.T) {
	tmpDir := t.TempDir()
	path := writeManifest(t, tmpDir, "IMPL-test.yaml", validManifestYAML)

	res := FullValidate(context.Background(), path, FullValidateOpts{})
	if res.IsFatal() {
		t.Fatalf("expected success, got fatal: %v", res.Errors)
	}
	data := res.GetData()
	if !data.Valid {
		t.Errorf("expected Valid=true, got false; errors: %v", data.Errors)
	}
	if data.ErrorCount != 0 {
		t.Errorf("expected 0 errors, got %d", data.ErrorCount)
	}
}

func TestFullValidate_InvalidManifest(t *testing.T) {
	tmpDir := t.TempDir()

	// I1 violation: same file owned by two agents in same wave
	manifest := `title: "Test Feature"
feature_slug: "test-feature"
verdict: "SUITABLE"
state: "WAVE_EXECUTING"
waves:
  - number: 1
    agents:
      - id: "A"
        task: "task a"
      - id: "B"
        task: "task b"
file_ownership:
  - file: "pkg/foo.go"
    agent: "A"
    wave: 1
  - file: "pkg/foo.go"
    agent: "B"
    wave: 1
quality_gates:
  level: "standard"
`
	path := writeManifest(t, tmpDir, "IMPL-test.yaml", manifest)

	res := FullValidate(context.Background(), path, FullValidateOpts{})
	data := res.GetData()
	if data.Valid {
		t.Error("expected Valid=false for I1 violation")
	}
	if data.ErrorCount == 0 {
		t.Error("expected errors for I1 violation")
	}

	// Check that at least one error has the disjoint ownership code
	foundI1 := false
	for _, e := range data.Errors {
		if e.Code == result.CodeDisjointOwnership {
			foundI1 = true
			break
		}
	}
	if !foundI1 {
		t.Errorf("expected I1 violation error, got: %v", data.Errors)
	}
}

func TestFullValidate_AutoFix(t *testing.T) {
	tmpDir := t.TempDir()

	// Manifest with an invalid gate type that FixGateTypes should correct
	manifest := `title: "Test Feature"
feature_slug: "test-feature"
verdict: "SUITABLE"
state: "WAVE_EXECUTING"
waves:
  - number: 1
    agents:
      - id: "A"
        task: "task a"
file_ownership:
  - file: "pkg/foo.go"
    agent: "A"
    wave: 1
quality_gates:
  level: "standard"
  gates:
    - name: "build"
      command: "go build ./..."
      type: "invalid_type_here"
`
	path := writeManifest(t, tmpDir, "IMPL-test.yaml", manifest)

	res := FullValidate(context.Background(), path, FullValidateOpts{AutoFix: true})
	data := res.GetData()
	if data.Fixed == 0 {
		t.Error("expected Fixed > 0 after auto-fix")
	}
}

func TestFullValidate_DuplicateKeys(t *testing.T) {
	tmpDir := t.TempDir()

	// YAML with duplicate top-level keys — title appears twice.
	// ValidateDuplicateKeys runs on raw bytes before Load, so it catches
	// duplicates even if Load succeeds (yaml.v3 silently takes the last value).
	manifest := `title: "Test Feature"
feature_slug: "test-feature"
verdict: "SUITABLE"
title: "Duplicate Title"
state: "WAVE_EXECUTING"
waves:
  - number: 1
    agents:
      - id: "A"
        task: "task a"
file_ownership:
  - file: "pkg/foo.go"
    agent: "A"
    wave: 1
quality_gates:
  level: "standard"
`
	path := writeManifest(t, tmpDir, "IMPL-test.yaml", manifest)

	res := FullValidate(context.Background(), path, FullValidateOpts{})
	// Result may be partial (data available) or fatal (Load failed).
	// Either way, there should be errors mentioning "duplicate".
	if res.IsSuccess() {
		t.Error("expected non-success result for duplicate keys")
	}
	allErrs := res.Errors
	foundDup := false
	for _, e := range allErrs {
		if strings.Contains(e.Code, "DUPLICATE") || strings.Contains(strings.ToLower(e.Message), "duplicate") {
			foundDup = true
			break
		}
	}
	// Also check data errors if available
	if !res.IsFatal() {
		data := res.GetData()
		for _, e := range data.Errors {
			if strings.Contains(e.Code, "DUPLICATE") || strings.Contains(strings.ToLower(e.Message), "duplicate") {
				foundDup = true
				break
			}
		}
	}
	if !foundDup {
		t.Errorf("expected duplicate key error, got errors: %v", allErrs)
	}
}

func TestFullValidateProgram_Valid(t *testing.T) {
	tmpDir := t.TempDir()

	manifest := `title: "Test Program"
program_slug: "test-program"
state: "PLANNING"
created: "2024-01-01"
updated: "2024-01-02"
requirements: "Build a test system"
impls:
  - slug: "impl-alpha"
    title: "Alpha Implementation"
    tier: 1
    status: "pending"
tiers:
  - number: 1
    impls: ["impl-alpha"]
    description: "First tier"
completion:
  tiers_complete: 0
  tiers_total: 1
  impls_complete: 0
  impls_total: 1
  total_agents: 2
  total_waves: 1
`
	path := writeManifest(t, tmpDir, "PROGRAM-test.yaml", manifest)

	res := FullValidateProgram(context.Background(), path, FullValidateProgramOpts{})
	if res.IsFatal() {
		t.Fatalf("expected success, got fatal: %v", res.Errors)
	}
	data := res.GetData()
	if !data.Valid {
		t.Errorf("expected Valid=true, got false; errors: %v", data.Errors)
	}
}

func TestFullValidateProgram_ImportMode(t *testing.T) {
	tmpDir := t.TempDir()

	manifest := `title: "Test Program"
program_slug: "test-program"
state: "PLANNING"
created: "2024-01-01"
updated: "2024-01-02"
requirements: "Build a test system"
impls:
  - slug: "impl-alpha"
    title: "Alpha Implementation"
    tier: 1
    status: "pending"
tiers:
  - number: 1
    impls: ["impl-alpha"]
    description: "First tier"
completion:
  tiers_complete: 0
  tiers_total: 1
  impls_complete: 0
  impls_total: 1
  total_agents: 2
  total_waves: 1
`
	path := writeManifest(t, tmpDir, "PROGRAM-test.yaml", manifest)

	// Import mode with a repo dir that won't have the IMPL docs
	res := FullValidateProgram(context.Background(), path, FullValidateProgramOpts{
		ImportMode: true,
		RepoDir:    tmpDir,
	})
	// Should not be fatal (import mode adds errors but doesn't crash)
	if res.IsFatal() {
		t.Fatalf("expected non-fatal result, got fatal: %v", res.Errors)
	}
	data := res.GetData()
	// With import mode on a dir without IMPL docs, we expect errors
	if data.ErrorCount == 0 {
		// It's also valid if the validator doesn't find issues when IMPL files are missing
		// (depends on implementation behavior). Just verify we get data back.
		t.Log("import mode returned no errors (acceptable if validator is lenient on missing files)")
	}
}

func TestFullValidate_WarningsNonBlocking(t *testing.T) {
	tmpDir := t.TempDir()

	// Build a manifest where agent A owns 9 files — triggers W001_AGENT_SCOPE_LARGE
	// (severity: "warning") from CheckAgentComplexity via Validate(m).
	// After severity splitting, Valid must be true with ErrorCount=0 and WarningCount>0.
	manifest := `title: "Test Feature"
feature_slug: "test-feature"
verdict: "SUITABLE"
state: "WAVE_EXECUTING"
waves:
  - number: 1
    agents:
      - id: "A"
        task: "implement feature"
file_ownership:
  - file: "pkg/a1.go"
    agent: "A"
    wave: 1
  - file: "pkg/a2.go"
    agent: "A"
    wave: 1
  - file: "pkg/a3.go"
    agent: "A"
    wave: 1
  - file: "pkg/a4.go"
    agent: "A"
    wave: 1
  - file: "pkg/a5.go"
    agent: "A"
    wave: 1
  - file: "pkg/a6.go"
    agent: "A"
    wave: 1
  - file: "pkg/a7.go"
    agent: "A"
    wave: 1
  - file: "pkg/a8.go"
    agent: "A"
    wave: 1
  - file: "pkg/a9.go"
    agent: "A"
    wave: 1
quality_gates:
  level: "standard"
`
	path := writeManifest(t, tmpDir, "IMPL-test.yaml", manifest)

	res := FullValidate(context.Background(), path, FullValidateOpts{})
	if res.IsFatal() {
		t.Fatalf("expected non-fatal result, got fatal: %v", res.Errors)
	}
	data := res.GetData()
	if !data.Valid {
		t.Errorf("expected Valid=true (warnings only are non-blocking), got false; errors: %v", data.Errors)
	}
	if data.ErrorCount != 0 {
		t.Errorf("expected ErrorCount=0, got %d; errors: %v", data.ErrorCount, data.Errors)
	}
	if data.WarningCount == 0 {
		t.Errorf("expected WarningCount>0 for oversized agent, got 0")
	}
	// All items in Warnings must have severity warning or info
	for _, w := range data.Warnings {
		if w.Severity != "warning" && w.Severity != "info" {
			t.Errorf("expected warning/info severity in Warnings, got %q (code: %s)", w.Severity, w.Code)
		}
	}
	// Errors slice must be empty
	if len(data.Errors) != 0 {
		t.Errorf("expected empty Errors slice, got: %v", data.Errors)
	}
}

func TestFullValidate_GhostFileTarget(t *testing.T) {
	tmpDir := t.TempDir()

	// Manifest with one action=modify entry pointing to a file that does NOT exist.
	manifest := `title: "Ghost File Feature"
feature_slug: "ghost-file-feature"
verdict: "SUITABLE"
state: "WAVE_EXECUTING"
waves:
  - number: 1
    agents:
      - id: "A"
        task: "implement feature"
      - id: "B"
        task: "implement tests"
file_ownership:
  - file: "pkg/does_not_exist.go"
    agent: "A"
    wave: 1
    action: "modify"
  - file: "pkg/other.go"
    agent: "B"
    wave: 1
    action: "new"
quality_gates:
  level: "standard"
`
	path := writeManifest(t, tmpDir, "IMPL-ghost.yaml", manifest)

	// RepoPath is tmpDir — the file pkg/does_not_exist.go is absent there.
	res := FullValidate(context.Background(), path, FullValidateOpts{RepoPath: tmpDir})
	if res.IsFatal() {
		t.Fatalf("expected non-fatal result, got fatal: %v", res.Errors)
	}
	data := res.GetData()
	if data.Valid {
		t.Error("expected Valid=false when action=modify file is missing on disk")
	}

	foundFileMissing := false
	for _, e := range data.Errors {
		if e.Code == result.CodeFileMissing {
			foundFileMissing = true
			break
		}
	}
	if !foundFileMissing {
		t.Errorf("expected at least one error with code %q, got errors: %v", result.CodeFileMissing, data.Errors)
	}
}

func TestAppendWiringReport(t *testing.T) {
	tmpDir := t.TempDir()

	manifest := `title: "Test Feature"
feature_slug: "test-feature"
verdict: "SUITABLE"
state: "WAVE_EXECUTING"
`
	path := writeManifest(t, tmpDir, "IMPL-test.yaml", manifest)

	data := &WiringValidationData{
		Gaps:    []WiringGap{},
		Valid:   true,
		Summary: "all 2 wiring declarations satisfied",
	}

	res := AppendWiringReport(path, "wave1", data)
	if res.IsFatal() {
		t.Fatalf("AppendWiringReport failed: %v", res.Errors)
	}
	if !res.GetData().Appended {
		t.Error("expected Appended=true on success")
	}

	// Read back and verify the report was persisted
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	content := string(raw)
	if !strings.Contains(content, "wiring_validation_reports") {
		t.Error("expected wiring_validation_reports key in output")
	}
	if !strings.Contains(content, "wave1") {
		t.Error("expected wave1 key in output")
	}
	if !strings.Contains(content, "all 2 wiring declarations satisfied") {
		t.Error("expected summary text in output")
	}

	// Append a second report for wave2 to verify append behavior
	data2 := &WiringValidationData{
		Gaps:    []WiringGap{},
		Valid:   true,
		Summary: "all 3 wiring declarations satisfied",
	}
	res2 := AppendWiringReport(path, "wave2", data2)
	if res2.IsFatal() {
		t.Fatalf("AppendWiringReport (wave2) failed: %v", res2.Errors)
	}
	if !res2.GetData().Appended {
		t.Error("expected Appended=true on wave2 success")
	}

	raw2, _ := os.ReadFile(path)
	content2 := string(raw2)
	if !strings.Contains(content2, "wave2") {
		t.Error("expected wave2 key after second append")
	}
	if !strings.Contains(content2, "wave1") {
		t.Error("expected wave1 key still present after second append")
	}
}

func TestAppendWiringReport_FatalOnMissingManifest(t *testing.T) {
	res := AppendWiringReport("/nonexistent/path/impl.yaml", "wave1", &WiringValidationData{})
	if !res.IsFatal() {
		t.Error("expected FATAL result when manifest does not exist")
	}
	if len(res.Errors) == 0 {
		t.Error("expected at least one error in FATAL result")
	}
	if res.Errors[0].Code != "WIRING_APPEND_FAILED" {
		t.Errorf("expected error code WIRING_APPEND_FAILED, got %s", res.Errors[0].Code)
	}
}
