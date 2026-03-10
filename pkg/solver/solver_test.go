package solver

import (
	"strings"
	"testing"
)

func TestSolve_SingleAgent(t *testing.T) {
	nodes := []DepNode{
		{AgentID: "A"},
	}
	result := Solve(nodes)
	if !result.Valid {
		t.Fatalf("expected valid result, got errors: %v", result.Errors)
	}
	if result.WaveCount != 1 {
		t.Errorf("expected WaveCount=1, got %d", result.WaveCount)
	}
	if len(result.Assignments) != 1 {
		t.Fatalf("expected 1 assignment, got %d", len(result.Assignments))
	}
	if result.Assignments[0].AgentID != "A" || result.Assignments[0].Wave != 1 {
		t.Errorf("expected A=wave1, got %s=wave%d", result.Assignments[0].AgentID, result.Assignments[0].Wave)
	}
}

func TestSolve_LinearChain(t *testing.T) {
	nodes := []DepNode{
		{AgentID: "A"},
		{AgentID: "B", DependsOn: []string{"A"}},
		{AgentID: "C", DependsOn: []string{"B"}},
	}
	result := Solve(nodes)
	if !result.Valid {
		t.Fatalf("expected valid result, got errors: %v", result.Errors)
	}
	if result.WaveCount != 3 {
		t.Errorf("expected WaveCount=3, got %d", result.WaveCount)
	}
	expected := map[string]int{"A": 1, "B": 2, "C": 3}
	for _, a := range result.Assignments {
		if expected[a.AgentID] != a.Wave {
			t.Errorf("expected %s=wave%d, got wave%d", a.AgentID, expected[a.AgentID], a.Wave)
		}
	}
}

func TestSolve_DiamondDependency(t *testing.T) {
	nodes := []DepNode{
		{AgentID: "A"},
		{AgentID: "B"},
		{AgentID: "C", DependsOn: []string{"A", "B"}},
	}
	result := Solve(nodes)
	if !result.Valid {
		t.Fatalf("expected valid result, got errors: %v", result.Errors)
	}
	if result.WaveCount != 2 {
		t.Errorf("expected WaveCount=2, got %d", result.WaveCount)
	}
	expected := map[string]int{"A": 1, "B": 1, "C": 2}
	for _, a := range result.Assignments {
		if expected[a.AgentID] != a.Wave {
			t.Errorf("expected %s=wave%d, got wave%d", a.AgentID, expected[a.AgentID], a.Wave)
		}
	}
}

func TestSolve_ComplexGraph(t *testing.T) {
	nodes := []DepNode{
		{AgentID: "A"},
		{AgentID: "B"},
		{AgentID: "C", DependsOn: []string{"A"}},
		{AgentID: "D", DependsOn: []string{"A", "B"}},
		{AgentID: "E", DependsOn: []string{"C", "D"}},
		{AgentID: "F", DependsOn: []string{"E"}},
	}
	result := Solve(nodes)
	if !result.Valid {
		t.Fatalf("expected valid result, got errors: %v", result.Errors)
	}
	if result.WaveCount != 4 {
		t.Errorf("expected WaveCount=4, got %d", result.WaveCount)
	}
	expected := map[string]int{"A": 1, "B": 1, "C": 2, "D": 2, "E": 3, "F": 4}
	for _, a := range result.Assignments {
		if expected[a.AgentID] != a.Wave {
			t.Errorf("expected %s=wave%d, got wave%d", a.AgentID, expected[a.AgentID], a.Wave)
		}
	}

	// Verify sort order: (Wave ASC, AgentID ASC).
	for i := 1; i < len(result.Assignments); i++ {
		prev := result.Assignments[i-1]
		curr := result.Assignments[i]
		if prev.Wave > curr.Wave || (prev.Wave == curr.Wave && prev.AgentID >= curr.AgentID) {
			t.Errorf("assignments not sorted correctly at index %d: %v before %v", i, prev, curr)
		}
	}
}

func TestSolve_CycleDetection(t *testing.T) {
	nodes := []DepNode{
		{AgentID: "A", DependsOn: []string{"B"}},
		{AgentID: "B", DependsOn: []string{"A"}},
	}
	result := Solve(nodes)
	if result.Valid {
		t.Fatal("expected invalid result for cycle, got valid")
	}
	if len(result.Errors) == 0 {
		t.Fatal("expected at least one error")
	}
	found := false
	for _, e := range result.Errors {
		if strings.Contains(strings.ToLower(e), "cycle") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected error containing 'cycle', got: %v", result.Errors)
	}
}

func TestSolve_MissingDependency(t *testing.T) {
	nodes := []DepNode{
		{AgentID: "A", DependsOn: []string{"Z"}},
	}
	result := Solve(nodes)
	if result.Valid {
		t.Fatal("expected invalid result for missing dep, got valid")
	}
	if len(result.Errors) == 0 {
		t.Fatal("expected at least one error")
	}
	found := false
	for _, e := range result.Errors {
		lower := strings.ToLower(e)
		if strings.Contains(lower, "missing") || strings.Contains(lower, "unknown") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected error containing 'missing' or 'unknown', got: %v", result.Errors)
	}
}

func TestSolve_AllRoots(t *testing.T) {
	nodes := []DepNode{
		{AgentID: "A"},
		{AgentID: "B"},
		{AgentID: "C"},
	}
	result := Solve(nodes)
	if !result.Valid {
		t.Fatalf("expected valid result, got errors: %v", result.Errors)
	}
	if result.WaveCount != 1 {
		t.Errorf("expected WaveCount=1, got %d", result.WaveCount)
	}
	for _, a := range result.Assignments {
		if a.Wave != 1 {
			t.Errorf("expected %s=wave1, got wave%d", a.AgentID, a.Wave)
		}
	}
}

func TestSolve_DeterministicOrdering(t *testing.T) {
	nodes := []DepNode{
		{AgentID: "E", DependsOn: []string{"C", "D"}},
		{AgentID: "A"},
		{AgentID: "D", DependsOn: []string{"A", "B"}},
		{AgentID: "B"},
		{AgentID: "C", DependsOn: []string{"A"}},
	}

	first := Solve(nodes)
	if !first.Valid {
		t.Fatalf("expected valid result, got errors: %v", first.Errors)
	}

	for i := 0; i < 10; i++ {
		result := Solve(nodes)
		if len(result.Assignments) != len(first.Assignments) {
			t.Fatalf("run %d: different assignment count", i)
		}
		for j, a := range result.Assignments {
			if a != first.Assignments[j] {
				t.Fatalf("run %d: assignment %d differs: %v vs %v", i, j, a, first.Assignments[j])
			}
		}
		if result.WaveCount != first.WaveCount {
			t.Fatalf("run %d: WaveCount differs: %d vs %d", i, result.WaveCount, first.WaveCount)
		}
	}
}

func TestSolve_EmptyInput(t *testing.T) {
	result := Solve(nil)
	if !result.Valid {
		t.Fatalf("expected valid result for empty input, got errors: %v", result.Errors)
	}
	if result.WaveCount != 0 {
		t.Errorf("expected WaveCount=0, got %d", result.WaveCount)
	}
	if len(result.Assignments) != 0 {
		t.Errorf("expected empty assignments, got %d", len(result.Assignments))
	}

	// Also test empty slice (not just nil).
	result2 := Solve([]DepNode{})
	if !result2.Valid {
		t.Fatalf("expected valid result for empty slice, got errors: %v", result2.Errors)
	}
	if result2.WaveCount != 0 {
		t.Errorf("expected WaveCount=0, got %d", result2.WaveCount)
	}
}

func TestSolve_SelfDependency(t *testing.T) {
	nodes := []DepNode{
		{AgentID: "A", DependsOn: []string{"A"}},
	}
	result := Solve(nodes)
	if result.Valid {
		t.Fatal("expected invalid result for self-dependency, got valid")
	}
	found := false
	for _, e := range result.Errors {
		if strings.Contains(strings.ToLower(e), "cycle") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected error containing 'cycle', got: %v", result.Errors)
	}
}
