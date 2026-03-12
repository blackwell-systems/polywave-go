package main

import (
	"encoding/json"
	"fmt"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/protocol"
	"github.com/blackwell-systems/scout-and-wave-go/pkg/scaffold"
	"github.com/spf13/cobra"
)

func newDetectScaffoldsCmd() *cobra.Command {
	var stage string

	cmd := &cobra.Command{
		Use:   "detect-scaffolds <impl-doc-path>",
		Short: "Detect shared types that should be extracted to scaffold files",
		Long: `Analyze IMPL document to detect types that should be scaffolds.

Pre-agent mode analyzes interface contracts to find types referenced by ≥2 agents.
Post-agent mode analyzes agent task fields to detect duplicate type definitions.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			implDocPath := args[0]

			// Validate stage flag
			if stage != "pre-agent" && stage != "post-agent" {
				return fmt.Errorf("invalid --stage value: must be 'pre-agent' or 'post-agent'")
			}

			// Load the manifest
			manifest, err := protocol.Load(implDocPath)
			if err != nil {
				return fmt.Errorf("failed to load IMPL doc: %w", err)
			}

			// Dispatch based on stage
			if stage == "pre-agent" {
				return runPreAgentDetection(manifest)
			} else {
				return runPostAgentDetection(manifest)
			}
		},
	}

	cmd.Flags().StringVar(&stage, "stage", "", "Detection stage: pre-agent or post-agent (required)")
	cmd.MarkFlagRequired("stage")

	return cmd
}

func runPreAgentDetection(manifest *protocol.IMPLManifest) error {
	result, err := scaffold.DetectScaffoldsPreAgent(manifest.InterfaceContracts)
	if err != nil {
		return fmt.Errorf("pre-agent detection failed: %w", err)
	}

	// Marshal to JSON and output
	output, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal result: %w", err)
	}

	fmt.Println(string(output))
	return nil
}

func runPostAgentDetection(manifest *protocol.IMPLManifest) error {
	// Post-agent detection is implemented by Agent B
	// For now, return a not-implemented error to make it clear this is a stub
	return fmt.Errorf("post-agent detection not yet implemented (Agent B's responsibility)")
}
