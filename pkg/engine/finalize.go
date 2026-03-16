package engine

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/builddiag"
	"github.com/blackwell-systems/scout-and-wave-go/pkg/protocol"
)

// FinalizeWaveOpts configures a full post-agent finalization pipeline.
type FinalizeWaveOpts struct {
	IMPLPath string // absolute path to IMPL manifest
	RepoPath string // absolute path to the target repository
	WaveNum  int    // wave number to finalize
}

// FinalizeWaveResult combines all post-agent verification, merge, and cleanup results.
type FinalizeWaveResult struct {
	Wave           int                        `json:"wave"`
	VerifyCommits  *protocol.VerifyCommitsResult `json:"verify_commits,omitempty"`
	StubReport     *protocol.ScanStubsResult  `json:"stub_report,omitempty"`
	GateResults    []protocol.GateResult      `json:"gate_results,omitempty"`
	MergeResult    *protocol.MergeAgentsResult `json:"merge_result,omitempty"`
	VerifyBuild    *protocol.VerifyBuildResult `json:"verify_build,omitempty"`
	BuildDiagnosis *builddiag.Diagnosis        `json:"build_diagnosis,omitempty"`
	CleanupResult  *protocol.CleanupResult    `json:"cleanup_result,omitempty"`
	BuildPassed    bool                       `json:"build_passed"`
	Success        bool                       `json:"success"`
}

// FinalizeWave runs the full post-agent finalization pipeline for a single wave.
// This is the engine-level equivalent of the CLI's `sawtools finalize-wave` command.
//
// Pipeline:
//  1. VerifyCommits (I5) - ensure all agents committed work
//  2. ScanStubs (E20) - scan changed files for TODO/FIXME markers
//  3. RunGates (E21) - execute quality gates
//  4. MergeAgents - merge agent branches into main
//  5. VerifyBuild - run test_command and lint_command
//  6. Cleanup - remove worktrees and branches
//
// Stops on first fatal failure and returns partial result.
func FinalizeWave(ctx context.Context, opts FinalizeWaveOpts) (*FinalizeWaveResult, error) {
	if opts.IMPLPath == "" {
		return nil, fmt.Errorf("engine.FinalizeWave: IMPLPath is required")
	}
	if opts.RepoPath == "" {
		return nil, fmt.Errorf("engine.FinalizeWave: RepoPath is required")
	}

	manifest, err := protocol.Load(opts.IMPLPath)
	if err != nil {
		return nil, fmt.Errorf("engine.FinalizeWave: load manifest: %w", err)
	}

	result := &FinalizeWaveResult{
		Wave: opts.WaveNum,
	}

	// Step 1: VerifyCommits (I5)
	verifyResult, err := protocol.VerifyCommits(opts.IMPLPath, opts.WaveNum, opts.RepoPath)
	if err != nil {
		return result, fmt.Errorf("engine.FinalizeWave: verify-commits: %w", err)
	}
	result.VerifyCommits = verifyResult
	if !verifyResult.AllValid {
		return result, fmt.Errorf("engine.FinalizeWave: verify-commits found agents with no commits")
	}

	// Step 2: ScanStubs (E20) - informational, does not block
	var changedFiles []string
	if opts.WaveNum > 0 && opts.WaveNum <= len(manifest.Waves) {
		for _, agent := range manifest.Waves[opts.WaveNum-1].Agents {
			if report, ok := manifest.CompletionReports[agent.ID]; ok {
				changedFiles = append(changedFiles, report.FilesChanged...)
				changedFiles = append(changedFiles, report.FilesCreated...)
			}
		}
	}
	if len(changedFiles) > 0 {
		stubResult, err := protocol.ScanStubs(changedFiles)
		if err != nil {
			// Non-fatal: log but continue
			fmt.Fprintf(os.Stderr, "engine.FinalizeWave: scan-stubs: %v\n", err)
		} else {
			result.StubReport = stubResult
		}
	}

	// Step 3: RunGates (E21)
	gateResults, err := protocol.RunGates(manifest, opts.WaveNum, opts.RepoPath)
	if err != nil {
		return result, fmt.Errorf("engine.FinalizeWave: run-gates: %w", err)
	}
	result.GateResults = gateResults
	for _, gate := range gateResults {
		if gate.Required && !gate.Passed {
			return result, fmt.Errorf("engine.FinalizeWave: required gate %q failed", gate.Type)
		}
	}

	// Step 4: MergeAgents
	mergeResult, err := protocol.MergeAgents(opts.IMPLPath, opts.WaveNum, opts.RepoPath)
	if err != nil {
		return result, fmt.Errorf("engine.FinalizeWave: merge-agents: %w", err)
	}
	result.MergeResult = mergeResult
	if !mergeResult.Success {
		return result, fmt.Errorf("engine.FinalizeWave: merge-agents encountered conflicts")
	}

	// Step 5: VerifyBuild
	verifyBuildResult, err := protocol.VerifyBuild(opts.IMPLPath, opts.RepoPath)
	if err != nil {
		return result, fmt.Errorf("engine.FinalizeWave: verify-build: %w", err)
	}
	result.VerifyBuild = verifyBuildResult
	result.BuildPassed = verifyBuildResult.TestPassed && verifyBuildResult.LintPassed

	if !result.BuildPassed {
		// Auto-diagnose build failure using H7 pattern matching
		language := inferLanguageFromTestCommand(manifest.TestCommand)
		if language != "" {
			errorLog := verifyBuildResult.TestOutput + "\n" + verifyBuildResult.LintOutput
			diagnosis, diagErr := builddiag.DiagnoseError(errorLog, language)
			if diagErr == nil {
				result.BuildDiagnosis = diagnosis
			}
		}
		return result, fmt.Errorf("engine.FinalizeWave: verify-build failed (test_passed=%v, lint_passed=%v)",
			verifyBuildResult.TestPassed, verifyBuildResult.LintPassed)
	}

	// Step 6: Cleanup
	cleanupResult, err := protocol.Cleanup(opts.IMPLPath, opts.WaveNum, opts.RepoPath)
	if err != nil {
		// Non-fatal: cleanup failure shouldn't fail the wave
		fmt.Fprintf(os.Stderr, "engine.FinalizeWave: cleanup: %v\n", err)
	} else {
		result.CleanupResult = cleanupResult
	}

	result.Success = true
	return result, nil
}

// MarkIMPLCompleteOpts configures IMPL completion marking.
type MarkIMPLCompleteOpts struct {
	IMPLPath string // absolute path to IMPL manifest
	RepoPath string // absolute path to the target repository
	Date     string // completion date in YYYY-MM-DD format
}

// MarkIMPLComplete writes the completion marker (E15), updates project context (E18),
// and archives the IMPL doc to docs/IMPL/complete/.
func MarkIMPLComplete(ctx context.Context, opts MarkIMPLCompleteOpts) error {
	if opts.IMPLPath == "" {
		return fmt.Errorf("engine.MarkIMPLComplete: IMPLPath is required")
	}

	date := opts.Date
	if date == "" {
		date = time.Now().Format("2006-01-02")
	}

	// E15: Write completion marker
	if err := protocol.WriteCompletionMarker(opts.IMPLPath, date); err != nil {
		return fmt.Errorf("engine.MarkIMPLComplete: write marker: %w", err)
	}

	// E18: Update project context
	if opts.RepoPath != "" {
		if _, err := protocol.UpdateContext(opts.IMPLPath, opts.RepoPath); err != nil {
			// Non-fatal: log but continue to archive
			fmt.Fprintf(os.Stderr, "engine.MarkIMPLComplete: update-context: %v\n", err)
		}
	}

	// Archive: move IMPL from docs/IMPL/ to docs/IMPL/complete/
	if _, err := protocol.ArchiveIMPL(opts.IMPLPath); err != nil {
		return fmt.Errorf("engine.MarkIMPLComplete: archive: %w", err)
	}

	return nil
}

// inferLanguageFromTestCommand infers the programming language from a test command string.
func inferLanguageFromTestCommand(testCommand string) string {
	if testCommand == "" {
		return ""
	}
	// Reuse the same heuristics as the CLI
	switch {
	case contains(testCommand, "cargo test", "cargo build"):
		return "rust"
	case contains(testCommand, "go test", "go build"):
		return "go"
	case contains(testCommand, "npm test", "jest", "vitest", "mocha"):
		return "javascript"
	case contains(testCommand, "pytest", "python -m unittest"):
		return "python"
	default:
		return ""
	}
}

// contains checks if s contains any of the substrings.
func contains(s string, substrs ...string) bool {
	for _, sub := range substrs {
		if len(s) >= len(sub) {
			// Case-insensitive check
			for i := 0; i <= len(s)-len(sub); i++ {
				match := true
				for j := 0; j < len(sub); j++ {
					sc := s[i+j]
					tc := sub[j]
					if sc >= 'A' && sc <= 'Z' {
						sc += 32
					}
					if tc >= 'A' && tc <= 'Z' {
						tc += 32
					}
					if sc != tc {
						match = false
						break
					}
				}
				if match {
					return true
				}
			}
		}
	}
	return false
}
