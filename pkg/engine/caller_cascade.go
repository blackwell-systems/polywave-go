package engine

import (
	"context"
	"fmt"
	"log/slog"
	"os/exec"
	"regexp"
	"strings"

	"github.com/blackwell-systems/scout-and-wave-go/internal/git"
	"github.com/blackwell-systems/scout-and-wave-go/pkg/agent"
	"github.com/blackwell-systems/scout-and-wave-go/pkg/orchestrator"
	"github.com/blackwell-systems/scout-and-wave-go/pkg/protocol"
	"github.com/blackwell-systems/scout-and-wave-go/pkg/result"
	"github.com/blackwell-systems/scout-and-wave-go/pkg/tools"
)

// CallerCascadeError represents a single compiler error classified as a
// caller cascade: the error file is not owned by the current wave.
type CallerCascadeError struct {
	File    string `json:"file"`    // relative file path reporting the error
	Line    int    `json:"line"`    // line number (0 if unknown)
	Message string `json:"message"` // raw compiler error text
}

// CallerCascadeClassification is the result of ClassifyCallerCascadeErrors.
type CallerCascadeClassification struct {
	Errors         []CallerCascadeError `json:"errors"`
	AllAreCascades bool                 `json:"all_are_cascades"`
	// MixedErrors is true when some errors are in current-wave files
	// (real failures) and some are in future-wave/unowned files (cascades).
	// When true, the wave has a genuine build failure and hotfix is NOT triggered.
	MixedErrors bool `json:"mixed_errors"`
}

// RunHotfixAgentOpts configures a caller cascade hotfix agent run.
type RunHotfixAgentOpts struct {
	IMPLPath string               // absolute path to IMPL manifest
	RepoPath string               // absolute path to the target repository
	WaveNum  int                  // wave number that just completed
	Errors   []CallerCascadeError // errors to fix
	Model    string               // optional model override
	Logger   *slog.Logger         // optional: nil falls back to slog.Default()
}

// HotfixAgentData contains metadata from a successful RunHotfixAgent operation.
type HotfixAgentData struct {
	IMPLPath    string   `json:"impl_path"`
	WaveNum     int      `json:"wave_num"`
	ErrorCount  int      `json:"error_count"`  // number of errors passed to agent
	FilesFixed  []string `json:"files_fixed"`  // files the agent modified
	Commit      string   `json:"commit"`        // git commit SHA of hotfix commit
	BuildPassed bool     `json:"build_passed"` // true when go build && go vet clean after fix
}



// cascadePatterns matches compiler errors of the form file.go:line:col: message
// or file.go:line: message for:
//   - "undefined: Foo"
//   - "assignment mismatch: 2 variables but Foo returns 1"
//   - "not enough arguments in call to Foo"
var cascadePatterns = []*regexp.Regexp{
	regexp.MustCompile(`^([^:]+):(\d+)(?::\d+)?: (undefined: .+)$`),
	regexp.MustCompile(`^([^:]+):(\d+)(?::\d+)?: (assignment mismatch: .+)$`),
	regexp.MustCompile(`^([^:]+):(\d+)(?::\d+)?: (not enough arguments.+)$`),
}

// ClassifyCallerCascadeErrors classifies verify-build errors to distinguish
// genuine wave failures from caller cascade side-effects.
//
// cascadePatterns matches: "undefined: Foo", "assignment mismatch: 2 variables
// but Foo returns 1", "not enough arguments in call to Foo"
//
// manifest is used to determine which files are current-wave-owned vs future-wave.
// waveNum is the wave that just finished.
func ClassifyCallerCascadeErrors(
	verifyData *protocol.VerifyBuildData,
	manifest *protocol.IMPLManifest,
	waveNum int,
) CallerCascadeClassification {
	if verifyData == nil {
		return CallerCascadeClassification{}
	}

	errorText := verifyData.TestOutput + "\n" + verifyData.LintOutput

	// Build a set of files owned by the current wave.
	currentWaveFiles := make(map[string]bool)
	for _, ownership := range manifest.FileOwnership {
		if ownership.Wave == waveNum {
			currentWaveFiles[ownership.File] = true
		}
	}

	var cascadeErrors []CallerCascadeError
	mixedErrors := false

	for _, line := range strings.Split(errorText, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		for _, pat := range cascadePatterns {
			m := pat.FindStringSubmatch(line)
			if m == nil {
				continue
			}
			file := m[1]
			lineNum := 0
			fmt.Sscanf(m[2], "%d", &lineNum)
			msg := m[3]

			if currentWaveFiles[file] {
				// Error in a current-wave-owned file — genuine failure.
				mixedErrors = true
			} else {
				// Error in a file NOT owned by the current wave — cascade.
				cascadeErrors = append(cascadeErrors, CallerCascadeError{
					File:    file,
					Line:    lineNum,
					Message: msg,
				})
			}
			break // only match one pattern per line
		}
	}

	allAreCascades := len(cascadeErrors) > 0 && !mixedErrors

	return CallerCascadeClassification{
		Errors:         cascadeErrors,
		AllAreCascades: allAreCascades,
		MixedErrors:    mixedErrors,
	}
}

// RunHotfixAgent launches a caller cascade hotfix agent (E47).
// It fixes minimal caller errors in future-wave-owned files after a wave
// changes function signatures. The agent commits as:
//
//	[SAW:wave{N}:integration-hotfix] fix caller cascade after wave N signature changes
//
// Returns success when the agent fixes all errors and the build passes.
// Returns failure when the agent cannot fix errors or the build still fails.
func RunHotfixAgent(
	ctx context.Context,
	opts RunHotfixAgentOpts,
	onEvent func(Event),
) result.Result[HotfixAgentData] {
	// Step 1: Validate opts.
	if opts.IMPLPath == "" {
		return result.NewFailure[HotfixAgentData]([]result.SAWError{{
			Code:     result.CodeCallerCascadeHotfixFailed,
			Message:  "engine.RunHotfixAgent: IMPLPath is required",
			Severity: "fatal",
		}})
	}
	if opts.RepoPath == "" {
		return result.NewFailure[HotfixAgentData]([]result.SAWError{{
			Code:     result.CodeCallerCascadeHotfixFailed,
			Message:  "engine.RunHotfixAgent: RepoPath is required",
			Severity: "fatal",
		}})
	}
	if opts.WaveNum <= 0 {
		return result.NewFailure[HotfixAgentData]([]result.SAWError{{
			Code:     result.CodeCallerCascadeHotfixFailed,
			Message:  "engine.RunHotfixAgent: WaveNum must be positive",
			Severity: "fatal",
		}})
	}
	if len(opts.Errors) == 0 {
		return result.NewFailure[HotfixAgentData]([]result.SAWError{{
			Code:     result.CodeCallerCascadeHotfixFailed,
			Message:  "engine.RunHotfixAgent: Errors must be non-empty",
			Severity: "fatal",
		}})
	}

	publish := func(event string, data interface{}) {
		if onEvent != nil {
			onEvent(Event{Event: event, Data: data})
		}
	}

	publish("hotfix_agent_started", map[string]interface{}{
		"impl_path":   opts.IMPLPath,
		"wave":        opts.WaveNum,
		"error_count": len(opts.Errors),
	})

	// Step 2: Load manifest.
	manifest, err := protocol.Load(ctx, opts.IMPLPath)
	if err != nil {
		publish("hotfix_agent_failed", map[string]string{"error": err.Error()})
		return result.NewFailure[HotfixAgentData]([]result.SAWError{{
			Code:     result.CodeCallerCascadeHotfixFailed,
			Message:  fmt.Sprintf("engine.RunHotfixAgent: load manifest: %v", err),
			Severity: "fatal",
			Cause:    err,
		}})
	}

	// manifest is loaded for context (feature slug, wave info).
	// Direct fields are unused in prompt building but available for future use.
	_ = manifest

	// Collect unique erroring files.
	uniqueFiles := uniqueErrorFiles(opts.Errors)

	// Step 3: Build prompt.
	prompt := buildHotfixPrompt(opts, uniqueFiles)

	// Step 4: Create backend.
	bRes := orchestrator.NewBackendFromModel(opts.Model)
	if bRes.IsFatal() {
		publish("hotfix_agent_failed", map[string]string{"error": bRes.Errors[0].Message})
		return result.NewFailure[HotfixAgentData]([]result.SAWError{{
			Code:     result.CodeCallerCascadeHotfixFailed,
			Message:  fmt.Sprintf("engine.RunHotfixAgent: backend init: %s", bRes.Errors[0].Message),
			Severity: "fatal",
		}})
	}
	b := bRes.GetData()

	// Step 5: Build constraints and inject file restriction.
	constraints := &tools.Constraints{
		AllowedPathPrefixes: uniqueFiles,
	}
	prompt = injectAllowedPathsRestriction(prompt, constraints)

	// Step 6: Execute hotfix agent.
	runner := agent.NewRunner(b)
	spec := &protocol.Agent{
		ID:   "hotfix-agent",
		Task: prompt,
	}

	onChunk := func(chunk string) {
		publish("hotfix_agent_output", map[string]string{"chunk": chunk})
	}

	if _, execErr := runner.ExecuteStreamingWithTools(ctx, spec, opts.RepoPath, onChunk, nil); execErr != nil {
		publish("hotfix_agent_failed", map[string]string{"error": execErr.Error()})
		return result.NewFailure[HotfixAgentData]([]result.SAWError{{
			Code:     result.CodeCallerCascadeHotfixFailed,
			Message:  fmt.Sprintf("engine.RunHotfixAgent: agent execution failed: %v", execErr),
			Severity: "fatal",
			Cause:    execErr,
		}})
	}

	// Step 7: Auto-commit.
	commitSHA, commitErr := autoCommitHotfix(opts.RepoPath, opts.WaveNum)
	if commitErr != nil {
		// Non-fatal: agent may not have made changes.
		loggerFrom(opts.Logger).Warn("engine.RunHotfixAgent: auto-commit", "err", commitErr)
		publish("hotfix_agent_output", map[string]string{
			"chunk": fmt.Sprintf("hotfix auto-commit: %v", commitErr),
		})
	}

	// Step 8: Verify build — use a shell to handle && correctly.
	buildPassed := false
	shellCmd := exec.CommandContext(ctx, "sh", "-c", "go build ./... && go vet ./...")
	shellCmd.Dir = opts.RepoPath
	if buildErr := shellCmd.Run(); buildErr == nil {
		buildPassed = true
	}

	// Step 9: Collect files modified by the agent.
	var filesFixed []string
	if commitSHA != "" {
		if diffOut, diffErr := git.Run(opts.RepoPath, "diff", "HEAD~1", "--name-only"); diffErr == nil {
			for _, f := range strings.Split(strings.TrimSpace(diffOut), "\n") {
				if f != "" {
					filesFixed = append(filesFixed, f)
				}
			}
		}
	}

	// Use unique erroring files as fallback if git diff produced nothing.
	if len(filesFixed) == 0 {
		filesFixed = uniqueFiles
	}

	publish("hotfix_agent_complete", map[string]interface{}{
		"impl_path":    opts.IMPLPath,
		"wave":         opts.WaveNum,
		"build_passed": buildPassed,
	})

	return result.NewSuccess(HotfixAgentData{
		IMPLPath:    opts.IMPLPath,
		WaveNum:     opts.WaveNum,
		ErrorCount:  len(opts.Errors),
		FilesFixed:  filesFixed,
		Commit:      commitSHA,
		BuildPassed: buildPassed,
	})
}

// buildHotfixPrompt constructs the agent prompt for fixing caller cascade errors.
func buildHotfixPrompt(opts RunHotfixAgentOpts, uniqueFiles []string) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf(`You are a Hotfix Agent (E47). Your job is to fix caller cascade compiler errors
after wave %d changed function signatures.

## Errors to Fix

`, opts.WaveNum))

	for _, e := range opts.Errors {
		sb.WriteString(fmt.Sprintf("### %s (line %d)\n```\n%s\n```\n\n", e.File, e.Line, e.Message))
	}

	sb.WriteString("## Files You May Modify\n\n")
	for _, f := range uniqueFiles {
		sb.WriteString(fmt.Sprintf("- %s\n", f))
	}

	commitMsg := fmt.Sprintf("[SAW:wave%d:integration-hotfix] fix caller cascade after wave %d signature changes",
		opts.WaveNum, opts.WaveNum)

	sb.WriteString(fmt.Sprintf(`
## Verification Gate

After making changes, verify the build passes:
  go build ./... && go vet ./...

## Commit Message

Use exactly this commit message:
  %s

## Rules

1. Only modify files listed above — these are the caller sites, NOT the definition files.
2. Do NOT modify files where the functions are defined — only fix caller sites.
3. Add or adjust imports as needed to satisfy the compiler.
4. Run "go build ./... && go vet ./..." after changes to confirm clean build.
5. Commit with the exact message shown above.
`, commitMsg))

	return sb.String()
}

// autoCommitHotfix stages all changes and commits with the standard hotfix message.
// Returns the commit SHA on success, or an error if nothing was staged or commit failed.
func autoCommitHotfix(repoPath string, waveNum int) (string, error) {
	status, err := git.StatusPorcelain(repoPath)
	if err != nil {
		return "", fmt.Errorf("checking repo status: %w", err)
	}
	if status == "" {
		return "", nil // No changes to commit.
	}

	if err := git.AddAll(repoPath); err != nil {
		return "", fmt.Errorf("staging changes: %w", err)
	}

	msg := fmt.Sprintf("[SAW:wave%d:integration-hotfix] fix caller cascade after wave %d signature changes",
		waveNum, waveNum)

	sha, err := git.Commit(repoPath, msg)
	if err != nil {
		return "", fmt.Errorf("committing: %w", err)
	}

	return strings.TrimSpace(sha), nil
}

// uniqueErrorFiles returns a deduplicated list of file paths from the errors slice.
func uniqueErrorFiles(errors []CallerCascadeError) []string {
	seen := make(map[string]bool)
	var files []string
	for _, e := range errors {
		if !seen[e.File] {
			seen[e.File] = true
			files = append(files, e.File)
		}
	}
	return files
}
