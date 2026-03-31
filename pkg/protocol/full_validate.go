package protocol

import (
	"fmt"
	"os"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/result"
)

// FullValidateData holds the result of a complete manifest validation.
type FullValidateData struct {
	Valid        bool              `json:"valid"`
	ErrorCount   int               `json:"error_count"`
	Errors       []result.SAWError `json:"errors"`
	WarningCount int               `json:"warning_count,omitempty"`
	Warnings     []result.SAWError `json:"warnings,omitempty"`
	Fixed        int               `json:"fixed,omitempty"`
}

// FullValidateOpts controls validation behavior.
type FullValidateOpts struct {
	AutoFix   bool   // auto-correct fixable issues (gate types, unknown keys)
	UseSolver bool   // use CSP solver for wave assignment validation
	RepoPath  string // optional: absolute path to repo root for file-existence checks
}

// FullValidate runs all validation checks on an IMPL manifest file.
// This is the single entry point for CLI (sawtools validate), web API,
// and programmatic validation when you have a file path.
//
// Pipeline (in order):
//  1. Load(path) — parse YAML manifest
//  2. If AutoFix: FixGateTypes(m) + Save; StripUnknownKeys + WriteFile
//  3. Validate(m) — structural/invariant checks
//  4. ValidateIMPLDoc(path) — E16 typed-block checks
//
// Use FullValidate in preference to calling Validate + ValidateIMPLDoc separately,
// since it handles auto-fix, deduplication, and severity-based filtering.
func FullValidate(manifestPath string, opts FullValidateOpts) result.Result[FullValidateData] {
	// Step 1: Load manifest
	m, err := Load(manifestPath)
	if err != nil {
		return result.NewFailure[FullValidateData]([]result.SAWError{{
			Code:     result.CodeManifestInvalid,
			Message:  fmt.Sprintf("failed to load manifest: %v", err),
			Severity: "fatal",
		}})
	}

	totalFixed := 0

	// Step 2: Auto-fix correctable issues
	if opts.AutoFix {
		totalFixed += FixGateTypes(m)
		if totalFixed > 0 {
			if saveRes := Save(m, manifestPath); saveRes.IsFatal() {
				return result.NewFailure[FullValidateData](saveRes.Errors)
			}
		}

		// Strip unknown top-level keys from the raw YAML.
		rawFix, readFixErr := os.ReadFile(manifestPath)
		if readFixErr == nil {
			cleaned, stripped, stripErr := StripUnknownKeys(rawFix)
			if stripErr == nil && len(stripped) > 0 {
				if writeErr := os.WriteFile(manifestPath, cleaned, 0644); writeErr != nil {
					return result.NewFailure[FullValidateData]([]result.SAWError{{
						Code:     result.CodeManifestInvalid,
						Message:  fmt.Sprintf("failed to write stripped YAML: %v", writeErr),
						Severity: "fatal",
					}})
				}
				totalFixed += len(stripped)
				// Re-load manifest after stripping keys.
				m, err = Load(manifestPath)
				if err != nil {
					return result.NewFailure[FullValidateData]([]result.SAWError{{
						Code:     result.CodeManifestInvalid,
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

	// Step 6b: File-existence check using the caller-supplied RepoPath.
	// ValidateFileExistence is also called (with empty string) inside Validate()
	// for backward compatibility; this call uses the real repo path when available.
	feErrs := ValidateFileExistence(m, opts.RepoPath)
	errs = append(errs, feErrs...)

	// TODO(saw-system-hardening): wired by Agent B — CheckAgentLOCBudget call goes here
	// errs = append(errs, CheckAgentLOCBudget(m, opts.RepoPath, 2000)...)

	// Step 7: E16 typed-block validation
	docErrs, docErr := ValidateIMPLDoc(manifestPath)
	if docErr != nil {
		errs = append(errs, result.SAWError{
			Code:     result.CodeManifestInvalid,
			Message:  fmt.Sprintf("typed-block validation error: %v", docErr),
			Severity: "error",
		})
	} else {
		errs = append(errs, docErrs...)
	}

	// Split errors by severity: warnings/info are advisory, others are blocking.
	var blockingErrs []result.SAWError
	var warnings []result.SAWError
	for _, e := range errs {
		if e.Severity == "warning" || e.Severity == "info" {
			warnings = append(warnings, e)
		} else {
			blockingErrs = append(blockingErrs, e)
		}
	}
	if blockingErrs == nil {
		blockingErrs = []result.SAWError{}
	}
	if warnings == nil {
		warnings = []result.SAWError{}
	}

	data := FullValidateData{
		Valid:        len(blockingErrs) == 0,
		ErrorCount:   len(blockingErrs),
		Errors:       blockingErrs,
		WarningCount: len(warnings),
		Warnings:     warnings,
		Fixed:        totalFixed,
	}

	if data.Valid {
		return result.NewSuccess(data)
	}
	return result.NewPartial(data, blockingErrs)
}

// FullValidateProgramData holds the result of program manifest validation.
type FullValidateProgramData struct {
	Valid        bool              `json:"valid"`
	ErrorCount   int               `json:"error_count"`
	Errors       []result.SAWError `json:"errors"`
	WarningCount int               `json:"warning_count,omitempty"`
	Warnings     []result.SAWError `json:"warnings,omitempty"`
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
			Code:     result.CodeParseError,
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

	// Split errors by severity: warnings/info are advisory, others are blocking.
	var blockingErrs []result.SAWError
	var warnings []result.SAWError
	for _, e := range validationErrs {
		if e.Severity == "warning" || e.Severity == "info" {
			warnings = append(warnings, e)
		} else {
			blockingErrs = append(blockingErrs, e)
		}
	}
	if blockingErrs == nil {
		blockingErrs = []result.SAWError{}
	}
	if warnings == nil {
		warnings = []result.SAWError{}
	}

	data := FullValidateProgramData{
		Valid:        len(blockingErrs) == 0,
		ErrorCount:   len(blockingErrs),
		Errors:       blockingErrs,
		WarningCount: len(warnings),
		Warnings:     warnings,
	}

	if data.Valid {
		return result.NewSuccess(data)
	}
	return result.NewPartial(data, blockingErrs)
}
