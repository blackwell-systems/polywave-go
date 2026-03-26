package main

import (
	"encoding/json"
	"fmt"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/analyzer"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

func newDetectCascadesCmd() *cobra.Command {
	var renamesFlag string

	cmd := &cobra.Command{
		Use:   "detect-cascades <repo-root>",
		Short: "Detect files affected by type renames",
		Long: `Analyzes a repository to detect files that reference renamed types.
Outputs cascade candidates with severity levels and reasons.

The --renames flag accepts a JSON array of rename objects:
  [{"old":"AuthToken","new":"SessionToken","scope":"pkg/auth"}]

Output format is YAML matching the CascadeResult schema.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			repoRoot := args[0]

			// Parse renames JSON
			var renames []analyzer.RenameInfo
			if err := json.Unmarshal([]byte(renamesFlag), &renames); err != nil {
				return fmt.Errorf("invalid --renames JSON: %w", err)
			}

			// Validate renames
			if len(renames) == 0 {
				return fmt.Errorf("--renames must contain at least one rename")
			}
			for i, r := range renames {
				if r.Old == "" || r.New == "" {
					return fmt.Errorf("rename at index %d: old and new fields are required", i)
				}
			}

			// Detect cascades
			result, err := analyzer.DetectCascades(repoRoot, renames)
			if err != nil {
				return fmt.Errorf("detect cascades: %w", err)
			}

			// Output YAML.
			// Cannot use protocol.SaveYAML: marshaling to []byte for stdout, not to a file path.
			data, err := yaml.Marshal(&result)
			if err != nil {
				return fmt.Errorf("marshal YAML: %w", err)
			}

			fmt.Fprintln(cmd.OutOrStdout(), string(data))
			return nil
		},
	}

	cmd.Flags().StringVar(&renamesFlag, "renames", "", "JSON array of renames (required)")
	cmd.MarkFlagRequired("renames")

	return cmd
}
