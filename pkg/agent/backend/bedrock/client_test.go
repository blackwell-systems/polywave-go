package bedrock

import (
	"context"
	"fmt"
	"testing"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/agent/backend"
	"github.com/blackwell-systems/scout-and-wave-go/pkg/tools"
)

// mockExecutor is a simple ToolExecutor for testing that returns a canned response.
type mockExecutor struct {
	result string
	err    error
}

func (m *mockExecutor) Execute(_ context.Context, _ tools.ExecutionContext, _ map[string]interface{}) (string, error) {
	return m.result, m.err
}

// newMockWorkshop creates a Workshop with the given tools for testing.
func newMockWorkshop(toolDefs ...tools.Tool) tools.Workshop {
	w := tools.NewWorkshop()
	for _, t := range toolDefs {
		w.Register(t)
	}
	return w
}

// TestBuildToolsJSON verifies that buildToolsJSON converts a Workshop with multiple
// tools into the correct Bedrock/Anthropic Messages API JSON structure.
func TestBuildToolsJSON(t *testing.T) {
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

	result := buildToolsJSON(w)

	if len(result) != 2 {
		t.Fatalf("expected 2 tools, got %d", len(result))
	}

	// Tools are sorted by name via Workshop.All(), so bash comes first.
	for _, item := range result {
		toolMap, ok := item.(map[string]interface{})
		if !ok {
			t.Fatalf("expected map[string]interface{}, got %T", item)
		}

		name, ok := toolMap["name"].(string)
		if !ok || name == "" {
			t.Error("tool missing 'name' field")
		}
		desc, ok := toolMap["description"].(string)
		if !ok || desc == "" {
			t.Error("tool missing 'description' field")
		}
		schema, ok := toolMap["input_schema"]
		if !ok || schema == nil {
			t.Error("tool missing 'input_schema' field")
		}
	}

	// Verify specific tool order (All() sorts by name).
	first := result[0].(map[string]interface{})
	if first["name"] != "bash" {
		t.Errorf("expected first tool to be 'bash', got %q", first["name"])
	}
	second := result[1].(map[string]interface{})
	if second["name"] != "read_file" {
		t.Errorf("expected second tool to be 'read_file', got %q", second["name"])
	}
}

// TestBuildToolsJSON_Empty verifies that buildToolsJSON returns an empty slice
// for an empty workshop.
func TestBuildToolsJSON_Empty(t *testing.T) {
	w := tools.NewWorkshop()
	result := buildToolsJSON(w)
	if len(result) != 0 {
		t.Errorf("expected 0 tools, got %d", len(result))
	}
}

// TestExecuteTool_KnownTool verifies that executeTool looks up and executes
// a known tool, returning its result with isError=false.
func TestExecuteTool_KnownTool(t *testing.T) {
	w := newMockWorkshop(tools.Tool{
		Name:     "bash",
		Executor: &mockExecutor{result: "hello world"},
	})

	result, isError := executeTool(context.Background(), w, "bash", map[string]interface{}{
		"command": "echo hello",
	}, t.TempDir())

	if isError {
		t.Error("expected isError=false for known tool")
	}
	if result != "hello world" {
		t.Errorf("expected %q, got %q", "hello world", result)
	}
}

// TestExecuteTool_UnknownTool verifies that executeTool returns an error message
// and isError=true for a tool not in the workshop.
func TestExecuteTool_UnknownTool(t *testing.T) {
	w := tools.NewWorkshop()

	result, isError := executeTool(context.Background(), w, "nonexistent", nil, t.TempDir())

	if !isError {
		t.Error("expected isError=true for unknown tool")
	}
	if result == "" {
		t.Error("expected non-empty error message")
	}
	// Should contain the tool name in the error.
	if !contains(result, "nonexistent") {
		t.Errorf("expected error to mention tool name, got %q", result)
	}
}

// TestExecuteTool_ExecutionError verifies that executeTool returns an error message
// and isError=true when the tool executor returns an error.
func TestExecuteTool_ExecutionError(t *testing.T) {
	w := newMockWorkshop(tools.Tool{
		Name:     "failing_tool",
		Executor: &mockExecutor{err: errMock},
	})

	result, isError := executeTool(context.Background(), w, "failing_tool", nil, t.TempDir())

	if !isError {
		t.Error("expected isError=true for execution error")
	}
	if result == "" {
		t.Error("expected non-empty error message")
	}
}

// errMock is a sentinel error for testing.
var errMock = fmt.Errorf("mock execution error")

// TestBuildWorkshop_StandardMode verifies that buildWorkshop with readOnly=false
// returns all 7 standard tools.
func TestBuildWorkshop_StandardMode(t *testing.T) {
	c := &Client{
		cfg:      backend.Config{},
		readOnly: false,
	}

	w := c.buildWorkshop(t.TempDir())
	allTools := w.All()

	if len(allTools) != 7 {
		t.Errorf("expected 7 standard tools, got %d", len(allTools))
		for _, tool := range allTools {
			t.Logf("  tool: %s", tool.Name)
		}
	}

	// Verify key tools are present.
	expectedTools := []string{"bash", "edit_file", "glob", "grep", "list_directory", "read_file", "write_file"}
	for _, name := range expectedTools {
		if _, found := w.Get(name); !found {
			t.Errorf("expected tool %q to be registered", name)
		}
	}
}

// TestBuildWorkshop_ReadOnlyMode verifies that buildWorkshop with readOnly=true
// applies permission middleware that blocks write_file and edit_file.
func TestBuildWorkshop_ReadOnlyMode(t *testing.T) {
	c := &Client{
		cfg:      backend.Config{},
		readOnly: true,
	}

	w := c.buildWorkshop(t.TempDir())
	allTools := w.All()

	// ReadOnly still registers all 7 tools (model sees them), but blocks execution.
	if len(allTools) != 7 {
		t.Errorf("expected 7 tools (read-only still registers all), got %d", len(allTools))
	}

	// write_file should be blocked at execution time.
	writeTool, found := w.Get("write_file")
	if !found {
		t.Fatal("expected write_file to be registered")
	}
	result, err := writeTool.Executor.Execute(context.Background(), tools.ExecutionContext{
		WorkDir: t.TempDir(),
	}, map[string]interface{}{
		"file_path": "/tmp/test.txt",
		"content":   "test",
	})
	if err != nil {
		t.Fatalf("expected permission denial message, not Go error: %v", err)
	}
	if !contains(result, "not permitted") {
		t.Errorf("expected permission denial message, got %q", result)
	}

	// read_file should work fine (not blocked).
	readTool, found := w.Get("read_file")
	if !found {
		t.Fatal("expected read_file to be registered")
	}
	// Just verify we can get the tool; actual execution needs a real file.
	_ = readTool
}

// TestBuildWorkshop_WithTimingCallback verifies that buildWorkshop wraps tools
// with TimingMiddleware when onToolCall is configured.
func TestBuildWorkshop_WithTimingCallback(t *testing.T) {
	var called bool
	c := &Client{
		cfg:      backend.Config{},
		readOnly: false,
		onToolCall: func(ev backend.ToolCallEvent) {
			called = true
		},
	}

	w := c.buildWorkshop(t.TempDir())

	// Execute a tool to trigger the timing callback.
	bashTool, found := w.Get("bash")
	if !found {
		t.Fatal("expected bash tool to be registered")
	}
	_, _ = bashTool.Executor.Execute(context.Background(), tools.ExecutionContext{
		WorkDir: t.TempDir(),
	}, map[string]interface{}{
		"command": "echo test",
	})

	if !called {
		t.Error("expected onToolCall callback to be invoked via timing middleware")
	}
}

// TestMaxTurns_Default verifies that maxTurns returns 200 when cfg.MaxTurns is 0.
func TestMaxTurns_Default(t *testing.T) {
	c := &Client{cfg: backend.Config{MaxTurns: 0}}
	if got := c.maxTurns(); got != 200 {
		t.Errorf("expected default maxTurns=200, got %d", got)
	}
}

// TestMaxTurns_Custom verifies that maxTurns returns the configured value.
func TestMaxTurns_Custom(t *testing.T) {
	c := &Client{cfg: backend.Config{MaxTurns: 25}}
	if got := c.maxTurns(); got != 25 {
		t.Errorf("expected maxTurns=25, got %d", got)
	}
}

// TestRunStreamingWithTools_NilClient verifies that RunStreamingWithTools returns
// an error when the AWS client is nil (failed to load AWS config).
func TestRunStreamingWithTools_NilClient(t *testing.T) {
	c := &Client{
		// client is nil — simulates failed AWS config load.
		cfg: backend.Config{Model: "anthropic.claude-3-sonnet-20240229-v1:0"},
	}

	_, err := c.RunStreamingWithTools(
		context.Background(),
		"system", "user message", t.TempDir(),
		nil, nil,
	)
	if err == nil {
		t.Fatal("expected error for nil AWS client")
	}
	if !contains(err.Error(), "AWS") && !contains(err.Error(), "bedrock") {
		t.Errorf("expected error to mention AWS/bedrock, got: %v", err)
	}
}

// TestWithOutputConfig verifies that WithOutputConfig stores the schema
// and returns the client for method chaining.
func TestWithOutputConfig(t *testing.T) {
	c := &Client{
		cfg: backend.Config{Model: "anthropic.claude-3-sonnet-20240229-v1:0"},
	}

	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"answer": map[string]any{"type": "string"},
		},
		"required": []string{"answer"},
	}

	result := c.WithOutputConfig(schema)

	// Verify method chaining
	if result != c {
		t.Error("expected WithOutputConfig to return the same client instance")
	}

	// Verify schema is stored
	if c.outputSchema == nil {
		t.Fatal("expected outputSchema to be set")
	}
	if c.outputSchema["type"] != "object" {
		t.Errorf("expected schema type 'object', got %v", c.outputSchema["type"])
	}
}

// TestRun_NilClient verifies that Run returns an error when the AWS client is nil.
func TestRun_NilClient(t *testing.T) {
	c := &Client{
		cfg: backend.Config{Model: "anthropic.claude-3-sonnet-20240229-v1:0"},
		// client is nil - simulates failed AWS config load
	}

	_, err := c.Run(context.Background(), "system", "user message", "")
	if err == nil {
		t.Fatal("expected error for nil AWS client")
	}
	if !contains(err.Error(), "AWS") && !contains(err.Error(), "bedrock") {
		t.Errorf("expected error to mention AWS/bedrock, got: %v", err)
	}
}

// TestRunStreaming_NilClient verifies that RunStreaming returns an error when the AWS client is nil.
func TestRunStreaming_NilClient(t *testing.T) {
	c := &Client{
		cfg: backend.Config{Model: "anthropic.claude-3-sonnet-20240229-v1:0"},
		// client is nil - simulates failed AWS config load
	}

	_, err := c.RunStreaming(context.Background(), "system", "user message", "", nil)
	if err == nil {
		t.Fatal("expected error for nil AWS client")
	}
	if !contains(err.Error(), "AWS") && !contains(err.Error(), "bedrock") {
		t.Errorf("expected error to mention AWS/bedrock, got: %v", err)
	}
}

// TestNew_WithStaticCredentials verifies that New() with populated credential
// fields creates a non-nil client (no panic).
func TestNew_WithStaticCredentials(t *testing.T) {
	cfg := backend.Config{
		Model:                 "anthropic.claude-3-sonnet-20240229-v1:0",
		BedrockRegion:         "us-west-2",
		BedrockAccessKeyID:    "AKIAIOSFODNN7EXAMPLE",
		BedrockSecretAccessKey: "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
		BedrockSessionToken:   "FwoGZXIvYXdzEBY",
	}

	c := New(cfg)
	if c == nil {
		t.Fatal("expected non-nil client")
	}
	// The internal AWS client should be non-nil when credentials are provided
	if c.client == nil {
		t.Error("expected internal bedrockruntime client to be non-nil with static credentials")
	}
}

// TestNew_EmptyCredentialsFallsBackToDefaultChain verifies that New() with empty
// credential fields falls back to the default AWS credential chain (existing behavior).
func TestNew_EmptyCredentialsFallsBackToDefaultChain(t *testing.T) {
	cfg := backend.Config{
		Model: "anthropic.claude-3-sonnet-20240229-v1:0",
		// All Bedrock* fields left empty — should use default chain
	}

	c := New(cfg)
	if c == nil {
		t.Fatal("expected non-nil client")
	}
	// Client should still be constructed (may or may not have internal client
	// depending on whether AWS default credentials are available in the test env)
}

// TestNew_WithRegionOnly verifies that New() with only BedrockRegion set
// uses the specified region but falls back to default credential chain.
func TestNew_WithRegionOnly(t *testing.T) {
	cfg := backend.Config{
		Model:         "anthropic.claude-3-sonnet-20240229-v1:0",
		BedrockRegion: "eu-west-1",
	}

	c := New(cfg)
	if c == nil {
		t.Fatal("expected non-nil client")
	}
}

// contains is a helper that checks if s contains substr.
func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
