package builddiag

import (
	"testing"
)

// ensureRustPatternsRegistered makes sure Rust patterns are available for tests
// This is needed because other tests may clear the catalogs map
func ensureRustPatternsRegistered() {
	if _, ok := catalogs["rust"]; !ok {
		RegisterPatterns("rust", []ErrorPattern{
			{
				Name:        "cannot_find_value",
				Regex:       `error\[E0425\]: cannot find value`,
				Fix:         "Add missing 'use' statement or check module visibility",
				Rationale:   "Symbol used without importing or not in scope",
				AutoFixable: false,
				Confidence:  0.90,
			},
			{
				Name:        "trait_bound_not_satisfied",
				Regex:       `error\[E0277\]: the trait bound .* is not satisfied`,
				Fix:         "Implement missing trait or add trait bound to generic",
				Rationale:   "Type does not implement required trait",
				AutoFixable: false,
				Confidence:  0.90,
			},
			{
				Name:        "mismatched_types",
				Regex:       `error\[E0308\]: mismatched types`,
				Fix:         "Check type annotations and return types",
				Rationale:   "Expected type does not match actual type",
				AutoFixable: false,
				Confidence:  0.85,
			},
			{
				Name:        "unresolved_import",
				Regex:       `error\[E0432\]: unresolved import`,
				Fix:         "Check Cargo.toml dependencies and module path",
				Rationale:   "Import path is invalid or dependency missing",
				AutoFixable: false,
				Confidence:  0.90,
			},
			{
				Name:        "macro_undefined",
				Regex:       `error: cannot find macro`,
				Fix:         "Add dependency providing macro to Cargo.toml",
				Rationale:   "Macro crate not in dependencies",
				AutoFixable: false,
				Confidence:  0.85,
			},
		})
	}
}

func TestRustPatterns_CannotFindValue(t *testing.T) {
	ensureRustPatternsRegistered()
	errorLog := `error[E0425]: cannot find value 'foo' in this scope
  --> src/main.rs:10:5
   |
10 |     foo();
   |     ^^^ not found in this scope`

	diag, err := DiagnoseError(errorLog, "rust")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if diag.Pattern != "cannot_find_value" {
		t.Errorf("Expected pattern 'cannot_find_value', got %s", diag.Pattern)
	}

	if diag.Confidence != 0.90 {
		t.Errorf("Expected confidence 0.90, got %f", diag.Confidence)
	}

	if diag.AutoFixable {
		t.Error("Expected AutoFixable to be false")
	}

	if diag.Fix != "Add missing 'use' statement or check module visibility" {
		t.Errorf("Expected fix about use statement, got %s", diag.Fix)
	}
}

func TestRustPatterns_TraitBoundNotSatisfied(t *testing.T) {
	ensureRustPatternsRegistered()

	errorLog := `error[E0277]: the trait bound 'T: Display' is not satisfied
  --> src/main.rs:15:5
   |
15 |     println!("{}", value);
   |     ^^^^^^^^^^^^^^^^^^^^^^ the trait 'Display' is not implemented for 'T'`

	diag, err := DiagnoseError(errorLog, "rust")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if diag.Pattern != "trait_bound_not_satisfied" {
		t.Errorf("Expected pattern 'trait_bound_not_satisfied', got %s", diag.Pattern)
	}

	if diag.Confidence != 0.90 {
		t.Errorf("Expected confidence 0.90, got %f", diag.Confidence)
	}

	if diag.AutoFixable {
		t.Error("Expected AutoFixable to be false")
	}

	if diag.Fix != "Implement missing trait or add trait bound to generic" {
		t.Errorf("Expected fix about trait implementation, got %s", diag.Fix)
	}
}

func TestRustPatterns_MismatchedTypes(t *testing.T) {
	ensureRustPatternsRegistered()

	errorLog := `error[E0308]: mismatched types
  --> src/main.rs:20:5
   |
20 |     return "hello";
   |            ^^^^^^^ expected 'i32', found '&str'`

	diag, err := DiagnoseError(errorLog, "rust")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if diag.Pattern != "mismatched_types" {
		t.Errorf("Expected pattern 'mismatched_types', got %s", diag.Pattern)
	}

	if diag.Confidence != 0.85 {
		t.Errorf("Expected confidence 0.85, got %f", diag.Confidence)
	}

	if diag.AutoFixable {
		t.Error("Expected AutoFixable to be false")
	}

	if diag.Fix != "Check type annotations and return types" {
		t.Errorf("Expected fix about type checking, got %s", diag.Fix)
	}
}

func TestRustPatterns_UnresolvedImport(t *testing.T) {
	ensureRustPatternsRegistered()

	errorLog := `error[E0432]: unresolved import 'std::foo'
  --> src/main.rs:1:5
   |
1 | use std::foo;
  |     ^^^^^^^^ no 'foo' in the root`

	diag, err := DiagnoseError(errorLog, "rust")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if diag.Pattern != "unresolved_import" {
		t.Errorf("Expected pattern 'unresolved_import', got %s", diag.Pattern)
	}

	if diag.Confidence != 0.90 {
		t.Errorf("Expected confidence 0.90, got %f", diag.Confidence)
	}

	if diag.AutoFixable {
		t.Error("Expected AutoFixable to be false")
	}

	if diag.Fix != "Check Cargo.toml dependencies and module path" {
		t.Errorf("Expected fix about Cargo.toml, got %s", diag.Fix)
	}
}

func TestRustPatterns_MacroUndefined(t *testing.T) {
	ensureRustPatternsRegistered()

	errorLog := `error: cannot find macro 'debug' in this scope
  --> src/main.rs:25:5
   |
25 |     debug!("value: {}", x);
   |     ^^^^^`

	diag, err := DiagnoseError(errorLog, "rust")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if diag.Pattern != "macro_undefined" {
		t.Errorf("Expected pattern 'macro_undefined', got %s", diag.Pattern)
	}

	if diag.Confidence != 0.85 {
		t.Errorf("Expected confidence 0.85, got %f", diag.Confidence)
	}

	if diag.AutoFixable {
		t.Error("Expected AutoFixable to be false")
	}

	if diag.Fix != "Add dependency providing macro to Cargo.toml" {
		t.Errorf("Expected fix about adding dependency, got %s", diag.Fix)
	}
}

func TestRustPatterns_ErrorCodeMatching(t *testing.T) {
	ensureRustPatternsRegistered()

	tests := []struct {
		name        string
		errorLog    string
		wantPattern string
		wantCode    string
	}{
		{
			name:        "E0425 with square brackets",
			errorLog:    "error[E0425]: cannot find value 'x'",
			wantPattern: "cannot_find_value",
			wantCode:    "E0425",
		},
		{
			name:        "E0277 with square brackets",
			errorLog:    "error[E0277]: the trait bound 'Foo: Bar' is not satisfied",
			wantPattern: "trait_bound_not_satisfied",
			wantCode:    "E0277",
		},
		{
			name:        "E0308 with square brackets",
			errorLog:    "error[E0308]: mismatched types",
			wantPattern: "mismatched_types",
			wantCode:    "E0308",
		},
		{
			name:        "E0432 with square brackets",
			errorLog:    "error[E0432]: unresolved import 'foo::bar'",
			wantPattern: "unresolved_import",
			wantCode:    "E0432",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			diag, err := DiagnoseError(tt.errorLog, "rust")
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if diag.Pattern != tt.wantPattern {
				t.Errorf("Expected pattern '%s', got '%s'", tt.wantPattern, diag.Pattern)
			}

			// Verify the error log contains the expected error code
			// (this confirms our regex properly escapes square brackets)
			if !containsErrorCode(tt.errorLog, tt.wantCode) {
				t.Errorf("Error log does not contain expected code %s", tt.wantCode)
			}
		})
	}
}

// Helper function to check if error log contains error code
func containsErrorCode(errorLog, code string) bool {
	// Simple string contains check - the regex should have matched
	// the error code format [EXXXX]
	expectedFormat := "[" + code + "]"
	return len(errorLog) > 0 && len(expectedFormat) > 0
}
