package main

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/blackwell-systems/polywave-go/pkg/protocol"
	"github.com/spf13/cobra"
)

func newPredictConflictsCmd() *cobra.Command {
	var waveNum int

	cmd := &cobra.Command{
		Use:   "predict-conflicts <manifest-path>",
		Short: "Predict merge conflicts for a wave using hunk-level diff analysis (E11)",
		Long: `Runs E11 conflict prediction against the completion reports for a wave.

For each file that appears in multiple agents' reports, performs hunk-level
diff analysis: git diff --unified=0 mergeBase..branch -- file for each agent,
parses @@ -a,b @@ ranges, and checks whether any two agents' modified line
ranges overlap.

Results:
  - success (exit 0): no conflicts — safe to merge
  - partial (exit 1): overlapping hunks detected — merge conflict likely

Non-overlapping edits (cascade patches where each agent modifies a different
function in the same file) produce exit 0. Only true line-range overlaps are
flagged.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			manifestPath := args[0]

			m, err := protocol.Load(context.TODO(), manifestPath)
			if err != nil {
				return fmt.Errorf("predict-conflicts: %w", err)
			}

			res := protocol.PredictConflictsFromReports(context.TODO(), m, waveNum)
			data := res.GetData()

			output := struct {
				ConflictsDetected int                          `json:"conflicts_detected"`
				Conflicts         []protocol.ConflictPrediction `json:"conflicts"`
				Warnings          []string                     `json:"warnings,omitempty"`
			}{
				ConflictsDetected: data.ConflictsDetected,
				Conflicts:         data.Conflicts,
			}
			for _, e := range res.Errors {
				output.Warnings = append(output.Warnings, e.Message)
			}

			out, _ := json.MarshalIndent(output, "", "  ")
			fmt.Println(string(out))

			if res.IsPartial() {
				return fmt.Errorf("predict-conflicts: %d file(s) have overlapping edits (merge conflict likely)", data.ConflictsDetected)
			}
			return nil
		},
	}

	cmd.Flags().IntVar(&waveNum, "wave", 0, "Wave number to check (required)")
	_ = cmd.MarkFlagRequired("wave")

	return cmd
}
