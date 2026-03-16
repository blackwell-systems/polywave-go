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
	var includeComplete bool

	cmd := &cobra.Command{
		Use:   "list-impls",
		Short: "List IMPL manifests in a directory (excludes completed by default)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			result, err := protocol.ListIMPLs(dir, protocol.ListIMPLsOpts{
				IncludeComplete: includeComplete,
			})
			if err != nil {
				return fmt.Errorf("list-impls: %w", err)
			}

			out, _ := json.MarshalIndent(result, "", "  ")
			fmt.Println(string(out))
			return nil
		},
	}

	cmd.Flags().StringVar(&dir, "dir", "docs/IMPL", "Directory to scan for IMPL manifests")
	cmd.Flags().BoolVar(&includeComplete, "include-complete", false, "Include completed/archived IMPL docs")

	return cmd
}
