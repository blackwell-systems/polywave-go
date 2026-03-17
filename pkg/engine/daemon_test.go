package engine

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/autonomy"
	"github.com/blackwell-systems/scout-and-wave-go/pkg/protocol"
)

// saveDaemonFuncs saves and restores all daemon function variables for test isolation.
func saveDaemonFuncs(t *testing.T) {
	t.Helper()
	origCheckQueue := daemonCheckQueueFunc
	origRunScout := daemonRunScoutFunc
	origFinalizeWave := daemonFinalizeWaveFunc
	origAutoRemediate := daemonAutoRemediateFunc
	origMarkComplete := daemonMarkIMPLCompleteFunc
	origCreateWorktrees := daemonCreateWorktreesFunc
	origUpdateStatus := daemonUpdateQueueStatusFunc
	origLoadWaves := daemonLoadIMPLWavesFunc

	t.Cleanup(func() {
		daemonCheckQueueFunc = origCheckQueue
		daemonRunScoutFunc = origRunScout
		daemonFinalizeWaveFunc = origFinalizeWave
		daemonAutoRemediateFunc = origAutoRemediate
		daemonMarkIMPLCompleteFunc = origMarkComplete
		daemonCreateWorktreesFunc = origCreateWorktrees
		daemonUpdateQueueStatusFunc = origUpdateStatus
		daemonLoadIMPLWavesFunc = origLoadWaves
	})
}

// setupDaemonMocks installs no-op mocks for all daemon function variables.
// Returns after saveDaemonFuncs so originals are restored on cleanup.
func setupDaemonMocks(t *testing.T) {
	t.Helper()
	saveDaemonFuncs(t)

	// Default: empty queue (no items).
	daemonCheckQueueFunc = func(ctx context.Context, opts CheckQueueOpts) (*CheckQueueResult, error) {
		return &CheckQueueResult{Advanced: false, Reason: "no_items"}, nil
	}
	daemonRunScoutFunc = func(ctx context.Context, opts RunScoutOpts, onChunk func(string)) error {
		return nil
	}
	daemonFinalizeWaveFunc = func(ctx context.Context, opts FinalizeWaveOpts) (*FinalizeWaveResult, error) {
		return &FinalizeWaveResult{Wave: opts.WaveNum, BuildPassed: true, Success: true}, nil
	}
	daemonAutoRemediateFunc = func(ctx context.Context, opts AutoRemediateOpts) (*AutoRemediateResult, error) {
		return &AutoRemediateResult{Fixed: true, Attempts: 1}, nil
	}
	daemonMarkIMPLCompleteFunc = func(ctx context.Context, opts MarkIMPLCompleteOpts) error {
		return nil
	}
	daemonCreateWorktreesFunc = func(manifestPath string, waveNum int, repoDir string) (*protocol.CreateWorktreesResult, error) {
		return &protocol.CreateWorktreesResult{}, nil
	}
	daemonUpdateQueueStatusFunc = func(repoPath, slug, status string) error {
		return nil
	}
	daemonLoadIMPLWavesFunc = func(implPath string) ([]int, error) {
		return []int{1}, nil
	}
}

func TestRunDaemon_ProcessesOneItem(t *testing.T) {
	setupDaemonMocks(t)

	callCount := 0
	var mu sync.Mutex

	// First call: return an item. Second call onwards: no items (triggers sleep, then cancel).
	daemonCheckQueueFunc = func(ctx context.Context, opts CheckQueueOpts) (*CheckQueueResult, error) {
		mu.Lock()
		callCount++
		n := callCount
		mu.Unlock()
		if n == 1 {
			return &CheckQueueResult{
				Advanced:  true,
				NextSlug:  "test-feature",
				NextTitle: "Test Feature",
				Reason:    "triggered",
			}, nil
		}
		return &CheckQueueResult{Advanced: false, Reason: "no_items"}, nil
	}

	daemonLoadIMPLWavesFunc = func(implPath string) ([]int, error) {
		return []int{1, 2}, nil
	}

	scoutCalled := false
	daemonRunScoutFunc = func(ctx context.Context, opts RunScoutOpts, onChunk func(string)) error {
		scoutCalled = true
		if opts.Feature != "Test Feature" {
			t.Errorf("expected feature 'Test Feature', got %q", opts.Feature)
		}
		return nil
	}

	wavesFinalized := 0
	daemonFinalizeWaveFunc = func(ctx context.Context, opts FinalizeWaveOpts) (*FinalizeWaveResult, error) {
		mu.Lock()
		wavesFinalized++
		mu.Unlock()
		return &FinalizeWaveResult{Wave: opts.WaveNum, BuildPassed: true, Success: true}, nil
	}

	completeCalled := false
	daemonMarkIMPLCompleteFunc = func(ctx context.Context, opts MarkIMPLCompleteOpts) error {
		completeCalled = true
		return nil
	}

	statusUpdates := make(map[string]string)
	daemonUpdateQueueStatusFunc = func(repoPath, slug, status string) error {
		mu.Lock()
		statusUpdates[slug] = status
		mu.Unlock()
		return nil
	}

	ctx, cancel := context.WithCancel(context.Background())
	// Cancel after second poll detects empty queue and tries to sleep.
	go func() {
		// Wait for the item to be fully processed.
		for i := 0; i < 100; i++ {
			time.Sleep(10 * time.Millisecond)
			mu.Lock()
			done := completeCalled
			mu.Unlock()
			if done {
				// Give it a moment to loop back and discover empty queue.
				time.Sleep(20 * time.Millisecond)
				cancel()
				return
			}
		}
		cancel()
	}()

	opts := DaemonOpts{
		RepoPath:       t.TempDir(),
		AutonomyConfig: autonomy.Config{Level: autonomy.LevelAutonomous, MaxAutoRetries: 2},
		PollInterval:   10 * time.Millisecond,
	}

	err := RunDaemon(ctx, opts)
	if err != nil {
		t.Fatalf("RunDaemon returned unexpected error: %v", err)
	}

	if !scoutCalled {
		t.Error("Scout was not called")
	}
	if wavesFinalized != 2 {
		t.Errorf("expected 2 waves finalized, got %d", wavesFinalized)
	}
	if !completeCalled {
		t.Error("MarkIMPLComplete was not called")
	}
	if statusUpdates["test-feature"] != "complete" {
		t.Errorf("expected queue status 'complete', got %q", statusUpdates["test-feature"])
	}
}

func TestRunDaemon_StopsOnCancel(t *testing.T) {
	setupDaemonMocks(t)

	ctx, cancel := context.WithCancel(context.Background())

	// Cancel immediately before any poll completes.
	cancel()

	opts := DaemonOpts{
		RepoPath:       t.TempDir(),
		AutonomyConfig: autonomy.DefaultConfig(),
		PollInterval:   time.Second,
	}

	err := RunDaemon(ctx, opts)
	if err != nil {
		t.Fatalf("RunDaemon should return nil on cancel, got: %v", err)
	}
}

func TestRunDaemon_EmptyQueue(t *testing.T) {
	setupDaemonMocks(t)

	pollCount := 0
	var mu sync.Mutex

	daemonCheckQueueFunc = func(ctx context.Context, opts CheckQueueOpts) (*CheckQueueResult, error) {
		mu.Lock()
		pollCount++
		n := pollCount
		mu.Unlock()
		if n >= 3 {
			// Cancel after 3 polls to stop the test.
			return &CheckQueueResult{Advanced: false, Reason: "no_items"}, nil
		}
		return &CheckQueueResult{Advanced: false, Reason: "no_items"}, nil
	}

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		for {
			time.Sleep(5 * time.Millisecond)
			mu.Lock()
			n := pollCount
			mu.Unlock()
			if n >= 3 {
				cancel()
				return
			}
		}
	}()

	opts := DaemonOpts{
		RepoPath:       t.TempDir(),
		AutonomyConfig: autonomy.DefaultConfig(),
		PollInterval:   5 * time.Millisecond,
	}

	err := RunDaemon(ctx, opts)
	if err != nil {
		t.Fatalf("RunDaemon should return nil, got: %v", err)
	}

	mu.Lock()
	finalCount := pollCount
	mu.Unlock()
	if finalCount < 3 {
		t.Errorf("expected at least 3 poll cycles, got %d", finalCount)
	}
}

func TestRunDaemon_AutoRemediatesOnFailure(t *testing.T) {
	setupDaemonMocks(t)

	callCount := 0
	var mu sync.Mutex

	daemonCheckQueueFunc = func(ctx context.Context, opts CheckQueueOpts) (*CheckQueueResult, error) {
		mu.Lock()
		callCount++
		n := callCount
		mu.Unlock()
		if n == 1 {
			return &CheckQueueResult{
				Advanced:  true,
				NextSlug:  "broken-feature",
				NextTitle: "Broken Feature",
				Reason:    "triggered",
			}, nil
		}
		return &CheckQueueResult{Advanced: false, Reason: "no_items"}, nil
	}

	// Wave 1 fails initially.
	daemonFinalizeWaveFunc = func(ctx context.Context, opts FinalizeWaveOpts) (*FinalizeWaveResult, error) {
		return &FinalizeWaveResult{
			Wave:        opts.WaveNum,
			BuildPassed: false,
			Success:     false,
		}, fmt.Errorf("build failed")
	}

	remediateCalled := false
	daemonAutoRemediateFunc = func(ctx context.Context, opts AutoRemediateOpts) (*AutoRemediateResult, error) {
		remediateCalled = true
		if opts.WaveNum != 1 {
			t.Errorf("expected wave 1, got %d", opts.WaveNum)
		}
		return &AutoRemediateResult{Fixed: true, Attempts: 1}, nil
	}

	completeCalled := false
	daemonMarkIMPLCompleteFunc = func(ctx context.Context, opts MarkIMPLCompleteOpts) error {
		completeCalled = true
		return nil
	}

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		for i := 0; i < 200; i++ {
			time.Sleep(10 * time.Millisecond)
			mu.Lock()
			n := callCount
			mu.Unlock()
			if n >= 2 {
				time.Sleep(20 * time.Millisecond)
				cancel()
				return
			}
		}
		cancel()
	}()

	opts := DaemonOpts{
		RepoPath:       t.TempDir(),
		AutonomyConfig: autonomy.Config{Level: autonomy.LevelAutonomous, MaxAutoRetries: 2},
		PollInterval:   10 * time.Millisecond,
	}

	_ = RunDaemon(ctx, opts)

	if !remediateCalled {
		t.Error("AutoRemediate was not called after finalize failure")
	}
	if !completeCalled {
		t.Error("MarkIMPLComplete should be called after successful remediation")
	}
}

func TestRunDaemon_PausesForReview(t *testing.T) {
	setupDaemonMocks(t)

	callCount := 0
	var mu sync.Mutex

	daemonCheckQueueFunc = func(ctx context.Context, opts CheckQueueOpts) (*CheckQueueResult, error) {
		mu.Lock()
		callCount++
		mu.Unlock()
		return &CheckQueueResult{
			Advanced:  true,
			NextSlug:  "review-me",
			NextTitle: "Review Me",
			Reason:    "triggered",
		}, nil
	}

	var events []string
	var eventMu sync.Mutex

	ctx, cancel := context.WithCancel(context.Background())
	// Cancel after a short delay to unblock the awaiting_review wait.
	go func() {
		time.Sleep(100 * time.Millisecond)
		cancel()
	}()

	opts := DaemonOpts{
		RepoPath: t.TempDir(),
		// Supervised: impl_review is NOT auto-approved, so daemon should pause.
		AutonomyConfig: autonomy.Config{Level: autonomy.LevelSupervised, MaxAutoRetries: 2},
		PollInterval:   10 * time.Millisecond,
		OnEvent: func(e Event) {
			eventMu.Lock()
			events = append(events, e.Event)
			eventMu.Unlock()
		},
	}

	_ = RunDaemon(ctx, opts)

	eventMu.Lock()
	defer eventMu.Unlock()

	foundAwaiting := false
	for _, ev := range events {
		if ev == "daemon_awaiting_review" {
			foundAwaiting = true
			break
		}
	}
	if !foundAwaiting {
		t.Errorf("expected daemon_awaiting_review event, got events: %v", events)
	}
}

func TestRunDaemon_EmitsEvents(t *testing.T) {
	setupDaemonMocks(t)

	callCount := 0
	var mu sync.Mutex

	daemonCheckQueueFunc = func(ctx context.Context, opts CheckQueueOpts) (*CheckQueueResult, error) {
		mu.Lock()
		callCount++
		n := callCount
		mu.Unlock()
		if n == 1 {
			return &CheckQueueResult{
				Advanced:  true,
				NextSlug:  "event-test",
				NextTitle: "Event Test",
				Reason:    "triggered",
			}, nil
		}
		return &CheckQueueResult{Advanced: false, Reason: "no_items"}, nil
	}

	var events []string
	var eventMu sync.Mutex

	completeDone := make(chan struct{})
	daemonMarkIMPLCompleteFunc = func(ctx context.Context, opts MarkIMPLCompleteOpts) error {
		close(completeDone)
		return nil
	}

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		select {
		case <-completeDone:
			time.Sleep(30 * time.Millisecond)
		case <-time.After(2 * time.Second):
		}
		cancel()
	}()

	opts := DaemonOpts{
		RepoPath:       t.TempDir(),
		AutonomyConfig: autonomy.Config{Level: autonomy.LevelAutonomous, MaxAutoRetries: 2},
		PollInterval:   10 * time.Millisecond,
		OnEvent: func(e Event) {
			eventMu.Lock()
			events = append(events, e.Event)
			eventMu.Unlock()
		},
	}

	_ = RunDaemon(ctx, opts)

	eventMu.Lock()
	defer eventMu.Unlock()

	// Verify key events were emitted.
	required := []string{"daemon_started", "daemon_poll", "daemon_processing", "daemon_wave_complete", "daemon_impl_complete", "daemon_stopped"}
	for _, req := range required {
		found := false
		for _, ev := range events {
			if ev == req {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("missing required event %q in events: %v", req, events)
		}
	}
}
