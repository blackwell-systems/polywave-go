package main

import (
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
