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
	if err == nil {
		t.Fatal("expected error for blocked path, got nil")
	}
	if called {
		t.Fatal("executor was called; expected block")
	}
	if result != "" {
		t.Errorf("expected empty result on violation, got %q", result)
	}
	if !strings.Contains(err.Error(), "I6_VIOLATION") {
		t.Errorf("expected I6_VIOLATION in error message, got %q", err.Error())
	}
	if !strings.Contains(err.Error(), "scout") {
		t.Errorf("expected role in error message, got %q", err.Error())
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
	if err == nil {
		t.Fatal("expected error for non-.yaml file, got nil")
	}
	if called {
		t.Fatal("executor was called; expected block for non-.yaml file")
	}
	if result != "" {
		t.Errorf("expected empty result on violation, got %q", result)
	}
	if !strings.Contains(err.Error(), "I6_VIOLATION") {
		t.Errorf("expected I6_VIOLATION in error message, got %q", err.Error())
	}

	// Different path entirely
	called = false
	result, err = wrapped.Execute(context.Background(), ExecutionContext{}, map[string]interface{}{
		"file_path": "docs/other.yaml",
	})
	if err == nil {
		t.Fatal("expected error for non-IMPL path, got nil")
	}
	if called {
		t.Fatal("executor was called; expected block for non-IMPL path")
	}
	if result != "" {
		t.Errorf("expected empty result on violation, got %q", result)
	}
	if !strings.Contains(err.Error(), "I6_VIOLATION") {
		t.Errorf("expected I6_VIOLATION in error message, got %q", err.Error())
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
	if err == nil {
		t.Fatal("expected error for blocked path, got nil")
	}
	if called {
		t.Fatal("executor was called; expected block for non-scaffold path")
	}
	if result != "" {
		t.Errorf("expected empty result on violation, got %q", result)
	}
	if !strings.Contains(err.Error(), "I6_VIOLATION") {
		t.Errorf("expected I6_VIOLATION in error message, got %q", err.Error())
	}
	if !strings.Contains(err.Error(), "scaffold") {
		t.Errorf("expected role in error message, got %q", err.Error())
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

func TestRolePath_PathKeyFallback(t *testing.T) {
	var called bool
	c := Constraints{
		AllowedPathPrefixes: []string{"docs/IMPL/IMPL-"},
		AgentRole:           "scout",
	}
	mw := RolePathMiddleware("edit_file", c)
	wrapped := mw(newPassthroughExecutor(&called))

	// edit_file uses "path" key — should be blocked
	_, err := wrapped.Execute(context.Background(), ExecutionContext{}, map[string]interface{}{
		"path": "pkg/foo.go",
	})
	if err == nil {
		t.Fatal("expected error for blocked path via 'path' key")
	}
	if called {
		t.Fatal("executor should not have been called")
	}

	// Allowed IMPL yaml via "path" key
	called = false
	result, err := wrapped.Execute(context.Background(), ExecutionContext{}, map[string]interface{}{
		"path": "docs/IMPL/IMPL-test.yaml",
	})
	if err != nil {
		t.Fatalf("unexpected error for allowed path via 'path' key: %v", err)
	}
	if !called {
		t.Fatal("executor should have been called for allowed path")
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
