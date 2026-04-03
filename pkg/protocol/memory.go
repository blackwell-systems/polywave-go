package protocol

import (
	"context"
	"fmt"
	"os"

	"gopkg.in/yaml.v3"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/result"
)

// SaveMemoryData holds the result payload for a successful SaveProjectMemory call.
type SaveMemoryData struct {
	MemoryPath   string
	BytesWritten int
}

// ProjectMemory represents the contents of docs/CONTEXT.md in structured YAML format.
// It serves as project memory for Scout-and-Wave protocol implementations.
type ProjectMemory struct {
	Created              string                  `yaml:"created"`
	ProtocolVersion      string                  `yaml:"protocol_version"`
	Architecture         ArchitectureDescription `yaml:"architecture,omitempty"`
	Decisions            []Decision              `yaml:"decisions,omitempty"`
	Conventions          Conventions             `yaml:"conventions,omitempty"`
	EstablishedInterfaces []EstablishedInterface `yaml:"established_interfaces,omitempty"`
	FeaturesCompleted    []CompletedFeature      `yaml:"features_completed,omitempty"`
}

// ArchitectureModule describes a single module within the architecture.
type ArchitectureModule struct {
	Name           string `yaml:"name" json:"name"`
	Path           string `yaml:"path" json:"path"`
	Responsibility string `yaml:"responsibility" json:"responsibility"`
}

// ArchitectureDescription captures the high-level architecture of the codebase.
type ArchitectureDescription struct {
	Language    string               `yaml:"language"`
	Stack       []string             `yaml:"stack,omitempty"`
	Summary     string               `yaml:"summary,omitempty"`
	Description string               `yaml:"description,omitempty"`
	Modules     []ArchitectureModule `yaml:"modules,omitempty"`
}

// Decision records an architectural or implementation decision.
type Decision struct {
	Date        string `yaml:"date"`
	Description string `yaml:"description"`
	Rationale   string `yaml:"rationale,omitempty"`
}

// Conventions captures coding conventions and tooling preferences.
type Conventions struct {
	TestFramework string `yaml:"test_framework,omitempty"`
	LintTool      string `yaml:"lint_tool,omitempty"`
}

// EstablishedInterface documents a stable interface that other code depends on.
type EstablishedInterface struct {
	Name       string `yaml:"name"`
	FilePath   string `yaml:"file_path"`
	ImportPath string `yaml:"import_path,omitempty"`
}

// CompletedFeature records a completed feature and its implementation metadata.
type CompletedFeature struct {
	Slug      string `yaml:"slug"`
	IMPLDoc   string `yaml:"impl_doc"`
	WaveCount int    `yaml:"wave_count"`
	AgentCount int   `yaml:"agent_count"`
	Date      string `yaml:"date"`
}

// LoadProjectMemory reads a YAML project memory file from the specified path and parses it into a ProjectMemory.
// Returns a fatal result if the context is cancelled, the file cannot be read, or the YAML is invalid.
func LoadProjectMemory(ctx context.Context, path string) result.Result[*ProjectMemory] {
	if err := ctx.Err(); err != nil {
		return result.NewFailure[*ProjectMemory]([]result.SAWError{
			{Code: result.CodeContextCancelled, Message: "context cancelled", Severity: "fatal"},
		})
	}
	pm, err := LoadYAML[ProjectMemory](path)
	if err != nil {
		return result.NewFailure[*ProjectMemory]([]result.SAWError{
			result.NewFatal("MEMORY_LOAD_FAILED", fmt.Sprintf("failed to read project memory file: %v", err)),
		})
	}
	return result.NewSuccess(&pm)
}

// SaveProjectMemory writes a ProjectMemory to the specified path as YAML.
// Returns a Fatal result if the file cannot be written or the memory cannot be marshaled.
func SaveProjectMemory(path string, pm *ProjectMemory) result.Result[SaveMemoryData] {
	data, err := yaml.Marshal(pm)
	if err != nil {
		return result.NewFailure[SaveMemoryData]([]result.SAWError{
			result.NewFatal("MEMORY_SAVE_FAILED", fmt.Sprintf("failed to marshal project memory: %v", err)),
		})
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		return result.NewFailure[SaveMemoryData]([]result.SAWError{
			result.NewFatal("MEMORY_SAVE_FAILED", fmt.Sprintf("failed to write project memory file: %v", err)),
		})
	}
	return result.NewSuccess(SaveMemoryData{
		MemoryPath:   path,
		BytesWritten: len(data),
	})
}

// AddCompletedFeature appends a feature to the memory's completed features list.
func AddCompletedFeature(pm *ProjectMemory, feature CompletedFeature) {
	pm.FeaturesCompleted = append(pm.FeaturesCompleted, feature)
}
