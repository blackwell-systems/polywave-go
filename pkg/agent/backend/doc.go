// Package backend defines the Backend interface for LLM provider abstraction.
//
// # Backend Interface
//
// Backends abstract LLM provider interactions. Implementing Backend allows the engine
// to support new providers without modifying core orchestrator logic.
//
//	type Backend interface {
//	    Run(ctx context.Context, systemPrompt, userPrompt, model string) (string, error)
//	    RunStreaming(ctx context.Context, systemPrompt, userPrompt, model string, onChunk ChunkCallback) (string, error)
//	    RunStreamingWithTools(ctx context.Context, systemPrompt, userPrompt, model string, onChunk ChunkCallback, onToolCall ToolCallCallback) (string, error)
//	}
//
// # Implementations
//
// The backend/ package provides implementations for multiple providers:
//
//   - api/ — Anthropic Messages API (official Anthropic endpoint)
//   - bedrock/ — AWS Bedrock using AWS SDK v2 (requires AWS credentials)
//   - openai/ — OpenAI-compatible API (OpenAI, Groq, Ollama, LM Studio)
//   - cli/ — Claude Code CLI subprocess (local development)
//
// # Provider Routing
//
// Model names with provider prefixes route to the appropriate backend:
//
//   - anthropic:claude-opus-4-6 → api backend
//   - bedrock:claude-sonnet-4-5 → bedrock backend
//   - openai:gpt-4o → openai backend
//   - ollama:qwen2.5-coder:32b → openai backend (localhost:11434)
//   - lmstudio:phi-4 → openai backend (localhost:1234)
//   - cli:claude-sonnet-4-6 → cli backend
//   - claude-sonnet-4-6 → api backend (default)
//
// # Tool Call Loop
//
// RunStreamingWithTools implements the agentic loop:
//
//  1. Send messages + tools to LLM
//  2. LLM responds with text or tool_use
//  3. If tool_use: execute tool via onToolCall, append result, loop
//  4. If text (finish_reason: "stop"): return final answer
//
// See docs/backends.md for implementation guide and examples/custom-backend/ for examples.
package backend
