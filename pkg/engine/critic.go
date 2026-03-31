package engine

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/agent"
	"github.com/blackwell-systems/scout-and-wave-go/pkg/orchestrator"
	"github.com/blackwell-systems/scout-and-wave-go/pkg/protocol"
)

// init wires RunCritic into the runCriticFn hook used by RunScoutFull in scout_run.go.
func init() {
	runCriticFn = RunCritic
}

// RunCritic runs the critic agent end-to-end: loads the IMPL doc, discovers
// repo roots, loads the critic-agent.md prompt, launches the agent, reads the
// critic_report from the IMPL doc, and returns a structured result. The caller
// is responsible for handling --no-review / --skip before invoking this.
func RunCritic(ctx context.Context, opts RunCriticOpts, onChunk func(string)) (RunCriticResult, error) {
	log := loggerFrom(opts.Logger)
	_ = log

	// Validate IMPL path is absolute and exists.
	if !filepath.IsAbs(opts.IMPLPath) {
		return RunCriticResult{}, fmt.Errorf("run-critic: impl-path must be absolute (got %q)", opts.IMPLPath)
	}
	if _, err := os.Stat(opts.IMPLPath); err != nil {
		return RunCriticResult{}, fmt.Errorf("run-critic: impl path does not exist: %s", opts.IMPLPath)
	}

	// Load the IMPL doc to collect repo roots.
	manifest, err := protocol.Load(context.TODO(), opts.IMPLPath)
	if err != nil {
		return RunCriticResult{}, fmt.Errorf("run-critic: failed to load IMPL doc: %w", err)
	}

	// Collect repo roots from the manifest; fall back to inferring from the IMPL path.
	repoPaths := collectRepoPaths(manifest)
	if len(repoPaths) == 0 {
		inferredRoot := inferRepoRoot(opts.IMPLPath)
		if inferredRoot != "" {
			repoPaths = []string{inferredRoot}
		}
	}

	// Resolve the SAW repo path for loading critic-agent.md.
	sawRepo := opts.SAWRepoPath
	if sawRepo == "" {
		sawRepo = os.Getenv("SAW_REPO")
	}
	if sawRepo == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return RunCriticResult{}, fmt.Errorf("run-critic: cannot determine home directory: %w", err)
		}
		sawRepo = filepath.Join(home, "code", "scout-and-wave")
	}

	// Load the critic-agent.md prompt with reference injection.
	criticMdPath := filepath.Join(sawRepo, "implementations", "claude-code", "prompts", "agents", "critic-agent.md")
	criticMdContent, err := LoadTypePromptWithRefs(criticMdPath)
	if err != nil {
		// Fallback prompt if the file doesn't exist yet.
		criticMdContent = "You are a Critic Agent. Review every agent brief in the IMPL doc against the actual codebase. Verify file_existence, symbol_accuracy, pattern_accuracy, interface_consistency, import_chains, and side_effect_completeness. Write the result using: sawtools set-critic-review <impl-path> --verdict <PASS|ISSUES> --summary <text> --issue-count <N> --agent-reviews <JSON>"
	}

	// Build the repo-roots section for the prompt.
	repoRootsSection := ""
	for _, root := range repoPaths {
		repoRootsSection += fmt.Sprintf("- %s\n", root)
	}
	prompt := fmt.Sprintf("%s\n\n## IMPL Doc Path\n%s\n\n## Repository Root(s)\n%s",
		criticMdContent, opts.IMPLPath, repoRootsSection)

	// Apply context timeout (default 20 minutes).
	timeoutMinutes := opts.Timeout
	if timeoutMinutes <= 0 {
		timeoutMinutes = 20
	}
	ctx, cancel := context.WithTimeout(ctx, time.Duration(timeoutMinutes)*time.Minute)
	defer cancel()

	// Determine working directory for the agent.
	workDir := ""
	if len(repoPaths) > 0 {
		workDir = repoPaths[0]
	} else {
		workDir = filepath.Dir(opts.IMPLPath)
	}

	// Initialise backend and launch the critic agent.
	b, bErr := orchestrator.NewBackendFromModel(opts.CriticModel)
	if bErr != nil {
		return RunCriticResult{}, fmt.Errorf("run-critic: backend init: %w", bErr)
	}
	runner := agent.NewRunner(b, nil)
	spec := &protocol.Agent{ID: "critic", Task: prompt}
	_, execErr := runner.ExecuteStreamingWithTools(ctx, spec, workDir, onChunk, nil)
	if execErr != nil {
		return RunCriticResult{}, fmt.Errorf("run-critic: critic agent execution failed: %w", execErr)
	}

	// Reload the manifest to pick up the critic_report written by the agent.
	updatedManifest, err := protocol.Load(context.TODO(), opts.IMPLPath)
	if err != nil {
		return RunCriticResult{}, fmt.Errorf("run-critic: failed to reload IMPL doc after critic run: %w", err)
	}

	review := protocol.GetCriticReview(ctx, updatedManifest)
	if review == nil {
		return RunCriticResult{}, fmt.Errorf("run-critic: critic agent completed but no critic_report was written to IMPL doc")
	}

	return RunCriticResult{
		Verdict:    review.Verdict,
		Summary:    review.Summary,
		IssueCount: review.IssueCount,
		ReviewedAt: review.ReviewedAt,
	}, nil
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
