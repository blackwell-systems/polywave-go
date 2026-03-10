package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/protocol"
)

func runMergeAgents(args []string) error {
	fs := flag.NewFlagSet("merge-agents", flag.ContinueOnError)
	waveNum := fs.Int("wave", 0, "Wave number (required)")
	repoDir := fs.String("repo-dir", ".", "Repository directory")

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return fmt.Errorf("merge-agents: %w", err)
	}

	if fs.NArg() < 1 {
		return fmt.Errorf("merge-agents: manifest path required\nUsage: saw merge-agents <manifest-path> --wave <N> [--repo-dir <path>]")
	}
	if *waveNum == 0 {
		return fmt.Errorf("merge-agents: --wave is required")
	}

	manifestPath := fs.Arg(0)
	result, err := protocol.MergeAgents(manifestPath, *waveNum, *repoDir)
	if err != nil {
		return fmt.Errorf("merge-agents: %w", err)
	}

	out, _ := json.MarshalIndent(result, "", "  ")
	fmt.Println(string(out))

	if !result.Success {
		os.Exit(1)
	}
	return nil
}
