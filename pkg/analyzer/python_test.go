package analyzer

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestParsePythonFiles_Simple(t *testing.T) {
	// Skip if python3 not available
	if _, err := exec.LookPath("python3"); err != nil {
		t.Skip("python3 not found, skipping test")
	}

	// Create a temporary directory with test files
	tmpDir := t.TempDir()

	// Create python-parser.py mock
	parserScript := filepath.Join(tmpDir, "python-parser.py")
	parserContent := `#!/usr/bin/env python3
import sys
import json
file = sys.argv[1]
# Mock: simple.py has no imports
if "simple.py" in file:
    print(json.dumps({"file": file, "imports": []}))
else:
    print(json.dumps({"file": file, "imports": ["json", "os"]}))
`
	if err := os.WriteFile(parserScript, []byte(parserContent), 0755); err != nil {
		t.Fatalf("failed to create parser script: %v", err)
	}

	// Create test files
	simplePy := filepath.Join(tmpDir, "simple.py")
	if err := os.WriteFile(simplePy, []byte("# No imports\n"), 0644); err != nil {
		t.Fatalf("failed to create simple.py: %v", err)
	}

	files := []string{simplePy}
	result, err := parsePythonFiles(tmpDir, files)
	if err != nil {
		t.Fatalf("parsePythonFiles failed: %v", err)
	}

	// simple.py should have no dependencies
	if len(result[simplePy]) != 0 {
		t.Errorf("expected 0 dependencies for simple.py, got %d: %v", len(result[simplePy]), result[simplePy])
	}
}

func TestParsePythonFiles_AbsoluteImports(t *testing.T) {
	// Skip if python3 not available
	if _, err := exec.LookPath("python3"); err != nil {
		t.Skip("python3 not found, skipping test")
	}

	tmpDir := t.TempDir()

	// Create python-parser.py mock
	parserScript := filepath.Join(tmpDir, "python-parser.py")
	parserContent := `#!/usr/bin/env python3
import sys
import json
file = sys.argv[1]
if "imports.py" in file:
    print(json.dumps({"file": file, "imports": ["internal.utils", "json"]}))
elif "internal/utils.py" in file:
    print(json.dumps({"file": file, "imports": []}))
else:
    print(json.dumps({"file": file, "imports": []}))
`
	if err := os.WriteFile(parserScript, []byte(parserContent), 0755); err != nil {
		t.Fatalf("failed to create parser script: %v", err)
	}

	// Create test files
	importsPy := filepath.Join(tmpDir, "imports.py")
	if err := os.WriteFile(importsPy, []byte("import internal.utils\n"), 0644); err != nil {
		t.Fatalf("failed to create imports.py: %v", err)
	}

	// Create internal/utils.py
	internalDir := filepath.Join(tmpDir, "internal")
	if err := os.MkdirAll(internalDir, 0755); err != nil {
		t.Fatalf("failed to create internal dir: %v", err)
	}
	utilsPy := filepath.Join(internalDir, "utils.py")
	if err := os.WriteFile(utilsPy, []byte("# Utils\n"), 0644); err != nil {
		t.Fatalf("failed to create utils.py: %v", err)
	}

	files := []string{importsPy, utilsPy}
	result, err := parsePythonFiles(tmpDir, files)
	if err != nil {
		t.Fatalf("parsePythonFiles failed: %v", err)
	}

	// imports.py should depend on internal/utils.py
	deps := result[importsPy]
	if len(deps) != 1 {
		t.Errorf("expected 1 dependency for imports.py, got %d: %v", len(deps), deps)
	} else if deps[0] != utilsPy {
		t.Errorf("expected dependency %s, got %s", utilsPy, deps[0])
	}

	// utils.py should have no dependencies
	if len(result[utilsPy]) != 0 {
		t.Errorf("expected 0 dependencies for utils.py, got %d: %v", len(result[utilsPy]), result[utilsPy])
	}
}

func TestParsePythonFiles_RelativeImports(t *testing.T) {
	// Skip if python3 not available
	if _, err := exec.LookPath("python3"); err != nil {
		t.Skip("python3 not found, skipping test")
	}

	tmpDir := t.TempDir()

	// Create python-parser.py mock
	parserScript := filepath.Join(tmpDir, "python-parser.py")
	parserContent := `#!/usr/bin/env python3
import sys
import json
file = sys.argv[1]
if "relative.py" in file:
    print(json.dumps({"file": file, "imports": [".simple"]}))
elif "simple.py" in file:
    print(json.dumps({"file": file, "imports": []}))
else:
    print(json.dumps({"file": file, "imports": []}))
`
	if err := os.WriteFile(parserScript, []byte(parserContent), 0755); err != nil {
		t.Fatalf("failed to create parser script: %v", err)
	}

	// Create test files in pkg/foo/
	pkgDir := filepath.Join(tmpDir, "pkg", "foo")
	if err := os.MkdirAll(pkgDir, 0755); err != nil {
		t.Fatalf("failed to create pkg/foo dir: %v", err)
	}

	relativePy := filepath.Join(pkgDir, "relative.py")
	if err := os.WriteFile(relativePy, []byte("from . import simple\n"), 0644); err != nil {
		t.Fatalf("failed to create relative.py: %v", err)
	}

	simplePy := filepath.Join(pkgDir, "simple.py")
	if err := os.WriteFile(simplePy, []byte("# Simple\n"), 0644); err != nil {
		t.Fatalf("failed to create simple.py: %v", err)
	}

	files := []string{relativePy, simplePy}
	result, err := parsePythonFiles(tmpDir, files)
	if err != nil {
		t.Fatalf("parsePythonFiles failed: %v", err)
	}

	// relative.py should depend on simple.py
	deps := result[relativePy]
	if len(deps) != 1 {
		t.Errorf("expected 1 dependency for relative.py, got %d: %v", len(deps), deps)
	} else if deps[0] != simplePy {
		t.Errorf("expected dependency %s, got %s", simplePy, deps[0])
	}

	// simple.py should have no dependencies
	if len(result[simplePy]) != 0 {
		t.Errorf("expected 0 dependencies for simple.py, got %d: %v", len(result[simplePy]), result[simplePy])
	}
}

func TestParsePythonFiles_BinaryMissing(t *testing.T) {
	// This test verifies that the function returns an error if python3 is not found
	// We can't actually test this without manipulating PATH, so we'll just document
	// the expected behavior

	tmpDir := t.TempDir()
	files := []string{filepath.Join(tmpDir, "test.py")}

	// If python3 is available, skip this test
	if _, err := exec.LookPath("python3"); err == nil {
		t.Skip("python3 is available, cannot test missing binary scenario")
	}

	_, err := parsePythonFiles(tmpDir, files)
	if err == nil {
		t.Error("expected error when python3 is not found, got nil")
	}
}

func TestParsePythonFiles_ParserScriptMissing(t *testing.T) {
	// Skip if python3 not available
	if _, err := exec.LookPath("python3"); err != nil {
		t.Skip("python3 not found, skipping test")
	}

	tmpDir := t.TempDir()

	// Don't create python-parser.py
	files := []string{filepath.Join(tmpDir, "test.py")}

	_, err := parsePythonFiles(tmpDir, files)
	if err == nil {
		t.Error("expected error when python-parser.py is missing, got nil")
	}
}

func TestIsPythonStdlib(t *testing.T) {
	tests := []struct {
		imp      string
		expected bool
	}{
		{"json", true},
		{"os", true},
		{"sys", true},
		{"ast", true},
		{"os.path", true},
		{"json.decoder", true},
		{"internal.utils", false},
		{"mypackage", false},
		{"mypackage.module", false},
		{"typing", true},
		{"collections", true},
		{"asyncio", true},
	}

	for _, tt := range tests {
		t.Run(tt.imp, func(t *testing.T) {
			result := isPythonStdlib(tt.imp)
			if result != tt.expected {
				t.Errorf("isPythonStdlib(%q) = %v, expected %v", tt.imp, result, tt.expected)
			}
		})
	}
}

func TestResolvePythonRelativeImport(t *testing.T) {
	tests := []struct {
		name     string
		file     string
		imp      string
		allFiles []string
		expected string
	}{
		{
			name:     "same directory import",
			file:     "pkg/foo/bar.py",
			imp:      ".utils",
			allFiles: []string{"pkg/foo/bar.py", "pkg/foo/utils.py"},
			expected: "pkg/foo/utils.py",
		},
		{
			name:     "parent directory import",
			file:     "pkg/foo/bar.py",
			imp:      "..types",
			allFiles: []string{"pkg/foo/bar.py", "pkg/types.py"},
			expected: "pkg/types.py",
		},
		{
			name:     "submodule import",
			file:     "pkg/foo/bar.py",
			imp:      ".utils.helpers",
			allFiles: []string{"pkg/foo/bar.py", "pkg/foo/utils/helpers.py"},
			expected: "pkg/foo/utils/helpers.py",
		},
		{
			name:     "not found",
			file:     "pkg/foo/bar.py",
			imp:      ".missing",
			allFiles: []string{"pkg/foo/bar.py"},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := resolvePythonRelativeImport(tt.file, tt.imp, "", tt.allFiles)
			if err != nil {
				t.Fatalf("resolvePythonRelativeImport failed: %v", err)
			}
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestResolvePythonAbsoluteImport(t *testing.T) {
	tests := []struct {
		name     string
		imp      string
		repoRoot string
		allFiles []string
		expected string
	}{
		{
			name:     "top-level module",
			imp:      "internal",
			repoRoot: "/repo",
			allFiles: []string{"/repo/internal.py"},
			expected: "/repo/internal.py",
		},
		{
			name:     "nested module",
			imp:      "internal.types",
			repoRoot: "/repo",
			allFiles: []string{"/repo/internal/types.py"},
			expected: "/repo/internal/types.py",
		},
		{
			name:     "package import",
			imp:      "internal",
			repoRoot: "/repo",
			allFiles: []string{"/repo/internal/__init__.py"},
			expected: "/repo/internal/__init__.py",
		},
		{
			name:     "not found",
			imp:      "missing",
			repoRoot: "/repo",
			allFiles: []string{"/repo/other.py"},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := resolvePythonAbsoluteImport(tt.imp, tt.repoRoot, tt.allFiles)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}
