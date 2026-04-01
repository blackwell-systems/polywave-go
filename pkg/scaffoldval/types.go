package scaffoldval

// ValidationResult is the complete result of scaffold validation
type ValidationResult struct {
	Syntax         ValidationStep `yaml:"syntax"`
	Imports        ValidationStep `yaml:"imports"`
	TypeReferences ValidationStep `yaml:"type_references"`
	Build          ValidationStep `yaml:"build"`
}

// ValidationStep represents a single validation check
type ValidationStep struct {
	Status      string   `yaml:"status"` // PASS | FAIL | SKIP
	Errors      []string `yaml:"errors,omitempty"`
	Fixes       []string `yaml:"fixes,omitempty"`
	AutoFixable bool     `yaml:"auto_fixable,omitempty"`
}

// NewValidationResult creates a ValidationResult with all steps in SKIP state
func NewValidationResult() *ValidationResult {
	return &ValidationResult{
		Syntax:         ValidationStep{Status: "SKIP"},
		Imports:        ValidationStep{Status: "SKIP"},
		TypeReferences: ValidationStep{Status: "SKIP"},
		Build:          ValidationStep{Status: "SKIP"},
	}
}

// OverallStatus returns FAIL if any step failed, SKIP if all steps are skipped,
// or PASS if at least one step passed and none failed.
func (r *ValidationResult) OverallStatus() string {
	if r.Syntax.Status == "FAIL" || r.Imports.Status == "FAIL" ||
		r.TypeReferences.Status == "FAIL" || r.Build.Status == "FAIL" {
		return "FAIL"
	}
	if r.Syntax.Status == "PASS" || r.Imports.Status == "PASS" ||
		r.TypeReferences.Status == "PASS" || r.Build.Status == "PASS" {
		return "PASS"
	}
	return "SKIP"
}
