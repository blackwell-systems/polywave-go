# Custom Tool Example

This example shows how to register a custom tool with the `ToolRegistry` (after tool system refactoring).

## Overview

The example implements a **vector database search** tool that agents can use to find similar code snippets.

## Files

- `main.go` — Full implementation of vector DB tool with registry registration
- `go.mod` — Dependencies

## Implementation

### 1. Create Tool Executor

```go
package main

import (
    "context"
    "fmt"
    "github.com/blackwell-systems/polywave-go/pkg/agent/tools"
)

type VectorDBExecutor struct {
    client *VectorDBClient // Your vector DB client
}

func (e *VectorDBExecutor) Execute(ctx context.Context, input map[string]interface{}) (string, error) {
    query := input["query"].(string)
    limit := int(input["limit"].(float64))

    results, err := e.client.Search(ctx, query, limit)
    if err != nil {
        return "", fmt.Errorf("vector search failed: %w", err)
    }

    var output strings.Builder
    for i, result := range results {
        fmt.Fprintf(&output, "%d. %s (score: %.2f)\n", i+1, result.Snippet, result.Score)
    }
    return output.String(), nil
}
```

### 2. Register Tool

```go
func main() {
    registry := tools.NewRegistry()

    // Register standard tools
    registry.Register(tools.Read(workDir))
    registry.Register(tools.Write(workDir))
    // ...

    // Register custom tool
    registry.Register(tools.Tool{
        Name:        "vectordb:search",
        Description: "Search the vector database for similar code snippets",
        InputSchema: map[string]interface{}{
            "type": "object",
            "properties": map[string]interface{}{
                "query": map[string]interface{}{
                    "type":        "string",
                    "description": "Natural language search query",
                },
                "limit": map[string]interface{}{
                    "type":        "integer",
                    "description": "Maximum number of results",
                    "default":     5,
                },
            },
            "required": []string{"query"},
        },
        Executor: &VectorDBExecutor{client: vectorClient},
    })

    // Use registry with agent runner
    runner := agent.NewRunner(backend, workDir, registry)
    runner.ExecuteStreaming(ctx, prompt, onChunk)
}
```

### 3. Add Middleware

```go
// Wrap tool with timing middleware for Agent Observatory
tool := tools.Tool{
    Name:     "vectordb:search",
    Executor: &VectorDBExecutor{client: vectorClient},
}.WithMiddleware(
    tools.LoggingMiddleware(logger),
    tools.TimingMiddleware(onDuration),
    tools.ValidationMiddleware(schema),
)

registry.Register(tool)
```

### 4. Test Tool

```go
func TestVectorDBTool(t *testing.T) {
    mockClient := &MockVectorDBClient{
        results: []SearchResult{
            {Snippet: "func Authenticate()", Score: 0.95},
            {Snippet: "func Login()", Score: 0.87},
        },
    }

    executor := &VectorDBExecutor{client: mockClient}
    result, err := executor.Execute(context.Background(), map[string]interface{}{
        "query": "authentication",
        "limit": 2,
    })

    assert.NoError(t, err)
    assert.Contains(t, result, "Authenticate")
}
```

## Running

```bash
go run main.go
```

## TODO

- [ ] Implement `main.go` with full vector DB tool
- [ ] Add vector DB client integration (e.g., Pinecone, Weaviate)
- [ ] Add middleware examples (logging, timing, validation)
- [ ] Test with Scout and Wave agents
