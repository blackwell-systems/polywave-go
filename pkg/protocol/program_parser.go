package protocol

import (
	"bytes"
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// ParseProgramManifest reads a YAML PROGRAM manifest from the specified path
// and parses it into a PROGRAMManifest struct.
// Returns an error if the file cannot be read or the YAML is invalid.
//
// This function follows the pattern of Load() in manifest.go for IMPL documents.
// The SAW:PROGRAM:COMPLETE marker (a non-YAML line appended by mark-program-complete)
// is stripped before parsing so completed manifests remain readable.
func ParseProgramManifest(path string) (*PROGRAMManifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read PROGRAM manifest file: %w", err)
	}

	// Strip the Polywave:PROGRAM:COMPLETE marker line before YAML parsing.
	// mark-program-complete appends this as a bare non-YAML string; the YAML
	// parser rejects it with "could not find expected ':'".
	data = bytes.ReplaceAll(data, []byte("\nSAW:PROGRAM:COMPLETE\n"), []byte("\n"))
	data = bytes.TrimSuffix(data, []byte("\nSAW:PROGRAM:COMPLETE"))

	// Cannot use LoadYAML: data has been pre-processed above to strip the Polywave:PROGRAM:COMPLETE
	// marker line before YAML parsing. LoadYAML reads raw file bytes without that transformation.
	var manifest PROGRAMManifest
	if err := yaml.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("failed to parse PROGRAM manifest YAML: %w", err)
	}

	return &manifest, nil
}
