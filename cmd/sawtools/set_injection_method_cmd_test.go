package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/protocol"
	"gopkg.in/yaml.v3"
)

// writeInjectionMethodTestManifest writes a minimal IMPL manifest to a temp dir.
func writeInjectionMethodTestManifest(t *testing.T) string {
	t.Helper()
	m := &protocol.IMPLManifest{
		Title:       "Test Feature",
		FeatureSlug: "test-feature",
		Verdict:     "SUITABLE",
		TestCommand: "go test ./...",
		LintCommand: "go vet ./...",
		State:       protocol.StateWaveExecuting,
		FileOwnership: []protocol.FileOwnership{
			{File: "pkg/foo.go", Agent: "A", Wave: 1, Action: "new"},
		},
		Waves: []protocol.Wave{
			{
				Number: 1,
				Agents: []protocol.Agent{
					{ID: "A", Task: "Implement foo", Files: []string{"pkg/foo.go"}},
				},
			},
		},
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "IMPL-test.yaml")
	data, err := yaml.Marshal(m)
	if err != nil {
		t.Fatalf("yaml.Marshal: %v", err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	return path
}

// TestSetInjectionMethod_Hook verifies that --method hook saves correctly and reloads.
func TestSetInjectionMethod_Hook(t *testing.T) {
	manifestPath := writeInjectionMethodTestManifest(t)

	cmd := newSetInjectionMethodCmd()
	cmd.SetArgs([]string{manifestPath, "--method", "hook"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	doc, err := protocol.Load(manifestPath)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if doc.InjectionMethod != protocol.InjectionMethodHook {
		t.Errorf("InjectionMethod = %q; want %q", doc.InjectionMethod, protocol.InjectionMethodHook)
	}
}

// TestSetInjectionMethod_ManualFallback verifies that --method manual-fallback saves correctly.
func TestSetInjectionMethod_ManualFallback(t *testing.T) {
	manifestPath := writeInjectionMethodTestManifest(t)

	cmd := newSetInjectionMethodCmd()
	cmd.SetArgs([]string{manifestPath, "--method", "manual-fallback"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	doc, err := protocol.Load(manifestPath)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if doc.InjectionMethod != protocol.InjectionMethodManualFallback {
		t.Errorf("InjectionMethod = %q; want %q", doc.InjectionMethod, protocol.InjectionMethodManualFallback)
	}
}

// TestSetInjectionMethod_Unknown verifies that --method unknown saves correctly.
func TestSetInjectionMethod_Unknown(t *testing.T) {
	manifestPath := writeInjectionMethodTestManifest(t)

	cmd := newSetInjectionMethodCmd()
	cmd.SetArgs([]string{manifestPath, "--method", "unknown"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	doc, err := protocol.Load(manifestPath)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if doc.InjectionMethod != protocol.InjectionMethodUnknown {
		t.Errorf("InjectionMethod = %q; want %q", doc.InjectionMethod, protocol.InjectionMethodUnknown)
	}
}

// TestSetInjectionMethod_InvalidMethod verifies that an invalid --method returns a non-nil error.
func TestSetInjectionMethod_InvalidMethod(t *testing.T) {
	manifestPath := writeInjectionMethodTestManifest(t)

	cmd := newSetInjectionMethodCmd()
	cmd.SetArgs([]string{manifestPath, "--method", "invalid-value"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for invalid --method, got nil")
	}
}

// TestSetInjectionMethod_MissingMethod verifies that omitting --method returns an error.
func TestSetInjectionMethod_MissingMethod(t *testing.T) {
	manifestPath := writeInjectionMethodTestManifest(t)

	cmd := newSetInjectionMethodCmd()
	cmd.SetArgs([]string{manifestPath})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for missing --method flag, got nil")
	}
}
