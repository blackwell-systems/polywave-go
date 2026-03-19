package protocol

import (
	"os"
	"path/filepath"
	"testing"
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
	if e.Code != "E16_FILE_NOT_FOUND" {
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
