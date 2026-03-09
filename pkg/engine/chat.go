package engine

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/agent/backend"
	"github.com/blackwell-systems/scout-and-wave-go/pkg/agent/backend/api"
	"github.com/blackwell-systems/scout-and-wave-go/pkg/agent/backend/cli"
)

// RunChat executes a chat agent with proper conversation history.
// Unlike RunScout which flattens history into the prompt, this uses the
// backend's native message API for proper turn-by-turn context.
func RunChat(ctx context.Context, opts RunChatOpts, onChunk func(string)) error {
	if opts.Message == "" {
		return fmt.Errorf("engine.RunChat: Message is required")
	}
	if opts.RepoPath == "" {
		return fmt.Errorf("engine.RunChat: RepoPath is required")
	}
	if opts.IMPLPath == "" {
		return fmt.Errorf("engine.RunChat: IMPLPath is required")
	}

	// Resolve SAW repo path.
	sawRepo := opts.SAWRepoPath
	if sawRepo == "" {
		sawRepo = os.Getenv("SAW_REPO")
	}
	if sawRepo == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("engine.RunChat: cannot determine home directory: %w", err)
		}
		sawRepo = filepath.Join(home, "code", "scout-and-wave")
	}

	// Build system prompt with IMPL doc context and instructions.
	systemPrompt := fmt.Sprintf(`You are an expert software architect answering questions about a Scout-and-Wave IMPL doc.
Read the IMPL doc at: %s
Use the Read tool to read it, then answer the user's question concisely.
You MUST NOT modify the IMPL doc or any source files. Read-only.`, opts.IMPLPath)

	// If using CLI backend (no API key), enable explanatory output mode.
	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	if apiKey == "" {
		systemPrompt += `

# Output Style: Explanatory

You are in explanatory mode. Before and after answering questions, provide brief educational insights about the IMPL doc structure, SAW protocol concepts, or implementation patterns using:

` + "`★ Insight ─────────────────────────────────────`" + `
[2-3 key educational points about what you observed]
` + "`─────────────────────────────────────────────────`" + `

Focus on interesting insights specific to this IMPL doc rather than general programming concepts. Help the user learn about SAW protocol patterns, wave structure decisions, interface contract design, and agent coordination strategies.`
	}

	// Select backend based on API key presence.
	var b backend.Backend
	if apiKey != "" {
		b = api.New(apiKey, backend.Config{})
	} else {
		b = cli.New("", backend.Config{})
	}

	// Format history into the system prompt for now.
	// TODO: Extend backend.Backend interface to accept message arrays for proper multi-turn support.
	if len(opts.History) > 0 {
		systemPrompt += "\n\nPrevious conversation:\n"
		for _, msg := range opts.History {
			systemPrompt += fmt.Sprintf("%s: %s\n", msg.Role, msg.Content)
		}
	}
	systemPrompt += fmt.Sprintf("\n\nUser question: %s", opts.Message)

	// Use the backend directly for streaming.
	response, err := b.RunStreaming(ctx, systemPrompt, "Begin now.", opts.RepoPath, onChunk)
	if err != nil {
		return fmt.Errorf("engine.RunChat: %w", err)
	}

	_ = response // Response is already streamed via onChunk
	return nil
}
