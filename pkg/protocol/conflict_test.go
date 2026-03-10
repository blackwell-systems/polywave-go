package protocol

import (
	"testing"
)

func TestDetectOwnershipConflicts_NoConflicts(t *testing.T) {
	manifest := &IMPLManifest{
		FileOwnership: []FileOwnership{
			{File: "a.go", Agent: "A", Wave: 1},
			{File: "b.go", Agent: "B", Wave: 1},
		},
		Waves: []Wave{
			{
				Number: 1,
				Agents: []Agent{
					{ID: "A", Files: []string{"a.go"}},
					{ID: "B", Files: []string{"b.go"}},
				},
			},
		},
	}

	reports := map[string]CompletionReport{
		"A": {
			Status:       "complete",
			FilesChanged: []string{"a.go"},
		},
		"B": {
			Status:       "complete",
			FilesChanged: []string{"b.go"},
		},
	}

	conflicts := DetectOwnershipConflicts(manifest, reports)
	if len(conflicts) != 0 {
		t.Errorf("expected no conflicts, got %d: %v", len(conflicts), conflicts)
	}
}

func TestDetectOwnershipConflicts_SameFile(t *testing.T) {
	manifest := &IMPLManifest{
		FileOwnership: []FileOwnership{
			{File: "shared.go", Agent: "A", Wave: 1},
		},
		Waves: []Wave{
			{
				Number: 1,
				Agents: []Agent{
					{ID: "A", Files: []string{"shared.go"}},
					{ID: "B", Files: []string{"other.go"}},
				},
			},
		},
	}

	reports := map[string]CompletionReport{
		"A": {
			Status:       "complete",
			FilesChanged: []string{"shared.go"},
		},
		"B": {
			Status:       "complete",
			FilesChanged: []string{"shared.go"}, // Conflict!
		},
	}

	conflicts := DetectOwnershipConflicts(manifest, reports)
	if len(conflicts) == 0 {
		t.Fatal("expected conflicts, got none")
	}

	// Should have at least one conflict for shared.go
	found := false
	for _, c := range conflicts {
		if c.File == "shared.go" && c.WaveNumber == 1 && len(c.Agents) >= 2 {
			found = true
			break
		}
	}

	if !found {
		t.Errorf("expected conflict on shared.go with 2+ agents in wave 1, got: %v", conflicts)
	}
}

func TestDetectOwnershipConflicts_FilesCreatedOverlap(t *testing.T) {
	manifest := &IMPLManifest{
		FileOwnership: []FileOwnership{
			{File: "new.go", Agent: "A", Wave: 1, Action: "new"},
		},
		Waves: []Wave{
			{
				Number: 1,
				Agents: []Agent{
					{ID: "A", Files: []string{"new.go"}},
					{ID: "B", Files: []string{"other.go"}},
				},
			},
		},
	}

	reports := map[string]CompletionReport{
		"A": {
			Status:       "complete",
			FilesCreated: []string{"new.go"},
		},
		"B": {
			Status:       "complete",
			FilesCreated: []string{"new.go"}, // Both created same file!
		},
	}

	conflicts := DetectOwnershipConflicts(manifest, reports)
	if len(conflicts) == 0 {
		t.Fatal("expected conflicts, got none")
	}

	// Should have at least one conflict for new.go
	found := false
	for _, c := range conflicts {
		if c.File == "new.go" && c.WaveNumber == 1 && len(c.Agents) >= 2 {
			found = true
			break
		}
	}

	if !found {
		t.Errorf("expected conflict on new.go with 2+ agents in wave 1, got: %v", conflicts)
	}
}

func TestDetectOwnershipConflicts_MixedChangedCreated(t *testing.T) {
	manifest := &IMPLManifest{
		FileOwnership: []FileOwnership{
			{File: "file.go", Agent: "A", Wave: 1, Action: "new"},
		},
		Waves: []Wave{
			{
				Number: 1,
				Agents: []Agent{
					{ID: "A", Files: []string{"file.go"}},
					{ID: "B", Files: []string{"other.go"}},
				},
			},
		},
	}

	reports := map[string]CompletionReport{
		"A": {
			Status:       "complete",
			FilesCreated: []string{"file.go"},
		},
		"B": {
			Status:       "complete",
			FilesChanged: []string{"file.go"}, // Changed a file A created
		},
	}

	conflicts := DetectOwnershipConflicts(manifest, reports)
	if len(conflicts) == 0 {
		t.Fatal("expected conflicts, got none")
	}

	// Should have at least one conflict for file.go
	found := false
	for _, c := range conflicts {
		if c.File == "file.go" && c.WaveNumber == 1 && len(c.Agents) >= 2 {
			found = true
			break
		}
	}

	if !found {
		t.Errorf("expected conflict on file.go with 2+ agents in wave 1, got: %v", conflicts)
	}
}

func TestDetectOwnershipConflicts_DifferentWaves(t *testing.T) {
	manifest := &IMPLManifest{
		FileOwnership: []FileOwnership{
			{File: "evolving.go", Agent: "A", Wave: 1},
			{File: "evolving.go", Agent: "C", Wave: 2},
		},
		Waves: []Wave{
			{
				Number: 1,
				Agents: []Agent{
					{ID: "A", Files: []string{"evolving.go"}},
					{ID: "B", Files: []string{"other.go"}},
				},
			},
			{
				Number: 2,
				Agents: []Agent{
					{ID: "C", Files: []string{"evolving.go"}},
				},
			},
		},
	}

	reports := map[string]CompletionReport{
		"A": {
			Status:       "complete",
			FilesChanged: []string{"evolving.go"},
		},
		"C": {
			Status:       "complete",
			FilesChanged: []string{"evolving.go"},
		},
	}

	conflicts := DetectOwnershipConflicts(manifest, reports)
	// No conflict: agents are in different waves
	// Should only detect conflicts within the same wave
	for _, c := range conflicts {
		if c.File == "evolving.go" && len(c.Agents) > 1 {
			// Check that both agents are NOT in same wave
			agentA := false
			agentC := false
			for _, a := range c.Agents {
				if a == "A" {
					agentA = true
				}
				if a == "C" {
					agentC = true
				}
			}
			if agentA && agentC {
				t.Errorf("expected no conflict for evolving.go between different waves, but got: %v", c)
			}
		}
	}
}

func TestDetectOwnershipConflicts_UndeclaredFile(t *testing.T) {
	manifest := &IMPLManifest{
		FileOwnership: []FileOwnership{
			{File: "a.go", Agent: "A", Wave: 1},
		},
		Waves: []Wave{
			{
				Number: 1,
				Agents: []Agent{
					{ID: "A", Files: []string{"a.go"}},
					{ID: "B", Files: []string{"b.go"}},
				},
			},
		},
	}

	reports := map[string]CompletionReport{
		"A": {
			Status:       "complete",
			FilesChanged: []string{"a.go"},
		},
		"B": {
			Status:       "complete",
			FilesChanged: []string{"a.go"}, // B touched a.go but a.go is owned by A
		},
	}

	conflicts := DetectOwnershipConflicts(manifest, reports)
	if len(conflicts) == 0 {
		t.Fatal("expected conflicts for undeclared modification, got none")
	}

	// Should have conflicts for B touching A's file
	found := false
	for _, c := range conflicts {
		if c.File == "a.go" && c.WaveNumber == 1 {
			// Should include both A (declared) and B (violator)
			hasA := false
			hasB := false
			for _, agent := range c.Agents {
				if agent == "A" {
					hasA = true
				}
				if agent == "B" {
					hasB = true
				}
			}
			if hasA && hasB {
				found = true
				break
			}
		}
	}

	if !found {
		t.Errorf("expected conflict showing A (owner) and B (violator) for a.go, got: %v", conflicts)
	}
}

func TestDetectOwnershipConflicts_NilInputs(t *testing.T) {
	// Nil manifest
	conflicts := DetectOwnershipConflicts(nil, map[string]CompletionReport{})
	if conflicts != nil {
		t.Errorf("expected nil result for nil manifest, got %v", conflicts)
	}

	// Nil reports
	manifest := &IMPLManifest{}
	conflicts = DetectOwnershipConflicts(manifest, nil)
	if conflicts != nil {
		t.Errorf("expected nil result for nil reports, got %v", conflicts)
	}
}

func TestDetectOwnershipConflicts_MultipleAgentsOnSameFile(t *testing.T) {
	manifest := &IMPLManifest{
		FileOwnership: []FileOwnership{
			{File: "conflict.go", Agent: "A", Wave: 1},
		},
		Waves: []Wave{
			{
				Number: 1,
				Agents: []Agent{
					{ID: "A", Files: []string{"conflict.go"}},
					{ID: "B", Files: []string{"b.go"}},
					{ID: "C", Files: []string{"c.go"}},
				},
			},
		},
	}

	reports := map[string]CompletionReport{
		"A": {
			Status:       "complete",
			FilesChanged: []string{"conflict.go"},
		},
		"B": {
			Status:       "complete",
			FilesChanged: []string{"conflict.go"},
		},
		"C": {
			Status:       "complete",
			FilesChanged: []string{"conflict.go"},
		},
	}

	conflicts := DetectOwnershipConflicts(manifest, reports)
	if len(conflicts) == 0 {
		t.Fatal("expected conflicts, got none")
	}

	// Should have a conflict with all 3 agents
	found := false
	for _, c := range conflicts {
		if c.File == "conflict.go" && c.WaveNumber == 1 && len(c.Agents) >= 3 {
			found = true
			break
		}
	}

	if !found {
		t.Errorf("expected conflict on conflict.go with 3+ agents in wave 1, got: %v", conflicts)
	}
}

func TestDetectOwnershipConflicts_CrossRepoTracking(t *testing.T) {
	manifest := &IMPLManifest{
		FileOwnership: []FileOwnership{
			{File: "shared.go", Agent: "A", Wave: 1, Repo: "repo1"},
		},
		Waves: []Wave{
			{
				Number: 1,
				Agents: []Agent{
					{ID: "A", Files: []string{"shared.go"}},
					{ID: "B", Files: []string{"other.go"}},
				},
			},
		},
	}

	reports := map[string]CompletionReport{
		"A": {
			Status:       "complete",
			FilesChanged: []string{"shared.go"},
			Repo:         "repo1",
		},
		"B": {
			Status:       "complete",
			FilesChanged: []string{"shared.go"},
			Repo:         "repo1",
		},
	}

	conflicts := DetectOwnershipConflicts(manifest, reports)
	if len(conflicts) == 0 {
		t.Fatal("expected conflicts, got none")
	}

	// Should track repo in conflict
	found := false
	for _, c := range conflicts {
		if c.File == "shared.go" && c.Repo == "repo1" && c.WaveNumber == 1 {
			found = true
			break
		}
	}

	if !found {
		t.Errorf("expected conflict with repo tracking, got: %v", conflicts)
	}
}
