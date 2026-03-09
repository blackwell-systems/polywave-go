# Tool System

> **Status:** This document describes the **proposed** tool system architecture after the refactoring outlined in the [scout-and-wave ROADMAP.md](https://github.com/blackwell-systems/scout-and-wave/blob/main/ROADMAP.md#tool-system-refactoring). The current implementation (as of v0.11.0) uses a `[]Tool` slice with `StandardTools()` — this document describes the target state.

## Overview

The tool system provides agents with capabilities to interact with the filesystem, execute commands, and perform searches. After refactoring, it is built on five foundational patterns:

1. **Tool Workshop** — Dynamic registration and composition
2. **ToolExecutor Interface** — Testable, stateful tool execution
3. **Middleware Stack** — Cross-cutting concerns (logging, timing, validation, permissions)
4. **Backend Adapters** — Decoupled serialization for each LLM provider
5. **Namespaces** — Permission models via tool categories

## Architecture

### 1. Tool Workshop

The `Workshop` replaces the hardcoded `StandardTools()` function.

**Interface:**

```go
type Workshop struct {
    tools map[string]Tool
}

func NewWorkshop() *Workshop
func (r *Workshop) Register(tool Tool) error
func (r *Workshop) Get(name string) (Tool, bool)
func (r *Workshop) All() []Tool
func (r *Workshop) Namespace(prefix string) []Tool
```

**Usage:**

```go
workshop := tools.NewWorkshop()

// Register standard tools
workshop.Register(tools.Read(workDir))
workshop.Register(tools.Write(workDir))
workshop.Register(tools.Edit(workDir))
workshop.Register(tools.Bash(workDir))
workshop.Register(tools.Glob(workDir))
workshop.Register(tools.Grep(workDir))

// Register custom tool
workshop.Register(tools.Tool{
    Name:        "query_vector_db",
    Description: "Search the vector database for similar code snippets",
    InputSchema: map[string]interface{}{
        "type": "object",
        "properties": map[string]interface{}{
            "query": map[string]interface{}{"type": "string"},
            "limit": map[string]interface{}{"type": "integer"},
        },
        "required": []string{"query"},
    },
    Executor: &VectorDBExecutor{client: vectorClient},
})
```

### 2. ToolExecutor Interface

Tools are executed via the `ToolExecutor` interface rather than raw function fields.

**Interface:**

```go
type ToolExecutor interface {
    Execute(ctx context.Context, input map[string]interface{}) (string, error)
}

type Tool struct {
    Name        string
    Description string
    InputSchema map[string]interface{}
    Executor    ToolExecutor
}
```

**Benefits:**
- Executors can carry state (DB connections, API clients)
- Easier to mock for testing
- Supports tool versioning (different executor per agent type)

**Example:**

```go
type FileReadExecutor struct {
    workDir string
    fs      afero.Fs // Abstract filesystem for testing
}

func (e *FileReadExecutor) Execute(ctx context.Context, input map[string]interface{}) (string, error) {
    path := input["path"].(string)
    fullPath := filepath.Join(e.workDir, path)
    content, err := e.fs.ReadFile(fullPath)
    if err != nil {
        return "", fmt.Errorf("read failed: %w", err)
    }
    return string(content), nil
}
```

### 3. Middleware Stack

Tool execution is wrapped in a middleware stack for cross-cutting concerns.

**Interface:**

```go
type ToolMiddleware func(next ToolExecutor) ToolExecutor
```

**Built-in Middleware:**

#### Logging Middleware

Logs every tool call with input and result.

```go
func LoggingMiddleware(logger *log.Logger) ToolMiddleware {
    return func(next ToolExecutor) ToolExecutor {
        return ToolExecutorFunc(func(ctx context.Context, input map[string]interface{}) (string, error) {
            logger.Printf("Tool call started: %v", input)
            result, err := next.Execute(ctx, input)
            logger.Printf("Tool call finished: err=%v", err)
            return result, err
        })
    }
}
```

#### Timing Middleware (Agent Observatory)

Measures tool execution duration and publishes SSE events.

```go
func TimingMiddleware(onDuration func(toolName string, dur time.Duration)) ToolMiddleware {
    return func(next ToolExecutor) ToolExecutor {
        return ToolExecutorFunc(func(ctx context.Context, input map[string]interface{}) (string, error) {
            start := time.Now()
            result, err := next.Execute(ctx, input)
            duration := time.Since(start)
            onDuration(getToolName(ctx), duration)
            return result, err
        })
    }
}
```

#### Validation Middleware

Enforces JSON schema validation before execution.

```go
func ValidationMiddleware(schema map[string]interface{}) ToolMiddleware {
    return func(next ToolExecutor) ToolExecutor {
        return ToolExecutorFunc(func(ctx context.Context, input map[string]interface{}) (string, error) {
            if err := validateSchema(input, schema); err != nil {
                return "", fmt.Errorf("validation failed: %w", err)
            }
            return next.Execute(ctx, input)
        })
    }
}
```

#### Permission Middleware

Enforces tool access control (e.g., Scout agents are read-only).

```go
func PermissionMiddleware(allowedTools []string) ToolMiddleware {
    return func(next ToolExecutor) ToolExecutor {
        return ToolExecutorFunc(func(ctx context.Context, input map[string]interface{}) (string, error) {
            toolName := getToolName(ctx)
            if !contains(allowedTools, toolName) {
                return "", fmt.Errorf("permission denied: %s not allowed", toolName)
            }
            return next.Execute(ctx, input)
        })
    }
}
```

**Composing Middleware:**

```go
tool := tools.Read(workDir).WithMiddleware(
    tools.LoggingMiddleware(logger),
    tools.TimingMiddleware(onDuration),
    tools.ValidationMiddleware(schema),
)
```

### 4. Backend Adapters

Tool serialization is extracted into backend-specific adapters.

**Interface:**

```go
type ToolAdapter interface {
    Serialize(tools []Tool) interface{}
    Deserialize(response interface{}) (name string, input map[string]interface{}, error)
}
```

**Anthropic Messages API Adapter:**

```go
type AnthropicToolAdapter struct{}

func (a *AnthropicToolAdapter) Serialize(tools []Tool) interface{} {
    anthropicTools := make([]map[string]interface{}, len(tools))
    for i, t := range tools {
        anthropicTools[i] = map[string]interface{}{
            "name":         t.Name,
            "description":  t.Description,
            "input_schema": t.InputSchema,
        }
    }
    return anthropicTools
}

func (a *AnthropicToolAdapter) Deserialize(response interface{}) (string, map[string]interface{}, error) {
    // Parse Anthropic tool_use block
    // Return (tool_name, input_json, error)
}
```

**OpenAI-Compatible Adapter:**

```go
type OpenAIToolAdapter struct{}

func (a *OpenAIToolAdapter) Serialize(tools []Tool) interface{} {
    openaiTools := make([]map[string]interface{}, len(tools))
    for i, t := range tools {
        openaiTools[i] = map[string]interface{}{
            "type": "function",
            "function": map[string]interface{}{
                "name":        t.Name,
                "description": t.Description,
                "parameters":  t.InputSchema,
            },
        }
    }
    return openaiTools
}
```

**Backend Usage:**

```go
// In pkg/agent/backend/api/client.go
adapter := tools.NewAnthropicToolAdapter()
serializedTools := adapter.Serialize(workshop.All())

reqBody := map[string]interface{}{
    "model":   model,
    "messages": messages,
    "tools":   serializedTools,
}
```

### 5. Namespaces

Tools use namespaced names for categorization and filtering.

**Naming Convention:**

```
<category>:<action>

file:read
file:write
file:list
git:commit
git:diff
bash:exec
web:fetch
agent:spawn
```

**Permission Models:**

```go
// Scout agents: read-only
scoutTools := workshop.Namespace("file:read", "file:list")

// Wave agents: read-write + bash
waveTools := workshop.Namespace("file:", "git:", "bash:")

// Chat: all tools
chatTools := workshop.All()
```

**Workshop Namespace Filtering:**

```go
func (r *Workshop) Namespace(prefixes ...string) []Tool {
    var filtered []Tool
    for _, tool := range r.tools {
        for _, prefix := range prefixes {
            if strings.HasPrefix(tool.Name, prefix) {
                filtered = append(filtered, tool)
                break
            }
        }
    }
    return filtered
}
```

## Standard Tools

After refactoring, the standard tool set uses namespaced names:

| Tool Name | Description | Scout | Wave |
|-----------|-------------|-------|------|
| `file:read` | Read file contents | ✓ | ✓ |
| `file:write` | Write file (create or overwrite) | ✗ | ✓ |
| `file:edit` | Edit file with string replacement | ✗ | ✓ |
| `file:list` | List files matching glob pattern | ✓ | ✓ |
| `file:search` | Search file contents (grep) | ✓ | ✓ |
| `bash:exec` | Execute bash command | ✗ | ✓ |
| `git:commit` | Commit changes | ✗ | ✓ |
| `git:diff` | Show diff | ✓ | ✓ |

## Agent Observatory Integration

The timing middleware publishes SSE events for every tool call:

```go
func (o *Orchestrator) launchAgent(ctx context.Context, agent types.Agent) {
    workshop := tools.NewWorkshop()

    // Wrap all tools with timing middleware
    onDuration := func(toolName string, dur time.Duration) {
        o.ssebroker.Publish(orchestrator.Event{
            Type: "agent_tool_result",
            Data: map[string]interface{}{
                "agent_id":    agent.Letter,
                "tool_name":   toolName,
                "duration_ms": dur.Milliseconds(),
            },
        })
    }

    for _, tool := range standardTools {
        wrappedTool := tool.WithMiddleware(tools.TimingMiddleware(onDuration))
        workshop.Register(wrappedTool)
    }

    // Pass registry to agent runner
    agent.ExecuteStreaming(ctx, registry)
}
```

## Custom Tool Example

Registering a custom tool for querying a vector database:

```go
type VectorDBExecutor struct {
    client *vectordb.Client
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

// Registration
workshop.Register(tools.Tool{
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
```

## Testing

Mock executors for unit tests:

```go
type MockReadExecutor struct {
    files map[string]string
}

func (m *MockReadExecutor) Execute(ctx context.Context, input map[string]interface{}) (string, error) {
    path := input["path"].(string)
    content, ok := m.files[path]
    if !ok {
        return "", fmt.Errorf("file not found: %s", path)
    }
    return content, nil
}

// Test
func TestAgentReadsFile(t *testing.T) {
    workshop := tools.NewWorkshop()
    workshop.Register(tools.Tool{
        Name:     "file:read",
        Executor: &MockReadExecutor{
            files: map[string]string{
                "auth.go": "package auth\n\nfunc Authenticate() {}",
            },
        },
    })

    // Run agent with mock registry
    agent.ExecuteStreaming(ctx, registry)
}
```

## Migration Path

The refactoring is implemented as a clean-slate breaking change:

1. Delete `StandardTools()` and current `[]Tool` slice implementation
2. Implement `Workshop` with namespace support
3. Convert all tools to use `ToolExecutor` interface
4. Apply middleware stack uniformly to all tools
5. Create backend adapters and remove inline serialization
6. Update all tool names to use namespace prefixes (`file:read`, `git:commit`, etc.)

See [scout-and-wave ROADMAP.md](https://github.com/blackwell-systems/scout-and-wave/blob/main/ROADMAP.md#tool-system-refactoring) for full rationale.
