package builddiag

import (
	"strings"
	"testing"
)

func TestRegisterPatterns(t *testing.T) {
	// Clear catalogs first
	catalogs = make(map[string][]ErrorPattern)

	patterns := []ErrorPattern{
		{
			Name:        "test_pattern",
			Regex:       "test error",
			Fix:         "test fix",
			Rationale:   "test rationale",
			AutoFixable: true,
			Confidence:  0.8,
		},
	}

	RegisterPatterns("testlang", patterns)

	registered, ok := catalogs["testlang"]
	if !ok {
		t.Fatal("Pattern not registered")
	}

	if len(registered) != 1 {
		t.Errorf("Expected 1 pattern, got %d", len(registered))
	}

	if registered[0].Name != "test_pattern" {
		t.Errorf("Expected pattern name 'test_pattern', got %s", registered[0].Name)
	}
}

func TestDiagnoseError_KnownPattern(t *testing.T) {
	// Clear and register test patterns
	catalogs = make(map[string][]ErrorPattern)

	patterns := []ErrorPattern{
		{
			Name:        "missing_import",
			Regex:       `undefined: \w+`,
			Fix:         "go mod tidy",
			Rationale:   "Missing dependency",
			AutoFixable: true,
			Confidence:  0.9,
		},
	}

	RegisterPatterns("go", patterns)

	errorLog := "main.go:10:5: undefined: fmt"
	diag, err := DiagnoseError(errorLog, "go")

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if diag.Pattern != "missing_import" {
		t.Errorf("Expected pattern 'missing_import', got %s", diag.Pattern)
	}

	if diag.Confidence != 0.9 {
		t.Errorf("Expected confidence 0.9, got %f", diag.Confidence)
	}

	if diag.Fix != "go mod tidy" {
		t.Errorf("Expected fix 'go mod tidy', got %s", diag.Fix)
	}

	if !diag.AutoFixable {
		t.Error("Expected AutoFixable to be true")
	}
}

func TestDiagnoseError_NoMatch(t *testing.T) {
	// Clear and register test patterns
	catalogs = make(map[string][]ErrorPattern)

	patterns := []ErrorPattern{
		{
			Name:        "missing_import",
			Regex:       `undefined: \w+`,
			Fix:         "go mod tidy",
			Rationale:   "Missing dependency",
			AutoFixable: true,
			Confidence:  0.9,
		},
	}

	RegisterPatterns("go", patterns)

	errorLog := "some random error that doesn't match"
	diag, err := DiagnoseError(errorLog, "go")

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if diag.Pattern != "unknown" {
		t.Errorf("Expected pattern 'unknown', got %s", diag.Pattern)
	}

	if diag.Confidence != 0.0 {
		t.Errorf("Expected confidence 0.0, got %f", diag.Confidence)
	}

	if diag.Fix != "Manual investigation required" {
		t.Errorf("Expected fix 'Manual investigation required', got %s", diag.Fix)
	}

	if diag.AutoFixable {
		t.Error("Expected AutoFixable to be false")
	}
}

func TestDiagnoseError_UnsupportedLanguage(t *testing.T) {
	// Clear catalogs
	catalogs = make(map[string][]ErrorPattern)

	errorLog := "some error"
	_, err := DiagnoseError(errorLog, "unsupported")

	if err == nil {
		t.Fatal("Expected error for unsupported language, got nil")
	}

	if !strings.Contains(err.Error(), "unsupported language") {
		t.Errorf("Expected 'unsupported language' error, got: %v", err)
	}
}

func TestDiagnoseError_InvalidRegex(t *testing.T) {
	// Clear and register patterns with invalid regex
	catalogs = make(map[string][]ErrorPattern)

	patterns := []ErrorPattern{
		{
			Name:        "bad_regex",
			Regex:       "[invalid(regex", // Invalid regex
			Fix:         "should skip",
			Rationale:   "bad pattern",
			AutoFixable: false,
			Confidence:  0.5,
		},
		{
			Name:        "good_pattern",
			Regex:       "valid error",
			Fix:         "valid fix",
			Rationale:   "good pattern",
			AutoFixable: true,
			Confidence:  0.8,
		},
	}

	RegisterPatterns("testlang", patterns)

	errorLog := "valid error message"
	diag, err := DiagnoseError(errorLog, "testlang")

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Should skip bad regex and match good pattern
	if diag.Pattern != "good_pattern" {
		t.Errorf("Expected pattern 'good_pattern', got %s", diag.Pattern)
	}
}

func TestSupportedLanguages(t *testing.T) {
	// Clear and register multiple languages
	catalogs = make(map[string][]ErrorPattern)

	patterns := []ErrorPattern{
		{Name: "test", Regex: "test", Fix: "fix", Rationale: "test", AutoFixable: true, Confidence: 0.8},
	}

	RegisterPatterns("go", patterns)
	RegisterPatterns("python", patterns)
	RegisterPatterns("rust", patterns)

	langs := SupportedLanguages()

	if len(langs) != 3 {
		t.Errorf("Expected 3 languages, got %d", len(langs))
	}

	// Check all languages are present
	langMap := make(map[string]bool)
	for _, lang := range langs {
		langMap[lang] = true
	}

	if !langMap["go"] || !langMap["python"] || !langMap["rust"] {
		t.Errorf("Missing expected languages. Got: %v", langs)
	}
}

func TestRegisterPatterns_CaseInsensitive(t *testing.T) {
	// Clear catalogs
	catalogs = make(map[string][]ErrorPattern)

	patterns := []ErrorPattern{
		{Name: "test", Regex: "test", Fix: "fix", Rationale: "test", AutoFixable: true, Confidence: 0.8},
	}

	// Register with uppercase
	RegisterPatterns("GO", patterns)

	// Should be stored as lowercase
	_, ok := catalogs["go"]
	if !ok {
		t.Error("Expected language to be stored as lowercase")
	}

	// DiagnoseError should work with any case
	errorLog := "test error"
	diag, err := DiagnoseError(errorLog, "Go")

	if err != nil {
		t.Fatalf("Expected case-insensitive lookup to work, got error: %v", err)
	}

	if diag.Pattern != "test" {
		t.Errorf("Expected pattern 'test', got %s", diag.Pattern)
	}
}
