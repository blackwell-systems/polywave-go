package backend

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/blackwell-systems/polywave-go/pkg/tools"
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

// TestExecuteTool_Success verifies that ExecuteTool returns the tool result and nil error
// when the tool is found and executes successfully.
func TestExecuteTool_Success(t *testing.T) {
	w := newMockWorkshop(tools.Tool{
		Name:     "bash",
		Executor: &mockExecutor{result: "hello world"},
	})

	result, err := ExecuteTool(context.Background(), w, "bash", map[string]interface{}{
		"command": "echo hello",
	}, t.TempDir())

	if err != nil {
		t.Errorf("expected nil error, got %v", err)
	}
	if result != "hello world" {
		t.Errorf("expected %q, got %q", "hello world", result)
	}
}

// TestExecuteTool_NotFound verifies that ExecuteTool returns an error wrapping
// ErrToolNotFound when the tool name is not registered.
func TestExecuteTool_NotFound(t *testing.T) {
	w := tools.NewWorkshop()

	result, err := ExecuteTool(context.Background(), w, "nonexistent", nil, t.TempDir())

	if err == nil {
		t.Fatal("expected non-nil error for unknown tool")
	}
	if !errors.Is(err, ErrToolNotFound) {
		t.Errorf("expected error to wrap ErrToolNotFound, got %v", err)
	}
	if !strings.Contains(result, "unknown tool") {
		t.Errorf("expected result to contain 'unknown tool', got %q", result)
	}
	if !strings.Contains(result, "nonexistent") {
		t.Errorf("expected result to contain tool name 'nonexistent', got %q", result)
	}
}

// TestExecuteTool_ExecError verifies that ExecuteTool returns the execution error
// and an error message result when the tool executor fails.
func TestExecuteTool_ExecError(t *testing.T) {
	execErr := fmt.Errorf("disk full")
	w := newMockWorkshop(tools.Tool{
		Name:     "failing_tool",
		Executor: &mockExecutor{err: execErr},
	})

	result, err := ExecuteTool(context.Background(), w, "failing_tool", nil, t.TempDir())

	if err == nil {
		t.Fatal("expected non-nil error for execution failure")
	}
	if !errors.Is(err, execErr) {
		t.Errorf("expected error to be the execution error, got %v", err)
	}
	if !strings.Contains(result, "disk full") {
		t.Errorf("expected result to contain error message, got %q", result)
	}
	// Should NOT be ErrToolNotFound
	if errors.Is(err, ErrToolNotFound) {
		t.Error("execution error should not wrap ErrToolNotFound")
	}
}
