package solver

import (
	"testing"
)

// --- DetectCycles tests ---

func TestDetectCycles_NoCycle(t *testing.T) {
	nodes := []DepNode{
		{AgentID: "A"},
		{AgentID: "B", DependsOn: []string{"A"}},
		{AgentID: "C", DependsOn: []string{"B"}},
	}
	cycles := detectCycles(nodes)
	if cycles != nil {
		t.Fatalf("expected no cycles, got %v", cycles)
	}
}

func TestDetectCycles_SimpleCycle(t *testing.T) {
	nodes := []DepNode{
		{AgentID: "A", DependsOn: []string{"B"}},
		{AgentID: "B", DependsOn: []string{"A"}},
	}
	cycles := detectCycles(nodes)
	if cycles == nil {
		t.Fatal("expected cycles, got nil")
	}
	// Should find cycle(s) containing both A and B
	found := false
	for _, c := range cycles {
		if containsAll(c, []string{"A", "B"}) {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected cycle containing A and B, got %v", cycles)
	}
}

func TestDetectCycles_MultipleCycles(t *testing.T) {
	nodes := []DepNode{
		{AgentID: "A", DependsOn: []string{"B"}},
		{AgentID: "B", DependsOn: []string{"A"}},
		{AgentID: "C", DependsOn: []string{"D"}},
		{AgentID: "D", DependsOn: []string{"C"}},
	}
	cycles := detectCycles(nodes)
	if cycles == nil {
		t.Fatal("expected cycles, got nil")
	}
	if len(cycles) < 2 {
		t.Fatalf("expected at least 2 cycle paths, got %d: %v", len(cycles), cycles)
	}
}

func TestDetectCycles_SelfLoop(t *testing.T) {
	nodes := []DepNode{
		{AgentID: "A", DependsOn: []string{"A"}},
	}
	cycles := detectCycles(nodes)
	if cycles == nil {
		t.Fatal("expected cycle, got nil")
	}
	// Should have a cycle path like ["A", "A"]
	found := false
	for _, c := range cycles {
		if len(c) == 2 && c[0] == "A" && c[1] == "A" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected self-loop cycle [A, A], got %v", cycles)
	}
}

// --- TransitiveDeps tests ---

func TestTransitiveDeps_Linear(t *testing.T) {
	nodes := []DepNode{
		{AgentID: "A"},
		{AgentID: "B", DependsOn: []string{"A"}},
		{AgentID: "C", DependsOn: []string{"B"}},
	}
	deps := transitiveDeps(nodes, "C")
	expected := []string{"A", "B"}
	if !sliceEqual(deps, expected) {
		t.Fatalf("expected %v, got %v", expected, deps)
	}
}

func TestTransitiveDeps_Diamond(t *testing.T) {
	nodes := []DepNode{
		{AgentID: "A"},
		{AgentID: "B"},
		{AgentID: "C", DependsOn: []string{"A", "B"}},
	}
	deps := transitiveDeps(nodes, "C")
	expected := []string{"A", "B"}
	if !sliceEqual(deps, expected) {
		t.Fatalf("expected %v, got %v", expected, deps)
	}
}

func TestTransitiveDeps_UnknownAgent(t *testing.T) {
	nodes := []DepNode{
		{AgentID: "A"},
		{AgentID: "B", DependsOn: []string{"A"}},
	}
	deps := transitiveDeps(nodes, "Z")
	if deps != nil {
		t.Fatalf("expected nil for unknown agent, got %v", deps)
	}
}

func TestTransitiveDeps_Root(t *testing.T) {
	nodes := []DepNode{
		{AgentID: "A"},
		{AgentID: "B", DependsOn: []string{"A"}},
	}
	deps := transitiveDeps(nodes, "A")
	if deps != nil {
		t.Fatalf("expected nil for root agent, got %v", deps)
	}
}

// --- CriticalPath tests ---

func TestCriticalPath_Linear(t *testing.T) {
	nodes := []DepNode{
		{AgentID: "A"},
		{AgentID: "B", DependsOn: []string{"A"}},
		{AgentID: "C", DependsOn: []string{"B"}},
	}
	path := CriticalPath(nodes)
	expected := []string{"A", "B", "C"}
	if !sliceEqual(path, expected) {
		t.Fatalf("expected %v, got %v", expected, path)
	}
}

func TestCriticalPath_Wide(t *testing.T) {
	nodes := []DepNode{
		{AgentID: "A"},
		{AgentID: "B"},
		{AgentID: "C"},
	}
	path := CriticalPath(nodes)
	if len(path) != 1 {
		t.Fatalf("expected critical path length 1, got %d: %v", len(path), path)
	}
}

func TestCriticalPath_WithCycle(t *testing.T) {
	nodes := []DepNode{
		{AgentID: "A", DependsOn: []string{"B"}},
		{AgentID: "B", DependsOn: []string{"A"}},
	}
	path := CriticalPath(nodes)
	if path != nil {
		t.Fatalf("expected nil for cyclic graph, got %v", path)
	}
}

// --- ValidateRefs tests ---

func TestValidateRefs_AllValid(t *testing.T) {
	nodes := []DepNode{
		{AgentID: "A"},
		{AgentID: "B", DependsOn: []string{"A"}},
		{AgentID: "C", DependsOn: []string{"A", "B"}},
	}
	errs := ValidateRefs(nodes)
	if errs != nil {
		t.Fatalf("expected no errors, got %v", errs)
	}
}

func TestValidateRefs_MissingRef(t *testing.T) {
	nodes := []DepNode{
		{AgentID: "A", DependsOn: []string{"Z"}},
	}
	errs := ValidateRefs(nodes)
	if errs == nil {
		t.Fatal("expected errors, got nil")
	}
	if len(errs) != 1 {
		t.Fatalf("expected 1 error, got %d: %v", len(errs), errs)
	}
	if !containsStr(errs[0], "Z") {
		t.Fatalf("expected error mentioning Z, got %q", errs[0])
	}
}

func TestValidateRefs_MultipleMissing(t *testing.T) {
	nodes := []DepNode{
		{AgentID: "A", DependsOn: []string{"X"}},
		{AgentID: "B", DependsOn: []string{"Y"}},
	}
	errs := ValidateRefs(nodes)
	if errs == nil {
		t.Fatal("expected errors, got nil")
	}
	if len(errs) != 2 {
		t.Fatalf("expected 2 errors, got %d: %v", len(errs), errs)
	}
}

// --- Helpers ---

func containsAll(slice []string, targets []string) bool {
	m := make(map[string]bool)
	for _, s := range slice {
		m[s] = true
	}
	for _, t := range targets {
		if !m[t] {
			return false
		}
	}
	return true
}

func sliceEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func containsStr(s, substr string) bool {
	return len(s) >= len(substr) && searchStr(s, substr)
}

func searchStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
