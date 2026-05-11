package cli

import (
	"testing"
	"time"

	"github.com/blackwell-systems/polywave-go/pkg/tools"
)

// TestCheckConstraints_I1_OwnedFiles verifies that writing a path not in OwnedFiles
// produces an I1 violation.
func TestCheckConstraints_I1_OwnedFiles(t *testing.T) {
	constraints := &tools.Constraints{
		OwnedFiles: map[string]bool{
			"pkg/allowed/file.go": true,
		},
		AgentID: "A",
	}
	events := []ToolUseEvent{
		{ToolName: "Write", FilePath: "pkg/other/file.go"},
	}

	violations := CheckConstraints(events, constraints)
	if len(violations) != 1 {
		t.Fatalf("expected 1 violation, got %d", len(violations))
	}
	if violations[0].Invariant != "I1" {
		t.Errorf("expected I1 violation, got %s", violations[0].Invariant)
	}
	if violations[0].FilePath != "pkg/other/file.go" {
		t.Errorf("expected FilePath pkg/other/file.go, got %s", violations[0].FilePath)
	}
}

// TestCheckConstraints_I1_AllowedPath verifies that writing an owned path produces
// no violation.
func TestCheckConstraints_I1_AllowedPath(t *testing.T) {
	constraints := &tools.Constraints{
		OwnedFiles: map[string]bool{
			"pkg/allowed/file.go": true,
		},
		AgentID: "A",
	}
	events := []ToolUseEvent{
		{ToolName: "Write", FilePath: "pkg/allowed/file.go"},
	}

	violations := CheckConstraints(events, constraints)
	if len(violations) != 0 {
		t.Errorf("expected no violations, got %d: %v", len(violations), violations)
	}
}

// TestCheckConstraints_I2_FrozenPath verifies that writing a frozen path when
// FreezeTime is set produces an I2 violation.
func TestCheckConstraints_I2_FrozenPath(t *testing.T) {
	ft := time.Now()
	constraints := &tools.Constraints{
		FrozenPaths: map[string]bool{
			"pkg/frozen/interface.go": true,
		},
		FreezeTime: &ft,
		AgentID:    "A",
	}
	events := []ToolUseEvent{
		{ToolName: "Edit", FilePath: "pkg/frozen/interface.go"},
	}

	violations := CheckConstraints(events, constraints)
	if len(violations) != 1 {
		t.Fatalf("expected 1 violation, got %d", len(violations))
	}
	if violations[0].Invariant != "I2" {
		t.Errorf("expected I2 violation, got %s", violations[0].Invariant)
	}
}

// TestCheckConstraints_I2_NoFreezeTime verifies that frozen paths with nil FreezeTime
// produce no violation.
func TestCheckConstraints_I2_NoFreezeTime(t *testing.T) {
	constraints := &tools.Constraints{
		FrozenPaths: map[string]bool{
			"pkg/frozen/interface.go": true,
		},
		FreezeTime: nil,
		AgentID:    "A",
	}
	events := []ToolUseEvent{
		{ToolName: "Write", FilePath: "pkg/frozen/interface.go"},
	}

	violations := CheckConstraints(events, constraints)
	if len(violations) != 0 {
		t.Errorf("expected no violations, got %d: %v", len(violations), violations)
	}
}

// TestCheckConstraints_I6_RolePath verifies that writing a path that doesn't match
// any AllowedPathPrefixes produces an I6 violation.
func TestCheckConstraints_I6_RolePath(t *testing.T) {
	constraints := &tools.Constraints{
		AllowedPathPrefixes: []string{"docs/IMPL/IMPL-"},
		AgentRole:           "scout",
		AgentID:             "scout",
	}
	events := []ToolUseEvent{
		{ToolName: "Write", FilePath: "pkg/engine/runner.go"},
	}

	violations := CheckConstraints(events, constraints)
	if len(violations) != 1 {
		t.Fatalf("expected 1 violation, got %d", len(violations))
	}
	if violations[0].Invariant != "I6" {
		t.Errorf("expected I6 violation, got %s", violations[0].Invariant)
	}
}

// TestCheckConstraints_I6_Allowed verifies that writing a path matching an
// AllowedPathPrefixes entry produces no violation.
func TestCheckConstraints_I6_Allowed(t *testing.T) {
	constraints := &tools.Constraints{
		AllowedPathPrefixes: []string{"docs/IMPL/IMPL-"},
		AgentRole:           "scout",
		AgentID:             "scout",
	}
	events := []ToolUseEvent{
		{ToolName: "Write", FilePath: "docs/IMPL/IMPL-my-feature.yaml"},
	}

	violations := CheckConstraints(events, constraints)
	if len(violations) != 0 {
		t.Errorf("expected no violations, got %d: %v", len(violations), violations)
	}
}

// TestCheckConstraints_SkipNonWriteTools verifies that Bash events with empty
// FilePath are skipped and produce no violations.
func TestCheckConstraints_SkipNonWriteTools(t *testing.T) {
	constraints := &tools.Constraints{
		OwnedFiles: map[string]bool{
			"pkg/allowed/file.go": true,
		},
		AgentID: "A",
	}
	events := []ToolUseEvent{
		{ToolName: "Bash", FilePath: ""},
		{ToolName: "Read", FilePath: ""},
		{ToolName: "Glob", FilePath: ""},
		{ToolName: "Grep", FilePath: ""},
	}

	violations := CheckConstraints(events, constraints)
	if len(violations) != 0 {
		t.Errorf("expected no violations for non-write tools, got %d: %v", len(violations), violations)
	}
}

// TestCheckConstraints_NilConstraints verifies that CheckConstraints returns nil
// when constraints is nil.
func TestCheckConstraints_NilConstraints(t *testing.T) {
	events := []ToolUseEvent{
		{ToolName: "Write", FilePath: "any/file.go"},
	}

	violations := CheckConstraints(events, nil)
	if violations != nil {
		t.Errorf("expected nil, got %v", violations)
	}
}
