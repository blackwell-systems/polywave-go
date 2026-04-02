package main

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/protocol"
	"github.com/spf13/cobra"
)

func newSuggestWaveStructureCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "suggest-wave-structure <manifest-path>",
		Short: "Validate wave structure for interface changes",
		Long: `Validates that all callers of any changed interface are either
in the same wave as the change, or in a downstream wave with correct depends_on.
Returns JSON report of wave structure problems.

Exit code 0: no problems found (or SUCCESS/PARTIAL with only warnings).
Exit code 1: problems found that would cause E21A failures.

Example:
  sawtools suggest-wave-structure docs/IMPL/IMPL-feature.yaml --repo-dir /path/to/repo`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			manifestPath := args[0]
			m, err := protocol.Load(context.TODO(), manifestPath)
			if err != nil {
				return fmt.Errorf("failed to load manifest: %w", err)
			}
			res := protocol.SuggestWaveStructure(context.TODO(), m, repoDir)
			if res.IsFatal() {
				for _, e := range res.Errors {
					fmt.Fprintln(cmd.ErrOrStderr(), e.Error())
				}
				return fmt.Errorf("suggest-wave-structure failed")
			}
			problems := res.GetData()
			out, err := json.MarshalIndent(problems, "", "  ")
			if err != nil {
				return fmt.Errorf("marshal output: %w", err)
			}
			fmt.Fprintln(cmd.OutOrStdout(), string(out))
			if len(problems) > 0 {
				return fmt.Errorf("suggest-wave-structure: %d problem(s) found", len(problems))
			}
			return nil
		},
	}
	return cmd
}
