# Implementing Custom Backends

The `backend.Backend` interface abstracts LLM provider interactions. Implementing a custom backend allows the engine to support new providers without modifying core orchestrator logic.

## Backend Interface

```go
// pkg/agent/backend/backend.go
type Backend interface {
    Run(ctx context.Context, systemPrompt, userPrompt, model string) (string, error)
    RunStreaming(ctx context.Context, systemPrompt, userPrompt, model string, onChunk ChunkCallback) (string, error)
    RunStreamingWithTools(ctx context.Context, systemPrompt, userPrompt, model string, onChunk ChunkCallback, onToolCall ToolCallCallback) (string, error)
}

type ChunkCallback func(chunk string)
type ToolCallCallback func(name string, input map[string]interface{}) (string, error)
```

### Method Responsibilities

**`Run`** — Non-streaming request/response.
- Send system + user prompts to LLM
- Wait for full response
- Return complete text

**`RunStreaming`** — Streaming text output (no tools).
- Send prompts, receive SSE stream
- Call `onChunk` for each text delta
- Return full accumulated text

**`RunStreamingWithTools`** — Streaming with tool use loop.
- Send prompts + tool definitions
- Receive stream of text/tool_use
- Call `onChunk` for text deltas
- Call `onToolCall` for tool invocations
- Append tool results to message history
- Loop until LLM returns stop reason
- Return final text

## Example: OpenAI-Compatible Backend

```go
package openai

import (
    "bytes"
    "context"
    "encoding/json"
    "fmt"
    "net/http"
    "strings"

    "github.com/blackwell-systems/scout-and-wave-go/pkg/agent/backend"
)

type Client struct {
    apiKey  string
    baseURL string
    cfg     backend.Config
}

func New(cfg backend.Config) *Client {
    baseURL := cfg.BaseURL
    if baseURL == "" {
        baseURL = "https://api.openai.com/v1"
    }
    return &Client{
        apiKey:  cfg.APIKey,
        baseURL: baseURL,
        cfg:     cfg,
    }
}

func (c *Client) Run(ctx context.Context, systemPrompt, userPrompt, model string) (string, error) {
    reqBody := map[string]interface{}{
        "model": model,
        "messages": []map[string]string{
            {"role": "system", "content": systemPrompt},
            {"role": "user", "content": userPrompt},
        },
    }

    bodyBytes, _ := json.Marshal(reqBody)
    req, _ := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/chat/completions", bytes.NewReader(bodyBytes))
    req.Header.Set("Authorization", "Bearer "+c.apiKey)
    req.Header.Set("Content-Type", "application/json")

    resp, err := http.DefaultClient.Do(req)
    if err != nil {
        return "", fmt.Errorf("openai: request failed: %w", err)
    }
    defer resp.Body.Close()

    var result struct {
        Choices []struct {
            Message struct {
                Content string `json:"content"`
            } `json:"message"`
        } `json:"choices"`
    }
    json.NewDecoder(resp.Body).Decode(&result)

    if len(result.Choices) == 0 {
        return "", fmt.Errorf("openai: no choices in response")
    }

    return result.Choices[0].Message.Content, nil
}

func (c *Client) RunStreaming(ctx context.Context, systemPrompt, userPrompt, model string, onChunk backend.ChunkCallback) (string, error) {
    // Similar to Run, but with "stream": true and SSE parsing
    // Call onChunk(delta) for each content delta
    // Return accumulated text
}

func (c *Client) RunStreamingWithTools(ctx context.Context, systemPrompt, userPrompt, model string, onChunk backend.ChunkCallback, onToolCall backend.ToolCallCallback) (string, error) {
    // Build messages with tool definitions
    // Loop: send request, handle tool_calls, append results
    // Break when finish_reason == "stop"
}
```

## Tool Serialization

Each backend must serialize tools into its native format.

### Anthropic Messages API

```go
tools := []map[string]interface{}{
    {
        "name":         "Read",
        "description":  "Read a file from the repository",
        "input_schema": map[string]interface{}{
            "type": "object",
            "properties": map[string]interface{}{
                "path": map[string]interface{}{"type": "string"},
            },
            "required": []string{"path"},
        },
    },
}
```

### OpenAI-Compatible

```go
tools := []map[string]interface{}{
    {
        "type": "function",
        "function": map[string]interface{}{
            "name":        "Read",
            "description": "Read a file from the repository",
            "parameters": map[string]interface{}{
                "type": "object",
                "properties": map[string]interface{}{
                    "path": map[string]interface{}{"type": "string"},
                },
                "required": []string{"path"},
            },
        },
    },
}
```

> **Note:** After tool system refactoring, serialization will be handled by backend adapters (see `docs/tools.md`). Current backends inline the serialization logic.

## Tool Call Loop

The `RunStreamingWithTools` method implements the agentic loop:

```go
messages := []map[string]interface{}{
    {"role": "system", "content": systemPrompt},
    {"role": "user", "content": userPrompt},
}

for turn := 0; turn < maxTurns; turn++ {
    // Send request
    resp := callAPI(messages, tools)

    if resp.FinishReason == "stop" {
        // LLM finished with text response
        return resp.Content, nil
    }

    if resp.FinishReason == "tool_calls" {
        // Execute tools and append results
        for _, toolCall := range resp.ToolCalls {
            result, err := onToolCall(toolCall.Name, toolCall.Input)
            messages = append(messages, map[string]interface{}{
                "role":         "tool",
                "tool_call_id": toolCall.ID,
                "content":      result,
            })
        }
        // Loop continues — send updated messages back to LLM
    }
}

return "", fmt.Errorf("exceeded max turns")
```

## Provider-Specific Quirks

### Anthropic API

- Uses `tool_use` content blocks (not top-level `tool_calls`)
- Requires `tool_result` content blocks in user message
- Streaming: parse SSE with `content_block_delta` events

### OpenAI-Compatible

- Uses top-level `tool_calls` array in assistant message
- Requires `tool` role messages with `tool_call_id`
- Some providers (Ollama, LM Studio) embed tool calls in `content` as JSON string instead of `tool_calls` array (see content-mode fallback in `pkg/agent/backend/openai/client.go`)

### AWS Bedrock

- Uses Anthropic Messages API format with `bedrock-2023-05-31` version
- Requires AWS SDK v2 with credentials from default chain
- Model IDs must be inference profile IDs (e.g., `us.anthropic.claude-sonnet-4-5-20250929-v1:0`)
- Streaming via `InvokeModelWithResponseStream` with event loop

### CLI Backend

- Spawns `claude` CLI subprocess
- No tool serialization needed (CLI handles it internally)
- Streams stdout line-by-line
- Cannot customize tool behavior (CLI provides fixed tool set)

## Provider Routing

The orchestrator routes model names to backends via provider prefixes:

```go
// pkg/orchestrator/orchestrator.go
func newBackendFunc(cfg BackendConfig) (backend.Backend, error) {
    provider, bareModel := parseProviderPrefix(cfg.Model)

    switch provider {
    case "anthropic":
        return apiclient.New(apiKey, backend.Config{Model: bareModel})
    case "bedrock":
        fullID := expandBedrockModelID(bareModel)
        return bedrockbackend.New(backend.Config{Model: fullID})
    case "openai":
        return openaibackend.New(backend.Config{
            Model:   bareModel,
            APIKey:  cfg.OpenAIKey,
            BaseURL: cfg.BaseURL,
        })
    case "ollama":
        return openaibackend.New(backend.Config{
            Model:   bareModel,
            BaseURL: "http://localhost:11434/v1",
        })
    case "lmstudio":
        return openaibackend.New(backend.Config{
            Model:   bareModel,
            BaseURL: "http://localhost:1234/v1",
        })
    case "cli":
        return cliclient.New(binaryPath, backend.Config{Model: bareModel})
    default:
        // No prefix — default to Anthropic API
        return apiclient.New(apiKey, backend.Config{Model: cfg.Model})
    }
}
```

## Adding a New Backend

1. Create package `pkg/agent/backend/<provider>/`
2. Implement `backend.Backend` interface
3. Handle tool serialization for provider's API format
4. Implement tool call loop in `RunStreamingWithTools`
5. Add provider case to `newBackendFunc` in `pkg/orchestrator/orchestrator.go`
6. Update `parseProviderPrefix` to recognize new prefix

**Example:**

```go
// pkg/agent/backend/groq/client.go
package groq

import "github.com/blackwell-systems/scout-and-wave-go/pkg/agent/backend"

type Client struct {
    apiKey string
}

func New(cfg backend.Config) *Client {
    return &Client{apiKey: cfg.APIKey}
}

func (c *Client) Run(ctx context.Context, systemPrompt, userPrompt, model string) (string, error) {
    // Groq uses OpenAI-compatible API
    // Call https://api.groq.com/openai/v1/chat/completions
}

// Implement RunStreaming and RunStreamingWithTools...
```

Then in orchestrator:

```go
case "groq":
    return groqbackend.New(backend.Config{
        Model:   bareModel,
        APIKey:  cfg.GroqAPIKey,
    })
```

## Testing Backends

Mock the `Backend` interface for unit tests:

```go
type MockBackend struct {
    responses []string
    callCount int
}

func (m *MockBackend) RunStreamingWithTools(ctx context.Context, systemPrompt, userPrompt, model string, onChunk backend.ChunkCallback, onToolCall backend.ToolCallCallback) (string, error) {
    response := m.responses[m.callCount]
    m.callCount++

    // Simulate streaming
    for _, chunk := range strings.Split(response, " ") {
        onChunk(chunk + " ")
    }

    return response, nil
}

// Test
func TestAgentExecution(t *testing.T) {
    mock := &MockBackend{
        responses: []string{
            "Let me read the file",
            "Now I will write the changes",
            "Done",
        },
    }

    agent := runner.New(mock)
    result, err := agent.ExecuteStreaming(ctx, prompt)
    assert.NoError(t, err)
    assert.Equal(t, 3, mock.callCount)
}
```

## See Also

- [Architecture Overview](architecture.md) — Backend role in the engine
- [Tool System](tools.md) — Tool call loop and backend adapters
- [API Endpoints](api-endpoints.md) — Model name format and provider prefixes
