package git

import (
	"os/exec"
	"testing"
)

// TestRunInvalidDir verifies that Run returns an error when given a non-existent directory.
func TestRunInvalidDir(t *testing.T) {
	_, err := Run("/nonexistent/path/that/does/not/exist", "status")
	if err == nil {
		t.Fatal("expected error for non-existent directory, got nil")
	}
}

// TestRevParseHEAD initializes a temporary git repo, makes an initial commit,
// and verifies that RevParse("HEAD") returns a valid 40-character SHA.
func TestRevParseHEAD(t *testing.T) {
	dir := t.TempDir()

	// Initialize repo
	if out, err := exec.Command("git", "-C", dir, "init").CombinedOutput(); err != nil {
		t.Fatalf("git init failed: %v: %s", err, out)
	}

	// Configure identity for the temp repo (needed for commit)
	if out, err := exec.Command("git", "-C", dir, "config", "user.email", "test@test.com").CombinedOutput(); err != nil {
		t.Fatalf("git config email failed: %v: %s", err, out)
	}
	if out, err := exec.Command("git", "-C", dir, "config", "user.name", "Test").CombinedOutput(); err != nil {
		t.Fatalf("git config name failed: %v: %s", err, out)
	}

	// Create a file and make the initial commit
	if out, err := exec.Command("git", "-C", dir, "commit", "--allow-empty", "-m", "initial").CombinedOutput(); err != nil {
		t.Fatalf("git commit failed: %v: %s", err, out)
	}

	sha, err := RevParse(dir, "HEAD")
	if err != nil {
		t.Fatalf("RevParse returned error: %v", err)
	}

	if len(sha) != 40 {
		t.Fatalf("expected 40-char SHA, got %q (len=%d)", sha, len(sha))
	}
}
