package main

import (
	"encoding/json"
	"fmt"
	"path/filepath"

	"github.com/blackwell-systems/polywave-go/pkg/engine"
	"github.com/spf13/cobra"
)

func newPrepareAgentCmd() *cobra.Command {
	var waveNum int
	var agentID string
	var noWorktree bool

	cmd := &cobra.Command{
		Use:   "prepare-agent <manifest-path>",
		Short: "Prepare agent environment before launch (extract brief, init journal)",
		Long: `Prepares an agent's execution environment by:
1. Extracting the agent's brief from the IMPL doc to .polywave-agent-brief.md
2. Initializing the journal observer (if not disabled)

For worktree-based agents, writes brief to worktree root.
For solo agents (--no-worktree), writes to .polywave-state/wave{N}/agent-{ID}/brief.md.

This eliminates the ~10s latency of agents calling extract-context at startup.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if waveNum == 0 {
				return fmt.Errorf("--wave is required")
			}
			if agentID == "" {
				return fmt.Errorf("--agent is required")
			}

			manifestPath := args[0]

			// Determine project root from manifest path or --repo-dir flag
			projectRoot := filepath.Dir(filepath.Dir(filepath.Dir(manifestPath)))
			if repoDir != "" {
				projectRoot = repoDir
			}

			res := engine.PrepareAgent(cmd.Context(), engine.PrepareAgentOpts{
				ManifestPath: manifestPath,
				ProjectRoot:  projectRoot,
				WaveNum:      waveNum,
				AgentID:      agentID,
				NoWorktree:   noWorktree,
			})
			if res.IsFatal() {
				if len(res.Errors) > 0 {
					return fmt.Errorf("%s", res.Errors[0].Message)
				}
				return fmt.Errorf("prepare-agent failed")
			}

			out, _ := json.MarshalIndent(res.GetData(), "", "  ")
			fmt.Println(string(out))

			return nil
		},
	}

	cmd.Flags().IntVar(&waveNum, "wave", 0, "Wave number (required)")
	cmd.Flags().StringVar(&agentID, "agent", "", "Agent ID (required)")
	cmd.Flags().BoolVar(&noWorktree, "no-worktree", false, "Solo agent mode (write brief to .polywave-state instead of worktree)")
	_ = cmd.MarkFlagRequired("wave")
	_ = cmd.MarkFlagRequired("agent")

	return cmd
}
