package protocol

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// LoadYAML reads the file at path and unmarshals YAML into a value of type T.
// Returns a wrapped error if reading or parsing fails.
//
// Do not use this for IMPLManifest — use Load() instead, which has special
// duplicate-key detection logic that this helper omits.
func LoadYAML[T any](path string) (T, error) {
	var zero T
	data, err := os.ReadFile(path)
	if err != nil {
		return zero, fmt.Errorf("LoadYAML: read %s: %w", path, err)
	}
	var v T
	if err := yaml.Unmarshal(data, &v); err != nil {
		return zero, fmt.Errorf("LoadYAML: parse %s: %w", path, err)
	}
	return v, nil
}

// SaveYAML marshals v to YAML and writes it to path with permissions 0644.
// Returns a wrapped error if marshaling or writing fails.
func SaveYAML[T any](path string, v T) error {
	data, err := yaml.Marshal(v)
	if err != nil {
		return fmt.Errorf("SaveYAML: marshal: %w", err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("SaveYAML: write %s: %w", path, err)
	}
	return nil
}
