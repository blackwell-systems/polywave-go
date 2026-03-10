package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/protocol"
)

func runCleanup(args []string) error {
	fs := flag.NewFlagSet("cleanup", flag.ContinueOnError)
	waveNum := fs.Int("wave", 0, "Wave number (required)")
	repoDir := fs.String("repo-dir", ".", "Repository directory")

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return fmt.Errorf("cleanup: %w", err)
	}

	if fs.NArg() < 1 {
		return fmt.Errorf("cleanup: manifest path required\nUsage: saw cleanup <manifest-path> --wave <N> [--repo-dir <path>]")
	}
	if *waveNum == 0 {
		return fmt.Errorf("cleanup: --wave is required")
	}

	manifestPath := fs.Arg(0)
	result, err := protocol.Cleanup(manifestPath, *waveNum, *repoDir)
	if err != nil {
		return fmt.Errorf("cleanup: %w", err)
	}

	out, _ := json.MarshalIndent(result, "", "  ")
	fmt.Println(string(out))
	return nil
}
