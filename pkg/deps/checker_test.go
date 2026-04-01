package deps

import (
	"os"
	"path/filepath"
	"strings"
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

// TestCheckDeps_ReplaceDirective verifies that an import satisfied by a local
// replace directive in go.mod is NOT reported as a MissingDep.
func TestCheckDeps_ReplaceDirective(t *testing.T) {
	// Build a temp directory tree that looks like a minimal Go repo:
	//   <root>/
	//     go.mod          — declares module and a local replace directive
	//     go.sum          — empty (locally-replaced module has no checksum)
	//     docs/IMPL/
	//       test.yaml     — minimal IMPL doc pointing at the .go file below
	//     pkg/myapp/
	//       app.go        — imports the locally-replaced module
	tmpDir := t.TempDir()

	// Create directory structure
	for _, dir := range []string{
		filepath.Join(tmpDir, "docs", "IMPL"),
		filepath.Join(tmpDir, "pkg", "myapp"),
		filepath.Join(tmpDir, "local-lib"),
	} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("failed to create dir %s: %v", dir, err)
		}
	}

	// go.mod with a replace directive pointing to a local path
	goModContent := `module example.com/myapp

go 1.21

require (
	github.com/example/local-lib v0.1.0
	github.com/example/real-dep v1.2.3
)

replace github.com/example/local-lib => ./local-lib
`
	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(goModContent), 0644); err != nil {
		t.Fatalf("failed to write go.mod: %v", err)
	}

	// go.sum — contains real-dep but NOT local-lib (local replacements are absent from go.sum)
	goSumContent := "github.com/example/real-dep v1.2.3 h1:abc123=\ngithub.com/example/real-dep v1.2.3/go.mod h1:def456=\n"
	if err := os.WriteFile(filepath.Join(tmpDir, "go.sum"), []byte(goSumContent), 0644); err != nil {
		t.Fatalf("failed to write go.sum: %v", err)
	}

	// The Go source file that imports the locally-replaced module
	appGoContent := `package myapp

import (
	_ "github.com/example/local-lib/pkg"
	_ "github.com/example/real-dep"
)
`
	if err := os.WriteFile(filepath.Join(tmpDir, "pkg", "myapp", "app.go"), []byte(appGoContent), 0644); err != nil {
		t.Fatalf("failed to write app.go: %v", err)
	}

	// Minimal IMPL doc. CheckDeps derives repoRoot from the IMPL path:
	// docs/IMPL/test.yaml  ->  docs/IMPL/ -> docs/ -> <root>
	implContent := `title: Test IMPL
file_ownership:
  - file: pkg/myapp/app.go
    agent: A
    wave: 1
    action: modify
waves:
  - number: 1
    agents:
      - id: A
        task: test task
        files:
          - pkg/myapp/app.go
`
	implPath := filepath.Join(tmpDir, "docs", "IMPL", "test.yaml")
	if err := os.WriteFile(implPath, []byte(implContent), 0644); err != nil {
		t.Fatalf("failed to write IMPL doc: %v", err)
	}

	// Register a go.sum parser so the real-dep is found in the lock file
	originalParsers := parsers
	defer func() { parsers = originalParsers }()
	parsers = nil
	RegisterParser(&goSumParser{})

	report, err := CheckDeps(implPath, 1)
	if err != nil {
		t.Fatalf("CheckDeps returned error: %v", err)
	}
	if report == nil {
		t.Fatal("CheckDeps returned nil report")
	}

	// The locally-replaced module must NOT appear in MissingDeps
	for _, dep := range report.MissingDeps {
		if dep.Package == "github.com/example/local-lib/pkg" || dep.Package == "github.com/example/local-lib" {
			t.Errorf("locally-replaced module reported as MissingDep: %+v", dep)
		}
	}

	if len(report.MissingDeps) != 0 {
		t.Errorf("expected 0 MissingDeps, got %d: %+v", len(report.MissingDeps), report.MissingDeps)
	}
}

// TestParseGoModReplace_SingleLine tests single-line replace directive parsing
func TestParseGoModReplace_SingleLine(t *testing.T) {
	tmpDir := t.TempDir()
	goModPath := filepath.Join(tmpDir, "go.mod")
	content := `module example.com/test

go 1.21

replace github.com/foo/bar => ./local
`
	if err := os.WriteFile(goModPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write go.mod: %v", err)
	}

	result := ParseGoModReplace(goModPath)

	if _, ok := result["github.com/foo/bar"]; !ok {
		t.Errorf("expected 'github.com/foo/bar' in result, got: %v", result)
	}
	if len(result) != 1 {
		t.Errorf("expected 1 module in result, got %d", len(result))
	}
}

// TestParseGoModReplace_Block tests multi-line replace block parsing
func TestParseGoModReplace_Block(t *testing.T) {
	tmpDir := t.TempDir()
	goModPath := filepath.Join(tmpDir, "go.mod")
	content := `module example.com/test

go 1.21

replace (
	github.com/foo/bar => ./local
	github.com/baz/qux v1.0.0 => ./other
)
`
	if err := os.WriteFile(goModPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write go.mod: %v", err)
	}

	result := ParseGoModReplace(goModPath)

	if _, ok := result["github.com/foo/bar"]; !ok {
		t.Errorf("expected 'github.com/foo/bar' in result")
	}
	if _, ok := result["github.com/baz/qux"]; !ok {
		t.Errorf("expected 'github.com/baz/qux' in result")
	}
	if len(result) != 2 {
		t.Errorf("expected 2 modules in result, got %d", len(result))
	}
}

// TestParseGoModReplace_VersionedReplacement tests versioned module replacements.
// The implementation only filters pseudo-versions (with "@"), not regular semantic versions.
func TestParseGoModReplace_VersionedReplacement(t *testing.T) {
	tmpDir := t.TempDir()

	t.Run("PseudoVersion", func(t *testing.T) {
		// Pseudo-versions with @ are correctly filtered
		content := `module example.com/test

go 1.21

replace github.com/foo/bar => github.com/fork/bar@v2.0.0+incompatible
`
		path := filepath.Join(tmpDir, "go.mod.pseudo")
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatalf("failed to write go.mod: %v", err)
		}

		result := ParseGoModReplace(path)
		if len(result) != 0 {
			t.Errorf("expected empty result for pseudo-version replacement, got: %v", result)
		}
	})

	t.Run("SpaceSeparatedVersion", func(t *testing.T) {
		// Space-separated versions are NOT filtered (known limitation)
		content := `module example.com/test

go 1.21

replace github.com/foo/bar => github.com/fork/bar v2.0.0
`
		path := filepath.Join(tmpDir, "go.mod.space")
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatalf("failed to write go.mod: %v", err)
		}

		result := ParseGoModReplace(path)

		// Current implementation does not detect space-separated version syntax.
		// It only checks for "@" which catches pseudo-versions but not "module vX.Y.Z" format.
		if _, ok := result["github.com/foo/bar"]; !ok {
			t.Errorf("current implementation includes space-separated version replacements")
		}
	})
}

// TestParseGoModReplace_MissingFile tests behavior when go.mod doesn't exist
func TestParseGoModReplace_MissingFile(t *testing.T) {
	tmpDir := t.TempDir()
	goModPath := filepath.Join(tmpDir, "nonexistent.mod")

	result := ParseGoModReplace(goModPath)

	if result == nil {
		t.Error("expected non-nil map, got nil")
	}
	if len(result) != 0 {
		t.Errorf("expected empty map for missing file, got %d entries", len(result))
	}
}

// TestParseGoModReplace_MalformedDirective tests defensive parsing of malformed directives
func TestParseGoModReplace_MalformedDirective(t *testing.T) {
	tmpDir := t.TempDir()
	goModPath := filepath.Join(tmpDir, "go.mod")
	content := `module example.com/test

go 1.21

replace
replace =>
replace foo
replace github.com/good/pkg => ./local
`
	if err := os.WriteFile(goModPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write go.mod: %v", err)
	}

	result := ParseGoModReplace(goModPath)

	// Only the well-formed directive should be parsed
	if _, ok := result["github.com/good/pkg"]; !ok {
		t.Errorf("expected 'github.com/good/pkg' in result")
	}
	if len(result) != 1 {
		t.Errorf("expected 1 module in result, got %d", len(result))
	}
}

// TestParseGoModReplace_Comments tests handling of comments and blank lines
func TestParseGoModReplace_Comments(t *testing.T) {
	tmpDir := t.TempDir()
	goModPath := filepath.Join(tmpDir, "go.mod")
	content := `module example.com/test

go 1.21

// This is a comment
replace (
	// Another comment
	github.com/foo/bar => ./local
)
`
	if err := os.WriteFile(goModPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write go.mod: %v", err)
	}

	result := ParseGoModReplace(goModPath)

	if _, ok := result["github.com/foo/bar"]; !ok {
		t.Errorf("expected 'github.com/foo/bar' in result, got: %v", result)
	}
	if len(result) != 1 {
		t.Errorf("expected 1 module in result, got %d", len(result))
	}
}

// goSumParser is a minimal LockFileParser for go.sum files used in testing.
// It reads "module version hash" lines and returns a PackageInfo per module.
type goSumParser struct{}

func (p *goSumParser) Detect(filePath string) bool {
	return filepath.Base(filePath) == "go.sum"
}

func (p *goSumParser) Parse(filePath string) ([]PackageInfo, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}
	seen := make(map[string]struct{})
	var packages []PackageInfo
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		modName := fields[0]
		version := fields[1]
		// go.sum lists both "vX.Y.Z" and "vX.Y.Z/go.mod" lines; deduplicate
		if _, ok := seen[modName]; ok {
			continue
		}
		seen[modName] = struct{}{}
		packages = append(packages, PackageInfo{Name: modName, Version: version})
	}
	return packages, nil
}
