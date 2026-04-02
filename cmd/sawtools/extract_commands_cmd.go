package main

import (
	"encoding/json"
	"fmt"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/commands"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

func newExtractCommandsCmd() *cobra.Command {
	var formatFlag string

	cmd := &cobra.Command{
		Use:   "extract-commands <repo-root>",
		Short: "Extract build/test/lint/format commands from CI and build system configs",
		Long: `Extracts build, test, lint, and format commands from CI configuration files
(GitHub Actions, GitLab CI, CircleCI) and build system files (Makefile, package.json).

Uses priority-based resolution to return the most specific/explicit command set found.
Falls back to language defaults when no config files are present.

Output format matches the Scout IMPL doc command specification.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			repoRoot := args[0]

			// Create extractor and register all parsers
			extractor := commands.New()
			extractor.RegisterCIParser(&commands.GithubActionsParser{})
			extractor.RegisterBuildSystemParser(&commands.MakefileParser{})
			extractor.RegisterBuildSystemParser(&commands.PackageJSONParser{})

			// Extract commands
			r := extractor.Extract(cmd.Context(), repoRoot)
			if r.IsFatal() {
				if len(r.Errors) > 0 {
					return fmt.Errorf("extract commands: %s", r.Errors[0].Message)
				}
				return fmt.Errorf("extract commands: unknown error")
			}
			commandSet := r.GetData().CommandSet

			// Serialize to requested format
			var data []byte
			var err error
			switch formatFlag {
			case "yaml":
				// Cannot use protocol.SaveYAML: marshaling to []byte for stdout, not to a file path.
				data, err = yaml.Marshal(commandSet)
				if err != nil {
					return fmt.Errorf("marshal yaml: %w", err)
				}
			case "json":
				data, err = json.MarshalIndent(commandSet, "", "  ")
				if err != nil {
					return fmt.Errorf("marshal json: %w", err)
				}
			default:
				return fmt.Errorf("unsupported format: %s (use yaml or json)", formatFlag)
			}

			fmt.Fprintln(cmd.OutOrStdout(), string(data))
			return nil
		},
	}

	cmd.Flags().StringVar(&formatFlag, "format", "yaml", "Output format: yaml or json")

	return cmd
}
