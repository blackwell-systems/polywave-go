package main

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/protocol"
	"github.com/spf13/cobra"
)

// RunPreCommitCheck discovers and executes the lint gate for the repo.
// Returns nil on pass (including silent pass with no gate configured).
// Returns error with lint output on failure.
func RunPreCommitCheck(repoDir string) error {
	command, err := protocol.DiscoverLintGate(repoDir)
	if err != nil {
		return fmt.Errorf("pre-commit-check: %w", err)
	}
	if command == "" {
		return nil
	}

	fmt.Fprintf(os.Stderr, "[pre-commit] running lint gate: %s\n", command)

	cmd := exec.Command("sh", "-c", command)
	cmd.Dir = repoDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return fmt.Errorf("lint gate failed: exit %d", exitErr.ExitCode())
		}
		return fmt.Errorf("lint gate failed: %w", err)
	}
	return nil
}

func newPreCommitCheckCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "pre-commit-check",
		Short: "Run lint gate from active IMPL doc (pre-commit hook entry point)",
		RunE: func(cmd *cobra.Command, args []string) error {
			return RunPreCommitCheck(repoDir)
		},
	}
	return cmd
}
