package autonomy

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfig_NoFile(t *testing.T) {
	dir := t.TempDir()
	r := LoadConfig(dir)
	if !r.IsSuccess() {
		t.Fatalf("expected success, got %v errors", r.Errors)
	}
	d := r.GetData()
	if !d.WasDefault {
		t.Errorf("expected WasDefault=true when no file exists, got false")
	}
	def := DefaultConfig()
	if d.Config != def {
		t.Errorf("expected DefaultConfig %+v, got %+v", def, d.Config)
	}
	if d.ConfigPath == "" {
		t.Errorf("expected ConfigPath to be set")
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

	r := LoadConfig(dir)
	if !r.IsSuccess() {
		t.Fatalf("unexpected failure: %v", r.Errors)
	}
	d := r.GetData()
	if d.WasDefault {
		t.Errorf("expected WasDefault=false when config file is present, got true")
	}
	if d.Config.Level != LevelSupervised {
		t.Errorf("expected level %q, got %q", LevelSupervised, d.Config.Level)
	}
	if d.Config.MaxAutoRetries != 5 {
		t.Errorf("expected max_auto_retries 5, got %d", d.Config.MaxAutoRetries)
	}
	if d.Config.MaxQueueDepth != 20 {
		t.Errorf("expected max_queue_depth 20, got %d", d.Config.MaxQueueDepth)
	}
}

func TestLoadConfig_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "saw.config.json"), []byte("{not valid json"), 0644); err != nil {
		t.Fatal(err)
	}

	r := LoadConfig(dir)
	if !r.IsFatal() {
		t.Fatal("expected Fatal result for invalid JSON, got non-fatal")
	}
	if len(r.Errors) == 0 {
		t.Fatal("expected at least one error")
	}
	if r.Errors[0].Code != "CONFIG_LOAD_FAILED" {
		t.Errorf("expected error code CONFIG_LOAD_FAILED, got %q", r.Errors[0].Code)
	}
}

func TestLoadConfig_MissingAutonomySection(t *testing.T) {
	dir := t.TempDir()
	content := `{"other_section": {"foo": "bar"}}`
	if err := os.WriteFile(filepath.Join(dir, "saw.config.json"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	r := LoadConfig(dir)
	if !r.IsSuccess() {
		t.Fatalf("unexpected failure: %v", r.Errors)
	}
	d := r.GetData()
	if !d.WasDefault {
		t.Errorf("expected WasDefault=true when autonomy section missing, got false")
	}
	def := DefaultConfig()
	if d.Config != def {
		t.Errorf("expected DefaultConfig %+v, got %+v", def, d.Config)
	}
}

func TestSaveConfig_NewFile(t *testing.T) {
	dir := t.TempDir()
	cfg := Config{
		Level:          LevelAutonomous,
		MaxAutoRetries: 3,
		MaxQueueDepth:  15,
	}

	sr := SaveConfig(dir, cfg)
	if !sr.IsSuccess() {
		t.Fatalf("unexpected failure: %v", sr.Errors)
	}
	sd := sr.GetData()
	if sd.ConfigPath == "" {
		t.Errorf("expected ConfigPath to be set after successful save")
	}
	if sd.BytesWritten == 0 {
		t.Errorf("expected BytesWritten > 0, got 0")
	}

	// Load it back and verify.
	lr := LoadConfig(dir)
	if !lr.IsSuccess() {
		t.Fatalf("unexpected error loading saved config: %v", lr.Errors)
	}
	loaded := lr.GetData().Config
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
	sr := SaveConfig(dir, newCfg)
	if !sr.IsSuccess() {
		t.Fatalf("unexpected failure: %v", sr.Errors)
	}

	// Verify autonomy section was updated.
	lr := LoadConfig(dir)
	if !lr.IsSuccess() {
		t.Fatalf("unexpected error loading saved config: %v", lr.Errors)
	}
	loaded := lr.GetData().Config
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
