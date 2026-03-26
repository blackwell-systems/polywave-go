package analyzer

import (
	"encoding/json"
	"sort"

	"gopkg.in/yaml.v3"
)

// Output is the CLI output struct matching roadmap YAML schema.
type Output struct {
	Nodes             []OutputNode     `yaml:"nodes" json:"nodes"`
	Waves             map[int][]string `yaml:"waves" json:"waves"`
	CascadeCandidates []OutputCascade  `yaml:"cascade_candidates,omitempty" json:"cascade_candidates,omitempty"`
}

// OutputNode matches roadmap spec node structure.
type OutputNode struct {
	File          string   `yaml:"file" json:"file"`
	DependsOn     []string `yaml:"depends_on" json:"depends_on"`
	DependedBy    []string `yaml:"depended_by" json:"depended_by"`
	WaveCandidate int      `yaml:"wave_candidate" json:"wave_candidate"`
}

// OutputCascade matches roadmap spec cascade structure.
type OutputCascade struct {
	File   string `yaml:"file" json:"file"`
	Reason string `yaml:"reason" json:"reason"`
	Type   string `yaml:"type" json:"type"`
}

// ToOutput converts DepGraph to Output struct.
// Maps FileNode -> OutputNode, Waves as-is, CascadeFile -> OutputCascade.
// Sorts nodes by file path and wave entries alphabetically for determinism.
func ToOutput(g *DepGraph) *Output {
	if g == nil {
		return &Output{
			Nodes: []OutputNode{},
			Waves: map[int][]string{},
		}
	}

	// Convert FileNode -> OutputNode
	nodes := make([]OutputNode, 0, len(g.Nodes))
	for _, n := range g.Nodes {
		nodes = append(nodes, OutputNode{
			File:          n.File,
			DependsOn:     n.DependsOn,
			DependedBy:    n.DependedBy,
			WaveCandidate: n.WaveCandidate,
		})
	}

	// Sort nodes by file path for determinism
	sort.Slice(nodes, func(i, j int) bool {
		return nodes[i].File < nodes[j].File
	})

	// Copy waves map, sorting each file list alphabetically
	waves := make(map[int][]string)
	for waveNum, files := range g.Waves {
		sortedFiles := make([]string, len(files))
		copy(sortedFiles, files)
		sort.Strings(sortedFiles)
		waves[waveNum] = sortedFiles
	}

	// Convert CascadeFile -> OutputCascade
	cascades := make([]OutputCascade, 0, len(g.CascadeCandidates))
	for _, c := range g.CascadeCandidates {
		cascades = append(cascades, OutputCascade{
			File:   c.File,
			Reason: c.Reason,
			Type:   c.Type,
		})
	}

	return &Output{
		Nodes:             nodes,
		Waves:             waves,
		CascadeCandidates: cascades,
	}
}

// FormatYAML serializes Output to YAML bytes.
// Cannot use protocol.SaveYAML: marshals to []byte for the caller, not to a file path.
func FormatYAML(o *Output) ([]byte, error) {
	return yaml.Marshal(o)
}

// FormatJSON serializes Output to indented JSON bytes.
func FormatJSON(o *Output) ([]byte, error) {
	return json.MarshalIndent(o, "", "  ")
}
