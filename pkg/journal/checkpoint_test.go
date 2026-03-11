package journal

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// JournalObserver is defined in observer.go - no need to redefine for tests

// setupTestObserver creates a test observer with temporary directories
func setupTestObserver(t *testing.T) (*JournalObserver, func()) {
	tmpDir := t.TempDir()
	journalDir := filepath.Join(tmpDir, ".saw-state", "wave1", "agent-test")

	if err := os.MkdirAll(journalDir, 0755); err != nil {
		t.Fatalf("failed to create journal dir: %v", err)
	}

	observer := &JournalObserver{
		ProjectRoot: Path(tmpDir),
		JournalDir:  Path(journalDir),
		AgentID:     "agent-test",
		cursorPath:  Path(filepath.Join(journalDir, "cursor.json")),
		indexPath:   Path(filepath.Join(journalDir, "index.jsonl")),
		recentPath:  Path(filepath.Join(journalDir, "recent.json")),
		resultsDir:  Path(filepath.Join(journalDir, "tool-results")),
	}

	cleanup := func() {
		// t.TempDir() handles cleanup automatically
	}

	return observer, cleanup
}

// writeTestIndex creates a test index.jsonl file with the given number of entries
func writeTestIndex(t *testing.T, path string, entryCount int) {
	t.Helper()
	file, err := os.Create(path)
	if err != nil {
		t.Fatalf("failed to create test index: %v", err)
	}
	defer file.Close()

	for i := 0; i < entryCount; i++ {
		entry := map[string]interface{}{
			"ts":          time.Now().Format(time.RFC3339),
			"kind":        "tool_use",
			"tool_use_id": "test-id-" + string(rune(i)),
		}
		data, _ := json.Marshal(entry)
		file.WriteString(string(data) + "\n")
	}
}

// writeTestCursor creates a test cursor.json file
func writeTestCursor(t *testing.T, path string) {
	t.Helper()
	cursor := SessionCursor{
		SessionFile: "test-session.jsonl",
		Offset:      1024,
	}
	data, err := json.MarshalIndent(cursor, "", "  ")
	if err != nil {
		t.Fatalf("failed to marshal cursor: %v", err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatalf("failed to write cursor: %v", err)
	}
}

func TestCheckpoint_CreatesSnapshot(t *testing.T) {
	observer, cleanup := setupTestObserver(t)
	defer cleanup()

	// Create test journal files
	writeTestIndex(t, string(observer.indexPath), 5)
	writeTestCursor(t, string(observer.cursorPath))

	// Create checkpoint
	err := observer.Checkpoint("test-checkpoint")
	if err != nil {
		t.Fatalf("Checkpoint failed: %v", err)
	}

	// Verify snapshot files exist
	checkpointsDir := filepath.Join(string(observer.JournalDir), "checkpoints")

	expectedFiles := []string{
		filepath.Join(checkpointsDir, "test-checkpoint.json"),
		filepath.Join(checkpointsDir, "test-checkpoint-index.jsonl"),
		filepath.Join(checkpointsDir, "test-checkpoint-cursor.json"),
	}

	for _, file := range expectedFiles {
		if _, err := os.Stat(file); os.IsNotExist(err) {
			t.Errorf("Expected snapshot file not created: %s", file)
		}
	}
}

func TestCheckpoint_SavesMetadata(t *testing.T) {
	observer, cleanup := setupTestObserver(t)
	defer cleanup()

	// Create test journal files
	writeTestIndex(t, string(observer.indexPath), 3)
	writeTestCursor(t, string(observer.cursorPath))

	// Create checkpoint
	err := observer.Checkpoint("metadata-test")
	if err != nil {
		t.Fatalf("Checkpoint failed: %v", err)
	}

	// Read and verify metadata
	metadataPath := filepath.Join(string(observer.JournalDir), "checkpoints", "metadata-test.json")
	data, err := os.ReadFile(metadataPath)
	if err != nil {
		t.Fatalf("Failed to read metadata: %v", err)
	}

	var checkpoint Checkpoint
	if err := json.Unmarshal(data, &checkpoint); err != nil {
		t.Fatalf("Failed to unmarshal metadata: %v", err)
	}

	if checkpoint.Name != "metadata-test" {
		t.Errorf("Expected name 'metadata-test', got %q", checkpoint.Name)
	}
	if checkpoint.EntryCount != 3 {
		t.Errorf("Expected entry count 3, got %d", checkpoint.EntryCount)
	}
	if checkpoint.Timestamp.IsZero() {
		t.Error("Expected non-zero timestamp")
	}
}

func TestListCheckpoints_ReturnsAll(t *testing.T) {
	observer, cleanup := setupTestObserver(t)
	defer cleanup()

	// Create test journal files
	writeTestIndex(t, string(observer.indexPath), 2)
	writeTestCursor(t, string(observer.cursorPath))

	// Create multiple checkpoints with delays to ensure different timestamps
	checkpoints := []string{"001-first", "002-second", "003-third"}
	for _, name := range checkpoints {
		if err := observer.Checkpoint(name); err != nil {
			t.Fatalf("Failed to create checkpoint %s: %v", name, err)
		}
		time.Sleep(10 * time.Millisecond) // Ensure different timestamps
	}

	// List checkpoints
	list, err := observer.ListCheckpoints()
	if err != nil {
		t.Fatalf("ListCheckpoints failed: %v", err)
	}

	if len(list) != 3 {
		t.Logf("Checkpoint names found:")
		for _, cp := range list {
			t.Logf("  - %s (timestamp: %v)", cp.Name, cp.Timestamp)
		}
		t.Fatalf("Expected 3 checkpoints, got %d", len(list))
	}

	// Verify sorted by timestamp (oldest first)
	for i := 0; i < len(list)-1; i++ {
		if !list[i].Timestamp.Before(list[i+1].Timestamp) {
			t.Errorf("Checkpoints not sorted by timestamp: %v >= %v", list[i].Timestamp, list[i+1].Timestamp)
		}
	}
}

func TestRestoreCheckpoint_RollsBackIndex(t *testing.T) {
	observer, cleanup := setupTestObserver(t)
	defer cleanup()

	// Create initial state with 5 entries
	writeTestIndex(t, string(observer.indexPath), 5)
	writeTestCursor(t, string(observer.cursorPath))

	// Create checkpoint
	if err := observer.Checkpoint("rollback-test"); err != nil {
		t.Fatalf("Checkpoint failed: %v", err)
	}

	// Modify index (simulate more entries added)
	writeTestIndex(t, string(observer.indexPath), 10)

	// Verify index now has 10 entries
	count, err := countJSONLLines(string(observer.indexPath))
	if err != nil {
		t.Fatalf("Failed to count lines: %v", err)
	}
	if count != 10 {
		t.Fatalf("Expected 10 entries before restore, got %d", count)
	}

	// Restore checkpoint
	if err := observer.RestoreCheckpoint("rollback-test"); err != nil {
		t.Fatalf("RestoreCheckpoint failed: %v", err)
	}

	// Verify index rolled back to 5 entries
	count, err = countJSONLLines(string(observer.indexPath))
	if err != nil {
		t.Fatalf("Failed to count lines after restore: %v", err)
	}
	if count != 5 {
		t.Errorf("Expected 5 entries after restore, got %d", count)
	}
}

func TestRestoreCheckpoint_RollsBackCursor(t *testing.T) {
	observer, cleanup := setupTestObserver(t)
	defer cleanup()

	// Create initial cursor
	originalCursor := SessionCursor{SessionFile: "session-1.jsonl", Offset: 100}
	data, _ := json.Marshal(originalCursor)
	os.WriteFile(string(observer.cursorPath), data, 0644)

	// Create checkpoint
	if err := observer.Checkpoint("cursor-test"); err != nil {
		t.Fatalf("Checkpoint failed: %v", err)
	}

	// Modify cursor
	modifiedCursor := SessionCursor{SessionFile: "session-2.jsonl", Offset: 500}
	data, _ = json.Marshal(modifiedCursor)
	os.WriteFile(string(observer.cursorPath), data, 0644)

	// Restore checkpoint
	if err := observer.RestoreCheckpoint("cursor-test"); err != nil {
		t.Fatalf("RestoreCheckpoint failed: %v", err)
	}

	// Verify cursor restored
	data, err := os.ReadFile(string(observer.cursorPath))
	if err != nil {
		t.Fatalf("Failed to read cursor: %v", err)
	}

	var restored SessionCursor
	if err := json.Unmarshal(data, &restored); err != nil {
		t.Fatalf("Failed to unmarshal cursor: %v", err)
	}

	if restored.SessionFile != originalCursor.SessionFile || restored.Offset != originalCursor.Offset {
		t.Errorf("Cursor not restored correctly: got %+v, want %+v", restored, originalCursor)
	}
}

func TestRestoreCheckpoint_PreservesCheckpoints(t *testing.T) {
	observer, cleanup := setupTestObserver(t)
	defer cleanup()

	// Create test journal files
	writeTestIndex(t, string(observer.indexPath), 2)
	writeTestCursor(t, string(observer.cursorPath))

	// Create checkpoint
	if err := observer.Checkpoint("preserve-test"); err != nil {
		t.Fatalf("Checkpoint failed: %v", err)
	}

	// Restore checkpoint
	if err := observer.RestoreCheckpoint("preserve-test"); err != nil {
		t.Fatalf("RestoreCheckpoint failed: %v", err)
	}

	// Verify checkpoint still exists
	metadataPath := filepath.Join(string(observer.JournalDir), "checkpoints", "preserve-test.json")
	if _, err := os.Stat(metadataPath); os.IsNotExist(err) {
		t.Error("Checkpoint metadata deleted after restore (should be preserved)")
	}

	// Verify we can restore again
	if err := observer.RestoreCheckpoint("preserve-test"); err != nil {
		t.Errorf("Cannot restore checkpoint twice: %v", err)
	}
}

func TestDeleteCheckpoint_RemovesFiles(t *testing.T) {
	observer, cleanup := setupTestObserver(t)
	defer cleanup()

	// Create test journal files
	writeTestIndex(t, string(observer.indexPath), 2)
	writeTestCursor(t, string(observer.cursorPath))

	// Create checkpoint
	if err := observer.Checkpoint("delete-test"); err != nil {
		t.Fatalf("Checkpoint failed: %v", err)
	}

	// Delete checkpoint
	if err := observer.DeleteCheckpoint("delete-test"); err != nil {
		t.Fatalf("DeleteCheckpoint failed: %v", err)
	}

	// Verify all files removed
	checkpointsDir := filepath.Join(string(observer.JournalDir), "checkpoints")
	files := []string{
		filepath.Join(checkpointsDir, "delete-test.json"),
		filepath.Join(checkpointsDir, "delete-test-index.jsonl"),
		filepath.Join(checkpointsDir, "delete-test-cursor.json"),
	}

	for _, file := range files {
		if _, err := os.Stat(file); !os.IsNotExist(err) {
			t.Errorf("File not deleted: %s", file)
		}
	}
}

func TestCheckpoint_DuplicateName(t *testing.T) {
	observer, cleanup := setupTestObserver(t)
	defer cleanup()

	// Create test journal files
	writeTestIndex(t, string(observer.indexPath), 2)
	writeTestCursor(t, string(observer.cursorPath))

	// Create first checkpoint
	if err := observer.Checkpoint("duplicate"); err != nil {
		t.Fatalf("First checkpoint failed: %v", err)
	}

	// Try to create duplicate
	err := observer.Checkpoint("duplicate")
	if err == nil {
		t.Fatal("Expected error for duplicate checkpoint name, got nil")
	}
	if err.Error() != `checkpoint "duplicate" already exists` {
		t.Errorf("Wrong error message: %v", err)
	}
}

func TestRestoreCheckpoint_NonexistentName(t *testing.T) {
	observer, cleanup := setupTestObserver(t)
	defer cleanup()

	// Try to restore non-existent checkpoint
	err := observer.RestoreCheckpoint("nonexistent")
	if err == nil {
		t.Fatal("Expected error for nonexistent checkpoint, got nil")
	}
	if err.Error() != `checkpoint "nonexistent" not found` {
		t.Errorf("Wrong error message: %v", err)
	}
}

func TestCheckpoint_CountsEntries(t *testing.T) {
	observer, cleanup := setupTestObserver(t)
	defer cleanup()

	// Create index with known number of entries
	entryCount := 7
	writeTestIndex(t, string(observer.indexPath), entryCount)
	writeTestCursor(t, string(observer.cursorPath))

	// Create checkpoint
	if err := observer.Checkpoint("count-test"); err != nil {
		t.Fatalf("Checkpoint failed: %v", err)
	}

	// Verify entry count in metadata
	metadataPath := filepath.Join(string(observer.JournalDir), "checkpoints", "count-test.json")
	data, err := os.ReadFile(metadataPath)
	if err != nil {
		t.Fatalf("Failed to read metadata: %v", err)
	}

	var checkpoint Checkpoint
	if err := json.Unmarshal(data, &checkpoint); err != nil {
		t.Fatalf("Failed to unmarshal metadata: %v", err)
	}

	if checkpoint.EntryCount != entryCount {
		t.Errorf("Expected entry count %d, got %d", entryCount, checkpoint.EntryCount)
	}
}

func TestCheckpoint_EmptyJournal(t *testing.T) {
	observer, cleanup := setupTestObserver(t)
	defer cleanup()

	// Create checkpoint with no journal files (valid empty state)
	err := observer.Checkpoint("empty-state")
	if err != nil {
		t.Fatalf("Checkpoint of empty journal should succeed, got: %v", err)
	}

	// Verify checkpoint created with zero entries
	metadataPath := filepath.Join(string(observer.JournalDir), "checkpoints", "empty-state.json")
	data, err := os.ReadFile(metadataPath)
	if err != nil {
		t.Fatalf("Failed to read metadata: %v", err)
	}

	var checkpoint Checkpoint
	if err := json.Unmarshal(data, &checkpoint); err != nil {
		t.Fatalf("Failed to unmarshal metadata: %v", err)
	}

	if checkpoint.EntryCount != 0 {
		t.Errorf("Expected entry count 0 for empty journal, got %d", checkpoint.EntryCount)
	}
}

func TestCheckpoint_InvalidNames(t *testing.T) {
	observer, cleanup := setupTestObserver(t)
	defer cleanup()

	invalidNames := []string{
		"",              // empty
		"foo/bar",       // slash
		"foo\\bar",      // backslash
		"foo bar",       // space
		"foo/bar/baz",   // multiple slashes
	}

	for _, name := range invalidNames {
		err := observer.Checkpoint(name)
		if err == nil {
			t.Errorf("Expected error for invalid checkpoint name %q, got nil", name)
		}
	}
}
