package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/protocol"
	"github.com/spf13/cobra"
)

// validateOutput is the JSON structure printed by sawtools validate.
type validateOutput struct {
	Valid      bool              `json:"valid"`
	ErrorCount int               `json:"error_count"`
	Errors     []validateError   `json:"errors"`
}

// validateError is the normalized error shape emitted in the errors array.
type validateError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Field   string `json:"field,omitempty"`
	Line    int    `json:"line,omitempty"`
}

// validateFixOutput extends validateOutput with fix metadata.
type validateFixOutput struct {
	Valid      bool            `json:"valid"`
	ErrorCount int            `json:"error_count"`
	Errors     []validateError `json:"errors"`
	Fixed      int            `json:"fixed,omitempty"`
}

func newValidateCmd() *cobra.Command {
	var useSolver bool
	var autoFix bool
	cmd := &cobra.Command{
		Use:   "validate <manifest-path>",
		Short: "Validate a YAML IMPL manifest against protocol invariants",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			manifestPath := args[0]

			// Load the manifest; fail fast if unparseable.
			m, err := protocol.Load(manifestPath)
			if err != nil {
				return fmt.Errorf("validate: %w", err)
			}

			// Auto-fix correctable issues before validation.
			totalFixed := 0
			if autoFix {
				totalFixed += protocol.FixGateTypes(m)
				if totalFixed > 0 {
					if err := protocol.Save(m, manifestPath); err != nil {
						return fmt.Errorf("validate --fix: failed to write corrections: %w", err)
					}
				}

				// Strip unknown top-level keys from the raw YAML.
				rawFix, readFixErr := os.ReadFile(manifestPath)
				if readFixErr == nil {
					cleaned, stripped, stripErr := protocol.StripUnknownKeys(rawFix)
					if stripErr == nil && len(stripped) > 0 {
						if writeErr := os.WriteFile(manifestPath, cleaned, 0644); writeErr != nil {
							return fmt.Errorf("validate --fix: failed to write stripped YAML: %w", writeErr)
						}
						totalFixed += len(stripped)
						// Re-load manifest after stripping keys.
						m, err = protocol.Load(manifestPath)
						if err != nil {
							return fmt.Errorf("validate --fix: reload after strip: %w", err)
						}
					}
				}
			}

			var errs []validateError

			// Read raw YAML for unknown key detection.
			rawData, readErr := os.ReadFile(manifestPath)
			if readErr == nil {
				ukErrs := protocol.DetectUnknownKeys(rawData)
				for _, uke := range ukErrs {
					errs = append(errs, validateError{
						Code:    uke.Code,
						Message: uke.Message,
						Field:   uke.Field,
					})
				}
			}

			// Run struct-level invariant checks (solver-based or standard).
			var structErrs []protocol.ValidationError
			if useSolver {
				structErrs = protocol.ValidateWithSolver(m)
			} else {
				structErrs = protocol.Validate(m)
			}
			for _, ve := range structErrs {
				errs = append(errs, validateError{
					Code:    ve.Code,
					Message: ve.Message,
					Field:   ve.Field,
					Line:    ve.Line,
				})
			}

			// Run E16 typed-block validation (returns nil for YAML files; that is fine).
			docErrs, docErr := protocol.ValidateIMPLDoc(manifestPath)
			if docErr != nil {
				return fmt.Errorf("validate: %w", docErr)
			}
			for _, de := range docErrs {
				errs = append(errs, validateError{
					Code:    de.BlockType,
					Message: de.Message,
					Line:    de.LineNumber,
				})
			}

			// Ensure the errors field is always a non-nil JSON array.
			if errs == nil {
				errs = []validateError{}
			}

			result := validateFixOutput{
				Valid:      len(errs) == 0,
				ErrorCount: len(errs),
				Errors:     errs,
				Fixed:      totalFixed,
			}

			out, _ := json.MarshalIndent(result, "", "  ")
			fmt.Println(string(out))

			if !result.Valid {
				os.Exit(1)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&useSolver, "solver", false, "use CSP solver for wave assignment validation")
	cmd.Flags().BoolVar(&autoFix, "fix", false, "auto-correct fixable issues (e.g. invalid gate types -> custom, unknown keys -> stripped)")
	return cmd
}
