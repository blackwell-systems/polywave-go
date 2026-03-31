package engine

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/protocol"
	"github.com/blackwell-systems/scout-and-wave-go/pkg/result"
)

// RunWaveTransactionOpts configures the atomic wave execution wrapper.
type RunWaveTransactionOpts struct {
	IMPLPath    string
	RepoPath    string
	WaveNum     int
	MergeTarget string
	ObsEmitter  ObsEmitter   // optional: non-blocking observability emitter
	Logger      *slog.Logger // optional: nil falls back to slog.Default()
}

// implSnapshot captures the IMPL doc state fields that may be mutated by
// FinalizeWave substeps. On failure, these are restored to roll back partial writes.
type implSnapshot struct {
	State             protocol.ProtocolState
	MergeState        protocol.MergeState
	CompletionReports map[string]protocol.CompletionReport
}

// RestoreData holds data returned by restoreSnapshot.
type RestoreData struct {
	IMPLPath string `json:"impl_path"`
}

// captureSnapshot loads the manifest from disk and copies the mutable state fields.
func captureSnapshot(implPath string) (*implSnapshot, error) {
	manifest, err := protocol.Load(context.TODO(), implPath)
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
func restoreSnapshot(implPath string, snap *implSnapshot) result.Result[RestoreData] {
	manifest, err := protocol.Load(context.TODO(), implPath)
	if err != nil {
		return result.NewFailure[RestoreData]([]result.SAWError{
			result.NewFatal("ENGINE_RESTORE_LOAD_FAILED",
				fmt.Sprintf("engine.RunWaveTransaction: load for rollback: %v", err)).
				WithContext("impl_path", implPath),
		})
	}

	manifest.State = snap.State
	manifest.MergeState = snap.MergeState
	manifest.CompletionReports = snap.CompletionReports

	if saveRes := protocol.Save(context.TODO(), manifest, implPath); saveRes.IsFatal() {
		saveErrMsg := "save failed"
		if len(saveRes.Errors) > 0 {
			saveErrMsg = saveRes.Errors[0].Message
		}
		return result.NewFailure[RestoreData]([]result.SAWError{
			result.NewFatal("ENGINE_RESTORE_SAVE_FAILED",
				fmt.Sprintf("engine.RunWaveTransaction: save rollback: %s", saveErrMsg)).
				WithContext("impl_path", implPath),
		})
	}

	return result.NewSuccess(RestoreData{IMPLPath: implPath})
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
		Logger:      opts.Logger,
	})

	if finalizeErr != nil {
		// Roll back IMPL doc state to pre-execution snapshot.
		restoreRes := restoreSnapshot(opts.IMPLPath, snap)
		if restoreRes.IsFatal() {
			rbErrMsg := "rollback failed"
			if len(restoreRes.Errors) > 0 {
				rbErrMsg = restoreRes.Errors[0].Message
			}
			return result, fmt.Errorf("engine.RunWaveTransaction: rollback failed (%s) after: %w", rbErrMsg, finalizeErr)
		}
		return result, fmt.Errorf("engine.RunWaveTransaction: %w", finalizeErr)
	}

	return result, nil
}
