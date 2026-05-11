package engine

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunInit_AlreadyExists(t *testing.T) {
	dir := t.TempDir()
	// Create existing polywave.config.json.
	if err := os.WriteFile(filepath.Join(dir, "polywave.config.json"), []byte("{}"), 0644); err != nil {
		t.Fatal(err)
	}

	res := RunInit(InitOpts{RepoDir: dir, Force: false})
	if res.IsSuccess() {
		t.Fatal("expected non-success when polywave.config.json exists without Force")
	}
	// AlreadyExists path returns PARTIAL with data
	if res.IsFatal() {
		t.Fatal("expected PARTIAL (not FATAL) when polywave.config.json exists without Force")
	}
	data := res.GetData()
	if !data.AlreadyExists {
		t.Error("expected AlreadyExists=true")
	}
	if len(res.Errors) == 0 {
		t.Fatal("expected errors in partial result")
	}
	if !strings.Contains(res.Errors[0].Message, "already exists") {
		t.Errorf("error message = %q, want it to mention 'already exists'", res.Errors[0].Message)
	}
}

func TestRunInit_Force(t *testing.T) {
	dir := t.TempDir()
	// Create existing polywave.config.json.
	if err := os.WriteFile(filepath.Join(dir, "polywave.config.json"), []byte("{}"), 0644); err != nil {
		t.Fatal(err)
	}
	// Add go.mod so detection works.
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module test"), 0644); err != nil {
		t.Fatal(err)
	}

	res := RunInit(InitOpts{RepoDir: dir, Force: true})
	if !res.IsSuccess() {
		t.Fatalf("unexpected failure with Force: %v", res.Errors)
	}
	data := res.GetData()
	if data.Language != "go" {
		t.Errorf("Language = %q, want 'go'", data.Language)
	}
}

func TestRunInit_GoProject(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module test"), 0644); err != nil {
		t.Fatal(err)
	}

	res := RunInit(InitOpts{RepoDir: dir, Force: false})
	if !res.IsSuccess() {
		t.Fatalf("unexpected failure: %v", res.Errors)
	}

	data := res.GetData()
	if data.Language != "go" {
		t.Errorf("Language = %q, want 'go'", data.Language)
	}
	if data.ProjectName != filepath.Base(dir) {
		t.Errorf("ProjectName = %q, want %q", data.ProjectName, filepath.Base(dir))
	}
	if data.ConfigPath != filepath.Join(dir, "polywave.config.json") {
		t.Errorf("ConfigPath = %q, want %q", data.ConfigPath, filepath.Join(dir, "polywave.config.json"))
	}

	// Verify config file was created.
	if _, err := os.Stat(data.ConfigPath); err != nil {
		t.Errorf("config file not found at %s: %v", data.ConfigPath, err)
	}

	// Verify InstallResult is populated.
	if data.InstallResult.Verdict == "" {
		t.Error("InstallResult.Verdict is empty")
	}
}

func TestRunInit_UnknownProject(t *testing.T) {
	dir := t.TempDir()
	// No marker files — unknown project.

	res := RunInit(InitOpts{RepoDir: dir, Force: false})
	if !res.IsSuccess() {
		t.Fatalf("unexpected failure: %v", res.Errors)
	}

	data := res.GetData()
	if data.Language != "unknown" {
		t.Errorf("Language = %q, want 'unknown'", data.Language)
	}
}

func TestDetectProjectInternal_Markers(t *testing.T) {
	tests := []struct {
		name     string
		marker   string
		wantLang string
		wantBld  string
		wantTest string
	}{
		{"go", "go.mod", "go", "go build ./...", "go test ./..."},
		{"rust", "Cargo.toml", "rust", "cargo build", "cargo test"},
		{"node", "package.json", "node", "npm run build", "npm test"},
		{"python_pyproject", "pyproject.toml", "python", "", "pytest"},
		{"ruby", "Gemfile", "ruby", "", "bundle exec rspec"},
		{"makefile", "Makefile", "makefile", "make", "make test"},
		{"unknown", "", "unknown", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			if tt.marker != "" {
				if err := os.WriteFile(filepath.Join(dir, tt.marker), []byte(""), 0644); err != nil {
					t.Fatal(err)
				}
			}
			got := detectProjectInternal(dir)
			if got.Language != tt.wantLang {
				t.Errorf("Language = %q, want %q", got.Language, tt.wantLang)
			}
			if got.Build != tt.wantBld {
				t.Errorf("Build = %q, want %q", got.Build, tt.wantBld)
			}
			if got.Test != tt.wantTest {
				t.Errorf("Test = %q, want %q", got.Test, tt.wantTest)
			}
			if got.Name != filepath.Base(dir) {
				t.Errorf("Name = %q, want %q", got.Name, filepath.Base(dir))
			}
		})
	}
}

func TestRunInit_ConfigHasBuildTestKeys(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module test"), 0644); err != nil {
		t.Fatal(err)
	}

	res := RunInit(InitOpts{RepoDir: dir})
	if !res.IsSuccess() {
		t.Fatalf("unexpected failure: %v", res.Errors)
	}

	data := res.GetData()
	content, err := os.ReadFile(data.ConfigPath)
	if err != nil {
		t.Fatalf("cannot read config: %v", err)
	}

	s := string(content)
	if !strings.Contains(s, `"build"`) {
		t.Error("config missing 'build' key")
	}
	if !strings.Contains(s, `"test"`) {
		t.Error("config missing 'test' key")
	}
	if !strings.Contains(s, `"go build ./..."`) {
		t.Error("config missing go build command")
	}
	if !strings.Contains(s, `"go test ./..."`) {
		t.Error("config missing go test command")
	}
}
