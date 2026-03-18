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
	cmd := &cobra.Command{
		Use:   "validate-program <program-manifest>",
		Short: "Validate a YAML PROGRAM manifest against schema rules",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			manifestPath := args[0]

			// Parse the manifest; fail fast if unparseable (exit 2)
			manifest, err := protocol.ParseProgramManifest(manifestPath)
			if err != nil {
				// Parse error - exit code 2
				fmt.Fprintf(os.Stderr, "validate-program: parse error: %v\n", err)
				os.Exit(2)
			}

			// Run validation
			validationErrs := protocol.ValidateProgram(manifest)

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

			// Exit code 1 if validation errors, 0 if valid
			if !result.Valid {
				os.Exit(1)
			}
			return nil
		},
	}
	return cmd
}
