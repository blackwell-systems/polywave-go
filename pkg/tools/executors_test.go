package tools

import (
	"context"
	"strings"
	"testing"
)

func TestExtractStringInput_NilMap(t *testing.T) {
	_, ok := extractStringInput(nil, "file_path")
	if ok {
		t.Error("expected false for nil map")
	}
}

func TestExtractStringInput_MissingKey(t *testing.T) {
	_, ok := extractStringInput(map[string]interface{}{"other": "val"}, "file_path")
	if ok {
		t.Error("expected false for missing key")
	}
}

func TestExtractStringInput_WrongType(t *testing.T) {
	_, ok := extractStringInput(map[string]interface{}{"file_path": 42}, "file_path")
	if ok {
		t.Error("expected false for non-string value")
	}
}

func TestExtractStringInput_EmptyString(t *testing.T) {
	_, ok := extractStringInput(map[string]interface{}{"file_path": ""}, "file_path")
	if ok {
		t.Error("expected false for empty string")
	}
}

func TestExtractStringInput_ValidString(t *testing.T) {
	v, ok := extractStringInput(map[string]interface{}{"file_path": "foo.go"}, "file_path")
	if !ok {
		t.Error("expected true for valid string")
	}
	if v != "foo.go" {
		t.Errorf("expected 'foo.go', got %q", v)
	}
}

func TestFileWriteExecutor_MissingFilePath(t *testing.T) {
	ex := &FileWriteExecutor{}
	result, err := ex.Execute(context.Background(),
		ExecutionContext{WorkDir: "/tmp"},
		map[string]interface{}{"content": "hello"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "input validation failed") {
		t.Errorf("expected validation error message, got %q", result)
	}
}

func TestFileWriteExecutor_WrongTypeFilePath(t *testing.T) {
	ex := &FileWriteExecutor{}
	result, err := ex.Execute(context.Background(),
		ExecutionContext{WorkDir: "/tmp"},
		map[string]interface{}{"file_path": 123, "content": "hello"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "input validation failed") {
		t.Errorf("expected validation error for wrong type, got %q", result)
	}
}

func TestBashExecutor_MissingCommand(t *testing.T) {
	ex := &BashExecutor{}
	result, err := ex.Execute(context.Background(),
		ExecutionContext{WorkDir: "/tmp"},
		map[string]interface{}{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "input validation failed") {
		t.Errorf("expected validation error message, got %q", result)
	}
}

func TestGrepFallback_NonexistentRoot(t *testing.T) {
	// Should not panic; may return walk error note or empty string
	result := grepFallback("/nonexistent/path/zzzz", "pattern")
	_ = result // just verify no panic
}

func TestGrepFallback_FindsMatches(t *testing.T) {
	// Use /tmp which should exist
	_ = grepFallback("/tmp", "test")
	// Just verify no panic; /tmp may or may not have matching files
}
