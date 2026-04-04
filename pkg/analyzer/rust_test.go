package analyzer

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestParseRustFiles_Simple(t *testing.T) {
	// Skip if rust-parser binary not available
	if _, err := exec.LookPath("rust-parser"); err != nil {
		t.Skip("rust-parser binary not found in PATH, skipping test")
	}

	repoRoot := filepath.Join("testdata", "rust")
	files := []string{
		filepath.Join(repoRoot, "simple.rs"),
	}

	result, err := parseRustFiles(repoRoot, files)
	if err != nil {
		t.Fatalf("parseRustFiles failed: %v", err)
	}

	// simple.rs should have no local imports
	simplePath := files[0]
	if deps, ok := result[simplePath]; !ok {
		t.Errorf("expected result to contain %s", simplePath)
	} else if len(deps) != 0 {
		t.Errorf("expected no dependencies for simple.rs, got %v", deps)
	}
}

func TestParseRustFiles_WithImports(t *testing.T) {
	// Skip if rust-parser binary not available
	if _, err := exec.LookPath("rust-parser"); err != nil {
		t.Skip("rust-parser binary not found in PATH, skipping test")
	}

	repoRoot := filepath.Join("testdata", "rust")
	files := []string{
		filepath.Join(repoRoot, "simple.rs"),
		filepath.Join(repoRoot, "with_imports.rs"),
	}

	result, err := parseRustFiles(repoRoot, files)
	if err != nil {
		t.Fatalf("parseRustFiles failed: %v", err)
	}

	// with_imports.rs should import simple.rs (via crate::simple)
	withImportsPath := files[1]
	if deps, ok := result[withImportsPath]; !ok {
		t.Errorf("expected result to contain %s", withImportsPath)
	} else {
		// Check that it depends on simple.rs
		simplePath := files[0]
		found := false
		for _, dep := range deps {
			if dep == simplePath {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected with_imports.rs to depend on simple.rs, got dependencies: %v", deps)
		}
	}
}

func TestParseRustFiles_StdlibFiltering(t *testing.T) {
	tests := []struct {
		name       string
		importPath string
		isStdlib   bool
	}{
		{"std::collections", "std::collections::HashMap", true},
		{"std::vec", "std::vec::Vec", true},
		{"core::fmt", "core::fmt::Display", true},
		{"alloc::string", "alloc::string::String", true},
		{"crate local", "crate::simple", false},
		{"super local", "super::other", false},
		{"self local", "self::nested", false},
		{"external crate", "serde::Serialize", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isRustStdlib(tt.importPath)
			if result != tt.isStdlib {
				t.Errorf("isRustStdlib(%q) = %v, want %v", tt.importPath, result, tt.isStdlib)
			}
		})
	}
}

func TestParseRustFiles_LocalImportDetection(t *testing.T) {
	tests := []struct {
		name       string
		importPath string
		isLocal    bool
	}{
		{"crate import", "crate::simple", true},
		{"super import", "super::other", true},
		{"self import", "self::nested", true},
		{"external crate", "serde::Serialize", false},
		{"std", "std::vec::Vec", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isLocalRustImport(tt.importPath)
			if result != tt.isLocal {
				t.Errorf("isLocalRustImport(%q) = %v, want %v", tt.importPath, result, tt.isLocal)
			}
		})
	}
}

func TestParseRustFiles_BinaryMissing(t *testing.T) {
	// This test verifies the behavior when rust-parser is not found
	// We can't easily simulate this in CI, so we just document the expected behavior

	_, err := exec.LookPath("rust-parser")
	if err != nil {
		// Binary not found - parseRustFiles should return error
		repoRoot := filepath.Join("testdata", "rust")
		files := []string{
			filepath.Join(repoRoot, "simple.rs"),
		}

		_, err := parseRustFiles(repoRoot, files)
		if err == nil {
			t.Error("expected error when rust-parser binary not found")
		}
	} else {
		t.Skip("rust-parser binary found, skipping binary-missing test")
	}
}

// TestParseRustFiles_BinaryMissingViaPath explicitly sets PATH to simulate
// missing rust-parser binary and asserts that parseRustFiles returns an error.
func TestParseRustFiles_BinaryMissingViaPath(t *testing.T) {
	// Use a temp dir with no executables as PATH.
	// rust-parser will not be found; other binaries (sh, etc.) are not needed here.
	originalPath := os.Getenv("PATH")
	defer os.Setenv("PATH", originalPath)

	emptyDir := t.TempDir()
	os.Setenv("PATH", emptyDir)

	repoRoot := t.TempDir()
	files := []string{filepath.Join(repoRoot, "main.rs")}

	_, err := parseRustFiles(repoRoot, files)
	if err == nil {
		t.Fatal("expected error when rust-parser binary is not in PATH, got nil")
	}

	if !contains(err.Error(), "rust-parser binary not found in PATH") {
		t.Errorf("expected error about rust-parser not in PATH, got: %v", err)
	}
}

func TestResolveRustImport(t *testing.T) {
	repoRoot := "/tmp/testproject"

	tests := []struct {
		name        string
		currentFile string
		importPath  string
		allFiles    []string
		wantPath    string
		wantErr     bool
	}{
		{
			name:        "crate import",
			currentFile: "/tmp/testproject/src/main.rs",
			importPath:  "crate::utils",
			allFiles:    []string{"/tmp/testproject/src/utils.rs"},
			wantPath:    "/tmp/testproject/src/utils.rs",
			wantErr:     false,
		},
		{
			name:        "crate import module",
			currentFile: "/tmp/testproject/src/main.rs",
			importPath:  "crate::utils",
			allFiles:    []string{"/tmp/testproject/src/utils/mod.rs"},
			wantPath:    "/tmp/testproject/src/utils/mod.rs",
			wantErr:     false,
		},
		{
			name:        "super import",
			currentFile: "/tmp/testproject/src/foo/bar.rs",
			importPath:  "super::utils",
			allFiles:    []string{"/tmp/testproject/src/utils.rs"},
			wantPath:    "/tmp/testproject/src/utils.rs",
			wantErr:     false,
		},
		{
			name:        "self import",
			currentFile: "/tmp/testproject/src/foo/main.rs",
			importPath:  "self::helper",
			allFiles:    []string{"/tmp/testproject/src/foo/helper.rs"},
			wantPath:    "/tmp/testproject/src/foo/helper.rs",
			wantErr:     false,
		},
		{
			name:        "unresolvable import",
			currentFile: "/tmp/testproject/src/main.rs",
			importPath:  "crate::missing",
			allFiles:    []string{"/tmp/testproject/src/utils.rs"},
			wantPath:    "",
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotPath, err := resolveRustImport(repoRoot, tt.currentFile, tt.importPath, tt.allFiles)
			if (err != nil) != tt.wantErr {
				t.Errorf("resolveRustImport() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if gotPath != tt.wantPath {
				t.Errorf("resolveRustImport() = %v, want %v", gotPath, tt.wantPath)
			}
		})
	}
}
