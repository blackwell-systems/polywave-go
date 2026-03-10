// Package api provides an Anthropic API backend that implements the backend.Backend
// interface. It runs a full tool-use loop against the Anthropic Messages API,
// using the standard SAW tools from pkg/tools.
package api

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	anthropic "github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/anthropics/anthropic-sdk-go/packages/param"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/agent/backend"
	"github.com/blackwell-systems/scout-and-wave-go/pkg/tools"
)

const (
	defaultModel     = "claude-sonnet-4-5"
	defaultMaxTokens = 8096
	defaultMaxTurns  = 50
)

// Client is an Anthropic API backend. It implements backend.Backend.
type Client struct {
	apiKey       string
	model        string
	maxTokens    int
	maxTurns     int
	baseURL      string         // optional override for testing
	outputSchema map[string]any // optional: structured output schema
	onToolCall   backend.ToolCallCallback // optional: timing events for Observatory
	readOnly     bool                     // if true, block write_file/edit_file
}

// New creates a new Client configured from cfg.
// If apiKey is empty, the ANTHROPIC_API_KEY environment variable is used.
// cfg.Model defaults to "claude-sonnet-4-5" if empty.
// cfg.MaxTokens defaults to 8096 if zero.
// cfg.MaxTurns defaults to 50 if zero.
func New(apiKey string, cfg backend.Config) *Client {
	if apiKey == "" {
		apiKey = os.Getenv("ANTHROPIC_API_KEY")
	}
	model := cfg.Model
	if model == "" {
		model = defaultModel
	}
	maxTokens := cfg.MaxTokens
	if maxTokens <= 0 {
		maxTokens = defaultMaxTokens
	}
	maxTurns := cfg.MaxTurns
	if maxTurns <= 0 {
		maxTurns = defaultMaxTurns
	}
	return &Client{
		apiKey:     apiKey,
		model:      model,
		maxTokens:  maxTokens,
		maxTurns:   maxTurns,
		onToolCall: cfg.OnToolCall,
		readOnly:   cfg.ReadOnly,
	}
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

// WithBaseURL overrides the Anthropic API endpoint. Used in tests to point at
// a mock HTTP server. Returns c for chaining.
func (c *Client) WithBaseURL(url string) *Client {
	c.baseURL = url
	return c
}

// WithOutputConfig sets a JSON schema for structured output. When non-nil,
// the schema is passed as OutputConfig.Format to the Anthropic API, enabling
// constrained JSON generation. Returns c for chaining.
func (c *Client) WithOutputConfig(schema map[string]any) *Client {
	c.outputSchema = schema
	return c
}

func (c *Client) sendOpts() []option.RequestOption {
	opts := []option.RequestOption{option.WithAPIKey(c.apiKey)}
	if c.baseURL != "" {
		opts = append(opts, option.WithBaseURL(c.baseURL))
	}
	return opts
}

// buildToolParams converts Workshop tools to Anthropic SDK tool params.
func buildToolParams(workshop tools.Workshop) []anthropic.ToolUnionParam {
	allTools := workshop.All()
	toolParams := make([]anthropic.ToolUnionParam, 0, len(allTools))
	for _, t := range allTools {
		schema := anthropic.ToolInputSchemaParam{
			Properties: t.InputSchema["properties"],
		}
		if req, ok := t.InputSchema["required"]; ok {
			if reqSlice, ok := req.([]string); ok {
				schema.Required = reqSlice
			}
		}
		tp := anthropic.ToolUnionParamOfTool(schema, t.Name)
		if tp.OfTool != nil && t.Description != "" {
			tp.OfTool.Description = param.NewOpt(t.Description)
		}
		toolParams = append(toolParams, tp)
	}
	return toolParams
}

// executeTool looks up and executes a tool from the Workshop.
func executeTool(ctx context.Context, workshop tools.Workshop, name string, input map[string]interface{}, workDir string) (string, bool) {
	tool, found := workshop.Get(name)
	if !found {
		return fmt.Sprintf("error: unknown tool %q", name), true
	}
	execCtx := tools.ExecutionContext{WorkDir: workDir}
	result, err := tool.Executor.Execute(ctx, execCtx, input)
	if err != nil {
		return fmt.Sprintf("error: %v", err), true
	}
	return result, false
}

// Run executes the agent described by systemPrompt and userMessage.
// It runs a tool-use loop using StandardTools scoped to workDir until the model
// signals end_turn or maxTurns is exceeded.
// Run implements backend.Backend.
func (c *Client) Run(ctx context.Context, systemPrompt, userMessage, workDir string) (string, error) {
	workshop := c.buildWorkshop(workDir)
	toolParams := buildToolParams(workshop)
	sdkClient := anthropic.NewClient(c.sendOpts()...)

	messages := []anthropic.MessageParam{
		anthropic.NewUserMessage(anthropic.NewTextBlock(userMessage)),
	}

	params := anthropic.MessageNewParams{
		Model:     anthropic.Model(c.model),
		MaxTokens: int64(c.maxTokens),
		Tools:     toolParams,
	}
	if systemPrompt != "" {
		params.System = []anthropic.TextBlockParam{
			{Text: systemPrompt},
		}
	}
	if c.outputSchema != nil {
		params.OutputConfig = anthropic.OutputConfigParam{
			Format: anthropic.JSONOutputFormatParam{
				Schema: c.outputSchema,
			},
		}
	}

	for turn := 0; turn < c.maxTurns; turn++ {
		params.Messages = messages

		resp, err := sdkClient.Messages.New(ctx, params)
		if err != nil {
			return "", fmt.Errorf("anthropic API error (turn %d): %w", turn, err)
		}

		if resp.StopReason == anthropic.StopReasonEndTurn {
			var sb strings.Builder
			for _, block := range resp.Content {
				if block.Type == "text" {
					sb.WriteString(block.AsText().Text)
				}
			}
			return sb.String(), nil
		}

		if resp.StopReason != anthropic.StopReasonToolUse {
			return "", fmt.Errorf("unexpected stop reason: %s", resp.StopReason)
		}

		// Append the assistant message with full content.
		assistantBlocks := make([]anthropic.ContentBlockParamUnion, 0, len(resp.Content))
		for _, block := range resp.Content {
			switch block.Type {
			case "text":
				tb := block.AsText()
				assistantBlocks = append(assistantBlocks, anthropic.NewTextBlock(tb.Text))
			case "tool_use":
				tu := block.AsToolUse()
				assistantBlocks = append(assistantBlocks, anthropic.NewToolUseBlock(tu.ID, tu.Input, tu.Name))
			}
		}
		messages = append(messages, anthropic.NewAssistantMessage(assistantBlocks...))

		// Execute each tool_use block and collect results.
		toolResultBlocks := make([]anthropic.ContentBlockParamUnion, 0)
		for _, block := range resp.Content {
			if block.Type != "tool_use" {
				continue
			}
			tu := block.AsToolUse()

			var inputMap map[string]interface{}
			if err := json.Unmarshal(tu.Input, &inputMap); err != nil {
				inputMap = map[string]interface{}{}
			}

			result, isError := executeTool(ctx, workshop, tu.Name, inputMap, workDir)
			toolResultBlocks = append(toolResultBlocks, anthropic.NewToolResultBlock(tu.ID, result, isError))
		}
		messages = append(messages, anthropic.NewUserMessage(toolResultBlocks...))
	}

	return "", fmt.Errorf("api: tool use loop exceeded maxTurns (%d)", c.maxTurns)
}

// RunStreaming implements backend.Backend.
// It behaves identically to Run for all tool-use turns (non-streaming).
// For the final end_turn response, it uses the streaming API and calls onChunk
// for each text_delta chunk as it arrives.
// If onChunk is nil, RunStreaming behaves identically to Run.
func (c *Client) RunStreaming(ctx context.Context, systemPrompt, userMessage, workDir string, onChunk backend.ChunkCallback) (string, error) {
	// If no callback provided, delegate to the non-streaming path.
	if onChunk == nil {
		return c.Run(ctx, systemPrompt, userMessage, workDir)
	}

	workshop := c.buildWorkshop(workDir)
	toolParams := buildToolParams(workshop)
	sdkClient := anthropic.NewClient(c.sendOpts()...)

	messages := []anthropic.MessageParam{
		anthropic.NewUserMessage(anthropic.NewTextBlock(userMessage)),
	}

	params := anthropic.MessageNewParams{
		Model:     anthropic.Model(c.model),
		MaxTokens: int64(c.maxTokens),
		Tools:     toolParams,
	}
	if systemPrompt != "" {
		params.System = []anthropic.TextBlockParam{
			{Text: systemPrompt},
		}
	}
	if c.outputSchema != nil {
		params.OutputConfig = anthropic.OutputConfigParam{
			Format: anthropic.JSONOutputFormatParam{
				Schema: c.outputSchema,
			},
		}
	}

	for turn := 0; turn < c.maxTurns; turn++ {
		params.Messages = messages

		// Peek ahead: use non-streaming for tool-use turns, streaming for end_turn.
		// We don't know the stop reason until we get the response, so we always
		// attempt streaming. If the turn ends in tool_use, we collect the full
		// streamed content and continue the loop. If it ends in end_turn, we
		// stream with onChunk and return.
		stream := sdkClient.Messages.NewStreaming(ctx, params)

		var sb strings.Builder
		var stopReason string
		var contentBlocks []anthropic.ContentBlockParamUnion

		// Collect streamed events, calling onChunk only for text_delta on the
		// final end_turn turn. We don't know if it's end_turn until we see the
		// message_delta event, so we buffer text and emit it after.
		type textChunk struct{ text string }
		var bufferedChunks []textChunk

		// Track content blocks being built from the stream.
		type streamBlock struct {
			blockType string
			text      string
			toolID    string
			toolName  string
			toolInput strings.Builder
		}
		blockMap := make(map[int]*streamBlock)

		for stream.Next() {
			event := stream.Current()
			switch event.Type {
			case "content_block_start":
				cbs := event.AsContentBlockStart()
				idx := int(cbs.Index)
				blk := &streamBlock{}
				switch cbs.ContentBlock.Type {
				case "text":
					blk.blockType = "text"
				case "tool_use":
					tu := cbs.ContentBlock.AsToolUse()
					blk.blockType = "tool_use"
					blk.toolID = tu.ID
					blk.toolName = tu.Name
				}
				blockMap[idx] = blk
			case "content_block_delta":
				cbd := event.AsContentBlockDelta()
				idx := int(cbd.Index)
				blk := blockMap[idx]
				if blk == nil {
					break
				}
				switch cbd.Delta.Type {
				case "text_delta":
					chunk := cbd.Delta.AsTextDelta().Text
					blk.text += chunk
					bufferedChunks = append(bufferedChunks, textChunk{chunk})
				case "input_json_delta":
					blk.toolInput.WriteString(cbd.Delta.AsInputJSONDelta().PartialJSON)
				}
			case "message_delta":
				md := event.AsMessageDelta()
				stopReason = string(md.Delta.StopReason)
			}
		}
		if err := stream.Err(); err != nil {
			return "", fmt.Errorf("anthropic streaming API error (turn %d): %w", turn, err)
		}

		// Reconstruct content blocks from stream.
		for i := 0; ; i++ {
			blk, ok := blockMap[i]
			if !ok {
				break
			}
			switch blk.blockType {
			case "text":
				sb.WriteString(blk.text)
				contentBlocks = append(contentBlocks, anthropic.NewTextBlock(blk.text))
			case "tool_use":
				inputJSON := []byte(blk.toolInput.String())
				if len(inputJSON) == 0 {
					inputJSON = []byte("{}")
				}
				contentBlocks = append(contentBlocks, anthropic.NewToolUseBlock(blk.toolID, inputJSON, blk.toolName))
			}
		}

		if stopReason == string(anthropic.StopReasonEndTurn) {
			// Emit buffered text chunks to onChunk.
			for _, tc := range bufferedChunks {
				onChunk(tc.text)
			}
			return sb.String(), nil
		}

		if stopReason != string(anthropic.StopReasonToolUse) {
			return "", fmt.Errorf("unexpected stop reason: %s", stopReason)
		}

		// Append assistant message and execute tool calls.
		messages = append(messages, anthropic.NewAssistantMessage(contentBlocks...))

		toolResultBlocks := make([]anthropic.ContentBlockParamUnion, 0)
		for i := 0; ; i++ {
			blk, ok := blockMap[i]
			if !ok {
				break
			}
			if blk.blockType != "tool_use" {
				continue
			}

			inputJSON := []byte(blk.toolInput.String())
			if len(inputJSON) == 0 {
				inputJSON = []byte("{}")
			}

			var inputMap map[string]interface{}
			if err := json.Unmarshal(inputJSON, &inputMap); err != nil {
				inputMap = map[string]interface{}{}
			}

			result, isError := executeTool(ctx, workshop, blk.toolName, inputMap, workDir)
			toolResultBlocks = append(toolResultBlocks, anthropic.NewToolResultBlock(blk.toolID, result, isError))
		}
		messages = append(messages, anthropic.NewUserMessage(toolResultBlocks...))

		// Reset for next turn.
		contentBlocks = contentBlocks[:0]
		bufferedChunks = bufferedChunks[:0]
		for k := range blockMap {
			delete(blockMap, k)
		}
	}

	return "", fmt.Errorf("api: tool use loop exceeded maxTurns (%d)", c.maxTurns)
}

// RunStreamingWithTools implements backend.Backend.
// API backend does not yet support tool call event streaming, so this
// delegates to RunStreaming (no-op for onToolCall).
func (c *Client) RunStreamingWithTools(ctx context.Context, systemPrompt, userMessage, workDir string, onChunk backend.ChunkCallback, onToolCall backend.ToolCallCallback) (string, error) {
	return c.RunStreaming(ctx, systemPrompt, userMessage, workDir, onChunk)
}
