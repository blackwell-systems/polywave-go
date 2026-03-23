package protocol

import (
	"bytes"
	"fmt"
	"os/exec"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/gatecache"
	"github.com/blackwell-systems/scout-and-wave-go/pkg/result"
)

// BaselineData captures the outcome of baseline gate verification (E21A).
type BaselineData struct {
	Passed      bool         `json:"passed"`
	GateResults []GateResult `json:"gate_results"`
	Reason      string       `json:"reason,omitempty"`     // "baseline_verification_failed" on failure
	CommitSHA   string       `json:"commit_sha,omitempty"` // HEAD at time of check
	FromCache   bool         `json:"from_cache"`           // true if all results served from cache
}

// BaselineResult is a backward-compatible alias for BaselineData.
// Deprecated: Use BaselineData directly.
type BaselineResult = BaselineData

// RunBaselineGates executes quality gates against the current codebase
// (before worktree creation) to verify the baseline is healthy.
// If cache is non-nil, results are cached keyed by HEAD commit.
// The waveNumber is used for docs-only wave detection (skip source gates).
func RunBaselineGates(manifest *IMPLManifest, waveNumber int, repoDir string, cache *gatecache.Cache) result.Result[*BaselineData] {
	data := &BaselineData{
		Passed: true,
	}

	// Step 1: If cache is non-nil, get HEAD SHA for CommitSHA field.
	// BuildKey failure is non-fatal — cache is best-effort.
	if cache != nil {
		if key, err := cache.BuildKey(repoDir); err == nil {
			data.CommitSHA = key.HeadCommit
		}
	}

	// Step 2: Delegate gate execution entirely to RunPreMergeGates.
	// RunPreMergeGates internally calls RunGatesWithCache which handles
	// cache lookup/store.
	res := RunPreMergeGates(manifest, waveNumber, repoDir, cache)
	if !res.IsSuccess() {
		return result.NewFailure[*BaselineData]([]result.StructuredError{{
			Code:     "E_BASELINE",
			Message:  fmt.Sprintf("pre-merge gates failed: %v", res.Errors),
			Severity: "fatal",
		}})
	}
	data.GateResults = res.GetData().Gates

	// Step 3: Check if any required gate failed.
	for _, gr := range data.GateResults {
		if gr.Required && !gr.Passed && !gr.Skipped {
			data.Passed = false
			data.Reason = "baseline_verification_failed"
			break
		}
	}

	// Step 4: If all results have FromCache=true (and there are results),
	// set BaselineData.FromCache=true.
	if len(data.GateResults) > 0 {
		allFromCache := true
		for _, gr := range data.GateResults {
			if !gr.FromCache {
				allFromCache = false
				break
			}
		}
		data.FromCache = allFromCache
	}

	return result.NewSuccess(data)
}

// CrossRepoBaselineData captures per-repo baseline gate results for E21B.
type CrossRepoBaselineData struct {
	Passed  bool                       `json:"passed"`
	Results map[string]*BaselineData `json:"results"` // repo name → result
}

// CrossRepoBaselineResult is a backward-compatible alias for CrossRepoBaselineData.
// Deprecated: Use CrossRepoBaselineData directly.
type CrossRepoBaselineResult = CrossRepoBaselineData

// RunCrossRepoBaselineGates runs baseline gates on all repos targeted by a
// cross-repo IMPL manifest (E21B). For each repo, it uses per-repo gate
// commands from configRepos if available, falling back to the IMPL's
// quality_gates. Returns early on first failure for fast feedback.
func RunCrossRepoBaselineGates(manifest *IMPLManifest, waveNumber int, targetRepos map[string]string, configRepos []RepoEntry) result.Result[*CrossRepoBaselineData] {
	// Build config lookup for per-repo commands
	configLookup := make(map[string]RepoEntry, len(configRepos))
	for _, r := range configRepos {
		configLookup[r.Name] = r
	}

	data := &CrossRepoBaselineData{
		Passed:  true,
		Results: make(map[string]*BaselineData),
	}

	for repoName, repoPath := range targetRepos {
		entry := configLookup[repoName]

		// If repo has explicit build/test commands, run those directly
		if entry.BuildCommand != "" || entry.TestCommand != "" {
			repoData := &BaselineData{Passed: true}
			if entry.BuildCommand != "" {
				gr := runSimpleGate("build", entry.BuildCommand, repoPath)
				repoData.GateResults = append(repoData.GateResults, gr)
				if !gr.Passed {
					repoData.Passed = false
					repoData.Reason = fmt.Sprintf("cross_repo_baseline_failed: %s", repoName)
				}
			}
			if entry.TestCommand != "" && repoData.Passed {
				gr := runSimpleGate("test", entry.TestCommand, repoPath)
				repoData.GateResults = append(repoData.GateResults, gr)
				if !gr.Passed {
					repoData.Passed = false
					repoData.Reason = fmt.Sprintf("cross_repo_baseline_failed: %s", repoName)
				}
			}
			data.Results[repoName] = repoData
			if !repoData.Passed {
				data.Passed = false
				return result.NewSuccess(data)
			}
			continue
		}

		// Fall back to IMPL quality_gates run in this repo's directory
		repoRes := RunBaselineGates(manifest, waveNumber, repoPath, nil)
		if repoRes.IsFatal() {
			return result.NewFailure[*CrossRepoBaselineData]([]result.StructuredError{{
				Code:     "E_BASELINE",
				Message:  fmt.Sprintf("E21B: baseline gates for repo %s failed: %v", repoName, repoRes.Errors),
				Severity: "fatal",
			}})
		}
		repoData := repoRes.GetData()
		data.Results[repoName] = repoData
		if !repoData.Passed {
			data.Passed = false
			return result.NewSuccess(data)
		}
	}

	return result.NewSuccess(data)
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
