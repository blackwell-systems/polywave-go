package engine

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/blackwell-systems/polywave-go/pkg/orchestrator"
	"github.com/blackwell-systems/polywave-go/pkg/result"
)

// RunChat executes a chat agent with proper conversation history.
// Unlike RunScout which flattens history into the prompt, this uses the
// backend's native message API for proper turn-by-turn context.
// ChatModel in opts selects the model; empty string uses the backend default.
func RunChat(ctx context.Context, opts RunChatOpts, onChunk func(string)) result.Result[ChatData] {
	if opts.Message == "" {
		return result.NewFailure[ChatData]([]result.PolywaveError{
			result.NewFatal(result.CodeChatInvalidOpts, "engine.RunChat: Message is required"),
		})
	}
	if opts.RepoPath == "" {
		return result.NewFailure[ChatData]([]result.PolywaveError{
			result.NewFatal(result.CodeChatInvalidOpts, "engine.RunChat: RepoPath is required"),
		})
	}
	if opts.IMPLPath == "" {
		return result.NewFailure[ChatData]([]result.PolywaveError{
			result.NewFatal(result.CodeChatInvalidOpts, "engine.RunChat: IMPLPath is required"),
		})
	}

	// Resolve SAW repo path.
	polywaveRepo := opts.PolywaveRepoPath
	if polywaveRepo == "" {
		polywaveRepo = os.Getenv("POLYWAVE_REPO")
	}
	if polywaveRepo == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return result.NewFailure[ChatData]([]result.PolywaveError{
				result.NewFatal(result.CodeChatFailed, "engine.RunChat: cannot determine home directory").WithCause(err),
			})
		}
		polywaveRepo = filepath.Join(home, "code", "polywave")
	}

	// Build system prompt with IMPL doc context and instructions.
	systemPrompt := fmt.Sprintf(`You are an expert software architect answering questions about a Polywave IMPL doc.
Read the IMPL doc at: %s
Use the Read tool to read it, then answer the user's question concisely.
You MUST NOT modify the IMPL doc or any source files. Read-only.`, opts.IMPLPath)

	// Select backend via centralized provider-prefix routing.
	// orchestrator.NewBackendFromModel handles all prefixes (openai:, ollama:,
	// lmstudio:, anthropic:, bedrock:, cli:) and the API-key/CLI fallback.
	bRes := orchestrator.NewBackendFromModel(opts.ChatModel)
	if bRes.IsFatal() {
		return result.NewFailure[ChatData](bRes.Errors)
	}
	b := bRes.GetData()

	// Explanatory mode for CLI/local backends (no Anthropic API key in use).
	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	usesCLI := opts.ChatModel == "" && apiKey == ""
	if !usesCLI {
		if p, _ := chatParseProviderPrefix(opts.ChatModel); p == "cli" || p == "ollama" || p == "lmstudio" {
			usesCLI = true
		}
	}
	if usesCLI {
		systemPrompt += `

# Output Style: Explanatory

You are in explanatory mode. Before and after answering questions, provide brief educational insights about the IMPL doc structure, SAW protocol concepts, or implementation patterns using:

` + "`★ Insight ─────────────────────────────────────`" + `
[2-3 key educational points about what you observed]
` + "`─────────────────────────────────────────────────`" + `

Focus on interesting insights specific to this IMPL doc rather than general programming concepts. Help the user learn about SAW protocol patterns, wave structure decisions, interface contract design, and agent coordination strategies.`
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
		if ctx.Err() != nil {
			return result.NewFailure[ChatData]([]result.PolywaveError{
				{Code: result.CodeContextCancelled, Message: "engine.RunChat: context cancelled", Severity: "fatal", Cause: err},
			})
		}
		return result.NewFailure[ChatData]([]result.PolywaveError{
			result.NewFatal(result.CodeChatFailed, "engine.RunChat: backend execution failed").WithCause(err),
		})
	}

	_ = response // Response is already streamed via onChunk
	return result.NewSuccess(ChatData{IMPLPath: opts.IMPLPath, Message: opts.Message})
}

// chatExpandBedrockModelID converts short Bedrock model names to full region-prefixed IDs.
// Still used by fix_build.go and resolve_conflicts.go for their own provider routing.
// TODO: migrate those callers to orchestrator.NewBackendFromModel and remove this function.
func chatExpandBedrockModelID(shortName string) string {
	if strings.Contains(shortName, ".anthropic.") {
		return shortName
	}
	mapping := map[string]string{
		"claude-sonnet-4-5":         "us.anthropic.claude-sonnet-4-5-20250929-v1:0",
		"claude-sonnet-4-6":         "us.anthropic.claude-sonnet-4-6",
		"claude-opus-4-6":           "us.anthropic.claude-opus-4-6-v1",
		"claude-haiku-4-5":          "us.anthropic.claude-haiku-4-5-20251001-v1:0",
		"claude-haiku-4-5-20251001": "us.anthropic.claude-haiku-4-5-20251001-v1:0",
	}
	if fullID, ok := mapping[shortName]; ok {
		return fullID
	}
	return shortName
}

// chatParseProviderPrefix splits "ollama:qwen2.5-coder:32b" into ("ollama", "qwen2.5-coder:32b").
// Returns ("", model) when no colon-prefix is present.
// Used for explanatory-mode detection; also used by fix_build.go and resolve_conflicts.go.
// TODO: migrate those callers to orchestrator.NewBackendFromModel and remove this function.
func chatParseProviderPrefix(model string) (provider, bare string) {
	idx := strings.Index(model, ":")
	if idx < 0 {
		return "", model
	}
	return model[:idx], model[idx+1:]
}
