package orchestrator

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/agent"
	"github.com/blackwell-systems/scout-and-wave-go/pkg/agent/backend"
	"github.com/blackwell-systems/scout-and-wave-go/pkg/protocol"
	"github.com/blackwell-systems/scout-and-wave-go/pkg/types"
	"github.com/blackwell-systems/scout-and-wave-go/pkg/worktree"
)

// buildMinimalManifestForOrchestratorTest writes a minimal IMPL YAML that has
// one wave with one agent (agentID). Returns the file path.
func buildMinimalManifestForOrchestratorTest(t *testing.T, dir string, agentID string) string {
	t.Helper()
	implPath := filepath.Join(dir, "IMPL.yaml")
	content := fmt.Sprintf(`title: test
feature_slug: test
verdict: SUITABLE
test_command: go test ./...
lint_command: go vet ./...
file_ownership: []
interface_contracts: []
waves:
  - number: 1
    agents:
      - id: %s
        task: do work
        files: []
quality_gates:
  level: standard
  gates: []
`, agentID)
	if err := os.WriteFile(implPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write IMPL manifest: %v", err)
	}
	return implPath
}

// saveCompletionReportToManifest is a helper that saves a completion report
// directly into the manifest YAML at implPath.
func saveCompletionReportToManifest(t *testing.T, implPath, agentID string, report protocol.CompletionReport) {
	t.Helper()
	reportMu.Lock()
	defer reportMu.Unlock()

	manifest, err := protocol.Load(implPath)
	if err != nil {
		t.Fatalf("saveCompletionReportToManifest: Load: %v", err)
	}
	if err := protocol.SetCompletionReport(manifest, agentID, report); err != nil {
		t.Fatalf("saveCompletionReportToManifest: SetCompletionReport: %v", err)
	}
	if err := protocol.Save(manifest, implPath); err != nil {
		t.Fatalf("saveCompletionReportToManifest: Save: %v", err)
	}
}

// --- TestLaunchAgentStructured_FallbackToCLI -------------------------------------

// TestLaunchAgentStructured_FallbackToCLI verifies that when the model has a
// "cli:" prefix, launchAgentStructured falls back to the standard launchAgent
// path (which calls ExecuteStreamingWithTools and polls for a completion report).
func TestLaunchAgentStructured_FallbackToCLI(t *testing.T) {
	origCreator := worktreeCreatorFunc
	origWait := waitForCompletionFunc
	origNewBackend := newBackendFunc
	origNewRunner := newRunnerFunc
	origStructured := runWaveAgentStructuredFunc
	t.Cleanup(func() {
		worktreeCreatorFunc = origCreator
		waitForCompletionFunc = origWait
		newBackendFunc = origNewBackend
		newRunnerFunc = origNewRunner
		runWaveAgentStructuredFunc = origStructured
	})

	var worktreeCreated bool
	worktreeCreatorFunc = func(_ *worktree.Manager, _ int, id string) (string, error) {
		worktreeCreated = true
		return "/tmp/fake-wt-" + id, nil
	}
	waitForCompletionFunc = func(_, _ string, _, _ time.Duration) (*protocol.CompletionReport, error) {
		return &protocol.CompletionReport{Status: "complete"}, nil
	}

	var structuredCalled bool
	runWaveAgentStructuredFunc = func(_ context.Context, _, _ string, _ types.AgentSpec, _ string, _ func(string)) error {
		structuredCalled = true
		return nil
	}

	fake := &fakeBackend{}
	newBackendFunc = func(_ BackendConfig) (backend.Backend, error) {
		return fake, nil
	}
	newRunnerFunc = func(b backend.Backend, wm *worktree.Manager) *agent.Runner {
		return agent.NewRunner(b, wm)
	}

	doc := &protocol.IMPLManifest{
		Waves: []protocol.Wave{
			{
				Number: 1,
				Agents: []protocol.Agent{
					{ID: "A", Task: "do work", Model: "cli:kimi"},
				},
			},
		},
	}
	o := newFromDoc(doc, "/repo", "/repo/IMPL.md")

	wm := worktree.New("/repo", "test-slug")
	runner := agent.NewRunner(fake, wm)

	// Use the CLI model on the agent spec.
	agentSpec := protocol.Agent{ID: "A", Task: "do work", Model: "cli:kimi"}
	err := o.launchAgentStructured(context.Background(), runner, wm, 1, agentSpec)
	if err != nil {
		t.Fatalf("launchAgentStructured returned unexpected error: %v", err)
	}

	// Verify the structured path was NOT called (CLI fell back to standard launchAgent).
	if structuredCalled {
		t.Error("runWaveAgentStructuredFunc should NOT be called for CLI backend; expected fallback to launchAgent")
	}
	// Verify a worktree was still created (launchAgent creates one).
	if !worktreeCreated {
		t.Error("expected worktree to be created via launchAgent fallback")
	}
}

// --- TestLaunchAgentStructured_APIPath ------------------------------------------

// TestLaunchAgentStructured_APIPath verifies that launchAgentStructured calls
// runWaveAgentStructuredFunc (not launchAgent) for a non-CLI model, and publishes
// the expected SSE events.
func TestLaunchAgentStructured_APIPath(t *testing.T) {
	origCreator := worktreeCreatorFunc
	origStructured := runWaveAgentStructuredFunc
	t.Cleanup(func() {
		worktreeCreatorFunc = origCreator
		runWaveAgentStructuredFunc = origStructured
	})

	worktreeCreatorFunc = func(_ *worktree.Manager, _ int, id string) (string, error) {
		return "/tmp/fake-wt-" + id, nil
	}

	dir := t.TempDir()
	implPath := buildMinimalManifestForOrchestratorTest(t, dir, "A")

	var structuredCalledForAgent string
	runWaveAgentStructuredFunc = func(_ context.Context, gotImplPath, _ string, agentSpec types.AgentSpec, _ string, _ func(string)) error {
		structuredCalledForAgent = agentSpec.Letter
		// Simulate saving a completion report.
		saveCompletionReportToManifest(t, gotImplPath, agentSpec.Letter, protocol.CompletionReport{
			Status: "complete",
			Notes:  "structured done",
		})
		return nil
	}

	doc := &protocol.IMPLManifest{
		Waves: []protocol.Wave{
			{Number: 1, Agents: []protocol.Agent{{ID: "A", Task: "do work"}}},
		},
	}
	o := newFromDoc(doc, dir, implPath)
	o.SetDefaultModel("claude-sonnet-4-5")

	var mu sync.Mutex
	var events []OrchestratorEvent
	o.SetEventPublisher(func(ev OrchestratorEvent) {
		mu.Lock()
		events = append(events, ev)
		mu.Unlock()
	})

	wm := worktree.New(dir, "test-slug")
	runner := agent.NewRunner(&fakeBackend{}, wm)
	agentSpec := protocol.Agent{ID: "A", Task: "do work"}

	err := o.launchAgentStructured(context.Background(), runner, wm, 1, agentSpec)
	if err != nil {
		t.Fatalf("launchAgentStructured returned error: %v", err)
	}

	if structuredCalledForAgent != "A" {
		t.Errorf("runWaveAgentStructuredFunc was called for agent %q, want A", structuredCalledForAgent)
	}

	mu.Lock()
	defer mu.Unlock()

	eventNames := make([]string, len(events))
	for i, ev := range events {
		eventNames[i] = ev.Event
	}

	if !containsStr(eventNames, "agent_started") {
		t.Errorf("expected agent_started event, got: %v", eventNames)
	}
	if !containsStr(eventNames, "agent_complete") {
		t.Errorf("expected agent_complete event, got: %v", eventNames)
	}
}

// --- TestLaunchAgentStructured_PublishesBlockedEvent ----------------------------

// TestLaunchAgentStructured_PublishesBlockedEvent verifies E19: when the
// structured run saves a "blocked" completion report, launchAgentStructured
// publishes an agent_blocked event with RouteFailure action.
func TestLaunchAgentStructured_PublishesBlockedEvent(t *testing.T) {
	origCreator := worktreeCreatorFunc
	origStructured := runWaveAgentStructuredFunc
	t.Cleanup(func() {
		worktreeCreatorFunc = origCreator
		runWaveAgentStructuredFunc = origStructured
	})

	worktreeCreatorFunc = func(_ *worktree.Manager, _ int, id string) (string, error) {
		return "/tmp/fake-wt-" + id, nil
	}

	dir := t.TempDir()
	implPath := buildMinimalManifestForOrchestratorTest(t, dir, "A")

	runWaveAgentStructuredFunc = func(_ context.Context, gotImplPath, _ string, agentSpec types.AgentSpec, _ string, _ func(string)) error {
		saveCompletionReportToManifest(t, gotImplPath, agentSpec.Letter, protocol.CompletionReport{
			Status:      "blocked",
			FailureType: "transient",
			Notes:       "dependency unavailable",
		})
		return nil
	}

	doc := &protocol.IMPLManifest{
		Waves: []protocol.Wave{
			{Number: 1, Agents: []protocol.Agent{{ID: "A", Task: "do work"}}},
		},
	}
	o := newFromDoc(doc, dir, implPath)

	var mu sync.Mutex
	var events []OrchestratorEvent
	o.SetEventPublisher(func(ev OrchestratorEvent) {
		mu.Lock()
		events = append(events, ev)
		mu.Unlock()
	})

	wm := worktree.New(dir, "test-slug")
	runner := agent.NewRunner(&fakeBackend{}, wm)
	agentSpec := protocol.Agent{ID: "A", Task: "do work"}

	err := o.launchAgentStructured(context.Background(), runner, wm, 1, agentSpec)
	if err != nil {
		t.Fatalf("launchAgentStructured returned error: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	var blockedEv *OrchestratorEvent
	for i := range events {
		if events[i].Event == "agent_blocked" {
			blockedEv = &events[i]
			break
		}
	}
	if blockedEv == nil {
		t.Fatalf("expected agent_blocked event, got events: %v", func() []string {
			names := make([]string, len(events))
			for i, ev := range events {
				names[i] = ev.Event
			}
			return names
		}())
	}

	payload, ok := blockedEv.Data.(AgentBlockedPayload)
	if !ok {
		t.Fatalf("agent_blocked Data is %T, want AgentBlockedPayload", blockedEv.Data)
	}
	if payload.Agent != "A" {
		t.Errorf("payload.Agent = %q, want A", payload.Agent)
	}
	if payload.Status != "blocked" {
		t.Errorf("payload.Status = %q, want blocked", payload.Status)
	}
	// "transient" failure type should route to ActionRetry.
	if payload.Action != ActionRetry {
		t.Errorf("payload.Action = %v, want ActionRetry for transient failure", payload.Action)
	}
}

// --- TestLaunchAgentStructured_WorktreeCreationFailure -------------------------

// TestLaunchAgentStructured_WorktreeCreationFailure verifies that launchAgentStructured
// returns an error and publishes agent_failed when worktree creation fails.
func TestLaunchAgentStructured_WorktreeCreationFailure(t *testing.T) {
	origCreator := worktreeCreatorFunc
	t.Cleanup(func() { worktreeCreatorFunc = origCreator })

	worktreeCreatorFunc = func(_ *worktree.Manager, _ int, _ string) (string, error) {
		return "", fmt.Errorf("simulated worktree failure")
	}

	doc := &protocol.IMPLManifest{
		Waves: []protocol.Wave{
			{Number: 1, Agents: []protocol.Agent{{ID: "A", Task: "do work"}}},
		},
	}
	o := newFromDoc(doc, "/repo", "/repo/IMPL.md")

	var mu sync.Mutex
	var events []OrchestratorEvent
	o.SetEventPublisher(func(ev OrchestratorEvent) {
		mu.Lock()
		events = append(events, ev)
		mu.Unlock()
	})

	wm := worktree.New("/repo", "test-slug")
	runner := agent.NewRunner(&fakeBackend{}, wm)
	agentSpec := protocol.Agent{ID: "A", Task: "do work"}

	err := o.launchAgentStructured(context.Background(), runner, wm, 1, agentSpec)
	if err == nil {
		t.Fatal("expected error when worktree creation fails, got nil")
	}
	if !strings.Contains(err.Error(), "create worktree") {
		t.Errorf("error should mention create worktree, got: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	var foundFailed bool
	for _, ev := range events {
		if ev.Event == "agent_failed" {
			foundFailed = true
			break
		}
	}
	if !foundFailed {
		t.Error("expected agent_failed event on worktree creation failure")
	}
}

// --- TestLaunchAgentStructured_StructuredFunctionError --------------------------

// TestLaunchAgentStructured_StructuredFunctionError verifies that when
// runWaveAgentStructuredFunc returns an error, launchAgentStructured propagates
// it and publishes an agent_failed event.
func TestLaunchAgentStructured_StructuredFunctionError(t *testing.T) {
	origCreator := worktreeCreatorFunc
	origStructured := runWaveAgentStructuredFunc
	t.Cleanup(func() {
		worktreeCreatorFunc = origCreator
		runWaveAgentStructuredFunc = origStructured
	})

	worktreeCreatorFunc = func(_ *worktree.Manager, _ int, id string) (string, error) {
		return "/tmp/fake-wt-" + id, nil
	}
	runWaveAgentStructuredFunc = func(_ context.Context, _, _ string, _ types.AgentSpec, _ string, _ func(string)) error {
		return fmt.Errorf("structured run failed: API timeout")
	}

	doc := &protocol.IMPLManifest{
		Waves: []protocol.Wave{
			{Number: 1, Agents: []protocol.Agent{{ID: "A", Task: "do work"}}},
		},
	}
	o := newFromDoc(doc, "/repo", "/repo/IMPL.md")

	var mu sync.Mutex
	var events []OrchestratorEvent
	o.SetEventPublisher(func(ev OrchestratorEvent) {
		mu.Lock()
		events = append(events, ev)
		mu.Unlock()
	})

	wm := worktree.New("/repo", "test-slug")
	runner := agent.NewRunner(&fakeBackend{}, wm)
	agentSpec := protocol.Agent{ID: "A", Task: "do work"}

	err := o.launchAgentStructured(context.Background(), runner, wm, 1, agentSpec)
	if err == nil {
		t.Fatal("expected error from structured run failure, got nil")
	}
	if !strings.Contains(err.Error(), "structured execute") {
		t.Errorf("error should mention 'structured execute', got: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	var foundFailed bool
	for _, ev := range events {
		if ev.Event == "agent_failed" {
			foundFailed = true
			payload, ok := ev.Data.(AgentFailedPayload)
			if !ok {
				t.Errorf("agent_failed Data is %T, want AgentFailedPayload", ev.Data)
			} else if payload.FailureType != "execute" {
				t.Errorf("agent_failed payload.FailureType = %q, want execute", payload.FailureType)
			}
			break
		}
	}
	if !foundFailed {
		t.Error("expected agent_failed event when structured function errors")
	}
}

// --- TestLaunchAgentStructured_OnChunkForwarding --------------------------------

// TestLaunchAgentStructured_OnChunkForwarding verifies that output chunks from
// runWaveAgentStructuredFunc are published as agent_output SSE events.
func TestLaunchAgentStructured_OnChunkForwarding(t *testing.T) {
	origCreator := worktreeCreatorFunc
	origStructured := runWaveAgentStructuredFunc
	t.Cleanup(func() {
		worktreeCreatorFunc = origCreator
		runWaveAgentStructuredFunc = origStructured
	})

	worktreeCreatorFunc = func(_ *worktree.Manager, _ int, id string) (string, error) {
		return "/tmp/fake-wt-" + id, nil
	}

	dir := t.TempDir()
	implPath := buildMinimalManifestForOrchestratorTest(t, dir, "A")

	var capturedOnChunk func(string)
	runWaveAgentStructuredFunc = func(_ context.Context, gotImplPath, _ string, agentSpec types.AgentSpec, _ string, onChunk func(string)) error {
		capturedOnChunk = onChunk
		// Call the onChunk callback to simulate streaming output.
		if onChunk != nil {
			onChunk("hello from structured agent")
		}
		saveCompletionReportToManifest(t, gotImplPath, agentSpec.Letter, protocol.CompletionReport{Status: "complete"})
		return nil
	}

	doc := &protocol.IMPLManifest{
		Waves: []protocol.Wave{
			{Number: 1, Agents: []protocol.Agent{{ID: "A", Task: "do work"}}},
		},
	}
	o := newFromDoc(doc, dir, implPath)

	var mu sync.Mutex
	var outputChunks []string
	o.SetEventPublisher(func(ev OrchestratorEvent) {
		if ev.Event == "agent_output" {
			if payload, ok := ev.Data.(AgentOutputPayload); ok {
				mu.Lock()
				outputChunks = append(outputChunks, payload.Chunk)
				mu.Unlock()
			}
		}
	})

	wm := worktree.New(dir, "test-slug")
	runner := agent.NewRunner(&fakeBackend{}, wm)
	agentSpec := protocol.Agent{ID: "A", Task: "do work"}

	err := o.launchAgentStructured(context.Background(), runner, wm, 1, agentSpec)
	if err != nil {
		t.Fatalf("launchAgentStructured returned error: %v", err)
	}

	if capturedOnChunk == nil {
		t.Fatal("onChunk was not passed to runWaveAgentStructuredFunc")
	}

	mu.Lock()
	defer mu.Unlock()

	if len(outputChunks) == 0 {
		t.Error("expected agent_output events for chunk forwarding, got none")
	}
	if outputChunks[0] != "hello from structured agent" {
		t.Errorf("chunk = %q, want %q", outputChunks[0], "hello from structured agent")
	}
}

// --- TestSetRunWaveAgentStructuredFunc ------------------------------------------

// TestSetRunWaveAgentStructuredFunc verifies that SetRunWaveAgentStructuredFunc
// replaces the injected function and it is called during structured execution.
func TestSetRunWaveAgentStructuredFunc(t *testing.T) {
	origFn := runWaveAgentStructuredFunc
	t.Cleanup(func() { runWaveAgentStructuredFunc = origFn })

	var called bool
	SetRunWaveAgentStructuredFunc(func(_ context.Context, _, _ string, _ types.AgentSpec, _ string, _ func(string)) error {
		called = true
		return fmt.Errorf("sentinel error to stop processing")
	})

	origCreator := worktreeCreatorFunc
	t.Cleanup(func() { worktreeCreatorFunc = origCreator })
	worktreeCreatorFunc = func(_ *worktree.Manager, _ int, id string) (string, error) {
		return "/tmp/fake-wt-" + id, nil
	}

	doc := &protocol.IMPLManifest{
		Waves: []protocol.Wave{
			{Number: 1, Agents: []protocol.Agent{{ID: "A", Task: "do work"}}},
		},
	}
	o := newFromDoc(doc, "/repo", "/repo/IMPL.md")
	wm := worktree.New("/repo", "test-slug")
	runner := agent.NewRunner(&fakeBackend{}, wm)

	// The error is expected; we just want to verify it was called.
	_ = o.launchAgentStructured(context.Background(), runner, wm, 1, protocol.Agent{ID: "A", Task: "do work"})
	if !called {
		t.Error("SetRunWaveAgentStructuredFunc: injected function was not called")
	}
}

// --- helpers -----------------------------------------------------------------

// containsStr returns true if s is in the slice.
func containsStr(slice []string, s string) bool {
	for _, v := range slice {
		if v == s {
			return true
		}
	}
	return false
}

// makeCompletionReportJSON is a helper to JSON-encode a completion report.
func makeCompletionReportJSON(t *testing.T, r protocol.CompletionReport) string {
	t.Helper()
	b, err := json.Marshal(r)
	if err != nil {
		t.Fatalf("makeCompletionReportJSON: %v", err)
	}
	return string(b)
}
