package backend

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// writeConfig writes a minimal saw.config.json with the given anthropic API key
// into the specified directory.
func writeConfig(t *testing.T, dir, apiKey string) {
	t.Helper()
	cfg := map[string]any{
		"providers": map[string]any{
			"anthropic": map[string]any{
				"api_key": apiKey,
			},
		},
	}
	data, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("failed to marshal test config: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "saw.config.json"), data, 0600); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}
}

// TestLoadProvidersFromConfig_NotFound verifies that a missing config file
// returns a zero-value SAWProviders struct.
func TestLoadProvidersFromConfig_NotFound(t *testing.T) {
	dir := t.TempDir()
	got := LoadProvidersFromConfig(dir)
	if got.Anthropic.APIKey != "" {
		t.Errorf("expected empty APIKey, got %q", got.Anthropic.APIKey)
	}
	if got.OpenAI.APIKey != "" {
		t.Errorf("expected empty OpenAI APIKey, got %q", got.OpenAI.APIKey)
	}
}

// TestLoadProvidersFromConfig_Found verifies that a config file in the given
// directory is found and parsed correctly.
func TestLoadProvidersFromConfig_Found(t *testing.T) {
	dir := t.TempDir()
	const wantKey = "sk-test-api-key-12345"
	writeConfig(t, dir, wantKey)

	got := LoadProvidersFromConfig(dir)
	if got.Anthropic.APIKey != wantKey {
		t.Errorf("expected APIKey %q, got %q", wantKey, got.Anthropic.APIKey)
	}
}

// TestLoadProvidersFromConfig_WalksUp verifies that LoadProvidersFromConfig
// walks up parent directories to find saw.config.json.
func TestLoadProvidersFromConfig_WalksUp(t *testing.T) {
	parentDir := t.TempDir()
	subDir := filepath.Join(parentDir, "sub")
	if err := os.Mkdir(subDir, 0755); err != nil {
		t.Fatalf("failed to create subdir: %v", err)
	}

	const wantKey = "sk-parent-key-99999"
	writeConfig(t, parentDir, wantKey)

	// Call with the subdirectory; config only exists in parent.
	got := LoadProvidersFromConfig(subDir)
	if got.Anthropic.APIKey != wantKey {
		t.Errorf("expected APIKey %q from parent dir walk-up, got %q", wantKey, got.Anthropic.APIKey)
	}
}
