package protocol

import (
	"testing"
)

func TestVerifyDependencies_Wave1NoDepsTriviallyValid(t *testing.T) {
	manifest := &IMPLManifest{
		FeatureSlug: "test-feature",
		Waves: []Wave{
			{
				Number: 1,
				Agents: []Agent{
					{ID: "A", Task: "task A", Files: []string{"a.go"}},
					{ID: "B", Task: "task B", Files: []string{"b.go"}},
				},
			},
		},
		CompletionReports: map[string]CompletionReport{},
	}

	result, err := VerifyDependenciesAvailable(manifest, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Valid {
		t.Errorf("expected Valid=true for wave 1 with no dependencies, got false")
	}
	if result.Wave != 1 {
		t.Errorf("expected Wave=1, got %d", result.Wave)
	}
	if len(result.Agents) != 2 {
		t.Fatalf("expected 2 agent checks, got %d", len(result.Agents))
	}
	for _, ac := range result.Agents {
		if !ac.Available {
			t.Errorf("agent %s: expected Available=true, got false", ac.Agent)
		}
		if len(ac.Missing) != 0 {
			t.Errorf("agent %s: expected no missing deps, got %v", ac.Agent, ac.Missing)
		}
	}
}

func TestVerifyDependencies_Wave2AllDepsCompleted(t *testing.T) {
	manifest := &IMPLManifest{
		FeatureSlug: "test-feature",
		Waves: []Wave{
			{Number: 1, Agents: []Agent{{ID: "A"}, {ID: "B"}}},
			{
				Number: 2,
				Agents: []Agent{
					{ID: "C", Task: "task C", Files: []string{"c.go"}, Dependencies: []string{"A", "B"}},
				},
			},
		},
		CompletionReports: map[string]CompletionReport{
			"A": {Status: "complete"},
			"B": {Status: "complete"},
		},
	}

	result, err := VerifyDependenciesAvailable(manifest, 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Valid {
		t.Errorf("expected Valid=true when all deps completed")
	}
	if len(result.Agents) != 1 {
		t.Fatalf("expected 1 agent check, got %d", len(result.Agents))
	}
	ac := result.Agents[0]
	if !ac.Available {
		t.Errorf("expected agent C Available=true")
	}
	if len(ac.Missing) != 0 {
		t.Errorf("expected no missing deps, got %v", ac.Missing)
	}
}

func TestVerifyDependencies_Wave2MissingCompletionReport(t *testing.T) {
	manifest := &IMPLManifest{
		FeatureSlug: "test-feature",
		Waves: []Wave{
			{Number: 1, Agents: []Agent{{ID: "A"}, {ID: "B"}}},
			{
				Number: 2,
				Agents: []Agent{
					{ID: "C", Task: "task C", Files: []string{"c.go"}, Dependencies: []string{"A", "B"}},
				},
			},
		},
		// Only A has completed; B has no report at all
		CompletionReports: map[string]CompletionReport{
			"A": {Status: "complete"},
		},
	}

	result, err := VerifyDependenciesAvailable(manifest, 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Valid {
		t.Errorf("expected Valid=false when dep B has no completion report")
	}
	if len(result.Agents) != 1 {
		t.Fatalf("expected 1 agent check, got %d", len(result.Agents))
	}
	ac := result.Agents[0]
	if ac.Available {
		t.Errorf("expected agent C Available=false")
	}
	if len(ac.Missing) != 1 || ac.Missing[0] != "B" {
		t.Errorf("expected Missing=[B], got %v", ac.Missing)
	}
}

func TestVerifyDependencies_Wave2DepStatusPartial(t *testing.T) {
	manifest := &IMPLManifest{
		FeatureSlug: "test-feature",
		Waves: []Wave{
			{Number: 1, Agents: []Agent{{ID: "A"}}},
			{
				Number: 2,
				Agents: []Agent{
					{ID: "B", Task: "task B", Files: []string{"b.go"}, Dependencies: []string{"A"}},
				},
			},
		},
		CompletionReports: map[string]CompletionReport{
			"A": {Status: "partial"}, // Not "complete"
		},
	}

	result, err := VerifyDependenciesAvailable(manifest, 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Valid {
		t.Errorf("expected Valid=false when dep A has status 'partial'")
	}
	ac := result.Agents[0]
	if ac.Available {
		t.Errorf("expected agent B Available=false")
	}
	if len(ac.Missing) != 1 || ac.Missing[0] != "A" {
		t.Errorf("expected Missing=[A], got %v", ac.Missing)
	}
}

func TestVerifyDependencies_WaveNotFound(t *testing.T) {
	manifest := &IMPLManifest{
		FeatureSlug: "test-feature",
		Waves: []Wave{
			{Number: 1, Agents: []Agent{{ID: "A"}}},
		},
	}

	_, err := VerifyDependenciesAvailable(manifest, 99)
	if err == nil {
		t.Fatal("expected error for missing wave, got nil")
	}
}
