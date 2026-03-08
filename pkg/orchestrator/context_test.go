package orchestrator

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// setupGitRepo initializes a minimal git repo in dir so git commit operations succeed.
func setupGitRepo(t *testing.T, dir string) {
	t.Helper()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("init")
	run("config", "user.email", "test@example.com")
	run("config", "user.name", "Test User")
	// Seed an initial commit so HEAD is valid.
	readmePath := filepath.Join(dir, "README.md")
	if err := os.WriteFile(readmePath, []byte("# test\n"), 0644); err != nil {
		t.Fatalf("write README: %v", err)
	}
	run("add", "README.md")
	run("commit", "-m", "init")
}

// TestUpdateContextMDCreatesFile verifies that UpdateContextMD creates
// docs/CONTEXT.md with the canonical schema and the entry when no file exists.
func TestUpdateContextMDCreatesFile(t *testing.T) {
	dir := t.TempDir()
	setupGitRepo(t, dir)

	entry := ContextMDEntry{
		Slug:    "my-feature",
		ImplDoc: "docs/IMPL/IMPL-my-feature.md",
		Waves:   2,
		Agents:  5,
		Date:    "2026-03-08",
	}

	if err := UpdateContextMD(dir, entry); err != nil {
		t.Fatalf("UpdateContextMD returned error: %v", err)
	}

	contextPath := filepath.Join(dir, "docs", "CONTEXT.md")
	data, err := os.ReadFile(contextPath)
	if err != nil {
		t.Fatalf("CONTEXT.md not created: %v", err)
	}

	content := string(data)
	checks := []struct {
		label string
		want  string
	}{
		{"schema header", "# docs/CONTEXT.md"},
		{"features_completed key", "features_completed:"},
		{"entry slug", "slug: my-feature"},
		{"entry impl_doc", "impl_doc: docs/IMPL/IMPL-my-feature.md"},
		{"entry waves", "waves: 2"},
		{"entry agents", "agents: 5"},
		{"entry date", "date: 2026-03-08"},
	}
	for _, c := range checks {
		if !strings.Contains(content, c.want) {
			t.Errorf("[%s] expected %q in CONTEXT.md, got:\n%s", c.label, c.want, content)
		}
	}
}

// TestUpdateContextMDAppendsToExisting verifies that calling UpdateContextMD
// twice results in both entries appearing in the file.
func TestUpdateContextMDAppendsToExisting(t *testing.T) {
	dir := t.TempDir()
	setupGitRepo(t, dir)

	entry1 := ContextMDEntry{
		Slug:    "feature-one",
		ImplDoc: "docs/IMPL/IMPL-feature-one.md",
		Waves:   1,
		Agents:  3,
		Date:    "2026-03-01",
	}
	entry2 := ContextMDEntry{
		Slug:    "feature-two",
		ImplDoc: "docs/IMPL/IMPL-feature-two.md",
		Waves:   2,
		Agents:  6,
		Date:    "2026-03-08",
	}

	if err := UpdateContextMD(dir, entry1); err != nil {
		t.Fatalf("first UpdateContextMD: %v", err)
	}
	if err := UpdateContextMD(dir, entry2); err != nil {
		t.Fatalf("second UpdateContextMD: %v", err)
	}

	contextPath := filepath.Join(dir, "docs", "CONTEXT.md")
	data, err := os.ReadFile(contextPath)
	if err != nil {
		t.Fatalf("CONTEXT.md not found: %v", err)
	}

	content := string(data)
	for _, want := range []string{"slug: feature-one", "slug: feature-two"} {
		if !strings.Contains(content, want) {
			t.Errorf("expected %q in CONTEXT.md after two calls, got:\n%s", want, content)
		}
	}
}
