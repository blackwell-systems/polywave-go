package commands

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPackageJSONParser_SimpleScripts(t *testing.T) {
	// Create temporary directory with package.json
	tmpDir := t.TempDir()
	pkgJSON := `{
  "name": "test-project",
  "scripts": {
    "build": "tsc",
    "test": "jest",
    "lint": "eslint .",
    "format": "prettier --check ."
  }
}`
	if err := os.WriteFile(filepath.Join(tmpDir, "package.json"), []byte(pkgJSON), 0644); err != nil {
		t.Fatalf("failed to write package.json: %v", err)
	}

	parser := &PackageJSONParser{}
	r := parser.ParseBuildSystem(tmpDir)
	if r.IsFatal() {
		t.Fatalf("ParseBuildSystem failed: %v", r.Errors)
	}

	cmdSet := r.GetData().CommandSet
	if cmdSet == nil {
		t.Fatal("expected CommandSet, got nil")
	}

	// Verify toolchain
	if cmdSet.Toolchain != "npm" {
		t.Errorf("expected toolchain 'npm', got %q", cmdSet.Toolchain)
	}

	// Verify detection source
	if len(cmdSet.DetectionSources) != 1 || cmdSet.DetectionSources[0] != "package.json" {
		t.Errorf("expected DetectionSources ['package.json'], got %v", cmdSet.DetectionSources)
	}

	// Verify commands
	if cmdSet.Commands.Build != "npm run build" {
		t.Errorf("expected Build 'npm run build', got %q", cmdSet.Commands.Build)
	}
	if cmdSet.Commands.Test.Full != "npm run test" {
		t.Errorf("expected Test.Full 'npm run test', got %q", cmdSet.Commands.Test.Full)
	}
	if cmdSet.Commands.Lint.Check != "npm run lint" {
		t.Errorf("expected Lint.Check 'npm run lint', got %q", cmdSet.Commands.Lint.Check)
	}
	if cmdSet.Commands.Format.Check != "npm run format" {
		t.Errorf("expected Format.Check 'npm run format', got %q", cmdSet.Commands.Format.Check)
	}
}

func TestPackageJSONParser_MonorepoWorkspaces(t *testing.T) {
	// Create temporary directory with monorepo package.json
	tmpDir := t.TempDir()
	pkgJSON := `{
  "name": "monorepo",
  "workspaces": [
    "packages/*",
    "apps/*"
  ],
  "scripts": {
    "test": "jest",
    "build": "turbo build"
  }
}`
	if err := os.WriteFile(filepath.Join(tmpDir, "package.json"), []byte(pkgJSON), 0644); err != nil {
		t.Fatalf("failed to write package.json: %v", err)
	}

	parser := &PackageJSONParser{}
	r := parser.ParseBuildSystem(tmpDir)
	if r.IsFatal() {
		t.Fatalf("ParseBuildSystem failed: %v", r.Errors)
	}

	cmdSet := r.GetData().CommandSet
	if cmdSet == nil {
		t.Fatal("expected CommandSet, got nil")
	}

	// Verify full test command
	if cmdSet.Commands.Test.Full != "npm run test" {
		t.Errorf("expected Test.Full 'npm run test', got %q", cmdSet.Commands.Test.Full)
	}

	// Verify focused pattern includes workspace flag
	expectedPattern := "npm run test --workspace=packages/*"
	if cmdSet.Commands.Test.FocusedPattern != expectedPattern {
		t.Errorf("expected FocusedPattern %q, got %q", expectedPattern, cmdSet.Commands.Test.FocusedPattern)
	}
}

func TestPackageJSONParser_NoPackageJSON(t *testing.T) {
	// Create temporary directory WITHOUT package.json
	tmpDir := t.TempDir()

	parser := &PackageJSONParser{}
	r := parser.ParseBuildSystem(tmpDir)

	// Should return nil (not error) when package.json doesn't exist
	if r.IsFatal() {
		t.Errorf("expected nil error when package.json absent, got: %v", r.Errors)
	}
	cmdSet := r.GetData().CommandSet
	if cmdSet != nil {
		t.Errorf("expected nil CommandSet when package.json absent, got: %+v", cmdSet)
	}
}

func TestPackageJSONParser_MalformedJSON(t *testing.T) {
	// Create temporary directory with malformed package.json
	tmpDir := t.TempDir()
	badJSON := `{
  "name": "test-project",
  "scripts": {
    "build": "tsc"
    "test": "jest"
  }
}`
	if err := os.WriteFile(filepath.Join(tmpDir, "package.json"), []byte(badJSON), 0644); err != nil {
		t.Fatalf("failed to write package.json: %v", err)
	}

	parser := &PackageJSONParser{}
	r := parser.ParseBuildSystem(tmpDir)

	// Should return error for syntax errors
	if !r.IsFatal() {
		t.Error("expected error for malformed JSON, got success")
	}
	cmdSet := r.GetData().CommandSet
	if cmdSet != nil {
		t.Errorf("expected nil CommandSet for malformed JSON, got: %+v", cmdSet)
	}
}

func TestPackageJSONParser_EmptyScripts(t *testing.T) {
	// Create temporary directory with package.json but no scripts
	tmpDir := t.TempDir()
	pkgJSON := `{
  "name": "test-project",
  "version": "1.0.0"
}`
	if err := os.WriteFile(filepath.Join(tmpDir, "package.json"), []byte(pkgJSON), 0644); err != nil {
		t.Fatalf("failed to write package.json: %v", err)
	}

	parser := &PackageJSONParser{}
	r := parser.ParseBuildSystem(tmpDir)

	// Should return nil when no scripts section exists
	if r.IsFatal() {
		t.Errorf("expected nil error when no scripts, got: %v", r.Errors)
	}
	cmdSet := r.GetData().CommandSet
	if cmdSet != nil {
		t.Errorf("expected nil CommandSet when no scripts, got: %+v", cmdSet)
	}
}

func TestPackageJSONParser_Priority(t *testing.T) {
	parser := &PackageJSONParser{}
	priority := parser.Priority()

	// Priority must be 40 (lower than Makefile's 50)
	if priority != 40 {
		t.Errorf("expected priority 40, got %d", priority)
	}
}

func TestPackageJSONParser_AlternativeScriptNames(t *testing.T) {
	// Test that parser recognizes alternative script names
	tmpDir := t.TempDir()
	pkgJSON := `{
  "name": "test-project",
  "scripts": {
    "compile": "webpack",
    "test:unit": "jest",
    "eslint": "eslint .",
    "prettier": "prettier --write ."
  }
}`
	if err := os.WriteFile(filepath.Join(tmpDir, "package.json"), []byte(pkgJSON), 0644); err != nil {
		t.Fatalf("failed to write package.json: %v", err)
	}

	parser := &PackageJSONParser{}
	r := parser.ParseBuildSystem(tmpDir)
	if r.IsFatal() {
		t.Fatalf("ParseBuildSystem failed: %v", r.Errors)
	}

	cmdSet := r.GetData().CommandSet
	if cmdSet == nil {
		t.Fatal("expected CommandSet, got nil")
	}

	// Verify alternative names are recognized
	if cmdSet.Commands.Build != "npm run compile" {
		t.Errorf("expected Build 'npm run compile', got %q", cmdSet.Commands.Build)
	}
	if cmdSet.Commands.Test.Full != "npm run test:unit" {
		t.Errorf("expected Test.Full 'npm run test:unit', got %q", cmdSet.Commands.Test.Full)
	}
	if cmdSet.Commands.Lint.Check != "npm run eslint" {
		t.Errorf("expected Lint.Check 'npm run eslint', got %q", cmdSet.Commands.Lint.Check)
	}
	if cmdSet.Commands.Format.Fix != "npm run prettier" {
		t.Errorf("expected Format.Fix 'npm run prettier', got %q", cmdSet.Commands.Format.Fix)
	}
}
