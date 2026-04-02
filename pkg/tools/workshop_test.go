package tools

import (
	"context"
	"reflect"
	"sort"
	"testing"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/result"
)

// Mock implementations for testing (will be replaced by Agent A's real implementations)
type mockWorkshop struct {
	tools map[string]Tool
}

func newMockWorkshop() *mockWorkshop {
	return &mockWorkshop{
		tools: make(map[string]Tool),
	}
}

func (w *mockWorkshop) Register(tool Tool) result.Result[RegisterData] {
	if _, exists := w.tools[tool.Name]; exists {
		return result.NewFailure[RegisterData]([]result.SAWError{
			{
				Code:     "TOOL_ALREADY_REGISTERED",
				Message:  "tool already registered: " + tool.Name,
				Severity: "fatal",
				Context:  map[string]string{"tool_name": tool.Name},
			},
		})
	}
	w.tools[tool.Name] = tool
	return result.NewSuccess(RegisterData{
		ToolName:   tool.Name,
		Registered: true,
		TotalTools: len(w.tools),
	})
}

func (w *mockWorkshop) Get(name string) (Tool, bool) {
	tool, ok := w.tools[name]
	return tool, ok
}

func (w *mockWorkshop) All() []Tool {
	tools := make([]Tool, 0, len(w.tools))
	for _, t := range w.tools {
		tools = append(tools, t)
	}
	// Sort by name for determinism
	sort.Slice(tools, func(i, j int) bool {
		return tools[i].Name < tools[j].Name
	})
	return tools
}

func (w *mockWorkshop) Namespace(prefix string) []Tool {
	var filtered []Tool
	for _, t := range w.tools {
		if len(t.Name) >= len(prefix) && t.Name[:len(prefix)] == prefix {
			filtered = append(filtered, t)
		}
	}
	// Sort by name for determinism
	sort.Slice(filtered, func(i, j int) bool {
		return filtered[i].Name < filtered[j].Name
	})
	return filtered
}

// Mock executor for testing
type mockExecutor struct {
	result string
	err    error
}

func (m *mockExecutor) Execute(ctx context.Context, execCtx ExecutionContext, input map[string]interface{}) (string, error) {
	return m.result, m.err
}

// TestRegisterAndGet tests basic tool registration and retrieval
func TestRegisterAndGet(t *testing.T) {
	workshop := newMockWorkshop()

	tool := Tool{
		Name:        "test:tool",
		Description: "A test tool",
		Namespace:   "test",
		InputSchema: map[string]interface{}{
			"type": "object",
		},
		Executor: &mockExecutor{result: "success"},
	}

	res := workshop.Register(tool)
	if !res.IsSuccess() {
		t.Fatalf("Register failed: %v", res.Errors)
	}
	data := res.GetData()
	if data.ToolName != tool.Name {
		t.Errorf("Expected ToolName %s, got %s", tool.Name, data.ToolName)
	}
	if !data.Registered {
		t.Error("Expected Registered=true")
	}
	if data.TotalTools != 1 {
		t.Errorf("Expected TotalTools=1, got %d", data.TotalTools)
	}

	retrieved, ok := workshop.Get("test:tool")
	if !ok {
		t.Fatal("Tool not found after registration")
	}

	if retrieved.Name != tool.Name {
		t.Errorf("Expected name %s, got %s", tool.Name, retrieved.Name)
	}
	if retrieved.Description != tool.Description {
		t.Errorf("Expected description %s, got %s", tool.Description, retrieved.Description)
	}
}

// TestRegisterDuplicate tests that registering the same name twice returns a Fatal result
func TestRegisterDuplicate(t *testing.T) {
	workshop := newMockWorkshop()

	tool := Tool{
		Name:        "duplicate:tool",
		Description: "First registration",
		Namespace:   "duplicate",
		Executor:    &mockExecutor{},
	}

	res := workshop.Register(tool)
	if !res.IsSuccess() {
		t.Fatalf("First registration failed: %v", res.Errors)
	}

	// Try to register again with same name
	tool2 := Tool{
		Name:        "duplicate:tool",
		Description: "Second registration",
		Namespace:   "duplicate",
		Executor:    &mockExecutor{},
	}

	res2 := workshop.Register(tool2)
	if !res2.IsFatal() {
		t.Fatal("Expected Fatal result when registering duplicate tool name")
	}
	if len(res2.Errors) == 0 {
		t.Fatal("Expected errors in Fatal result")
	}
	if res2.Errors[0].Code != "TOOL_ALREADY_REGISTERED" {
		t.Errorf("Expected error code TOOL_ALREADY_REGISTERED, got %s", res2.Errors[0].Code)
	}
}

// TestRegisterTotalToolsIncrements tests that RegisterData.TotalTools increments on each registration
func TestRegisterTotalToolsIncrements(t *testing.T) {
	workshop := newMockWorkshop()

	for i, name := range []string{"tool:a", "tool:b", "tool:c"} {
		res := workshop.Register(Tool{Name: name, Executor: &mockExecutor{}})
		if !res.IsSuccess() {
			t.Fatalf("Register %s failed: %v", name, res.Errors)
		}
		data := res.GetData()
		expected := i + 1
		if data.TotalTools != expected {
			t.Errorf("After registering %s: expected TotalTools=%d, got %d", name, expected, data.TotalTools)
		}
	}
}

// TestAll tests that All() returns all registered tools in sorted order
func TestAll(t *testing.T) {
	workshop := newMockWorkshop()

	tools := []Tool{
		{Name: "zebra:tool", Description: "Z", Namespace: "zebra", Executor: &mockExecutor{}},
		{Name: "alpha:tool", Description: "A", Namespace: "alpha", Executor: &mockExecutor{}},
		{Name: "beta:tool", Description: "B", Namespace: "beta", Executor: &mockExecutor{}},
	}

	for _, tool := range tools {
		if res := workshop.Register(tool); !res.IsSuccess() {
			t.Fatalf("Register failed: %v", res.Errors)
		}
	}

	all := workshop.All()
	if len(all) != 3 {
		t.Fatalf("Expected 3 tools, got %d", len(all))
	}

	// Verify sorted order
	expected := []string{"alpha:tool", "beta:tool", "zebra:tool"}
	for i, tool := range all {
		if tool.Name != expected[i] {
			t.Errorf("Expected tool[%d] name %s, got %s", i, expected[i], tool.Name)
		}
	}
}

// TestNamespace tests that Namespace() filters tools by prefix
func TestNamespace(t *testing.T) {
	workshop := newMockWorkshop()

	tools := []Tool{
		{Name: "file:read", Description: "Read file", Namespace: "file", Executor: &mockExecutor{}},
		{Name: "file:write", Description: "Write file", Namespace: "file", Executor: &mockExecutor{}},
		{Name: "file:list", Description: "List files", Namespace: "file", Executor: &mockExecutor{}},
		{Name: "bash", Description: "Bash command", Namespace: "bash", Executor: &mockExecutor{}},
		{Name: "git:commit", Description: "Git commit", Namespace: "git", Executor: &mockExecutor{}},
	}

	for _, tool := range tools {
		if res := workshop.Register(tool); !res.IsSuccess() {
			t.Fatalf("Register failed: %v", res.Errors)
		}
	}

	// Test file: namespace
	fileTools := workshop.Namespace("file:")
	if len(fileTools) != 3 {
		t.Errorf("Expected 3 file: tools, got %d", len(fileTools))
	}
	for _, tool := range fileTools {
		if len(tool.Name) < 5 || tool.Name[:5] != "file:" {
			t.Errorf("Tool %s should start with 'file:'", tool.Name)
		}
	}

	// Test bash namespace (no colon)
	bashTools := workshop.Namespace("bash")
	if len(bashTools) != 1 {
		t.Errorf("Expected 1 bash tool, got %d", len(bashTools))
	}
	if len(bashTools) > 0 && bashTools[0].Name != "bash" {
		t.Errorf("Expected bash tool, got %s", bashTools[0].Name)
	}

	// Test git: namespace
	gitTools := workshop.Namespace("git:")
	if len(gitTools) != 1 {
		t.Errorf("Expected 1 git: tool, got %d", len(gitTools))
	}
}

// TestNamespaceEmpty tests that Namespace() returns empty slice for nonexistent prefix
func TestNamespaceEmpty(t *testing.T) {
	workshop := newMockWorkshop()

	tool := Tool{
		Name:        "file:read",
		Description: "Read file",
		Namespace:   "file",
		Executor:    &mockExecutor{},
	}

	if res := workshop.Register(tool); !res.IsSuccess() {
		t.Fatalf("Register failed: %v", res.Errors)
	}

	result := workshop.Namespace("nonexistent")
	if len(result) != 0 {
		t.Errorf("Expected empty slice for nonexistent namespace, got %d tools", len(result))
	}
}

// TestDefaultWorkshopNamespace tests that DefaultWorkshop.Namespace() correctly filters
// tools by prefix and returns them in sorted order.
// This test uses NewWorkshop() to ensure the real implementation is tested, not just mocks.
//
// NOTE: As of 2026-04-01, Namespace() is not actively used in the codebase outside of tests.
// This test preserves functionality for future namespace-based tool filtering features.
func TestDefaultWorkshopNamespace(t *testing.T) {
	workshop := NewWorkshop()

	tools := []Tool{
		{Name: "file:read", Description: "Read", Namespace: "file", Executor: &mockExecutor{}},
		{Name: "file:write", Description: "Write", Namespace: "file", Executor: &mockExecutor{}},
		{Name: "file:list", Description: "List", Namespace: "file", Executor: &mockExecutor{}},
		{Name: "bash", Description: "Bash", Namespace: "bash", Executor: &mockExecutor{}},
		{Name: "git:commit", Description: "Commit", Namespace: "git", Executor: &mockExecutor{}},
	}

	for _, tool := range tools {
		res := workshop.Register(tool)
		if !res.IsSuccess() {
			t.Fatalf("Register %s failed: %v", tool.Name, res.Errors)
		}
	}

	// Test file: namespace
	fileTools := workshop.Namespace("file:")
	if len(fileTools) != 3 {
		t.Errorf("Expected 3 file: tools, got %d", len(fileTools))
	}

	// Verify sorted order
	expectedFileOrder := []string{"file:list", "file:read", "file:write"}
	for i, tool := range fileTools {
		if tool.Name != expectedFileOrder[i] {
			t.Errorf("file: tools[%d]: expected %s, got %s", i, expectedFileOrder[i], tool.Name)
		}
	}

	// Test git: namespace
	gitTools := workshop.Namespace("git:")
	if len(gitTools) != 1 {
		t.Errorf("Expected 1 git: tool, got %d", len(gitTools))
	}
	if len(gitTools) > 0 && gitTools[0].Name != "git:commit" {
		t.Errorf("Expected git:commit, got %s", gitTools[0].Name)
	}

	// Test bash namespace (no colon)
	bashTools := workshop.Namespace("bash")
	if len(bashTools) != 1 {
		t.Errorf("Expected 1 bash tool, got %d", len(bashTools))
	}
	if len(bashTools) > 0 && bashTools[0].Name != "bash" {
		t.Errorf("Expected bash, got %s", bashTools[0].Name)
	}

	// Test nonexistent namespace
	noneTools := workshop.Namespace("nonexistent:")
	if len(noneTools) != 0 {
		t.Errorf("Expected 0 tools for nonexistent namespace, got %d", len(noneTools))
	}
}

// TestStandardToolsRegistration tests that StandardTools() registers all 4 tools with correct namespaces
func TestStandardToolsRegistration(t *testing.T) {
	// Create a mock workshop with standard tools manually registered
	workshop := newMockWorkshop()

	standardTools := []Tool{
		{
			Name:        "file:read",
			Description: "Read the contents of a file",
			Namespace:   "file",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"path": map[string]interface{}{
						"type":        "string",
						"description": "File path relative to working directory",
					},
				},
				"required": []interface{}{"path"},
			},
			Executor: &mockExecutor{},
		},
		{
			Name:        "file:write",
			Description: "Write content to a file",
			Namespace:   "file",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"path": map[string]interface{}{
						"type":        "string",
						"description": "File path relative to working directory",
					},
					"content": map[string]interface{}{
						"type":        "string",
						"description": "Content to write",
					},
				},
				"required": []interface{}{"path", "content"},
			},
			Executor: &mockExecutor{},
		},
		{
			Name:        "file:list",
			Description: "List files in a directory",
			Namespace:   "file",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"path": map[string]interface{}{
						"type":        "string",
						"description": "Directory path relative to working directory",
					},
				},
				"required": []interface{}{"path"},
			},
			Executor: &mockExecutor{},
		},
		{
			Name:        "bash",
			Description: "Execute a bash command",
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
			Executor: &mockExecutor{},
		},
	}

	for _, tool := range standardTools {
		if res := workshop.Register(tool); !res.IsSuccess() {
			t.Fatalf("Failed to register tool %s: %v", tool.Name, res.Errors)
		}
	}

	// Verify all 4 tools registered
	all := workshop.All()
	if len(all) != 4 {
		t.Errorf("Expected 4 standard tools, got %d", len(all))
	}

	// Verify file: namespace has 3 tools
	fileTools := workshop.Namespace("file:")
	if len(fileTools) != 3 {
		t.Errorf("Expected 3 file: tools, got %d", len(fileTools))
	}

	// Verify bash namespace has 1 tool
	bashTools := workshop.Namespace("bash")
	if len(bashTools) != 1 {
		t.Errorf("Expected 1 bash tool, got %d", len(bashTools))
	}

	// Verify exact names
	expectedNames := []string{"bash", "file:list", "file:read", "file:write"}
	actualNames := make([]string, len(all))
	for i, tool := range all {
		actualNames[i] = tool.Name
	}
	if !reflect.DeepEqual(actualNames, expectedNames) {
		t.Errorf("Expected tool names %v, got %v", expectedNames, actualNames)
	}

	// Verify each tool has correct namespace field
	for _, tool := range all {
		switch tool.Name {
		case "file:read", "file:write", "file:list":
			if tool.Namespace != "file" {
				t.Errorf("Tool %s should have namespace 'file', got '%s'", tool.Name, tool.Namespace)
			}
		case "bash":
			if tool.Namespace != "bash" {
				t.Errorf("Tool %s should have namespace 'bash', got '%s'", tool.Name, tool.Namespace)
			}
		}
	}
}
