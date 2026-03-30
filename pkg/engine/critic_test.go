package engine

import (
	"context"
	"testing"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/protocol"
)

// TestRunCritic_RelativePathRejected verifies that a relative IMPL path is rejected.
func TestRunCritic_RelativePathRejected(t *testing.T) {
	_, err := RunCritic(context.Background(), RunCriticOpts{
		IMPLPath: "relative/path/IMPL-foo.yaml",
	}, nil)
	if err == nil {
		t.Fatal("expected error for relative impl-path")
	}
	want := `run-critic: impl-path must be absolute (got "relative/path/IMPL-foo.yaml")`
	if err.Error() != want {
		t.Errorf("unexpected error message:\ngot:  %s\nwant: %s", err.Error(), want)
	}
}

// TestRunCritic_NonExistentPathRejected verifies that a non-existent IMPL path is rejected.
func TestRunCritic_NonExistentPathRejected(t *testing.T) {
	_, err := RunCritic(context.Background(), RunCriticOpts{
		IMPLPath: "/nonexistent/path/IMPL-foo.yaml",
	}, nil)
	if err == nil {
		t.Fatal("expected error for non-existent impl-path")
	}
}

// TestCollectRepoPaths_SingleRepo verifies that a single Repository field is collected.
func TestCollectRepoPaths_SingleRepo(t *testing.T) {
	manifest := &protocol.IMPLManifest{
		Repository: "/usr/src/myrepo",
	}
	paths := collectRepoPaths(manifest)
	if len(paths) != 1 {
		t.Fatalf("expected 1 path, got %d: %v", len(paths), paths)
	}
	if paths[0] != "/usr/src/myrepo" {
		t.Errorf("expected /usr/src/myrepo, got %s", paths[0])
	}
}

// TestCollectRepoPaths_DedupAndOrder verifies deduplication across Repository and Repositories.
func TestCollectRepoPaths_DedupAndOrder(t *testing.T) {
	manifest := &protocol.IMPLManifest{
		Repository:   "/root/repo",
		Repositories: []string{"/root/repo", "/other/repo", ""},
	}
	paths := collectRepoPaths(manifest)
	if len(paths) != 2 {
		t.Fatalf("expected 2 unique paths, got %d: %v", len(paths), paths)
	}
	if paths[0] != "/root/repo" {
		t.Errorf("expected first path /root/repo, got %s", paths[0])
	}
	if paths[1] != "/other/repo" {
		t.Errorf("expected second path /other/repo, got %s", paths[1])
	}
}

// TestCollectRepoPaths_EmptyManifest verifies that an empty manifest returns nil.
func TestCollectRepoPaths_EmptyManifest(t *testing.T) {
	paths := collectRepoPaths(&protocol.IMPLManifest{})
	if len(paths) != 0 {
		t.Errorf("expected empty paths for empty manifest, got %v", paths)
	}
}

// TestInferRepoRoot_StandardLayout verifies standard docs/IMPL layout detection.
func TestInferRepoRoot_StandardLayout(t *testing.T) {
	implPath := "/home/user/myrepo/docs/IMPL/IMPL-feature.yaml"
	got := inferRepoRoot(implPath)
	want := "/home/user/myrepo"
	if got != want {
		t.Errorf("inferRepoRoot(%q) = %q, want %q", implPath, got, want)
	}
}

// TestInferRepoRoot_NonStandardLayout verifies that a non-standard path returns empty string.
func TestInferRepoRoot_NonStandardLayout(t *testing.T) {
	implPath := "/home/user/myrepo/IMPL-feature.yaml"
	got := inferRepoRoot(implPath)
	if got != "" {
		t.Errorf("inferRepoRoot(%q) = %q, want empty string", implPath, got)
	}
}

// TestInferRepoRoot_TmpPath verifies shallow paths return empty string.
func TestInferRepoRoot_TmpPath(t *testing.T) {
	implPath := "/tmp/IMPL-feature.yaml"
	got := inferRepoRoot(implPath)
	if got != "" {
		t.Errorf("inferRepoRoot(%q) = %q, want empty string", implPath, got)
	}
}
