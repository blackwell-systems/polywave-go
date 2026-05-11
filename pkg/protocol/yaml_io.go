package protocol

// Direct yaml.* calls outside this file are intentional exemptions.
// Every such call must appear in the list below with its reason.
// Any new direct yaml.* call NOT in this list is a bug — use LoadYAML or SaveYAML.
//
// Exempt call sites (yaml.Node API — cannot use generic helpers):
//
//	pkg/protocol/integration.go          — AppendIntegrationReport: yaml.Node tree splice
//	pkg/protocol/wiring_validation.go    — AppendWiringValidationReport: yaml.Node tree splice
//	pkg/protocol/schema_unknown_keys.go  — StripUnknownKeys: yaml.Node tree traversal
//	pkg/protocol/duplicate_key_validator.go — ValidateDuplicateKeys: yaml.Node scan
//	pkg/protocol/marker.go               — SAW:COMPLETE marker detection via yaml.Node
//	cmd/polywave-tools/validate_integration.go — raw yaml.Node manipulation
//
// Exempt call sites (encoder-to-stdout — no file path; yaml.NewEncoder target is os.Stdout):
//
//	cmd/polywave-tools/validate_scaffold_cmd.go       — yaml.NewEncoder(cmd.OutOrStdout())
//	cmd/polywave-tools/diagnose_build_failure_cmd.go  — yaml.NewEncoder(cmd.OutOrStdout())
//
// Exempt call sites (in-memory / no file path):
//
//	pkg/engine/runner.go                  — marshal to []byte for inline string embed
//	pkg/protocol/solver_integration.go    — deep copy via marshal+unmarshal roundtrip
//	pkg/protocol/program_parser.go        — pre-processed bytes (SAW:PROGRAM:COMPLETE stripped)
//	pkg/protocol/program_status.go        — swallows errors; anonymous struct
//	pkg/protocol/validation.go            — ValidateBytes takes []byte, not file path
//	pkg/queue/manager.go                  — unmarshals already-read bytes from dir scan
//	pkg/commands/github_actions.go        — unmarshals already-read bytes from caller
//	pkg/analyzer/output.go                — marshal to []byte as return value
//	cmd/polywave-tools/detect_cascades_cmd.go   — marshal to []byte for stdout print
//	cmd/polywave-tools/extract_commands_cmd.go  — marshal to []byte for stdout print

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
