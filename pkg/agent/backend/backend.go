package backend

import (
	"context"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/tools"
)

// Config carries backend-agnostic configuration.
type Config struct {
	// Model is the model identifier (e.g. "claude-sonnet-4-6", "gpt-4o").
	// CLI backend passes it as --model; API backend passes it in the request body.
	Model string

	// MaxTokens caps output token count. Ignored by the CLI backend.
	MaxTokens int

	// MaxTurns is the tool-use loop limit. 0 means use the backend default (50).
	MaxTurns int

	// BinaryPath is the path to the CLI binary used by the CLI backend.
	// If empty, the CLI backend locates "claude" via PATH.
	// Set this to use a different compatible CLI (e.g. "/usr/local/bin/kimi").
	BinaryPath string

	// APIKey is the API key for the OpenAI-compatible backend.
	// If empty, the OPENAI_API_KEY environment variable is used.
	APIKey string

	// BaseURL is an optional endpoint override for the OpenAI-compatible backend
	// (e.g. "https://api.groq.com/openai/v1" for Groq, "http://localhost:11434/v1" for Ollama).
	// If empty, the official OpenAI endpoint is used.
	BaseURL string

	// OnToolCall, if non-nil, is called after each tool execution with timing
	// data. Backends use this to feed the Agent Observatory SSE pipeline.
	OnToolCall ToolCallCallback

	// AnthropicKey is the Anthropic API key. If empty, ANTHROPIC_API_KEY env var is used.
	AnthropicKey string

	// BedrockRegion is the AWS region for Bedrock (e.g. "us-east-1").
	// If empty, uses AWS SDK default chain.
	BedrockRegion string

	// BedrockAccessKeyID is the AWS access key for Bedrock.
	// If empty, uses AWS SDK default credential chain.
	BedrockAccessKeyID string

	// BedrockSecretAccessKey is the AWS secret key for Bedrock.
	// If empty, uses AWS SDK default credential chain.
	BedrockSecretAccessKey string

	// BedrockSessionToken is an optional AWS session token for temporary credentials.
	BedrockSessionToken string

	// BedrockProfile is an AWS CLI named profile (e.g. "my-sso-profile").
	// When set, uses config.WithSharedConfigProfile to load credentials
	// from ~/.aws/config, supporting SSO, assume-role, and other profile types.
	BedrockProfile string

	// ReadOnly, when true, applies permission middleware that blocks write_file
	// and edit_file at execution time. Used for Scout agents.
	ReadOnly bool

	// Constraints, if non-nil, configures SAW protocol invariant enforcement
	// (I1 file ownership, I2 interface freeze, I5 commit tracking, I6 role separation).
	// When nil, no constraints are applied (backward compatible).
	Constraints *tools.Constraints
}

// ChunkCallback is called with each text chunk as it arrives from the backend.
// Implementations must be safe to call from a goroutine.
// chunk is a raw text fragment (may be a partial word or sentence).
type ChunkCallback func(chunk string)

// ToolCallEvent carries a single tool invocation or result from the CLI stream.
type ToolCallEvent struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Input      string `json:"input"`
	IsResult   bool   `json:"is_result"`
	IsError    bool   `json:"is_error"`
	DurationMs int64  `json:"duration_ms"`
}

// ToolCallCallback is called for each tool_use/tool_result event parsed
// from the CLI stream. Implementations must be goroutine-safe.
type ToolCallCallback func(ev ToolCallEvent)

// Backend is the abstraction both the API client and the CLI client implement.
// Runner accepts a Backend and delegates all LLM interaction through it.
type Backend interface {
	// Run executes the agent described by systemPrompt and userMessage,
	// using workDir as the working directory for any file/shell operations.
	// It returns the final assistant text when the agent signals completion.
	Run(ctx context.Context, systemPrompt, userMessage, workDir string) (string, error)

	// RunStreaming executes the agent identically to Run, but calls onChunk
	// with each text fragment as it arrives. onChunk may be nil, in which
	// case RunStreaming behaves identically to Run.
	// Returns the full concatenated output and any error, same as Run.
	RunStreaming(ctx context.Context, systemPrompt, userMessage, workDir string, onChunk ChunkCallback) (string, error)

	// RunStreamingWithTools executes the agent identically to RunStreaming,
	// but additionally calls onToolCall for each tool_use and tool_result
	// event parsed from the stream. Both callbacks may be nil.
	// Returns the full concatenated output and any error, same as RunStreaming.
	RunStreamingWithTools(ctx context.Context, systemPrompt, userMessage, workDir string, onChunk ChunkCallback, onToolCall ToolCallCallback) (string, error)
}
