package engine

import (
	"context"
	"fmt"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/autonomy"
	"github.com/blackwell-systems/scout-and-wave-go/pkg/queue"
)

// CheckQueueOpts configures a CheckQueue call.
type CheckQueueOpts struct {
	RepoPath       string
	AutonomyConfig autonomy.Config
	OnEvent        func(Event)
}

// CheckQueueResult is the result of a CheckQueue call.
type CheckQueueResult struct {
	Advanced  bool   `json:"advanced"`
	NextSlug  string `json:"next_slug,omitempty"`
	NextTitle string `json:"next_title,omitempty"`
	// Reason is one of: "no_items", "deps_unmet", "autonomy_blocked", "triggered"
	Reason string `json:"reason"`
}

// CheckQueue checks the IMPL queue and, if autonomy permits, marks the next
// eligible item as "in_progress" so the caller can trigger Scout.
//
// It does NOT trigger Scout directly — that is the caller's responsibility.
func CheckQueue(ctx context.Context, opts CheckQueueOpts) (*CheckQueueResult, error) {
	if opts.RepoPath == "" {
		return nil, fmt.Errorf("engine.CheckQueue: RepoPath is required")
	}

	// Check context before doing any work.
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	mgr := queue.NewManager(opts.RepoPath)

	item, err := mgr.Next()
	if err != nil {
		return nil, fmt.Errorf("engine.CheckQueue: queue.Next: %w", err)
	}
	if item == nil {
		return &CheckQueueResult{
			Advanced: false,
			Reason:   "no_items",
		}, nil
	}

	// Determine effective autonomy level (honour per-item override).
	level := autonomy.EffectiveLevel(opts.AutonomyConfig, item.AutonomyOverride)

	if !autonomy.ShouldAutoApprove(level, autonomy.StageQueueAdvance) {
		return &CheckQueueResult{
			Advanced:  false,
			NextSlug:  item.Slug,
			NextTitle: item.Title,
			Reason:    "autonomy_blocked",
		}, nil
	}

	// Autonomy allows it — mark item in_progress.
	if err := mgr.UpdateStatus(item.Slug, "in_progress"); err != nil {
		return nil, fmt.Errorf("engine.CheckQueue: UpdateStatus: %w", err)
	}

	result := &CheckQueueResult{
		Advanced:  true,
		NextSlug:  item.Slug,
		NextTitle: item.Title,
		Reason:    "triggered",
	}

	// Emit event if a listener is registered.
	if opts.OnEvent != nil {
		opts.OnEvent(Event{
			Event: "queue_advanced",
			Data:  result,
		})
	}

	return result, nil
}
