package engine

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/idgen"
	"github.com/blackwell-systems/scout-and-wave-go/pkg/protocol"
	"github.com/blackwell-systems/scout-and-wave-go/pkg/result"
)

// RunCriticOpts configures the engine.RunCritic call. The RunCritic function
// and its full implementation live in pkg/engine/critic.go (Agent D's file).
// These types are declared here so RunScoutFull compiles; Agent D's critic.go
// implements RunCritic and registers it via runCriticFn.
type RunCriticOpts struct {
	IMPLPath    string       // absolute path to IMPL doc (required)
	CriticModel string       // optional model override
	SAWRepoPath string       // optional; falls back to $SAW_REPO then ~/code/scout-and-wave
	Timeout     int          // minutes; default 20
	Logger      *slog.Logger // optional
}

// RunCriticResult holds the result of a RunCritic call.
type RunCriticResult struct {
	Verdict    string `json:"verdict"`     // "PASS" or "ISSUES"
	Summary    string `json:"summary"`
	IssueCount int    `json:"issue_count"`
	ReviewedAt string `json:"reviewed_at"`
}

// BuildCriticPromptOpts configures prompt-building for the critic agent
// without launching it. Used by --backend agent-tool.
type BuildCriticPromptOpts struct {
	IMPLPath    string // absolute path to IMPL doc (required)
	SAWRepoPath string // optional; falls back to $SAW_REPO then ~/code/scout-and-wave
}

// runCriticFn is the function variable through which RunScoutFull calls the
// critic gate. It is set by pkg/engine/critic.go (Agent D) via its package
// init(). Before critic.go is present (e.g. in agent C's isolated worktree),
// it remains nil and the critic gate is skipped.
var runCriticFn func(ctx context.Context, opts RunCriticOpts, onChunk func(string)) (RunCriticResult, error)

// runCriticOptsBuilder constructs a RunCriticOpts from the resolved fields.
func runCriticOptsBuilder(implPath, criticModel, sawRepoPath string, logger *slog.Logger) RunCriticOpts {
	return RunCriticOpts{
		IMPLPath:    implPath,
		CriticModel: criticModel,
		SAWRepoPath: sawRepoPath,
		Logger:      logger,
	}
}

// RunScoutFullOpts configures the full Scout orchestration workflow.
type RunScoutFullOpts struct {
	Feature             string
	RepoPath            string
	SAWRepoPath         string
	ScoutModel          string
	Timeout             int    // minutes; default 10
	ProgramManifestPath string
	NoCritic            bool
	CriticModel         string
	Logger              *slog.Logger
}

// RunScoutFullResult holds the result of a full Scout run.
type RunScoutFullResult struct {
	IMPLPath       string `json:"impl_path"`
	Slug           string `json:"impl_slug"`
	GatesPopulated int    `json:"gates_populated"`
	CriticVerdict  string `json:"critic_verdict,omitempty"`
}

// RunScoutFull is the full Scout orchestration wrapper. It:
//  1. Resolves repoPath (absolute); generates slug; computes implPath
//  2. Returns early if IMPL doc already has advanced state (StateReviewed+)
//  3. Creates docs/IMPL dir via os.MkdirAll
//  4. Calls ScoutCorrectionLoop with MaxRetries: 3
//  5. Waits for IMPL file (10s poll, 500ms interval)
//  6. Calls protocol.ValidateIMPLDoc; handles agent-ID errors with idgen.AssignAgentIDs
//  7. Calls FinalizeIMPLEngine for gate population
//  8. If !NoCritic and criticThresholdMet: calls engine.RunCritic (Agent D's function)
//  9. Returns RunScoutFullResult
func RunScoutFull(ctx context.Context, opts RunScoutFullOpts, onChunk func(string)) (RunScoutFullResult, error) {
	log := loggerFrom(opts.Logger)

	// Resolve repoPath to absolute.
	repoPath := opts.RepoPath
	if repoPath == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return RunScoutFullResult{}, fmt.Errorf("engine.RunScoutFull: failed to get current directory: %w", err)
		}
		repoPath = cwd
	}
	absRepo, err := filepath.Abs(repoPath)
	if err != nil {
		return RunScoutFullResult{}, fmt.Errorf("engine.RunScoutFull: failed to resolve repo path: %w", err)
	}
	repoPath = absRepo

	if _, err := os.Stat(repoPath); err != nil {
		return RunScoutFullResult{}, fmt.Errorf("engine.RunScoutFull: repo path does not exist: %s", repoPath)
	}

	// Generate slug and compute IMPL path.
	slug := generateSlug(opts.Feature)
	implPath := protocol.IMPLPath(repoPath, slug)

	log.Debug("RunScoutFull: starting", "feature", opts.Feature, "slug", slug, "impl_path", implPath)

	// Check if IMPL doc already exists with advanced state — return early if so.
	if existingDoc, loadErr := protocol.Load(ctx, implPath); loadErr == nil {
		switch existingDoc.State {
		case protocol.StateReviewed,
			protocol.StateScaffoldPending,
			protocol.StateWavePending,
			protocol.StateWaveExecuting,
			protocol.StateWaveMerging,
			protocol.StateWaveVerified,
			protocol.StateComplete,
			protocol.StateScoutValidating:
			log.Info("RunScoutFull: IMPL doc already exists with advanced state; skipping Scout",
				"state", existingDoc.State, "impl_path", implPath)
			return RunScoutFullResult{
				IMPLPath: implPath,
				Slug:     slug,
			}, nil
		}
		// StateScoutPending or unknown states: proceed with Scout.
	}

	// Ensure docs/IMPL directory exists.
	implDir := filepath.Dir(implPath)
	if err := os.MkdirAll(implDir, 0755); err != nil {
		return RunScoutFullResult{}, fmt.Errorf("engine.RunScoutFull: failed to create IMPL directory: %w", err)
	}

	// Apply default timeout.
	timeoutMin := opts.Timeout
	if timeoutMin <= 0 {
		timeoutMin = 10
	}

	// Build the context with timeout.
	runCtx, cancel := context.WithTimeout(ctx, time.Duration(timeoutMin)*time.Minute)
	defer cancel()

	// Build RunScoutOpts for the correction loop.
	scoutOpts := RunScoutOpts{
		Feature:             opts.Feature,
		RepoPath:            repoPath,
		SAWRepoPath:         opts.SAWRepoPath,
		IMPLOutPath:         implPath,
		ScoutModel:          opts.ScoutModel,
		ProgramManifestPath: opts.ProgramManifestPath,
		Logger:              opts.Logger,
	}

	// Run ScoutCorrectionLoop (C9: self-healing validation).
	correctionOpts := ScoutCorrectionOpts{
		ScoutOpts:  scoutOpts,
		MaxRetries: 3,
		OnRetry: func(attempt int, errors []string) {
			if onChunk != nil {
				onChunk(fmt.Sprintf("\nIMPL validation failed (attempt %d), retrying with corrections...\n", attempt))
				for _, e := range errors {
					onChunk(fmt.Sprintf("   - %s\n", e))
				}
				onChunk("\n")
			}
		},
	}

	if corrRes := ScoutCorrectionLoop(runCtx, correctionOpts, onChunk); corrRes.IsFatal() {
		errMsg := "unknown error"
		if len(corrRes.Errors) > 0 {
			errMsg = corrRes.Errors[0].Message
		}
		return RunScoutFullResult{}, fmt.Errorf("engine.RunScoutFull: Scout execution failed: %s", errMsg)
	}

	// Wait for IMPL file to appear (race condition guard).
	if !waitForFile(implPath, 10*time.Second) {
		return RunScoutFullResult{}, fmt.Errorf("engine.RunScoutFull: IMPL doc not found at %s after Scout completion", implPath)
	}

	// Validate IMPL doc (defense-in-depth — Scout self-validates internally).
	errs, err := protocol.ValidateIMPLDoc(implPath)
	if err != nil {
		return RunScoutFullResult{}, fmt.Errorf("engine.RunScoutFull: validation system error: %w", err)
	}

	// Handle agent-ID errors with idgen.AssignAgentIDs auto-correction.
	if len(errs) > 0 {
		hasAgentIDErrors := false
		for _, e := range errs {
			if e.Code == "agent-id" {
				hasAgentIDErrors = true
				break
			}
		}

		if hasAgentIDErrors {
			agentCount := countAgentsFromErrors(errs)
			if agentCount > 0 {
				res := idgen.AssignAgentIDs(agentCount, nil)
				if res.IsFatal() {
					return RunScoutFullResult{}, fmt.Errorf("engine.RunScoutFull: failed to generate agent IDs: %s", res.Errors[0].Message)
				}
				correctIDs := res.GetData()
				log.Warn("RunScoutFull: agent ID validation errors found; manual correction required",
					"correct_ids", strings.Join(correctIDs, " "))
				return RunScoutFullResult{}, fmt.Errorf("engine.RunScoutFull: IMPL doc validation failed (agent ID errors); suggested IDs: %s",
					strings.Join(correctIDs, " "))
			}
		}

		// Non-agent-ID errors (or agent ID errors without count).
		msgs := make([]string, 0, len(errs))
		for _, e := range errs {
			msgs = append(msgs, fmt.Sprintf("line %d [%s]: %s", e.Line, e.Code, e.Message))
		}
		return RunScoutFullResult{}, fmt.Errorf("engine.RunScoutFull: IMPL doc validation failed: %s",
			strings.Join(msgs, "; "))
	}

	// Finalize IMPL doc (M4: populate verification gates).
	finalizeRes, _ := FinalizeIMPLEngine(ctx, implPath, repoPath)
	gatesPopulated := 0
	if finalizeRes.IsSuccess() {
		gatesPopulated = finalizeRes.GetData().GatePopulation.AgentsUpdated
	} else {
		log.Warn("RunScoutFull: finalize-impl completed with warnings (gates may not be populated)")
	}

	res := RunScoutFullResult{
		IMPLPath:       implPath,
		Slug:           slug,
		GatesPopulated: gatesPopulated,
	}

	// Optional: run critic gate if agent count threshold is met.
	// RunCritic is defined in pkg/engine/critic.go (Agent D's file).
	// runCriticFn is set by init() in critic.go after merge; defaults to nil (skip).
	// E37: if the critic returns verdict "ISSUES", do NOT advance to REVIEWED state.
	if !opts.NoCritic && runCriticFn != nil {
		manifest, loadErr := protocol.Load(ctx, implPath)
		if loadErr != nil {
			log.Warn("RunScoutFull: could not load IMPL manifest for critic threshold check; skipping critic gate",
				"error", loadErr)
		} else if criticThresholdMet(manifest) {
			criticOpts := runCriticOptsBuilder(implPath, opts.CriticModel, opts.SAWRepoPath, opts.Logger)
			criticRes, criticErr := runCriticFn(ctx, criticOpts, onChunk)
			if criticErr != nil {
				log.Warn("RunScoutFull: critic gate failed or not available; skipping gracefully", "error", criticErr)
			} else {
				res.CriticVerdict = criticRes.Verdict
				// E37: critic gate — ISSUES verdict blocks advance to REVIEWED state.
				if criticRes.Verdict == "ISSUES" {
					return RunScoutFullResult{
						CriticVerdict: criticRes.Verdict,
						IMPLPath:      res.IMPLPath,
					}, fmt.Errorf("E37 critic gate: verdict ISSUES — resolve critic findings before proceeding")
				}
			}
		}
	}

	return res, nil
}

// generateSlug creates a URL-safe slug from a feature description.
// Matches the slug generation logic from Scout prompt.
func generateSlug(feature string) string {
	// Convert to lowercase.
	slug := strings.ToLower(feature)

	// Replace whitespace and special chars with hyphens.
	slug = strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			return r
		}
		return '-'
	}, slug)

	// Collapse multiple hyphens.
	for strings.Contains(slug, "--") {
		slug = strings.ReplaceAll(slug, "--", "-")
	}

	// Trim leading/trailing hyphens.
	slug = strings.Trim(slug, "-")

	// Truncate to 49 chars (not 50 — off-by-one fix).
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
	// Check wave 1 agent count.
	for _, wave := range manifest.Waves {
		if wave.Number == 1 && len(wave.Agents) >= 3 {
			return true
		}
	}

	// Check if file_ownership spans 2+ distinct repos.
	repos := make(map[string]struct{})
	for _, fo := range manifest.FileOwnership {
		if fo.Repo != "" {
			repos[fo.Repo] = struct{}{}
		}
	}
	return len(repos) >= 2
}

// countAgentsFromErrors extracts the agent count from validation error messages.
// The validator appends "Run: sawtools assign-agent-ids --count N" as the last error.
func countAgentsFromErrors(errs []result.SAWError) int {
	for _, e := range errs {
		if e.Code == "agent-id" && e.Line == 0 {
			// This is the suggestion message: "Run: sawtools assign-agent-ids --count N"
			msg := e.Message
			if strings.Contains(msg, "--count") {
				// Extract number after "--count ".
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
