package main

import (
	"fmt"
	"os"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/engine"
	"github.com/spf13/cobra"
)

// newInitCmd returns the cobra.Command for "sawtools init".
func newInitCmd() *cobra.Command {
	var repoFlag string
	var forceFlag bool

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize a project for SAW (auto-detects language, build, and test commands)",
		Long: `Auto-detect project language (Go, Rust, Node, Python, Ruby, Makefile) and
generate a saw.config.json with sensible defaults. No manual configuration
needed for most projects.

After running init, use /saw scout "feature" in Claude Code or saw serve for the web UI.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// 1. Resolve repoDir (cwd fallback); make absolute
			repoDir := repoFlag
			if repoDir == "" {
				cwd, err := os.Getwd()
				if err != nil {
					return fmt.Errorf("init: cannot determine working directory: %w", err)
				}
				repoDir = cwd
			}

			// 2. Call engine.RunInit
			res := engine.RunInit(engine.InitOpts{
				RepoDir: repoDir,
				Force:   forceFlag,
			})

			// 3. Handle error: special message for already-exists case
			if res.IsFatal() {
				if len(res.Errors) > 0 {
					return fmt.Errorf("%s", res.Errors[0].Message)
				}
				return fmt.Errorf("init failed")
			}
			if res.IsPartial() {
				data := res.GetData()
				if data.AlreadyExists {
					fmt.Fprintf(cmd.OutOrStdout(), "saw.config.json already exists. Use --force to overwrite.\n")
				}
				if len(res.Errors) > 0 {
					return fmt.Errorf("%s", res.Errors[0].Message)
				}
				return fmt.Errorf("init failed")
			}
			result := res.GetData()

			// 4. Print human-readable install check output
			printHumanOutput(cmd, result.InstallResult)

			// 5. Print next-step messages
			fmt.Fprintf(cmd.OutOrStdout(), "\nSAW initialized for %s (%s).\n", result.ProjectName, result.Language)
			fmt.Fprintf(cmd.OutOrStdout(), "  Config written to: %s\n", result.ConfigPath)
			fmt.Fprintf(cmd.OutOrStdout(), "\nNext steps:\n")
			fmt.Fprintf(cmd.OutOrStdout(), "  /saw scout \"describe your feature\"   Plan a feature (Claude Code skill)\n")
			fmt.Fprintf(cmd.OutOrStdout(), "  saw serve                            Open the web dashboard\n")
			fmt.Fprintf(cmd.OutOrStdout(), "\nLearn more:\n")
			fmt.Fprintf(cmd.OutOrStdout(), "  sawtools --help                      All CLI commands\n")
			fmt.Fprintf(cmd.OutOrStdout(), "  https://github.com/blackwell-systems/scout-and-wave#quick-start\n")

			// 6. Print install failure help if verdict is FAIL
			if result.InstallResult.Verdict == "FAIL" {
				fmt.Fprintf(cmd.OutOrStdout(), "\nsawtools not found. Install it:\n")
				fmt.Fprintf(cmd.OutOrStdout(), "  go install github.com/blackwell-systems/scout-and-wave-go/cmd/sawtools@latest\n\n")
				fmt.Fprintf(cmd.OutOrStdout(), "Or build from source:\n")
				fmt.Fprintf(cmd.OutOrStdout(), "  git clone https://github.com/blackwell-systems/scout-and-wave-go.git\n")
				fmt.Fprintf(cmd.OutOrStdout(), "  cd scout-and-wave-go && go build -o ~/.local/bin/sawtools ./cmd/sawtools\n")
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&repoFlag, "repo", "", "Directory to initialize (default: current working directory)")
	cmd.Flags().BoolVar(&forceFlag, "force", false, "Overwrite existing saw.config.json")

	return cmd
}
