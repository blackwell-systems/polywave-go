package orchestrator

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/agent"
	"github.com/blackwell-systems/scout-and-wave-go/pkg/agent/backend"
	"github.com/blackwell-systems/scout-and-wave-go/pkg/protocol"
	"github.com/blackwell-systems/scout-and-wave-go/pkg/types"
	"github.com/blackwell-systems/scout-and-wave-go/pkg/worktree"
)

// makeOrch is a helper that returns a fresh Orchestrator backed by an
// empty IMPLDoc so tests have no pkg/protocol dependency.
func makeOrch() *Orchestrator {
	return newFromDoc(&types.IMPLDoc{}, "/repo", "/repo/IMPL.md")
}

// makeOrchWithWave returns an Orchestrator whose IMPLDoc contains one wave
// with the provided agents.
func makeOrchWithWave(waveNum int, letters ...string) *Orchestrator {
	agents := make([]types.AgentSpec, len(letters))
	for i, l := range letters {
		agents[i] = types.AgentSpec{Letter: l, Prompt: "do work"}
	}
	doc := &types.IMPLDoc{
		Waves: []types.Wave{
			{Number: waveNum, Agents: agents},
		},
	}
	return newFromDoc(doc, "/repo", "/repo/IMPL.md")
}

// fakeBackend is a backend.Backend test double that records Run calls and
// optionally returns an error for a specific agent prompt.
type fakeBackend struct {
	mu         sync.Mutex
	called     []string
	failPrompt string
	// runFn, if non-nil, is called instead of the default behaviour.
	runFn func(systemPrompt string) (string, error)
}

func (f *fakeBackend) RunStreaming(_ context.Context, systemPrompt, _, _ string, _ backend.ChunkCallback) (string, error) {
	return f.Run(context.Background(), systemPrompt, "", "")
}

func (f *fakeBackend) RunStreamingWithTools(_ context.Context, systemPrompt, _, _ string, _ backend.ChunkCallback, _ backend.ToolCallCallback) (string, error) {
	return f.Run(context.Background(), systemPrompt, "", "")
}


func (f *fakeBackend) Run(_ context.Context, systemPrompt, _, _ string) (string, error) {
	f.mu.Lock()
	f.called = append(f.called, systemPrompt)
	fn := f.runFn
	failPrompt := f.failPrompt
	f.mu.Unlock()

	if fn != nil {
		return fn(systemPrompt)
	}
	if failPrompt != "" && systemPrompt == failPrompt {
		return "", errors.New("simulated agent failure")
	}
	return "response", nil
}

// TestOrchestratorNew verifies that New returns a valid orchestrator in ScoutPending state.
func TestOrchestratorNew(t *testing.T) {
	orig := parseIMPLDocFunc
	t.Cleanup(func() { parseIMPLDocFunc = orig })
	parseIMPLDocFunc = func(_ string) (*types.IMPLDoc, error) {
		return &types.IMPLDoc{FeatureName: "test"}, nil
	}

	o, err := New("/repo", "/repo/IMPL.md")
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}
	if o == nil {
		t.Fatal("New returned nil orchestrator")
	}
	if o.State() != protocol.StateScoutPending {
		t.Errorf("initial state = %s, want ScoutPending", o.State())
	}
	if o.IMPLDoc().FeatureName != "test" {
		t.Errorf("FeatureName = %q, want %q", o.IMPLDoc().FeatureName, "test")
	}
}

// TestRunWaveNilDoc verifies that RunWave returns an error when no waves are defined.
// (Uses an empty doc with no Waves, then requests wave 1 — not found error.)
func TestRunWaveNilDoc(t *testing.T) {
	// Orchestrator with a doc that has waves defined — request a non-existent wave.
	o := makeOrchWithWave(1, "A")
	err := o.RunWave(99)
	if err == nil {
		t.Fatal("expected error for missing wave 99, got nil")
	}
	if !strings.Contains(err.Error(), "99") {
		t.Errorf("error should mention wave number 99, got: %v", err)
	}
}

// TestNew_LoadsDoc verifies that New returns a non-nil Orchestrator in
// ScoutPending state when parseIMPLDocFunc succeeds.
func TestNew_LoadsDoc(t *testing.T) {
	orig := parseIMPLDocFunc
	t.Cleanup(func() { parseIMPLDocFunc = orig })
	parseIMPLDocFunc = func(_ string) (*types.IMPLDoc, error) {
		return &types.IMPLDoc{FeatureName: "test"}, nil
	}

	o, err := New("/repo", "/repo/IMPL.md")
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}
	if o == nil {
		t.Fatal("New returned nil orchestrator")
	}
	if o.State() != protocol.StateScoutPending {
		t.Errorf("initial state = %s, want ScoutPending", o.State())
	}
	if o.IMPLDoc().FeatureName != "test" {
		t.Errorf("FeatureName = %q, want %q", o.IMPLDoc().FeatureName, "test")
	}
}

// TestSetValidateInvariantsFunc verifies the func is replaced and called.
func TestSetValidateInvariantsFunc(t *testing.T) {
	orig := validateInvariantsFunc
	t.Cleanup(func() { validateInvariantsFunc = orig })

	called := false
	SetValidateInvariantsFunc(func(_ *types.IMPLDoc) error {
		called = true
		return nil
	})
	_ = validateInvariantsFunc(nil)
	if !called {
		t.Error("SetValidateInvariantsFunc: replacement was not called")
	}
}

// TestMergeWave_DelegatesTo_mergeWaveFunc verifies that MergeWave delegates
// to mergeWaveFunc and propagates its return value.
func TestMergeWave_DelegatesTo_mergeWaveFunc(t *testing.T) {
	orig := mergeWaveFunc
	t.Cleanup(func() { mergeWaveFunc = orig })

	var gotWave int
	mergeWaveFunc = func(_ *Orchestrator, waveNum int) error {
		gotWave = waveNum
		return nil
	}

	o := makeOrch()
	if err := o.MergeWave(3); err != nil {
		t.Fatalf("MergeWave returned error: %v", err)
	}
	if gotWave != 3 {
		t.Errorf("mergeWaveFunc called with wave %d, want 3", gotWave)
	}
}

// TestRunVerification_DelegatesTo_runVerificationFunc verifies delegation.
func TestRunVerification_DelegatesTo_runVerificationFunc(t *testing.T) {
	orig := runVerificationFunc
	t.Cleanup(func() { runVerificationFunc = orig })

	var gotCmd string
	runVerificationFunc = func(_ *Orchestrator, cmd string) error {
		gotCmd = cmd
		return nil
	}

	o := makeOrch()
	if err := o.RunVerification("go test ./..."); err != nil {
		t.Fatalf("RunVerification returned error: %v", err)
	}
	if gotCmd != "go test ./..." {
		t.Errorf("runVerificationFunc called with %q, want %q", gotCmd, "go test ./...")
	}
}

// TestRunWave_WaveNotFound verifies that RunWave returns an error when the
// requested wave number is absent from the IMPL doc.
func TestRunWave_WaveNotFound(t *testing.T) {
	o := makeOrchWithWave(1, "A", "B")
	err := o.RunWave(99)
	if err == nil {
		t.Fatal("expected error for missing wave 99, got nil")
	}
	if !strings.Contains(err.Error(), "99") {
		t.Errorf("error should mention wave number 99, got: %v", err)
	}
}

// TestRunWave_LaunchesAllAgents verifies that RunWave calls Execute
// for every agent in the wave, and does so concurrently.
func TestRunWave_LaunchesAllAgents(t *testing.T) {
	var inFlight int32
	barrier := make(chan struct{})

	fake := &fakeBackend{}
	fake.runFn = func(systemPrompt string) (string, error) {
		if atomic.AddInt32(&inFlight, 1) == 2 {
			close(barrier)
		}
		select {
		case <-barrier:
		case <-time.After(5 * time.Second):
			return "", errors.New("barrier timeout: agents did not run concurrently")
		}
		return "response", nil
	}

	var worktreeCount int32
	origCreator := worktreeCreatorFunc
	origWait := waitForCompletionFunc
	origNewBackend := newBackendFunc
	origNewRunner := newRunnerFunc
	t.Cleanup(func() {
		worktreeCreatorFunc = origCreator
		waitForCompletionFunc = origWait
		newBackendFunc = origNewBackend
		newRunnerFunc = origNewRunner
	})
	worktreeCreatorFunc = func(_ *worktree.Manager, _ int, letter string) (string, error) {
		atomic.AddInt32(&worktreeCount, 1)
		return "/tmp/fake-wt-" + letter, nil
	}
	waitForCompletionFunc = func(_, _ string, _, _ time.Duration) (*protocol.CompletionReport, error) {
		return &protocol.CompletionReport{Status: "complete"}, nil
	}
	newBackendFunc = func(_ BackendConfig) (backend.Backend, error) {
		return fake, nil
	}
	newRunnerFunc = func(b backend.Backend, wm *worktree.Manager) *agent.Runner {
		return agent.NewRunner(b, wm)
	}

	doc := &types.IMPLDoc{
		Waves: []types.Wave{
			{
				Number: 1,
				Agents: []types.AgentSpec{
					{Letter: "A", Prompt: "A"},
					{Letter: "B", Prompt: "B"},
				},
			},
		},
	}
	o := newFromDoc(doc, "/repo", "/repo/IMPL.md")

	if err := o.RunWave(1); err != nil {
		t.Fatalf("RunWave returned unexpected error: %v", err)
	}

	if n := atomic.LoadInt32(&worktreeCount); n != 2 {
		t.Errorf("expected 2 worktrees created, got %d", n)
	}

	fake.mu.Lock()
	calledCount := len(fake.called)
	fake.mu.Unlock()
	if calledCount != 2 {
		t.Errorf("expected 2 Execute calls, got %d", calledCount)
	}
}

// TestRunWave_ReturnsErrorOnAgentFailure verifies that RunWave propagates an
// error when one agent's Execute call fails.
func TestRunWave_ReturnsErrorOnAgentFailure(t *testing.T) {
	fake := &fakeBackend{failPrompt: "B"}

	origCreator := worktreeCreatorFunc
	origWait := waitForCompletionFunc
	origNewBackend := newBackendFunc
	origNewRunner := newRunnerFunc
	t.Cleanup(func() {
		worktreeCreatorFunc = origCreator
		waitForCompletionFunc = origWait
		newBackendFunc = origNewBackend
		newRunnerFunc = origNewRunner
	})
	worktreeCreatorFunc = func(_ *worktree.Manager, _ int, letter string) (string, error) {
		return "/tmp/fake-wt-" + letter, nil
	}
	waitForCompletionFunc = func(_, _ string, _, _ time.Duration) (*protocol.CompletionReport, error) {
		return &protocol.CompletionReport{Status: "complete"}, nil
	}
	newBackendFunc = func(_ BackendConfig) (backend.Backend, error) {
		return fake, nil
	}
	newRunnerFunc = func(b backend.Backend, wm *worktree.Manager) *agent.Runner {
		return agent.NewRunner(b, wm)
	}

	doc := &types.IMPLDoc{
		Waves: []types.Wave{
			{
				Number: 1,
				Agents: []types.AgentSpec{
					{Letter: "A", Prompt: "A"},
					{Letter: "B", Prompt: "B"},
				},
			},
		},
	}
	o := newFromDoc(doc, "/repo", "/repo/IMPL.md")

	err := o.RunWave(1)
	if err == nil {
		t.Fatal("expected error when agent B fails, got nil")
	}
	if !strings.Contains(err.Error(), "simulated agent failure") {
		t.Errorf("error should contain the agent failure message, got: %v", err)
	}
}

// TestTransitionTo_ValidTransitions exercises every edge in the state graph.
func TestTransitionTo_ValidTransitions(t *testing.T) {
	cases := []struct {
		from protocol.ProtocolState
		to   protocol.ProtocolState
	}{
		{protocol.StateScoutPending, protocol.StateReviewed},
		{protocol.StateScoutPending, protocol.StateNotSuitable},
		{protocol.StateReviewed, protocol.StateWavePending},
		{protocol.StateWavePending, protocol.StateWaveExecuting},
		{protocol.StateWaveExecuting, protocol.StateWaveMerging},
		{protocol.StateWaveMerging, protocol.StateWaveVerified},
		{protocol.StateWaveVerified, protocol.StateComplete},
		{protocol.StateWaveVerified, protocol.StateWavePending},
	}

	for _, tc := range cases {
		o := makeOrch()
		o.state = tc.from

		if err := o.TransitionTo(tc.to); err != nil {
			t.Errorf("TransitionTo(%s -> %s): unexpected error: %v", tc.from, tc.to, err)
		}
		if o.State() != tc.to {
			t.Errorf("after TransitionTo(%s -> %s): state is %s, want %s",
				tc.from, tc.to, o.State(), tc.to)
		}
	}
}

// TestTransitionTo_InvalidTransition verifies that an illegal transition returns an error.
func TestTransitionTo_InvalidTransition(t *testing.T) {
	o := makeOrch()
	err := o.TransitionTo(protocol.StateComplete)
	if err == nil {
		t.Fatal("expected error for SCOUT_PENDING -> COMPLETE, got nil")
	}
	if !strings.Contains(err.Error(), "SCOUT_PENDING") {
		t.Errorf("error should mention source state SCOUT_PENDING, got: %v", err)
	}
	if !strings.Contains(err.Error(), "COMPLETE") {
		t.Errorf("error should mention target state COMPLETE, got: %v", err)
	}
	if o.State() != protocol.StateScoutPending {
		t.Errorf("state changed after invalid transition: got %s", o.State())
	}
}

// TestTransitionTo_TerminalState verifies that NotSuitable and Complete cannot be exited.
func TestTransitionTo_TerminalState(t *testing.T) {
	terminalStates := []protocol.ProtocolState{protocol.StateNotSuitable, protocol.StateComplete}
	allStates := []protocol.ProtocolState{
		protocol.StateScoutPending,
		protocol.StateReviewed,
		protocol.StateWavePending,
		protocol.StateWaveExecuting,
		protocol.StateWaveVerified,
		protocol.StateNotSuitable,
		protocol.StateComplete,
	}

	for _, terminal := range terminalStates {
		for _, target := range allStates {
			o := makeOrch()
			o.state = terminal
			err := o.TransitionTo(target)
			if err == nil {
				t.Errorf("expected error transitioning from terminal state %s to %s, got nil",
					terminal, target)
			}
		}
	}
}

// TestIsValidTransition unit-tests the guard function directly.
func TestIsValidTransition(t *testing.T) {
	valid := []struct{ from, to protocol.ProtocolState }{
		{protocol.StateScoutPending, protocol.StateReviewed},
		{protocol.StateScoutPending, protocol.StateNotSuitable},
		{protocol.StateReviewed, protocol.StateWavePending},
		{protocol.StateWavePending, protocol.StateWaveExecuting},
		{protocol.StateWaveExecuting, protocol.StateWaveMerging},
		{protocol.StateWaveMerging, protocol.StateWaveVerified},
		{protocol.StateWaveVerified, protocol.StateComplete},
		{protocol.StateWaveVerified, protocol.StateWavePending},
	}
	for _, tc := range valid {
		if !isValidTransition(tc.from, tc.to) {
			t.Errorf("isValidTransition(%s, %s) = false, want true", tc.from, tc.to)
		}
	}

	invalid := []struct{ from, to protocol.ProtocolState }{
		{protocol.StateScoutPending, protocol.StateComplete},
		{protocol.StateScoutPending, protocol.StateWaveExecuting},
		{protocol.StateReviewed, protocol.StateComplete},
		{protocol.StateNotSuitable, protocol.StateReviewed},
		{protocol.StateComplete, protocol.StateWavePending},
		{protocol.StateWaveExecuting, protocol.StateWavePending},
	}
	for _, tc := range invalid {
		if isValidTransition(tc.from, tc.to) {
			t.Errorf("isValidTransition(%s, %s) = true, want false", tc.from, tc.to)
		}
	}
}

// TestNewFromDoc verifies that newFromDoc sets the initial state correctly.
func TestNewFromDoc(t *testing.T) {
	doc := &types.IMPLDoc{
		FeatureName: "test-feature",
		Status:      "pending",
	}
	o := newFromDoc(doc, "/some/repo", "/some/repo/IMPL.md")

	if o.State() != protocol.StateScoutPending {
		t.Errorf("initial state: got %s, want ScoutPending", o.State())
	}
	if o.IMPLDoc() != doc {
		t.Error("IMPLDoc() did not return the same pointer passed to newFromDoc")
	}
	if o.RepoPath() != "/some/repo" {
		t.Errorf("RepoPath(): got %q, want %q", o.RepoPath(), "/some/repo")
	}
	if o.implDocPath != "/some/repo/IMPL.md" {
		t.Errorf("implDocPath: got %q, want %q", o.implDocPath, "/some/repo/IMPL.md")
	}
}

// TestSetEventPublisher_NilPublisher_NoOp verifies that RunWave works correctly
// when no EventPublisher has been set (no panic on publish calls).
func TestSetEventPublisher_NilPublisher_NoOp(t *testing.T) {
	origCreator := worktreeCreatorFunc
	origWait := waitForCompletionFunc
	origNewBackend := newBackendFunc
	origNewRunner := newRunnerFunc
	t.Cleanup(func() {
		worktreeCreatorFunc = origCreator
		waitForCompletionFunc = origWait
		newBackendFunc = origNewBackend
		newRunnerFunc = origNewRunner
	})

	worktreeCreatorFunc = func(_ *worktree.Manager, _ int, letter string) (string, error) {
		return "/tmp/fake-wt-" + letter, nil
	}
	waitForCompletionFunc = func(_, _ string, _, _ time.Duration) (*protocol.CompletionReport, error) {
		return &protocol.CompletionReport{Status: "complete"}, nil
	}

	fake := &fakeBackend{}
	newBackendFunc = func(_ BackendConfig) (backend.Backend, error) {
		return fake, nil
	}
	newRunnerFunc = func(b backend.Backend, wm *worktree.Manager) *agent.Runner {
		return agent.NewRunner(b, wm)
	}

	o := makeOrchWithWave(1, "A")

	if err := o.RunWave(1); err != nil {
		t.Fatalf("RunWave with nil publisher returned unexpected error: %v", err)
	}
}

// TestPublish_EmitsAgentStarted verifies that an injected EventPublisher
// receives an "agent_started" event when RunWave launches an agent.
func TestPublish_EmitsAgentStarted(t *testing.T) {
	origCreator := worktreeCreatorFunc
	origWait := waitForCompletionFunc
	origNewBackend := newBackendFunc
	origNewRunner := newRunnerFunc
	t.Cleanup(func() {
		worktreeCreatorFunc = origCreator
		waitForCompletionFunc = origWait
		newBackendFunc = origNewBackend
		newRunnerFunc = origNewRunner
	})

	worktreeCreatorFunc = func(_ *worktree.Manager, _ int, letter string) (string, error) {
		return "/tmp/fake-wt-" + letter, nil
	}
	waitForCompletionFunc = func(_, _ string, _, _ time.Duration) (*protocol.CompletionReport, error) {
		return &protocol.CompletionReport{Status: "complete"}, nil
	}

	fake := &fakeBackend{}
	newBackendFunc = func(_ BackendConfig) (backend.Backend, error) {
		return fake, nil
	}
	newRunnerFunc = func(b backend.Backend, wm *worktree.Manager) *agent.Runner {
		return agent.NewRunner(b, wm)
	}

	var mu sync.Mutex
	var received []OrchestratorEvent
	publisher := func(ev OrchestratorEvent) {
		mu.Lock()
		received = append(received, ev)
		mu.Unlock()
	}

	o := makeOrchWithWave(1, "A")
	o.SetEventPublisher(publisher)

	if err := o.RunWave(1); err != nil {
		t.Fatalf("RunWave returned unexpected error: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	var found bool
	for _, ev := range received {
		if ev.Event == "agent_started" {
			found = true
			payload, ok := ev.Data.(AgentStartedPayload)
			if !ok {
				t.Errorf("agent_started Data is %T, want AgentStartedPayload", ev.Data)
				break
			}
			if payload.Agent != "A" {
				t.Errorf("agent_started payload.Agent = %q, want %q", payload.Agent, "A")
			}
			if payload.Wave != 1 {
				t.Errorf("agent_started payload.Wave = %d, want 1", payload.Wave)
			}
			break
		}
	}
	if !found {
		t.Errorf("no agent_started event received; got events: %v", received)
	}
}

// TestWtIMPLPath verifies that wtIMPLPath derives the correct IMPL doc path
// inside a worktree by replacing the repo root prefix with the worktree path.
func TestWtIMPLPath(t *testing.T) {
	cases := []struct {
		name        string
		repoPath    string
		implDocPath string
		wtPath      string
		want        string
	}{
		{
			name:        "standard nested path",
			repoPath:    "/repo",
			implDocPath: "/repo/docs/IMPL/IMPL-foo.md",
			wtPath:      "/repo/.claude/worktrees/wave1-agent-A",
			want:        "/repo/.claude/worktrees/wave1-agent-A/docs/IMPL/IMPL-foo.md",
		},
		{
			name:        "IMPL doc at repo root",
			repoPath:    "/repo",
			implDocPath: "/repo/IMPL.md",
			wtPath:      "/repo/.claude/worktrees/wave1-agent-B",
			want:        "/repo/.claude/worktrees/wave1-agent-B/IMPL.md",
		},
		{
			name:        "deeper nesting",
			repoPath:    "/home/user/project",
			implDocPath: "/home/user/project/docs/IMPL/feature/IMPL-bar.md",
			wtPath:      "/home/user/project/.claude/worktrees/wave2-agent-C",
			want:        "/home/user/project/.claude/worktrees/wave2-agent-C/docs/IMPL/feature/IMPL-bar.md",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := wtIMPLPath(tc.repoPath, tc.implDocPath, tc.wtPath)
			if got != tc.want {
				t.Errorf("wtIMPLPath(%q, %q, %q) = %q, want %q",
					tc.repoPath, tc.implDocPath, tc.wtPath, got, tc.want)
			}
		})
	}
}

// TestWtIMPLPath_Fallback verifies wtIMPLPath fallback behaviour.
func TestWtIMPLPath_Fallback(t *testing.T) {
	repoPath := "/repo"
	implDocPath := "/other/IMPL.md"
	wtPath := "/repo/.claude/worktrees/wave1-agent-A"

	got := wtIMPLPath(repoPath, implDocPath, wtPath)
	rel, err := filepath.Rel(repoPath, implDocPath)
	if err != nil {
		if got != implDocPath {
			t.Errorf("fallback case: got %q, want %q (implDocPath)", got, implDocPath)
		}
	} else {
		expected := filepath.Join(wtPath, rel)
		if got != expected {
			t.Errorf("non-error case: got %q, want %q", got, expected)
		}
	}
}

// TestLaunchAgent_PollsWorktreeIMPLDoc verifies that launchAgent passes the
// worktree IMPL doc path to waitForCompletionFunc, not the main repo IMPL path.
func TestLaunchAgent_PollsWorktreeIMPLDoc(t *testing.T) {
	const (
		repoPath    = "/repo"
		implDocPath = "/repo/docs/IMPL/IMPL-feature.md"
		fakeWtPath  = "/repo/.claude/worktrees/wave1-agent-A"
	)
	wantIMPLPath := wtIMPLPath(repoPath, implDocPath, fakeWtPath)

	origCreator := worktreeCreatorFunc
	origWait := waitForCompletionFunc
	origNewBackend := newBackendFunc
	origNewRunner := newRunnerFunc
	t.Cleanup(func() {
		worktreeCreatorFunc = origCreator
		waitForCompletionFunc = origWait
		newBackendFunc = origNewBackend
		newRunnerFunc = origNewRunner
	})

	worktreeCreatorFunc = func(_ *worktree.Manager, _ int, _ string) (string, error) {
		return fakeWtPath, nil
	}

	var gotIMPLPath string
	waitForCompletionFunc = func(implDoc, _ string, _, _ time.Duration) (*protocol.CompletionReport, error) {
		gotIMPLPath = implDoc
		return &protocol.CompletionReport{Status: "complete"}, nil
	}

	fake := &fakeBackend{}
	newBackendFunc = func(_ BackendConfig) (backend.Backend, error) {
		return fake, nil
	}
	newRunnerFunc = func(b backend.Backend, wm *worktree.Manager) *agent.Runner {
		return agent.NewRunner(b, wm)
	}

	doc := &types.IMPLDoc{
		Waves: []types.Wave{
			{
				Number: 1,
				Agents: []types.AgentSpec{
					{Letter: "A", Prompt: "do work"},
				},
			},
		},
	}
	o := newFromDoc(doc, repoPath, implDocPath)

	if err := o.RunWave(1); err != nil {
		t.Fatalf("RunWave returned unexpected error: %v", err)
	}

	if gotIMPLPath != wantIMPLPath {
		t.Errorf("waitForCompletionFunc received implDocPath = %q, want %q (worktree path)",
			gotIMPLPath, wantIMPLPath)
	}
	if gotIMPLPath == implDocPath {
		t.Errorf("waitForCompletionFunc received main repo IMPL path %q — should be worktree path", implDocPath)
	}
}

// TestLaunchAgentE23FallbackOnExtractError verifies that when ExtractAgentContext
// fails (e.g. agent letter not found in IMPL doc), launchAgent still runs the
// agent with its original prompt and does not panic.
func TestLaunchAgentE23FallbackOnExtractError(t *testing.T) {
	origCreator := worktreeCreatorFunc
	origWait := waitForCompletionFunc
	origNewBackend := newBackendFunc
	origNewRunner := newRunnerFunc
	t.Cleanup(func() {
		worktreeCreatorFunc = origCreator
		waitForCompletionFunc = origWait
		newBackendFunc = origNewBackend
		newRunnerFunc = origNewRunner
	})

	worktreeCreatorFunc = func(_ *worktree.Manager, _ int, letter string) (string, error) {
		return "/tmp/fake-wt-" + letter, nil
	}
	waitForCompletionFunc = func(_, _ string, _, _ time.Duration) (*protocol.CompletionReport, error) {
		return &protocol.CompletionReport{Status: "complete"}, nil
	}

	var capturedPrompt string
	fake := &fakeBackend{}
	fake.runFn = func(systemPrompt string) (string, error) {
		capturedPrompt = systemPrompt
		return "response", nil
	}
	newBackendFunc = func(_ BackendConfig) (backend.Backend, error) {
		return fake, nil
	}
	newRunnerFunc = func(b backend.Backend, wm *worktree.Manager) *agent.Runner {
		return agent.NewRunner(b, wm)
	}

	// IMPL doc path does not exist, so ExtractAgentContext will fail.
	// The agent should still run with its original prompt (fallback).
	const originalPrompt = "original agent prompt"
	doc := &types.IMPLDoc{
		Waves: []types.Wave{
			{
				Number: 1,
				Agents: []types.AgentSpec{
					{Letter: "Z", Prompt: originalPrompt},
				},
			},
		},
	}
	// Use a non-existent implDocPath so ExtractAgentContext always errors.
	o := newFromDoc(doc, "/repo", "/nonexistent/IMPL.md")

	if err := o.RunWave(1); err != nil {
		t.Fatalf("RunWave returned unexpected error: %v", err)
	}

	// The backend should have been called with the original prompt (fallback).
	if capturedPrompt != originalPrompt {
		t.Errorf("E23 fallback: backend received prompt %q, want original %q", capturedPrompt, originalPrompt)
	}
}

// TestLaunchAgentE19BlockedEvent verifies that when an agent reports blocked status,
// an "agent_blocked" event is published with the correct RouteFailure action.
func TestLaunchAgentE19BlockedEvent(t *testing.T) {
	origCreator := worktreeCreatorFunc
	origWait := waitForCompletionFunc
	origNewBackend := newBackendFunc
	origNewRunner := newRunnerFunc
	t.Cleanup(func() {
		worktreeCreatorFunc = origCreator
		waitForCompletionFunc = origWait
		newBackendFunc = origNewBackend
		newRunnerFunc = origNewRunner
	})

	worktreeCreatorFunc = func(_ *worktree.Manager, _ int, letter string) (string, error) {
		return "/tmp/fake-wt-" + letter, nil
	}
	waitForCompletionFunc = func(_, _ string, _, _ time.Duration) (*protocol.CompletionReport, error) {
		return &protocol.CompletionReport{
			Status:      "blocked",
			FailureType: "transient",
		}, nil
	}

	fake := &fakeBackend{}
	newBackendFunc = func(_ BackendConfig) (backend.Backend, error) {
		return fake, nil
	}
	newRunnerFunc = func(b backend.Backend, wm *worktree.Manager) *agent.Runner {
		return agent.NewRunner(b, wm)
	}

	var mu sync.Mutex
	var received []OrchestratorEvent
	o := makeOrchWithWave(1, "A")
	o.SetEventPublisher(func(ev OrchestratorEvent) {
		mu.Lock()
		received = append(received, ev)
		mu.Unlock()
	})

	if err := o.RunWave(1); err != nil {
		t.Fatalf("RunWave returned unexpected error: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	var blockedEv *OrchestratorEvent
	for i := range received {
		if received[i].Event == "agent_blocked" {
			blockedEv = &received[i]
			break
		}
	}
	if blockedEv == nil {
		t.Fatal("expected agent_blocked event, got none")
	}
	payload, ok := blockedEv.Data.(AgentBlockedPayload)
	if !ok {
		t.Fatalf("agent_blocked Data is %T, want AgentBlockedPayload", blockedEv.Data)
	}
	if payload.Agent != "A" {
		t.Errorf("agent_blocked payload.Agent = %q, want A", payload.Agent)
	}
	if payload.Status != "blocked" {
		t.Errorf("agent_blocked payload.Status = %q, want blocked", payload.Status)
	}
	if payload.Action != ActionRetry {
		t.Errorf("agent_blocked payload.Action = %v, want ActionRetry (transient failure)", payload.Action)
	}
}

// TestParseProviderPrefix_WithPrefix verifies "openai:gpt-4o" splits correctly.
func TestParseProviderPrefix_WithPrefix(t *testing.T) {
	provider, bareModel := parseProviderPrefix("openai:gpt-4o")
	if provider != "openai" {
		t.Errorf("provider = %q, want %q", provider, "openai")
	}
	if bareModel != "gpt-4o" {
		t.Errorf("bareModel = %q, want %q", bareModel, "gpt-4o")
	}
}

// TestParseProviderPrefix_NoPrefix verifies "gpt-4o" (no colon) returns empty provider.
func TestParseProviderPrefix_NoPrefix(t *testing.T) {
	provider, bareModel := parseProviderPrefix("gpt-4o")
	if provider != "" {
		t.Errorf("provider = %q, want %q", provider, "")
	}
	if bareModel != "gpt-4o" {
		t.Errorf("bareModel = %q, want %q", bareModel, "gpt-4o")
	}
}

// TestParseProviderPrefix_CLIPrefix verifies "cli:kimi" splits correctly.
func TestParseProviderPrefix_CLIPrefix(t *testing.T) {
	provider, bareModel := parseProviderPrefix("cli:kimi")
	if provider != "cli" {
		t.Errorf("provider = %q, want %q", provider, "cli")
	}
	if bareModel != "kimi" {
		t.Errorf("bareModel = %q, want %q", bareModel, "kimi")
	}
}

// TestNewBackendFunc_OpenAIKind verifies BackendConfig{Kind:"openai"} produces a backend without error.
func TestNewBackendFunc_OpenAIKind(t *testing.T) {
	b, err := newBackendFunc(BackendConfig{Kind: "openai", Model: "gpt-4o"})
	if err != nil {
		t.Fatalf("newBackendFunc returned error: %v", err)
	}
	if b == nil {
		t.Fatal("newBackendFunc returned nil backend")
	}
}

// TestNewBackendFunc_OpenAIPrefix verifies BackendConfig{Kind:"auto", Model:"openai:gpt-4o"} produces a backend without error.
func TestNewBackendFunc_OpenAIPrefix(t *testing.T) {
	b, err := newBackendFunc(BackendConfig{Kind: "auto", Model: "openai:gpt-4o"})
	if err != nil {
		t.Fatalf("newBackendFunc returned error: %v", err)
	}
	if b == nil {
		t.Fatal("newBackendFunc returned nil backend")
	}
}

// TestState_String verifies that each state constant has the expected string value.
func TestState_String(t *testing.T) {
	cases := []struct {
		state protocol.ProtocolState
		want  string
	}{
		{protocol.StateScoutPending, "SCOUT_PENDING"},
		{protocol.StateNotSuitable, "NOT_SUITABLE"},
		{protocol.StateReviewed, "REVIEWED"},
		{protocol.StateWavePending, "WAVE_PENDING"},
		{protocol.StateWaveExecuting, "WAVE_EXECUTING"},
		{protocol.StateWaveVerified, "WAVE_VERIFIED"},
		{protocol.StateComplete, "COMPLETE"},
	}

	for _, tc := range cases {
		got := string(tc.state)
		if got != tc.want {
			t.Errorf("State constant = %q, want %q", got, tc.want)
		}
	}
}

// TestRunWave_AgentPrioritization verifies that prioritizeAgentsFunc is called and used.
func TestRunWave_AgentPrioritization(t *testing.T) {
	origCreator := worktreeCreatorFunc
	origWait := waitForCompletionFunc
	origNewBackend := newBackendFunc
	origNewRunner := newRunnerFunc
	origPrioritize := prioritizeAgentsFunc
	t.Cleanup(func() {
		worktreeCreatorFunc = origCreator
		waitForCompletionFunc = origWait
		newBackendFunc = origNewBackend
		newRunnerFunc = origNewRunner
		prioritizeAgentsFunc = origPrioritize
	})

	// Track which agents were launched
	var launchedAgents []string
	var mu sync.Mutex

	worktreeCreatorFunc = func(_ *worktree.Manager, _ int, letter string) (string, error) {
		mu.Lock()
		launchedAgents = append(launchedAgents, letter)
		mu.Unlock()
		return "/tmp/fake-wt-" + letter, nil
	}
	waitForCompletionFunc = func(_, _ string, _, _ time.Duration) (*protocol.CompletionReport, error) {
		return &protocol.CompletionReport{Status: "complete"}, nil
	}

	fake := &fakeBackend{}
	newBackendFunc = func(_ BackendConfig) (backend.Backend, error) {
		return fake, nil
	}
	newRunnerFunc = func(b backend.Backend, wm *worktree.Manager) *agent.Runner {
		return agent.NewRunner(b, wm)
	}

	// Track that prioritization was called with correct parameters
	var prioritizeCalled atomic.Bool
	var prioritizeWaveNum atomic.Int32

	prioritizeAgentsFunc = func(manifest *types.IMPLDoc, waveNum int) []string {
		prioritizeCalled.Store(true)
		prioritizeWaveNum.Store(int32(waveNum))
		// Return prioritized order (reversed for testing)
		return []string{"C", "B", "A"}
	}

	o := makeOrchWithWave(1, "A", "B", "C")

	if err := o.RunWave(1); err != nil {
		t.Fatalf("RunWave returned error: %v", err)
	}

	// Verify prioritization function was called
	if !prioritizeCalled.Load() {
		t.Error("prioritizeAgentsFunc was not called")
	}
	if prioritizeWaveNum.Load() != 1 {
		t.Errorf("prioritizeAgentsFunc called with wave %d, want 1", prioritizeWaveNum.Load())
	}

	// Verify all agents were launched (order may vary due to concurrency)
	mu.Lock()
	defer mu.Unlock()

	if len(launchedAgents) != 3 {
		t.Fatalf("expected 3 agents to launch, got %d", len(launchedAgents))
	}

	// Verify all expected agents are present (regardless of order)
	expectedAgents := map[string]bool{"A": true, "B": true, "C": true}
	for _, agent := range launchedAgents {
		if !expectedAgents[agent] {
			t.Errorf("unexpected agent launched: %q", agent)
		}
		delete(expectedAgents, agent)
	}
	if len(expectedAgents) > 0 {
		t.Errorf("agents not launched: %v", expectedAgents)
	}
}

// TestRunWave_AgentPrioritizedEvent verifies that the agent_prioritized SSE event is emitted.
func TestRunWave_AgentPrioritizedEvent(t *testing.T) {
	origCreator := worktreeCreatorFunc
	origWait := waitForCompletionFunc
	origNewBackend := newBackendFunc
	origNewRunner := newRunnerFunc
	origPrioritize := prioritizeAgentsFunc
	t.Cleanup(func() {
		worktreeCreatorFunc = origCreator
		waitForCompletionFunc = origWait
		newBackendFunc = origNewBackend
		newRunnerFunc = origNewRunner
		prioritizeAgentsFunc = origPrioritize
	})

	worktreeCreatorFunc = func(_ *worktree.Manager, _ int, letter string) (string, error) {
		return "/tmp/fake-wt-" + letter, nil
	}
	waitForCompletionFunc = func(_, _ string, _, _ time.Duration) (*protocol.CompletionReport, error) {
		return &protocol.CompletionReport{Status: "complete"}, nil
	}

	fake := &fakeBackend{}
	newBackendFunc = func(_ BackendConfig) (backend.Backend, error) {
		return fake, nil
	}
	newRunnerFunc = func(b backend.Backend, wm *worktree.Manager) *agent.Runner {
		return agent.NewRunner(b, wm)
	}

	// Mock prioritization: reverse the agent order
	prioritizeAgentsFunc = func(manifest *types.IMPLDoc, waveNum int) []string {
		return []string{"C", "B", "A"}
	}

	// Capture published events
	var events []OrchestratorEvent
	var mu sync.Mutex

	o := makeOrchWithWave(1, "A", "B", "C")
	o.SetEventPublisher(func(ev OrchestratorEvent) {
		mu.Lock()
		events = append(events, ev)
		mu.Unlock()
	})

	if err := o.RunWave(1); err != nil {
		t.Fatalf("RunWave returned error: %v", err)
	}

	// Find the agent_prioritized event
	mu.Lock()
	defer mu.Unlock()

	var foundEvent *OrchestratorEvent
	for i := range events {
		if events[i].Event == "agent_prioritized" {
			foundEvent = &events[i]
			break
		}
	}

	if foundEvent == nil {
		t.Fatal("agent_prioritized event was not published")
	}

	// Verify payload structure
	payload, ok := foundEvent.Data.(AgentPrioritizedPayload)
	if !ok {
		t.Fatalf("agent_prioritized event Data is not AgentPrioritizedPayload, got %T", foundEvent.Data)
	}

	if payload.Wave != 1 {
		t.Errorf("payload.Wave = %d, want 1", payload.Wave)
	}
	if len(payload.OriginalOrder) != 3 {
		t.Errorf("payload.OriginalOrder length = %d, want 3", len(payload.OriginalOrder))
	}
	if len(payload.PrioritizedOrder) != 3 {
		t.Errorf("payload.PrioritizedOrder length = %d, want 3", len(payload.PrioritizedOrder))
	}

	// Verify original order is A, B, C (declaration order)
	expectedOriginal := []string{"A", "B", "C"}
	for i, expected := range expectedOriginal {
		if payload.OriginalOrder[i] != expected {
			t.Errorf("payload.OriginalOrder[%d] = %q, want %q", i, payload.OriginalOrder[i], expected)
		}
	}

	// Verify prioritized order is C, B, A (reversed)
	expectedPrioritized := []string{"C", "B", "A"}
	for i, expected := range expectedPrioritized {
		if payload.PrioritizedOrder[i] != expected {
			t.Errorf("payload.PrioritizedOrder[%d] = %q, want %q", i, payload.PrioritizedOrder[i], expected)
		}
	}

	if !payload.Reordered {
		t.Error("payload.Reordered = false, want true")
	}
	if payload.Reason != "critical_path_scheduling" {
		t.Errorf("payload.Reason = %q, want %q", payload.Reason, "critical_path_scheduling")
	}
}
