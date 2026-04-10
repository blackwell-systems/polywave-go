package protocol

import (
	"testing"
)

// makeMinimalManifest builds a minimal IMPLManifest for stale-constraint tests.
// agents is a list of (agentID, waveNum, ownedFiles..., taskText) but we use
// explicit parameters for clarity.
func makeManifestForStaleTest(ownership []FileOwnership, waves []Wave) *IMPLManifest {
	return &IMPLManifest{
		FileOwnership: ownership,
		Waves:         waves,
	}
}

// TestDetectStaleConstraints_StaleDetected: Agent B's task mentions a file owned by A.
func TestDetectStaleConstraints_StaleDetected(t *testing.T) {
	m := makeManifestForStaleTest(
		[]FileOwnership{
			{File: "pkg/engine/manager.go", Agent: "A", Wave: 1},
		},
		[]Wave{
			{Number: 1, Agents: []Agent{
				{ID: "A", Task: "Implement the manager.", Files: []string{"pkg/engine/manager.go"}},
				{ID: "B", Task: "In manager.go, ONLY edit the LanguageIDFromPath switch.", Files: []string{"pkg/engine/other.go"}},
			}},
		},
	)

	warnings := DetectStaleConstraints(m)
	if len(warnings) != 1 {
		t.Fatalf("expected 1 warning, got %d: %+v", len(warnings), warnings)
	}
	w := warnings[0]
	if w.AgentID != "B" {
		t.Errorf("expected AgentID=B, got %q", w.AgentID)
	}
	if w.MentionedFile != "manager.go" {
		t.Errorf("expected MentionedFile=manager.go, got %q", w.MentionedFile)
	}
	if w.OwnedByAgent != "A" {
		t.Errorf("expected OwnedByAgent=A, got %q", w.OwnedByAgent)
	}
}

// TestDetectStaleConstraints_UnownedFile: Agent B mentions README.md which nobody owns — no warning.
func TestDetectStaleConstraints_UnownedFile(t *testing.T) {
	m := makeManifestForStaleTest(
		[]FileOwnership{
			{File: "pkg/engine/manager.go", Agent: "A", Wave: 1},
		},
		[]Wave{
			{Number: 1, Agents: []Agent{
				{ID: "A", Task: "Implement the manager.", Files: []string{"pkg/engine/manager.go"}},
				{ID: "B", Task: "See README.md for context.", Files: []string{"pkg/engine/other.go"}},
			}},
		},
	)

	warnings := DetectStaleConstraints(m)
	if len(warnings) != 0 {
		t.Errorf("expected 0 warnings for unowned file, got %d: %+v", len(warnings), warnings)
	}
}

// TestDetectStaleConstraints_OwnFileReference: Agent A mentions its own file — no warning.
func TestDetectStaleConstraints_OwnFileReference(t *testing.T) {
	m := makeManifestForStaleTest(
		[]FileOwnership{
			{File: "pkg/engine/manager.go", Agent: "A", Wave: 1},
		},
		[]Wave{
			{Number: 1, Agents: []Agent{
				{ID: "A", Task: "Edit manager.go to add the new switch case.", Files: []string{"pkg/engine/manager.go"}},
			}},
		},
	)

	warnings := DetectStaleConstraints(m)
	if len(warnings) != 0 {
		t.Errorf("expected 0 warnings when agent references own file, got %d: %+v", len(warnings), warnings)
	}
}

// TestDetectStaleConstraints_CrossWaveStaleReference: Agent B (wave 2) mentions A's file (wave 1) — warning.
func TestDetectStaleConstraints_CrossWaveStaleReference(t *testing.T) {
	m := makeManifestForStaleTest(
		[]FileOwnership{
			{File: "cmd/sawtools/main.go", Agent: "A", Wave: 1},
		},
		[]Wave{
			{Number: 1, Agents: []Agent{
				{ID: "A", Task: "Add the new subcommand in cmd/sawtools/main.go.", Files: []string{"cmd/sawtools/main.go"}},
			}},
			{Number: 2, Agents: []Agent{
				{ID: "B", Task: "Wire the new flag into cmd/sawtools/main.go.", Files: []string{"pkg/engine/wiring.go"}},
			}},
		},
	)

	warnings := DetectStaleConstraints(m)
	if len(warnings) != 1 {
		t.Fatalf("expected 1 warning for cross-wave stale reference, got %d: %+v", len(warnings), warnings)
	}
	w := warnings[0]
	if w.AgentID != "B" {
		t.Errorf("expected AgentID=B, got %q", w.AgentID)
	}
	if w.MentionedFile != "cmd/sawtools/main.go" {
		t.Errorf("expected MentionedFile=cmd/sawtools/main.go, got %q", w.MentionedFile)
	}
	if w.OwnedByAgent != "A" {
		t.Errorf("expected OwnedByAgent=A, got %q", w.OwnedByAgent)
	}
}

// TestDetectStaleConstraints_NilManifest: nil manifest returns nil without panic.
func TestDetectStaleConstraints_NilManifest(t *testing.T) {
	var warnings []StaleConstraintWarning
	// Must not panic.
	warnings = DetectStaleConstraints(nil)
	if warnings != nil {
		t.Errorf("expected nil for nil manifest, got %+v", warnings)
	}
}

// TestDetectStaleConstraints_FullPathMatch: Agent B mentions the full path — warning.
func TestDetectStaleConstraints_FullPathMatch(t *testing.T) {
	m := makeManifestForStaleTest(
		[]FileOwnership{
			{File: "pkg/engine/manager.go", Agent: "A", Wave: 1},
		},
		[]Wave{
			{Number: 1, Agents: []Agent{
				{ID: "A", Task: "Implement manager.", Files: []string{"pkg/engine/manager.go"}},
				{ID: "B", Task: "Do NOT edit pkg/engine/manager.go.", Files: []string{"pkg/engine/other.go"}},
			}},
		},
	)

	warnings := DetectStaleConstraints(m)
	if len(warnings) != 1 {
		t.Fatalf("expected 1 warning for full path match, got %d: %+v", len(warnings), warnings)
	}
	w := warnings[0]
	if w.AgentID != "B" {
		t.Errorf("expected AgentID=B, got %q", w.AgentID)
	}
	if w.MentionedFile != "pkg/engine/manager.go" {
		t.Errorf("expected MentionedFile=pkg/engine/manager.go, got %q", w.MentionedFile)
	}
	if w.OwnedByAgent != "A" {
		t.Errorf("expected OwnedByAgent=A, got %q", w.OwnedByAgent)
	}
}
