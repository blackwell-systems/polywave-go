package main

import (
	"context"
	"fmt"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/protocol"
	"github.com/spf13/cobra"
)

func newSolveCmd() *cobra.Command {
	var dryRun bool
	cmd := &cobra.Command{
		Use:   "solve <manifest-path>",
		Short: "Compute optimal wave assignments from dependency declarations",
		Long: "Runs the constraint solver on a YAML IMPL manifest. Computes wave " +
			"numbers from dependency declarations using topological sort. Rewrites " +
			"the manifest in-place with corrected wave assignments.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			manifestPath := args[0]

			// Load the manifest.
			m, err := protocol.Load(context.TODO(), manifestPath)
			if err != nil {
				return fmt.Errorf("solve: %w", err)
			}

			// Run the solver.
			fixed, changes, err := protocol.SolveManifest(m)
			if err != nil {
				return fmt.Errorf("solve: cannot solve: %w", err)
			}

			// Count agents and waves for summary.
			agentCount := 0
			for _, w := range m.Waves {
				agentCount += len(w.Agents)
			}

			// No changes needed.
			if len(changes) == 0 {
				fmt.Printf("✓ All wave assignments are optimal (%d agents, %d waves)\n", agentCount, len(m.Waves))
				return nil
			}

			// Print each change.
			for _, ch := range changes {
				fmt.Printf("solver: %s\n", ch)
			}

			// Dry run — don't write.
			if dryRun {
				fmt.Println("ⓘ Dry run — no changes written")
				return nil
			}

			// Write the corrected manifest.
			if saveRes := protocol.Save(context.TODO(), fixed, manifestPath); saveRes.IsFatal() {
				saveErrMsg := "save failed"
				if len(saveRes.Errors) > 0 {
					saveErrMsg = saveRes.Errors[0].Message
				}
				return fmt.Errorf("solve: writing manifest: %s", saveErrMsg)
			}

			fmt.Printf("✓ Manifest updated with %d wave reassignments (%d agents, %d waves)\n",
				len(changes), agentCount, len(fixed.Waves))
			return nil
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "print changes without writing")
	return cmd
}
