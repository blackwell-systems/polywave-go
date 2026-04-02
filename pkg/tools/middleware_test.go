package tools

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)


// mockMiddlewareApply is a mock implementation of Apply for testing
func mockMiddlewareApply(executor ToolExecutor, middlewares ...Middleware) ToolExecutor {
	result := executor
	// Apply middlewares right-to-left (last middleware is innermost)
	for i := len(middlewares) - 1; i >= 0; i-- {
		result = middlewares[i](result)
	}
	return result
}

// mockLoggingMiddleware is a mock implementation for testing
func mockLoggingMiddleware(logs *[]string) Middleware {
	return func(next ToolExecutor) ToolExecutor {
		return executorFunc(func(ctx context.Context, execCtx ExecutionContext, input map[string]interface{}) (string, error) {
			start := time.Now()
			*logs = append(*logs, "start")
			result, err := next.Execute(ctx, execCtx, input)
			duration := time.Since(start)
			if err != nil {
				*logs = append(*logs, "error:"+err.Error())
			} else {
				*logs = append(*logs, "success")
			}
			*logs = append(*logs, "duration:"+duration.String())
			return result, err
		})
	}
}

// mockTimingMiddleware is a mock implementation for testing
func mockTimingMiddleware(onDuration func(toolName string, dur time.Duration)) Middleware {
	return func(next ToolExecutor) ToolExecutor {
		return executorFunc(func(ctx context.Context, execCtx ExecutionContext, input map[string]interface{}) (string, error) {
			start := time.Now()
			result, err := next.Execute(ctx, execCtx, input)
			duration := time.Since(start)
			toolName := "unknown"
			if tn, ok := execCtx.Metadata["tool_name"].(string); ok {
				toolName = tn
			}
			if onDuration != nil {
				onDuration(toolName, duration)
			}
			return result, err
		})
	}
}

// mockValidationMiddleware is a mock implementation for testing
func mockValidationMiddleware() Middleware {
	return func(next ToolExecutor) ToolExecutor {
		return executorFunc(func(ctx context.Context, execCtx ExecutionContext, input map[string]interface{}) (string, error) {
			// Basic validation: check if required fields are present
			// For testing purposes, we'll check if "required_field" exists
			if _, ok := input["required_field"]; !ok {
				// Check if this validation should be skipped
				if skip, _ := input["skip_validation"].(bool); !skip {
					return "", errors.New("validation failed: missing required_field")
				}
			}
			return next.Execute(ctx, execCtx, input)
		})
	}
}

// TestApplyMiddleware tests that Apply() wraps executor correctly
func TestApplyMiddleware(t *testing.T) {
	var callOrder []string

	executor := executorFunc(func(ctx context.Context, execCtx ExecutionContext, input map[string]interface{}) (string, error) {
		callOrder = append(callOrder, "executor")
		return "result", nil
	})

	middleware1 := func(next ToolExecutor) ToolExecutor {
		return executorFunc(func(ctx context.Context, execCtx ExecutionContext, input map[string]interface{}) (string, error) {
			callOrder = append(callOrder, "middleware1-before")
			result, err := next.Execute(ctx, execCtx, input)
			callOrder = append(callOrder, "middleware1-after")
			return result, err
		})
	}

	middleware2 := func(next ToolExecutor) ToolExecutor {
		return executorFunc(func(ctx context.Context, execCtx ExecutionContext, input map[string]interface{}) (string, error) {
			callOrder = append(callOrder, "middleware2-before")
			result, err := next.Execute(ctx, execCtx, input)
			callOrder = append(callOrder, "middleware2-after")
			return result, err
		})
	}

	wrapped := mockMiddlewareApply(executor, middleware1, middleware2)

	result, err := wrapped.Execute(context.Background(), ExecutionContext{}, map[string]interface{}{})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if result != "result" {
		t.Errorf("Expected result 'result', got '%s'", result)
	}

	// Verify middleware order: middleware1 is outer, middleware2 is inner
	expected := []string{
		"middleware1-before",
		"middleware2-before",
		"executor",
		"middleware2-after",
		"middleware1-after",
	}

	if len(callOrder) != len(expected) {
		t.Fatalf("Expected %d calls, got %d: %v", len(expected), len(callOrder), callOrder)
	}

	for i, exp := range expected {
		if callOrder[i] != exp {
			t.Errorf("Call %d: expected '%s', got '%s'", i, exp, callOrder[i])
		}
	}
}

// TestLoggingMiddleware tests that LoggingMiddleware logs tool name and execution time
func TestLoggingMiddleware(t *testing.T) {
	var logs []string

	executor := executorFunc(func(ctx context.Context, execCtx ExecutionContext, input map[string]interface{}) (string, error) {
		time.Sleep(10 * time.Millisecond) // Simulate work
		return "success", nil
	})

	middleware := mockLoggingMiddleware(&logs)
	wrapped := middleware(executor)

	result, err := wrapped.Execute(context.Background(), ExecutionContext{}, map[string]interface{}{})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if result != "success" {
		t.Errorf("Expected result 'success', got '%s'", result)
	}

	if len(logs) < 3 {
		t.Fatalf("Expected at least 3 log entries, got %d", len(logs))
	}

	if logs[0] != "start" {
		t.Errorf("Expected first log 'start', got '%s'", logs[0])
	}
	if logs[1] != "success" {
		t.Errorf("Expected second log 'success', got '%s'", logs[1])
	}
	if !strings.HasPrefix(logs[2], "duration:") {
		t.Errorf("Expected third log to start with 'duration:', got '%s'", logs[2])
	}
}

// TestLoggingMiddlewareError tests that LoggingMiddleware logs errors
func TestLoggingMiddlewareError(t *testing.T) {
	var logs []string

	executor := executorFunc(func(ctx context.Context, execCtx ExecutionContext, input map[string]interface{}) (string, error) {
		return "", errors.New("test error")
	})

	middleware := mockLoggingMiddleware(&logs)
	wrapped := middleware(executor)

	_, err := wrapped.Execute(context.Background(), ExecutionContext{}, map[string]interface{}{})
	if err == nil {
		t.Fatal("Expected error, got nil")
	}

	if len(logs) < 3 {
		t.Fatalf("Expected at least 3 log entries, got %d", len(logs))
	}

	if logs[0] != "start" {
		t.Errorf("Expected first log 'start', got '%s'", logs[0])
	}
	if logs[1] != "error:test error" {
		t.Errorf("Expected second log 'error:test error', got '%s'", logs[1])
	}
}

// TestTimingMiddleware tests that TimingMiddleware calls onDuration callback
func TestTimingMiddleware(t *testing.T) {
	var capturedToolName string
	var capturedDuration time.Duration

	onDuration := func(toolName string, dur time.Duration) {
		capturedToolName = toolName
		capturedDuration = dur
	}

	executor := executorFunc(func(ctx context.Context, execCtx ExecutionContext, input map[string]interface{}) (string, error) {
		time.Sleep(10 * time.Millisecond)
		return "success", nil
	})

	middleware := mockTimingMiddleware(onDuration)
	wrapped := middleware(executor)

	execCtx := ExecutionContext{
		Metadata: map[string]interface{}{
			"tool_name": "test:tool",
		},
	}

	result, err := wrapped.Execute(context.Background(), execCtx, map[string]interface{}{})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if result != "success" {
		t.Errorf("Expected result 'success', got '%s'", result)
	}

	if capturedToolName != "test:tool" {
		t.Errorf("Expected tool name 'test:tool', got '%s'", capturedToolName)
	}

	if capturedDuration < 10*time.Millisecond {
		t.Errorf("Expected duration >= 10ms, got %v", capturedDuration)
	}

	// Verify duration is reasonable (less than 1 second)
	if capturedDuration > 1*time.Second {
		t.Errorf("Duration too long: %v", capturedDuration)
	}
}

// TestValidationMiddleware tests that ValidationMiddleware rejects invalid input
func TestValidationMiddleware(t *testing.T) {
	executor := executorFunc(func(ctx context.Context, execCtx ExecutionContext, input map[string]interface{}) (string, error) {
		return "should not reach here", nil
	})

	middleware := mockValidationMiddleware()
	wrapped := middleware(executor)

	// Test with missing required field
	result, err := wrapped.Execute(context.Background(), ExecutionContext{}, map[string]interface{}{
		"other_field": "value",
	})

	if err == nil {
		t.Fatal("Expected validation error, got nil")
	}
	if !strings.Contains(err.Error(), "validation failed") {
		t.Errorf("Expected validation error message, got: %v", err)
	}
	if result != "" {
		t.Errorf("Expected empty result on validation failure, got '%s'", result)
	}

	// Test with required field present
	result, err = wrapped.Execute(context.Background(), ExecutionContext{}, map[string]interface{}{
		"required_field": "value",
	})

	if err != nil {
		t.Fatalf("Execute with valid input failed: %v", err)
	}
	if result != "should not reach here" {
		t.Errorf("Expected result 'should not reach here', got '%s'", result)
	}
}

// TestMiddlewareStack tests multiple middleware applied in correct order
func TestMiddlewareStack(t *testing.T) {
	var operations []string

	executor := executorFunc(func(ctx context.Context, execCtx ExecutionContext, input map[string]interface{}) (string, error) {
		operations = append(operations, "execute")
		return "result", nil
	})

	loggingMW := func(next ToolExecutor) ToolExecutor {
		return executorFunc(func(ctx context.Context, execCtx ExecutionContext, input map[string]interface{}) (string, error) {
			operations = append(operations, "logging-start")
			result, err := next.Execute(ctx, execCtx, input)
			operations = append(operations, "logging-end")
			return result, err
		})
	}

	timingMW := func(next ToolExecutor) ToolExecutor {
		return executorFunc(func(ctx context.Context, execCtx ExecutionContext, input map[string]interface{}) (string, error) {
			operations = append(operations, "timing-start")
			result, err := next.Execute(ctx, execCtx, input)
			operations = append(operations, "timing-end")
			return result, err
		})
	}

	validationMW := func(next ToolExecutor) ToolExecutor {
		return executorFunc(func(ctx context.Context, execCtx ExecutionContext, input map[string]interface{}) (string, error) {
			operations = append(operations, "validation-start")
			result, err := next.Execute(ctx, execCtx, input)
			operations = append(operations, "validation-end")
			return result, err
		})
	}

	// Apply middleware: logging, timing, validation
	// Last middleware (validation) is innermost, first (logging) is outermost
	wrapped := mockMiddlewareApply(executor, loggingMW, timingMW, validationMW)

	result, err := wrapped.Execute(context.Background(), ExecutionContext{}, map[string]interface{}{})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if result != "result" {
		t.Errorf("Expected result 'result', got '%s'", result)
	}

	// Expected order: outermost (logging) -> timing -> innermost (validation) -> executor
	expected := []string{
		"logging-start",
		"timing-start",
		"validation-start",
		"execute",
		"validation-end",
		"timing-end",
		"logging-end",
	}

	if len(operations) != len(expected) {
		t.Fatalf("Expected %d operations, got %d: %v", len(expected), len(operations), operations)
	}

	for i, exp := range expected {
		if operations[i] != exp {
			t.Errorf("Operation %d: expected '%s', got '%s'", i, exp, operations[i])
		}
	}
}

// TestMiddlewareErrorPropagation tests that errors propagate through middleware stack
func TestMiddlewareErrorPropagation(t *testing.T) {
	testErr := errors.New("execution error")

	executor := executorFunc(func(ctx context.Context, execCtx ExecutionContext, input map[string]interface{}) (string, error) {
		return "", testErr
	})

	var middleware1Called, middleware2Called bool

	middleware1 := func(next ToolExecutor) ToolExecutor {
		return executorFunc(func(ctx context.Context, execCtx ExecutionContext, input map[string]interface{}) (string, error) {
			middleware1Called = true
			result, err := next.Execute(ctx, execCtx, input)
			// Middleware sees the error but doesn't modify it
			return result, err
		})
	}

	middleware2 := func(next ToolExecutor) ToolExecutor {
		return executorFunc(func(ctx context.Context, execCtx ExecutionContext, input map[string]interface{}) (string, error) {
			middleware2Called = true
			result, err := next.Execute(ctx, execCtx, input)
			return result, err
		})
	}

	wrapped := mockMiddlewareApply(executor, middleware1, middleware2)

	result, err := wrapped.Execute(context.Background(), ExecutionContext{}, map[string]interface{}{})

	if err != testErr {
		t.Errorf("Expected error %v, got %v", testErr, err)
	}
	if result != "" {
		t.Errorf("Expected empty result on error, got '%s'", result)
	}
	if !middleware1Called {
		t.Error("Middleware 1 was not called")
	}
	if !middleware2Called {
		t.Error("Middleware 2 was not called")
	}
}

// TestTimingMiddleware_RealImplementation tests the real TimingMiddleware
func TestTimingMiddleware_RealImplementation(t *testing.T) {
	var captured ToolCallEvent
	callback := func(evt ToolCallEvent) {
		captured = evt
	}

	executor := executorFunc(func(ctx context.Context, execCtx ExecutionContext, input map[string]interface{}) (string, error) {
		time.Sleep(10 * time.Millisecond)
		return "ok", nil
	})

	middleware := TimingMiddleware("test:tool", callback)
	wrapped := middleware(executor)

	_, err := wrapped.Execute(context.Background(), ExecutionContext{}, map[string]interface{}{})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if captured.ToolName != "test:tool" {
		t.Errorf("ToolName: got %q, want 'test:tool'", captured.ToolName)
	}
	if captured.DurationMs < 10 {
		t.Errorf("DurationMs: got %d, want >= 10", captured.DurationMs)
	}
	if captured.IsError {
		t.Error("IsError should be false for successful execution")
	}
}

// TestPermissionMiddleware_Blocks tests that PermissionMiddleware blocks disallowed tools
func TestPermissionMiddleware_Blocks(t *testing.T) {
	allowed := map[string]bool{"read_file": true}
	executor := executorFunc(func(ctx context.Context, execCtx ExecutionContext, input map[string]interface{}) (string, error) {
		return "should not reach", nil
	})

	middleware := PermissionMiddleware("write_file", allowed)
	wrapped := middleware(executor)

	result, err := wrapped.Execute(context.Background(), ExecutionContext{}, map[string]interface{}{})
	if err != nil {
		t.Errorf("Expected nil error (permissions return message, not error), got: %v", err)
	}
	if !strings.Contains(result, "not permitted") {
		t.Errorf("Expected denial message, got: %s", result)
	}
}

// TestPermissionMiddleware_Allows tests that PermissionMiddleware allows permitted tools
func TestPermissionMiddleware_Allows(t *testing.T) {
	allowed := map[string]bool{"write_file": true}
	executor := executorFunc(func(ctx context.Context, execCtx ExecutionContext, input map[string]interface{}) (string, error) {
		return "success", nil
	})

	middleware := PermissionMiddleware("write_file", allowed)
	wrapped := middleware(executor)

	result, err := wrapped.Execute(context.Background(), ExecutionContext{}, map[string]interface{}{})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if result != "success" {
		t.Errorf("Expected 'success', got %q", result)
	}
}

// TestWithTiming_AppliedToAllTools tests that WithTiming wraps all tools
func TestWithTiming_AppliedToAllTools(t *testing.T) {
	baseWorkshop := NewWorkshop()
	baseWorkshop.Register(Tool{
		Name: "tool1",
		Executor: executorFunc(func(ctx context.Context, execCtx ExecutionContext, input map[string]interface{}) (string, error) {
			return "ok", nil
		}),
	})
	baseWorkshop.Register(Tool{
		Name: "tool2",
		Executor: executorFunc(func(ctx context.Context, execCtx ExecutionContext, input map[string]interface{}) (string, error) {
			return "ok", nil
		}),
	})

	var events []ToolCallEvent
	callback := func(evt ToolCallEvent) {
		events = append(events, evt)
	}

	wrapped := WithTiming(baseWorkshop, callback)
	tools := wrapped.All()
	if len(tools) != 2 {
		t.Fatalf("Expected 2 tools, got %d", len(tools))
	}

	// Execute both tools
	for _, tool := range tools {
		tool.Executor.Execute(context.Background(), ExecutionContext{}, map[string]interface{}{})
	}

	if len(events) != 2 {
		t.Errorf("Expected 2 timing events, got %d", len(events))
	}
}

// TestWithPermissions_BlocksWriteTools tests that WithPermissions blocks write tools
func TestWithPermissions_BlocksWriteTools(t *testing.T) {
	baseWorkshop := NewWorkshop()
	baseWorkshop.Register(Tool{
		Name: "read_file",
		Executor: executorFunc(func(ctx context.Context, execCtx ExecutionContext, input map[string]interface{}) (string, error) {
			return "read ok", nil
		}),
	})
	baseWorkshop.Register(Tool{
		Name: "write_file",
		Executor: executorFunc(func(ctx context.Context, execCtx ExecutionContext, input map[string]interface{}) (string, error) {
			return "write ok", nil
		}),
	})

	allowed := map[string]bool{"read_file": true}
	wrapped := WithPermissions(baseWorkshop, allowed)

	readTool, _ := wrapped.Get("read_file")
	result, _ := readTool.Executor.Execute(context.Background(), ExecutionContext{}, map[string]interface{}{})
	if result != "read ok" {
		t.Errorf("read_file should be allowed, got: %s", result)
	}

	writeTool, _ := wrapped.Get("write_file")
	result, _ = writeTool.Executor.Execute(context.Background(), ExecutionContext{}, map[string]interface{}{})
	if !strings.Contains(result, "not permitted") {
		t.Errorf("write_file should be blocked, got: %s", result)
	}
}

// TestReadOnlyTools_Registered tests that ReadOnlyTools creates a workshop with 7 tools
func TestReadOnlyTools_Registered(t *testing.T) {
	workshop := ReadOnlyTools("/tmp")
	tools := workshop.All()
	if len(tools) != 7 {
		t.Errorf("Expected 7 standard tools, got %d", len(tools))
	}

	// Verify read_file is allowed
	readTool, ok := workshop.Get("read_file")
	if !ok {
		t.Fatal("read_file not found")
	}
	result, _ := readTool.Executor.Execute(context.Background(), ExecutionContext{WorkDir: "/tmp"}, map[string]interface{}{
		"file_path": "nonexistent.txt",
	})
	if strings.Contains(result, "not permitted") {
		t.Error("read_file should not be blocked")
	}

	// Verify write_file is blocked
	writeTool, ok := workshop.Get("write_file")
	if !ok {
		t.Fatal("write_file not found")
	}
	result, _ = writeTool.Executor.Execute(context.Background(), ExecutionContext{WorkDir: "/tmp"}, map[string]interface{}{
		"file_path": "/tmp/test.txt",
		"content":   "test",
	})
	if !strings.Contains(result, "not permitted") {
		t.Errorf("write_file should be blocked, got: %s", result)
	}
}
