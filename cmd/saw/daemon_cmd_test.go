package main

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"
)

// TestDaemonCmd_Flags verifies that the daemon command parses all expected flags.
func TestDaemonCmd_Flags(t *testing.T) {
	cmd := newDaemonCmd()

	// Verify flag existence and defaults.
	autonomyFlag := cmd.Flags().Lookup("autonomy")
	if autonomyFlag == nil {
		t.Fatal("expected --autonomy flag to exist")
	}
	if autonomyFlag.DefValue != "" {
		t.Errorf("expected --autonomy default to be empty, got %q", autonomyFlag.DefValue)
	}

	modelFlag := cmd.Flags().Lookup("model")
	if modelFlag == nil {
		t.Fatal("expected --model flag to exist")
	}
	if modelFlag.DefValue != "" {
		t.Errorf("expected --model default to be empty, got %q", modelFlag.DefValue)
	}

	pollFlag := cmd.Flags().Lookup("poll-interval")
	if pollFlag == nil {
		t.Fatal("expected --poll-interval flag to exist")
	}
	expected := (30 * time.Second).String()
	if pollFlag.DefValue != expected {
		t.Errorf("expected --poll-interval default %q, got %q", expected, pollFlag.DefValue)
	}
}

// TestDaemonCmd_DefaultConfig verifies the daemon loads config from saw.config.json
// (falls back to defaults when file is missing).
func TestDaemonCmd_DefaultConfig(t *testing.T) {
	tmpDir := t.TempDir()

	origRepoDir := repoDir
	repoDir = tmpDir
	defer func() { repoDir = origRepoDir }()

	// Use a context that cancels quickly so RunDaemon exits.
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	var stderr bytes.Buffer
	cmd := newDaemonCmd()
	cmd.SetContext(ctx)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"--poll-interval", "50ms"})

	err := cmd.Execute()

	// RunDaemon should return after context cancellation.
	// The key assertion: error should NOT be about config loading
	// (since DefaultConfig is used when saw.config.json is missing).
	if err != nil && strings.Contains(err.Error(), "load config") {
		t.Errorf("expected default config to be used when saw.config.json is missing, got error: %v", err)
	}
}

// TestDaemonCmd_OverrideLevel verifies the --autonomy flag overrides config level.
func TestDaemonCmd_OverrideLevel(t *testing.T) {
	tmpDir := t.TempDir()

	origRepoDir := repoDir
	repoDir = tmpDir
	defer func() { repoDir = origRepoDir }()

	// Test with invalid level -- should fail at parse time (before RunDaemon).
	var stderr bytes.Buffer
	cmd := newDaemonCmd()
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"--autonomy", "invalid-level"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for invalid autonomy level")
	}
	if !strings.Contains(err.Error(), "invalid autonomy level") {
		t.Errorf("expected 'invalid autonomy level' error, got: %v", err)
	}

	// Test with valid level -- should pass config loading and enter RunDaemon.
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	cmd2 := newDaemonCmd()
	cmd2.SetContext(ctx)
	cmd2.SetErr(&bytes.Buffer{})
	cmd2.SetArgs([]string{"--autonomy", "autonomous", "--poll-interval", "50ms"})

	err2 := cmd2.Execute()
	if err2 != nil && strings.Contains(err2.Error(), "invalid autonomy level") {
		t.Errorf("valid level 'autonomous' should not produce invalid level error, got: %v", err2)
	}
}
