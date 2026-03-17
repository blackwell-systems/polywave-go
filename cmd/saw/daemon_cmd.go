package main

import (
	"encoding/json"
	"fmt"
	"os/signal"
	"syscall"
	"time"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/autonomy"
	"github.com/blackwell-systems/scout-and-wave-go/pkg/engine"
	"github.com/spf13/cobra"
)

// newDaemonCmd returns the "daemon" subcommand which runs the continuous
// Scout-and-Wave processing loop.
func newDaemonCmd() *cobra.Command {
	var (
		autonomyLevel string
		model         string
		pollInterval  time.Duration
	)

	cmd := &cobra.Command{
		Use:   "daemon",
		Short: "Run the SAW daemon loop (processes queue items continuously)",
		Long: `Start the SAW daemon which continuously monitors the IMPL queue,
runs Scout/Wave cycles, auto-remediates failures, and advances the queue.
Events are streamed as JSON lines to stdout.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Load autonomy config from saw.config.json.
			cfg, err := autonomy.LoadConfig(repoDir)
			if err != nil {
				return fmt.Errorf("daemon: load config: %w", err)
			}

			// Override level if --autonomy flag is set.
			if autonomyLevel != "" {
				lvl, err := autonomy.ParseLevel(autonomyLevel)
				if err != nil {
					return fmt.Errorf("daemon: invalid autonomy level: %w", err)
				}
				cfg.Level = lvl
			}

			opts := engine.DaemonOpts{
				RepoPath:       repoDir,
				AutonomyConfig: cfg,
				ChatModel:      model,
				PollInterval:   pollInterval,
				OnEvent: func(ev engine.Event) {
					data, err := json.Marshal(ev)
					if err != nil {
						return
					}
					fmt.Fprintln(cmd.OutOrStdout(), string(data))
				},
			}

			// Set up signal handling for clean shutdown.
			ctx, stop := signal.NotifyContext(cmd.Context(), syscall.SIGINT, syscall.SIGTERM)
			defer stop()

			// Notify when shutting down.
			go func() {
				<-ctx.Done()
				fmt.Fprintln(cmd.ErrOrStderr(), "daemon shutting down...")
			}()

			if err := engine.RunDaemon(ctx, opts); err != nil {
				return fmt.Errorf("daemon: %w", err)
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&autonomyLevel, "autonomy", "", "Override autonomy level (gated|supervised|autonomous)")
	cmd.Flags().StringVar(&model, "model", "", "Chat model to use")
	cmd.Flags().DurationVar(&pollInterval, "poll-interval", 30*time.Second, "How often to check the queue")

	return cmd
}
