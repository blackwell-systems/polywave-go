package deps

import (
	"fmt"
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

// TestIsStdLib verifies the isStdLib helper correctly identifies standard library packages
func TestIsStdLib(t *testing.T) {
	tests := []struct {
		name       string
		importPath string
		want       bool
	}{
		{"simple stdlib", "fmt", true},
		{"nested stdlib", "encoding/json", true},
		{"github package", "github.com/foo/bar", false},
		{"golang.org extended stdlib", "golang.org/x/tools", false},
		{"domain with dot", "example.com", false},
		{"domain with path", "example.com/foo/bar", false},
		{"net stdlib", "net/http", true},
		{"os stdlib", "os", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isStdLib(tt.importPath)
			if got != tt.want {
				t.Errorf("isStdLib(%q) = %v, want %v", tt.importPath, got, tt.want)
			}
		})
	}
}

// TestGetModulePath verifies the getModulePath helper extracts module paths correctly
func TestGetModulePath(t *testing.T) {
	tests := []struct {
		name      string
		goModData string
		wantPath  string
		wantError bool
	}{
		{
			name:      "valid go.mod",
			goModData: "module example.com/foo\n\ngo 1.21\n",
			wantPath:  "example.com/foo",
			wantError: false,
		},
		{
			name:      "go.mod with comments",
			goModData: "// This is a comment\nmodule example.com/bar\n\ngo 1.21\n",
			wantPath:  "example.com/bar",
			wantError: false,
		},
		{
			name:      "go.mod with blank lines",
			goModData: "\n\nmodule example.com/baz\n\n\ngo 1.21\n",
			wantPath:  "example.com/baz",
			wantError: false,
		},
		{
			name:      "malformed go.mod no module directive",
			goModData: "go 1.21\n\nrequire (\n  example.com/foo v1.0.0\n)\n",
			wantPath:  "",
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			goModPath := filepath.Join(tmpDir, "go.mod")
			if err := os.WriteFile(goModPath, []byte(tt.goModData), 0644); err != nil {
				t.Fatalf("failed to write go.mod: %v", err)
			}

			got, err := getModulePath(tmpDir)
			if tt.wantError {
				if err == nil {
					t.Error("expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if got != tt.wantPath {
					t.Errorf("getModulePath() = %q, want %q", got, tt.wantPath)
				}
			}
		})
	}

	// Test missing go.mod
	t.Run("missing go.mod", func(t *testing.T) {
		tmpDir := t.TempDir()
		_, err := getModulePath(tmpDir)
		if err == nil {
			t.Error("expected error for missing go.mod, got nil")
		}
	})
}

// TestExtractExternalImports verifies external import extraction logic
func TestExtractExternalImports(t *testing.T) {
	tests := []struct {
		name         string
		modulePath   string
		files        map[string]string // filename -> content
		ownedFiles   map[string]string // filename -> agent
		wantImports  map[string][]string
	}{
		{
			name:       "stdlib imports only",
			modulePath: "example.com/myapp",
			files: map[string]string{
				"main.go": `package main
import (
	"fmt"
	"encoding/json"
)
`,
			},
			ownedFiles:  map[string]string{"main.go": "A"},
			wantImports: map[string][]string{},
		},
		{
			name:       "external imports",
			modulePath: "example.com/myapp",
			files: map[string]string{
				"main.go": `package main
import (
	"fmt"
	"github.com/external/lib"
	"golang.org/x/tools"
)
`,
			},
			ownedFiles: map[string]string{"main.go": "A"},
			wantImports: map[string][]string{
				"main.go": {"github.com/external/lib", "golang.org/x/tools"},
			},
		},
		{
			name:       "filters local imports",
			modulePath: "example.com/myapp",
			files: map[string]string{
				"pkg/foo/foo.go": `package foo
import (
	"fmt"
	"example.com/myapp/pkg/bar"
	"github.com/external/lib"
)
`,
			},
			ownedFiles: map[string]string{"pkg/foo/foo.go": "A"},
			wantImports: map[string][]string{
				"pkg/foo/foo.go": {"github.com/external/lib"},
			},
		},
		{
			name:       "skips non-existent files",
			modulePath: "example.com/myapp",
			files:      map[string]string{},
			ownedFiles: map[string]string{"newfile.go": "A"},
			wantImports: map[string][]string{},
		},
		{
			name:       "skips non-go files",
			modulePath: "example.com/myapp",
			files: map[string]string{
				"config.yaml": "key: value\n",
			},
			ownedFiles:  map[string]string{"config.yaml": "A"},
			wantImports: map[string][]string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()

			// Write go.mod
			goModContent := fmt.Sprintf("module %s\n\ngo 1.21\n", tt.modulePath)
			if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(goModContent), 0644); err != nil {
				t.Fatalf("failed to write go.mod: %v", err)
			}

			// Write test files
			for filename, content := range tt.files {
				path := filepath.Join(tmpDir, filename)
				dir := filepath.Dir(path)
				if err := os.MkdirAll(dir, 0755); err != nil {
					t.Fatalf("failed to create dir %s: %v", dir, err)
				}
				if err := os.WriteFile(path, []byte(content), 0644); err != nil {
					t.Fatalf("failed to write %s: %v", filename, err)
				}
			}

			got, err := extractExternalImports(tmpDir, tt.ownedFiles)
			if err != nil {
				t.Fatalf("extractExternalImports failed: %v", err)
			}

			// Compare results
			if len(got) != len(tt.wantImports) {
				t.Errorf("got %d files with imports, want %d", len(got), len(tt.wantImports))
			}

			for file, wantPkgs := range tt.wantImports {
				gotPkgs, ok := got[file]
				if !ok {
					t.Errorf("missing imports for file %s", file)
					continue
				}
				if len(gotPkgs) != len(wantPkgs) {
					t.Errorf("file %s: got %d imports, want %d\ngot: %v\nwant: %v",
						file, len(gotPkgs), len(wantPkgs), gotPkgs, wantPkgs)
					continue
				}
				// Check each expected import is present
				gotSet := make(map[string]bool)
				for _, pkg := range gotPkgs {
					gotSet[pkg] = true
				}
				for _, wantPkg := range wantPkgs {
					if !gotSet[wantPkg] {
						t.Errorf("file %s: missing import %q", file, wantPkg)
					}
				}
			}
		})
	}
}

// TestIsLocalReplace verifies the isLocalReplace longest-prefix matching logic
func TestIsLocalReplace(t *testing.T) {
	tests := []struct {
		name         string
		localReplace map[string]struct{}
		importPath   string
		want         bool
	}{
		{
			name:         "exact match",
			localReplace: map[string]struct{}{"github.com/foo/bar": {}},
			importPath:   "github.com/foo/bar",
			want:         true,
		},
		{
			name:         "prefix match",
			localReplace: map[string]struct{}{"github.com/foo": {}},
			importPath:   "github.com/foo/bar/baz",
			want:         true,
		},
		{
			name:         "no match",
			localReplace: map[string]struct{}{},
			importPath:   "github.com/foo/bar",
			want:         false,
		},
		{
			name:         "longest prefix wins",
			localReplace: map[string]struct{}{
				"github.com/foo":     {},
				"github.com/foo/bar": {},
			},
			importPath: "github.com/foo/bar/baz",
			want:       true,
		},
		{
			name: "no false prefix match",
			localReplace: map[string]struct{}{
				"github.com/foo": {},
			},
			importPath: "github.com/foobar/baz",
			want:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isLocalReplace(tt.localReplace, tt.importPath)
			if got != tt.want {
				t.Errorf("isLocalReplace(%v, %q) = %v, want %v",
					tt.localReplace, tt.importPath, got, tt.want)
			}
		})
	}
}
