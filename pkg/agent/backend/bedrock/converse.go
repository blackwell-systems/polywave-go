package bedrock

import (
	"encoding/json"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime/document"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime/types"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/tools"
)

// buildConverseTools converts Workshop tools to Bedrock Converse API ToolConfiguration format.
// Replaces buildToolsJSON for the Converse path.
func buildConverseTools(workshop tools.Workshop) *types.ToolConfiguration {
	allTools := workshop.All()
	if len(allTools) == 0 {
		return nil
	}

	toolSpecs := make([]types.Tool, 0, len(allTools))
	for _, t := range allTools {
		// Convert InputSchema map to document.Interface using LazyDocument
		doc := document.NewLazyDocument(t.InputSchema)

		toolSpec := &types.ToolMemberToolSpec{
			Value: types.ToolSpecification{
				Name:        aws.String(t.Name),
				Description: aws.String(t.Description),
				InputSchema: &types.ToolInputSchemaMemberJson{
					Value: doc,
				},
			},
		}
		toolSpecs = append(toolSpecs, toolSpec)
	}

	return &types.ToolConfiguration{
		Tools: toolSpecs,
	}
}

// buildConverseMessages converts the existing []interface{} message format
// (used in RunStreamingWithTools) to []types.Message with typed content blocks.
func buildConverseMessages(messages []interface{}) []types.Message {
	result := make([]types.Message, 0, len(messages))

	for _, msg := range messages {
		msgMap, ok := msg.(map[string]interface{})
		if !ok {
			continue
		}

		role, ok := msgMap["role"].(string)
		if !ok {
			continue
		}

		// Build content blocks
		var contentBlocks []types.ContentBlock

		// Handle content field - can be string or array
		content := msgMap["content"]
		switch v := content.(type) {
		case string:
			// Simple text content
			contentBlocks = append(contentBlocks, &types.ContentBlockMemberText{
				Value: v,
			})

		case []interface{}:
			// Array of content blocks (text, tool_use, tool_result)
			for _, item := range v {
				itemMap, ok := item.(map[string]interface{})
				if !ok {
					continue
				}

				blockType, ok := itemMap["type"].(string)
				if !ok {
					continue
				}

				switch blockType {
				case "text":
					if text, ok := itemMap["text"].(string); ok {
						contentBlocks = append(contentBlocks, &types.ContentBlockMemberText{
							Value: text,
						})
					}

				case "tool_use":
					id, _ := itemMap["id"].(string)
					name, _ := itemMap["name"].(string)
					input := itemMap["input"]

					// Convert input to document.Interface
					var inputDoc document.Interface
					if input != nil {
						inputDoc = document.NewLazyDocument(input)
					} else {
						inputDoc = document.NewLazyDocument(map[string]interface{}{})
					}

					contentBlocks = append(contentBlocks, &types.ContentBlockMemberToolUse{
						Value: types.ToolUseBlock{
							ToolUseId: aws.String(id),
							Name:      aws.String(name),
							Input:     inputDoc,
						},
					})

				case "tool_result":
					toolUseID, _ := itemMap["tool_use_id"].(string)
					resultContent, _ := itemMap["content"].(string)
					isError, _ := itemMap["is_error"].(bool)

					contentBlocks = append(contentBlocks, &types.ContentBlockMemberToolResult{
						Value: types.ToolResultBlock{
							ToolUseId: aws.String(toolUseID),
							Content: []types.ToolResultContentBlock{
								&types.ToolResultContentBlockMemberText{
									Value: resultContent,
								},
							},
							Status: types.ToolResultStatusSuccess,
						},
					})

					// Set error status if is_error is true
					if isError {
						lastIdx := len(contentBlocks) - 1
						if block, ok := contentBlocks[lastIdx].(*types.ContentBlockMemberToolResult); ok {
							block.Value.Status = types.ToolResultStatusError
						}
					}
				}
			}
		}

		// Map role to ConversationRole
		var conversationRole types.ConversationRole
		switch role {
		case "user":
			conversationRole = types.ConversationRoleUser
		case "assistant":
			conversationRole = types.ConversationRoleAssistant
		default:
			continue
		}

		result = append(result, types.Message{
			Role:    conversationRole,
			Content: contentBlocks,
		})
	}

	return result
}

// buildSystemBlocks converts system prompt string to []types.SystemContentBlock.
func buildSystemBlocks(systemPrompt string) []types.SystemContentBlock {
	if systemPrompt == "" {
		return nil
	}

	return []types.SystemContentBlock{
		&types.SystemContentBlockMemberText{
			Value: systemPrompt,
		},
	}
}

// buildOutputConfig creates OutputConfig for structured output when schema is provided.
// When schema is nil, returns nil.
func buildOutputConfig(schema map[string]any) (*types.OutputConfig, error) {
	if schema == nil {
		return nil, nil
	}

	// Marshal schema to JSON string
	jsonBytes, err := json.Marshal(schema)
	if err != nil {
		return nil, fmt.Errorf("marshal output schema: %w", err)
	}

	return &types.OutputConfig{
		TextFormat: &types.OutputFormat{
			Type: types.OutputFormatTypeJsonSchema,
			Structure: &types.OutputFormatStructureMemberJsonSchema{
				Value: types.JsonSchemaDefinition{
					Schema: aws.String(string(jsonBytes)),
					Name:   aws.String("structured_output"),
				},
			},
		},
	}, nil
}

// extractTextFromOutput extracts text content from ConverseOutput.
// Handles ConverseOutputMemberMessage and iterates content blocks to extract text.
func extractTextFromOutput(output types.ConverseOutput) string {
	// ConverseOutput is a union type - check for the Message member
	if msgOutput, ok := output.(*types.ConverseOutputMemberMessage); ok {
		var result string
		for _, block := range msgOutput.Value.Content {
			if textBlock, ok := block.(*types.ContentBlockMemberText); ok {
				result += textBlock.Value
			}
		}
		return result
	}
	return ""
}

// toolUseResult represents a parsed tool_use block from the model's output.
type toolUseResult struct {
	id    string
	name  string
	input json.RawMessage
}

// extractToolUseFromOutput extracts tool_use blocks from ConverseOutput for the tool loop.
func extractToolUseFromOutput(output types.ConverseOutput) []toolUseResult {
	var results []toolUseResult

	// ConverseOutput is a union type - check for the Message member
	if msgOutput, ok := output.(*types.ConverseOutputMemberMessage); ok {
		for _, block := range msgOutput.Value.Content {
			if toolUseBlock, ok := block.(*types.ContentBlockMemberToolUse); ok {
				// Extract the input document using UnmarshalSmithyDocument
				var inputJSON json.RawMessage
				if toolUseBlock.Value.Input != nil {
					// Use UnmarshalSmithyDocument to extract the data
					var inputData interface{}
					if err := toolUseBlock.Value.Input.UnmarshalSmithyDocument(&inputData); err == nil {
						// Marshal the extracted data to JSON
						if jsonBytes, err := json.Marshal(inputData); err == nil {
							inputJSON = jsonBytes
						} else {
							inputJSON = json.RawMessage("{}")
						}
					} else {
						inputJSON = json.RawMessage("{}")
					}
				} else {
					inputJSON = json.RawMessage("{}")
				}

				results = append(results, toolUseResult{
					id:    aws.ToString(toolUseBlock.Value.ToolUseId),
					name:  aws.ToString(toolUseBlock.Value.Name),
					input: inputJSON,
				})
			}
		}
	}

	return results
}
