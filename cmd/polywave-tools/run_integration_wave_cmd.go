package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/blackwell-systems/polywave-go/pkg/protocol"
	"github.com/spf13/cobra"
)

func newRunIntegrationWaveCmd() *cobra.Command {
	var waveNum int

	cmd := &cobra.Command{
		Use:   "run-integration-wave <manifest-path>",
		Short: "Execute planned integration wave (E27)",
		Long: `Execute a planned integration wave where Scout pre-planned the wiring work.

This command handles waves with type: integration, where Scout has already
identified the wiring work and assigned it to integration agents. Unlike
reactive E25/E26 gap detection, E27 planned integration waves are part of
the IMPL doc's execution plan from the start.

Flow:
1. Load manifest and find wave N
2. Verify wave.Type == "integration" (error if not)
3. For each agent in wave:
   a. Extract brief to .polywave-agent-brief.md
   b. Print agent metadata (files, task summary)
4. Return structured JSON result

The orchestrator still launches agents via the Agent tool; this command
prepares the metadata and briefs for orchestrator consumption.

Example:
  polywave-tools run-integration-wave docs/IMPL/IMPL-feature.yaml --wave 2`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			manifestPath := args[0]

			// Load manifest
			manifest, err := protocol.Load(context.TODO(), manifestPath)
			if err != nil {
				return fmt.Errorf("run-integration-wave: failed to load manifest: %w", err)
			}

			// Find target wave
			var targetWave *protocol.Wave
			for i := range manifest.Waves {
				if manifest.Waves[i].Number == waveNum {
					targetWave = &manifest.Waves[i]
					break
				}
			}
			if targetWave == nil {
				return fmt.Errorf("run-integration-wave: wave %d not found in manifest", waveNum)
			}

			// Verify type: integration
			if targetWave.Type != "integration" {
				waveType := targetWave.Type
				if waveType == "" {
					waveType = "standard"
				}
				return fmt.Errorf("run-integration-wave: wave %d is not type: integration (found: %s)", waveNum, waveType)
			}

			// Prepare agents
			agentStatuses := make([]map[string]interface{}, 0, len(targetWave.Agents))
			for _, agent := range targetWave.Agents {
				fmt.Fprintf(os.Stderr, "Preparing integration agent %s...\n", agent.ID)

				// Extract brief (no worktree needed for integration agents)
				// Integration agents work directly on main branch
				briefPath := filepath.Join(repoDir, ".polywave-agent-brief.md")
				if err := os.WriteFile(briefPath, []byte(agent.Task), 0644); err != nil {
					return fmt.Errorf("run-integration-wave: failed to write brief for %s: %w", agent.ID, err)
				}

				// Print agent metadata for orchestrator (to stderr to avoid interfering with JSON output)
				fmt.Fprintf(os.Stderr, "Agent %s ready. Orchestrator should launch with:\n", agent.ID)
				fmt.Fprintf(os.Stderr, "  subagent_type: integration-agent\n")
				fmt.Fprintf(os.Stderr, "  files: %v\n", agent.Files)
				fmt.Fprintf(os.Stderr, "  prompt: see %s\n", briefPath)
				fmt.Fprintf(os.Stderr, "\n")

				// Record agent status
				agentStatuses = append(agentStatuses, map[string]interface{}{
					"id":    agent.ID,
					"files": agent.Files,
					"brief": briefPath,
				})
			}

			// Output structured result
			result := map[string]interface{}{
				"success": true,
				"wave":    waveNum,
				"agents":  agentStatuses,
				"message": "Integration wave agents prepared. Launch via orchestrator.",
			}
			out, _ := json.MarshalIndent(result, "", "  ")
			fmt.Println(string(out))

			return nil
		},
	}

	cmd.Flags().IntVar(&waveNum, "wave", 0, "Wave number (required)")
	_ = cmd.MarkFlagRequired("wave")

	return cmd
}
