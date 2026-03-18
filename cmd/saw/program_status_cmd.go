package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/protocol"
	"github.com/spf13/cobra"
)

func newProgramStatusCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "program-status <program-manifest>",
		Short: "Display full program status report",
		Long: `Display comprehensive status report for a PROGRAM manifest.

Reports include:
- Current tier
- Per-tier IMPL statuses
- Contract freeze states
- Completion tracking

Examples:
  sawtools program-status docs/PROGRAM.yaml

Exit codes:
  0 - Always (status is informational)
  2 - Parse error`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			manifestPath := args[0]

			// Parse the PROGRAM manifest
			manifest, err := protocol.ParseProgramManifest(manifestPath)
			if err != nil {
				fmt.Fprintf(os.Stderr, "program-status: parse error: %v\n", err)
				os.Exit(2)
			}

			// Get current working directory as repo path
			repoPath, err := os.Getwd()
			if err != nil {
				fmt.Fprintf(os.Stderr, "program-status: failed to get current directory: %v\n", err)
				os.Exit(2)
			}

			// Get program status
			result, err := protocol.GetProgramStatus(manifest, repoPath)
			if err != nil {
				fmt.Fprintf(os.Stderr, "program-status: %v\n", err)
				os.Exit(2)
			}

			// Output JSON result
			out, _ := json.MarshalIndent(result, "", "  ")
			fmt.Fprintln(cmd.OutOrStdout(), string(out))

			return nil
		},
	}

	return cmd
}
