package main

import "github.com/spf13/cobra"

// repoDir is the repository root directory, bound via --repo-dir persistent flag.
// All subcommand RunE closures read this package-level variable.
var repoDir string

// newRootCmd returns the configured root cobra.Command.
// Called by main() in main.go. Subcommands are added there via AddCommand.
func newRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sawtools",
		Short: "SAW Protocol SDK CLI (toolkit)",
	}

	cmd.PersistentFlags().StringVar(&repoDir, "repo-dir", ".", "Repository root directory")

	return cmd
}
