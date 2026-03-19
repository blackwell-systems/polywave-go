package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/deps"
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

// sawRepoEntry represents a single entry in the saw.config.json repos array.
type sawRepoEntry struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

// loadSAWConfigRepos reads saw.config.json at configPath and returns
// the repos array. Returns nil slice if file absent or parse fails.
func loadSAWConfigRepos(configPath string) []sawRepoEntry {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil
	}
	var cfg struct {
		Repos []sawRepoEntry `json:"repos"`
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
	repos []sawRepoEntry,
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

			// Step 0a: Resume detection — warn if orphaned worktrees exist
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

			// Step 2: Load manifest for brief extraction
			doc, err := protocol.Load(manifestPath)
			if err != nil {
				return fmt.Errorf("failed to load IMPL doc: %w", err)
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
				if doc.QualityGates.Level != "" {
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

				// Build the agent brief
				brief := fmt.Sprintf(`# Agent %s Brief - Wave %d

**IMPL Doc:** %s

## Files Owned

%s

## Task

%s
%s%s
`,
					agentID,
					waveNum,
					manifestPath,
					formatFileList(agentFiles),
					agentTask,
					contractsSection,
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
