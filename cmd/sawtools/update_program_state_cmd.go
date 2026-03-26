package main

import (
	"encoding/json"
	"fmt"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/protocol"
	"github.com/spf13/cobra"
)

// UpdateProgramStateResult is the JSON output of the update-program-state command.
type UpdateProgramStateResult struct {
	Updated       bool   `json:"updated"`
	ManifestPath  string `json:"manifest_path"`
	PreviousState string `json:"previous_state"`
	NewState      string `json:"new_state"`
}

func newUpdateProgramStateCmd() *cobra.Command {
	var state string

	cmd := &cobra.Command{
		Use:   "update-program-state <program-manifest>",
		Short: "Update the state field of a PROGRAM manifest",
		Long: `Update the state field of a PROGRAM manifest YAML file.

Examples:
  sawtools update-program-state docs/PROGRAM/PROGRAM.yaml --state REVIEWED
  sawtools update-program-state docs/PROGRAM/PROGRAM.yaml --state TIER_EXECUTING

Exit codes:
  0 - Success
  1 - Update or write error
  2 - Parse error`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			manifestPath := args[0]

			// Parse the PROGRAM manifest
			manifest, err := protocol.ParseProgramManifest(manifestPath)
			if err != nil {
				return fmt.Errorf("update-program-state: parse error: %v", err)
			}

			// Record previous state
			previousState := string(manifest.State)

			// Update state
			manifest.State = protocol.ProgramState(state)

			// Write manifest back to disk
			if err := protocol.SaveYAML(manifestPath, manifest); err != nil {
				return fmt.Errorf("update-program-state: %w", err)
			}

			result := UpdateProgramStateResult{
				Updated:       true,
				ManifestPath:  manifestPath,
				PreviousState: previousState,
				NewState:      state,
			}

			out, _ := json.MarshalIndent(result, "", "  ")
			fmt.Fprintln(cmd.OutOrStdout(), string(out))
			return nil
		},
	}

	cmd.Flags().StringVar(&state, "state", "", "New state value (e.g. REVIEWED, TIER_EXECUTING) (required)")
	_ = cmd.MarkFlagRequired("state")

	return cmd
}
