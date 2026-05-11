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

	"github.com/blackwell-systems/polywave-go/pkg/protocol"
	"github.com/blackwell-systems/polywave-go/pkg/result"
)

// saveCursorData holds the result data from a successful saveCursor operation.
type saveCursorData struct {
	SessionFile string `json:"session_file"`
	Offset      int64  `json:"offset"`
}

// appendData holds the result data from a successful appendToIndex operation.
type appendData struct {
	EntriesAppended int `json:"entries_appended"`
}

// updateData holds the result data from a successful updateRecent operation.
type updateData struct {
	TotalEntries int `json:"total_entries"`
}

// JournalObserver tails Claude Code session logs and extracts tool execution history.
type JournalObserver struct {
	ProjectRoot string
	JournalDir  string
	AgentID     string
	RawAgentID  string // original agentID before wave-prefix strip
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
// It creates the necessary directory structure under .polywave-state/wave{N}/agent-{ID}/.
func NewObserver(projectRoot string, agentID string) result.Result[*JournalObserver] {
	rawAgentID := agentID

	// Extract wave number from agentID (e.g., "agent-A" -> wave1)
	// For simplicity, assume wave1 if not specified
	waveDir := "wave1"
	strippedAgentID := agentID
	if strings.Contains(agentID, "wave") {
		// Handle agentID like "wave2-agent-B"
		parts := strings.Split(agentID, "-")
		if len(parts) > 0 && strings.HasPrefix(parts[0], "wave") {
			waveDir = parts[0]
			strippedAgentID = strings.Join(parts[1:], "-")
		}
	}

	journalDir := filepath.Join(protocol.PolywaveStateDir(projectRoot), waveDir, strippedAgentID)
	resultsDir := filepath.Join(journalDir, "tool-results")

	// Create directory structure
	if err := os.MkdirAll(journalDir, 0755); err != nil {
		return result.NewFailure[*JournalObserver]([]result.PolywaveError{{
			Code:     result.CodeJournalObserverCursorFailed,
			Message:  fmt.Sprintf("failed to create journal dir: %s", err.Error()),
			Severity: "fatal",
		}})
	}
	if err := os.MkdirAll(resultsDir, 0755); err != nil {
		return result.NewFailure[*JournalObserver]([]result.PolywaveError{{
			Code:     result.CodeJournalObserverCursorFailed,
			Message:  fmt.Sprintf("failed to create results dir: %s", err.Error()),
			Severity: "fatal",
		}})
	}

	o := &JournalObserver{
		ProjectRoot: projectRoot,
		JournalDir:  journalDir,
		AgentID:     strippedAgentID,
		RawAgentID:  rawAgentID,
		CursorPath:  filepath.Join(journalDir, "cursor.json"),
		IndexPath:   filepath.Join(journalDir, "index.jsonl"),
		RecentPath:  filepath.Join(journalDir, "recent.json"),
		ResultsDir:  resultsDir,
	}

	return result.NewSuccess(o)
}

// Sync incrementally reads the session log from the cursor position and extracts
// tool_use and tool_result entries. Returns the number of new entries processed.
func (o *JournalObserver) Sync() result.Result[*SyncResult] {
	// Find the latest session file
	sessionFile, err := o.findLatestSessionFile()
	if err != nil {
		return result.NewFailure[*SyncResult]([]result.PolywaveError{{
			Code:     result.CodeJournalObserverCursorFailed,
			Message:  fmt.Sprintf("failed to find session file: %s", err.Error()),
			Severity: "fatal",
		}})
	}

	// No session files yet - return empty result (fresh session)
	if sessionFile == "" {
		return result.NewSuccess(&SyncResult{
			NewToolUses:    0,
			NewToolResults: 0,
			NewBytes:       0,
		})
	}

	// Load cursor
	cursor, err := o.loadCursor()
	if err != nil {
		return result.NewFailure[*SyncResult]([]result.PolywaveError{{
			Code:     result.CodeJournalObserverCursorFailed,
			Message:  fmt.Sprintf("failed to load cursor: %s", err.Error()),
			Severity: "fatal",
		}})
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
		return result.NewFailure[*SyncResult]([]result.PolywaveError{{
			Code:     result.CodeJournalObserverCursorFailed,
			Message:  fmt.Sprintf("failed to open session file: %s", err.Error()),
			Severity: "fatal",
		}})
	}
	defer f.Close()

	// Seek to cursor position
	if _, err := f.Seek(cursor.Offset, 0); err != nil {
		return result.NewFailure[*SyncResult]([]result.PolywaveError{{
			Code:     result.CodeJournalObserverCursorFailed,
			Message:  fmt.Sprintf("failed to seek to cursor: %s", err.Error()),
			Severity: "fatal",
		}})
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
		return result.NewFailure[*SyncResult]([]result.PolywaveError{{
			Code:     result.CodeJournalObserverAppendFailed,
			Message:  fmt.Sprintf("failed to read session file: %s", err.Error()),
			Severity: "fatal",
		}})
	}

	// Update cursor
	cursor.Offset += bytesRead
	if r := o.saveCursor(cursor); r.IsFatal() {
		return result.NewFailure[*SyncResult]([]result.PolywaveError{{
			Code:     result.CodeJournalObserverCursorFailed,
			Message:  fmt.Sprintf("failed to save cursor: %s", r.Errors[0].Message),
			Severity: "fatal",
		}})
	}

	// Append to index
	if len(entries) > 0 {
		if r := o.appendToIndex(entries); r.IsFatal() {
			return result.NewFailure[*SyncResult]([]result.PolywaveError{{
				Code:     result.CodeJournalObserverAppendFailed,
				Message:  fmt.Sprintf("failed to append to index: %s", r.Errors[0].Message),
				Severity: "fatal",
			}})
		}

		// Update recent cache
		if r := o.updateRecent(entries); r.IsFatal() {
			return result.NewFailure[*SyncResult]([]result.PolywaveError{{
				Code:     result.CodeJournalObserverUpdateFailed,
				Message:  fmt.Sprintf("failed to update recent: %s", r.Errors[0].Message),
				Severity: "fatal",
			}})
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

	return result.NewSuccess(syncRes)
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
		o.log().Warn("failed to write tool result content file", "tool_use_id", toolUseID, "err", err)
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
// Hashes the full project root path to produce a stable, unique directory name.
func (o *JournalObserver) getClaudeProjectDir() string {
	hash := md5.Sum([]byte(o.ProjectRoot))
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
func (o *JournalObserver) saveCursor(cursor *SessionCursor) result.Result[saveCursorData] {
	data, err := json.MarshalIndent(cursor, "", "  ")
	if err != nil {
		return result.NewFailure[saveCursorData]([]result.PolywaveError{{
			Code:     result.CodeJournalObserverCursorFailed,
			Message:  err.Error(),
			Severity: "fatal",
		}})
	}
	if err := os.WriteFile(o.CursorPath, data, 0644); err != nil {
		return result.NewFailure[saveCursorData]([]result.PolywaveError{{
			Code:     result.CodeJournalObserverCursorFailed,
			Message:  err.Error(),
			Severity: "fatal",
		}})
	}
	return result.NewSuccess(saveCursorData{
		SessionFile: cursor.SessionFile,
		Offset:      cursor.Offset,
	})
}

// appendToIndex appends new tool entries to the index.jsonl file.
func (o *JournalObserver) appendToIndex(entries []ToolEntry) result.Result[appendData] {
	f, err := os.OpenFile(o.IndexPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return result.NewFailure[appendData]([]result.PolywaveError{{
			Code:     result.CodeJournalObserverAppendFailed,
			Message:  err.Error(),
			Severity: "fatal",
		}})
	}
	defer f.Close()

	encoder := json.NewEncoder(f)
	for _, entry := range entries {
		if err := encoder.Encode(entry); err != nil {
			return result.NewFailure[appendData]([]result.PolywaveError{{
				Code:     result.CodeJournalObserverAppendFailed,
				Message:  err.Error(),
				Severity: "fatal",
			}})
		}
	}

	return result.NewSuccess(appendData{EntriesAppended: len(entries)})
}

// updateRecent maintains a sliding window of the last 30 tool entries in recent.json.
func (o *JournalObserver) updateRecent(newEntries []ToolEntry) result.Result[updateData] {
	// Load existing recent entries
	var existing []ToolEntry
	if data, err := os.ReadFile(o.RecentPath); err == nil {
		if err := json.Unmarshal(data, &existing); err != nil {
			o.log().Debug("failed to parse recent.json, starting fresh", "err", err)
		}
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
		return result.NewFailure[updateData]([]result.PolywaveError{{
			Code:     result.CodeJournalObserverUpdateFailed,
			Message:  err.Error(),
			Severity: "fatal",
		}})
	}
	if err := os.WriteFile(o.RecentPath, data, 0644); err != nil {
		return result.NewFailure[updateData]([]result.PolywaveError{{
			Code:     result.CodeJournalObserverUpdateFailed,
			Message:  err.Error(),
			Severity: "fatal",
		}})
	}
	return result.NewSuccess(updateData{TotalEntries: len(existing)})
}

// LoadJournal reads all entries from index.jsonl.
// Returns empty slice (not error) if index.jsonl doesn't exist.
func (o *JournalObserver) LoadJournal() result.Result[[]ToolEntry] {
	// Check if index file exists
	data, err := os.ReadFile(o.IndexPath)
	if os.IsNotExist(err) {
		// No journal yet - return empty slice
		return result.NewSuccess([]ToolEntry{})
	}
	if err != nil {
		return result.NewFailure[[]ToolEntry]([]result.PolywaveError{{
			Code:     result.CodeJournalObserverAppendFailed,
			Message:  fmt.Sprintf("failed to read index: %s", err.Error()),
			Severity: "fatal",
		}})
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

	return result.NewSuccess(entries)
}

// GenerateContext generates markdown context summary from journal entries.
// Calls journal.GenerateContext() with loaded journal entries.
func (o *JournalObserver) GenerateContext() result.Result[string] {
	res := o.LoadJournal()
	if res.IsFatal() {
		return result.NewFailure[string]([]result.PolywaveError{{
			Code:     result.CodeJournalObserverAppendFailed,
			Message:  fmt.Sprintf("failed to load journal: %s", res.Errors[0].Message),
			Severity: "fatal",
		}})
	}
	ctx := GenerateContext(*res.Data, 0)
	return result.NewSuccess(ctx)
}
