package protocol

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// setupTestGitRepo creates a temporary directory with a git repo and an
// initial commit. Returns the repo path and the HEAD SHA.
func setupTestGitRepo(t *testing.T) (repoDir, headSHA string) {
	t.Helper()
	dir := t.TempDir()

	mustGit := func(args ...string) string {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, string(out))
		}
		return string(out)
	}

	mustGit("init")
	mustGit("config", "user.email", "test@example.com")
	mustGit("config", "user.name", "Test")

	readme := filepath.Join(dir, "README.md")
	if err := os.WriteFile(readme, []byte("test"), 0644); err != nil {
		t.Fatalf("write README: %v", err)
	}
	mustGit("add", "README.md")
	mustGit("commit", "--no-verify", "-m", "initial commit")

	shaOut := mustGit("rev-parse", "HEAD")
	sha := shaOut[:len(shaOut)-1] // trim trailing newline
	if len(sha) > 7 {
		sha = sha[:7]
	}

	return dir, sha
}

// buildManifestForAgent returns a minimal IMPLManifest with one wave containing
// one agent with the given owned files.
func buildManifestForAgent(agentID, featureSlug string, ownedFiles []string, waveNum int) *IMPLManifest {
	fo := make([]FileOwnership, len(ownedFiles))
	for i, f := range ownedFiles {
		fo[i] = FileOwnership{File: f, Agent: agentID, Wave: waveNum}
	}
	return &IMPLManifest{
		FeatureSlug:   featureSlug,
		FileOwnership: fo,
		Waves: []Wave{
			{
				Number: waveNum,
				Agents: []Agent{{ID: agentID, Task: "test task"}},
			},
		},
	}
}

// TestValidateCompletionReportClaims_Valid verifies that a report whose commit
// exists and whose files are all within the agent's owned set is accepted.
func TestValidateCompletionReportClaims_Valid(t *testing.T) {
	repoDir, sha := setupTestGitRepo(t)

	// Create an owned file on disk so FilesCreated check passes.
	ownedFile := "pkg/foo/bar.go"
	if err := os.MkdirAll(filepath.Join(repoDir, "pkg/foo"), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repoDir, ownedFile), []byte("package foo"), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	manifest := buildManifestForAgent("B", "my-feature", []string{ownedFile}, 1)

	report := CompletionReport{
		Status:       "complete",
		Commit:       sha,
		FilesChanged: []string{ownedFile},
		FilesCreated: []string{ownedFile},
	}

	res := ValidateCompletionReportClaims(manifest, "B", report, repoDir)
	if !res.IsSuccess() && !res.IsPartial() {
		t.Errorf("expected success or partial result, got code=%s errors=%v", res.Code, res.Errors)
		return
	}
	data := res.GetData()
	if !data.Valid {
		t.Errorf("expected Valid=true, got false. Errors: %v", res.Errors)
	}
}

// TestValidateCompletionReportClaims_FakeCommit verifies that a report with a
// non-existent commit SHA is rejected with a commit-related error.
func TestValidateCompletionReportClaims_FakeCommit(t *testing.T) {
	repoDir, _ := setupTestGitRepo(t)

	manifest := buildManifestForAgent("B", "my-feature", []string{"pkg/foo/bar.go"}, 1)

	report := CompletionReport{
		Status: "complete",
		Commit: "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef",
	}

	res := ValidateCompletionReportClaims(manifest, "B", report, repoDir)
	if res.IsSuccess() {
		t.Error("expected non-success result for fake commit SHA")
	}
	found := false
	for _, e := range res.Errors {
		if strContains(e.Message, "deadbeef") || strContains(e.Message, "commit") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected error mentioning commit SHA, got: %v", res.Errors)
	}
}

// TestValidateCompletionReportClaims_FilesOutsideOwnership verifies that a
// report listing files outside the agent's owned set is rejected.
func TestValidateCompletionReportClaims_FilesOutsideOwnership(t *testing.T) {
	repoDir, sha := setupTestGitRepo(t)

	manifest := buildManifestForAgent("B", "my-feature", []string{"pkg/owned/file.go"}, 1)

	report := CompletionReport{
		Status:       "complete",
		Commit:       sha,
		FilesChanged: []string{"pkg/owned/file.go", "pkg/OTHER/secret.go"},
	}

	res := ValidateCompletionReportClaims(manifest, "B", report, repoDir)
	if res.IsSuccess() {
		t.Error("expected non-success result for files outside ownership")
	}
	found := false
	for _, e := range res.Errors {
		if strContains(e.Message, "pkg/OTHER/secret.go") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected error mentioning unowned file, got: %v", res.Errors)
	}
}

// TestValidateCompletionReportClaims_NonExistentFilesCreated verifies that a
// report listing a FilesCreated entry that doesn't exist on disk is rejected.
func TestValidateCompletionReportClaims_NonExistentFilesCreated(t *testing.T) {
	repoDir, sha := setupTestGitRepo(t)

	ownedFile := "pkg/missing/ghost.go"
	manifest := buildManifestForAgent("B", "my-feature", []string{ownedFile}, 1)

	report := CompletionReport{
		Status:       "complete",
		Commit:       sha,
		FilesChanged: []string{ownedFile},
		FilesCreated: []string{ownedFile}, // file does NOT exist on disk
	}

	res := ValidateCompletionReportClaims(manifest, "B", report, repoDir)
	if res.IsSuccess() {
		t.Error("expected non-success result for non-existent FilesCreated entry")
	}
	found := false
	for _, e := range res.Errors {
		if strContains(e.Message, "ghost.go") || strContains(e.Message, "does not exist") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected error mentioning missing file, got: %v", res.Errors)
	}
}

// TestValidateCompletionReportClaims_FrozenPathAllowed verifies that scaffold
// (frozen) paths are not treated as ownership violations.
func TestValidateCompletionReportClaims_FrozenPathAllowed(t *testing.T) {
	repoDir, sha := setupTestGitRepo(t)

	scaffoldFile := "pkg/scaffold/types.go"
	// Create the scaffold file on disk.
	if err := os.MkdirAll(filepath.Join(repoDir, "pkg/scaffold"), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repoDir, scaffoldFile), []byte("package scaffold"), 0644); err != nil {
		t.Fatalf("write scaffold: %v", err)
	}

	manifest := buildManifestForAgent("B", "my-feature", []string{"pkg/owned/file.go"}, 1)
	manifest.Scaffolds = []ScaffoldFile{{FilePath: scaffoldFile}}

	report := CompletionReport{
		Status:       "complete",
		Commit:       sha,
		FilesChanged: []string{scaffoldFile}, // scaffold, not in owned files
		FilesCreated: []string{scaffoldFile},
	}

	res := ValidateCompletionReportClaims(manifest, "B", report, repoDir)
	if !res.IsSuccess() && !res.IsPartial() {
		t.Errorf("expected success or partial result for scaffold path, got code=%s errors=%v", res.Code, res.Errors)
		return
	}
	data := res.GetData()
	if !data.Valid {
		t.Errorf("expected Valid=true for scaffold path, got false. Errors: %v", res.Errors)
	}
}

// strContains is a test helper checking whether s contains substr.
func strContains(s, substr string) bool {
	if len(substr) == 0 {
		return true
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
