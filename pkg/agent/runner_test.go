package agent

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/worktree"
)

// TestNewRunner verifies that NewRunner with a nil backend returns a non-nil Runner.
func TestNewRunner(t *testing.T) {
	wm := worktree.New("/tmp", "test-slug")
	r := NewRunner(nil, wm)
	if r == nil {
		t.Fatal("NewRunner returned nil")
	}
}

// TestWaitForCompletionTimeout verifies that WaitForCompletion returns an error
// when the IMPL doc does not exist and the timeout is exceeded.
func TestWaitForCompletionTimeout(t *testing.T) {
	_, err := WaitForCompletion(
		"/nonexistent/path/IMPL-doc.md",
		"A",
		50*time.Millisecond,
		10*time.Millisecond,
	)
	if err == nil {
		t.Fatal("expected error from WaitForCompletion with nonexistent path, got nil")
	}
}

// TestWaitForCompletionResultLoadFailed verifies that WaitForCompletionResult
// returns a fatal result with AGENT_COMPLETION_LOAD_FAILED when the manifest
// cannot be loaded.
func TestWaitForCompletionResultLoadFailed(t *testing.T) {
	r := WaitForCompletionResult(
		"/nonexistent/path/IMPL-doc.md",
		"A",
		50*time.Millisecond,
		10*time.Millisecond,
	)
	if !r.IsFatal() {
		t.Fatalf("expected fatal result, got code %q", r.Code)
	}
	if len(r.Errors) == 0 {
		t.Fatal("expected at least one error in fatal result")
	}
	if r.Errors[0].Code != "AGENT_COMPLETION_LOAD_FAILED" {
		t.Fatalf("expected error code AGENT_COMPLETION_LOAD_FAILED, got %q", r.Errors[0].Code)
	}
}

// TestWaitForCompletionResultTimeout verifies that WaitForCompletionResult
// returns a fatal result with AGENT_COMPLETION_TIMEOUT when the agent's
// completion report never appears.
func TestWaitForCompletionResultTimeout(t *testing.T) {
	// Create a valid but empty IMPL doc to avoid load errors
	tmpDir := t.TempDir()
	implPath := tmpDir + "/IMPL-test.yaml"
	// Write minimal valid YAML
	content := "slug: test\nstatus: active\nwaves:\n  - wave: 1\n    agents: []\n"
	if err := os.WriteFile(implPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test IMPL: %v", err)
	}

	r := WaitForCompletionResult(
		implPath,
		"Z", // agent that won't have a completion report
		50*time.Millisecond,
		10*time.Millisecond,
	)
	if !r.IsFatal() {
		t.Fatalf("expected fatal result, got code %q", r.Code)
	}
	if len(r.Errors) == 0 {
		t.Fatal("expected at least one error in fatal result")
	}
	if r.Errors[0].Code != "AGENT_COMPLETION_TIMEOUT" {
		t.Fatalf("expected error code AGENT_COMPLETION_TIMEOUT, got %q", r.Errors[0].Code)
	}
	if !strings.Contains(r.Errors[0].Message, "timed out") {
		t.Fatalf("expected timeout message, got %q", r.Errors[0].Message)
	}
}
