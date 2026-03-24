package main

import (
	"encoding/json"
	"fmt"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/protocol"
	"github.com/spf13/cobra"
)

// newListProgramsCmd returns a cobra.Command that scans a directory for PROGRAM manifest
// files and prints JSON summaries (path, slug, state, title).
// Always exits 0 (empty list is valid).
func newListProgramsCmd() *cobra.Command {
	var dir string

	cmd := &cobra.Command{
		Use:   "list-programs",
		Short: "List PROGRAM manifests in a directory",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			result, err := protocol.ListPrograms(dir)
			if err != nil {
				return fmt.Errorf("list-programs: %w", err)
			}

			out, _ := json.MarshalIndent(result, "", "  ")
			fmt.Fprintln(cmd.OutOrStdout(), string(out))
			return nil
		},
	}

	cmd.Flags().StringVar(&dir, "dir", "docs/", "Base directory to scan for PROGRAM manifests (scans dir/PROGRAM/)")

	return cmd
}
