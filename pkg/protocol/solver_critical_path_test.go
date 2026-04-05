package protocol

import (
	"testing"
)

func TestValidateWithSolver_OverSequenced(t *testing.T) {
	// A->B->C is a linear chain: critical path length = 3, minimum waves = 3.
	// Putting them in 3 waves is optimal — should NOT warn.
	m := buildManifest("Test", "test", "SUITABLE",
		[]Wave{
			makeWave(1, agentSpec{"A", nil, []string{"a.go"}}),
			makeWave(2, agentSpec{"B", []string{"A"}, []string{"b.go"}}),
			makeWave(3, agentSpec{"C", []string{"B"}, []string{"c.go"}}),
		},
		[]FileOwnership{
			makeFO("a.go", "A", 1),
			makeFO("b.go", "B", 2, "A"),
			makeFO("c.go", "C", 3, "B"),
		},
	)
	errs := ValidateWithSolver(m)
	for _, e := range errs {
		if e.Code == "SOLVER_OVER_SEQUENCED" {
			t.Errorf("unexpected SOLVER_OVER_SEQUENCED for optimal manifest: %+v", e)
		}
	}
}

func TestValidateWithSolver_OverSequencedWarns(t *testing.T) {
	// A and B are independent (no deps). Critical path = 1 agent = 1 minimum wave.
	// Putting them in 2 separate waves is over-sequenced.
	m := buildManifest("Test", "test", "SUITABLE",
		[]Wave{
			makeWave(1, agentSpec{"A", nil, []string{"a.go"}}),
			makeWave(2, agentSpec{"B", nil, []string{"b.go"}}),
		},
		[]FileOwnership{
			makeFO("a.go", "A", 1),
			makeFO("b.go", "B", 2),
		},
	)
	errs := ValidateWithSolver(m)
	found := false
	for _, e := range errs {
		if e.Code == "SOLVER_OVER_SEQUENCED" {
			found = true
			if e.Severity != "warning" {
				t.Errorf("expected severity=warning, got %s", e.Severity)
			}
		}
	}
	if !found {
		t.Errorf("expected SOLVER_OVER_SEQUENCED warning, got: %+v", errs)
	}
}
