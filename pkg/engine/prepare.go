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

// recordStepWithData appends a StepResult with structured data to the result and fires an event.
func recordStepWithData(res *PrepareWaveResult, cb EventCallback, step, status, detail string, data interface{}) {
	sr := StepResult{Step: step, Status: status, Detail: detail, Data: data}
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

// isSAWOwnedPath reports whether the given git-porcelain relative path
// is a SAW-managed state file that is safe to auto-commit in program context.
// A path is SAW-owned if it is the IMPL yaml file or falls under .saw-state/.
func isSAWOwnedPath(path, implPath, projectRoot string) bool {
	// Normalize implPath to repo-relative
	rel, err := filepath.Rel(projectRoot, implPath)
	if err != nil {
		rel = implPath
	}
	// git porcelain output has a 3-char prefix (e.g. " M "), trim it
	trimmed := strings.TrimSpace(path)
	if len(trimmed) > 3 {
		trimmed = strings.TrimSpace(trimmed[2:])
	}
	return trimmed == rel ||
		strings.HasPrefix(trimmed, ".saw-state/") ||
		strings.HasPrefix(trimmed, ".saw-state\\") ||
		strings.HasPrefix(trimmed, "docs/IMPL/") ||
		trimmed == "docs/CONTEXT.md" ||
		trimmed == "go.work" ||
		trimmed == "go.work.sum"
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

	// Write active-impl marker so SubagentStop hooks can find the IMPL doc.
	sawStateDir := protocol.SAWStateDir(projectRoot)
	_ = os.MkdirAll(sawStateDir, 0o755)
	_ = os.WriteFile(filepath.Join(sawStateDir, "active-impl"), []byte(opts.IMPLPath), 0o644)

	// Step: Resume detection — warn on orphaned worktrees
	if sessions, err := resume.Detect(ctx, projectRoot); err == nil {
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
	doc, err := protocol.Load(ctx, opts.IMPLPath)
	if err != nil {
		recordStep(res, opts.OnEvent, "load_manifest", "failed", err.Error())
		return res, fmt.Errorf("failed to load IMPL doc: %w", err)
	}
	recordStep(res, opts.OnEvent, "load_manifest", "success", "manifest loaded")

	// Step: I3 wave sequencing gate — Wave N-1 must be verified before Wave N launches.
	// Prevents Wave N from launching while Wave N-1 is still executing, blocked, or unmerged.
	if opts.WaveNum > 1 {
		seqRes := checkPreviousWaveVerified(doc, opts.WaveNum-1)
		if seqRes.IsFatal() {
			msg := "wave sequencing check failed"
			if len(seqRes.Errors) > 0 {
				msg = seqRes.Errors[0].Message
			}
			recordStep(res, opts.OnEvent, "wave_sequencing", "failed", msg)
			return res, fmt.Errorf("I3 wave sequencing violation: %s", msg)
		}
		recordStep(res, opts.OnEvent, "wave_sequencing", "success",
			fmt.Sprintf("wave %d verified — launching wave %d", opts.WaveNum-1, opts.WaveNum))
	}

	// Step: E37 critic verdict enforcement — only applies when threshold is met (3+ wave-1 agents OR 2+ repos).
	if !protocol.E37Required(doc) {
		recordStep(res, opts.OnEvent, "critic_verdict", "skipped", "threshold not met — critic review not required")
	} else if !protocol.CriticGatePasses(doc, false) {
		// Gate failed — either no critic report or ISSUES with errors
		if doc.CriticReport == nil {
			recordStep(res, opts.OnEvent, "critic_verdict", "failed", "no critic report present")
			return res, fmt.Errorf("E37: critic gate failed — no critic report")
		}
		// Count errors for detailed diagnostics
		errorCount := 0
		for _, review := range doc.CriticReport.AgentReviews {
			for _, issue := range review.Issues {
				if issue.Severity == protocol.CriticSeverityError {
					errorCount++
				}
			}
		}
		detail := fmt.Sprintf("critic found %d error(s) in agent briefs", errorCount)
		recordStep(res, opts.OnEvent, "critic_verdict", "failed", detail)
		return res, fmt.Errorf("E37: critic found %d error(s) in agent briefs — fix before launching wave", errorCount)
	} else {
		// Gate passed — PASS verdict, ISSUES with warnings only, or SKIPPED
		switch doc.CriticReport.Verdict {
		case protocol.CriticVerdictIssues:
			warningCount := doc.CriticReport.IssueCount
			recordStep(res, opts.OnEvent, "critic_verdict", "success",
				fmt.Sprintf("critic found %d warning(s) only — proceeding (advisory)", warningCount))
		case protocol.CriticVerdictSkipped:
			recordStep(res, opts.OnEvent, "critic_verdict", "skipped", "critic review was skipped")
		default:
			recordStep(res, opts.OnEvent, "critic_verdict", "success", "critic review passed")
		}
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
	repoMatchRes := protocol.ValidateRepoMatch(doc, projectRoot, repos)
	if repoMatchRes.IsFatal() || (repoMatchRes.IsPartial() && !repoMatchRes.GetData().Valid) {
		var errMsg string
		if len(repoMatchRes.Errors) > 0 {
			errMsg = repoMatchRes.Errors[0].Message
		} else {
			errMsg = "repo mismatch"
		}
		recordStep(res, opts.OnEvent, "repo_match", "failed", errMsg)
		return res, fmt.Errorf("repo mismatch: %s", errMsg)
	}
	recordStep(res, opts.OnEvent, "repo_match", "success", "repo match validated")

	// Step: Dependency output verification (H2, wave > 1 only)
	if opts.WaveNum > 1 {
		depResult := protocol.VerifyDependenciesAvailable(ctx, doc, opts.WaveNum)
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
		if w.Code == result.CodeRepoMismatch {
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
		// P1 fix: Save current branch before checkout, restore via defer
		out, err := git.Run(projectRoot, "branch", "--show-current")
		if err == nil {
			res.OriginalBranch = strings.TrimSpace(string(out))
			if res.OriginalBranch != "" {
				// Restore original branch on function exit (success or failure)
				defer func() {
					git.Run(projectRoot, "checkout", res.OriginalBranch)
				}()
			}
		}

		// In program context, create-program-worktrees may have already checked out
		// this branch as a long-lived IMPL branch worktree. Attempting git checkout on
		// an already-worktree branch fails with "already used by worktree at <path>".
		// Detect this case and skip the checkout — the worktree is the merge target.
		alreadyWorktree := false
		if worktrees, wtErr := git.WorktreeList(projectRoot); wtErr == nil {
			for _, wt := range worktrees {
				if wt[1] == opts.MergeTarget {
					alreadyWorktree = true
					recordStep(res, opts.OnEvent, "merge_target_checkout", "skipped",
						fmt.Sprintf("branch already checked out as worktree at %s", wt[0]))
					break
				}
			}
		}
		if !alreadyWorktree {
			if _, err := git.Run(projectRoot, "checkout", opts.MergeTarget); err != nil {
				recordStep(res, opts.OnEvent, "merge_target_checkout", "failed", err.Error())
				return res, fmt.Errorf("failed to checkout merge target %s: %w", opts.MergeTarget, err)
			}
			recordStep(res, opts.OnEvent, "merge_target_checkout", "success",
				fmt.Sprintf("checked out %s", opts.MergeTarget))
		}
	}

	// Step: Working directory check + auto-commit baseline fixes
	out, err := git.Run(projectRoot, "status", "--porcelain")
	if err != nil {
		recordStep(res, opts.OnEvent, "working_dir_check", "warning",
			fmt.Sprintf("could not check git status: %v", err))
	} else if len(out) > 0 {
		// Working directory is dirty
		modifiedFiles := strings.Split(strings.TrimSpace(string(out)), "\n")
		fileCount := len(modifiedFiles)

		if opts.CommitBaseline {
			// Auto-commit baseline fixes
			os.Setenv("SAW_ALLOW_MAIN_COMMIT", "1")
			defer os.Unsetenv("SAW_ALLOW_MAIN_COMMIT")

			// Stage all changes
			if _, addErr := git.Run(projectRoot, "add", "-A"); addErr != nil {
				recordStep(res, opts.OnEvent, "commit_baseline", "failed",
					fmt.Sprintf("failed to stage files: %v", addErr))
				return res, fmt.Errorf("failed to stage baseline fixes: %w", addErr)
			}

			// Commit with descriptive message
			commitMsg := fmt.Sprintf("chore: fix baseline for wave %d\n\nAuto-committed %d modified file(s) before worktree creation.",
				opts.WaveNum, fileCount)
			if _, commitErr := git.Run(projectRoot, "commit", "-m", commitMsg); commitErr != nil {
				recordStep(res, opts.OnEvent, "commit_baseline", "failed",
					fmt.Sprintf("failed to create commit: %v", commitErr))
				return res, fmt.Errorf("failed to commit baseline fixes: %w", commitErr)
			}

			recordStep(res, opts.OnEvent, "commit_baseline", "success",
				fmt.Sprintf("committed %d file(s)", fileCount))
		} else if opts.CommitState {
			// Classify dirty files: SAW-owned vs user code
			var sawFiles []string
			var userFiles []string
			for _, f := range modifiedFiles {
				if isSAWOwnedPath(f, opts.IMPLPath, projectRoot) {
					sawFiles = append(sawFiles, f)
				} else {
					userFiles = append(userFiles, f)
				}
			}
			if len(userFiles) > 0 {
				detail := fmt.Sprintf("%d user-code file(s) modified — --commit-state only commits SAW state files; commit user code manually", len(userFiles))
				recordStep(res, opts.OnEvent, "working_dir_check", "failed", detail)
				return res, fmt.Errorf("working directory has user-code changes: %d file(s) (--commit-state does not commit user code)", len(userFiles))
			}
			if len(sawFiles) == 0 {
				recordStep(res, opts.OnEvent, "working_dir_check", "success", "working directory is clean")
			} else {
				os.Setenv("SAW_ALLOW_MAIN_COMMIT", "1")
				defer os.Unsetenv("SAW_ALLOW_MAIN_COMMIT")
				for _, f := range sawFiles {
					// Stage each SAW-owned file individually (not -A)
					trimmed := strings.TrimSpace(f)
					if len(trimmed) > 3 {
						trimmed = strings.TrimSpace(trimmed[2:])
					}
					if _, addErr := git.Run(projectRoot, "add", trimmed); addErr != nil {
						recordStep(res, opts.OnEvent, "commit_state", "failed",
							fmt.Sprintf("failed to stage %s: %v", trimmed, addErr))
						return res, fmt.Errorf("failed to stage SAW state file %s: %w", trimmed, addErr)
					}
				}
				commitMsg := fmt.Sprintf("chore: update SAW state for wave %d preparation\n\nAuto-committed %d SAW state file(s): %s",
					opts.WaveNum, len(sawFiles), strings.Join(sawFiles, ", "))
				if _, commitErr := git.Run(projectRoot, "commit", "-m", commitMsg); commitErr != nil {
					recordStep(res, opts.OnEvent, "commit_state", "failed",
						fmt.Sprintf("failed to commit: %v", commitErr))
					return res, fmt.Errorf("failed to commit SAW state files: %w", commitErr)
				}
				recordStep(res, opts.OnEvent, "commit_state", "success",
					fmt.Sprintf("committed %d SAW state file(s)", len(sawFiles)))
			}
		} else {
			// Working directory is dirty but auto-commit not requested
			detail := fmt.Sprintf("%d modified file(s) detected. Use --commit-baseline to auto-commit, or commit manually", fileCount)
			recordStep(res, opts.OnEvent, "working_dir_check", "failed", detail)
			return res, fmt.Errorf("working directory is dirty: %d modified files (use --commit-baseline or commit manually)", fileCount)
		}
	} else {
		recordStep(res, opts.OnEvent, "working_dir_check", "success", "working directory is clean")
	}

	// Step: Baseline quality gates (E21A) with gate cache
	var cache *gatecache.Cache
	if !opts.NoCache {
		stateDir := protocol.SAWStateDir(projectRoot)
		cache = gatecache.New(ctx, stateDir, gatecache.DefaultTTL)
	}

	baselineRes := protocol.RunBaselineGates(ctx, doc, opts.WaveNum, projectRoot, cache)
	if baselineRes.IsFatal() {
		recordStep(res, opts.OnEvent, "baseline_gates", "failed", fmt.Sprintf("%v", baselineRes.Errors))
		return res, fmt.Errorf("failed to run baseline quality gates: %v", baselineRes.Errors)
	}
	baselineResult := baselineRes.GetData()
	if !baselineResult.Passed {
		recordStepWithData(res, opts.OnEvent, "baseline_gates", "failed", baselineResult.Reason, baselineResult)
		return res, fmt.Errorf("baseline verification failed: %s", baselineResult.Reason)
	}
	recordStep(res, opts.OnEvent, "baseline_gates", "success", "baseline gates passed")

	// Step: Cross-repo baseline gates (E21B, multi-repo only)
	targetRepos, resolveErr := protocol.ResolveTargetRepos(doc, projectRoot, repos)
	if resolveErr == nil && len(targetRepos) > 1 {
		crossRes := protocol.RunCrossRepoBaselineGates(ctx, doc, opts.WaveNum, targetRepos, repos)
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
		violations, freezeErr := protocol.CheckFreeze(ctx, doc)
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
			if e.Code == result.CodeDisjointOwnership {
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
	collisionReport, err := collision.DetectCollisions(ctx, opts.IMPLPath, opts.WaveNum, projectRoot)
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

	// Auto-advance SCOUT_PENDING → REVIEWED now that validation + critic have cleared.
	// finalize-wave needs the state to be at least REVIEWED to reach WAVE_VERIFIED.
	if doc.State == protocol.StateScoutPending || doc.State == protocol.StateScoutValidating {
		// Bypass isolation hook for orchestration commit
		os.Setenv("SAW_ALLOW_MAIN_COMMIT", "1")
		defer os.Unsetenv("SAW_ALLOW_MAIN_COMMIT")

		stateRes := protocol.SetImplState(ctx, opts.IMPLPath, protocol.StateReviewed, protocol.SetImplStateOpts{
			Commit:    true,
			CommitMsg: "chore: advance IMPL state to REVIEWED (pre-wave gate passed)",
		})
		if stateRes.IsFatal() {
			recordStep(res, opts.OnEvent, "advance_state", "warning",
				fmt.Sprintf("could not advance to REVIEWED: %v", stateRes.Errors))
		} else {
			recordStep(res, opts.OnEvent, "advance_state", "success", "IMPL state advanced to REVIEWED")
			// Reload manifest so subsequent steps see the updated state
			doc, err = protocol.Load(ctx, opts.IMPLPath)
			if err != nil {
				return res, fmt.Errorf("failed to reload manifest after state advance: %w", err)
			}
		}
	}

	// Step: Create worktrees
	wtRes := protocol.CreateWorktrees(ctx, opts.IMPLPath, opts.WaveNum, projectRoot, opts.Logger)
	if !wtRes.IsSuccess() {
		recordStep(res, opts.OnEvent, "create_worktrees", "failed", fmt.Sprintf("%v", wtRes.Errors))
		return res, fmt.Errorf("failed to create worktrees: %v", wtRes.Errors)
	}
	worktreeResult := wtRes.GetData()
	res.Worktrees = worktreeResult.Worktrees
	recordStep(res, opts.OnEvent, "create_worktrees", "success",
		fmt.Sprintf("created %d worktree(s)", len(worktreeResult.Worktrees)))

	// Step: gowork_setup — create/update go.work for LSP cross-package resolution
	if !opts.NoGoWork {
		goworkResult := StepGoWorkSetup(ctx, projectRoot, opts.WaveNum, worktreeResult.Worktrees, opts.OnEvent, opts.Logger)
		if goworkResult != nil {
			recordStepWithData(res, opts.OnEvent, goworkResult.Step, goworkResult.Status, goworkResult.Detail, goworkResult.Data)
		}
	} else {
		recordStep(res, opts.OnEvent, "gowork_setup", "skipped", "--no-gowork flag set")
	}

	// Step: Verify pre-commit hooks (H10)
	for _, wtInfo := range worktreeResult.Worktrees {
		hookRes := verifyHookInWorktree(wtInfo.Path)
		if hookRes.IsFatal() {
			msg := "hook verification failed"
			if len(hookRes.Errors) > 0 {
				msg = hookRes.Errors[0].Message
			}
			recordStep(res, opts.OnEvent, "verify_hooks", "failed",
				fmt.Sprintf("agent %s: %s", wtInfo.Agent, msg))
			return res, fmt.Errorf("hook verification failed for agent %s: %s", wtInfo.Agent, msg)
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

	// Write result to state file for automation-friendly access
	stateDir := protocol.SAWStateDir(projectRoot)
	waveStateDir := filepath.Join(stateDir, fmt.Sprintf("wave%d", opts.WaveNum))
	if err := os.MkdirAll(waveStateDir, 0o755); err == nil {
		resultPath := filepath.Join(waveStateDir, "prepare-result.json")
		resultJSON, _ := json.MarshalIndent(res, "", "  ")
		if writeErr := os.WriteFile(resultPath, resultJSON, 0o644); writeErr != nil {
			recordStep(res, opts.OnEvent, "write_state_file", "warning",
				fmt.Sprintf("failed to write state file: %v", writeErr))
		} else {
			recordStep(res, opts.OnEvent, "write_state_file", "success",
				fmt.Sprintf("wrote %s", resultPath))
		}
	}

	res.Success = true
	return res, nil
}

// checkPreviousWaveVerified enforces I3 (wave sequencing): all agents in prevWaveNum
// must have completion reports with status "complete" before the next wave can launch.
// This is the execution-time gate that prevents Wave N from launching while Wave N-1
// is still executing, has blocked agents, or has not been finalized and merged.
func checkPreviousWaveVerified(doc *protocol.IMPLManifest, prevWaveNum int) result.Result[CheckData] {
	var prevWave *protocol.Wave
	for i := range doc.Waves {
		if doc.Waves[i].Number == prevWaveNum {
			prevWave = &doc.Waves[i]
			break
		}
	}
	if prevWave == nil {
		return result.NewFailure[CheckData]([]result.SAWError{
			result.NewFatal(result.CodeWaveSequencingFailed,
				fmt.Sprintf("wave %d not found in manifest — cannot verify sequencing", prevWaveNum)),
		})
	}

	for _, agent := range prevWave.Agents {
		report, exists := doc.CompletionReports[agent.ID]
		if !exists {
			return result.NewFailure[CheckData]([]result.SAWError{
				result.NewFatal(result.CodeWaveSequencingFailed,
					fmt.Sprintf("wave %d agent %s has no completion report — wave must be merged and verified before wave %d launches",
						prevWaveNum, agent.ID, prevWaveNum+1)),
			})
		}
		if report.Status != protocol.StatusComplete {
			return result.NewFailure[CheckData]([]result.SAWError{
				result.NewFatal(result.CodeWaveSequencingFailed,
					fmt.Sprintf("wave %d agent %s status is %q (expected \"complete\") — wave must be fully merged and verified before wave %d launches",
						prevWaveNum, agent.ID, report.Status, prevWaveNum+1)),
			})
		}
	}

	return result.NewSuccess(CheckData{PrevWaveNum: prevWaveNum})
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
func verifyHookInWorktree(worktreePath string) result.Result[VerifyHookData] {
	absPath, err := filepath.Abs(worktreePath)
	if err != nil {
		return result.NewFailure[VerifyHookData]([]result.SAWError{
			result.NewFatal(result.CodeHookVerifyFailed, "failed to resolve worktree path").WithCause(err),
		})
	}

	gitDir := filepath.Join(absPath, ".git")
	gitStat, err := os.Stat(gitDir)
	if err != nil {
		return result.NewFailure[VerifyHookData]([]result.SAWError{
			result.NewFatal(result.CodeHookVerifyFailed, "failed to stat .git").WithCause(err),
		})
	}

	var hookPath string
	if gitStat.IsDir() {
		hookPath = filepath.Join(gitDir, "hooks", "pre-commit")
	} else {
		// Worktree: .git is a file pointing to actual git dir
		content, err := os.ReadFile(gitDir)
		if err != nil {
			return result.NewFailure[VerifyHookData]([]result.SAWError{
				result.NewFatal(result.CodeHookVerifyFailed, "failed to read .git file").WithCause(err),
			})
		}
		line := strings.TrimSpace(string(content))
		if !strings.HasPrefix(line, "gitdir: ") {
			return result.NewFailure[VerifyHookData]([]result.SAWError{
				result.NewFatal(result.CodeHookVerifyFailed, fmt.Sprintf("invalid .git file format: %s", line)),
			})
		}
		actualGitDir := strings.TrimPrefix(line, "gitdir: ")
		hookPath = filepath.Join(actualGitDir, "hooks", "pre-commit")
	}

	stat, err := os.Stat(hookPath)
	if os.IsNotExist(err) {
		return result.NewFailure[VerifyHookData]([]result.SAWError{
			result.NewFatal(result.CodeHookVerifyFailed, fmt.Sprintf("pre-commit hook does not exist at %s", hookPath)),
		})
	}
	if err != nil {
		return result.NewFailure[VerifyHookData]([]result.SAWError{
			result.NewFatal(result.CodeHookVerifyFailed, "failed to stat hook").WithCause(err),
		})
	}

	if stat.Mode()&0111 == 0 {
		return result.NewFailure[VerifyHookData]([]result.SAWError{
			result.NewFatal(result.CodeHookVerifyFailed, "pre-commit hook exists but is not executable"),
		})
	}

	hookContent, err := os.ReadFile(hookPath)
	if err != nil {
		return result.NewFailure[VerifyHookData]([]result.SAWError{
			result.NewFatal(result.CodeHookVerifyFailed, "failed to read hook").WithCause(err),
		})
	}

	if !strings.Contains(string(hookContent), "SAW_ALLOW_MAIN_COMMIT") &&
		!strings.Contains(string(hookContent), "SAW pre-commit guard") {
		return result.NewFailure[VerifyHookData]([]result.SAWError{
			result.NewFatal(result.CodeHookVerifyFailed, "pre-commit hook missing isolation logic"),
		})
	}

	return result.NewSuccess(VerifyHookData{WorktreePath: worktreePath})
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

		// Build the agent brief with YAML frontmatter (E44: saw_name for Orchestrator naming)
		sawName := fmt.Sprintf("[SAW:wave%d:agent-%s] %s", opts.WaveNum, agentID, briefTaskSummary(agentTask))
		brief := fmt.Sprintf("---\nsaw_name: %q\n---\n\n# Agent %s Brief - Wave %d\n\n**IMPL Doc:** %s\n\n## Files Owned\n\n%s\n\n## Task\n\n%s\n%s%s%s%s\n",
			sawName,
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

// briefTaskSummary extracts a short task summary for use in saw_name frontmatter.
// It looks for a "**Role:**" line first, then falls back to the first non-empty line.
// Result is truncated to 60 characters.
func briefTaskSummary(task string) string {
	const maxLen = 60
	for _, line := range strings.Split(task, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "**Role:**") {
			line = strings.TrimSpace(strings.TrimPrefix(line, "**Role:**"))
			if len(line) > maxLen {
				return line[:maxLen]
			}
			return line
		}
	}
	// Fallback: first non-empty, non-heading line
	for _, line := range strings.Split(task, "\n") {
		line = strings.TrimSpace(line)
		if line != "" && !strings.HasPrefix(line, "#") {
			// Strip markdown bold markers for cleaner output
			line = strings.ReplaceAll(line, "**", "")
			if len(line) > maxLen {
				return line[:maxLen]
			}
			return line
		}
	}
	return "implement task"
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
