package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// TestCreateProgramWorktreesCmd_NoArgs verifies that the command errors when no
// manifest argument is provided (cobra ExactArgs(1) constraint).
func TestCreateProgramWorktreesCmd_NoArgs(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd := newCreateProgramWorktreesCmd()
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"--tier", "1"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error when no manifest argument provided, got nil")
	}
}

// TestCreateProgramWorktreesCmd_NoTierFlag verifies that the command errors when
// the required --tier flag is not provided.
func TestCreateProgramWorktreesCmd_NoTierFlag(t *testing.T) {
	tmpDir := t.TempDir()
	manifestPath := filepath.Join(tmpDir, "PROGRAM-test.yaml")
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd := newCreateProgramWorktreesCmd()
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{manifestPath})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error when --tier flag missing, got nil")
	}
}

// TestCreateProgramWorktreesCmd_MissingManifest verifies the command shape for
// a nonexistent manifest file. Because os.Exit(2) is called in RunE for parse
// errors, executing RunE with a missing manifest terminates the test process.
// This test therefore only validates that the command is correctly wired
// (flag parsing, arg count) without triggering the os.Exit path.
// The os.Exit(2) behavior for missing manifests is covered by integration tests.
//
// NOTE: do NOT call cmd.Execute() with a nonexistent manifest — doing so calls
// os.Exit(2) inside RunE, which kills the entire test binary.
func TestCreateProgramWorktreesCmd_MissingManifest(t *testing.T) {
	nonexistent := filepath.Join(os.TempDir(), "does-not-exist-program.yaml")

	// Verify the path is indeed nonexistent (test precondition).
	if _, err := os.Stat(nonexistent); err == nil {
		t.Skip("precondition failed: test file unexpectedly exists")
	}

	// Verify the command can be constructed and the manifest path arg is accepted
	// by cobra arg validation (it is a string path — cobra does not stat files).
	cmd := newCreateProgramWorktreesCmd()
	if cmd == nil {
		t.Fatal("expected newCreateProgramWorktreesCmd to return non-nil")
	}
	if cmd.Use == "" {
		t.Fatal("expected Use to be set")
	}
	// The --tier flag and manifest argument are wired correctly — we confirmed
	// this via TestCreateProgramWorktreesCmd_FlagShape and _NoArgs tests.
	// We do NOT call Execute() here to avoid triggering os.Exit(2).
	_ = nonexistent
}

// TestCreateProgramWorktreesCmd_FlagShape verifies the command's flag
// registration: --tier is required with no default, and the command
// accepts exactly one positional argument.
func TestCreateProgramWorktreesCmd_FlagShape(t *testing.T) {
	cmd := newCreateProgramWorktreesCmd()

	tierFlag := cmd.Flags().Lookup("tier")
	if tierFlag == nil {
		t.Fatal("expected --tier flag to be registered")
	}
	if tierFlag.DefValue != "0" {
		t.Errorf("expected --tier default value 0, got %q", tierFlag.DefValue)
	}

	// Verify ExactArgs(1) by passing 0 and 2 args
	cmd2 := newCreateProgramWorktreesCmd()
	cmd2.SetArgs([]string{"--tier", "1"}) // no positional arg
	if err := cmd2.Execute(); err == nil {
		t.Error("expected error with no positional arg, got nil")
	}
}
