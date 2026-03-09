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
)

const (
	defaultModel     = "gpt-4o"
	defaultMaxTokens = 4096
	defaultMaxTurns  = 50
	defaultBaseURL   = "https://api.openai.com/v1"
)

// Client is an OpenAI-compatible backend. It implements backend.Backend.
type Client struct {
	apiKey    string
	model     string
	baseURL   string
	maxTokens int
	maxTurns  int
}

// New creates a new Client configured from cfg.
// APIKey is read from cfg.APIKey (if the field exists) then OPENAI_API_KEY env var.
// BaseURL defaults to "https://api.openai.com/v1".
// Model defaults to "gpt-4o"; MaxTokens to 4096; MaxTurns to 50.
func New(cfg backend.Config) *Client {
	apiKey := os.Getenv("OPENAI_API_KEY")

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
	return &Client{
		apiKey:    apiKey,
		model:     model,
		baseURL:   defaultBaseURL,
		maxTokens: maxTokens,
		maxTurns:  maxTurns,
	}
}

// WithAPIKey sets the API key. Returns c for chaining. Used in tests and
// to support cfg.APIKey once Wave 2 adds it to backend.Config.
func (c *Client) WithAPIKey(key string) *Client {
	c.apiKey = key
	return c
}

// WithBaseURL overrides the API endpoint. Used in tests to point at a mock server.
func (c *Client) WithBaseURL(url string) *Client {
	c.baseURL = url
	return c
}

// Run executes the agent described by systemPrompt and userMessage.
// It runs a tool-use loop until finish_reason == "stop" or maxTurns is exceeded.
// Run implements backend.Backend.
func (c *Client) Run(ctx context.Context, systemPrompt, userMessage, workDir string) (string, error) {
	tools := standardTools(workDir)
	toolMap := buildToolMap(tools)

	// Build initial messages.
	messages := buildInitialMessages(systemPrompt, userMessage)

	for turn := 0; turn < c.maxTurns; turn++ {
		resp, err := c.chatCompletion(ctx, messages, tools, false)
		if err != nil {
			return "", fmt.Errorf("openai backend: API error (turn %d): %w", turn, err)
		}

		if len(resp.Choices) == 0 {
			return "", fmt.Errorf("openai backend: no choices in response (turn %d)", turn)
		}
		choice := resp.Choices[0]

		switch choice.FinishReason {
		case "stop":
			return choice.Message.Content, nil

		case "tool_calls":
			// Append the assistant message.
			messages = append(messages, assistantMessage(choice.Message))

			// Execute all tool calls and append results.
			for _, tc := range choice.Message.ToolCalls {
				var inputMap map[string]interface{}
				if err := json.Unmarshal([]byte(tc.Function.Arguments), &inputMap); err != nil {
					inputMap = map[string]interface{}{}
				}
				result := executeTool(toolMap, tc.Function.Name, inputMap, workDir)
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

	tools := standardTools(workDir)
	toolMap := buildToolMap(tools)

	messages := buildInitialMessages(systemPrompt, userMessage)

	for turn := 0; turn < c.maxTurns; turn++ {
		// Use non-streaming to get the full response for tool-call turns.
		// We always do a non-streaming call first, then if it was "stop" we
		// re-issue as streaming to deliver chunks. This is the simplest correct
		// approach that avoids buffering streaming tool-call inputs.
		resp, err := c.chatCompletion(ctx, messages, tools, false)
		if err != nil {
			return "", fmt.Errorf("openai backend: API error (turn %d): %w", turn, err)
		}

		if len(resp.Choices) == 0 {
			return "", fmt.Errorf("openai backend: no choices in response (turn %d)", turn)
		}
		choice := resp.Choices[0]

		switch choice.FinishReason {
		case "stop":
			// Re-issue as streaming so onChunk receives fragments.
			return c.streamFinalTurn(ctx, messages, tools, onChunk)

		case "tool_calls":
			messages = append(messages, assistantMessage(choice.Message))
			for _, tc := range choice.Message.ToolCalls {
				var inputMap map[string]interface{}
				if err := json.Unmarshal([]byte(tc.Function.Arguments), &inputMap); err != nil {
					inputMap = map[string]interface{}{}
				}
				result := executeTool(toolMap, tc.Function.Name, inputMap, workDir)
				messages = append(messages, toolResultMessage(tc.ID, result))
			}

		default:
			return "", fmt.Errorf("openai backend: unexpected finish_reason %q (turn %d)", choice.FinishReason, turn)
		}
	}

	return "", fmt.Errorf("openai backend: tool use loop exceeded maxTurns (%d)", c.maxTurns)
}

// streamFinalTurn issues a streaming chat completion and calls onChunk for each delta.
// Returns the full concatenated text.
func (c *Client) streamFinalTurn(ctx context.Context, messages []chatMessage, tools []tool, onChunk backend.ChunkCallback) (string, error) {
	body := c.buildRequestBody(messages, tools, true)
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

// chatCompletion sends a POST /chat/completions request and decodes the response.
func (c *Client) chatCompletion(ctx context.Context, messages []chatMessage, tools []tool, stream bool) (*chatCompletionResponse, error) {
	body := c.buildRequestBody(messages, tools, stream)
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

func (c *Client) buildRequestBody(messages []chatMessage, tools []tool, stream bool) map[string]interface{} {
	toolDefs := make([]map[string]interface{}, 0, len(tools))
	for _, t := range tools {
		toolDefs = append(toolDefs, map[string]interface{}{
			"type": "function",
			"function": map[string]interface{}{
				"name":        t.Name,
				"description": t.Description,
				"parameters":  t.Parameters,
			},
		})
	}

	body := map[string]interface{}{
		"model":      c.model,
		"max_tokens": c.maxTokens,
		"messages":   messages,
		"tools":      toolDefs,
		"tool_choice": "auto",
	}
	if stream {
		body["stream"] = true
	}
	return body
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

// streamChunk is one SSE event from a streaming completion.
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

// newSSEScanner returns a line scanner over r suitable for SSE streams.
func newSSEScanner(r io.Reader) *bufioScanner {
	return &bufioScanner{s: bufio.NewScanner(r)}
}

type bufioScanner struct {
	s *bufio.Scanner
}

func (b *bufioScanner) Scan() bool  { return b.s.Scan() }
func (b *bufioScanner) Text() string { return b.s.Text() }
func (b *bufioScanner) Err() error   { return b.s.Err() }
