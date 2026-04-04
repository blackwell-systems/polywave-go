package analyzer

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// setupMockJSParserOnPath creates a mock js-parser.js in a temp dir and prepends it to PATH.
// Returns a cleanup function that restores the original PATH.
func setupMockJSParserOnPath(t *testing.T) (repoRoot string, cleanup func()) {
	t.Helper()

	repoRoot = t.TempDir()

	// Create the mock parser in a separate bin dir on PATH.
	binDir := t.TempDir()
	mockParser := filepath.Join(binDir, "js-parser.js")
	createMockJSParser(t, mockParser)

	// Prepend binDir to PATH so exec.LookPath("js-parser.js") finds it.
	originalPath := os.Getenv("PATH")
	os.Setenv("PATH", binDir+string(os.PathListSeparator)+originalPath)

	cleanup = func() {
		os.Setenv("PATH", originalPath)
	}
	return repoRoot, cleanup
}

// TestParseJavaScriptFiles_ES6 tests parsing ES6 module syntax
func TestParseJavaScriptFiles_ES6(t *testing.T) {
	// Skip if node not found
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("node binary not found, skipping JavaScript tests")
	}

	repoRoot, cleanup := setupMockJSParserOnPath(t)
	defer cleanup()

	// Create test files including the actual imported files so resolveJSImport can find them.
	testDir := filepath.Join(repoRoot, "testdata", "javascript")
	if err := os.MkdirAll(testDir, 0755); err != nil {
		t.Fatalf("failed to create test directory: %v", err)
	}

	testFile := filepath.Join(testDir, "es6.js")
	if err := os.WriteFile(testFile, []byte(`
import { foo, bar } from './utils';
import { Baz } from '../types';
import lodash from 'lodash';
`), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	// Create the files that are imported so resolveJSImport can resolve them.
	if err := os.WriteFile(filepath.Join(testDir, "utils.js"), []byte("// utils"), 0644); err != nil {
		t.Fatalf("failed to create utils.js: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repoRoot, "testdata", "types.js"), []byte("// types"), 0644); err != nil {
		t.Fatalf("failed to create types.js: %v", err)
	}

	// Test parsing
	result, err := parseJavaScriptFiles(repoRoot, []string{testFile})
	if err != nil {
		t.Fatalf("parseJavaScriptFiles failed: %v", err)
	}

	// Verify results
	deps, ok := result[testFile]
	if !ok {
		t.Fatalf("expected result for %s, got none", testFile)
	}

	// Should have 2 local imports (./utils and ../types), lodash is filtered out
	if len(deps) != 2 {
		t.Errorf("expected 2 local dependencies, got %d: %v", len(deps), deps)
	}

	// Check that lodash is not in the dependencies
	for _, dep := range deps {
		if filepath.Base(dep) == "lodash" {
			t.Errorf("npm package 'lodash' should be filtered out, but found in deps")
		}
	}
}

// TestParseJavaScriptFiles_CommonJS tests parsing CommonJS require() syntax
func TestParseJavaScriptFiles_CommonJS(t *testing.T) {
	// Skip if node not found
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("node binary not found, skipping JavaScript tests")
	}

	repoRoot, cleanup := setupMockJSParserOnPath(t)
	defer cleanup()

	testDir := filepath.Join(repoRoot, "testdata", "javascript")
	if err := os.MkdirAll(testDir, 0755); err != nil {
		t.Fatalf("failed to create test directory: %v", err)
	}

	testFile := filepath.Join(testDir, "commonjs.js")
	if err := os.WriteFile(testFile, []byte(`
const utils = require('./utils');
const types = require('../types');
const express = require('express');
`), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	// Create the imported files so resolveJSImport can resolve them.
	if err := os.WriteFile(filepath.Join(testDir, "utils.js"), []byte("// utils"), 0644); err != nil {
		t.Fatalf("failed to create utils.js: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repoRoot, "testdata", "types.js"), []byte("// types"), 0644); err != nil {
		t.Fatalf("failed to create types.js: %v", err)
	}

	result, err := parseJavaScriptFiles(repoRoot, []string{testFile})
	if err != nil {
		t.Fatalf("parseJavaScriptFiles failed: %v", err)
	}

	deps, ok := result[testFile]
	if !ok {
		t.Fatalf("expected result for %s, got none", testFile)
	}

	// Should have 2 local requires (./utils and ../types), express is filtered out
	if len(deps) != 2 {
		t.Errorf("expected 2 local dependencies, got %d: %v", len(deps), deps)
	}
}

// TestParseJavaScriptFiles_TypeScript tests parsing TypeScript import syntax
func TestParseJavaScriptFiles_TypeScript(t *testing.T) {
	// Skip if node not found
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("node binary not found, skipping JavaScript tests")
	}

	repoRoot, cleanup := setupMockJSParserOnPath(t)
	defer cleanup()

	testDir := filepath.Join(repoRoot, "testdata", "javascript")
	if err := os.MkdirAll(testDir, 0755); err != nil {
		t.Fatalf("failed to create test directory: %v", err)
	}

	testFile := filepath.Join(testDir, "typescript.ts")
	if err := os.WriteFile(testFile, []byte(`
import { foo } from './utils';
import type { Baz } from './interfaces';
import { Component } from 'react';
`), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	// Create the imported utils file so resolveJSImport can resolve it.
	if err := os.WriteFile(filepath.Join(testDir, "utils.js"), []byte("// utils"), 0644); err != nil {
		t.Fatalf("failed to create utils.js: %v", err)
	}

	result, err := parseJavaScriptFiles(repoRoot, []string{testFile})
	if err != nil {
		t.Fatalf("parseJavaScriptFiles failed: %v", err)
	}

	deps, ok := result[testFile]
	if !ok {
		t.Fatalf("expected result for %s, got none", testFile)
	}

	// Should have 1 local import (./utils), react is filtered out, interfaces not on disk
	if len(deps) < 1 {
		t.Errorf("expected at least 1 local dependency, got %d: %v", len(deps), deps)
	}

	// Verify ./utils is in dependencies
	hasUtils := false
	for _, dep := range deps {
		if contains(filepath.Base(dep), "utils") {
			hasUtils = true
			break
		}
	}
	if !hasUtils {
		t.Errorf("expected ./utils in dependencies, got: %v", deps)
	}
}

// TestParseJavaScriptFiles_BinaryMissing tests behavior when node is not available
func TestParseJavaScriptFiles_BinaryMissing(t *testing.T) {
	// Save original PATH
	originalPath := os.Getenv("PATH")
	defer os.Setenv("PATH", originalPath)

	// Set PATH to empty to simulate missing node
	os.Setenv("PATH", "/nonexistent")

	repoRoot := t.TempDir()
	testFile := filepath.Join(repoRoot, "test.js")

	_, err := parseJavaScriptFiles(repoRoot, []string{testFile})
	if err == nil {
		t.Fatal("expected error when node binary is missing, got nil")
	}

	if !contains(err.Error(), "node binary not found") {
		t.Errorf("expected error about node binary, got: %v", err)
	}
}

// TestParseJavaScriptFiles_ParserScriptMissing tests behavior when js-parser.js is not in PATH
func TestParseJavaScriptFiles_ParserScriptMissing(t *testing.T) {
	// Skip if node not found
	nodePath, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node binary not found, skipping JavaScript tests")
	}

	// Build a PATH that contains node but not js-parser.js.
	// Use the directory of the node binary so node is still found.
	nodeDir := filepath.Dir(nodePath)
	originalPath := os.Getenv("PATH")
	defer os.Setenv("PATH", originalPath)
	os.Setenv("PATH", nodeDir)

	repoRoot := t.TempDir()
	testFile := filepath.Join(repoRoot, "test.js")

	_, err = parseJavaScriptFiles(repoRoot, []string{testFile})
	if err == nil {
		t.Fatal("expected error when js-parser.js is not in PATH, got nil")
	}

	if !contains(err.Error(), "js-parser.js not found in PATH") {
		t.Errorf("expected error about js-parser.js, got: %v", err)
	}
}

// TestResolveJSImport tests JavaScript import path resolution
func TestResolveJSImport(t *testing.T) {
	repoRoot := t.TempDir()

	// Create test file structure
	testDir := filepath.Join(repoRoot, "src")
	if err := os.MkdirAll(testDir, 0755); err != nil {
		t.Fatalf("failed to create test directory: %v", err)
	}

	// Create some test files
	files := []string{
		filepath.Join(testDir, "main.js"),
		filepath.Join(testDir, "utils.js"),
		filepath.Join(testDir, "helpers.ts"),
	}
	for _, f := range files {
		if err := os.WriteFile(f, []byte("// test"), 0644); err != nil {
			t.Fatalf("failed to create test file: %v", err)
		}
	}

	tests := []struct {
		name       string
		importer   string
		importPath string
		wantSuffix string
	}{
		{
			name:       "resolve .js file",
			importer:   filepath.Join(testDir, "main.js"),
			importPath: "./utils",
			wantSuffix: "utils.js",
		},
		{
			name:       "resolve .ts file",
			importer:   filepath.Join(testDir, "main.js"),
			importPath: "./helpers",
			wantSuffix: "helpers.ts",
		},
		{
			name:       "resolve parent directory",
			importer:   filepath.Join(testDir, "subdir", "file.js"),
			importPath: "../utils",
			wantSuffix: "utils.js",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resolved, err := resolveJSImport(tt.importer, tt.importPath, repoRoot)
			if err != nil {
				t.Fatalf("resolveJSImport failed: %v", err)
			}

			if !contains(resolved, tt.wantSuffix) {
				t.Errorf("expected resolved path to contain %q, got %q", tt.wantSuffix, resolved)
			}
		})
	}
}

// TestResolveJSImport_Unresolvable tests that resolveJSImport returns an error
// when the import cannot be mapped to a known file on disk.
func TestResolveJSImport_Unresolvable(t *testing.T) {
	repoRoot := t.TempDir()

	// No actual files created — import cannot be resolved.
	importer := filepath.Join(repoRoot, "src", "main.js")
	importPath := "./nonexistent-module"

	resolved, err := resolveJSImport(importer, importPath, repoRoot)
	if err == nil {
		t.Fatalf("expected error for unresolvable import, got resolved path: %q", resolved)
	}

	if !contains(err.Error(), "cannot resolve JS import") {
		t.Errorf("expected error to mention 'cannot resolve JS import', got: %v", err)
	}

	if resolved != "" {
		t.Errorf("expected empty resolved path on error, got: %q", resolved)
	}
}

// createMockJSParser creates a mock js-parser.js that returns predictable output
func createMockJSParser(t *testing.T, path string) {
	t.Helper()

	script := `#!/usr/bin/env node
// Mock js-parser.js for testing
const fs = require('fs');
const path = require('path');

const file = process.argv[2];
if (!file) {
  console.error('Usage: node js-parser.js <file>');
  process.exit(1);
}

// Read file content
const content = fs.readFileSync(file, 'utf-8');

// Simple regex-based import extraction (good enough for tests)
const imports = [];

// ES6 imports
const importRegex = /import\s+(?:(?:\{[^}]*\}|\*\s+as\s+\w+|\w+)\s+from\s+)?['"]([^'"]+)['"]/g;
let match;
while ((match = importRegex.exec(content)) !== null) {
  imports.push(match[1]);
}

// CommonJS requires
const requireRegex = /require\s*\(['"]([^'"]+)['"]\)/g;
while ((match = requireRegex.exec(content)) !== null) {
  imports.push(match[1]);
}

// Output JSON
const output = {
  file: file,
  imports: imports
};

console.log(JSON.stringify(output));
`

	if err := os.WriteFile(path, []byte(script), 0755); err != nil {
		t.Fatalf("failed to create mock parser: %v", err)
	}
}

// TestParseJavaScriptFiles_MultipleFiles tests parsing multiple files at once
func TestParseJavaScriptFiles_MultipleFiles(t *testing.T) {
	// Skip if node not found
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("node binary not found, skipping JavaScript tests")
	}

	repoRoot, cleanup := setupMockJSParserOnPath(t)
	defer cleanup()

	// Create test files
	testDir := filepath.Join(repoRoot, "src")
	if err := os.MkdirAll(testDir, 0755); err != nil {
		t.Fatalf("failed to create test directory: %v", err)
	}

	file1 := filepath.Join(testDir, "a.js")
	file2 := filepath.Join(testDir, "b.js")

	if err := os.WriteFile(file1, []byte(`import { foo } from './utils';`), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}
	if err := os.WriteFile(file2, []byte(`import { bar } from './helpers';`), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	result, err := parseJavaScriptFiles(repoRoot, []string{file1, file2})
	if err != nil {
		t.Fatalf("parseJavaScriptFiles failed: %v", err)
	}

	if len(result) != 2 {
		t.Errorf("expected 2 files in result, got %d", len(result))
	}

	if _, ok := result[file1]; !ok {
		t.Errorf("expected result for %s", file1)
	}
	if _, ok := result[file2]; !ok {
		t.Errorf("expected result for %s", file2)
	}
}
