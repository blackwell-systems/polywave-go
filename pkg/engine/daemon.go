package engine

import (
	"context"
	"fmt"
	"time"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/autonomy"
	"github.com/blackwell-systems/scout-and-wave-go/pkg/protocol"
	"github.com/blackwell-systems/scout-and-wave-go/pkg/queue"
	"github.com/blackwell-systems/scout-and-wave-go/pkg/result"
)

// DaemonOpts configures the daemon run loop.
type DaemonOpts struct {
	RepoPath       string
	AutonomyConfig autonomy.Config
	SAWRepoPath    string        // for Scout agent
	ChatModel      string        // default model
	PollInterval   time.Duration // how often to check queue (default: 30s)
	OnEvent        func(Event)
}

// DaemonState reports current daemon status.
type DaemonState struct {
	Running        bool   `json:"running"`
	CurrentIMPL    string `json:"current_impl,omitempty"`
	CurrentWave    int    `json:"current_wave,omitempty"`
	QueueDepth     int    `json:"queue_depth"`
	CompletedCount int    `json:"completed_count"`
	BlockedCount   int    `json:"blocked_count"`
}

// ─── Function variables (override in tests) ────────────────────────────────

// daemonCheckQueueFunc is the function called to advance the queue.
var daemonCheckQueueFunc = func(ctx context.Context, opts CheckQueueOpts) (*CheckQueueResult, error) {
	return CheckQueue(ctx, opts)
}

// daemonRunScoutFunc is the function called to generate an IMPL doc.
var daemonRunScoutFunc = func(ctx context.Context, opts RunScoutOpts, onChunk func(string)) error {
	res := RunScout(ctx, opts, onChunk)
	if res.IsFatal() {
		if len(res.Errors) > 0 {
			return fmt.Errorf("%s", res.Errors[0].Message)
		}
		return fmt.Errorf("RunScout failed")
	}
	return nil
}

// daemonFinalizeWaveFunc is the function called to finalize a wave.
var daemonFinalizeWaveFunc = func(ctx context.Context, opts FinalizeWaveOpts) (*FinalizeWaveResult, error) {
	return FinalizeWave(ctx, opts)
}

// daemonAutoRemediateFunc is the function called to auto-remediate a failed wave.
var daemonAutoRemediateFunc = func(ctx context.Context, opts AutoRemediateOpts) (*AutoRemediateResult, error) {
	return AutoRemediate(ctx, opts)
}

// daemonMarkIMPLCompleteFunc is the function called to mark an IMPL complete.
var daemonMarkIMPLCompleteFunc = func(ctx context.Context, opts MarkIMPLCompleteOpts) error {
	res := MarkIMPLComplete(ctx, opts)
	if res.IsFatal() {
		if len(res.Errors) > 0 {
			return fmt.Errorf("%s", res.Errors[0].Message)
		}
		return fmt.Errorf("mark IMPL complete failed")
	}
	return nil
}

// daemonCreateWorktreesFunc is the function called to create worktrees before a wave.
var daemonCreateWorktreesFunc = func(manifestPath string, waveNum int, repoDir string) (*protocol.CreateWorktreesData, error) {
	res := protocol.CreateWorktrees(manifestPath, waveNum, repoDir, nil)
	if !res.IsSuccess() {
		return nil, fmt.Errorf("create worktrees failed: %v", res.Errors)
	}
	data := res.GetData()
	return &data, nil
}

// daemonUpdateQueueStatusFunc is the function called to update a queue item status.
var daemonUpdateQueueStatusFunc = func(repoPath, slug, status string) error {
	mgr := queue.NewManager(repoPath)
	res := mgr.UpdateStatus(slug, status)
	if res.IsFatal() {
		return fmt.Errorf("queue.UpdateStatus: %s", res.Errors[0].Message)
	}
	return nil
}

// daemonLoadIMPLWavesFunc loads the wave list from a manifest path.
// Returns (waveNums, error).
var daemonLoadIMPLWavesFunc = func(implPath string) ([]int, error) {
	manifest, err := protocol.Load(context.TODO(), implPath)
	if err != nil {
		return nil, err
	}
	waveNums := make([]int, 0, len(manifest.Waves))
	for _, w := range manifest.Waves {
		waveNums = append(waveNums, w.Number)
	}
	return waveNums, nil
}

// ─── Daemon implementation ─────────────────────────────────────────────────

// RunDaemon runs the daemon loop until ctx is cancelled.
// It sequentially processes IMPL queue items through the full Scout → Wave → Merge → Complete cycle.
func RunDaemon(ctx context.Context, opts DaemonOpts) result.Result[DaemonData] {
	if opts.PollInterval <= 0 {
		opts.PollInterval = 30 * time.Second
	}

	emit := func(eventName string, data interface{}) {
		if opts.OnEvent != nil {
			opts.OnEvent(Event{Event: eventName, Data: data})
		}
	}

	state := &DaemonState{Running: true}
	completedCount := 0
	emit("daemon_started", state)

	defer func() {
		state.Running = false
		emit("daemon_stopped", state)
	}()

	for {
		// Check context before each iteration.
		select {
		case <-ctx.Done():
			return result.NewSuccess(DaemonData{RepoPath: opts.RepoPath, CompletedCount: completedCount})
		default:
		}

		emit("daemon_poll", map[string]interface{}{
			"queue_depth":     state.QueueDepth,
			"completed_count": state.CompletedCount,
			"blocked_count":   state.BlockedCount,
		})

		// ── Step a: Check queue ────────────────────────────────────────────
		queueResult, err := daemonCheckQueueFunc(ctx, CheckQueueOpts{
			RepoPath:       opts.RepoPath,
			AutonomyConfig: opts.AutonomyConfig,
			OnEvent:        opts.OnEvent,
		})
		if err != nil {
			// Context cancellation during queue check.
			if ctx.Err() != nil {
				return result.NewSuccess(DaemonData{RepoPath: opts.RepoPath, CompletedCount: completedCount})
			}
			// Transient error — sleep and retry.
			select {
			case <-ctx.Done():
				return result.NewSuccess(DaemonData{RepoPath: opts.RepoPath, CompletedCount: completedCount})
			case <-time.After(opts.PollInterval):
			}
			continue
		}

		// Refresh queue depth from list.
		if mgr := queue.NewManager(opts.RepoPath); mgr != nil {
			if listRes := mgr.List(); !listRes.IsFatal() {
				pending := 0
				for _, item := range listRes.GetData().Items {
					if item.Status == "queued" || item.Status == "in_progress" {
						pending++
					}
				}
				state.QueueDepth = pending
			}
		}

		// ── Step b: Empty queue → sleep ────────────────────────────────────
		if !queueResult.Advanced {
			select {
			case <-ctx.Done():
				return result.NewSuccess(DaemonData{RepoPath: opts.RepoPath, CompletedCount: completedCount})
			case <-time.After(opts.PollInterval):
			}
			continue
		}

		// ── Step c: Process the queue item ─────────────────────────────────
		slug := queueResult.NextSlug
		title := queueResult.NextTitle
		state.CurrentIMPL = slug

		emit("daemon_processing", map[string]interface{}{
			"slug":  slug,
			"title": title,
		})

		blocked := false // set to true if the item must be marked blocked

		err = daemonProcessItem(ctx, opts, state, slug, title, emit)
		if err != nil {
			if ctx.Err() != nil {
				// Context cancelled mid-item — clean shutdown.
				return result.NewSuccess(DaemonData{RepoPath: opts.RepoPath, CompletedCount: completedCount})
			}
			// Item processing failed — mark blocked.
			blocked = true
			emit("daemon_blocked", map[string]interface{}{
				"slug":  slug,
				"error": err.Error(),
			})
			state.BlockedCount++
		}

		// Update queue item status.
		finalStatus := "complete"
		if blocked {
			finalStatus = "blocked"
		}
		if statusErr := daemonUpdateQueueStatusFunc(opts.RepoPath, slug, finalStatus); statusErr != nil {
			emit("daemon_blocked", map[string]interface{}{
				"slug":  slug,
				"error": fmt.Sprintf("failed to update queue status to %s: %v", finalStatus, statusErr),
			})
		}

		state.CurrentIMPL = ""
		state.CurrentWave = 0

		if !blocked {
			state.CompletedCount++
		completedCount++
			emit("daemon_impl_complete", map[string]interface{}{
				"slug":            slug,
				"completed_count": state.CompletedCount,
			})
		}
	}
}

// daemonProcessItem processes a single queue item through the full cycle.
// Returns an error if the item should be marked as blocked.
func daemonProcessItem(
	ctx context.Context,
	opts DaemonOpts,
	state *DaemonState,
	slug, title string,
	emit func(string, interface{}),
) error {
	// ── Step c-i: Run Scout to generate IMPL ──────────────────────────────
	implPath := protocol.IMPLPath(opts.RepoPath, slug)

	scoutOpts := RunScoutOpts{
		Feature:     title,
		RepoPath:    opts.RepoPath,
		SAWRepoPath: opts.SAWRepoPath,
		IMPLOutPath: implPath,
		ScoutModel:  opts.ChatModel,
	}

	if scoutErr := daemonRunScoutFunc(ctx, scoutOpts, func(chunk string) {
		emit("daemon_scout_output", map[string]interface{}{"slug": slug, "chunk": chunk})
	}); scoutErr != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return fmt.Errorf("scout failed: %w", scoutErr)
	}

	// ── Step c-ii: Supervised mode pauses for human review ────────────────
	level := autonomy.EffectiveLevel(opts.AutonomyConfig, "")
	if !autonomy.ShouldAutoApprove(level, autonomy.StageIMPLReview) {
		emit("daemon_awaiting_review", map[string]interface{}{
			"slug":      slug,
			"impl_path": implPath,
			"stage":     string(autonomy.StageIMPLReview),
		})
		// Wait for ctx cancellation (human will restart daemon / manually approve).
		<-ctx.Done()
		return ctx.Err()
	}

	// ── Step c-iii: Load wave list ─────────────────────────────────────────
	waveNums, loadErr := daemonLoadIMPLWavesFunc(implPath)
	if loadErr != nil {
		return fmt.Errorf("load IMPL waves: %w", loadErr)
	}

	// ── Step c-iv: Run each wave ──────────────────────────────────────────
	for _, waveNum := range waveNums {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		state.CurrentWave = waveNum

		// Create worktrees for this wave.
		if _, wtErr := daemonCreateWorktreesFunc(implPath, waveNum, opts.RepoPath); wtErr != nil {
			return fmt.Errorf("create worktrees wave %d: %w", waveNum, wtErr)
		}

		// Finalize wave (verify + merge + build).
		finalizeResult, finalizeErr := daemonFinalizeWaveFunc(ctx, FinalizeWaveOpts{
			IMPLPath: implPath,
			RepoPath: opts.RepoPath,
			WaveNum:  waveNum,
		})

		if finalizeErr != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}

			// Auto-remediate if autonomy permits.
			if autonomy.ShouldAutoApprove(level, autonomy.StageGateFailure) {
				maxRetries := opts.AutonomyConfig.MaxAutoRetries
				if maxRetries <= 0 {
					maxRetries = 2
				}
				remResult, remErr := daemonAutoRemediateFunc(ctx, AutoRemediateOpts{
					IMPLPath:     implPath,
					RepoPath:     opts.RepoPath,
					WaveNum:      waveNum,
					FailedResult: finalizeResult,
					MaxRetries:   maxRetries,
					ChatModel:    opts.ChatModel,
					OnEvent:      opts.OnEvent,
				})
				if remErr != nil {
					if ctx.Err() != nil {
						return ctx.Err()
					}
					// Remediation itself errored — mark blocked.
					emit("daemon_blocked", map[string]interface{}{
						"slug":  slug,
						"wave":  waveNum,
						"error": remErr.Error(),
					})
					return fmt.Errorf("auto-remediate wave %d: %w", waveNum, remErr)
				}
				if !remResult.Fixed {
					emit("daemon_blocked", map[string]interface{}{
						"slug": slug,
						"wave": waveNum,
					})
					return fmt.Errorf("wave %d failed after auto-remediation", waveNum)
				}
			} else {
				// Autonomy doesn't permit auto-remediation — mark blocked.
				emit("daemon_blocked", map[string]interface{}{
					"slug": slug,
					"wave": waveNum,
				})
				return fmt.Errorf("wave %d failed and autonomy blocks remediation: %w", waveNum, finalizeErr)
			}
		}

		emit("daemon_wave_complete", map[string]interface{}{
			"slug": slug,
			"wave": waveNum,
		})
	}

	// ── Step c-v: Mark IMPL complete ──────────────────────────────────────
	if completeErr := daemonMarkIMPLCompleteFunc(ctx, MarkIMPLCompleteOpts{
		IMPLPath: implPath,
		RepoPath: opts.RepoPath,
	}); completeErr != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return fmt.Errorf("mark IMPL complete: %w", completeErr)
	}

	return nil
}
