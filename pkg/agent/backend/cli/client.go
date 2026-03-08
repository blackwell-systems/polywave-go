package cli

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"

	"github.com/creack/pty"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/agent/backend"
)

// ansiRE matches ANSI/VT100 escape sequences emitted by a PTY-connected process.
var ansiRE = regexp.MustCompile(`\x1b(?:[@-Z\\-_]|\[[0-?]*[ -/]*[@-~])`)

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

// Run implements backend.Backend.
func (c *Client) Run(ctx context.Context, systemPrompt, userMessage, workDir string) (string, error) {
	return c.RunStreaming(ctx, systemPrompt, userMessage, workDir, nil)
}

// RunStreaming implements backend.Backend.
// Uses --output-format stream-json inside a PTY so Node.js line-buffers output
// (enabling real-time streaming). The PTY is set to 65535 columns to prevent
// line-wrapping artifacts in JSON, and the scanner accumulates PTY-wrapped
// fragments until a complete JSON object is assembled before parsing.
func (c *Client) RunStreaming(ctx context.Context, systemPrompt, userMessage, workDir string, onChunk backend.ChunkCallback) (string, error) {
	claudePath := c.claudePath
	if claudePath == "" {
		var err error
		claudePath, err = exec.LookPath("claude")
		if err != nil {
			return "", fmt.Errorf("cli backend: claude binary not found in PATH: %w", err)
		}
	}

	var prompt string
	if systemPrompt == "" {
		prompt = userMessage
	} else {
		prompt = systemPrompt + "\n\n" + userMessage
	}

	args := []string{
		"--print",
		"--output-format", "stream-json",
		"--allowedTools", "Bash,Read,Write,Edit,Glob,Grep",
		"--dangerously-skip-permissions",
	}
	if c.cfg.MaxTurns > 0 {
		args = append(args, "--max-turns", fmt.Sprintf("%d", c.cfg.MaxTurns))
	}
	args = append(args, "-p", prompt)

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

	var sb strings.Builder
	scanner := bufio.NewScanner(ptmx)
	// 1 MB scanner buffer for large tool-result lines.
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)

	// pending accumulates PTY-wrapped fragments until we have a full JSON object.
	var pending strings.Builder

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
			// Incomplete — keep accumulating (PTY wrapped mid-JSON).
			continue
		}

		// Complete JSON object — process it.
		pending.Reset()
		sb.WriteString(candidate + "\n")

		if onChunk != nil {
			if formatted := formatStreamEvent(candidate); formatted != "" {
				onChunk(formatted + "\n")
			}
		}
	}

	if scanErr := scanner.Err(); scanErr != nil {
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
			return "", fmt.Errorf("cli backend: claude exited with code %d", exitErr.ExitCode())
		}
		return "", fmt.Errorf("cli backend: claude failed: %w", err)
	}

	return sb.String(), nil
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

func toolLabel(name string, inputRaw json.RawMessage) string {
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
