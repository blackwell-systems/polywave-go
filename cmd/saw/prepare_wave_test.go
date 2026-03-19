package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/protocol"
)

func TestLoadSAWConfigRepos_ValidFile(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "saw.config.json")

	content := `{
		"repos": [
			{"name": "scout-and-wave-go", "path": "/tmp/saw-go"},
			{"name": "scout-and-wave-web", "path": "/tmp/saw-web"}
		]
	}`
	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write temp config: %v", err)
	}

	repos := loadSAWConfigRepos(configPath)
	if len(repos) != 2 {
		t.Fatalf("expected 2 repos, got %d", len(repos))
	}
	if repos[0].Name != "scout-and-wave-go" || repos[0].Path != "/tmp/saw-go" {
		t.Errorf("unexpected repos[0]: %+v", repos[0])
	}
	if repos[1].Name != "scout-and-wave-web" || repos[1].Path != "/tmp/saw-web" {
		t.Errorf("unexpected repos[1]: %+v", repos[1])
	}
}

func TestLoadSAWConfigRepos_AbsentFile(t *testing.T) {
	repos := loadSAWConfigRepos("/nonexistent/path/saw.config.json")
	if repos != nil {
		t.Errorf("expected nil for absent file, got %v", repos)
	}
}

func TestLoadSAWConfigRepos_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "saw.config.json")

	if err := os.WriteFile(configPath, []byte(`{invalid json`), 0644); err != nil {
		t.Fatalf("failed to write temp config: %v", err)
	}

	repos := loadSAWConfigRepos(configPath)
	if repos != nil {
		t.Errorf("expected nil for invalid JSON, got %v", repos)
	}
}

func TestLoadSAWConfigRepos_NoReposKey(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "saw.config.json")

	if err := os.WriteFile(configPath, []byte(`{"planner_model": "claude-3-opus"}`), 0644); err != nil {
		t.Fatalf("failed to write temp config: %v", err)
	}

	repos := loadSAWConfigRepos(configPath)
	// Should return nil slice (empty from json unmarshal with no repos key)
	if len(repos) != 0 {
		t.Errorf("expected empty repos, got %v", repos)
	}
}

func TestResolveAgentRepoRoot_WithMatchingRepo(t *testing.T) {
	fileOwnership := []protocol.FileOwnership{
		{File: "pkg/foo/foo.go", Agent: "A", Wave: 1, Repo: "scout-and-wave-go"},
		{File: "pkg/api/handler.go", Agent: "B", Wave: 1, Repo: "scout-and-wave-web"},
	}
	repos := []sawRepoEntry{
		{Name: "scout-and-wave-go", Path: "/abs/path/saw-go"},
		{Name: "scout-and-wave-web", Path: "/abs/path/saw-web"},
	}

	got := resolveAgentRepoRoot(fileOwnership, "A", "/fallback", repos)
	if got != "/abs/path/saw-go" {
		t.Errorf("expected /abs/path/saw-go, got %s", got)
	}

	got = resolveAgentRepoRoot(fileOwnership, "B", "/fallback", repos)
	if got != "/abs/path/saw-web" {
		t.Errorf("expected /abs/path/saw-web, got %s", got)
	}
}

func TestResolveAgentRepoRoot_FallbackWhenNoRepoField(t *testing.T) {
	fileOwnership := []protocol.FileOwnership{
		{File: "cmd/saw/prepare_wave.go", Agent: "B", Wave: 1},
	}
	repos := []sawRepoEntry{
		{Name: "some-repo", Path: "/abs/path/some-repo"},
	}

	got := resolveAgentRepoRoot(fileOwnership, "B", "/project/root", repos)
	if got != "/project/root" {
		t.Errorf("expected fallback /project/root, got %s", got)
	}
}

func TestResolveAgentRepoRoot_FallbackWhenRepoNotInRegistry(t *testing.T) {
	fileOwnership := []protocol.FileOwnership{
		{File: "pkg/foo/foo.go", Agent: "A", Wave: 1, Repo: "unknown-repo"},
	}
	repos := []sawRepoEntry{
		{Name: "other-repo", Path: "/abs/path/other"},
	}

	got := resolveAgentRepoRoot(fileOwnership, "A", "/project/root", repos)
	if got != "/project/root" {
		t.Errorf("expected fallback /project/root, got %s", got)
	}
}

func TestResolveAgentRepoRoot_FallbackWhenNilRepos(t *testing.T) {
	fileOwnership := []protocol.FileOwnership{
		{File: "pkg/foo/foo.go", Agent: "A", Wave: 1, Repo: "scout-and-wave-go"},
	}

	got := resolveAgentRepoRoot(fileOwnership, "A", "/project/root", nil)
	if got != "/project/root" {
		t.Errorf("expected fallback /project/root with nil repos, got %s", got)
	}
}

func TestResolveAgentRepoRoot_FallbackWhenEmptyOwnership(t *testing.T) {
	repos := []sawRepoEntry{
		{Name: "scout-and-wave-go", Path: "/abs/path/saw-go"},
	}

	got := resolveAgentRepoRoot(nil, "A", "/project/root", repos)
	if got != "/project/root" {
		t.Errorf("expected fallback /project/root for empty ownership, got %s", got)
	}
}

func TestPrepareWaveResult_JSONSerialization(t *testing.T) {
	result := PrepareWaveResult{
		Wave:      1,
		Worktrees: []protocol.WorktreeInfo{},
		AgentBriefs: []AgentBriefInfo{
			{
				Agent:       "A",
				BriefPath:   "/some/path/.saw-agent-brief.md",
				BriefLength: 100,
				JournalDir:  "/some/path/.claude/tool-journal",
				FilesOwned:  2,
				Repo:        "scout-and-wave-go",
			},
			{
				Agent:       "B",
				BriefPath:   "/other/path/.saw-agent-brief.md",
				BriefLength: 80,
				JournalDir:  "/other/path/.claude/tool-journal",
				FilesOwned:  1,
				// No Repo — primary repo agent
			},
		},
		Repos: []string{"scout-and-wave-go"},
	}

	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("failed to marshal result: %v", err)
	}

	var decoded PrepareWaveResult
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal result: %v", err)
	}

	if decoded.Wave != 1 {
		t.Errorf("expected wave 1, got %d", decoded.Wave)
	}
	if len(decoded.AgentBriefs) != 2 {
		t.Fatalf("expected 2 agent briefs, got %d", len(decoded.AgentBriefs))
	}
	if decoded.AgentBriefs[0].Repo != "scout-and-wave-go" {
		t.Errorf("expected Repo scout-and-wave-go, got %q", decoded.AgentBriefs[0].Repo)
	}
	if decoded.AgentBriefs[1].Repo != "" {
		t.Errorf("expected empty Repo for agent B, got %q", decoded.AgentBriefs[1].Repo)
	}
	if len(decoded.Repos) != 1 || decoded.Repos[0] != "scout-and-wave-go" {
		t.Errorf("unexpected Repos: %v", decoded.Repos)
	}
}

func TestAgentBriefInfo_OmitEmptyRepo(t *testing.T) {
	info := AgentBriefInfo{
		Agent:      "B",
		FilesOwned: 1,
	}
	data, err := json.Marshal(info)
	if err != nil {
		t.Fatalf("failed to marshal AgentBriefInfo: %v", err)
	}
	var m map[string]interface{}
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if _, ok := m["repo"]; ok {
		t.Errorf("expected 'repo' field to be omitted when empty, but it was present")
	}
}
