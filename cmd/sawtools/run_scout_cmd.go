package main

import (
	"fmt"
	"os"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/engine"
	"github.com/spf13/cobra"
)

func newRunScoutCmd() *cobra.Command {
	var (
		repoPath            string
		sawRepoPath         string
		implOutputPath      string // --impl-output-path: explicit IMPL doc output path
		scoutModel          string
		timeout             int    // minutes
		programManifestPath string // path to PROGRAM manifest
		noCritic            bool   // --no-critic: skip critic gate even if threshold met
		criticModel         string // --critic-model: override model for critic agent
		refreshBrief        bool   // --refresh-brief: re-run Scout preserving structure
	)

	cmd := &cobra.Command{
		Use:   "run-scout <feature-description>",
		Short: "I3: Automated Scout execution with validation and agent ID correction",
		Long: `Fully automated Scout workflow (Phase 5, I3 integration):

1. Launch Scout agent to analyze codebase and create IMPL doc
2. Wait for IMPL doc creation
3. Validate IMPL doc using E16 validation
4. Auto-correct agent IDs if validation fails (M1 integration)
5. Return validated, ready-to-execute IMPL doc

Examples:
  # Basic usage (infers repo from current directory)
  sawtools run-scout "Add audit logging to auth module"

  # Specify target repository
  sawtools run-scout "Add audit logging" --repo-dir /path/to/project

  # Custom Scout model
  sawtools run-scout "Add audit logging" --scout-model claude-opus-4-6

Output:
  - IMPL doc created at docs/IMPL/IMPL-<slug>.yaml
  - Validated and ready for wave execution
  - Agent IDs corrected if needed`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			featureDesc := args[0]

			// Resolve repo path (default to current directory).
			if repoPath == "" {
				cwd, err := os.Getwd()
				if err != nil {
					return fmt.Errorf("run-scout: failed to get current directory: %w", err)
				}
				repoPath = cwd
			}

			fmt.Printf("Launching Scout agent...\n")
			fmt.Printf("   Feature: %s\n", featureDesc)
			fmt.Printf("   Repository: %s\n", repoPath)
			fmt.Println()

			opts := engine.RunScoutFullOpts{
				Feature:             featureDesc,
				RepoPath:            repoPath,
				ImplOutputPath:      implOutputPath,
				SAWRepoPath:         sawRepoPath,
				ScoutModel:          scoutModel,
				Timeout:             timeout,
				ProgramManifestPath: programManifestPath,
				NoCritic:            noCritic,
				CriticModel:         criticModel,
				RefreshBrief:        refreshBrief,
				Logger:              newSawLogger(),
			}

			scoutResult := engine.RunScoutFull(cmd.Context(), opts, func(chunk string) {
				fmt.Print(chunk)
			})
			if scoutResult.IsFatal() {
				if len(scoutResult.Errors) > 0 {
					return fmt.Errorf("%s: %s", scoutResult.Errors[0].Code, scoutResult.Errors[0].Message)
				}
				return fmt.Errorf("run-scout: scout failed")
			}
			res := scoutResult.GetData()

			fmt.Println()
			fmt.Printf("IMPL doc: %s\n", res.IMPLPath)
			fmt.Println()
			fmt.Println("Next steps:")
			fmt.Println("  1. Review the IMPL doc")
			fmt.Println("  2. Run: sawtools run-wave --wave 1")

			return nil
		},
	}

	cmd.Flags().StringVar(&repoPath, "repo-dir", "", "Target repository path (default: current directory)")
	cmd.Flags().StringVar(&sawRepoPath, "saw-repo", "", "Scout-and-Wave protocol repo path (default: $SAW_REPO or ~/code/scout-and-wave)")
	cmd.Flags().StringVar(&implOutputPath, "impl-output-path", "", "Explicit IMPL doc output path (overrides default derivation from --repo-dir + slug)")
	cmd.Flags().StringVar(&scoutModel, "scout-model", "", "Scout model override (e.g., claude-opus-4-6)")
	cmd.Flags().IntVar(&timeout, "timeout", 10, "Timeout in minutes (default: 10)")
	cmd.Flags().StringVar(&programManifestPath, "program", "", "Path to PROGRAM manifest (Scout receives frozen contracts as input)")
	cmd.Flags().BoolVar(&noCritic, "no-critic", false, "Skip critic gate even if agent count threshold is met")
	cmd.Flags().StringVar(&criticModel, "critic-model", "", "Model override for critic agent (e.g., claude-opus-4-6)")
	cmd.Flags().BoolVar(&refreshBrief, "refresh-brief", false,
		"Re-run Scout preserving file_ownership/wave structure, only refreshing agent task descriptions")

	return cmd
}
