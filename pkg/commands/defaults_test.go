package commands

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLanguageDefaults_Go(t *testing.T) {
	// Create temp directory with go.mod
	tmpDir := t.TempDir()
	goModPath := filepath.Join(tmpDir, "go.mod")
	if err := os.WriteFile(goModPath, []byte("module example.com/test\n"), 0644); err != nil {
		t.Fatalf("failed to create go.mod: %v", err)
	}

	// Call LanguageDefaults
	r := LanguageDefaults(tmpDir)
	if r.IsFatal() {
		t.Fatalf("LanguageDefaults returned error: %v", r.Errors)
	}

	result := r.GetData().CommandSet
	// Verify toolchain
	if result.Toolchain != "go" {
		t.Errorf("expected toolchain 'go', got %q", result.Toolchain)
	}

	// Verify build command
	if result.Commands.Build != "go build ./..." {
		t.Errorf("expected build command 'go build ./...', got %q", result.Commands.Build)
	}

	// Verify test commands
	if result.Commands.Test.Full != "go test ./..." {
		t.Errorf("expected test.full 'go test ./...', got %q", result.Commands.Test.Full)
	}
	if result.Commands.Test.FocusedPattern == "" {
		t.Error("expected non-empty test.focused_pattern")
	}

	// Verify lint command
	if result.Commands.Lint.Check != "go vet ./..." {
		t.Errorf("expected lint.check 'go vet ./...', got %q", result.Commands.Lint.Check)
	}

	// Verify format command
	if result.Commands.Format.Fix != "gofmt -w ." {
		t.Errorf("expected format.fix 'gofmt -w .', got %q", result.Commands.Format.Fix)
	}

	// Verify detection sources
	if len(result.DetectionSources) != 1 || result.DetectionSources[0] != "go.mod" {
		t.Errorf("expected detection_sources ['go.mod'], got %v", result.DetectionSources)
	}
}

func TestLanguageDefaults_Rust(t *testing.T) {
	// Create temp directory with Cargo.toml
	tmpDir := t.TempDir()
	cargoPath := filepath.Join(tmpDir, "Cargo.toml")
	if err := os.WriteFile(cargoPath, []byte("[package]\nname = \"test\"\n"), 0644); err != nil {
		t.Fatalf("failed to create Cargo.toml: %v", err)
	}

	// Call LanguageDefaults
	r := LanguageDefaults(tmpDir)
	if r.IsFatal() {
		t.Fatalf("LanguageDefaults returned error: %v", r.Errors)
	}

	result := r.GetData().CommandSet
	// Verify toolchain
	if result.Toolchain != "rust" {
		t.Errorf("expected toolchain 'rust', got %q", result.Toolchain)
	}

	// Verify build command
	if result.Commands.Build != "cargo build" {
		t.Errorf("expected build command 'cargo build', got %q", result.Commands.Build)
	}

	// Verify test command
	if result.Commands.Test.Full != "cargo test" {
		t.Errorf("expected test.full 'cargo test', got %q", result.Commands.Test.Full)
	}

	// Verify lint command
	if result.Commands.Lint.Check != "cargo clippy -- -D warnings" {
		t.Errorf("expected lint.check 'cargo clippy -- -D warnings', got %q", result.Commands.Lint.Check)
	}

	// Verify format commands
	if result.Commands.Format.Check != "cargo fmt -- --check" {
		t.Errorf("expected format.check 'cargo fmt -- --check', got %q", result.Commands.Format.Check)
	}
	if result.Commands.Format.Fix != "cargo fmt" {
		t.Errorf("expected format.fix 'cargo fmt', got %q", result.Commands.Format.Fix)
	}

	// Verify detection sources
	if len(result.DetectionSources) != 1 || result.DetectionSources[0] != "Cargo.toml" {
		t.Errorf("expected detection_sources ['Cargo.toml'], got %v", result.DetectionSources)
	}
}

func TestLanguageDefaults_Node(t *testing.T) {
	// Create temp directory with package.json
	tmpDir := t.TempDir()
	pkgPath := filepath.Join(tmpDir, "package.json")
	if err := os.WriteFile(pkgPath, []byte("{\"name\": \"test\"}\n"), 0644); err != nil {
		t.Fatalf("failed to create package.json: %v", err)
	}

	// Call LanguageDefaults
	r := LanguageDefaults(tmpDir)
	if r.IsFatal() {
		t.Fatalf("LanguageDefaults returned error: %v", r.Errors)
	}

	result := r.GetData().CommandSet
	// Verify toolchain
	if result.Toolchain != "node" {
		t.Errorf("expected toolchain 'node', got %q", result.Toolchain)
	}

	// Verify build command
	if result.Commands.Build != "npm run build" {
		t.Errorf("expected build command 'npm run build', got %q", result.Commands.Build)
	}

	// Verify test command
	if result.Commands.Test.Full != "npm test" {
		t.Errorf("expected test.full 'npm test', got %q", result.Commands.Test.Full)
	}

	// Verify lint command
	if result.Commands.Lint.Check != "npm run lint" {
		t.Errorf("expected lint.check 'npm run lint', got %q", result.Commands.Lint.Check)
	}

	// Verify format command
	if result.Commands.Format.Check != "npm run format:check" {
		t.Errorf("expected format.check 'npm run format:check', got %q", result.Commands.Format.Check)
	}

	// Verify detection sources
	if len(result.DetectionSources) != 1 || result.DetectionSources[0] != "package.json" {
		t.Errorf("expected detection_sources ['package.json'], got %v", result.DetectionSources)
	}
}

func TestLanguageDefaults_Python(t *testing.T) {
	// Create temp directory with pyproject.toml
	tmpDir := t.TempDir()
	pyprojectPath := filepath.Join(tmpDir, "pyproject.toml")
	if err := os.WriteFile(pyprojectPath, []byte("[project]\nname = \"test\"\n"), 0644); err != nil {
		t.Fatalf("failed to create pyproject.toml: %v", err)
	}

	// Call LanguageDefaults
	r := LanguageDefaults(tmpDir)
	if r.IsFatal() {
		t.Fatalf("LanguageDefaults returned error: %v", r.Errors)
	}

	result := r.GetData().CommandSet
	// Verify toolchain
	if result.Toolchain != "python" {
		t.Errorf("expected toolchain 'python', got %q", result.Toolchain)
	}

	// Verify build command (should be empty for interpreted language)
	if result.Commands.Build != "" {
		t.Errorf("expected empty build command for Python, got %q", result.Commands.Build)
	}

	// Verify test command
	if result.Commands.Test.Full != "pytest" {
		t.Errorf("expected test.full 'pytest', got %q", result.Commands.Test.Full)
	}

	// Verify lint command
	if result.Commands.Lint.Check != "ruff check ." {
		t.Errorf("expected lint.check 'ruff check .', got %q", result.Commands.Lint.Check)
	}

	// Verify format command
	if result.Commands.Format.Fix != "black ." {
		t.Errorf("expected format.fix 'black .', got %q", result.Commands.Format.Fix)
	}

	// Verify detection sources
	if len(result.DetectionSources) != 1 || result.DetectionSources[0] != "pyproject.toml" {
		t.Errorf("expected detection_sources ['pyproject.toml'], got %v", result.DetectionSources)
	}
}

func TestLanguageDefaults_NoMarkers(t *testing.T) {
	// Create empty temp directory
	tmpDir := t.TempDir()

	// Call LanguageDefaults - should return error
	r := LanguageDefaults(tmpDir)
	if !r.IsFatal() {
		t.Fatalf("expected error when no marker files present, got success: %+v", r.GetData())
	}

	// Verify error message mentions the missing files
	if len(r.Errors) == 0 {
		t.Error("expected error list, got empty")
	}
}
