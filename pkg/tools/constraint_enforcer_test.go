// pkg/tools/constraint_enforcer_test.go
package tools

import (
	"context"
	"strings"
	"testing"
	"time"
)

// enforcerPassthrough is a simple passthrough executor for constraint tests.
func enforcerPassthrough() ToolExecutor {
	return executorFunc(func(ctx context.Context, execCtx ExecutionContext, input map[string]interface{}) (string, error) {
		return "ok", nil
	})
}

// TestConstraintEnforcer_I1_OwnedFileSucceeds verifies that writing to an owned
// file passes through the ownership middleware without error.
func TestConstraintEnforcer_I1_OwnedFileSucceeds(t *testing.T) {
	c := Constraints{
		AgentID:    "A",
		OwnedFiles: map[string]bool{"pkg/tools/myfile.go": true},
	}
	mw := newOwnershipMiddleware("write_file", c)
	wrapped := mw(enforcerPassthrough())

	result, err := wrapped.Execute(context.Background(), ExecutionContext{}, map[string]interface{}{
		"file_path": "pkg/tools/myfile.go",
	})
	if err != nil {
		t.Fatalf("unexpected error for owned file: %v", err)
	}
	if result != "ok" {
		t.Errorf("expected 'ok', got %q", result)
	}
}

// TestConstraintEnforcer_I1_UnownedFileReturnsViolation verifies that writing to
// a file not in OwnedFiles returns an I1_VIOLATION error.
func TestConstraintEnforcer_I1_UnownedFileReturnsViolation(t *testing.T) {
	c := Constraints{
		AgentID:    "B",
		OwnedFiles: map[string]bool{"pkg/tools/myfile.go": true},
	}
	mw := newOwnershipMiddleware("write_file", c)
	wrapped := mw(enforcerPassthrough())

	_, err := wrapped.Execute(context.Background(), ExecutionContext{}, map[string]interface{}{
		"file_path": "pkg/tools/other.go",
	})
	if err == nil {
		t.Fatal("expected I1_VIOLATION error, got nil")
	}
	if !strings.Contains(err.Error(), "I1_VIOLATION") {
		t.Errorf("expected I1_VIOLATION in error, got: %v", err)
	}
	if !strings.Contains(err.Error(), "B") {
		t.Errorf("expected agent ID 'B' in error, got: %v", err)
	}
	if !strings.Contains(err.Error(), "pkg/tools/other.go") {
		t.Errorf("expected file path in error, got: %v", err)
	}
}

// TestConstraintEnforcer_I1_PathKeyFallback verifies that "path" key is checked
// when "file_path" is absent (edit_file tool uses "path").
func TestConstraintEnforcer_I1_PathKeyFallback(t *testing.T) {
	c := Constraints{
		AgentID:    "A",
		OwnedFiles: map[string]bool{"pkg/foo.go": true},
	}
	mw := newOwnershipMiddleware("edit_file", c)
	wrapped := mw(enforcerPassthrough())

	// Should block via "path" key
	_, err := wrapped.Execute(context.Background(), ExecutionContext{}, map[string]interface{}{
		"path": "pkg/bar.go",
	})
	if err == nil {
		t.Fatal("expected I1_VIOLATION error for unowned path via 'path' key")
	}
	if !strings.Contains(err.Error(), "I1_VIOLATION") {
		t.Errorf("expected I1_VIOLATION, got: %v", err)
	}

	// Should allow via "path" key
	result, err := wrapped.Execute(context.Background(), ExecutionContext{}, map[string]interface{}{
		"path": "pkg/foo.go",
	})
	if err != nil {
		t.Fatalf("unexpected error for owned path via 'path' key: %v", err)
	}
	if result != "ok" {
		t.Errorf("expected 'ok', got %q", result)
	}
}

// TestConstraintEnforcer_I2_FrozenPathReturnsViolation verifies that writing to a
// frozen path returns an I2_VIOLATION error.
func TestConstraintEnforcer_I2_FrozenPathReturnsViolation(t *testing.T) {
	now := time.Now()
	c := Constraints{
		AgentID:     "A",
		FrozenPaths: map[string]bool{"pkg/tools/types.go": true},
		FreezeTime:  &now,
	}
	mw := newFreezeMiddleware("edit_file", c)
	wrapped := mw(enforcerPassthrough())

	_, err := wrapped.Execute(context.Background(), ExecutionContext{}, map[string]interface{}{
		"file_path": "pkg/tools/types.go",
	})
	if err == nil {
		t.Fatal("expected I2_VIOLATION error for frozen path, got nil")
	}
	if !strings.Contains(err.Error(), "I2_VIOLATION") {
		t.Errorf("expected I2_VIOLATION in error, got: %v", err)
	}
	if !strings.Contains(err.Error(), "pkg/tools/types.go") {
		t.Errorf("expected frozen path in error, got: %v", err)
	}
}

// TestConstraintEnforcer_I2_NonFrozenPathSucceeds verifies that a non-frozen path
// passes through the freeze middleware without error.
func TestConstraintEnforcer_I2_NonFrozenPathSucceeds(t *testing.T) {
	now := time.Now()
	c := Constraints{
		AgentID:     "A",
		FrozenPaths: map[string]bool{"pkg/tools/types.go": true},
		FreezeTime:  &now,
	}
	mw := newFreezeMiddleware("write_file", c)
	wrapped := mw(enforcerPassthrough())

	result, err := wrapped.Execute(context.Background(), ExecutionContext{}, map[string]interface{}{
		"file_path": "pkg/tools/other.go",
	})
	if err != nil {
		t.Fatalf("unexpected error for non-frozen path: %v", err)
	}
	if result != "ok" {
		t.Errorf("expected 'ok', got %q", result)
	}
}

// TestConstraintEnforcer_I2_NilFreezeTimeDisablesFreeze verifies that freeze
// enforcement is disabled when FreezeTime is nil, even if FrozenPaths is set.
func TestConstraintEnforcer_I2_NilFreezeTimeDisablesFreeze(t *testing.T) {
	c := Constraints{
		AgentID:     "A",
		FrozenPaths: map[string]bool{"pkg/tools/types.go": true},
		FreezeTime:  nil, // freeze disabled
	}
	mw := newFreezeMiddleware("edit_file", c)
	wrapped := mw(enforcerPassthrough())

	result, err := wrapped.Execute(context.Background(), ExecutionContext{}, map[string]interface{}{
		"file_path": "pkg/tools/types.go",
	})
	if err != nil {
		t.Fatalf("unexpected error when FreezeTime is nil: %v", err)
	}
	if result != "ok" {
		t.Errorf("expected passthrough when FreezeTime is nil, got %q", result)
	}
}

// TestConstraintEnforcer_I6_ScoutAllowedPathSucceeds verifies that a scout agent
// can write to docs/IMPL/IMPL-*.yaml paths.
func TestConstraintEnforcer_I6_ScoutAllowedPathSucceeds(t *testing.T) {
	c := Constraints{
		AgentRole:           "scout",
		AllowedPathPrefixes: []string{"docs/IMPL/IMPL-"},
	}
	mw := newRolePathMiddleware("write_file", c)
	wrapped := mw(enforcerPassthrough())

	result, err := wrapped.Execute(context.Background(), ExecutionContext{}, map[string]interface{}{
		"file_path": "docs/IMPL/IMPL-my-feature.yaml",
	})
	if err != nil {
		t.Fatalf("unexpected error for allowed scout path: %v", err)
	}
	if result != "ok" {
		t.Errorf("expected 'ok', got %q", result)
	}
}

// TestConstraintEnforcer_I6_ScoutBlocksSourceCode verifies that a scout agent
// cannot write to source code files outside allowed prefixes.
func TestConstraintEnforcer_I6_ScoutBlocksSourceCode(t *testing.T) {
	c := Constraints{
		AgentRole:           "scout",
		AllowedPathPrefixes: []string{"docs/IMPL/IMPL-"},
	}
	mw := newRolePathMiddleware("write_file", c)
	wrapped := mw(enforcerPassthrough())

	_, err := wrapped.Execute(context.Background(), ExecutionContext{}, map[string]interface{}{
		"file_path": "src/main.go",
	})
	if err == nil {
		t.Fatal("expected I6_VIOLATION error for scout writing to src/main.go, got nil")
	}
	if !strings.Contains(err.Error(), "I6_VIOLATION") {
		t.Errorf("expected I6_VIOLATION in error, got: %v", err)
	}
	if !strings.Contains(err.Error(), "scout") {
		t.Errorf("expected role 'scout' in error, got: %v", err)
	}
	if !strings.Contains(err.Error(), "src/main.go") {
		t.Errorf("expected attempted path in error, got: %v", err)
	}
}

// TestConstraintEnforcer_I6_EmptyPrefixesPassthrough verifies that an empty
// AllowedPathPrefixes list results in passthrough (Wave agents use OwnedFiles).
func TestConstraintEnforcer_I6_EmptyPrefixesPassthrough(t *testing.T) {
	c := Constraints{
		AgentRole:           "wave",
		AllowedPathPrefixes: nil,
	}
	mw := newRolePathMiddleware("write_file", c)
	wrapped := mw(enforcerPassthrough())

	result, err := wrapped.Execute(context.Background(), ExecutionContext{}, map[string]interface{}{
		"file_path": "anywhere/anything.go",
	})
	if err != nil {
		t.Fatalf("unexpected error with empty AllowedPathPrefixes: %v", err)
	}
	if result != "ok" {
		t.Errorf("expected passthrough with empty prefixes, got %q", result)
	}
}

// TestConstraintEnforcer_Composition verifies that all three middlewares composed
// together correctly enforce all constraints in sequence.
func TestConstraintEnforcer_Composition(t *testing.T) {
	now := time.Now()
	c := Constraints{
		AgentID:             "A",
		AgentRole:           "wave",
		OwnedFiles:          map[string]bool{"pkg/tools/myfile.go": true},
		FrozenPaths:         map[string]bool{"pkg/tools/frozen.go": true},
		FreezeTime:          &now,
		AllowedPathPrefixes: []string{"pkg/tools/"},
	}

	base := enforcerPassthrough()
	// Apply in the same order as WithConstraints: role -> freeze -> ownership (innermost)
	wrapped := Apply(base,
		newRolePathMiddleware("write_file", c),
		newFreezeMiddleware("write_file", c),
		newOwnershipMiddleware("write_file", c),
	)

	// Case 1: owned, not frozen, allowed prefix -> succeeds
	result, err := wrapped.Execute(context.Background(), ExecutionContext{}, map[string]interface{}{
		"file_path": "pkg/tools/myfile.go",
	})
	if err != nil {
		t.Fatalf("expected success for valid file, got: %v", err)
	}
	if result != "ok" {
		t.Errorf("expected 'ok', got %q", result)
	}

	// Case 2: outside allowed prefix -> I6_VIOLATION (role middleware fires first)
	_, err = wrapped.Execute(context.Background(), ExecutionContext{}, map[string]interface{}{
		"file_path": "cmd/main.go",
	})
	if err == nil {
		t.Fatal("expected I6_VIOLATION, got nil")
	}
	if !strings.Contains(err.Error(), "I6_VIOLATION") {
		t.Errorf("expected I6_VIOLATION, got: %v", err)
	}

	// Case 3: frozen path with allowed prefix -> I2_VIOLATION
	_, err = wrapped.Execute(context.Background(), ExecutionContext{}, map[string]interface{}{
		"file_path": "pkg/tools/frozen.go",
	})
	if err == nil {
		t.Fatal("expected I2_VIOLATION, got nil")
	}
	if !strings.Contains(err.Error(), "I2_VIOLATION") {
		t.Errorf("expected I2_VIOLATION, got: %v", err)
	}

	// Case 4: allowed prefix, not frozen, but not owned -> I1_VIOLATION
	_, err = wrapped.Execute(context.Background(), ExecutionContext{}, map[string]interface{}{
		"file_path": "pkg/tools/unowned.go",
	})
	if err == nil {
		t.Fatal("expected I1_VIOLATION, got nil")
	}
	if !strings.Contains(err.Error(), "I1_VIOLATION") {
		t.Errorf("expected I1_VIOLATION, got: %v", err)
	}
}

// TestConstraintEnforcer_InitRegistersRealImplementations verifies that the init()
// function unconditionally sets all three package-level middleware variables to
// the real implementations (not the passthrough stubs).
func TestConstraintEnforcer_InitRegistersRealImplementations(t *testing.T) {
	// After init() runs, the package-level vars should point to real implementations.
	// We verify this by checking that they produce violations (not passthroughs).
	now := time.Now()
	c := Constraints{
		AgentID:             "A",
		AgentRole:           "scout",
		OwnedFiles:          map[string]bool{"pkg/tools/owned.go": true},
		FrozenPaths:         map[string]bool{"pkg/tools/frozen.go": true},
		FreezeTime:          &now,
		AllowedPathPrefixes: []string{"docs/IMPL/IMPL-"},
	}
	base := enforcerPassthrough()

	// I1: unowned file should produce error, not passthrough
	ownMW := ownershipMiddlewareFn("write_file", c)
	wrapped := ownMW(base)
	_, err := wrapped.Execute(context.Background(), ExecutionContext{}, map[string]interface{}{
		"file_path": "pkg/tools/notowned.go",
	})
	if err == nil || !strings.Contains(err.Error(), "I1_VIOLATION") {
		t.Errorf("ownershipMiddlewareFn should enforce I1, got err=%v", err)
	}

	// I2: frozen path should produce error, not passthrough
	freezeMW := freezeMiddlewareFn("edit_file", c)
	wrapped = freezeMW(base)
	_, err = wrapped.Execute(context.Background(), ExecutionContext{}, map[string]interface{}{
		"file_path": "pkg/tools/frozen.go",
	})
	if err == nil || !strings.Contains(err.Error(), "I2_VIOLATION") {
		t.Errorf("freezeMiddlewareFn should enforce I2, got err=%v", err)
	}

	// I6: path outside allowed prefixes should produce error, not passthrough
	roleMW := rolePathMiddlewareFn("write_file", c)
	wrapped = roleMW(base)
	_, err = wrapped.Execute(context.Background(), ExecutionContext{}, map[string]interface{}{
		"file_path": "src/main.go",
	})
	if err == nil || !strings.Contains(err.Error(), "I6_VIOLATION") {
		t.Errorf("rolePathMiddlewareFn should enforce I6, got err=%v", err)
	}
}
