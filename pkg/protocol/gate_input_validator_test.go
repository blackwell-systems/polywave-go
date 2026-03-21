package protocol

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// initGateTestRepo creates a minimal git repo in dir with an initial empty commit.
func initGateTestRepo(t *testing.T, dir string) {
	t.Helper()
	cmds := [][]string{
		{"git", "-C", dir, "init"},
		{"git", "-C", dir, "config", "user.email", "test@test.com"},
		{"git", "-C", dir, "config", "user.name", "Test"},
		{"git", "-C", dir, "commit", "--allow-empty", "-m", "init"},
	}
	for _, args := range cmds {
		out, err := exec.Command(args[0], args[1:]...).CombinedOutput()
		if err != nil {
			t.Fatalf("git setup failed (%v): %s", args, out)
		}
	}
}

// createGateBranchWithFile creates a new branch from HEAD, writes a file, and commits it.
// Returns to the default branch when done.
func createGateBranchWithFile(t *testing.T, repoDir, branch, filename, content string) {
	t.Helper()

	out, err := exec.Command("git", "-C", repoDir, "checkout", "-b", branch).CombinedOutput()
	if err != nil {
		t.Fatalf("git checkout -b %s failed: %s", branch, out)
	}

	// Write the file (create parent dirs if needed)
	fullPath := filepath.Join(repoDir, filename)
	if mkErr := os.MkdirAll(filepath.Dir(fullPath), 0755); mkErr != nil {
		t.Fatalf("failed to create dirs for %s: %v", filename, mkErr)
	}
	if writeErr := os.WriteFile(fullPath, []byte(content), 0644); writeErr != nil {
		t.Fatalf("failed to write file %s: %v", filename, writeErr)
	}

	for _, args := range [][]string{
		{"git", "-C", repoDir, "add", filename},
		{"git", "-C", repoDir, "commit", "-m", "add " + filename},
	} {
		out2, err2 := exec.Command(args[0], args[1:]...).CombinedOutput()
		if err2 != nil {
			t.Fatalf("git command failed (%v): %s", args, out2)
		}
	}

	// Switch back to the default branch
	defaultBranch := gateGetDefaultBranchName(t, repoDir)
	out3, err3 := exec.Command("git", "-C", repoDir, "checkout", defaultBranch).CombinedOutput()
	if err3 != nil {
		t.Fatalf("git checkout %s failed: %s", defaultBranch, out3)
	}
}

// gateGetDefaultBranchName returns the name of the current branch.
func gateGetDefaultBranchName(t *testing.T, repoDir string) string {
	t.Helper()
	out, err := exec.Command("git", "-C", repoDir, "rev-parse", "--abbrev-ref", "HEAD").CombinedOutput()
	if err != nil {
		t.Fatalf("failed to get current branch: %s", out)
	}
	branch := string(out)
	for len(branch) > 0 && (branch[len(branch)-1] == '\n' || branch[len(branch)-1] == '\r') {
		branch = branch[:len(branch)-1]
	}
	return branch
}

// TestValidateGateInputs_MatchingReportedAndActual verifies that when reported
// files exactly match the files changed in the agent's branch, Valid=true.
func TestValidateGateInputs_MatchingReportedAndActual(t *testing.T) {
	dir := t.TempDir()
	initGateTestRepo(t, dir)
	defaultBranch := gateGetDefaultBranchName(t, dir)

	slug := "test-feature"
	agentBranch := BranchName(slug, 1, "A")
	createGateBranchWithFile(t, dir, agentBranch, "pkg/foo.go", "package foo\n")

	manifest := &IMPLManifest{
		FeatureSlug: slug,
		Waves: []Wave{
			{
				Number:     1,
				BaseCommit: defaultBranch,
				Agents:     []Agent{{ID: "A", Files: []string{"pkg/foo.go"}}},
			},
		},
		CompletionReports: map[string]CompletionReport{
			"A": {
				Status:       "complete",
				FilesChanged: []string{"pkg/foo.go"},
			},
		},
	}

	result, err := ValidateGateInputs(manifest, 1, dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Valid {
		t.Errorf("expected Valid=true; MissingFromReport=%v ExtraInReport=%v",
			result.MissingFromReport, result.ExtraInReport)
	}
}

// TestValidateGateInputs_ExtraFilesInReport verifies that when the completion
// report lists a file that doesn't appear in the git diff, it is flagged as
// ExtraInReport and Valid=false.
func TestValidateGateInputs_ExtraFilesInReport(t *testing.T) {
	dir := t.TempDir()
	initGateTestRepo(t, dir)
	defaultBranch := gateGetDefaultBranchName(t, dir)

	slug := "test-feature"
	agentBranch := BranchName(slug, 1, "A")
	// Only pkg/foo.go is actually changed
	createGateBranchWithFile(t, dir, agentBranch, "pkg/foo.go", "package foo\n")

	manifest := &IMPLManifest{
		FeatureSlug: slug,
		Waves: []Wave{
			{
				Number:     1,
				BaseCommit: defaultBranch,
				Agents:     []Agent{{ID: "A", Files: []string{"pkg/foo.go"}}},
			},
		},
		CompletionReports: map[string]CompletionReport{
			"A": {
				Status: "complete",
				// Reports an extra file that was never actually changed
				FilesChanged: []string{"pkg/foo.go", "pkg/bar.go"},
			},
		},
	}

	result, err := ValidateGateInputs(manifest, 1, dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Valid {
		t.Errorf("expected Valid=false when report contains extra file not in git diff")
	}
	if len(result.ExtraInReport) == 0 {
		t.Errorf("expected ExtraInReport to contain pkg/bar.go, got empty")
	} else if result.ExtraInReport[0] != "pkg/bar.go" {
		t.Errorf("expected ExtraInReport[0]=pkg/bar.go, got %s", result.ExtraInReport[0])
	}
}

// TestValidateGateInputs_MissingFilesFromReport verifies that when a file
// appears in git diff but is not listed in the completion report, it is flagged
// as MissingFromReport and Valid=false.
func TestValidateGateInputs_MissingFilesFromReport(t *testing.T) {
	dir := t.TempDir()
	initGateTestRepo(t, dir)
	defaultBranch := gateGetDefaultBranchName(t, dir)

	slug := "test-feature"
	agentBranch := BranchName(slug, 1, "A")
	// pkg/foo.go is actually changed but agent only reports pkg/bar.go
	createGateBranchWithFile(t, dir, agentBranch, "pkg/foo.go", "package foo\n")

	manifest := &IMPLManifest{
		FeatureSlug: slug,
		Waves: []Wave{
			{
				Number:     1,
				BaseCommit: defaultBranch,
				Agents:     []Agent{{ID: "A", Files: []string{"pkg/foo.go"}}},
			},
		},
		CompletionReports: map[string]CompletionReport{
			"A": {
				Status:       "complete",
				FilesChanged: []string{"pkg/bar.go"}, // wrong file reported
			},
		},
	}

	result, err := ValidateGateInputs(manifest, 1, dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Valid {
		t.Errorf("expected Valid=false when git diff has file missing from report")
	}
	if len(result.MissingFromReport) == 0 {
		t.Errorf("expected MissingFromReport to contain pkg/foo.go, got empty")
	} else if result.MissingFromReport[0] != "pkg/foo.go" {
		t.Errorf("expected MissingFromReport[0]=pkg/foo.go, got %s", result.MissingFromReport[0])
	}
}

// TestValidateGateInputs_WaveNotFound verifies an error is returned for missing wave.
func TestValidateGateInputs_WaveNotFound(t *testing.T) {
	dir := t.TempDir()
	initGateTestRepo(t, dir)

	manifest := &IMPLManifest{
		FeatureSlug: "test-feature",
		Waves:       []Wave{{Number: 1, Agents: []Agent{{ID: "A"}}}},
	}

	_, err := ValidateGateInputs(manifest, 99, dir)
	if err == nil {
		t.Fatal("expected error for missing wave, got nil")
	}
}
