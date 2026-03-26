package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/protocol"
	"github.com/spf13/cobra"
)

// validateProgramOutput is the JSON structure printed by sawtools validate-program.
type validateProgramOutput struct {
	Valid  bool                        `json:"valid"`
	Errors []validateProgramError      `json:"errors"`
}

// validateProgramError is the normalized error shape emitted in the errors array.
type validateProgramError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Field   string `json:"field,omitempty"`
}

func newValidateProgramCmd() *cobra.Command {
	var (
		importMode bool
		repoDir    string
	)

	cmd := &cobra.Command{
		Use:   "validate-program <program-manifest>",
		Short: "Validate a YAML PROGRAM manifest against schema rules",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			manifestPath := args[0]

			// Parse the manifest; fail fast if unparseable
			manifest, err := protocol.ParseProgramManifest(manifestPath)
			if err != nil {
				return fmt.Errorf("validate-program: parse error: %v", err)
			}

			// Run validation
			validationErrs := protocol.ValidateProgram(manifest)

			// If --import-mode, also run import-mode validation
			if importMode {
				rd := repoDir
				if rd == "" {
					rd, _ = os.Getwd()
				}
				importErrs := protocol.ValidateProgramImportMode(manifest, rd)
				validationErrs = append(validationErrs, importErrs...)
			}

			// Convert validation errors to output format
			var errs []validateProgramError
			for _, ve := range validationErrs {
				errs = append(errs, validateProgramError{
					Code:    ve.Code,
					Message: ve.Message,
					Field:   ve.Field,
				})
			}

			// Ensure errors is always a non-nil JSON array
			if errs == nil {
				errs = []validateProgramError{}
			}

			result := validateProgramOutput{
				Valid:  len(errs) == 0,
				Errors: errs,
			}

			out, _ := json.MarshalIndent(result, "", "  ")
			fmt.Fprintln(cmd.OutOrStdout(), string(out))

			// Return error if validation errors
			if !result.Valid {
				return fmt.Errorf("validate-program: validation failed")
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&importMode, "import-mode", false, "Also run import-mode validation (P1/P2 checks against on-disk IMPL docs)")
	cmd.Flags().StringVar(&repoDir, "repo-dir", "", "Repository root directory for import-mode checks (default: cwd)")

	return cmd
}
