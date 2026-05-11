package engine

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/blackwell-systems/polywave-go/pkg/autonomy"
	"github.com/blackwell-systems/polywave-go/pkg/protocol"
	"github.com/blackwell-systems/polywave-go/pkg/queue"
)

// ─── Helpers ──────────────────────────────────────────────────────────────

// saveDaemonFuncs returns a function that restores all daemon function variables.
func saveDaemonFuncs() func() {
	origCheckQueue := daemonCheckQueueFunc
	origRunScout := daemonRunScoutFunc
	origFinalizeWave := daemonFinalizeWaveFunc
	origAutoRemediate := daemonAutoRemediateFunc
	origMarkIMPLComplete := daemonMarkIMPLCompleteFunc
	origCreateWorktrees := daemonCreateWorktreesFunc
	origUpdateQueueStatus := daemonUpdateQueueStatusFunc
	origLoadIMPLWaves := daemonLoadIMPLWavesFunc

	return func() {
		daemonCheckQueueFunc = origCheckQueue
		daemonRunScoutFunc = origRunScout
		daemonFinalizeWaveFunc = origFinalizeWave
		daemonAutoRemediateFunc = origAutoRemediate
		daemonMarkIMPLCompleteFunc = origMarkIMPLComplete
		daemonCreateWorktreesFunc = origCreateWorktrees
		daemonUpdateQueueStatusFunc = origUpdateQueueStatus
		daemonLoadIMPLWavesFunc = origLoadIMPLWaves
	}
}

// installNoopWorktrees replaces daemonCreateWorktreesFunc with a no-op.
func installNoopWorktrees() {
	daemonCreateWorktreesFunc = func(ctx context.Context, manifestPath string, waveNum int, repoDir string) (*protocol.CreateWorktreesData, error) {
		return &protocol.CreateWorktreesData{}, nil
	}
}

// installNoopMarkComplete replaces daemonMarkIMPLCompleteFunc with a no-op.
func installNoopMarkComplete() {
	daemonMarkIMPLCompleteFunc = func(ctx context.Context, opts MarkIMPLCompleteOpts) error {
		return nil
	}
}

// installNoopUpdateQueueStatus replaces daemonUpdateQueueStatusFunc with a no-op.
func installNoopUpdateQueueStatus() {
	daemonUpdateQueueStatusFunc = func(repoPath, slug, status string) error {
		return nil
	}
}

// installSuccessFinalizeWave replaces daemonFinalizeWaveFunc with a successful no-op.
func installSuccessFinalizeWave() {
	daemonFinalizeWaveFunc = func(ctx context.Context, opts FinalizeWaveOpts) (*FinalizeWaveResult, error) {
		return &FinalizeWaveResult{Wave: opts.WaveNum, BuildPassed: true, Success: true}, nil
	}
}

// installNoopScout replaces daemonRunScoutFunc with a no-op.
func installNoopScout() {
	daemonRunScoutFunc = func(ctx context.Context, opts RunScoutOpts, onChunk func(string)) error {
		return nil
	}
}

// installSingleWaveIMPL makes daemonLoadIMPLWavesFunc return a single wave.
func installSingleWaveIMPL(waveNum int) {
	daemonLoadIMPLWavesFunc = func(implPath string) ([]int, error) {
		return []int{waveNum}, nil
	}
}

// collectEvents is a thread-safe event collector for test assertions.
type collectEvents struct {
	mu     sync.Mutex
	events []Event
}

func (c *collectEvents) add(ev Event) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.events = append(c.events, ev)
}

func (c *collectEvents) names() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	names := make([]string, len(c.events))
	for i, ev := range c.events {
		names[i] = ev.Event
	}
	return names
}

func (c *collectEvents) hasEvent(name string) bool {
	for _, n := range c.names() {
		if n == name {
			return true
		}
	}
	return false
}

// seedQueueForDaemon is a thin helper that adds a single item and returns its slug.
func seedQueueForDaemon(t *testing.T, repoPath string, item queue.Item) string {
	t.Helper()
	mgr := queue.NewManager(repoPath)
	res := mgr.Add(item)
	if res.IsFatal() {
		t.Fatalf("seedQueueForDaemon: %s", res.Errors[0].Message)
	}
	return res.GetData().Slug
}

// ─── TestRunDaemon_StopsOnCancel ──────────────────────────────────────────

// TestRunDaemon_StopsOnCancel verifies that RunDaemon exits cleanly when ctx
// is cancelled before a queue item is dequeued.
func TestRunDaemon_StopsOnCancel(t *testing.T) {
	defer saveDaemonFuncs()()

	// Queue returns nothing (empty queue) so daemon sleeps.
	daemonCheckQueueFunc = func(ctx context.Context, opts CheckQueueOpts) (*CheckQueueResult, error) {
		return &CheckQueueResult{Advanced: false, Reason: "no_items"}, nil
	}

	col := &collectEvents{}
	ctx, cancel := context.WithCancel(context.Background())

	// Cancel almost immediately.
	go func() {
		time.Sleep(20 * time.Millisecond)
		cancel()
	}()

	opts := DaemonOpts{
		RepoPath:     t.TempDir(),
		PollInterval: 5 * time.Millisecond,
		OnEvent:      col.add,
	}

	res := RunDaemon(ctx, opts)
	if res.IsFatal() {
		t.Fatalf("RunDaemon returned fatal: %v", res.Errors)
	}
	if !col.hasEvent("daemon_started") {
		t.Error("expected daemon_started event")
	}
	if !col.hasEvent("daemon_stopped") {
		t.Error("expected daemon_stopped event")
	}
}

// ─── TestRunDaemon_EmptyQueue ─────────────────────────────────────────────

// TestRunDaemon_EmptyQueue verifies that daemon_poll events are emitted when
// the queue is empty and the daemon sleeps between polls.
func TestRunDaemon_EmptyQueue(t *testing.T) {
	defer saveDaemonFuncs()()

	pollCount := 0
	daemonCheckQueueFunc = func(ctx context.Context, opts CheckQueueOpts) (*CheckQueueResult, error) {
		pollCount++
		return &CheckQueueResult{Advanced: false, Reason: "no_items"}, nil
	}

	col := &collectEvents{}
	ctx, cancel := context.WithCancel(context.Background())

	// Let daemon poll at least twice, then cancel.
	go func() {
		time.Sleep(60 * time.Millisecond)
		cancel()
	}()

	opts := DaemonOpts{
		RepoPath:     t.TempDir(),
		PollInterval: 10 * time.Millisecond,
		OnEvent:      col.add,
	}

	if daemonRes := RunDaemon(ctx, opts); daemonRes.IsFatal() {
		t.Fatalf("RunDaemon: %v", daemonRes.Errors)
	}

	if !col.hasEvent("daemon_poll") {
		t.Error("expected at least one daemon_poll event")
	}
	if pollCount < 2 {
		t.Errorf("expected at least 2 polls, got %d", pollCount)
	}
}

// ─── TestRunDaemon_ProcessesOneItem ──────────────────────────────────────

// TestRunDaemon_ProcessesOneItem verifies the full cycle is executed for a
// single queue item and the daemon then goes back to polling.
func TestRunDaemon_ProcessesOneItem(t *testing.T) {
	defer saveDaemonFuncs()()

	// Queue advances once, then returns empty.
	callCount := 0
	daemonCheckQueueFunc = func(ctx context.Context, opts CheckQueueOpts) (*CheckQueueResult, error) {
		callCount++
		if callCount == 1 {
			return &CheckQueueResult{
				Advanced:  true,
				NextSlug:  "my-feature",
				NextTitle: "My Feature",
				Reason:    "triggered",
			}, nil
		}
		return &CheckQueueResult{Advanced: false, Reason: "no_items"}, nil
	}

	installNoopScout()
	installSingleWaveIMPL(1)
	installNoopWorktrees()
	installSuccessFinalizeWave()
	installNoopMarkComplete()
	installNoopUpdateQueueStatus()

	col := &collectEvents{}
	ctx, cancel := context.WithCancel(context.Background())

	// Cancel after daemon processes the item and goes back to polling.
	go func() {
		time.Sleep(200 * time.Millisecond)
		cancel()
	}()

	opts := DaemonOpts{
		RepoPath:       t.TempDir(),
		PollInterval:   10 * time.Millisecond,
		AutonomyConfig: autonomy.Config{Level: autonomy.LevelAutonomous, MaxAutoRetries: 2},
		OnEvent:        col.add,
	}

	if daemonRes2 := RunDaemon(ctx, opts); daemonRes2.IsFatal() {
		t.Fatalf("RunDaemon: %v", daemonRes2.Errors)
	}

	// Verify full cycle events.
	for _, want := range []string{
		"daemon_started",
		"daemon_poll",
		"daemon_processing",
		"daemon_wave_complete",
		"daemon_impl_complete",
		"daemon_stopped",
	} {
		if !col.hasEvent(want) {
			t.Errorf("missing expected event %q; got: %v", want, col.names())
		}
	}
}

// ─── TestRunDaemon_AutoRemediatesOnFailure ────────────────────────────────

// TestRunDaemon_AutoRemediatesOnFailure verifies that when FinalizeWave fails
// and autonomy permits, AutoRemediate is called and the item completes.
func TestRunDaemon_AutoRemediatesOnFailure(t *testing.T) {
	defer saveDaemonFuncs()()

	// Queue advances once.
	callCount := 0
	daemonCheckQueueFunc = func(ctx context.Context, opts CheckQueueOpts) (*CheckQueueResult, error) {
		callCount++
		if callCount == 1 {
			return &CheckQueueResult{
				Advanced:  true,
				NextSlug:  "remediatable-feature",
				NextTitle: "Remediatable Feature",
				Reason:    "triggered",
			}, nil
		}
		return &CheckQueueResult{Advanced: false, Reason: "no_items"}, nil
	}

	installNoopScout()
	installSingleWaveIMPL(1)
	installNoopWorktrees()

	// FinalizeWave fails.
	daemonFinalizeWaveFunc = func(ctx context.Context, opts FinalizeWaveOpts) (*FinalizeWaveResult, error) {
		return &FinalizeWaveResult{Wave: opts.WaveNum, BuildPassed: false}, fmt.Errorf("build failed")
	}

	// AutoRemediate fixes it.
	remediateCalled := false
	daemonAutoRemediateFunc = func(ctx context.Context, opts AutoRemediateOpts) (*AutoRemediateResult, error) {
		remediateCalled = true
		return &AutoRemediateResult{Fixed: true, Attempts: 1}, nil
	}

	installNoopMarkComplete()
	installNoopUpdateQueueStatus()

	col := &collectEvents{}
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(200 * time.Millisecond)
		cancel()
	}()

	opts := DaemonOpts{
		RepoPath: t.TempDir(),
		// autonomous permits gate_failure remediation
		AutonomyConfig: autonomy.Config{Level: autonomy.LevelAutonomous, MaxAutoRetries: 2},
		PollInterval:   10 * time.Millisecond,
		OnEvent:        col.add,
	}

	if daemonRes2 := RunDaemon(ctx, opts); daemonRes2.IsFatal() {
		t.Fatalf("RunDaemon: %v", daemonRes2.Errors)
	}

	if !remediateCalled {
		t.Error("expected AutoRemediate to be called")
	}
	if !col.hasEvent("daemon_impl_complete") {
		t.Errorf("expected daemon_impl_complete event; got: %v", col.names())
	}
	if col.hasEvent("daemon_blocked") {
		t.Errorf("unexpected daemon_blocked event when remediation succeeded")
	}
}

// ─── TestRunDaemon_PausesForReview ────────────────────────────────────────

// TestRunDaemon_PausesForReview verifies that in supervised mode the daemon
// emits daemon_awaiting_review and waits (ctx cancellation is the release).
func TestRunDaemon_PausesForReview(t *testing.T) {
	defer saveDaemonFuncs()()

	// Queue advances once.
	callCount := 0
	daemonCheckQueueFunc = func(ctx context.Context, opts CheckQueueOpts) (*CheckQueueResult, error) {
		callCount++
		if callCount == 1 {
			return &CheckQueueResult{
				Advanced:  true,
				NextSlug:  "supervised-feature",
				NextTitle: "Supervised Feature",
				Reason:    "triggered",
			}, nil
		}
		return &CheckQueueResult{Advanced: false, Reason: "no_items"}, nil
	}

	installNoopScout()
	installSingleWaveIMPL(1)
	installNoopWorktrees()
	installSuccessFinalizeWave()
	installNoopMarkComplete()
	installNoopUpdateQueueStatus()

	col := &collectEvents{}
	ctx, cancel := context.WithCancel(context.Background())

	// Cancel quickly — this simulates "human reviewed and stopped the daemon".
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	opts := DaemonOpts{
		RepoPath: t.TempDir(),
		// supervised: impl_review stage is NOT auto-approved → daemon pauses
		AutonomyConfig: autonomy.Config{Level: autonomy.LevelSupervised, MaxAutoRetries: 2},
		PollInterval:   5 * time.Millisecond,
		OnEvent:        col.add,
	}

	if daemonRes2 := RunDaemon(ctx, opts); daemonRes2.IsFatal() {
		t.Fatalf("RunDaemon: %v", daemonRes2.Errors)
	}

	if !col.hasEvent("daemon_awaiting_review") {
		t.Errorf("expected daemon_awaiting_review event; got: %v", col.names())
	}
	// In supervised mode the wave should NOT run after we cancel.
	if col.hasEvent("daemon_wave_complete") {
		t.Error("unexpected daemon_wave_complete: daemon should have paused for review")
	}
}

// ─── TestRunDaemon_EmitsEvents ─────────────────────────────────────────────

// TestRunDaemon_EmitsEvents verifies that all lifecycle events are emitted in
// the happy path (autonomous mode, one item, one wave).
func TestRunDaemon_EmitsEvents(t *testing.T) {
	defer saveDaemonFuncs()()

	callCount := 0
	daemonCheckQueueFunc = func(ctx context.Context, opts CheckQueueOpts) (*CheckQueueResult, error) {
		callCount++
		if callCount == 1 {
			return &CheckQueueResult{
				Advanced:  true,
				NextSlug:  "event-feature",
				NextTitle: "Event Feature",
				Reason:    "triggered",
			}, nil
		}
		return &CheckQueueResult{Advanced: false, Reason: "no_items"}, nil
	}

	installNoopScout()
	installSingleWaveIMPL(1)
	installNoopWorktrees()
	installSuccessFinalizeWave()
	installNoopMarkComplete()
	installNoopUpdateQueueStatus()

	col := &collectEvents{}
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(200 * time.Millisecond)
		cancel()
	}()

	opts := DaemonOpts{
		RepoPath:       t.TempDir(),
		AutonomyConfig: autonomy.Config{Level: autonomy.LevelAutonomous, MaxAutoRetries: 2},
		PollInterval:   10 * time.Millisecond,
		OnEvent:        col.add,
	}

	if daemonRes2 := RunDaemon(ctx, opts); daemonRes2.IsFatal() {
		t.Fatalf("RunDaemon: %v", daemonRes2.Errors)
	}

	required := []string{
		"daemon_started",
		"daemon_poll",
		"daemon_processing",
		"daemon_wave_complete",
		"daemon_impl_complete",
		"daemon_stopped",
	}
	for _, want := range required {
		if !col.hasEvent(want) {
			t.Errorf("missing required event %q; all events: %v", want, col.names())
		}
	}
}

// ─── TestRunDaemon_BlockedOnRemediationFailure ────────────────────────────

// TestRunDaemon_BlockedOnRemediationFailure verifies that an item is marked
// blocked when FinalizeWave fails and AutoRemediate cannot fix it.
func TestRunDaemon_BlockedOnRemediationFailure(t *testing.T) {
	defer saveDaemonFuncs()()

	callCount := 0
	daemonCheckQueueFunc = func(ctx context.Context, opts CheckQueueOpts) (*CheckQueueResult, error) {
		callCount++
		if callCount == 1 {
			return &CheckQueueResult{
				Advanced:  true,
				NextSlug:  "unfixable-feature",
				NextTitle: "Unfixable Feature",
				Reason:    "triggered",
			}, nil
		}
		return &CheckQueueResult{Advanced: false, Reason: "no_items"}, nil
	}

	installNoopScout()
	installSingleWaveIMPL(1)
	installNoopWorktrees()

	daemonFinalizeWaveFunc = func(ctx context.Context, opts FinalizeWaveOpts) (*FinalizeWaveResult, error) {
		return &FinalizeWaveResult{Wave: opts.WaveNum, BuildPassed: false}, errors.New("persistent build failure")
	}

	// AutoRemediate cannot fix it.
	daemonAutoRemediateFunc = func(ctx context.Context, opts AutoRemediateOpts) (*AutoRemediateResult, error) {
		return &AutoRemediateResult{Fixed: false, Attempts: 2}, nil
	}

	installNoopMarkComplete()

	statusUpdates := map[string]string{}
	daemonUpdateQueueStatusFunc = func(repoPath, slug, status string) error {
		statusUpdates[slug] = status
		return nil
	}

	col := &collectEvents{}
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(200 * time.Millisecond)
		cancel()
	}()

	opts := DaemonOpts{
		RepoPath:       t.TempDir(),
		AutonomyConfig: autonomy.Config{Level: autonomy.LevelAutonomous, MaxAutoRetries: 2},
		PollInterval:   10 * time.Millisecond,
		OnEvent:        col.add,
	}

	if daemonRes2 := RunDaemon(ctx, opts); daemonRes2.IsFatal() {
		t.Fatalf("RunDaemon: %v", daemonRes2.Errors)
	}

	if !col.hasEvent("daemon_blocked") {
		t.Errorf("expected daemon_blocked event; got: %v", col.names())
	}
	if statusUpdates["unfixable-feature"] != "blocked" {
		t.Errorf("expected queue status 'blocked', got %q", statusUpdates["unfixable-feature"])
	}
}
