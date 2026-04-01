package deps

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPackageLockParser_Parse_Valid(t *testing.T) {
	// Create a temporary valid package-lock.json
	content := `{
  "name": "test-project",
  "version": "1.0.0",
  "lockfileVersion": 3,
  "packages": {
    "": {
      "name": "test-project",
      "version": "1.0.0"
    },
    "node_modules/lodash": {
      "version": "4.17.21",
      "resolved": "https://registry.npmjs.org/lodash/-/lodash-4.17.21.tgz"
    },
    "node_modules/express": {
      "version": "4.18.2",
      "resolved": "https://registry.npmjs.org/express/-/express-4.18.2.tgz"
    }
  }
}`

	tmpFile := createTempFile(t, "package-lock.json", content)
	defer os.Remove(tmpFile)

	parser := &PackageLockParser{}
	res := parser.Parse(tmpFile)
	if !res.IsSuccess() {
		t.Fatalf("Parse() failed: %v", res.Errors)
	}
	packages := res.GetData()

	if len(packages) != 2 {
		t.Fatalf("expected 2 packages, got %d", len(packages))
	}

	// Check lodash
	foundLodash := false
	foundExpress := false
	for _, pkg := range packages {
		if pkg.Name == "lodash" {
			foundLodash = true
			if pkg.Version != "4.17.21" {
				t.Errorf("lodash version = %s, want 4.17.21", pkg.Version)
			}
			if pkg.Source != "https://registry.npmjs.org/lodash/-/lodash-4.17.21.tgz" {
				t.Errorf("lodash source = %s, want https://registry.npmjs.org/lodash/-/lodash-4.17.21.tgz", pkg.Source)
			}
		}
		if pkg.Name == "express" {
			foundExpress = true
			if pkg.Version != "4.18.2" {
				t.Errorf("express version = %s, want 4.18.2", pkg.Version)
			}
		}
	}

	if !foundLodash {
		t.Error("lodash not found in parsed packages")
	}
	if !foundExpress {
		t.Error("express not found in parsed packages")
	}
}

func TestPackageLockParser_Parse_EmptyPackages(t *testing.T) {
	content := `{
  "name": "test-project",
  "version": "1.0.0",
  "lockfileVersion": 2,
  "packages": {}
}`

	tmpFile := createTempFile(t, "package-lock.json", content)
	defer os.Remove(tmpFile)

	parser := &PackageLockParser{}
	res := parser.Parse(tmpFile)
	if !res.IsSuccess() {
		t.Fatalf("Parse() failed: %v", res.Errors)
	}
	packages := res.GetData()

	if len(packages) != 0 {
		t.Errorf("expected 0 packages for empty packages map, got %d", len(packages))
	}
}

func TestPackageLockParser_Parse_MalformedJSON(t *testing.T) {
	content := `{
  "name": "test-project",
  "version": "1.0.0",
  "lockfileVersion": 2,
  "packages": {
    "node_modules/lodash": {
      "version": "4.17.21"
    }
  }
  // malformed JSON with trailing comment
}`

	tmpFile := createTempFile(t, "package-lock.json", content)
	defer os.Remove(tmpFile)

	parser := &PackageLockParser{}
	res := parser.Parse(tmpFile)
	if res.IsSuccess() {
		t.Error("Parse() should return error for malformed JSON")
	}
}

func TestPackageLockParser_Detect(t *testing.T) {
	parser := &PackageLockParser{}

	tests := []struct {
		name     string
		filePath string
		want     bool
	}{
		{
			name:     "valid package-lock.json",
			filePath: "/path/to/package-lock.json",
			want:     true,
		},
		{
			name:     "package-lock.json in subdirectory",
			filePath: "/path/to/subdir/package-lock.json",
			want:     true,
		},
		{
			name:     "yarn.lock",
			filePath: "/path/to/yarn.lock",
			want:     false,
		},
		{
			name:     "go.mod",
			filePath: "/path/to/go.mod",
			want:     false,
		},
		{
			name:     "package.json (not lock file)",
			filePath: "/path/to/package.json",
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parser.Detect(tt.filePath)
			if got != tt.want {
				t.Errorf("Detect(%q) = %v, want %v", tt.filePath, got, tt.want)
			}
		})
	}
}

func TestPackageLockParser_StripNodeModulesPrefix(t *testing.T) {
	content := `{
  "name": "test-project",
  "version": "1.0.0",
  "lockfileVersion": 3,
  "packages": {
    "": {
      "name": "test-project",
      "version": "1.0.0"
    },
    "node_modules/@types/node": {
      "version": "18.11.9",
      "resolved": "https://registry.npmjs.org/@types/node/-/node-18.11.9.tgz"
    },
    "node_modules/@babel/core": {
      "version": "7.20.0",
      "resolved": "https://registry.npmjs.org/@babel/core/-/core-7.20.0.tgz"
    }
  }
}`

	tmpFile := createTempFile(t, "package-lock.json", content)
	defer os.Remove(tmpFile)

	parser := &PackageLockParser{}
	res := parser.Parse(tmpFile)
	if !res.IsSuccess() {
		t.Fatalf("Parse() failed: %v", res.Errors)
	}
	packages := res.GetData()

	if len(packages) != 2 {
		t.Fatalf("expected 2 packages, got %d", len(packages))
	}

	// Verify scoped packages are handled correctly
	foundTypesNode := false
	foundBabelCore := false
	for _, pkg := range packages {
		if pkg.Name == "@types/node" {
			foundTypesNode = true
			if pkg.Version != "18.11.9" {
				t.Errorf("@types/node version = %s, want 18.11.9", pkg.Version)
			}
		}
		if pkg.Name == "@babel/core" {
			foundBabelCore = true
			if pkg.Version != "7.20.0" {
				t.Errorf("@babel/core version = %s, want 7.20.0", pkg.Version)
			}
		}
		// Ensure node_modules/ prefix is stripped
		if strings.HasPrefix(pkg.Name, "node_modules/") {
			t.Errorf("package name %q still has node_modules/ prefix", pkg.Name)
		}
	}

	if !foundTypesNode {
		t.Error("@types/node not found in parsed packages")
	}
	if !foundBabelCore {
		t.Error("@babel/core not found in parsed packages")
	}
}

func TestPackageLockParser_Parse_UnsupportedVersion(t *testing.T) {
	// Test with npm v6 format (lockfileVersion 1)
	content := `{
  "name": "test-project",
  "version": "1.0.0",
  "lockfileVersion": 1,
  "dependencies": {
    "lodash": {
      "version": "4.17.21"
    }
  }
}`

	tmpFile := createTempFile(t, "package-lock.json", content)
	defer os.Remove(tmpFile)

	parser := &PackageLockParser{}
	res := parser.Parse(tmpFile)
	if res.IsSuccess() {
		t.Error("Parse() should return error for unsupported lockfile version")
	}
	if len(res.Errors) > 0 && !strings.Contains(res.Errors[0].Message, "unsupported lockfile version") {
		t.Errorf("error message should mention unsupported version, got: %v", res.Errors[0].Message)
	}
}

func TestPackageLockParser_Parse_SkipsRootPackage(t *testing.T) {
	content := `{
  "name": "test-project",
  "version": "1.0.0",
  "lockfileVersion": 2,
  "packages": {
    "": {
      "name": "test-project",
      "version": "1.0.0"
    },
    "node_modules/lodash": {
      "version": "4.17.21",
      "resolved": "https://registry.npmjs.org/lodash/-/lodash-4.17.21.tgz"
    }
  }
}`

	tmpFile := createTempFile(t, "package-lock.json", content)
	defer os.Remove(tmpFile)

	parser := &PackageLockParser{}
	res := parser.Parse(tmpFile)
	if !res.IsSuccess() {
		t.Fatalf("Parse() failed: %v", res.Errors)
	}
	packages := res.GetData()

	// Should only have lodash, not the root package
	if len(packages) != 1 {
		t.Errorf("expected 1 package (root should be skipped), got %d", len(packages))
	}

	for _, pkg := range packages {
		if pkg.Name == "test-project" || pkg.Name == "" {
			t.Error("root package should be excluded from results")
		}
	}
}

func TestPackageLockParser_Parse_SkipsPackagesWithoutVersion(t *testing.T) {
	// Some workspace entries may not have versions
	content := `{
  "name": "monorepo",
  "version": "1.0.0",
  "lockfileVersion": 3,
  "packages": {
    "": {
      "name": "monorepo",
      "version": "1.0.0"
    },
    "node_modules/lodash": {
      "version": "4.17.21",
      "resolved": "https://registry.npmjs.org/lodash/-/lodash-4.17.21.tgz"
    },
    "packages/workspace-a": {
      "name": "workspace-a"
    }
  }
}`

	tmpFile := createTempFile(t, "package-lock.json", content)
	defer os.Remove(tmpFile)

	parser := &PackageLockParser{}
	res := parser.Parse(tmpFile)
	if !res.IsSuccess() {
		t.Fatalf("Parse() failed: %v", res.Errors)
	}
	packages := res.GetData()

	// Should only have lodash, workspace without version should be skipped
	if len(packages) != 1 {
		t.Errorf("expected 1 package (workspace without version should be skipped), got %d", len(packages))
	}

	if packages[0].Name != "lodash" {
		t.Errorf("expected only lodash, got %s", packages[0].Name)
	}
}

// Helper function to create temporary test files
func createTempFile(t *testing.T, name, content string) string {
	t.Helper()
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, name)
	if err := os.WriteFile(tmpFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	return tmpFile
}
