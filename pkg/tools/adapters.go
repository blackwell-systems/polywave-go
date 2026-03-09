// pkg/tools/adapters.go
package tools

import "fmt"

// AnthropicAdapter serializes tools for the Anthropic Messages API.
// Format matches existing logic in pkg/agent/backend/api/client.go.
type AnthropicAdapter struct{}

// Serialize converts tools to Anthropic Messages API format.
// Returns []map[string]interface{} with name, description, and input_schema fields.
func (a *AnthropicAdapter) Serialize(tools []Tool) interface{} {
	result := make([]map[string]interface{}, 0, len(tools))
	for _, t := range tools {
		result = append(result, map[string]interface{}{
			"name":         t.Name,
			"description":  t.Description,
			"input_schema": t.InputSchema,
		})
	}
	return result
}

// Deserialize is not implemented for Anthropic adapter.
// Tool responses are handled inline in the tool-use loop.
func (a *AnthropicAdapter) Deserialize(response interface{}) (string, map[string]interface{}, error) {
	return "", nil, fmt.Errorf("AnthropicAdapter.Deserialize not implemented")
}

// NewAnthropicAdapter creates a new Anthropic tool adapter.
func NewAnthropicAdapter() ToolAdapter {
	return &AnthropicAdapter{}
}

// OpenAIAdapter serializes tools for OpenAI-compatible APIs.
// This includes OpenAI, Ollama, and LMStudio backends.
type OpenAIAdapter struct{}

// Serialize converts tools to OpenAI function calling format.
// Returns []map[string]interface{} with type=function and function object.
func (a *OpenAIAdapter) Serialize(tools []Tool) interface{} {
	result := make([]map[string]interface{}, 0, len(tools))
	for _, t := range tools {
		result = append(result, map[string]interface{}{
			"type": "function",
			"function": map[string]interface{}{
				"name":        t.Name,
				"description": t.Description,
				"parameters":  t.InputSchema,
			},
		})
	}
	return result
}

// Deserialize is not implemented for OpenAI adapter.
// Tool responses are handled inline in the tool-use loop.
func (a *OpenAIAdapter) Deserialize(response interface{}) (string, map[string]interface{}, error) {
	return "", nil, fmt.Errorf("OpenAIAdapter.Deserialize not implemented")
}

// NewOpenAIAdapter creates a new OpenAI tool adapter.
func NewOpenAIAdapter() ToolAdapter {
	return &OpenAIAdapter{}
}

// BedrockAdapter serializes tools for AWS Bedrock.
// Bedrock uses the Anthropic Messages API format when invoking Claude models.
type BedrockAdapter struct{}

// Serialize converts tools to Bedrock format (same as Anthropic).
// Delegates to AnthropicAdapter since Bedrock uses the same schema.
func (a *BedrockAdapter) Serialize(tools []Tool) interface{} {
	return (&AnthropicAdapter{}).Serialize(tools)
}

// Deserialize is not implemented for Bedrock adapter.
// Tool responses are handled inline in the tool-use loop.
func (a *BedrockAdapter) Deserialize(response interface{}) (string, map[string]interface{}, error) {
	return "", nil, fmt.Errorf("BedrockAdapter.Deserialize not implemented")
}

// NewBedrockAdapter creates a new Bedrock tool adapter.
func NewBedrockAdapter() ToolAdapter {
	return &BedrockAdapter{}
}
