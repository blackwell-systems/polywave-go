package main

import (
	"encoding/json"
	"fmt"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/protocol"
	"github.com/spf13/cobra"
)

func newExtractContextCmd() *cobra.Command {
	var agentID string

	cmd := &cobra.Command{
		Use:   "extract-context <manifest-path>",
		Short: "Extract per-agent context payload from a YAML IMPL manifest (E23)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			manifestPath := args[0]

			m, err := protocol.Load(manifestPath)
			if err != nil {
				return fmt.Errorf("extract-context: %w", err)
			}

			payload, err := protocol.ExtractAgentContextFromManifest(m, agentID)
			if err != nil {
				return fmt.Errorf("extract-context: %w", err)
			}

			// Set the impl_doc_path from the manifest path provided on the command line.
			payload.IMPLDocPath = manifestPath

			out, err := json.MarshalIndent(payload, "", "  ")
			if err != nil {
				return fmt.Errorf("extract-context: marshal JSON: %w", err)
			}

			fmt.Println(string(out))
			return nil
		},
	}

	cmd.Flags().StringVar(&agentID, "agent", "", "Agent ID to extract context for (required)")
	_ = cmd.MarkFlagRequired("agent")

	return cmd
}
