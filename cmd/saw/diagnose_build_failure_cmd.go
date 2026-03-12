package main

import (
	"fmt"
	"os"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/builddiag"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

func newDiagnoseBuildFailureCmd() *cobra.Command {
	var language string

	cmd := &cobra.Command{
		Use:   "diagnose-build-failure <error-log>",
		Short: "Pattern-match build errors and suggest fixes",
		Long: `Analyzes build error logs against known pattern catalogs.

Supported languages: go, rust, javascript, typescript, python

Outputs structured YAML with:
- diagnosis: Pattern name
- confidence: Match confidence (0.0-1.0)
- fix: Recommended action
- rationale: Why this fix works
- auto_fixable: Whether fix can be automated`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			errorLogPath := args[0]

			// Read error log
			content, err := os.ReadFile(errorLogPath)
			if err != nil {
				return fmt.Errorf("failed to read error log: %w", err)
			}

			// Diagnose error
			diagnosis, err := builddiag.DiagnoseError(string(content), language)
			if err != nil {
				return fmt.Errorf("failed to diagnose error: %w", err)
			}

			// Output YAML to command's configured output
			enc := yaml.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent(2)
			if err := enc.Encode(diagnosis); err != nil {
				return fmt.Errorf("failed to encode diagnosis: %w", err)
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&language, "language", "", "Language (go, rust, js, ts, python) [required]")
	cmd.MarkFlagRequired("language")

	return cmd
}
