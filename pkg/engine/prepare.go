package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/blackwell-systems/scout-and-wave-go/internal/git"
	"github.com/blackwell-systems/scout-and-wave-go/pkg/collision"
	"github.com/blackwell-systems/scout-and-wave-go/pkg/deps"
	"github.com/blackwell-systems/scout-and-wave-go/pkg/gatecache"
	"github.com/blackwell-systems/scout-and-wave-go/pkg/journal"
	"github.com/blackwell-systems/scout-and-wave-go/pkg/protocol"
	"github.com/blackwell-systems/scout-and-wave-go/pkg/result"
	"github.com/blackwell-systems/scout-and-wave-go/pkg/resume"
)

// fireEvent safely invokes the OnEvent callback if non-nil.
func fireEvent(cb EventCallback, step, status, detail string) {
	if cb != nil {
		cb(step, status, detail)
	}
}

// recordStep appends a StepResult to the result and fires an event.
func recordStep(res *PrepareWaveResult, cb EventCallback, step, status, detail string) {
	sr := StepResult{Step: step, Status: status, Detail: detail}
	res.Steps = append(res.Steps, sr)
	fireEvent(cb, step, status, detail)
}

// loadSAWConfigRepos reads saw.config.json and returns the repos array.
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

// PrepareWave encapsulates the full wave preparation pipeline.
// It performs all pre-flight checks, creates worktrees, extracts agent briefs,
// and initializes journal observers. On failure, returns a partial result with
// completed steps.
func PrepareWave(ctx context.Context, opts PrepareWaveOpts) (*PrepareWaveResult, error) {
	res := &PrepareWaveResult{
		Wave:    opts.WaveNum,
		Success: false,
	}

	if opts.IMPLPath == "" {
		return res, fmt.Errorf("IMPLPath is required")
	}
	if opts.RepoPath == "" {
		return res, fmt.Errorf("RepoPath is required")
	}
	if opts.WaveNum == 0 {
		return res, fmt.Errorf("WaveNum is required (must be >= 1)")
	}

	projectRoot := opts.RepoPath

	// Step: Resume detection — warn on orphaned worktrees
	if sessions, err := resume.Detect(projectRoot); err == nil {
		for _, s := range sessions {
			if len(s.OrphanedWorktrees) > 0 {
				detail := fmt.Sprintf("%d orphaned worktree(s) for %s (wave %d)",
					len(s.OrphanedWorktrees), s.IMPLSlug, s.CurrentWave)
				recordStep(res, opts.OnEvent, "resume_detection", "warning", detail)
			}
		}
	}
	if len(res.Steps) == 0 || res.Steps[len(res.Steps)-1].Step != "resume_detection" {
		recordStep(res, opts.OnEvent, "resume_detection", "success", "no orphaned worktrees")
	}

	// Step: Check dependencies
	depReport, err := checkDependencies(opts.IMPLPath, opts.WaveNum)
	if err != nil {
		recordStep(res, opts.OnEvent, "dep_check", "failed", err.Error())
		return res, fmt.Errorf("failed to check dependencies: %w", err)
	}
	if depReport != nil {
		recordStep(res, opts.OnEvent, "dep_check", "failed", "dependency conflicts detected")
		return res, fmt.Errorf("dependency conflicts detected - resolve before creating worktrees")
	}
	recordStep(res, opts.OnEvent, "dep_check", "success", "no conflicts")

	// Load repos config for multi-repo agent routing
	repos := loadSAWConfigRepos(filepath.Join(projectRoot, "saw.config.json"))

	// Step: Load and validate manifest
	doc, err := protocol.Load(opts.IMPLPath)
	if err != nil {
		recordStep(res, opts.OnEvent, "load_manifest", "failed", err.Error())
		return res, fmt.Errorf("failed to load IMPL doc: %w", err)
	}
	recordStep(res, opts.OnEvent, "load_manifest", "success", "manifest loaded")

	// Step: E37 critic verdict enforcement
	if doc.CriticReport != nil {
		switch doc.CriticReport.Verdict {
		case protocol.CriticVerdictIssues:
			detail := fmt.Sprintf("critic found errors in agent briefs")
			recordStep(res, opts.OnEvent, "critic_verdict", "failed", detail)
			return res, fmt.Errorf("E37: critic verdict is ISSUES - fix briefs before launching wave")
		case protocol.CriticVerdictSkipped:
			recordStep(res, opts.OnEvent, "critic_verdict", "skipped", "critic review was skipped")
		case protocol.CriticVerdictPass:
			recordStep(res, opts.OnEvent, "critic_verdict", "success", "critic review passed")
		default:
			recordStep(res, opts.OnEvent, "critic_verdict", "success", fmt.Sprintf("verdict: %s", doc.CriticReport.Verdict))
		}
	} else {
		recordStep(res, opts.OnEvent, "critic_verdict", "skipped", "no critic report present")
	}

	// Step: Stale worktree cleanup for same slug
	stale, staleErr := protocol.DetectStaleWorktrees(projectRoot)
	if staleErr == nil {
		var sameSlug []protocol.StaleWorktree
		for _, s := range stale {
			if s.Slug == doc.FeatureSlug {
				sameSlug = append(sameSlug, s)
			}
		}
		if len(sameSlug) > 0 {
			cleanRes := protocol.CleanStaleWorktrees(sameSlug, true)
			cleanData := cleanRes.GetData()
			if cleanData != nil {
				if cleanRes.IsSuccess() || cleanRes.IsPartial() {
					recordStep(res, opts.OnEvent, "stale_cleanup", "success",
						fmt.Sprintf("cleaned %d stale worktree(s)", len(cleanData.Cleaned)))
				}
				for _, e := range cleanData.Errors {
					recordStep(res, opts.OnEvent, "stale_cleanup", "warning",
						fmt.Sprintf("failed for %s: %s", e.Worktree.BranchName, e.Error))
				}
			}
		} else {
			recordStep(res, opts.OnEvent, "stale_cleanup", "success", "no stale worktrees")
		}
	} else {
		recordStep(res, opts.OnEvent, "stale_cleanup", "warning", staleErr.Error())
	}

	// Step: Repo match validation
	if err := protocol.ValidateRepoMatch(doc, projectRoot, repos); err != nil {
		recordStep(res, opts.OnEvent, "repo_match", "failed", err.Error())
		return res, fmt.Errorf("repo mismatch: %w", err)
	}
	recordStep(res, opts.OnEvent, "repo_match", "success", "repo match validated")

	// Step: Dependency output verification (H2, wave > 1 only)
	if opts.WaveNum > 1 {
		depResult := protocol.VerifyDependenciesAvailable(doc, opts.WaveNum)
		if depResult.IsFatal() || !depResult.GetData().Valid {
			var missing []string
			for _, agent := range depResult.GetData().Agents {
				if !agent.Available {
					missing = append(missing, fmt.Sprintf("agent %s: %v", agent.Agent, agent.Missing))
				}
			}
			detail := strings.Join(missing, "; ")
			recordStep(res, opts.OnEvent, "dep_outputs", "failed", detail)
			return res, fmt.Errorf("dependency outputs not available for wave %d", opts.WaveNum)
		}
		recordStep(res, opts.OnEvent, "dep_outputs", "success", "dependency outputs verified")
	}

	// Step: File ownership coverage check (H1, wave > 1, warning only)
	if opts.WaveNum > 1 {
		coverageResult, err := protocol.ValidateFileOwnershipCoverage(doc, opts.WaveNum-1)
		if err != nil {
			recordStep(res, opts.OnEvent, "ownership_coverage", "warning", err.Error())
		} else if coverageResult.IsSuccess() && !coverageResult.GetData().Valid {
			var violations []string
			for _, v := range coverageResult.GetData().Violations {
				violations = append(violations, fmt.Sprintf("agent %s modified unowned files: %v", v.Agent, v.UnownedFiles))
			}
			recordStep(res, opts.OnEvent, "ownership_coverage", "warning", strings.Join(violations, "; "))
		} else {
			recordStep(res, opts.OnEvent, "ownership_coverage", "success", "coverage valid")
		}
	}

	// Step: File existence validation across repos
	fileWarnings := protocol.ValidateFileExistenceMultiRepo(doc, projectRoot, repos)
	for _, w := range fileWarnings {
		if w.Code == "E16_REPO_MISMATCH_SUSPECTED" {
			recordStep(res, opts.OnEvent, "file_existence", "failed", w.Message)
			return res, fmt.Errorf("file existence: %s", w.Message)
		}
	}
	if len(fileWarnings) > 0 {
		var msgs []string
		for _, w := range fileWarnings {
			msgs = append(msgs, w.Message)
		}
		recordStep(res, opts.OnEvent, "file_existence", "warning", strings.Join(msgs, "; "))
	} else {
		recordStep(res, opts.OnEvent, "file_existence", "success", "all files validated")
	}

	// Step: Merge target checkout (when set)
	if opts.MergeTarget != "" {
		if _, err := git.Run(projectRoot, "checkout", opts.MergeTarget); err != nil {
			recordStep(res, opts.OnEvent, "merge_target_checkout", "failed", err.Error())
			return res, fmt.Errorf("failed to checkout merge target %s: %w", opts.MergeTarget, err)
		}
		recordStep(res, opts.OnEvent, "merge_target_checkout", "success",
			fmt.Sprintf("checked out %s", opts.MergeTarget))
	}

	// Step: Baseline quality gates (E21A) with gate cache
	var cache *gatecache.Cache
	if !opts.NoCache {
		stateDir := filepath.Join(projectRoot, ".saw-state")
		cache = gatecache.New(stateDir, gatecache.DefaultTTL)
	}

	baselineRes := protocol.RunBaselineGates(doc, opts.WaveNum, projectRoot, cache)
	if baselineRes.IsFatal() {
		recordStep(res, opts.OnEvent, "baseline_gates", "failed", fmt.Sprintf("%v", baselineRes.Errors))
		return res, fmt.Errorf("failed to run baseline quality gates: %v", baselineRes.Errors)
	}
	baselineResult := baselineRes.GetData()
	if !baselineResult.Passed {
		recordStep(res, opts.OnEvent, "baseline_gates", "failed", baselineResult.Reason)
		return res, fmt.Errorf("baseline verification failed: %s", baselineResult.Reason)
	}
	recordStep(res, opts.OnEvent, "baseline_gates", "success", "baseline gates passed")

	// Step: Cross-repo baseline gates (E21B, multi-repo only)
	targetRepos, resolveErr := protocol.ResolveTargetRepos(doc, projectRoot, repos)
	if resolveErr == nil && len(targetRepos) > 1 {
		crossRes := protocol.RunCrossRepoBaselineGates(doc, opts.WaveNum, targetRepos, repos)
		if crossRes.IsFatal() {
			recordStep(res, opts.OnEvent, "cross_repo_baseline", "failed", fmt.Sprintf("%v", crossRes.Errors))
			return res, fmt.Errorf("E21B cross-repo baseline check failed: %v", crossRes.Errors)
		}
		crossResult := crossRes.GetData()
		if !crossResult.Passed {
			recordStep(res, opts.OnEvent, "cross_repo_baseline", "failed", "one or more repos do not build")
			return res, fmt.Errorf("E21B: cross-repo baseline verification failed")
		}
		recordStep(res, opts.OnEvent, "cross_repo_baseline", "success",
			fmt.Sprintf("passed (%d repos)", len(targetRepos)))
	} else {
		recordStep(res, opts.OnEvent, "cross_repo_baseline", "skipped", "single-repo IMPL")
	}

	// Step: Wiring ownership validation (E35 Layer 3A)
	if violations := protocol.CheckWiringOwnership(doc, opts.WaveNum); len(violations) > 0 {
		detail := strings.Join(violations, "; ")
		recordStep(res, opts.OnEvent, "wiring_ownership", "failed", detail)
		return res, fmt.Errorf("%d wiring ownership violation(s)", len(violations))
	}
	recordStep(res, opts.OnEvent, "wiring_ownership", "success", "no violations")

	// Step: Scaffold commit validation (I2)
	if len(doc.Scaffolds) > 0 && !protocol.AllScaffoldsCommitted(doc) {
		statuses := protocol.ValidateScaffolds(doc)
		var uncommitted []string
		for _, s := range statuses {
			if s.Status != "committed" {
				uncommitted = append(uncommitted, fmt.Sprintf("%s (status=%s)", s.FilePath, s.Status))
			}
		}
		detail := strings.Join(uncommitted, "; ")
		recordStep(res, opts.OnEvent, "scaffold_validation", "failed", detail)
		return res, fmt.Errorf("scaffolds not committed - run Scaffold Agent before creating worktrees")
	}
	recordStep(res, opts.OnEvent, "scaffold_validation", "success", "scaffolds committed")

	// Step: Freeze check (I2, when WorktreesCreatedAt set)
	if doc.WorktreesCreatedAt != nil {
		violations, freezeErr := protocol.CheckFreeze(doc)
		if freezeErr != nil {
			recordStep(res, opts.OnEvent, "freeze_check", "warning", freezeErr.Error())
		} else if len(violations) > 0 {
			var msgs []string
			for _, v := range violations {
				msgs = append(msgs, fmt.Sprintf("%s: %s", v.Section, v.Message))
			}
			detail := strings.Join(msgs, "; ")
			recordStep(res, opts.OnEvent, "freeze_check", "failed", detail)
			return res, fmt.Errorf("%d freeze violation(s) - interface contracts modified after worktree creation", len(violations))
		} else {
			recordStep(res, opts.OnEvent, "freeze_check", "success", "no freeze violations")
		}
	} else {
		recordStep(res, opts.OnEvent, "freeze_check", "skipped", "no worktrees created yet")
	}

	// Step: I1 disjoint ownership verification (E3)
	if allErrs := protocol.Validate(doc); len(allErrs) > 0 {
		var i1Errs []result.SAWError
		for _, e := range allErrs {
			if e.Code == "I1_VIOLATION" {
				i1Errs = append(i1Errs, e)
			}
		}
		if len(i1Errs) > 0 {
			var msgs []string
			for _, e := range i1Errs {
				msgs = append(msgs, e.Message)
			}
			detail := strings.Join(msgs, "; ")
			recordStep(res, opts.OnEvent, "i1_disjoint", "failed", detail)
			return res, fmt.Errorf("%d I1 disjoint ownership violation(s)", len(i1Errs))
		}
	}
	recordStep(res, opts.OnEvent, "i1_disjoint", "success", "ownership is disjoint")

	// Step: Type collision check (E41)
	collisionReport, err := collision.DetectCollisions(opts.IMPLPath, opts.WaveNum, projectRoot)
	if err != nil {
		recordStep(res, opts.OnEvent, "type_collision", "warning", err.Error())
	} else if !collisionReport.Valid {
		recordStep(res, opts.OnEvent, "type_collision", "failed", "type collisions detected")
		return res, fmt.Errorf("type collisions detected - resolve before creating worktrees")
	} else {
		recordStep(res, opts.OnEvent, "type_collision", "success", "no collisions")
	}

	// Step: Pre-wave readiness gate (C7)
	gateResult := protocol.PreWaveGate(doc)
	if !gateResult.Ready {
		var failedChecks []string
		for _, check := range gateResult.Checks {
			if check.Status == "fail" {
				failedChecks = append(failedChecks, check.Name)
			}
		}
		detail := fmt.Sprintf("checks failed: %s", strings.Join(failedChecks, ", "))
		recordStep(res, opts.OnEvent, "pre_wave_gate", "failed", detail)
		return res, fmt.Errorf("pre-wave gate failed: %s", detail)
	}
	recordStep(res, opts.OnEvent, "pre_wave_gate", "success", "pre-wave gate passed")

	// Step: Create worktrees
	wtRes := protocol.CreateWorktrees(opts.IMPLPath, opts.WaveNum, projectRoot)
	if !wtRes.IsSuccess() {
		recordStep(res, opts.OnEvent, "create_worktrees", "failed", fmt.Sprintf("%v", wtRes.Errors))
		return res, fmt.Errorf("failed to create worktrees: %v", wtRes.Errors)
	}
	worktreeResult := wtRes.GetData()
	res.Worktrees = worktreeResult.Worktrees
	recordStep(res, opts.OnEvent, "create_worktrees", "success",
		fmt.Sprintf("created %d worktree(s)", len(worktreeResult.Worktrees)))

	// Step: Verify pre-commit hooks (H10)
	for _, wtInfo := range worktreeResult.Worktrees {
		if err := verifyHookInWorktree(wtInfo.Path); err != nil {
			recordStep(res, opts.OnEvent, "verify_hooks", "failed",
				fmt.Sprintf("agent %s: %s", wtInfo.Agent, err.Error()))
			return res, fmt.Errorf("hook verification failed for agent %s: %w", wtInfo.Agent, err)
		}
	}
	recordStep(res, opts.OnEvent, "verify_hooks", "success", "all hooks verified")

	// Step: Extract agent briefs + init journals
	agentBriefs, err := extractBriefsAndInitJournals(doc, worktreeResult.Worktrees, opts)
	if err != nil {
		recordStep(res, opts.OnEvent, "extract_briefs", "failed", err.Error())
		return res, fmt.Errorf("failed to extract briefs: %w", err)
	}
	res.AgentBriefs = agentBriefs
	recordStep(res, opts.OnEvent, "extract_briefs", "success",
		fmt.Sprintf("prepared %d agent(s)", len(agentBriefs)))

	res.Success = true
	return res, nil
}

// checkDependencies wraps deps.CheckDeps to return nil report if no conflicts detected.
func checkDependencies(implPath string, wave int) (*deps.ConflictReport, error) {
	report, err := deps.CheckDeps(implPath, wave)
	if err != nil {
		return nil, err
	}

	realConflicts := 0
	for _, vc := range report.VersionConflicts {
		if vc.ResolutionNeeded {
			realConflicts++
		}
	}
	if len(report.MissingDeps) == 0 && realConflicts == 0 {
		return nil, nil
	}

	return report, nil
}

// verifyHookInWorktree checks that a pre-commit hook exists and is executable.
func verifyHookInWorktree(worktreePath string) error {
	absPath, err := filepath.Abs(worktreePath)
	if err != nil {
		return fmt.Errorf("failed to resolve worktree path: %w", err)
	}

	gitDir := filepath.Join(absPath, ".git")
	gitStat, err := os.Stat(gitDir)
	if err != nil {
		return fmt.Errorf("failed to stat .git: %w", err)
	}

	var hookPath string
	if gitStat.IsDir() {
		hookPath = filepath.Join(gitDir, "hooks", "pre-commit")
	} else {
		// Worktree: .git is a file pointing to actual git dir
		content, err := os.ReadFile(gitDir)
		if err != nil {
			return fmt.Errorf("failed to read .git file: %w", err)
		}
		line := strings.TrimSpace(string(content))
		if !strings.HasPrefix(line, "gitdir: ") {
			return fmt.Errorf("invalid .git file format: %s", line)
		}
		actualGitDir := strings.TrimPrefix(line, "gitdir: ")
		hookPath = filepath.Join(actualGitDir, "hooks", "pre-commit")
	}

	stat, err := os.Stat(hookPath)
	if os.IsNotExist(err) {
		return fmt.Errorf("pre-commit hook does not exist at %s", hookPath)
	}
	if err != nil {
		return fmt.Errorf("failed to stat hook: %w", err)
	}

	if stat.Mode()&0111 == 0 {
		return fmt.Errorf("pre-commit hook exists but is not executable")
	}

	hookContent, err := os.ReadFile(hookPath)
	if err != nil {
		return fmt.Errorf("failed to read hook: %w", err)
	}

	if !strings.Contains(string(hookContent), "SAW_ALLOW_MAIN_COMMIT") &&
		!strings.Contains(string(hookContent), "SAW pre-commit guard") {
		return fmt.Errorf("pre-commit hook missing isolation logic")
	}

	return nil
}

// extractBriefsAndInitJournals prepares agent briefs and initializes journals
// for all worktrees.
func extractBriefsAndInitJournals(
	doc *protocol.IMPLManifest,
	worktrees []protocol.WorktreeInfo,
	opts PrepareWaveOpts,
) ([]AgentBriefInfo, error) {
	repos := loadSAWConfigRepos(filepath.Join(opts.RepoPath, "saw.config.json"))
	agentBriefs := make([]AgentBriefInfo, 0, len(worktrees))

	for _, wtInfo := range worktrees {
		agentID := wtInfo.Agent

		// Find the agent's task and files
		var agentTask string
		var agentFiles []string
		for _, wave := range doc.Waves {
			if wave.Number != opts.WaveNum {
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
			return nil, fmt.Errorf("agent %s not found in wave %d", agentID, opts.WaveNum)
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

		// Wiring obligations (E35 Layer 3C)
		wiringSection := protocol.InjectWiringInstructions(doc, agentID, opts.WaveNum)

		// Merge target section
		var mergeTargetSection string
		if opts.MergeTarget != "" {
			mergeTargetSection = "\n\n## Merge Target\n\n**Merge your branch to:** `" + opts.MergeTarget + "`\n\n" +
				"Do NOT merge to HEAD or main. All wave commits must land on " +
				"this IMPL branch. The finalize-tier command will merge the " +
				"IMPL branch to main after all IMPLs in the tier complete.\n"
		}

		// Build the agent brief
		brief := fmt.Sprintf(`# Agent %s Brief - Wave %d

**IMPL Doc:** %s

## Files Owned

%s

## Task

%s
%s%s%s%s
`,
			agentID,
			opts.WaveNum,
			opts.IMPLPath,
			formatFileList(agentFiles),
			agentTask,
			mergeTargetSection,
			contractsSection,
			wiringSection,
			gatesSection,
		)

		// Write brief to worktree root
		briefPath := filepath.Join(wtInfo.Path, ".saw-agent-brief.md")
		if err := os.WriteFile(briefPath, []byte(brief), 0644); err != nil {
			return nil, fmt.Errorf("failed to write brief for agent %s: %w", agentID, err)
		}

		// Resolve the repo root for this agent
		agentRoot := protocol.ResolveAgentRepo(doc.FileOwnership, agentID, opts.RepoPath, repos)

		// Determine agent's repo name
		agentRepoName := ""
		for _, fo := range doc.FileOwnership {
			if fo.Agent == agentID && fo.Repo != "" {
				agentRepoName = fo.Repo
				break
			}
		}

		// Initialize journal observer
		fullAgentID := fmt.Sprintf("wave%d-agent-%s", opts.WaveNum, agentID)
		observer, err := journal.NewObserver(agentRoot, fullAgentID)
		if err != nil {
			return nil, fmt.Errorf("failed to create journal observer for agent %s: %w", agentID, err)
		}

		// Initialize cursor if it doesn't exist
		if _, err := os.Stat(observer.CursorPath); os.IsNotExist(err) {
			emptyCursor := journal.SessionCursor{
				SessionFile: "",
				Offset:      0,
			}
			cursorData, _ := json.MarshalIndent(emptyCursor, "", "  ")
			if err := os.WriteFile(observer.CursorPath, cursorData, 0644); err != nil {
				return nil, fmt.Errorf("failed to write cursor file for agent %s: %w", agentID, err)
			}
		}

		agentBriefs = append(agentBriefs, AgentBriefInfo{
			Agent:       agentID,
			BriefPath:   briefPath,
			BriefLength: len(brief),
			JournalDir:  observer.JournalDir,
			FilesOwned:  len(agentFiles),
			Repo:        agentRepoName,
			MergeTarget: opts.MergeTarget,
		})
	}

	return agentBriefs, nil
}

// formatFileList formats a list of file paths as a markdown bullet list.
func formatFileList(files []string) string {
	if len(files) == 0 {
		return "(none)"
	}
	var sb strings.Builder
	for _, f := range files {
		sb.WriteString("- `")
		sb.WriteString(f)
		sb.WriteString("`\n")
	}
	return sb.String()
}
