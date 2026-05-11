package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/blackwell-systems/polywave-go/pkg/engine"
	"github.com/blackwell-systems/polywave-go/pkg/protocol"
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
  sawtools program-status docs/PROGRAM/PROGRAM.yaml

Exit codes:
  0 - Always (status is informational)
  2 - Parse error`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			manifestPath := args[0]

			// Parse the PROGRAM manifest
			manifest, err := protocol.ParseProgramManifest(manifestPath)
			if err != nil {
				return fmt.Errorf("program-status: parse error: %w", err)
			}

			// Get current working directory as repo path
			repoPath, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("program-status: failed to get current directory: %w", err)
			}

			// Sync status from disk before displaying (non-fatal on error)
			if syncRes := engine.SyncProgramStatusFromDisk(manifestPath, repoPath); syncRes.IsFatal() {
				syncErrMsg := "unknown error"
				if len(syncRes.Errors) > 0 {
					syncErrMsg = syncRes.Errors[0].Message
				}
				fmt.Fprintf(os.Stderr, "program-status: warning: sync from disk failed: %s\n", syncErrMsg)
			}

			// Get program status
			res := protocol.GetProgramStatus(manifest, repoPath)
			if res.IsFatal() {
				return fmt.Errorf("program-status: %s", res.Errors[0].Message)
			}
			data := res.GetData()

			// Output JSON result
			out, _ := json.MarshalIndent(data, "", "  ")
			fmt.Fprintln(cmd.OutOrStdout(), string(out))

			return nil
		},
	}

	return cmd
}
