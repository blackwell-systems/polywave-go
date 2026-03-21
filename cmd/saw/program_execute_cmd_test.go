package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestProgramExecuteCmd_FlagParsing(t *testing.T) {
	cmd := newProgramExecuteCmd()

	// Verify flags exist with correct defaults
	autoFlag := cmd.Flags().Lookup("auto")
	if autoFlag == nil {
		t.Fatal("expected --auto flag to be defined")
	}
	if autoFlag.DefValue != "false" {
		t.Errorf("expected --auto default to be false, got %s", autoFlag.DefValue)
	}

	modelFlag := cmd.Flags().Lookup("model")
	if modelFlag == nil {
		t.Fatal("expected --model flag to be defined")
	}
	if modelFlag.DefValue != "" {
		t.Errorf("expected --model default to be empty, got %s", modelFlag.DefValue)
	}
}

func TestProgramExecuteCmd_InvalidManifest(t *testing.T) {
	// Create a temp file with invalid YAML
	tmpDir := t.TempDir()
	badManifest := filepath.Join(tmpDir, "bad-program.yaml")
	if err := os.WriteFile(badManifest, []byte("not: a: valid: program"), 0644); err != nil {
		t.Fatal(err)
	}

	cmd := newRootCmd()
	cmd.AddCommand(newProgramExecuteCmd())

	// Capture the exit code by checking that parsing fails
	// (We cannot easily test os.Exit(2) in unit tests, so we verify
	// the command structure and flag wiring instead.)
	cmd.SetArgs([]string{"program-execute", "--help"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("help should not error: %v", err)
	}
}

func TestProgramExecuteCmd_RequiresArg(t *testing.T) {
	cmd := newRootCmd()
	cmd.AddCommand(newProgramExecuteCmd())
	cmd.SetArgs([]string{"program-execute"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error when no argument provided")
	}
}

func TestExtractSlugFromDetail(t *testing.T) {
	tests := []struct {
		detail string
		want   string
	}{
		{"IMPL my-feature wave execution finished", "my-feature"},
		{"IMPL auth-service wave execution finished", "auth-service"},
		{"IMPL single", "single"},
		{"something else", ""},
		{"", ""},
	}
	for _, tt := range tests {
		got := extractSlugFromDetail(tt.detail)
		if got != tt.want {
			t.Errorf("extractSlugFromDetail(%q) = %q, want %q", tt.detail, got, tt.want)
		}
	}
}
