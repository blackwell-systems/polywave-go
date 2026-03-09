// Package agent provides the agent execution runtime, tool system, and backend abstraction.
//
// # Agent Execution
//
// The Runner executes agent prompts with streaming output and tool use:
//
//	runner := agent.New(backend, workDir)
//	result, err := runner.ExecuteStreaming(ctx, prompt, onChunk)
//
// # Tool System
//
// Tools provide agents with capabilities to interact with the filesystem and execute commands.
// The current implementation (v0.11.0) uses a []Tool slice with StandardTools().
//
// After refactoring (see docs/tools.md), tools will be registered via ToolRegistry:
//
//	registry := tools.NewRegistry()
//	registry.Register(tools.Read(workDir))
//	registry.Register(tools.Write(workDir))
//	registry.Register(customTool)
//
// # Backend Abstraction
//
// Backends implement the Backend interface to support multiple LLM providers:
//
//	type Backend interface {
//	    Run(ctx, system, user, model string) (string, error)
//	    RunStreaming(ctx, system, user, model string, onChunk ChunkCallback) (string, error)
//	    RunStreamingWithTools(ctx, system, user, model string, onChunk ChunkCallback, onToolCall ToolCallCallback) (string, error)
//	}
//
// Supported backends:
//   - api (pkg/agent/backend/api) — Anthropic Messages API
//   - bedrock (pkg/agent/backend/bedrock) — AWS Bedrock with AWS SDK v2
//   - openai (pkg/agent/backend/openai) — OpenAI-compatible (OpenAI, Groq, Ollama, LM Studio)
//   - cli (pkg/agent/backend/cli) — Claude Code CLI subprocess
//
// See docs/tools.md for tool system architecture and examples/custom-tool/ for registration examples.
// See docs/backends.md for backend implementation guide and examples/custom-backend/ for examples.
package agent
