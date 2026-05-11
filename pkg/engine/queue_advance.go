package engine

import (
	"context"
	"fmt"

	"github.com/blackwell-systems/polywave-go/pkg/autonomy"
	"github.com/blackwell-systems/polywave-go/pkg/queue"
	"github.com/blackwell-systems/polywave-go/pkg/result"
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

	nextRes := mgr.Next()
	if nextRes.IsFatal() {
		// QUEUE_EMPTY is not an error — it means no eligible items.
		if len(nextRes.Errors) > 0 && nextRes.Errors[0].Code == result.CodeQueueEmpty {
			return &CheckQueueResult{
				Advanced: false,
				Reason:   "no_items",
			}, nil
		}
		return nil, fmt.Errorf("engine.CheckQueue: queue.Next: %s", nextRes.Errors[0].Message)
	}
	item := nextRes.GetData()

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
	if updateRes := mgr.UpdateStatus(item.Slug, "in_progress"); updateRes.IsFatal() {
		return nil, fmt.Errorf("engine.CheckQueue: UpdateStatus: %s", updateRes.Errors[0].Message)
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
