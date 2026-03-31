package retry

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/protocol"
)

// TestRetryLoop_MaxRetries verifies that once attempt > MaxRetries, the loop
// returns FinalState "blocked" and does not write a retry IMPL file.
func TestRetryLoop_MaxRetries(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := RetryConfig{
		MaxRetries: 2,
		RepoPath:   tmpDir,
	}
	loop := NewRetryLoop(cfg)

	gate := QualityGateFailure{
		GateType:    "build",
		Command:     "go build ./...",
		Output:      "build failed",
		FailedFiles: []string{"pkg/foo/foo.go"},
	}

	// First attempt — should produce a retry IMPL
	r1, err := loop.Run(context.Background(), gate, nil)
	if err != nil {
		t.Fatalf("attempt 1: unexpected error: %v", err)
	}
	if r1.FinalState != "retrying" {
		t.Errorf("attempt 1: FinalState = %q, want %q", r1.FinalState, "retrying")
	}
	if r1.AttemptNumber != 1 {
		t.Errorf("attempt 1: AttemptNumber = %d, want 1", r1.AttemptNumber)
	}
	if r1.RetryIMPL == "" {
		t.Error("attempt 1: RetryIMPL path should not be empty")
	}

	// Second attempt — should still produce a retry IMPL (last allowed attempt)
	r2, err := loop.Run(context.Background(), gate, nil)
	if err != nil {
		t.Fatalf("attempt 2: unexpected error: %v", err)
	}
	if r2.AttemptNumber != 2 {
		t.Errorf("attempt 2: AttemptNumber = %d, want 2", r2.AttemptNumber)
	}
	if r2.RetryIMPL == "" {
		t.Error("attempt 2: RetryIMPL path should not be empty")
	}

	// Third attempt — exceeds MaxRetries=2, should be blocked
	var events []Event
	r3, err := loop.Run(context.Background(), gate, func(e Event) {
		events = append(events, e)
	})
	if err != nil {
		t.Fatalf("attempt 3: unexpected error: %v", err)
	}
	if r3.FinalState != "blocked" {
		t.Errorf("attempt 3: FinalState = %q, want %q", r3.FinalState, "blocked")
	}
	if r3.RetryIMPL != "" {
		t.Errorf("attempt 3: RetryIMPL should be empty when blocked, got %q", r3.RetryIMPL)
	}
	if r3.AttemptNumber != 3 {
		t.Errorf("attempt 3: AttemptNumber = %d, want 3", r3.AttemptNumber)
	}

	// Should have emitted "retry_blocked" event
	foundBlocked := false
	for _, e := range events {
		if e.Event == "retry_blocked" {
			foundBlocked = true
		}
	}
	if !foundBlocked {
		t.Error("expected retry_blocked event but none received")
	}
}

// TestRetryLoop_PassOnFirstRetry verifies the success/retrying path: the first
// call to Run generates the IMPL doc and returns FinalState "retrying" with
// the retry IMPL path set.
func TestRetryLoop_PassOnFirstRetry(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := RetryConfig{
		MaxRetries: 2,
		RepoPath:   tmpDir,
	}
	loop := NewRetryLoop(cfg)

	gate := QualityGateFailure{
		GateType:    "test",
		Command:     "go test ./...",
		Output:      "FAIL: TestFoo",
		FailedFiles: []string{"pkg/bar/bar.go"},
	}

	var events []Event
	result, err := loop.Run(context.Background(), gate, func(e Event) {
		events = append(events, e)
	})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	// Should have FinalState "retrying" (not blocked)
	if result.FinalState != "retrying" {
		t.Errorf("FinalState = %q, want %q", result.FinalState, "retrying")
	}

	// AttemptNumber should be 1
	if result.AttemptNumber != 1 {
		t.Errorf("AttemptNumber = %d, want 1", result.AttemptNumber)
	}

	// AgentID should be "R"
	if result.AgentID != "R" {
		t.Errorf("AgentID = %q, want R", result.AgentID)
	}

	// RetryIMPL path should be set
	if result.RetryIMPL == "" {
		t.Error("RetryIMPL path should be set")
	}

	// The file should actually exist on disk
	absPath := filepath.Join(tmpDir, result.RetryIMPL)
	if _, statErr := os.Stat(absPath); statErr != nil {
		t.Errorf("retry IMPL file does not exist at %s: %v", absPath, statErr)
	}

	// "retry_started" event should have been emitted
	foundStarted := false
	for _, e := range events {
		if e.Event == "retry_started" {
			foundStarted = true
		}
	}
	if !foundStarted {
		t.Error("expected retry_started event but none received")
	}
}

// TestRetryResult_States verifies the FinalState values over the lifecycle of
// the retry loop.
func TestRetryResult_States(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := RetryConfig{MaxRetries: 2, RepoPath: tmpDir}
	loop := NewRetryLoop(cfg)

	gate := QualityGateFailure{
		GateType:    "build",
		Command:     "go build ./...",
		Output:      "error: undefined",
		FailedFiles: []string{"pkg/z/z.go"},
	}

	states := make([]string, 0, 4)
	for i := 0; i < 4; i++ {
		r, err := loop.Run(context.Background(), gate, nil)
		if err != nil {
			t.Fatalf("attempt %d: unexpected error: %v", i+1, err)
		}
		states = append(states, r.FinalState)
	}

	// Expected states: retrying, retrying, blocked, blocked
	expectedStates := []string{"retrying", "retrying", "blocked", "blocked"}
	for i, want := range expectedStates {
		if states[i] != want {
			t.Errorf("attempt %d: FinalState = %q, want %q", i+1, states[i], want)
		}
	}
}

// TestRetryLoop_DefaultMaxRetries verifies that MaxRetries defaults to 2
// when zero is passed.
func TestRetryLoop_DefaultMaxRetries(t *testing.T) {
	cfg := RetryConfig{MaxRetries: 0}
	loop := NewRetryLoop(cfg)

	if loop.cfg.MaxRetries != 2 {
		t.Errorf("default MaxRetries = %d, want 2", loop.cfg.MaxRetries)
	}
}

// TestRetryLoop_IMPLFileContents verifies that the saved IMPL manifest file
// contains valid YAML and passes protocol.Validate().
func TestRetryLoop_IMPLFileContents(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := RetryConfig{
		MaxRetries: 2,
		RepoPath:   tmpDir,
	}
	loop := NewRetryLoop(cfg)

	gate := QualityGateFailure{
		GateType:    "build",
		Command:     "go build ./...",
		Output:      "cannot find package",
		FailedFiles: []string{"pkg/retry/loop.go"},
	}

	result, err := loop.Run(context.Background(), gate, nil)
	if err != nil {
		t.Fatalf("Run error: %v", err)
	}

	absPath := filepath.Join(tmpDir, result.RetryIMPL)
	loaded, err := protocol.Load(absPath)
	if err != nil {
		t.Fatalf("protocol.Load(%s) error: %v", absPath, err)
	}

	errs := protocol.Validate(loaded)
	if len(errs) > 0 {
		t.Errorf("saved IMPL doc failed validation:")
		for _, e := range errs {
			t.Errorf("  [%s] %s (field: %s)", e.Code, e.Message, e.Field)
		}
	}
}

// TestRetryLoop_FilenameFormat verifies the retry IMPL file path format:
// docs/IMPL/IMPL-{parentSlug}-retry-{N}.yaml
func TestRetryLoop_FilenameFormat(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a minimal parent IMPL manifest for slug extraction
	parentIMPL := filepath.Join(tmpDir, "docs", "IMPL", "IMPL-my-slug.yaml")
	if err := os.MkdirAll(filepath.Dir(parentIMPL), 0755); err != nil {
		t.Fatal(err)
	}
	m := &protocol.IMPLManifest{
		Title:       "My Slug",
		FeatureSlug: "my-slug",
		Verdict:     "SUITABLE",
		Waves: []protocol.Wave{
			{Number: 1, Agents: []protocol.Agent{
				{ID: "A", Task: "task", Files: []string{"file.go"}},
				{ID: "B", Task: "tests", Files: []string{"file_test.go"}},
			}},
		},
		FileOwnership: []protocol.FileOwnership{
			{File: "file.go", Agent: "A", Wave: 1},
			{File: "file_test.go", Agent: "B", Wave: 1},
		},
		State: protocol.StateWavePending,
	}
	if saveRes := protocol.Save(m, parentIMPL); saveRes.IsFatal() {
		t.Fatal(saveRes.Errors)
	}

	cfg := RetryConfig{
		MaxRetries: 2,
		RepoPath:   tmpDir,
		IMPLPath:   parentIMPL,
	}
	loop := NewRetryLoop(cfg)

	gate := QualityGateFailure{
		GateType:    "build",
		Command:     "go build ./...",
		Output:      "err",
		FailedFiles: []string{"file.go"},
	}

	result, err := loop.Run(context.Background(), gate, nil)
	if err != nil {
		t.Fatalf("Run error: %v", err)
	}

	expected := filepath.Join("docs", "IMPL", "IMPL-my-slug-retry-1.yaml")
	if result.RetryIMPL != expected {
		t.Errorf("RetryIMPL = %q, want %q", result.RetryIMPL, expected)
	}
}

// TestRetryLoop_SlugFromIMPLPath verifies slug extraction from various IMPL paths.
func TestRetryLoop_SlugFromIMPLPath(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		{"docs/IMPL/IMPL-my-feature.yaml", "my-feature"},
		{"docs/IMPL/IMPL-foo-bar-baz.yaml", "foo-bar-baz"},
		{"IMPL-single.yaml", "single"},
		{"", "unknown"},
		{"IMPL.yaml", "IMPL"}, // no IMPL- prefix — falls back to full name
	}

	for _, tc := range tests {
		got := slugFromIMPLPath(tc.path)
		if got != tc.want {
			t.Errorf("slugFromIMPLPath(%q) = %q, want %q", tc.path, got, tc.want)
		}
	}
}

// TestRetryLoop_EventsEmitted verifies all expected events are emitted.
func TestRetryLoop_EventsEmitted(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := RetryConfig{MaxRetries: 1, RepoPath: tmpDir}
	loop := NewRetryLoop(cfg)

	gate := QualityGateFailure{
		GateType:    "lint",
		Command:     "go vet ./...",
		Output:      "vet: suspicious usage",
		FailedFiles: []string{"pkg/a/a.go"},
	}

	// First attempt — should emit retry_started
	events1 := make([]string, 0)
	loop.Run(context.Background(), gate, func(e Event) {
		events1 = append(events1, e.Event)
	})
	if !contains(events1, "retry_started") {
		t.Errorf("attempt 1: missing retry_started event, got: %v", events1)
	}

	// Second attempt (exceeds MaxRetries=1) — should emit retry_blocked
	events2 := make([]string, 0)
	loop.Run(context.Background(), gate, func(e Event) {
		events2 = append(events2, e.Event)
	})
	if !contains(events2, "retry_blocked") {
		t.Errorf("attempt 2: missing retry_blocked event, got: %v", events2)
	}
}

// TestRetryLoop_GenerateRetryIMPLMethod verifies the method form of GenerateRetryIMPL.
func TestRetryLoop_GenerateRetryIMPLMethod(t *testing.T) {
	tmpDir := t.TempDir()
	parentIMPL := filepath.Join(tmpDir, "docs", "IMPL", "IMPL-method-test.yaml")
	if err := os.MkdirAll(filepath.Dir(parentIMPL), 0755); err != nil {
		t.Fatal(err)
	}
	parent := &protocol.IMPLManifest{
		Title:       "Method Test",
		FeatureSlug: "method-test",
		Verdict:     "SUITABLE",
		TestCommand: "go test ./pkg/method/...",
		Waves: []protocol.Wave{
			{Number: 1, Agents: []protocol.Agent{
				{ID: "A", Task: "task", Files: []string{"pkg/method/m.go"}},
				{ID: "B", Task: "tests", Files: []string{"pkg/method/m_test.go"}},
			}},
		},
		FileOwnership: []protocol.FileOwnership{
			{File: "pkg/method/m.go", Agent: "A", Wave: 1},
			{File: "pkg/method/m_test.go", Agent: "B", Wave: 1},
		},
		State: protocol.StateWavePending,
	}
	if saveRes := protocol.Save(parent, parentIMPL); saveRes.IsFatal() {
		t.Fatal(saveRes.Errors)
	}

	cfg := RetryConfig{
		MaxRetries: 2,
		RepoPath:   tmpDir,
		IMPLPath:   parentIMPL,
	}
	loop := NewRetryLoop(cfg)

	// Manually set attempt to test without calling Run
	loop.attempt = 1

	m := loop.GenerateRetryIMPL([]string{"pkg/method/m.go"}, "test failed")

	if m == nil {
		t.Fatal("GenerateRetryIMPL method returned nil")
	}

	// Title should embed parent slug
	if !strings.Contains(m.Title, "method-test") {
		t.Errorf("Title %q does not contain parent slug", m.Title)
	}

	// Gate command should be the parent's test_command
	if m.QualityGates != nil && len(m.QualityGates.Gates) > 0 {
		if m.QualityGates.Gates[0].Command != "go test ./pkg/method/..." {
			t.Errorf("gate.Command = %q, want %q", m.QualityGates.Gates[0].Command, "go test ./pkg/method/...")
		}
	}

	// Should pass validation
	errs := protocol.Validate(m)
	if len(errs) > 0 {
		for _, e := range errs {
			t.Errorf("[%s] %s", e.Code, e.Message)
		}
	}
}

// TestRetryLoop_FallbackToIMPLFiles verifies that when FailedFiles is empty,
// the loop reads files from the parent IMPL manifest.
func TestRetryLoop_FallbackToIMPLFiles(t *testing.T) {
	tmpDir := t.TempDir()
	parentIMPL := filepath.Join(tmpDir, "docs", "IMPL", "IMPL-fallback.yaml")
	if err := os.MkdirAll(filepath.Dir(parentIMPL), 0755); err != nil {
		t.Fatal(err)
	}
	parent := &protocol.IMPLManifest{
		Title:       "Fallback Test",
		FeatureSlug: "fallback",
		Verdict:     "SUITABLE",
		Waves: []protocol.Wave{
			{Number: 1, Agents: []protocol.Agent{{ID: "A", Task: "task", Files: []string{"pkg/fb/fb.go"}}}},
		},
		FileOwnership: []protocol.FileOwnership{{File: "pkg/fb/fb.go", Agent: "A", Wave: 1}},
		State:         protocol.StateWavePending,
	}
	if saveRes := protocol.Save(parent, parentIMPL); saveRes.IsFatal() {
		t.Fatal(saveRes.Errors)
	}

	cfg := RetryConfig{
		MaxRetries: 2,
		RepoPath:   tmpDir,
		IMPLPath:   parentIMPL,
	}
	loop := NewRetryLoop(cfg)

	// No FailedFiles set — should fall back to IMPL files
	gate := QualityGateFailure{
		GateType:    "build",
		Command:     "go build ./...",
		Output:      "error",
		FailedFiles: nil,
	}

	result, err := loop.Run(context.Background(), gate, nil)
	if err != nil {
		t.Fatalf("Run error: %v", err)
	}

	// Load the generated IMPL and check it includes the parent's files
	absPath := filepath.Join(tmpDir, result.RetryIMPL)
	loaded, err := protocol.Load(absPath)
	if err != nil {
		t.Fatalf("protocol.Load error: %v", err)
	}

	foundFbFile := false
	for _, fo := range loaded.FileOwnership {
		if fo.File == "pkg/fb/fb.go" {
			foundFbFile = true
		}
	}
	if !foundFbFile {
		t.Error("retry IMPL should include parent IMPL's files as fallback")
	}
}

// helper: contains checks if a string is in a slice.
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// helper: ensures fmt and strings are used.
var _ = fmt.Sprintf
var _ = strings.Contains
