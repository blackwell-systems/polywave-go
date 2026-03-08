package engine

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// TestRunScoutMissingFeature verifies that RunScout returns an error when Feature is empty.
func TestRunScoutMissingFeature(t *testing.T) {
	err := RunScout(context.Background(), RunScoutOpts{
		Feature:     "",
		RepoPath:    "/tmp/repo",
		IMPLOutPath: "/tmp/repo/IMPL.md",
	}, func(string) {})
	if err == nil {
		t.Fatal("expected error when Feature is empty, got nil")
	}
}

// TestStartWaveEmptyIMPL verifies that StartWave returns an error when the IMPL path does not exist.
func TestStartWaveEmptyIMPL(t *testing.T) {
	err := StartWave(context.Background(), RunWaveOpts{
		IMPLPath: "/nonexistent/path/IMPL.md",
		RepoPath: "/tmp/repo",
		Slug:     "test",
	}, func(Event) {})
	if err == nil {
		t.Fatal("expected error when IMPL path does not exist, got nil")
	}
}

// TestReadContextMDMissing verifies that readContextMD returns "" when no CONTEXT.md exists.
func TestReadContextMDMissing(t *testing.T) {
	dir := t.TempDir()
	result := readContextMD(dir)
	if result != "" {
		t.Errorf("expected empty string for missing CONTEXT.md, got %q", result)
	}
}

// TestParseIMPLDocDelegate writes a minimal IMPL doc and verifies ParseIMPLDoc returns non-nil.
func TestParseIMPLDocDelegate(t *testing.T) {
	dir := t.TempDir()
	implPath := filepath.Join(dir, "IMPL-test.md")

	content := `# IMPL: Test Feature

## Feature Name
Test Feature

## Status
| Agent | Status |
|-------|--------|
| A     | [ ]    |

## Wave 1
| Agent | Files Owned | Description |
|-------|-------------|-------------|
| A     | pkg/foo.go  | implement foo |
`
	if err := os.WriteFile(implPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test IMPL doc: %v", err)
	}

	doc, err := ParseIMPLDoc(implPath)
	if err != nil {
		t.Fatalf("ParseIMPLDoc returned error: %v", err)
	}
	if doc == nil {
		t.Fatal("ParseIMPLDoc returned nil doc")
	}
}
