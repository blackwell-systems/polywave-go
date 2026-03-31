package protocol

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// setupIntegrationTestRepo creates a temporary Go repo with the given files.
// Returns the repo path.
func setupIntegrationTestRepo(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for relPath, content := range files {
		absPath := filepath.Join(dir, relPath)
		if err := os.MkdirAll(filepath.Dir(absPath), 0755); err != nil {
			t.Fatalf("failed to create dir for %s: %v", relPath, err)
		}
		if err := os.WriteFile(absPath, []byte(content), 0644); err != nil {
			t.Fatalf("failed to write %s: %v", relPath, err)
		}
	}
	return dir
}

func TestValidateIntegration_NoGaps(t *testing.T) {
	repoPath := setupIntegrationTestRepo(t, map[string]string{
		"pkg/foo/foo.go": `package foo

func NewFoo() *Foo { return &Foo{} }

type Foo struct{}
`,
		"cmd/main.go": `package main

import "example/pkg/foo"

func main() {
	f := foo.NewFoo()
	_ = f
}
`,
	})

	manifest := &IMPLManifest{
		Waves: []Wave{
			{Number: 1, Agents: []Agent{{ID: "A", Files: []string{"pkg/foo/foo.go"}}}},
		},
		CompletionReports: map[string]CompletionReport{
			"A": {
				Status:       "complete",
				FilesChanged: []string{"pkg/foo/foo.go"},
			},
		},
	}

	report, err := ValidateIntegration(manifest, 1, repoPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !report.Valid {
		t.Errorf("expected valid report, got %d gaps: %v", len(report.Gaps), report.Gaps)
		for _, g := range report.Gaps {
			t.Logf("  gap: %s (%s) in %s", g.ExportName, g.Category, g.FilePath)
		}
	}
	if report.Wave != 1 {
		t.Errorf("expected wave 1, got %d", report.Wave)
	}
}

func TestValidateIntegration_DetectsGap(t *testing.T) {
	repoPath := setupIntegrationTestRepo(t, map[string]string{
		"pkg/bar/bar.go": `package bar

// NewBar creates a new Bar instance.
func NewBar() *Bar { return &Bar{} }

// Bar is an unused type.
type Bar struct {
	Name string
}

// RunBar executes bar logic.
func RunBar() error { return nil }
`,
		// No other file references NewBar or RunBar
		"cmd/main.go": `package main

func main() {
	// does not use bar at all
}
`,
	})

	manifest := &IMPLManifest{
		Waves: []Wave{
			{Number: 1, Agents: []Agent{{ID: "B", Files: []string{"pkg/bar/bar.go"}}}},
		},
		CompletionReports: map[string]CompletionReport{
			"B": {
				Status:       "complete",
				FilesCreated: []string{"pkg/bar/bar.go"},
			},
		},
	}

	report, err := ValidateIntegration(manifest, 1, repoPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if report.Valid {
		t.Error("expected invalid report (gaps detected), got valid")
	}
	if len(report.Gaps) == 0 {
		t.Fatal("expected at least one gap")
	}

	// Check that NewBar and RunBar are detected as gaps
	foundNewBar := false
	foundRunBar := false
	for _, gap := range report.Gaps {
		if gap.ExportName == "NewBar" {
			foundNewBar = true
			if gap.Severity != "error" {
				t.Errorf("NewBar should be error severity, got %s", gap.Severity)
			}
			if gap.AgentID != "B" {
				t.Errorf("expected agent B, got %s", gap.AgentID)
			}
		}
		if gap.ExportName == "RunBar" {
			foundRunBar = true
		}
	}
	if !foundNewBar {
		t.Error("expected gap for NewBar")
	}
	if !foundRunBar {
		t.Error("expected gap for RunBar")
	}

	// Summary should mention gaps
	if !strings.Contains(report.Summary, "gap") {
		t.Errorf("summary should mention gaps: %s", report.Summary)
	}
}

func TestValidateIntegration_SkipsPreExisting(t *testing.T) {
	// Pre-existing exports that are referenced elsewhere should not be flagged.
	// Here, NewOld is referenced in cmd/main.go so it should not be a gap.
	repoPath := setupIntegrationTestRepo(t, map[string]string{
		"pkg/old/old.go": `package old

func NewOld() *Old { return &Old{} }

type Old struct{}
`,
		"cmd/main.go": `package main

import "example/pkg/old"

func main() {
	o := old.NewOld()
	_ = o
}
`,
	})

	manifest := &IMPLManifest{
		Waves: []Wave{
			{Number: 1, Agents: []Agent{{ID: "C", Files: []string{"pkg/old/old.go"}}}},
		},
		CompletionReports: map[string]CompletionReport{
			"C": {
				Status:       "complete",
				FilesChanged: []string{"pkg/old/old.go"},
			},
		},
	}

	report, err := ValidateIntegration(manifest, 1, repoPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// NewOld is referenced in cmd/main.go, so no gap
	if !report.Valid {
		for _, g := range report.Gaps {
			if g.ExportName == "NewOld" {
				t.Errorf("NewOld should not be flagged as a gap (it is referenced in cmd/main.go)")
			}
		}
	}
}

func TestValidateIntegration_EmptyWave(t *testing.T) {
	repoPath := setupIntegrationTestRepo(t, map[string]string{
		"cmd/main.go": `package main

func main() {}
`,
	})

	manifest := &IMPLManifest{
		Waves: []Wave{
			{Number: 1, Agents: []Agent{{ID: "D", Files: []string{"pkg/empty/empty.go"}}}},
		},
		CompletionReports: map[string]CompletionReport{},
	}

	report, err := ValidateIntegration(manifest, 1, repoPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !report.Valid {
		t.Error("expected valid report for empty wave (no completion reports)")
	}
	if len(report.Gaps) != 0 {
		t.Errorf("expected 0 gaps, got %d", len(report.Gaps))
	}
}

func TestAppendIntegrationReport(t *testing.T) {
	// Create a minimal manifest YAML
	dir := t.TempDir()
	manifestPath := filepath.Join(dir, "impl.yaml")

	manifest := &IMPLManifest{
		Title:       "Test",
		FeatureSlug: "test-feature",
		Waves: []Wave{
			{Number: 1, Agents: []Agent{{ID: "A"}}},
		},
		CompletionReports: map[string]CompletionReport{
			"A": {Status: "complete"},
		},
	}

	data, err := yaml.Marshal(manifest)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}
	if err := os.WriteFile(manifestPath, data, 0644); err != nil {
		t.Fatalf("failed to write: %v", err)
	}

	report := &IntegrationReport{
		Wave: 1,
		Gaps: []IntegrationGap{
			{
				ExportName: "NewFoo",
				FilePath:   "pkg/foo/foo.go",
				AgentID:    "A",
				Category:   "function_call",
				Severity:   "error",
				Reason:     "no call-sites",
			},
		},
		Valid:   false,
		Summary: "1 gap",
	}

	appendRes := AppendIntegrationReport(manifestPath, "wave1", report)
	if appendRes.IsFatal() {
		t.Fatalf("AppendIntegrationReport failed: %v", appendRes.Errors)
	}
	if !appendRes.GetData().Appended {
		t.Error("expected Appended=true on success")
	}

	// Read back and verify
	content, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("failed to read back: %v", err)
	}

	// Parse the raw YAML to check integration_reports section exists
	var raw map[string]interface{}
	if err := yaml.Unmarshal(content, &raw); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	irRaw, ok := raw["integration_reports"]
	if !ok {
		t.Fatal("integration_reports key not found in output")
	}

	irMap, ok := irRaw.(map[string]interface{})
	if !ok {
		t.Fatalf("integration_reports is not a map, got %T", irRaw)
	}

	wave1Raw, ok := irMap["wave1"]
	if !ok {
		t.Fatal("wave1 key not found in integration_reports")
	}

	// Re-marshal and unmarshal to verify structure
	wave1Bytes, err := yaml.Marshal(wave1Raw)
	if err != nil {
		t.Fatalf("failed to marshal wave1: %v", err)
	}

	var recovered IntegrationReport
	if err := yaml.Unmarshal(wave1Bytes, &recovered); err != nil {
		t.Fatalf("failed to unmarshal report: %v", err)
	}

	if recovered.Wave != 1 {
		t.Errorf("expected wave 1, got %d", recovered.Wave)
	}
	if recovered.Valid {
		t.Error("expected valid=false")
	}
	if len(recovered.Gaps) != 1 {
		t.Fatalf("expected 1 gap, got %d", len(recovered.Gaps))
	}
	if recovered.Gaps[0].ExportName != "NewFoo" {
		t.Errorf("expected NewFoo gap, got %s", recovered.Gaps[0].ExportName)
	}

	// Test overwriting the same wave key
	report2 := &IntegrationReport{
		Wave:    1,
		Gaps:    []IntegrationGap{},
		Valid:   true,
		Summary: "no gaps after fix",
	}
	appendRes2 := AppendIntegrationReport(manifestPath, "wave1", report2)
	if appendRes2.IsFatal() {
		t.Fatalf("AppendIntegrationReport (overwrite) failed: %v", appendRes2.Errors)
	}

	content2, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("failed to read: %v", err)
	}
	var raw2 map[string]interface{}
	if err := yaml.Unmarshal(content2, &raw2); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	irMap2, _ := raw2["integration_reports"].(map[string]interface{})
	wave1Bytes2, _ := yaml.Marshal(irMap2["wave1"])
	var recovered2 IntegrationReport
	if err := yaml.Unmarshal(wave1Bytes2, &recovered2); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if !recovered2.Valid {
		t.Error("expected valid=true after overwrite")
	}
}

func TestAppendIntegrationReport_FatalOnMissingManifest(t *testing.T) {
	report := &IntegrationReport{Wave: 1, Gaps: []IntegrationGap{}, Valid: true, Summary: "ok"}
	res := AppendIntegrationReport("/nonexistent/path/impl.yaml", "wave1", report)
	if !res.IsFatal() {
		t.Error("expected FATAL result when manifest does not exist")
	}
	if len(res.Errors) == 0 {
		t.Error("expected at least one error in FATAL result")
	}
	if res.Errors[0].Code != "INTEGRATION_APPEND_FAILED" {
		t.Errorf("expected error code INTEGRATION_APPEND_FAILED, got %s", res.Errors[0].Code)
	}
}

func TestClassifyExport(t *testing.T) {
	tests := []struct {
		name     string
		kind     string
		expected string
	}{
		{"NewFoo", "func", "function_call"},
		{"Bar", "type", "type_usage"},
		{"DoSomething", "method", "function_call"},
		{"Name", "field", "field_init"},
	}
	for _, tt := range tests {
		got := ClassifyExport(tt.name, tt.kind)
		if got != tt.expected {
			t.Errorf("ClassifyExport(%q, %q) = %q, want %q", tt.name, tt.kind, got, tt.expected)
		}
	}
}

func TestIsIntegrationRequired(t *testing.T) {
	tests := []struct {
		name     string
		category string
		expected bool
	}{
		{"NewFoo", "function_call", true},
		{"BuildBar", "function_call", true},
		{"RegisterHandler", "function_call", true},
		{"helperFunc", "function_call", false}, // lowercase, wouldn't be exported anyway
		{"FooType", "type_usage", false},  // no action prefix/suffix
		{"Name", "field_init", true},      // all field_init requires integration
	}
	for _, tt := range tests {
		got := IsIntegrationRequired(tt.name, tt.category)
		if got != tt.expected {
			t.Errorf("IsIntegrationRequired(%q, %q) = %v, want %v", tt.name, tt.category, got, tt.expected)
		}
	}
}
