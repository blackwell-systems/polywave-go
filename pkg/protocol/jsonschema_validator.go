package protocol

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/invopop/jsonschema"
)

// JS01 error code prefix distinguishes JSON Schema validation errors from
// existing SV01_ hand-coded validation errors.
const (
	JS01RequiredField = "JS01_REQUIRED_FIELD"
	JS01InvalidEnum   = "JS01_INVALID_ENUM"
	JS01InvalidType   = "JS01_INVALID_TYPE"
)

// manifestSchemaCache caches the generated manifest schema after the first call.
var (
	manifestSchemaOnce  sync.Once
	manifestSchemaCached map[string]any
	manifestSchemaErr   error
)

// GenerateManifestSchema generates a JSON Schema for the full IMPLManifest struct
// using github.com/invopop/jsonschema reflection. The schema is generated once and
// cached for all subsequent calls.
//
// Returns a map[string]any representing the JSON Schema object.
func GenerateManifestSchema() (map[string]any, error) {
	manifestSchemaOnce.Do(func() {
		r := &jsonschema.Reflector{
			AllowAdditionalProperties: false,
			DoNotReference:            true,
		}
		schema := r.Reflect(&IMPLManifest{})

		data, err := json.Marshal(schema)
		if err != nil {
			manifestSchemaErr = fmt.Errorf("failed to marshal manifest schema: %w", err)
			return
		}

		var result map[string]any
		if err := json.Unmarshal(data, &result); err != nil {
			manifestSchemaErr = fmt.Errorf("failed to unmarshal manifest schema: %w", err)
			return
		}

		manifestSchemaCached = result
	})

	return manifestSchemaCached, manifestSchemaErr
}

// ValidateManifestJSON validates an IMPLManifest against the JSON Schema derived
// from the IMPLManifest struct. It performs structural validation including:
//   - Required top-level fields (title, feature_slug, test_command, lint_command)
//   - Enum value correctness for verdict, state, merge_state
//   - Type correctness for slice/string fields
//
// This is an additive validation path that complements existing ValidateSchema()
// checks. Error codes use the "JS01_" prefix.
//
// Returns a (possibly empty) slice of ValidationErrors.
func ValidateManifestJSON(m *IMPLManifest) []ValidationError {
	var errs []ValidationError

	// Marshal to JSON for schema-based field inspection.
	data, err := json.Marshal(m)
	if err != nil {
		errs = append(errs, ValidationError{
			Code:    JS01RequiredField,
			Message: fmt.Sprintf("failed to marshal manifest to JSON: %v", err),
			Field:   "",
		})
		return errs
	}

	// Decode into a generic map for inspection.
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		errs = append(errs, ValidationError{
			Code:    JS01RequiredField,
			Message: fmt.Sprintf("failed to decode manifest JSON: %v", err),
			Field:   "",
		})
		return errs
	}

	// --- Required top-level string fields ---
	requiredStringFields := []string{
		"title",
		"feature_slug",
		"test_command",
		"lint_command",
	}
	for _, field := range requiredStringFields {
		val, exists := raw[field]
		if !exists || val == nil {
			errs = append(errs, ValidationError{
				Code:    JS01RequiredField,
				Message: fmt.Sprintf("required field %q is missing", field),
				Field:   field,
			})
			continue
		}
		str, ok := val.(string)
		if !ok {
			errs = append(errs, ValidationError{
				Code:    JS01InvalidType,
				Message: fmt.Sprintf("field %q must be a string, got %T", field, val),
				Field:   field,
			})
			continue
		}
		if strings.TrimSpace(str) == "" {
			errs = append(errs, ValidationError{
				Code:    JS01RequiredField,
				Message: fmt.Sprintf("required field %q must not be empty", field),
				Field:   field,
			})
		}
	}

	// --- Required array fields ---
	requiredArrayFields := []string{
		"file_ownership",
		"interface_contracts",
		"waves",
	}
	for _, field := range requiredArrayFields {
		val, exists := raw[field]
		if !exists || val == nil {
			errs = append(errs, ValidationError{
				Code:    JS01RequiredField,
				Message: fmt.Sprintf("required field %q is missing", field),
				Field:   field,
			})
			continue
		}
		if _, ok := val.([]any); !ok {
			errs = append(errs, ValidationError{
				Code:    JS01InvalidType,
				Message: fmt.Sprintf("field %q must be an array, got %T", field, val),
				Field:   field,
			})
		}
	}

	// --- Enum: verdict ---
	errs = append(errs, validateJSEnum(raw, "verdict", []string{
		"SUITABLE",
		"NOT_SUITABLE",
		"SUITABLE_WITH_CAVEATS",
	}, false)...)

	// --- Enum: state (optional) ---
	errs = append(errs, validateJSEnum(raw, "state", []string{
		string(StateScoutPending),
		string(StateScoutValidating),
		string(StateReviewed),
		string(StateScaffoldPending),
		string(StateWavePending),
		string(StateWaveExecuting),
		string(StateWaveMerging),
		string(StateWaveVerified),
		string(StateBlocked),
		string(StateComplete),
		string(StateNotSuitable),
	}, true)...)

	// --- Enum: merge_state (optional) ---
	errs = append(errs, validateJSEnum(raw, "merge_state", []string{
		string(MergeStateIdle),
		string(MergeStateInProgress),
		string(MergeStateCompleted),
		string(MergeStateFailed),
	}, true)...)

	// --- Nested: waves array ---
	errs = append(errs, validateJSWaves(raw)...)

	// --- Nested: file_ownership array ---
	errs = append(errs, validateJSFileOwnership(raw)...)

	// --- Nested: quality_gates (optional) ---
	errs = append(errs, validateJSQualityGates(raw)...)

	return errs
}

// validateJSEnum checks that a field (if present and non-empty) is one of the allowed enum values.
// If optional=true, absent or empty string values are skipped.
func validateJSEnum(raw map[string]any, field string, allowed []string, optional bool) []ValidationError {
	var errs []ValidationError

	val, exists := raw[field]
	if !exists || val == nil {
		if !optional {
			errs = append(errs, ValidationError{
				Code:    JS01RequiredField,
				Message: fmt.Sprintf("required field %q is missing", field),
				Field:   field,
			})
		}
		return errs
	}

	str, ok := val.(string)
	if !ok {
		errs = append(errs, ValidationError{
			Code:    JS01InvalidType,
			Message: fmt.Sprintf("field %q must be a string, got %T", field, val),
			Field:   field,
		})
		return errs
	}

	// Empty string is OK for optional fields.
	if str == "" && optional {
		return errs
	}

	// Empty string for required enum fields is a missing value.
	if str == "" && !optional {
		errs = append(errs, ValidationError{
			Code:    JS01RequiredField,
			Message: fmt.Sprintf("required field %q must not be empty", field),
			Field:   field,
		})
		return errs
	}

	allowedSet := make(map[string]bool, len(allowed))
	for _, a := range allowed {
		allowedSet[a] = true
	}

	if !allowedSet[str] {
		errs = append(errs, ValidationError{
			Code:    JS01InvalidEnum,
			Message: fmt.Sprintf("field %q has invalid value %q — must be one of: %s", field, str, strings.Join(allowed, ", ")),
			Field:   field,
		})
	}

	return errs
}

// validateJSWaves validates the structure of each wave and agent within the waves array.
func validateJSWaves(raw map[string]any) []ValidationError {
	var errs []ValidationError

	wavesVal, ok := raw["waves"]
	if !ok || wavesVal == nil {
		return errs // already caught by required field check
	}

	waves, ok := wavesVal.([]any)
	if !ok {
		return errs // already caught by type check
	}

	for i, waveVal := range waves {
		wave, ok := waveVal.(map[string]any)
		if !ok {
			errs = append(errs, ValidationError{
				Code:    JS01InvalidType,
				Message: fmt.Sprintf("waves[%d] must be an object", i),
				Field:   fmt.Sprintf("waves[%d]", i),
			})
			continue
		}

		// waves[i].number must be a positive integer
		if numVal, exists := wave["number"]; exists && numVal != nil {
			switch n := numVal.(type) {
			case float64:
				if n <= 0 {
					errs = append(errs, ValidationError{
						Code:    JS01RequiredField,
						Message: fmt.Sprintf("waves[%d].number must be > 0, got %v", i, n),
						Field:   fmt.Sprintf("waves[%d].number", i),
					})
				}
			default:
				errs = append(errs, ValidationError{
					Code:    JS01InvalidType,
					Message: fmt.Sprintf("waves[%d].number must be a number, got %T", i, numVal),
					Field:   fmt.Sprintf("waves[%d].number", i),
				})
			}
		}

		// waves[i].agents must be a non-empty array
		agentsVal, agentsExist := wave["agents"]
		if !agentsExist || agentsVal == nil {
			errs = append(errs, ValidationError{
				Code:    JS01RequiredField,
				Message: fmt.Sprintf("waves[%d].agents is required", i),
				Field:   fmt.Sprintf("waves[%d].agents", i),
			})
			continue
		}

		agents, ok := agentsVal.([]any)
		if !ok {
			errs = append(errs, ValidationError{
				Code:    JS01InvalidType,
				Message: fmt.Sprintf("waves[%d].agents must be an array, got %T", i, agentsVal),
				Field:   fmt.Sprintf("waves[%d].agents", i),
			})
			continue
		}

		// Validate each agent in the wave
		for j, agentVal := range agents {
			agent, ok := agentVal.(map[string]any)
			if !ok {
				errs = append(errs, ValidationError{
					Code:    JS01InvalidType,
					Message: fmt.Sprintf("waves[%d].agents[%d] must be an object", i, j),
					Field:   fmt.Sprintf("waves[%d].agents[%d]", i, j),
				})
				continue
			}

			// agent.id must be a non-empty string
			errs = append(errs, validateJSStringField(agent,
				fmt.Sprintf("waves[%d].agents[%d]", i, j), "id", true)...)

			// agent.task must be a non-empty string
			errs = append(errs, validateJSStringField(agent,
				fmt.Sprintf("waves[%d].agents[%d]", i, j), "task", true)...)
		}
	}

	return errs
}

// validateJSFileOwnership validates the structure of each file_ownership entry.
func validateJSFileOwnership(raw map[string]any) []ValidationError {
	var errs []ValidationError

	foVal, ok := raw["file_ownership"]
	if !ok || foVal == nil {
		return errs // already caught by required field check
	}

	foArr, ok := foVal.([]any)
	if !ok {
		return errs // already caught by type check
	}

	for i, entryVal := range foArr {
		entry, ok := entryVal.(map[string]any)
		if !ok {
			errs = append(errs, ValidationError{
				Code:    JS01InvalidType,
				Message: fmt.Sprintf("file_ownership[%d] must be an object", i),
				Field:   fmt.Sprintf("file_ownership[%d]", i),
			})
			continue
		}

		prefix := fmt.Sprintf("file_ownership[%d]", i)
		errs = append(errs, validateJSStringField(entry, prefix, "file", true)...)
		errs = append(errs, validateJSStringField(entry, prefix, "agent", true)...)

		// wave must be a positive integer
		if waveVal, exists := entry["wave"]; exists && waveVal != nil {
			switch w := waveVal.(type) {
			case float64:
				if w <= 0 {
					errs = append(errs, ValidationError{
						Code:    JS01RequiredField,
						Message: fmt.Sprintf("%s.wave must be > 0, got %v", prefix, w),
						Field:   fmt.Sprintf("%s.wave", prefix),
					})
				}
			default:
				errs = append(errs, ValidationError{
					Code:    JS01InvalidType,
					Message: fmt.Sprintf("%s.wave must be a number, got %T", prefix, waveVal),
					Field:   fmt.Sprintf("%s.wave", prefix),
				})
			}
		}
	}

	return errs
}

// validateJSQualityGates validates the quality_gates object if present.
func validateJSQualityGates(raw map[string]any) []ValidationError {
	var errs []ValidationError

	qgVal, exists := raw["quality_gates"]
	if !exists || qgVal == nil {
		return errs // optional
	}

	qg, ok := qgVal.(map[string]any)
	if !ok {
		errs = append(errs, ValidationError{
			Code:    JS01InvalidType,
			Message: fmt.Sprintf("quality_gates must be an object, got %T", qgVal),
			Field:   "quality_gates",
		})
		return errs
	}

	// Validate gates array
	gatesVal, exists := qg["gates"]
	if !exists || gatesVal == nil {
		return errs
	}

	gates, ok := gatesVal.([]any)
	if !ok {
		errs = append(errs, ValidationError{
			Code:    JS01InvalidType,
			Message: fmt.Sprintf("quality_gates.gates must be an array, got %T", gatesVal),
			Field:   "quality_gates.gates",
		})
		return errs
	}

	validGateTypes := []string{"build", "lint", "test", "typecheck", "format", "custom"}
	for i, gateVal := range gates {
		gate, ok := gateVal.(map[string]any)
		if !ok {
			errs = append(errs, ValidationError{
				Code:    JS01InvalidType,
				Message: fmt.Sprintf("quality_gates.gates[%d] must be an object", i),
				Field:   fmt.Sprintf("quality_gates.gates[%d]", i),
			})
			continue
		}

		prefix := fmt.Sprintf("quality_gates.gates[%d]", i)
		errs = append(errs, validateJSStringField(gate, prefix, "type", true)...)
		errs = append(errs, validateJSStringField(gate, prefix, "command", true)...)

		// Validate gate type enum if present and non-empty.
		if typeVal, exists := gate["type"]; exists && typeVal != nil {
			if typeStr, ok := typeVal.(string); ok && typeStr != "" {
				errs = append(errs, validateJSEnum(gate, "type", validGateTypes, false)...)
			}
		}
	}

	return errs
}

// validateJSStringField checks that a named string field within an object map
// is present (if required) and is a non-empty string.
func validateJSStringField(obj map[string]any, prefix, field string, required bool) []ValidationError {
	var errs []ValidationError

	fullField := fmt.Sprintf("%s.%s", prefix, field)
	val, exists := obj[field]

	if !exists || val == nil {
		if required {
			errs = append(errs, ValidationError{
				Code:    JS01RequiredField,
				Message: fmt.Sprintf("%s is required", fullField),
				Field:   fullField,
			})
		}
		return errs
	}

	str, ok := val.(string)
	if !ok {
		errs = append(errs, ValidationError{
			Code:    JS01InvalidType,
			Message: fmt.Sprintf("%s must be a string, got %T", fullField, val),
			Field:   fullField,
		})
		return errs
	}

	if required && strings.TrimSpace(str) == "" {
		errs = append(errs, ValidationError{
			Code:    JS01RequiredField,
			Message: fmt.Sprintf("%s must not be empty", fullField),
			Field:   fullField,
		})
	}

	return errs
}
