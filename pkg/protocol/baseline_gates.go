package protocol

import (
	"bytes"
	"fmt"
	"os/exec"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/gatecache"
)

// BaselineResult captures the outcome of baseline gate verification (E21A).
type BaselineResult struct {
	Passed      bool        `json:"passed"`
	GateResults []GateResult `json:"gate_results"`
	Reason      string      `json:"reason,omitempty"`     // "baseline_verification_failed" on failure
	CommitSHA   string      `json:"commit_sha,omitempty"` // HEAD at time of check
	FromCache   bool        `json:"from_cache"`           // true if all results served from cache
}

// RunBaselineGates executes quality gates against the current codebase
// (before worktree creation) to verify the baseline is healthy.
// If cache is non-nil, results are cached keyed by HEAD commit.
// The waveNumber is used for docs-only wave detection (skip source gates).
func RunBaselineGates(manifest *IMPLManifest, waveNumber int, repoDir string, cache *gatecache.Cache) (*BaselineResult, error) {
	result := &BaselineResult{
		Passed: true,
	}

	// Step 1: If cache is non-nil, get HEAD SHA for CommitSHA field.
	// BuildKey failure is non-fatal — cache is best-effort.
	if cache != nil {
		if key, err := cache.BuildKey(repoDir); err == nil {
			result.CommitSHA = key.HeadCommit
		}
	}

	// Step 2: Delegate gate execution entirely to RunPreMergeGates.
	// RunPreMergeGates internally calls RunGatesWithCache which handles
	// cache lookup/store.
	gateResults, err := RunPreMergeGates(manifest, waveNumber, repoDir, cache)
	if err != nil {
		return nil, err
	}
	result.GateResults = gateResults

	// Step 3: Check if any required gate failed.
	for _, gr := range gateResults {
		if gr.Required && !gr.Passed && !gr.Skipped {
			result.Passed = false
			result.Reason = "baseline_verification_failed"
			break
		}
	}

	// Step 4: If all results have FromCache=true (and there are results),
	// set BaselineResult.FromCache=true.
	if len(gateResults) > 0 {
		allFromCache := true
		for _, gr := range gateResults {
			if !gr.FromCache {
				allFromCache = false
				break
			}
		}
		result.FromCache = allFromCache
	}

	return result, nil
}

// CrossRepoBaselineResult captures per-repo baseline gate results for E21B.
type CrossRepoBaselineResult struct {
	Passed  bool                       `json:"passed"`
	Results map[string]*BaselineResult `json:"results"` // repo name → result
}

// RunCrossRepoBaselineGates runs baseline gates on all repos targeted by a
// cross-repo IMPL manifest (E21B). For each repo, it uses per-repo gate
// commands from configRepos if available, falling back to the IMPL's
// quality_gates. Returns early on first failure for fast feedback.
func RunCrossRepoBaselineGates(manifest *IMPLManifest, waveNumber int, targetRepos map[string]string, configRepos []RepoEntry) (*CrossRepoBaselineResult, error) {
	// Build config lookup for per-repo commands
	configLookup := make(map[string]RepoEntry, len(configRepos))
	for _, r := range configRepos {
		configLookup[r.Name] = r
	}

	result := &CrossRepoBaselineResult{
		Passed:  true,
		Results: make(map[string]*BaselineResult),
	}

	for repoName, repoPath := range targetRepos {
		entry := configLookup[repoName]

		// If repo has explicit build/test commands, run those directly
		if entry.BuildCommand != "" || entry.TestCommand != "" {
			repoResult := &BaselineResult{Passed: true}
			if entry.BuildCommand != "" {
				gr := runSimpleGate("build", entry.BuildCommand, repoPath)
				repoResult.GateResults = append(repoResult.GateResults, gr)
				if !gr.Passed {
					repoResult.Passed = false
					repoResult.Reason = fmt.Sprintf("cross_repo_baseline_failed: %s", repoName)
				}
			}
			if entry.TestCommand != "" && repoResult.Passed {
				gr := runSimpleGate("test", entry.TestCommand, repoPath)
				repoResult.GateResults = append(repoResult.GateResults, gr)
				if !gr.Passed {
					repoResult.Passed = false
					repoResult.Reason = fmt.Sprintf("cross_repo_baseline_failed: %s", repoName)
				}
			}
			result.Results[repoName] = repoResult
			if !repoResult.Passed {
				result.Passed = false
				return result, nil
			}
			continue
		}

		// Fall back to IMPL quality_gates run in this repo's directory
		repoResult, err := RunBaselineGates(manifest, waveNumber, repoPath, nil)
		if err != nil {
			return nil, fmt.Errorf("E21B: baseline gates for repo %s: %w", repoName, err)
		}
		result.Results[repoName] = repoResult
		if !repoResult.Passed {
			result.Passed = false
			return result, nil
		}
	}

	return result, nil
}

// runSimpleGate executes a single shell command as a gate check.
func runSimpleGate(gateType, command, dir string) GateResult {
	gr := GateResult{
		Type:     gateType,
		Command:  command,
		Required: true,
	}
	cmd := exec.Command("sh", "-c", command)
	cmd.Dir = dir
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	gr.Stdout = stdout.String()
	gr.Stderr = stderr.String()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			gr.ExitCode = exitErr.ExitCode()
		} else {
			gr.ExitCode = 1
		}
		gr.Passed = false
	} else {
		gr.Passed = true
	}
	return gr
}
