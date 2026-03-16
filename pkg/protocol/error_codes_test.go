package protocol

import (
	"testing"
)

// validCategories is the exhaustive set of allowed category strings.
var validCategories = map[string]bool{
	"invariant": true,
	"schema":    true,
	"state":     true,
	"field":     true,
}

// validSeverities is the exhaustive set of allowed severity strings.
var validSeverities = map[string]bool{
	"error":   true,
	"warning": true,
}

// TestAllErrorCodes_Complete verifies that every code returned by AllErrorCodes
// has a non-empty registry entry (Code, Category, Severity, Description all set).
func TestAllErrorCodes_Complete(t *testing.T) {
	all := AllErrorCodes()
	if len(all) == 0 {
		t.Fatal("AllErrorCodes returned empty slice")
	}

	for _, info := range all {
		if info.Code == "" {
			t.Error("AllErrorCodes returned an entry with an empty Code field")
		}
		if info.Category == "" {
			t.Errorf("code %q has empty Category", info.Code)
		}
		if info.Severity == "" {
			t.Errorf("code %q has empty Severity", info.Code)
		}
		if info.Description == "" {
			t.Errorf("code %q has empty Description", info.Code)
		}
	}
}

// TestLookupErrorCode_Known verifies that LookupErrorCode returns the correct
// ErrorCodeInfo for a representative set of known codes.
func TestLookupErrorCode_Known(t *testing.T) {
	cases := []struct {
		code     ErrorCode
		wantCat  string
		wantSev  string
	}{
		{ErrMissingTitle, "field", "error"},
		{ErrMissingSlug, "field", "error"},
		{ErrInvalidVerdict, "field", "error"},
		{ErrInvalidAgentID, "field", "error"},
		{ErrI1Violation, "invariant", "error"},
		{ErrI2MissingDep, "invariant", "error"},
		{ErrI2WaveOrder, "invariant", "error"},
		{ErrI3WaveSequence, "invariant", "error"},
		{ErrI4MissingField, "invariant", "error"},
		{ErrI4InvalidValue, "invariant", "error"},
		{ErrI5OrphanFile, "invariant", "error"},
		{ErrI5Uncommitted, "invariant", "error"},
		{ErrI6Cycle, "invariant", "error"},
		{ErrInvalidGateType, "field", "error"},
		{ErrInvalidMergeState, "state", "error"},
		{ErrInvalidState, "state", "error"},
		{ErrMultiRepoMixed, "invariant", "error"},
		{ErrSVRequiredField, "schema", "error"},
		{ErrSVInvalidEnum, "schema", "error"},
		{ErrSVInvalidPath, "schema", "error"},
		{ErrSVUnknownKey, "schema", "warning"},
		{ErrSVCrossField, "schema", "error"},
	}

	for _, tc := range cases {
		info := LookupErrorCode(tc.code)
		if info.Code != tc.code {
			t.Errorf("LookupErrorCode(%q).Code = %q, want %q", tc.code, info.Code, tc.code)
		}
		if info.Category != tc.wantCat {
			t.Errorf("LookupErrorCode(%q).Category = %q, want %q", tc.code, info.Category, tc.wantCat)
		}
		if info.Severity != tc.wantSev {
			t.Errorf("LookupErrorCode(%q).Severity = %q, want %q", tc.code, info.Severity, tc.wantSev)
		}
		if info.Description == "" {
			t.Errorf("LookupErrorCode(%q).Description is empty", tc.code)
		}
	}
}

// TestLookupErrorCode_Unknown verifies that LookupErrorCode returns a zero-value
// ErrorCodeInfo when given an unregistered code.
func TestLookupErrorCode_Unknown(t *testing.T) {
	info := LookupErrorCode("TOTALLY_UNKNOWN_CODE")
	if info.Code != "" {
		t.Errorf("expected zero Code for unknown lookup, got %q", info.Code)
	}
	if info.Category != "" {
		t.Errorf("expected zero Category for unknown lookup, got %q", info.Category)
	}
	if info.Severity != "" {
		t.Errorf("expected zero Severity for unknown lookup, got %q", info.Severity)
	}
	if info.Description != "" {
		t.Errorf("expected zero Description for unknown lookup, got %q", info.Description)
	}
}

// TestMigrateErrorCode_AllMapped verifies that every legacy string code that
// exists in validation.go and schema_errors.go maps to a known typed ErrorCode.
func TestMigrateErrorCode_AllMapped(t *testing.T) {
	// All legacy codes sourced from validation.go + schema_errors.go
	legacyCodes := []string{
		// validation.go
		"I1_VIOLATION",
		"I2_MISSING_DEP",
		"I2_WAVE_ORDER",
		"I3_WAVE_ORDER",
		"I4_MISSING_FIELD",
		"I4_INVALID_VALUE",
		"I5_ORPHAN_FILE",
		"I5_UNCOMMITTED",
		"I6_CYCLE",
		"DC04_INVALID_AGENT_ID",
		"DC07_INVALID_GATE_TYPE",
		"E9_INVALID_MERGE_STATE",
		"SM01_INVALID_STATE",
		"MR01_INCONSISTENT_REPO",
		// schema_errors.go (using the constants directly)
		SV01RequiredField,
		SV01InvalidEnum,
		SV01InvalidPath,
		SV01UnknownKey,
		SV01CrossFieldError,
	}

	for _, old := range legacyCodes {
		got := MigrateErrorCode(old)
		if got == "" {
			t.Errorf("MigrateErrorCode(%q) returned empty string — mapping is missing", old)
			continue
		}
		// The returned code must also be registered
		info := LookupErrorCode(got)
		if info.Code == "" {
			t.Errorf("MigrateErrorCode(%q) = %q but that code is not in the registry", old, got)
		}
	}
}

// TestMigrateErrorCode_Unknown verifies that an unrecognised legacy code
// returns an empty ErrorCode rather than panicking.
func TestMigrateErrorCode_Unknown(t *testing.T) {
	got := MigrateErrorCode("DOES_NOT_EXIST")
	if got != "" {
		t.Errorf("expected empty ErrorCode for unknown migrate, got %q", got)
	}
}

// TestErrorCodeInfo_Categories verifies that every registered code uses one of
// the declared valid category strings.
func TestErrorCodeInfo_Categories(t *testing.T) {
	for _, info := range AllErrorCodes() {
		if !validCategories[info.Category] {
			t.Errorf("code %q has invalid category %q (must be one of: invariant, schema, state, field)", info.Code, info.Category)
		}
	}
}

// TestErrorCodeInfo_Severities verifies that every registered code uses one of
// the declared valid severity strings.
func TestErrorCodeInfo_Severities(t *testing.T) {
	for _, info := range AllErrorCodes() {
		if !validSeverities[info.Severity] {
			t.Errorf("code %q has invalid severity %q (must be one of: error, warning)", info.Code, info.Severity)
		}
	}
}

// TestErrorCode_NoDuplicates verifies that no two distinct ErrorCode constants
// share the same string value.
func TestErrorCode_NoDuplicates(t *testing.T) {
	all := AllErrorCodes()
	seen := make(map[ErrorCode]bool, len(all))
	for _, info := range all {
		if seen[info.Code] {
			t.Errorf("duplicate ErrorCode value: %q", info.Code)
		}
		seen[info.Code] = true
	}
}

// TestAllErrorCodes_RegistryConsistency verifies that AllErrorCodes and the
// registry agree: every code in AllErrorCodes is in the registry, and the
// registry contains every code exposed by AllErrorCodes.
func TestAllErrorCodes_RegistryConsistency(t *testing.T) {
	all := AllErrorCodes()
	for _, info := range all {
		got := LookupErrorCode(info.Code)
		if got.Code != info.Code {
			t.Errorf("registry inconsistency: AllErrorCodes has %q but LookupErrorCode returned %q", info.Code, got.Code)
		}
	}
}
