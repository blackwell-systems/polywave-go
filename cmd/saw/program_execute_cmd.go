package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/engine"
	"github.com/blackwell-systems/scout-and-wave-go/pkg/protocol"
	"github.com/spf13/cobra"
)

func newProgramExecuteCmd() *cobra.Command {
	var (
		autoMode bool
		model    string
	)

	cmd := &cobra.Command{
		Use:   "program-execute <program-manifest>",
		Short: "Execute a PROGRAM manifest through the tier loop",
		Long: `Execute a PROGRAM manifest through the tier loop (E28-E34).

Reads the PROGRAM manifest, partitions IMPLs by status, launches parallel
Scouts for pending IMPLs, executes waves, runs tier gates, freezes contracts,
and advances through tiers.

Events are streamed to stdout as JSON lines for observability.

Examples:
  sawtools program-execute docs/PROGRAM.yaml
  sawtools program-execute docs/PROGRAM.yaml --auto
  sawtools program-execute docs/PROGRAM.yaml --auto --model claude-opus-4-6

Exit codes:
  0 - Program complete or paused awaiting review
  1 - Execution failure
  2 - Parse error (invalid manifest)`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			manifestPath := args[0]

			// Validate manifest is parseable (exit 2 on parse error)
			_, err := protocol.ParseProgramManifest(manifestPath)
			if err != nil {
				fmt.Fprintf(os.Stderr, "program-execute: parse error: %v\n", err)
				os.Exit(2)
			}

			// Set up context with signal handling
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			sigCh := make(chan os.Signal, 1)
			signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
			go func() {
				<-sigCh
				cancel()
			}()

			// Build event handler that streams JSON lines to stdout
			// and calls UpdateProgramIMPLStatus on impl_complete events
			onEvent := func(event engine.TierLoopEvent) {
				line, _ := json.Marshal(event)
				fmt.Fprintln(cmd.OutOrStdout(), string(line))

				// Wire UpdateProgramIMPLStatus: when an IMPL completes,
				// update the PROGRAM manifest status
				if event.Type == "impl_complete" {
					slug := extractSlugFromDetail(event.Detail)
					if slug != "" {
						_ = engine.UpdateProgramIMPLStatus(manifestPath, slug, "complete")
					}
				}
			}

			opts := engine.TierLoopOpts{
				ManifestPath: manifestPath,
				RepoPath:     repoDir,
				AutoMode:     autoMode,
				Model:        model,
				OnEvent:      onEvent,
			}

			result, err := engine.RunTierLoop(ctx, opts)
			if err != nil {
				fmt.Fprintf(os.Stderr, "program-execute: %v\n", err)
				os.Exit(1)
			}

			// Output final result as JSON
			out, _ := json.MarshalIndent(result, "", "  ")
			fmt.Fprintln(cmd.OutOrStdout(), string(out))

			if !result.ProgramComplete && len(result.Errors) > 0 {
				os.Exit(1)
			}

			return nil
		},
	}

	cmd.Flags().BoolVar(&autoMode, "auto", false, "Enable auto-advancement through tiers")
	cmd.Flags().StringVar(&model, "model", "", "Model override for Scout/Planner agents")

	return cmd
}

// extractSlugFromDetail extracts an IMPL slug from an impl_complete event detail string.
// Expected format: "IMPL <slug> wave execution finished"
func extractSlugFromDetail(detail string) string {
	const prefix = "IMPL "
	if !strings.HasPrefix(detail, prefix) {
		return ""
	}
	rest := detail[len(prefix):]
	idx := strings.Index(rest, " ")
	if idx == -1 {
		return rest
	}
	return rest[:idx]
}
