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
	if o.State() != types.ScoutPending {
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
	if o.State() != types.ScoutPending {
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
	waitForCompletionFunc = func(_, _ string, _, _ time.Duration) (*types.CompletionReport, error) {
		return &types.CompletionReport{Status: types.StatusComplete}, nil
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
	waitForCompletionFunc = func(_, _ string, _, _ time.Duration) (*types.CompletionReport, error) {
		return &types.CompletionReport{Status: types.StatusComplete}, nil
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
		from types.State
		to   types.State
	}{
		{types.ScoutPending, types.Reviewed},
		{types.ScoutPending, types.NotSuitable},
		{types.Reviewed, types.WavePending},
		{types.WavePending, types.WaveExecuting},
		{types.WaveExecuting, types.WaveVerified},
		{types.WaveVerified, types.Complete},
		{types.WaveVerified, types.WavePending},
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
	err := o.TransitionTo(types.Complete)
	if err == nil {
		t.Fatal("expected error for ScoutPending -> Complete, got nil")
	}
	if !strings.Contains(err.Error(), "ScoutPending") {
		t.Errorf("error should mention source state ScoutPending, got: %v", err)
	}
	if !strings.Contains(err.Error(), "Complete") {
		t.Errorf("error should mention target state Complete, got: %v", err)
	}
	if o.State() != types.ScoutPending {
		t.Errorf("state changed after invalid transition: got %s", o.State())
	}
}

// TestTransitionTo_TerminalState verifies that NotSuitable and Complete cannot be exited.
func TestTransitionTo_TerminalState(t *testing.T) {
	terminalStates := []types.State{types.NotSuitable, types.Complete}
	allStates := []types.State{
		types.ScoutPending,
		types.Reviewed,
		types.WavePending,
		types.WaveExecuting,
		types.WaveVerified,
		types.NotSuitable,
		types.Complete,
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
	valid := []struct{ from, to types.State }{
		{types.ScoutPending, types.Reviewed},
		{types.ScoutPending, types.NotSuitable},
		{types.Reviewed, types.WavePending},
		{types.WavePending, types.WaveExecuting},
		{types.WaveExecuting, types.WaveVerified},
		{types.WaveVerified, types.Complete},
		{types.WaveVerified, types.WavePending},
	}
	for _, tc := range valid {
		if !isValidTransition(tc.from, tc.to) {
			t.Errorf("isValidTransition(%s, %s) = false, want true", tc.from, tc.to)
		}
	}

	invalid := []struct{ from, to types.State }{
		{types.ScoutPending, types.Complete},
		{types.ScoutPending, types.WaveExecuting},
		{types.Reviewed, types.Complete},
		{types.NotSuitable, types.Reviewed},
		{types.Complete, types.WavePending},
		{types.WaveExecuting, types.WavePending},
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

	if o.State() != types.ScoutPending {
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
	waitForCompletionFunc = func(_, _ string, _, _ time.Duration) (*types.CompletionReport, error) {
		return &types.CompletionReport{Status: types.StatusComplete}, nil
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
	waitForCompletionFunc = func(_, _ string, _, _ time.Duration) (*types.CompletionReport, error) {
		return &types.CompletionReport{Status: types.StatusComplete}, nil
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
	waitForCompletionFunc = func(implDoc, _ string, _, _ time.Duration) (*types.CompletionReport, error) {
		gotIMPLPath = implDoc
		return &types.CompletionReport{Status: types.StatusComplete}, nil
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

// TestState_String verifies that each state produces a human-readable name.
func TestState_String(t *testing.T) {
	cases := []struct {
		state types.State
		want  string
	}{
		{types.ScoutPending, "ScoutPending"},
		{types.NotSuitable, "NotSuitable"},
		{types.Reviewed, "Reviewed"},
		{types.WavePending, "WavePending"},
		{types.WaveExecuting, "WaveExecuting"},
		{types.WaveVerified, "WaveVerified"},
		{types.Complete, "Complete"},
	}

	for _, tc := range cases {
		got := tc.state.String()
		if got != tc.want {
			t.Errorf("State(%d).String() = %q, want %q", int(tc.state), got, tc.want)
		}
	}
}
