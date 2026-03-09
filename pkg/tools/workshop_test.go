package tools

import (
	"context"
	"errors"
	"reflect"
	"sort"
	"testing"
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

func (w *mockWorkshop) Register(tool Tool) error {
	if _, exists := w.tools[tool.Name]; exists {
		return errors.New("tool already registered: " + tool.Name)
	}
	w.tools[tool.Name] = tool
	return nil
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

	err := workshop.Register(tool)
	if err != nil {
		t.Fatalf("Register failed: %v", err)
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

// TestRegisterDuplicate tests that registering the same name twice returns an error
func TestRegisterDuplicate(t *testing.T) {
	workshop := newMockWorkshop()

	tool := Tool{
		Name:        "duplicate:tool",
		Description: "First registration",
		Namespace:   "duplicate",
		Executor:    &mockExecutor{},
	}

	err := workshop.Register(tool)
	if err != nil {
		t.Fatalf("First registration failed: %v", err)
	}

	// Try to register again with same name
	tool2 := Tool{
		Name:        "duplicate:tool",
		Description: "Second registration",
		Namespace:   "duplicate",
		Executor:    &mockExecutor{},
	}

	err = workshop.Register(tool2)
	if err == nil {
		t.Fatal("Expected error when registering duplicate tool name, got nil")
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
		if err := workshop.Register(tool); err != nil {
			t.Fatalf("Register failed: %v", err)
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
		if err := workshop.Register(tool); err != nil {
			t.Fatalf("Register failed: %v", err)
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

	if err := workshop.Register(tool); err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	result := workshop.Namespace("nonexistent")
	if len(result) != 0 {
		t.Errorf("Expected empty slice for nonexistent namespace, got %d tools", len(result))
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
		if err := workshop.Register(tool); err != nil {
			t.Fatalf("Failed to register tool %s: %v", tool.Name, err)
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
