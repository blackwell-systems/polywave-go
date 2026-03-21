package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/collision"
	"github.com/blackwell-systems/scout-and-wave-go/pkg/deps"
	"github.com/blackwell-systems/scout-and-wave-go/pkg/gatecache"
	"github.com/blackwell-systems/scout-and-wave-go/pkg/journal"
	"github.com/blackwell-systems/scout-and-wave-go/pkg/protocol"
	"github.com/blackwell-systems/scout-and-wave-go/pkg/resume"
	"github.com/spf13/cobra"
)

// PrepareWaveResult combines worktree creation and per-agent preparation results
type PrepareWaveResult struct {
	Wave        int                     `json:"wave"`
	Worktrees   []protocol.WorktreeInfo `json:"worktrees"`
	AgentBriefs []AgentBriefInfo        `json:"agent_briefs"`
	Repos       []string                `json:"repos,omitempty"` // distinct repo names used this wave
}

// AgentBriefInfo contains metadata about a prepared agent brief
type AgentBriefInfo struct {
	Agent       string `json:"agent"`
	BriefPath   string `json:"brief_path"`
	BriefLength int    `json:"brief_length"`
	JournalDir  string `json:"journal_dir"`
	FilesOwned  int    `json:"files_owned"`
	Repo        string `json:"repo,omitempty"` // repo name from file_ownership; empty = primary repo
}

// loadSAWConfigRepos reads saw.config.json at configPath and returns
// the repos array as protocol.RepoEntry values.
// Returns nil slice if file absent or parse fails.
func loadSAWConfigRepos(configPath string) []protocol.RepoEntry {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil
	}
	var cfg struct {
		Repos []protocol.RepoEntry `json:"repos"`
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil
	}
	return cfg.Repos
}

// resolveAgentRepoRoot returns the absolute repo root for agentID.
// It looks up the agent's repo name from fileOwnership, then resolves
// the path from the repos registry. Falls back to projectRoot if the
// agent has no repo field or the repo is not found in the registry.
func resolveAgentRepoRoot(
	fileOwnership []protocol.FileOwnership,
	agentID string,
	projectRoot string,
	repos []protocol.RepoEntry,
) string {
	// Find the repo name for this agent from file ownership
	repoName := ""
	for _, fo := range fileOwnership {
		if fo.Agent == agentID && fo.Repo != "" {
			repoName = fo.Repo
			break
		}
	}
	if repoName == "" {
		return projectRoot
	}
	// Look up the repo path in the registry
	for _, r := range repos {
		if r.Name == repoName {
			return r.Path
		}
	}
	return projectRoot
}

func newPrepareWaveCmd() *cobra.Command {
	var waveNum int
	var noCache bool

	cmd := &cobra.Command{
		Use:   "prepare-wave <manifest-path>",
		Short: "Prepare all agents in a wave (check deps + create worktrees + extract briefs + init journals)",
		Long: `Prepares all agents in a wave for parallel execution by:
0. Checking for dependency conflicts (missing packages, version conflicts)
1. Creating git worktrees for all agents in the wave
2. Extracting each agent's brief to their worktree root (.saw-agent-brief.md)
3. Initializing journal observers for all agents

This combines check-deps + create-worktrees + prepare-agent into a single atomic operation,
reducing wave setup from 6+ commands to 1 command.

If dependency conflicts are detected (missing packages or version conflicts), the command
exits with code 1 and prints a JSON report. Resolve conflicts in main branch and re-run.

NOTE: For solo agents (1 agent in wave), use prepare-agent --no-worktree instead.
prepare-wave always creates worktrees, which is unnecessary overhead for single-agent
waves that execute on the main branch.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if waveNum == 0 {
				return fmt.Errorf("--wave is required")
			}

			manifestPath := args[0]

			// Determine project root from manifest path or --repo-dir flag
			projectRoot := filepath.Dir(filepath.Dir(filepath.Dir(manifestPath)))
			if repoDir != "" {
				projectRoot = repoDir
			}

			// Step 0a: Resume detection — warn if orphaned worktrees exist (legacy)
			if sessions, err := resume.Detect(projectRoot); err == nil {
				for _, s := range sessions {
					if len(s.OrphanedWorktrees) > 0 {
						fmt.Fprintf(os.Stderr, "warning: %d orphaned worktree(s) detected for %s (wave %d). Run 'sawtools cleanup' first or use --force.\n",
							len(s.OrphanedWorktrees), s.IMPLSlug, s.CurrentWave)
					}
				}
			}

			// Step 0b: Check dependencies before creating worktrees
			report, err := checkDependencies(manifestPath, waveNum, projectRoot)
			if err != nil {
				return fmt.Errorf("failed to check dependencies: %w", err)
			}
			if report != nil {
				// Conflicts detected - print report and exit
				reportJSON, _ := json.MarshalIndent(report, "", "  ")
				fmt.Fprintln(cmd.OutOrStdout(), string(reportJSON))
				return fmt.Errorf("dependency conflicts detected - resolve before creating worktrees")
			}

			// Load repos config for multi-repo agent routing
			repos := loadSAWConfigRepos(filepath.Join(projectRoot, "saw.config.json"))

			// Load manifest early so we can run pre-flight checks before creating worktrees
			doc, err := protocol.Load(manifestPath)
			if err != nil {
				return fmt.Errorf("failed to load IMPL doc: %w", err)
			}

			// Step 0a2: Clean stale worktrees from the SAME slug (previous failed run)
			stale, staleErr := protocol.DetectStaleWorktrees(projectRoot)
			if staleErr == nil {
				var sameSlug []protocol.StaleWorktree
				for _, s := range stale {
					if s.Slug == doc.FeatureSlug {
						sameSlug = append(sameSlug, s)
					}
				}
				if len(sameSlug) > 0 {
					result, cleanErr := protocol.CleanStaleWorktrees(sameSlug, true)
					if cleanErr == nil {
						fmt.Fprintf(os.Stderr, "prepare-wave: cleaned %d stale worktree(s) from previous runs\n", len(result.Cleaned))
					}
				}
			}

			// Step 0b1: Validate repo match — must run BEFORE baseline quality gates
			// because gates run in the target repo and we need to verify it's correct first.
			if err := protocol.ValidateRepoMatch(doc, projectRoot, repos); err != nil {
				return fmt.Errorf("prepare-wave: repo mismatch: %w", err)
			}

			// Step 0b1.5: Verify dependency outputs available (H2)
			// Skip wave 1 — no prior wave outputs to verify.
			if waveNum > 1 {
				depResult, err := protocol.VerifyDependenciesAvailable(doc, waveNum)
				if err != nil {
					return fmt.Errorf("dependency verification failed: %w", err)
				}
				if !depResult.Valid {
					for _, agent := range depResult.Agents {
						if !agent.Available {
							fmt.Fprintf(os.Stderr, "agent %s missing dependencies: %v\n",
								agent.Agent, agent.Missing)
						}
					}
					return fmt.Errorf("dependency outputs not available for wave %d", waveNum)
				}
				fmt.Fprintf(os.Stderr, "prepare-wave: dependency outputs verified ✓\n")
			}

			// Step 0b1.6: Validate prior wave ownership coverage (H1)
			// Warning only, not fatal — agents may have legitimate reasons.
			if waveNum > 1 {
				coverageResult, err := protocol.ValidateFileOwnershipCoverage(doc, waveNum-1)
				if err != nil {
					fmt.Fprintf(os.Stderr, "prepare-wave: ownership coverage check: %v\n", err)
				} else if !coverageResult.Valid {
					for _, v := range coverageResult.Violations {
						fmt.Fprintf(os.Stderr, "warning: agent %s modified unowned files: %v\n",
							v.Agent, v.UnownedFiles)
					}
				}
			}

			// Step 0b2: Validate file existence across repos
			fileWarnings := protocol.ValidateFileExistenceMultiRepo(doc, projectRoot, repos)
			for _, w := range fileWarnings {
				if w.Code == "E16_REPO_MISMATCH_SUSPECTED" {
					return fmt.Errorf("prepare-wave: %s", w.Message)
				}
				fmt.Fprintf(os.Stderr, "prepare-wave: warning: %s\n", w.Message)
			}

			// Step 0b2: Run baseline quality gates (E21A)
			// This prevents wasted agent work when the codebase is already broken.
			// Quality gates run BEFORE worktree creation so failures surface immediately.
			fmt.Fprintf(os.Stderr, "prepare-wave: running baseline quality gates (wave %d)...\n", waveNum)

			// Build gate cache (unless --no-cache)
			var cache *gatecache.Cache
			if !noCache {
				stateDir := filepath.Join(projectRoot, ".saw-state")
				cache = gatecache.New(stateDir, gatecache.DefaultTTL)
			}

			baselineResult, err := protocol.RunBaselineGates(doc, waveNum, projectRoot, cache)
			if err != nil {
				return fmt.Errorf("failed to run baseline quality gates: %w", err)
			}

			// Print human-readable output to stderr
			fmt.Fprint(os.Stderr, FormatBaselineOutput(baselineResult))

			if !baselineResult.Passed {
				// Output structured JSON to stdout for programmatic consumers
				out, _ := json.MarshalIndent(baselineResult, "", "  ")
				fmt.Fprintln(cmd.OutOrStdout(), string(out))
				return fmt.Errorf("baseline verification failed: %s", baselineResult.Reason)
			}

			// Step 0c: Validate wiring ownership (E35 Layer 3A)
			// For each wiring declaration in the current wave, the must_be_called_from
			// file must be in the owning agent's file_ownership. Fail fast before
			// creating worktrees so the human sees the error immediately.
			if violations := protocol.CheckWiringOwnership(doc, waveNum); len(violations) > 0 {
				for _, v := range violations {
					fmt.Fprintf(os.Stderr, "prepare-wave: wiring ownership violation: %s\n", v)
				}
				return fmt.Errorf("prepare-wave: %d wiring ownership violation(s) — fix IMPL doc file_ownership before creating worktrees", len(violations))
			}

			// Step 0d: Validate scaffolds are committed (I2 — contracts precede implementation)
			if len(doc.Scaffolds) > 0 && !protocol.AllScaffoldsCommitted(doc) {
				statuses := protocol.ValidateScaffolds(doc)
				for _, s := range statuses {
					if s.Status != "committed" {
						fmt.Fprintf(os.Stderr, "prepare-wave: scaffold %s status=%s (expected committed)\n", s.FilePath, s.Status)
					}
				}
				return fmt.Errorf("prepare-wave: scaffolds not committed — run Scaffold Agent before creating worktrees")
			}

			// Step 0e: Freeze check — interface contracts immutable at worktree creation (I2)
			if doc.WorktreesCreatedAt != nil {
				violations, freezeErr := protocol.CheckFreeze(doc)
				if freezeErr != nil {
					fmt.Fprintf(os.Stderr, "prepare-wave: freeze check error: %v\n", freezeErr)
				} else if len(violations) > 0 {
					for _, v := range violations {
						fmt.Fprintf(os.Stderr, "prepare-wave: freeze violation: %s — %s\n", v.Section, v.Message)
					}
					return fmt.Errorf("prepare-wave: %d freeze violation(s) — interface contracts were modified after worktree creation", len(violations))
				}
			}

			// Step 0f: E3 — Pre-launch ownership verification (I1 disjoint check)
			// protocol.Validate() calls validateI1DisjointOwnership() internally.
			// Filter its output for I1_VIOLATION error codes.
			if allErrs := protocol.Validate(doc); len(allErrs) > 0 {
				var i1Errs []protocol.ValidationError
				for _, e := range allErrs {
					if e.Code == "I1_VIOLATION" {
						i1Errs = append(i1Errs, e)
					}
				}
				if len(i1Errs) > 0 {
					for _, e := range i1Errs {
						fmt.Fprintf(os.Stderr, "prepare-wave: I1 ownership violation: %s\n", e.Message)
					}
					return fmt.Errorf("prepare-wave: %d I1 disjoint ownership violation(s)", len(i1Errs))
				}
			}

			// Step 0g: E41 — Pre-launch type collision check
			// collision.DetectCollisions takes a manifest path string, wave number, and repo path.
			collisionReport, err := collision.DetectCollisions(manifestPath, waveNum, projectRoot)
			if err != nil {
				fmt.Fprintf(os.Stderr, "prepare-wave: collision check: %v\n", err)
			} else if !collisionReport.Valid {
				out, _ := json.MarshalIndent(collisionReport, "", "  ")
				fmt.Fprintln(cmd.OutOrStdout(), string(out))
				return fmt.Errorf("prepare-wave: type collisions detected — resolve before creating worktrees")
			}

			// Step 1: Create worktrees (records base commit, must happen first)
			worktreeResult, err := protocol.CreateWorktrees(manifestPath, waveNum, projectRoot)
			if err != nil {
				return fmt.Errorf("failed to create worktrees: %w", err)
			}

			// Step 1.5: Verify pre-commit hooks (H10 - Layer 0 isolation enforcement)
			for _, wtInfo := range worktreeResult.Worktrees {
				hookResult, err := verifyHookInstalled(wtInfo.Path, waveNum)
				if err != nil {
					return fmt.Errorf("hook verification failed for agent %s: %w", wtInfo.Agent, err)
				}
				if !hookResult.Valid {
					return fmt.Errorf("hook verification failed for agent %s: %s", wtInfo.Agent, hookResult.Reason)
				}
			}

			// Step 3: Prepare each agent (extract brief + init journal)
			agentBriefs := make([]AgentBriefInfo, 0, len(worktreeResult.Worktrees))
			repoSet := make(map[string]struct{})

			for _, wtInfo := range worktreeResult.Worktrees {
				agentID := wtInfo.Agent

				// Find the agent's task and files
				var agentTask string
				var agentFiles []string
				for _, wave := range doc.Waves {
					if wave.Number != waveNum {
						continue
					}
					for _, agent := range wave.Agents {
						if agent.ID == agentID {
							agentTask = agent.Task
							agentFiles = agent.Files
							break
						}
					}
				}

				if agentTask == "" {
					return fmt.Errorf("agent %s not found in wave %d", agentID, waveNum)
				}

				// Extract interface contracts
				contractsSection := ""
				if len(doc.InterfaceContracts) > 0 {
					contractsSection = "\n\n## Interface Contracts\n\n"
					for _, contract := range doc.InterfaceContracts {
						contractsSection += fmt.Sprintf("### %s\n\n%s\n\n```\n%s\n```\n\n",
							contract.Name, contract.Description, contract.Definition)
					}
				}

				// Extract quality gates
				gatesSection := ""
				if doc.QualityGates != nil && doc.QualityGates.Level != "" {
					gatesSection = "\n\n## Quality Gates\n\n"
					gatesSection += fmt.Sprintf("Level: %s\n\n", doc.QualityGates.Level)
					for _, gate := range doc.QualityGates.Gates {
						gatesSection += fmt.Sprintf("- **%s**: `%s` (required: %t)\n",
							gate.Type, gate.Command, gate.Required)
						if gate.Description != "" {
							gatesSection += fmt.Sprintf("  %s\n", gate.Description)
						}
					}
				}

				// Wiring obligations for this agent (E35 Layer 3C)
				wiringSection := protocol.InjectWiringInstructions(doc, agentID, waveNum)

				// Build the agent brief
				brief := fmt.Sprintf(`# Agent %s Brief - Wave %d

**IMPL Doc:** %s

## Files Owned

%s

## Task

%s
%s%s%s
`,
					agentID,
					waveNum,
					manifestPath,
					formatFileList(agentFiles),
					agentTask,
					contractsSection,
					wiringSection,
					gatesSection,
				)

				// Write brief to worktree root
				briefPath := filepath.Join(wtInfo.Path, ".saw-agent-brief.md")
				if err := os.WriteFile(briefPath, []byte(brief), 0644); err != nil {
					return fmt.Errorf("failed to write brief for agent %s: %w", agentID, err)
				}

				// Resolve the repo root for this agent (supports multi-repo waves)
				agentRoot := resolveAgentRepoRoot(doc.FileOwnership, agentID, projectRoot, repos)

				// Determine agent's repo name for result metadata
				agentRepoName := ""
				for _, fo := range doc.FileOwnership {
					if fo.Agent == agentID && fo.Repo != "" {
						agentRepoName = fo.Repo
						break
					}
				}
				if agentRepoName != "" {
					repoSet[agentRepoName] = struct{}{}
				}

				// Initialize journal observer
				fullAgentID := fmt.Sprintf("wave%d-agent-%s", waveNum, agentID)
				observer, err := journal.NewObserver(agentRoot, fullAgentID)
				if err != nil {
					return fmt.Errorf("failed to create journal observer for agent %s: %w", agentID, err)
				}

				// Initialize cursor if it doesn't exist
				if _, err := os.Stat(observer.CursorPath); os.IsNotExist(err) {
					emptyCursor := journal.SessionCursor{
						SessionFile: "",
						Offset:      0,
					}
					cursorData, _ := json.MarshalIndent(emptyCursor, "", "  ")
					if err := os.WriteFile(observer.CursorPath, cursorData, 0644); err != nil {
						return fmt.Errorf("failed to write cursor file for agent %s: %w", agentID, err)
					}
				}

				// Record agent brief info
				agentBriefs = append(agentBriefs, AgentBriefInfo{
					Agent:       agentID,
					BriefPath:   briefPath,
					BriefLength: len(brief),
					JournalDir:  observer.JournalDir,
					FilesOwned:  len(agentFiles),
					Repo:        agentRepoName,
				})
			}

			// Collect distinct repo names
			var repoList []string
			for name := range repoSet {
				repoList = append(repoList, name)
			}

			// Output result
			result := PrepareWaveResult{
				Wave:        waveNum,
				Worktrees:   worktreeResult.Worktrees,
				AgentBriefs: agentBriefs,
				Repos:       repoList,
			}

			out, _ := json.MarshalIndent(result, "", "  ")
			fmt.Println(string(out))

			return nil
		},
	}

	cmd.Flags().IntVar(&waveNum, "wave", 0, "Wave number (required)")
	_ = cmd.MarkFlagRequired("wave")
	cmd.Flags().BoolVar(&noCache, "no-cache", false, "Disable baseline gate result caching")

	return cmd
}

// checkDependencies wraps deps.CheckDeps to return nil report if no conflicts detected
func checkDependencies(implPath string, wave int, repoDir string) (*deps.ConflictReport, error) {
	report, err := deps.CheckDeps(implPath, wave)
	if err != nil {
		return nil, err
	}

	// Return nil if no conflicts (makes caller logic cleaner)
	if len(report.MissingDeps) == 0 && len(report.VersionConflicts) == 0 {
		return nil, nil
	}

	return report, nil
}
