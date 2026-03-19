package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/protocol"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

// UpdateProgramImplResult is the JSON output of the update-program-impl command.
type UpdateProgramImplResult struct {
	Updated        bool   `json:"updated"`
	ManifestPath   string `json:"manifest_path"`
	ImplSlug       string `json:"impl_slug"`
	PreviousStatus string `json:"previous_status"`
	NewStatus      string `json:"new_status"`
}

func newUpdateProgramImplCmd() *cobra.Command {
	var implSlug string
	var status string

	cmd := &cobra.Command{
		Use:   "update-program-impl <program-manifest>",
		Short: "Update the status of a specific IMPL entry in a PROGRAM manifest",
		Long: `Update the status of a specific IMPL entry in a PROGRAM manifest YAML file.

The impl is identified by its slug field.

Examples:
  sawtools update-program-impl docs/PROGRAM.yaml --impl my-feature --status complete
  sawtools update-program-impl docs/PROGRAM.yaml --impl my-feature --status in_progress

Exit codes:
  0 - Success
  1 - Impl not found or write error
  2 - Parse error`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			manifestPath := args[0]

			// Parse the PROGRAM manifest
			manifest, err := protocol.ParseProgramManifest(manifestPath)
			if err != nil {
				fmt.Fprintf(os.Stderr, "update-program-impl: parse error: %v\n", err)
				os.Exit(2)
			}

			// Find the impl entry by slug
			implIdx := -1
			for i, impl := range manifest.Impls {
				if impl.Slug == implSlug {
					implIdx = i
					break
				}
			}

			if implIdx == -1 {
				fmt.Fprintf(os.Stderr, "update-program-impl: impl %s not found in manifest\n", implSlug)
				os.Exit(1)
			}

			// Record previous status
			previousStatus := manifest.Impls[implIdx].Status

			// Update the impl status
			manifest.Impls[implIdx].Status = status

			// Marshal manifest back to YAML
			data, err := yaml.Marshal(manifest)
			if err != nil {
				fmt.Fprintf(os.Stderr, "update-program-impl: failed to marshal manifest: %v\n", err)
				os.Exit(1)
			}

			// Write manifest back to disk
			if err := os.WriteFile(manifestPath, data, 0644); err != nil {
				fmt.Fprintf(os.Stderr, "update-program-impl: failed to write manifest: %v\n", err)
				os.Exit(1)
			}

			result := UpdateProgramImplResult{
				Updated:        true,
				ManifestPath:   manifestPath,
				ImplSlug:       implSlug,
				PreviousStatus: previousStatus,
				NewStatus:      status,
			}

			out, _ := json.MarshalIndent(result, "", "  ")
			fmt.Fprintln(cmd.OutOrStdout(), string(out))
			return nil
		},
	}

	cmd.Flags().StringVar(&implSlug, "impl", "", "IMPL slug to update (required)")
	cmd.Flags().StringVar(&status, "status", "", "New status value (e.g. complete, in_progress, blocked) (required)")
	_ = cmd.MarkFlagRequired("impl")
	_ = cmd.MarkFlagRequired("status")

	return cmd
}
