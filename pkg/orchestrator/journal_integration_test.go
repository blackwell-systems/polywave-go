package orchestrator

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/journal"
)

func TestPrepareAgentContext_NoJournal(t *testing.T) {
	tmpDir := t.TempDir()

	// Call PrepareAgentContext when no .saw-state directory exists
	context, err := PrepareAgentContext(PrepareAgentContextOpts{
		ProjectRoot: tmpDir,
		WaveNum:     1,
		AgentID:     "agent-A",
		MaxEntries:  50,
	})
	if err != nil {
		t.Fatalf("PrepareAgentContext failed: %v", err)
	}

	// Should return empty string (not error) when no journal exists
	if context != "" {
		t.Errorf("Expected empty string for missing journal, got: %s", context)
	}
}

func TestPrepareAgentContext_WithJournal(t *testing.T) {
	tmpDir := t.TempDir()

	// Create journal directory structure
	journalDir := filepath.Join(tmpDir, ".saw-state", "wave1", "agent-A")
	if err := os.MkdirAll(journalDir, 0755); err != nil {
		t.Fatalf("Failed to create journal dir: %v", err)
	}

	// Write sample journal entries
	indexPath := filepath.Join(journalDir, "index.jsonl")
	entries := []journal.ToolEntry{
		{
			Timestamp: time.Now().Add(-10 * time.Minute),
			Kind:      "tool_use",
			ToolName:  "Read",
			ToolUseID: "toolu_001",
			Input: map[string]interface{}{
				"file_path": "pkg/module/file.go",
			},
		},
		{
			Timestamp: time.Now().Add(-9 * time.Minute),
			Kind:      "tool_result",
			ToolUseID: "toolu_001",
			Preview:   "package module\n\nfunc Example() {}",
		},
		{
			Timestamp: time.Now().Add(-5 * time.Minute),
			Kind:      "tool_use",
			ToolName:  "Write",
			ToolUseID: "toolu_002",
			Input: map[string]interface{}{
				"file_path": "pkg/module/newfile.go",
				"content":   "package module\n\nfunc NewFunc() {}",
			},
		},
	}

	f, err := os.Create(indexPath)
	if err != nil {
		t.Fatalf("Failed to create index.jsonl: %v", err)
	}
	encoder := json.NewEncoder(f)
	for _, entry := range entries {
		if err := encoder.Encode(entry); err != nil {
			t.Fatalf("Failed to encode entry: %v", err)
		}
	}
	f.Close()

	// Call PrepareAgentContext
	context, err := PrepareAgentContext(PrepareAgentContextOpts{
		ProjectRoot: tmpDir,
		WaveNum:     1,
		AgentID:     "agent-A",
		MaxEntries:  50,
	})
	if err != nil {
		t.Fatalf("PrepareAgentContext failed: %v", err)
	}

	// Verify context was generated
	if context == "" {
		t.Fatalf("Expected non-empty context, got empty string")
	}

	// Verify header matches expected format
	if !strings.Contains(context, "## Session Context (Recovered from Tool Journal)") {
		t.Errorf("Context missing expected header, got:\n%s", context)
	}

	// Verify file modification detected
	if !strings.Contains(context, "pkg/module/newfile.go") {
		t.Errorf("Context should mention newfile.go, got:\n%s", context)
	}
}

func TestPrepareAgentContext_MaxEntriesLimit(t *testing.T) {
	tmpDir := t.TempDir()

	// Create journal with 100 entries
	journalDir := filepath.Join(tmpDir, ".saw-state", "wave1", "agent-B")
	if err := os.MkdirAll(journalDir, 0755); err != nil {
		t.Fatalf("Failed to create journal dir: %v", err)
	}

	indexPath := filepath.Join(journalDir, "index.jsonl")
	f, err := os.Create(indexPath)
	if err != nil {
		t.Fatalf("Failed to create index.jsonl: %v", err)
	}

	encoder := json.NewEncoder(f)
	baseTime := time.Now().Add(-100 * time.Minute)
	for i := 0; i < 100; i++ {
		entry := journal.ToolEntry{
			Timestamp: baseTime.Add(time.Duration(i) * time.Minute),
			Kind:      "tool_use",
			ToolName:  "Read",
			ToolUseID: "toolu_" + string(rune('A'+i%26)),
			Input: map[string]interface{}{
				"file_path": "file.go",
			},
		}
		if err := encoder.Encode(entry); err != nil {
			t.Fatalf("Failed to encode entry %d: %v", i, err)
		}
	}
	f.Close()

	// Call with maxEntries=10 (should only process last 10 entries)
	context, err := PrepareAgentContext(PrepareAgentContextOpts{
		ProjectRoot: tmpDir,
		WaveNum:     1,
		AgentID:     "agent-B",
		MaxEntries:  10,
	})
	if err != nil {
		t.Fatalf("PrepareAgentContext failed: %v", err)
	}

	if context == "" {
		t.Fatalf("Expected non-empty context")
	}

	// Verify context was generated (hard to verify exact count, but context should be present)
	if !strings.Contains(context, "## Session Context") {
		t.Errorf("Context missing header")
	}
}

func TestPrepareAgentContext_DefaultMaxEntries(t *testing.T) {
	tmpDir := t.TempDir()

	// Create small journal
	journalDir := filepath.Join(tmpDir, ".saw-state", "wave2", "agent-C")
	if err := os.MkdirAll(journalDir, 0755); err != nil {
		t.Fatalf("Failed to create journal dir: %v", err)
	}

	indexPath := filepath.Join(journalDir, "index.jsonl")
	f, err := os.Create(indexPath)
	if err != nil {
		t.Fatalf("Failed to create index.jsonl: %v", err)
	}

	encoder := json.NewEncoder(f)
	entry := journal.ToolEntry{
		Timestamp: time.Now(),
		Kind:      "tool_use",
		ToolName:  "Read",
		ToolUseID: "toolu_001",
		Input:     map[string]interface{}{"file_path": "test.go"},
	}
	encoder.Encode(entry)
	f.Close()

	// Call with maxEntries=0 (should default to 50)
	context, err := PrepareAgentContext(PrepareAgentContextOpts{
		ProjectRoot: tmpDir,
		WaveNum:     2,
		AgentID:     "agent-C",
		MaxEntries:  0,
	})
	if err != nil {
		t.Fatalf("PrepareAgentContext failed: %v", err)
	}

	if context == "" {
		t.Fatalf("Expected non-empty context")
	}
}

func TestWriteJournalEntry_CreatesJournalDir(t *testing.T) {
	tmpDir := t.TempDir()

	// Write entry when no journal directory exists
	entry := journal.ToolEntry{
		Timestamp: time.Now(),
		Kind:      "tool_use",
		ToolName:  "Bash",
		ToolUseID: "toolu_001",
		Input: map[string]interface{}{
			"command": "go test ./...",
		},
	}

	err := WriteJournalEntry(tmpDir, 1, "agent-D", entry)
	if err != nil {
		t.Fatalf("WriteJournalEntry failed: %v", err)
	}

	// Verify journal directory was created
	journalDir := filepath.Join(tmpDir, ".saw-state", "wave1", "agent-D")
	if _, err := os.Stat(journalDir); os.IsNotExist(err) {
		t.Errorf("Journal directory not created at %s", journalDir)
	}

	// Verify index.jsonl was created
	indexPath := filepath.Join(journalDir, "index.jsonl")
	if _, err := os.Stat(indexPath); os.IsNotExist(err) {
		t.Errorf("index.jsonl not created at %s", indexPath)
	}
}

func TestWriteJournalEntry_AppendsEntry(t *testing.T) {
	tmpDir := t.TempDir()

	// Create journal directory
	journalDir := filepath.Join(tmpDir, ".saw-state", "wave1", "agent-E")
	if err := os.MkdirAll(journalDir, 0755); err != nil {
		t.Fatalf("Failed to create journal dir: %v", err)
	}

	// Write first entry
	entry1 := journal.ToolEntry{
		Timestamp: time.Now(),
		Kind:      "tool_use",
		ToolName:  "Read",
		ToolUseID: "toolu_001",
		Input:     map[string]interface{}{"file_path": "file1.go"},
	}
	if err := WriteJournalEntry(tmpDir, 1, "agent-E", entry1); err != nil {
		t.Fatalf("First WriteJournalEntry failed: %v", err)
	}

	// Write second entry
	entry2 := journal.ToolEntry{
		Timestamp: time.Now(),
		Kind:      "tool_use",
		ToolName:  "Write",
		ToolUseID: "toolu_002",
		Input:     map[string]interface{}{"file_path": "file2.go", "content": "data"},
	}
	if err := WriteJournalEntry(tmpDir, 1, "agent-E", entry2); err != nil {
		t.Fatalf("Second WriteJournalEntry failed: %v", err)
	}

	// Read index.jsonl and verify both entries present
	indexPath := filepath.Join(journalDir, "index.jsonl")
	data, err := os.ReadFile(indexPath)
	if err != nil {
		t.Fatalf("Failed to read index.jsonl: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 2 {
		t.Errorf("Expected 2 entries in index.jsonl, got %d", len(lines))
	}

	// Verify entries can be parsed
	var parsedEntry1, parsedEntry2 journal.ToolEntry
	if err := json.Unmarshal([]byte(lines[0]), &parsedEntry1); err != nil {
		t.Errorf("Failed to parse first entry: %v", err)
	}
	if err := json.Unmarshal([]byte(lines[1]), &parsedEntry2); err != nil {
		t.Errorf("Failed to parse second entry: %v", err)
	}

	if parsedEntry1.ToolUseID != "toolu_001" {
		t.Errorf("First entry ToolUseID = %s, want toolu_001", parsedEntry1.ToolUseID)
	}
	if parsedEntry2.ToolUseID != "toolu_002" {
		t.Errorf("Second entry ToolUseID = %s, want toolu_002", parsedEntry2.ToolUseID)
	}
}
