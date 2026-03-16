package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/journal"
	"github.com/spf13/cobra"
)

func newJournalContextCmd() *cobra.Command {
	var waveNum int
	var agentID string
	var outputPath string
	var maxEntries int

	cmd := &cobra.Command{
		Use:   "journal-context <manifest-path>",
		Short: "Generate context.md from journal entries for agent recovery",
		Long: `Syncs the journal from Claude Code session logs and generates a markdown summary
of the agent's execution history (files modified, tests run, git commits, etc.).

The generated context.md can be prepended to the agent's prompt after context
compaction to preserve working memory. Called by the LLM orchestrator before
launching or resuming agents.`,
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
			projectRoot := filepath.Dir(filepath.Dir(filepath.Dir(manifestPath)))

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

			// Check if journal is initialized
			if _, err := os.Stat(observer.CursorPath); os.IsNotExist(err) {
				return fmt.Errorf("journal not initialized (run journal-init first)")
			}

			// Sync journal from Claude Code session logs
			result, err := observer.Sync()
			if err != nil {
				return fmt.Errorf("journal sync failed: %w", err)
			}

			// Load entries from index.jsonl
			entries, err := loadJournalEntries(observer.IndexPath)
			if err != nil {
				return fmt.Errorf("failed to load journal entries: %w", err)
			}

			// Generate context markdown
			contextMD, err := journal.GenerateContext(entries, maxEntries)
			if err != nil {
				return fmt.Errorf("context generation failed: %w", err)
			}

			// Determine output path
			outPath := outputPath
			if outPath == "" {
				outPath = filepath.Join(observer.JournalDir, "context.md")
			}

			// Write context to file
			if err := os.WriteFile(outPath, []byte(contextMD), 0644); err != nil {
				return fmt.Errorf("failed to write context file: %w", err)
			}

			// Output JSON result
			response := map[string]interface{}{
				"journal_dir":       observer.JournalDir,
				"new_tool_uses":     result.NewToolUses,
				"new_tool_results":  result.NewToolResults,
				"new_bytes":         result.NewBytes,
				"context_file":      outPath,
				"context_length":    len(contextMD),
				"context_available": len(contextMD) > 0,
				"total_entries":     len(entries),
			}
			out, _ := json.MarshalIndent(response, "", "  ")
			fmt.Println(string(out))

			return nil
		},
	}

	cmd.Flags().IntVar(&waveNum, "wave", 0, "Wave number (required)")
	cmd.Flags().StringVar(&agentID, "agent", "", "Agent ID (required)")
	cmd.Flags().StringVar(&outputPath, "output", "", "Output path for context.md (default: <journal-dir>/context.md)")
	cmd.Flags().IntVar(&maxEntries, "max-entries", 0, "Maximum entries to include (0 = all)")
	_ = cmd.MarkFlagRequired("wave")
	_ = cmd.MarkFlagRequired("agent")

	return cmd
}

