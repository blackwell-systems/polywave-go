package journal

import "time"

// SessionCursor tracks read position in Claude Code session log
type SessionCursor struct {
	SessionFile string `json:"session_file"` // e.g., "1a2b3c4d.jsonl"
	Offset      int64  `json:"offset"`       // Byte offset in file
}

// ToolEntry represents a single tool use or tool result in the journal
type ToolEntry struct {
	Timestamp   time.Time              `json:"ts"`
	Kind        string                 `json:"kind"` // "tool_use" or "tool_result"
	ToolName    string                 `json:"tool_name,omitempty"`
	ToolUseID   string                 `json:"tool_use_id"`
	Input       map[string]interface{} `json:"input,omitempty"`
	ContentFile string                 `json:"content_file,omitempty"` // Path to full output
	Preview     string                 `json:"preview,omitempty"`      // First 800 chars
	Truncated   bool                   `json:"truncated,omitempty"`
}

// SyncResult summarizes a journal sync operation
type SyncResult struct {
	NewToolUses    int
	NewToolResults int
	NewBytes       int64
}
