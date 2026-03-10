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

func newValidateCmd() *cobra.Command {
	return &cobra.Command{
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

			var errs []validateError

			// Run struct-level I1-I6 invariant checks.
			for _, ve := range protocol.Validate(m) {
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

			result := validateOutput{
				Valid:      len(errs) == 0,
				ErrorCount: len(errs),
				Errors:     errs,
			}

			out, _ := json.MarshalIndent(result, "", "  ")
			fmt.Println(string(out))

			if !result.Valid {
				os.Exit(1)
			}
			return nil
		},
	}
}
