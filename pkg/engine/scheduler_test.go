package engine

import (
	"os"
	"reflect"
	"testing"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/protocol"
)

func TestPrioritizeAgents_LinearDependencies(t *testing.T) {
	// A → B → C (linear chain)
	manifest := &protocol.IMPLManifest{
		Waves: []protocol.Wave{
			{
				Number: 1,
				Agents: []protocol.Agent{
					{ID: "C", Files: []string{"c.go"}},
					{ID: "B", Files: []string{"b.go"}},
					{ID: "A", Files: []string{"a.go"}},
				},
			},
		},
		FileOwnership: []protocol.FileOwnership{
			{File: "a.go", Agent: "A", Wave: 1},
			{File: "b.go", Agent: "B", Wave: 1, DependsOn: []string{"A"}},
			{File: "c.go", Agent: "C", Wave: 1, DependsOn: []string{"B"}},
		},
	}

	result := PrioritizeAgents(manifest, 1)
	expected := []string{"A", "B", "C"}

	if !reflect.DeepEqual(result, expected) {
		t.Errorf("Expected %v, got %v", expected, result)
	}
}

func TestPrioritizeAgents_ParallelAgents(t *testing.T) {
	// All agents independent, should sort by file count
	manifest := &protocol.IMPLManifest{
		Waves: []protocol.Wave{
			{
				Number: 1,
				Agents: []protocol.Agent{
					{ID: "A", Files: []string{"a1.go", "a2.go", "a3.go"}},
					{ID: "B", Files: []string{"b1.go"}},
					{ID: "C", Files: []string{"c1.go", "c2.go"}},
				},
			},
		},
		FileOwnership: []protocol.FileOwnership{
			{File: "a1.go", Agent: "A", Wave: 1},
			{File: "a2.go", Agent: "A", Wave: 1},
			{File: "a3.go", Agent: "A", Wave: 1},
			{File: "b1.go", Agent: "B", Wave: 1},
			{File: "c1.go", Agent: "C", Wave: 1},
			{File: "c2.go", Agent: "C", Wave: 1},
		},
	}

	result := PrioritizeAgents(manifest, 1)
	expected := []string{"B", "C", "A"} // sorted by file count: 1, 2, 3

	if !reflect.DeepEqual(result, expected) {
		t.Errorf("Expected %v, got %v", expected, result)
	}
}

func TestPrioritizeAgents_DiamondGraph(t *testing.T) {
	// A → B, A → C, B → D, C → D
	// Critical paths: A=3, B=2, C=2, D=1
	// A launches first (depth 3), then B and C sorted by file count, then D
	manifest := &protocol.IMPLManifest{
		Waves: []protocol.Wave{
			{
				Number: 1,
				Agents: []protocol.Agent{
					{ID: "D", Files: []string{"d.go"}},
					{ID: "C", Files: []string{"c1.go", "c2.go", "c3.go"}},
					{ID: "B", Files: []string{"b1.go", "b2.go"}},
					{ID: "A", Files: []string{"a.go"}},
				},
			},
		},
		FileOwnership: []protocol.FileOwnership{
			{File: "a.go", Agent: "A", Wave: 1},
			{File: "b1.go", Agent: "B", Wave: 1, DependsOn: []string{"A"}},
			{File: "b2.go", Agent: "B", Wave: 1, DependsOn: []string{"A"}},
			{File: "c1.go", Agent: "C", Wave: 1, DependsOn: []string{"A"}},
			{File: "c2.go", Agent: "C", Wave: 1, DependsOn: []string{"A"}},
			{File: "c3.go", Agent: "C", Wave: 1, DependsOn: []string{"A"}},
			{File: "d.go", Agent: "D", Wave: 1, DependsOn: []string{"B", "C"}},
		},
	}

	result := PrioritizeAgents(manifest, 1)
	// A has depth 3, B has depth 2, C has depth 2, D has depth 1
	// B and C tied at depth 2, so sort by file count: B (2 files) < C (3 files)
	expected := []string{"A", "B", "C", "D"}

	if !reflect.DeepEqual(result, expected) {
		t.Errorf("Expected %v, got %v", expected, result)
	}
}

func TestPrioritizeAgents_SingleAgent(t *testing.T) {
	// Single agent wave should return agent ID unchanged
	manifest := &protocol.IMPLManifest{
		Waves: []protocol.Wave{
			{
				Number: 1,
				Agents: []protocol.Agent{
					{ID: "SOLO", Files: []string{"solo.go"}},
				},
			},
		},
		FileOwnership: []protocol.FileOwnership{
			{File: "solo.go", Agent: "SOLO", Wave: 1},
		},
	}

	result := PrioritizeAgents(manifest, 1)
	expected := []string{"SOLO"}

	if !reflect.DeepEqual(result, expected) {
		t.Errorf("Expected %v, got %v", expected, result)
	}
}

func TestPrioritizeAgents_FileCountTieBreaker(t *testing.T) {
	// A → B, A → C, A → D
	// All of B, C, D have same critical path depth (1)
	// Should sort by file count: C (1 file) < D (2 files) < B (3 files)
	manifest := &protocol.IMPLManifest{
		Waves: []protocol.Wave{
			{
				Number: 1,
				Agents: []protocol.Agent{
					{ID: "B", Files: []string{"b1.go", "b2.go", "b3.go"}},
					{ID: "D", Files: []string{"d1.go", "d2.go"}},
					{ID: "C", Files: []string{"c1.go"}},
					{ID: "A", Files: []string{"a.go"}},
				},
			},
		},
		FileOwnership: []protocol.FileOwnership{
			{File: "a.go", Agent: "A", Wave: 1},
			{File: "b1.go", Agent: "B", Wave: 1, DependsOn: []string{"A"}},
			{File: "b2.go", Agent: "B", Wave: 1, DependsOn: []string{"A"}},
			{File: "b3.go", Agent: "B", Wave: 1, DependsOn: []string{"A"}},
			{File: "c1.go", Agent: "C", Wave: 1, DependsOn: []string{"A"}},
			{File: "d1.go", Agent: "D", Wave: 1, DependsOn: []string{"A"}},
			{File: "d2.go", Agent: "D", Wave: 1, DependsOn: []string{"A"}},
		},
	}

	result := PrioritizeAgents(manifest, 1)
	// A has depth 2 (launches first)
	// B, C, D all have depth 1, sort by file count: C (1), D (2), B (3)
	expected := []string{"A", "C", "D", "B"}

	if !reflect.DeepEqual(result, expected) {
		t.Errorf("Expected %v, got %v", expected, result)
	}
}

func TestPrioritizeAgents_NoReorderNeeded(t *testing.T) {
	// Agents already in optimal order: A → B → C
	manifest := &protocol.IMPLManifest{
		Waves: []protocol.Wave{
			{
				Number: 1,
				Agents: []protocol.Agent{
					{ID: "A", Files: []string{"a.go"}},
					{ID: "B", Files: []string{"b.go"}},
					{ID: "C", Files: []string{"c.go"}},
				},
			},
		},
		FileOwnership: []protocol.FileOwnership{
			{File: "a.go", Agent: "A", Wave: 1},
			{File: "b.go", Agent: "B", Wave: 1, DependsOn: []string{"A"}},
			{File: "c.go", Agent: "C", Wave: 1, DependsOn: []string{"B"}},
		},
	}

	result := PrioritizeAgents(manifest, 1)
	expected := []string{"A", "B", "C"}

	if !reflect.DeepEqual(result, expected) {
		t.Errorf("Expected %v, got %v", expected, result)
	}
}

func TestPrioritizeAgents_EmptyWave(t *testing.T) {
	// Defensive: empty wave should return empty slice
	manifest := &protocol.IMPLManifest{
		Waves: []protocol.Wave{
			{
				Number: 1,
				Agents: []protocol.Agent{},
			},
		},
		FileOwnership: []protocol.FileOwnership{},
	}

	result := PrioritizeAgents(manifest, 1)
	expected := []string{}

	if !reflect.DeepEqual(result, expected) {
		t.Errorf("Expected %v, got %v", expected, result)
	}
}

func TestPrioritizeAgents_WaveNotFound(t *testing.T) {
	// Defensive: non-existent wave should return empty slice
	manifest := &protocol.IMPLManifest{
		Waves: []protocol.Wave{
			{
				Number: 1,
				Agents: []protocol.Agent{
					{ID: "A", Files: []string{"a.go"}},
				},
			},
		},
		FileOwnership: []protocol.FileOwnership{
			{File: "a.go", Agent: "A", Wave: 1},
		},
	}

	result := PrioritizeAgents(manifest, 99)
	expected := []string{}

	if !reflect.DeepEqual(result, expected) {
		t.Errorf("Expected %v, got %v", expected, result)
	}
}

func TestPrioritizeAgents_Disabled(t *testing.T) {
	// Set environment variable to disable prioritization
	os.Setenv("SAW_NO_PRIORITIZE", "1")
	defer os.Unsetenv("SAW_NO_PRIORITIZE")

	// A → B → C (linear chain)
	// Without prioritization, should return declaration order: C, B, A
	// With prioritization, would return: A, B, C
	manifest := &protocol.IMPLManifest{
		Waves: []protocol.Wave{
			{
				Number: 1,
				Agents: []protocol.Agent{
					{ID: "C", Files: []string{"c.go"}},
					{ID: "B", Files: []string{"b.go"}},
					{ID: "A", Files: []string{"a.go"}},
				},
			},
		},
		FileOwnership: []protocol.FileOwnership{
			{File: "a.go", Agent: "A", Wave: 1},
			{File: "b.go", Agent: "B", Wave: 1, DependsOn: []string{"A"}},
			{File: "c.go", Agent: "C", Wave: 1, DependsOn: []string{"B"}},
		},
	}

	result := PrioritizeAgents(manifest, 1)
	expected := []string{"C", "B", "A"} // Declaration order, not prioritized

	if !reflect.DeepEqual(result, expected) {
		t.Errorf("Expected declaration order %v, got %v", expected, result)
	}
}
