package main

import (
	"encoding/json"
	"fmt"

	"github.com/blackwell-systems/polywave-go/pkg/engine"
	"github.com/blackwell-systems/polywave-go/pkg/journal"
	"github.com/spf13/cobra"
)

// DebugOpts holds flag values for the debug-journal command.
type DebugOpts struct {
	Summary      bool
	FailuresOnly bool
	Last         int
	Export       string
	Force        bool
}

// newDebugJournalCmd returns a cobra.Command that inspects journal contents.
func newDebugJournalCmd() *cobra.Command {
	opts := DebugOpts{}

	cmd := &cobra.Command{
		Use:   "debug-journal <agent-path>",
		Short: "Inspect journal contents for debugging failed agents",
		Long: `Inspect tool execution journal for a specific agent.

Agent path format: wave1/agent-A or wave2-agent-B

Examples:
  polywave-tools debug-journal wave1/agent-A                    # dump full journal (JSONL)
  polywave-tools debug-journal wave1/agent-A --summary          # human-readable summary
  polywave-tools debug-journal wave1/agent-A --failures-only    # show only failed tool calls
  polywave-tools debug-journal wave1/agent-A --last 20          # show last N entries
  polywave-tools debug-journal wave1/agent-A --export timeline.html  # export HTML timeline
`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return debugJournalCommand(repoDir, args[0], opts)
		},
	}

	cmd.Flags().BoolVar(&opts.Summary, "summary", false, "Show human-readable summary")
	cmd.Flags().BoolVar(&opts.FailuresOnly, "failures-only", false, "Show only failed tool calls")
	cmd.Flags().IntVar(&opts.Last, "last", 0, "Show last N entries only")
	cmd.Flags().StringVar(&opts.Export, "export", "", "Export HTML timeline to file")
	cmd.Flags().BoolVar(&opts.Force, "force", false, "Overwrite export file if it exists")

	return cmd
}

// debugJournalCommand delegates to engine.DebugJournal and handles all output.
// Kept as a package-level function so tests and other cmd files can call it directly.
func debugJournalCommand(repoRoot string, agentPath string, opts DebugOpts) error {
	res := engine.DebugJournal(engine.DebugJournalOpts{
		RepoPath:     repoRoot,
		AgentPath:    agentPath,
		Summary:      opts.Summary,
		FailuresOnly: opts.FailuresOnly,
		Last:         opts.Last,
		Export:       opts.Export,
		Force:        opts.Force,
	})
	if res.IsFatal() {
		if len(res.Errors) > 0 {
			return fmt.Errorf("%s: %s", res.Errors[0].Code, res.Errors[0].Message)
		}
		return fmt.Errorf("debug-journal failed")
	}
	result := res.GetData()

	if result.TotalCount == 0 {
		fmt.Printf("Journal for %s exists but contains no entries yet.\n", agentPath)
		return nil
	}

	// Export: print path confirmation.
	if opts.Export != "" {
		fmt.Printf("Timeline exported to: %s\n", result.ExportPath)
		return nil
	}

	// Summary: print structured fields.
	if opts.Summary {
		s := result.Summary
		if s == nil {
			return nil
		}
		fmt.Printf("Journal: %s\n", agentPath)
		fmt.Printf("Duration: %s\n", s.Duration)
		fmt.Printf("Total tool calls: %d\n\n", result.TotalCount)

		if len(s.FilesModified) > 0 {
			fmt.Printf("Files modified: %d\n", len(s.FilesModified))
			for _, fm := range s.FilesModified {
				fmt.Printf("  %-40s (%s)\n", fm.Path, formatFileChangeSummary(fm))
			}
			fmt.Println()
		}

		if len(s.CommandsRun) > 0 {
			fmt.Printf("Commands run: %d\n", len(s.CommandsRun))
			for _, c := range s.CommandsRun {
				fmt.Printf("  %-40s (%s)\n", c.Command, c.Status)
			}
			fmt.Println()
		}

		if len(s.GitCommits) > 0 {
			fmt.Printf("Commits: %d\n", len(s.GitCommits))
			for _, gc := range s.GitCommits {
				sha := gc.SHA
				if len(sha) > 7 {
					sha = sha[:7]
				}
				fmt.Printf("  %s \"%s\" (%s)\n", sha, gc.Message, gc.Branch)
			}
			fmt.Println()
		}

		fmt.Println("Completion report:")
		if s.ReportWritten {
			fmt.Println("  Written")
		} else {
			fmt.Println("  Not yet written")
		}
		return nil
	}

	// Default: dump filtered entries as raw JSONL.
	for _, entry := range result.Entries {
		data, err := json.Marshal(entry)
		if err != nil {
			continue
		}
		fmt.Println(string(data))
	}

	return nil
}

// loadJournalEntries delegates to engine.LoadJournalEntries so that
// journal_context.go (which calls this package-level function) continues to compile.
func loadJournalEntries(journalPath string) ([]journal.ToolEntry, error) {
	res := engine.LoadJournalEntries(journalPath)
	if res.IsFatal() {
		if len(res.Errors) > 0 {
			return nil, fmt.Errorf("%s: %s", res.Errors[0].Code, res.Errors[0].Message)
		}
		return nil, fmt.Errorf("load journal entries failed")
	}
	return res.GetData(), nil
}

// formatFileChangeSummary formats a FileModification for human display.
func formatFileChangeSummary(fm engine.FileModification) string {
	if fm.Operation == "added" {
		return fmt.Sprintf("added, %d lines", fm.LinesAdded)
	}
	return fmt.Sprintf("%d lines added", fm.LinesAdded)
}
