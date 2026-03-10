package protocol

import (
	"testing"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/solver"
)

// helper to build a minimal valid manifest with the given waves.
// Each waveSpec is {number, []agentSpec} where agentSpec is {id, deps, files}.
type agentSpec struct {
	id   string
	deps []string
	files []string
}

func buildManifest(title, slug, verdict string, waves []Wave, fo []FileOwnership) *IMPLManifest {
	return &IMPLManifest{
		Title:         title,
		FeatureSlug:   slug,
		Verdict:       verdict,
		Waves:         waves,
		FileOwnership: fo,
	}
}

func makeWave(num int, agents ...agentSpec) Wave {
	w := Wave{Number: num}
	for _, a := range agents {
		w.Agents = append(w.Agents, Agent{
			ID:           a.id,
			Task:         "task for " + a.id,
			Files:        a.files,
			Dependencies: a.deps,
		})
	}
	return w
}

func makeFO(file, agent string, wave int, deps ...string) FileOwnership {
	return FileOwnership{
		File:      file,
		Agent:     agent,
		Wave:      wave,
		Action:    "new",
		DependsOn: deps,
	}
}

func hasErrorCode(errs []ValidationError, code string) bool {
	for _, e := range errs {
		if e.Code == code {
			return true
		}
	}
	return false
}

func countErrorCode(errs []ValidationError, code string) int {
	n := 0
	for _, e := range errs {
		if e.Code == code {
			n++
		}
	}
	return n
}

// --- ValidateWithSolver tests ---

func TestValidateWithSolver_ValidManifest(t *testing.T) {
	m := buildManifest("Test Feature", "test-feature", "SUITABLE",
		[]Wave{
			makeWave(1, agentSpec{"A", nil, []string{"a.go"}}, agentSpec{"B", nil, []string{"b.go"}}),
			makeWave(2, agentSpec{"C", []string{"A", "B"}, []string{"c.go"}}),
		},
		[]FileOwnership{
			makeFO("a.go", "A", 1),
			makeFO("b.go", "B", 1),
			makeFO("c.go", "C", 2, "A", "B"),
		},
	)

	errs := ValidateWithSolver(m)
	for _, e := range errs {
		if e.Code == "SOLVER_CYCLE" || e.Code == "SOLVER_MISSING_DEP" || e.Code == "SOLVER_WAVE_MISMATCH" {
			t.Errorf("unexpected SOLVER error: %+v", e)
		}
	}
}

func TestValidateWithSolver_DetectsCycle(t *testing.T) {
	m := buildManifest("Test", "test", "SUITABLE",
		[]Wave{
			makeWave(1, agentSpec{"A", []string{"B"}, []string{"a.go"}}, agentSpec{"B", []string{"A"}, []string{"b.go"}}),
		},
		[]FileOwnership{
			makeFO("a.go", "A", 1),
			makeFO("b.go", "B", 1),
		},
	)

	errs := ValidateWithSolver(m)
	if !hasErrorCode(errs, "SOLVER_CYCLE") {
		t.Errorf("expected SOLVER_CYCLE error, got: %+v", errs)
	}
}

func TestValidateWithSolver_DetectsMissingDep(t *testing.T) {
	m := buildManifest("Test", "test", "SUITABLE",
		[]Wave{
			makeWave(1, agentSpec{"A", nil, []string{"a.go"}}),
			makeWave(2, agentSpec{"C", []string{"Z"}, []string{"c.go"}}),
		},
		[]FileOwnership{
			makeFO("a.go", "A", 1),
			makeFO("c.go", "C", 2),
		},
	)

	errs := ValidateWithSolver(m)
	if !hasErrorCode(errs, "SOLVER_MISSING_DEP") {
		t.Errorf("expected SOLVER_MISSING_DEP error, got: %+v", errs)
	}
}

func TestValidateWithSolver_DetectsMismatch(t *testing.T) {
	// C depends on A but is placed in wave 1 alongside A.
	m := buildManifest("Test", "test", "SUITABLE",
		[]Wave{
			makeWave(1,
				agentSpec{"A", nil, []string{"a.go"}},
				agentSpec{"C", []string{"A"}, []string{"c.go"}},
			),
		},
		[]FileOwnership{
			makeFO("a.go", "A", 1),
			makeFO("c.go", "C", 1),
		},
	)

	errs := ValidateWithSolver(m)
	if !hasErrorCode(errs, "SOLVER_WAVE_MISMATCH") {
		t.Errorf("expected SOLVER_WAVE_MISMATCH error, got: %+v", errs)
	}
}

func TestValidateWithSolver_IncludesBaseValidation(t *testing.T) {
	// Valid graph but missing title.
	m := buildManifest("", "test", "SUITABLE",
		[]Wave{
			makeWave(1, agentSpec{"A", nil, []string{"a.go"}}),
		},
		[]FileOwnership{
			makeFO("a.go", "A", 1),
		},
	)

	errs := ValidateWithSolver(m)
	if !hasErrorCode(errs, "I4_MISSING_FIELD") {
		t.Errorf("expected I4_MISSING_FIELD error from base validation, got: %+v", errs)
	}
	// Should have no solver errors (graph is valid and optimal).
	for _, e := range errs {
		if e.Code == "SOLVER_CYCLE" || e.Code == "SOLVER_MISSING_DEP" {
			t.Errorf("unexpected solver error: %+v", e)
		}
	}
}

func TestValidateWithSolver_OptimalAssignment(t *testing.T) {
	// All agents already in solver-optimal waves.
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
	if hasErrorCode(errs, "SOLVER_WAVE_MISMATCH") {
		t.Errorf("expected no SOLVER_WAVE_MISMATCH, got: %+v", errs)
	}
}

// --- manifestToNodes tests ---

func TestManifestToNodes_Basic(t *testing.T) {
	m := buildManifest("Test", "test", "SUITABLE",
		[]Wave{
			makeWave(1, agentSpec{"A", nil, []string{"a.go"}}, agentSpec{"B", nil, []string{"b.go"}}),
			makeWave(2, agentSpec{"C", []string{"A", "B"}, []string{"c.go"}}),
		},
		nil,
	)

	nodes := manifestToNodes(m)
	if len(nodes) != 3 {
		t.Fatalf("expected 3 nodes, got %d", len(nodes))
	}

	// Should be sorted by AgentID.
	if nodes[0].AgentID != "A" || nodes[1].AgentID != "B" || nodes[2].AgentID != "C" {
		t.Errorf("nodes not sorted by AgentID: %v", nodes)
	}

	// C should have deps A, B.
	if len(nodes[2].DependsOn) != 2 {
		t.Errorf("expected C to have 2 deps, got %d", len(nodes[2].DependsOn))
	}

	// A should have file a.go.
	if len(nodes[0].Files) != 1 || nodes[0].Files[0] != "a.go" {
		t.Errorf("expected A to have file a.go, got %v", nodes[0].Files)
	}
}

func TestManifestToNodes_Empty(t *testing.T) {
	m := buildManifest("Test", "test", "SUITABLE", nil, nil)

	nodes := manifestToNodes(m)
	if len(nodes) != 0 {
		t.Errorf("expected 0 nodes, got %d", len(nodes))
	}
}

// --- applyResult tests ---

func TestApplyResult_Reorders(t *testing.T) {
	// C depends on A but both are in wave 1. Solver says C should be wave 2.
	m := buildManifest("Test", "test", "SUITABLE",
		[]Wave{
			makeWave(1,
				agentSpec{"A", nil, []string{"a.go"}},
				agentSpec{"C", []string{"A"}, []string{"c.go"}},
			),
		},
		[]FileOwnership{
			makeFO("a.go", "A", 1),
			makeFO("c.go", "C", 1),
		},
	)

	result := solver.SolveResult{
		Valid:     true,
		WaveCount: 2,
		Assignments: []solver.Assignment{
			{AgentID: "A", Wave: 1},
			{AgentID: "C", Wave: 2},
		},
	}

	fixed, err := applyResult(result, m)
	if err != nil {
		t.Fatalf("applyResult error: %v", err)
	}

	if len(fixed.Waves) != 2 {
		t.Fatalf("expected 2 waves, got %d", len(fixed.Waves))
	}
	if fixed.Waves[0].Number != 1 {
		t.Errorf("expected wave 1, got %d", fixed.Waves[0].Number)
	}
	if fixed.Waves[1].Number != 2 {
		t.Errorf("expected wave 2, got %d", fixed.Waves[1].Number)
	}
	if len(fixed.Waves[0].Agents) != 1 || fixed.Waves[0].Agents[0].ID != "A" {
		t.Errorf("expected wave 1 to contain only A, got %v", fixed.Waves[0].Agents)
	}
	if len(fixed.Waves[1].Agents) != 1 || fixed.Waves[1].Agents[0].ID != "C" {
		t.Errorf("expected wave 2 to contain only C, got %v", fixed.Waves[1].Agents)
	}

	// Verify original not mutated.
	if len(m.Waves) != 1 {
		t.Errorf("original manifest was mutated: expected 1 wave, got %d", len(m.Waves))
	}
}

func TestApplyResult_UpdatesFileOwnership(t *testing.T) {
	m := buildManifest("Test", "test", "SUITABLE",
		[]Wave{
			makeWave(1,
				agentSpec{"A", nil, []string{"a.go"}},
				agentSpec{"C", []string{"A"}, []string{"c.go"}},
			),
		},
		[]FileOwnership{
			makeFO("a.go", "A", 1),
			makeFO("c.go", "C", 1),
		},
	)

	result := solver.SolveResult{
		Valid:     true,
		WaveCount: 2,
		Assignments: []solver.Assignment{
			{AgentID: "A", Wave: 1},
			{AgentID: "C", Wave: 2},
		},
	}

	fixed, err := applyResult(result, m)
	if err != nil {
		t.Fatalf("applyResult error: %v", err)
	}

	for _, fo := range fixed.FileOwnership {
		if fo.File == "c.go" && fo.Wave != 2 {
			t.Errorf("expected c.go FileOwnership wave to be 2, got %d", fo.Wave)
		}
		if fo.File == "a.go" && fo.Wave != 1 {
			t.Errorf("expected a.go FileOwnership wave to be 1, got %d", fo.Wave)
		}
	}
}

func TestApplyResult_InvalidResult(t *testing.T) {
	m := buildManifest("Test", "test", "SUITABLE", nil, nil)

	result := solver.SolveResult{Valid: false, Errors: []string{"cycle"}}
	_, err := applyResult(result, m)
	if err == nil {
		t.Error("expected error for invalid solver result")
	}
}

// --- compareAssignments tests ---

func TestCompareAssignments_AllMatch(t *testing.T) {
	m := buildManifest("Test", "test", "SUITABLE",
		[]Wave{
			makeWave(1, agentSpec{"A", nil, nil}),
			makeWave(2, agentSpec{"B", []string{"A"}, nil}),
		},
		nil,
	)

	result := solver.SolveResult{
		Valid: true,
		Assignments: []solver.Assignment{
			{AgentID: "A", Wave: 1},
			{AgentID: "B", Wave: 2},
		},
	}

	mismatches := compareAssignments(m, result)
	if mismatches != nil {
		t.Errorf("expected nil mismatches, got %+v", mismatches)
	}
}

func TestCompareAssignments_OneMismatch(t *testing.T) {
	m := buildManifest("Test", "test", "SUITABLE",
		[]Wave{
			makeWave(1, agentSpec{"A", nil, nil}, agentSpec{"C", []string{"A"}, nil}),
		},
		nil,
	)

	result := solver.SolveResult{
		Valid: true,
		Assignments: []solver.Assignment{
			{AgentID: "A", Wave: 1},
			{AgentID: "C", Wave: 2},
		},
	}

	mismatches := compareAssignments(m, result)
	if len(mismatches) != 1 {
		t.Fatalf("expected 1 mismatch, got %d", len(mismatches))
	}
	if mismatches[0].agentID != "C" {
		t.Errorf("expected mismatch for C, got %s", mismatches[0].agentID)
	}
	if mismatches[0].manifestWave != 1 || mismatches[0].solverWave != 2 {
		t.Errorf("expected manifest=1 solver=2, got manifest=%d solver=%d",
			mismatches[0].manifestWave, mismatches[0].solverWave)
	}
}

// --- RoundTrip test ---

func TestRoundTrip_SolveAndApply(t *testing.T) {
	// Manifest with agents in wrong waves.
	m := buildManifest("RoundTrip", "round-trip", "SUITABLE",
		[]Wave{
			makeWave(1,
				agentSpec{"A", nil, []string{"a.go"}},
				agentSpec{"B", nil, []string{"b.go"}},
				agentSpec{"C", []string{"A", "B"}, []string{"c.go"}},
			),
		},
		[]FileOwnership{
			makeFO("a.go", "A", 1),
			makeFO("b.go", "B", 1),
			makeFO("c.go", "C", 1, "A", "B"),
		},
	)

	nodes := manifestToNodes(m)
	result := solver.Solve(nodes)
	if !result.Valid {
		t.Fatalf("solver failed: %v", result.Errors)
	}

	fixed, err := applyResult(result, m)
	if err != nil {
		t.Fatalf("applyResult error: %v", err)
	}

	// The fixed manifest should pass I2 and I3 validation.
	errs := Validate(fixed)
	for _, e := range errs {
		if e.Code == "I2_WAVE_ORDER" || e.Code == "I3_WAVE_ORDER" {
			t.Errorf("fixed manifest has ordering error: %+v", e)
		}
	}
}

// --- SolveManifest tests ---

func TestSolveManifest_NoChanges(t *testing.T) {
	m := buildManifest("Test", "test", "SUITABLE",
		[]Wave{
			makeWave(1, agentSpec{"A", nil, []string{"a.go"}}),
			makeWave(2, agentSpec{"B", []string{"A"}, []string{"b.go"}}),
		},
		[]FileOwnership{
			makeFO("a.go", "A", 1),
			makeFO("b.go", "B", 2, "A"),
		},
	)

	result, changes, err := SolveManifest(m)
	if err != nil {
		t.Fatalf("SolveManifest error: %v", err)
	}
	if changes != nil {
		t.Errorf("expected nil changes, got %v", changes)
	}
	if result != m {
		t.Errorf("expected same manifest pointer when no changes needed")
	}
}

func TestSolveManifest_FixesMisassignment(t *testing.T) {
	// C depends on A but is in wave 1 alongside A.
	m := buildManifest("Test", "test", "SUITABLE",
		[]Wave{
			makeWave(1,
				agentSpec{"A", nil, []string{"a.go"}},
				agentSpec{"C", []string{"A"}, []string{"c.go"}},
			),
		},
		[]FileOwnership{
			makeFO("a.go", "A", 1),
			makeFO("c.go", "C", 1),
		},
	)

	result, changes, err := SolveManifest(m)
	if err != nil {
		t.Fatalf("SolveManifest error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if len(changes) == 0 {
		t.Fatal("expected changes")
	}

	// Check change description.
	found := false
	for _, c := range changes {
		if c == "agent C: wave 1 → wave 2" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected change 'agent C: wave 1 → wave 2', got %v", changes)
	}

	// Verify the result has C in wave 2.
	for _, w := range result.Waves {
		for _, a := range w.Agents {
			if a.ID == "C" && w.Number != 2 {
				t.Errorf("expected C in wave 2, got wave %d", w.Number)
			}
		}
	}
}

func TestSolveManifest_CycleReturnsError(t *testing.T) {
	m := buildManifest("Test", "test", "SUITABLE",
		[]Wave{
			makeWave(1,
				agentSpec{"A", []string{"B"}, []string{"a.go"}},
				agentSpec{"B", []string{"A"}, []string{"b.go"}},
			),
		},
		[]FileOwnership{
			makeFO("a.go", "A", 1),
			makeFO("b.go", "B", 1),
		},
	)

	result, changes, err := SolveManifest(m)
	if err == nil {
		t.Fatal("expected error for cycle")
	}
	if result != nil {
		t.Errorf("expected nil manifest, got %+v", result)
	}
	if changes != nil {
		t.Errorf("expected nil changes, got %v", changes)
	}
	if !containsStr(err.Error(), "cycle") {
		t.Errorf("expected error to mention 'cycle', got: %s", err.Error())
	}
}

func TestSolveManifest_UpdatesFileOwnership(t *testing.T) {
	m := buildManifest("Test", "test", "SUITABLE",
		[]Wave{
			makeWave(1,
				agentSpec{"A", nil, []string{"a.go"}},
				agentSpec{"C", []string{"A"}, []string{"c.go"}},
			),
		},
		[]FileOwnership{
			makeFO("a.go", "A", 1),
			makeFO("c.go", "C", 1),
		},
	)

	result, _, err := SolveManifest(m)
	if err != nil {
		t.Fatalf("SolveManifest error: %v", err)
	}

	for _, fo := range result.FileOwnership {
		if fo.File == "c.go" && fo.Wave != 2 {
			t.Errorf("expected c.go FileOwnership wave=2, got %d", fo.Wave)
		}
	}
}

func containsStr(s, substr string) bool {
	return len(s) >= len(substr) && findSubstr(s, substr)
}

func findSubstr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
