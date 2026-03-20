package interview

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
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

	result, err := CompileToRequirements(doc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

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
		if !strings.Contains(result, section) {
			t.Errorf("missing section %q in output:\n%s", section, result)
		}
	}

	// Verify specific content rendered
	if !strings.Contains(result, "Build a CLI tool for testing") {
		t.Error("expected goal in Project Type section")
	}
	if !strings.Contains(result, "GitHub API") {
		t.Error("expected external integrations")
	}
	if !strings.Contains(result, "Must deploy to Kubernetes") {
		t.Error("expected constraints in Architectural Decisions")
	}
	if !strings.Contains(result, "99.9% uptime SLA") {
		t.Error("expected non-functional requirements in Architectural Decisions")
	}
	if !strings.Contains(result, "PostgreSQL for primary storage") {
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

	result, err := CompileToRequirements(doc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Count placeholder occurrences — most sections should have placeholders
	placeholderCount := strings.Count(result, placeholder)
	if placeholderCount < 5 {
		t.Errorf("expected at least 5 placeholder comments for empty fields, got %d\n%s", placeholderCount, result)
	}

	// Title should still be rendered
	if !strings.Contains(result, "# Requirements: Empty Project") {
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

	result, err := CompileToRequirements(doc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify numbered list
	if !strings.Contains(result, "1. User management") {
		t.Error("expected numbered item 1")
	}
	if !strings.Contains(result, "2. API gateway") {
		t.Error("expected numbered item 2")
	}
	if !strings.Contains(result, "3. Monitoring") {
		t.Error("expected numbered item 3")
	}
	if !strings.Contains(result, "4. Alerting") {
		t.Error("expected numbered item 4")
	}

	// Should NOT have placeholder for Key Concerns
	keyConcernsIdx := strings.Index(result, "## Key Concerns")
	storageIdx := strings.Index(result, "## Storage")
	keyConcernsSection := result[keyConcernsIdx:storageIdx]
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

	result, err := CompileToRequirements(doc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Find the Arch Decisions section
	archIdx := strings.Index(result, "## Architectural Decisions Already Made")
	if archIdx == -1 {
		t.Fatal("missing Architectural Decisions section")
	}
	archSection := result[archIdx:]

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

	err := WriteRequirementsFile(doc, outputPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Read back and verify
	content, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("failed to read written file: %v", err)
	}

	result := string(content)
	if !strings.Contains(result, "# Requirements: Write Test Project") {
		t.Error("written file missing title")
	}
	if !strings.Contains(result, "1. File I/O") {
		t.Error("written file missing Key Concerns")
	}

	// Verify it's the same as CompileToRequirements output
	expected, _ := CompileToRequirements(doc)
	if result != expected {
		t.Error("written file content does not match CompileToRequirements output")
	}
}
