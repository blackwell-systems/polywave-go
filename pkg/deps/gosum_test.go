package deps

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGoSumParser_Parse_Valid(t *testing.T) {
	// Create a temporary go.sum file with valid content
	tmpDir := t.TempDir()
	goSumPath := filepath.Join(tmpDir, "go.sum")

	content := `github.com/stretchr/testify v1.8.4 h1:CcVxjf3Q8PM0mHUKJCdn+eZZtm5yQwehR5yeSVQQcUk=
github.com/stretchr/testify v1.8.4/go.mod h1:sz/lmYIOXD/1dqDmKjjqLyZ2RngseejIcXlSw2iwfAo=
github.com/davecgh/go-spew v1.1.1 h1:vj9j/u1bqnvCEfJOwUhtlOARqs3+rkHYY13jYWTU97c=
github.com/davecgh/go-spew v1.1.1/go.mod h1:J7Y8YcW2NihsgmVo/mv3lAwl/skON4iLHjSsI+c5H38=
golang.org/x/sync v0.5.0 h1:60k92dhOjHxJkrqnwsfl8KuaHbn/5dl0lUPUklKo3qE=
golang.org/x/sync v0.5.0/go.mod h1:Czt+wKu1gCyEFDUtn0jG5QVvpJ6rzVqr5aXyt9drQfk=
`

	if err := os.WriteFile(goSumPath, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to create test go.sum: %v", err)
	}

	parser := &GoSumParser{}
	res := parser.Parse(goSumPath)
	if !res.IsSuccess() {
		t.Fatalf("Parse() error = %v", res.Errors)
	}
	packages := res.GetData()

	// Should have 3 unique packages (each has 2 lines, we skip /go.mod lines)
	if len(packages) != 3 {
		t.Errorf("Parse() returned %d packages, want 3", len(packages))
	}

	// Verify package contents
	packageMap := make(map[string]PackageInfo)
	for _, pkg := range packages {
		packageMap[pkg.Name] = pkg
	}

	expected := []struct {
		name    string
		version string
	}{
		{"github.com/stretchr/testify", "v1.8.4"},
		{"github.com/davecgh/go-spew", "v1.1.1"},
		{"golang.org/x/sync", "v0.5.0"},
	}

	for _, exp := range expected {
		pkg, found := packageMap[exp.name]
		if !found {
			t.Errorf("Expected package %s not found", exp.name)
			continue
		}
		if pkg.Version != exp.version {
			t.Errorf("Package %s version = %s, want %s", exp.name, pkg.Version, exp.version)
		}
		if pkg.Source != exp.name {
			t.Errorf("Package %s source = %s, want %s", exp.name, pkg.Source, exp.name)
		}
	}
}

func TestGoSumParser_Parse_EmptyFile(t *testing.T) {
	tmpDir := t.TempDir()
	goSumPath := filepath.Join(tmpDir, "go.sum")

	// Create empty file
	if err := os.WriteFile(goSumPath, []byte(""), 0644); err != nil {
		t.Fatalf("Failed to create empty go.sum: %v", err)
	}

	parser := &GoSumParser{}
	res := parser.Parse(goSumPath)
	if !res.IsSuccess() {
		t.Fatalf("Parse() error = %v", res.Errors)
	}
	packages := res.GetData()

	if len(packages) != 0 {
		t.Errorf("Parse() returned %d packages for empty file, want 0", len(packages))
	}
}

func TestGoSumParser_Parse_MalformedLine(t *testing.T) {
	tmpDir := t.TempDir()
	goSumPath := filepath.Join(tmpDir, "go.sum")

	content := `github.com/stretchr/testify v1.8.4 h1:CcVxjf3Q8PM0mHUKJCdn+eZZtm5yQwehR5yeSVQQcUk=
malformed line without version
github.com/davecgh/go-spew v1.1.1 h1:vj9j/u1bqnvCEfJOwUhtlOARqs3+rkHYY13jYWTU97c=
another bad line
only-two-fields v1.0.0

golang.org/x/sync v0.5.0 h1:60k92dhOjHxJkrqnwsfl8KuaHbn/5dl0lUPUklKo3qE=
`

	if err := os.WriteFile(goSumPath, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to create test go.sum: %v", err)
	}

	parser := &GoSumParser{}
	res := parser.Parse(goSumPath)
	if !res.IsSuccess() {
		t.Fatalf("Parse() error = %v", res.Errors)
	}
	packages := res.GetData()

	// Should have 3 valid packages, malformed lines should be skipped
	if len(packages) != 3 {
		t.Errorf("Parse() returned %d packages, want 3 (malformed lines should be skipped)", len(packages))
		for i, pkg := range packages {
			t.Logf("Package %d: Name=%s, Version=%s", i, pkg.Name, pkg.Version)
		}
	}

	// Verify that valid packages were parsed
	packageMap := make(map[string]PackageInfo)
	for _, pkg := range packages {
		packageMap[pkg.Name] = pkg
	}

	expectedPackages := []string{
		"github.com/stretchr/testify",
		"github.com/davecgh/go-spew",
		"golang.org/x/sync",
	}

	for _, name := range expectedPackages {
		if _, found := packageMap[name]; !found {
			t.Errorf("Expected package %s not found in results", name)
		}
	}
}

func TestGoSumParser_Detect(t *testing.T) {
	parser := &GoSumParser{}

	tests := []struct {
		name     string
		filePath string
		want     bool
	}{
		{
			name:     "valid go.sum",
			filePath: "/path/to/go.sum",
			want:     true,
		},
		{
			name:     "nested go.sum",
			filePath: "/path/to/nested/dir/go.sum",
			want:     true,
		},
		{
			name:     "go.mod file",
			filePath: "/path/to/go.mod",
			want:     false,
		},
		{
			name:     "package.json",
			filePath: "/path/to/package.json",
			want:     false,
		},
		{
			name:     "random file",
			filePath: "/path/to/random.txt",
			want:     false,
		},
		{
			name:     "go.sum in name but not suffix",
			filePath: "/path/to/go.sum.backup",
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

func TestGoSumParser_DeduplicatesModLines(t *testing.T) {
	tmpDir := t.TempDir()
	goSumPath := filepath.Join(tmpDir, "go.sum")

	// go.sum typically has two lines per dependency:
	// one for the module itself and one with /go.mod suffix
	content := `github.com/stretchr/testify v1.8.4 h1:CcVxjf3Q8PM0mHUKJCdn+eZZtm5yQwehR5yeSVQQcUk=
github.com/stretchr/testify v1.8.4/go.mod h1:sz/lmYIOXD/1dqDmKjjqLyZ2RngseejIcXlSw2iwfAo=
`

	if err := os.WriteFile(goSumPath, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to create test go.sum: %v", err)
	}

	parser := &GoSumParser{}
	res := parser.Parse(goSumPath)
	if !res.IsSuccess() {
		t.Fatalf("Parse() error = %v", res.Errors)
	}
	packages := res.GetData()

	// Should return only 1 package, not 2 (the /go.mod line should be skipped)
	if len(packages) != 1 {
		t.Errorf("Parse() returned %d packages, want 1 (should deduplicate /go.mod lines)", len(packages))
	}

	if len(packages) > 0 {
		pkg := packages[0]
		if pkg.Name != "github.com/stretchr/testify" {
			t.Errorf("Package name = %s, want github.com/stretchr/testify", pkg.Name)
		}
		if pkg.Version != "v1.8.4" {
			t.Errorf("Package version = %s, want v1.8.4", pkg.Version)
		}
	}
}

func TestGoSumParser_WhitespaceVariations(t *testing.T) {
	tmpDir := t.TempDir()
	goSumPath := filepath.Join(tmpDir, "go.sum")

	// Test various whitespace scenarios
	content := `  github.com/pkg/errors v0.9.1 h1:FEBLx1zS214owpjy7qsBeixbURkuhQAwrK5UwLGTwt4=
github.com/pkg/errors   v0.9.1/go.mod   h1:bwawxfHBFNV+L2hUp1rHADufV3IMtnDRdf1r5NINEl0=
	github.com/google/uuid v1.3.0 h1:t6JiXgmwXMjEs8VusXIJk2BXHsn+wx8BZdTaoZ5fu7I=

github.com/google/uuid v1.3.0/go.mod h1:TIyPZe4MgqvfeYDBFedMoGGpEw/LqOeaOT+nhxU+yHo=
`

	if err := os.WriteFile(goSumPath, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to create test go.sum: %v", err)
	}

	parser := &GoSumParser{}
	res := parser.Parse(goSumPath)
	if !res.IsSuccess() {
		t.Fatalf("Parse() error = %v", res.Errors)
	}
	packages := res.GetData()

	// Should have 2 unique packages despite whitespace variations
	if len(packages) != 2 {
		t.Errorf("Parse() returned %d packages, want 2", len(packages))
	}

	packageMap := make(map[string]PackageInfo)
	for _, pkg := range packages {
		packageMap[pkg.Name] = pkg
	}

	expected := []string{
		"github.com/pkg/errors",
		"github.com/google/uuid",
	}

	for _, name := range expected {
		if _, found := packageMap[name]; !found {
			t.Errorf("Expected package %s not found", name)
		}
	}
}
