package protocol

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveWorktreePath_canonical(t *testing.T) {
	tmp := t.TempDir()
	branch := "wave1-agent-A"

	// Create the canonical .claude path
	canonicalPath := filepath.Join(tmp, ".claude", "worktrees", branch)
	if err := os.MkdirAll(canonicalPath, 0o755); err != nil {
		t.Fatal(err)
	}

	got := ResolveWorktreePath(tmp, branch)
	if got != canonicalPath {
		t.Errorf("ResolveWorktreePath = %q, want %q", got, canonicalPath)
	}
}

func TestResolveWorktreePath_claire_fallback(t *testing.T) {
	tmp := t.TempDir()
	branch := "wave2-agent-B"

	// Only create the .claire path (no .claude)
	clairePath := filepath.Join(tmp, ".claire", "worktrees", branch)
	if err := os.MkdirAll(clairePath, 0o755); err != nil {
		t.Fatal(err)
	}

	got := ResolveWorktreePath(tmp, branch)
	if got != clairePath {
		t.Errorf("ResolveWorktreePath = %q, want %q (should find .claire fallback)", got, clairePath)
	}
}

func TestResolveWorktreePath_neither_exists(t *testing.T) {
	tmp := t.TempDir()
	branch := "wave1-agent-C"

	got := ResolveWorktreePath(tmp, branch)
	want := filepath.Join(tmp, ".claude", "worktrees", branch)
	if got != want {
		t.Errorf("ResolveWorktreePath = %q, want canonical fallback %q", got, want)
	}
}

func TestResolveWorktreePath_prefers_claude_over_claire(t *testing.T) {
	tmp := t.TempDir()
	branch := "wave1-agent-D"

	// Create both paths — .claude should win
	for _, base := range []string{".claude", ".claire"} {
		if err := os.MkdirAll(filepath.Join(tmp, base, "worktrees", branch), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	got := ResolveWorktreePath(tmp, branch)
	want := filepath.Join(tmp, ".claude", "worktrees", branch)
	if got != want {
		t.Errorf("ResolveWorktreePath = %q, want %q (.claude should take precedence)", got, want)
	}
}

func TestIsWorktreePath(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{"/repo/.claude/worktrees/wave1-agent-A", true},
		{"/repo/.claire/worktrees/wave1-agent-A", true},
		{"/repo/.claude/worktrees/wave2-agent-B2", true},
		{"/repo/.claude/worktrees/polywave/my-slug/wave1-agent-A", true},
		{"/repo/.claire/worktrees/polywave/my-slug/wave2-agent-B2", true},
		{"/repo/src/main.go", false},
		{"/repo/.claudewatch/data", false},
		{"", false},
	}
	for _, tt := range tests {
		if got := IsWorktreePath(tt.path); got != tt.want {
			t.Errorf("IsWorktreePath(%q) = %v, want %v", tt.path, got, tt.want)
		}
	}
}

func TestResolveWorktreePath_slugScoped(t *testing.T) {
	tmp := t.TempDir()
	branch := "polywave/my-slug/wave1-agent-A"

	// Create the slug-scoped path
	scopedPath := filepath.Join(tmp, ".claude", "worktrees", "polywave", "my-slug", "wave1-agent-A")
	if err := os.MkdirAll(scopedPath, 0o755); err != nil {
		t.Fatal(err)
	}

	got := ResolveWorktreePath(tmp, branch)
	if got != scopedPath {
		t.Errorf("ResolveWorktreePath = %q, want %q", got, scopedPath)
	}
}

func TestResolveWorktreePath_slugScoped_legacyFallback(t *testing.T) {
	tmp := t.TempDir()
	branch := "polywave/my-slug/wave1-agent-A"

	// Only create the legacy flat path (simulates pre-upgrade worktree)
	legacyPath := filepath.Join(tmp, ".claude", "worktrees", "wave1-agent-A")
	if err := os.MkdirAll(legacyPath, 0o755); err != nil {
		t.Fatal(err)
	}

	got := ResolveWorktreePath(tmp, branch)
	if got != legacyPath {
		t.Errorf("ResolveWorktreePath = %q, want legacy fallback %q", got, legacyPath)
	}
}

func TestResolveWorktreePathWithSlug(t *testing.T) {
	tmp := t.TempDir()

	// Create the slug-scoped path
	scopedPath := filepath.Join(tmp, ".claude", "worktrees", "polywave", "my-slug", "wave1-agent-A")
	if err := os.MkdirAll(scopedPath, 0o755); err != nil {
		t.Fatal(err)
	}

	got := ResolveWorktreePathWithSlug(tmp, "my-slug", 1, "A")
	if got != scopedPath {
		t.Errorf("ResolveWorktreePathWithSlug = %q, want %q", got, scopedPath)
	}
}

func TestResolveWorktreePathWithSlug_legacyFallback(t *testing.T) {
	tmp := t.TempDir()

	// Only create the legacy path
	legacyPath := filepath.Join(tmp, ".claude", "worktrees", "wave2-agent-B")
	if err := os.MkdirAll(legacyPath, 0o755); err != nil {
		t.Fatal(err)
	}

	got := ResolveWorktreePathWithSlug(tmp, "my-slug", 2, "B")
	if got != legacyPath {
		t.Errorf("ResolveWorktreePathWithSlug = %q, want legacy fallback %q", got, legacyPath)
	}
}

func TestResolveWorktreePathWithSlug_canonicalFallback(t *testing.T) {
	tmp := t.TempDir()

	// Neither path exists — should return slug-scoped canonical path
	got := ResolveWorktreePathWithSlug(tmp, "my-slug", 1, "C")
	want := filepath.Join(tmp, ".claude", "worktrees", "polywave", "my-slug", "wave1-agent-C")
	if got != want {
		t.Errorf("ResolveWorktreePathWithSlug = %q, want canonical %q", got, want)
	}
}
