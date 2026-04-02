package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/protocol"
	"github.com/spf13/cobra"
)

// PreWaveValidateResult combines E16 validation and E35 gap detection results.
type PreWaveValidateResult struct {
	Validation    protocol.FullValidateData `json:"validation"`
	E35Gaps       E35GapsResult             `json:"e35_gaps"`
	TestCascade   TestCascadeCheckResult    `json:"test_cascade"`
	WaveStructure WaveStructureCheckResult  `json:"wave_structure"` // Step 4
}

// E35GapsResult holds the result of E35 same-package caller detection.
type E35GapsResult struct {
	Passed bool              `json:"passed"`
	Gaps   []protocol.E35Gap `json:"gaps"`
}

// TestCascadeCheckResult holds the result of the check-test-cascade pre-flight gate.
type TestCascadeCheckResult struct {
	Passed bool                        `json:"passed"`
	Errors []protocol.TestCascadeError `json:"errors"`
}

// WaveStructureCheckResult holds the result of the suggest-wave-structure pre-flight gate.
type WaveStructureCheckResult struct {
	Passed   bool                             `json:"passed"`
	Problems []protocol.WaveStructureProblem `json:"problems"`
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
  3. E46 test cascade detection
  4. Wave structure check (X004/X005)

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
			validateRes := protocol.FullValidate(context.TODO(), manifestPath, protocol.FullValidateOpts{
				AutoFix: autoFix,
			})

			// Step 2: Run E35 detection
			manifest, err := protocol.Load(context.TODO(), manifestPath)
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

			// Step 3: Run test cascade check
			cascadeRes := protocol.CheckTestCascade(context.TODO(), manifest, repoRoot)
			var testCascadeResult TestCascadeCheckResult
			if cascadeRes.IsFatal() {
				// Log but don't fail — cascade check failure is non-blocking if manifest is valid
				fmt.Fprintf(os.Stderr, "pre-wave-validate: test cascade check failed: %v\n", cascadeRes.Errors)
				testCascadeResult = TestCascadeCheckResult{Passed: true, Errors: nil}
			} else {
				cascadeErrors := cascadeRes.GetData()
				testCascadeResult = TestCascadeCheckResult{
					Passed: len(cascadeErrors) == 0,
					Errors: cascadeErrors,
				}
			}

			// Step 4: Run wave structure check
			waveStructureRes := protocol.SuggestWaveStructure(context.TODO(), manifest, repoRoot)
			var waveStructureResult WaveStructureCheckResult
			if waveStructureRes.IsFatal() {
				// Log but don't fail — fatal means the check itself errored (e.g. nil manifest),
				// not that problems were found.
				fmt.Fprintf(os.Stderr, "pre-wave-validate: wave structure check failed: %v\n", waveStructureRes.Errors)
				waveStructureResult = WaveStructureCheckResult{Passed: true, Problems: nil}
			} else {
				problems := waveStructureRes.GetData()
				waveStructureResult = WaveStructureCheckResult{
					Passed:   len(problems) == 0,
					Problems: problems,
				}
			}

			// Combine results
			output := PreWaveValidateResult{
				Validation: validateRes.GetData(),
				E35Gaps: E35GapsResult{
					Passed: len(e35Gaps) == 0,
					Gaps:   e35Gaps,
				},
				TestCascade:   testCascadeResult,
				WaveStructure: waveStructureResult, // Step 4
			}

			// Step 5: JSON output
			jsonOut, err := json.MarshalIndent(output, "", "  ")
			if err != nil {
				return fmt.Errorf("failed to marshal output: %w", err)
			}
			fmt.Println(string(jsonOut))

			// Exit 1 if validation, E35, test cascade, or wave structure failed
			if !output.Validation.Valid || !output.E35Gaps.Passed || !output.TestCascade.Passed || !output.WaveStructure.Passed {
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
