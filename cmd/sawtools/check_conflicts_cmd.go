package main

import (
	"encoding/json"
	"fmt"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/protocol"
	"github.com/spf13/cobra"
)

func newCheckConflictsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "check-conflicts <manifest-path>",
		Short: "Detect file ownership conflicts in completion reports",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			manifestPath := args[0]

			m, err := protocol.Load(manifestPath)
			if err != nil {
				return fmt.Errorf("check-conflicts: %w", err)
			}

			conflicts := protocol.DetectOwnershipConflicts(m, m.CompletionReports)

			result := struct {
				ConflictCount int                         `json:"conflict_count"`
				Conflicts     []protocol.OwnershipConflict `json:"conflicts"`
			}{
				ConflictCount: len(conflicts),
				Conflicts:     conflicts,
			}

			out, _ := json.MarshalIndent(result, "", "  ")
			fmt.Println(string(out))

			if len(conflicts) > 0 {
				return fmt.Errorf("check-conflicts: %d conflict(s) detected", len(conflicts))
			}
			return nil
		},
	}
}
