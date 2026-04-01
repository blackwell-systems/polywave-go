package builddiag

import (
	"testing"
)

func TestJSPatterns_ModuleNotFound(t *testing.T) {
	tests := []struct {
		name     string
		errorLog string
		wantName string
	}{
		{
			name:     "npm cannot find module",
			errorLog: "Error: Cannot find module 'express'",
			wantName: "module_not_found",
		},
		{
			name:     "webpack module not found",
			errorLog: "Module not found: Error: Can't resolve './utils'",
			wantName: "module_not_found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			diag := DiagnoseError(tt.errorLog, "javascript")
			if diag.Pattern != tt.wantName {
				t.Errorf("Pattern = %v, want %v", diag.Pattern, tt.wantName)
			}
			if !diag.AutoFixable {
				t.Errorf("AutoFixable = false, want true for module_not_found")
			}
			if diag.Confidence != 0.95 {
				t.Errorf("Confidence = %v, want 0.95", diag.Confidence)
			}
		})
	}
}

func TestJSPatterns_PropertyNotExist(t *testing.T) {
	errorLog := "TS2339: Property 'foo' does not exist on type 'Bar'"

	diag := DiagnoseError(errorLog, "typescript")

	if diag.Pattern != "property_not_exist" {
		t.Errorf("Pattern = %v, want property_not_exist", diag.Pattern)
	}
	if diag.AutoFixable {
		t.Errorf("AutoFixable = true, want false")
	}
	if diag.Confidence != 0.90 {
		t.Errorf("Confidence = %v, want 0.90", diag.Confidence)
	}
}

func TestJSPatterns_SyntaxError(t *testing.T) {
	errorLog := "SyntaxError: Unexpected token )"

	diag := DiagnoseError(errorLog, "js")

	if diag.Pattern != "syntax_error" {
		t.Errorf("Pattern = %v, want syntax_error", diag.Pattern)
	}
	if diag.AutoFixable {
		t.Errorf("AutoFixable = true, want false")
	}
	if diag.Confidence != 0.80 {
		t.Errorf("Confidence = %v, want 0.80", diag.Confidence)
	}
}

func TestJSPatterns_TypeAnyImplicit(t *testing.T) {
	errorLog := "TS7006: Parameter 'config' implicitly has an 'any' type"

	diag := DiagnoseError(errorLog, "typescript")

	if diag.Pattern != "type_any_implicit" {
		t.Errorf("Pattern = %v, want type_any_implicit", diag.Pattern)
	}
	if diag.AutoFixable {
		t.Errorf("AutoFixable = true, want false")
	}
	if diag.Confidence != 0.85 {
		t.Errorf("Confidence = %v, want 0.85", diag.Confidence)
	}
}

func TestJSPatterns_ImportPathInvalid(t *testing.T) {
	tests := []struct {
		name     string
		errorLog string
	}{
		{
			name:     "cannot resolve module",
			errorLog: "Cannot resolve module './missing'",
		},
		{
			name:     "file not a module",
			errorLog: "File '/path/to/file.js' is not a module",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			diag := DiagnoseError(tt.errorLog, "typescript")
			if diag.Pattern != "import_path_invalid" {
				t.Errorf("Pattern = %v, want import_path_invalid", diag.Pattern)
			}
			if diag.AutoFixable {
				t.Errorf("AutoFixable = true, want false")
			}
			if diag.Confidence != 0.85 {
				t.Errorf("Confidence = %v, want 0.85", diag.Confidence)
			}
		})
	}
}

func TestJSPatterns_MultipleAliases(t *testing.T) {
	errorLog := "Cannot find module 'react'"

	aliases := []string{"js", "javascript", "ts", "typescript"}

	for _, alias := range aliases {
		t.Run(alias, func(t *testing.T) {
			diag := DiagnoseError(errorLog, alias)
			if diag.Pattern != "module_not_found" {
				t.Errorf("Pattern = %v, want module_not_found for alias %q", diag.Pattern, alias)
			}
		})
	}
}
