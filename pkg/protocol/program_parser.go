package protocol

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// ParseProgramManifest reads a YAML PROGRAM manifest from the specified path
// and parses it into a PROGRAMManifest struct.
// Returns an error if the file cannot be read or the YAML is invalid.
//
// This function follows the pattern of Load() in manifest.go for IMPL documents.
func ParseProgramManifest(path string) (*PROGRAMManifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read PROGRAM manifest file: %w", err)
	}

	var manifest PROGRAMManifest
	if err := yaml.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("failed to parse PROGRAM manifest YAML: %w", err)
	}

	return &manifest, nil
}
