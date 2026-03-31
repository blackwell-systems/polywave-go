package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/protocol"
	"github.com/spf13/cobra"
)

// RunPreCommitCheck discovers and executes the lint and build gates for the repo.
// Returns nil on pass (including silent pass with no gate configured).
// Returns error with gate output on failure.
func RunPreCommitCheck(repoDir string) error {
	ctx := context.Background()
	// Lint gate (existing behavior)
	lintCmd, err := protocol.DiscoverLintGate(ctx, repoDir)
	if err != nil {
		return fmt.Errorf("pre-commit-check: %w", err)
	}
	if lintCmd != "" {
		fmt.Fprintf(os.Stderr, "[pre-commit] running lint gate: %s\n", lintCmd)
		if err := runGateCommand(repoDir, lintCmd); err != nil {
			return err
		}
	}

	// Build gate (new — runs after lint so lint fails fast)
	buildCmd, err := protocol.DiscoverBuildGate(ctx, repoDir)
	if err != nil {
		return fmt.Errorf("pre-commit-check: %w", err)
	}
	if buildCmd != "" {
		fmt.Fprintf(os.Stderr, "[pre-commit] running build gate: %s\n", buildCmd)
		if err := runGateCommand(repoDir, buildCmd); err != nil {
			return err
		}
	}

	return nil
}

// runGateCommand executes a shell command in repoDir, streaming output
// to stdout/stderr. Returns error on non-zero exit.
func runGateCommand(repoDir, command string) error {
	cmd := exec.Command("sh", "-c", command)
	cmd.Dir = repoDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return fmt.Errorf("gate failed: exit %d", exitErr.ExitCode())
		}
		return fmt.Errorf("gate failed: %w", err)
	}
	return nil
}

func newPreCommitCheckCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "pre-commit-check",
		Short: "Run lint and build gates from active IMPL doc (pre-commit hook entry point)",
		RunE: func(cmd *cobra.Command, args []string) error {
			return RunPreCommitCheck(repoDir)
		},
	}
	return cmd
}
