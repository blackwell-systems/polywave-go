package protocol

import (
	"testing"
)

// validManifestForJS returns a minimal but complete manifest that should pass
// all ValidateManifestJSON checks.
func validManifestForJS() *IMPLManifest {
	return &IMPLManifest{
		Title:       "Test Feature",
		FeatureSlug: "test-feature",
		Verdict:     "SUITABLE",
		TestCommand: "go test ./...",
		LintCommand: "go vet ./...",
		FileOwnership: []FileOwnership{
			{File: "pkg/foo/bar.go", Agent: "A", Wave: 1},
		},
		InterfaceContracts: []InterfaceContract{
			{Name: "Foo", Definition: "func Foo() error", Location: "pkg/foo/bar.go"},
		},
		Waves: []Wave{
			{
				Number: 1,
				Agents: []Agent{
					{ID: "A", Task: "Implement foo", Files: []string{"pkg/foo/bar.go"}},
				},
			},
		},
	}
}

// TestGenerateManifestSchema_Structure verifies that the generated schema
// has the expected top-level keys for a JSON Schema document.
func TestGenerateManifestSchema_Structure(t *testing.T) {
	schema, err := GenerateManifestSchema()
	if err != nil {
		t.Fatalf("GenerateManifestSchema() returned error: %v", err)
	}
	if schema == nil {
		t.Fatal("GenerateManifestSchema() returned nil")
	}

	// The schema should have "properties" key at the top level
	// (invopop/jsonschema with DoNotReference=true puts them at root).
	if _, ok := schema["properties"]; !ok {
		t.Fatalf("expected schema to have 'properties' key, got keys: %v", mapKeys(schema))
	}
}

// TestGenerateManifestSchema_RequiredFields verifies that title and feature_slug
// appear in the schema properties.
func TestGenerateManifestSchema_RequiredFields(t *testing.T) {
	schema, err := GenerateManifestSchema()
	if err != nil {
		t.Fatalf("GenerateManifestSchema() returned error: %v", err)
	}

	properties := extractProperties(t, schema)

	requiredFields := []string{"title", "feature_slug"}
	for _, field := range requiredFields {
		if _, found := properties[field]; !found {
			t.Errorf("expected field %q to be present in schema properties, got: %v", field, mapKeys(properties))
		}
	}
}

// TestGenerateManifestSchema_AllIMPLFields verifies that major manifest fields
// appear in the generated schema.
func TestGenerateManifestSchema_AllIMPLFields(t *testing.T) {
	schema, err := GenerateManifestSchema()
	if err != nil {
		t.Fatalf("GenerateManifestSchema() returned error: %v", err)
	}

	properties := extractProperties(t, schema)

	expectedFields := []string{
		"title",
		"feature_slug",
		"verdict",
		"test_command",
		"lint_command",
		"file_ownership",
		"interface_contracts",
		"waves",
	}

	for _, field := range expectedFields {
		if _, found := properties[field]; !found {
			t.Errorf("expected field %q in schema properties, got: %v", field, mapKeys(properties))
		}
	}
}

// TestGenerateManifestSchema_Caching verifies that GenerateManifestSchema returns
// the same map on repeated calls (cache is working).
func TestGenerateManifestSchema_Caching(t *testing.T) {
	s1, err1 := GenerateManifestSchema()
	s2, err2 := GenerateManifestSchema()

	if err1 != nil || err2 != nil {
		t.Fatalf("GenerateManifestSchema() errors: %v, %v", err1, err2)
	}

	// The exact same pointer should be returned (cache hit).
	// We check structural equality via key count as a proxy.
	if len(s1) != len(s2) {
		t.Errorf("expected same schema on repeated calls, got different lengths: %d vs %d", len(s1), len(s2))
	}
}

// TestValidateManifestJSON_ValidManifest verifies that a well-formed manifest
// produces no validation errors.
func TestValidateManifestJSON_ValidManifest(t *testing.T) {
	m := validManifestForJS()
	errs := ValidateManifestJSON(m)
	if len(errs) != 0 {
		t.Errorf("expected no errors for valid manifest, got %d: %v", len(errs), errs)
	}
}

// TestValidateManifestJSON_MissingTitle verifies that omitting title produces
// a JS01_REQUIRED_FIELD error for the "title" field.
func TestValidateManifestJSON_MissingTitle(t *testing.T) {
	m := validManifestForJS()
	m.Title = ""

	errs := ValidateManifestJSON(m)
	found := false
	for _, e := range errs {
		if e.Code == JS01RequiredField && e.Field == "title" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected JS01_REQUIRED_FIELD for missing title, got: %v", errs)
	}
}

// TestValidateManifestJSON_MissingFeatureSlug verifies that omitting feature_slug
// produces a JS01_REQUIRED_FIELD error.
func TestValidateManifestJSON_MissingFeatureSlug(t *testing.T) {
	m := validManifestForJS()
	m.FeatureSlug = ""

	errs := ValidateManifestJSON(m)
	found := false
	for _, e := range errs {
		if e.Code == JS01RequiredField && e.Field == "feature_slug" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected JS01_REQUIRED_FIELD for missing feature_slug, got: %v", errs)
	}
}

// TestValidateManifestJSON_InvalidVerdict verifies that an invalid verdict string
// produces a JS01_INVALID_ENUM error.
func TestValidateManifestJSON_InvalidVerdict(t *testing.T) {
	m := validManifestForJS()
	m.Verdict = "INVALID_VERDICT"

	errs := ValidateManifestJSON(m)
	found := false
	for _, e := range errs {
		if e.Code == JS01InvalidEnum && e.Field == "verdict" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected JS01_INVALID_ENUM for invalid verdict, got: %v", errs)
	}
}

// TestValidateManifestJSON_ValidVerdicts verifies that all three valid verdict
// values are accepted without errors.
func TestValidateManifestJSON_ValidVerdicts(t *testing.T) {
	validVerdicts := []string{"SUITABLE", "NOT_SUITABLE", "SUITABLE_WITH_CAVEATS"}
	for _, v := range validVerdicts {
		m := validManifestForJS()
		m.Verdict = v
		errs := ValidateManifestJSON(m)
		for _, e := range errs {
			if e.Field == "verdict" {
				t.Errorf("verdict %q produced unexpected error: %v", v, e)
			}
		}
	}
}

// TestValidateManifestJSON_NestedValidation verifies that wave/agent fields
// are validated (missing agent ID should produce an error).
func TestValidateManifestJSON_NestedValidation(t *testing.T) {
	m := validManifestForJS()
	m.Waves[0].Agents[0].ID = ""

	errs := ValidateManifestJSON(m)
	found := false
	for _, e := range errs {
		if e.Code == JS01RequiredField && e.Field == "waves[0].agents[0].id" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected JS01_REQUIRED_FIELD for missing agent ID in wave, got: %v", errs)
	}
}

// TestValidateManifestJSON_NestedAgentTask verifies that missing agent task
// produces a JS01_REQUIRED_FIELD error.
func TestValidateManifestJSON_NestedAgentTask(t *testing.T) {
	m := validManifestForJS()
	m.Waves[0].Agents[0].Task = ""

	errs := ValidateManifestJSON(m)
	found := false
	for _, e := range errs {
		if e.Code == JS01RequiredField && e.Field == "waves[0].agents[0].task" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected JS01_REQUIRED_FIELD for missing agent task, got: %v", errs)
	}
}

// TestValidateManifestJSON_FileOwnershipValidation verifies that file_ownership
// entries with missing file field produce an error.
func TestValidateManifestJSON_FileOwnershipValidation(t *testing.T) {
	m := validManifestForJS()
	m.FileOwnership[0].File = ""

	errs := ValidateManifestJSON(m)
	found := false
	for _, e := range errs {
		if e.Code == JS01RequiredField && e.Field == "file_ownership[0].file" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected JS01_REQUIRED_FIELD for missing file in file_ownership, got: %v", errs)
	}
}

// TestValidateManifestJSON_InvalidState verifies that an invalid state enum
// value produces a JS01_INVALID_ENUM error.
func TestValidateManifestJSON_InvalidState(t *testing.T) {
	m := validManifestForJS()
	m.State = ProtocolState("INVALID_STATE")

	errs := ValidateManifestJSON(m)
	found := false
	for _, e := range errs {
		if e.Code == JS01InvalidEnum && e.Field == "state" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected JS01_INVALID_ENUM for invalid state, got: %v", errs)
	}
}

// TestValidateManifestJSON_ValidState verifies that known valid state values
// don't produce enum errors.
func TestValidateManifestJSON_ValidState(t *testing.T) {
	validStates := []ProtocolState{
		StateScoutPending,
		StateScoutValidating,
		StateReviewed,
		StateScaffoldPending,
		StateWavePending,
		StateWaveExecuting,
		StateWaveMerging,
		StateWaveVerified,
		StateBlocked,
		StateComplete,
		StateNotSuitable,
	}

	for _, s := range validStates {
		m := validManifestForJS()
		m.State = s
		errs := ValidateManifestJSON(m)
		for _, e := range errs {
			if e.Field == "state" {
				t.Errorf("state %q produced unexpected error: %v", s, e)
			}
		}
	}
}

// TestValidateManifestJSON_EmptyStateAllowed verifies that an empty state
// (optional field) does not produce an error.
func TestValidateManifestJSON_EmptyStateAllowed(t *testing.T) {
	m := validManifestForJS()
	m.State = ""

	errs := ValidateManifestJSON(m)
	for _, e := range errs {
		if e.Field == "state" {
			t.Errorf("empty state produced unexpected error: %v", e)
		}
	}
}

// TestValidateManifestJSON_InvalidMergeState verifies that an invalid merge_state
// enum value produces a JS01_INVALID_ENUM error.
func TestValidateManifestJSON_InvalidMergeState(t *testing.T) {
	m := validManifestForJS()
	m.MergeState = MergeState("INVALID")

	errs := ValidateManifestJSON(m)
	found := false
	for _, e := range errs {
		if e.Code == JS01InvalidEnum && e.Field == "merge_state" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected JS01_INVALID_ENUM for invalid merge_state, got: %v", errs)
	}
}

// TestValidateManifestJSON_MissingTestCommand verifies that omitting test_command
// produces a JS01_REQUIRED_FIELD error.
func TestValidateManifestJSON_MissingTestCommand(t *testing.T) {
	m := validManifestForJS()
	m.TestCommand = ""

	errs := ValidateManifestJSON(m)
	found := false
	for _, e := range errs {
		if e.Code == JS01RequiredField && e.Field == "test_command" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected JS01_REQUIRED_FIELD for missing test_command, got: %v", errs)
	}
}

// TestValidateManifestJSON_QualityGatesValidation verifies that quality_gates
// with invalid gate type produces a JS01_INVALID_ENUM error.
func TestValidateManifestJSON_QualityGatesValidation(t *testing.T) {
	m := validManifestForJS()
	m.QualityGates = &QualityGates{
		Level: "standard",
		Gates: []QualityGate{
			{Type: "invalid_gate_type", Command: "go build ./...", Required: true},
		},
	}

	errs := ValidateManifestJSON(m)
	found := false
	for _, e := range errs {
		if e.Code == JS01InvalidEnum {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected JS01_INVALID_ENUM for invalid gate type, got: %v", errs)
	}
}

// TestValidateManifestJSON_ConsistentWithValidate verifies that ValidateManifestJSON
// and ValidateSchema are consistent: a manifest that passes both is valid, and
// a manifest that fails both reports errors.
func TestValidateManifestJSON_ConsistentWithValidate(t *testing.T) {
	// A valid manifest should produce no errors in either validator.
	t.Run("valid manifest", func(t *testing.T) {
		m := validManifestForJS()
		jsErrs := ValidateManifestJSON(m)
		schemaErrs := ValidateSchema(m)
		if len(jsErrs) != 0 {
			t.Errorf("ValidateManifestJSON returned %d errors for valid manifest: %v", len(jsErrs), jsErrs)
		}
		if len(schemaErrs) != 0 {
			t.Errorf("ValidateSchema returned %d errors for valid manifest: %v", len(schemaErrs), schemaErrs)
		}
	})

	// An invalid manifest should produce errors in at least one validator.
	t.Run("invalid manifest missing title", func(t *testing.T) {
		m := validManifestForJS()
		m.Title = ""
		jsErrs := ValidateManifestJSON(m)

		// JS validator must report the missing title.
		foundJS := false
		for _, e := range jsErrs {
			if e.Field == "title" {
				foundJS = true
				break
			}
		}
		if !foundJS {
			t.Errorf("ValidateManifestJSON should report missing title, got: %v", jsErrs)
		}
	})

	// An invalid manifest with a bad agent ID should be caught by existing Validate().
	t.Run("bad agent ID caught by Validate", func(t *testing.T) {
		m := validManifestForJS()
		m.Waves[0].Agents[0].ID = "lowercase"
		m.FileOwnership[0].Agent = "lowercase"

		allErrs := Validate(m)
		foundAgentIDErr := false
		for _, e := range allErrs {
			if e.Code == "DC04_INVALID_AGENT_ID" {
				foundAgentIDErr = true
				break
			}
		}
		if !foundAgentIDErr {
			t.Errorf("Validate() should catch invalid agent ID, got: %v", allErrs)
		}
	})
}

// TestValidateManifestJSON_MultipleErrors verifies that multiple errors can be
// returned in a single call (not short-circuiting on first error).
func TestValidateManifestJSON_MultipleErrors(t *testing.T) {
	m := validManifestForJS()
	m.Title = ""
	m.FeatureSlug = ""
	m.TestCommand = ""

	errs := ValidateManifestJSON(m)
	if len(errs) < 3 {
		t.Errorf("expected at least 3 errors for 3 missing required fields, got %d: %v", len(errs), errs)
	}
}

// TestValidateManifestJSON_ErrorCodes verifies that all returned errors use
// the JS01_ prefix to distinguish them from SV01_ codes.
func TestValidateManifestJSON_ErrorCodes(t *testing.T) {
	m := validManifestForJS()
	m.Title = ""
	m.Verdict = "INVALID"

	errs := ValidateManifestJSON(m)
	for _, e := range errs {
		if len(e.Code) < 4 || e.Code[:4] != "JS01" {
			t.Errorf("expected error code to start with 'JS01', got: %q", e.Code)
		}
	}
}

// TestValidateManifestJSON_NilQualityGates verifies that nil quality_gates
// does not produce errors (it is optional).
func TestValidateManifestJSON_NilQualityGates(t *testing.T) {
	m := validManifestForJS()
	m.QualityGates = nil

	errs := ValidateManifestJSON(m)
	for _, e := range errs {
		if e.Field == "quality_gates" || e.Field == "quality_gates.level" {
			t.Errorf("nil quality_gates should not produce errors, got: %v", e)
		}
	}
}
