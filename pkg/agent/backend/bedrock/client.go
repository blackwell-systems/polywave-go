// Package bedrock provides a backend.Backend implementation that uses AWS Bedrock
// via the AWS SDK v2. It supports streaming responses and uses AWS credentials
// from the default credential chain (~/.aws/credentials, env vars, IAM roles).
package bedrock

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime/document"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime/types"

	"github.com/blackwell-systems/polywave-go/pkg/agent/backend"
	"github.com/blackwell-systems/polywave-go/pkg/agent/dedup"
	"github.com/blackwell-systems/polywave-go/pkg/tools"
)

// Client implements backend.Backend using AWS Bedrock.
type Client struct {
	client        *bedrockruntime.Client
	cfg           backend.Config
	onToolCall    backend.ToolCallCallback
	readOnly      bool
	commitTracker *tools.CommitTracker
	dedupCache    *dedup.Cache
	outputSchema  map[string]any // optional: structured output schema
	logger        *slog.Logger
	mu            sync.Mutex // protects dedupCache and commitTracker
}

// SetLogger configures the logger used for debug traces and diagnostics.
// If never called, the client falls back to slog.Default().
func (c *Client) SetLogger(logger *slog.Logger) {
	c.logger = logger
}

// log returns the configured logger, falling back to slog.Default() if nil.
func (c *Client) log() *slog.Logger {
	if c.logger == nil {
		return slog.Default()
	}
	return c.logger
}

// New creates a Bedrock backend client using AWS credentials from the default chain.
// If cfg.BedrockAccessKeyID and cfg.BedrockSecretAccessKey are non-empty, static
// credentials are used instead of the default chain. If cfg.BedrockRegion is
// non-empty, it overrides the AWS region.
func New(cfg backend.Config) *Client {
	// Build config options for AWS SDK
	var opts []func(*config.LoadOptions) error

	// Fall back to polywave.config.json when no explicit credentials provided
	if cfg.BedrockProfile == "" && cfg.BedrockRegion == "" && cfg.BedrockAccessKeyID == "" {
		cwd, _ := os.Getwd()
		providers := backend.LoadProvidersFromConfig(cwd)
		if providers.Bedrock.Profile != "" {
			cfg.BedrockProfile = providers.Bedrock.Profile
		}
		if providers.Bedrock.Region != "" && cfg.BedrockRegion == "" {
			cfg.BedrockRegion = providers.Bedrock.Region
		}
		if providers.Bedrock.AccessKeyID != "" {
			cfg.BedrockAccessKeyID = providers.Bedrock.AccessKeyID
			cfg.BedrockSecretAccessKey = providers.Bedrock.SecretAccessKey
			cfg.BedrockSessionToken = providers.Bedrock.SessionToken
		}
	}

	// Use explicit region if provided
	if cfg.BedrockRegion != "" {
		opts = append(opts, config.WithRegion(cfg.BedrockRegion))
	}

	// Use named profile if provided (supports SSO, assume-role, etc.)
	if cfg.BedrockProfile != "" {
		opts = append(opts, config.WithSharedConfigProfile(cfg.BedrockProfile))
	}

	// Use static credentials if provided
	if cfg.BedrockAccessKeyID != "" && cfg.BedrockSecretAccessKey != "" {
		opts = append(opts, config.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(
				cfg.BedrockAccessKeyID,
				cfg.BedrockSecretAccessKey,
				cfg.BedrockSessionToken,
			),
		))
	}

	// Load AWS config from environment/credentials (with optional overrides)
	awsCfg, err := config.LoadDefaultConfig(context.Background(), opts...)
	if err != nil {
		// Fall back to creating client without config - will fail on invoke but allows construction
		return &Client{cfg: cfg}
	}

	return &Client{
		client:     bedrockruntime.NewFromConfig(awsCfg),
		cfg:        cfg,
		onToolCall: cfg.OnToolCall,
		readOnly:   cfg.ReadOnly,
	}
}

// WithOutputConfig configures structured output by providing a JSON schema.
// Returns the client for method chaining.
func (c *Client) WithOutputConfig(schema map[string]any) *Client {
	c.outputSchema = schema
	return c
}

// Run sends a non-streaming request to Bedrock using the Converse API.
func (c *Client) Run(ctx context.Context, systemPrompt, userPrompt, _ string) (string, error) {
	if c.client == nil {
		return "", fmt.Errorf("bedrock: AWS config failed to load")
	}

	// Build Converse API request
	input := &bedrockruntime.ConverseInput{
		ModelId: aws.String(c.cfg.Model),
		Messages: []types.Message{
			{
				Role: types.ConversationRoleUser,
				Content: []types.ContentBlock{
					&types.ContentBlockMemberText{
						Value: userPrompt,
					},
				},
			},
		},
		InferenceConfig: &types.InferenceConfiguration{
			MaxTokens: aws.Int32(int32(c.maxTokens())),
		},
	}

	// Add system prompt if provided
	if systemPrompt != "" {
		input.System = buildSystemBlocks(systemPrompt)
	}

	// Add structured output configuration if schema is set
	if c.outputSchema != nil {
		outputConfig, err := buildOutputConfig(c.outputSchema)
		if err != nil {
			return "", fmt.Errorf("bedrock: buildOutputConfig: %w", err)
		}
		input.OutputConfig = outputConfig
	}

	resp, err := c.client.Converse(ctx, input)
	if err != nil {
		return "", fmt.Errorf("bedrock: Converse: %w", err)
	}

	// Extract text from response
	text := extractTextFromOutput(resp.Output)
	if text == "" {
		return "", fmt.Errorf("bedrock: empty response content")
	}

	return text, nil
}

// RunStreaming sends a streaming request to Bedrock using ConverseStream API.
func (c *Client) RunStreaming(ctx context.Context, systemPrompt, userPrompt, _ string, onChunk backend.ChunkCallback) (string, error) {
	if c.client == nil {
		return "", fmt.Errorf("bedrock: AWS config failed to load")
	}

	// Build ConverseStream API request
	input := &bedrockruntime.ConverseStreamInput{
		ModelId: aws.String(c.cfg.Model),
		Messages: []types.Message{
			{
				Role: types.ConversationRoleUser,
				Content: []types.ContentBlock{
					&types.ContentBlockMemberText{
						Value: userPrompt,
					},
				},
			},
		},
		InferenceConfig: &types.InferenceConfiguration{
			MaxTokens: aws.Int32(int32(c.maxTokens())),
		},
	}

	// Add system prompt if provided
	if systemPrompt != "" {
		input.System = buildSystemBlocks(systemPrompt)
	}

	// Add structured output configuration if schema is set
	if c.outputSchema != nil {
		outputConfig, err := buildOutputConfig(c.outputSchema)
		if err != nil {
			return "", fmt.Errorf("bedrock: buildOutputConfig: %w", err)
		}
		input.OutputConfig = outputConfig
	}

	resp, err := c.client.ConverseStream(ctx, input)
	if err != nil {
		return "", fmt.Errorf("bedrock: ConverseStream: %w", err)
	}

	stream := resp.GetStream()
	defer stream.Close()

	var fullText strings.Builder

	for event := range stream.Events() {
		switch v := event.(type) {
		case *types.ConverseStreamOutputMemberContentBlockStart:
			// Content block started - no action needed for text blocks

		case *types.ConverseStreamOutputMemberContentBlockDelta:
			if v.Value.Delta != nil {
				if textDelta, ok := v.Value.Delta.(*types.ContentBlockDeltaMemberText); ok {
					text := textDelta.Value
					fullText.WriteString(text)
					if onChunk != nil {
						onChunk(text)
					}
				}
			}

		case *types.ConverseStreamOutputMemberMessageStop:
			// Message complete

		case *types.ConverseStreamOutputMemberMetadata:
			// Metadata event - ignore

		default:
			// Ignore other event types
		}
	}

	if err := stream.Err(); err != nil {
		return fullText.String(), fmt.Errorf("bedrock: stream error: %w", err)
	}

	return fullText.String(), nil
}

// buildWorkshop creates a Workshop with middleware applied based on client config.
// Uses the shared backend.BuildWorkshop helper for consistent middleware ordering.
func (c *Client) buildWorkshop(workDir string) tools.Workshop {
	c.mu.Lock()
	defer c.mu.Unlock()
	result := backend.BuildWorkshop(workDir, c.readOnly, c.cfg)
	c.dedupCache = result.DedupCache
	c.commitTracker = result.CommitTracker
	return result.Workshop
}

// CommitCount returns the number of git commits tracked by the constraint
// middleware. Returns 0 if constraints are not configured or no commits detected.
func (c *Client) CommitCount() int {
	if c.commitTracker == nil {
		return 0
	}
	return int(atomic.LoadInt64(&c.commitTracker.Count))
}

// DedupStats returns dedup metrics from the most recent Run call.
// Returns nil if no run has been made yet or dedup is not configured.
func (c *Client) DedupStats() *dedup.Stats {
	if c.dedupCache == nil {
		return nil
	}
	stats := c.dedupCache.Stats()
	return &stats
}

// toolBlock tracks state for a tool_use content block being streamed.
type toolBlock struct {
	id        string
	name      string
	inputJSON strings.Builder
}

// RunStreamingWithTools implements backend.Backend with full multi-turn tool-use loop using ConverseStream API.
// Streams text deltas via onChunk, emits tool call events via onToolCall.
// Loops until stop_reason is "end_turn" or maxTurns exceeded.
func (c *Client) RunStreamingWithTools(ctx context.Context, systemPrompt, userPrompt, workDir string, onChunk backend.ChunkCallback, onToolCall backend.ToolCallCallback) (string, error) {
	if c.client == nil {
		return "", fmt.Errorf("bedrock: AWS config failed to load")
	}

	workshop := c.buildWorkshop(workDir)

	// Build typed message history
	messages := []types.Message{
		{
			Role: types.ConversationRoleUser,
			Content: []types.ContentBlock{
				&types.ContentBlockMemberText{
					Value: userPrompt,
				},
			},
		},
	}

	maxT := c.maxTurns()

	for turn := 0; turn < maxT; turn++ {
		// Build ConverseStream request
		input := &bedrockruntime.ConverseStreamInput{
			ModelId:  aws.String(c.cfg.Model),
			Messages: messages,
			InferenceConfig: &types.InferenceConfiguration{
				MaxTokens: aws.Int32(int32(c.maxTokens())),
			},
			ToolConfig: buildConverseTools(workshop),
		}

		// Add system prompt if provided
		if systemPrompt != "" {
			input.System = buildSystemBlocks(systemPrompt)
		}

		// Add structured output configuration on final turn (when no tool use expected)
		if c.outputSchema != nil && turn > 0 {
			outputConfig, err := buildOutputConfig(c.outputSchema)
			if err != nil {
				return "", fmt.Errorf("bedrock: buildOutputConfig (turn %d): %w", turn, err)
			}
			input.OutputConfig = outputConfig
		}

		resp, err := c.client.ConverseStream(ctx, input)
		if err != nil {
			return "", fmt.Errorf("bedrock: ConverseStream (turn %d): %w", turn, err)
		}

		stream := resp.GetStream()

		var fullText strings.Builder
		var stopReason types.StopReason
		blockMap := make(map[int]*toolBlock)              // index -> tool block
		textBlocks := make(map[int]*strings.Builder)      // index -> text accumulator
		currentIndex := 0
		nextAutoIndex := 0 // auto-increment when ContentBlockIndex is nil

		for event := range stream.Events() {
			switch v := event.(type) {
			case *types.ConverseStreamOutputMemberContentBlockStart:
				if v.Value.ContentBlockIndex != nil {
					currentIndex = int(*v.Value.ContentBlockIndex)
				} else {
					// ContentBlockIndex is nil; auto-increment based on blocks seen.
					currentIndex = nextAutoIndex
				}
				nextAutoIndex = currentIndex + 1

				if v.Value.Start != nil {
					if toolUse, ok := v.Value.Start.(*types.ContentBlockStartMemberToolUse); ok {
						blockMap[currentIndex] = &toolBlock{
							id:   aws.ToString(toolUse.Value.ToolUseId),
							name: aws.ToString(toolUse.Value.Name),
						}
					}
				}
				// Initialize text block accumulator for non-tool blocks
				if _, exists := blockMap[currentIndex]; !exists {
					textBlocks[currentIndex] = &strings.Builder{}
				}

			case *types.ConverseStreamOutputMemberContentBlockDelta:
				if v.Value.ContentBlockIndex != nil {
					currentIndex = int(*v.Value.ContentBlockIndex)
				}
				if v.Value.Delta != nil {
					switch delta := v.Value.Delta.(type) {
					case *types.ContentBlockDeltaMemberText:
						text := delta.Value
						fullText.WriteString(text)
						if tb, ok := textBlocks[currentIndex]; ok {
							tb.WriteString(text)
						}
						if onChunk != nil {
							onChunk(text)
						}
					case *types.ContentBlockDeltaMemberToolUse:
						if tb, ok := blockMap[currentIndex]; ok {
							tb.inputJSON.WriteString(aws.ToString(delta.Value.Input))
						}
					}
				}

			case *types.ConverseStreamOutputMemberMessageStop:
				if v.Value.StopReason != "" {
					stopReason = v.Value.StopReason
				}

			case *types.ConverseStreamOutputMemberMetadata:
				// Metadata event - ignore

			default:
				// Ignore other event types
			}
		}

		if err := stream.Err(); err != nil {
			stream.Close()
			return fullText.String(), fmt.Errorf("bedrock: stream error (turn %d): %w", turn, err)
		}
		stream.Close()

		if stopReason == types.StopReasonEndTurn {
			c.log().Debug("bedrock: end_turn", "turn", turn, "tool_calls", len(blockMap), "text_len", fullText.Len())
			return fullText.String(), nil
		}

		if stopReason == types.StopReasonMaxTokens {
			// Treat max_tokens like end_turn for tool-use turns: the model ran out of
			// output space. Continue the loop by sending what we have as an assistant
			// message so the model can resume. If there are pending tool_use blocks,
			// execute them; otherwise just continue the conversation.
			if len(blockMap) == 0 {
				// No tool calls in this turn — send accumulated text back and let
				// the model continue from where it left off.
				if fullText.Len() > 0 {
					messages = append(messages, types.Message{
						Role: types.ConversationRoleAssistant,
						Content: []types.ContentBlock{
							&types.ContentBlockMemberText{
								Value: fullText.String(),
							},
						},
					})
					messages = append(messages, types.Message{
						Role: types.ConversationRoleUser,
						Content: []types.ContentBlock{
							&types.ContentBlockMemberText{
								Value: "Continue from where you left off.",
							},
						},
					})
				}
				continue
			}
			// Fall through to tool execution — treat the pending tool_use blocks normally
		} else if stopReason != types.StopReasonToolUse {
			return fullText.String(), fmt.Errorf("bedrock: unexpected stop reason (turn %d): %s", turn, stopReason)
		}

		// Build assistant content blocks for the message history
		assistantContent := make([]types.ContentBlock, 0)

		// Add blocks in index order
		for i := 0; i <= currentIndex; i++ {
			if tb, ok := textBlocks[i]; ok {
				text := tb.String()
				if text != "" {
					assistantContent = append(assistantContent, &types.ContentBlockMemberText{
						Value: text,
					})
				}
			}
			if toolBlk, ok := blockMap[i]; ok {
				inputStr := toolBlk.inputJSON.String()
				if inputStr == "" {
					inputStr = "{}"
				}
				// Parse JSON string into a Go value so the SDK serializes it as
				// a JSON object, not as raw bytes (which Bedrock rejects).
				var inputObj interface{}
				if err := json.Unmarshal([]byte(inputStr), &inputObj); err != nil {
					inputObj = map[string]interface{}{}
				}
				inputDoc := document.NewLazyDocument(inputObj)
				assistantContent = append(assistantContent, &types.ContentBlockMemberToolUse{
					Value: types.ToolUseBlock{
						ToolUseId: aws.String(toolBlk.id),
						Name:      aws.String(toolBlk.name),
						Input:     inputDoc,
					},
				})
			}
		}

		messages = append(messages, types.Message{
			Role:    types.ConversationRoleAssistant,
			Content: assistantContent,
		})

		// Execute tools and build tool_result blocks
		toolResults := make([]types.ContentBlock, 0)
		for _, toolBlk := range blockMap {
			inputStr := toolBlk.inputJSON.String()
			if inputStr == "" {
				inputStr = "{}"
			}
			var inputMap map[string]interface{}
			if err := json.Unmarshal([]byte(inputStr), &inputMap); err != nil {
				inputMap = map[string]interface{}{}
			}

			// Emit tool call event
			if onToolCall != nil {
				onToolCall(backend.ToolCallEvent{
					ID:    toolBlk.id,
					Name:  toolBlk.name,
					Input: inputStr,
				})
			}

			result, execErr := backend.ExecuteTool(ctx, workshop, toolBlk.name, inputMap, workDir)
			isError := execErr != nil

			// Debug: log tool calls and results for diagnosis (off by default, opt-in via POLYWAVE_LOG_LEVEL=DEBUG)
			truncResult := result
			if len(truncResult) > 200 {
				truncResult = truncResult[:200] + "...[truncated]"
			}
			c.log().Debug("bedrock tool", "turn", turn, "tool", toolBlk.name, "input", inputStr[:min(len(inputStr), 100)], "error", isError, "result", truncResult)

			// Emit tool result event
			if onToolCall != nil {
				onToolCall(backend.ToolCallEvent{
					Name:     toolBlk.name,
					IsResult: true,
					IsError:  isError,
				})
			}

			toolResult := &types.ContentBlockMemberToolResult{
				Value: types.ToolResultBlock{
					ToolUseId: aws.String(toolBlk.id),
					Content: []types.ToolResultContentBlock{
						&types.ToolResultContentBlockMemberText{
							Value: result,
						},
					},
				},
			}
			if isError {
				toolResult.Value.Status = types.ToolResultStatusError
			}
			toolResults = append(toolResults, toolResult)
		}

		messages = append(messages, types.Message{
			Role:    types.ConversationRoleUser,
			Content: toolResults,
		})
	}

	return "", fmt.Errorf("bedrock: tool use loop exceeded maxTurns (%d)", maxT)
}

func (c *Client) maxTurns() int {
	if c.cfg.MaxTurns > 0 {
		return c.cfg.MaxTurns
	}
	// Default matches API and OpenAI backends.
	return 50
}

func (c *Client) maxTokens() int {
	if c.cfg.MaxTokens > 0 {
		return c.cfg.MaxTokens
	}
	return 16384
}

// Verify Client implements backend.Backend at compile time.
var _ backend.Backend = (*Client)(nil)
