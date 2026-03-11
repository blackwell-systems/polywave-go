package engine

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/journal"
)

// TestRunScoutMissingFeature verifies that RunScout returns an error when Feature is empty.
func TestRunScoutMissingFeature(t *testing.T) {
	err := RunScout(context.Background(), RunScoutOpts{
		Feature:     "",
		RepoPath:    "/tmp/repo",
		IMPLOutPath: "/tmp/repo/IMPL.md",
	}, func(string) {})
	if err == nil {
		t.Fatal("expected error when Feature is empty, got nil")
	}
}

// TestStartWaveEmptyIMPL verifies that StartWave returns an error when the IMPL path does not exist.
func TestStartWaveEmptyIMPL(t *testing.T) {
	err := StartWave(context.Background(), RunWaveOpts{
		IMPLPath: "/nonexistent/path/IMPL.md",
		RepoPath: "/tmp/repo",
		Slug:     "test",
	}, func(Event) {})
	if err == nil {
		t.Fatal("expected error when IMPL path does not exist, got nil")
	}
}

// TestReadContextMDMissing verifies that readContextMD returns "" when no CONTEXT.md exists.
func TestReadContextMDMissing(t *testing.T) {
	dir := t.TempDir()
	result := readContextMD(dir)
	if result != "" {
		t.Errorf("expected empty string for missing CONTEXT.md, got %q", result)
	}
}

// TestParseIMPLDocDelegate writes a minimal IMPL doc and verifies ParseIMPLDoc returns non-nil.
func TestParseIMPLDocDelegate(t *testing.T) {
	dir := t.TempDir()
	implPath := filepath.Join(dir, "IMPL-test.md")

	content := `# IMPL: Test Feature

## Feature Name
Test Feature

## Status
| Agent | Status |
|-------|--------|
| A     | [ ]    |

## Wave 1
| Agent | Files Owned | Description |
|-------|-------------|-------------|
| A     | pkg/foo.go  | implement foo |
`
	if err := os.WriteFile(implPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test IMPL doc: %v", err)
	}

	doc, err := ParseIMPLDoc(implPath)
	if err != nil {
		t.Fatalf("ParseIMPLDoc returned error: %v", err)
	}
	if doc == nil {
		t.Fatal("ParseIMPLDoc returned nil doc")
	}
}

// TestJournalIntegration_PrepareAgentContext verifies that PrepareAgentContext
// creates a journal observer and returns the original prompt when no journal exists.
func TestJournalIntegration_PrepareAgentContext(t *testing.T) {
	dir := t.TempDir()
	ji := NewJournalIntegration(dir, nil)

	originalPrompt := "Test agent prompt"
	enriched, observer, err := ji.PrepareAgentContext(1, "A", originalPrompt)

	if err != nil {
		t.Fatalf("PrepareAgentContext returned error: %v", err)
	}
	if observer == nil {
		t.Fatal("PrepareAgentContext returned nil observer")
	}
	// With no journal entries, prompt should be unchanged
	if enriched != originalPrompt {
		t.Errorf("expected prompt unchanged when journal empty, got %q", enriched)
	}
}

// TestJournalIntegration_StartPeriodicSync verifies that periodic sync can be started and stopped.
func TestJournalIntegration_StartPeriodicSync(t *testing.T) {
	dir := t.TempDir()
	ji := NewJournalIntegration(dir, nil)

	// Create observer
	observer, err := journal.NewObserver(dir, "wave1/agent-A")
	if err != nil {
		t.Fatalf("failed to create observer: %v", err)
	}

	ctx := context.Background()
	cancel := ji.StartPeriodicSync(ctx, observer)
	defer cancel()

	// Wait a bit to ensure goroutine starts
	time.Sleep(100 * time.Millisecond)

	// Cancel should not panic
	cancel()
}

// TestJournalIntegration_TriggerCheckpoint verifies checkpoint creation.
func TestJournalIntegration_TriggerCheckpoint(t *testing.T) {
	dir := t.TempDir()
	ji := NewJournalIntegration(dir, nil)

	// Create observer
	observer, err := journal.NewObserver(dir, "wave1/agent-A")
	if err != nil {
		t.Fatalf("failed to create observer: %v", err)
	}

	// Create checkpoint
	if err := ji.TriggerCheckpoint(observer, "001-isolation"); err != nil {
		t.Fatalf("TriggerCheckpoint failed: %v", err)
	}

	// Verify checkpoint was created
	checkpoints, err := observer.ListCheckpoints()
	if err != nil {
		t.Fatalf("ListCheckpoints failed: %v", err)
	}
	if len(checkpoints) != 1 {
		t.Fatalf("expected 1 checkpoint, got %d", len(checkpoints))
	}
	if checkpoints[0].Name != "001-isolation" {
		t.Errorf("expected checkpoint name '001-isolation', got %q", checkpoints[0].Name)
	}
}

// TestJournalIntegration_ArchiveJournal verifies journal archival.
func TestJournalIntegration_ArchiveJournal(t *testing.T) {
	dir := t.TempDir()
	ji := NewJournalIntegration(dir, nil)

	// Create observer - use the format that archive.go expects (wave1/agent-A)
	observer, err := journal.NewObserver(dir, "wave1/agent-A")
	if err != nil {
		t.Fatalf("failed to create observer: %v", err)
	}

	t.Logf("JournalDir: %s", observer.JournalDir)
	t.Logf("ProjectRoot: %s", observer.ProjectRoot)

	// Write some test data to journal (observer already created the directory)
	testData := `{"test": "data"}`
	indexPath := filepath.Join(observer.JournalDir, "index.jsonl")
	if err := os.WriteFile(indexPath, []byte(testData), 0644); err != nil {
		t.Fatalf("failed to write test journal: %v", err)
	}

	// Archive
	if err := ji.ArchiveJournal(observer); err != nil {
		t.Fatalf("ArchiveJournal failed: %v", err)
	}

	// Verify archive was created
	// The archive name is derived from the journal directory structure
	// which NewObserver creates as .saw-state/wave1/agent/A (parsing wave1/agent-A)
	// So the archive becomes wave1-agent-agent.tar.gz (agent is the folder name)
	archiveDir := filepath.Join(dir, ".saw-state", "archive")
	entries, err := os.ReadDir(archiveDir)
	if err != nil {
		t.Fatalf("failed to read archive directory: %v", err)
	}

	// Check that at least one .tar.gz file was created
	foundArchive := false
	for _, e := range entries {
		if filepath.Ext(e.Name()) == ".gz" {
			foundArchive = true
			t.Logf("Found archive: %s", e.Name())
			break
		}
	}

	if !foundArchive {
		t.Errorf("no .tar.gz archive file found in %s", archiveDir)
	}
}

// TestJournalIntegration_NilObserver verifies that journal operations are safe with nil observer.
func TestJournalIntegration_NilObserver(t *testing.T) {
	dir := t.TempDir()
	ji := NewJournalIntegration(dir, nil)

	// All operations should be no-ops with nil observer
	ctx := context.Background()
	cancel := ji.StartPeriodicSync(ctx, nil)
	cancel() // Should not panic

	if err := ji.TriggerCheckpoint(nil, "test"); err != nil {
		t.Errorf("TriggerCheckpoint with nil observer returned error: %v", err)
	}

	if err := ji.ArchiveJournal(nil); err != nil {
		t.Errorf("ArchiveJournal with nil observer returned error: %v", err)
	}
}
