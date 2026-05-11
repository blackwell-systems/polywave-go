package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/blackwell-systems/polywave-go/pkg/engine"
	"github.com/blackwell-systems/polywave-go/pkg/protocol"
	"github.com/spf13/cobra"
)

func newAutoCmd() *cobra.Command {
	var (
		sawRepoPath string
		scoutModel  string
		waveModel   string
		timeout     int
		skipConfirm bool
	)

	cmd := &cobra.Command{
		Use:   "auto <feature-description>",
		Short: "Scout + confirm + wave: the full SAW flow in one command",
		Long: `Collapses the three-step scout -> review -> wave flow into one command.

The human confirmation checkpoint is preserved: after Scout completes, you will
be shown the IMPL summary and asked "Proceed with wave execution? [y/N]"
before any agent writes code.

Behavior by verdict:
  NOT_SUITABLE          -- shows reason and suggested alternative, exits
  SUITABLE              -- shows IMPL summary, asks to confirm, then waves
  SUITABLE_WITH_CAVEATS -- shows IMPL summary + caveats, asks to confirm, then waves

Use --skip-confirm to omit the confirmation prompt (expert/CI use only).

Examples:
  sawtools auto "Add audit logging to auth module"
  sawtools auto "Add caching layer" --repo-dir /path/to/project
  sawtools auto "Refactor storage layer" --skip-confirm`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			featureDesc := args[0]

			// Resolve repo path (repoDir is the package-level persistent flag).
			if repoDir == "" || repoDir == "." {
				cwd, err := os.Getwd()
				if err != nil {
					return fmt.Errorf("auto: failed to get current directory: %w", err)
				}
				repoDir = cwd
			}

			fmt.Printf("Running Scout...\n")
			fmt.Printf("  Feature: %s\n", featureDesc)
			fmt.Printf("  Repository: %s\n\n", repoDir)

			// Phase 1: Scout.
			scoutOpts := engine.RunScoutFullOpts{
				Feature:     featureDesc,
				RepoPath:    repoDir,
				PolywaveRepoPath: sawRepoPath,
				ScoutModel:  scoutModel,
				Timeout:     timeout,
				Logger:      newSawLogger(),
			}
			scoutResultR := engine.RunScoutFull(cmd.Context(), scoutOpts, func(chunk string) {
				fmt.Print(chunk)
			})
			if scoutResultR.IsFatal() {
				errMsg := "scout failed"
				if len(scoutResultR.Errors) > 0 {
					errMsg = scoutResultR.Errors[0].Message
				}
				return fmt.Errorf("auto: scout failed: %s", errMsg)
			}
			scoutRes := scoutResultR.GetData()

			// Phase 2: Read IMPL doc and check verdict.
			implPath := scoutRes.IMPLPath
			manifest, err := protocol.Load(context.Background(), implPath)
			if err != nil {
				return fmt.Errorf("auto: failed to read IMPL doc %s: %w", implPath, err)
			}

			verdict := strings.TrimSpace(manifest.Verdict)
			fmt.Println()

			if strings.EqualFold(verdict, "NOT_SUITABLE") {
				fmt.Println("Verdict: NOT_SUITABLE")
				fmt.Println()
				fmt.Println("Reason:")
				fmt.Println(manifest.SuitabilityAssessment)
				fmt.Println()
				fmt.Println("Suggested alternative: implement sequentially or run /saw interview first.")
				return nil
			}

			// SUITABLE or SUITABLE_WITH_CAVEATS -- print summary.
			fmt.Printf("Verdict: %s\n", verdict)
			fmt.Println()
			waveCount := len(manifest.Waves)
			agentCount := 0
			for _, w := range manifest.Waves {
				agentCount += len(w.Agents)
			}
			fmt.Printf("IMPL doc: %s\n", implPath)
			fmt.Printf("Waves: %d | Agents: %d | Files: %d\n",
				waveCount, agentCount, len(manifest.FileOwnership))
			if len(manifest.InterfaceContracts) > 0 {
				fmt.Printf("Interface contracts: %d\n", len(manifest.InterfaceContracts))
			}
			if strings.EqualFold(verdict, "SUITABLE_WITH_CAVEATS") {
				fmt.Println()
				fmt.Println("Caveats:")
				fmt.Println(manifest.SuitabilityAssessment)
			}
			fmt.Println()

			// Phase 3: Human confirmation.
			if !skipConfirm {
				fmt.Print("Proceed with wave execution? [y/N] ")
				scanner := bufio.NewScanner(os.Stdin)
				scanner.Scan()
				response := strings.TrimSpace(scanner.Text())
				if !strings.EqualFold(response, "y") && !strings.EqualFold(response, "yes") {
					fmt.Println("Auto flow cancelled. Review the IMPL doc and run:")
					fmt.Printf("  sawtools run-wave %s --wave 1\n", implPath)
					return nil
				}
				fmt.Println()
			}

			// Phase 4: Wave execution (all waves in sequence).
			fmt.Println("Executing waves...")
			fmt.Println()
			for waveNum := 1; waveNum <= waveCount; waveNum++ {
				fmt.Printf("Wave %d of %d...\n", waveNum, waveCount)
				waveRes := engine.RunWaveFull(cmd.Context(), engine.RunWaveFullOpts{
					ManifestPath: implPath,
					RepoPath:     repoDir,
					WaveNum:      waveNum,
				})
				if waveRes.IsFatal() {
					msg := "wave execution failed"
					if len(waveRes.Errors) > 0 {
						msg = waveRes.Errors[0].Message
					}
					return fmt.Errorf("auto: wave %d failed: %s", waveNum, msg)
				}
				if !waveRes.GetData().Success {
					return fmt.Errorf("auto: wave %d failed: wave execution failed", waveNum)
				}
				fmt.Printf("Wave %d complete.\n\n", waveNum)
			}

			fmt.Println("All waves complete.")
			fmt.Printf("IMPL doc: %s\n", implPath)
			return nil
		},
	}

	cmd.Flags().StringVar(&sawRepoPath, "protocol-repo", "", "Polywave protocol repo path")
	cmd.Flags().StringVar(&scoutModel, "scout-model", "", "Scout model override (e.g., claude-opus-4-6)")
	cmd.Flags().StringVar(&waveModel, "wave-model", "", "Wave model override (reserved for future use)")
	cmd.Flags().IntVar(&timeout, "timeout", 10, "Scout timeout in minutes (default: 10)")
	cmd.Flags().BoolVar(&skipConfirm, "skip-confirm", false, "Skip confirmation prompt (expert/CI use only)")

	_ = waveModel // reserved for future use

	return cmd
}
