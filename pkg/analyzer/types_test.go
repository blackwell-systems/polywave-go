package analyzer

import (
	"testing"
)

func TestDepGraph_EmptyGraph(t *testing.T) {
	// Zero nodes, empty waves map, no cascades
	g := DepGraph{}

	if g.Nodes != nil {
		t.Errorf("expected nil Nodes, got %v", g.Nodes)
	}
	if g.Waves != nil {
		t.Errorf("expected nil Waves, got %v", g.Waves)
	}
	if g.CascadeCandidates != nil {
		t.Errorf("expected nil CascadeCandidates, got %v", g.CascadeCandidates)
	}

	// Empty initialization
	g2 := DepGraph{
		Nodes:             []FileNode{},
		Waves:             map[int][]string{},
		CascadeCandidates: []CascadeCandidate{},
	}

	if len(g2.Nodes) != 0 {
		t.Errorf("expected 0 nodes, got %d", len(g2.Nodes))
	}
	if len(g2.Waves) != 0 {
		t.Errorf("expected 0 waves, got %d", len(g2.Waves))
	}
	if len(g2.CascadeCandidates) != 0 {
		t.Errorf("expected 0 cascade candidates, got %d", len(g2.CascadeCandidates))
	}
}

func TestFileNode_ZeroFields(t *testing.T) {
	// Verify zero-value struct behavior
	n := FileNode{}

	if n.File != "" {
		t.Errorf("expected empty File, got %q", n.File)
	}
	if n.DependsOn != nil {
		t.Errorf("expected nil DependsOn, got %v", n.DependsOn)
	}
	if n.DependedBy != nil {
		t.Errorf("expected nil DependedBy, got %v", n.DependedBy)
	}
	if n.WaveCandidate != 0 {
		t.Errorf("expected WaveCandidate 0, got %d", n.WaveCandidate)
	}

	// Populated node
	n2 := FileNode{
		File:          "/abs/path/to/file.go",
		DependsOn:     []string{"/abs/path/to/dep1.go", "/abs/path/to/dep2.go"},
		DependedBy:    []string{"/abs/path/to/consumer.go"},
		WaveCandidate: 2,
	}

	if n2.File != "/abs/path/to/file.go" {
		t.Errorf("unexpected File: %q", n2.File)
	}
	if len(n2.DependsOn) != 2 {
		t.Errorf("expected 2 dependencies, got %d", len(n2.DependsOn))
	}
	if len(n2.DependedBy) != 1 {
		t.Errorf("expected 1 dependent, got %d", len(n2.DependedBy))
	}
	if n2.WaveCandidate != 2 {
		t.Errorf("expected WaveCandidate 2, got %d", n2.WaveCandidate)
	}
}

func TestCascadeCandidate_TypeValidation(t *testing.T) {
	// Ensure CascadeType field is "semantic" or empty
	tests := []struct {
		name     string
		cascade  CascadeCandidate
		wantType string
	}{
		{
			name:     "zero value",
			cascade:  CascadeCandidate{},
			wantType: "",
		},
		{
			name: "semantic type",
			cascade: CascadeCandidate{
				File:        "/abs/path/to/cascade.go",
				Reason:      "Imports modified file pkg/foo/bar.go",
				CascadeType: "semantic",
			},
			wantType: "semantic",
		},
		{
			name: "empty type with reason",
			cascade: CascadeCandidate{
				File:        "/abs/path/to/cascade.go",
				Reason:      "Some reason",
				CascadeType: "",
			},
			wantType: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.cascade.CascadeType != tt.wantType {
				t.Errorf("expected CascadeType %q, got %q", tt.wantType, tt.cascade.CascadeType)
			}

			// Verify CascadeType is either "semantic" or empty (Phase 1 constraint)
			if tt.cascade.CascadeType != "" && tt.cascade.CascadeType != "semantic" {
				t.Errorf("invalid CascadeType value %q, must be \"semantic\" or empty in Phase 1", tt.cascade.CascadeType)
			}
		})
	}
}
