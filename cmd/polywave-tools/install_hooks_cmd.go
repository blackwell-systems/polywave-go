package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

func newInstallHooksCmd() *cobra.Command {
	var repoDir string

	cmd := &cobra.Command{
		Use:   "install-hooks",
		Short: "Install Polywave pre-commit quality gate hook into .git/hooks/pre-commit",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runInstallHooks(repoDir)
		},
	}

	cmd.Flags().StringVar(&repoDir, "repo-dir", ".", "Repository directory")

	return cmd
}

func runInstallHooks(repoDir string) error {
	absRepoDir, err := filepath.Abs(repoDir)
	if err != nil {
		return fmt.Errorf("failed to resolve repo dir: %w", err)
	}

	// Resolve the .git directory (handles both regular repos and worktrees)
	gitDir := filepath.Join(absRepoDir, ".git")

	gitStat, err := os.Stat(gitDir)
	if err != nil {
		return fmt.Errorf("failed to stat .git: %w", err)
	}

	var hooksDir string
	if gitStat.IsDir() {
		// Regular repo
		hooksDir = filepath.Join(gitDir, "hooks")
	} else {
		// Worktree: .git is a file pointing to actual git dir
		gitFileContent, err := os.ReadFile(gitDir)
		if err != nil {
			return fmt.Errorf("failed to read .git file: %w", err)
		}

		gitdirLine := strings.TrimSpace(string(gitFileContent))
		if !strings.HasPrefix(gitdirLine, "gitdir: ") {
			return fmt.Errorf("invalid .git file format: %s", gitdirLine)
		}

		actualGitDir := strings.TrimPrefix(gitdirLine, "gitdir: ")
		if !filepath.IsAbs(actualGitDir) {
			actualGitDir = filepath.Join(absRepoDir, actualGitDir)
		}
		hooksDir = filepath.Join(actualGitDir, "hooks")
	}

	hookPath := filepath.Join(hooksDir, "pre-commit")

	const snippet = `# Polywave pre-commit quality gate (M4)
polywave-tools pre-commit-check --repo-dir "$(git rev-parse --show-toplevel)"`

	// Read existing hook content
	existing, err := os.ReadFile(hookPath)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to read existing hook: %w", err)
	}

	existingContent := string(existing)

	// Idempotent: already installed
	if strings.Contains(existingContent, "polywave-tools pre-commit-check") {
		fmt.Println("[install-hooks] already installed")
		return nil
	}

	// Ensure hooks directory exists
	if err := os.MkdirAll(hooksDir, 0755); err != nil {
		return fmt.Errorf("failed to create hooks directory: %w", err)
	}

	var newContent string
	if existingContent == "" {
		// Fresh install
		newContent = "#!/bin/sh\nset -e\n" + snippet + "\n"
	} else {
		// Append to existing hook
		newContent = existingContent
		if !strings.HasSuffix(newContent, "\n") {
			newContent += "\n"
		}
		newContent += snippet + "\n"
	}

	if err := os.WriteFile(hookPath, []byte(newContent), 0755); err != nil {
		return fmt.Errorf("failed to write hook: %w", err)
	}

	fmt.Printf("[install-hooks] pre-commit hook installed at %s\n", hookPath)
	return nil
}
