// Package bedrock provides a backend.Backend implementation that uses AWS Bedrock
// via the AWS SDK v2. It supports streaming responses and uses AWS credentials
// from the default credential chain (~/.aws/credentials, env vars, IAM roles).
package bedrock

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime/types"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/agent/backend"
	"github.com/blackwell-systems/scout-and-wave-go/pkg/tools"
)

// Client implements backend.Backend using AWS Bedrock.
type Client struct {
	client     *bedrockruntime.Client
	cfg        backend.Config
	onToolCall backend.ToolCallCallback
	readOnly   bool
}

// New creates a Bedrock backend client using AWS credentials from the default chain.
func New(cfg backend.Config) *Client {
	// Load AWS config from environment/credentials
	awsCfg, err := config.LoadDefaultConfig(context.Background())
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

// Run sends a non-streaming request to Bedrock.
func (c *Client) Run(ctx context.Context, systemPrompt, userPrompt, _ string) (string, error) {
	if c.client == nil {
		return "", fmt.Errorf("bedrock: AWS config failed to load")
	}

	// Build Anthropic Messages API request body
	reqBody := map[string]interface{}{
		"anthropic_version": "bedrock-2023-05-31",
		"max_tokens":        c.maxTokens(),
		"messages": []map[string]string{
			{"role": "user", "content": userPrompt},
		},
	}
	if systemPrompt != "" {
		reqBody["system"] = systemPrompt
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("bedrock: marshal request: %w", err)
	}

	resp, err := c.client.InvokeModel(ctx, &bedrockruntime.InvokeModelInput{
		ModelId:     aws.String(c.cfg.Model),
		Body:        bodyBytes,
		ContentType: aws.String("application/json"),
		Accept:      aws.String("application/json"),
	})
	if err != nil {
		return "", fmt.Errorf("bedrock: InvokeModel: %w", err)
	}

	var result struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.Unmarshal(resp.Body, &result); err != nil {
		return "", fmt.Errorf("bedrock: unmarshal response: %w", err)
	}

	if len(result.Content) == 0 {
		return "", fmt.Errorf("bedrock: empty response content")
	}

	return result.Content[0].Text, nil
}

// RunStreaming sends a streaming request to Bedrock and calls onChunk for each text delta.
func (c *Client) RunStreaming(ctx context.Context, systemPrompt, userPrompt, _ string, onChunk backend.ChunkCallback) (string, error) {
	if c.client == nil {
		return "", fmt.Errorf("bedrock: AWS config failed to load")
	}

	reqBody := map[string]interface{}{
		"anthropic_version": "bedrock-2023-05-31",
		"max_tokens":        c.maxTokens(),
		"messages": []map[string]string{
			{"role": "user", "content": userPrompt},
		},
	}
	if systemPrompt != "" {
		reqBody["system"] = systemPrompt
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("bedrock: marshal request: %w", err)
	}

	resp, err := c.client.InvokeModelWithResponseStream(ctx, &bedrockruntime.InvokeModelWithResponseStreamInput{
		ModelId:     aws.String(c.cfg.Model),
		Body:        bodyBytes,
		ContentType: aws.String("application/json"),
		Accept:      aws.String("application/json"),
	})
	if err != nil {
		return "", fmt.Errorf("bedrock: InvokeModelWithResponseStream: %w", err)
	}

	stream := resp.GetStream()
	defer stream.Close()

	var fullText strings.Builder

	for event := range stream.Events() {
		switch v := event.(type) {
		case *types.ResponseStreamMemberChunk:
			var chunk struct {
				Type  string `json:"type"`
				Delta struct {
					Text string `json:"text"`
				} `json:"delta"`
			}
			if err := json.Unmarshal(v.Value.Bytes, &chunk); err != nil {
				continue
			}
			if chunk.Type == "content_block_delta" && chunk.Delta.Text != "" {
				fullText.WriteString(chunk.Delta.Text)
				onChunk(chunk.Delta.Text)
			}

		case *types.UnknownUnionMember:
			// Ignore unknown event types

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
func (c *Client) buildWorkshop(workDir string) tools.Workshop {
	var w tools.Workshop
	if c.readOnly {
		w = tools.ReadOnlyTools(workDir)
	} else {
		w = tools.StandardTools(workDir)
	}
	if c.onToolCall != nil {
		w = tools.WithTiming(w, func(ev tools.ToolCallEvent) {
			c.onToolCall(backend.ToolCallEvent{
				Name:       ev.ToolName,
				DurationMs: ev.DurationMs,
				IsError:    ev.IsError,
				IsResult:   true,
			})
		})
	}
	return w
}

// streamEvent represents a parsed chunk from Bedrock's streaming response.
type streamEvent struct {
	Type         string        `json:"type"`
	Index        int           `json:"index"`
	ContentBlock *contentBlock `json:"content_block,omitempty"`
	Delta        *deltaBlock   `json:"delta,omitempty"`
}

type contentBlock struct {
	Type string `json:"type"`
	ID   string `json:"id"`
	Name string `json:"name"`
	Text string `json:"text"`
}

type deltaBlock struct {
	Type        string `json:"type"`
	Text        string `json:"text"`
	PartialJSON string `json:"partial_json"`
	StopReason  string `json:"stop_reason"`
}

// toolBlock tracks state for a tool_use content block being streamed.
type toolBlock struct {
	id        string
	name      string
	inputJSON strings.Builder
}

// RunStreamingWithTools implements backend.Backend with full multi-turn tool-use loop.
// Streams text deltas via onChunk, emits tool call events via onToolCall.
// Loops until stop_reason is "end_turn" or maxTurns exceeded.
func (c *Client) RunStreamingWithTools(ctx context.Context, systemPrompt, userPrompt, workDir string, onChunk backend.ChunkCallback, onToolCall backend.ToolCallCallback) (string, error) {
	if c.client == nil {
		return "", fmt.Errorf("bedrock: AWS config failed to load")
	}

	workshop := c.buildWorkshop(workDir)
	toolsJSON := buildToolsJSON(workshop)

	// messages is []interface{} where each element is map[string]interface{}
	messages := []interface{}{
		map[string]interface{}{
			"role":    "user",
			"content": userPrompt,
		},
	}

	maxT := c.maxTurns()

	for turn := 0; turn < maxT; turn++ {
		// Build request body
		reqBody := map[string]interface{}{
			"anthropic_version": "bedrock-2023-05-31",
			"max_tokens":        c.maxTokens(),
			"tools":             toolsJSON,
			"messages":          messages,
		}
		if systemPrompt != "" {
			reqBody["system"] = systemPrompt
		}

		bodyBytes, err := json.Marshal(reqBody)
		if err != nil {
			return "", fmt.Errorf("bedrock: marshal request (turn %d): %w", turn, err)
		}

		resp, err := c.client.InvokeModelWithResponseStream(ctx, &bedrockruntime.InvokeModelWithResponseStreamInput{
			ModelId:     aws.String(c.cfg.Model),
			Body:        bodyBytes,
			ContentType: aws.String("application/json"),
			Accept:      aws.String("application/json"),
		})
		if err != nil {
			return "", fmt.Errorf("bedrock: InvokeModelWithResponseStream (turn %d): %w", turn, err)
		}

		stream := resp.GetStream()

		var fullText strings.Builder
		var stopReason string
		blockMap := make(map[int]*toolBlock)   // index -> tool block
		textBlocks := make(map[int]*strings.Builder) // index -> text accumulator

		for event := range stream.Events() {
			switch v := event.(type) {
			case *types.ResponseStreamMemberChunk:
				var ev streamEvent
				if err := json.Unmarshal(v.Value.Bytes, &ev); err != nil {
					continue
				}

				switch ev.Type {
				case "content_block_start":
					if ev.ContentBlock != nil && ev.ContentBlock.Type == "tool_use" {
						blockMap[ev.Index] = &toolBlock{
							id:   ev.ContentBlock.ID,
							name: ev.ContentBlock.Name,
						}
					} else if ev.ContentBlock != nil && ev.ContentBlock.Type == "text" {
						textBlocks[ev.Index] = &strings.Builder{}
					}

				case "content_block_delta":
					if ev.Delta == nil {
						break
					}
					switch ev.Delta.Type {
					case "text_delta":
						text := ev.Delta.Text
						fullText.WriteString(text)
						if tb, ok := textBlocks[ev.Index]; ok {
							tb.WriteString(text)
						}
						if onChunk != nil {
							onChunk(text)
						}
					case "input_json_delta":
						if tb, ok := blockMap[ev.Index]; ok {
							tb.inputJSON.WriteString(ev.Delta.PartialJSON)
						}
					}

				case "message_delta":
					if ev.Delta != nil {
						stopReason = ev.Delta.StopReason
					}
				}

			case *types.UnknownUnionMember:
				// Ignore unknown event types
			}
		}

		if err := stream.Err(); err != nil {
			stream.Close()
			return fullText.String(), fmt.Errorf("bedrock: stream error (turn %d): %w", turn, err)
		}
		stream.Close()

		if stopReason == "end_turn" {
			return fullText.String(), nil
		}

		if stopReason == "max_tokens" {
			// Treat max_tokens like end_turn for tool-use turns: the model ran out of
			// output space. Continue the loop by sending what we have as an assistant
			// message so the model can resume. If there are pending tool_use blocks,
			// execute them; otherwise just continue the conversation.
			if len(blockMap) == 0 {
				// No tool calls in this turn — send accumulated text back and let
				// the model continue from where it left off.
				if fullText.Len() > 0 {
					messages = append(messages, map[string]interface{}{
						"role": "assistant",
						"content": []interface{}{
							map[string]interface{}{"type": "text", "text": fullText.String()},
						},
					})
					messages = append(messages, map[string]interface{}{
						"role":    "user",
						"content": "Continue from where you left off.",
					})
				}
				continue
			}
			// Fall through to tool execution — treat the pending tool_use blocks normally
		} else if stopReason != "tool_use" {
			return fullText.String(), fmt.Errorf("bedrock: unexpected stop reason (turn %d): %s", turn, stopReason)
		}

		// Build assistant content blocks for the message history
		assistantContent := make([]interface{}, 0)

		// Add text blocks
		for i := 0; ; i++ {
			if tb, ok := textBlocks[i]; ok {
				text := tb.String()
				if text != "" {
					assistantContent = append(assistantContent, map[string]interface{}{
						"type": "text",
						"text": text,
					})
				}
				continue
			}
			if toolBlk, ok := blockMap[i]; ok {
				inputStr := toolBlk.inputJSON.String()
				if inputStr == "" {
					inputStr = "{}"
				}
				var inputParsed interface{}
				if err := json.Unmarshal([]byte(inputStr), &inputParsed); err != nil {
					inputParsed = map[string]interface{}{}
				}
				assistantContent = append(assistantContent, map[string]interface{}{
					"type":  "tool_use",
					"id":    toolBlk.id,
					"name":  toolBlk.name,
					"input": inputParsed,
				})
				continue
			}
			break
		}

		messages = append(messages, map[string]interface{}{
			"role":    "assistant",
			"content": assistantContent,
		})

		// Execute tools and build tool_result blocks
		toolResults := make([]interface{}, 0)
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
					Name:  toolBlk.name,
					Input: inputStr,
				})
			}

			result, isError := executeTool(ctx, workshop, toolBlk.name, inputMap, workDir)

			// Emit tool result event
			if onToolCall != nil {
				onToolCall(backend.ToolCallEvent{
					Name:     toolBlk.name,
					IsResult: true,
					IsError:  isError,
				})
			}

			toolResult := map[string]interface{}{
				"type":        "tool_result",
				"tool_use_id": toolBlk.id,
				"content":     result,
			}
			if isError {
				toolResult["is_error"] = true
			}
			toolResults = append(toolResults, toolResult)
		}

		messages = append(messages, map[string]interface{}{
			"role":    "user",
			"content": toolResults,
		})
	}

	return "", fmt.Errorf("bedrock: tool use loop exceeded maxTurns (%d)", maxT)
}

func (c *Client) maxTurns() int {
	if c.cfg.MaxTurns > 0 {
		return c.cfg.MaxTurns
	}
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
