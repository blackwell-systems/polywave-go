package main

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/protocol"
	"github.com/blackwell-systems/scout-and-wave-go/pkg/retry"
	"github.com/spf13/cobra"
)

func newRetryCmd() *cobra.Command {
	var waveNum int
	var gateType string
	var maxRetries int
	var repoRoot string

	cmd := &cobra.Command{
		Use:   "retry <impl-doc>",
		Short: "Generate a retry IMPL doc for a failed quality gate (E24 retry loop)",
		Long: `Wraps the E24 verification loop to generate a single-agent retry IMPL doc
targeting the files that failed a quality gate. The retry agent is NOT executed
by this command — it only generates the IMPL doc for the caller to act on.

State transitions:
  attempt < max-retries  → final_state = "retrying"
  attempt >= max-retries → final_state = "blocked"

Output is JSON to stdout for programmatic consumption.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			implPath := args[0]

			// Determine repo root: prefer local --repo-root, then global --repo-dir,
			// then infer from the IMPL path (docs/IMPL/IMPL-foo.yaml → three levels up).
			effectiveRepoRoot := repoRoot
			if effectiveRepoRoot == "" {
				effectiveRepoRoot = repoDir
			}
			if effectiveRepoRoot == "" || effectiveRepoRoot == "." {
				// Infer: docs/IMPL/IMPL-foo.yaml → three levels up is repo root
				inferred := filepath.Dir(filepath.Dir(filepath.Dir(implPath)))
				if inferred != "" && inferred != "." {
					effectiveRepoRoot = inferred
				} else {
					effectiveRepoRoot = "."
				}
			}

			// Load the parent IMPL manifest.
			manifest, err := protocol.Load(context.TODO(), implPath)
			if err != nil {
				return fmt.Errorf("retry: failed to load IMPL doc: %w", err)
			}

			// Collect files owned by agents in the target wave.
			failedFiles := collectWaveFiles(manifest, waveNum)

			// Build retry configuration.
			cfg := retry.RetryConfig{
				MaxRetries: maxRetries,
				IMPLPath:   implPath,
				RepoPath:   effectiveRepoRoot,
			}

			// Create the retry loop and run one attempt.
			rl := retry.NewRetryLoop(cfg)

			gate := retry.QualityGateFailure{
				GateType:    gateType,
				FailedFiles: failedFiles,
			}

			result, err := rl.Run(cmd.Context(), gate, nil)
			if err != nil {
				return fmt.Errorf("retry: %w", err)
			}

			// Print JSON result to stdout.
			out, err := json.MarshalIndent(result, "", "  ")
			if err != nil {
				return fmt.Errorf("retry: failed to marshal result: %w", err)
			}
			fmt.Fprintln(cmd.OutOrStdout(), string(out))

			return nil
		},
	}

	cmd.Flags().IntVar(&waveNum, "wave", 0, "Wave number that failed (required)")
	cmd.Flags().StringVar(&gateType, "gate-type", "", "Type of gate that failed: build, test, lint (required)")
	cmd.Flags().IntVar(&maxRetries, "max-retries", 2, "Maximum retry attempts before transitioning to blocked state")
	cmd.Flags().StringVar(&repoRoot, "repo-root", "", "Repository root directory (defaults to --repo-dir or inferred from impl path)")

	_ = cmd.MarkFlagRequired("wave")
	_ = cmd.MarkFlagRequired("gate-type")

	return cmd
}

// collectWaveFiles returns all files owned by agents in the specified wave.
// If waveNum is 0 or no matching wave is found, all file ownership entries are
// returned (safe fallback for retry generation).
func collectWaveFiles(m *protocol.IMPLManifest, waveNum int) []string {
	seen := make(map[string]bool)
	var files []string

	for _, fo := range m.FileOwnership {
		if waveNum == 0 || fo.Wave == waveNum {
			if !seen[fo.File] {
				seen[fo.File] = true
				files = append(files, fo.File)
			}
		}
	}

	return files
}
