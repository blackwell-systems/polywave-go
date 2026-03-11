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
	var archive bool

	cmd := &cobra.Command{
		Use:   "mark-complete <manifest-path>",
		Short: "Write completion marker to IMPL manifest and optionally archive to complete/ subdirectory",
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

			finalPath := manifestPath
			if archive {
				archivedPath, err := protocol.ArchiveIMPL(manifestPath)
				if err != nil {
					return fmt.Errorf("mark-complete: archive failed: %w", err)
				}
				finalPath = archivedPath
			}

			out, _ := json.Marshal(map[string]interface{}{
				"marked":   true,
				"date":     date,
				"path":     finalPath,
				"archived": archive,
			})
			fmt.Println(string(out))
			return nil
		},
	}

	cmd.Flags().StringVar(&date, "date", "", "Completion date (YYYY-MM-DD, defaults to today)")
	cmd.Flags().BoolVar(&archive, "archive", false, "Move IMPL to docs/IMPL/complete/ after marking complete")

	return cmd
}
