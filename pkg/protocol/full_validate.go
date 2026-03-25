package protocol

import (
	"fmt"
	"os"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/result"
)

// FullValidateData holds the result of a complete manifest validation.
type FullValidateData struct {
	Valid      bool              `json:"valid"`
	ErrorCount int              `json:"error_count"`
	Errors     []result.SAWError `json:"errors"`
	Fixed      int              `json:"fixed,omitempty"`
}

// FullValidateOpts controls validation behavior.
type FullValidateOpts struct {
	AutoFix   bool // auto-correct fixable issues (gate types, unknown keys)
	UseSolver bool // use CSP solver for wave assignment validation
}

// FullValidate runs all validation checks on an IMPL manifest file.
// This is the single entry point for CLI, web, and programmatic validation.
// When AutoFix is true, correctable issues are fixed in-place before validation.
func FullValidate(manifestPath string, opts FullValidateOpts) result.Result[FullValidateData] {
	// Step 1: Load manifest
	m, err := Load(manifestPath)
	if err != nil {
		return result.NewFailure[FullValidateData]([]result.SAWError{{
			Code:     "V001_MANIFEST_INVALID",
			Message:  fmt.Sprintf("failed to load manifest: %v", err),
			Severity: "fatal",
		}})
	}

	totalFixed := 0

	// Step 2: Auto-fix correctable issues
	if opts.AutoFix {
		totalFixed += FixGateTypes(m)
		if totalFixed > 0 {
			if err := Save(m, manifestPath); err != nil {
				return result.NewFailure[FullValidateData]([]result.SAWError{{
					Code:     "V001_MANIFEST_INVALID",
					Message:  fmt.Sprintf("failed to write corrections: %v", err),
					Severity: "fatal",
				}})
			}
		}

		// Strip unknown top-level keys from the raw YAML.
		rawFix, readFixErr := os.ReadFile(manifestPath)
		if readFixErr == nil {
			cleaned, stripped, stripErr := StripUnknownKeys(rawFix)
			if stripErr == nil && len(stripped) > 0 {
				if writeErr := os.WriteFile(manifestPath, cleaned, 0644); writeErr != nil {
					return result.NewFailure[FullValidateData]([]result.SAWError{{
						Code:     "V001_MANIFEST_INVALID",
						Message:  fmt.Sprintf("failed to write stripped YAML: %v", writeErr),
						Severity: "fatal",
					}})
				}
				totalFixed += len(stripped)
				// Re-load manifest after stripping keys.
				m, err = Load(manifestPath)
				if err != nil {
					return result.NewFailure[FullValidateData]([]result.SAWError{{
						Code:     "V001_MANIFEST_INVALID",
						Message:  fmt.Sprintf("reload after strip: %v", err),
						Severity: "fatal",
					}})
				}
			}
		}
	}

	var errs []result.SAWError

	// Step 3: Read raw YAML for byte-level checks
	rawData, readErr := os.ReadFile(manifestPath)
	if readErr == nil {
		// Step 4: Duplicate key detection
		dkErrs := ValidateDuplicateKeys(rawData)
		errs = append(errs, dkErrs...)

		// Step 5: Unknown key detection
		ukErrs := DetectUnknownKeys(rawData)
		errs = append(errs, ukErrs...)
	}

	// Step 6: Struct-level invariant checks
	if opts.UseSolver {
		errs = append(errs, ValidateWithSolver(m)...)
	} else {
		errs = append(errs, Validate(m)...)
	}

	// Step 7: E16 typed-block validation
	docErrs, docErr := ValidateIMPLDoc(manifestPath)
	if docErr != nil {
		errs = append(errs, result.SAWError{
			Code:     "V001_MANIFEST_INVALID",
			Message:  fmt.Sprintf("typed-block validation error: %v", docErr),
			Severity: "error",
		})
	} else {
		errs = append(errs, docErrs...)
	}

	// Ensure errors is never nil
	if errs == nil {
		errs = []result.SAWError{}
	}

	data := FullValidateData{
		Valid:      len(errs) == 0,
		ErrorCount: len(errs),
		Errors:     errs,
		Fixed:      totalFixed,
	}

	if data.Valid {
		return result.NewSuccess(data)
	}
	return result.NewPartial(data, errs)
}

// FullValidateProgramData holds the result of program manifest validation.
type FullValidateProgramData struct {
	Valid      bool              `json:"valid"`
	ErrorCount int              `json:"error_count"`
	Errors     []result.SAWError `json:"errors"`
}

// FullValidateProgramOpts controls program validation behavior.
type FullValidateProgramOpts struct {
	ImportMode bool   // run import-mode checks (P1/P2 against on-disk IMPLs)
	RepoDir    string // repo root for import-mode (default: cwd)
}

// FullValidateProgram runs all validation checks on a PROGRAM manifest file.
func FullValidateProgram(manifestPath string, opts FullValidateProgramOpts) result.Result[FullValidateProgramData] {
	// Step 1: Parse manifest
	manifest, err := ParseProgramManifest(manifestPath)
	if err != nil {
		return result.NewFailure[FullValidateProgramData]([]result.SAWError{{
			Code:     "P000_PARSE_ERROR",
			Message:  fmt.Sprintf("failed to parse program manifest: %v", err),
			Severity: "fatal",
		}})
	}

	// Step 2: Run validation
	validationErrs := ValidateProgram(manifest)

	// Step 3: Optional import-mode checks
	if opts.ImportMode {
		rd := opts.RepoDir
		if rd == "" {
			rd, _ = os.Getwd()
		}
		importErrs := ValidateProgramImportMode(manifest, rd)
		validationErrs = append(validationErrs, importErrs...)
	}

	// Ensure errors is never nil
	if validationErrs == nil {
		validationErrs = []result.SAWError{}
	}

	data := FullValidateProgramData{
		Valid:      len(validationErrs) == 0,
		ErrorCount: len(validationErrs),
		Errors:     validationErrs,
	}

	if data.Valid {
		return result.NewSuccess(data)
	}
	return result.NewPartial(data, validationErrs)
}
