package main

import (
	"fmt"
	"strings"

	"github.com/blackwell-systems/polywave-go/pkg/analyzer"
	"github.com/spf13/cobra"
)

func newAnalyzeDepsCmd() *cobra.Command {
	var filesFlag string
	var formatFlag string

	cmd := &cobra.Command{
		Use:   "analyze-deps <repo-root>",
		Short: "Analyze Go file dependencies and compute wave structure",
		Long: `Analyzes Go source files to extract import dependencies, detect cycles,
compute topological sort, and assign wave structure for parallel agent execution.

Output format matches Scout IMPL doc dependency graph schema.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			repoRoot := args[0]

			// Parse files flag (comma-separated list)
			if filesFlag == "" {
				return fmt.Errorf("--files flag is required")
			}
			files := strings.Split(filesFlag, ",")
			for i := range files {
				files[i] = strings.TrimSpace(files[i])
			}

			// Build dependency graph
			graphResult := analyzer.BuildGraph(cmd.Context(), repoRoot, files)
			if graphResult.IsFatal() {
				return fmt.Errorf("analyze-deps: %s", graphResult.Errors[0].Message)
			}

			// Convert to output format
			output := analyzer.ToOutput(graphResult.GetData())

			// Serialize
			var data []byte
			switch formatFlag {
			case "yaml":
				fmtResult := analyzer.FormatYAML(output)
				if fmtResult.IsFatal() {
					return fmt.Errorf("format output: %s", fmtResult.Errors[0].Message)
				}
				data = fmtResult.GetData()
			case "json":
				fmtResult := analyzer.FormatJSON(output)
				if fmtResult.IsFatal() {
					return fmt.Errorf("format output: %s", fmtResult.Errors[0].Message)
				}
				data = fmtResult.GetData()
			default:
				return fmt.Errorf("unsupported format: %s (use yaml or json)", formatFlag)
			}

			fmt.Fprintln(cmd.OutOrStdout(), string(data))
			return nil
		},
	}

	cmd.Flags().StringVar(&filesFlag, "files", "", "Comma-separated list of Go files to analyze (required)")
	cmd.Flags().StringVar(&formatFlag, "format", "yaml", "Output format: yaml or json")
	cmd.MarkFlagRequired("files")

	return cmd
}
