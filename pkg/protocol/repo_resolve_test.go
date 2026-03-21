package protocol

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveTargetRepos_SingleRepo(t *testing.T) {
	m := &IMPLManifest{
		FileOwnership: []FileOwnership{
			{File: "pkg/foo/bar.go", Agent: "A", Action: "modify"},
			{File: "pkg/foo/baz.go", Agent: "B", Action: "new"},
		},
	}

	fallback := "/home/user/code/my-project"
	result, err := ResolveTargetRepos(m, fallback, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 1 {
		t.Fatalf("expected 1 repo, got %d", len(result))
	}
	if result["my-project"] != fallback {
		t.Errorf("expected fallback path, got %v", result)
	}
}

func TestResolveTargetRepos_MultiRepo(t *testing.T) {
	// Create sibling directories for resolution
	parent := t.TempDir()
	repoA := filepath.Join(parent, "repo-a")
	repoB := filepath.Join(parent, "repo-b")
	if err := os.MkdirAll(repoA, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(repoB, 0o755); err != nil {
		t.Fatal(err)
	}

	m := &IMPLManifest{
		FileOwnership: []FileOwnership{
			{File: "pkg/foo/bar.go", Agent: "A", Action: "modify", Repo: "repo-a"},
			{File: "pkg/baz/qux.go", Agent: "B", Action: "new", Repo: "repo-b"},
		},
	}

	configRepos := []RepoEntry{
		{Name: "repo-a", Path: repoA},
		{Name: "repo-b", Path: repoB},
	}

	result, err := ResolveTargetRepos(m, repoA, configRepos)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 2 {
		t.Fatalf("expected 2 repos, got %d", len(result))
	}
	if result["repo-a"] != repoA {
		t.Errorf("repo-a: expected %s, got %s", repoA, result["repo-a"])
	}
	if result["repo-b"] != repoB {
		t.Errorf("repo-b: expected %s, got %s", repoB, result["repo-b"])
	}
}

func TestResolveTargetRepos_FallbackToManifestRepository(t *testing.T) {
	m := &IMPLManifest{
		Repository: "/home/user/code/special-repo",
		FileOwnership: []FileOwnership{
			{File: "pkg/foo/bar.go", Agent: "A", Action: "modify"},
		},
	}

	result, err := ResolveTargetRepos(m, "/some/other/path", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 1 {
		t.Fatalf("expected 1 repo, got %d", len(result))
	}
	if result["special-repo"] != "/home/user/code/special-repo" {
		t.Errorf("expected manifest repository path, got %v", result)
	}
}

func TestResolveTargetRepos_UnknownRepoSiblingResolution(t *testing.T) {
	parent := t.TempDir()
	fallback := filepath.Join(parent, "main-repo")
	sibling := filepath.Join(parent, "sibling-repo")
	if err := os.MkdirAll(fallback, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(sibling, 0o755); err != nil {
		t.Fatal(err)
	}

	m := &IMPLManifest{
		FileOwnership: []FileOwnership{
			{File: "pkg/foo/bar.go", Agent: "A", Action: "modify", Repo: "sibling-repo"},
		},
	}

	// No configRepos - should resolve via sibling directory
	result, err := ResolveTargetRepos(m, fallback, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result["sibling-repo"] != sibling {
		t.Errorf("expected sibling path %s, got %s", sibling, result["sibling-repo"])
	}
}

func TestResolveTargetRepos_UnknownRepoError(t *testing.T) {
	m := &IMPLManifest{
		FileOwnership: []FileOwnership{
			{File: "pkg/foo/bar.go", Agent: "A", Action: "modify", Repo: "nonexistent-repo"},
		},
	}

	_, err := ResolveTargetRepos(m, "/tmp/some-repo", nil)
	if err == nil {
		t.Fatal("expected error for unknown repo, got nil")
	}
	if !strings.Contains(err.Error(), "nonexistent-repo") {
		t.Errorf("error should mention repo name, got: %v", err)
	}
}

func TestValidateRepoMatch_Match(t *testing.T) {
	m := &IMPLManifest{
		FileOwnership: []FileOwnership{
			{File: "pkg/foo/bar.go", Agent: "A", Action: "modify"},
		},
	}

	err := ValidateRepoMatch(m, "/home/user/code/my-project", nil)
	if err != nil {
		t.Errorf("expected nil for matching repo, got: %v", err)
	}
}

func TestValidateRepoMatch_Mismatch(t *testing.T) {
	parent := t.TempDir()
	targetRepo := filepath.Join(parent, "target-repo")
	worktreeRepo := filepath.Join(parent, "wrong-repo")
	if err := os.MkdirAll(targetRepo, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(worktreeRepo, 0o755); err != nil {
		t.Fatal(err)
	}

	m := &IMPLManifest{
		FileOwnership: []FileOwnership{
			{File: "pkg/foo/bar.go", Agent: "A", Action: "modify", Repo: "target-repo"},
		},
	}

	configRepos := []RepoEntry{
		{Name: "target-repo", Path: targetRepo},
	}

	err := ValidateRepoMatch(m, worktreeRepo, configRepos)
	if err == nil {
		t.Fatal("expected error for mismatched repo, got nil")
	}
	if !strings.Contains(err.Error(), "target-repo") {
		t.Errorf("error should mention target repo name, got: %v", err)
	}
	if !strings.Contains(err.Error(), "aborting") {
		t.Errorf("error should contain 'aborting', got: %v", err)
	}
}

func TestValidateRepoMatch_SingleRepoNoField(t *testing.T) {
	m := &IMPLManifest{
		FileOwnership: []FileOwnership{
			{File: "pkg/foo/bar.go", Agent: "A", Action: "modify"},
			{File: "pkg/baz/qux.go", Agent: "B", Action: "new"},
		},
	}

	// Even though fallback resolves to a different basename, single-repo
	// with no repo: fields should pass
	err := ValidateRepoMatch(m, "/any/path/whatever", nil)
	if err != nil {
		t.Errorf("expected nil for single-repo no-field case, got: %v", err)
	}
}
