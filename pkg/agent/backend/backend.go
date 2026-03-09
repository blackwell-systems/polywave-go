package backend

import "context"

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
}

// ChunkCallback is called with each text chunk as it arrives from the backend.
// Implementations must be safe to call from a goroutine.
// chunk is a raw text fragment (may be a partial word or sentence).
type ChunkCallback func(chunk string)

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
}
