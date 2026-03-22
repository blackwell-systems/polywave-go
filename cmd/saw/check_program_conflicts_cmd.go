// Registration: add newCheckProgramConflictsCmd() to main.go AddCommand list.

package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/protocol"
	"github.com/spf13/cobra"
)

func newCheckProgramConflictsCmd() *cobra.Command {
	var tierNum int

	cmd := &cobra.Command{
		Use:   "check-program-conflicts <program-manifest> --tier N",
		Short: "Detect file ownership conflicts across IMPLs in a program tier",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			manifestPath := args[0]

			report, exitCode, err := runCheckProgramConflicts(manifestPath, tierNum, repoDir, cmd)
			if err != nil {
				return err
			}

			out, _ := json.MarshalIndent(report, "", "  ")
			fmt.Fprintln(cmd.OutOrStdout(), string(out))

			if exitCode != 0 {
				// Write BLOCKED message to stderr
				fmt.Fprintf(cmd.ErrOrStderr(), "check-program-conflicts: BLOCKED — %d conflict(s) detected in tier %d\n", len(report.Conflicts), tierNum)
				fmt.Fprintf(cmd.ErrOrStderr(), "Conflicting IMPLs:\n")
				for _, c := range report.Conflicts {
					fmt.Fprintf(cmd.ErrOrStderr(), "  %s: %v\n", c.File, c.Impls)
				}
				fmt.Fprintf(cmd.ErrOrStderr(), "Resolve by moving conflicting IMPLs to different tiers.\n")
				os.Exit(1)
			}
			return nil
		},
	}

	cmd.Flags().IntVar(&tierNum, "tier", 0, "Tier number to check (required)")
	_ = cmd.MarkFlagRequired("tier")

	return cmd
}

// runCheckProgramConflicts is the testable core logic for check-program-conflicts.
// It returns the conflict report, an exit code (0 = ok, 1 = conflicts found), and any error.
// Callers that cannot tolerate os.Exit (tests) call this function directly.
func runCheckProgramConflicts(manifestPath string, tierNum int, repoDirPath string, cmd *cobra.Command) (*protocol.ConflictReport, int, error) {
	manifest, err := protocol.ParseProgramManifest(manifestPath)
	if err != nil {
		return nil, 0, fmt.Errorf("check-program-conflicts: %w", err)
	}

	// Find the specified tier
	var tier *protocol.ProgramTier
	for i := range manifest.Tiers {
		if manifest.Tiers[i].Number == tierNum {
			tier = &manifest.Tiers[i]
			break
		}
	}
	if tier == nil {
		return nil, 0, fmt.Errorf("check-program-conflicts: tier %d not found in manifest", tierNum)
	}

	// Extract slugs from tier
	slugs := tier.Impls

	// Run conflict check
	report, err := protocol.CheckIMPLConflicts(slugs, repoDirPath)
	if err != nil {
		return nil, 0, fmt.Errorf("check-program-conflicts: %w", err)
	}

	exitCode := 0
	if len(report.Conflicts) > 0 {
		exitCode = 1
	}

	return report, exitCode, nil
}
