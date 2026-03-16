package protocol

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadMergeLog_NoFile(t *testing.T) {
	// Create temp directory for test
	tmpDir := t.TempDir()
	manifestPath := filepath.Join(tmpDir, "IMPL-test.yaml")

	// Load merge log when file doesn't exist
	log, err := LoadMergeLog(manifestPath, 1)
	if err != nil {
		t.Fatalf("expected no error when file doesn't exist, got: %v", err)
	}

	if log == nil {
		t.Fatal("expected non-nil log")
	}

	if log.Wave != 1 {
		t.Errorf("expected wave=1, got: %d", log.Wave)
	}

	if len(log.Merges) != 0 {
		t.Errorf("expected empty merges array, got: %d entries", len(log.Merges))
	}
}

func TestLoadMergeLog_ValidFile(t *testing.T) {
	// Create temp directory and merge log file
	tmpDir := t.TempDir()
	manifestPath := filepath.Join(tmpDir, "IMPL-test.yaml")
	logDir := filepath.Join(tmpDir, ".saw-state", "wave1")
	logPath := filepath.Join(logDir, "merge-log.json")

	if err := os.MkdirAll(logDir, 0755); err != nil {
		t.Fatalf("failed to create log directory: %v", err)
	}

	// Write test data
	testLog := MergeLog{
		Wave: 1,
		Merges: []MergeEntry{
			{
				Agent:     "A",
				MergeSHA:  "abc123",
				Timestamp: time.Date(2024, 3, 14, 10, 30, 0, 0, time.UTC),
			},
			{
				Agent:     "B",
				MergeSHA:  "def456",
				Timestamp: time.Date(2024, 3, 14, 10, 35, 0, 0, time.UTC),
			},
		},
	}

	data, err := json.Marshal(testLog)
	if err != nil {
		t.Fatalf("failed to marshal test data: %v", err)
	}

	if err := os.WriteFile(logPath, data, 0644); err != nil {
		t.Fatalf("failed to write test log: %v", err)
	}

	// Load merge log
	log, err := LoadMergeLog(manifestPath, 1)
	if err != nil {
		t.Fatalf("failed to load merge log: %v", err)
	}

	if log.Wave != 1 {
		t.Errorf("expected wave=1, got: %d", log.Wave)
	}

	if len(log.Merges) != 2 {
		t.Fatalf("expected 2 merge entries, got: %d", len(log.Merges))
	}

	if log.Merges[0].Agent != "A" {
		t.Errorf("expected agent A, got: %s", log.Merges[0].Agent)
	}

	if log.Merges[0].MergeSHA != "abc123" {
		t.Errorf("expected SHA abc123, got: %s", log.Merges[0].MergeSHA)
	}

	if log.Merges[1].Agent != "B" {
		t.Errorf("expected agent B, got: %s", log.Merges[1].Agent)
	}

	if log.Merges[1].MergeSHA != "def456" {
		t.Errorf("expected SHA def456, got: %s", log.Merges[1].MergeSHA)
	}
}

func TestSaveMergeLog_CreatesDirectory(t *testing.T) {
	// Create temp directory (without .saw-state subdirectory)
	tmpDir := t.TempDir()
	manifestPath := filepath.Join(tmpDir, "IMPL-test.yaml")

	log := &MergeLog{
		Wave:   1,
		Merges: []MergeEntry{},
	}

	// Save log - should create directory
	if err := SaveMergeLog(manifestPath, 1, log); err != nil {
		t.Fatalf("failed to save merge log: %v", err)
	}

	// Verify directory was created
	logDir := filepath.Join(tmpDir, ".saw-state", "wave1")
	if _, err := os.Stat(logDir); os.IsNotExist(err) {
		t.Errorf("expected directory to be created: %s", logDir)
	}

	// Verify file was created
	logPath := filepath.Join(logDir, "merge-log.json")
	if _, err := os.Stat(logPath); os.IsNotExist(err) {
		t.Errorf("expected file to be created: %s", logPath)
	}
}

func TestSaveMergeLog_WritesJSON(t *testing.T) {
	// Create temp directory
	tmpDir := t.TempDir()
	manifestPath := filepath.Join(tmpDir, "IMPL-test.yaml")

	log := &MergeLog{
		Wave: 1,
		Merges: []MergeEntry{
			{
				Agent:     "A",
				MergeSHA:  "abc123",
				Timestamp: time.Date(2024, 3, 14, 10, 30, 0, 0, time.UTC),
			},
		},
	}

	// Save log
	if err := SaveMergeLog(manifestPath, 1, log); err != nil {
		t.Fatalf("failed to save merge log: %v", err)
	}

	// Read file and verify content
	logPath := filepath.Join(tmpDir, ".saw-state", "wave1", "merge-log.json")
	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("failed to read merge log: %v", err)
	}

	var loaded MergeLog
	if err := json.Unmarshal(data, &loaded); err != nil {
		t.Fatalf("failed to parse merge log: %v", err)
	}

	if loaded.Wave != 1 {
		t.Errorf("expected wave=1, got: %d", loaded.Wave)
	}

	if len(loaded.Merges) != 1 {
		t.Fatalf("expected 1 merge entry, got: %d", len(loaded.Merges))
	}

	if loaded.Merges[0].Agent != "A" {
		t.Errorf("expected agent A, got: %s", loaded.Merges[0].Agent)
	}

	if loaded.Merges[0].MergeSHA != "abc123" {
		t.Errorf("expected SHA abc123, got: %s", loaded.Merges[0].MergeSHA)
	}
}

func TestAddMergeEntry_AppendsEntry(t *testing.T) {
	log := &MergeLog{
		Wave:   1,
		Merges: []MergeEntry{},
	}

	// Add first entry
	if err := log.AddMergeEntry("A", "abc123"); err != nil {
		t.Fatalf("failed to add first entry: %v", err)
	}

	if len(log.Merges) != 1 {
		t.Fatalf("expected 1 entry after first add, got: %d", len(log.Merges))
	}

	if log.Merges[0].Agent != "A" {
		t.Errorf("expected agent A, got: %s", log.Merges[0].Agent)
	}

	if log.Merges[0].MergeSHA != "abc123" {
		t.Errorf("expected SHA abc123, got: %s", log.Merges[0].MergeSHA)
	}

	// Add second entry
	if err := log.AddMergeEntry("B", "def456"); err != nil {
		t.Fatalf("failed to add second entry: %v", err)
	}

	if len(log.Merges) != 2 {
		t.Fatalf("expected 2 entries after second add, got: %d", len(log.Merges))
	}

	if log.Merges[1].Agent != "B" {
		t.Errorf("expected agent B, got: %s", log.Merges[1].Agent)
	}

	if log.Merges[1].MergeSHA != "def456" {
		t.Errorf("expected SHA def456, got: %s", log.Merges[1].MergeSHA)
	}

	// Verify timestamps are set
	if log.Merges[0].Timestamp.IsZero() {
		t.Error("expected non-zero timestamp for first entry")
	}

	if log.Merges[1].Timestamp.IsZero() {
		t.Error("expected non-zero timestamp for second entry")
	}
}

func TestAddMergeEntry_ValidatesInput(t *testing.T) {
	log := &MergeLog{
		Wave:   1,
		Merges: []MergeEntry{},
	}

	// Test empty agent ID
	if err := log.AddMergeEntry("", "abc123"); err == nil {
		t.Error("expected error for empty agent ID")
	}

	// Test empty merge SHA
	if err := log.AddMergeEntry("A", ""); err == nil {
		t.Error("expected error for empty merge SHA")
	}

	// Verify no entries were added
	if len(log.Merges) != 0 {
		t.Errorf("expected 0 entries after validation failures, got: %d", len(log.Merges))
	}
}

func TestIsMerged_ReturnsTrueForMergedAgent(t *testing.T) {
	log := &MergeLog{
		Wave: 1,
		Merges: []MergeEntry{
			{Agent: "A", MergeSHA: "abc123", Timestamp: time.Now()},
			{Agent: "B", MergeSHA: "def456", Timestamp: time.Now()},
		},
	}

	if !log.IsMerged("A") {
		t.Error("expected IsMerged(A) to return true")
	}

	if !log.IsMerged("B") {
		t.Error("expected IsMerged(B) to return true")
	}

	// Test case-insensitive comparison
	if !log.IsMerged("a") {
		t.Error("expected IsMerged(a) to return true (case-insensitive)")
	}

	if !log.IsMerged("b") {
		t.Error("expected IsMerged(b) to return true (case-insensitive)")
	}
}

func TestIsMerged_ReturnsFalseForUnmergedAgent(t *testing.T) {
	log := &MergeLog{
		Wave: 1,
		Merges: []MergeEntry{
			{Agent: "A", MergeSHA: "abc123", Timestamp: time.Now()},
		},
	}

	if log.IsMerged("B") {
		t.Error("expected IsMerged(B) to return false")
	}

	if log.IsMerged("C") {
		t.Error("expected IsMerged(C) to return false")
	}

	// Test case-insensitive comparison
	if log.IsMerged("b") {
		t.Error("expected IsMerged(b) to return false (case-insensitive)")
	}
}

func TestGetMergeSHA_ReturnsCorrectSHA(t *testing.T) {
	log := &MergeLog{
		Wave: 1,
		Merges: []MergeEntry{
			{Agent: "A", MergeSHA: "abc123", Timestamp: time.Now()},
			{Agent: "B", MergeSHA: "def456", Timestamp: time.Now()},
		},
	}

	if sha := log.GetMergeSHA("A"); sha != "abc123" {
		t.Errorf("expected SHA abc123 for agent A, got: %s", sha)
	}

	if sha := log.GetMergeSHA("B"); sha != "def456" {
		t.Errorf("expected SHA def456 for agent B, got: %s", sha)
	}

	// Test case-insensitive comparison
	if sha := log.GetMergeSHA("a"); sha != "abc123" {
		t.Errorf("expected SHA abc123 for agent a (case-insensitive), got: %s", sha)
	}

	if sha := log.GetMergeSHA("b"); sha != "def456" {
		t.Errorf("expected SHA def456 for agent b (case-insensitive), got: %s", sha)
	}

	// Test unmerged agent
	if sha := log.GetMergeSHA("C"); sha != "" {
		t.Errorf("expected empty string for unmerged agent C, got: %s", sha)
	}
}
