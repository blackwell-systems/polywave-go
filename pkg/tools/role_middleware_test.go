package tools

import (
	"context"
	"strings"
	"testing"
)

// newPassthroughExecutor returns an executor that records it was called and returns "ok".
func newPassthroughExecutor(called *bool) ToolExecutor {
	return executorFunc(func(ctx context.Context, execCtx ExecutionContext, input map[string]interface{}) (string, error) {
		*called = true
		return "ok", nil
	})
}

func TestRolePath_ScoutAllowsIMPLYaml(t *testing.T) {
	var called bool
	c := Constraints{
		AllowedPathPrefixes: []string{"docs/IMPL/IMPL-"},
		AgentRole:           "scout",
	}
	mw := RolePathMiddleware("write_file", c)
	wrapped := mw(newPassthroughExecutor(&called))

	result, err := wrapped.Execute(context.Background(), ExecutionContext{}, map[string]interface{}{
		"file_path": "docs/IMPL/IMPL-foo.yaml",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Fatal("executor was not called; expected passthrough")
	}
	if result != "ok" {
		t.Errorf("expected 'ok', got %q", result)
	}
}

func TestRolePath_ScoutBlocksSourceCode(t *testing.T) {
	var called bool
	c := Constraints{
		AllowedPathPrefixes: []string{"docs/IMPL/IMPL-"},
		AgentRole:           "scout",
	}
	mw := RolePathMiddleware("write_file", c)
	wrapped := mw(newPassthroughExecutor(&called))

	result, err := wrapped.Execute(context.Background(), ExecutionContext{}, map[string]interface{}{
		"file_path": "pkg/foo.go",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if called {
		t.Fatal("executor was called; expected block")
	}
	if !strings.Contains(result, "BLOCKED") {
		t.Errorf("expected BLOCKED message, got %q", result)
	}
	if !strings.Contains(result, "scout") {
		t.Errorf("expected role in message, got %q", result)
	}
}

func TestRolePath_ScoutBlocksNonIMPLYaml(t *testing.T) {
	var called bool
	c := Constraints{
		AllowedPathPrefixes: []string{"docs/IMPL/IMPL-"},
		AgentRole:           "scout",
	}
	mw := RolePathMiddleware("edit_file", c)
	wrapped := mw(newPassthroughExecutor(&called))

	// Has the right prefix but wrong extension
	result, err := wrapped.Execute(context.Background(), ExecutionContext{}, map[string]interface{}{
		"file_path": "docs/IMPL/IMPL-foo.txt",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if called {
		t.Fatal("executor was called; expected block for non-.yaml file")
	}
	if !strings.Contains(result, "BLOCKED") {
		t.Errorf("expected BLOCKED message, got %q", result)
	}

	// Different path entirely
	called = false
	result, err = wrapped.Execute(context.Background(), ExecutionContext{}, map[string]interface{}{
		"file_path": "docs/other.yaml",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if called {
		t.Fatal("executor was called; expected block for non-IMPL path")
	}
	if !strings.Contains(result, "BLOCKED") {
		t.Errorf("expected BLOCKED message, got %q", result)
	}
}

func TestRolePath_ScaffoldAllowsScaffoldPaths(t *testing.T) {
	var called bool
	c := Constraints{
		AllowedPathPrefixes: []string{"pkg/tools/", "internal/scaffold/"},
		AgentRole:           "scaffold",
	}
	mw := RolePathMiddleware("write_file", c)
	wrapped := mw(newPassthroughExecutor(&called))

	result, err := wrapped.Execute(context.Background(), ExecutionContext{}, map[string]interface{}{
		"file_path": "pkg/tools/constraints.go",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Fatal("executor was not called; expected passthrough for scaffold path")
	}
	if result != "ok" {
		t.Errorf("expected 'ok', got %q", result)
	}

	// Test second prefix
	called = false
	result, err = wrapped.Execute(context.Background(), ExecutionContext{}, map[string]interface{}{
		"file_path": "internal/scaffold/main.go",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Fatal("executor was not called; expected passthrough for second scaffold path")
	}
}

func TestRolePath_ScaffoldBlocksOtherPaths(t *testing.T) {
	var called bool
	c := Constraints{
		AllowedPathPrefixes: []string{"pkg/tools/", "internal/scaffold/"},
		AgentRole:           "scaffold",
	}
	mw := RolePathMiddleware("edit_file", c)
	wrapped := mw(newPassthroughExecutor(&called))

	result, err := wrapped.Execute(context.Background(), ExecutionContext{}, map[string]interface{}{
		"file_path": "cmd/main.go",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if called {
		t.Fatal("executor was called; expected block for non-scaffold path")
	}
	if !strings.Contains(result, "BLOCKED") {
		t.Errorf("expected BLOCKED message, got %q", result)
	}
	if !strings.Contains(result, "scaffold") {
		t.Errorf("expected role in message, got %q", result)
	}
}

func TestRolePath_EmptyPrefixes_Passthrough(t *testing.T) {
	var called bool
	c := Constraints{
		AllowedPathPrefixes: nil,
		AgentRole:           "wave",
	}
	mw := RolePathMiddleware("write_file", c)
	wrapped := mw(newPassthroughExecutor(&called))

	result, err := wrapped.Execute(context.Background(), ExecutionContext{}, map[string]interface{}{
		"file_path": "anywhere/anything.go",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Fatal("executor was not called; expected passthrough with empty prefixes")
	}
	if result != "ok" {
		t.Errorf("expected 'ok', got %q", result)
	}
}

func TestRolePath_OnlyAppliesToWriteTools(t *testing.T) {
	readTools := []string{"read_file", "list_directory", "glob", "grep", "bash"}

	for _, toolName := range readTools {
		var called bool
		c := Constraints{
			AllowedPathPrefixes: []string{"docs/IMPL/IMPL-"},
			AgentRole:           "scout",
		}
		mw := RolePathMiddleware(toolName, c)
		wrapped := mw(newPassthroughExecutor(&called))

		result, err := wrapped.Execute(context.Background(), ExecutionContext{}, map[string]interface{}{
			"file_path": "pkg/secret.go",
		})
		if err != nil {
			t.Fatalf("[%s] unexpected error: %v", toolName, err)
		}
		if !called {
			t.Errorf("[%s] executor was not called; read tools should always passthrough", toolName)
		}
		if result != "ok" {
			t.Errorf("[%s] expected 'ok', got %q", toolName, result)
		}
	}
}
