package engine

import (
	"context"
	"fmt"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/protocol"
)

// PostMergeCleanup runs post-merge processing: go.mod replace path fixup
// followed by worktree/branch cleanup. Both steps are non-fatal — errors
// are reported through the onEvent callback but do not cause the function
// to return an error. This consolidates identical cleanup blocks previously
// duplicated across service.MergeWave, api.handleWaveMerge, and
// api.handleResolveConflicts.
func PostMergeCleanup(ctx context.Context, implPath string, waveNum int, repoPath string, onEvent EventCallback) error {
	_ = ctx // reserved for future cancellation support

	// Step 1: Fix go.mod replace paths (deep relative paths from worktree agents).
	if onEvent != nil {
		onEvent("gomod_fixup", "running", "Checking go.mod replace paths")
	}
	fixed, err := protocol.FixGoModReplacePaths(repoPath)
	if err != nil {
		if onEvent != nil {
			onEvent("gomod_fixup", "error", fmt.Sprintf("go.mod fixup failed (non-fatal): %v", err))
		}
	} else if fixed {
		if onEvent != nil {
			onEvent("gomod_fixup", "success", "Fixed deep replace paths in go.mod")
		}
	} else {
		if onEvent != nil {
			onEvent("gomod_fixup", "success", "No go.mod fixes needed")
		}
	}

	// Step 2: Clean up worktrees and branches for the wave.
	if onEvent != nil {
		onEvent("cleanup", "running", fmt.Sprintf("Cleaning up wave %d worktrees", waveNum))
	}
	_, cleanupErr := protocol.Cleanup(implPath, waveNum, repoPath)
	if cleanupErr != nil {
		if onEvent != nil {
			onEvent("cleanup", "error", fmt.Sprintf("Cleanup failed (non-fatal): %v", cleanupErr))
		}
	} else {
		if onEvent != nil {
			onEvent("cleanup", "success", fmt.Sprintf("Wave %d worktrees cleaned up", waveNum))
		}
	}

	return nil
}
