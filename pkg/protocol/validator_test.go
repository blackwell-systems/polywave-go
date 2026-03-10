package protocol

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeTempFile writes content to a temporary file and returns the path.
// The file is automatically cleaned up when the test ends.
func writeTempFile(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "test-impl.md")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("writeTempFile: %v", err)
	}
	return path
}

func TestValidateIMPLDoc_E16A_MissingRequiredBlocks(t *testing.T) {
	content := "# IMPL: Test\n\n" +
		"```yaml type=impl-completion-report\n" +
		"status: complete\n" +
		"worktree: w\n" +
		"branch: b\n" +
		"commit: \"abc\"\n" +
		"files_changed: []\n" +
		"interface_deviations: none\n" +
		"verification: ok\n" +
		"```\n"
	path := writeTempFile(t, content)
	errs, err := ValidateIMPLDoc(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	e16aCount := 0
	for _, e := range errs {
		if strings.HasPrefix(e.Message, "missing required block:") {
			e16aCount++
		}
	}
	if e16aCount != 3 {
		t.Fatalf("expected 3 E16A errors, got %d: %v", e16aCount, errs)
	}
}

func TestValidateIMPLDoc_E16A_AllRequiredBlocksPresent(t *testing.T) {
	content := "# IMPL: Test\n\n" +
		"```yaml type=impl-file-ownership\n" +
		"| File | Agent | Wave | Depends On |\n" +
		"|------|-------|------|------------|\n" +
		"| foo.go | A | 1 | — |\n" +
		"```\n\n" +
		"```yaml type=impl-dep-graph\n" +
		"Wave 1 (parallel):\n" +
		"    [A] foo.go\n" +
		"        ✓ root\n" +
		"```\n\n" +
		"```yaml type=impl-wave-structure\n" +
		"Wave 1: [A]\n" +
		"```\n"
	path := writeTempFile(t, content)
	errs, err := ValidateIMPLDoc(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, e := range errs {
		if strings.HasPrefix(e.Message, "missing required block:") {
			t.Errorf("unexpected E16A error: %s", e.Message)
		}
	}
}

func TestValidateIMPLDoc_E16A_NoTypedBlocks(t *testing.T) {
	content := "# IMPL: Test\n\nJust prose.\n"
	path := writeTempFile(t, content)
	errs, err := ValidateIMPLDoc(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if errs != nil {
		t.Fatalf("expected nil errors for doc with no typed blocks, got: %v", errs)
	}
}

func TestValidateIMPLDoc_E16C_WarnOnPlainFencedDepGraph(t *testing.T) {
	content := "# IMPL: Test\n\n" +
		"```yaml type=impl-file-ownership\n" +
		"| File | Agent | Wave | Depends On |\n" +
		"|------|-------|------|------------|\n" +
		"| foo.go | A | 1 | — |\n" +
		"```\n\n" +
		"```yaml type=impl-dep-graph\n" +
		"Wave 1 (parallel):\n" +
		"    [A] foo.go\n" +
		"        ✓ root\n" +
		"```\n\n" +
		"```yaml type=impl-wave-structure\n" +
		"Wave 1: [A]\n" +
		"```\n\n" +
		"```\n" +
		"Wave 1 (parallel):\n" +
		"    [A] foo.go\n" +
		"        ✓ root\n" +
		"```\n"
	path := writeTempFile(t, content)
	errs, err := ValidateIMPLDoc(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	warnCount := 0
	for _, e := range errs {
		if e.BlockType == "warning" {
			warnCount++
		}
	}
	if warnCount != 1 {
		t.Fatalf("expected 1 E16C warning, got %d: %v", warnCount, errs)
	}
}

func TestValidateIMPLDoc_E16C_NoWarnOnTypedDepGraph(t *testing.T) {
	content := "# IMPL: Test\n\n" +
		"```yaml type=impl-file-ownership\n" +
		"| File | Agent | Wave | Depends On |\n" +
		"|------|-------|------|------------|\n" +
		"| foo.go | A | 1 | — |\n" +
		"```\n\n" +
		"```yaml type=impl-dep-graph\n" +
		"Wave 1 (parallel):\n" +
		"    [A] foo.go\n" +
		"        ✓ root\n" +
		"```\n\n" +
		"```yaml type=impl-wave-structure\n" +
		"Wave 1: [A]\n" +
		"```\n"
	path := writeTempFile(t, content)
	errs, err := ValidateIMPLDoc(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, e := range errs {
		if e.BlockType == "warning" {
			t.Errorf("unexpected E16C warning on typed block: %s", e.Message)
		}
	}
}
