package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/blackwell-systems/polywave-go/pkg/engine"
	"github.com/spf13/cobra"
)

func newImportImplsCmd() *cobra.Command {
	var (
		programPath string
		fromImpls   []string
		discover    bool
		repoDir     string
	)

	cmd := &cobra.Command{
		Use:   "import-impls",
		Short: "Import existing IMPL docs into a PROGRAM manifest",
		Long: `Import existing IMPL documents into a PROGRAM manifest for tiered execution.

Examples:
  # Discover all IMPL docs in the repo
  polywave-tools import-impls --program PROGRAM-my-feature.yaml --discover

  # Import specific IMPL docs
  polywave-tools import-impls --program PROGRAM-my-feature.yaml --from-impls IMPL-a.yaml IMPL-b.yaml

  # Discover from a specific repo directory
  polywave-tools import-impls --program PROGRAM-my-feature.yaml --discover --repo-dir /path/to/repo`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if programPath == "" {
				return fmt.Errorf("import-impls: --program flag is required")
			}
			if !discover && len(fromImpls) == 0 {
				return fmt.Errorf("import-impls: must specify --discover or --from-impls")
			}

			// Default repoDir to cwd
			if repoDir == "" {
				cwd, err := os.Getwd()
				if err != nil {
					return fmt.Errorf("import-impls: failed to get current directory: %w", err)
				}
				repoDir = cwd
			}

			res := engine.ImportImpls(cmd.Context(), engine.ImportImplsOpts{
				ProgramPath: programPath,
				FromImpls:   fromImpls,
				Discover:    discover,
				RepoDir:     repoDir,
			})
			if res.IsFatal() {
				if len(res.Errors) > 0 {
					return fmt.Errorf("%s: %s", res.Errors[0].Code, res.Errors[0].Message)
				}
				return fmt.Errorf("import-impls: operation failed")
			}

			out, _ := json.MarshalIndent(res.GetData(), "", "  ")
			fmt.Fprintln(cmd.OutOrStdout(), string(out))

			return nil
		},
	}

	cmd.Flags().StringVar(&programPath, "program", "", "Path to PROGRAM manifest (created if missing)")
	cmd.Flags().StringSliceVar(&fromImpls, "from-impls", nil, "Explicit IMPL doc paths to import")
	cmd.Flags().BoolVar(&discover, "discover", false, "Auto-discover IMPL docs in docs/IMPL/")
	cmd.Flags().StringVar(&repoDir, "repo-dir", "", "Repository root directory (default: cwd)")

	return cmd
}
