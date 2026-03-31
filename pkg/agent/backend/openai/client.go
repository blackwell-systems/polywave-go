package openai

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/agent/backend"
	"github.com/blackwell-systems/scout-and-wave-go/pkg/agent/dedup"
	"github.com/blackwell-systems/scout-and-wave-go/pkg/tools"
)

const (
	defaultModel     = "gpt-4o"
	defaultMaxTokens = 4096
	defaultMaxTurns  = 50
	defaultBaseURL   = "https://api.openai.com/v1"
)

// Client is an OpenAI-compatible backend. It implements backend.Backend.
type Client struct {
	apiKey     string
	model      string
	baseURL    string
	maxTokens  int
	maxTurns   int
	onToolCall backend.ToolCallCallback
	readOnly   bool
	dedupCache *dedup.Cache
}

// New creates a new Client configured from cfg.
// APIKey is read from cfg.APIKey then OPENAI_API_KEY env var.
// BaseURL defaults to "https://api.openai.com/v1".
// Model defaults to "gpt-4o"; MaxTokens to 4096; MaxTurns to 50.
func New(cfg backend.Config) *Client {
	apiKey := cfg.APIKey
	if apiKey == "" {
		apiKey = os.Getenv("OPENAI_API_KEY")
	}

	model := cfg.Model
	if model == "" {
		model = defaultModel
	}
	maxTokens := cfg.MaxTokens
	if maxTokens <= 0 {
		maxTokens = defaultMaxTokens
	}
	maxTurns := cfg.MaxTurns
	if maxTurns <= 0 {
		maxTurns = defaultMaxTurns
	}
	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	return &Client{
		apiKey:     apiKey,
		model:      model,
		baseURL:    baseURL,
		maxTokens:  maxTokens,
		maxTurns:   maxTurns,
		onToolCall: cfg.OnToolCall,
		readOnly:   cfg.ReadOnly,
	}
}

// buildWorkshop creates a Workshop with middleware applied based on client config.
func (c *Client) buildWorkshop(workDir string) tools.Workshop {
	var w tools.Workshop
	if c.readOnly {
		w = tools.ReadOnlyTools(workDir)
	} else {
		w = tools.StandardTools(workDir)
	}
	w, c.dedupCache = dedup.WithDedup(w)
	if c.onToolCall != nil {
		w = tools.WithTiming(w, func(ev tools.ToolCallEvent) {
			c.onToolCall(backend.ToolCallEvent{
				Name:       ev.ToolName,
				DurationMs: ev.DurationMs,
				IsError:    ev.IsError,
				IsResult:   true,
			})
		})
	}
	return w
}

// WithAPIKey sets the API key. Returns c for chaining.
func (c *Client) WithAPIKey(key string) *Client {
	c.apiKey = key
	return c
}

// WithBaseURL overrides the API endpoint. Used in tests to point at a mock server.
func (c *Client) WithBaseURL(url string) *Client {
	c.baseURL = url
	return c
}

// DedupStats returns dedup metrics from the most recent Run call.
// Returns nil if no run has been made yet.
func (c *Client) DedupStats() *dedup.Stats {
	if c.dedupCache == nil {
		return nil
	}
	stats := c.dedupCache.Stats()
	return &stats
}

// buildToolDefs converts Workshop tools to OpenAI function calling format.
func buildToolDefs(workshop tools.Workshop) []map[string]interface{} {
	allTools := workshop.All()
	defs := make([]map[string]interface{}, 0, len(allTools))
	for _, t := range allTools {
		defs = append(defs, map[string]interface{}{
			"type": "function",
			"function": map[string]interface{}{
				"name":        t.Name,
				"description": t.Description,
				"parameters":  t.InputSchema,
			},
		})
	}
	return defs
}

// toolNameSet returns a set of registered tool names for content-mode fallback detection.
func toolNameSet(workshop tools.Workshop) map[string]bool {
	allTools := workshop.All()
	set := make(map[string]bool, len(allTools))
	for _, t := range allTools {
		set[t.Name] = true
	}
	return set
}

// Run executes the agent described by systemPrompt and userMessage.
// It runs a tool-use loop until finish_reason == "stop" or maxTurns is exceeded.
// Run implements backend.Backend.
func (c *Client) Run(ctx context.Context, systemPrompt, userMessage, workDir string) (string, error) {
	workshop := c.buildWorkshop(workDir)
	toolDefs := buildToolDefs(workshop)
	nameSet := toolNameSet(workshop)

	messages := buildInitialMessages(systemPrompt, userMessage)

	for turn := 0; turn < c.maxTurns; turn++ {
		resp, err := c.chatCompletion(ctx, messages, toolDefs, false)
		if err != nil {
			return "", fmt.Errorf("openai backend: API error (turn %d): %w", turn, err)
		}

		if len(resp.Choices) == 0 {
			return "", fmt.Errorf("openai backend: no choices in response (turn %d)", turn)
		}
		choice := resp.Choices[0]

		switch choice.FinishReason {
		case "stop":
			// Content-mode tool call fallback: some local models (e.g. Qwen via Ollama)
			// embed the tool call as JSON in content instead of using the tool_calls array.
			if ctc := parseContentToolCall(choice.Message.Content, nameSet); ctc != nil {
				messages = append(messages, chatMessage{Role: "assistant", Content: choice.Message.Content})
				result, _ := backend.ExecuteTool(ctx, workshop, ctc.Name, ctc.Arguments, workDir)
				messages = append(messages, chatMessage{Role: "user", Content: "Function result:\n" + result})
				continue
			}
			return choice.Message.Content, nil

		case "tool_calls":
			messages = append(messages, assistantMessage(choice.Message))
			for _, tc := range choice.Message.ToolCalls {
				var inputMap map[string]interface{}
				if err := json.Unmarshal([]byte(tc.Function.Arguments), &inputMap); err != nil {
					inputMap = map[string]interface{}{}
				}
				result, _ := backend.ExecuteTool(ctx, workshop, tc.Function.Name, inputMap, workDir)
				messages = append(messages, toolResultMessage(tc.ID, result))
			}

		default:
			return "", fmt.Errorf("openai backend: unexpected finish_reason %q (turn %d)", choice.FinishReason, turn)
		}
	}

	return "", fmt.Errorf("openai backend: tool use loop exceeded maxTurns (%d)", c.maxTurns)
}

// RunStreaming implements backend.Backend.
// For tool-call turns the loop runs non-streaming (identical to Run).
// For the final stop turn it uses the streaming API and calls onChunk per content delta.
// If onChunk is nil, RunStreaming delegates to Run.
func (c *Client) RunStreaming(ctx context.Context, systemPrompt, userMessage, workDir string, onChunk backend.ChunkCallback) (string, error) {
	if onChunk == nil {
		return c.Run(ctx, systemPrompt, userMessage, workDir)
	}

	workshop := c.buildWorkshop(workDir)
	toolDefs := buildToolDefs(workshop)
	nameSet := toolNameSet(workshop)

	messages := buildInitialMessages(systemPrompt, userMessage)

	for turn := 0; turn < c.maxTurns; turn++ {
		resp, err := c.chatCompletion(ctx, messages, toolDefs, false)
		if err != nil {
			return "", fmt.Errorf("openai backend: API error (turn %d): %w", turn, err)
		}

		if len(resp.Choices) == 0 {
			return "", fmt.Errorf("openai backend: no choices in response (turn %d)", turn)
		}
		choice := resp.Choices[0]

		switch choice.FinishReason {
		case "stop":
			if ctc := parseContentToolCall(choice.Message.Content, nameSet); ctc != nil {
				messages = append(messages, chatMessage{Role: "assistant", Content: choice.Message.Content})
				result, _ := backend.ExecuteTool(ctx, workshop, ctc.Name, ctc.Arguments, workDir)
				messages = append(messages, chatMessage{Role: "user", Content: "Function result:\n" + result})
				continue
			}
			return c.streamFinalTurn(ctx, messages, toolDefs, onChunk)

		case "tool_calls":
			messages = append(messages, assistantMessage(choice.Message))
			for _, tc := range choice.Message.ToolCalls {
				var inputMap map[string]interface{}
				if err := json.Unmarshal([]byte(tc.Function.Arguments), &inputMap); err != nil {
					inputMap = map[string]interface{}{}
				}
				result, _ := backend.ExecuteTool(ctx, workshop, tc.Function.Name, inputMap, workDir)
				messages = append(messages, toolResultMessage(tc.ID, result))
			}

		default:
			return "", fmt.Errorf("openai backend: unexpected finish_reason %q (turn %d)", choice.FinishReason, turn)
		}
	}

	return "", fmt.Errorf("openai backend: tool use loop exceeded maxTurns (%d)", c.maxTurns)
}

// RunStreamingWithTools implements backend.Backend.
// OpenAI backend does not yet support tool call event streaming, so this
// delegates to RunStreaming (no-op for onToolCall).
func (c *Client) RunStreamingWithTools(ctx context.Context, systemPrompt, userMessage, workDir string, onChunk backend.ChunkCallback, onToolCall backend.ToolCallCallback) (string, error) {
	return c.RunStreaming(ctx, systemPrompt, userMessage, workDir, onChunk)
}

// streamFinalTurn issues a streaming chat completion and calls onChunk for each delta.
func (c *Client) streamFinalTurn(ctx context.Context, messages []chatMessage, toolDefs []map[string]interface{}, onChunk backend.ChunkCallback) (string, error) {
	body := c.buildRequestBody(messages, toolDefs, true)
	reqData, err := json.Marshal(body)
	if err != nil {
		return "", fmt.Errorf("openai backend: marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/chat/completions", bytes.NewReader(reqData))
	if err != nil {
		return "", fmt.Errorf("openai backend: create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	httpResp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("openai backend: HTTP request: %w", err)
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(httpResp.Body)
		return "", fmt.Errorf("openai backend: HTTP %d: %s", httpResp.StatusCode, string(body))
	}

	var sb strings.Builder
	scanner := newSSEScanner(httpResp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			break
		}
		var chunk streamChunk
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue
		}
		if len(chunk.Choices) == 0 {
			continue
		}
		delta := chunk.Choices[0].Delta.Content
		if delta != "" {
			sb.WriteString(delta)
			onChunk(delta)
		}
	}
	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("openai backend: reading stream: %w", err)
	}
	return sb.String(), nil
}

// --- HTTP request helpers ---

func (c *Client) chatCompletion(ctx context.Context, messages []chatMessage, toolDefs []map[string]interface{}, stream bool) (*chatCompletionResponse, error) {
	body := c.buildRequestBody(messages, toolDefs, stream)
	reqData, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("openai backend: marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/chat/completions", bytes.NewReader(reqData))
	if err != nil {
		return nil, fmt.Errorf("openai backend: create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("openai backend: HTTP request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("openai backend: HTTP %d: %s", resp.StatusCode, string(body))
	}

	var result chatCompletionResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("openai backend: decode response: %w", err)
	}
	return &result, nil
}

func (c *Client) buildRequestBody(messages []chatMessage, toolDefs []map[string]interface{}, stream bool) map[string]interface{} {
	body := map[string]interface{}{
		"model":       c.model,
		"max_tokens":  c.maxTokens,
		"messages":    messages,
		"tools":       toolDefs,
		"tool_choice": "auto",
	}
	if stream {
		body["stream"] = true
	}
	return body
}

// --- Content-mode tool call fallback ---

type contentToolCallJSON struct {
	Name      string                 `json:"name"`
	Arguments map[string]interface{} `json:"arguments"`
}

// parseContentToolCall detects content-mode tool calls (used by local models like Qwen via Ollama).
func parseContentToolCall(content string, nameSet map[string]bool) *contentToolCallJSON {
	content = strings.TrimSpace(content)
	if !strings.HasPrefix(content, "{") {
		return nil
	}
	var tc contentToolCallJSON
	if err := json.Unmarshal([]byte(content), &tc); err != nil {
		return nil
	}
	if tc.Name == "" {
		return nil
	}
	if !nameSet[tc.Name] {
		return nil
	}
	return &tc
}

// --- Message construction helpers ---

type chatMessage struct {
	Role       string     `json:"role"`
	Content    string     `json:"content,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
	ToolCalls  []toolCall `json:"tool_calls,omitempty"`
}

type toolCall struct {
	ID       string       `json:"id"`
	Type     string       `json:"type"`
	Function toolFunction `json:"function"`
}

type toolFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type chatCompletionResponse struct {
	Choices []struct {
		FinishReason string `json:"finish_reason"`
		Message      struct {
			Role      string     `json:"role"`
			Content   string     `json:"content"`
			ToolCalls []toolCall `json:"tool_calls"`
		} `json:"message"`
	} `json:"choices"`
}

type streamChunk struct {
	Choices []struct {
		Delta struct {
			Content string `json:"content"`
		} `json:"delta"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
}

func buildInitialMessages(systemPrompt, userMessage string) []chatMessage {
	var msgs []chatMessage
	if systemPrompt != "" {
		msgs = append(msgs, chatMessage{Role: "system", Content: systemPrompt})
	}
	msgs = append(msgs, chatMessage{Role: "user", Content: userMessage})
	return msgs
}

func assistantMessage(msg struct {
	Role      string     `json:"role"`
	Content   string     `json:"content"`
	ToolCalls []toolCall `json:"tool_calls"`
}) chatMessage {
	return chatMessage{
		Role:      "assistant",
		Content:   msg.Content,
		ToolCalls: msg.ToolCalls,
	}
}

func toolResultMessage(toolCallID, content string) chatMessage {
	return chatMessage{
		Role:       "tool",
		ToolCallID: toolCallID,
		Content:    content,
	}
}

func newSSEScanner(r io.Reader) *bufioScanner {
	return &bufioScanner{s: bufio.NewScanner(r)}
}

type bufioScanner struct {
	s *bufio.Scanner
}

func (b *bufioScanner) Scan() bool  { return b.s.Scan() }
func (b *bufioScanner) Text() string { return b.s.Text() }
func (b *bufioScanner) Err() error   { return b.s.Err() }
