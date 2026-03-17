package autonomy

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfig_NoFile(t *testing.T) {
	dir := t.TempDir()
	cfg, err := LoadConfig(dir)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	def := DefaultConfig()
	if cfg != def {
		t.Errorf("expected DefaultConfig %+v, got %+v", def, cfg)
	}
}

func TestLoadConfig_ValidFile(t *testing.T) {
	dir := t.TempDir()
	content := `{
		"autonomy": {
			"level": "supervised",
			"max_auto_retries": 5,
			"max_queue_depth": 20
		}
	}`
	if err := os.WriteFile(filepath.Join(dir, "saw.config.json"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadConfig(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Level != LevelSupervised {
		t.Errorf("expected level %q, got %q", LevelSupervised, cfg.Level)
	}
	if cfg.MaxAutoRetries != 5 {
		t.Errorf("expected max_auto_retries 5, got %d", cfg.MaxAutoRetries)
	}
	if cfg.MaxQueueDepth != 20 {
		t.Errorf("expected max_queue_depth 20, got %d", cfg.MaxQueueDepth)
	}
}

func TestLoadConfig_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "saw.config.json"), []byte("{not valid json"), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := LoadConfig(dir)
	if err == nil {
		t.Fatal("expected error for invalid JSON, got nil")
	}
}

func TestLoadConfig_MissingAutonomySection(t *testing.T) {
	dir := t.TempDir()
	content := `{"other_section": {"foo": "bar"}}`
	if err := os.WriteFile(filepath.Join(dir, "saw.config.json"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadConfig(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	def := DefaultConfig()
	if cfg != def {
		t.Errorf("expected DefaultConfig %+v, got %+v", def, cfg)
	}
}

func TestSaveConfig_NewFile(t *testing.T) {
	dir := t.TempDir()
	cfg := Config{
		Level:          LevelAutonomous,
		MaxAutoRetries: 3,
		MaxQueueDepth:  15,
	}

	if err := SaveConfig(dir, cfg); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Load it back and verify.
	loaded, err := LoadConfig(dir)
	if err != nil {
		t.Fatalf("unexpected error loading saved config: %v", err)
	}
	if loaded != cfg {
		t.Errorf("expected %+v, got %+v", cfg, loaded)
	}
}

func TestSaveConfig_PreservesOtherKeys(t *testing.T) {
	dir := t.TempDir()
	existing := `{
		"other_section": {"key": "value"},
		"autonomy": {
			"level": "gated",
			"max_auto_retries": 2,
			"max_queue_depth": 10
		}
	}`
	if err := os.WriteFile(filepath.Join(dir, "saw.config.json"), []byte(existing), 0644); err != nil {
		t.Fatal(err)
	}

	newCfg := Config{
		Level:          LevelSupervised,
		MaxAutoRetries: 4,
		MaxQueueDepth:  8,
	}
	if err := SaveConfig(dir, newCfg); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify autonomy section was updated.
	loaded, err := LoadConfig(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if loaded != newCfg {
		t.Errorf("expected %+v, got %+v", newCfg, loaded)
	}

	// Verify other_section was preserved.
	raw, err := os.ReadFile(filepath.Join(dir, "saw.config.json"))
	if err != nil {
		t.Fatal(err)
	}
	content := string(raw)
	if !containsSubstring(content, "other_section") {
		t.Errorf("other_section was not preserved in saved config: %s", content)
	}
}

func containsSubstring(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(s) > 0 && findSubstring(s, sub))
}

func findSubstring(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
