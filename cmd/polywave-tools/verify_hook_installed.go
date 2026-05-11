package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

func newVerifyHookInstalledCmd() *cobra.Command {
	var waveNum int

	cmd := &cobra.Command{
		Use:   "verify-hook-installed <worktree-path>",
		Short: "Verify pre-commit hook is installed and functional in worktree",
		Long: `Verifies that the pre-commit hook exists in the worktree's .git/hooks directory,
contains isolation check logic, and is executable. This is Layer 0 of worktree
isolation enforcement (E4).

The hook should block commits to main/master branches unless POLYWAVE_ALLOW_MAIN_COMMIT
is set. This prevents agents from accidentally committing to the main branch instead
of their isolated worktree branch.

Exit code 0 if hook is valid, 1 if hook is missing/broken.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			worktreePath := args[0]

			result, err := verifyHookInstalled(worktreePath, waveNum)
			if err != nil {
				return err
			}

			out, _ := json.MarshalIndent(result, "", "  ")
			fmt.Println(string(out))

			if !result.Valid {
				return fmt.Errorf("pre-commit hook verification failed: %s", result.Reason)
			}

			return nil
		},
	}

	cmd.Flags().IntVar(&waveNum, "wave", 0, "Wave number (for context in error messages)")

	return cmd
}

type HookVerificationResult struct {
	Valid      bool   `json:"valid"`
	Reason     string `json:"reason,omitempty"`
	HookPath   string `json:"hook_path"`
	Executable bool   `json:"executable"`
	HasLogic   bool   `json:"has_logic"`
}

func verifyHookInstalled(worktreePath string, waveNum int) (*HookVerificationResult, error) {
	// Resolve absolute path
	absWorktreePath, err := filepath.Abs(worktreePath)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve worktree path: %w", err)
	}

	// Determine actual git directory (handles both regular repos and worktrees)
	gitDir := filepath.Join(absWorktreePath, ".git")

	// Check if .git is a file (worktree) or directory (regular repo)
	gitStat, err := os.Stat(gitDir)
	if err != nil {
		return nil, fmt.Errorf("failed to stat .git: %w", err)
	}

	var hookPath string
	if gitStat.IsDir() {
		// Regular repo: hooks are in .git/hooks/
		hookPath = filepath.Join(gitDir, "hooks", "pre-commit")
	} else {
		// Worktree: .git is a file pointing to actual git dir
		// Read the gitdir pointer
		gitFileContent, err := os.ReadFile(gitDir)
		if err != nil {
			return nil, fmt.Errorf("failed to read .git file: %w", err)
		}

		// Parse "gitdir: /path/to/repo/.git/worktrees/name"
		gitdirLine := strings.TrimSpace(string(gitFileContent))
		if !strings.HasPrefix(gitdirLine, "gitdir: ") {
			return nil, fmt.Errorf("invalid .git file format: %s", gitdirLine)
		}

		actualGitDir := strings.TrimPrefix(gitdirLine, "gitdir: ")
		hookPath = filepath.Join(actualGitDir, "hooks", "pre-commit")
	}

	result := &HookVerificationResult{
		HookPath:   hookPath,
		Valid:      false,
		Executable: false,
		HasLogic:   false,
	}

	// Check if hook file exists
	stat, err := os.Stat(hookPath)
	if os.IsNotExist(err) {
		result.Reason = "pre-commit hook file does not exist"
		return result, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to stat hook file: %w", err)
	}

	// Check if hook is executable
	if stat.Mode()&0111 == 0 {
		result.Reason = "pre-commit hook exists but is not executable"
		return result, nil
	}
	result.Executable = true

	// Read hook content and verify it contains isolation logic
	content, err := os.ReadFile(hookPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read hook file: %w", err)
	}

	hookContent := string(content)

	// Check for key isolation logic markers
	hasIsolationCheck := strings.Contains(hookContent, "POLYWAVE_ALLOW_MAIN_COMMIT") ||
		strings.Contains(hookContent, "Polywave pre-commit guard")

	if !hasIsolationCheck {
		result.Reason = "pre-commit hook exists and is executable, but does not contain SAW isolation logic"
		return result, nil
	}
	result.HasLogic = true

	// All checks passed
	result.Valid = true
	return result, nil
}
