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
	var noPrioritize bool

	cmd := &cobra.Command{
		Use:   "run-wave <manifest-path>",
		Short: "Execute full wave lifecycle (create, verify, merge, build, cleanup)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			manifestPath := args[0]

			// Set environment variable to disable prioritization if flag is set
			if noPrioritize {
				os.Setenv("SAW_NO_PRIORITIZE", "1")
			}

			res := engine.RunWaveFull(context.Background(), engine.RunWaveFullOpts{
				ManifestPath: manifestPath,
				RepoPath:     repoDir,
				WaveNum:      waveNum,
			})

			// Always output result data (includes partial results on failure)
			data := res.GetData()
			out, _ := json.MarshalIndent(data, "", "  ")
			fmt.Println(string(out))

			if res.IsFatal() {
				return fmt.Errorf("run-wave: %s", res.Errors[0].Message)
			}
			if !data.Success {
				return fmt.Errorf("run-wave: wave execution failed")
			}
			return nil
		},
	}

	cmd.Flags().IntVar(&waveNum, "wave", 0, "Wave number (required)")
	_ = cmd.MarkFlagRequired("wave")
	cmd.Flags().BoolVar(&noPrioritize, "no-prioritize", false, "Disable agent launch prioritization (use declaration order)")

	return cmd
}
