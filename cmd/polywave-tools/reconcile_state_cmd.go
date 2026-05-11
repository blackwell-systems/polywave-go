package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/blackwell-systems/polywave-go/internal/git"
	"github.com/blackwell-systems/polywave-go/pkg/config"
	"github.com/blackwell-systems/polywave-go/pkg/protocol"
	"github.com/spf13/cobra"
)

// ReconcileStateResult is the JSON output of the reconcile-state command.
// It describes the observed state of the IMPL and any state change performed.
type ReconcileStateResult struct {
	PreviousState     string             `json:"previous_state"`
	DerivedState      string             `json:"derived_state"`
	StateChanged      bool               `json:"state_changed"`
	Evidence          []string           `json:"evidence"`
	RecommendedAction string             `json:"recommended_action"`
	AgentSummary      []AgentObservation `json:"agent_summary"`
}

// AgentObservation records what was observed for a single agent's git state.
type AgentObservation struct {
	ID              string `json:"id"`
	Wave            int    `json:"wave"`
	BranchExists    bool   `json:"branch_exists"`
	BranchMerged    bool   `json:"branch_merged"`
	HasReport       bool   `json:"has_report"`
	CommitReachable bool   `json:"commit_reachable"`
}

func newReconcileStateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "reconcile-state <manifest-path>",
		Short: "Reconcile IMPL manifest state with actual git state",
		Long: `Inspects the git state of all agent branches and completion reports
in an IMPL manifest, then derives and sets the correct ProtocolState.

The command is idempotent and safe to run at any time. It does NOT commit
the state change — the orchestrator decides whether to commit afterward.

Derived states:
  WAVE_VERIFIED   — all agents in the highest wave have reports AND all commits are reachable
  WAVE_EXECUTING  — any agent has a branch OR any agent has a report (but not all do)
  REVIEWED        — CriticReport has a non-empty Verdict and current state is SCOUT_PENDING/SCOUT_VALIDATING
  SCOUT_PENDING   — no completion reports, no branches, no critic report
  (unchanged)     — cannot determine a better state from observations

Examples:
  polywave-tools reconcile-state docs/IMPL/IMPL-feature.yaml
  polywave-tools --repo-dir /path/to/repo reconcile-state docs/IMPL/IMPL-feature.yaml`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			manifestPath := args[0]
			return runReconcileState(ctx, manifestPath)
		},
	}
	return cmd
}

func runReconcileState(ctx context.Context, manifestPath string) error {
	manifest, err := protocol.Load(ctx, manifestPath)
	if err != nil {
		return fmt.Errorf("reconcile-state: failed to load manifest: %w", err)
	}

	previousState := manifest.State

	// COMPLETE is terminal — no-op.
	if previousState == protocol.StateComplete {
		fmt.Fprintf(os.Stderr, "reconcile-state: manifest is already COMPLETE, no changes made\n")
		result := ReconcileStateResult{
			PreviousState:     string(previousState),
			DerivedState:      string(previousState),
			StateChanged:      false,
			Evidence:          []string{"state is COMPLETE (terminal)"},
			RecommendedAction: "no action needed",
			AgentSummary:      []AgentObservation{},
		}
		return writeReconcileResult(result)
	}

	// Resolve repo paths from config, same pattern as engine.ExtractReposFromManifest.
	manifestDir := filepath.Dir(manifestPath)
	repoPaths := resolveRepoPaths(manifest, manifestDir)

	// Observe git state for every agent in every wave.
	var observations []AgentObservation
	var evidence []string

	for _, wave := range manifest.Waves {
		for _, agent := range wave.Agents {
			obs := observeAgent(manifest, wave.Number, agent.ID, repoPaths)
			observations = append(observations, obs)

			if obs.BranchExists {
				evidence = append(evidence, fmt.Sprintf("wave%d agent %s: branch exists", wave.Number, agent.ID))
			}
			if obs.BranchMerged {
				evidence = append(evidence, fmt.Sprintf("wave%d agent %s: branch merged", wave.Number, agent.ID))
			}
			if obs.HasReport {
				evidence = append(evidence, fmt.Sprintf("wave%d agent %s: completion report present", wave.Number, agent.ID))
			}
			if obs.CommitReachable {
				evidence = append(evidence, fmt.Sprintf("wave%d agent %s: commit reachable from HEAD", wave.Number, agent.ID))
			}
		}
	}

	// Critic report evidence.
	criticHasVerdict := manifest.CriticReport != nil && manifest.CriticReport.Verdict != ""
	if criticHasVerdict {
		evidence = append(evidence, fmt.Sprintf("critic report present with verdict: %s", manifest.CriticReport.Verdict))
	}

	// Derive the new state.
	derivedState, recommendedAction := deriveState(manifest, observations, previousState, criticHasVerdict)

	// Print per-agent summary to stderr.
	fmt.Fprintf(os.Stderr, "reconcile-state: observations:\n")
	for _, obs := range observations {
		fmt.Fprintf(os.Stderr, "  wave%d agent %s: branch_exists=%v branch_merged=%v has_report=%v commit_reachable=%v\n",
			obs.Wave, obs.ID, obs.BranchExists, obs.BranchMerged, obs.HasReport, obs.CommitReachable)
	}
	fmt.Fprintf(os.Stderr, "reconcile-state: derived state: %s (previous: %s)\n", derivedState, previousState)

	stateChanged := false
	if derivedState != previousState {
		// Write new state without committing (caller decides whether to commit).
		res := protocol.SetImplState(ctx, manifestPath, derivedState, protocol.SetImplStateOpts{Commit: false})
		if res.IsSuccess() {
			stateChanged = true
			fmt.Fprintf(os.Stderr, "reconcile-state: state changed: %s -> %s\n", previousState, derivedState)
		} else {
			// State transition may be invalid (e.g. derived state not reachable from current).
			// Emit warning and keep previous state.
			fmt.Fprintf(os.Stderr, "reconcile-state: warning: could not set state to %s: %v\n", derivedState, res.Errors)
			derivedState = previousState
			evidence = append(evidence, fmt.Sprintf("warning: transition to %s not allowed from %s", derivedState, previousState))
		}
	}

	result := ReconcileStateResult{
		PreviousState:     string(previousState),
		DerivedState:      string(derivedState),
		StateChanged:      stateChanged,
		Evidence:          evidence,
		RecommendedAction: recommendedAction,
		AgentSummary:      observations,
	}

	fmt.Fprintf(os.Stderr, "reconcile-state: recommended action: %s\n", recommendedAction)

	return writeReconcileResult(result)
}

// writeReconcileResult marshals the result as JSON to stdout.
func writeReconcileResult(result ReconcileStateResult) error {
	out, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("reconcile-state: failed to marshal result: %w", err)
	}
	fmt.Println(string(out))
	return nil
}

// resolveRepoPaths builds a map from repo name (or ".") to absolute path,
// using config.Load to resolve named repos from polywave.config.json.
// This mirrors the pattern used by engine.ExtractReposFromManifest.
func resolveRepoPaths(manifest *protocol.IMPLManifest, manifestDir string) map[string]string {
	repoPaths := make(map[string]string)

	// Collect unique repo names referenced in file_ownership.
	repoNames := make(map[string]bool)
	for _, fo := range manifest.FileOwnership {
		if fo.Repo != "" {
			repoNames[fo.Repo] = true
		}
	}

	if len(repoNames) == 0 {
		// Single-repo IMPL: use manifest.Repository or repoDir global.
		primaryRepo := manifest.Repository
		if primaryRepo == "" {
			primaryRepo = repoDir
			if primaryRepo == "" || primaryRepo == "." {
				primaryRepo = manifestDir
			}
		}
		repoPaths["."] = primaryRepo
		return repoPaths
	}

	// Cross-repo: resolve repo names via polywave.config.json.
	cfg := config.Load(manifestDir)
	for name := range repoNames {
		// Default: treat name as relative/absolute path.
		resolved := name
		if !filepath.IsAbs(name) {
			resolved = filepath.Join(filepath.Dir(manifestDir), name)
		}

		// Override with config entry if available.
		if cfg.IsSuccess() {
			for _, entry := range cfg.GetData().Repos {
				if entry.Name == name {
					resolved = entry.Path
					break
				}
			}
		}
		repoPaths[name] = resolved
	}

	return repoPaths
}

// observeAgent checks the git state for one agent across all known repos.
func observeAgent(manifest *protocol.IMPLManifest, waveNum int, agentID string, repoPaths map[string]string) AgentObservation {
	obs := AgentObservation{
		ID:   agentID,
		Wave: waveNum,
	}

	// Check completion report.
	report, hasReport := manifest.CompletionReports[agentID]
	obs.HasReport = hasReport

	// Determine which repo to use for this agent.
	agentRepo := resolveAgentRepo(manifest, waveNum, agentID, repoPaths)

	// Check branch existence (slug-scoped, then legacy).
	slugBranch := protocol.BranchName(manifest.FeatureSlug, waveNum, agentID)
	legacyBranch := protocol.LegacyBranchName(waveNum, agentID)

	slugExists := git.BranchExists(agentRepo, slugBranch)
	legacyExists := git.BranchExists(agentRepo, legacyBranch)
	obs.BranchExists = slugExists || legacyExists

	// Check if commit is reachable from HEAD and whether branch is merged.
	if hasReport && report.Commit != "" {
		obs.CommitReachable = git.IsAncestor(agentRepo, report.Commit, "HEAD")
		obs.BranchMerged = obs.CommitReachable
	} else if obs.BranchExists {
		// No commit SHA in report — check if branch is an ancestor of HEAD.
		branchName := slugBranch
		if !slugExists && legacyExists {
			branchName = legacyBranch
		}
		obs.BranchMerged = git.IsAncestor(agentRepo, branchName, "HEAD")
	}

	return obs
}

// resolveAgentRepo returns the absolute repo path for a given agent based on
// file_ownership entries. Falls back to the "." default repo.
func resolveAgentRepo(manifest *protocol.IMPLManifest, waveNum int, agentID string, repoPaths map[string]string) string {
	for _, fo := range manifest.FileOwnership {
		if fo.Wave == waveNum && fo.Agent == agentID && fo.Repo != "" {
			if path, ok := repoPaths[fo.Repo]; ok {
				return path
			}
		}
	}
	// Default repo.
	if path, ok := repoPaths["."]; ok {
		return path
	}
	// Last resort: use repoDir global or cwd.
	if repoDir != "" && repoDir != "." {
		return repoDir
	}
	cwd, _ := os.Getwd()
	return cwd
}

// deriveState determines the correct ProtocolState from observations.
// Returns (derivedState, recommendedAction).
func deriveState(
	manifest *protocol.IMPLManifest,
	observations []AgentObservation,
	currentState protocol.ProtocolState,
	criticHasVerdict bool,
) (protocol.ProtocolState, string) {
	if len(observations) == 0 {
		return currentState, "no agents found; check manifest structure"
	}

	// Find highest-numbered wave.
	highestWave := 0
	for _, obs := range observations {
		if obs.Wave > highestWave {
			highestWave = obs.Wave
		}
	}

	// Count agents in highest wave.
	var highestWaveObs []AgentObservation
	for _, obs := range observations {
		if obs.Wave == highestWave {
			highestWaveObs = append(highestWaveObs, obs)
		}
	}

	// Check WAVE_VERIFIED: all agents in highest wave have reports AND commits reachable.
	if len(highestWaveObs) > 0 {
		allComplete := true
		for _, obs := range highestWaveObs {
			if !obs.HasReport || !obs.CommitReachable {
				allComplete = false
				break
			}
		}
		if allComplete {
			return protocol.StateWaveVerified, "run `polywave-tools finalize-wave` to merge and verify the build"
		}
	}

	// Check REVIEWED: critic report with non-empty verdict AND current state is pre-wave.
	if criticHasVerdict {
		if currentState == protocol.StateScoutPending || currentState == protocol.StateScoutValidating {
			return protocol.StateReviewed, "run `polywave-tools prepare-wave` to create agent worktrees"
		}
	}

	// Check WAVE_EXECUTING: any agent has a branch OR any has a completion report.
	anyBranch := false
	anyReport := false
	for _, obs := range observations {
		if obs.BranchExists {
			anyBranch = true
		}
		if obs.HasReport {
			anyReport = true
		}
	}
	if anyBranch || anyReport {
		return protocol.StateWaveExecuting, "wait for remaining agents to complete, then run `polywave-tools finalize-wave`"
	}

	// Check SCOUT_PENDING: no branches, no reports, no critic.
	if !anyBranch && !anyReport && !criticHasVerdict {
		return protocol.StateScoutPending, "run `polywave-tools run-scout` to begin planning"
	}

	// Cannot determine a clear state — keep current.
	return currentState, fmt.Sprintf("current state %s cannot be automatically reconciled; inspect manually", currentState)
}
