package tools

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"
)

// constraintMockExecutor records calls and returns a configurable result.
type constraintMockExecutor struct {
	called bool
	result string
	err    error
}

func (m *constraintMockExecutor) Execute(_ context.Context, _ ExecutionContext, _ map[string]interface{}) (string, error) {
	m.called = true
	return m.result, m.err
}

// mockMiddleware returns a middleware that records whether it was applied
// and optionally blocks execution with a message.
func mockMiddleware(name string, applied *[]string, blockMsg string) Middleware {
	return func(next ToolExecutor) ToolExecutor {
		return executorFunc(func(ctx context.Context, execCtx ExecutionContext, input map[string]interface{}) (string, error) {
			*applied = append(*applied, name)
			if blockMsg != "" {
				return blockMsg, nil
			}
			return next.Execute(ctx, execCtx, input)
		})
	}
}

// blockingOwnership returns a mock OwnershipMiddleware that blocks unowned files.
func blockingOwnership(toolName string, c Constraints) Middleware {
	return func(next ToolExecutor) ToolExecutor {
		return executorFunc(func(ctx context.Context, execCtx ExecutionContext, input map[string]interface{}) (string, error) {
			path, _ := input["file_path"].(string)
			if !c.OwnedFiles[path] {
				return fmt.Sprintf("BLOCKED: file %s is not in your ownership list", path), nil
			}
			return next.Execute(ctx, execCtx, input)
		})
	}
}

// blockingFreeze returns a mock FreezeMiddleware that blocks frozen files.
func blockingFreeze(toolName string, c Constraints) Middleware {
	return func(next ToolExecutor) ToolExecutor {
		return executorFunc(func(ctx context.Context, execCtx ExecutionContext, input map[string]interface{}) (string, error) {
			path, _ := input["file_path"].(string)
			if c.FrozenPaths[path] && c.FreezeTime != nil {
				return fmt.Sprintf("BLOCKED: file %s is frozen", path), nil
			}
			return next.Execute(ctx, execCtx, input)
		})
	}
}

// blockingRolePath returns a mock RolePathMiddleware that blocks paths outside allowed prefixes.
func blockingRolePath(toolName string, c Constraints) Middleware {
	return func(next ToolExecutor) ToolExecutor {
		return executorFunc(func(ctx context.Context, execCtx ExecutionContext, input map[string]interface{}) (string, error) {
			path, _ := input["file_path"].(string)
			for _, prefix := range c.AllowedPathPrefixes {
				if strings.HasPrefix(path, prefix) {
					return next.Execute(ctx, execCtx, input)
				}
			}
			return fmt.Sprintf("BLOCKED: %s agent cannot write to %s", c.AgentRole, path), nil
		})
	}
}

// trackingBash returns a mock BashConstraintMiddleware that tracks git commits.
func trackingBash(c Constraints, tracker *CommitTracker) Middleware {
	return func(next ToolExecutor) ToolExecutor {
		return executorFunc(func(ctx context.Context, execCtx ExecutionContext, input map[string]interface{}) (string, error) {
			cmd, _ := input["command"].(string)
			if tracker != nil && strings.Contains(cmd, "git commit") {
				tracker.Count++
			}
			return next.Execute(ctx, execCtx, input)
		})
	}
}

// buildTestWorkshop creates a Workshop with write_file, edit_file, bash, and read_file tools.
func buildTestWorkshop() Workshop {
	w := NewWorkshop()
	for _, name := range []string{"write_file", "edit_file", "bash", "read_file", "glob", "grep", "list_directory"} {
		w.Register(Tool{
			Name:     name,
			Executor: &constraintMockExecutor{result: "ok:" + name},
		})
	}
	return w
}

func TestWithConstraints_ZeroValue_Passthrough(t *testing.T) {
	orig := buildTestWorkshop()
	result, tracker := WithConstraints(orig, Constraints{})

	if tracker != nil {
		t.Fatal("expected nil tracker for zero-value Constraints")
	}

	// Should return the exact same workshop instance
	if fmt.Sprintf("%p", result) != fmt.Sprintf("%p", orig) {
		t.Fatal("expected same workshop pointer for zero-value Constraints")
	}
}

func TestWithConstraints_OwnershipApplied(t *testing.T) {
	// Inject blocking ownership middleware
	origFn := ownershipMiddlewareFn
	ownershipMiddlewareFn = blockingOwnership
	defer func() { ownershipMiddlewareFn = origFn }()

	w := buildTestWorkshop()
	c := Constraints{
		OwnedFiles: map[string]bool{"owned.go": true},
	}

	constrained, _ := WithConstraints(w, c)

	// write_file to unowned file should be blocked
	tool, ok := constrained.Get("write_file")
	if !ok {
		t.Fatal("write_file not found")
	}

	result, err := tool.Executor.Execute(context.Background(), ExecutionContext{}, map[string]interface{}{
		"file_path": "unowned.go",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "BLOCKED") {
		t.Fatalf("expected BLOCKED message, got: %s", result)
	}

	// write_file to owned file should pass through
	result, err = tool.Executor.Execute(context.Background(), ExecutionContext{}, map[string]interface{}{
		"file_path": "owned.go",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(result, "BLOCKED") {
		t.Fatalf("expected passthrough, got: %s", result)
	}
}

func TestWithConstraints_FreezeApplied(t *testing.T) {
	origFn := freezeMiddlewareFn
	freezeMiddlewareFn = blockingFreeze
	defer func() { freezeMiddlewareFn = origFn }()

	now := time.Now()
	w := buildTestWorkshop()
	c := Constraints{
		FrozenPaths: map[string]bool{"frozen.go": true},
		FreezeTime:  &now,
	}

	constrained, _ := WithConstraints(w, c)

	tool, _ := constrained.Get("edit_file")
	result, err := tool.Executor.Execute(context.Background(), ExecutionContext{}, map[string]interface{}{
		"file_path": "frozen.go",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "BLOCKED") {
		t.Fatalf("expected BLOCKED for frozen file, got: %s", result)
	}

	// Non-frozen file should pass through
	result, _ = tool.Executor.Execute(context.Background(), ExecutionContext{}, map[string]interface{}{
		"file_path": "ok.go",
	})
	if strings.Contains(result, "BLOCKED") {
		t.Fatalf("expected passthrough for non-frozen file, got: %s", result)
	}
}

func TestWithConstraints_RoleApplied(t *testing.T) {
	origFn := rolePathMiddlewareFn
	rolePathMiddlewareFn = blockingRolePath
	defer func() { rolePathMiddlewareFn = origFn }()

	w := buildTestWorkshop()
	c := Constraints{
		AllowedPathPrefixes: []string{"docs/IMPL/IMPL-"},
		AgentRole:           "scout",
	}

	constrained, _ := WithConstraints(w, c)

	tool, _ := constrained.Get("write_file")

	// Allowed path
	result, _ := tool.Executor.Execute(context.Background(), ExecutionContext{}, map[string]interface{}{
		"file_path": "docs/IMPL/IMPL-foo.yaml",
	})
	if strings.Contains(result, "BLOCKED") {
		t.Fatalf("expected passthrough for allowed path, got: %s", result)
	}

	// Disallowed path
	result, _ = tool.Executor.Execute(context.Background(), ExecutionContext{}, map[string]interface{}{
		"file_path": "pkg/foo.go",
	})
	if !strings.Contains(result, "BLOCKED") {
		t.Fatalf("expected BLOCKED for disallowed path, got: %s", result)
	}
}

func TestWithConstraints_BashTracking(t *testing.T) {
	origFn := bashConstraintMiddlewareFn
	bashConstraintMiddlewareFn = trackingBash
	defer func() { bashConstraintMiddlewareFn = origFn }()

	w := buildTestWorkshop()
	c := Constraints{
		TrackCommits: true,
		OwnedFiles:   map[string]bool{"a.go": true}, // need non-zero constraints
	}

	constrained, tracker := WithConstraints(w, c)

	if tracker == nil {
		t.Fatal("expected non-nil tracker when TrackCommits=true")
	}

	tool, _ := constrained.Get("bash")

	// Execute git commit
	_, _ = tool.Executor.Execute(context.Background(), ExecutionContext{}, map[string]interface{}{
		"command": "git commit -m 'test'",
	})

	if tracker.Count != 1 {
		t.Fatalf("expected tracker count 1, got %d", tracker.Count)
	}

	// Execute non-commit command
	_, _ = tool.Executor.Execute(context.Background(), ExecutionContext{}, map[string]interface{}{
		"command": "ls -la",
	})

	if tracker.Count != 1 {
		t.Fatalf("expected tracker count still 1, got %d", tracker.Count)
	}
}

func TestWithConstraints_CommitTracker_NilWhenDisabled(t *testing.T) {
	w := buildTestWorkshop()
	c := Constraints{
		TrackCommits: false,
		OwnedFiles:   map[string]bool{"a.go": true},
	}

	_, tracker := WithConstraints(w, c)
	if tracker != nil {
		t.Fatal("expected nil tracker when TrackCommits=false")
	}
}

func TestConstrainedTools_Integration(t *testing.T) {
	origOwn := ownershipMiddlewareFn
	origBash := bashConstraintMiddlewareFn
	ownershipMiddlewareFn = blockingOwnership
	bashConstraintMiddlewareFn = trackingBash
	defer func() {
		ownershipMiddlewareFn = origOwn
		bashConstraintMiddlewareFn = origBash
	}()

	c := Constraints{
		OwnedFiles:   map[string]bool{"pkg/foo.go": true},
		TrackCommits: true,
	}

	w, tracker := ConstrainedTools("/tmp/test", c)

	// Verify all 7 standard tools are present
	tools := w.All()
	if len(tools) != 7 {
		t.Fatalf("expected 7 tools, got %d", len(tools))
	}

	// Verify tracker is non-nil
	if tracker == nil {
		t.Fatal("expected non-nil tracker")
	}

	// Verify ownership applies to write_file
	writeTool, _ := w.Get("write_file")
	result, _ := writeTool.Executor.Execute(context.Background(), ExecutionContext{}, map[string]interface{}{
		"file_path": "unowned.go",
	})
	if !strings.Contains(result, "BLOCKED") {
		t.Fatalf("expected BLOCKED for unowned file via ConstrainedTools, got: %s", result)
	}

	// Verify read_file is unconstrained
	readTool, _ := w.Get("read_file")
	result, _ = readTool.Executor.Execute(context.Background(), ExecutionContext{WorkDir: "/tmp/test"}, map[string]interface{}{
		"file_path": "/dev/null",
	})
	// read_file should call through to the real executor (not blocked)
	if strings.Contains(result, "BLOCKED") {
		t.Fatal("read_file should not be constrained")
	}
}

func TestWithConstraints_MultipleMiddleware_Order(t *testing.T) {
	// Track the order in which middleware executes
	var order []string

	origOwn := ownershipMiddlewareFn
	origFreeze := freezeMiddlewareFn
	origRole := rolePathMiddlewareFn
	defer func() {
		ownershipMiddlewareFn = origOwn
		freezeMiddlewareFn = origFreeze
		rolePathMiddlewareFn = origRole
	}()

	// Each middleware appends its name to the order slice
	rolePathMiddlewareFn = func(_ string, _ Constraints) Middleware {
		return mockMiddleware("role", &order, "")
	}
	freezeMiddlewareFn = func(_ string, _ Constraints) Middleware {
		return mockMiddleware("freeze", &order, "")
	}
	ownershipMiddlewareFn = func(_ string, _ Constraints) Middleware {
		return mockMiddleware("ownership", &order, "")
	}

	now := time.Now()
	w := buildTestWorkshop()
	c := Constraints{
		OwnedFiles:          map[string]bool{"a.go": true},
		FrozenPaths:         map[string]bool{"b.go": true},
		FreezeTime:          &now,
		AllowedPathPrefixes: []string{"docs/"},
	}

	constrained, _ := WithConstraints(w, c)

	tool, _ := constrained.Get("write_file")
	_, _ = tool.Executor.Execute(context.Background(), ExecutionContext{}, map[string]interface{}{
		"file_path": "docs/a.go",
	})

	// Verify middleware executed in the correct order:
	// RolePath (outermost) -> Freeze -> Ownership (innermost)
	expected := []string{"role", "freeze", "ownership"}
	if len(order) != len(expected) {
		t.Fatalf("expected %d middleware calls, got %d: %v", len(expected), len(order), order)
	}
	for i, name := range expected {
		if order[i] != name {
			t.Fatalf("expected middleware[%d]=%s, got %s (full order: %v)", i, name, order[i], order)
		}
	}
}

func TestWithConstraints_ReadToolsUnconstrained(t *testing.T) {
	// Verify that read-only tools get no middleware at all
	origOwn := ownershipMiddlewareFn
	ownershipMiddlewareFn = func(_ string, _ Constraints) Middleware {
		return func(next ToolExecutor) ToolExecutor {
			return executorFunc(func(ctx context.Context, execCtx ExecutionContext, input map[string]interface{}) (string, error) {
				return "SHOULD_NOT_SEE_THIS", nil
			})
		}
	}
	defer func() { ownershipMiddlewareFn = origOwn }()

	w := buildTestWorkshop()
	c := Constraints{
		OwnedFiles: map[string]bool{"a.go": true},
	}

	constrained, _ := WithConstraints(w, c)

	for _, name := range []string{"read_file", "glob", "grep", "list_directory"} {
		tool, ok := constrained.Get(name)
		if !ok {
			t.Fatalf("tool %s not found", name)
		}
		result, _ := tool.Executor.Execute(context.Background(), ExecutionContext{}, nil)
		if result == "SHOULD_NOT_SEE_THIS" {
			t.Fatalf("tool %s should not have constraint middleware applied", name)
		}
	}
}

func TestWithConstraints_BashToolNotGetWriteMiddleware(t *testing.T) {
	// Verify bash tool gets BashConstraintMiddleware, NOT ownership/freeze middleware
	var ownershipExecuted bool
	origOwn := ownershipMiddlewareFn
	origBash := bashConstraintMiddlewareFn
	ownershipMiddlewareFn = func(_ string, _ Constraints) Middleware {
		return func(next ToolExecutor) ToolExecutor {
			return executorFunc(func(ctx context.Context, execCtx ExecutionContext, input map[string]interface{}) (string, error) {
				ownershipExecuted = true
				return next.Execute(ctx, execCtx, input)
			})
		}
	}
	bashConstraintMiddlewareFn = func(_ Constraints, _ *CommitTracker) Middleware {
		return func(next ToolExecutor) ToolExecutor {
			return executorFunc(func(ctx context.Context, execCtx ExecutionContext, input map[string]interface{}) (string, error) {
				return "bash-constrained", nil
			})
		}
	}
	defer func() {
		ownershipMiddlewareFn = origOwn
		bashConstraintMiddlewareFn = origBash
	}()

	w := buildTestWorkshop()
	c := Constraints{
		OwnedFiles:   map[string]bool{"a.go": true},
		TrackCommits: true,
	}

	constrained, _ := WithConstraints(w, c)

	tool, _ := constrained.Get("bash")
	result, _ := tool.Executor.Execute(context.Background(), ExecutionContext{}, map[string]interface{}{
		"command": "echo hello",
	})

	if result != "bash-constrained" {
		t.Fatalf("expected bash to use BashConstraintMiddleware, got: %s", result)
	}
	if ownershipExecuted {
		t.Fatal("ownership middleware should not be executed for bash tool")
	}
}
