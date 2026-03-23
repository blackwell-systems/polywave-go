package engine

import (
	"context"
	"fmt"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/observability"
	"github.com/blackwell-systems/scout-and-wave-go/pkg/protocol"
)

// RunWaveTransactionOpts configures the atomic wave execution wrapper.
type RunWaveTransactionOpts struct {
	IMPLPath    string
	RepoPath    string
	WaveNum     int
	MergeTarget string
	ObsEmitter  *observability.Emitter
}

// implSnapshot captures the IMPL doc state fields that may be mutated by
// FinalizeWave substeps. On failure, these are restored to roll back partial writes.
type implSnapshot struct {
	State             protocol.ProtocolState
	MergeState        protocol.MergeState
	CompletionReports map[string]protocol.CompletionReport
}

// captureSnapshot loads the manifest from disk and copies the mutable state fields.
func captureSnapshot(implPath string) (*implSnapshot, error) {
	manifest, err := protocol.Load(implPath)
	if err != nil {
		return nil, fmt.Errorf("engine.RunWaveTransaction: load snapshot: %w", err)
	}

	// Deep-copy completion reports so the snapshot is independent of the manifest.
	reports := make(map[string]protocol.CompletionReport, len(manifest.CompletionReports))
	for k, v := range manifest.CompletionReports {
		reports[k] = v
	}

	return &implSnapshot{
		State:             manifest.State,
		MergeState:        manifest.MergeState,
		CompletionReports: reports,
	}, nil
}

// restoreSnapshot reloads the manifest from disk, resets state fields to the
// snapshot values, and saves the manifest back. This handles partial state
// written by FinalizeWave substeps.
func restoreSnapshot(implPath string, snap *implSnapshot) error {
	manifest, err := protocol.Load(implPath)
	if err != nil {
		return fmt.Errorf("engine.RunWaveTransaction: load for rollback: %w", err)
	}

	manifest.State = snap.State
	manifest.MergeState = snap.MergeState
	manifest.CompletionReports = snap.CompletionReports

	if err := protocol.Save(manifest, implPath); err != nil {
		return fmt.Errorf("engine.RunWaveTransaction: save rollback: %w", err)
	}
	return nil
}

// RunWaveTransaction executes FinalizeWave atomically: on any step failure,
// the IMPL doc state is rolled back to its value before execution began.
// Returns (*FinalizeWaveResult, error).
func RunWaveTransaction(ctx context.Context, opts RunWaveTransactionOpts) (*FinalizeWaveResult, error) {
	if opts.IMPLPath == "" {
		return nil, fmt.Errorf("engine.RunWaveTransaction: IMPLPath is required")
	}
	if opts.RepoPath == "" {
		return nil, fmt.Errorf("engine.RunWaveTransaction: RepoPath is required")
	}

	// Snapshot current IMPL doc state before executing the pipeline.
	snap, err := captureSnapshot(opts.IMPLPath)
	if err != nil {
		return nil, err
	}

	// Execute FinalizeWave with the provided options.
	result, finalizeErr := FinalizeWave(ctx, FinalizeWaveOpts{
		IMPLPath:    opts.IMPLPath,
		RepoPath:    opts.RepoPath,
		WaveNum:     opts.WaveNum,
		MergeTarget: opts.MergeTarget,
		ObsEmitter:  opts.ObsEmitter,
	})

	if finalizeErr != nil {
		// Roll back IMPL doc state to pre-execution snapshot.
		if rbErr := restoreSnapshot(opts.IMPLPath, snap); rbErr != nil {
			// Return both the original error and the rollback failure.
			return result, fmt.Errorf("engine.RunWaveTransaction: rollback failed (%v) after: %w", rbErr, finalizeErr)
		}
		return result, fmt.Errorf("engine.RunWaveTransaction: %w", finalizeErr)
	}

	return result, nil
}
