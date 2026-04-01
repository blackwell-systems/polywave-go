package cli

import (
	"context"
	"strings"
	"testing"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/agent/backend"
)

func TestBuildCLIArgs_Default(t *testing.T) {
	c := New("", backend.Config{})
	args := c.buildCLIArgs("hello world")

	// Verify the fixed args are present.
	expected := []string{
		"--print",
		"--output-format", "stream-json",
		"--allowedTools", "Bash,Read,Write,Edit,Glob,Grep",
		"--dangerously-skip-permissions",
		"-p", "hello world",
	}
	if len(args) != len(expected) {
		t.Fatalf("expected %d args, got %d: %v", len(expected), len(args), args)
	}
	for i, a := range expected {
		if args[i] != a {
			t.Errorf("args[%d] = %q, want %q", i, args[i], a)
		}
	}
}

func TestBuildCLIArgs_WithModel(t *testing.T) {
	c := New("", backend.Config{Model: "claude-sonnet-4-5"})
	args := c.buildCLIArgs("test prompt")

	// Should contain --model flag.
	found := false
	for i, a := range args {
		if a == "--model" && i+1 < len(args) && args[i+1] == "claude-sonnet-4-5" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected --model claude-sonnet-4-5 in args: %v", args)
	}
}

func TestBuildCLIArgs_WithMaxTurns(t *testing.T) {
	c := New("", backend.Config{MaxTurns: 10})
	args := c.buildCLIArgs("test prompt")

	found := false
	for i, a := range args {
		if a == "--max-turns" && i+1 < len(args) && args[i+1] == "10" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected --max-turns 10 in args: %v", args)
	}
}

func TestBuildPrompt(t *testing.T) {
	tests := []struct {
		name     string
		system   string
		user     string
		expected string
	}{
		{"user only", "", "hello", "hello"},
		{"system and user", "be helpful", "hello", "be helpful\n\nhello"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildPrompt(tt.system, tt.user)
			if got != tt.expected {
				t.Errorf("buildPrompt(%q, %q) = %q, want %q", tt.system, tt.user, got, tt.expected)
			}
		})
	}
}

func TestDedupStats_ReturnsNil(t *testing.T) {
	c := New("", backend.Config{})
	if c.DedupStats() != nil {
		t.Error("expected DedupStats() to return nil for CLI backend")
	}
}

func TestCommitCount_ReturnsZero(t *testing.T) {
	c := New("", backend.Config{})
	if c.CommitCount() != 0 {
		t.Error("expected CommitCount() to return 0 for CLI backend")
	}
}

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
