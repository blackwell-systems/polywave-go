package engine

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/blackwell-systems/scout-and-wave-go/internal/git"
	"github.com/blackwell-systems/scout-and-wave-go/pkg/agent/backend"
	apiclient "github.com/blackwell-systems/scout-and-wave-go/pkg/agent/backend/api"
	bedrockbackend "github.com/blackwell-systems/scout-and-wave-go/pkg/agent/backend/bedrock"
	openaibackend "github.com/blackwell-systems/scout-and-wave-go/pkg/agent/backend/openai"
	"github.com/blackwell-systems/scout-and-wave-go/pkg/protocol"
	"github.com/blackwell-systems/scout-and-wave-go/pkg/result"
)

// ResolveData holds data returned by ResolveConflicts.
type ResolveData struct {
	FilesResolved []string `json:"files_resolved"`
	CommitCreated bool     `json:"commit_created"`
}

// ResolveFileData holds data returned by resolveConflictedFile.
type ResolveFileData struct {
	File string `json:"file"`
}

// ResolveConflicts resolves all conflicted files in a merge using Claude.
// It reads conflicted files from git, builds prompts with IMPL context,
// calls Claude to resolve each file, writes resolved content back, and commits.
//
// Returns fatal result on first file that cannot be resolved (partial failure aborts).
func ResolveConflicts(ctx context.Context, opts ResolveConflictsOpts) result.Result[ResolveData] {
	if opts.IMPLPath == "" {
		return result.NewFailure[ResolveData]([]result.SAWError{
			result.NewFatal(result.CodeResolveInvalidOpts,
				"engine.ResolveConflicts: IMPLPath is required"),
		})
	}
	if opts.RepoPath == "" {
		return result.NewFailure[ResolveData]([]result.SAWError{
			result.NewFatal(result.CodeResolveInvalidOpts,
				"engine.ResolveConflicts: RepoPath is required"),
		})
	}

	// Load IMPL manifest for agent context
	manifest, err := protocol.Load(ctx, opts.IMPLPath)
	if err != nil {
		return result.NewFailure[ResolveData]([]result.SAWError{
			result.NewFatal(result.CodeResolveLoadFailed,
				fmt.Sprintf("engine.ResolveConflicts: failed to load IMPL manifest: %v", err)).
				WithContext("impl_path", opts.IMPLPath),
		})
	}

	// Get list of conflicted files
	conflictedFiles, err := git.ConflictedFiles(opts.RepoPath)
	if err != nil {
		return result.NewFailure[ResolveData]([]result.SAWError{
			result.NewFatal(result.CodeResolveGitFailed,
				fmt.Sprintf("engine.ResolveConflicts: failed to get conflicted files: %v", err)).
				WithContext("repo_path", opts.RepoPath),
		})
	}

	if len(conflictedFiles) == 0 {
		return result.NewFailure[ResolveData]([]result.SAWError{
			result.NewFatal(result.CodeResolveNoConflicts,
				"engine.ResolveConflicts: no conflicted files found"),
		})
	}

	// Select backend for conflict resolution
	b, err := selectConflictResolutionBackend(opts.ChatModel)
	if err != nil {
		return result.NewFailure[ResolveData]([]result.SAWError{
			result.NewFatal(result.CodeResolveBackendFailed,
				fmt.Sprintf("engine.ResolveConflicts: failed to select backend: %v", err)),
		})
	}

	// Resolve each conflicted file
	var resolved []string
	for _, file := range conflictedFiles {
		if opts.OnProgress != nil {
			opts.OnProgress(file, "resolving")
		}

		fileRes := resolveConflictedFile(ctx, file, manifest, opts, b)
		if fileRes.IsFatal() {
			return result.NewFailure[ResolveData]([]result.SAWError{
				result.NewFatal(result.CodeResolveFileFailed,
					fmt.Sprintf("engine.ResolveConflicts: failed to resolve %s: %s", file, fileRes.Errors[0].Message)).
					WithContext("file", file),
			})
		}

		resolved = append(resolved, file)

		if opts.OnProgress != nil {
			opts.OnProgress(file, "resolved")
		}
	}

	// Commit the merge
	if _, err := git.Run(opts.RepoPath, "commit", "--no-edit"); err != nil {
		return result.NewFailure[ResolveData]([]result.SAWError{
			result.NewFatal(result.CodeResolveCommitFailed,
				fmt.Sprintf("engine.ResolveConflicts: failed to commit merge: %v", err)).
				WithContext("repo_path", opts.RepoPath),
		})
	}

	return result.NewSuccess(ResolveData{
		FilesResolved: resolved,
		CommitCreated: true,
	})
}

// resolveConflictedFile resolves a single conflicted file using Claude.
func resolveConflictedFile(ctx context.Context, file string, manifest *protocol.IMPLManifest, opts ResolveConflictsOpts, b backend.Backend) result.Result[ResolveFileData] {
	// Read conflicted file content
	filePath := filepath.Join(opts.RepoPath, file)
	content, err := os.ReadFile(filePath)
	if err != nil {
		return result.NewFailure[ResolveFileData]([]result.SAWError{
			result.NewFatal(result.CodeResolveFileReadFailed,
				fmt.Sprintf("failed to read conflicted file: %v", err)).
				WithContext("file", file),
		})
	}

	// Build context from IMPL manifest
	agentCtx := buildAgentContext(file, manifest, opts.WaveNum)

	// Build prompts
	systemPrompt := buildSystemPrompt()
	userMessage := buildUserMessage(string(content), file, agentCtx)

	// Call Claude to resolve the conflict (streaming for live output)
	resolvedContent, err := b.RunStreaming(ctx, systemPrompt, userMessage, opts.RepoPath, func(chunk string) {
		if opts.OnOutput != nil {
			opts.OnOutput(chunk)
		}
	})
	if err != nil {
		return result.NewFailure[ResolveFileData]([]result.SAWError{
			result.NewFatal(result.CodeResolveBackendCallFailed,
				fmt.Sprintf("backend call failed: %v", err)).
				WithContext("file", file),
		})
	}

	// Write resolved content back to file
	if err := os.WriteFile(filePath, []byte(resolvedContent), 0644); err != nil {
		return result.NewFailure[ResolveFileData]([]result.SAWError{
			result.NewFatal(result.CodeResolveFileWriteFailed,
				fmt.Sprintf("failed to write resolved file: %v", err)).
				WithContext("file", file),
		})
	}

	// Stage the resolved file
	if _, err := git.Run(opts.RepoPath, "add", file); err != nil {
		return result.NewFailure[ResolveFileData]([]result.SAWError{
			result.NewFatal(result.CodeResolveGitAddFailed,
				fmt.Sprintf("git add failed: %v", err)).
				WithContext("file", file),
		})
	}

	return result.NewSuccess(ResolveFileData{File: file})
}

// agentContextInfo holds agent context for a file.
type agentContextInfo struct {
	Owners            []string // agent IDs that own this file
	AgentTasks        map[string]string
	RelevantContracts []protocol.InterfaceContract
}

// buildAgentContext extracts relevant agent and contract information from the manifest.
func buildAgentContext(file string, manifest *protocol.IMPLManifest, waveNum int) agentContextInfo {
	ctx := agentContextInfo{
		Owners:            make([]string, 0),
		AgentTasks:        make(map[string]string),
		RelevantContracts: make([]protocol.InterfaceContract, 0),
	}

	// Find agents that own this file
	for _, fo := range manifest.FileOwnership {
		if fo.File == file && fo.Wave <= waveNum {
			ctx.Owners = append(ctx.Owners, fo.Agent)
		}
	}

	// Get task descriptions for owning agents
	for _, wave := range manifest.Waves {
		for _, agent := range wave.Agents {
			for _, owner := range ctx.Owners {
				if agent.ID == owner {
					ctx.AgentTasks[agent.ID] = agent.Task
				}
			}
		}
	}

	// Find relevant interface contracts (for now, include all contracts)
	// TODO: Filter contracts based on file location or agent dependencies
	ctx.RelevantContracts = manifest.InterfaceContracts

	return ctx
}

// buildSystemPrompt creates the system prompt for conflict resolution.
func buildSystemPrompt() string {
	return `You are resolving a git merge conflict. Your task is to analyze the conflicting changes and produce a resolved version that preserves the intent of both sides when possible.

CRITICAL RULES:
1. Output ONLY the resolved file content
2. Remove ALL conflict markers (<<<<<<< ======= >>>>>>>)
3. NO explanations, NO markdown fences, NO commentary
4. The output will be written directly to the file

When resolving conflicts:
- Prefer combining features from both sides rather than choosing one
- Preserve the implementation intent of each agent
- Maintain code consistency and style
- Ensure the result compiles and makes logical sense`
}

// buildUserMessage creates the user message with conflicted content and context.
func buildUserMessage(conflictedContent, file string, ctx agentContextInfo) string {
	var msg strings.Builder

	msg.WriteString(fmt.Sprintf("File: %s\n\n", file))

	// Add agent context if available
	if len(ctx.Owners) > 0 {
		msg.WriteString("Agents that worked on this file:\n")
		for _, owner := range ctx.Owners {
			task := ctx.AgentTasks[owner]
			if task != "" {
				msg.WriteString(fmt.Sprintf("- Agent %s: %s\n", owner, task))
			} else {
				msg.WriteString(fmt.Sprintf("- Agent %s\n", owner))
			}
		}
		msg.WriteString("\n")
	}

	// Add relevant interface contracts
	if len(ctx.RelevantContracts) > 0 {
		msg.WriteString("Relevant Interface Contracts:\n")
		for _, contract := range ctx.RelevantContracts {
			msg.WriteString(fmt.Sprintf("- %s: %s\n", contract.Name, contract.Description))
			if contract.Definition != "" {
				msg.WriteString(fmt.Sprintf("  Definition: %s\n", contract.Definition))
			}
		}
		msg.WriteString("\n")
	}

	msg.WriteString("Conflicted file content:\n")
	msg.WriteString("```\n")
	msg.WriteString(conflictedContent)
	msg.WriteString("\n```\n\n")

	msg.WriteString("Resolve the conflict and output ONLY the resolved file content (no markers, no explanation).")

	return msg.String()
}

// selectConflictResolutionBackend selects the appropriate backend for conflict resolution.
func selectConflictResolutionBackend(chatModel string) (backend.Backend, error) {
	// Default model for conflict resolution (cost-effective)
	defaultModel := "claude-sonnet-4-5"

	// Parse model string to determine provider
	model := chatModel
	if model == "" {
		model = os.Getenv("SAW_CONFLICT_MODEL")
	}
	if model == "" {
		model = defaultModel
	}

	provider, bareModel := chatParseProviderPrefix(model)

	// Configure backend with single-shot settings
	config := backend.Config{
		Model:     bareModel,
		MaxTurns:  1,     // Single-shot, no tool loop
		MaxTokens: 16384, // Conflicts can be large
		ReadOnly:  true,  // No file writes via tools
	}

	// Select backend based on provider
	switch provider {
	case "openai":
		config.APIKey = os.Getenv("OPENAI_API_KEY")
		return openaibackend.New(config), nil

	case "ollama":
		config.BaseURL = "http://localhost:11434/v1"
		return openaibackend.New(config), nil

	case "lmstudio":
		config.BaseURL = "http://localhost:1234/v1"
		return openaibackend.New(config), nil

	case "anthropic":
		apiKey := os.Getenv("ANTHROPIC_API_KEY")
		if apiKey == "" {
			return nil, fmt.Errorf("ANTHROPIC_API_KEY not set")
		}
		return apiclient.New(apiKey, config), nil

	case "bedrock":
		fullID := chatExpandBedrockModelID(bareModel)
		config.Model = fullID
		return bedrockbackend.New(config), nil

	case "cli":
		return nil, fmt.Errorf("CLI backend not supported for conflict resolution (use API backends only)")

	default:
		// No prefix: use Anthropic if API key is available
		apiKey := os.Getenv("ANTHROPIC_API_KEY")
		if apiKey != "" {
			config.Model = model
			return apiclient.New(apiKey, config), nil
		}
		return nil, fmt.Errorf("no API key found for default backend (set ANTHROPIC_API_KEY or use provider prefix)")
	}
}
