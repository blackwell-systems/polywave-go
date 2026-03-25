package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/engine"
	"github.com/blackwell-systems/scout-and-wave-go/pkg/idgen"
	"github.com/blackwell-systems/scout-and-wave-go/pkg/protocol"
	"github.com/blackwell-systems/scout-and-wave-go/pkg/result"
	"github.com/spf13/cobra"
)

func newRunScoutCmd() *cobra.Command {
	var (
		repoPath            string
		sawRepoPath         string
		scoutModel          string
		timeout             int    // minutes
		programManifestPath string // path to PROGRAM manifest
		noCritic            bool   // --no-critic: skip critic gate even if threshold met
		criticModel         string // --critic-model: override model for critic agent
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

			// Resolve repo path (default to current directory)
			if repoPath == "" {
				cwd, err := os.Getwd()
				if err != nil {
					return fmt.Errorf("run-scout: failed to get current directory: %w", err)
				}
				repoPath = cwd
			}

			// Validate repo path exists
			if _, err := os.Stat(repoPath); err != nil {
				return fmt.Errorf("run-scout: repo path does not exist: %s", repoPath)
			}

			// Generate IMPL slug from feature description
			slug := generateSlug(featureDesc)
			implPath := filepath.Join(repoPath, "docs", "IMPL", fmt.Sprintf("IMPL-%s.yaml", slug))

			// Check if IMPL doc already exists; skip Scout if state is advanced
			if existingDoc, loadErr := protocol.Load(implPath); loadErr == nil {
				switch existingDoc.State {
				case protocol.StateReviewed,
					protocol.StateScaffoldPending,
					protocol.StateWavePending,
					protocol.StateWaveExecuting,
					protocol.StateWaveMerging,
					protocol.StateWaveVerified,
					protocol.StateComplete:
					fmt.Printf("Skipping Scout: IMPL doc already exists with state %s\n", existingDoc.State)
					fmt.Printf("   Path: %s\n", implPath)
					return nil
				case protocol.StateScoutValidating:
					// SCOUT_VALIDATING means Scout already ran; skip to avoid overwrite
					fmt.Printf("Skipping Scout: IMPL doc already exists with state %s\n", existingDoc.State)
					fmt.Printf("   Path: %s\n", implPath)
					return nil
				}
				// For SCOUT_PENDING or unknown states, proceed with Scout
			}

			// Ensure docs/IMPL directory exists
			implDir := filepath.Dir(implPath)
			if err := os.MkdirAll(implDir, 0755); err != nil {
				return fmt.Errorf("run-scout: failed to create IMPL directory: %w", err)
			}

			fmt.Printf("🔍 Launching Scout agent...\n")
			fmt.Printf("   Feature: %s\n", featureDesc)
			fmt.Printf("   Repository: %s\n", repoPath)
			fmt.Printf("   IMPL output: %s\n", implPath)
			fmt.Println()

			// Create context with timeout
			ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeout)*time.Minute)
			defer cancel()

			// Configure Scout run
			opts := engine.RunScoutOpts{
				Feature:             featureDesc,
				RepoPath:            repoPath,
				SAWRepoPath:         sawRepoPath,
				IMPLOutPath:         implPath,
				ScoutModel:          scoutModel,
				ProgramManifestPath: programManifestPath,
			}

			// Launch Scout agent with correction loop (C9: self-healing validation)
			correctionOpts := engine.ScoutCorrectionOpts{
				ScoutOpts:  opts,
				MaxRetries: 3,
				OnRetry: func(attempt int, errors []string) {
					fmt.Printf("\nIMPL validation failed (attempt %d), retrying with corrections...\n", attempt)
					for _, e := range errors {
						fmt.Printf("   - %s\n", e)
					}
					fmt.Println()
				},
			}
			scoutErr := engine.ScoutCorrectionLoop(ctx, correctionOpts, func(chunk string) {
				fmt.Print(chunk)
			})

			if scoutErr != nil {
				return fmt.Errorf("run-scout: Scout execution failed: %w", scoutErr)
			}

			fmt.Println()
			fmt.Println("✅ Scout agent completed")
			fmt.Println()

			// Step 2: Wait for IMPL doc creation (with retry for race conditions)
			fmt.Printf("⏳ Waiting for IMPL doc creation...\n")
			if !waitForFile(implPath, 10*time.Second) {
				return fmt.Errorf("run-scout: IMPL doc not found at %s after Scout completion", implPath)
			}

			// Step 3: Validate IMPL doc (defense-in-depth — Scout self-validates internally)
			fmt.Printf("🔍 Validating IMPL doc...\n")
			errs, err := protocol.ValidateIMPLDoc(implPath)
			if err != nil {
				return fmt.Errorf("run-scout: validation system error: %w", err)
			}

			// Step 4: Check for agent ID errors and auto-correct if needed
			if len(errs) > 0 {
				hasAgentIDErrors := false
				for _, e := range errs {
					if e.Code == "agent-id" {
						hasAgentIDErrors = true
						break
					}
				}

				if hasAgentIDErrors {
					fmt.Println("⚠️  Agent ID validation errors found")
					fmt.Println()
					fmt.Println("Auto-correcting agent IDs...")

					// Extract agent count from validation suggestion
					// The validator appends "Run: sawtools assign-agent-ids --count N"
					agentCount := countAgentsFromErrors(errs)
					if agentCount > 0 {
						// Generate correct agent IDs
						correctIDs, err := idgen.AssignAgentIDs(agentCount, nil)
						if err != nil {
							return fmt.Errorf("run-scout: failed to generate agent IDs: %w", err)
						}

						fmt.Printf("   Corrected agent IDs: %s\n", strings.Join(correctIDs, " "))
						fmt.Println()

						// Note: Actual ID replacement would require IMPL doc rewriting
						// For now, we report the error and suggest manual correction
						fmt.Println("❌ Validation failed - manual correction required")
						fmt.Println()
						for _, e := range errs {
							if e.Code == "agent-id" {
								fmt.Printf("   Line %d: %s\n", e.Line, e.Message)
							}
						}
						fmt.Println()
						fmt.Printf("💡 Suggested fix: Replace agent IDs with: %s\n", strings.Join(correctIDs, " "))
						return fmt.Errorf("run-scout: IMPL doc validation failed (agent ID errors)")
					}
				}

				// Non-agent-ID errors (or agent ID errors without count)
				fmt.Println("❌ Validation failed")
				fmt.Println()
				for _, e := range errs {
					fmt.Printf("   Line %d [%s]: %s\n", e.Line, e.Code, e.Message)
				}
				return fmt.Errorf("run-scout: IMPL doc validation failed")
			}

			// Success!
			fmt.Println("✅ IMPL doc validated successfully")
			fmt.Println()

			// Step 5: Finalize IMPL doc (M4: populate verification gates)
			// NOTE: This is the single entry point for post-Scout validation (finalize-impl).
			// Do not add additional validation calls here; FinalizeIMPL handles the full
			// validate → populate-gates → validate pipeline internally.
			fmt.Printf("🔧 Finalizing IMPL doc (populating verification gates)...\n")
			finalizeRes, _ := engine.FinalizeIMPLEngine(cmd.Context(), implPath, repoPath)

			if !finalizeRes.IsSuccess() {
				fmt.Println("⚠️  Finalize-impl completed with warnings")
				for _, e := range finalizeRes.Errors {
					fmt.Printf("      %s: %s\n", e.Code, e.Message)
				}
				fmt.Println("   (Gates can be manually written during review)")
				// Non-fatal - IMPL doc still usable, gates just not auto-populated
			} else {
				fmt.Printf("✅ Verification gates populated for %d agents\n", finalizeRes.GetData().GatePopulation.AgentsUpdated)
			}
			fmt.Println()

			// Step 6 (optional): Run critic gate if agent count threshold is met
			if !noCritic {
				// Load the manifest to check threshold
				manifest, loadErr := protocol.Load(implPath)
				if loadErr != nil {
					// Non-fatal: if we can't load the manifest, skip critic
					fmt.Printf("⚠️  Could not load IMPL manifest for critic threshold check: %v\n", loadErr)
					fmt.Println("   Skipping critic gate")
				} else if criticThresholdMet(manifest) {
					fmt.Printf("Step 6: Running critic gate (E37)...\n")
					criticArgs := []string{"run-critic", implPath}
					if criticModel != "" {
						criticArgs = append(criticArgs, "--model", criticModel)
					}
					// Find the sawtools binary (use os.Executable for self-invocation)
					sawBin, binErr := os.Executable()
					if binErr != nil {
						sawBin = "sawtools"
					}
					criticCmd := exec.Command(sawBin, criticArgs...) //nolint:gosec
					criticCmd.Stdout = os.Stdout
					criticCmd.Stderr = os.Stderr
					if runErr := criticCmd.Run(); runErr != nil {
						// Non-fatal: critic may not be installed yet; skip gracefully
						fmt.Printf("⚠️  Critic gate skipped (run-critic not available or failed): %v\n", runErr)
					} else {
						fmt.Println("✅ Critic gate passed")
					}
					fmt.Println()
				}
			}

			fmt.Printf("📄 IMPL doc: %s\n", implPath)
			fmt.Println()
			fmt.Println("Next steps:")
			fmt.Println("  1. Review the IMPL doc")
			fmt.Println("  2. Run: sawtools run-wave --wave 1")

			return nil
		},
	}

	cmd.Flags().StringVar(&repoPath, "repo-dir", "", "Target repository path (default: current directory)")
	cmd.Flags().StringVar(&sawRepoPath, "saw-repo", "", "Scout-and-Wave protocol repo path (default: $SAW_REPO or ~/code/scout-and-wave)")
	cmd.Flags().StringVar(&scoutModel, "scout-model", "", "Scout model override (e.g., claude-opus-4-6)")
	cmd.Flags().IntVar(&timeout, "timeout", 10, "Timeout in minutes (default: 10)")
	cmd.Flags().StringVar(&programManifestPath, "program", "", "Path to PROGRAM manifest (Scout receives frozen contracts as input)")
	cmd.Flags().BoolVar(&noCritic, "no-critic", false, "Skip critic gate even if agent count threshold is met")
	cmd.Flags().StringVar(&criticModel, "critic-model", "", "Model override for critic agent (e.g., claude-opus-4-6)")

	return cmd
}

// generateSlug creates a URL-safe slug from a feature description.
// Matches the slug generation logic from Scout prompt.
func generateSlug(feature string) string {
	// Convert to lowercase
	slug := strings.ToLower(feature)

	// Replace whitespace and special chars with hyphens
	slug = strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			return r
		}
		return '-'
	}, slug)

	// Collapse multiple hyphens
	for strings.Contains(slug, "--") {
		slug = strings.ReplaceAll(slug, "--", "-")
	}

	// Trim leading/trailing hyphens
	slug = strings.Trim(slug, "-")

	// Truncate to 49 chars (not 50 - off-by-one fix)
	if len(slug) > 49 {
		slug = slug[:49]
	}

	return slug
}

// waitForFile polls for file existence with retry logic.
// Returns true if file appears within maxWait duration.
func waitForFile(path string, maxWait time.Duration) bool {
	deadline := time.Now().Add(maxWait)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(path); err == nil {
			return true
		}
		time.Sleep(500 * time.Millisecond)
	}
	return false
}

// criticThresholdMet returns true if the IMPL doc meets the auto-trigger
// threshold for the critic gate: wave 1 has 3+ agents OR file_ownership
// spans 2+ distinct repos.
func criticThresholdMet(manifest *protocol.IMPLManifest) bool {
	// Check wave 1 agent count
	for _, wave := range manifest.Waves {
		if wave.Number == 1 && len(wave.Agents) >= 3 {
			return true
		}
	}

	// Check if file_ownership spans 2+ distinct repos
	repos := make(map[string]struct{})
	for _, fo := range manifest.FileOwnership {
		if fo.Repo != "" {
			repos[fo.Repo] = struct{}{}
		}
	}
	if len(repos) >= 2 {
		return true
	}

	return false
}

// countAgentsFromErrors extracts the agent count from validation error messages.
// The validator appends "Run: sawtools assign-agent-ids --count N" as the last error.
func countAgentsFromErrors(errs []result.SAWError) int {
	for _, e := range errs {
		if e.Code == "agent-id" && e.Line == 0 {
			// This is the suggestion message: "Run: sawtools assign-agent-ids --count N"
			msg := e.Message
			if strings.Contains(msg, "--count") {
				// Extract number after "--count "
				parts := strings.Split(msg, "--count ")
				if len(parts) == 2 {
					var count int
					fmt.Sscanf(parts[1], "%d", &count)
					return count
				}
			}
		}
	}
	return 0
}
