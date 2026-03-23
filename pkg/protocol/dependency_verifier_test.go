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

	res := VerifyDependenciesAvailable(manifest, 1)
	if !res.IsSuccess() {
		t.Fatalf("unexpected failure: %+v", res.Errors)
	}
	data := res.GetData()
	if !data.Valid {
		t.Errorf("expected Valid=true for wave 1 with no dependencies, got false")
	}
	if data.Wave != 1 {
		t.Errorf("expected Wave=1, got %d", data.Wave)
	}
	if len(data.Agents) != 2 {
		t.Fatalf("expected 2 agent checks, got %d", len(data.Agents))
	}
	for _, ac := range data.Agents {
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

	res := VerifyDependenciesAvailable(manifest, 2)
	if !res.IsSuccess() {
		t.Fatalf("unexpected failure: %+v", res.Errors)
	}
	data := res.GetData()
	if !data.Valid {
		t.Errorf("expected Valid=true when all deps completed")
	}
	if len(data.Agents) != 1 {
		t.Fatalf("expected 1 agent check, got %d", len(data.Agents))
	}
	ac := data.Agents[0]
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

	res := VerifyDependenciesAvailable(manifest, 2)
	if !res.IsSuccess() {
		t.Fatalf("unexpected failure: %+v", res.Errors)
	}
	data := res.GetData()
	if data.Valid {
		t.Errorf("expected Valid=false when dep B has no completion report")
	}
	if len(data.Agents) != 1 {
		t.Fatalf("expected 1 agent check, got %d", len(data.Agents))
	}
	ac := data.Agents[0]
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

	res := VerifyDependenciesAvailable(manifest, 2)
	if !res.IsSuccess() {
		t.Fatalf("unexpected failure: %+v", res.Errors)
	}
	data := res.GetData()
	if data.Valid {
		t.Errorf("expected Valid=false when dep A has status 'partial'")
	}
	ac := data.Agents[0]
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

	res := VerifyDependenciesAvailable(manifest, 99)
	if !res.IsFatal() {
		t.Fatal("expected FATAL result for missing wave")
	}
	if len(res.Errors) == 0 {
		t.Fatal("expected at least one error in FATAL result")
	}
}
