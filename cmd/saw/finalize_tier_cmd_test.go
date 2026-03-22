package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// TestFinalizeTierCmd_NoArgs verifies that the command errors when no manifest
// argument is provided (cobra ExactArgs(1) constraint).
func TestFinalizeTierCmd_NoArgs(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	cmd := newFinalizeTierCmd()
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"--tier", "1"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error when no manifest argument is provided, got nil")
	}
}

// TestFinalizeTierCmd_NoTierFlag verifies that the command errors when the
// required --tier flag is not provided.
func TestFinalizeTierCmd_NoTierFlag(t *testing.T) {
	tmpDir := t.TempDir()
	manifestPath := filepath.Join(tmpDir, "PROGRAM-test.yaml")

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	cmd := newFinalizeTierCmd()
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{manifestPath})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error when --tier flag is missing, got nil")
	}
}

// TestFinalizeTierCmd_MissingManifest verifies that the command returns a parse
// error when the manifest file does not exist.
func TestFinalizeTierCmd_MissingManifest(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	nonexistent := filepath.Join(os.TempDir(), "does-not-exist-program.yaml")

	cmd := newFinalizeTierCmd()
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{nonexistent, "--tier", "1"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for nonexistent manifest file, got nil")
	}
}
