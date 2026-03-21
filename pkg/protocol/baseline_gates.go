package protocol

import "github.com/blackwell-systems/scout-and-wave-go/pkg/gatecache"

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
