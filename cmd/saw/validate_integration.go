package main

import (
	"encoding/json"
	"fmt"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/protocol"
	"github.com/spf13/cobra"
)

func newValidateIntegrationCmd() *cobra.Command {
	var waveNum int

	cmd := &cobra.Command{
		Use:   "validate-integration <manifest-path>",
		Short: "Validate integration gaps after a wave completes",
		Long: `Scans a completed wave for unconnected exports using Go AST analysis.
Loads the IMPL manifest, calls ValidateIntegration to detect gaps, persists
the report back to the manifest, and prints the report as JSON.

Exits 0 if no gaps found (report.Valid), exits 1 if gaps are detected.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			manifestPath := args[0]

			// Step 1: Load manifest
			manifest, err := protocol.Load(manifestPath)
			if err != nil {
				return fmt.Errorf("validate-integration: failed to load manifest: %w", err)
			}

			// Step 2: Run integration validation
			report, err := protocol.ValidateIntegration(manifest, waveNum, repoDir)
			if err != nil {
				return fmt.Errorf("validate-integration: validation failed: %w", err)
			}

			// Step 3: Persist report to manifest
			waveKey := fmt.Sprintf("wave%d", waveNum)
			if err := protocol.AppendIntegrationReport(manifestPath, waveKey, report); err != nil {
				return fmt.Errorf("validate-integration: failed to persist report: %w", err)
			}

			// Step 4: Print report as indented JSON
			out, err := json.MarshalIndent(report, "", "  ")
			if err != nil {
				return fmt.Errorf("validate-integration: failed to marshal report: %w", err)
			}
			fmt.Println(string(out))

			// Step 5: Exit 1 if gaps found
			if !report.Valid {
				return fmt.Errorf("validate-integration: %d integration gap(s) detected", len(report.Gaps))
			}

			return nil
		},
	}

	cmd.Flags().IntVar(&waveNum, "wave", 0, "Wave number (required)")
	_ = cmd.MarkFlagRequired("wave")

	return cmd
}
