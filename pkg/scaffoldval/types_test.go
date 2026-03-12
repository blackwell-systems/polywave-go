package scaffoldval

import (
	"testing"

	"gopkg.in/yaml.v3"
)

// TestNewValidationResult creates result with all SKIP status
func TestNewValidationResult(t *testing.T) {
	result := NewValidationResult()

	if result.Syntax.Status != "SKIP" {
		t.Errorf("Expected Syntax.Status to be SKIP, got %s", result.Syntax.Status)
	}
	if result.Imports.Status != "SKIP" {
		t.Errorf("Expected Imports.Status to be SKIP, got %s", result.Imports.Status)
	}
	if result.TypeReferences.Status != "SKIP" {
		t.Errorf("Expected TypeReferences.Status to be SKIP, got %s", result.TypeReferences.Status)
	}
	if result.Build.Status != "SKIP" {
		t.Errorf("Expected Build.Status to be SKIP, got %s", result.Build.Status)
	}
}

// TestValidationResult_OverallStatus_AllPass returns PASS when all pass
func TestValidationResult_OverallStatus_AllPass(t *testing.T) {
	result := &ValidationResult{
		Syntax:         ValidationStep{Status: "PASS"},
		Imports:        ValidationStep{Status: "PASS"},
		TypeReferences: ValidationStep{Status: "PASS"},
		Build:          ValidationStep{Status: "PASS"},
	}

	status := result.OverallStatus()
	if status != "PASS" {
		t.Errorf("Expected OverallStatus to be PASS when all steps pass, got %s", status)
	}
}

// TestValidationResult_OverallStatus_OneFail returns FAIL when any step fails
func TestValidationResult_OverallStatus_OneFail(t *testing.T) {
	testCases := []struct {
		name   string
		result *ValidationResult
	}{
		{
			name: "Syntax fails",
			result: &ValidationResult{
				Syntax:         ValidationStep{Status: "FAIL"},
				Imports:        ValidationStep{Status: "PASS"},
				TypeReferences: ValidationStep{Status: "PASS"},
				Build:          ValidationStep{Status: "PASS"},
			},
		},
		{
			name: "Imports fails",
			result: &ValidationResult{
				Syntax:         ValidationStep{Status: "PASS"},
				Imports:        ValidationStep{Status: "FAIL"},
				TypeReferences: ValidationStep{Status: "PASS"},
				Build:          ValidationStep{Status: "PASS"},
			},
		},
		{
			name: "TypeReferences fails",
			result: &ValidationResult{
				Syntax:         ValidationStep{Status: "PASS"},
				Imports:        ValidationStep{Status: "PASS"},
				TypeReferences: ValidationStep{Status: "FAIL"},
				Build:          ValidationStep{Status: "PASS"},
			},
		},
		{
			name: "Build fails",
			result: &ValidationResult{
				Syntax:         ValidationStep{Status: "PASS"},
				Imports:        ValidationStep{Status: "PASS"},
				TypeReferences: ValidationStep{Status: "PASS"},
				Build:          ValidationStep{Status: "FAIL"},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			status := tc.result.OverallStatus()
			if status != "FAIL" {
				t.Errorf("Expected OverallStatus to be FAIL when %s, got %s", tc.name, status)
			}
		})
	}
}

// TestValidationResult_OverallStatus_AllSkip returns PASS when all skipped
func TestValidationResult_OverallStatus_AllSkip(t *testing.T) {
	result := NewValidationResult()

	status := result.OverallStatus()
	if status != "PASS" {
		t.Errorf("Expected OverallStatus to be PASS when all steps skipped, got %s", status)
	}
}

// TestValidationStep_YAMLSerialization validates YAML output format
func TestValidationStep_YAMLSerialization(t *testing.T) {
	testCases := []struct {
		name     string
		step     ValidationStep
		expected string
	}{
		{
			name: "PASS with no errors",
			step: ValidationStep{
				Status: "PASS",
			},
			expected: "status: PASS\n",
		},
		{
			name: "FAIL with errors and fixes",
			step: ValidationStep{
				Status:      "FAIL",
				Errors:      []string{"syntax error on line 10", "missing import"},
				Fixes:       []string{"add semicolon", "import fmt"},
				AutoFixable: true,
			},
			expected: `status: FAIL
errors:
    - syntax error on line 10
    - missing import
fixes:
    - add semicolon
    - import fmt
auto_fixable: true
`,
		},
		{
			name: "SKIP with empty slices omitted",
			step: ValidationStep{
				Status: "SKIP",
				Errors: []string{},
				Fixes:  []string{},
			},
			expected: "status: SKIP\n",
		},
		{
			name: "FAIL with errors only",
			step: ValidationStep{
				Status: "FAIL",
				Errors: []string{"build failed"},
			},
			expected: `status: FAIL
errors:
    - build failed
`,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			data, err := yaml.Marshal(&tc.step)
			if err != nil {
				t.Fatalf("Failed to marshal YAML: %v", err)
			}

			if string(data) != tc.expected {
				t.Errorf("YAML output mismatch.\nExpected:\n%s\nGot:\n%s", tc.expected, string(data))
			}
		})
	}
}
