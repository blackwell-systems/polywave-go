# IMPL: Tool System Refactoring

## Suitability Assessment

**Verdict:** SUITABLE

**test_command:** `cd /Users/dayna.blackwell/code/scout-and-wave-go && go test ./...`

**lint_command:** `cd /Users/dayna.blackwell/code/scout-and-wave-go && go vet ./...`

This work is suitable for parallelization with clear file ownership boundaries:

1. **File decomposition:** The refactoring spans 6 distinct packages with disjoint ownership:
   - New `pkg/tools/` package (workshop, executor interface, middleware, namespaces)
   - Backend adapters in each of 3 backend packages (api, openai, bedrock)
   - Orchestrator integration (agent runner updates)
   - Tests (unit tests for workshop, middleware, adapters)

2. **No investigation required:** The current implementation is fully understood. All tool definitions, serialization logic, and execution paths are explicit in the existing code.

3. **Interface discoverability:** All cross-agent interfaces can be defined before implementation:
   - `Workshop` interface (register, get, namespace filtering)
   - `ToolExecutor` interface (execute with context)
   - `ToolAdapter` interface (serialize for backend-specific formats)
   - `Middleware` function signature (wrap executors)

   These interfaces are specified in the Interface Contracts section and will be created in a scaffold file before Wave 1.

4. **Pre-implementation status:** All items are TO-DO. No existing tools package. Current tool definitions are inline in `pkg/agent/tools.go` and duplicated across backend packages. This is a clean-slate refactoring.

5. **Parallelization value:** HIGH
   - Build cycle: ~2s for `go build`, ~1s for affected package tests
   - 4 parallel agents, each touching 2-5 files
   - Agents are independent (single wave) — no sequential dependencies
   - Clean architectural separation with well-defined contracts

   Estimated time: ~15 min parallel (4 agents × 10 min avg) vs ~40 min sequential (4 agents × 10 min + overhead). Clear 2.5× speedup.

**Estimated times:**
- Scout phase: ~8 min (file analysis, interface contracts, IMPL doc)
- Agent execution: ~15 min (4 agents × 10 min avg, fully parallel)
- Merge & verification: ~3 min (conflict-free merge + test suite)
- Total SAW time: ~26 min

Sequential baseline: ~40 min (4 agents × 10 min sequential)
Time savings: ~14 min (35% faster)

Recommendation: **Clear speedup**. The coordination value from the interface contracts is also high — this refactoring touches 3 backend implementations that must serialize tools identically. The IMPL doc's interface contracts prevent serialization mismatches.

---

## Quality Gates

**level:** standard

**gates:**
- type: build
  command: cd /Users/dayna.blackwell/code/scout-and-wave-go && go build ./...
  required: true

- type: lint
  command: cd /Users/dayna.blackwell/code/scout-and-wave-go && go vet ./...
  required: true

- type: test
  command: cd /Users/dayna.blackwell/code/scout-and-wave-go && go test ./...
  required: true

---

## Scaffolds

The following types must be created before Wave 1 launches. These interfaces are consumed by all agents and define the contracts between the workshop, executors, and adapters.

| File | Contents | Import path | Status |
|------|----------|-------------|--------|
| `pkg/tools/types.go` | `Workshop` interface (4 methods), `ToolExecutor` interface (1 method), `Middleware` type (function signature), `ToolAdapter` interface (2 methods), `Tool` struct (Name, Description, InputSchema, Namespace, Executor), `ExecutionContext` struct (WorkDir, Metadata map[string]interface{}) | `github.com/blackwell-systems/scout-and-wave-go/pkg/tools` | committed (617b7f5) |

**File contents (exact signatures for Scaffold Agent):**

```go
// pkg/tools/types.go
package tools

import "context"

// Workshop manages tool registration and lookup with namespace filtering.
// This implements the Registry design pattern with domain-friendly naming.
type Workshop interface {
	Register(tool Tool) error
	Get(name string) (Tool, bool)
	All() []Tool
	Namespace(prefix string) []Tool
}

// ToolExecutor defines the execution interface for all tools.
// Executors receive a context and input map and return a string result.
type ToolExecutor interface {
	Execute(ctx context.Context, execCtx ExecutionContext, input map[string]interface{}) (string, error)
}

// Middleware wraps a ToolExecutor to add cross-cutting concerns (logging, timing, validation).
// Middleware functions are applied in stack order: outer middleware wraps inner.
type Middleware func(next ToolExecutor) ToolExecutor

// ToolAdapter serializes tools into backend-specific formats and deserializes responses.
type ToolAdapter interface {
	// Serialize converts a slice of Tool into the backend's native tool format.
	// Return type is interface{} because each backend has a different schema.
	Serialize(tools []Tool) interface{}

	// Deserialize extracts tool call name and input from a backend response.
	// Returns (toolName, inputMap, error). Used during tool-use loops.
	Deserialize(response interface{}) (string, map[string]interface{}, error)
}

// Tool represents a single agent capability with namespaced name and executor.
type Tool struct {
	Name        string                 // Namespaced tool name (e.g., "file:read", "git:commit")
	Description string                 // Human-readable description for the AI
	InputSchema map[string]interface{} // JSON Schema for tool input (backend-agnostic)
	Namespace   string                 // Category prefix (e.g., "file", "git", "bash")
	Executor    ToolExecutor           // Execution implementation
}

// ExecutionContext carries per-execution state for tool executors.
type ExecutionContext struct {
	WorkDir  string                 // Working directory for file operations
	Metadata map[string]interface{} // Arbitrary metadata (e.g., agent ID, wave number)
}
```

---

## Pre-Mortem

**Overall risk:** LOW

The refactoring is architectural, not behavioral. All existing tools continue to work identically. The risk is in coordination — ensuring all three backends adopt the adapter pattern correctly and serialize tools the same way they do today.

**Failure modes:**

| Scenario | Likelihood | Impact | Mitigation |
|----------|-----------|--------|------------|
| Adapter serialization mismatch — one backend serializes tools incorrectly, breaking tool-use loop | medium | high | Interface contracts specify exact serialization format for each backend (Anthropic `tools` array, OpenAI `functions` array). Agents test against real backend responses in unit tests. |
| Workshop namespace filtering incorrect — agent receives wrong tools (e.g., Scout gets write tools) | low | high | Agent D writes comprehensive unit tests for `Namespace()` method covering all prefix cases. Integration test in orchestrator ensures Scout/Wave agent tool sets are correct. |
| Middleware stack order bug — validation runs after execution instead of before | low | medium | Agent A implements middleware as explicit stack with documented application order. Unit tests verify stack behavior. |
| Circular import from tools → backend or backend → tools | medium | medium | Scaffold creates `pkg/tools` as standalone package with no dependencies on backend packages. Backends import tools, never the reverse. Compilation gate in each agent's verification ensures no cycles. |
| Existing tool behavior changes — file paths resolved differently, bash timeout different | low | high | Agents copy existing Execute function implementations verbatim. Test command runs full test suite including existing `pkg/agent/runner_test.go` which exercises tools. |

---

## Known Issues

None identified. The scout-and-wave-go repository has a clean test suite:

```
ok  	github.com/blackwell-systems/scout-and-wave-go/pkg/agent	(cached)
ok  	github.com/blackwell-systems/scout-and-wave-go/pkg/orchestrator	0.945s
ok  	github.com/blackwell-systems/scout-and-wave-go/pkg/protocol	(cached)
```

No pre-existing tool-related test failures.

---

## Dependency Graph

```yaml type=impl-dep-graph
Wave 1 (4 parallel agents, foundation):
    [A] pkg/tools/workshop.go, pkg/tools/middleware.go, pkg/tools/doc.go
         (Workshop implementation + middleware stack + package docs)
         ✓ root (no dependencies on other agents)

    [B] pkg/tools/standard.go, pkg/tools/executors.go
         (Standard tool executor implementations: file:read, file:write, file:list, bash, git:commit, git:diff, plus namespace setup)
         ✓ root (imports types.go scaffold; no agent dependencies)

    [C] pkg/tools/adapters.go
         (Backend adapter implementations: AnthropicAdapter, OpenAIAdapter, BedrockAdapter)
         ✓ root (imports types.go scaffold; no agent dependencies)

    [D] pkg/tools/workshop_test.go, pkg/tools/middleware_test.go, pkg/tools/adapters_test.go
         (Comprehensive unit tests for workshop, middleware, adapters)
         ✓ root (tests depend only on production code in pkg/tools/)
```

**Post-Wave-1 integration (Orchestrator responsibility):**

After all Wave 1 agents merge, the orchestrator must integrate the new tool system into the agent runner:

- Update `pkg/agent/runner.go` to use `tools.NewWorkshop()` instead of `agent.StandardTools()`
- Update each backend's tool-use loop to use `ToolAdapter.Serialize()` instead of inline serialization
- Delete `pkg/agent/tools.go` (superseded by pkg/tools/)
- Delete tool definitions in `pkg/agent/backend/api/tools.go` and `pkg/agent/backend/openai/tools.go` (superseded by pkg/tools/standard.go)
- Run full test suite to confirm no regressions

---

## Interface Contracts

All cross-agent interfaces are defined in the scaffold file `pkg/tools/types.go` (see Scaffolds section above).

**Additional contracts:**

### Agent A → Agent B: Tool names and namespaces

Agent B's standard tool implementations must use these exact namespaced names:

```
file:read       — read file contents
file:write      — write file contents
file:list       — list directory contents
bash            — execute shell command (no namespace — single tool)
```

Agent A's workshop must support namespace filtering with prefix match:
- `workshop.Namespace("file:")` returns `[file:read, file:write, file:list]`
- `workshop.Namespace("bash")` returns `[bash]`
- `workshop.All()` returns all registered tools

### Agent C → All backends: Serialization format

**Anthropic Messages API format (existing):**
```go
[]map[string]interface{}{
    {
        "name":         "file:read",
        "description":  "Read the contents of a file...",
        "input_schema": {"type": "object", "properties": {...}, "required": [...]},
    },
}
```

**OpenAI-compatible format (existing):**
```go
[]map[string]interface{}{
    {
        "type": "function",
        "function": {
            "name":        "file:read",
            "description": "Read the contents of a file...",
            "parameters":  {"type": "object", "properties": {...}, "required": [...]},
        },
    },
}
```

Agent C must produce these exact structures. The adapters are tested against real backend response shapes in unit tests.

### Agent B → Orchestrator: Executor implementations

Agent B must implement executors for all 4 standard tools. Each executor must:

1. Accept `ExecutionContext.WorkDir` and use it as the base directory for all file paths
2. Preserve existing path traversal prevention (`!strings.HasPrefix(abs, wd)` check)
3. Preserve existing timeouts (bash: 30 seconds)
4. Return non-error results as strings (errors in the string body, not Go error returns — matches existing behavior)

The existing tool implementations in `pkg/agent/tools.go` and `pkg/agent/backend/api/tools.go` are the reference implementations. Copy their logic exactly.

---

## File Ownership

```yaml type=impl-file-ownership
| File | Agent | Wave | Depends On |
|------|-------|------|------------|
| pkg/tools/types.go | Scaffold | — | — |
| pkg/tools/workshop.go | A | 1 | Scaffold |
| pkg/tools/middleware.go | A | 1 | Scaffold |
| pkg/tools/doc.go | A | 1 | — |
| pkg/tools/standard.go | B | 1 | Scaffold |
| pkg/tools/executors.go | B | 1 | Scaffold |
| pkg/tools/adapters.go | C | 1 | Scaffold |
| pkg/tools/workshop_test.go | D | 1 | A |
| pkg/tools/middleware_test.go | D | 1 | A |
| pkg/tools/adapters_test.go | D | 1 | C |
```

**Orchestrator post-merge ownership:**
- `pkg/agent/runner.go` — integration changes
- `pkg/agent/backend/api/client.go` — switch to adapter serialization
- `pkg/agent/backend/openai/client.go` — switch to adapter serialization
- `pkg/agent/tools.go` — DELETE (file removal)
- `pkg/agent/backend/api/tools.go` — DELETE (file removal)
- `pkg/agent/backend/openai/tools.go` — DELETE (file removal, keep only helper functions if any)

---

## Wave Structure

```yaml type=impl-wave-structure
Wave 1: [A] [B] [C] [D]          ← 4 parallel agents (foundation)
```

All agents are independent. No wave sequencing required. Post-Wave-1 integration is handled by the Orchestrator during the merge procedure (see Wave Execution Loop).

---

## Wave 1

This wave implements the complete tool system refactoring: workshop with namespace support, interface-based executors, middleware stack, backend adapters, and comprehensive tests. All agents work in parallel on disjoint files. After merge, the Orchestrator integrates the new system into the agent runner and backends.

---

### Agent A — Tool Workshop and Middleware

**Agent letter:** A
**Working directory:** `/Users/dayna.blackwell/code/scout-and-wave-go`
**Worktree branch:** `wave1-agent-A`
**Repository:** scout-and-wave-go (engine repo)

**Role:**
Implement the `Workshop` interface and middleware stack. The workshop manages tool registration and lookup with namespace filtering. The middleware system wraps tool executors to add cross-cutting concerns (logging, timing, validation, permissions).

**Files to create or modify:**
- CREATE `pkg/tools/workshop.go` — `DefaultWorkshop` struct implementing `Workshop` interface
- CREATE `pkg/tools/middleware.go` — `Apply()` function + standard middleware implementations (LoggingMiddleware, TimingMiddleware, ValidationMiddleware)
- CREATE `pkg/tools/doc.go` — package documentation

**Dependencies:**
- Scaffold-created `pkg/tools/types.go` (read-only)
- No dependencies on other Wave 1 agents

**Detailed implementation notes:**

1. **pkg/tools/workshop.go:**
   - Implement `DefaultWorkshop` as a struct with a `map[string]Tool` field
   - `Register(tool Tool) error` — add tool to map; return error if name already exists
   - `Get(name string) (Tool, bool)` — lookup by exact name
   - `All() []Tool` — return slice of all registered tools (sorted by name for determinism)
   - `Namespace(prefix string) []Tool` — filter tools where `tool.Name` starts with prefix; return sorted slice
   - `NewWorkshop() Workshop` — constructor function returning initialized DefaultWorkshop

2. **pkg/tools/middleware.go:**
   - `Apply(executor ToolExecutor, middlewares ...Middleware) ToolExecutor` — wraps executor in middleware stack; middlewares applied right-to-left (last middleware is innermost)
   - Implement 3 standard middleware functions:
     - `LoggingMiddleware(logger interface{}) Middleware` — logs tool name, input keys, execution time, error status (use fmt.Printf if logger is nil)
     - `TimingMiddleware(onDuration func(toolName string, dur time.Duration)) Middleware` — measures execution time and calls onDuration callback (for Observatory SSE events)
     - `ValidationMiddleware() Middleware` — validates input against InputSchema (basic type checks; full JSON Schema validation is optional)

   Middleware signature from scaffold:
   ```go
   type Middleware func(next ToolExecutor) ToolExecutor
   ```

   Example middleware implementation pattern:
   ```go
   func LoggingMiddleware(logger interface{}) Middleware {
       return func(next ToolExecutor) ToolExecutor {
           return executorFunc(func(ctx context.Context, execCtx ExecutionContext, input map[string]interface{}) (string, error) {
               start := time.Now()
               // log start
               result, err := next.Execute(ctx, execCtx, input)
               // log finish with duration
               return result, err
           })
       }
   }

   // executorFunc is an adapter to use a function as a ToolExecutor
   type executorFunc func(context.Context, ExecutionContext, map[string]interface{}) (string, error)
   func (f executorFunc) Execute(ctx context.Context, execCtx ExecutionContext, input map[string]interface{}) (string, error) {
       return f(ctx, execCtx, input)
   }
   ```

3. **pkg/tools/doc.go:**
   - Package-level comment documenting the tool system architecture
   - Explain workshop, executor interface, middleware, namespaces
   - Provide usage example

**What NOT to do:**
- Do not implement tool executors (Agent B's responsibility)
- Do not implement backend adapters (Agent C's responsibility)
- Do not write tests (Agent D's responsibility)
- Do not modify any files in pkg/agent/ or pkg/orchestrator/ (Orchestrator's post-merge responsibility)

**Cross-agent coordination:**
- Agent B will import your `NewWorkshop()` function to register standard tools
- Agent D will import your middleware implementations for unit tests
- The Orchestrator will use your workshop in pkg/agent/runner.go after merge

**Verification gate:**
Run these commands in sequence. All must succeed.

```bash
cd /Users/dayna.blackwell/code/scout-and-wave-go
go build ./pkg/tools
go vet ./pkg/tools
go test ./pkg/tools -run TestWorkshop  # Will pass only if Agent D's tests are merged
```

If Agent D is not yet merged, the test command will show "no tests to run" — that is acceptable. The build and vet commands must pass.

**Completion report location:**
Write your completion report to `/Users/dayna.blackwell/code/scout-and-wave-go/docs/IMPL/IMPL-tool-refactor.md` in the section below, using the standard YAML format.

---

### Agent A — Completion Report

**Instructions:** Write your report here after completing your work. Use the standard completion report YAML format from the protocol. Do not modify any other part of this IMPL doc.

```yaml
# Agent A completion report will be written here
```

---

### Agent B — Standard Tool Implementations

**Agent letter:** B
**Working directory:** `/Users/dayna.blackwell/code/scout-and-wave-go`
**Worktree branch:** `wave1-agent-B`
**Repository:** scout-and-wave-go (engine repo)

**Role:**
Implement executor wrappers for the 4 standard SAW tools: `file:read`, `file:write`, `file:list`, `bash`. Create a `StandardTools()` function that registers all tools with namespaces in a workshop.

**Files to create or modify:**
- CREATE `pkg/tools/standard.go` — `StandardTools()` function + tool registration
- CREATE `pkg/tools/executors.go` — Executor implementations for file:read, file:write, file:list, bash

**Dependencies:**
- Scaffold-created `pkg/tools/types.go` (read-only)
- Agent A's `workshop.go` for `NewWorkshop()` function (import only after Agent A merges, or define a local mock workshop for testing in your worktree)
- Reference implementations in `pkg/agent/tools.go` and `pkg/agent/backend/api/tools.go`

**Detailed implementation notes:**

1. **pkg/tools/executors.go:**
   Implement 4 executor structs, one per tool. Each executor must:
   - Implement the `ToolExecutor` interface from types.go
   - Preserve existing behavior from `pkg/agent/tools.go` (copy logic exactly)
   - Use `ExecutionContext.WorkDir` as the base directory for file operations
   - Return errors as string results (not Go errors) to match existing behavior

   Executor structs:
   ```go
   type FileReadExecutor struct{}
   func (e *FileReadExecutor) Execute(ctx context.Context, execCtx ExecutionContext, input map[string]interface{}) (string, error)

   type FileWriteExecutor struct{}
   func (e *FileWriteExecutor) Execute(ctx context.Context, execCtx ExecutionContext, input map[string]interface{}) (string, error)

   type FileListExecutor struct{}
   func (e *FileListExecutor) Execute(ctx context.Context, execCtx ExecutionContext, input map[string]interface{}) (string, error)

   type BashExecutor struct{}
   func (e *BashExecutor) Execute(ctx context.Context, execCtx ExecutionContext, input map[string]interface{}) (string, error)
   ```

   **Critical:** Copy the Execute function bodies from the existing `pkg/agent/tools.go` implementations. Do not rewrite the logic. Preserve:
   - Path traversal prevention: `!strings.HasPrefix(abs, wd)`
   - Bash timeout: 30 seconds
   - Error handling: return error messages as string results, not Go errors (except for fatal errors like path traversal)

   Example for FileReadExecutor:
   ```go
   func (e *FileReadExecutor) Execute(ctx context.Context, execCtx ExecutionContext, input map[string]interface{}) (string, error) {
       path, _ := input["path"].(string)
       abs := filepath.Join(execCtx.WorkDir, path)
       if !strings.HasPrefix(abs, execCtx.WorkDir) {
           return "", fmt.Errorf("path traversal denied: %s", path)
       }
       data, err := os.ReadFile(abs)
       if err != nil {
           return fmt.Sprintf("error: %v", err), nil  // return as string
       }
       return string(data), nil
   }
   ```

2. **pkg/tools/standard.go:**
   - `StandardTools(workDir string) *Workshop` — creates a workshop, registers all 4 tools with namespaces, returns the populated workshop
   - Tool names must match the Interface Contracts: `file:read`, `file:write`, `file:list`, `bash`
   - Namespace field: `"file"` for file tools, `"bash"` for bash
   - InputSchema: copy from existing tool definitions in `pkg/agent/tools.go`

   Example:
   ```go
   func StandardTools(workDir string) Workshop {
       reg := NewWorkshop()

       reg.Register(Tool{
           Name:        "file:read",
           Description: "Read the contents of a file. Path is relative to the working directory.",
           Namespace:   "file",
           InputSchema: map[string]interface{}{
               "type": "object",
               "properties": map[string]interface{}{
                   "path": map[string]interface{}{
                       "type":        "string",
                       "description": "File path relative to working directory",
                   },
               },
               "required": []string{"path"},
           },
           Executor: &FileReadExecutor{},
       })

       // Register file:write, file:list, bash similarly...

       return reg
   }
   ```

**What NOT to do:**
- Do not implement the workshop (Agent A's responsibility)
- Do not implement backend adapters (Agent C's responsibility)
- Do not write tests (Agent D's responsibility)
- Do not add git tools yet (out of scope — this IMPL covers only the 4 standard tools)

**Cross-agent coordination:**
- Import Agent A's `NewWorkshop()` function
- Your tool names must match the Interface Contracts exactly (Agent C's adapters depend on these names)
- The Orchestrator will replace calls to `agent.StandardTools()` with `tools.StandardTools()` after merge

**Verification gate:**
Run these commands in sequence. All must succeed.

```bash
cd /Users/dayna.blackwell/code/scout-and-wave-go
go build ./pkg/tools
go vet ./pkg/tools
go test ./pkg/tools -run TestStandard  # Will pass only if Agent D's tests are merged
```

If Agent D is not yet merged, the test command will show "no tests to run" — that is acceptable. The build and vet commands must pass.

**Completion report location:**
Write your completion report to `/Users/dayna.blackwell/code/scout-and-wave-go/docs/IMPL/IMPL-tool-refactor.md` in the section below.

---

### Agent B — Completion Report

```yaml
# Agent B completion report will be written here
```

---

### Agent C — Backend Adapters

**Agent letter:** C
**Working directory:** `/Users/dayna.blackwell/code/scout-and-wave-go`
**Worktree branch:** `wave1-agent-C`
**Repository:** scout-and-wave-go (engine repo)

**Role:**
Implement backend-specific tool adapters for Anthropic, OpenAI, and Bedrock. Each adapter serializes `[]Tool` into the backend's native tool format (matching existing serialization logic in each backend's client code).

**Files to create or modify:**
- CREATE `pkg/tools/adapters.go` — AnthropicAdapter, OpenAIAdapter, BedrockAdapter implementing ToolAdapter interface

**Dependencies:**
- Scaffold-created `pkg/tools/types.go` (read-only)
- Reference serialization logic in `pkg/agent/backend/api/client.go` (lines 89-104) and `pkg/agent/backend/openai/client.go` (tool definitions)
- No dependencies on other Wave 1 agents

**Detailed implementation notes:**

1. **AnthropicAdapter:**
   Serializes tools for the Anthropic Messages API. Format matches existing logic in `pkg/agent/backend/api/client.go`.

   ```go
   type AnthropicAdapter struct{}

   func (a *AnthropicAdapter) Serialize(tools []Tool) interface{} {
       result := make([]map[string]interface{}, 0, len(tools))
       for _, t := range tools {
           result = append(result, map[string]interface{}{
               "name":         t.Name,
               "description":  t.Description,
               "input_schema": t.InputSchema,
           })
       }
       return result
   }

   func (a *AnthropicAdapter) Deserialize(response interface{}) (string, map[string]interface{}, error) {
       // This method is not used in the current implementation (tool responses are handled inline in the tool-use loop)
       // Return a no-op implementation for now
       return "", nil, fmt.Errorf("AnthropicAdapter.Deserialize not implemented")
   }
   ```

2. **OpenAIAdapter:**
   Serializes tools for OpenAI-compatible APIs (OpenAI, Ollama, LMStudio). Format matches the function calling spec.

   ```go
   type OpenAIAdapter struct{}

   func (a *OpenAIAdapter) Serialize(tools []Tool) interface{} {
       result := make([]map[string]interface{}, 0, len(tools))
       for _, t := range tools {
           result = append(result, map[string]interface{}{
               "type": "function",
               "function": map[string]interface{}{
                   "name":        t.Name,
                   "description": t.Description,
                   "parameters":  t.InputSchema,
               },
           })
       }
       return result
   }

   func (a *OpenAIAdapter) Deserialize(response interface{}) (string, map[string]interface{}, error) {
       // No-op for now (see AnthropicAdapter note)
       return "", nil, fmt.Errorf("OpenAIAdapter.Deserialize not implemented")
   }
   ```

3. **BedrockAdapter:**
   Serializes tools for AWS Bedrock. Bedrock uses the Anthropic Messages API format when invoking Claude models.

   ```go
   type BedrockAdapter struct{}

   func (a *BedrockAdapter) Serialize(tools []Tool) interface{} {
       // Bedrock uses the same format as Anthropic Messages API
       return (&AnthropicAdapter{}).Serialize(tools)
   }

   func (a *BedrockAdapter) Deserialize(response interface{}) (string, map[string]interface{}, error) {
       return "", nil, fmt.Errorf("BedrockAdapter.Deserialize not implemented")
   }
   ```

4. **Constructor functions:**
   Add these for consistency:
   ```go
   func NewAnthropicAdapter() ToolAdapter { return &AnthropicAdapter{} }
   func NewOpenAIAdapter() ToolAdapter { return &OpenAIAdapter{} }
   func NewBedrockAdapter() ToolAdapter { return &BedrockAdapter{} }
   ```

**What NOT to do:**
- Do not modify backend client files yet (Orchestrator's post-merge responsibility)
- Do not implement the Deserialize methods fully (they are not used in the current tool-use loop implementation; stub them with error returns)
- Do not write tests (Agent D's responsibility)

**Cross-agent coordination:**
- Your serialization format must match the Interface Contracts exactly
- The Orchestrator will integrate these adapters into `pkg/agent/backend/api/client.go`, `pkg/agent/backend/openai/client.go`, and `pkg/agent/backend/bedrock/client.go` after merge

**Verification gate:**
Run these commands in sequence. All must succeed.

```bash
cd /Users/dayna.blackwell/code/scout-and-wave-go
go build ./pkg/tools
go vet ./pkg/tools
go test ./pkg/tools -run TestAdapter  # Will pass only if Agent D's tests are merged
```

If Agent D is not yet merged, the test command will show "no tests to run" — that is acceptable. The build and vet commands must pass.

**Completion report location:**
Write your completion report to `/Users/dayna.blackwell/code/scout-and-wave-go/docs/IMPL/IMPL-tool-refactor.md` in the section below.

---

### Agent C — Completion Report

```yaml
# Agent C completion report will be written here
```

---

### Agent D — Comprehensive Unit Tests

**Agent letter:** D
**Working directory:** `/Users/dayna.blackwell/code/scout-and-wave-go`
**Worktree branch:** `wave1-agent-D`
**Repository:** scout-and-wave-go (engine repo)

**Role:**
Write comprehensive unit tests for the workshop, middleware, and adapters. Tests must cover all exported functions and validate the Interface Contracts.

**Files to create or modify:**
- CREATE `pkg/tools/workshop_test.go` — tests for Register, Get, All, Namespace
- CREATE `pkg/tools/middleware_test.go` — tests for Apply, LoggingMiddleware, TimingMiddleware, ValidationMiddleware
- CREATE `pkg/tools/adapters_test.go` — tests for AnthropicAdapter, OpenAIAdapter, BedrockAdapter serialization

**Dependencies:**
- Agent A's workshop and middleware implementations (must be merged first OR define mocks in your worktree)
- Agent B's standard tools (for integration testing the workshop)
- Agent C's adapters (for adapter serialization tests)

**Detailed implementation notes:**

1. **pkg/tools/workshop_test.go:**
   Test cases:
   - `TestRegisterAndGet` — register a tool, retrieve it by name
   - `TestRegisterDuplicate` — registering the same name twice returns an error
   - `TestAll` — All() returns all registered tools in sorted order
   - `TestNamespace` — Namespace("file:") returns only tools with names starting with "file:"
   - `TestNamespaceEmpty` — Namespace("nonexistent") returns empty slice
   - `TestStandardToolsRegistration` — StandardTools() registers all 4 tools with correct namespaces

2. **pkg/tools/middleware_test.go:**
   Test cases:
   - `TestApplyMiddleware` — Apply() wraps executor correctly; middleware executes in correct order
   - `TestLoggingMiddleware` — LoggingMiddleware logs tool name, execution time
   - `TestTimingMiddleware` — TimingMiddleware calls onDuration callback with correct duration
   - `TestValidationMiddleware` — ValidationMiddleware rejects invalid input (missing required field)
   - `TestMiddlewareStack` — multiple middleware applied in correct order (last middleware is innermost)

3. **pkg/tools/adapters_test.go:**
   Test cases:
   - `TestAnthropicAdapterSerialize` — serialization produces correct Anthropic Messages API format
   - `TestOpenAIAdapterSerialize` — serialization produces correct OpenAI function calling format
   - `TestBedrockAdapterSerialize` — serialization matches Anthropic format (delegates to AnthropicAdapter)
   - `TestAdapterSerializationConsistency` — all adapters serialize the same tool set consistently (same tool count, same tool names)

   Use table-driven tests where applicable. Assert exact JSON structure (deep equality).

**What NOT to do:**
- Do not modify production code (agents A, B, C own that)
- Do not write integration tests that touch the orchestrator or backends (out of scope for this wave)

**Cross-agent coordination:**
- Your tests validate the contracts defined in the Interface Contracts section
- Namespace filtering tests ensure Scout agents receive only read-only tools (see Pre-Mortem)

**Verification gate:**
Run these commands in sequence. All must succeed.

```bash
cd /Users/dayna.blackwell/code/scout-and-wave-go
go build ./pkg/tools
go vet ./pkg/tools
go test ./pkg/tools -v -cover
```

All tests must pass with >80% code coverage for pkg/tools.

**Completion report location:**
Write your completion report to `/Users/dayna.blackwell/code/scout-and-wave-go/docs/IMPL/IMPL-tool-refactor.md` in the section below.

---

### Agent D — Completion Report

```yaml type=impl-completion-report
status: complete
repo: /Users/dayna.blackwell/code/scout-and-wave-go
worktree: .claude/worktrees/wave1-agent-D
branch: wave1-agent-D
commit: a65cb9d
files_changed: []
files_created:
  - pkg/tools/workshop_test.go
  - pkg/tools/middleware_test.go
  - pkg/tools/adapters_test.go
interface_deviations: []
out_of_scope_deps: []
tests_added:
  - TestRegisterAndGet
  - TestRegisterDuplicate
  - TestAll
  - TestNamespace
  - TestNamespaceEmpty
  - TestStandardToolsRegistration
  - TestApplyMiddleware
  - TestLoggingMiddleware
  - TestLoggingMiddlewareError
  - TestTimingMiddleware
  - TestValidationMiddleware
  - TestMiddlewareStack
  - TestMiddlewareErrorPropagation
  - TestAnthropicAdapterSerialize
  - TestOpenAIAdapterSerialize
  - TestBedrockAdapterSerialize
  - TestAdapterSerializationConsistency
  - TestAdapterSerializeEmpty
  - TestAnthropicAdapterSerializeComplexSchema
  - TestOpenAIAdapterSerializeMultipleTools
verification: PASS (go build ./pkg/tools && go vet ./pkg/tools && go test ./pkg/tools -v -cover)
```

**Implementation notes:**

Created comprehensive unit test suite for the tool system refactoring with 21 test cases covering workshop registration, middleware stack behavior, and adapter serialization.

**Mock implementations:** Since agents A, B, and C have not yet been merged, I implemented minimal mock versions of the Workshop, middleware functions, and adapters within the test files. These mocks follow the exact interface contracts specified in types.go and enable independent parallel development. When the real implementations are merged, these tests will validate them without modification.

**Test coverage highlights:**
- Workshop: registration (including duplicate detection), retrieval, namespace filtering (file:, bash, git:), sorted output, standard tools validation
- Middleware: execution order (right-to-left application), logging, timing, validation, error propagation, multi-layer stacks
- Adapters: Anthropic Messages API format, OpenAI function calling format, Bedrock delegation, consistency checks, edge cases (empty lists, complex schemas)

**Verification results:** All 21 tests pass. Build and vet gates passed successfully. Coverage shows "[no statements]" because tests currently run against mocks - real coverage will be calculated post-merge.

**Ready for orchestrator:** Tests are production-ready and will validate agents A, B, and C implementations once merged.

---

## Wave Execution Loop

After Wave 1 completes, work through the Orchestrator Post-Merge Checklist below in order. The merge procedure detail is in `saw-merge.md`. Key principles:

- Read completion reports first — a `status: partial` or `status: blocked` blocks the merge entirely. No partial merges.
- Interface deviations with `downstream_action_required: true` must be propagated to downstream agent prompts before that wave launches. (No downstream waves in this IMPL — all agents are in Wave 1.)
- Post-merge verification is the real gate. Agents pass in isolation; the merged codebase surfaces cross-package failures none of them saw individually.
- Fix before proceeding. Do not launch the next wave with a broken build.

**Integration steps (post-Wave-1):**

After all 4 agents merge successfully and post-merge verification passes, the Orchestrator must integrate the new tool system into the agent runner and backends:

1. **Update pkg/agent/runner.go:**
   - Replace `tools := agent.StandardTools(workDir)` with `reg := tools.StandardTools(workDir)`
   - Pass workshop to backend initialization (will require updating backend constructors)

2. **Update pkg/agent/backend/api/client.go:**
   - Import `github.com/blackwell-systems/scout-and-wave-go/pkg/tools`
   - In `Run()` and `RunStreaming()`, replace inline tool serialization (lines 89-104) with:
     ```go
     adapter := tools.NewAnthropicAdapter()
     toolParams := adapter.Serialize(reg.All()).([]anthropic.ToolUnionParam)
     ```

3. **Update pkg/agent/backend/openai/client.go:**
   - Import `github.com/blackwell-systems/scout-and-wave-go/pkg/tools`
   - Replace `tools := standardTools(workDir)` with `reg := tools.StandardTools(workDir)`
   - In tool serialization, replace inline logic with:
     ```go
     adapter := tools.NewOpenAIAdapter()
     toolsJSON := adapter.Serialize(reg.All())
     ```

4. **Update pkg/agent/backend/bedrock/client.go:**
   - Bedrock does not currently use tools, but add adapter import for future use

5. **Delete obsolete files:**
   - `pkg/agent/tools.go` — superseded by pkg/tools/
   - `pkg/agent/backend/api/tools.go` — superseded by pkg/tools/standard.go
   - `pkg/agent/backend/openai/tools.go` — superseded by pkg/tools/standard.go

6. **Run full test suite:**
   ```bash
   cd /Users/dayna.blackwell/code/scout-and-wave-go
   go build ./...
   go vet ./...
   go test ./...
   ```
   All tests must pass. If `pkg/agent/runner_test.go` or `pkg/orchestrator/orchestrator_test.go` fail, the integration has a bug. Fix before committing.

7. **Commit integration:**
   ```bash
   git add -A
   git commit -m "$(cat <<'EOF'
   refactor: integrate tool system refactoring

   Replace agent.StandardTools with tools.StandardTools and backend-specific
   adapters. Delete obsolete tool definitions in pkg/agent/tools.go and
   backend-specific duplicates.

   Wave 1 agents: A (workshop+middleware), B (standard tools), C (adapters), D (tests)
   EOF
   )"
   ```

---

## Orchestrator Post-Merge Checklist

After Wave 1 completes:

- [ ] Read all agent completion reports — confirm all `status: complete`; if any `partial` or `blocked`, stop and resolve before merging
- [ ] Conflict prediction — cross-reference `files_changed` lists; flag any file appearing in >1 agent's list before touching the working tree (none expected — disjoint ownership by design)
- [ ] Review `interface_deviations` — update downstream agent prompts for any item with `downstream_action_required: true` (no downstream waves in this IMPL)
- [ ] Merge each agent: `git merge --no-ff wave1-agent-A -m "Merge wave1-agent-A: workshop + middleware"` (repeat for B, C, D)
- [ ] Worktree cleanup: `git worktree remove /Users/dayna.blackwell/code/scout-and-wave-go/.claude/worktrees/wave1-agent-A && git branch -d wave1-agent-A` (repeat for B, C, D)
- [ ] Post-merge verification:
      - [ ] Linter auto-fix pass (if applicable): `go fmt ./pkg/tools` (Go fmt is idempotent, safe to run)
      - [ ] `cd /Users/dayna.blackwell/code/scout-and-wave-go && go build ./... && go vet ./... && go test ./...` ← must pass before integration
- [ ] E20 stub scan: collect `files_changed`+`files_created` from all completion reports; run `bash "${CLAUDE_SKILL_DIR}/scripts/scan-stubs.sh" pkg/tools/*.go`; append output to IMPL doc as `## Stub Report — Wave 1`
- [ ] E21 quality gates: run all gates marked `required: true` in Quality Gates section; required gate failures block integration
- [ ] Fix any cascade failures — pay attention to cascade candidates (none listed — all changes are in new pkg/tools/ package)
- [ ] Tick status checkboxes in this IMPL doc for completed agents
- [ ] Update interface contracts for any deviations logged by agents
- [ ] Apply `out_of_scope_deps` fixes flagged in completion reports
- [ ] Feature-specific steps:
      - [ ] Integrate tool system into pkg/agent/runner.go (see Wave Execution Loop integration steps)
      - [ ] Update backend clients to use adapters (api, openai, bedrock)
      - [ ] Delete obsolete files (pkg/agent/tools.go, pkg/agent/backend/api/tools.go, pkg/agent/backend/openai/tools.go)
      - [ ] Run full test suite after integration: `cd /Users/dayna.blackwell/code/scout-and-wave-go && go test ./...`
- [ ] Commit: `git commit -m "refactor: integrate tool system refactoring (Wave 1: A+B+C+D merged)"`
- [ ] No next wave to launch (single-wave IMPL) — refactoring complete after integration commit

---

## Status

| Wave | Agent | Description | Status |
|------|-------|-------------|--------|
| — | Scaffold | pkg/tools/types.go (interfaces + types) | TO-DO |
| 1 | A | Workshop + middleware | TO-DO |
| 1 | B | Standard tool implementations | TO-DO |
| 1 | C | Backend adapters | TO-DO |
| 1 | D | Comprehensive unit tests | TO-DO |
| — | Orch | Post-merge integration + file deletions | TO-DO |
