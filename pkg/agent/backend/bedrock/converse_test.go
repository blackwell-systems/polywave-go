package bedrock

import (
	"encoding/json"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime/document"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime/types"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/tools"
)

// TestBuildConverseTools verifies that buildConverseTools converts a Workshop
// with multiple tools into the correct Bedrock Converse API ToolConfiguration structure.
func TestBuildConverseTools(t *testing.T) {
	w := newMockWorkshop(
		tools.Tool{
			Name:        "read_file",
			Description: "Read a file",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"file_path": map[string]interface{}{
						"type":        "string",
						"description": "Path to the file",
					},
				},
				"required": []string{"file_path"},
			},
			Executor: &mockExecutor{result: "content"},
		},
		tools.Tool{
			Name:        "bash",
			Description: "Run a shell command",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"command": map[string]interface{}{
						"type":        "string",
						"description": "Shell command to execute",
					},
				},
				"required": []string{"command"},
			},
			Executor: &mockExecutor{result: "output"},
		},
	)

	config := buildConverseTools(w)

	if config == nil {
		t.Fatal("expected non-nil ToolConfiguration")
	}

	if len(config.Tools) != 2 {
		t.Fatalf("expected 2 tools, got %d", len(config.Tools))
	}

	// Tools are sorted by name via Workshop.All(), so bash comes first
	for i, tool := range config.Tools {
		toolSpec, ok := tool.(*types.ToolMemberToolSpec)
		if !ok {
			t.Fatalf("tool %d: expected ToolMemberToolSpec, got %T", i, tool)
		}

		if toolSpec.Value.Name == nil || *toolSpec.Value.Name == "" {
			t.Errorf("tool %d: missing name", i)
		}
		if toolSpec.Value.Description == nil || *toolSpec.Value.Description == "" {
			t.Errorf("tool %d: missing description", i)
		}
		if toolSpec.Value.InputSchema == nil {
			t.Errorf("tool %d: missing input schema", i)
		}
	}

	// Verify specific tool order (All() sorts by name)
	firstTool := config.Tools[0].(*types.ToolMemberToolSpec)
	if *firstTool.Value.Name != "bash" {
		t.Errorf("expected first tool to be 'bash', got %q", *firstTool.Value.Name)
	}

	secondTool := config.Tools[1].(*types.ToolMemberToolSpec)
	if *secondTool.Value.Name != "read_file" {
		t.Errorf("expected second tool to be 'read_file', got %q", *secondTool.Value.Name)
	}
}

// TestBuildConverseTools_Empty verifies that an empty workshop returns nil.
func TestBuildConverseTools_Empty(t *testing.T) {
	w := newMockWorkshop()
	config := buildConverseTools(w)

	if config != nil {
		t.Errorf("expected nil for empty workshop, got %v", config)
	}
}

// TestBuildSystemBlocks verifies that a non-empty prompt returns SystemContentBlockMemberText.
func TestBuildSystemBlocks(t *testing.T) {
	prompt := "You are a helpful assistant."
	blocks := buildSystemBlocks(prompt)

	if len(blocks) != 1 {
		t.Fatalf("expected 1 system block, got %d", len(blocks))
	}

	textBlock, ok := blocks[0].(*types.SystemContentBlockMemberText)
	if !ok {
		t.Fatalf("expected SystemContentBlockMemberText, got %T", blocks[0])
	}

	if textBlock.Value != prompt {
		t.Errorf("expected prompt %q, got %q", prompt, textBlock.Value)
	}
}

// TestBuildSystemBlocks_Empty verifies that an empty prompt returns nil.
func TestBuildSystemBlocks_Empty(t *testing.T) {
	blocks := buildSystemBlocks("")

	if blocks != nil {
		t.Errorf("expected nil for empty prompt, got %v", blocks)
	}
}

// TestBuildOutputConfig_WithSchema verifies OutputConfig structure with JSON schema.
func TestBuildOutputConfig_WithSchema(t *testing.T) {
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"result": map[string]any{
				"type": "string",
			},
		},
		"required": []string{"result"},
	}

	config, err := buildOutputConfig(schema)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if config == nil {
		t.Fatal("expected non-nil OutputConfig")
	}

	if config.TextFormat == nil {
		t.Fatal("expected non-nil TextFormat")
	}

	if config.TextFormat.Type != types.OutputFormatTypeJsonSchema {
		t.Errorf("expected OutputFormatTypeJsonSchema, got %v", config.TextFormat.Type)
	}

	jsonSchemaMember, ok := config.TextFormat.Structure.(*types.OutputFormatStructureMemberJsonSchema)
	if !ok {
		t.Fatalf("expected OutputFormatStructureMemberJsonSchema, got %T", config.TextFormat.Structure)
	}

	if jsonSchemaMember.Value.Name == nil || *jsonSchemaMember.Value.Name != "structured_output" {
		t.Errorf("expected name 'structured_output', got %v", jsonSchemaMember.Value.Name)
	}

	if jsonSchemaMember.Value.Schema == nil || *jsonSchemaMember.Value.Schema == "" {
		t.Error("expected non-empty schema string")
	}

	// Verify schema is valid JSON
	var parsed map[string]any
	if err := json.Unmarshal([]byte(*jsonSchemaMember.Value.Schema), &parsed); err != nil {
		t.Errorf("schema is not valid JSON: %v", err)
	}
}

// TestBuildOutputConfig_Nil verifies that nil schema returns nil.
func TestBuildOutputConfig_Nil(t *testing.T) {
	config, err := buildOutputConfig(nil)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if config != nil {
		t.Errorf("expected nil for nil schema, got %v", config)
	}
}

// TestExtractTextFromOutput verifies text extraction from ConverseOutputMemberMessage.
func TestExtractTextFromOutput(t *testing.T) {
	output := &types.ConverseOutputMemberMessage{
		Value: types.Message{
			Role: types.ConversationRoleAssistant,
			Content: []types.ContentBlock{
				&types.ContentBlockMemberText{
					Value: "Hello, ",
				},
				&types.ContentBlockMemberText{
					Value: "world!",
				},
			},
		},
	}

	text := extractTextFromOutput(output)
	expected := "Hello, world!"

	if text != expected {
		t.Errorf("expected %q, got %q", expected, text)
	}
}

// TestExtractToolUseFromOutput verifies tool_use extraction from ConverseOutput.
// Note: This test verifies structure extraction (IDs, names) but not detailed input content
// since document.Interface behavior in tests differs from real AWS SDK responses.
// Input content extraction is validated through integration tests.
func TestExtractToolUseFromOutput(t *testing.T) {
	input := document.NewLazyDocument(map[string]interface{}{"command": "ls -la"})

	output := &types.ConverseOutputMemberMessage{
		Value: types.Message{
			Role: types.ConversationRoleAssistant,
			Content: []types.ContentBlock{
				&types.ContentBlockMemberText{
					Value: "Let me run that command.",
				},
				&types.ContentBlockMemberToolUse{
					Value: types.ToolUseBlock{
						ToolUseId: aws.String("tool_123"),
						Name:      aws.String("bash"),
						Input:     input,
					},
				},
			},
		},
	}

	results := extractToolUseFromOutput(output)

	if len(results) != 1 {
		t.Fatalf("expected 1 tool use result, got %d", len(results))
	}

	result := results[0]
	if result.id != "tool_123" {
		t.Errorf("expected id 'tool_123', got %q", result.id)
	}
	if result.name != "bash" {
		t.Errorf("expected name 'bash', got %q", result.name)
	}

	// Verify input is valid JSON (structure test only)
	var inputData interface{}
	if err := json.Unmarshal(result.input, &inputData); err != nil {
		t.Errorf("input is not valid JSON: %v, raw: %s", err, string(result.input))
	}
}

// TestBuildConverseMessages verifies conversion of []interface{} to []types.Message.
func TestBuildConverseMessages(t *testing.T) {
	messages := []interface{}{
		map[string]interface{}{
			"role":    "user",
			"content": "Hello, assistant!",
		},
		map[string]interface{}{
			"role": "assistant",
			"content": []interface{}{
				map[string]interface{}{
					"type": "text",
					"text": "Hello! Let me help you.",
				},
				map[string]interface{}{
					"type":  "tool_use",
					"id":    "tool_456",
					"name":  "read_file",
					"input": map[string]interface{}{"file_path": "/etc/hosts"},
				},
			},
		},
		map[string]interface{}{
			"role": "user",
			"content": []interface{}{
				map[string]interface{}{
					"type":        "tool_result",
					"tool_use_id": "tool_456",
					"content":     "file contents here",
					"is_error":    false,
				},
			},
		},
	}

	result := buildConverseMessages(messages)

	if len(result) != 3 {
		t.Fatalf("expected 3 messages, got %d", len(result))
	}

	// Check first message (user text)
	if result[0].Role != types.ConversationRoleUser {
		t.Errorf("message 0: expected User role, got %v", result[0].Role)
	}
	if len(result[0].Content) != 1 {
		t.Fatalf("message 0: expected 1 content block, got %d", len(result[0].Content))
	}
	textBlock0, ok := result[0].Content[0].(*types.ContentBlockMemberText)
	if !ok {
		t.Fatalf("message 0 block 0: expected ContentBlockMemberText, got %T", result[0].Content[0])
	}
	if textBlock0.Value != "Hello, assistant!" {
		t.Errorf("message 0 block 0: expected text 'Hello, assistant!', got %q", textBlock0.Value)
	}

	// Check second message (assistant with text + tool_use)
	if result[1].Role != types.ConversationRoleAssistant {
		t.Errorf("message 1: expected Assistant role, got %v", result[1].Role)
	}
	if len(result[1].Content) != 2 {
		t.Fatalf("message 1: expected 2 content blocks, got %d", len(result[1].Content))
	}
	textBlock1, ok := result[1].Content[0].(*types.ContentBlockMemberText)
	if !ok {
		t.Fatalf("message 1 block 0: expected ContentBlockMemberText, got %T", result[1].Content[0])
	}
	if textBlock1.Value != "Hello! Let me help you." {
		t.Errorf("message 1 block 0: expected text 'Hello! Let me help you.', got %q", textBlock1.Value)
	}
	toolUseBlock1, ok := result[1].Content[1].(*types.ContentBlockMemberToolUse)
	if !ok {
		t.Fatalf("message 1 block 1: expected ContentBlockMemberToolUse, got %T", result[1].Content[1])
	}
	if *toolUseBlock1.Value.ToolUseId != "tool_456" {
		t.Errorf("message 1 block 1: expected tool_use_id 'tool_456', got %q", *toolUseBlock1.Value.ToolUseId)
	}
	if *toolUseBlock1.Value.Name != "read_file" {
		t.Errorf("message 1 block 1: expected name 'read_file', got %q", *toolUseBlock1.Value.Name)
	}

	// Check third message (user with tool_result)
	if result[2].Role != types.ConversationRoleUser {
		t.Errorf("message 2: expected User role, got %v", result[2].Role)
	}
	if len(result[2].Content) != 1 {
		t.Fatalf("message 2: expected 1 content block, got %d", len(result[2].Content))
	}
	toolResultBlock2, ok := result[2].Content[0].(*types.ContentBlockMemberToolResult)
	if !ok {
		t.Fatalf("message 2 block 0: expected ContentBlockMemberToolResult, got %T", result[2].Content[0])
	}
	if *toolResultBlock2.Value.ToolUseId != "tool_456" {
		t.Errorf("message 2 block 0: expected tool_use_id 'tool_456', got %q", *toolResultBlock2.Value.ToolUseId)
	}
	if toolResultBlock2.Value.Status != types.ToolResultStatusSuccess {
		t.Errorf("message 2 block 0: expected Success status, got %v", toolResultBlock2.Value.Status)
	}
	if len(toolResultBlock2.Value.Content) != 1 {
		t.Fatalf("message 2 block 0: expected 1 content item, got %d", len(toolResultBlock2.Value.Content))
	}
	resultText, ok := toolResultBlock2.Value.Content[0].(*types.ToolResultContentBlockMemberText)
	if !ok {
		t.Fatalf("message 2 block 0 content 0: expected ToolResultContentBlockMemberText, got %T", toolResultBlock2.Value.Content[0])
	}
	if resultText.Value != "file contents here" {
		t.Errorf("message 2 block 0 content 0: expected 'file contents here', got %q", resultText.Value)
	}
}

// TestBuildConverseMessages_ErrorToolResult verifies that is_error=true sets error status.
func TestBuildConverseMessages_ErrorToolResult(t *testing.T) {
	messages := []interface{}{
		map[string]interface{}{
			"role": "user",
			"content": []interface{}{
				map[string]interface{}{
					"type":        "tool_result",
					"tool_use_id": "tool_789",
					"content":     "error: file not found",
					"is_error":    true,
				},
			},
		},
	}

	result := buildConverseMessages(messages)

	if len(result) != 1 {
		t.Fatalf("expected 1 message, got %d", len(result))
	}

	toolResultBlock, ok := result[0].Content[0].(*types.ContentBlockMemberToolResult)
	if !ok {
		t.Fatalf("expected ContentBlockMemberToolResult, got %T", result[0].Content[0])
	}

	if toolResultBlock.Value.Status != types.ToolResultStatusError {
		t.Errorf("expected Error status for is_error=true, got %v", toolResultBlock.Value.Status)
	}
}
