package engine

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeTestManifest(t *testing.T, dir, testCommand string) string {
	t.Helper()
	implDir := filepath.Join(dir, "docs", "IMPL")
	if err := os.MkdirAll(implDir, 0o755); err != nil {
		t.Fatal(err)
	}
	implPath := filepath.Join(implDir, "IMPL-test.yaml")
	content := "feature: test-feature\nfeature_slug: test-feature\ntest_command: " + testCommand + "\nwaves:\n  - number: 1\n    agents:\n      - id: A\n        task: test task\n"
	if err := os.WriteFile(implPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return implPath
}

func TestRunTestCommand_EmptyTestCommand(t *testing.T) {
	dir := t.TempDir()
	implPath := writeTestManifest(t, dir, `""`)

	res := RunTestCommand(context.Background(), implPath, dir, nil)
	if !res.IsFatal() {
		t.Fatal("expected fatal result for empty test_command")
	}
	if len(res.Errors) == 0 {
		t.Fatal("expected at least one error in fatal result")
	}
	if !strings.Contains(res.Errors[0].Message, "no test_command") {
		t.Errorf("expected 'no test_command' error message, got: %v", res.Errors[0].Message)
	}
}

func TestRunTestCommand_CallbackInvoked(t *testing.T) {
	dir := t.TempDir()
	implPath := writeTestManifest(t, dir, "echo hello && echo world")

	var lines []string
	res := RunTestCommand(context.Background(), implPath, dir, func(line string) {
		lines = append(lines, line)
	})
	if res.IsFatal() {
		t.Fatalf("unexpected fatal result: %v", res.Errors)
	}
	if len(lines) < 2 {
		t.Fatalf("expected at least 2 lines, got %d: %v", len(lines), lines)
	}
	if lines[0] != "hello" {
		t.Errorf("expected first line 'hello', got %q", lines[0])
	}
	if lines[1] != "world" {
		t.Errorf("expected second line 'world', got %q", lines[1])
	}
	// Verify result data
	data := res.GetData()
	if !strings.Contains(data.Command, "echo") {
		t.Errorf("expected command to contain 'echo', got %q", data.Command)
	}
}

func TestRunTestCommand_NilCallback(t *testing.T) {
	dir := t.TempDir()
	implPath := writeTestManifest(t, dir, "echo ok")

	// Should not panic with nil callback.
	res := RunTestCommand(context.Background(), implPath, dir, nil)
	if res.IsFatal() {
		t.Fatalf("unexpected fatal result: %v", res.Errors)
	}
}

func TestRunTestCommand_FailingCommand(t *testing.T) {
	dir := t.TempDir()
	implPath := writeTestManifest(t, dir, "echo failing && exit 1")

	var lines []string
	res := RunTestCommand(context.Background(), implPath, dir, func(line string) {
		lines = append(lines, line)
	})
	if !res.IsFatal() {
		t.Fatal("expected fatal result for failing command")
	}
	if len(res.Errors) == 0 {
		t.Fatal("expected at least one error in fatal result")
	}
	if !strings.Contains(res.Errors[0].Message, "test command failed") {
		t.Errorf("expected 'test command failed' error message, got: %v", res.Errors[0].Message)
	}
	// Output should still be captured in the error message.
	if len(lines) == 0 {
		t.Error("expected at least some output lines from failing command")
	}
}
