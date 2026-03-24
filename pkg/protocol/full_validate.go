package protocol

import (
	"github.com/blackwell-systems/scout-and-wave-go/pkg/result"
)

// FullValidateOpts controls optional behaviour of FullValidate.
type FullValidateOpts struct {
	// Reserved for future options (e.g. strict mode, skip checks).
}

// ValidationResult is the data payload returned by FullValidate.
type ValidationResult struct {
	Valid  bool             `json:"valid"`
	Errors []result.SAWError `json:"errors,omitempty"`
}

// FullValidate runs all validation checks on the IMPL doc at yamlPath:
//  1. Load the manifest (parse + struct validation via Validate)
//  2. E16 typed-block validation (ValidateIMPLDoc)
//
// Returns a fatal Result if the file cannot be loaded at all.
// Returns a non-fatal Result with Valid=false if validation errors exist.
// Returns a non-fatal Result with Valid=true if all checks pass.
func FullValidate(yamlPath string, _ FullValidateOpts) result.Result[*ValidationResult] {
	// Step 1: parse + struct validation
	m, err := Load(yamlPath)
	if err != nil {
		return result.NewFailure[*ValidationResult]([]result.SAWError{{
			Code:     result.CodeIMPLParseFailed,
			Message:  "failed to load manifest: " + err.Error(),
			Severity: "fatal",
			File:     yamlPath,
		}})
	}

	var errs []result.SAWError

	// Struct validation (required fields, disjoint ownership, etc.)
	errs = append(errs, Validate(m)...)

	// Step 2: E16 typed-block validation
	blockErrs, blockErr := ValidateIMPLDoc(yamlPath)
	if blockErr != nil {
		return result.NewFailure[*ValidationResult]([]result.SAWError{{
			Code:     result.CodeIMPLParseFailed,
			Message:  "typed-block validation failed: " + blockErr.Error(),
			Severity: "fatal",
			File:     yamlPath,
		}})
	}
	errs = append(errs, blockErrs...)

	return result.NewSuccess(&ValidationResult{
		Valid:  len(errs) == 0,
		Errors: errs,
	})
}
