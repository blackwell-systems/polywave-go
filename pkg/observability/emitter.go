package observability

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/blackwell-systems/polywave-go/pkg/result"
)

// EmitData holds the result data returned by a synchronous Emit operation.
type EmitData struct {
	EventID   string
	Timestamp time.Time
}

// Emitter is a nil-safe, non-blocking wrapper around Store.RecordEvent.
// All writes happen in a goroutine so they never block the caller.
// A nil Emitter silently no-ops on every method call.
// *Emitter satisfies pkg/engine.ObsEmitter (defined there to avoid circular imports).
type Emitter struct {
	store Store
}

// NewEmitter constructs an Emitter backed by store.
// If store is nil, NewEmitter returns nil; all Emit calls on a nil Emitter
// are safe no-ops.
func NewEmitter(store Store) *Emitter {
	if store == nil {
		return nil
	}
	return &Emitter{store: store}
}

// Emit records event asynchronously. It is safe to call on a nil *Emitter.
// Errors are written to stderr and never propagated to the caller.
func (e *Emitter) Emit(ctx context.Context, event Event) {
	if e == nil || e.store == nil {
		return
	}
	go func() {
		if err := e.store.RecordEvent(ctx, event); err != nil {
			fmt.Fprintf(os.Stderr, "observability: emit %s: %v\n", event.EventType(), err)
		}
	}()
}

// EmitSync records event synchronously and returns a result.Result[EmitData].
// Unlike Emit, it blocks until the store write completes and surfaces any error
// through the Result type rather than writing to stderr.
// It is safe to call on a nil *Emitter; a successful no-op result is returned.
func (e *Emitter) EmitSync(ctx context.Context, event Event) result.Result[EmitData] {
	if e == nil || e.store == nil {
		return result.NewSuccess(EmitData{
			EventID:   event.EventID(),
			Timestamp: event.Timestamp(),
		})
	}
	if err := e.store.RecordEvent(ctx, event); err != nil {
		return result.NewFailure[EmitData]([]result.PolywaveError{
			result.NewFatal(result.CodeObsEmitFailed, err.Error()).WithCause(err),
		})
	}
	return result.NewSuccess(EmitData{
		EventID:   event.EventID(),
		Timestamp: event.Timestamp(),
	})
}

// ---------------------------------------------------------------------------
// Helper constructors for common ActivityEvents
// ---------------------------------------------------------------------------

// NewScoutLaunchEvent creates an ActivityEvent for a scout_launch action.
func NewScoutLaunchEvent(implSlug string) *ActivityEvent {
	return &ActivityEvent{
		ID:           NewEventID(),
		Type:         "activity",
		Time:         time.Now(),
		ActivityType: "scout_launch",
		IMPLSlug:     implSlug,
	}
}

// NewScoutCompleteEvent creates an ActivityEvent for a scout_complete action.
func NewScoutCompleteEvent(implSlug string) *ActivityEvent {
	return &ActivityEvent{
		ID:           NewEventID(),
		Type:         "activity",
		Time:         time.Now(),
		ActivityType: "scout_complete",
		IMPLSlug:     implSlug,
	}
}

// NewWaveStartEvent creates an ActivityEvent for a wave_start action.
func NewWaveStartEvent(implSlug string, wave int) *ActivityEvent {
	return &ActivityEvent{
		ID:           NewEventID(),
		Type:         "activity",
		Time:         time.Now(),
		ActivityType: "wave_start",
		IMPLSlug:     implSlug,
		WaveNumber:   wave,
	}
}

// NewWaveMergeEvent creates an ActivityEvent for a wave_merge action.
func NewWaveMergeEvent(implSlug string, wave int) *ActivityEvent {
	return &ActivityEvent{
		ID:           NewEventID(),
		Type:         "activity",
		Time:         time.Now(),
		ActivityType: "wave_merge",
		IMPLSlug:     implSlug,
		WaveNumber:   wave,
	}
}

// NewWaveFailedEvent creates an ActivityEvent for a wave_failed action.
func NewWaveFailedEvent(implSlug string, wave int, details string) *ActivityEvent {
	return &ActivityEvent{
		ID:           NewEventID(),
		Type:         "activity",
		Time:         time.Now(),
		ActivityType: "wave_failed",
		IMPLSlug:     implSlug,
		WaveNumber:   wave,
		Details:      details,
	}
}

// NewImplCompleteEvent creates an ActivityEvent for an impl_complete action.
func NewImplCompleteEvent(implSlug string) *ActivityEvent {
	return &ActivityEvent{
		ID:           NewEventID(),
		Type:         "activity",
		Time:         time.Now(),
		ActivityType: "impl_complete",
		IMPLSlug:     implSlug,
	}
}

// NewGateExecutedEvent creates an ActivityEvent for a gate_executed action.
// passed indicates whether the gate passed or failed.
func NewGateExecutedEvent(implSlug string, wave int, gateType string, passed bool) *ActivityEvent {
	activityType := "gate_executed"
	if !passed {
		activityType = "gate_failed"
	}
	return &ActivityEvent{
		ID:           NewEventID(),
		Type:         "activity",
		Time:         time.Now(),
		ActivityType: activityType,
		IMPLSlug:     implSlug,
		WaveNumber:   wave,
		Details:      fmt.Sprintf("gate=%s passed=%v", gateType, passed),
	}
}

// NewTierAdvancedEvent creates an ActivityEvent for a tier_advanced action.
func NewTierAdvancedEvent(programSlug string, tier int) *ActivityEvent {
	return &ActivityEvent{
		ID:           NewEventID(),
		Type:         "activity",
		Time:         time.Now(),
		ActivityType: "tier_advanced",
		ProgramSlug:  programSlug,
		WaveNumber:   tier,
	}
}

// NewTierGatePassedEvent creates an ActivityEvent for a tier_gate_passed action.
func NewTierGatePassedEvent(programSlug string, tier int) *ActivityEvent {
	return &ActivityEvent{
		ID:           NewEventID(),
		Type:         "activity",
		Time:         time.Now(),
		ActivityType: "tier_gate_passed",
		ProgramSlug:  programSlug,
		WaveNumber:   tier,
	}
}

// NewTierGateFailedEvent creates an ActivityEvent for a tier_gate_failed action.
func NewTierGateFailedEvent(programSlug string, tier int, details string) *ActivityEvent {
	return &ActivityEvent{
		ID:           NewEventID(),
		Type:         "activity",
		Time:         time.Now(),
		ActivityType: "tier_gate_failed",
		ProgramSlug:  programSlug,
		WaveNumber:   tier,
		Details:      details,
	}
}
