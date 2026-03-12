package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/journal"
	"github.com/blackwell-systems/scout-and-wave-go/pkg/protocol"
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
1. Extracting the agent's brief from the IMPL doc to .saw-agent-brief.md
2. Initializing the journal observer (if not disabled)

For worktree-based agents, writes brief to worktree root.
For solo agents (--no-worktree), writes to .saw-state/wave{N}/agent-{ID}/brief.md.

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

			// Parse IMPL doc
			doc, err := protocol.Load(manifestPath)
			if err != nil {
				return fmt.Errorf("failed to parse IMPL doc: %w", err)
			}

			// Find the agent's wave and task
			var agentTask string
			var agentFiles []string
			for _, wave := range doc.Waves {
				if wave.Number != waveNum {
					continue
				}
				for _, agent := range wave.Agents {
					if agent.ID == agentID {
						agentTask = agent.Task
						agentFiles = agent.Files
						break
					}
				}
			}

			if agentTask == "" {
				return fmt.Errorf("agent %s not found in wave %d", agentID, waveNum)
			}

			// Extract interface contracts
			contractsSection := ""
			if len(doc.InterfaceContracts) > 0 {
				contractsSection = "\n\n## Interface Contracts\n\n"
				for _, contract := range doc.InterfaceContracts {
					contractsSection += fmt.Sprintf("### %s\n\n%s\n\n```\n%s\n```\n\n",
						contract.Name, contract.Description, contract.Definition)
				}
			}

			// Extract quality gates
			gatesSection := ""
			if doc.QualityGates.Level != "" {
				gatesSection = "\n\n## Quality Gates\n\n"
				gatesSection += fmt.Sprintf("Level: %s\n\n", doc.QualityGates.Level)
				for _, gate := range doc.QualityGates.Gates {
					gatesSection += fmt.Sprintf("- **%s**: `%s` (required: %t)\n",
						gate.Type, gate.Command, gate.Required)
					if gate.Description != "" {
						gatesSection += fmt.Sprintf("  %s\n", gate.Description)
					}
				}
			}

			// Build the agent brief
			brief := fmt.Sprintf(`# Agent %s Brief - Wave %d

**IMPL Doc:** %s

## Files Owned

%s

## Task

%s
%s%s
`,
				agentID,
				waveNum,
				manifestPath,
				formatFileList(agentFiles),
				agentTask,
				contractsSection,
				gatesSection,
			)

			// Determine output path
			var briefPath string
			if noWorktree {
				// Solo agent - write to .saw-state
				stateDir := filepath.Join(projectRoot, ".saw-state", fmt.Sprintf("wave%d", waveNum), fmt.Sprintf("agent-%s", agentID))
				if err := os.MkdirAll(stateDir, 0755); err != nil {
					return fmt.Errorf("failed to create state dir: %w", err)
				}
				briefPath = filepath.Join(stateDir, "brief.md")
			} else {
				// Worktree agent - write to worktree root
				worktreePath := filepath.Join(projectRoot, ".claude", "worktrees", fmt.Sprintf("wave%d-agent-%s", waveNum, agentID))
				briefPath = filepath.Join(worktreePath, ".saw-agent-brief.md")
			}

			// Write brief
			if err := os.WriteFile(briefPath, []byte(brief), 0644); err != nil {
				return fmt.Errorf("failed to write brief: %w", err)
			}

			// Initialize journal observer
			fullAgentID := fmt.Sprintf("wave%d-agent-%s", waveNum, agentID)
			observer, err := journal.NewObserver(projectRoot, fullAgentID)
			if err != nil {
				return fmt.Errorf("failed to create journal observer: %w", err)
			}

			// Initialize cursor if it doesn't exist
			if _, err := os.Stat(observer.CursorPath); os.IsNotExist(err) {
				emptyCursor := journal.SessionCursor{
					SessionFile: "",
					Offset:      0,
				}
				cursorData, _ := json.MarshalIndent(emptyCursor, "", "  ")
				if err := os.WriteFile(observer.CursorPath, cursorData, 0644); err != nil {
					return fmt.Errorf("failed to write cursor file: %w", err)
				}
			}

			// Output result
			result := map[string]interface{}{
				"brief_path":   briefPath,
				"brief_length": len(brief),
				"journal_dir":  observer.JournalDir,
				"cursor_path":  observer.CursorPath,
				"index_path":   observer.IndexPath,
				"results_dir":  observer.ResultsDir,
				"agent_id":     agentID,
				"wave":         waveNum,
				"files_owned":  len(agentFiles),
			}

			out, _ := json.MarshalIndent(result, "", "  ")
			fmt.Println(string(out))

			return nil
		},
	}

	cmd.Flags().IntVar(&waveNum, "wave", 0, "Wave number (required)")
	cmd.Flags().StringVar(&agentID, "agent", "", "Agent ID (required)")
	cmd.Flags().BoolVar(&noWorktree, "no-worktree", false, "Solo agent mode (write brief to .saw-state instead of worktree)")
	_ = cmd.MarkFlagRequired("wave")
	_ = cmd.MarkFlagRequired("agent")

	return cmd
}

func formatFileList(files []string) string {
	if len(files) == 0 {
		return "(no files specified)"
	}
	result := ""
	for _, f := range files {
		result += fmt.Sprintf("- `%s`\n", f)
	}
	return result
}
