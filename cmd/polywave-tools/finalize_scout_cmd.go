package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/blackwell-systems/polywave-go/pkg/protocol"
	"github.com/spf13/cobra"
)

// FinalizeScoutResult holds the combined result of all Scout finalization checks.
type FinalizeScoutResult struct {
	// Step 1: Schema/structural validation (polywave-tools validate --fix)
	Validation protocol.FullValidateData `json:"validation"`

	// Step 2: Pre-wave validation (E35 + test cascade + wave structure)
	E35Gaps          E35GapsResult            `json:"e35_gaps"`
	TestCascade      TestCascadeCheckResult   `json:"test_cascade"`
	WaveStructure    WaveStructureCheckResult `json:"wave_structure"`
	StaleConstraints StaleConstraintsResult   `json:"stale_constraints"`

	// Step 3: Brief accuracy validation (polywave-tools validate-briefs)
	BriefValidation *protocol.BriefValidationData `json:"brief_validation,omitempty"`

	// Step 4: Injection method (auto-set)
	InjectionMethod string `json:"injection_method"`

	// Overall
	Passed     bool   `json:"passed"`
	FailedStep string `json:"failed_step,omitempty"`
}

func newFinalizeScoutCmd() *cobra.Command {
	var injectionMethod string

	cmd := &cobra.Command{
		Use:   "finalize-scout <manifest-path>",
		Short: "Run all Scout finalization checks in sequence",
		Long: `Consolidates Scout steps 16-18 into a single command:

  1. Schema/structural validation (polywave-tools validate --fix)
  2. Pre-wave validation: E35 gaps, test cascade, wave structure
  3. Brief accuracy validation (polywave-tools validate-briefs)
  4. Auto-set injection_method

Outputs structured JSON showing which step failed and why.
Scout retains the fix-between-retries loop — this command does
validation only, not auto-retry.

Example:
  polywave-tools finalize-scout docs/IMPL/IMPL-feature.yaml --injection-method hook`,
		Args:         cobra.ExactArgs(1),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			manifestPath := args[0]
			ctx := context.TODO()

			res := FinalizeScoutResult{
				InjectionMethod: injectionMethod,
			}

			// Step 1: Schema/structural validation with auto-fix
			validateRes := protocol.FullValidate(ctx, manifestPath, protocol.FullValidateOpts{
				AutoFix: true,
			})
			res.Validation = validateRes.GetData()

			if !res.Validation.Valid {
				res.FailedStep = "validation"
				return outputResult(cmd, res)
			}

			// Step 2: Pre-wave validation (E35 + test cascade + wave structure)
			manifest, err := protocol.Load(ctx, manifestPath)
			if err != nil {
				res.FailedStep = "load"
				return outputResultErr(cmd, res, fmt.Errorf("failed to load manifest: %w", err))
			}

			repoRoot := manifest.Repository
			if repoRoot == "" {
				repoRoot, err = os.Getwd()
				if err != nil {
					res.FailedStep = "repo_root"
					return outputResultErr(cmd, res, fmt.Errorf("failed to determine repo root: %w", err))
				}
			}

			// E35 gap detection
			e35Gaps, err := protocol.DetectE35Gaps(manifest, 1, repoRoot)
			if err != nil {
				res.FailedStep = "e35_detection"
				return outputResultErr(cmd, res, fmt.Errorf("E35 detection failed: %w", err))
			}
			res.E35Gaps = E35GapsResult{
				Passed: len(e35Gaps) == 0,
				Gaps:   e35Gaps,
			}

			// Test cascade check
			cascadeRes := protocol.CheckTestCascade(ctx, manifest, repoRoot)
			if cascadeRes.IsFatal() {
				res.TestCascade = TestCascadeCheckResult{Passed: true, Errors: nil}
			} else {
				cascadeErrors := cascadeRes.GetData()
				res.TestCascade = TestCascadeCheckResult{
					Passed: len(cascadeErrors) == 0,
					Errors: cascadeErrors,
				}
			}

			// Wave structure check
			waveStructureRes := protocol.SuggestWaveStructure(ctx, manifest, repoRoot)
			if waveStructureRes.IsFatal() {
				res.WaveStructure = WaveStructureCheckResult{Passed: true, Problems: nil}
			} else {
				problems := waveStructureRes.GetData()
				res.WaveStructure = WaveStructureCheckResult{
					Passed:   len(problems) == 0,
					Problems: problems,
				}
			}

			// Stale constraint lint (warning-only, never blocks)
			staleWarnings := protocol.DetectStaleConstraints(manifest)
			res.StaleConstraints = StaleConstraintsResult{
				Passed:   true,
				Warnings: staleWarnings,
			}

			if !res.E35Gaps.Passed || !res.TestCascade.Passed || !res.WaveStructure.Passed {
				res.FailedStep = "pre_wave_validate"
				return outputResult(cmd, res)
			}

			// Step 3: Brief accuracy validation
			briefData, briefErr := protocol.ValidateBriefs(ctx, manifestPath)
			if briefErr != nil {
				res.FailedStep = "validate_briefs"
				return outputResultErr(cmd, res, fmt.Errorf("brief validation failed: %w", briefErr))
			}
			res.BriefValidation = &briefData
			if !briefData.Valid {
				res.FailedStep = "validate_briefs"
				return outputResult(cmd, res)
			}

			// Step 4: Auto-set injection method
			if injectionMethod != "" {
				validMethods := map[string]protocol.InjectionMethod{
					"hook":            protocol.InjectionMethodHook,
					"manual-fallback": protocol.InjectionMethodManualFallback,
					"unknown":         protocol.InjectionMethodUnknown,
				}
				im, ok := validMethods[injectionMethod]
				if !ok {
					res.FailedStep = "injection_method"
					return outputResultErr(cmd, res, fmt.Errorf("invalid injection method %q", injectionMethod))
				}
				manifest.InjectionMethod = im
				if saveRes := protocol.Save(ctx, manifest, manifestPath); saveRes.IsFatal() {
					res.FailedStep = "injection_method"
					return outputResultErr(cmd, res, fmt.Errorf("failed to save injection method"))
				}
			}

			// All passed
			res.Passed = true
			return outputResult(cmd, res)
		},
	}

	cmd.Flags().StringVar(&injectionMethod, "injection-method", "",
		"Injection method to record (hook, manual-fallback, unknown). If empty, skips.")

	return cmd
}

func outputResult(cmd *cobra.Command, res FinalizeScoutResult) error {
	jsonOut, _ := json.MarshalIndent(res, "", "  ")
	fmt.Fprintln(cmd.OutOrStdout(), string(jsonOut))
	if !res.Passed {
		return fmt.Errorf("finalize-scout failed at step: %s", res.FailedStep)
	}
	return nil
}

func outputResultErr(cmd *cobra.Command, res FinalizeScoutResult, err error) error {
	jsonOut, _ := json.MarshalIndent(res, "", "  ")
	fmt.Fprintln(cmd.OutOrStdout(), string(jsonOut))
	return err
}
