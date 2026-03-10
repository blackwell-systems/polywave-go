package protocol

import (
	"errors"
	"testing"
)

// minimalManifest returns a minimal IMPLManifest with two agents (A and B)
// used by the YAML-mode extract tests.
func minimalManifest() *IMPLManifest {
	return &IMPLManifest{
		Title:       "Test Feature",
		FeatureSlug: "test-feature",
		FileOwnership: []FileOwnership{
			{File: "pkg/foo/foo.go", Agent: "A", Wave: 1, Action: "new"},
			{File: "pkg/bar/bar.go", Agent: "B", Wave: 1, Action: "new"},
		},
		InterfaceContracts: []InterfaceContract{
			{Name: "FooInterface", Definition: "type Foo interface { Bar() string }", Location: "pkg/foo/foo.go"},
			{Name: "BarInterface", Definition: "type Bar interface { Baz() int }", Location: "pkg/bar/bar.go"},
		},
		Scaffolds: []ScaffoldFile{
			{FilePath: "pkg/types/scaffold.go", Contents: "package types", Status: "committed"},
		},
		QualityGates: &QualityGates{
			Level: "standard",
			Gates: []QualityGate{
				{Type: "build", Command: "go build ./...", Required: true, Description: "must compile"},
			},
		},
		Waves: []Wave{
			{
				Number: 1,
				Agents: []Agent{
					{ID: "A", Task: "Implement Foo interface in pkg/foo"},
					{ID: "B", Task: "Implement Bar interface in pkg/bar"},
				},
			},
		},
	}
}

// TestExtractAgentContextFromManifest_Found verifies that a known agent's context
// is extracted correctly: AgentTask, FileOwnership (only agent A's), all
// InterfaceContracts, and QualityGates.
func TestExtractAgentContextFromManifest_Found(t *testing.T) {
	m := minimalManifest()

	payload, err := ExtractAgentContextFromManifest(m, "A")
	if err != nil {
		t.Fatalf("ExtractAgentContextFromManifest returned unexpected error: %v", err)
	}
	if payload == nil {
		t.Fatal("ExtractAgentContextFromManifest returned nil payload")
	}

	// AgentID must match.
	if payload.AgentID != "A" {
		t.Errorf("AgentID = %q, want %q", payload.AgentID, "A")
	}

	// AgentTask must match the task defined for agent A.
	if payload.AgentTask != "Implement Foo interface in pkg/foo" {
		t.Errorf("AgentTask = %q, want %q", payload.AgentTask, "Implement Foo interface in pkg/foo")
	}

	// FileOwnership must contain only agent A's files.
	if len(payload.FileOwnership) != 1 {
		t.Errorf("FileOwnership len = %d, want 1", len(payload.FileOwnership))
	} else if payload.FileOwnership[0].File != "pkg/foo/foo.go" {
		t.Errorf("FileOwnership[0].File = %q, want %q", payload.FileOwnership[0].File, "pkg/foo/foo.go")
	}

	// InterfaceContracts must contain all contracts (not filtered by agent).
	if len(payload.InterfaceContracts) != 2 {
		t.Errorf("InterfaceContracts len = %d, want 2", len(payload.InterfaceContracts))
	}

	// QualityGates must be set and match manifest.
	if payload.QualityGates == nil {
		t.Fatal("QualityGates is nil, want non-nil")
	}
	if payload.QualityGates.Level != "standard" {
		t.Errorf("QualityGates.Level = %q, want %q", payload.QualityGates.Level, "standard")
	}
}

// TestExtractAgentContextFromManifest_NotFound verifies that ErrAgentNotFound is
// returned (wrapped) when the requested agent ID does not exist in any wave.
func TestExtractAgentContextFromManifest_NotFound(t *testing.T) {
	m := minimalManifest()

	payload, err := ExtractAgentContextFromManifest(m, "Z")
	if err == nil {
		t.Fatalf("expected error for missing agent Z, got payload: %+v", payload)
	}
	if !errors.Is(err, ErrAgentNotFound) {
		t.Errorf("expected ErrAgentNotFound, got: %v", err)
	}
}

// TestExtractAgentContextFromManifest_FiltersByAgent verifies that agent B's files
// are not included in agent A's context payload.
func TestExtractAgentContextFromManifest_FiltersByAgent(t *testing.T) {
	m := minimalManifest()

	payload, err := ExtractAgentContextFromManifest(m, "A")
	if err != nil {
		t.Fatalf("ExtractAgentContextFromManifest returned unexpected error: %v", err)
	}

	// Verify agent B's file is not present.
	for _, fo := range payload.FileOwnership {
		if fo.File == "pkg/bar/bar.go" {
			t.Errorf("FileOwnership contains agent B's file %q in agent A's context", fo.File)
		}
		if fo.Agent == "B" {
			t.Errorf("FileOwnership contains entry for agent B in agent A's context")
		}
	}

	// Agent B payload should include B's file and not A's.
	payloadB, err := ExtractAgentContextFromManifest(m, "B")
	if err != nil {
		t.Fatalf("ExtractAgentContextFromManifest(B) returned unexpected error: %v", err)
	}
	if len(payloadB.FileOwnership) != 1 {
		t.Errorf("Agent B FileOwnership len = %d, want 1", len(payloadB.FileOwnership))
	} else if payloadB.FileOwnership[0].File != "pkg/bar/bar.go" {
		t.Errorf("Agent B FileOwnership[0].File = %q, want %q", payloadB.FileOwnership[0].File, "pkg/bar/bar.go")
	}
	for _, fo := range payloadB.FileOwnership {
		if fo.Agent == "A" {
			t.Errorf("FileOwnership contains agent A's entry in agent B's context")
		}
	}
}
