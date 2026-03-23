package protocol

import "testing"

func TestResolveAgentRepo_WithMatchingRegistry(t *testing.T) {
	fo := []FileOwnership{
		{File: "pkg/foo/foo.go", Agent: "A", Wave: 1, Repo: "saw-go"},
	}
	repos := []RepoEntry{
		{Name: "saw-go", Path: "/abs/saw-go"},
	}

	got := ResolveAgentRepo(fo, "A", "/fallback", repos)
	if got != "/abs/saw-go" {
		t.Errorf("expected /abs/saw-go, got %s", got)
	}
}

func TestResolveAgentRepo_FallbackWhenRepoNotInRegistry(t *testing.T) {
	fo := []FileOwnership{
		{File: "pkg/foo/foo.go", Agent: "A", Wave: 1, Repo: "unknown-repo"},
	}
	repos := []RepoEntry{
		{Name: "other-repo", Path: "/abs/other"},
	}

	got := ResolveAgentRepo(fo, "A", "/fallback", repos)
	if got != "/fallback" {
		t.Errorf("expected /fallback, got %s", got)
	}
}

func TestResolveAgentRepo_FallbackWhenNoRepoField(t *testing.T) {
	fo := []FileOwnership{
		{File: "pkg/foo/foo.go", Agent: "A", Wave: 1},
	}
	repos := []RepoEntry{
		{Name: "saw-go", Path: "/abs/saw-go"},
	}

	got := ResolveAgentRepo(fo, "A", "/fallback", repos)
	if got != "/fallback" {
		t.Errorf("expected /fallback, got %s", got)
	}
}

func TestResolveAgentRepo_FallbackWhenNilRepos(t *testing.T) {
	fo := []FileOwnership{
		{File: "pkg/foo/foo.go", Agent: "A", Wave: 1, Repo: "saw-go"},
	}

	got := ResolveAgentRepo(fo, "A", "/fallback", nil)
	if got != "/fallback" {
		t.Errorf("expected /fallback, got %s", got)
	}
}

func TestResolveAgentRepo_FallbackWhenNilOwnership(t *testing.T) {
	repos := []RepoEntry{
		{Name: "saw-go", Path: "/abs/saw-go"},
	}

	got := ResolveAgentRepo(nil, "A", "/fallback", repos)
	if got != "/fallback" {
		t.Errorf("expected /fallback, got %s", got)
	}
}

func TestAgentRepoName_ReturnsName(t *testing.T) {
	fo := []FileOwnership{
		{File: "pkg/foo/foo.go", Agent: "A", Wave: 1, Repo: "saw-go"},
	}

	got := AgentRepoName(fo, "A")
	if got != "saw-go" {
		t.Errorf("expected saw-go, got %s", got)
	}
}

func TestAgentRepoName_EmptyWhenNoRepoField(t *testing.T) {
	fo := []FileOwnership{
		{File: "pkg/foo/foo.go", Agent: "A", Wave: 1},
	}

	got := AgentRepoName(fo, "A")
	if got != "" {
		t.Errorf("expected empty string, got %s", got)
	}
}
