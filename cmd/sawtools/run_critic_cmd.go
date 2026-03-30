package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/engine"
	"github.com/blackwell-systems/scout-and-wave-go/pkg/protocol"
	"github.com/spf13/cobra"
)

// newRunCriticCmd creates the run-critic subcommand.
// Usage: sawtools run-critic <impl-path> [--no-review] [--skip] [--model <model>]
//
// Launches a critic agent that reviews agent briefs in the IMPL doc against
// the actual codebase. Writes CriticData to impl doc critic_report field.
// Exits 0 if verdict is PASS, exits 1 if verdict is ISSUES.
func newRunCriticCmd() *cobra.Command {
	var (
		noReview    bool
		skip        bool
		criticModel string
		timeout     int // minutes
	)

	cmd := &cobra.Command{
		Use:   "run-critic <impl-path>",
		Short: "E37: Run critic agent to review agent briefs against the codebase",
		Long: `Launch a critic agent (E37) that reviews every agent brief in the IMPL doc
against the actual codebase before wave execution begins.

The critic verifies:
  - file_existence: action=modify files must exist; action=new files must not
  - symbol_accuracy: named symbols referenced in briefs exist as described
  - pattern_accuracy: implementation patterns match actual source patterns
  - interface_consistency: contracts are syntactically valid and type-consistent
  - import_chains: all required packages are importable from the target module
  - side_effect_completeness: registration files are included in file_ownership

Writes a structured CriticData to the IMPL doc critic_report field.
Exits 0 if overall verdict is PASS. Exits 1 if verdict is ISSUES.

Examples:
  sawtools run-critic /path/to/IMPL-feature.yaml
  sawtools run-critic /path/to/IMPL-feature.yaml --model claude-opus-4-6
  sawtools run-critic /path/to/IMPL-feature.yaml --skip`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			implPath := args[0]

			// Validate impl path is absolute and exists.
			if !filepath.IsAbs(implPath) {
				return fmt.Errorf("run-critic: impl-path must be absolute (got %q)", implPath)
			}
			if _, err := os.Stat(implPath); err != nil {
				return fmt.Errorf("run-critic: impl path does not exist: %s", implPath)
			}

			// --skip is an alias for --no-review.
			if skip {
				noReview = true
			}

			// Handle --no-review / --skip: write PASS result immediately without launching an agent.
			if noReview {
				fmt.Println("Skipping critic review (operator flag)")
				skipResult := protocol.CriticData{
					Verdict:      "PASS",
					AgentReviews: map[string]protocol.AgentCriticReview{},
					Summary:      "Skipped by operator",
					ReviewedAt:   time.Now().UTC().Format(time.RFC3339),
					IssueCount:   0,
				}
				if err := protocol.WriteCriticReview(implPath, skipResult); err != nil {
					return fmt.Errorf("run-critic: failed to write skip result: %w", err)
				}
				fmt.Println("Critic Review: PASS (Skipped by operator)")
				return nil
			}

			fmt.Println("Launching critic agent (E37)...")
			fmt.Printf("  IMPL doc: %s\n", implPath)
			fmt.Println()

			result, err := engine.RunCritic(cmd.Context(), engine.RunCriticOpts{
				IMPLPath:    implPath,
				CriticModel: criticModel,
				Timeout:     timeout,
			}, func(chunk string) { fmt.Print(chunk) })
			if err != nil {
				return err
			}

			fmt.Println()
			fmt.Println("Critic agent completed")
			fmt.Println()

			// Print human-readable summary.
			fmt.Printf("Critic Review: %s\n", result.Verdict)
			fmt.Printf("  Summary: %s\n", result.Summary)
			fmt.Printf("  Issues found: %d\n", result.IssueCount)

			// Reload manifest to print per-agent verdicts (AgentReviews is not in RunCriticResult).
			updatedManifest, loadErr := protocol.Load(implPath)
			if loadErr == nil {
				review := protocol.GetCriticReview(updatedManifest)
				if review != nil && len(review.AgentReviews) > 0 {
					fmt.Println("  Per-agent verdicts:")
					for agentID, agentReview := range review.AgentReviews {
						fmt.Printf("    Agent %s: %s", agentID, agentReview.Verdict)
						if len(agentReview.Issues) > 0 {
							fmt.Printf(" (%d issue(s))", len(agentReview.Issues))
						}
						fmt.Println()
						for _, issue := range agentReview.Issues {
							fmt.Printf("      [%s/%s] %s\n", issue.Severity, issue.Check, issue.Description)
							if issue.File != "" {
								fmt.Printf("        File: %s\n", issue.File)
							}
							if issue.Symbol != "" {
								fmt.Printf("        Symbol: %s\n", issue.Symbol)
							}
						}
					}
				}
			}

			// Exit 1 if ISSUES verdict.
			if result.Verdict == "ISSUES" {
				return fmt.Errorf("run-critic: critic found issues — review and correct IMPL doc before proceeding")
			}

			return nil
		},
	}

	cmd.Flags().BoolVar(&noReview, "no-review", false, "Skip critic review, write PASS result with 'Skipped by operator' summary")
	cmd.Flags().BoolVar(&skip, "skip", false, "Alias for --no-review")
	cmd.Flags().StringVar(&criticModel, "model", "", "Model override for critic agent (e.g. claude-opus-4-6)")
	cmd.Flags().IntVar(&timeout, "timeout", 20, "Timeout in minutes (default: 20)")

	return cmd
}

// newSetCriticReviewCmd creates the set-critic-review subcommand.
// Usage: sawtools set-critic-review <impl-path> --verdict <PASS|ISSUES>
//
//	--summary <string> --issue-count <N> --agent-reviews <JSON>
//
// Parses the JSON agent reviews, constructs a CriticData, and calls
// protocol.WriteCriticReview. Used by critic agents to write their output.
func newSetCriticReviewCmd() *cobra.Command {
	var (
		verdict      string
		summary      string
		issueCount   int
		agentReviews string
	)

	cmd := &cobra.Command{
		Use:   "set-critic-review <impl-path>",
		Short: "Write critic review result to IMPL doc (used by critic agents)",
		Long: `Parse the JSON agent reviews, construct a CriticData, and write it to
the IMPL doc's critic_report field. Called by critic agents after completing
their review. Not intended for direct human use.

The --agent-reviews flag accepts a JSON array of AgentCriticReview objects:
  [
    {
      "agent_id": "A",
      "verdict": "PASS",
      "issues": []
    },
    {
      "agent_id": "B",
      "verdict": "ISSUES",
      "issues": [
        {
          "check": "symbol_accuracy",
          "severity": "error",
          "description": "Function WriteCriticReview does not exist",
          "file": "pkg/protocol/critic.go",
          "symbol": "WriteCriticReview"
        }
      ]
    }
  ]`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			implPath := args[0]

			// Validate verdict.
			if verdict != "PASS" && verdict != "ISSUES" {
				return fmt.Errorf("set-critic-review: --verdict must be PASS or ISSUES (got %q)", verdict)
			}

			// Validate impl path exists.
			if _, err := os.Stat(implPath); err != nil {
				return fmt.Errorf("set-critic-review: impl path does not exist: %s", implPath)
			}

			// Parse agent reviews JSON.
			var reviews []protocol.AgentCriticReview
			if agentReviews != "" {
				if err := json.Unmarshal([]byte(agentReviews), &reviews); err != nil {
					return fmt.Errorf("set-critic-review: invalid --agent-reviews JSON: %w", err)
				}
			}

			// Build AgentReviews map indexed by agent ID.
			reviewMap := make(map[string]protocol.AgentCriticReview, len(reviews))
			for _, r := range reviews {
				reviewMap[r.AgentID] = r
			}

			// Build CriticData.
			result := protocol.CriticData{
				Verdict:      verdict,
				AgentReviews: reviewMap,
				Summary:      summary,
				ReviewedAt:   time.Now().UTC().Format(time.RFC3339),
				IssueCount:   issueCount,
			}

			// Write to IMPL doc.
			if err := protocol.WriteCriticReview(implPath, result); err != nil {
				return fmt.Errorf("set-critic-review: %w", err)
			}

			// Print confirmation.
			out, _ := json.Marshal(map[string]interface{}{
				"verdict":     result.Verdict,
				"issue_count": result.IssueCount,
				"reviewed_at": result.ReviewedAt,
				"saved":       true,
			})
			fmt.Fprintln(cmd.OutOrStdout(), string(out))
			return nil
		},
	}

	cmd.Flags().StringVar(&verdict, "verdict", "", "Overall verdict: PASS or ISSUES (required)")
	cmd.Flags().StringVar(&summary, "summary", "", "Human-readable summary of findings (required)")
	cmd.Flags().IntVar(&issueCount, "issue-count", 0, "Total number of issues found across all agents")
	cmd.Flags().StringVar(&agentReviews, "agent-reviews", "", "JSON array of AgentCriticReview objects")

	_ = cmd.MarkFlagRequired("verdict")
	_ = cmd.MarkFlagRequired("summary")

	return cmd
}

// collectRepoPaths returns all unique repository root paths referenced by the
// IMPL manifest (Repository + Repositories fields).
func collectRepoPaths(manifest *protocol.IMPLManifest) []string {
	seen := make(map[string]bool)
	var paths []string

	if manifest.Repository != "" && !seen[manifest.Repository] {
		seen[manifest.Repository] = true
		paths = append(paths, manifest.Repository)
	}
	for _, r := range manifest.Repositories {
		if r != "" && !seen[r] {
			seen[r] = true
			paths = append(paths, r)
		}
	}
	return paths
}

// inferRepoRoot attempts to derive the repository root from an IMPL doc path
// by stripping the trailing /docs/IMPL/IMPL-*.yaml suffix.
func inferRepoRoot(implPath string) string {
	dir := filepath.Dir(implPath)    // .../docs/IMPL
	implDir := filepath.Dir(dir)     // .../docs
	if filepath.Base(implDir) == "docs" {
		return filepath.Dir(implDir) // strip /docs
	}
	return ""
}
