package bedrock

import (
	"encoding/json"
	"testing"

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
