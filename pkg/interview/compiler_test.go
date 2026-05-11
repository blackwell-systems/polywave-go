package interview

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/blackwell-systems/polywave-go/pkg/result"
)

func TestCompileToRequirements_FullDoc(t *testing.T) {
	doc := &InterviewDoc{
		Slug: "test-project",
		SpecData: SpecData{
			Overview: OverviewSpec{
				Title: "Test Project",
				Goal:  "Build a CLI tool for testing",
			},
			Scope: ScopeSpec{
				InScope: []string{"Authentication", "Authorization", "Logging"},
			},
			Requirements: RequirementsSpec{
				Constraints:   []string{"Must deploy to Kubernetes", "Go 1.22+"},
				NonFunctional: []string{"99.9% uptime SLA"},
			},
			Interfaces: InterfacesSpec{
				DataModels: []string{"PostgreSQL for primary storage"},
				External:   []string{"GitHub API", "Slack webhook", "Go language SDK"},
			},
		},
	}

	r := CompileToRequirements(doc)
	if r.IsFatal() {
		t.Fatalf("unexpected error: %v", r.Errors)
	}
	got := *r.Data

	// Verify all sections are present
	requiredSections := []string{
		"# Requirements: Test Project",
		"## Language & Ecosystem",
		"## Project Type",
		"## Deployment Target",
		"## Key Concerns",
		"## Storage",
		"## External Integrations",
		"## Source Codebase",
		"## Architectural Decisions Already Made",
	}
	for _, section := range requiredSections {
		if !strings.Contains(got, section) {
			t.Errorf("missing section %q in output:\n%s", section, got)
		}
	}

	// Verify specific content rendered
	if !strings.Contains(got, "Build a CLI tool for testing") {
		t.Error("expected goal in Project Type section")
	}
	if !strings.Contains(got, "GitHub API") {
		t.Error("expected external integrations")
	}
	if !strings.Contains(got, "Must deploy to Kubernetes") {
		t.Error("expected constraints in Architectural Decisions")
	}
	if !strings.Contains(got, "99.9% uptime SLA") {
		t.Error("expected non-functional requirements in Architectural Decisions")
	}
	if !strings.Contains(got, "PostgreSQL for primary storage") {
		t.Error("expected data models in Storage section")
	}
}

func TestCompileToRequirements_EmptyFields(t *testing.T) {
	doc := &InterviewDoc{
		Slug: "empty-project",
		SpecData: SpecData{
			Overview: OverviewSpec{
				Title: "Empty Project",
			},
		},
	}

	r := CompileToRequirements(doc)
	if r.IsFatal() {
		t.Fatalf("unexpected error: %v", r.Errors)
	}
	got := *r.Data

	// Count placeholder occurrences — most sections should have placeholders
	placeholderCount := strings.Count(got, placeholder)
	if placeholderCount < 5 {
		t.Errorf("expected at least 5 placeholder comments for empty fields, got %d\n%s", placeholderCount, got)
	}

	// Title should still be rendered
	if !strings.Contains(got, "# Requirements: Empty Project") {
		t.Error("expected title even with empty fields")
	}
}

func TestCompileToRequirements_KeyConcernsFromScope(t *testing.T) {
	doc := &InterviewDoc{
		Slug: "scope-test",
		SpecData: SpecData{
			Overview: OverviewSpec{Title: "Scope Test"},
			Scope: ScopeSpec{
				InScope: []string{"User management", "API gateway", "Monitoring", "Alerting"},
			},
		},
	}

	r := CompileToRequirements(doc)
	if r.IsFatal() {
		t.Fatalf("unexpected error: %v", r.Errors)
	}
	got := *r.Data

	// Verify numbered list
	if !strings.Contains(got, "1. User management") {
		t.Error("expected numbered item 1")
	}
	if !strings.Contains(got, "2. API gateway") {
		t.Error("expected numbered item 2")
	}
	if !strings.Contains(got, "3. Monitoring") {
		t.Error("expected numbered item 3")
	}
	if !strings.Contains(got, "4. Alerting") {
		t.Error("expected numbered item 4")
	}

	// Should NOT have placeholder for Key Concerns
	keyConcernsIdx := strings.Index(got, "## Key Concerns")
	storageIdx := strings.Index(got, "## Storage")
	keyConcernsSection := got[keyConcernsIdx:storageIdx]
	if strings.Contains(keyConcernsSection, placeholder) {
		t.Error("Key Concerns should not have placeholder when InScope items exist")
	}
}

func TestCompileToRequirements_ArchDecisionsFromConstraints(t *testing.T) {
	doc := &InterviewDoc{
		Slug: "arch-test",
		SpecData: SpecData{
			Overview: OverviewSpec{Title: "Arch Test"},
			Requirements: RequirementsSpec{
				Constraints:   []string{"Must use gRPC", "No external databases"},
				NonFunctional: []string{"< 100ms p99 latency", "Horizontal scalability"},
			},
		},
	}

	r := CompileToRequirements(doc)
	if r.IsFatal() {
		t.Fatalf("unexpected error: %v", r.Errors)
	}
	got := *r.Data

	// Find the Arch Decisions section
	archIdx := strings.Index(got, "## Architectural Decisions Already Made")
	if archIdx == -1 {
		t.Fatal("missing Architectural Decisions section")
	}
	archSection := got[archIdx:]

	expectedItems := []string{
		"Must use gRPC",
		"No external databases",
		"< 100ms p99 latency",
		"Horizontal scalability",
	}
	for _, item := range expectedItems {
		if !strings.Contains(archSection, item) {
			t.Errorf("expected %q in Architectural Decisions section", item)
		}
	}
}

// TestCompileToRequirements_Warnings verifies that the Warnings section appears
// when the interview is truncated (QuestionCursor >= MaxQuestions) and required fields are missing.
func TestCompileToRequirements_Warnings(t *testing.T) {
	doc := &InterviewDoc{
		Slug:           "truncated",
		QuestionCursor: 18,
		MaxQuestions:   18,
		SpecData: SpecData{
			// Missing Title, Goal, InScope, and Functional — all required fields
		},
	}

	r := CompileToRequirements(doc)
	if r.IsFatal() {
		t.Fatalf("unexpected error: %v", r.Errors)
	}
	got := *r.Data

	if !strings.Contains(got, "## Warnings") {
		t.Error("expected '## Warnings' section when truncated with missing required fields")
	}
	if !strings.Contains(got, "truncated at max_questions limit") {
		t.Error("expected truncation warning message in output")
	}
}

// TestCompileToRequirements_NoWarnings_NotTruncated verifies no Warnings section
// when max_questions limit is NOT reached, even if required fields are missing.
func TestCompileToRequirements_NoWarnings_NotTruncated(t *testing.T) {
	doc := &InterviewDoc{
		Slug:           "not-truncated",
		QuestionCursor: 5,
		MaxQuestions:   18,
		SpecData: SpecData{
			// Missing required fields, but not truncated
		},
	}

	r := CompileToRequirements(doc)
	if r.IsFatal() {
		t.Fatalf("unexpected error: %v", r.Errors)
	}
	got := *r.Data

	if strings.Contains(got, "## Warnings") {
		t.Error("expected NO Warnings section when not truncated")
	}
}

// TestCompileToRequirements_NoWarnings_Complete verifies no Warnings section
// when all required fields are populated, even at max_questions.
func TestCompileToRequirements_NoWarnings_Complete(t *testing.T) {
	doc := &InterviewDoc{
		Slug:           "complete",
		QuestionCursor: 18,
		MaxQuestions:   18,
		SpecData: SpecData{
			Overview: OverviewSpec{
				Title: "My Project",
				Goal:  "Build something",
			},
			Scope: ScopeSpec{
				InScope: []string{"feature A"},
			},
			Requirements: RequirementsSpec{
				Functional: []string{"req1"},
			},
		},
	}

	r := CompileToRequirements(doc)
	if r.IsFatal() {
		t.Fatalf("unexpected error: %v", r.Errors)
	}
	got := *r.Data

	if strings.Contains(got, "## Warnings") {
		t.Error("expected NO Warnings section when required fields are all populated")
	}
}

func TestWriteRequirementsFile(t *testing.T) {
	doc := &InterviewDoc{
		Slug: "write-test",
		SpecData: SpecData{
			Overview: OverviewSpec{
				Title: "Write Test Project",
				Goal:  "Test file writing",
			},
			Scope: ScopeSpec{
				InScope: []string{"File I/O", "Template rendering"},
			},
		},
	}

	tmpDir := t.TempDir()
	outputPath := filepath.Join(tmpDir, "docs", "REQUIREMENTS.md")

	writeResult := WriteRequirementsFile(doc, outputPath)
	if writeResult.IsFatal() {
		t.Fatalf("unexpected failure: %v", writeResult.Errors)
	}
	if !writeResult.IsSuccess() {
		t.Fatalf("expected success result, got code: %s", writeResult.Code)
	}

	// Verify data fields are populated correctly.
	data := writeResult.GetData()
	if data.OutputPath != outputPath {
		t.Errorf("expected OutputPath %q, got %q", outputPath, data.OutputPath)
	}
	if data.LineCount <= 0 {
		t.Errorf("expected positive LineCount, got %d", data.LineCount)
	}

	// Read back and verify
	content, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("failed to read written file: %v", err)
	}

	fileContent := string(content)
	if !strings.Contains(fileContent, "# Requirements: Write Test Project") {
		t.Error("written file missing title")
	}
	if !strings.Contains(fileContent, "1. File I/O") {
		t.Error("written file missing Key Concerns")
	}

	// Verify it's the same as CompileToRequirements output
	compileR := CompileToRequirements(doc)
	if compileR.IsFatal() {
		t.Fatalf("CompileToRequirements failed: %v", compileR.Errors)
	}
	expected := *compileR.Data
	if fileContent != expected {
		t.Error("written file content does not match CompileToRequirements output")
	}
}

func TestWriteRequirementsFile_WriteFailure(t *testing.T) {
	doc := &InterviewDoc{
		Slug: "fail-test",
		SpecData: SpecData{
			Overview: OverviewSpec{Title: "Fail Test"},
		},
	}

	// Use a path where the directory cannot be created (file exists where dir should be).
	tmpDir := t.TempDir()
	// Create a file at the directory path so MkdirAll fails.
	blockingFile := filepath.Join(tmpDir, "blocked")
	if err := os.WriteFile(blockingFile, []byte("x"), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}
	outputPath := filepath.Join(blockingFile, "REQUIREMENTS.md")

	writeResult := WriteRequirementsFile(doc, outputPath)
	if !writeResult.IsFatal() {
		t.Fatalf("expected FATAL result for unwritable path, got code: %s", writeResult.Code)
	}
	if len(writeResult.Errors) == 0 {
		t.Fatal("expected at least one error in FATAL result")
	}
	if writeResult.Errors[0].Code != result.CodeRequirementsWriteFailed {
		t.Errorf("expected error code %s, got %q", result.CodeRequirementsWriteFailed, writeResult.Errors[0].Code)
	}
	if writeResult.Errors[0].Context["path"] != outputPath {
		t.Errorf("expected path context %q, got %q", outputPath, writeResult.Errors[0].Context["path"])
	}
}
