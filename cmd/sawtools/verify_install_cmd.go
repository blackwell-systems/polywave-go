package main

import (
	"encoding/json"
	"fmt"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/engine"

	"github.com/spf13/cobra"
)

// newVerifyInstallCmd returns a cobra.Command for "sawtools verify-install".
// Checks:
//  1. sawtools binary is on PATH and executable
//  2. Git version >= 2.20 (worktree support)
//  3. ~/.claude/skills/saw/ directory exists with expected symlinks
//  4. saw.config.json exists and has valid repos entries
//  5. All configured repo paths exist on disk
//
// Output: JSON with per-check pass/fail + overall verdict.
func newVerifyInstallCmd() *cobra.Command {
	var humanFlag bool

	cmd := &cobra.Command{
		Use:   "verify-install",
		Short: "Check that all SAW prerequisites are met",
		RunE: func(cmd *cobra.Command, args []string) error {
			result := engine.RunVerifyInstall(engine.VerifyInstallOpts{})

			if humanFlag {
				printHumanOutput(cmd, result)
				return nil
			}

			out, err := json.MarshalIndent(result, "", "  ")
			if err != nil {
				return fmt.Errorf("verify-install: marshal: %w", err)
			}
			fmt.Println(string(out))
			return nil
		},
	}

	cmd.Flags().BoolVar(&humanFlag, "human", false, "Print human-readable output instead of JSON")

	return cmd
}

// printHumanOutput renders the install result in a human-readable format.
func printHumanOutput(cmd *cobra.Command, result engine.InstallResult) {
	for _, c := range result.Checks {
		var icon string
		switch c.Status {
		case "pass":
			icon = "[OK]"
		case "fail":
			icon = "[FAIL]"
		case "warn":
			icon = "[WARN]"
		case "skip":
			icon = "[SKIP]"
		}
		fmt.Fprintf(cmd.OutOrStdout(), "  %s %s: %s\n", icon, c.Name, c.Detail)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "\nVerdict: %s\n", result.Verdict)
	fmt.Fprintf(cmd.OutOrStdout(), "%s\n", result.Summary)
}
