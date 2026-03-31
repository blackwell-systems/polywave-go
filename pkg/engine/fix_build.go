package engine

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/agent/backend"
	apiclient "github.com/blackwell-systems/scout-and-wave-go/pkg/agent/backend/api"
	bedrockbackend "github.com/blackwell-systems/scout-and-wave-go/pkg/agent/backend/bedrock"
	openaibackend "github.com/blackwell-systems/scout-and-wave-go/pkg/agent/backend/openai"
	"github.com/blackwell-systems/scout-and-wave-go/pkg/protocol"
	"github.com/blackwell-systems/scout-and-wave-go/pkg/result"
)

// FixBuildFailure uses an AI agent to diagnose and fix a build/test/gate failure.
// The agent has full tool use (Read, Edit, Bash) and works in the repo directory.
// It streams progress via OnOutput and OnToolCall callbacks.
func FixBuildFailure(ctx context.Context, opts FixBuildOpts) result.Result[FixBuildData] {
	if opts.IMPLPath == "" {
		return result.NewFailure[FixBuildData]([]result.SAWError{
			result.NewFatal("ENGINE_FIX_BUILD_INVALID_OPTS", "engine.FixBuildFailure: IMPLPath is required"),
		})
	}
	if opts.RepoPath == "" {
		return result.NewFailure[FixBuildData]([]result.SAWError{
			result.NewFatal("ENGINE_FIX_BUILD_INVALID_OPTS", "engine.FixBuildFailure: RepoPath is required"),
		})
	}
	if opts.ErrorLog == "" {
		return result.NewFailure[FixBuildData]([]result.SAWError{
			result.NewFatal("ENGINE_FIX_BUILD_INVALID_OPTS", "engine.FixBuildFailure: ErrorLog is required"),
		})
	}

	manifest, err := protocol.Load(context.TODO(), opts.IMPLPath)
	if err != nil {
		return result.NewFailure[FixBuildData]([]result.SAWError{
			result.NewFatal("ENGINE_FIX_BUILD_FAILED", "engine.FixBuildFailure: load manifest failed").WithCause(err),
		})
	}

	b, err := selectFixBuildBackend(opts.ChatModel, opts.OnToolCall)
	if err != nil {
		return result.NewFailure[FixBuildData]([]result.SAWError{
			result.NewFatal("ENGINE_FIX_BUILD_FAILED", "engine.FixBuildFailure: select backend failed").WithCause(err),
		})
	}

	systemPrompt := buildFixBuildSystemPrompt()
	userMessage := buildFixBuildUserMessage(opts, manifest)

	_, err = b.RunStreamingWithTools(ctx, systemPrompt, userMessage, opts.RepoPath, func(chunk string) {
		if opts.OnOutput != nil {
			opts.OnOutput(chunk)
		}
	}, opts.OnToolCall)
	if err != nil {
		if ctx.Err() != nil {
			return result.NewFailure[FixBuildData]([]result.SAWError{
				{Code: "CONTEXT_CANCELLED", Message: "engine.FixBuildFailure: context cancelled", Severity: "fatal", Cause: err},
			})
		}
		return result.NewFailure[FixBuildData]([]result.SAWError{
			result.NewFatal("ENGINE_FIX_BUILD_FAILED", "engine.FixBuildFailure: agent execution failed").WithCause(err),
		})
	}

	return result.NewSuccess(FixBuildData{IMPLPath: opts.IMPLPath, WaveNum: opts.WaveNum, GateType: opts.GateType})
}

func buildFixBuildSystemPrompt() string {
	return `You are a build failure fixer for the Scout-and-Wave parallel agent system.

After wave agents complete their work and branches are merged, a quality gate (typecheck, test, lint, or build) has failed. Your job is to diagnose the root cause and apply a minimal fix.

RULES:
1. Read the error output carefully — most failures have a clear root cause
2. Use your tools to read the failing files, understand the context, and make targeted edits
3. Keep changes MINIMAL — fix only what's broken, don't refactor or improve
4. After fixing, run the verification command to confirm the fix works
5. If you cannot fix the issue (e.g., fundamental design problem), explain why clearly

COMMON FAILURE PATTERNS:
- Missing type fields (backend added a field, frontend type not updated)
- Import path errors after merge (wrong relative paths)
- Interface mismatches between agents' code
- Unused imports/variables after merge
- Test expectations that don't match merged behavior

After applying your fix, always run the verification command to confirm it passes.`
}

func buildFixBuildUserMessage(opts FixBuildOpts, manifest *protocol.IMPLManifest) string {
	var msg strings.Builder

	msg.WriteString(fmt.Sprintf("## Build Failure — Wave %d\n\n", opts.WaveNum))
	msg.WriteString(fmt.Sprintf("**Gate type:** %s\n", opts.GateType))
	msg.WriteString(fmt.Sprintf("**Repository:** %s\n\n", opts.RepoPath))

	// Include verification commands from manifest
	if manifest.TestCommand != "" {
		msg.WriteString(fmt.Sprintf("**Test command:** `%s`\n", manifest.TestCommand))
	}
	if manifest.LintCommand != "" {
		msg.WriteString(fmt.Sprintf("**Lint command:** `%s`\n", manifest.LintCommand))
	}

	// Include quality gates for the specific gate type
	for _, gate := range manifest.QualityGates.Gates {
		if gate.Type == opts.GateType || opts.GateType == "" {
			msg.WriteString(fmt.Sprintf("**Gate command:** `%s`\n", gate.Command))
		}
	}
	msg.WriteString("\n")

	// Include files changed in this wave for context
	if opts.WaveNum > 0 && opts.WaveNum <= len(manifest.Waves) {
		wave := manifest.Waves[opts.WaveNum-1]
		msg.WriteString("### Files changed in this wave\n")
		for _, agent := range wave.Agents {
			if report, ok := manifest.CompletionReports[agent.ID]; ok {
				for _, f := range report.FilesChanged {
					msg.WriteString(fmt.Sprintf("- %s (modified by Agent %s)\n", f, agent.ID))
				}
				for _, f := range report.FilesCreated {
					msg.WriteString(fmt.Sprintf("- %s (created by Agent %s)\n", f, agent.ID))
				}
			}
		}
		msg.WriteString("\n")
	}

	// Include the error log (truncated if very large)
	msg.WriteString("### Error output\n```\n")
	errorLog := opts.ErrorLog
	if len(errorLog) > 8000 {
		errorLog = errorLog[:4000] + "\n\n... (truncated) ...\n\n" + errorLog[len(errorLog)-4000:]
	}
	msg.WriteString(errorLog)
	msg.WriteString("\n```\n\n")

	msg.WriteString("Diagnose the root cause, fix it, and run the verification command to confirm the fix works.")

	return msg.String()
}

// selectFixBuildBackend selects a backend with full tool use enabled.
func selectFixBuildBackend(chatModel string, onToolCall func(ev backend.ToolCallEvent)) (backend.Backend, error) {
	defaultModel := "claude-sonnet-4-5"

	model := chatModel
	if model == "" {
		model = os.Getenv("SAW_FIX_BUILD_MODEL")
	}
	if model == "" {
		model = defaultModel
	}

	provider, bareModel := chatParseProviderPrefix(model)

	config := backend.Config{
		Model:      bareModel,
		MaxTurns:   50,    // Diagnosis + multi-file reads + edits + verify + iterate
		MaxTokens:  16384,
		OnToolCall: onToolCall,
	}

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
		return nil, fmt.Errorf("CLI backend not supported for build fix (use API backends only)")

	default:
		apiKey := os.Getenv("ANTHROPIC_API_KEY")
		if apiKey != "" {
			config.Model = model
			return apiclient.New(apiKey, config), nil
		}
		return nil, fmt.Errorf("no API key found for default backend (set ANTHROPIC_API_KEY or use provider prefix)")
	}
}
