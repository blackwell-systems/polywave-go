package agent

import (
	"testing"
	"time"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/worktree"
)

// TestNewRunner verifies that NewRunner with a nil backend returns a non-nil Runner.
func TestNewRunner(t *testing.T) {
	wm := worktree.New("/tmp")
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
