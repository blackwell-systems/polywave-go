package main

import (
	"fmt"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/scaffoldval"
)

func newValidateScaffoldCmd() *cobra.Command {
	var implDoc string

	cmd := &cobra.Command{
		Use:   "validate-scaffold <scaffold-file>",
		Short: "Validate scaffold file before committing",
		Long: `Runs validation pipeline on scaffold file:
1. Syntax check (parses Go AST)
2. Import resolution (checks if imports exist)
3. Type references (checks for undeclared types)
4. Partial build (compiles scaffold in isolation)

Outputs structured YAML with pass/fail status per step.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			scaffoldPath := args[0]

			if implDoc == "" {
				return fmt.Errorf("--impl-doc is required")
			}

			// Run validation
			result, err := scaffoldval.ValidateScaffold(scaffoldPath, implDoc)
			if err != nil {
				return fmt.Errorf("validation failed: %w", err)
			}

			// Output YAML to command's output stream (for testability).
			// Cannot use protocol.SaveYAML: writes to an io.Writer (stdout), not a file path,
			// and uses SetIndent(2) which SaveYAML does not support.
			enc := yaml.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent(2)
			if err := enc.Encode(result); err != nil {
				return fmt.Errorf("failed to encode result: %w", err)
			}

			// Return error if FAIL
			if result.OverallStatus() == "FAIL" {
				return fmt.Errorf("validate-scaffold: validation failed")
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&implDoc, "impl-doc", "", "Path to IMPL doc (for build command extraction)")
	cmd.MarkFlagRequired("impl-doc")

	return cmd
}
