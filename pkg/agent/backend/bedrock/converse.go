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

