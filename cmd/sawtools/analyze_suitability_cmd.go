package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/suitability"
	"github.com/spf13/cobra"
)

func newAnalyzeSuitabilityCmd() *cobra.Command {
	var requirementsFlag string
	var repoRootFlag string
	var outputFlag string

	cmd := &cobra.Command{
		Use:   "analyze-suitability",
		Short: "Scan codebase for pre-implemented requirements",
		Long: `Analyzes a requirements or audit document to determine which items are
already implemented (DONE), partially implemented (PARTIAL), or not yet
implemented (TODO).

The requirements document should contain structured items in markdown format:

  ## F1: Add authentication handler
  Location: pkg/auth/handler.go

  ## F2: Add session timeout
  Location: pkg/session/timeout.go

Each item must include a "Location:" field specifying the file path.

Output is JSON with status, test coverage, and time savings estimates.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Read requirements document
			reqData, err := os.ReadFile(requirementsFlag)
			if err != nil {
				return fmt.Errorf("read requirements doc: %w", err)
			}

			// Parse requirements
			reqResult := suitability.ParseRequirements(string(reqData))
			if reqResult.IsFatal() {
				return fmt.Errorf("parse requirements: %v", reqResult.Errors[0])
			}
			requirements := reqResult.GetData()

			if len(requirements) == 0 {
				return fmt.Errorf("no valid requirements found in %s", requirementsFlag)
			}

			// Verify repo root exists
			if _, err := os.Stat(repoRootFlag); os.IsNotExist(err) {
				return fmt.Errorf("repo root does not exist: %s", repoRootFlag)
			} else if err != nil {
				return fmt.Errorf("failed to stat repo root: %w", err)
			}

			// Scan pre-implementation status
			result := suitability.ScanPreImplementation(repoRootFlag, requirements)
			if result.IsFatal() {
				return fmt.Errorf("scan pre-implementation: %v", result.Errors[0])
			}

			// Marshal to JSON
			var data []byte
			switch outputFlag {
			case "json":
				data, err = json.MarshalIndent(result.GetData(), "", "  ")
				if err != nil {
					return fmt.Errorf("marshal JSON: %w", err)
				}
			default:
				return fmt.Errorf("unsupported output format: %s (only json supported)", outputFlag)
			}

			fmt.Fprintln(cmd.OutOrStdout(), string(data))
			return nil
		},
	}

	cmd.Flags().StringVar(&requirementsFlag, "requirements", "", "Path to requirements/audit doc (markdown format)")
	cmd.Flags().StringVar(&repoRootFlag, "repo-root", ".", "Repository root directory")
	cmd.Flags().StringVar(&outputFlag, "output", "json", "Output format (json only)")
	cmd.MarkFlagRequired("requirements")

	return cmd
}
