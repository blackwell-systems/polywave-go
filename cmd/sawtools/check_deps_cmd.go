package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/deps"
	"github.com/spf13/cobra"
)

func newCheckDepsCmd() *cobra.Command {
	var wave int

	cmd := &cobra.Command{
		Use:   "check-deps <impl-doc>",
		Short: "Detect dependency conflicts before wave execution",
		Long: `Analyzes IMPL doc file ownership and lock files to detect:
- Missing dependencies (imports not in lock file)
- Version conflicts (agents requiring different versions)

Prevents agents wasting time on dependency thrashing.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			implPath := args[0]

			// Run conflict detection
			res := deps.CheckDeps(implPath, wave)
			if !res.IsSuccess() {
				return fmt.Errorf("failed to check deps: %v", res.Errors)
			}

			report := res.GetData()
			// Output JSON
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			if err := enc.Encode(report); err != nil {
				return fmt.Errorf("failed to encode report: %w", err)
			}

			// Return error if conflicts found
			if len(report.MissingDeps) > 0 || len(report.VersionConflicts) > 0 {
				return fmt.Errorf("check-deps: dependency conflicts detected")
			}

			return nil
		},
	}

	cmd.Flags().IntVar(&wave, "wave", 0, "Wave number to check (0 = all waves)")

	return cmd
}
