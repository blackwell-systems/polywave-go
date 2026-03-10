package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/protocol"
)

func runVerifyCommits(args []string) error {
	fs := flag.NewFlagSet("verify-commits", flag.ContinueOnError)
	waveNum := fs.Int("wave", 0, "Wave number (required)")
	repoDir := fs.String("repo-dir", ".", "Repository directory")

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return fmt.Errorf("verify-commits: %w", err)
	}

	if fs.NArg() < 1 {
		return fmt.Errorf("verify-commits: manifest path required\nUsage: saw verify-commits <manifest-path> --wave <N> [--repo-dir <path>]")
	}
	if *waveNum == 0 {
		return fmt.Errorf("verify-commits: --wave is required")
	}

	manifestPath := fs.Arg(0)
	result, err := protocol.VerifyCommits(manifestPath, *waveNum, *repoDir)
	if err != nil {
		return fmt.Errorf("verify-commits: %w", err)
	}

	out, _ := json.MarshalIndent(result, "", "  ")
	fmt.Println(string(out))

	if !result.AllValid {
		os.Exit(1)
	}
	return nil
}
