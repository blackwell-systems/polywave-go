package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInitCmd_AlreadyExists(t *testing.T) {
	dir := t.TempDir()
	// Create existing polywave.config.json.
	if err := os.WriteFile(filepath.Join(dir, "polywave.config.json"), []byte("{}"), 0644); err != nil {
		t.Fatal(err)
	}

	cmd := newInitCmd()
	cmd.SetArgs([]string{"--repo", dir})
	var buf bytes.Buffer
	cmd.SetOut(&buf)

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error when polywave.config.json exists without --force")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Errorf("error = %q, want it to mention 'already exists'", err.Error())
	}
}

func TestInitCmd_Force(t *testing.T) {
	dir := t.TempDir()
	// Create existing polywave.config.json.
	if err := os.WriteFile(filepath.Join(dir, "polywave.config.json"), []byte("{}"), 0644); err != nil {
		t.Fatal(err)
	}
	// Also add a go.mod to make detection work.
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module test"), 0644); err != nil {
		t.Fatal(err)
	}

	cmd := newInitCmd()
	cmd.SetArgs([]string{"--repo", dir, "--force"})
	var buf bytes.Buffer
	cmd.SetOut(&buf)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error with --force: %v", err)
	}

	// Verify the config was overwritten (should have repos now).
	data, err := os.ReadFile(filepath.Join(dir, "polywave.config.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "repos") {
		t.Error("overwritten config should contain 'repos'")
	}
}

func TestInitCmd_WritesConfig(t *testing.T) {
	dir := t.TempDir()
	// Create go.mod marker.
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module test"), 0644); err != nil {
		t.Fatal(err)
	}

	cmd := newInitCmd()
	cmd.SetArgs([]string{"--repo", dir})
	var buf bytes.Buffer
	cmd.SetOut(&buf)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "polywave.config.json"))
	if err != nil {
		t.Fatal(err)
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("cannot unmarshal config: %v", err)
	}

	// Check repos.
	var repos []struct {
		Name string `json:"name"`
		Path string `json:"path"`
	}
	if err := json.Unmarshal(raw["repos"], &repos); err != nil {
		t.Fatalf("cannot unmarshal repos: %v", err)
	}
	if len(repos) != 1 {
		t.Fatalf("repos count = %d, want 1", len(repos))
	}
	if repos[0].Name != filepath.Base(dir) {
		t.Errorf("repos[0].name = %q, want %q", repos[0].Name, filepath.Base(dir))
	}
	if repos[0].Path != dir {
		t.Errorf("repos[0].path = %q, want %q", repos[0].Path, dir)
	}

	// Check agent config.
	var agent struct {
		ScoutModel string `json:"scout_model"`
	}
	if err := json.Unmarshal(raw["agent"], &agent); err != nil {
		t.Fatalf("cannot unmarshal agent: %v", err)
	}
	if agent.ScoutModel != "claude-sonnet-4-6" {
		t.Errorf("scout_model = %q, want %q", agent.ScoutModel, "claude-sonnet-4-6")
	}

	// Check build key.
	var build struct {
		Command  string `json:"command"`
		Detected bool   `json:"detected"`
	}
	if _, ok := raw["build"]; !ok {
		t.Fatal("missing 'build' key in config")
	}
	if err := json.Unmarshal(raw["build"], &build); err != nil {
		t.Fatalf("cannot unmarshal build: %v", err)
	}
	if build.Command != "go build ./..." {
		t.Errorf("build.command = %q, want %q", build.Command, "go build ./...")
	}
	if !build.Detected {
		t.Error("build.detected = false, want true")
	}

	// Check test key.
	var testEntry struct {
		Command  string `json:"command"`
		Detected bool   `json:"detected"`
	}
	if _, ok := raw["test"]; !ok {
		t.Fatal("missing 'test' key in config")
	}
	if err := json.Unmarshal(raw["test"], &testEntry); err != nil {
		t.Fatalf("cannot unmarshal test: %v", err)
	}
	if testEntry.Command != "go test ./..." {
		t.Errorf("test.command = %q, want %q", testEntry.Command, "go test ./...")
	}
	if !testEntry.Detected {
		t.Error("test.detected = false, want true")
	}
}

func TestInitCmd_Output(t *testing.T) {
	dir := t.TempDir()
	// Create go.mod marker.
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module test"), 0644); err != nil {
		t.Fatal(err)
	}

	cmd := newInitCmd()
	cmd.SetArgs([]string{"--repo", dir})
	var buf bytes.Buffer
	cmd.SetOut(&buf)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "SAW initialized") {
		t.Errorf("output missing 'SAW initialized', got:\n%s", output)
	}
	baseName := filepath.Base(dir)
	if !strings.Contains(output, baseName) {
		t.Errorf("output missing project name %q, got:\n%s", baseName, output)
	}
}
