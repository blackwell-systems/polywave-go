package protocol

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// TestLoadProjectMemory_Valid tests roundtrip: save then load.
func TestLoadProjectMemory_Valid(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "CONTEXT.yaml")

	original := &ProjectMemory{
		Created:         "2026-03-09",
		ProtocolVersion: "0.1.0",
		Architecture: ArchitectureDescription{
			Language: "Go",
			Stack:    []string{"Vite", "React", "TypeScript"},
			Summary:  "Monorepo with Go backend and React frontend",
		},
		Decisions: []Decision{
			{
				Date:        "2026-03-09",
				Description: "Use YAML for project memory format",
				Rationale:   "Human-readable and machine-parseable",
			},
		},
		Conventions: Conventions{
			TestFramework: "go test",
			LintTool:      "golangci-lint",
			BuildTool:     "go build",
		},
		EstablishedInterfaces: []EstablishedInterface{
			{
				Name:       "IMPLManifest",
				FilePath:   "pkg/protocol/types.go",
				ImportPath: "github.com/blackwell-systems/scout-and-wave-go/pkg/protocol",
			},
		},
		FeaturesCompleted: []CompletedFeature{
			{
				Slug:      "protocol-parser",
				IMPLDoc:   "docs/IMPL/IMPL-protocol-parser.yaml",
				WaveCount: 3,
				AgentCount: 5,
				Date:      "2026-03-08",
			},
		},
	}

	// Save
	if err := SaveProjectMemory(path, original); err != nil {
		t.Fatalf("SaveProjectMemory failed: %v", err)
	}

	// Load
	loaded, err := LoadProjectMemory(path)
	if err != nil {
		t.Fatalf("LoadProjectMemory failed: %v", err)
	}

	// Verify all fields
	if loaded.Created != original.Created {
		t.Errorf("Created mismatch: got %q, want %q", loaded.Created, original.Created)
	}
	if loaded.ProtocolVersion != original.ProtocolVersion {
		t.Errorf("ProtocolVersion mismatch: got %q, want %q", loaded.ProtocolVersion, original.ProtocolVersion)
	}
	if loaded.Architecture.Language != original.Architecture.Language {
		t.Errorf("Architecture.Language mismatch: got %q, want %q", loaded.Architecture.Language, original.Architecture.Language)
	}
	if len(loaded.Architecture.Stack) != len(original.Architecture.Stack) {
		t.Errorf("Architecture.Stack length mismatch: got %d, want %d", len(loaded.Architecture.Stack), len(original.Architecture.Stack))
	}
	if len(loaded.Decisions) != len(original.Decisions) {
		t.Errorf("Decisions length mismatch: got %d, want %d", len(loaded.Decisions), len(original.Decisions))
	} else if len(loaded.Decisions) > 0 {
		if loaded.Decisions[0].Date != original.Decisions[0].Date {
			t.Errorf("Decisions[0].Date mismatch: got %q, want %q", loaded.Decisions[0].Date, original.Decisions[0].Date)
		}
	}
	if loaded.Conventions.TestFramework != original.Conventions.TestFramework {
		t.Errorf("Conventions.TestFramework mismatch: got %q, want %q", loaded.Conventions.TestFramework, original.Conventions.TestFramework)
	}
	if len(loaded.EstablishedInterfaces) != len(original.EstablishedInterfaces) {
		t.Errorf("EstablishedInterfaces length mismatch: got %d, want %d", len(loaded.EstablishedInterfaces), len(original.EstablishedInterfaces))
	}
	if len(loaded.FeaturesCompleted) != len(original.FeaturesCompleted) {
		t.Errorf("FeaturesCompleted length mismatch: got %d, want %d", len(loaded.FeaturesCompleted), len(original.FeaturesCompleted))
	} else if len(loaded.FeaturesCompleted) > 0 {
		if loaded.FeaturesCompleted[0].Slug != original.FeaturesCompleted[0].Slug {
			t.Errorf("FeaturesCompleted[0].Slug mismatch: got %q, want %q", loaded.FeaturesCompleted[0].Slug, original.FeaturesCompleted[0].Slug)
		}
	}
}

// TestLoadProjectMemory_NotFound tests that loading a missing file returns an error.
func TestLoadProjectMemory_NotFound(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nonexistent.yaml")

	_, err := LoadProjectMemory(path)
	if err == nil {
		t.Fatal("Expected error for missing file, got nil")
	}
	if !os.IsNotExist(err) && !isReadError(err) {
		t.Errorf("Expected os.IsNotExist or read error, got: %v", err)
	}
}

// isReadError checks if an error is a read error (wrapped with our message).
func isReadError(err error) bool {
	return err != nil && (os.IsNotExist(err) || err.Error() != "")
}

// TestSaveProjectMemory_Creates tests that SaveProjectMemory creates a file that doesn't exist.
func TestSaveProjectMemory_Creates(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "new-context.yaml")

	pm := &ProjectMemory{
		Created:         "2026-03-09",
		ProtocolVersion: "0.1.0",
	}

	if err := SaveProjectMemory(path, pm); err != nil {
		t.Fatalf("SaveProjectMemory failed: %v", err)
	}

	// Verify file was created
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("File was not created: %v", err)
	}

	// Verify permissions
	if info.Mode().Perm() != 0644 {
		t.Errorf("File permissions: got %o, want 0644", info.Mode().Perm())
	}
}

// TestSaveProjectMemory_Roundtrip tests that save + load preserves all fields.
func TestSaveProjectMemory_Roundtrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "roundtrip.yaml")

	original := &ProjectMemory{
		Created:         "2026-03-09",
		ProtocolVersion: "0.2.0",
		Architecture: ArchitectureDescription{
			Language: "TypeScript",
			Stack:    []string{"Node.js", "Express", "PostgreSQL"},
			Summary:  "REST API with database backend",
		},
		Decisions: []Decision{
			{
				Date:        "2026-03-01",
				Description: "Use PostgreSQL for persistence",
				Rationale:   "Strong consistency guarantees",
			},
			{
				Date:        "2026-03-05",
				Description: "Use Express framework",
				Rationale:   "Minimal and widely adopted",
			},
		},
		Conventions: Conventions{
			TestFramework: "jest",
			LintTool:      "eslint",
			BuildTool:     "tsc",
		},
		EstablishedInterfaces: []EstablishedInterface{
			{
				Name:       "UserService",
				FilePath:   "src/services/user.ts",
				ImportPath: "@/services/user",
			},
			{
				Name:       "AuthMiddleware",
				FilePath:   "src/middleware/auth.ts",
				ImportPath: "@/middleware/auth",
			},
		},
		FeaturesCompleted: []CompletedFeature{
			{
				Slug:      "user-auth",
				IMPLDoc:   "docs/IMPL/IMPL-user-auth.yaml",
				WaveCount: 2,
				AgentCount: 4,
				Date:      "2026-03-07",
			},
		},
	}

	// Save
	if err := SaveProjectMemory(path, original); err != nil {
		t.Fatalf("SaveProjectMemory failed: %v", err)
	}

	// Load
	loaded, err := LoadProjectMemory(path)
	if err != nil {
		t.Fatalf("LoadProjectMemory failed: %v", err)
	}

	// Deep verification
	if loaded.Created != original.Created {
		t.Errorf("Created: got %q, want %q", loaded.Created, original.Created)
	}
	if loaded.ProtocolVersion != original.ProtocolVersion {
		t.Errorf("ProtocolVersion: got %q, want %q", loaded.ProtocolVersion, original.ProtocolVersion)
	}

	// Architecture
	if loaded.Architecture.Language != original.Architecture.Language {
		t.Errorf("Architecture.Language: got %q, want %q", loaded.Architecture.Language, original.Architecture.Language)
	}
	if loaded.Architecture.Summary != original.Architecture.Summary {
		t.Errorf("Architecture.Summary: got %q, want %q", loaded.Architecture.Summary, original.Architecture.Summary)
	}
	if len(loaded.Architecture.Stack) != len(original.Architecture.Stack) {
		t.Fatalf("Architecture.Stack length: got %d, want %d", len(loaded.Architecture.Stack), len(original.Architecture.Stack))
	}
	for i := range original.Architecture.Stack {
		if loaded.Architecture.Stack[i] != original.Architecture.Stack[i] {
			t.Errorf("Architecture.Stack[%d]: got %q, want %q", i, loaded.Architecture.Stack[i], original.Architecture.Stack[i])
		}
	}

	// Decisions
	if len(loaded.Decisions) != len(original.Decisions) {
		t.Fatalf("Decisions length: got %d, want %d", len(loaded.Decisions), len(original.Decisions))
	}
	for i := range original.Decisions {
		if loaded.Decisions[i].Date != original.Decisions[i].Date {
			t.Errorf("Decisions[%d].Date: got %q, want %q", i, loaded.Decisions[i].Date, original.Decisions[i].Date)
		}
		if loaded.Decisions[i].Description != original.Decisions[i].Description {
			t.Errorf("Decisions[%d].Description: got %q, want %q", i, loaded.Decisions[i].Description, original.Decisions[i].Description)
		}
		if loaded.Decisions[i].Rationale != original.Decisions[i].Rationale {
			t.Errorf("Decisions[%d].Rationale: got %q, want %q", i, loaded.Decisions[i].Rationale, original.Decisions[i].Rationale)
		}
	}

	// Conventions
	if loaded.Conventions.TestFramework != original.Conventions.TestFramework {
		t.Errorf("Conventions.TestFramework: got %q, want %q", loaded.Conventions.TestFramework, original.Conventions.TestFramework)
	}
	if loaded.Conventions.LintTool != original.Conventions.LintTool {
		t.Errorf("Conventions.LintTool: got %q, want %q", loaded.Conventions.LintTool, original.Conventions.LintTool)
	}
	if loaded.Conventions.BuildTool != original.Conventions.BuildTool {
		t.Errorf("Conventions.BuildTool: got %q, want %q", loaded.Conventions.BuildTool, original.Conventions.BuildTool)
	}

	// EstablishedInterfaces
	if len(loaded.EstablishedInterfaces) != len(original.EstablishedInterfaces) {
		t.Fatalf("EstablishedInterfaces length: got %d, want %d", len(loaded.EstablishedInterfaces), len(original.EstablishedInterfaces))
	}
	for i := range original.EstablishedInterfaces {
		if loaded.EstablishedInterfaces[i].Name != original.EstablishedInterfaces[i].Name {
			t.Errorf("EstablishedInterfaces[%d].Name: got %q, want %q", i, loaded.EstablishedInterfaces[i].Name, original.EstablishedInterfaces[i].Name)
		}
		if loaded.EstablishedInterfaces[i].FilePath != original.EstablishedInterfaces[i].FilePath {
			t.Errorf("EstablishedInterfaces[%d].FilePath: got %q, want %q", i, loaded.EstablishedInterfaces[i].FilePath, original.EstablishedInterfaces[i].FilePath)
		}
		if loaded.EstablishedInterfaces[i].ImportPath != original.EstablishedInterfaces[i].ImportPath {
			t.Errorf("EstablishedInterfaces[%d].ImportPath: got %q, want %q", i, loaded.EstablishedInterfaces[i].ImportPath, original.EstablishedInterfaces[i].ImportPath)
		}
	}

	// FeaturesCompleted
	if len(loaded.FeaturesCompleted) != len(original.FeaturesCompleted) {
		t.Fatalf("FeaturesCompleted length: got %d, want %d", len(loaded.FeaturesCompleted), len(original.FeaturesCompleted))
	}
	for i := range original.FeaturesCompleted {
		if loaded.FeaturesCompleted[i].Slug != original.FeaturesCompleted[i].Slug {
			t.Errorf("FeaturesCompleted[%d].Slug: got %q, want %q", i, loaded.FeaturesCompleted[i].Slug, original.FeaturesCompleted[i].Slug)
		}
		if loaded.FeaturesCompleted[i].IMPLDoc != original.FeaturesCompleted[i].IMPLDoc {
			t.Errorf("FeaturesCompleted[%d].IMPLDoc: got %q, want %q", i, loaded.FeaturesCompleted[i].IMPLDoc, original.FeaturesCompleted[i].IMPLDoc)
		}
		if loaded.FeaturesCompleted[i].WaveCount != original.FeaturesCompleted[i].WaveCount {
			t.Errorf("FeaturesCompleted[%d].WaveCount: got %d, want %d", i, loaded.FeaturesCompleted[i].WaveCount, original.FeaturesCompleted[i].WaveCount)
		}
		if loaded.FeaturesCompleted[i].AgentCount != original.FeaturesCompleted[i].AgentCount {
			t.Errorf("FeaturesCompleted[%d].AgentCount: got %d, want %d", i, loaded.FeaturesCompleted[i].AgentCount, original.FeaturesCompleted[i].AgentCount)
		}
		if loaded.FeaturesCompleted[i].Date != original.FeaturesCompleted[i].Date {
			t.Errorf("FeaturesCompleted[%d].Date: got %q, want %q", i, loaded.FeaturesCompleted[i].Date, original.FeaturesCompleted[i].Date)
		}
	}
}

// TestAddCompletedFeature tests appending to existing list.
func TestAddCompletedFeature(t *testing.T) {
	pm := &ProjectMemory{
		Created:         "2026-03-09",
		ProtocolVersion: "0.1.0",
		FeaturesCompleted: []CompletedFeature{
			{
				Slug:      "feature-1",
				IMPLDoc:   "docs/IMPL/IMPL-feature-1.yaml",
				WaveCount: 1,
				AgentCount: 2,
				Date:      "2026-03-08",
			},
		},
	}

	newFeature := CompletedFeature{
		Slug:      "feature-2",
		IMPLDoc:   "docs/IMPL/IMPL-feature-2.yaml",
		WaveCount: 2,
		AgentCount: 3,
		Date:      "2026-03-09",
	}

	AddCompletedFeature(pm, newFeature)

	if len(pm.FeaturesCompleted) != 2 {
		t.Fatalf("Expected 2 features, got %d", len(pm.FeaturesCompleted))
	}

	if pm.FeaturesCompleted[1].Slug != "feature-2" {
		t.Errorf("Added feature slug: got %q, want %q", pm.FeaturesCompleted[1].Slug, "feature-2")
	}
	if pm.FeaturesCompleted[1].WaveCount != 2 {
		t.Errorf("Added feature wave count: got %d, want %d", pm.FeaturesCompleted[1].WaveCount, 2)
	}
}

// TestAddCompletedFeature_Empty tests appending to nil list.
func TestAddCompletedFeature_Empty(t *testing.T) {
	pm := &ProjectMemory{
		Created:         "2026-03-09",
		ProtocolVersion: "0.1.0",
		FeaturesCompleted: nil,
	}

	newFeature := CompletedFeature{
		Slug:      "first-feature",
		IMPLDoc:   "docs/IMPL/IMPL-first.md",
		WaveCount: 1,
		AgentCount: 1,
		Date:      "2026-03-09",
	}

	AddCompletedFeature(pm, newFeature)

	if len(pm.FeaturesCompleted) != 1 {
		t.Fatalf("Expected 1 feature, got %d", len(pm.FeaturesCompleted))
	}

	if pm.FeaturesCompleted[0].Slug != "first-feature" {
		t.Errorf("Added feature slug: got %q, want %q", pm.FeaturesCompleted[0].Slug, "first-feature")
	}
}

// TestArchitectureModule_Roundtrip tests that ArchitectureModule round-trips through YAML.
func TestArchitectureModule_Roundtrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "context.yaml")

	original := &ProjectMemory{
		Created:         "2026-03-21",
		ProtocolVersion: "0.1.0",
		Architecture: ArchitectureDescription{
			Language: "Go",
			Summary:  "Multi-module Go project",
			Description: "Detailed description of the architecture",
			Modules: []ArchitectureModule{
				{
					Name:           "pkg/protocol",
					Path:           "pkg/protocol",
					Responsibility: "Core protocol types and validation",
				},
				{
					Name:           "pkg/engine",
					Path:           "pkg/engine",
					Responsibility: "Orchestration engine",
				},
			},
		},
	}

	if err := SaveProjectMemory(path, original); err != nil {
		t.Fatalf("SaveProjectMemory failed: %v", err)
	}

	loaded, err := LoadProjectMemory(path)
	if err != nil {
		t.Fatalf("LoadProjectMemory failed: %v", err)
	}

	if loaded.Architecture.Description != original.Architecture.Description {
		t.Errorf("Architecture.Description: got %q, want %q", loaded.Architecture.Description, original.Architecture.Description)
	}

	if len(loaded.Architecture.Modules) != len(original.Architecture.Modules) {
		t.Fatalf("Architecture.Modules length: got %d, want %d", len(loaded.Architecture.Modules), len(original.Architecture.Modules))
	}

	for i, mod := range original.Architecture.Modules {
		got := loaded.Architecture.Modules[i]
		if got.Name != mod.Name {
			t.Errorf("Modules[%d].Name: got %q, want %q", i, got.Name, mod.Name)
		}
		if got.Path != mod.Path {
			t.Errorf("Modules[%d].Path: got %q, want %q", i, got.Path, mod.Path)
		}
		if got.Responsibility != mod.Responsibility {
			t.Errorf("Modules[%d].Responsibility: got %q, want %q", i, got.Responsibility, mod.Responsibility)
		}
	}
}

// TestKnownIssue_ValidationRejectsEmptyTitle tests that KnownIssue entries without a title fail validation.
func TestKnownIssue_ValidationRejectsEmptyTitle(t *testing.T) {
	m := makeMinimalManifest()
	m.KnownIssues = []KnownIssue{
		{Title: "", Description: "Some issue without a title"},
	}

	errs := validateKnownIssueTitles(m)
	if len(errs) != 1 {
		t.Fatalf("Expected 1 error for empty title, got %d: %v", len(errs), errs)
	}
	if errs[0].Code != "KNOWN_ISSUE_MISSING_TITLE" {
		t.Errorf("Expected KNOWN_ISSUE_MISSING_TITLE, got %s", errs[0].Code)
	}
	if !strings.Contains(errs[0].Field, "known_issues[0]") {
		t.Errorf("Expected field to contain 'known_issues[0]', got %s", errs[0].Field)
	}
}

// TestKnownIssue_ValidationAcceptsTitle tests that KnownIssue entries with a title pass validation.
func TestKnownIssue_ValidationAcceptsTitle(t *testing.T) {
	m := makeMinimalManifest()
	m.KnownIssues = []KnownIssue{
		{Title: "Known race condition", Description: "Under high load a race may occur"},
	}

	errs := validateKnownIssueTitles(m)
	if len(errs) != 0 {
		t.Errorf("Expected no errors for valid KnownIssue, got %d: %v", len(errs), errs)
	}
}

// TestKnownIssue_MultipleEmptyTitles tests that multiple missing titles are all reported.
func TestKnownIssue_MultipleEmptyTitles(t *testing.T) {
	m := makeMinimalManifest()
	m.KnownIssues = []KnownIssue{
		{Title: "Has a title", Description: "OK"},
		{Title: "", Description: "Missing title"},
		{Title: "", Description: "Also missing title"},
	}

	errs := validateKnownIssueTitles(m)
	if len(errs) != 2 {
		t.Fatalf("Expected 2 errors for 2 missing titles, got %d: %v", len(errs), errs)
	}
}

// TestSuitabilityReasoning_ParsesCorrectly tests that SuitabilityReasoning field parses from YAML.
func TestSuitabilityReasoning_ParsesCorrectly(t *testing.T) {
	yamlData := `
title: Test IMPL
feature_slug: test-feature
verdict: SUITABLE
suitability_assessment: This is a suitable feature.
suitability_reasoning: The codebase is well-structured and ready for this change.
test_command: go test ./...
lint_command: go vet ./...
file_ownership: []
interface_contracts: []
waves: []
`
	var m IMPLManifest
	if err := yaml.Unmarshal([]byte(yamlData), &m); err != nil {
		t.Fatalf("Failed to unmarshal YAML: %v", err)
	}

	if m.SuitabilityReasoning != "The codebase is well-structured and ready for this change." {
		t.Errorf("SuitabilityReasoning: got %q, want expected value", m.SuitabilityReasoning)
	}
	if m.SuitabilityAssessment != "This is a suitable feature." {
		t.Errorf("SuitabilityAssessment: got %q, want expected value", m.SuitabilityAssessment)
	}
}

// makeMinimalManifest returns a minimal valid IMPLManifest for testing.
func makeMinimalManifest() *IMPLManifest {
	return &IMPLManifest{
		Title:        "Test",
		FeatureSlug:  "test-feature",
		Verdict:      "SUITABLE",
		TestCommand:  "go test ./...",
		LintCommand:  "go vet ./...",
	}
}
