package protocol

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/blackwell-systems/polywave-go/pkg/result"
)

func TestValidateFileExistence_EmptyRepoPath(t *testing.T) {
	m := &IMPLManifest{
		FileOwnership: []FileOwnership{
			{File: "pkg/foo/missing.go", Agent: "A", Action: "modify"},
		},
	}

	errs := ValidateFileExistence(m, "")
	if len(errs) != 0 {
		t.Errorf("expected no warnings with empty repoPath, got %d: %v", len(errs), errs)
	}
}

func TestValidateFileExistence_FileExists(t *testing.T) {
	dir := t.TempDir()

	// Create the file so it actually exists
	subDir := filepath.Join(dir, "pkg", "foo")
	if err := os.MkdirAll(subDir, 0o755); err != nil {
		t.Fatal(err)
	}
	existingFile := filepath.Join(subDir, "bar.go")
	if err := os.WriteFile(existingFile, []byte("package foo\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	m := &IMPLManifest{
		FileOwnership: []FileOwnership{
			{File: "pkg/foo/bar.go", Agent: "A", Action: "modify"},
		},
	}

	errs := ValidateFileExistence(m, dir)
	if len(errs) != 0 {
		t.Errorf("expected no warnings for existing file, got %d: %v", len(errs), errs)
	}
}

func TestValidateFileExistence_FileMissing(t *testing.T) {
	dir := t.TempDir()

	m := &IMPLManifest{
		FileOwnership: []FileOwnership{
			{File: "pkg/foo/missing.go", Agent: "A", Action: "modify"},
		},
	}

	errs := ValidateFileExistence(m, dir)
	if len(errs) != 1 {
		t.Fatalf("expected 1 warning for missing file, got %d: %v", len(errs), errs)
	}

	e := errs[0]
	if e.Code != result.CodeFileMissing {
		t.Errorf("expected code E16_FILE_NOT_FOUND, got %q", e.Code)
	}
	if e.Field != "file_ownership[0]" {
		t.Errorf("expected field file_ownership[0], got %q", e.Field)
	}
	if e.Message != "file 'pkg/foo/missing.go' marked action=modify but does not exist" {
		t.Errorf("unexpected message: %q", e.Message)
	}
}

func TestValidateFileExistence_ActionNewIgnored(t *testing.T) {
	dir := t.TempDir()

	// action=new: file doesn't need to exist yet
	m := &IMPLManifest{
		FileOwnership: []FileOwnership{
			{File: "pkg/foo/newfile.go", Agent: "A", Action: "new"},
		},
	}

	errs := ValidateFileExistence(m, dir)
	if len(errs) != 0 {
		t.Errorf("expected no warnings for action=new, got %d: %v", len(errs), errs)
	}
}

func TestValidateFileExistence_CrossRepoSkipped(t *testing.T) {
	dir := t.TempDir()
	// dir basename acts as repo name; entry targets a different repo

	m := &IMPLManifest{
		FileOwnership: []FileOwnership{
			{File: "pkg/foo/missing.go", Agent: "A", Action: "modify", Repo: "other-repo"},
		},
	}

	errs := ValidateFileExistence(m, dir)
	if len(errs) != 0 {
		t.Errorf("expected no warnings for cross-repo entry, got %d: %v", len(errs), errs)
	}
}

func TestValidateFileExistenceMultiRepo_CrossRepoResolved(t *testing.T) {
	// Set up two repos
	parent := t.TempDir()
	repoA := filepath.Join(parent, "repo-a")
	repoB := filepath.Join(parent, "repo-b")

	// Create file in repo-a
	dirA := filepath.Join(repoA, "pkg", "foo")
	if err := os.MkdirAll(dirA, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dirA, "bar.go"), []byte("package foo\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Create file in repo-b
	dirB := filepath.Join(repoB, "pkg", "baz")
	if err := os.MkdirAll(dirB, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dirB, "qux.go"), []byte("package baz\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	m := &IMPLManifest{
		FileOwnership: []FileOwnership{
			{File: "pkg/foo/bar.go", Agent: "A", Action: "modify", Repo: "repo-a"},
			{File: "pkg/baz/qux.go", Agent: "B", Action: "modify", Repo: "repo-b"},
		},
	}

	configRepos := []RepoEntry{
		{Name: "repo-a", Path: repoA},
		{Name: "repo-b", Path: repoB},
	}

	errs := ValidateFileExistenceMultiRepo(m, repoA, configRepos)
	if len(errs) != 0 {
		t.Errorf("expected no errors when files exist in correct repos, got %d: %v", len(errs), errs)
	}
}

func TestValidateFileExistenceMultiRepo_AllMissingSuspectsRepoMismatch(t *testing.T) {
	dir := t.TempDir()

	m := &IMPLManifest{
		FileOwnership: []FileOwnership{
			{File: "pkg/foo/missing1.go", Agent: "A", Action: "modify"},
			{File: "pkg/bar/missing2.go", Agent: "B", Action: "modify"},
			{File: "pkg/new/file.go", Agent: "C", Action: "new"}, // should be ignored
		},
	}

	errs := ValidateFileExistenceMultiRepo(m, dir, nil)

	// Should have 2 FILE_NOT_FOUND + 1 REPO_MISMATCH_SUSPECTED = 3 errors
	if len(errs) != 3 {
		t.Fatalf("expected 3 errors, got %d: %v", len(errs), errs)
	}

	// First two should be FILE_NOT_FOUND
	for i := 0; i < 2; i++ {
		if errs[i].Code != result.CodeFileMissing {
			t.Errorf("errs[%d]: expected E16_FILE_NOT_FOUND, got %q", i, errs[i].Code)
		}
	}

	// Last should be REPO_MISMATCH_SUSPECTED
	if errs[2].Code != result.CodeRepoMismatch {
		t.Errorf("expected E16_REPO_MISMATCH_SUSPECTED, got %q", errs[2].Code)
	}
}
