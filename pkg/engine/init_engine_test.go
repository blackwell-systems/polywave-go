package engine

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunInit_AlreadyExists(t *testing.T) {
	dir := t.TempDir()
	// Create existing saw.config.json.
	if err := os.WriteFile(filepath.Join(dir, "saw.config.json"), []byte("{}"), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := RunInit(InitOpts{RepoDir: dir, Force: false})
	if err == nil {
		t.Fatal("expected error when saw.config.json exists without Force")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Errorf("error = %q, want it to mention 'already exists'", err.Error())
	}
}

func TestRunInit_Force(t *testing.T) {
	dir := t.TempDir()
	// Create existing saw.config.json.
	if err := os.WriteFile(filepath.Join(dir, "saw.config.json"), []byte("{}"), 0644); err != nil {
		t.Fatal(err)
	}
	// Add go.mod so detection works.
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module test"), 0644); err != nil {
		t.Fatal(err)
	}

	result, err := RunInit(InitOpts{RepoDir: dir, Force: true})
	if err != nil {
		t.Fatalf("unexpected error with Force: %v", err)
	}
	if result.Language != "go" {
		t.Errorf("Language = %q, want 'go'", result.Language)
	}
}

func TestRunInit_GoProject(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module test"), 0644); err != nil {
		t.Fatal(err)
	}

	result, err := RunInit(InitOpts{RepoDir: dir, Force: false})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Language != "go" {
		t.Errorf("Language = %q, want 'go'", result.Language)
	}
	if result.ProjectName != filepath.Base(dir) {
		t.Errorf("ProjectName = %q, want %q", result.ProjectName, filepath.Base(dir))
	}
	if result.ConfigPath != filepath.Join(dir, "saw.config.json") {
		t.Errorf("ConfigPath = %q, want %q", result.ConfigPath, filepath.Join(dir, "saw.config.json"))
	}

	// Verify config file was created.
	if _, err := os.Stat(result.ConfigPath); err != nil {
		t.Errorf("config file not found at %s: %v", result.ConfigPath, err)
	}

	// Verify InstallResult is populated.
	if result.InstallResult.Verdict == "" {
		t.Error("InstallResult.Verdict is empty")
	}
}

func TestRunInit_UnknownProject(t *testing.T) {
	dir := t.TempDir()
	// No marker files — unknown project.

	result, err := RunInit(InitOpts{RepoDir: dir, Force: false})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Language != "unknown" {
		t.Errorf("Language = %q, want 'unknown'", result.Language)
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

	result, err := RunInit(InitOpts{RepoDir: dir})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := os.ReadFile(result.ConfigPath)
	if err != nil {
		t.Fatalf("cannot read config: %v", err)
	}

	content := string(data)
	if !strings.Contains(content, `"build"`) {
		t.Error("config missing 'build' key")
	}
	if !strings.Contains(content, `"test"`) {
		t.Error("config missing 'test' key")
	}
	if !strings.Contains(content, `"go build ./..."`) {
		t.Error("config missing go build command")
	}
	if !strings.Contains(content, `"go test ./..."`) {
		t.Error("config missing go test command")
	}
}
