# Custom Backend Example

This example shows how to implement the `backend.Backend` interface for a new LLM provider.

## Overview

The example implements a backend for **Groq** (OpenAI-compatible API with fast inference).

## Files

- `main.go` — Full implementation of `backend.Backend` for Groq
- `go.mod` — Dependencies

## Implementation

### 1. Create Backend Struct

```go
package main

import (
    "context"
    "github.com/blackwell-systems/polywave-go/pkg/agent/backend"
)

type GroqBackend struct {
    apiKey  string
    baseURL string
}

func NewGroqBackend(apiKey string) *GroqBackend {
    return &GroqBackend{
        apiKey:  apiKey,
        baseURL: "https://api.groq.com/openai/v1",
    }
}
```

### 2. Implement Run Method

```go
func (g *GroqBackend) Run(ctx context.Context, systemPrompt, userPrompt, model string) (string, error) {
    // Send POST /chat/completions request
    // Parse response
    // Return text
}
```

### 3. Implement RunStreaming

```go
func (g *GroqBackend) RunStreaming(ctx context.Context, systemPrompt, userPrompt, model string, onChunk backend.ChunkCallback) (string, error) {
    // Send POST /chat/completions with "stream": true
    // Parse SSE stream
    // Call onChunk(delta) for each text delta
    // Return full accumulated text
}
```

### 4. Implement RunStreamingWithTools

```go
func (g *GroqBackend) RunStreamingWithTools(ctx context.Context, systemPrompt, userPrompt, model string, onChunk backend.ChunkCallback, onToolCall backend.ToolCallCallback) (string, error) {
    messages := []map[string]interface{}{
        {"role": "system", "content": systemPrompt},
        {"role": "user", "content": userPrompt},
    }

    for turn := 0; turn < maxTurns; turn++ {
        resp := g.callAPI(ctx, messages, tools)

        if resp.FinishReason == "stop" {
            return resp.Content, nil
        }

        if resp.FinishReason == "tool_calls" {
            for _, toolCall := range resp.ToolCalls {
                result, err := onToolCall(toolCall.Name, toolCall.Input)
                if err != nil {
                    return "", err
                }
                messages = append(messages, map[string]interface{}{
                    "role":         "tool",
                    "tool_call_id": toolCall.ID,
                    "content":      result,
                })
            }
        }
    }

    return "", fmt.Errorf("exceeded max turns")
}
```

### 5. Use in Orchestrator

```go
// In pkg/orchestrator/orchestrator.go
case "groq":
    return NewGroqBackend(cfg.GroqAPIKey), nil
```

## Running

```bash
export GROQ_API_KEY="your-groq-api-key"
go run main.go
```

## TODO

- [ ] Implement `main.go` with full Groq backend
- [ ] Add tool serialization (OpenAI-compatible format)
- [ ] Add streaming SSE parser
- [ ] Add tool call loop
- [ ] Test with Polywave agents
