package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/protocol"
)

// TestResolveImplCmd_ExplicitSlug tests resolution by feature_slug.
func TestResolveImplCmd_ExplicitSlug(t *testing.T) {
	// Create a temporary directory structure
	tmpDir := t.TempDir()
	implDocsDir := filepath.Join(tmpDir, "docs", "IMPL")
	if err := os.MkdirAll(implDocsDir, 0755); err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}

	// Create a test IMPL manifest
	testSlug := "test-feature"
	testIMPLPath := filepath.Join(implDocsDir, "IMPL-test-feature.yaml")
	testManifest := &protocol.IMPLManifest{
		FeatureSlug: testSlug,
		Verdict:     "scout-approved",
		State:       protocol.StateWavePending,
		Waves: []protocol.Wave{
			{Number: 1, Agents: []protocol.Agent{{ID: "A"}}},
		},
	}
	if saveRes := protocol.Save(testManifest, testIMPLPath); saveRes.IsFatal() {
		t.Fatalf("Failed to save test IMPL: %v", saveRes.Errors)
	}

	// Run command
	cmd := newResolveImplCmd()
	cmd.SetArgs([]string{"--impl", testSlug})

	var outBuf bytes.Buffer
	cmd.SetOut(&outBuf)

	// Set the global repoDir variable
	repoDir = tmpDir

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Command failed: %v", err)
	}

	// Parse JSON output
	var result ResolveImplData
	output := bytes.TrimSpace(outBuf.Bytes())
	if err := json.Unmarshal(output, &result); err != nil {
		t.Fatalf("Failed to parse JSON output: %v (output: %s)", err, string(output))
	}

	// Verify result
	if result.Slug != testSlug {
		t.Errorf("Expected slug %s, got %s", testSlug, result.Slug)
	}
	if result.ResolutionMethod != "explicit-slug" {
		t.Errorf("Expected resolution_method 'explicit-slug', got %s", result.ResolutionMethod)
	}
	if result.ImplPath != testIMPLPath {
		t.Errorf("Expected impl_path %s, got %s", testIMPLPath, result.ImplPath)
	}
}

// TestResolveImplCmd_ExplicitFilename tests resolution by filename.
func TestResolveImplCmd_ExplicitFilename(t *testing.T) {
	// Create a temporary directory structure
	tmpDir := t.TempDir()
	implDocsDir := filepath.Join(tmpDir, "docs", "IMPL")
	if err := os.MkdirAll(implDocsDir, 0755); err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}

	// Create a test IMPL manifest
	testSlug := "test-filename"
	filename := "IMPL-test-filename.yaml"
	testIMPLPath := filepath.Join(implDocsDir, filename)
	testManifest := &protocol.IMPLManifest{
		FeatureSlug: testSlug,
		Verdict:     "scout-approved",
		State:       protocol.StateWavePending,
		Waves: []protocol.Wave{
			{Number: 1, Agents: []protocol.Agent{{ID: "A"}}},
		},
	}
	if saveRes := protocol.Save(testManifest, testIMPLPath); saveRes.IsFatal() {
		t.Fatalf("Failed to save test IMPL: %v", saveRes.Errors)
	}

	// Run command
	cmd := newResolveImplCmd()
	cmd.SetArgs([]string{"--impl", filename})

	var outBuf bytes.Buffer
	cmd.SetOut(&outBuf)

	// Set the global repoDir variable
	repoDir = tmpDir

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Command failed: %v", err)
	}

	// Parse JSON output
	var result ResolveImplData
	output := bytes.TrimSpace(outBuf.Bytes())
	if err := json.Unmarshal(output, &result); err != nil {
		t.Fatalf("Failed to parse JSON output: %v (output: %s)", err, string(output))
	}

	// Verify result
	if result.Slug != testSlug {
		t.Errorf("Expected slug %s, got %s", testSlug, result.Slug)
	}
	if result.ResolutionMethod != "explicit-filename" {
		t.Errorf("Expected resolution_method 'explicit-filename', got %s", result.ResolutionMethod)
	}
	if result.ImplPath != testIMPLPath {
		t.Errorf("Expected impl_path %s, got %s", testIMPLPath, result.ImplPath)
	}
}

// TestResolveImplCmd_ExplicitAbsolutePath tests resolution by absolute path.
func TestResolveImplCmd_ExplicitAbsolutePath(t *testing.T) {
	// Create a temporary directory structure
	tmpDir := t.TempDir()
	testIMPLPath := filepath.Join(tmpDir, "IMPL-test-abs.yaml")
	testSlug := "test-absolute"
	testManifest := &protocol.IMPLManifest{
		FeatureSlug: testSlug,
		Verdict:     "scout-approved",
		State:       protocol.StateWavePending,
		Waves: []protocol.Wave{
			{Number: 1, Agents: []protocol.Agent{{ID: "A"}}},
		},
	}
	if saveRes := protocol.Save(testManifest, testIMPLPath); saveRes.IsFatal() {
		t.Fatalf("Failed to save test IMPL: %v", saveRes.Errors)
	}

	// Run command
	cmd := newResolveImplCmd()
	cmd.SetArgs([]string{"--impl", testIMPLPath})

	var outBuf bytes.Buffer
	cmd.SetOut(&outBuf)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Command failed: %v", err)
	}

	// Parse JSON output
	var result ResolveImplData
	output := bytes.TrimSpace(outBuf.Bytes())
	if err := json.Unmarshal(output, &result); err != nil {
		t.Fatalf("Failed to parse JSON output: %v (output: %s)", err, string(output))
	}

	// Verify result
	if result.Slug != testSlug {
		t.Errorf("Expected slug %s, got %s", testSlug, result.Slug)
	}
	if result.ResolutionMethod != "explicit-path" {
		t.Errorf("Expected resolution_method 'explicit-path', got %s", result.ResolutionMethod)
	}
	if result.ImplPath != testIMPLPath {
		t.Errorf("Expected impl_path %s, got %s", testIMPLPath, result.ImplPath)
	}
}

// TestResolveImplCmd_AutoSelect tests auto-selection with exactly 1 pending IMPL.
func TestResolveImplCmd_AutoSelect(t *testing.T) {
	// Create a temporary directory structure
	tmpDir := t.TempDir()
	implDocsDir := filepath.Join(tmpDir, "docs", "IMPL")
	if err := os.MkdirAll(implDocsDir, 0755); err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}

	// Create exactly 1 pending IMPL
	testSlug := "only-pending"
	testIMPLPath := filepath.Join(implDocsDir, "IMPL-only-pending.yaml")
	testManifest := &protocol.IMPLManifest{
		FeatureSlug: testSlug,
		Verdict:     "scout-approved",
		State:       protocol.StateWavePending,
		Waves: []protocol.Wave{
			{Number: 1, Agents: []protocol.Agent{{ID: "A"}}},
		},
	}
	if saveRes := protocol.Save(testManifest, testIMPLPath); saveRes.IsFatal() {
		t.Fatalf("Failed to save test IMPL: %v", saveRes.Errors)
	}

	// Run command without --impl flag
	cmd := newResolveImplCmd()
	cmd.SetArgs([]string{})

	var outBuf bytes.Buffer
	cmd.SetOut(&outBuf)

	// Set the global repoDir variable
	repoDir = tmpDir

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Command failed: %v", err)
	}

	// Parse JSON output
	var result ResolveImplData
	output := bytes.TrimSpace(outBuf.Bytes())
	if err := json.Unmarshal(output, &result); err != nil {
		t.Fatalf("Failed to parse JSON output: %v (output: %s)", err, string(output))
	}

	// Verify result
	if result.Slug != testSlug {
		t.Errorf("Expected slug %s, got %s", testSlug, result.Slug)
	}
	if result.ResolutionMethod != "auto-select" {
		t.Errorf("Expected resolution_method 'auto-select', got %s", result.ResolutionMethod)
	}
	if result.PendingCount != 1 {
		t.Errorf("Expected pending_count 1, got %d", result.PendingCount)
	}
}

// TestResolveImplCmd_AutoSelectMultipleFails tests auto-selection failure with multiple pending IMPLs.
func TestResolveImplCmd_AutoSelectMultipleFails(t *testing.T) {
	// Create a temporary directory structure
	tmpDir := t.TempDir()
	implDocsDir := filepath.Join(tmpDir, "docs", "IMPL")
	if err := os.MkdirAll(implDocsDir, 0755); err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}

	// Create 2 pending IMPLs
	for i := 1; i <= 2; i++ {
		testIMPLPath := filepath.Join(implDocsDir, "IMPL-pending-"+string(rune('A'+i-1))+".yaml")
		testManifest := &protocol.IMPLManifest{
			FeatureSlug: "pending-" + string(rune('A'+i-1)),
			Verdict:     "scout-approved",
			State:       protocol.StateWavePending,
			Waves: []protocol.Wave{
				{Number: 1, Agents: []protocol.Agent{{ID: "A"}}},
			},
		}
		if saveRes := protocol.Save(testManifest, testIMPLPath); saveRes.IsFatal() {
			t.Fatalf("Failed to save test IMPL: %v", saveRes.Errors)
		}
	}

	// Run command without --impl flag (should fail)
	cmd := newResolveImplCmd()
	cmd.SetArgs([]string{})

	var outBuf bytes.Buffer
	cmd.SetOut(&outBuf)

	// Set the global repoDir variable
	repoDir = tmpDir

	err := cmd.Execute()
	if err == nil {
		t.Fatal("Expected command to fail with multiple pending IMPLs, but it succeeded")
	}

	// Verify error message mentions multiple IMPLs
	errMsg := err.Error()
	if errMsg == "" {
		t.Fatal("Expected error message, got empty string")
	}
	// Error message should mention "multiple pending IMPLs"
	if !contains(errMsg, "multiple pending IMPLs") {
		t.Errorf("Expected error about multiple pending IMPLs, got: %s", errMsg)
	}
}

// TestResolveImplCmd_AutoSelectNoneFails tests auto-selection failure with no pending IMPLs.
func TestResolveImplCmd_AutoSelectNoneFails(t *testing.T) {
	// Create a temporary directory structure
	tmpDir := t.TempDir()
	implDocsDir := filepath.Join(tmpDir, "docs", "IMPL")
	if err := os.MkdirAll(implDocsDir, 0755); err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}

	// Don't create any IMPL files

	// Run command without --impl flag (should fail)
	cmd := newResolveImplCmd()
	cmd.SetArgs([]string{})

	var outBuf bytes.Buffer
	cmd.SetOut(&outBuf)

	// Set the global repoDir variable
	repoDir = tmpDir

	err := cmd.Execute()
	if err == nil {
		t.Fatal("Expected command to fail with no pending IMPLs, but it succeeded")
	}

	// Verify error message mentions no pending IMPLs
	errMsg := err.Error()
	if errMsg == "" {
		t.Fatal("Expected error message, got empty string")
	}
	// Error message should mention "no pending IMPLs"
	if !contains(errMsg, "no pending IMPLs") {
		t.Errorf("Expected error about no pending IMPLs, got: %s", errMsg)
	}
}

// TestResolveImplCmd_InvalidSlug tests resolution failure with non-existent slug.
func TestResolveImplCmd_InvalidSlug(t *testing.T) {
	// Create a temporary directory structure
	tmpDir := t.TempDir()
	implDocsDir := filepath.Join(tmpDir, "docs", "IMPL")
	if err := os.MkdirAll(implDocsDir, 0755); err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}

	// Create a test IMPL with different slug
	testIMPLPath := filepath.Join(implDocsDir, "IMPL-other.yaml")
	testManifest := &protocol.IMPLManifest{
		FeatureSlug: "other-slug",
		Verdict:     "scout-approved",
		State:       protocol.StateWavePending,
		Waves: []protocol.Wave{
			{Number: 1, Agents: []protocol.Agent{{ID: "A"}}},
		},
	}
	if saveRes := protocol.Save(testManifest, testIMPLPath); saveRes.IsFatal() {
		t.Fatalf("Failed to save test IMPL: %v", saveRes.Errors)
	}

	// Run command with non-existent slug
	cmd := newResolveImplCmd()
	cmd.SetArgs([]string{"--impl", "nonexistent-slug"})

	var outBuf bytes.Buffer
	cmd.SetOut(&outBuf)

	// Set the global repoDir variable
	repoDir = tmpDir

	err := cmd.Execute()
	if err == nil {
		t.Fatal("Expected command to fail with invalid slug, but it succeeded")
	}

	// Verify error message mentions no match
	errMsg := err.Error()
	if errMsg == "" {
		t.Fatal("Expected error message, got empty string")
	}
	// Error message should mention "no pending IMPL found"
	if !contains(errMsg, "no pending IMPL found") {
		t.Errorf("Expected error about no match, got: %s", errMsg)
	}
}

// Helper function to check if string contains substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
