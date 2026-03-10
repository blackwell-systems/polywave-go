package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/protocol"
)

// runCreateWorktrees creates git worktrees for all agents in a wave.
// Command: saw create-worktrees <manifest-path> --wave <N> [--repo-dir <path>]
// Outputs JSON result with created worktrees and branches.
func runCreateWorktrees(args []string) error {
	fs := flag.NewFlagSet("create-worktrees", flag.ContinueOnError)
	waveNum := fs.Int("wave", 0, "Wave number (required)")
	repoDir := fs.String("repo-dir", ".", "Repository directory")

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return fmt.Errorf("create-worktrees: %w", err)
	}

	if fs.NArg() < 1 {
		return fmt.Errorf("create-worktrees: manifest path required\nUsage: saw create-worktrees <manifest-path> --wave <N> [--repo-dir <path>]")
	}
	if *waveNum == 0 {
		return fmt.Errorf("create-worktrees: --wave is required")
	}

	manifestPath := fs.Arg(0)
	result, err := protocol.CreateWorktrees(manifestPath, *waveNum, *repoDir)
	if err != nil {
		return fmt.Errorf("create-worktrees: %w", err)
	}

	out, _ := json.MarshalIndent(result, "", "  ")
	fmt.Println(string(out))
	return nil
}
