package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/journal"
	"github.com/spf13/cobra"
)

func newJournalInitCmd() *cobra.Command {
	var waveNum int
	var agentID string

	cmd := &cobra.Command{
		Use:   "journal-init <manifest-path>",
		Short: "Initialize journal directory structure for a wave agent",
		Long: `Creates the journal directory structure (.saw-state/journals/wave<N>/agent-<ID>/)
and initializes the cursor file for tracking Claude Code session log position.

This command is called by the LLM orchestrator before launching wave agents
to prepare journal infrastructure. The Go orchestrator (web app) handles this
automatically via pkg/engine integration.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if waveNum == 0 {
				return fmt.Errorf("--wave is required")
			}
			if agentID == "" {
				return fmt.Errorf("--agent is required")
			}

			// Determine project root from manifest path
			manifestPath := args[0]
			projectRoot := filepath.Dir(filepath.Dir(filepath.Dir(manifestPath))) // docs/IMPL/IMPL-*.yaml -> project root

			// Use repoDir as project root if provided
			if repoDir != "" {
				projectRoot = repoDir
			}

			// Create full agent ID with wave prefix
			fullAgentID := fmt.Sprintf("wave%d-agent-%s", waveNum, agentID)

			// Create journal observer
			observer, err := journal.NewObserver(projectRoot, fullAgentID)
			if err != nil {
				return fmt.Errorf("failed to create journal observer: %w", err)
			}

			// Check if already initialized
			if _, err := os.Stat(observer.CursorPath); err == nil {
				// Already initialized
				result := map[string]interface{}{
					"status":      "already_initialized",
					"journal_dir": observer.JournalDir,
					"cursor_path": observer.CursorPath,
				}
				out, _ := json.MarshalIndent(result, "", "  ")
				fmt.Println(string(out))
				return nil
			}

			// Cursor doesn't exist - initialize it
			// Create all parent directories
			if err := os.MkdirAll(filepath.Dir(observer.CursorPath), 0755); err != nil {
				return fmt.Errorf("failed to create journal directories: %w", err)
			}
			if err := os.MkdirAll(observer.ResultsDir, 0755); err != nil {
				return fmt.Errorf("failed to create tool-results directory: %w", err)
			}

			// Initialize cursor to empty state
			emptyCursor := journal.SessionCursor{
				SessionFile: "",
				Offset:      0,
			}
			cursorData, _ := json.MarshalIndent(emptyCursor, "", "  ")
			if err := os.WriteFile(observer.CursorPath, cursorData, 0644); err != nil {
				return fmt.Errorf("failed to write cursor file: %w", err)
			}

			result := map[string]interface{}{
				"status":      "initialized",
				"journal_dir": observer.JournalDir,
				"cursor_path": observer.CursorPath,
				"index_path":  observer.IndexPath,
				"results_dir": observer.ResultsDir,
			}
			out, _ := json.MarshalIndent(result, "", "  ")
			fmt.Println(string(out))
			return nil
		},
	}

	cmd.Flags().IntVar(&waveNum, "wave", 0, "Wave number (required)")
	cmd.Flags().StringVar(&agentID, "agent", "", "Agent ID (required)")
	_ = cmd.MarkFlagRequired("wave")
	_ = cmd.MarkFlagRequired("agent")

	return cmd
}
