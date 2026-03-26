package main

import (
	"encoding/json"
	"fmt"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/collision"
	"github.com/spf13/cobra"
)

func newCheckTypeCollisionsCmd() *cobra.Command {
	var waveNum int
	var repoDir string

	cmd := &cobra.Command{
		Use:   "check-type-collisions <manifest-path>",
		Short: "Detect type name collisions across agent branches",
		Long: `Scans all agent branches in the specified wave for duplicate type
declarations in the same package. Uses AST parsing to extract type names from
git diffs (base..branch). Returns structured collision report with suggested
resolution.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			manifestPath := args[0]
			if repoDir == "" {
				repoDir = "."
			}

			report, err := collision.DetectCollisions(manifestPath, waveNum, repoDir)
			if err != nil {
				return fmt.Errorf("check-type-collisions: %w", err)
			}

			out, _ := json.MarshalIndent(report, "", "  ")
			fmt.Println(string(out))

			if !report.Valid {
				return fmt.Errorf("check-type-collisions: type collisions detected")
			}
			return nil
		},
	}

	cmd.Flags().IntVar(&waveNum, "wave", 0, "Wave number (required)")
	cmd.Flags().StringVar(&repoDir, "repo-dir", ".", "Repository root directory")
	_ = cmd.MarkFlagRequired("wave")

	return cmd
}
