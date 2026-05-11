package main

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestQueueAddCmd verifies that the add subcommand creates a queue item file.
func TestQueueAddCmd(t *testing.T) {
	tmpDir := t.TempDir()

	// Override repoDir to point at temp dir.
	origRepoDir := repoDir
	repoDir = tmpDir
	defer func() { repoDir = origRepoDir }()

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	cmd := newQueueCmd()
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{
		"add",
		"--title", "My Feature",
		"--priority", "10",
		"--description", "A test feature",
	})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("command failed: %v\nstderr: %s", err, stderr.String())
	}

	// Verify JSON output.
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(strings.TrimSpace(stdout.String())), &result); err != nil {
		t.Fatalf("failed to parse output JSON: %v\noutput: %s", err, stdout.String())
	}

	if added, ok := result["added"].(bool); !ok || !added {
		t.Errorf("expected added=true, got %v", result["added"])
	}

	slug, _ := result["slug"].(string)
	if slug == "" {
		t.Errorf("expected non-empty slug, got %q", slug)
	}

	// The slug for "My Feature" should be "my-feature".
	if slug != "my-feature" {
		t.Errorf("expected slug 'my-feature', got %q", slug)
	}

	path, _ := result["path"].(string)
	if !strings.Contains(path, "010-my-feature.yaml") {
		t.Errorf("expected path to contain '010-my-feature.yaml', got %q", path)
	}

	// Verify the file was actually created.
	expectedFile := filepath.Join(tmpDir, "docs", "IMPL", "queue", "010-my-feature.yaml")
	if _, err := os.Stat(expectedFile); err != nil {
		t.Errorf("expected queue file to exist at %s: %v", expectedFile, err)
	}
}

// TestQueueListCmd verifies that the list subcommand returns a sorted list.
func TestQueueListCmd(t *testing.T) {
	tmpDir := t.TempDir()

	origRepoDir := repoDir
	repoDir = tmpDir
	defer func() { repoDir = origRepoDir }()

	// Pre-populate queue items by using the add command.
	addItems := []struct {
		title    string
		priority string
	}{
		{"Bravo Feature", "20"},
		{"Alpha Feature", "10"},
		{"Charlie Feature", "30"},
	}

	for _, it := range addItems {
		addCmd := newQueueCmd()
		addCmd.SetArgs([]string{"add", "--title", it.title, "--priority", it.priority})
		if err := addCmd.Execute(); err != nil {
			t.Fatalf("setup add failed for %q: %v", it.title, err)
		}
	}

	// Now list.
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	cmd := newQueueCmd()
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"list"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("list command failed: %v\nstderr: %s", err, stderr.String())
	}

	var items []map[string]interface{}
	if err := json.Unmarshal([]byte(strings.TrimSpace(stdout.String())), &items); err != nil {
		t.Fatalf("failed to parse list output JSON: %v\noutput: %s", err, stdout.String())
	}

	if len(items) != 3 {
		t.Fatalf("expected 3 items, got %d", len(items))
	}

	// Items should be sorted by priority (ascending).
	priorities := []float64{10, 20, 30}
	for i, item := range items {
		p, _ := item["priority"].(float64)
		if p != priorities[i] {
			t.Errorf("item[%d] priority: expected %v, got %v", i, priorities[i], p)
		}
	}

	// Verify required fields.
	firstItem := items[0]
	if _, ok := firstItem["slug"]; !ok {
		t.Error("expected 'slug' field in list output")
	}
	if _, ok := firstItem["title"]; !ok {
		t.Error("expected 'title' field in list output")
	}
	if _, ok := firstItem["status"]; !ok {
		t.Error("expected 'status' field in list output")
	}
}

// TestQueueNextCmd verifies that the next subcommand returns the next eligible item.
func TestQueueNextCmd(t *testing.T) {
	tmpDir := t.TempDir()

	origRepoDir := repoDir
	repoDir = tmpDir
	defer func() { repoDir = origRepoDir }()

	// Add two items; item with priority 5 should be next.
	for _, it := range []struct {
		title    string
		priority string
	}{
		{"Low Priority Feature", "50"},
		{"High Priority Feature", "5"},
	} {
		addCmd := newQueueCmd()
		addCmd.SetArgs([]string{"add", "--title", it.title, "--priority", it.priority})
		if err := addCmd.Execute(); err != nil {
			t.Fatalf("setup add failed: %v", err)
		}
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	cmd := newQueueCmd()
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"next"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("next command failed: %v\nstderr: %s", err, stderr.String())
	}

	var result map[string]interface{}
	if err := json.Unmarshal([]byte(strings.TrimSpace(stdout.String())), &result); err != nil {
		t.Fatalf("failed to parse next output JSON: %v\noutput: %s", err, stdout.String())
	}

	// Should return the high-priority (priority=5) item.
	slug, _ := result["slug"].(string)
	if slug != "high-priority-feature" {
		t.Errorf("expected slug 'high-priority-feature', got %q", slug)
	}

	title, _ := result["title"].(string)
	if title != "High Priority Feature" {
		t.Errorf("expected title 'High Priority Feature', got %q", title)
	}

	priority, _ := result["priority"].(float64)
	if priority != 5 {
		t.Errorf("expected priority 5, got %v", priority)
	}

	// Should NOT have a "next" null key.
	if _, hasNext := result["next"]; hasNext {
		t.Error("unexpected 'next' key in result (should only appear when empty)")
	}
}

// TestQueueNextCmd_Empty verifies that next returns {\"next\": null} when queue is empty.
func TestQueueNextCmd_Empty(t *testing.T) {
	tmpDir := t.TempDir()

	// Initialize as git repository
	initCmd := exec.Command("git", "init")
	initCmd.Dir = tmpDir
	if err := initCmd.Run(); err != nil {
		t.Fatalf("Failed to init git repo: %v", err)
	}

	origRepoDir := repoDir
	repoDir = tmpDir
	defer func() { repoDir = origRepoDir }()

	// Do NOT add any queue items.

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	cmd := newQueueCmd()
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"next"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("next command failed: %v\nstderr: %s", err, stderr.String())
	}

	var result map[string]interface{}
	if err := json.Unmarshal([]byte(strings.TrimSpace(stdout.String())), &result); err != nil {
		t.Fatalf("failed to parse next output JSON: %v\noutput: %s", err, stdout.String())
	}

	// Should have "next": null.
	nextVal, ok := result["next"]
	if !ok {
		t.Error("expected 'next' key in result")
	}
	if nextVal != nil {
		t.Errorf("expected next=null, got %v", nextVal)
	}
}
