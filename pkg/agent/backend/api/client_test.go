package api

import (
	"testing"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/agent/backend"
)

// TestNew_ExplicitAPIKey verifies that New() with an explicit apiKey stores it
// and does not fall back to the environment variable.
func TestNew_ExplicitAPIKey(t *testing.T) {
	key := "sk-test-explicit-key"
	client := New(key, backend.Config{})
	if client.apiKey != key {
		t.Errorf("expected apiKey %q, got %q", key, client.apiKey)
	}
}

// TestNew_DefaultModel verifies that New() defaults to claude-sonnet-4-5 when
// cfg.Model is empty.
func TestNew_DefaultModel(t *testing.T) {
	client := New("test-key", backend.Config{})
	if client.model != defaultModel {
		t.Errorf("expected model %q, got %q", defaultModel, client.model)
	}
}

// TestNew_CustomModel verifies that New() uses cfg.Model when provided.
func TestNew_CustomModel(t *testing.T) {
	client := New("test-key", backend.Config{Model: "claude-opus-4-6"})
	if client.model != "claude-opus-4-6" {
		t.Errorf("expected model %q, got %q", "claude-opus-4-6", client.model)
	}
}

// TestNew_DefaultMaxTokens verifies that New() defaults MaxTokens to 8096.
func TestNew_DefaultMaxTokens(t *testing.T) {
	client := New("test-key", backend.Config{})
	if client.maxTokens != defaultMaxTokens {
		t.Errorf("expected maxTokens %d, got %d", defaultMaxTokens, client.maxTokens)
	}
}

// TestNew_DefaultMaxTurns verifies that New() defaults MaxTurns to 50.
func TestNew_DefaultMaxTurns(t *testing.T) {
	client := New("test-key", backend.Config{})
	if client.maxTurns != defaultMaxTurns {
		t.Errorf("expected maxTurns %d, got %d", defaultMaxTurns, client.maxTurns)
	}
}
