package journal

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/result"
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
		ProjectRoot: tmpDir,
		JournalDir:  journalDir,
		AgentID:     "agent-test",
		CursorPath:  filepath.Join(journalDir, "cursor.json"),
		IndexPath:   filepath.Join(journalDir, "index.jsonl"),
		RecentPath:  filepath.Join(journalDir, "recent.json"),
		ResultsDir:  filepath.Join(journalDir, "tool-results"),
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
	writeTestIndex(t, string(observer.IndexPath), 5)
	writeTestCursor(t, string(observer.CursorPath))

	// Create checkpoint
	r := observer.Checkpoint("test-checkpoint")
	if !r.IsSuccess() {
		t.Fatalf("Checkpoint failed: %v", r.Errors[0].Message)
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
	writeTestIndex(t, string(observer.IndexPath), 3)
	writeTestCursor(t, string(observer.CursorPath))

	// Create checkpoint
	r := observer.Checkpoint("metadata-test")
	if !r.IsSuccess() {
		t.Fatalf("Checkpoint failed: %v", r.Errors[0].Message)
	}

	// Verify result data
	data := r.GetData()
	if data.Name != "metadata-test" {
		t.Errorf("Expected name 'metadata-test', got %q", data.Name)
	}
	if data.EntryCount != 3 {
		t.Errorf("Expected entry count 3, got %d", data.EntryCount)
	}
	if data.Timestamp.IsZero() {
		t.Error("Expected non-zero timestamp")
	}

	// Read and verify metadata file
	metadataPath := filepath.Join(string(observer.JournalDir), "checkpoints", "metadata-test.json")
	fileData, err := os.ReadFile(metadataPath)
	if err != nil {
		t.Fatalf("Failed to read metadata: %v", err)
	}

	var checkpoint CheckpointRecord
	if err := json.Unmarshal(fileData, &checkpoint); err != nil {
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
	writeTestIndex(t, string(observer.IndexPath), 2)
	writeTestCursor(t, string(observer.CursorPath))

	// Create multiple checkpoints with delays to ensure different timestamps
	checkpoints := []string{"001-first", "002-second", "003-third"}
	for _, name := range checkpoints {
		r := observer.Checkpoint(name)
		if !r.IsSuccess() {
			t.Fatalf("Failed to create checkpoint %s: %v", name, r.Errors[0].Message)
		}
		time.Sleep(10 * time.Millisecond) // Ensure different timestamps
	}

	// List checkpoints
	lr := observer.ListCheckpoints()
	if !lr.IsSuccess() {
		t.Fatalf("ListCheckpoints failed: %v", lr.Errors[0].Message)
	}
	list := lr.GetData()

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
	writeTestIndex(t, string(observer.IndexPath), 5)
	writeTestCursor(t, string(observer.CursorPath))

	// Create checkpoint
	r := observer.Checkpoint("rollback-test")
	if !r.IsSuccess() {
		t.Fatalf("Checkpoint failed: %v", r.Errors[0].Message)
	}

	// Modify index (simulate more entries added)
	writeTestIndex(t, string(observer.IndexPath), 10)

	// Verify index now has 10 entries
	count, err := countJSONLLines(string(observer.IndexPath))
	if err != nil {
		t.Fatalf("Failed to count lines: %v", err)
	}
	if count != 10 {
		t.Fatalf("Expected 10 entries before restore, got %d", count)
	}

	// Restore checkpoint
	rr := observer.RestoreCheckpoint("rollback-test")
	if !rr.IsSuccess() {
		t.Fatalf("RestoreCheckpoint failed: %v", rr.Errors[0].Message)
	}

	// Verify index rolled back to 5 entries
	count, err = countJSONLLines(string(observer.IndexPath))
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
	os.WriteFile(string(observer.CursorPath), data, 0644)

	// Create checkpoint
	r := observer.Checkpoint("cursor-test")
	if !r.IsSuccess() {
		t.Fatalf("Checkpoint failed: %v", r.Errors[0].Message)
	}

	// Modify cursor
	modifiedCursor := SessionCursor{SessionFile: "session-2.jsonl", Offset: 500}
	data, _ = json.Marshal(modifiedCursor)
	os.WriteFile(string(observer.CursorPath), data, 0644)

	// Restore checkpoint
	rr := observer.RestoreCheckpoint("cursor-test")
	if !rr.IsSuccess() {
		t.Fatalf("RestoreCheckpoint failed: %v", rr.Errors[0].Message)
	}

	// Verify cursor restored
	data, err := os.ReadFile(string(observer.CursorPath))
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
	writeTestIndex(t, string(observer.IndexPath), 2)
	writeTestCursor(t, string(observer.CursorPath))

	// Create checkpoint
	r := observer.Checkpoint("preserve-test")
	if !r.IsSuccess() {
		t.Fatalf("Checkpoint failed: %v", r.Errors[0].Message)
	}

	// Restore checkpoint
	rr := observer.RestoreCheckpoint("preserve-test")
	if !rr.IsSuccess() {
		t.Fatalf("RestoreCheckpoint failed: %v", rr.Errors[0].Message)
	}

	// Verify checkpoint still exists
	metadataPath := filepath.Join(string(observer.JournalDir), "checkpoints", "preserve-test.json")
	if _, err := os.Stat(metadataPath); os.IsNotExist(err) {
		t.Error("Checkpoint metadata deleted after restore (should be preserved)")
	}

	// Verify we can restore again
	rr2 := observer.RestoreCheckpoint("preserve-test")
	if !rr2.IsSuccess() {
		t.Errorf("Cannot restore checkpoint twice: %v", rr2.Errors[0].Message)
	}
}

func TestDeleteCheckpoint_RemovesFiles(t *testing.T) {
	observer, cleanup := setupTestObserver(t)
	defer cleanup()

	// Create test journal files
	writeTestIndex(t, string(observer.IndexPath), 2)
	writeTestCursor(t, string(observer.CursorPath))

	// Create checkpoint
	r := observer.Checkpoint("delete-test")
	if !r.IsSuccess() {
		t.Fatalf("Checkpoint failed: %v", r.Errors[0].Message)
	}

	// Delete checkpoint
	rd := observer.DeleteCheckpoint("delete-test")
	if !rd.IsSuccess() {
		t.Fatalf("DeleteCheckpoint failed: %v", rd.Errors[0].Message)
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

	// Verify result data
	dd := rd.GetData()
	if !dd.Deleted {
		t.Error("Expected Deleted=true in result data")
	}
	if dd.Name != "delete-test" {
		t.Errorf("Expected Name='delete-test', got %q", dd.Name)
	}
}

func TestCheckpoint_DuplicateName(t *testing.T) {
	observer, cleanup := setupTestObserver(t)
	defer cleanup()

	// Create test journal files
	writeTestIndex(t, string(observer.IndexPath), 2)
	writeTestCursor(t, string(observer.CursorPath))

	// Create first checkpoint
	r := observer.Checkpoint("duplicate")
	if !r.IsSuccess() {
		t.Fatalf("First checkpoint failed: %v", r.Errors[0].Message)
	}

	// Try to create duplicate
	r2 := observer.Checkpoint("duplicate")
	if r2.IsSuccess() {
		t.Fatal("Expected failure for duplicate checkpoint name, got success")
	}
	if !r2.IsFatal() {
		t.Error("Expected IsFatal() to be true for duplicate checkpoint")
	}
	if r2.Errors[0].Code != result.CodeJournalCheckpointAlreadyExists {
		t.Errorf("Wrong error code: %v", r2.Errors[0].Code)
	}
}

func TestRestoreCheckpoint_NonexistentName(t *testing.T) {
	observer, cleanup := setupTestObserver(t)
	defer cleanup()

	// Try to restore non-existent checkpoint
	r := observer.RestoreCheckpoint("nonexistent")
	if r.IsSuccess() {
		t.Fatal("Expected failure for nonexistent checkpoint, got success")
	}
	if !r.IsFatal() {
		t.Error("Expected IsFatal() to be true for nonexistent checkpoint")
	}
	if r.Errors[0].Code != result.CodeJournalCheckpointNotFound {
		t.Errorf("Wrong error code: %v", r.Errors[0].Code)
	}
	if !strings.Contains(r.Errors[0].Message, "nonexistent") {
		t.Errorf("Error message should mention checkpoint name: %v", r.Errors[0].Message)
	}
}

func TestCheckpoint_CountsEntries(t *testing.T) {
	observer, cleanup := setupTestObserver(t)
	defer cleanup()

	// Create index with known number of entries
	entryCount := 7
	writeTestIndex(t, string(observer.IndexPath), entryCount)
	writeTestCursor(t, string(observer.CursorPath))

	// Create checkpoint
	r := observer.Checkpoint("count-test")
	if !r.IsSuccess() {
		t.Fatalf("Checkpoint failed: %v", r.Errors[0].Message)
	}

	// Verify entry count in result data
	data := r.GetData()
	if data.EntryCount != entryCount {
		t.Errorf("Expected entry count %d, got %d", entryCount, data.EntryCount)
	}

	// Also verify via metadata file
	metadataPath := filepath.Join(string(observer.JournalDir), "checkpoints", "count-test.json")
	fileData, err := os.ReadFile(metadataPath)
	if err != nil {
		t.Fatalf("Failed to read metadata: %v", err)
	}

	var checkpoint CheckpointRecord
	if err := json.Unmarshal(fileData, &checkpoint); err != nil {
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
	r := observer.Checkpoint("empty-state")
	if !r.IsSuccess() {
		t.Fatalf("Checkpoint of empty journal should succeed, got: %v", r.Errors[0].Message)
	}

	// Verify checkpoint created with zero entries
	data := r.GetData()
	if data.EntryCount != 0 {
		t.Errorf("Expected entry count 0 for empty journal, got %d", data.EntryCount)
	}
}

func TestCheckpoint_InvalidNames(t *testing.T) {
	observer, cleanup := setupTestObserver(t)
	defer cleanup()

	invalidNames := []string{
		"",            // empty
		"foo/bar",     // slash
		"foo\\bar",    // backslash
		"foo bar",     // space
		"foo/bar/baz", // multiple slashes
	}

	for _, name := range invalidNames {
		r := observer.Checkpoint(name)
		if r.IsSuccess() {
			t.Errorf("Expected failure for invalid checkpoint name %q, got success", name)
		}
		if !r.IsFatal() {
			t.Errorf("Expected IsFatal() for invalid checkpoint name %q", name)
		}
	}
}

func TestCheckpoint_RestoreReturnsData(t *testing.T) {
	observer, cleanup := setupTestObserver(t)
	defer cleanup()

	writeTestIndex(t, string(observer.IndexPath), 3)
	writeTestCursor(t, string(observer.CursorPath))

	r := observer.Checkpoint("restore-data-test")
	if !r.IsSuccess() {
		t.Fatalf("Checkpoint failed: %v", r.Errors[0].Message)
	}

	rr := observer.RestoreCheckpoint("restore-data-test")
	if !rr.IsSuccess() {
		t.Fatalf("RestoreCheckpoint failed: %v", rr.Errors[0].Message)
	}

	rd := rr.GetData()
	if rd.Name != "restore-data-test" {
		t.Errorf("Expected Name='restore-data-test', got %q", rd.Name)
	}
	if rd.RestoredAt.IsZero() {
		t.Error("Expected non-zero RestoredAt")
	}
}
