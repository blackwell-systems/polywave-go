package retry_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/retry"
)

// minimalManifestYAML creates a minimal valid IMPL manifest YAML with a
// completion report for the given agentID.
func minimalManifestYAML(agentID, notes, verification string) string {
	return `title: "Test IMPL"
feature_slug: "test-feature"
verdict: "SUITABLE"
test_command: "go test ./..."
lint_command: "go vet ./..."
file_ownership: []
interface_contracts: []
waves:
  - number: 1
    agents:
      - id: "` + agentID + `"
        task: "do stuff"
        files: []
completion_reports:
  ` + agentID + `:
    status: "partial"
    notes: "` + notes + `"
    verification: "` + verification + `"
`
}

func TestBuildRetryAttempt_Complete(t *testing.T) {
	dir := t.TempDir()
	manifestPath := filepath.Join(dir, "impl.yaml")

	notes := "Build failed: cannot use x as type string in main.go"
	verification := "go build ./... exited with code 1"

	if err := os.WriteFile(manifestPath, []byte(minimalManifestYAML("A", notes, verification)), 0644); err != nil {
		t.Fatalf("failed to write test manifest: %v", err)
	}

	ra, err := retry.BuildRetryAttempt(manifestPath, "A", 2)
	if err != nil {
		t.Fatalf("BuildRetryAttempt() returned error: %v", err)
	}

	if ra.AgentID != "A" {
		t.Errorf("AgentID = %q; want %q", ra.AgentID, "A")
	}
	if ra.AttemptNumber != 2 {
		t.Errorf("AttemptNumber = %d; want 2", ra.AttemptNumber)
	}
	if ra.ErrorClass != retry.ErrorClassType {
		t.Errorf("ErrorClass = %v; want %v", ra.ErrorClass, retry.ErrorClassType)
	}
	if len(ra.SuggestedFixes) == 0 {
		t.Error("SuggestedFixes is empty; expected suggestions for type_error")
	}
	if ra.PriorNotes != notes {
		t.Errorf("PriorNotes = %q; want %q", ra.PriorNotes, notes)
	}
	if !strings.Contains(ra.PromptText, "## Retry Context (Attempt 2)") {
		t.Error("PromptText missing expected heading")
	}
}

func TestBuildRetryAttempt_ExcerptTruncation(t *testing.T) {
	dir := t.TempDir()
	manifestPath := filepath.Join(dir, "impl.yaml")

	// Build a YAML with notes > 2000 chars using block scalar to avoid quoting issues.
	yaml := "title: \"T\"\nfeature_slug: \"f\"\nverdict: \"SUITABLE\"\ntest_command: \"go test\"\nlint_command: \"go vet\"\nfile_ownership: []\ninterface_contracts: []\nwaves:\n  - number: 1\n    agents:\n      - id: \"A\"\n        task: \"t\"\n        files: []\ncompletion_reports:\n  A:\n    status: \"partial\"\n    notes: |\n"
	for i := 0; i < 200; i++ {
		yaml += "      error output line\n"
	}

	if err := os.WriteFile(manifestPath, []byte(yaml), 0644); err != nil {
		t.Fatalf("failed to write test manifest: %v", err)
	}

	ra, err := retry.BuildRetryAttempt(manifestPath, "A", 1)
	if err != nil {
		t.Fatalf("BuildRetryAttempt() returned error: %v", err)
	}

	if len(ra.ErrorExcerpt) > 2000 {
		t.Errorf("ErrorExcerpt length %d exceeds 2000-char limit", len(ra.ErrorExcerpt))
	}
}

func TestBuildRetryAttempt_FixableFailureType(t *testing.T) {
	dir := t.TempDir()
	manifestPath := filepath.Join(dir, "impl.yaml")

	notes := "install missing dep"
	yaml := `title: "Test IMPL"
feature_slug: "test-feature"
verdict: "SUITABLE"
test_command: "go test ./..."
lint_command: "go vet ./..."
file_ownership: []
interface_contracts: []
waves:
  - number: 1
    agents:
      - id: "A"
        task: "do stuff"
        files: []
completion_reports:
  A:
    status: "partial"
    failure_type: "fixable"
    notes: "install missing dep"
    verification: "FAIL"
`
	if err := os.WriteFile(manifestPath, []byte(yaml), 0644); err != nil {
		t.Fatalf("failed to write test manifest: %v", err)
	}

	ra, err := retry.BuildRetryAttempt(manifestPath, "A", 1)
	if err != nil {
		t.Fatalf("BuildRetryAttempt() returned error: %v", err)
	}

	if ra.FailureType != "fixable" {
		t.Errorf("FailureType = %q; want %q", ra.FailureType, "fixable")
	}
	if !strings.Contains(ra.PromptText, "## Fix Required") {
		t.Error("PromptText missing '## Fix Required' section for fixable failure")
	}
	if !strings.Contains(ra.PromptText, notes) {
		t.Errorf("PromptText missing notes %q in Fix Required section", notes)
	}
}

func TestBuildRetryAttempt_MissingReport(t *testing.T) {
	dir := t.TempDir()
	manifestPath := filepath.Join(dir, "impl.yaml")

	yaml := `title: "T"
feature_slug: "f"
verdict: "SUITABLE"
test_command: "go test"
lint_command: "go vet"
file_ownership: []
interface_contracts: []
waves:
  - number: 1
    agents:
      - id: "A"
        task: "t"
        files: []
`
	if err := os.WriteFile(manifestPath, []byte(yaml), 0644); err != nil {
		t.Fatalf("failed to write test manifest: %v", err)
	}

	_, err := retry.BuildRetryAttempt(manifestPath, "A", 1)
	if err == nil {
		t.Fatal("expected error for missing completion report, got nil")
	}
	if !strings.Contains(err.Error(), "no completion report found") {
		t.Errorf("error message %q does not mention missing report", err.Error())
	}
}

func TestBuildPromptText_FixRequired(t *testing.T) {
	attempt := retry.RetryAttempt{
		AttemptNumber:  1,
		ErrorClass:     retry.ErrorClassBuild,
		ErrorExcerpt:   "syntax error: unexpected token",
		SuggestedFixes: []string{"Check for syntax errors in recent changes"},
		PriorNotes:     "Run gofmt on all changed files",
		FailureType:    "fixable",
	}

	text := retry.BuildPromptText(attempt)

	if !strings.Contains(text, "## Fix Required") {
		t.Error("BuildPromptText missing '## Fix Required' section for fixable failure")
	}
	if !strings.Contains(text, "Run gofmt on all changed files") {
		t.Error("BuildPromptText missing PriorNotes in Fix Required section")
	}
	if !strings.Contains(text, "## Retry Context (Attempt 1)") {
		t.Error("BuildPromptText missing retry context heading")
	}
}
