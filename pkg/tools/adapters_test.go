package tools

import (
	"reflect"
	"testing"
)

// Mock adapter implementations for testing
type mockAnthropicAdapter struct{}

func (a *mockAnthropicAdapter) Serialize(tools []Tool) interface{} {
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

func (a *mockAnthropicAdapter) Deserialize(response interface{}) (string, map[string]interface{}, error) {
	return "", nil, nil // Not implemented for testing
}

type mockOpenAIAdapter struct{}

func (a *mockOpenAIAdapter) Serialize(tools []Tool) interface{} {
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

func (a *mockOpenAIAdapter) Deserialize(response interface{}) (string, map[string]interface{}, error) {
	return "", nil, nil // Not implemented for testing
}

type mockBedrockAdapter struct{}

func (a *mockBedrockAdapter) Serialize(tools []Tool) interface{} {
	// Bedrock uses the same format as Anthropic
	return (&mockAnthropicAdapter{}).Serialize(tools)
}

func (a *mockBedrockAdapter) Deserialize(response interface{}) (string, map[string]interface{}, error) {
	return "", nil, nil // Not implemented for testing
}

// TestAnthropicAdapterSerialize tests Anthropic Messages API format
func TestAnthropicAdapterSerialize(t *testing.T) {
	adapter := &mockAnthropicAdapter{}

	tools := []Tool{
		{
			Name:        "file:read",
			Description: "Read a file",
			Namespace:   "file",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"path": map[string]interface{}{
						"type":        "string",
						"description": "File path",
					},
				},
				"required": []interface{}{"path"},
			},
		},
		{
			Name:        "bash",
			Description: "Execute bash command",
			Namespace:   "bash",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"command": map[string]interface{}{
						"type":        "string",
						"description": "Command to execute",
					},
				},
				"required": []interface{}{"command"},
			},
		},
	}

	result := adapter.Serialize(tools)

	// Type assertion
	serialized, ok := result.([]map[string]interface{})
	if !ok {
		t.Fatalf("Expected []map[string]interface{}, got %T", result)
	}

	if len(serialized) != 2 {
		t.Fatalf("Expected 2 tools, got %d", len(serialized))
	}

	// Verify first tool
	tool1 := serialized[0]
	if tool1["name"] != "file:read" {
		t.Errorf("Expected name 'file:read', got %v", tool1["name"])
	}
	if tool1["description"] != "Read a file" {
		t.Errorf("Expected description 'Read a file', got %v", tool1["description"])
	}
	if tool1["input_schema"] == nil {
		t.Error("Expected input_schema to be present")
	}

	// Verify input_schema structure
	schema, ok := tool1["input_schema"].(map[string]interface{})
	if !ok {
		t.Fatalf("Expected input_schema to be map[string]interface{}, got %T", tool1["input_schema"])
	}
	if schema["type"] != "object" {
		t.Errorf("Expected schema type 'object', got %v", schema["type"])
	}

	// Verify second tool
	tool2 := serialized[1]
	if tool2["name"] != "bash" {
		t.Errorf("Expected name 'bash', got %v", tool2["name"])
	}
}

// TestOpenAIAdapterSerialize tests OpenAI function calling format
func TestOpenAIAdapterSerialize(t *testing.T) {
	adapter := &mockOpenAIAdapter{}

	tools := []Tool{
		{
			Name:        "file:write",
			Description: "Write to a file",
			Namespace:   "file",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"path": map[string]interface{}{
						"type": "string",
					},
					"content": map[string]interface{}{
						"type": "string",
					},
				},
				"required": []interface{}{"path", "content"},
			},
		},
	}

	result := adapter.Serialize(tools)

	// Type assertion
	serialized, ok := result.([]map[string]interface{})
	if !ok {
		t.Fatalf("Expected []map[string]interface{}, got %T", result)
	}

	if len(serialized) != 1 {
		t.Fatalf("Expected 1 tool, got %d", len(serialized))
	}

	// Verify OpenAI format structure
	tool := serialized[0]
	if tool["type"] != "function" {
		t.Errorf("Expected type 'function', got %v", tool["type"])
	}

	function, ok := tool["function"].(map[string]interface{})
	if !ok {
		t.Fatalf("Expected function to be map[string]interface{}, got %T", tool["function"])
	}

	if function["name"] != "file:write" {
		t.Errorf("Expected name 'file:write', got %v", function["name"])
	}
	if function["description"] != "Write to a file" {
		t.Errorf("Expected description 'Write to a file', got %v", function["description"])
	}

	// Verify parameters (should be the InputSchema)
	params, ok := function["parameters"].(map[string]interface{})
	if !ok {
		t.Fatalf("Expected parameters to be map[string]interface{}, got %T", function["parameters"])
	}
	if params["type"] != "object" {
		t.Errorf("Expected parameters type 'object', got %v", params["type"])
	}
}

// TestBedrockAdapterSerialize tests Bedrock format (should match Anthropic)
func TestBedrockAdapterSerialize(t *testing.T) {
	bedrockAdapter := &mockBedrockAdapter{}
	anthropicAdapter := &mockAnthropicAdapter{}

	tools := []Tool{
		{
			Name:        "test:tool",
			Description: "Test tool",
			Namespace:   "test",
			InputSchema: map[string]interface{}{
				"type": "object",
			},
		},
	}

	bedrockResult := bedrockAdapter.Serialize(tools)
	anthropicResult := anthropicAdapter.Serialize(tools)

	// Bedrock should produce identical output to Anthropic
	if !reflect.DeepEqual(bedrockResult, anthropicResult) {
		t.Errorf("Bedrock serialization differs from Anthropic.\nBedrock: %+v\nAnthropic: %+v",
			bedrockResult, anthropicResult)
	}
}

// TestAdapterSerializationConsistency tests that all adapters serialize the same tools consistently
func TestAdapterSerializationConsistency(t *testing.T) {
	tools := []Tool{
		{
			Name:        "file:read",
			Description: "Read file",
			Namespace:   "file",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"path": map[string]interface{}{
						"type": "string",
					},
				},
			},
		},
		{
			Name:        "file:write",
			Description: "Write file",
			Namespace:   "file",
			InputSchema: map[string]interface{}{
				"type": "object",
			},
		},
		{
			Name:        "bash",
			Description: "Bash command",
			Namespace:   "bash",
			InputSchema: map[string]interface{}{
				"type": "object",
			},
		},
	}

	anthropicAdapter := &mockAnthropicAdapter{}
	openaiAdapter := &mockOpenAIAdapter{}
	bedrockAdapter := &mockBedrockAdapter{}

	anthropicResult := anthropicAdapter.Serialize(tools)
	openaiResult := openaiAdapter.Serialize(tools)
	bedrockResult := bedrockAdapter.Serialize(tools)

	// All should produce slices of the same length
	anthropicSlice := anthropicResult.([]map[string]interface{})
	openaiSlice := openaiResult.([]map[string]interface{})
	bedrockSlice := bedrockResult.([]map[string]interface{})

	if len(anthropicSlice) != 3 {
		t.Errorf("Anthropic: expected 3 tools, got %d", len(anthropicSlice))
	}
	if len(openaiSlice) != 3 {
		t.Errorf("OpenAI: expected 3 tools, got %d", len(openaiSlice))
	}
	if len(bedrockSlice) != 3 {
		t.Errorf("Bedrock: expected 3 tools, got %d", len(bedrockSlice))
	}

	// Verify Anthropic tool names
	for i, tool := range anthropicSlice {
		if tool["name"] != tools[i].Name {
			t.Errorf("Anthropic tool %d: expected name %s, got %v", i, tools[i].Name, tool["name"])
		}
	}

	// Verify OpenAI tool names (nested in function field)
	for i, tool := range openaiSlice {
		function := tool["function"].(map[string]interface{})
		if function["name"] != tools[i].Name {
			t.Errorf("OpenAI tool %d: expected name %s, got %v", i, tools[i].Name, function["name"])
		}
	}

	// Verify Bedrock matches Anthropic
	if !reflect.DeepEqual(bedrockSlice, anthropicSlice) {
		t.Error("Bedrock serialization differs from Anthropic")
	}
}

// TestAdapterSerializeEmpty tests that adapters handle empty tool list
func TestAdapterSerializeEmpty(t *testing.T) {
	anthropicAdapter := &mockAnthropicAdapter{}
	openaiAdapter := &mockOpenAIAdapter{}
	bedrockAdapter := &mockBedrockAdapter{}

	emptyTools := []Tool{}

	anthropicResult := anthropicAdapter.Serialize(emptyTools)
	openaiResult := openaiAdapter.Serialize(emptyTools)
	bedrockResult := bedrockAdapter.Serialize(emptyTools)

	anthropicSlice := anthropicResult.([]map[string]interface{})
	openaiSlice := openaiResult.([]map[string]interface{})
	bedrockSlice := bedrockResult.([]map[string]interface{})

	if len(anthropicSlice) != 0 {
		t.Errorf("Anthropic: expected 0 tools, got %d", len(anthropicSlice))
	}
	if len(openaiSlice) != 0 {
		t.Errorf("OpenAI: expected 0 tools, got %d", len(openaiSlice))
	}
	if len(bedrockSlice) != 0 {
		t.Errorf("Bedrock: expected 0 tools, got %d", len(bedrockSlice))
	}
}

// TestAnthropicAdapterSerializeComplexSchema tests serialization with complex input schema
func TestAnthropicAdapterSerializeComplexSchema(t *testing.T) {
	adapter := &mockAnthropicAdapter{}

	tools := []Tool{
		{
			Name:        "complex:tool",
			Description: "Tool with complex schema",
			Namespace:   "complex",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"string_field": map[string]interface{}{
						"type":        "string",
						"description": "A string field",
					},
					"number_field": map[string]interface{}{
						"type":        "number",
						"description": "A number field",
					},
					"array_field": map[string]interface{}{
						"type": "array",
						"items": map[string]interface{}{
							"type": "string",
						},
					},
					"nested_object": map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"nested_field": map[string]interface{}{
								"type": "boolean",
							},
						},
					},
				},
				"required": []interface{}{"string_field", "number_field"},
			},
		},
	}

	result := adapter.Serialize(tools)
	serialized := result.([]map[string]interface{})

	if len(serialized) != 1 {
		t.Fatalf("Expected 1 tool, got %d", len(serialized))
	}

	tool := serialized[0]
	schema := tool["input_schema"].(map[string]interface{})
	properties := schema["properties"].(map[string]interface{})

	// Verify all fields are present
	expectedFields := []string{"string_field", "number_field", "array_field", "nested_object"}
	for _, field := range expectedFields {
		if properties[field] == nil {
			t.Errorf("Expected property %s to be present", field)
		}
	}

	// Verify required array
	required := schema["required"].([]interface{})
	if len(required) != 2 {
		t.Errorf("Expected 2 required fields, got %d", len(required))
	}
}

// TestOpenAIAdapterSerializeMultipleTools tests OpenAI format with multiple tools
func TestOpenAIAdapterSerializeMultipleTools(t *testing.T) {
	adapter := &mockOpenAIAdapter{}

	tools := []Tool{
		{
			Name:        "tool1",
			Description: "First tool",
			Namespace:   "ns1",
			InputSchema: map[string]interface{}{"type": "object"},
		},
		{
			Name:        "tool2",
			Description: "Second tool",
			Namespace:   "ns2",
			InputSchema: map[string]interface{}{"type": "object"},
		},
		{
			Name:        "tool3",
			Description: "Third tool",
			Namespace:   "ns3",
			InputSchema: map[string]interface{}{"type": "object"},
		},
	}

	result := adapter.Serialize(tools)
	serialized := result.([]map[string]interface{})

	if len(serialized) != 3 {
		t.Fatalf("Expected 3 tools, got %d", len(serialized))
	}

	// Verify each tool has correct structure
	for i, toolData := range serialized {
		if toolData["type"] != "function" {
			t.Errorf("Tool %d: expected type 'function', got %v", i, toolData["type"])
		}

		function, ok := toolData["function"].(map[string]interface{})
		if !ok {
			t.Errorf("Tool %d: function field is not a map", i)
			continue
		}

		expectedName := tools[i].Name
		if function["name"] != expectedName {
			t.Errorf("Tool %d: expected name %s, got %v", i, expectedName, function["name"])
		}

		expectedDesc := tools[i].Description
		if function["description"] != expectedDesc {
			t.Errorf("Tool %d: expected description %s, got %v", i, expectedDesc, function["description"])
		}

		if function["parameters"] == nil {
			t.Errorf("Tool %d: parameters field is missing", i)
		}
	}
}
