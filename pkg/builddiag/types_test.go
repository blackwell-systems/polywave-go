package builddiag

import (
	"testing"
)

func TestErrorPattern_Structure(t *testing.T) {
	pattern := ErrorPattern{
		Name:        "missing_import",
		Regex:       `undefined: \w+`,
		Fix:         "go mod tidy",
		Rationale:   "Missing dependency in go.mod",
		AutoFixable: true,
		Confidence:  0.9,
	}

	if pattern.Name != "missing_import" {
		t.Errorf("Expected Name 'missing_import', got %s", pattern.Name)
	}
	if pattern.Confidence != 0.9 {
		t.Errorf("Expected Confidence 0.9, got %f", pattern.Confidence)
	}
	if !pattern.AutoFixable {
		t.Error("Expected AutoFixable to be true")
	}
}

func TestDiagnosis_Structure(t *testing.T) {
	diag := Diagnosis{
		Pattern:     "type_mismatch",
		Confidence:  0.85,
		Fix:         "Check type compatibility",
		Rationale:   "Types do not match",
		AutoFixable: false,
	}

	if diag.Pattern != "type_mismatch" {
		t.Errorf("Expected Pattern 'type_mismatch', got %s", diag.Pattern)
	}
	if diag.Confidence != 0.85 {
		t.Errorf("Expected Confidence 0.85, got %f", diag.Confidence)
	}
	if diag.AutoFixable {
		t.Error("Expected AutoFixable to be false")
	}
}
