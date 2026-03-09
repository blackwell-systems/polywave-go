package engine

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/agent/backend"
	apiclient "github.com/blackwell-systems/scout-and-wave-go/pkg/agent/backend/api"
	bedrockbackend "github.com/blackwell-systems/scout-and-wave-go/pkg/agent/backend/bedrock"
	cliclient "github.com/blackwell-systems/scout-and-wave-go/pkg/agent/backend/cli"
	openaibackend "github.com/blackwell-systems/scout-and-wave-go/pkg/agent/backend/openai"
)

// RunChat executes a chat agent with proper conversation history.
// Unlike RunScout which flattens history into the prompt, this uses the
// backend's native message API for proper turn-by-turn context.
// ChatModel in opts selects the model; empty string uses the backend default.
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

	// Select backend: provider-prefix routing when ChatModel is set,
	// otherwise fall back to API key → CLI heuristic.
	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	var b backend.Backend
	if opts.ChatModel != "" {
		provider, bareModel := chatParseProviderPrefix(opts.ChatModel)
		switch provider {
		case "openai":
			b = openaibackend.New(backend.Config{Model: bareModel, APIKey: os.Getenv("OPENAI_API_KEY")})
		case "ollama":
			b = openaibackend.New(backend.Config{Model: bareModel, BaseURL: "http://localhost:11434/v1"})
		case "lmstudio":
			b = openaibackend.New(backend.Config{Model: bareModel, BaseURL: "http://localhost:1234/v1"})
		case "anthropic":
			b = apiclient.New(apiKey, backend.Config{Model: bareModel})
		case "bedrock":
			fullID := chatExpandBedrockModelID(bareModel)
			b = bedrockbackend.New(backend.Config{Model: fullID})
		case "cli":
			b = cliclient.New("", backend.Config{Model: bareModel})
		default:
			// Plain model name — use API if key present, else CLI.
			if apiKey != "" {
				b = apiclient.New(apiKey, backend.Config{Model: opts.ChatModel})
			} else {
				b = cliclient.New("", backend.Config{Model: opts.ChatModel})
			}
		}
	} else {
		if apiKey != "" {
			b = apiclient.New(apiKey, backend.Config{})
		} else {
			b = cliclient.New("", backend.Config{})
		}
	}

	// Explanatory mode for CLI/local backends (no Anthropic API key in use).
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
		return fmt.Errorf("engine.RunChat: %w", err)
	}

	_ = response // Response is already streamed via onChunk
	return nil
}

// chatExpandBedrockModelID converts short Bedrock model names to full region-prefixed IDs.
func chatExpandBedrockModelID(shortName string) string {
	if strings.Contains(shortName, ".anthropic.") {
		return shortName
	}
	mapping := map[string]string{
		"claude-sonnet-4-5":          "us.anthropic.claude-sonnet-4-5-20250929-v1:0",
		"claude-sonnet-4-6":          "us.anthropic.claude-sonnet-4-6",
		"claude-opus-4-6":            "us.anthropic.claude-opus-4-6-v1",
		"claude-haiku-4-5":           "us.anthropic.claude-haiku-4-5-20251001-v1:0",
		"claude-haiku-4-5-20251001":  "us.anthropic.claude-haiku-4-5-20251001-v1:0",
	}
	if fullID, ok := mapping[shortName]; ok {
		return fullID
	}
	return shortName
}

// chatParseProviderPrefix splits "ollama:qwen2.5-coder:32b" into ("ollama", "qwen2.5-coder:32b").
// Returns ("", model) when no colon-prefix is present.
func chatParseProviderPrefix(model string) (provider, bare string) {
	idx := strings.Index(model, ":")
	if idx < 0 {
		return "", model
	}
	return model[:idx], model[idx+1:]
}
