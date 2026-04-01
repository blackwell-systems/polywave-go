package workspace

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/protocol"
)

// TestDetectAndSetup_SkipsAllWhenEmptyDir verifies that an empty directory
// produces a "success" result without panicking.
func TestDetectAndSetup_SkipsAllWhenEmptyDir(t *testing.T) {
	dir := t.TempDir()

	result := DetectAndSetup(context.Background(), dir, 1, nil, nil, nil)

	if result == nil {
		t.Fatal("expected non-nil StepResult")
	}
	if result.Status != "success" {
		t.Errorf("expected status=success, got %q", result.Status)
	}
	if result.Step != "workspace_setup" {
		t.Errorf("expected step=workspace_setup, got %q", result.Step)
	}
}

// TestDetectAndSetup_GoRepo_NonFatal creates a tmpdir with go.mod,
// calls DetectAndSetup, and verifies result is non-nil with an acceptable status.
func TestDetectAndSetup_GoRepo_NonFatal(t *testing.T) {
	dir := t.TempDir()

	// Write a minimal go.mod so GoWorkspaceManager.Detect returns true.
	gomod := []byte("module example.com/test\n\ngo 1.21\n")
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), gomod, 0644); err != nil {
		t.Fatalf("writing go.mod: %v", err)
	}

	result := DetectAndSetup(context.Background(), dir, 1, nil, nil, nil)

	if result == nil {
		t.Fatal("expected non-nil StepResult")
	}
	// Status is "success" or "warning" depending on whether go work init succeeds.
	if result.Status != "success" && result.Status != "warning" {
		t.Errorf("expected success or warning, got %q", result.Status)
	}
}

// TestDetectAndRestore_GoRepo_RemovesGoWork creates a tmpdir with go.mod and go.work,
// calls DetectAndRestore, and verifies go.work is removed (no backup = delete).
func TestDetectAndRestore_GoRepo_RemovesGoWork(t *testing.T) {
	dir := t.TempDir()

	// Write go.mod so GoWorkspaceManager.Detect returns true.
	gomod := []byte("module example.com/test\n\ngo 1.21\n")
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), gomod, 0644); err != nil {
		t.Fatalf("writing go.mod: %v", err)
	}

	// Write go.work to simulate a SAW-created workspace file.
	gowork := []byte("go 1.21\n\nuse .\n\n// SAW-managed: restored by finalize-wave\n")
	if err := os.WriteFile(filepath.Join(dir, "go.work"), gowork, 0644); err != nil {
		t.Fatalf("writing go.work: %v", err)
	}

	result := DetectAndRestore(context.Background(), dir, 1, nil, nil)

	if result == nil {
		t.Fatal("expected non-nil StepResult")
	}

	// go.work should be removed (no backup existed for wave 1).
	if _, err := os.Stat(filepath.Join(dir, "go.work")); !os.IsNotExist(err) {
		t.Errorf("expected go.work to be removed after DetectAndRestore")
	}
}

// TestDetectAndSetup_MultiLanguage creates a tmpdir with both go.mod and tsconfig.json,
// calls DetectAndSetup, and verifies tsconfig.json was backed up (evidence both managers ran).
func TestDetectAndSetup_MultiLanguage(t *testing.T) {
	dir := t.TempDir()

	// Write go.mod so GoWorkspaceManager detects Go.
	gomod := []byte("module example.com/test\n\ngo 1.21\n")
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), gomod, 0644); err != nil {
		t.Fatalf("writing go.mod: %v", err)
	}

	// Write a valid tsconfig.json so TypeScriptWorkspaceManager detects TypeScript.
	tsconfig := []byte(`{"compilerOptions": {"strict": true}}`)
	if err := os.WriteFile(filepath.Join(dir, "tsconfig.json"), tsconfig, 0644); err != nil {
		t.Fatalf("writing tsconfig.json: %v", err)
	}

	worktrees := []protocol.WorktreeInfo{
		{Agent: "A", Path: dir, Branch: "test-branch"},
	}

	result := DetectAndSetup(context.Background(), dir, 1, worktrees, nil, nil)

	if result == nil {
		t.Fatal("expected non-nil StepResult")
	}

	// TypeScript manager should have backed up tsconfig.json to .saw-state/wave1/tsconfig.json.backup.
	backupPath := filepath.Join(BackupDir(dir, 1), "tsconfig.json.backup")
	if _, err := os.Stat(backupPath); os.IsNotExist(err) {
		t.Errorf("expected tsconfig.json.backup to exist at %s (evidence TypeScript manager ran)", backupPath)
	}
}

// TestDetectAndSetup_OnEventCalled verifies that a capturing onEvent func
// receives at least one "workspace_setup" call.
func TestDetectAndSetup_OnEventCalled(t *testing.T) {
	dir := t.TempDir()

	// Write go.mod so at least GoWorkspaceManager fires an event.
	gomod := []byte("module example.com/test\n\ngo 1.21\n")
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), gomod, 0644); err != nil {
		t.Fatalf("writing go.mod: %v", err)
	}

	var events []string
	onEvent := func(step, status, detail string) {
		events = append(events, step)
	}

	DetectAndSetup(context.Background(), dir, 1, nil, onEvent, nil)

	// Verify "workspace_setup" (the overall event) was received.
	found := false
	for _, e := range events {
		if e == "workspace_setup" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected 'workspace_setup' event; got events: %v", events)
	}
}
