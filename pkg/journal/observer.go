package journal

import (
	"bufio"
	"crypto/md5"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/protocol"
	"github.com/blackwell-systems/scout-and-wave-go/pkg/result"
)

// SaveCursorData holds the result data from a successful saveCursor operation.
type SaveCursorData struct {
	SessionFile string `json:"session_file"`
	Offset      int64  `json:"offset"`
}

// AppendData holds the result data from a successful appendToIndex operation.
type AppendData struct {
	EntriesAppended int `json:"entries_appended"`
}

// UpdateData holds the result data from a successful updateRecent operation.
type UpdateData struct {
	TotalEntries int `json:"total_entries"`
}

// JournalObserver tails Claude Code session logs and extracts tool execution history.
type JournalObserver struct {
	ProjectRoot string
	JournalDir  string
	AgentID     string
	CursorPath  string
	IndexPath   string
	RecentPath  string
	ResultsDir  string
	logger      *slog.Logger
}

// SetLogger configures the logger used for non-fatal diagnostic messages.
// If never called, the observer falls back to slog.Default().
func (o *JournalObserver) SetLogger(logger *slog.Logger) {
	o.logger = logger
}

// log returns the configured logger, falling back to slog.Default() if nil.
func (o *JournalObserver) log() *slog.Logger {
	if o.logger == nil {
		return slog.Default()
	}
	return o.logger
}

// NewObserver creates a journal observer instance for an agent.
// It creates the necessary directory structure under .saw-state/wave{N}/agent-{ID}/.
func NewObserver(projectRoot string, agentID string) (*JournalObserver, error) {
	// Extract wave number from agentID (e.g., "agent-A" -> wave1)
	// For simplicity, assume wave1 if not specified
	waveDir := "wave1"
	if strings.Contains(agentID, "wave") {
		// Handle agentID like "wave2-agent-B"
		parts := strings.Split(agentID, "-")
		if len(parts) > 0 && strings.HasPrefix(parts[0], "wave") {
			waveDir = parts[0]
			agentID = strings.Join(parts[1:], "-")
		}
	}

	journalDir := filepath.Join(protocol.SAWStateDir(projectRoot), waveDir, agentID)
	ResultsDir := filepath.Join(journalDir, "tool-results")

	// Create directory structure
	if err := os.MkdirAll(journalDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create journal dir: %w", err)
	}
	if err := os.MkdirAll(ResultsDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create results dir: %w", err)
	}

	o := &JournalObserver{
		ProjectRoot: projectRoot,
		JournalDir:  journalDir,
		AgentID:     agentID,
		CursorPath:  filepath.Join(journalDir, "cursor.json"),
		IndexPath:   filepath.Join(journalDir, "index.jsonl"),
		RecentPath:  filepath.Join(journalDir, "recent.json"),
		ResultsDir:  ResultsDir,
	}

	return o, nil
}

// Sync incrementally reads the session log from the cursor position and extracts
// tool_use and tool_result entries. Returns the number of new entries processed.
func (o *JournalObserver) Sync() (*SyncResult, error) {
	// Find the latest session file
	sessionFile, err := o.findLatestSessionFile()
	if err != nil {
		return nil, fmt.Errorf("failed to find session file: %w", err)
	}

	// No session files yet - return empty result (fresh session)
	if sessionFile == "" {
		return &SyncResult{
			NewToolUses:    0,
			NewToolResults: 0,
			NewBytes:       0,
		}, nil
	}

	// Load cursor
	cursor, err := o.loadCursor()
	if err != nil {
		return nil, fmt.Errorf("failed to load cursor: %w", err)
	}

	// If session file changed, reset cursor
	if cursor.SessionFile != sessionFile {
		cursor = &SessionCursor{
			SessionFile: sessionFile,
			Offset:      0,
		}
	}

	// Open session file
	f, err := os.Open(filepath.Join(o.getClaudeProjectDir(), sessionFile))
	if err != nil {
		return nil, fmt.Errorf("failed to open session file: %w", err)
	}
	defer f.Close()

	// Seek to cursor position
	if _, err := f.Seek(cursor.Offset, 0); err != nil {
		return nil, fmt.Errorf("failed to seek to cursor: %w", err)
	}

	// Parse new entries
	entries := []ToolEntry{}
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 1024*1024), 10*1024*1024) // 10MB max line size
	bytesRead := int64(0)

	for scanner.Scan() {
		line := scanner.Bytes()
		bytesRead += int64(len(line)) + 1 // +1 for newline

		// Parse JSONL line
		var logEntry map[string]interface{}
		if err := json.Unmarshal(line, &logEntry); err != nil {
			// Skip malformed lines
			continue
		}

		// Extract tool blocks from this log entry
		extracted := o.extractToolEntries(logEntry)
		entries = append(entries, extracted...)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("failed to read session file: %w", err)
	}

	// Update cursor
	cursor.Offset += bytesRead
	if r := o.saveCursor(cursor); r.IsFatal() {
		return nil, fmt.Errorf("failed to save cursor: %s", r.Errors[0].Message)
	}

	// Append to index
	if len(entries) > 0 {
		if r := o.appendToIndex(entries); r.IsFatal() {
			return nil, fmt.Errorf("failed to append to index: %s", r.Errors[0].Message)
		}

		// Update recent cache
		if r := o.updateRecent(entries); r.IsFatal() {
			return nil, fmt.Errorf("failed to update recent: %s", r.Errors[0].Message)
		}
	}

	// Count tool uses vs results
	result := &SyncResult{
		NewBytes: bytesRead,
	}
	for _, e := range entries {
		if e.Kind == "tool_use" {
			result.NewToolUses++
		} else if e.Kind == "tool_result" {
			result.NewToolResults++
		}
	}

	return result, nil
}

// extractToolEntries parses a Claude Code log entry and extracts tool_use and
// tool_result blocks from the nested content structure.
func (o *JournalObserver) extractToolEntries(logEntry map[string]interface{}) []ToolEntry {
	entries := []ToolEntry{}

	// Extract timestamp
	timestampStr, ok := logEntry["timestamp"].(string)
	if !ok {
		return entries
	}
	timestamp, err := time.Parse(time.RFC3339Nano, timestampStr)
	if err != nil {
		timestamp = time.Now()
	}

	// Navigate to content.messages[].content[] where tool blocks live
	content, ok := logEntry["content"].(map[string]interface{})
	if !ok {
		return entries
	}

	messages, ok := content["messages"].([]interface{})
	if !ok {
		return entries
	}

	for _, msg := range messages {
		msgMap, ok := msg.(map[string]interface{})
		if !ok {
			continue
		}

		contentBlocks, ok := msgMap["content"].([]interface{})
		if !ok {
			continue
		}

		for _, block := range contentBlocks {
			blockMap, ok := block.(map[string]interface{})
			if !ok {
				continue
			}

			blockType, ok := blockMap["type"].(string)
			if !ok {
				continue
			}

			if blockType == "tool_use" {
				entry := o.extractToolUse(blockMap, timestamp)
				if entry != nil {
					entries = append(entries, *entry)
				}
			} else if blockType == "tool_result" {
				entry := o.extractToolResult(blockMap, timestamp)
				if entry != nil {
					entries = append(entries, *entry)
				}
			}
		}
	}

	return entries
}

// extractToolUse parses a tool_use block into a ToolEntry.
func (o *JournalObserver) extractToolUse(block map[string]interface{}, timestamp time.Time) *ToolEntry {
	toolUseID, ok := block["id"].(string)
	if !ok {
		return nil
	}

	toolName, _ := block["name"].(string)
	input, _ := block["input"].(map[string]interface{})

	return &ToolEntry{
		Timestamp: timestamp,
		Kind:      "tool_use",
		ToolName:  toolName,
		ToolUseID: toolUseID,
		Input:     input,
	}
}

// extractToolResult parses a tool_result block into a ToolEntry.
func (o *JournalObserver) extractToolResult(block map[string]interface{}, timestamp time.Time) *ToolEntry {
	toolUseID, ok := block["tool_use_id"].(string)
	if !ok {
		return nil
	}

	// Extract content (can be string or array of content blocks)
	var fullContent string
	if contentStr, ok := block["content"].(string); ok {
		fullContent = contentStr
	} else if contentBlocks, ok := block["content"].([]interface{}); ok {
		parts := []string{}
		for _, cb := range contentBlocks {
			if cbMap, ok := cb.(map[string]interface{}); ok {
				if text, ok := cbMap["text"].(string); ok {
					parts = append(parts, text)
				}
			}
		}
		fullContent = strings.Join(parts, "\n")
	}

	// Save full content to file
	contentFile := filepath.Join(o.ResultsDir, fmt.Sprintf("%s.txt", toolUseID))
	if err := os.WriteFile(contentFile, []byte(fullContent), 0644); err == nil {
		contentFile = filepath.Join("tool-results", fmt.Sprintf("%s.txt", toolUseID))
	} else {
		contentFile = ""
	}

	// Create preview (first 800 chars)
	preview := fullContent
	truncated := false
	if len(preview) > 800 {
		preview = preview[:800]
		truncated = true
	}

	return &ToolEntry{
		Timestamp:   timestamp,
		Kind:        "tool_result",
		ToolUseID:   toolUseID,
		ContentFile: contentFile,
		Preview:     preview,
		Truncated:   truncated,
	}
}

// findLatestSessionFile locates the most recent .jsonl file in the Claude project directory.
// Returns empty string (not error) when no session files exist yet.
func (o *JournalObserver) findLatestSessionFile() (string, error) {
	projectDir := o.getClaudeProjectDir()

	// Find all .jsonl files
	pattern := filepath.Join(projectDir, "*.jsonl")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return "", fmt.Errorf("failed to glob session files: %w", err)
	}

	if len(matches) == 0 {
		// No session files yet - fresh session with no prior tool execution
		return "", nil
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

	// Return just the filename, not the full path
	return filepath.Base(files[0].path), nil
}

// getClaudeProjectDir returns the path to the Claude Code project directory.
// For simplicity, we use an MD5 hash of the project root basename.
func (o *JournalObserver) getClaudeProjectDir() string {
	projectName := filepath.Base(o.ProjectRoot)
	hash := md5.Sum([]byte(projectName))
	projectHash := fmt.Sprintf("%x", hash)[:16]

	homeDir, err := os.UserHomeDir()
	if err != nil {
		homeDir = "/tmp"
	}

	return filepath.Join(homeDir, ".claude", "projects", projectHash)
}

// loadCursor loads the session cursor from disk.
func (o *JournalObserver) loadCursor() (*SessionCursor, error) {
	data, err := os.ReadFile(o.CursorPath)
	if os.IsNotExist(err) {
		// No cursor yet, start from beginning
		return &SessionCursor{}, nil
	}
	if err != nil {
		return nil, err
	}

	var cursor SessionCursor
	if err := json.Unmarshal(data, &cursor); err != nil {
		return nil, err
	}

	return &cursor, nil
}

// saveCursor persists the session cursor to disk.
func (o *JournalObserver) saveCursor(cursor *SessionCursor) result.Result[SaveCursorData] {
	data, err := json.MarshalIndent(cursor, "", "  ")
	if err != nil {
		return result.NewFailure[SaveCursorData]([]result.SAWError{{
			Code:     "JOURNAL_ERROR",
			Message:  err.Error(),
			Severity: "fatal",
		}})
	}
	if err := os.WriteFile(o.CursorPath, data, 0644); err != nil {
		return result.NewFailure[SaveCursorData]([]result.SAWError{{
			Code:     "JOURNAL_ERROR",
			Message:  err.Error(),
			Severity: "fatal",
		}})
	}
	return result.NewSuccess(SaveCursorData{
		SessionFile: cursor.SessionFile,
		Offset:      cursor.Offset,
	})
}

// appendToIndex appends new tool entries to the index.jsonl file.
func (o *JournalObserver) appendToIndex(entries []ToolEntry) result.Result[AppendData] {
	f, err := os.OpenFile(o.IndexPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return result.NewFailure[AppendData]([]result.SAWError{{
			Code:     "JOURNAL_ERROR",
			Message:  err.Error(),
			Severity: "fatal",
		}})
	}
	defer f.Close()

	encoder := json.NewEncoder(f)
	for _, entry := range entries {
		if err := encoder.Encode(entry); err != nil {
			return result.NewFailure[AppendData]([]result.SAWError{{
				Code:     "JOURNAL_ERROR",
				Message:  err.Error(),
				Severity: "fatal",
			}})
		}
	}

	return result.NewSuccess(AppendData{EntriesAppended: len(entries)})
}

// updateRecent maintains a sliding window of the last 30 tool entries in recent.json.
func (o *JournalObserver) updateRecent(newEntries []ToolEntry) result.Result[UpdateData] {
	// Load existing recent entries
	var existing []ToolEntry
	if data, err := os.ReadFile(o.RecentPath); err == nil {
		_ = json.Unmarshal(data, &existing)
	}

	// Append new entries
	existing = append(existing, newEntries...)

	// Keep only last 30
	if len(existing) > 30 {
		existing = existing[len(existing)-30:]
	}

	// Save back
	data, err := json.MarshalIndent(existing, "", "  ")
	if err != nil {
		return result.NewFailure[UpdateData]([]result.SAWError{{
			Code:     "JOURNAL_ERROR",
			Message:  err.Error(),
			Severity: "fatal",
		}})
	}
	if err := os.WriteFile(o.RecentPath, data, 0644); err != nil {
		return result.NewFailure[UpdateData]([]result.SAWError{{
			Code:     "JOURNAL_ERROR",
			Message:  err.Error(),
			Severity: "fatal",
		}})
	}
	return result.NewSuccess(UpdateData{TotalEntries: len(existing)})
}

// LoadJournal reads all entries from index.jsonl.
// Returns empty slice (not error) if index.jsonl doesn't exist.
func (o *JournalObserver) LoadJournal() ([]ToolEntry, error) {
	// Check if index file exists
	data, err := os.ReadFile(o.IndexPath)
	if os.IsNotExist(err) {
		// No journal yet - return empty slice
		return []ToolEntry{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to read index: %w", err)
	}

	// Parse JSONL (one JSON object per line)
	entries := []ToolEntry{}
	lines := strings.Split(string(data), "\n")

	for i, line := range lines {
		// Skip empty lines
		if strings.TrimSpace(line) == "" {
			continue
		}

		var entry ToolEntry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			// Skip malformed lines with warning, but don't fail
			o.log().Warn("skipping malformed JSONL line", "line", i+1, "path", o.IndexPath, "err", err)
			continue
		}

		entries = append(entries, entry)
	}

	return entries, nil
}

// GenerateContext generates markdown context summary from journal entries.
// Calls journal.GenerateContext() with loaded journal entries.
func (o *JournalObserver) GenerateContext() (string, error) {
	// Load all journal entries
	entries, err := o.LoadJournal()
	if err != nil {
		return "", fmt.Errorf("failed to load journal: %w", err)
	}

	// Call journal.GenerateContext with no limit (maxEntries=0 means all entries)
	context, err := GenerateContext(entries, 0)
	if err != nil {
		return "", fmt.Errorf("failed to generate context: %w", err)
	}

	return context, nil
}

