package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/engine"
)

func runRunWave(args []string) error {
	fs := flag.NewFlagSet("run-wave", flag.ContinueOnError)
	waveNum := fs.Int("wave", 0, "Wave number (required)")
	repoDir := fs.String("repo-dir", ".", "Repository directory")

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return fmt.Errorf("run-wave: %w", err)
	}

	if fs.NArg() < 1 {
		return fmt.Errorf("run-wave: manifest path required\nUsage: saw run-wave <manifest-path> --wave <N> [--repo-dir <path>]")
	}
	if *waveNum == 0 {
		return fmt.Errorf("run-wave: --wave is required")
	}

	manifestPath := fs.Arg(0)
	result, err := engine.RunWaveFull(context.Background(), engine.RunWaveFullOpts{
		ManifestPath: manifestPath,
		RepoPath:     *repoDir,
		WaveNum:      *waveNum,
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
}
