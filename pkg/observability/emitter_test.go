package observability

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

// fakeStore is a test double for Store that records calls and can return an error.
type fakeStore struct {
	mu     sync.Mutex
	events []Event
	err    error
}

func (f *fakeStore) RecordEvent(_ context.Context, event Event) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return f.err
	}
	f.events = append(f.events, event)
	return nil
}

func (f *fakeStore) QueryEvents(_ context.Context, _ QueryFilters) ([]Event, error) {
	return nil, nil
}

func (f *fakeStore) GetRollup(_ context.Context, _ RollupRequest) (*RollupResult, error) {
	return nil, nil
}

func (f *fakeStore) Close() error { return nil }

func (f *fakeStore) recorded() []Event {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]Event, len(f.events))
	copy(out, f.events)
	return out
}

// waitForEvent polls until n events are recorded or the deadline is exceeded.
func waitForEvents(f *fakeStore, n int, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if len(f.recorded()) >= n {
			return true
		}
		time.Sleep(5 * time.Millisecond)
	}
	return false
}

// TestNilEmitterDoesNotPanic verifies that calling Emit on a nil *Emitter is safe.
func TestNilEmitterDoesNotPanic(t *testing.T) {
	var e *Emitter
	// Should not panic.
	e.Emit(context.Background(), NewScoutLaunchEvent("test-slug"))
}

// TestNewEmitterWithNilStoreReturnsNil verifies that NewEmitter(nil) returns nil.
func TestNewEmitterWithNilStoreReturnsNil(t *testing.T) {
	e := NewEmitter(nil)
	if e != nil {
		t.Fatal("expected nil emitter when store is nil")
	}
}

// TestEmitCallsStoreRecordEvent verifies that Emit eventually calls Store.RecordEvent.
func TestEmitCallsStoreRecordEvent(t *testing.T) {
	store := &fakeStore{}
	e := NewEmitter(store)

	e.Emit(context.Background(), NewScoutLaunchEvent("my-impl"))

	if !waitForEvents(store, 1, time.Second) {
		t.Fatal("expected RecordEvent to be called within 1s")
	}

	events := store.recorded()
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].EventType() != "activity" {
		t.Errorf("expected event type 'activity', got %q", events[0].EventType())
	}
}

// TestEmitIsNonBlocking verifies that Emit returns immediately without waiting
// for the store write to complete.
func TestEmitIsNonBlocking(t *testing.T) {
	// slow store: each RecordEvent blocks for 200ms.
	slow := &slowStore{delay: 200 * time.Millisecond}
	e := NewEmitter(slow)

	start := time.Now()
	e.Emit(context.Background(), NewScoutLaunchEvent("slug"))
	elapsed := time.Since(start)

	// Emit should return in well under the store delay.
	if elapsed > 50*time.Millisecond {
		t.Errorf("Emit blocked for %v; expected non-blocking (<50ms)", elapsed)
	}
}

// slowStore is a Store that sleeps before recording to simulate latency.
type slowStore struct {
	delay time.Duration
}

func (s *slowStore) RecordEvent(_ context.Context, _ Event) error {
	time.Sleep(s.delay)
	return nil
}

func (s *slowStore) QueryEvents(_ context.Context, _ QueryFilters) ([]Event, error) {
	return nil, nil
}

func (s *slowStore) GetRollup(_ context.Context, _ RollupRequest) (*RollupResult, error) {
	return nil, nil
}

func (s *slowStore) Close() error { return nil }

// TestEmitStoreErrorLogsToStderr verifies that a store error doesn't panic or propagate.
func TestEmitStoreErrorLogsToStderr(t *testing.T) {
	store := &fakeStore{err: errors.New("db unavailable")}
	e := NewEmitter(store)
	// Should not panic.
	e.Emit(context.Background(), NewScoutLaunchEvent("slug"))
	// Give the goroutine a moment to run.
	time.Sleep(50 * time.Millisecond)
}

// ---------------------------------------------------------------------------
// Helper constructor tests
// ---------------------------------------------------------------------------

func TestNewScoutLaunchEvent(t *testing.T) {
	ev := NewScoutLaunchEvent("my-impl")
	if ev.ActivityType != "scout_launch" {
		t.Errorf("expected scout_launch, got %s", ev.ActivityType)
	}
	if ev.IMPLSlug != "my-impl" {
		t.Errorf("expected impl slug 'my-impl', got %s", ev.IMPLSlug)
	}
	if ev.EventType() != "activity" {
		t.Errorf("expected type 'activity', got %s", ev.EventType())
	}
	if ev.EventID() == "" {
		t.Error("expected non-empty event ID")
	}
}

func TestNewScoutCompleteEvent(t *testing.T) {
	ev := NewScoutCompleteEvent("my-impl")
	if ev.ActivityType != "scout_complete" {
		t.Errorf("expected scout_complete, got %s", ev.ActivityType)
	}
}

func TestNewWaveStartEvent(t *testing.T) {
	ev := NewWaveStartEvent("my-impl", 2)
	if ev.ActivityType != "wave_start" {
		t.Errorf("expected wave_start, got %s", ev.ActivityType)
	}
	if ev.WaveNumber != 2 {
		t.Errorf("expected wave 2, got %d", ev.WaveNumber)
	}
}

func TestNewWaveMergeEvent(t *testing.T) {
	ev := NewWaveMergeEvent("my-impl", 1)
	if ev.ActivityType != "wave_merge" {
		t.Errorf("expected wave_merge, got %s", ev.ActivityType)
	}
}

func TestNewWaveFailedEvent(t *testing.T) {
	ev := NewWaveFailedEvent("my-impl", 1, "merge conflict")
	if ev.ActivityType != "wave_failed" {
		t.Errorf("expected wave_failed, got %s", ev.ActivityType)
	}
	if ev.Details != "merge conflict" {
		t.Errorf("expected details 'merge conflict', got %s", ev.Details)
	}
}

func TestNewImplCompleteEvent(t *testing.T) {
	ev := NewImplCompleteEvent("my-impl")
	if ev.ActivityType != "impl_complete" {
		t.Errorf("expected impl_complete, got %s", ev.ActivityType)
	}
}

func TestNewGateExecutedEvent(t *testing.T) {
	ev := NewGateExecutedEvent("my-impl", 1, "build", true)
	if ev.ActivityType != "gate_executed" {
		t.Errorf("expected gate_executed, got %s", ev.ActivityType)
	}

	evFailed := NewGateExecutedEvent("my-impl", 1, "test", false)
	if evFailed.ActivityType != "gate_failed" {
		t.Errorf("expected gate_failed, got %s", evFailed.ActivityType)
	}
}

func TestNewTierAdvancedEvent(t *testing.T) {
	ev := NewTierAdvancedEvent("my-program", 3)
	if ev.ActivityType != "tier_advanced" {
		t.Errorf("expected tier_advanced, got %s", ev.ActivityType)
	}
	if ev.ProgramSlug != "my-program" {
		t.Errorf("expected program slug 'my-program', got %s", ev.ProgramSlug)
	}
	if ev.WaveNumber != 3 {
		t.Errorf("expected tier 3, got %d", ev.WaveNumber)
	}
}

func TestNewTierGatePassedEvent(t *testing.T) {
	ev := NewTierGatePassedEvent("my-program", 2)
	if ev.ActivityType != "tier_gate_passed" {
		t.Errorf("expected tier_gate_passed, got %s", ev.ActivityType)
	}
}

func TestNewTierGateFailedEvent(t *testing.T) {
	ev := NewTierGateFailedEvent("my-program", 2, "integration test failed")
	if ev.ActivityType != "tier_gate_failed" {
		t.Errorf("expected tier_gate_failed, got %s", ev.ActivityType)
	}
	if ev.Details != "integration test failed" {
		t.Errorf("expected details 'integration test failed', got %s", ev.Details)
	}
}

// TestEmitSyncReturnsEventID verifies that EmitSync populates EmitData.EventID.
func TestEmitSyncReturnsEventID(t *testing.T) {
	store := &fakeStore{}
	e := NewEmitter(store)
	event := NewScoutLaunchEvent("my-impl")

	res := e.EmitSync(context.Background(), event)
	if !res.IsSuccess() {
		t.Fatalf("expected success, got code=%s errors=%v", res.Code, res.Errors)
	}
	data := res.GetData()
	if data.EventID == "" {
		t.Error("expected non-empty EventID in EmitData")
	}
	if data.EventID != event.EventID() {
		t.Errorf("EventID = %q, want %q", data.EventID, event.EventID())
	}
	if data.Timestamp.IsZero() {
		t.Error("expected non-zero Timestamp in EmitData")
	}
}

// TestEmitSyncStoreErrorReturnsFatal verifies that a store error returns a FATAL result.
func TestEmitSyncStoreErrorReturnsFatal(t *testing.T) {
	store := &fakeStore{err: errors.New("disk full")}
	e := NewEmitter(store)

	res := e.EmitSync(context.Background(), NewScoutLaunchEvent("my-impl"))
	if !res.IsFatal() {
		t.Fatalf("expected FATAL result, got code=%s", res.Code)
	}
	if len(res.Errors) == 0 {
		t.Fatal("expected at least one error")
	}
	if res.Errors[0].Code != "O001_OBS_EMIT_FAILED" {
		t.Errorf("error code = %q, want O001_OBS_EMIT_FAILED", res.Errors[0].Code)
	}
}

// TestEmitSyncNilEmitterReturnsSuccess verifies that EmitSync on a nil Emitter is safe.
func TestEmitSyncNilEmitterReturnsSuccess(t *testing.T) {
	var e *Emitter
	event := NewScoutLaunchEvent("my-impl")
	res := e.EmitSync(context.Background(), event)
	if !res.IsSuccess() {
		t.Fatalf("expected success for nil emitter, got code=%s", res.Code)
	}
	if res.GetData().EventID == "" {
		t.Error("expected non-empty EventID even for nil emitter")
	}
}

// TestMultipleEmitsAreConcurrencySafe verifies concurrent Emit calls don't race.
func TestMultipleEmitsAreConcurrencySafe(t *testing.T) {
	store := &fakeStore{}
	e := NewEmitter(store)

	const n = 20
	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func() {
			defer wg.Done()
			e.Emit(context.Background(), NewScoutLaunchEvent("slug"))
		}()
	}
	wg.Wait()

	if !waitForEvents(store, n, 2*time.Second) {
		t.Errorf("expected %d events, got %d", n, len(store.recorded()))
	}
}
