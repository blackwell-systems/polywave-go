package protocol

import (
	"fmt"
	"strings"
)

// ValidateSchema is the top-level schema validation entry point.
// It runs all structural checks (nested required fields, path format)
// and aggregates their results. Called by Validate() in validation.go
// alongside existing semantic checks. Returns warnings with SV01 prefix.
//
// NOTE: In Wave 1, this only calls sub-validators defined in this file.
// Agent F (Wave 2) will wire in validateAllEnums and validateCrossFieldConsistency.
func ValidateSchema(m *IMPLManifest) []ValidationError {
	var errs []ValidationError
	errs = append(errs, validateNestedRequiredFields(m)...)
	errs = append(errs, validateFilePaths(m)...)
	return errs
}

// validateNestedRequiredFields checks required fields on nested types:
// FileOwnership, Wave, Agent, InterfaceContract, QualityGate, ScaffoldFile, PreMortemRow.
func validateNestedRequiredFields(m *IMPLManifest) []ValidationError {
	var errs []ValidationError

	// FileOwnership: File (non-empty), Agent (non-empty), Wave (> 0)
	for i, fo := range m.FileOwnership {
		if strings.TrimSpace(fo.File) == "" {
			errs = append(errs, ValidationError{
				Code:    SV01RequiredField,
				Message: fmt.Sprintf("file_ownership[%d].file is required and must be non-empty", i),
				Field:   fmt.Sprintf("file_ownership[%d].file", i),
			})
		}
		if strings.TrimSpace(fo.Agent) == "" {
			errs = append(errs, ValidationError{
				Code:    SV01RequiredField,
				Message: fmt.Sprintf("file_ownership[%d].agent is required and must be non-empty", i),
				Field:   fmt.Sprintf("file_ownership[%d].agent", i),
			})
		}
		if fo.Wave <= 0 {
			errs = append(errs, ValidationError{
				Code:    SV01RequiredField,
				Message: fmt.Sprintf("file_ownership[%d].wave must be > 0, got %d", i, fo.Wave),
				Field:   fmt.Sprintf("file_ownership[%d].wave", i),
			})
		}
	}

	// Waves: Number (> 0), Agents (non-empty slice)
	for i, w := range m.Waves {
		if w.Number <= 0 {
			errs = append(errs, ValidationError{
				Code:    SV01RequiredField,
				Message: fmt.Sprintf("waves[%d].number must be > 0, got %d", i, w.Number),
				Field:   fmt.Sprintf("waves[%d].number", i),
			})
		}
		if len(w.Agents) == 0 {
			errs = append(errs, ValidationError{
				Code:    SV01RequiredField,
				Message: fmt.Sprintf("waves[%d].agents must be non-empty", i),
				Field:   fmt.Sprintf("waves[%d].agents", i),
			})
		}

		// Agent: ID (non-empty), Task (non-empty)
		for j, agent := range w.Agents {
			if strings.TrimSpace(agent.ID) == "" {
				errs = append(errs, ValidationError{
					Code:    SV01RequiredField,
					Message: fmt.Sprintf("waves[%d].agents[%d].id is required and must be non-empty", i, j),
					Field:   fmt.Sprintf("waves[%d].agents[%d].id", i, j),
				})
			}
			if strings.TrimSpace(agent.Task) == "" {
				errs = append(errs, ValidationError{
					Code:    SV01RequiredField,
					Message: fmt.Sprintf("waves[%d].agents[%d].task is required and must be non-empty", i, j),
					Field:   fmt.Sprintf("waves[%d].agents[%d].task", i, j),
				})
			}
		}
	}

	// InterfaceContracts: Name (non-empty), Definition (non-empty), Location (non-empty)
	for i, ic := range m.InterfaceContracts {
		if strings.TrimSpace(ic.Name) == "" {
			errs = append(errs, ValidationError{
				Code:    SV01RequiredField,
				Message: fmt.Sprintf("interface_contracts[%d].name is required and must be non-empty", i),
				Field:   fmt.Sprintf("interface_contracts[%d].name", i),
			})
		}
		if strings.TrimSpace(ic.Definition) == "" {
			errs = append(errs, ValidationError{
				Code:    SV01RequiredField,
				Message: fmt.Sprintf("interface_contracts[%d].definition is required and must be non-empty", i),
				Field:   fmt.Sprintf("interface_contracts[%d].definition", i),
			})
		}
		if strings.TrimSpace(ic.Location) == "" {
			errs = append(errs, ValidationError{
				Code:    SV01RequiredField,
				Message: fmt.Sprintf("interface_contracts[%d].location is required and must be non-empty", i),
				Field:   fmt.Sprintf("interface_contracts[%d].location", i),
			})
		}
	}

	// QualityGates: Type (non-empty), Command (non-empty)
	if m.QualityGates != nil {
		for i, gate := range m.QualityGates.Gates {
			if strings.TrimSpace(gate.Type) == "" {
				errs = append(errs, ValidationError{
					Code:    SV01RequiredField,
					Message: fmt.Sprintf("quality_gates.gates[%d].type is required and must be non-empty", i),
					Field:   fmt.Sprintf("quality_gates.gates[%d].type", i),
				})
			}
			if strings.TrimSpace(gate.Command) == "" {
				errs = append(errs, ValidationError{
					Code:    SV01RequiredField,
					Message: fmt.Sprintf("quality_gates.gates[%d].command is required and must be non-empty", i),
					Field:   fmt.Sprintf("quality_gates.gates[%d].command", i),
				})
			}
		}
	}

	// ScaffoldFiles: FilePath (non-empty)
	for i, sf := range m.Scaffolds {
		if strings.TrimSpace(sf.FilePath) == "" {
			errs = append(errs, ValidationError{
				Code:    SV01RequiredField,
				Message: fmt.Sprintf("scaffolds[%d].file_path is required and must be non-empty", i),
				Field:   fmt.Sprintf("scaffolds[%d].file_path", i),
			})
		}
	}

	// PreMortemRows: Scenario (non-empty), Mitigation (non-empty)
	if m.PreMortem != nil {
		for i, row := range m.PreMortem.Rows {
			if strings.TrimSpace(row.Scenario) == "" {
				errs = append(errs, ValidationError{
					Code:    SV01RequiredField,
					Message: fmt.Sprintf("pre_mortem.rows[%d].scenario is required and must be non-empty", i),
					Field:   fmt.Sprintf("pre_mortem.rows[%d].scenario", i),
				})
			}
			if strings.TrimSpace(row.Mitigation) == "" {
				errs = append(errs, ValidationError{
					Code:    SV01RequiredField,
					Message: fmt.Sprintf("pre_mortem.rows[%d].mitigation is required and must be non-empty", i),
					Field:   fmt.Sprintf("pre_mortem.rows[%d].mitigation", i),
				})
			}
		}
	}

	return errs
}

// validateFilePaths validates file path format in FileOwnership, Agent.Files,
// and ScaffoldFile.FilePath. Checks:
// - No leading "/" (relative paths only)
// - No ".." path traversal segments
// - No null bytes
// - No backslash characters (use forward slash)
func validateFilePaths(m *IMPLManifest) []ValidationError {
	var errs []ValidationError

	// Helper to check a single path
	checkPath := func(path, fieldPath string) {
		if strings.TrimSpace(path) == "" {
			return // Empty paths are caught by required field validation
		}
		if strings.HasPrefix(path, "/") {
			errs = append(errs, ValidationError{
				Code:    SV01InvalidPath,
				Message: fmt.Sprintf("%s: path %q must not start with '/' (use relative paths)", fieldPath, path),
				Field:   fieldPath,
			})
		}
		if containsDotDot(path) {
			errs = append(errs, ValidationError{
				Code:    SV01InvalidPath,
				Message: fmt.Sprintf("%s: path %q must not contain '..' traversal segments", fieldPath, path),
				Field:   fieldPath,
			})
		}
		if strings.ContainsRune(path, 0) {
			errs = append(errs, ValidationError{
				Code:    SV01InvalidPath,
				Message: fmt.Sprintf("%s: path must not contain null bytes", fieldPath),
				Field:   fieldPath,
			})
		}
		if strings.Contains(path, "\\") {
			errs = append(errs, ValidationError{
				Code:    SV01InvalidPath,
				Message: fmt.Sprintf("%s: path %q must not contain backslashes (use forward slashes)", fieldPath, path),
				Field:   fieldPath,
			})
		}
	}

	// FileOwnership.File
	for i, fo := range m.FileOwnership {
		checkPath(fo.File, fmt.Sprintf("file_ownership[%d].file", i))
	}

	// Agent.Files
	for i, w := range m.Waves {
		for j, agent := range w.Agents {
			for k, file := range agent.Files {
				checkPath(file, fmt.Sprintf("waves[%d].agents[%d].files[%d]", i, j, k))
			}
		}
	}

	// ScaffoldFile.FilePath
	for i, sf := range m.Scaffolds {
		checkPath(sf.FilePath, fmt.Sprintf("scaffolds[%d].file_path", i))
	}

	return errs
}

// containsDotDot checks whether a path contains ".." as a path segment.
// It matches ".." at the start, end, or middle of a path (e.g., "../foo", "foo/../bar", "foo/..").
// It does NOT match "..." or other strings that merely contain two dots.
func containsDotDot(path string) bool {
	parts := strings.Split(path, "/")
	for _, part := range parts {
		if part == ".." {
			return true
		}
	}
	return false
}
