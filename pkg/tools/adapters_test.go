package tools

import (
	"reflect"
	"testing"
)

// TestAnthropicAdapterSerialize tests Anthropic Messages API format
func TestAnthropicAdapterSerialize(t *testing.T) {
	adapter := NewAnthropicAdapter()

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
	adapter := NewOpenAIAdapter()

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
	bedrockAdapter := NewBedrockAdapter()
	anthropicAdapter := NewAnthropicAdapter()

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

	anthropicAdapter := NewAnthropicAdapter()
	openaiAdapter := NewOpenAIAdapter()
	bedrockAdapter := NewBedrockAdapter()

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
	adapters := []ToolAdapter{
		NewAnthropicAdapter(),
		NewOpenAIAdapter(),
		NewBedrockAdapter(),
	}
	for _, adapter := range adapters {
		result := adapter.Serialize([]Tool{})
		// Verify returns empty slice, not nil
		if result == nil {
			t.Errorf("%T.Serialize([]Tool{}) returned nil", adapter)
		}
		// Type-assert to slice and verify length
		slice, ok := result.([]map[string]interface{})
		if !ok {
			t.Errorf("%T.Serialize returned wrong type: %T", adapter, result)
		}
		if len(slice) != 0 {
			t.Errorf("%T.Serialize([]Tool{}) returned %d items, want 0", adapter, len(slice))
		}
	}
}

// TestAnthropicAdapterSerializeComplexSchema tests serialization with complex input schema
func TestAnthropicAdapterSerializeComplexSchema(t *testing.T) {
	adapter := NewAnthropicAdapter()
	tools := []Tool{
		{
			Name:        "complex_tool",
			Description: "A tool with complex schema",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"nested": map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"field": map[string]interface{}{
								"type": "string",
							},
						},
					},
					"array_field": map[string]interface{}{
						"type": "array",
						"items": map[string]interface{}{
							"type": "string",
						},
					},
				},
				"required": []string{"nested"},
			},
		},
	}
	result := adapter.Serialize(tools)
	serialized := result.([]map[string]interface{})
	if len(serialized) != 1 {
		t.Fatalf("Expected 1 tool, got %d", len(serialized))
	}
	// Verify nested schema preserved
	schema := serialized[0]["input_schema"].(map[string]interface{})
	props := schema["properties"].(map[string]interface{})
	if _, ok := props["nested"]; !ok {
		t.Error("Nested property missing from serialized schema")
	}
	if _, ok := props["array_field"]; !ok {
		t.Error("Array field missing from serialized schema")
	}
}

// TestOpenAIAdapterSerializeMultipleTools tests OpenAI format with multiple tools
func TestOpenAIAdapterSerializeMultipleTools(t *testing.T) {
	adapter := NewOpenAIAdapter()
	tools := []Tool{
		{Name: "tool1", Description: "First", InputSchema: map[string]interface{}{"type": "object"}},
		{Name: "tool2", Description: "Second", InputSchema: map[string]interface{}{"type": "object"}},
		{Name: "tool3", Description: "Third", InputSchema: map[string]interface{}{"type": "object"}},
	}
	result := adapter.Serialize(tools)
	serialized := result.([]map[string]interface{})
	if len(serialized) != 3 {
		t.Fatalf("Expected 3 tools, got %d", len(serialized))
	}
	// Verify each has type=function and function object
	for i, item := range serialized {
		if item["type"] != "function" {
			t.Errorf("Tool %d: type=%v, want 'function'", i, item["type"])
		}
		fn := item["function"].(map[string]interface{})
		if fn["name"] != tools[i].Name {
			t.Errorf("Tool %d: name=%v, want %q", i, fn["name"], tools[i].Name)
		}
	}
}
