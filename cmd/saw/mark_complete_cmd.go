package main

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/protocol"
	"github.com/spf13/cobra"
)

func newMarkCompleteCmd() *cobra.Command {
	var date string

	cmd := &cobra.Command{
		Use:   "mark-complete <manifest-path>",
		Short: "Write completion marker to IMPL manifest and archive to complete/ subdirectory",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			manifestPath := args[0]

			// Default date to today if not provided
			if date == "" {
				date = time.Now().Format("2006-01-02")
			}

			if err := protocol.WriteCompletionMarker(manifestPath, date); err != nil {
				return fmt.Errorf("mark-complete: %w", err)
			}

			// Always archive to docs/IMPL/complete/
			archivedPath, err := protocol.ArchiveIMPL(manifestPath)
			if err != nil {
				return fmt.Errorf("mark-complete: archive failed: %w", err)
			}

			out, _ := json.Marshal(map[string]interface{}{
				"marked":   true,
				"date":     date,
				"path":     archivedPath,
				"archived": true,
			})
			fmt.Println(string(out))
			return nil
		},
	}

	cmd.Flags().StringVar(&date, "date", "", "Completion date (YYYY-MM-DD, defaults to today)")

	return cmd
}
