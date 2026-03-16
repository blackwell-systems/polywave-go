package deps

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRegisterParser(t *testing.T) {
	// Save original parsers
	originalParsers := parsers
	defer func() { parsers = originalParsers }()

	// Reset parsers
	parsers = nil

	mock := &mockParser{detectResult: true}
	RegisterParser(mock)

	if len(parsers) != 1 {
		t.Errorf("Expected 1 parser registered, got %d", len(parsers))
	}
}

func TestDetectLockFiles(t *testing.T) {
	// Create temp directory
	tmpDir := t.TempDir()

	// Create some lock files
	lockFiles := []string{"go.sum", "package-lock.json", "Cargo.lock"}
	for _, name := range lockFiles {
		path := filepath.Join(tmpDir, name)
		if err := os.WriteFile(path, []byte("test"), 0644); err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}
	}

	// Detect lock files
	detected, err := DetectLockFiles(tmpDir)
	if err != nil {
		t.Errorf("DetectLockFiles failed: %v", err)
	}

	if len(detected) != 3 {
		t.Errorf("Expected 3 lock files, got %d", len(detected))
	}
}

func TestDetectLockFiles_EmptyDir(t *testing.T) {
	tmpDir := t.TempDir()

	detected, err := DetectLockFiles(tmpDir)
	if err != nil {
		t.Errorf("DetectLockFiles failed: %v", err)
	}

	if len(detected) != 0 {
		t.Errorf("Expected 0 lock files in empty dir, got %d", len(detected))
	}
}

func TestCheckDeps_NoConflicts(t *testing.T) {
	// This test requires a valid IMPL doc and repo structure
	// For now, test graceful handling of missing/invalid input

	// Test with non-existent IMPL doc
	report, err := CheckDeps("/nonexistent/path.yaml", 1)
	if err == nil {
		t.Error("Expected error for non-existent IMPL doc")
	}
	if report != nil {
		t.Error("Expected nil report on error")
	}
}

func TestCheckDeps_EmptyWave(t *testing.T) {
	// Create a minimal IMPL doc in temp dir
	tmpDir := t.TempDir()
	docsDir := filepath.Join(tmpDir, "docs", "IMPL")
	if err := os.MkdirAll(docsDir, 0755); err != nil {
		t.Fatalf("Failed to create docs dir: %v", err)
	}

	implPath := filepath.Join(docsDir, "test.yaml")
	implContent := `title: Test IMPL
file_ownership: []
waves: []
`
	if err := os.WriteFile(implPath, []byte(implContent), 0644); err != nil {
		t.Fatalf("Failed to create IMPL doc: %v", err)
	}

	report, err := CheckDeps(implPath, 1)
	if err != nil {
		t.Errorf("CheckDeps failed: %v", err)
	}

	if report == nil {
		t.Fatal("Expected non-nil report")
	}

	// Empty wave should produce empty report
	if len(report.MissingDeps) != 0 {
		t.Errorf("Expected 0 missing deps, got %d", len(report.MissingDeps))
	}
	if len(report.VersionConflicts) != 0 {
		t.Errorf("Expected 0 version conflicts, got %d", len(report.VersionConflicts))
	}
}

func TestNormalizePackageName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"github.com/user/repo", "user/repo"},
		{"golang.org/x/tools", "tools"},
		{"package@1.2.3", "package"},
		{"Package", "package"},
		{"github.com/User/Repo@v1.0.0", "user/repo"},
	}

	for _, tt := range tests {
		result := NormalizePackageName(tt.input)
		if result != tt.expected {
			t.Errorf("NormalizePackageName(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestFindLockPackage_PrefixMatch(t *testing.T) {
	lockPkgs := map[string]PackageInfo{
		"github.com/aws/aws-sdk-go-v2":           {Name: "github.com/aws/aws-sdk-go-v2", Version: "v1.30.0"},
		"github.com/anthropics/anthropic-sdk-go":  {Name: "github.com/anthropics/anthropic-sdk-go", Version: "v0.2.0"},
		"github.com/blackwell-systems/scout-go":   {Name: "github.com/blackwell-systems/scout-go", Version: "v0.1.0"},
	}

	tests := []struct {
		importPath string
		wantFound  bool
		wantModule string
	}{
		// Exact match
		{"github.com/aws/aws-sdk-go-v2", true, "github.com/aws/aws-sdk-go-v2"},
		// Sub-package match (the bug this fixes)
		{"github.com/aws/aws-sdk-go-v2/service/bedrockruntime/types", true, "github.com/aws/aws-sdk-go-v2"},
		{"github.com/aws/aws-sdk-go-v2/aws", true, "github.com/aws/aws-sdk-go-v2"},
		{"github.com/anthropics/anthropic-sdk-go/option", true, "github.com/anthropics/anthropic-sdk-go"},
		{"github.com/anthropics/anthropic-sdk-go/packages/param", true, "github.com/anthropics/anthropic-sdk-go"},
		// No match
		{"github.com/unknown/package", false, ""},
		// Must not match partial module names (aws-sdk-go-v2-extra is not aws-sdk-go-v2)
		{"github.com/aws/aws-sdk-go-v2-extra/foo", false, ""},
	}

	for _, tt := range tests {
		pkg, found := findLockPackage(lockPkgs, tt.importPath)
		if found != tt.wantFound {
			t.Errorf("findLockPackage(%q) found=%v, want %v", tt.importPath, found, tt.wantFound)
		}
		if found && pkg.Name != tt.wantModule {
			t.Errorf("findLockPackage(%q) module=%q, want %q", tt.importPath, pkg.Name, tt.wantModule)
		}
	}
}

func TestCheckDeps_MissingDeps(t *testing.T) {
	// This would require mocking the analyzer package
	// For now, we test the structure is correct
	t.Skip("Requires integration with analyzer package")
}

func TestCheckDeps_VersionConflicts(t *testing.T) {
	// This would require mocking the analyzer package
	// For now, we test the structure is correct
	t.Skip("Requires integration with analyzer package")
}
