package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestLoad_NoFile(t *testing.T) {
	dir := t.TempDir()
	r := Load(dir)
	if r.IsSuccess() {
		t.Fatal("expected failure when no config file exists")
	}
	if len(r.Errors) == 0 {
		t.Fatal("expected at least one error")
	}
	if r.Errors[0].Code != "N013_CONFIG_NOT_FOUND" {
		t.Fatalf("expected N013_CONFIG_NOT_FOUND, got %s", r.Errors[0].Code)
	}
}

func TestLoad_ValidFile(t *testing.T) {
	dir := t.TempDir()
	cfg := map[string]any{
		"providers": map[string]any{
			"anthropic": map[string]any{"api_key": "sk-test"},
			"bedrock":   map[string]any{"region": "us-west-2", "profile": "prod"},
			"openai":    map[string]any{"api_key": "oai-test"},
		},
		"autonomy": map[string]any{
			"level":            "full",
			"max_auto_retries": 5,
			"max_queue_depth":  20,
		},
		"repos": []map[string]any{
			{"name": "repo1", "path": "/tmp/repo1"},
			{"name": "repo2", "path": "/tmp/repo2"},
		},
		"agent": map[string]any{
			"scout_model": "opus",
			"wave_model":  "sonnet",
		},
		"quality": map[string]any{
			"require_tests":   true,
			"block_on_failure": true,
			"code_review": map[string]any{
				"enabled":   true,
				"blocking":  true,
				"model":     "haiku",
				"threshold": 80,
			},
		},
		"appearance": map[string]any{
			"theme":    "dark",
			"contrast": "high",
		},
		"notifications": map[string]any{"slack": true},
		"code_review":   map[string]any{"auto": true},
	}

	data, _ := json.Marshal(cfg)
	if err := os.WriteFile(filepath.Join(dir, configFileName), data, 0644); err != nil {
		t.Fatal(err)
	}

	r := Load(dir)
	if !r.IsSuccess() {
		t.Fatalf("expected success, got errors: %v", r.Errors)
	}
	c := r.GetData()

	// Providers
	if c.Providers.Anthropic.APIKey != "sk-test" {
		t.Errorf("anthropic api_key = %q, want %q", c.Providers.Anthropic.APIKey, "sk-test")
	}
	if c.Providers.Bedrock.Region != "us-west-2" {
		t.Errorf("bedrock region = %q, want %q", c.Providers.Bedrock.Region, "us-west-2")
	}
	if c.Providers.OpenAI.APIKey != "oai-test" {
		t.Errorf("openai api_key = %q, want %q", c.Providers.OpenAI.APIKey, "oai-test")
	}

	// Autonomy
	if c.Autonomy.Level != "full" {
		t.Errorf("autonomy level = %q, want %q", c.Autonomy.Level, "full")
	}
	if c.Autonomy.MaxAutoRetries != 5 {
		t.Errorf("max_auto_retries = %d, want 5", c.Autonomy.MaxAutoRetries)
	}

	// Repos
	if len(c.Repos) != 2 {
		t.Fatalf("repos len = %d, want 2", len(c.Repos))
	}
	if c.Repos[0].Name != "repo1" {
		t.Errorf("repos[0].name = %q, want %q", c.Repos[0].Name, "repo1")
	}

	// Agent
	if c.Agent.ScoutModel != "opus" {
		t.Errorf("scout_model = %q, want %q", c.Agent.ScoutModel, "opus")
	}

	// Quality
	if !c.Quality.RequireTests {
		t.Error("require_tests should be true")
	}
	if c.Quality.CodeReview.Threshold != 80 {
		t.Errorf("code_review threshold = %d, want 80", c.Quality.CodeReview.Threshold)
	}

	// Appearance
	if c.Appear.Theme != "dark" {
		t.Errorf("theme = %q, want %q", c.Appear.Theme, "dark")
	}
}

func TestLoad_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, configFileName), []byte("{invalid"), 0644); err != nil {
		t.Fatal(err)
	}

	r := Load(dir)
	if r.IsSuccess() {
		t.Fatal("expected failure for invalid JSON")
	}
	if r.Errors[0].Code != "N014_CONFIG_INVALID" {
		t.Fatalf("expected N014_CONFIG_INVALID, got %s", r.Errors[0].Code)
	}
}

func TestLoad_BackwardCompat(t *testing.T) {
	dir := t.TempDir()
	// Legacy format: single "repo" object instead of "repos" array.
	cfg := map[string]any{
		"repo": map[string]any{
			"path": "/tmp/legacy-repo",
		},
	}
	data, _ := json.Marshal(cfg)
	if err := os.WriteFile(filepath.Join(dir, configFileName), data, 0644); err != nil {
		t.Fatal(err)
	}

	r := Load(dir)
	if !r.IsSuccess() {
		t.Fatalf("expected success, got errors: %v", r.Errors)
	}
	c := r.GetData()
	if len(c.Repos) != 1 {
		t.Fatalf("repos len = %d, want 1", len(c.Repos))
	}
	if c.Repos[0].Path != "/tmp/legacy-repo" {
		t.Errorf("repos[0].path = %q, want %q", c.Repos[0].Path, "/tmp/legacy-repo")
	}
	if c.Repos[0].Name != "legacy-repo" {
		t.Errorf("repos[0].name = %q, want %q (derived from path base)", c.Repos[0].Name, "legacy-repo")
	}
}

func TestSave_AtomicWrite(t *testing.T) {
	dir := t.TempDir()
	cfg := &SAWConfig{
		Providers: ProvidersConfig{
			Anthropic: AnthropicProvider{APIKey: "test-key"},
		},
		Autonomy: AutonomyConfig{Level: "gated"},
	}

	r := Save(dir, cfg)
	if !r.IsSuccess() {
		t.Fatalf("save failed: %v", r.Errors)
	}

	// Verify the file was written correctly.
	data, err := os.ReadFile(filepath.Join(dir, configFileName))
	if err != nil {
		t.Fatal(err)
	}

	var loaded SAWConfig
	if err := json.Unmarshal(data, &loaded); err != nil {
		t.Fatalf("cannot parse saved file: %v", err)
	}
	if loaded.Providers.Anthropic.APIKey != "test-key" {
		t.Errorf("api_key = %q, want %q", loaded.Providers.Anthropic.APIKey, "test-key")
	}

	// Verify no temp files remain.
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if e.Name() != configFileName {
			t.Errorf("unexpected file remaining: %s", e.Name())
		}
	}
}

func TestSave_PreservesUnknownKeys(t *testing.T) {
	dir := t.TempDir()

	// Write initial config with an unknown key.
	initial := map[string]any{
		"custom_plugin": map[string]any{"enabled": true},
		"providers":     map[string]any{"anthropic": map[string]any{"api_key": "old"}},
	}
	data, _ := json.Marshal(initial)
	if err := os.WriteFile(filepath.Join(dir, configFileName), data, 0644); err != nil {
		t.Fatal(err)
	}

	// Save with updated providers.
	cfg := &SAWConfig{
		Providers: ProvidersConfig{
			Anthropic: AnthropicProvider{APIKey: "new"},
		},
	}
	r := Save(dir, cfg)
	if !r.IsSuccess() {
		t.Fatalf("save failed: %v", r.Errors)
	}

	// Verify custom_plugin key was preserved.
	saved, _ := os.ReadFile(filepath.Join(dir, configFileName))
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(saved, &raw); err != nil {
		t.Fatal(err)
	}
	if _, ok := raw["custom_plugin"]; !ok {
		t.Fatal("custom_plugin key was not preserved")
	}

	// Verify the provider was updated.
	var loaded SAWConfig
	if err := json.Unmarshal(saved, &loaded); err != nil {
		t.Fatal(err)
	}
	if loaded.Providers.Anthropic.APIKey != "new" {
		t.Errorf("api_key = %q, want %q", loaded.Providers.Anthropic.APIKey, "new")
	}
}

func TestFindConfigPath_WalksUp(t *testing.T) {
	// Create a directory tree: root/a/b/c
	root := t.TempDir()
	deep := filepath.Join(root, "a", "b", "c")
	if err := os.MkdirAll(deep, 0755); err != nil {
		t.Fatal(err)
	}

	// Place config at root.
	configPath := filepath.Join(root, configFileName)
	if err := os.WriteFile(configPath, []byte("{}"), 0644); err != nil {
		t.Fatal(err)
	}

	found := FindConfigPath(deep)
	if found == "" {
		t.Fatal("expected to find config file")
	}

	abs, _ := filepath.Abs(configPath)
	if found != abs {
		t.Errorf("found = %q, want %q", found, abs)
	}
}

func TestFindConfigPath_NotFound(t *testing.T) {
	dir := t.TempDir()
	found := FindConfigPath(dir)
	if found != "" {
		t.Errorf("expected empty string, got %q", found)
	}
}

func TestLoadOrDefault_FallsBack(t *testing.T) {
	dir := t.TempDir() // no config file
	cfg := LoadOrDefault(dir)

	if cfg.Autonomy.Level != "gated" {
		t.Errorf("level = %q, want %q", cfg.Autonomy.Level, "gated")
	}
	if cfg.Autonomy.MaxAutoRetries != 2 {
		t.Errorf("max_auto_retries = %d, want 2", cfg.Autonomy.MaxAutoRetries)
	}
	if cfg.Autonomy.MaxQueueDepth != 10 {
		t.Errorf("max_queue_depth = %d, want 10", cfg.Autonomy.MaxQueueDepth)
	}
}

func TestLoadOrDefault_LoadsWhenExists(t *testing.T) {
	dir := t.TempDir()
	cfg := map[string]any{
		"autonomy": map[string]any{"level": "full"},
	}
	data, _ := json.Marshal(cfg)
	if err := os.WriteFile(filepath.Join(dir, configFileName), data, 0644); err != nil {
		t.Fatal(err)
	}

	loaded := LoadOrDefault(dir)
	if loaded.Autonomy.Level != "full" {
		t.Errorf("level = %q, want %q", loaded.Autonomy.Level, "full")
	}
}

// TestSave_FilePermissions verifies that Save() writes saw.config.json with
// 0600 permissions, ensuring API keys are never world-readable.
func TestSave_FilePermissions(t *testing.T) {
	dir := t.TempDir()
	cfg := &SAWConfig{
		Providers: ProvidersConfig{
			Anthropic: AnthropicProvider{APIKey: "secret-key"},
		},
	}

	r := Save(dir, cfg)
	if !r.IsSuccess() {
		t.Fatalf("save failed: %v", r.Errors)
	}

	info, err := os.Stat(filepath.Join(dir, configFileName))
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); perm != 0600 {
		t.Errorf("file permissions = %04o, want 0600", perm)
	}
}

// TestLoad_LegacyMigrationMalformedRepo verifies that a config file with a
// "repo" key that is not an object (e.g. a string) loads successfully with
// empty Repos — migration is skipped gracefully.
func TestLoad_LegacyMigrationMalformedRepo(t *testing.T) {
	dir := t.TempDir()
	// repo is a string, not an object — malformed for migration purposes.
	raw := []byte(`{"repo": "not-an-object"}`)
	if err := os.WriteFile(filepath.Join(dir, configFileName), raw, 0644); err != nil {
		t.Fatal(err)
	}

	r := Load(dir)
	if !r.IsSuccess() {
		t.Fatalf("expected success (migration should be skipped gracefully), got: %v", r.Errors)
	}
	c := r.GetData()
	if len(c.Repos) != 0 {
		t.Errorf("repos len = %d, want 0 (migration skipped)", len(c.Repos))
	}
}

// TestLoadOrDefault_DefaultValues confirms that LoadOrDefault returns the
// expected default values when no config file exists, validating behavior
// after the slog.Warn call was added.
// (The existing TestLoadOrDefault_FallsBack also covers this; this test
// makes the intent explicit after Issue 8 changes.)
func TestLoadOrDefault_DefaultValues(t *testing.T) {
	dir := t.TempDir() // no config file
	cfg := LoadOrDefault(dir)

	if cfg == nil {
		t.Fatal("expected non-nil config")
	}
	if cfg.Autonomy.Level != "gated" {
		t.Errorf("level = %q, want \"gated\"", cfg.Autonomy.Level)
	}
	if cfg.Autonomy.MaxAutoRetries != 2 {
		t.Errorf("max_auto_retries = %d, want 2", cfg.Autonomy.MaxAutoRetries)
	}
	if cfg.Autonomy.MaxQueueDepth != 10 {
		t.Errorf("max_queue_depth = %d, want 10", cfg.Autonomy.MaxQueueDepth)
	}
}
