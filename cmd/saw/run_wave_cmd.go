package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/engine"
	"github.com/spf13/cobra"
)

func newRunWaveCmd() *cobra.Command {
	var waveNum int

	cmd := &cobra.Command{
		Use:   "run-wave <manifest-path>",
		Short: "Execute full wave lifecycle (create, verify, merge, build, cleanup)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			manifestPath := args[0]

			result, err := engine.RunWaveFull(context.Background(), engine.RunWaveFullOpts{
				ManifestPath: manifestPath,
				RepoPath:     repoDir,
				WaveNum:      waveNum,
			})
			if err != nil {
				// Still output partial result if available
				if result != nil {
					out, _ := json.MarshalIndent(result, "", "  ")
					fmt.Println(string(out))
				}
				return fmt.Errorf("run-wave: %w", err)
			}

			out, _ := json.MarshalIndent(result, "", "  ")
			fmt.Println(string(out))

			if !result.Success {
				os.Exit(1)
			}
			return nil
		},
	}

	cmd.Flags().IntVar(&waveNum, "wave", 0, "Wave number (required)")
	_ = cmd.MarkFlagRequired("wave")

	return cmd
}
