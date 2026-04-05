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

func TestLoadConfig_InvalidLevel(t *testing.T) {
	dir := t.TempDir()
	content := `{"autonomy": {"level": "turbo"}}`
	if err := os.WriteFile(filepath.Join(dir, "saw.config.json"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	r := LoadConfig(dir)
	if !r.IsFatal() {
		t.Fatal("expected fatal result for unrecognized level, got non-fatal")
	}
	if len(r.Errors) == 0 {
		t.Fatal("expected at least one error")
	}
}
