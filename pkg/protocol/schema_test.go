package protocol

import (
	"testing"
)

func TestGenerateScoutSchema_ReturnsSchema(t *testing.T) {
	schema, err := GenerateScoutSchema()
	if err != nil {
		t.Fatalf("GenerateScoutSchema() returned error: %v", err)
	}
	if schema == nil {
		t.Fatal("GenerateScoutSchema() returned nil map")
	}
	if _, ok := schema["properties"]; !ok {
		// The top-level schema wraps definitions; look one level deeper.
		// With DoNotReference=true the root $defs may be absent; check $defs too.
		// invopop/jsonschema wraps the reflected type under a top-level schema
		// that has a "$defs" key; the actual properties live inside $defs or
		// directly on the root when DoNotReference is true.
		//
		// When DoNotReference=true and we reflect a struct, the root schema
		// should contain "properties" directly (no $ref needed).
		t.Fatalf("expected schema map to have 'properties' key, got keys: %v", mapKeys(schema))
	}
}

func TestGenerateScoutSchema_ExcludesRuntimeFields(t *testing.T) {
	schema, err := GenerateScoutSchema()
	if err != nil {
		t.Fatalf("GenerateScoutSchema() returned error: %v", err)
	}

	properties := extractProperties(t, schema)

	runtimeFields := []string{
		"completion_reports",
		"stub_reports",
		"merge_state",
		"worktrees_created_at",
		"frozen_contracts_hash",
		"frozen_scaffolds_hash",
	}

	for _, field := range runtimeFields {
		if _, found := properties[field]; found {
			t.Errorf("expected runtime field %q to be excluded from schema properties, but it was present", field)
		}
	}
}

func TestGenerateScoutSchema_IncludesRequiredFields(t *testing.T) {
	schema, err := GenerateScoutSchema()
	if err != nil {
		t.Fatalf("GenerateScoutSchema() returned error: %v", err)
	}

	properties := extractProperties(t, schema)

	requiredFields := []string{
		"feature_slug",
		"verdict",
		"waves",
		"file_ownership",
	}

	for _, field := range requiredFields {
		if _, found := properties[field]; !found {
			t.Errorf("expected required field %q to be present in schema properties, but it was missing; got: %v", field, mapKeys(properties))
		}
	}
}

// extractProperties digs out the "properties" map from the top-level schema.
// invopop/jsonschema with DoNotReference=true places properties directly on
// the root object schema.
func extractProperties(t *testing.T, schema map[string]any) map[string]any {
	t.Helper()

	// Direct properties on root
	if props, ok := schema["properties"]; ok {
		if m, ok := props.(map[string]any); ok {
			return m
		}
		t.Fatalf("'properties' key exists but is not map[string]any: %T", props)
	}

	t.Fatalf("could not find 'properties' in schema; top-level keys: %v", mapKeys(schema))
	return nil
}

// mapKeys returns the keys of a map[string]any for diagnostic messages.
func mapKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

func TestGenerateCompletionReportSchema_Valid(t *testing.T) {
	schema, err := GenerateCompletionReportSchema()
	if err != nil {
		t.Fatalf("GenerateCompletionReportSchema() returned error: %v", err)
	}
	if schema == nil {
		t.Fatal("GenerateCompletionReportSchema() returned nil map")
	}
	if _, ok := schema["properties"]; !ok {
		t.Fatalf("expected schema map to have 'properties' key, got keys: %v", mapKeys(schema))
	}
}

func TestGenerateCompletionReportSchema_HasRequiredFields(t *testing.T) {
	schema, err := GenerateCompletionReportSchema()
	if err != nil {
		t.Fatalf("GenerateCompletionReportSchema() returned error: %v", err)
	}

	properties := extractProperties(t, schema)

	// Status is the primary required field for a completion report
	if _, found := properties["status"]; !found {
		t.Errorf("expected required field 'status' to be present in schema properties; got: %v", mapKeys(properties))
	}
}

func TestGenerateCompletionReportSchema_RoundTrip(t *testing.T) {
	schema, err := GenerateCompletionReportSchema()
	if err != nil {
		t.Fatalf("GenerateCompletionReportSchema() returned error: %v", err)
	}

	properties := extractProperties(t, schema)

	// Verify all expected CompletionReport fields are present in the schema
	expectedFields := []string{
		"status",
		"worktree",
		"branch",
		"commit",
		"files_changed",
		"files_created",
		"interface_deviations",
		"out_of_scope_deps",
		"tests_added",
		"verification",
		"failure_type",
		"notes",
	}

	for _, field := range expectedFields {
		if _, found := properties[field]; !found {
			t.Errorf("expected field %q to be present in schema properties; got: %v", field, mapKeys(properties))
		}
	}

	// Verify that properties have correct types
	if statusProp, ok := properties["status"]; ok {
		if propMap, ok := statusProp.(map[string]any); ok {
			if propType, ok := propMap["type"]; ok {
				if propType != "string" {
					t.Errorf("expected 'status' field to have type 'string', got: %v", propType)
				}
			}
		}
	}

	// Verify array fields have correct type
	arrayFields := []string{"files_changed", "files_created", "out_of_scope_deps", "tests_added"}
	for _, field := range arrayFields {
		if fieldProp, ok := properties[field]; ok {
			if propMap, ok := fieldProp.(map[string]any); ok {
				if propType, ok := propMap["type"]; ok {
					if propType != "array" {
						t.Errorf("expected %q field to have type 'array', got: %v", field, propType)
					}
				}
			}
		}
	}
}
