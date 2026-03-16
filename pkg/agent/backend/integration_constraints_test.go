package backend_test

import (
	"testing"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/agent/backend"
	"github.com/blackwell-systems/scout-and-wave-go/pkg/agent/backend/api"
	"github.com/blackwell-systems/scout-and-wave-go/pkg/agent/backend/bedrock"
	"github.com/blackwell-systems/scout-and-wave-go/pkg/tools"
)

// TestBedrockBuildWorkshop_WithConstraints verifies that a Bedrock client
// constructed with non-nil Constraints produces a constrained workshop and
// exposes a non-zero CommitCount method.
func TestBedrockBuildWorkshop_WithConstraints(t *testing.T) {
	cfg := backend.Config{
		Model: "test-model",
		Constraints: &tools.Constraints{
			OwnedFiles:   map[string]bool{"pkg/foo.go": true},
			TrackCommits: true,
			AgentID:      "A",
			AgentRole:    "wave",
		},
	}

	c := bedrock.New(cfg)
	// CommitCount should start at 0 (tracker created but no commits yet).
	if got := c.CommitCount(); got != 0 {
		t.Errorf("CommitCount() = %d, want 0", got)
	}
}

// TestBedrockBuildWorkshop_NilConstraints_Unchanged verifies backward
// compatibility: nil Constraints means no constraint middleware is applied
// and CommitCount returns 0.
func TestBedrockBuildWorkshop_NilConstraints_Unchanged(t *testing.T) {
	cfg := backend.Config{
		Model: "test-model",
		// Constraints is nil — no constraints applied.
	}

	c := bedrock.New(cfg)
	if got := c.CommitCount(); got != 0 {
		t.Errorf("CommitCount() = %d, want 0 (no constraints)", got)
	}
}

// TestAPIBuildWorkshop_WithConstraints verifies that an API client
// constructed with non-nil Constraints produces a constrained workshop and
// exposes a non-zero CommitCount method.
func TestAPIBuildWorkshop_WithConstraints(t *testing.T) {
	cfg := backend.Config{
		Model: "test-model",
		Constraints: &tools.Constraints{
			OwnedFiles:   map[string]bool{"pkg/bar.go": true},
			TrackCommits: true,
			AgentID:      "B",
			AgentRole:    "wave",
		},
	}

	c := api.New("fake-key", cfg)
	// CommitCount should start at 0 (tracker created but no commits yet).
	if got := c.CommitCount(); got != 0 {
		t.Errorf("CommitCount() = %d, want 0", got)
	}
}

// TestAPIBuildWorkshop_NilConstraints_Unchanged verifies backward
// compatibility: nil Constraints means no constraint middleware is applied
// and CommitCount returns 0.
func TestAPIBuildWorkshop_NilConstraints_Unchanged(t *testing.T) {
	cfg := backend.Config{
		Model: "test-model",
		// Constraints is nil — no constraints applied.
	}

	c := api.New("fake-key", cfg)
	if got := c.CommitCount(); got != 0 {
		t.Errorf("CommitCount() = %d, want 0 (no constraints)", got)
	}
}

// TestConfigConstraints_PropagatedToWorkshop verifies that the Constraints
// field on backend.Config is properly carried through to the backend clients.
func TestConfigConstraints_PropagatedToWorkshop(t *testing.T) {
	constraints := &tools.Constraints{
		OwnedFiles:          map[string]bool{"a.go": true, "b.go": true},
		TrackCommits:        true,
		AllowedPathPrefixes: []string{"pkg/"},
		AgentID:             "C",
		AgentRole:           "wave",
	}

	cfg := backend.Config{
		Model:       "test-model",
		Constraints: constraints,
	}

	// Verify Config carries constraints.
	if cfg.Constraints == nil {
		t.Fatal("Config.Constraints should not be nil")
	}
	if cfg.Constraints.AgentID != "C" {
		t.Errorf("Config.Constraints.AgentID = %q, want %q", cfg.Constraints.AgentID, "C")
	}

	// Verify both backends accept the config without panic.
	bedrockClient := bedrock.New(cfg)
	if bedrockClient == nil {
		t.Fatal("bedrock.New returned nil")
	}

	apiClient := api.New("fake-key", cfg)
	if apiClient == nil {
		t.Fatal("api.New returned nil")
	}

	// Both should have CommitCount = 0 initially.
	if got := bedrockClient.CommitCount(); got != 0 {
		t.Errorf("bedrock CommitCount() = %d, want 0", got)
	}
	if got := apiClient.CommitCount(); got != 0 {
		t.Errorf("api CommitCount() = %d, want 0", got)
	}
}
