package cli

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"github.com/creack/pty"

	"github.com/blackwell-systems/polywave-go/pkg/agent/backend"
	"github.com/blackwell-systems/polywave-go/pkg/agent/dedup"
)

// ansiRE matches ANSI/VT100 escape sequences emitted by a PTY-connected process.
var ansiRE = regexp.MustCompile(`\x1b(?:[@-Z\\-_]|\[[0-?]*[ -/]*[@-~])`)

// nonJSONCap is the maximum number of bytes accumulated in nonJSON builders.
const nonJSONCap = 4096

// pendingCap is the maximum number of bytes accumulated in the pending JSON fragment builder.
// When exceeded, the fragment is discarded to prevent unbounded memory growth from malformed PTY output.
const pendingCap = 1024 * 1024 // 1 MB

// IsClaudeCodeSession returns true when the current process is running inside
// a Claude Code session (CLAUDECODE env var is set). This check must be
// performed before the env filtering loop strips the variable.
func IsClaudeCodeSession() bool {
	return os.Getenv("CLAUDECODE") != ""
}

// Client implements backend.Backend by shelling out to the claude CLI.
type Client struct {
	claudePath string
	cfg        backend.Config
}

// New creates a CLI Client. claudePath is the path to the claude binary;
// if empty, it is located via PATH at Run time.
func New(claudePath string, cfg backend.Config) *Client {
	return &Client{
		claudePath: claudePath,
		cfg:        cfg,
	}
}

// buildCLIArgs constructs the common CLI argument list for the claude binary.
func (c *Client) buildCLIArgs(prompt string) []string {
	args := []string{
		"--print",
		"--output-format", "stream-json",
		"--allowedTools", "Bash,Read,Write,Edit,Glob,Grep",
		"--dangerously-skip-permissions",
	}
	if c.cfg.Model != "" {
		args = append(args, "--model", c.cfg.Model)
	}
	if c.cfg.MaxTurns > 0 {
		args = append(args, "--max-turns", fmt.Sprintf("%d", c.cfg.MaxTurns))
	}
	args = append(args, "-p", prompt)
	return args
}

// buildPrompt combines systemPrompt and userMessage into a single prompt string.
func buildPrompt(systemPrompt, userMessage string) string {
	if systemPrompt == "" {
		return userMessage
	}
	return systemPrompt + "\n\n" + userMessage
}

// DedupStats returns nil for CLI backend — dedup runs inside the claude subprocess.
func (c *Client) DedupStats() *dedup.Stats { return nil }

// CommitCount returns 0 for CLI backend — commit tracking is post-hoc via constraints.
func (c *Client) CommitCount() int { return 0 }

// Run implements backend.Backend.
// Note: Run delegates to RunStreaming, which does not parse tool_use events.
// Post-hoc constraint enforcement (I1/I2/I6) is not applied on this path.
func (c *Client) Run(ctx context.Context, systemPrompt, userMessage, workDir string) (string, error) {
	return c.RunStreaming(ctx, systemPrompt, userMessage, workDir, nil)
}

// RunStreaming implements backend.Backend.
// Uses --output-format stream-json inside a PTY so Node.js line-buffers output
// (enabling real-time streaming). The PTY is set to 65535 columns to prevent
// line-wrapping artifacts in JSON, and the scanner accumulates PTY-wrapped
// fragments until a complete JSON object is assembled before parsing.
// Note: RunStreaming does not parse tool_use content blocks. Post-hoc constraint
// enforcement (I1/I2/I6) is not applied on this path; use RunStreamingWithTools instead.
func (c *Client) RunStreaming(ctx context.Context, systemPrompt, userMessage, workDir string, onChunk backend.ChunkCallback) (string, error) {
	if IsClaudeCodeSession() {
		return "", fmt.Errorf("cli backend: cannot spawn claude subprocess from within a Claude Code session — the nested process will fail. Run from a standalone terminal, or use --skip to bypass agent execution")
	}

	claudePath := c.claudePath
	if claudePath == "" {
		claudePath = c.cfg.BinaryPath
	}
	if claudePath == "" {
		var err error
		claudePath, err = exec.LookPath("claude")
		if err != nil {
			return "", fmt.Errorf("cli backend: claude binary not found in PATH: %w", err)
		}
	}

	prompt := buildPrompt(systemPrompt, userMessage)
	args := c.buildCLIArgs(prompt)

	cmd := exec.CommandContext(ctx, claudePath, args...)
	cmd.Dir = workDir

	// Strip CLAUDECODE so a nested claude process isn't rejected.
	filtered := make([]string, 0, len(os.Environ()))
	for _, env := range os.Environ() {
		if !strings.HasPrefix(env, "CLAUDECODE=") {
			filtered = append(filtered, env)
		}
	}
	cmd.Env = filtered

	// PTY forces Node.js to line-buffer stdout (real-time streaming).
	ptmx, err := pty.Start(cmd)
	if err != nil {
		return "", fmt.Errorf("cli backend: failed to start claude with PTY: %w", err)
	}
	defer ptmx.Close()

	// Max columns prevents PTY from wrapping JSON lines.
	// uint16 max = 65535; virtually no stream-json event reaches this length.
	_ = pty.Setsize(ptmx, &pty.Winsize{Rows: 50, Cols: 65535})

	rawOutput, nonJSON, scanErr := c.readPTYStream(ctx, ptmx, func(candidate string) {
		if onChunk != nil {
			if formatted := formatStreamEvent(candidate); formatted != "" {
				onChunk(formatted + "\n")
			}
		}
	})

	if scanErr != nil {
		if ctx.Err() != nil {
			_ = cmd.Wait()
			return "", fmt.Errorf("cli backend: context cancelled: %w", ctx.Err())
		}
		return "", fmt.Errorf("cli backend: error reading PTY: %w", scanErr)
	}

	if err := cmd.Wait(); err != nil {
		if ctx.Err() != nil {
			return "", fmt.Errorf("cli backend: context cancelled: %w", ctx.Err())
		}
		if exitErr, ok := err.(*exec.ExitError); ok {
			msg := fmt.Sprintf("cli backend: claude exited with code %d", exitErr.ExitCode())
			if extra := strings.TrimSpace(nonJSON); extra != "" {
				msg += ": " + extra
			}
			return "", fmt.Errorf("%s", msg)
		}
		return "", fmt.Errorf("cli backend: claude failed: %w", err)
	}

	return rawOutput, nil
}

// RunStreamingWithTools implements backend.Backend.
// It behaves identically to RunStreaming, but additionally calls onToolCall
// for each tool_use and tool_result event parsed from the stream.
// Both onChunk and onToolCall may be nil.
// When c.cfg.Constraints is non-nil, post-hoc constraint enforcement (I1/I2/I6)
// is applied after the subprocess completes: Write and Edit events are inspected
// against the constraint rules. Any violations are returned as an error.
// Post-hoc means the writes already occurred; enforcement detects and reports
// violations so the wave result can be marked as blocked.
func (c *Client) RunStreamingWithTools(ctx context.Context, systemPrompt, userMessage, workDir string, onChunk backend.ChunkCallback, onToolCall backend.ToolCallCallback) (string, error) {
	if IsClaudeCodeSession() {
		return "", fmt.Errorf("cli backend: cannot spawn claude subprocess from within a Claude Code session — the nested process will fail. Run from a standalone terminal, or use --skip to bypass agent execution")
	}

	claudePath := c.claudePath
	if claudePath == "" {
		claudePath = c.cfg.BinaryPath
	}
	if claudePath == "" {
		var err error
		claudePath, err = exec.LookPath("claude")
		if err != nil {
			return "", fmt.Errorf("cli backend: claude binary not found in PATH: %w", err)
		}
	}

	prompt := buildPrompt(systemPrompt, userMessage)
	args := c.buildCLIArgs(prompt)

	cmd := exec.CommandContext(ctx, claudePath, args...)
	cmd.Dir = workDir

	// Strip CLAUDECODE so a nested claude process isn't rejected.
	filtered := make([]string, 0, len(os.Environ()))
	for _, env := range os.Environ() {
		if !strings.HasPrefix(env, "CLAUDECODE=") {
			filtered = append(filtered, env)
		}
	}
	cmd.Env = filtered

	// PTY forces Node.js to line-buffer stdout (real-time streaming).
	ptmx, err := pty.Start(cmd)
	if err != nil {
		return "", fmt.Errorf("cli backend: failed to start claude with PTY: %w", err)
	}
	defer ptmx.Close()

	// Max columns prevents PTY from wrapping JSON lines.
	// uint16 max = 65535; virtually no stream-json event reaches this length.
	_ = pty.Setsize(ptmx, &pty.Winsize{Rows: 50, Cols: 65535})

	// Track tool call start times for duration calculation.
	toolStartTimes := make(map[string]int64)

	// Accumulate tool use events for post-hoc constraint enforcement.
	var toolUseEvents []ToolUseEvent

	rawOutput, nonJSON, scanErr := c.readPTYStream(ctx, ptmx, func(candidate string) {
		// Parse event for tool call tracking and constraint event recording.
		var ev streamEvent
		if json.Unmarshal([]byte(candidate), &ev) == nil {
			// Handle assistant messages with tool_use content blocks.
			if ev.Type == "assistant" && ev.Message != nil {
				for _, blk := range ev.Message.Content {
					if blk.Type == "tool_use" {
						// Record event for post-hoc constraint enforcement.
						filePath := extractToolInput(blk.Name, blk.Input)
						toolUseEvents = append(toolUseEvents, ToolUseEvent{
							ToolName: blk.Name,
							FilePath: filePath,
						})

						if onToolCall != nil {
							// Record start time and emit tool_use event.
							toolStartTimes[blk.ID] = time.Now().UnixMilli()
							onToolCall(backend.ToolCallEvent{
								ID:       blk.ID,
								Name:     blk.Name,
								Input:    filePath,
								IsResult: false,
							})
						}
					}
				}
			}
			// Handle tool_result events (top-level with tool_use_id).
			if ev.Type == "tool_result" && onToolCall != nil {
				// Parse the full tool_result event to get tool_use_id and is_error.
				var toolResult struct {
					Type      string `json:"type"`
					ToolUseID string `json:"tool_use_id"`
					IsError   bool   `json:"is_error"`
				}
				if json.Unmarshal([]byte(candidate), &toolResult) == nil {
					var durationMs int64
					if startTime, ok := toolStartTimes[toolResult.ToolUseID]; ok {
						durationMs = time.Now().UnixMilli() - startTime
						delete(toolStartTimes, toolResult.ToolUseID)
					}
					onToolCall(backend.ToolCallEvent{
						ID:         toolResult.ToolUseID,
						IsResult:   true,
						IsError:    toolResult.IsError,
						DurationMs: durationMs,
					})
				}
			}
		}

		if onChunk != nil {
			if formatted := formatStreamEvent(candidate); formatted != "" {
				onChunk(formatted + "\n")
			}
		}
	})

	if scanErr != nil {
		if ctx.Err() != nil {
			_ = cmd.Wait()
			return "", fmt.Errorf("cli backend: context cancelled: %w", ctx.Err())
		}
		return "", fmt.Errorf("cli backend: error reading PTY: %w", scanErr)
	}

	if err := cmd.Wait(); err != nil {
		if ctx.Err() != nil {
			return "", fmt.Errorf("cli backend: context cancelled: %w", ctx.Err())
		}
		if exitErr, ok := err.(*exec.ExitError); ok {
			msg := fmt.Sprintf("cli backend: claude exited with code %d", exitErr.ExitCode())
			if extra := strings.TrimSpace(nonJSON); extra != "" {
				msg += ": " + extra
			}
			return "", fmt.Errorf("%s", msg)
		}
		return "", fmt.Errorf("cli backend: claude failed: %w", err)
	}

	// Post-hoc constraint enforcement: inspect recorded Write/Edit events against
	// constraint rules. This runs after the subprocess completes; writes already
	// occurred. Violations are returned as an error so the engine marks the run failed.
	if c.cfg.Constraints != nil {
		if violations := CheckConstraints(toolUseEvents, c.cfg.Constraints); len(violations) > 0 {
			msgs := make([]string, 0, len(violations))
			for _, v := range violations {
				msgs = append(msgs, fmt.Sprintf("[%s] %s: %s", v.Invariant, v.ToolName, v.Message))
			}
			return rawOutput, fmt.Errorf("cli backend: post-hoc constraint violations detected (writes already occurred):\n%s",
				strings.Join(msgs, "\n"))
		}
	}

	return rawOutput, nil
}

// ── PTY stream reading ───────────────────────────────────────────────────────

// ptyEventHandler is called for each complete JSON object read from the PTY stream.
type ptyEventHandler func(candidate string)

// readPTYStream reads PTY output, strips ANSI codes, reassembles PTY-wrapped
// JSON fragments, and calls onEvent for each complete JSON object.
// Returns the raw JSON output (newline-separated), any non-JSON text (for error
// reporting, truncated with suffix if over nonJSONCap), and the scanner error.
func (c *Client) readPTYStream(ctx context.Context, ptmx *os.File, onEvent ptyEventHandler) (rawOutput string, nonJSONText string, scanErr error) {
	var sb strings.Builder
	scanner := bufio.NewScanner(ptmx)
	// 1 MB scanner buffer for large tool-result lines.
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)

	// pending accumulates PTY-wrapped fragments until we have a full JSON object.
	var pending strings.Builder
	// nonJSON accumulates non-JSON output (error messages, warnings) for error reporting.
	var nonJSON strings.Builder
	nonJSONTruncated := false

	for scanner.Scan() {
		// PTY converts \n -> \r\n; strip trailing \r.
		text := strings.TrimRight(scanner.Text(), "\r")
		// Strip ANSI escape sequences.
		text = ansiRE.ReplaceAllString(text, "")
		if text == "" {
			continue
		}

		pending.WriteString(text)
		candidate := pending.String()

		// Test if we have a complete JSON object yet.
		var probe json.RawMessage
		if json.Unmarshal([]byte(candidate), &probe) != nil {
			// If pending was empty before this line (i.e., the line itself is not valid JSON
			// and we are not accumulating a multi-line fragment), capture it as non-JSON.
			if pending.Len() == len(text) {
				if !nonJSONTruncated {
					if nonJSON.Len()+len(text)+1 > nonJSONCap {
						nonJSONTruncated = true
					} else {
						if nonJSON.Len() > 0 {
							nonJSON.WriteByte('\n')
						}
						nonJSON.WriteString(text)
					}
				}
				// Reset pending — this line is non-JSON, not a fragment.
				pending.Reset()
			} else if pending.Len() > pendingCap {
				// Accumulated fragment exceeds cap — discard to prevent unbounded growth.
				slog.Warn("cli backend: PTY JSON fragment accumulation exceeded cap, discarding", "size", pending.Len())
				pending.Reset()
			}
			// Incomplete JSON fragment — keep accumulating (PTY wrapped mid-JSON).
			continue
		}

		// Complete JSON object — process it.
		pending.Reset()
		sb.WriteString(candidate + "\n")

		if onEvent != nil {
			onEvent(candidate)
		}
	}

	nonJSONStr := nonJSON.String()
	if nonJSONTruncated {
		nonJSONStr += "... (truncated)"
	}

	return sb.String(), nonJSONStr, scanner.Err()
}

// ── stream-json event parsing ────────────────────────────────────────────────

type streamEvent struct {
	Type    string         `json:"type"`
	Subtype string         `json:"subtype,omitempty"`
	Message *streamMessage `json:"message,omitempty"`
	// tool_result
	Content json.RawMessage `json:"content,omitempty"`
	// result
	Result string `json:"result,omitempty"`
}

type streamMessage struct {
	Content []streamContent `json:"content"`
}

type streamContent struct {
	Type  string          `json:"type"`
	Text  string          `json:"text,omitempty"`
	ID    string          `json:"id,omitempty"`    // tool_use
	Name  string          `json:"name,omitempty"`  // tool_use
	Input json.RawMessage `json:"input,omitempty"` // tool_use
}

// formatStreamEvent converts a raw stream-json line into a human-readable
// string. Returns "" for events that should be skipped (system init, etc.).
func formatStreamEvent(raw string) string {
	var ev streamEvent
	if err := json.Unmarshal([]byte(raw), &ev); err != nil {
		return strings.TrimSpace(raw)
	}

	switch ev.Type {
	case "assistant":
		if ev.Message == nil {
			return ""
		}
		var parts []string
		for _, c := range ev.Message.Content {
			switch c.Type {
			case "text":
				if t := strings.TrimSpace(c.Text); t != "" {
					parts = append(parts, t)
				}
			case "tool_use":
				parts = append(parts, toolLabel(c.Name, c.Input))
			}
		}
		return strings.Join(parts, "\n")

	case "tool_result":
		text := contentText(ev.Content)
		if text == "" {
			return ""
		}
		const maxLen = 400
		if len(text) > maxLen {
			text = text[:maxLen] + "..."
		}
		lines := strings.Split(strings.TrimSpace(text), "\n")
		for i, l := range lines {
			lines[i] = "  " + l
		}
		return strings.Join(lines, "\n")

	case "result":
		if ev.Subtype == "success" {
			return "complete"
		}
		return ""

	case "system":
		return ""
	}

	return ""
}

// extractToolInput extracts the primary input field from a tool invocation.
// Used by both toolLabel (for display) and RunStreamingWithTools (for events).
func extractToolInput(name string, inputRaw json.RawMessage) string {
	var input map[string]interface{}
	if inputRaw != nil {
		_ = json.Unmarshal(inputRaw, &input)
	}

	detail := ""
	switch name {
	case "Read":
		detail = strField(input, "file_path")
	case "Write":
		detail = strField(input, "file_path")
	case "Edit":
		detail = strField(input, "file_path")
	case "Bash":
		detail = strField(input, "command")
	case "Glob":
		detail = strField(input, "pattern")
	case "Grep":
		p := strField(input, "pattern")
		path := strField(input, "path")
		if path != "" {
			detail = p + " in " + path
		} else {
			detail = p
		}
	default:
		for _, v := range input {
			if s, ok := v.(string); ok {
				detail = s
				break
			}
		}
	}
	return detail
}

func toolLabel(name string, inputRaw json.RawMessage) string {
	detail := extractToolInput(name, inputRaw)
	if detail == "" {
		return "-> " + name
	}
	const maxDetail = 100
	if len(detail) > maxDetail {
		detail = detail[:maxDetail] + "..."
	}
	return fmt.Sprintf("-> %s(%s)", name, detail)
}

func strField(m map[string]interface{}, key string) string {
	if m == nil {
		return ""
	}
	v, ok := m[key]
	if !ok {
		return ""
	}
	s, _ := v.(string)
	return s
}

func contentText(raw json.RawMessage) string {
	if raw == nil {
		return ""
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return strings.TrimSpace(s)
	}
	var blocks []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if err := json.Unmarshal(raw, &blocks); err == nil {
		var parts []string
		for _, b := range blocks {
			if b.Type == "text" && b.Text != "" {
				parts = append(parts, strings.TrimSpace(b.Text))
			}
		}
		return strings.Join(parts, "\n")
	}
	return ""
}
