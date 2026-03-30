package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/protocol"
	"github.com/spf13/cobra"
)

// PreWaveValidateResult combines E16 validation and E35 gap detection results.
type PreWaveValidateResult struct {
	Validation protocol.FullValidateData `json:"validation"`
	E35Gaps    E35GapsResult             `json:"e35_gaps"`
}

// E35GapsResult holds the result of E35 same-package caller detection.
type E35GapsResult struct {
	Passed bool              `json:"passed"`
	Gaps   []protocol.E35Gap `json:"gaps"`
}

func newPreWaveValidateCmd() *cobra.Command {
	var waveNum int
	var autoFix bool

	cmd := &cobra.Command{
		Use:   "pre-wave-validate <manifest-path>",
		Short: "Run E16 validation + E35 gap detection before wave execution",
		Long: `Pre-wave validation combines:
  1. E16 validation (invariants, gates, contracts)
  2. E35 gap detection (same-package caller validation)

E35 detects when an agent owns a function definition but does not own all
call sites in the same package. This prevents build failures after merge.

Example from wave 1 post-mortem: Agent C owned CreateProgramWorktrees() in
pkg/protocol/worktree.go but didn't own the 2 call sites in
pkg/protocol/program_tier_prepare.go. After merge, build failed because
Agent C added a parameter but the unowned call sites still had the old signature.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			manifestPath := args[0]

			// Step 1: Run FullValidate (E16)
			validateRes := protocol.FullValidate(manifestPath, protocol.FullValidateOpts{
				AutoFix: autoFix,
			})

			// Step 2: Run E35 detection
			manifest, err := protocol.Load(manifestPath)
			if err != nil {
				return fmt.Errorf("failed to load manifest for E35 detection: %w", err)
			}

			// Determine repo root
			repoRoot := manifest.Repository
			if repoRoot == "" {
				// Try to get from current directory
				repoRoot, err = os.Getwd()
				if err != nil {
					return fmt.Errorf("failed to determine repo root: %w", err)
				}
			}

			e35Gaps, err := protocol.DetectE35Gaps(manifest, waveNum, repoRoot)
			if err != nil {
				return fmt.Errorf("E35 detection failed: %w", err)
			}

			// Step 3: Combine results
			output := PreWaveValidateResult{
				Validation: validateRes.GetData(),
				E35Gaps: E35GapsResult{
					Passed: len(e35Gaps) == 0,
					Gaps:   e35Gaps,
				},
			}

			// Step 4: JSON output
			jsonOut, err := json.MarshalIndent(output, "", "  ")
			if err != nil {
				return fmt.Errorf("failed to marshal output: %w", err)
			}
			fmt.Println(string(jsonOut))

			// Exit 1 if either validation or E35 failed
			if !output.Validation.Valid || !output.E35Gaps.Passed {
				return fmt.Errorf("pre-wave validation failed")
			}
			return nil
		},
	}

	cmd.Flags().IntVar(&waveNum, "wave", 1, "wave number to validate")
	cmd.Flags().BoolVar(&autoFix, "fix", false, "auto-correct fixable validation issues")
	cmd.MarkFlagRequired("wave")

	return cmd
}
