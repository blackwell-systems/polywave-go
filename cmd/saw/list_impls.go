package main

import (
	"encoding/json"
	"fmt"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/protocol"
	"github.com/spf13/cobra"
)

// newListIMPLsCmd returns a cobra.Command that scans a directory for IMPL manifest
// files and prints JSON summaries (path, feature_slug, verdict, current_wave, total_waves).
// Always exits 0 (empty list is valid).
func newListIMPLsCmd() *cobra.Command {
	var dir string

	cmd := &cobra.Command{
		Use:   "list-impls",
		Short: "List all IMPL manifests in a directory",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			result, err := protocol.ListIMPLs(dir)
			if err != nil {
				return fmt.Errorf("list-impls: %w", err)
			}

			out, _ := json.MarshalIndent(result, "", "  ")
			fmt.Println(string(out))
			return nil
		},
	}

	cmd.Flags().StringVar(&dir, "dir", "docs/IMPL", "Directory to scan for IMPL manifests")

	return cmd
}
