package main

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/analyzer"
	"github.com/blackwell-systems/scout-and-wave-go/pkg/protocol"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

func newDetectWiringCmd() *cobra.Command {
	var format string

	cmd := &cobra.Command{
		Use:   "detect-wiring <impl-doc-path>",
		Short: "Detect cross-agent function calls and generate wiring declarations",
		Long: `Analyze IMPL document agent task prompts for cross-agent calls.

Outputs wiring declarations in YAML or JSON format. Wiring declarations specify
which functions must be called from which files to satisfy cross-agent dependencies.

Patterns detected:
- "calls FunctionName()"
- "uses pkg.FunctionName"
- "delegates to X"
- "invokes FunctionName"

Exit code 0 if analysis succeeds (even if no wiring declarations found).
Exit code 1 if IMPL doc is malformed or repo root cannot be determined.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			implDocPath := args[0]

			// Validate format flag
			if format != "yaml" && format != "json" {
				return fmt.Errorf("invalid --format value: must be 'yaml' or 'json'")
			}

			// Load the manifest
			manifest, err := protocol.Load(context.TODO(), implDocPath)
			if err != nil {
				return fmt.Errorf("error: failed to parse IMPL doc: %w", err)
			}

			// Determine repo root
			repoRoot, err := findRepoRoot(implDocPath, manifest)
			if err != nil {
				return fmt.Errorf("error: could not determine repo root from %s: %w", implDocPath, err)
			}

			// Call DetectWiring
			declarations, err := analyzer.DetectWiring(manifest, repoRoot)
			if err != nil {
				return fmt.Errorf("error: detection failed: %w", err)
			}

			// Prepare output structure
			output := map[string]interface{}{
				"wiring": declarations,
			}

			// Marshal to requested format
			var outputBytes []byte
			if format == "yaml" {
				outputBytes, err = yaml.Marshal(output)
			} else {
				outputBytes, err = json.MarshalIndent(output, "", "  ")
			}
			if err != nil {
				return fmt.Errorf("error: failed to marshal output: %w", err)
			}

			// Write to stdout
			fmt.Println(string(outputBytes))
			return nil
		},
	}

	cmd.Flags().StringVar(&format, "format", "yaml", "Output format: yaml or json")

	return cmd
}
