package engine

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/blackwell-systems/polywave-go/pkg/agent"
	"github.com/blackwell-systems/polywave-go/pkg/orchestrator"
	"github.com/blackwell-systems/polywave-go/pkg/protocol"
	"github.com/blackwell-systems/polywave-go/pkg/result"
)

// init wires RunCritic into the runCriticFn hook used by RunScoutFull in scout_run.go.
// The adapter bridges between RunCritic's result.Result[RunCriticResult] return type
// and the legacy (RunCriticResult, error) function variable type declared in scout_run.go.
// Agent K's scout_run.go migration will update runCriticFn's type to result.Result; until
// then this adapter maintains a compilable build.
func init() {
	runCriticFn = func(ctx context.Context, opts RunCriticOpts, onChunk func(string)) (RunCriticResult, error) {
		r := RunCritic(ctx, opts, onChunk)
		if r.IsFatal() {
			msgs := make([]string, len(r.Errors))
			for i, e := range r.Errors {
				msgs[i] = e.Message
			}
			return RunCriticResult{}, fmt.Errorf("run-critic failed: %v", msgs)
		}
		return r.GetData(), nil
	}
}

// BuildCriticPrompt extracts the prompt-building logic from RunCritic into a
// reusable function. It loads the IMPL doc, collects repo roots, loads
// critic-agent.md with reference injection, and returns the assembled prompt
// string. Used by --backend agent-tool to emit the prompt without spawning a
// subprocess.
func BuildCriticPrompt(ctx context.Context, opts BuildCriticPromptOpts) result.Result[string] {
	// Validate IMPL path is absolute and exists.
	if !filepath.IsAbs(opts.IMPLPath) {
		return result.NewFailure[string]([]result.PolywaveError{
			result.NewFatal(result.CodeInvalidPath,
				fmt.Sprintf("run-critic: impl-path must be absolute (got %q)", opts.IMPLPath)),
		})
	}
	if _, err := os.Stat(opts.IMPLPath); err != nil {
		return result.NewFailure[string]([]result.PolywaveError{
			result.NewFatal(result.CodeIMPLNotFound,
				fmt.Sprintf("run-critic: impl path does not exist: %s", opts.IMPLPath)),
		})
	}

	// Load the IMPL doc to collect repo roots.
	manifest, err := protocol.Load(ctx, opts.IMPLPath)
	if err != nil {
		return result.NewFailure[string]([]result.PolywaveError{
			result.NewFatal(result.CodeIMPLParseFailed,
				fmt.Sprintf("run-critic: failed to load IMPL doc: %v", err)),
		})
	}

	// Collect repo roots from the manifest; fall back to inferring from the IMPL path.
	repoPaths := collectRepoPaths(manifest)
	if len(repoPaths) == 0 {
		inferredRoot := inferRepoRoot(opts.IMPLPath)
		if inferredRoot != "" {
			repoPaths = []string{inferredRoot}
		}
	}

	// Resolve the Polywave repo path for loading critic-agent.md.
	polywaveRepo := opts.PolywaveRepoPath
	if polywaveRepo == "" {
		polywaveRepo = os.Getenv("POLYWAVE_REPO")
	}
	if polywaveRepo == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return result.NewFailure[string]([]result.PolywaveError{
				result.NewFatal(result.CodeContextError,
					fmt.Sprintf("run-critic: cannot determine home directory: %v", err)),
			})
		}
		polywaveRepo = filepath.Join(home, "code", "polywave")
	}

	// Load the critic-agent.md prompt with reference injection.
	criticMdPath := filepath.Join(polywaveRepo, "implementations", "claude-code", "prompts", "agents", "critic-agent.md")
	criticMdRes := LoadTypePromptWithRefs(criticMdPath)
	if criticMdRes.IsFatal() {
		return result.NewFailure[string]([]result.PolywaveError{
			result.NewFatal(result.CodeBriefExtractFail,
				fmt.Sprintf("run-critic: critic-agent.md not found at %s — verify SAW installation or set POLYWAVE_REPO environment variable: %v", criticMdPath, criticMdRes.Errors[0].Message)),
		})
	}
	criticMdContent := criticMdRes.GetData()

	// Build the repo-roots section for the prompt.
	repoRootsSection := ""
	for _, root := range repoPaths {
		repoRootsSection += fmt.Sprintf("- %s\n", root)
	}
	prompt := fmt.Sprintf("%s\n\n## IMPL Doc Path\n%s\n\n## Repository Root(s)\n%s",
		criticMdContent, opts.IMPLPath, repoRootsSection)
	return result.NewSuccess(prompt)
}

// RunCritic runs the critic agent end-to-end: loads the IMPL doc, discovers
// repo roots, loads the critic-agent.md prompt, launches the agent, reads the
// critic_report from the IMPL doc, and returns a structured result. The caller
// is responsible for handling --no-review / --skip before invoking this.
func RunCritic(ctx context.Context, opts RunCriticOpts, onChunk func(string)) result.Result[RunCriticResult] {
	log := loggerFrom(opts.Logger)
	_ = log

	promptResult := BuildCriticPrompt(ctx, BuildCriticPromptOpts{
		IMPLPath:    opts.IMPLPath,
		PolywaveRepoPath: opts.PolywaveRepoPath,
	})
	if promptResult.IsFatal() {
		return result.NewFailure[RunCriticResult](promptResult.Errors)
	}
	prompt := promptResult.GetData()

	// Apply context timeout (default 20 minutes).
	timeoutMinutes := opts.Timeout
	if timeoutMinutes <= 0 {
		timeoutMinutes = 20
	}
	ctx, cancel := context.WithTimeout(ctx, time.Duration(timeoutMinutes)*time.Minute)
	defer cancel()

	// Determine working directory for the agent by reloading the manifest.
	workDir := filepath.Dir(opts.IMPLPath)
	if wdManifest, wdErr := protocol.Load(ctx, opts.IMPLPath); wdErr == nil {
		wdRepoPaths := collectRepoPaths(wdManifest)
		if len(wdRepoPaths) == 0 {
			if inferredRoot := inferRepoRoot(opts.IMPLPath); inferredRoot != "" {
				wdRepoPaths = []string{inferredRoot}
			}
		}
		if len(wdRepoPaths) > 0 {
			workDir = wdRepoPaths[0]
		}
	}

	// Initialise backend and launch the critic agent.
	bRes := orchestrator.NewBackendFromModel(opts.CriticModel)
	if bRes.IsFatal() {
		return result.NewFailure[RunCriticResult](bRes.Errors)
	}
	b := bRes.GetData()
	runner := agent.NewRunner(b)
	spec := &protocol.Agent{ID: "critic", Task: prompt}
	_, execErr := runner.ExecuteStreamingWithTools(ctx, spec, workDir, onChunk, nil)
	if execErr != nil {
		return result.NewFailure[RunCriticResult]([]result.PolywaveError{
			result.NewFatal(result.CodeAgentRunFailed,
				fmt.Sprintf("run-critic: critic agent execution failed: %v", execErr)),
		})
	}

	// Reload the manifest to pick up the critic_report written by the agent.
	updatedManifest, err := protocol.Load(ctx, opts.IMPLPath)
	if err != nil {
		return result.NewFailure[RunCriticResult]([]result.PolywaveError{
			result.NewFatal(result.CodeIMPLParseFailed,
				fmt.Sprintf("run-critic: failed to reload IMPL doc after critic run: %v", err)),
		})
	}

	review := protocol.GetCriticReview(ctx, updatedManifest)
	if review == nil {
		return result.NewFailure[RunCriticResult]([]result.PolywaveError{
			result.NewFatal(result.CodeCompletionReportMissing,
				"run-critic: critic agent completed but no critic_report was written to IMPL doc"),
		})
	}

	return result.NewSuccess(RunCriticResult{
		Verdict:    review.Verdict,
		Summary:    review.Summary,
		IssueCount: review.IssueCount,
		ReviewedAt: review.ReviewedAt,
	})
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
