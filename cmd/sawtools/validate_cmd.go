package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/protocol"
	"github.com/spf13/cobra"
)

// inferRepoDir walks up from startDir until it finds a directory containing
// go.mod or go.sum, returning that directory as the repo root. Returns empty
// string if none found (caller treats this as "skip file-existence check").
func inferRepoDir(startDir string) string {
	dir := startDir
	for {
		for _, marker := range []string{"go.mod", "go.sum"} {
			if _, err := os.Stat(filepath.Join(dir, marker)); err == nil {
				return dir
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			// Reached filesystem root without finding a marker.
			return ""
		}
		dir = parent
	}
}

func newValidateCmd() *cobra.Command {
	var useSolver bool
	var autoFix bool
	var repoDir string
	cmd := &cobra.Command{
		Use:   "validate <manifest-path>",
		Short: "Validate a YAML IMPL manifest against protocol invariants",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			manifestPath := args[0]

			// Determine effective repo dir for file-existence checks.
			// Prefer explicit --repo-dir; fall back to inference from manifest location.
			effectiveRepoDir := repoDir
			if effectiveRepoDir == "" {
				manifestDir := filepath.Dir(manifestPath)
				effectiveRepoDir = inferRepoDir(manifestDir)
			}

			res := protocol.FullValidate(manifestPath, protocol.FullValidateOpts{
				AutoFix:   autoFix,
				UseSolver: useSolver,
				RepoPath:  effectiveRepoDir,
			})

			data := res.GetData()
			out, _ := json.MarshalIndent(data, "", "  ")
			fmt.Println(string(out))

			if !data.Valid {
				return fmt.Errorf("validate: manifest is not valid")
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&useSolver, "solver", false, "use CSP solver for wave assignment validation")
	cmd.Flags().BoolVar(&autoFix, "fix", false, "auto-correct fixable issues (e.g. invalid gate types -> custom, unknown keys -> stripped)")
	cmd.Flags().StringVar(&repoDir, "repo-dir", "", "repo root for file-existence checks (inferred from manifest path if not set)")
	return cmd
}
