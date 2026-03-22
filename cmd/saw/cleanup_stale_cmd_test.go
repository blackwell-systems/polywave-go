package main

import (
	"strings"
	"testing"
)

func TestNewCleanupStaleCmd_Construction(t *testing.T) {
	cmd := newCleanupStaleCmd()

	if cmd.Use != "cleanup-stale" {
		t.Errorf("expected Use = %q, got %q", "cleanup-stale", cmd.Use)
	}

	if cmd.Short == "" {
		t.Error("expected Short description to be set")
	}

	if cmd.RunE == nil {
		t.Error("expected RunE to be set")
	}
}

func TestNewCleanupStaleCmd_Flags(t *testing.T) {
	cmd := newCleanupStaleCmd()

	tests := []struct {
		name         string
		defaultValue string
	}{
		{"dry-run", "false"},
		{"force", "false"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := cmd.Flags().Lookup(tt.name)
			if f == nil {
				t.Fatalf("flag %q not found", tt.name)
			}
			if f.DefValue != tt.defaultValue {
				t.Errorf("flag %q default = %q, want %q", tt.name, f.DefValue, tt.defaultValue)
			}
		})
	}
}

func TestNewCleanupStaleCmd_DryRunFlag(t *testing.T) {
	cmd := newCleanupStaleCmd()

	// Verify dry-run flag can be set
	err := cmd.Flags().Set("dry-run", "true")
	if err != nil {
		t.Fatalf("failed to set dry-run flag: %v", err)
	}

	f := cmd.Flags().Lookup("dry-run")
	if f.Value.String() != "true" {
		t.Errorf("dry-run flag value = %q, want %q", f.Value.String(), "true")
	}
}

func TestNewCleanupStaleCmd_ForceFlag(t *testing.T) {
	cmd := newCleanupStaleCmd()

	err := cmd.Flags().Set("force", "true")
	if err != nil {
		t.Fatalf("failed to set force flag: %v", err)
	}

	f := cmd.Flags().Lookup("force")
	if f.Value.String() != "true" {
		t.Errorf("force flag value = %q, want %q", f.Value.String(), "true")
	}
}

func TestNewCleanupStaleCmd_SlugAndAllFlags(t *testing.T) {
	cmd := newCleanupStaleCmd()

	slugFlag := cmd.Flags().Lookup("slug")
	if slugFlag == nil {
		t.Fatal("flag --slug not found")
	}
	if slugFlag.DefValue != "" {
		t.Errorf("--slug default = %q, want empty string", slugFlag.DefValue)
	}

	allFlag := cmd.Flags().Lookup("all")
	if allFlag == nil {
		t.Fatal("flag --all not found")
	}
	if allFlag.DefValue != "false" {
		t.Errorf("--all default = %q, want %q", allFlag.DefValue, "false")
	}
}

func TestCleanupStaleCmd_SlugAndAllMutuallyExclusive(t *testing.T) {
	cmd := newCleanupStaleCmd()
	cmd.SetArgs([]string{"--slug", "my-feature", "--all"})
	// Execute and expect an error about mutual exclusion.
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error when both --slug and --all are set, got nil")
	}
	if !strings.Contains(err.Error(), "mutually exclusive") {
		t.Errorf("expected 'mutually exclusive' in error, got: %v", err)
	}
}

func TestCleanupStaleCmd_NeitherSlugNorAll(t *testing.T) {
	cmd := newCleanupStaleCmd()
	cmd.SetArgs([]string{})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error when neither --slug nor --all is set, got nil")
	}
	if !strings.Contains(err.Error(), "specify --slug") {
		t.Errorf("expected 'specify --slug' in error, got: %v", err)
	}
}

func TestCleanupStaleCmd_SlugDryRun(t *testing.T) {
	// --slug my-feature --dry-run should not error on flag parsing/validation.
	// It will run against the actual repo, finding 0 stale worktrees for this slug.
	cmd := newCleanupStaleCmd()
	cmd.SetArgs([]string{"--slug", "my-feature", "--dry-run"})
	// We expect either success or a git error (acceptable in test environment).
	// The key is that flag validation passes (no "mutually exclusive" or "specify --slug" error).
	err := cmd.Execute()
	if err != nil {
		if strings.Contains(err.Error(), "mutually exclusive") || strings.Contains(err.Error(), "specify --slug") {
			t.Errorf("unexpected flag validation error: %v", err)
		}
		// Other errors (git, I/O) are acceptable in test environment.
		t.Logf("acceptable non-flag error: %v", err)
	}
}

func TestCleanupStaleCmd_AllDryRun(t *testing.T) {
	// --all --dry-run should not error on flag parsing/validation.
	cmd := newCleanupStaleCmd()
	cmd.SetArgs([]string{"--all", "--dry-run"})
	err := cmd.Execute()
	if err != nil {
		if strings.Contains(err.Error(), "mutually exclusive") || strings.Contains(err.Error(), "specify --slug") {
			t.Errorf("unexpected flag validation error: %v", err)
		}
		t.Logf("acceptable non-flag error: %v", err)
	}
}
