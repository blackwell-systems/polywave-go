package main

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/idgen"
	"github.com/spf13/cobra"
)

func newAssignAgentIDsCmd() *cobra.Command {
	var count int
	var groupingJSON string

	cmd := &cobra.Command{
		Use:   "assign-agent-ids",
		Short: "Generate agent IDs following the ^[A-Z][2-9]?$ pattern",
		Long: `Generate agent IDs following the ^[A-Z][2-9]?$ pattern.

Supports two modes:
1. Sequential mode (no --grouping): A-Z for first 26, then A2-Z2, etc.
2. Grouped mode (with --grouping): Groups agents by category tags and assigns
   multi-generation IDs within each group.

Examples:
  # Sequential mode: 30 agents
  sawtools assign-agent-ids --count 30
  # Output: A B C ... Z A2 B2 C2 D2

  # Grouped mode: 9 agents with categories
  sawtools assign-agent-ids --count 9 --grouping '[["data"],["data"],["data"],["api"],["api"],["ui"],["ui"],["ui"],["ui"]]'
  # Output: A A2 A3 B B2 C C2 C3 C4
`,
		RunE: func(cmd *cobra.Command, args []string) error {
			var grouping [][]string

			// Parse grouping JSON if provided (check if flag was set, not just if value is non-empty)
			if cmd.Flags().Changed("grouping") {
				if err := json.Unmarshal([]byte(groupingJSON), &grouping); err != nil {
					return fmt.Errorf("invalid --grouping JSON: %w", err)
				}
			}

			// Generate IDs
			ids, err := idgen.AssignAgentIDs(count, grouping)
			if err != nil {
				return err
			}

			// Output space-separated IDs
			fmt.Fprintln(cmd.OutOrStdout(), strings.Join(ids, " "))
			return nil
		},
	}

	cmd.Flags().IntVar(&count, "count", 0, "Number of agents (required)")
	cmd.Flags().StringVar(&groupingJSON, "grouping", "", "JSON array of category tags (optional)")
	cmd.MarkFlagRequired("count")

	return cmd
}
