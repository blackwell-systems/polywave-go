package engine

import (
	"context"
	"fmt"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/protocol"
	"github.com/blackwell-systems/scout-and-wave-go/pkg/result"
)

// CleanupData holds data returned by PostMergeCleanup.
type CleanupData struct {
	GoModFixed bool `json:"gomod_fixed"`
	Cleaned    bool `json:"cleaned"`
}

// PostMergeCleanup runs post-merge processing: go.mod replace path fixup
// followed by worktree/branch cleanup. Both steps are non-fatal — errors
// are reported through the onEvent callback but do not cause the function
// to return a fatal result. This consolidates identical cleanup blocks previously
// duplicated across service.MergeWave, api.handleWaveMerge, and
// api.handleResolveConflicts.
func PostMergeCleanup(ctx context.Context, implPath string, waveNum int, repoPath string, onEvent EventCallback) result.Result[CleanupData] {
	data := CleanupData{}
	var warnings []result.SAWError

	// Step 1: Fix go.mod replace paths (deep relative paths from worktree agents).
	if onEvent != nil {
		onEvent("gomod_fixup", "running", "Checking go.mod replace paths")
	}
	fixed, err := protocol.FixGoModReplacePaths(repoPath)
	if err != nil {
		if onEvent != nil {
			onEvent("gomod_fixup", "error", fmt.Sprintf("go.mod fixup failed (non-fatal): %v", err))
		}
		warnings = append(warnings, result.NewWarning(result.CodeGomodFixupFailed,
			fmt.Sprintf("go.mod fixup failed: %v", err)).
			WithContext("repo_path", repoPath))
	} else {
		data.GoModFixed = fixed
		if fixed {
			if onEvent != nil {
				onEvent("gomod_fixup", "success", "Fixed deep replace paths in go.mod")
			}
		} else {
			if onEvent != nil {
				onEvent("gomod_fixup", "success", "No go.mod fixes needed")
			}
		}
	}

	// Step 2: Clean up worktrees and branches for the wave.
	if onEvent != nil {
		onEvent("cleanup", "running", fmt.Sprintf("Cleaning up wave %d worktrees", waveNum))
	}
	cleanupRes := protocol.Cleanup(ctx, implPath, waveNum, repoPath, nil)
	if cleanupRes.IsFatal() {
		if onEvent != nil {
			onEvent("cleanup", "error", fmt.Sprintf("Cleanup failed (non-fatal): %s", cleanupRes.Errors[0].Message))
		}
		warnings = append(warnings, result.NewWarning(result.CodeCleanupFailed,
			fmt.Sprintf("cleanup failed: %s", cleanupRes.Errors[0].Message)).
			WithContext("impl_path", implPath).
			WithContext("wave_num", fmt.Sprintf("%d", waveNum)))
	} else {
		data.Cleaned = true
		if onEvent != nil {
			onEvent("cleanup", "success", fmt.Sprintf("Wave %d worktrees cleaned up", waveNum))
		}
	}

	if len(warnings) > 0 {
		return result.NewPartial(data, warnings)
	}
	return result.NewSuccess(data)
}
