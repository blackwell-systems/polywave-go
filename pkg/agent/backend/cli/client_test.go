package cli

import (
	"context"
	"strings"
	"testing"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/agent/backend"
)

func TestIsClaudeCodeSession_Detected(t *testing.T) {
	t.Setenv("CLAUDECODE", "1")
	if !IsClaudeCodeSession() {
		t.Fatal("expected IsClaudeCodeSession() to return true when CLAUDECODE=1")
	}
}

func TestIsClaudeCodeSession_NotDetected(t *testing.T) {
	// Ensure CLAUDECODE is not set.
	t.Setenv("CLAUDECODE", "")
	if IsClaudeCodeSession() {
		t.Fatal("expected IsClaudeCodeSession() to return false when CLAUDECODE is empty")
	}
}

func TestRunStreaming_ClaudeCodeSessionBlocked(t *testing.T) {
	t.Setenv("CLAUDECODE", "1")
	c := New("", backend.Config{})
	_, err := c.RunStreaming(context.Background(), "", "hello", t.TempDir(), nil)
	if err == nil {
		t.Fatal("expected error when CLAUDECODE is set, got nil")
	}
	if !strings.Contains(err.Error(), "cannot spawn claude subprocess from within a Claude Code session") {
		t.Fatalf("unexpected error message: %v", err)
	}
}

func TestRunStreamingWithTools_ClaudeCodeSessionBlocked(t *testing.T) {
	t.Setenv("CLAUDECODE", "1")
	c := New("", backend.Config{})
	_, err := c.RunStreamingWithTools(context.Background(), "", "hello", t.TempDir(), nil, nil)
	if err == nil {
		t.Fatal("expected error when CLAUDECODE is set, got nil")
	}
	if !strings.Contains(err.Error(), "cannot spawn claude subprocess from within a Claude Code session") {
		t.Fatalf("unexpected error message: %v", err)
	}
}
