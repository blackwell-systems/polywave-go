package journal

import (
	"bufio"
	"bytes"
	"crypto/md5"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"
)

// createTestObserver creates an observer configured for testing with a mock Claude project
func createTestObserver(t *testing.T, projectName string) (*testObserverWrapper, string) {
	tmpDir := t.TempDir()

	// Create mock Claude project directory — hash the full path (matching fixed getClaudeProjectDir)
	hash := md5.Sum([]byte(tmpDir))
	projectHash := fmt.Sprintf("%x", hash)[:16]
	claudeDir := filepath.Join(tmpDir, ".claude", "projects", projectHash)
	if err := os.MkdirAll(claudeDir, 0755); err != nil {
		t.Fatalf("Failed to create mock Claude project dir: %v", err)
	}

	// Create observer with tmpDir as project root
	obsRes := NewObserver(tmpDir, "agent-A")
	if !obsRes.IsSuccess() {
		t.Fatalf("NewObserver failed: %v", obsRes.Errors[0].Message)
	}
	obs := *obsRes.Data

	// Wrap in test wrapper to override getClaudeProjectDir
	wrapper := &testObserverWrapper{
		JournalObserver: obs,
		claudeDir:       claudeDir,
	}

	return wrapper, claudeDir
}

// testObserverWrapper wraps JournalObserver for testing
type testObserverWrapper struct {
	*JournalObserver
	claudeDir string
}

// Override getClaudeProjectDir for testing
func (w *testObserverWrapper) getClaudeProjectDir() string {
	return w.claudeDir
}

// Sync wraps the parent Sync but uses test claude dir
func (w *testObserverWrapper) Sync() result_Sync {
	// Find the latest session file in test claude dir
	sessionFile, err := w.findLatestSessionFileIn(w.claudeDir)
	if err != nil {
		return result_Sync{err: fmt.Errorf("failed to find session file: %w", err)}
	}

	// Load cursor
	cursor, err := w.loadCursor()
	if err != nil {
		return result_Sync{err: fmt.Errorf("failed to load cursor: %w", err)}
	}

	// If session file changed, reset cursor
	if cursor.SessionFile != sessionFile {
		cursor = &SessionCursor{
			SessionFile: sessionFile,
			Offset:      0,
		}
	}

	// Open session file from test claude dir
	f, err := os.Open(filepath.Join(w.claudeDir, sessionFile))
	if err != nil {
		return result_Sync{err: fmt.Errorf("failed to open session file: %w", err)}
	}
	defer f.Close()

	// Rest is same as parent
	syncResult, err := w.syncFromFile(f, cursor)
	return result_Sync{data: syncResult, err: err}
}

// result_Sync is a helper to allow testObserverWrapper.Sync to mirror the old
// (*SyncResult, error) interface that tests use.
type result_Sync struct {
	data *SyncResult
	err  error
}

func (w *testObserverWrapper) findLatestSessionFile() (string, error) {
	return w.findLatestSessionFileIn(w.claudeDir)
}

func (w *testObserverWrapper) findLatestSessionFileIn(dir string) (string, error) {
	pattern := filepath.Join(dir, "*.jsonl")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return "", fmt.Errorf("failed to glob session files: %w", err)
	}

	if len(matches) == 0 {
		return "", fmt.Errorf("no session files found in %s", dir)
	}

	// Sort by mtime (most recent first)
	type fileInfo struct {
		path  string
		mtime time.Time
	}
	files := make([]fileInfo, 0, len(matches))
	for _, path := range matches {
		stat, err := os.Stat(path)
		if err != nil {
			continue
		}
		files = append(files, fileInfo{path: path, mtime: stat.ModTime()})
	}

	sort.Slice(files, func(i, j int) bool {
		return files[i].mtime.After(files[j].mtime)
	})

	return filepath.Base(files[0].path), nil
}

// syncFromFile extracts the sync logic from Sync
func (w *testObserverWrapper) syncFromFile(f *os.File, cursor *SessionCursor) (*SyncResult, error) {
	// Seek to cursor position
	if _, err := f.Seek(cursor.Offset, 0); err != nil {
		return nil, fmt.Errorf("failed to seek to cursor: %w", err)
	}

	// Parse new entries
	entries := []ToolEntry{}
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 1024*1024), 10*1024*1024)
	bytesRead := int64(0)

	for scanner.Scan() {
		line := scanner.Bytes()
		bytesRead += int64(len(line)) + 1

		var logEntry map[string]interface{}
		if err := json.Unmarshal(line, &logEntry); err != nil {
			continue
		}

		extracted := w.JournalObserver.extractToolEntries(logEntry)
		entries = append(entries, extracted...)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("failed to read session file: %w", err)
	}

	// Update cursor
	cursor.Offset += bytesRead
	if r := w.saveCursor(cursor); r.IsFatal() {
		return nil, fmt.Errorf("failed to save cursor: %s", r.Errors[0].Message)
	}

	// Append to index
	if len(entries) > 0 {
		if r := w.appendToIndex(entries); r.IsFatal() {
			return nil, fmt.Errorf("failed to append to index: %s", r.Errors[0].Message)
		}

		// Update recent cache
		if r := w.updateRecent(entries); r.IsFatal() {
			return nil, fmt.Errorf("failed to update recent: %s", r.Errors[0].Message)
		}
	}

	// Count tool uses vs results
	syncRes := &SyncResult{
		NewBytes: bytesRead,
	}
	for _, e := range entries {
		if e.Kind == "tool_use" {
			syncRes.NewToolUses++
		} else if e.Kind == "tool_result" {
			syncRes.NewToolResults++
		}
	}

	return syncRes, nil
}

func TestNewObserver_CreatesDirectories(t *testing.T) {
	tmpDir := t.TempDir()

	obsRes := NewObserver(tmpDir, "agent-A")
	if !obsRes.IsSuccess() {
		t.Fatalf("NewObserver failed: %v", obsRes.Errors[0].Message)
	}
	obs := *obsRes.Data

	// Verify journal directory exists
	if _, err := os.Stat(obs.JournalDir); os.IsNotExist(err) {
		t.Errorf("JournalDir not created: %s", obs.JournalDir)
	}

	// Verify tool-results directory exists
	resultsDir := filepath.Join(obs.JournalDir, "tool-results")
	if _, err := os.Stat(resultsDir); os.IsNotExist(err) {
		t.Errorf("tool-results dir not created: %s", resultsDir)
	}

	// Verify expected paths are set
	expectedJournalDir := filepath.Join(tmpDir, ".polywave-state", "wave1", "agent-A")
	if obs.JournalDir != expectedJournalDir {
		t.Errorf("JournalDir = %s, want %s", obs.JournalDir, expectedJournalDir)
	}
}

func TestSync_FirstRun(t *testing.T) {
	obs, claudeDir := createTestObserver(t, "test-project")
	sessionFile := "test-session-001.jsonl"

	createMockSessionLog(t, claudeDir, sessionFile, []mockLogEntry{
		{
			Timestamp: time.Now().Add(-5 * time.Minute),
			ToolUse: &mockToolUse{
				ID:    "toolu_001",
				Name:  "Read",
				Input: map[string]interface{}{"file_path": "/tmp/test.txt"},
			},
		},
		{
			Timestamp: time.Now().Add(-4 * time.Minute),
			ToolResult: &mockToolResult{
				ToolUseID: "toolu_001",
				Content:   "File contents here",
			},
		},
	})

	syncResult := obs.Sync()
	if syncResult.err != nil {
		t.Fatalf("Sync failed: %v", syncResult.err)
	}
	result := syncResult.data

	if result.NewToolUses != 1 {
		t.Errorf("NewToolUses = %d, want 1", result.NewToolUses)
	}
	if result.NewToolResults != 1 {
		t.Errorf("NewToolResults = %d, want 1", result.NewToolResults)
	}

	// Verify index.jsonl was created
	if _, err := os.Stat(obs.IndexPath); os.IsNotExist(err) {
		t.Errorf("index.jsonl not created")
	}

	// Verify cursor was saved
	cursor, err := obs.loadCursor()
	if err != nil {
		t.Fatalf("loadCursor failed: %v", err)
	}
	if cursor.SessionFile != sessionFile {
		t.Errorf("cursor.SessionFile = %s, want %s", cursor.SessionFile, sessionFile)
	}
	if cursor.Offset == 0 {
		t.Errorf("cursor.Offset = 0, expected non-zero")
	}
}

func TestSync_Incremental(t *testing.T) {
	obs, claudeDir := createTestObserver(t, "test-project-2")
	sessionFile := "test-session-002.jsonl"

	// Create initial session log
	createMockSessionLog(t, claudeDir, sessionFile, []mockLogEntry{
		{
			Timestamp: time.Now().Add(-10 * time.Minute),
			ToolUse: &mockToolUse{
				ID:    "toolu_001",
				Name:  "Bash",
				Input: map[string]interface{}{"command": "ls"},
			},
		},
	})

	// First sync
	sync1 := obs.Sync()
	if sync1.err != nil {
		t.Fatalf("First sync failed: %v", sync1.err)
	}
	result1 := sync1.data
	if result1.NewToolUses != 1 {
		t.Errorf("First sync: NewToolUses = %d, want 1", result1.NewToolUses)
	}

	// Append more entries to session log
	appendMockSessionLog(t, claudeDir, sessionFile, []mockLogEntry{
		{
			Timestamp: time.Now().Add(-5 * time.Minute),
			ToolUse: &mockToolUse{
				ID:    "toolu_002",
				Name:  "Read",
				Input: map[string]interface{}{"file_path": "/tmp/other.txt"},
			},
		},
	})

	// Second sync should only process new entries
	sync2 := obs.Sync()
	if sync2.err != nil {
		t.Fatalf("Second sync failed: %v", sync2.err)
	}
	result2 := sync2.data
	if result2.NewToolUses != 1 {
		t.Errorf("Second sync: NewToolUses = %d, want 1", result2.NewToolUses)
	}

	// Verify cursor advanced
	cursor, err := obs.loadCursor()
	if err != nil {
		t.Fatalf("loadCursor failed: %v", err)
	}
	expectedOffset := result1.NewBytes + result2.NewBytes
	if cursor.Offset != expectedOffset {
		t.Errorf("cursor.Offset = %d, want %d", cursor.Offset, expectedOffset)
	}
}

func TestSync_SessionFileChanged(t *testing.T) {
	obs, claudeDir := createTestObserver(t, "test-project-3")

	// Create first session log
	sessionFile1 := "session-001.jsonl"
	createMockSessionLog(t, claudeDir, sessionFile1, []mockLogEntry{
		{
			Timestamp: time.Now().Add(-20 * time.Minute),
			ToolUse: &mockToolUse{
				ID:    "toolu_001",
				Name:  "Bash",
				Input: map[string]interface{}{"command": "echo old"},
			},
		},
	})

	// First sync
	if s := obs.Sync(); s.err != nil {
		t.Fatalf("First sync failed: %v", s.err)
	}

	// Create newer session log
	time.Sleep(10 * time.Millisecond) // Ensure different mtime
	sessionFile2 := "session-002.jsonl"
	createMockSessionLog(t, claudeDir, sessionFile2, []mockLogEntry{
		{
			Timestamp: time.Now().Add(-5 * time.Minute),
			ToolUse: &mockToolUse{
				ID:    "toolu_002",
				Name:  "Read",
				Input: map[string]interface{}{"file_path": "/tmp/new.txt"},
			},
		},
	})

	// Second sync should detect new session and reset cursor
	sync2 := obs.Sync()
	if sync2.err != nil {
		t.Fatalf("Second sync failed: %v", sync2.err)
	}
	result := sync2.data

	cursor, err := obs.loadCursor()
	if err != nil {
		t.Fatalf("loadCursor failed: %v", err)
	}

	if cursor.SessionFile != sessionFile2 {
		t.Errorf("cursor.SessionFile = %s, want %s", cursor.SessionFile, sessionFile2)
	}
	if result.NewToolUses != 1 {
		t.Errorf("NewToolUses = %d, want 1", result.NewToolUses)
	}
}

func TestFindLatestSessionFile_SelectsMostRecent(t *testing.T) {
	obs, claudeDir := createTestObserver(t, "test-project-4")

	// Create multiple session files with different mtimes
	files := []string{"session-001.jsonl", "session-002.jsonl", "session-003.jsonl"}
	for i, name := range files {
		path := filepath.Join(claudeDir, name)
		if err := os.WriteFile(path, []byte("{}"), 0644); err != nil {
			t.Fatalf("Failed to create session file: %v", err)
		}

		// Set mtime to make session-003.jsonl the newest
		mtime := time.Now().Add(time.Duration(i) * time.Hour)
		if err := os.Chtimes(path, mtime, mtime); err != nil {
			t.Fatalf("Failed to set mtime: %v", err)
		}

		time.Sleep(10 * time.Millisecond) // Ensure different mtimes
	}

	latestFile, err := obs.findLatestSessionFile()
	if err != nil {
		t.Fatalf("findLatestSessionFile failed: %v", err)
	}

	if latestFile != "session-003.jsonl" {
		t.Errorf("findLatestSessionFile = %s, want session-003.jsonl", latestFile)
	}
}

func TestAppendToIndex_PreservesExisting(t *testing.T) {
	tmpDir := t.TempDir()

	obsRes := NewObserver(tmpDir, "agent-A")
	if !obsRes.IsSuccess() {
		t.Fatalf("NewObserver failed: %v", obsRes.Errors[0].Message)
	}
	obs := *obsRes.Data

	// First append
	entries1 := []ToolEntry{
		{
			Timestamp: time.Now(),
			Kind:      "tool_use",
			ToolName:  "Bash",
			ToolUseID: "toolu_001",
		},
	}
	if r := obs.appendToIndex(entries1); !r.IsSuccess() {
		t.Fatalf("First appendToIndex failed: %v", r.Errors[0].Message)
	}

	// Second append
	entries2 := []ToolEntry{
		{
			Timestamp: time.Now(),
			Kind:      "tool_result",
			ToolUseID: "toolu_001",
			Preview:   "output",
		},
	}
	if r := obs.appendToIndex(entries2); !r.IsSuccess() {
		t.Fatalf("Second appendToIndex failed: %v", r.Errors[0].Message)
	}

	// Read back and verify both entries exist
	data, err := os.ReadFile(obs.IndexPath)
	if err != nil {
		t.Fatalf("Failed to read index: %v", err)
	}

	lines := 0
	decoder := json.NewDecoder(bytes.NewReader(data))
	for decoder.More() {
		var entry ToolEntry
		if err := decoder.Decode(&entry); err != nil {
			t.Fatalf("Failed to decode entry: %v", err)
		}
		lines++
	}

	if lines != 2 {
		t.Errorf("index.jsonl has %d lines, want 2", lines)
	}
}

func TestUpdateRecent_MaintainsLast30(t *testing.T) {
	tmpDir := t.TempDir()

	obsRes := NewObserver(tmpDir, "agent-A")
	if !obsRes.IsSuccess() {
		t.Fatalf("NewObserver failed: %v", obsRes.Errors[0].Message)
	}
	obs := *obsRes.Data

	// Add 40 entries in batches
	for i := 0; i < 40; i++ {
		entry := ToolEntry{
			Timestamp: time.Now().Add(time.Duration(i) * time.Second),
			Kind:      "tool_use",
			ToolName:  "Bash",
			ToolUseID: fmt.Sprintf("toolu_%03d", i),
		}
		if r := obs.updateRecent([]ToolEntry{entry}); !r.IsSuccess() {
			t.Fatalf("updateRecent failed at entry %d: %v", i, r.Errors[0].Message)
		}
	}

	// Load recent.json and verify it has exactly 30 entries
	data, err := os.ReadFile(obs.RecentPath)
	if err != nil {
		t.Fatalf("Failed to read recent.json: %v", err)
	}

	var entries []ToolEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		t.Fatalf("Failed to unmarshal recent.json: %v", err)
	}

	if len(entries) != 30 {
		t.Errorf("recent.json has %d entries, want 30", len(entries))
	}

	// Verify it's the last 30 entries (indices 10-39)
	if len(entries) > 0 && entries[0].ToolUseID != "toolu_010" {
		t.Errorf("First entry in recent.json has ID %s, want toolu_010", entries[0].ToolUseID)
	}
}

func TestSync_ExtractsToolBlocks(t *testing.T) {
	obs, claudeDir := createTestObserver(t, "test-project-5")
	sessionFile := "test-session-003.jsonl"

	createMockSessionLog(t, claudeDir, sessionFile, []mockLogEntry{
		{
			Timestamp: time.Now(),
			ToolUse: &mockToolUse{
				ID:   "toolu_abc123",
				Name: "Read",
				Input: map[string]interface{}{
					"file_path": "/path/to/file.go",
				},
			},
		},
		{
			Timestamp: time.Now(),
			ToolResult: &mockToolResult{
				ToolUseID: "toolu_abc123",
				Content:   "package main\n\nfunc main() { }",
			},
		},
	})

	syncResult := obs.Sync()
	if syncResult.err != nil {
		t.Fatalf("Sync failed: %v", syncResult.err)
	}
	result := syncResult.data

	if result.NewToolUses != 1 {
		t.Errorf("NewToolUses = %d, want 1", result.NewToolUses)
	}
	if result.NewToolResults != 1 {
		t.Errorf("NewToolResults = %d, want 1", result.NewToolResults)
	}

	// Read index and verify extracted fields
	data, err := os.ReadFile(obs.IndexPath)
	if err != nil {
		t.Fatalf("Failed to read index: %v", err)
	}

	var entries []ToolEntry
	decoder := json.NewDecoder(bytes.NewReader(data))
	for decoder.More() {
		var entry ToolEntry
		if err := decoder.Decode(&entry); err != nil {
			t.Fatalf("Failed to decode entry: %v", err)
		}
		entries = append(entries, entry)
	}

	if len(entries) != 2 {
		t.Fatalf("Expected 2 entries, got %d", len(entries))
	}

	// Verify tool_use entry
	toolUse := entries[0]
	if toolUse.Kind != "tool_use" {
		t.Errorf("First entry kind = %s, want tool_use", toolUse.Kind)
	}
	if toolUse.ToolName != "Read" {
		t.Errorf("ToolName = %s, want Read", toolUse.ToolName)
	}
	if toolUse.ToolUseID != "toolu_abc123" {
		t.Errorf("ToolUseID = %s, want toolu_abc123", toolUse.ToolUseID)
	}

	// Verify tool_result entry
	toolResult := entries[1]
	if toolResult.Kind != "tool_result" {
		t.Errorf("Second entry kind = %s, want tool_result", toolResult.Kind)
	}
	if toolResult.ToolUseID != "toolu_abc123" {
		t.Errorf("ToolUseID = %s, want toolu_abc123", toolResult.ToolUseID)
	}
}

func TestSync_SavesFullOutputToFiles(t *testing.T) {
	obs, claudeDir := createTestObserver(t, "test-project-6")
	sessionFile := "test-session-004.jsonl"

	longContent := strings.Repeat("x", 15000) // 15KB of content

	createMockSessionLog(t, claudeDir, sessionFile, []mockLogEntry{
		{
			Timestamp: time.Now(),
			ToolUse: &mockToolUse{
				ID:    "toolu_big",
				Name:  "Bash",
				Input: map[string]interface{}{"command": "cat largefile.txt"},
			},
		},
		{
			Timestamp: time.Now(),
			ToolResult: &mockToolResult{
				ToolUseID: "toolu_big",
				Content:   longContent,
			},
		},
	})

	if s := obs.Sync(); s.err != nil {
		t.Fatalf("Sync failed: %v", s.err)
	}

	// Verify full content was saved to file
	resultFile := filepath.Join(obs.ResultsDir, "toolu_big.txt")
	savedContent, err := os.ReadFile(resultFile)
	if err != nil {
		t.Fatalf("Failed to read result file: %v", err)
	}

	if len(savedContent) != len(longContent) {
		t.Errorf("Saved content length = %d, want %d", len(savedContent), len(longContent))
	}

	// Verify preview was truncated in index
	data, err := os.ReadFile(obs.IndexPath)
	if err != nil {
		t.Fatalf("Failed to read index: %v", err)
	}

	var entries []ToolEntry
	decoder := json.NewDecoder(bytes.NewReader(data))
	for decoder.More() {
		var entry ToolEntry
		if err := decoder.Decode(&entry); err != nil {
			t.Fatalf("Failed to decode entry: %v", err)
		}
		if entry.Kind == "tool_result" {
			entries = append(entries, entry)
		}
	}

	if len(entries) != 1 {
		t.Fatalf("Expected 1 tool_result entry, got %d", len(entries))
	}

	entry := entries[0]
	if len(entry.Preview) != 800 {
		t.Errorf("Preview length = %d, want 800", len(entry.Preview))
	}
	if !entry.Truncated {
		t.Errorf("Truncated = false, want true")
	}
	if entry.ContentFile != "tool-results/toolu_big.txt" {
		t.Errorf("ContentFile = %s, want tool-results/toolu_big.txt", entry.ContentFile)
	}
}

// Helper types and functions for creating mock Claude Code session logs

type mockLogEntry struct {
	Timestamp  time.Time
	ToolUse    *mockToolUse
	ToolResult *mockToolResult
}

type mockToolUse struct {
	ID    string
	Name  string
	Input map[string]interface{}
}

type mockToolResult struct {
	ToolUseID string
	Content   string
}

func createMockSessionLog(t *testing.T, claudeDir, sessionFile string, entries []mockLogEntry) {
	path := filepath.Join(claudeDir, sessionFile)
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("Failed to create session log: %v", err)
	}
	defer f.Close()

	writeMockEntries(t, f, entries)
}

func appendMockSessionLog(t *testing.T, claudeDir, sessionFile string, entries []mockLogEntry) {
	path := filepath.Join(claudeDir, sessionFile)
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		t.Fatalf("Failed to open session log: %v", err)
	}
	defer f.Close()

	writeMockEntries(t, f, entries)
}

func writeMockEntries(t *testing.T, f *os.File, entries []mockLogEntry) {
	encoder := json.NewEncoder(f)

	for _, entry := range entries {
		var contentBlocks []map[string]interface{}

		if entry.ToolUse != nil {
			contentBlocks = append(contentBlocks, map[string]interface{}{
				"type":  "tool_use",
				"id":    entry.ToolUse.ID,
				"name":  entry.ToolUse.Name,
				"input": entry.ToolUse.Input,
			})
		}

		if entry.ToolResult != nil {
			contentBlocks = append(contentBlocks, map[string]interface{}{
				"type":        "tool_result",
				"tool_use_id": entry.ToolResult.ToolUseID,
				"content":     entry.ToolResult.Content,
			})
		}

		logEntry := map[string]interface{}{
			"timestamp": entry.Timestamp.Format(time.RFC3339Nano),
			"type":      "message",
			"content": map[string]interface{}{
				"messages": []map[string]interface{}{
					{
						"role":    "assistant",
						"content": contentBlocks,
					},
				},
			},
		}

		if err := encoder.Encode(logEntry); err != nil {
			t.Fatalf("Failed to write mock log entry: %v", err)
		}
	}
}

// Tests for Agent B implementation (LoadJournal, GenerateContext)

func TestGenerateContext_CallsGenerateContextFunc(t *testing.T) {
	tmpDir := t.TempDir()

	obsRes := NewObserver(tmpDir, "agent-A")
	if !obsRes.IsSuccess() {
		t.Fatalf("NewObserver failed: %v", obsRes.Errors[0].Message)
	}
	obs := *obsRes.Data

	// Create index.jsonl with sample entries
	entries := []ToolEntry{
		{
			Timestamp: time.Now().Add(-10 * time.Minute),
			Kind:      "tool_use",
			ToolName:  "Bash",
			ToolUseID: "toolu_001",
			Input:     map[string]interface{}{"command": "go test ./..."},
		},
		{
			Timestamp: time.Now().Add(-9 * time.Minute),
			Kind:      "tool_result",
			ToolUseID: "toolu_001",
			Preview:   "PASS",
		},
	}
	if r := obs.appendToIndex(entries); !r.IsSuccess() {
		t.Fatalf("Failed to create index: %v", r.Errors[0].Message)
	}

	// Call GenerateContext
	ctxRes := obs.GenerateContext()
	if !ctxRes.IsSuccess() {
		t.Fatalf("GenerateContext failed: %v", ctxRes.Errors[0].Message)
	}
	context := *ctxRes.Data

	// Verify output has expected structure (from journal.GenerateContext)
	if !strings.Contains(context, "## Session Context (Recovered from Tool Journal)") {
		t.Errorf("Context missing expected header")
	}
	if !strings.Contains(context, "Total tool calls") {
		t.Errorf("Context missing session stats")
	}
}

func TestLoadJournal_EmptyJournal(t *testing.T) {
	tmpDir := t.TempDir()

	obsRes := NewObserver(tmpDir, "agent-A")
	if !obsRes.IsSuccess() {
		t.Fatalf("NewObserver failed: %v", obsRes.Errors[0].Message)
	}
	obs := *obsRes.Data

	// No index.jsonl exists yet
	res := obs.LoadJournal()
	if !res.IsSuccess() {
		t.Fatalf("LoadJournal failed on non-existent file: %v", res.Errors[0].Message)
	}

	// Should return empty slice, not error
	if len(*res.Data) != 0 {
		t.Errorf("LoadJournal returned %d entries, want 0", len(*res.Data))
	}
}

func TestLoadJournal_ParsesJSONL(t *testing.T) {
	tmpDir := t.TempDir()

	obsRes := NewObserver(tmpDir, "agent-A")
	if !obsRes.IsSuccess() {
		t.Fatalf("NewObserver failed: %v", obsRes.Errors[0].Message)
	}
	obs := *obsRes.Data

	// Create index.jsonl manually
	testEntries := []ToolEntry{
		{
			Timestamp: time.Now().Add(-30 * time.Minute),
			Kind:      "tool_use",
			ToolName:  "Read",
			ToolUseID: "toolu_001",
			Input:     map[string]interface{}{"file_path": "/tmp/test.go"},
		},
		{
			Timestamp: time.Now().Add(-29 * time.Minute),
			Kind:      "tool_result",
			ToolUseID: "toolu_001",
			Preview:   "package main",
		},
		{
			Timestamp: time.Now().Add(-25 * time.Minute),
			Kind:      "tool_use",
			ToolName:  "Edit",
			ToolUseID: "toolu_002",
			Input:     map[string]interface{}{"file_path": "/tmp/test.go"},
		},
	}

	// Write entries as JSONL
	f, err := os.Create(obs.IndexPath)
	if err != nil {
		t.Fatalf("Failed to create index: %v", err)
	}
	encoder := json.NewEncoder(f)
	for _, entry := range testEntries {
		if err := encoder.Encode(entry); err != nil {
			t.Fatalf("Failed to write entry: %v", err)
		}
	}
	f.Close()

	// Load journal
	res := obs.LoadJournal()
	if !res.IsSuccess() {
		t.Fatalf("LoadJournal failed: %v", res.Errors[0].Message)
	}
	loaded := *res.Data

	// Verify all entries loaded
	if len(loaded) != len(testEntries) {
		t.Errorf("LoadJournal returned %d entries, want %d", len(loaded), len(testEntries))
	}

	// Verify entries loaded in correct order
	for i, entry := range loaded {
		if entry.ToolUseID != testEntries[i].ToolUseID {
			t.Errorf("Entry %d: ToolUseID = %s, want %s", i, entry.ToolUseID, testEntries[i].ToolUseID)
		}
		if entry.Kind != testEntries[i].Kind {
			t.Errorf("Entry %d: Kind = %s, want %s", i, entry.Kind, testEntries[i].Kind)
		}
	}
}

func TestLoadJournal_IgnoresInvalidLines(t *testing.T) {
	tmpDir := t.TempDir()

	obsRes := NewObserver(tmpDir, "agent-A")
	if !obsRes.IsSuccess() {
		t.Fatalf("NewObserver failed: %v", obsRes.Errors[0].Message)
	}
	obs := *obsRes.Data

	// Create index.jsonl with valid and invalid lines
	validEntry := ToolEntry{
		Timestamp: time.Now(),
		Kind:      "tool_use",
		ToolName:  "Bash",
		ToolUseID: "toolu_valid",
	}

	// Write mixed content
	var content bytes.Buffer
	encoder := json.NewEncoder(&content)
	encoder.Encode(validEntry)                 // Valid
	content.WriteString("this is not json\n")  // Invalid
	content.WriteString("\n")                  // Empty line
	content.WriteString("{\"incomplete\": \n") // Invalid
	encoder.Encode(validEntry)                 // Valid
	content.WriteString("   \n")               // Whitespace-only line

	if err := os.WriteFile(obs.IndexPath, content.Bytes(), 0644); err != nil {
		t.Fatalf("Failed to write index: %v", err)
	}

	// LoadJournal should skip invalid lines and return valid ones
	res := obs.LoadJournal()
	if !res.IsSuccess() {
		t.Fatalf("LoadJournal failed: %v", res.Errors[0].Message)
	}
	entries := *res.Data

	// Should have loaded 2 valid entries
	if len(entries) != 2 {
		t.Errorf("LoadJournal returned %d entries, want 2", len(entries))
	}

	// Both should be the valid entry
	for i, entry := range entries {
		if entry.ToolUseID != "toolu_valid" {
			t.Errorf("Entry %d: ToolUseID = %s, want toolu_valid", i, entry.ToolUseID)
		}
	}
}

func TestGenerateContext_Integration(t *testing.T) {
	obs, claudeDir := createTestObserver(t, "test-project-context")
	sessionFile := "test-session-context.jsonl"

	// Create session log with realistic entries
	createMockSessionLog(t, claudeDir, sessionFile, []mockLogEntry{
		{
			Timestamp: time.Now().Add(-15 * time.Minute),
			ToolUse: &mockToolUse{
				ID:    "toolu_001",
				Name:  "Edit",
				Input: map[string]interface{}{
					"file_path":  "pkg/journal/observer.go",
					"old_string": "not implemented",
					"new_string": "actual implementation",
				},
			},
		},
		{
			Timestamp: time.Now().Add(-14 * time.Minute),
			ToolResult: &mockToolResult{
				ToolUseID: "toolu_001",
				Content:   "File updated successfully",
			},
		},
		{
			Timestamp: time.Now().Add(-10 * time.Minute),
			ToolUse: &mockToolUse{
				ID:    "toolu_002",
				Name:  "Bash",
				Input: map[string]interface{}{"command": "go test ./pkg/journal/..."},
			},
		},
		{
			Timestamp: time.Now().Add(-9 * time.Minute),
			ToolResult: &mockToolResult{
				ToolUseID: "toolu_002",
				Content:   "PASS\nok   pkg/journal 0.123s",
			},
		},
		{
			Timestamp: time.Now().Add(-5 * time.Minute),
			ToolUse: &mockToolUse{
				ID:    "toolu_003",
				Name:  "Bash",
				Input: map[string]interface{}{"command": "git -C /worktree/path commit -m \"Implement LoadJournal and GenerateContext\""},
			},
		},
		{
			Timestamp: time.Now().Add(-4 * time.Minute),
			ToolResult: &mockToolResult{
				ToolUseID: "toolu_003",
				Content:   "[wave1-agent-B abc1234] Implement LoadJournal and GenerateContext\n 2 files changed, 50 insertions(+), 2 deletions(-)",
			},
		},
	})

	// Sync to load entries into journal
	syncResult := obs.Sync()
	if syncResult.err != nil {
		t.Fatalf("Sync failed: %v", syncResult.err)
	}
	result := syncResult.data
	if result.NewToolUses != 3 {
		t.Errorf("Sync found %d tool uses, want 3", result.NewToolUses)
	}

	// Now call GenerateContext (end-to-end)
	ctxRes := obs.JournalObserver.GenerateContext()
	if !ctxRes.IsSuccess() {
		t.Fatalf("GenerateContext failed: %v", ctxRes.Errors[0].Message)
	}
	context := *ctxRes.Data

	// Verify context has expected sections
	expectedSections := []string{
		"## Session Context (Recovered from Tool Journal)",
		"Total tool calls",
		"### Files Modified",
		"pkg/journal/observer.go",
		"### Tests Run",
		"go test ./pkg/journal/...",
		"### Git Commits",
		"abc1234",
		"Implement LoadJournal and GenerateContext",
	}

	for _, section := range expectedSections {
		if !strings.Contains(context, section) {
			t.Errorf("Context missing expected section: %s", section)
		}
	}

	// Verify context is non-trivial (not just empty state message)
	if strings.Contains(context, "No tool activity recorded yet") {
		t.Errorf("Context should not indicate empty activity")
	}
}

// TestNewObserver_WavePrefix verifies that wave prefix stripping works correctly
// and that RawAgentID preserves the original value.
func TestNewObserver_WavePrefix(t *testing.T) {
	tmpDir := t.TempDir()

	res := NewObserver(tmpDir, "wave3-agent-C")
	if !res.IsSuccess() {
		t.Fatalf("NewObserver failed: %v", res.Errors[0].Message)
	}
	obs := *res.Data

	if obs.AgentID != "agent-C" {
		t.Errorf("AgentID = %q, want %q", obs.AgentID, "agent-C")
	}
	if obs.RawAgentID != "wave3-agent-C" {
		t.Errorf("RawAgentID = %q, want %q", obs.RawAgentID, "wave3-agent-C")
	}
	if !strings.Contains(obs.JournalDir, filepath.Join("wave3", "agent-C")) {
		t.Errorf("JournalDir = %q, expected to contain wave3/agent-C", obs.JournalDir)
	}
}

// TestExtractToolResult_ArrayContent verifies that content sent as []interface{}
// (array of text blocks) is correctly joined into the Preview field.
func TestExtractToolResult_ArrayContent(t *testing.T) {
	tmpDir := t.TempDir()

	obsRes := NewObserver(tmpDir, "agent-A")
	if !obsRes.IsSuccess() {
		t.Fatalf("NewObserver failed: %v", obsRes.Errors[0].Message)
	}
	obs := *obsRes.Data

	block := map[string]interface{}{
		"type":        "tool_result",
		"tool_use_id": "toolu_array_001",
		"content": []interface{}{
			map[string]interface{}{"type": "text", "text": "first part"},
			map[string]interface{}{"type": "text", "text": "second part"},
		},
	}

	timestamp := time.Now()
	entry := obs.extractToolResult(block, timestamp)

	if entry == nil {
		t.Fatal("extractToolResult returned nil, want non-nil entry")
	}
	if !strings.Contains(entry.Preview, "first part") {
		t.Errorf("Preview = %q, expected to contain 'first part'", entry.Preview)
	}
	if !strings.Contains(entry.Preview, "second part") {
		t.Errorf("Preview = %q, expected to contain 'second part'", entry.Preview)
	}
}

// TestGetClaudeProjectDir_FullPath verifies that observers with the same base name
// but different full paths produce different Claude project directories.
func TestGetClaudeProjectDir_FullPath(t *testing.T) {
	// Two paths with the same base name but different parent directories
	path1 := filepath.Join(t.TempDir(), "myproject")
	path2 := filepath.Join(t.TempDir(), "myproject")

	// Ensure the directories exist
	if err := os.MkdirAll(path1, 0755); err != nil {
		t.Fatalf("Failed to create path1: %v", err)
	}
	if err := os.MkdirAll(path2, 0755); err != nil {
		t.Fatalf("Failed to create path2: %v", err)
	}

	res1 := NewObserver(path1, "agent-A")
	if !res1.IsSuccess() {
		t.Fatalf("NewObserver(path1) failed: %v", res1.Errors[0].Message)
	}
	res2 := NewObserver(path2, "agent-A")
	if !res2.IsSuccess() {
		t.Fatalf("NewObserver(path2) failed: %v", res2.Errors[0].Message)
	}

	dir1 := (*res1.Data).getClaudeProjectDir()
	dir2 := (*res2.Data).getClaudeProjectDir()

	if dir1 == dir2 {
		t.Errorf("getClaudeProjectDir returned the same path for different project roots: %s", dir1)
	}
}

// TestNewObserver_ResultType verifies the result-based API contract for NewObserver.
func TestNewObserver_ResultType(t *testing.T) {
	tmpDir := t.TempDir()

	res := NewObserver(tmpDir, "agent-A")

	if !res.IsSuccess() {
		t.Fatalf("NewObserver expected success, got failure: %v", res.Errors)
	}
	if res.Data == nil {
		t.Fatal("NewObserver result.Data is nil, want non-nil *JournalObserver")
	}
	if (*res.Data).JournalDir == "" {
		t.Error("NewObserver result.Data.JournalDir is empty")
	}
}
