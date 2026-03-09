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
)

// Client implements backend.Backend using AWS Bedrock.
type Client struct {
	client *bedrockruntime.Client
	cfg    backend.Config
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
		client: bedrockruntime.NewFromConfig(awsCfg),
		cfg:    cfg,
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

// RunStreamingWithTools is not yet implemented for Bedrock.
// Falls back to non-tool streaming for now.
func (c *Client) RunStreamingWithTools(ctx context.Context, systemPrompt, userPrompt, _ string, onChunk backend.ChunkCallback, onToolCall backend.ToolCallCallback) (string, error) {
	// TODO: Implement tool use for Bedrock
	return c.RunStreaming(ctx, systemPrompt, userPrompt, "", onChunk)
}

func (c *Client) maxTokens() int {
	if c.cfg.MaxTokens > 0 {
		return c.cfg.MaxTokens
	}
	return 4096
}

// Verify Client implements backend.Backend at compile time.
var _ backend.Backend = (*Client)(nil)
