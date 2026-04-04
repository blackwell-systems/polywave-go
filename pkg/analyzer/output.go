package analyzer

import (
	"encoding/json"
	"sort"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/result"
	"gopkg.in/yaml.v3"
)

// Output is the CLI output struct matching roadmap YAML schema.
type Output struct {
	Nodes             []OutputNode       `yaml:"nodes" json:"nodes"`
	Waves             map[int][]string   `yaml:"waves" json:"waves"`
	CascadeCandidates []CascadeCandidate `yaml:"cascade_candidates,omitempty" json:"cascade_candidates,omitempty"`
}

// OutputNode matches roadmap spec node structure.
type OutputNode struct {
	File          string   `yaml:"file" json:"file"`
	DependsOn     []string `yaml:"depends_on" json:"depends_on"`
	DependedBy    []string `yaml:"depended_by" json:"depended_by"`
	WaveCandidate int      `yaml:"wave_candidate" json:"wave_candidate"`
}

// ToOutput converts DepGraph to Output struct.
// Maps FileNode -> OutputNode, Waves as-is, CascadeCandidate -> CascadeCandidate (direct copy).
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

	// Copy CascadeCandidates directly (already []CascadeCandidate)
	cascades := make([]CascadeCandidate, len(g.CascadeCandidates))
	copy(cascades, g.CascadeCandidates)

	return &Output{
		Nodes:             nodes,
		Waves:             waves,
		CascadeCandidates: cascades,
	}
}

// FormatYAML serializes Output to YAML bytes.
// Cannot use protocol.SaveYAML: marshals to []byte for the caller, not to a file path.
func FormatYAML(o *Output) result.Result[[]byte] {
	data, err := yaml.Marshal(o)
	if err != nil {
		return result.NewFailure[[]byte]([]result.SAWError{result.NewFatal(result.CodeAnalyzeParseFailed, err.Error())})
	}
	return result.NewSuccess(data)
}

// FormatJSON serializes Output to indented JSON bytes.
func FormatJSON(o *Output) result.Result[[]byte] {
	data, err := json.MarshalIndent(o, "", "  ")
	if err != nil {
		return result.NewFailure[[]byte]([]result.SAWError{result.NewFatal(result.CodeAnalyzeParseFailed, err.Error())})
	}
	return result.NewSuccess(data)
}
