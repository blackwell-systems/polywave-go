package analyzer

import (
	"encoding/json"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestToOutput_EmptyGraph(t *testing.T) {
	g := &DepGraph{
		Nodes:             []FileNode{},
		Waves:             map[int][]string{},
		CascadeCandidates: []CascadeFile{},
	}

	out := ToOutput(g)

	if out == nil {
		t.Fatal("ToOutput returned nil for empty graph")
	}

	if len(out.Nodes) != 0 {
		t.Errorf("Expected 0 nodes, got %d", len(out.Nodes))
	}

	if len(out.Waves) != 0 {
		t.Errorf("Expected 0 waves, got %d", len(out.Waves))
	}

	if len(out.CascadeCandidates) != 0 {
		t.Errorf("Expected 0 cascade candidates, got %d", len(out.CascadeCandidates))
	}
}

func TestToOutput_NilGraph(t *testing.T) {
	out := ToOutput(nil)

	if out == nil {
		t.Fatal("ToOutput returned nil for nil graph")
	}

	if len(out.Nodes) != 0 {
		t.Errorf("Expected 0 nodes, got %d", len(out.Nodes))
	}

	if len(out.Waves) != 0 {
		t.Errorf("Expected 0 waves, got %d", len(out.Waves))
	}
}

func TestToOutput_SingleNode(t *testing.T) {
	g := &DepGraph{
		Nodes: []FileNode{
			{
				File:          "pkg/foo/bar.go",
				DependsOn:     []string{},
				DependedBy:    []string{},
				WaveCandidate: 1,
			},
		},
		Waves: map[int][]string{
			1: {"pkg/foo/bar.go"},
		},
		CascadeCandidates: []CascadeFile{},
	}

	out := ToOutput(g)

	if len(out.Nodes) != 1 {
		t.Fatalf("Expected 1 node, got %d", len(out.Nodes))
	}

	node := out.Nodes[0]
	if node.File != "pkg/foo/bar.go" {
		t.Errorf("Expected file 'pkg/foo/bar.go', got '%s'", node.File)
	}
	if node.WaveCandidate != 1 {
		t.Errorf("Expected wave candidate 1, got %d", node.WaveCandidate)
	}
	if len(node.DependsOn) != 0 {
		t.Errorf("Expected 0 dependencies, got %d", len(node.DependsOn))
	}

	if len(out.Waves) != 1 {
		t.Fatalf("Expected 1 wave, got %d", len(out.Waves))
	}

	wave1 := out.Waves[1]
	if len(wave1) != 1 || wave1[0] != "pkg/foo/bar.go" {
		t.Errorf("Expected wave 1 to contain ['pkg/foo/bar.go'], got %v", wave1)
	}
}

func TestToOutput_WithDeps(t *testing.T) {
	// A -> B -> C (C has no deps, B depends on C, A depends on B)
	// So waves are: {1: [C], 2: [B], 3: [A]}
	g := &DepGraph{
		Nodes: []FileNode{
			{
				File:          "pkg/a.go",
				DependsOn:     []string{"pkg/b.go"},
				DependedBy:    []string{},
				WaveCandidate: 3,
			},
			{
				File:          "pkg/b.go",
				DependsOn:     []string{"pkg/c.go"},
				DependedBy:    []string{"pkg/a.go"},
				WaveCandidate: 2,
			},
			{
				File:          "pkg/c.go",
				DependsOn:     []string{},
				DependedBy:    []string{"pkg/b.go"},
				WaveCandidate: 1,
			},
		},
		Waves: map[int][]string{
			1: {"pkg/c.go"},
			2: {"pkg/b.go"},
			3: {"pkg/a.go"},
		},
		CascadeCandidates: []CascadeFile{},
	}

	out := ToOutput(g)

	if len(out.Nodes) != 3 {
		t.Fatalf("Expected 3 nodes, got %d", len(out.Nodes))
	}

	// Nodes should be sorted by file path
	// Expected order: pkg/a.go, pkg/b.go, pkg/c.go
	expectedFiles := []string{"pkg/a.go", "pkg/b.go", "pkg/c.go"}
	for i, expected := range expectedFiles {
		if out.Nodes[i].File != expected {
			t.Errorf("Node %d: expected file '%s', got '%s'", i, expected, out.Nodes[i].File)
		}
	}

	// Verify node dependencies
	for _, node := range out.Nodes {
		switch node.File {
		case "pkg/a.go":
			if node.WaveCandidate != 3 {
				t.Errorf("pkg/a.go: expected wave 3, got %d", node.WaveCandidate)
			}
			if len(node.DependsOn) != 1 || node.DependsOn[0] != "pkg/b.go" {
				t.Errorf("pkg/a.go: expected depends_on ['pkg/b.go'], got %v", node.DependsOn)
			}
			if len(node.DependedBy) != 0 {
				t.Errorf("pkg/a.go: expected depended_by [], got %v", node.DependedBy)
			}
		case "pkg/b.go":
			if node.WaveCandidate != 2 {
				t.Errorf("pkg/b.go: expected wave 2, got %d", node.WaveCandidate)
			}
			if len(node.DependsOn) != 1 || node.DependsOn[0] != "pkg/c.go" {
				t.Errorf("pkg/b.go: expected depends_on ['pkg/c.go'], got %v", node.DependsOn)
			}
			if len(node.DependedBy) != 1 || node.DependedBy[0] != "pkg/a.go" {
				t.Errorf("pkg/b.go: expected depended_by ['pkg/a.go'], got %v", node.DependedBy)
			}
		case "pkg/c.go":
			if node.WaveCandidate != 1 {
				t.Errorf("pkg/c.go: expected wave 1, got %d", node.WaveCandidate)
			}
			if len(node.DependsOn) != 0 {
				t.Errorf("pkg/c.go: expected depends_on [], got %v", node.DependsOn)
			}
			if len(node.DependedBy) != 1 || node.DependedBy[0] != "pkg/b.go" {
				t.Errorf("pkg/c.go: expected depended_by ['pkg/b.go'], got %v", node.DependedBy)
			}
		}
	}

	// Verify waves
	if len(out.Waves) != 3 {
		t.Fatalf("Expected 3 waves, got %d", len(out.Waves))
	}

	if len(out.Waves[1]) != 1 || out.Waves[1][0] != "pkg/c.go" {
		t.Errorf("Wave 1: expected ['pkg/c.go'], got %v", out.Waves[1])
	}
	if len(out.Waves[2]) != 1 || out.Waves[2][0] != "pkg/b.go" {
		t.Errorf("Wave 2: expected ['pkg/b.go'], got %v", out.Waves[2])
	}
	if len(out.Waves[3]) != 1 || out.Waves[3][0] != "pkg/a.go" {
		t.Errorf("Wave 3: expected ['pkg/a.go'], got %v", out.Waves[3])
	}
}

func TestToOutput_Cascades(t *testing.T) {
	g := &DepGraph{
		Nodes: []FileNode{
			{
				File:          "pkg/foo.go",
				DependsOn:     []string{},
				DependedBy:    []string{},
				WaveCandidate: 1,
			},
		},
		Waves: map[int][]string{
			1: {"pkg/foo.go"},
		},
		CascadeCandidates: []CascadeFile{
			{
				File:   "pkg/bar.go",
				Reason: "shares type definition with pkg/foo.go",
				Type:   "semantic",
			},
			{
				File:   "pkg/baz.go",
				Reason: "contains syntax error",
				Type:   "syntax",
			},
		},
	}

	out := ToOutput(g)

	if len(out.CascadeCandidates) != 2 {
		t.Fatalf("Expected 2 cascade candidates, got %d", len(out.CascadeCandidates))
	}

	// Verify first cascade
	c1 := out.CascadeCandidates[0]
	if c1.File != "pkg/bar.go" {
		t.Errorf("Cascade 0: expected file 'pkg/bar.go', got '%s'", c1.File)
	}
	if c1.Type != "semantic" {
		t.Errorf("Cascade 0: expected type 'semantic', got '%s'", c1.Type)
	}
	if c1.Reason != "shares type definition with pkg/foo.go" {
		t.Errorf("Cascade 0: unexpected reason: %s", c1.Reason)
	}

	// Verify second cascade
	c2 := out.CascadeCandidates[1]
	if c2.File != "pkg/baz.go" {
		t.Errorf("Cascade 1: expected file 'pkg/baz.go', got '%s'", c2.File)
	}
	if c2.Type != "syntax" {
		t.Errorf("Cascade 1: expected type 'syntax', got '%s'", c2.Type)
	}
}

func TestToOutput_SortsDeterministically(t *testing.T) {
	// Test that multiple nodes with same deps are sorted alphabetically
	g := &DepGraph{
		Nodes: []FileNode{
			{File: "pkg/z.go", WaveCandidate: 1},
			{File: "pkg/a.go", WaveCandidate: 1},
			{File: "pkg/m.go", WaveCandidate: 1},
		},
		Waves: map[int][]string{
			1: {"pkg/z.go", "pkg/a.go", "pkg/m.go"},
		},
	}

	out := ToOutput(g)

	// Nodes should be sorted: a, m, z
	expectedOrder := []string{"pkg/a.go", "pkg/m.go", "pkg/z.go"}
	for i, expected := range expectedOrder {
		if out.Nodes[i].File != expected {
			t.Errorf("Node %d: expected '%s', got '%s'", i, expected, out.Nodes[i].File)
		}
	}

	// Wave file list should also be sorted
	if len(out.Waves[1]) != 3 {
		t.Fatalf("Expected 3 files in wave 1, got %d", len(out.Waves[1]))
	}
	for i, expected := range expectedOrder {
		if out.Waves[1][i] != expected {
			t.Errorf("Wave 1[%d]: expected '%s', got '%s'", i, expected, out.Waves[1][i])
		}
	}
}

func TestFormatYAML_ValidOutput(t *testing.T) {
	out := &Output{
		Nodes: []OutputNode{
			{
				File:          "pkg/foo.go",
				DependsOn:     []string{"pkg/bar.go"},
				DependedBy:    []string{},
				WaveCandidate: 2,
			},
			{
				File:          "pkg/bar.go",
				DependsOn:     []string{},
				DependedBy:    []string{"pkg/foo.go"},
				WaveCandidate: 1,
			},
		},
		Waves: map[int][]string{
			1: {"pkg/bar.go"},
			2: {"pkg/foo.go"},
		},
		CascadeCandidates: []OutputCascade{
			{
				File:   "pkg/baz.go",
				Reason: "test reason",
				Type:   "semantic",
			},
		},
	}

	data, err := FormatYAML(out)
	if err != nil {
		t.Fatalf("FormatYAML failed: %v", err)
	}

	// Verify it's valid YAML
	var parsed Output
	if err := yaml.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Failed to parse generated YAML: %v", err)
	}

	// Verify field names match roadmap spec
	yamlStr := string(data)
	requiredFields := []string{
		"nodes:",
		"file:",
		"depends_on:",
		"depended_by:",
		"wave_candidate:",
		"waves:",
		"cascade_candidates:",
		"reason:",
		"type:",
	}

	for _, field := range requiredFields {
		if !contains(yamlStr, field) {
			t.Errorf("YAML output missing required field: %s", field)
		}
	}

	// Verify parsed structure matches input
	if len(parsed.Nodes) != 2 {
		t.Errorf("Expected 2 nodes in parsed YAML, got %d", len(parsed.Nodes))
	}
	if len(parsed.Waves) != 2 {
		t.Errorf("Expected 2 waves in parsed YAML, got %d", len(parsed.Waves))
	}
	if len(parsed.CascadeCandidates) != 1 {
		t.Errorf("Expected 1 cascade candidate in parsed YAML, got %d", len(parsed.CascadeCandidates))
	}
}

func TestFormatJSON_ValidOutput(t *testing.T) {
	out := &Output{
		Nodes: []OutputNode{
			{
				File:          "pkg/foo.go",
				DependsOn:     []string{"pkg/bar.go"},
				DependedBy:    []string{},
				WaveCandidate: 2,
			},
		},
		Waves: map[int][]string{
			1: {"pkg/bar.go"},
			2: {"pkg/foo.go"},
		},
		CascadeCandidates: []OutputCascade{
			{
				File:   "pkg/baz.go",
				Reason: "test reason",
				Type:   "semantic",
			},
		},
	}

	data, err := FormatJSON(out)
	if err != nil {
		t.Fatalf("FormatJSON failed: %v", err)
	}

	// Verify it's valid JSON
	var parsed Output
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Failed to parse generated JSON: %v", err)
	}

	// Verify field names match roadmap spec
	jsonStr := string(data)
	requiredFields := []string{
		`"nodes"`,
		`"file"`,
		`"depends_on"`,
		`"depended_by"`,
		`"wave_candidate"`,
		`"waves"`,
		`"cascade_candidates"`,
		`"reason"`,
		`"type"`,
	}

	for _, field := range requiredFields {
		if !contains(jsonStr, field) {
			t.Errorf("JSON output missing required field: %s", field)
		}
	}

	// Verify parsed structure matches input
	if len(parsed.Nodes) != 1 {
		t.Errorf("Expected 1 node in parsed JSON, got %d", len(parsed.Nodes))
	}
	if len(parsed.Waves) != 2 {
		t.Errorf("Expected 2 waves in parsed JSON, got %d", len(parsed.Waves))
	}
	if len(parsed.CascadeCandidates) != 1 {
		t.Errorf("Expected 1 cascade candidate in parsed JSON, got %d", len(parsed.CascadeCandidates))
	}

	// Verify JSON is indented (pretty-printed)
	if !contains(jsonStr, "\n") {
		t.Error("JSON output is not indented")
	}
}

func TestFormatYAML_OmitEmptyCascades(t *testing.T) {
	out := &Output{
		Nodes: []OutputNode{
			{
				File:          "pkg/foo.go",
				WaveCandidate: 1,
			},
		},
		Waves: map[int][]string{
			1: {"pkg/foo.go"},
		},
		CascadeCandidates: []OutputCascade{},
	}

	data, err := FormatYAML(out)
	if err != nil {
		t.Fatalf("FormatYAML failed: %v", err)
	}

	// cascade_candidates should be omitted when empty (due to omitempty tag)
	yamlStr := string(data)
	if contains(yamlStr, "cascade_candidates") {
		t.Error("YAML output should omit cascade_candidates when empty")
	}
}

func TestFormatJSON_OmitEmptyCascades(t *testing.T) {
	out := &Output{
		Nodes: []OutputNode{
			{
				File:          "pkg/foo.go",
				WaveCandidate: 1,
			},
		},
		Waves: map[int][]string{
			1: {"pkg/foo.go"},
		},
		CascadeCandidates: []OutputCascade{},
	}

	data, err := FormatJSON(out)
	if err != nil {
		t.Fatalf("FormatJSON failed: %v", err)
	}

	// cascade_candidates should be omitted when empty (due to omitempty tag)
	jsonStr := string(data)
	if contains(jsonStr, "cascade_candidates") {
		t.Error("JSON output should omit cascade_candidates when empty")
	}
}

// Helper function to check if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && containsAt(s, substr))
}

func containsAt(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
