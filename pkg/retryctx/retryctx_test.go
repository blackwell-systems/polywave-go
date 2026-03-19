package retryctx_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/retryctx"
)

// ---------------------------------------------------------------------------
// ClassifyError tests
// ---------------------------------------------------------------------------

func TestClassifyError_ImportError(t *testing.T) {
	cases := []string{
		"could not import github.com/foo/bar",
		"cannot find package \"foo\" in any of",
		"no required module provides github.com/foo",
	}
	for _, c := range cases {
		got := retryctx.ClassifyError(c)
		if got != retryctx.ErrorClassImport {
			t.Errorf("ClassifyError(%q) = %v; want %v", c, got, retryctx.ErrorClassImport)
		}
	}
}

func TestClassifyError_TypeError(t *testing.T) {
	cases := []string{
		"cannot use x as type string",
		"undefined: MyFunc",
		"has no field or method Foo",
		"not enough arguments in call to bar",
		"too many arguments in call to baz",
	}
	for _, c := range cases {
		got := retryctx.ClassifyError(c)
		if got != retryctx.ErrorClassType {
			t.Errorf("ClassifyError(%q) = %v; want %v", c, got, retryctx.ErrorClassType)
		}
	}
}

func TestClassifyError_TestFailure(t *testing.T) {
	cases := []string{
		"--- FAIL: TestMyFunc (0.01s)",
		"FAIL\tgithub.com/foo/bar",
		"panic: test timed out after 30s",
	}
	for _, c := range cases {
		got := retryctx.ClassifyError(c)
		if got != retryctx.ErrorClassTest {
			t.Errorf("ClassifyError(%q) = %v; want %v", c, got, retryctx.ErrorClassTest)
		}
	}
}

func TestClassifyError_BuildError(t *testing.T) {
	cases := []string{
		"cannot find module providing package foo",
		"build constraints exclude all Go files",
		"syntax error: unexpected token",
	}
	for _, c := range cases {
		got := retryctx.ClassifyError(c)
		if got != retryctx.ErrorClassBuild {
			t.Errorf("ClassifyError(%q) = %v; want %v", c, got, retryctx.ErrorClassBuild)
		}
	}
}

func TestClassifyError_LintError(t *testing.T) {
	cases := []string{
		"go vet: suspicious usage",
		"should have comment on exported function Foo",
		"exported type Bar should have comment",
	}
	for _, c := range cases {
		got := retryctx.ClassifyError(c)
		if got != retryctx.ErrorClassLint {
			t.Errorf("ClassifyError(%q) = %v; want %v", c, got, retryctx.ErrorClassLint)
		}
	}
}

func TestClassifyError_Unknown(t *testing.T) {
	cases := []string{
		"",
		"everything is fine",
		"no recognisable pattern here",
	}
	for _, c := range cases {
		got := retryctx.ClassifyError(c)
		if got != retryctx.ErrorClassUnknown {
			t.Errorf("ClassifyError(%q) = %v; want %v", c, got, retryctx.ErrorClassUnknown)
		}
	}
}

// ---------------------------------------------------------------------------
// SuggestFixes tests
// ---------------------------------------------------------------------------

func TestSuggestFixes_AllClasses(t *testing.T) {
	classes := []retryctx.ErrorClass{
		retryctx.ErrorClassImport,
		retryctx.ErrorClassType,
		retryctx.ErrorClassTest,
		retryctx.ErrorClassBuild,
		retryctx.ErrorClassLint,
	}
	for _, class := range classes {
		fixes := retryctx.SuggestFixes(class)
		if len(fixes) == 0 {
			t.Errorf("SuggestFixes(%v) returned no suggestions", class)
		}
		for _, fix := range fixes {
			if fix == "" {
				t.Errorf("SuggestFixes(%v) returned an empty suggestion string", class)
			}
		}
	}

	// Unknown class should return an empty (but non-nil) slice.
	unknownFixes := retryctx.SuggestFixes(retryctx.ErrorClassUnknown)
	if unknownFixes == nil {
		t.Error("SuggestFixes(ErrorClassUnknown) returned nil; want empty slice")
	}
}

// ---------------------------------------------------------------------------
// BuildRetryContext tests
// ---------------------------------------------------------------------------

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

func TestBuildRetryContext_Complete(t *testing.T) {
	dir := t.TempDir()
	manifestPath := filepath.Join(dir, "impl.yaml")

	notes := "Build failed: cannot use x as type string in main.go"
	verification := "go build ./... exited with code 1"

	if err := os.WriteFile(manifestPath, []byte(minimalManifestYAML("A", notes, verification)), 0644); err != nil {
		t.Fatalf("failed to write test manifest: %v", err)
	}

	rc, err := retryctx.BuildRetryContext(manifestPath, "A", 2)
	if err != nil {
		t.Fatalf("BuildRetryContext() returned error: %v", err)
	}

	if rc.AgentID != "A" {
		t.Errorf("AgentID = %q; want %q", rc.AgentID, "A")
	}
	if rc.AttemptNumber != 2 {
		t.Errorf("AttemptNumber = %d; want 2", rc.AttemptNumber)
	}
	if rc.ErrorClass != retryctx.ErrorClassType {
		t.Errorf("ErrorClass = %v; want %v", rc.ErrorClass, retryctx.ErrorClassType)
	}
	if len(rc.SuggestedFixes) == 0 {
		t.Error("SuggestedFixes is empty; expected suggestions for type_error")
	}
	if rc.PriorNotes != notes {
		t.Errorf("PriorNotes = %q; want %q", rc.PriorNotes, notes)
	}
	if !strings.Contains(rc.PromptText, "## Retry Context (Attempt 2)") {
		t.Error("PromptText missing expected heading")
	}
	if !strings.Contains(rc.PromptText, "### Error Classification: type_error") {
		t.Error("PromptText missing error classification section")
	}
	if !strings.Contains(rc.PromptText, "### Suggested Fixes") {
		t.Error("PromptText missing suggested fixes section")
	}
	if !strings.Contains(rc.PromptText, "### Prior Attempt Notes") {
		t.Error("PromptText missing prior notes section")
	}

	// Excerpt must be at most 2000 chars.
	if len(rc.ErrorExcerpt) > 2000 {
		t.Errorf("ErrorExcerpt length %d exceeds 2000-char limit", len(rc.ErrorExcerpt))
	}
}

func TestBuildRetryContext_ExcerptTruncation(t *testing.T) {
	dir := t.TempDir()
	manifestPath := filepath.Join(dir, "impl.yaml")

	// Construct a notes string longer than 2000 characters.
	longNotes := strings.Repeat("error output line\n", 200) // well over 2000 chars

	if err := os.WriteFile(manifestPath, []byte(minimalManifestYAML("A", "", "")), 0644); err != nil {
		t.Fatalf("failed to write test manifest: %v", err)
	}

	// Re-write with long notes using direct YAML (avoid quoting issues).
	yaml := "title: \"T\"\nfeature_slug: \"f\"\nverdict: \"SUITABLE\"\ntest_command: \"go test\"\nlint_command: \"go vet\"\nfile_ownership: []\ninterface_contracts: []\nwaves:\n  - number: 1\n    agents:\n      - id: \"A\"\n        task: \"t\"\n        files: []\ncompletion_reports:\n  A:\n    status: \"partial\"\n    notes: |\n"
	for i := 0; i < 200; i++ {
		yaml += "      error output line\n"
	}

	if err := os.WriteFile(manifestPath, []byte(yaml), 0644); err != nil {
		t.Fatalf("failed to rewrite test manifest: %v", err)
	}

	rc, err := retryctx.BuildRetryContext(manifestPath, "A", 1)
	if err != nil {
		t.Fatalf("BuildRetryContext() returned error: %v", err)
	}

	_ = longNotes // used conceptually above
	if len(rc.ErrorExcerpt) > 2000 {
		t.Errorf("ErrorExcerpt length %d exceeds 2000-char limit", len(rc.ErrorExcerpt))
	}
}

func TestBuildRetryContext_FixableFailureType(t *testing.T) {
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

	rc, err := retryctx.BuildRetryContext(manifestPath, "A", 1)
	if err != nil {
		t.Fatalf("BuildRetryContext() returned error: %v", err)
	}

	// FailureType must be propagated from the completion report.
	if rc.FailureType != "fixable" {
		t.Errorf("FailureType = %q; want %q", rc.FailureType, "fixable")
	}

	// PromptText must contain a "Fix Required" section.
	if !strings.Contains(rc.PromptText, "## Fix Required") {
		t.Error("PromptText missing '## Fix Required' section for fixable failure")
	}

	// PromptText must include the agent's notes in the fix section.
	if !strings.Contains(rc.PromptText, notes) {
		t.Errorf("PromptText missing notes %q in Fix Required section", notes)
	}
}

func TestBuildRetryContext_MissingReport(t *testing.T) {
	dir := t.TempDir()
	manifestPath := filepath.Join(dir, "impl.yaml")

	// Manifest with no completion reports.
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

	_, err := retryctx.BuildRetryContext(manifestPath, "A", 1)
	if err == nil {
		t.Fatal("expected error for missing completion report, got nil")
	}
	if !strings.Contains(err.Error(), "no completion report found") {
		t.Errorf("error message %q does not mention missing report", err.Error())
	}
}
