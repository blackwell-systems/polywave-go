package main

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/journal"
)

// setupTestJournal creates a temporary journal directory with test data
func setupTestJournal(t *testing.T) (string, string) {
	t.Helper()

	tmpDir := t.TempDir()
	journalDir := filepath.Join(tmpDir, ".saw-state", "wave1", "agent-A")
	if err := os.MkdirAll(journalDir, 0755); err != nil {
		t.Fatalf("failed to create journal dir: %v", err)
	}

	indexPath := filepath.Join(journalDir, "index.jsonl")
	return tmpDir, indexPath
}

// writeTestEntries writes ToolEntry structs to the journal index
func writeTestEntries(t *testing.T, indexPath string, entries []journal.ToolEntry) {
	t.Helper()

	f, err := os.Create(indexPath)
	if err != nil {
		t.Fatalf("failed to create index: %v", err)
	}
	defer f.Close()

	encoder := json.NewEncoder(f)
	for _, entry := range entries {
		if err := encoder.Encode(entry); err != nil {
			t.Fatalf("failed to encode entry: %v", err)
		}
	}
}

func TestDebugJournal_DumpRaw(t *testing.T) {
	tmpDir, indexPath := setupTestJournal(t)

	entries := []journal.ToolEntry{
		{
			Timestamp: time.Now(),
			Kind:      "tool_use",
			ToolName:  "Read",
			ToolUseID: "test-1",
			Input: map[string]interface{}{
				"file_path": "test.go",
			},
		},
		{
			Timestamp: time.Now(),
			Kind:      "tool_result",
			ToolUseID: "test-1",
			Preview:   "file contents here",
		},
	}
	writeTestEntries(t, indexPath, entries)

	// Redirect stdout to capture output
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	opts := DebugOpts{}
	err := debugJournalCommand(tmpDir, "wave1/agent-A", opts)

	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("debugJournalCommand failed: %v", err)
	}

	// Read captured output
	var buf strings.Builder
	io.Copy(&buf, r)
	output := buf.String()

	// Verify JSONL format
	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) != 2 {
		t.Errorf("expected 2 JSONL lines, got %d", len(lines))
	}

	// Verify first line is valid JSON
	var entry journal.ToolEntry
	if err := json.Unmarshal([]byte(lines[0]), &entry); err != nil {
		t.Errorf("invalid JSONL output: %v", err)
	}

	if entry.ToolName != "Read" {
		t.Errorf("expected tool_name=Read, got %s", entry.ToolName)
	}
}

func TestDebugJournal_Summary(t *testing.T) {
	tmpDir, indexPath := setupTestJournal(t)

	baseTime := time.Now().Add(-1 * time.Hour)
	entries := []journal.ToolEntry{
		{
			Timestamp: baseTime,
			Kind:      "tool_use",
			ToolName:  "Write",
			ToolUseID: "test-1",
			Input: map[string]interface{}{
				"file_path": "pkg/api/routes.go",
				"content":   "package api\n\nfunc Routes() {}\n",
			},
		},
		{
			Timestamp: baseTime.Add(1 * time.Minute),
			Kind:      "tool_result",
			ToolUseID: "test-1",
			Preview:   "File written successfully",
		},
		{
			Timestamp: baseTime.Add(2 * time.Minute),
			Kind:      "tool_use",
			ToolName:  "Bash",
			ToolUseID: "test-2",
			Input: map[string]interface{}{
				"command": "go build ./pkg/api",
			},
		},
		{
			Timestamp: baseTime.Add(3 * time.Minute),
			Kind:      "tool_result",
			ToolUseID: "test-2",
			Preview:   "Build completed successfully",
		},
	}
	writeTestEntries(t, indexPath, entries)

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	opts := DebugOpts{Summary: true}
	err := debugJournalCommand(tmpDir, "wave1/agent-A", opts)

	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("debugJournalCommand failed: %v", err)
	}

	var buf strings.Builder
	io.Copy(&buf, r)
	output := buf.String()

	// Verify summary contains expected sections
	if !strings.Contains(output, "Journal: wave1/agent-A") {
		t.Error("summary missing journal header")
	}
	if !strings.Contains(output, "Duration:") {
		t.Error("summary missing duration")
	}
	if !strings.Contains(output, "Total tool calls:") {
		t.Error("summary missing tool call count")
	}
	if !strings.Contains(output, "Files modified:") {
		t.Error("summary missing files modified section")
	}
	if !strings.Contains(output, "pkg/api/routes.go") {
		t.Error("summary missing modified file")
	}
}

func TestDebugJournal_FailuresOnly(t *testing.T) {
	tmpDir, indexPath := setupTestJournal(t)

	baseTime := time.Now()
	entries := []journal.ToolEntry{
		{
			Timestamp: baseTime,
			Kind:      "tool_use",
			ToolName:  "Bash",
			ToolUseID: "test-1",
			Input: map[string]interface{}{
				"command": "go test ./pkg/api",
			},
		},
		{
			Timestamp: baseTime.Add(1 * time.Second),
			Kind:      "tool_result",
			ToolUseID: "test-1",
			Preview:   "FAIL: TestRoutes\nFAIL\texit status 1",
		},
		{
			Timestamp: baseTime.Add(10 * time.Second),
			Kind:      "tool_use",
			ToolName:  "Bash",
			ToolUseID: "test-2",
			Input: map[string]interface{}{
				"command": "go build ./pkg/api",
			},
		},
		{
			Timestamp: baseTime.Add(11 * time.Second),
			Kind:      "tool_result",
			ToolUseID: "test-2",
			Preview:   "Build successful",
		},
	}
	writeTestEntries(t, indexPath, entries)

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	opts := DebugOpts{FailuresOnly: true}
	err := debugJournalCommand(tmpDir, "wave1/agent-A", opts)

	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("debugJournalCommand failed: %v", err)
	}

	var buf strings.Builder
	io.Copy(&buf, r)
	output := buf.String()

	// Should contain failed test but not successful build
	if !strings.Contains(output, "FAIL") {
		t.Error("failures-only missing failed test")
	}

	// Count JSONL lines - should be 2 (tool_use + tool_result for failed test)
	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) < 1 {
		t.Errorf("expected at least 1 JSONL line for failures, got %d", len(lines))
	}
}

func TestDebugJournal_Last(t *testing.T) {
	tmpDir, indexPath := setupTestJournal(t)

	baseTime := time.Now()
	entries := make([]journal.ToolEntry, 50)
	for i := 0; i < 50; i++ {
		entries[i] = journal.ToolEntry{
			Timestamp: baseTime.Add(time.Duration(i) * time.Second),
			Kind:      "tool_use",
			ToolName:  "Read",
			ToolUseID: "test-" + string(rune(i)),
			Input:     map[string]interface{}{"file_path": "test.go"},
		}
	}
	writeTestEntries(t, indexPath, entries)

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	opts := DebugOpts{Last: 10}
	err := debugJournalCommand(tmpDir, "wave1/agent-A", opts)

	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("debugJournalCommand failed: %v", err)
	}

	var buf strings.Builder
	io.Copy(&buf, r)
	output := buf.String()

	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) != 10 {
		t.Errorf("expected 10 entries with --last 10, got %d", len(lines))
	}
}

func TestDebugJournal_ExportHTML(t *testing.T) {
	tmpDir, indexPath := setupTestJournal(t)

	baseTime := time.Now()
	entries := []journal.ToolEntry{
		{
			Timestamp: baseTime,
			Kind:      "tool_use",
			ToolName:  "Read",
			ToolUseID: "test-1",
			Input: map[string]interface{}{
				"file_path": "test.go",
			},
		},
		{
			Timestamp: baseTime.Add(1 * time.Second),
			Kind:      "tool_result",
			ToolUseID: "test-1",
			Preview:   "file contents",
		},
	}
	writeTestEntries(t, indexPath, entries)

	exportPath := filepath.Join(tmpDir, "timeline.html")

	opts := DebugOpts{Export: exportPath}
	err := debugJournalCommand(tmpDir, "wave1/agent-A", opts)

	if err != nil {
		t.Fatalf("debugJournalCommand failed: %v", err)
	}

	// Verify HTML file was created
	if _, err := os.Stat(exportPath); os.IsNotExist(err) {
		t.Error("HTML file was not created")
	}

	// Verify HTML content
	content, err := os.ReadFile(exportPath)
	if err != nil {
		t.Fatalf("failed to read HTML file: %v", err)
	}

	html := string(content)
	if !strings.Contains(html, "<!DOCTYPE html>") {
		t.Error("HTML missing DOCTYPE")
	}
	if !strings.Contains(html, "Journal Timeline") {
		t.Error("HTML missing title")
	}
	if !strings.Contains(html, "timeline") {
		t.Error("HTML missing timeline div")
	}
}

func TestDebugJournal_NonexistentAgent(t *testing.T) {
	tmpDir := t.TempDir()

	opts := DebugOpts{}
	err := debugJournalCommand(tmpDir, "wave1/nonexistent", opts)

	if err == nil {
		t.Error("expected error for nonexistent agent, got nil")
	}

	if !strings.Contains(err.Error(), "journal not found") {
		t.Errorf("expected 'journal not found' error, got: %v", err)
	}
}

func TestDebugJournal_EmptyJournal(t *testing.T) {
	tmpDir, indexPath := setupTestJournal(t)

	// Create empty journal file
	if err := os.WriteFile(indexPath, []byte{}, 0644); err != nil {
		t.Fatalf("failed to create empty journal: %v", err)
	}

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	opts := DebugOpts{}
	err := debugJournalCommand(tmpDir, "wave1/agent-A", opts)

	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("debugJournalCommand failed: %v", err)
	}

	var buf strings.Builder
	io.Copy(&buf, r)
	output := buf.String()

	if !strings.Contains(output, "no entries") {
		t.Error("expected 'no entries' message for empty journal")
	}
}

func TestDebugJournal_ExportWithoutForce(t *testing.T) {
	tmpDir, indexPath := setupTestJournal(t)

	entries := []journal.ToolEntry{
		{
			Timestamp: time.Now(),
			Kind:      "tool_use",
			ToolName:  "Read",
			ToolUseID: "test-1",
			Input:     map[string]interface{}{"file_path": "test.go"},
		},
	}
	writeTestEntries(t, indexPath, entries)

	exportPath := filepath.Join(tmpDir, "timeline.html")

	// Create the file first
	if err := os.WriteFile(exportPath, []byte("existing"), 0644); err != nil {
		t.Fatalf("failed to create existing file: %v", err)
	}

	// Try to export without --force
	opts := DebugOpts{Export: exportPath, Force: false}
	err := debugJournalCommand(tmpDir, "wave1/agent-A", opts)

	if err == nil {
		t.Error("expected error when exporting to existing file without --force")
	}

	if !strings.Contains(err.Error(), "already exists") {
		t.Errorf("expected 'already exists' error, got: %v", err)
	}
}

func TestDebugJournal_ExportWithForce(t *testing.T) {
	tmpDir, indexPath := setupTestJournal(t)

	entries := []journal.ToolEntry{
		{
			Timestamp: time.Now(),
			Kind:      "tool_use",
			ToolName:  "Read",
			ToolUseID: "test-1",
			Input:     map[string]interface{}{"file_path": "test.go"},
		},
	}
	writeTestEntries(t, indexPath, entries)

	exportPath := filepath.Join(tmpDir, "timeline.html")

	// Create the file first
	if err := os.WriteFile(exportPath, []byte("existing"), 0644); err != nil {
		t.Fatalf("failed to create existing file: %v", err)
	}

	// Export with --force
	opts := DebugOpts{Export: exportPath, Force: true}
	err := debugJournalCommand(tmpDir, "wave1/agent-A", opts)

	if err != nil {
		t.Fatalf("debugJournalCommand failed with --force: %v", err)
	}

	// Verify file was overwritten
	content, err := os.ReadFile(exportPath)
	if err != nil {
		t.Fatalf("failed to read HTML file: %v", err)
	}

	if string(content) == "existing" {
		t.Error("file was not overwritten with --force")
	}
}
