package engine

import (
	"context"
	"crypto/md5"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// minimalIMPLYAML returns a minimal valid IMPL YAML for testing PrepareAgent.
func minimalIMPLYAML(agentID string, waveNum int) string {
	return fmt.Sprintf(`title: Test Feature
feature_slug: test-feature
verdict: SUITABLE
state: REVIEWED
test_command: "go test ./..."
file_ownership:
  - file: pkg/test/file.go
    agent: %s
    wave: %d
waves:
  - number: %d
    agents:
      - id: %s
        task: "Implement test feature"
        files:
          - pkg/test/file.go
`, agentID, waveNum, waveNum, agentID)
}

// TestPrepareAgent_JournalContextAvailable_FalseWhenNoSessionFiles verifies that
// when no Claude session files exist, JournalContextAvailable is false.
func TestPrepareAgent_JournalContextAvailable_FalseWhenNoSessionFiles(t *testing.T) {
	tmpDir := t.TempDir()

	// Write a minimal IMPL doc
	implPath := filepath.Join(tmpDir, "IMPL-test.yaml")
	if err := os.WriteFile(implPath, []byte(minimalIMPLYAML("A", 1)), 0644); err != nil {
		t.Fatalf("failed to write IMPL doc: %v", err)
	}

	res := PrepareAgent(context.Background(), PrepareAgentOpts{
		ManifestPath: implPath,
		ProjectRoot:  tmpDir,
		WaveNum:      1,
		AgentID:      "A",
		NoWorktree:   true,
	})

	if !res.IsSuccess() {
		t.Fatalf("PrepareAgent failed: %v", res.Errors)
	}

	data := res.GetData()

	if data.JournalContextAvailable {
		t.Error("expected JournalContextAvailable=false when no session files exist")
	}
	if data.JournalContextFile != "" {
		t.Errorf("expected JournalContextFile empty, got %q", data.JournalContextFile)
	}
}

// writeMockSessionEntry writes a single JSONL log entry containing a tool use
// block in the format that JournalObserver.extractToolEntries expects:
//
//	{ timestamp, type, content: { messages: [{ role, content: [{ type: tool_use, ... }] }] } }
func writeMockSessionEntry(f *os.File) error {
	logEntry := map[string]interface{}{
		"timestamp": time.Now().Add(-2 * time.Minute).Format(time.RFC3339Nano),
		"type":      "message",
		"content": map[string]interface{}{
			"messages": []map[string]interface{}{
				{
					"role": "assistant",
					"content": []map[string]interface{}{
						{
							"type": "tool_use",
							"id":   "toolu_test_001",
							"name": "Read",
							"input": map[string]interface{}{
								"file_path": "/tmp/test.go",
							},
						},
					},
				},
			},
		},
	}

	enc := json.NewEncoder(f)
	if err := enc.Encode(logEntry); err != nil {
		return fmt.Errorf("encode: %w", err)
	}
	return nil
}

// TestPrepareAgent_JournalContextAvailable_TrueWhenSessionHasToolUses verifies
// that when a populated session file exists, JournalContextAvailable is true
// and JournalContextFile points to a written context.md file.
func TestPrepareAgent_JournalContextAvailable_TrueWhenSessionHasToolUses(t *testing.T) {
	tmpDir := t.TempDir()

	// Compute the Claude project dir hash — must match getClaudeProjectDir logic.
	hash := md5.Sum([]byte(tmpDir))
	projectHash := fmt.Sprintf("%x", hash)[:16]
	claudeProjectDir := filepath.Join(os.Getenv("HOME"), ".claude", "projects", projectHash)

	// Create the mock Claude project directory and session file.
	if err := os.MkdirAll(claudeProjectDir, 0755); err != nil {
		t.Fatalf("failed to create mock claude project dir: %v", err)
	}
	// Clean up after the test to avoid polluting the real Claude state.
	t.Cleanup(func() {
		os.RemoveAll(claudeProjectDir)
	})

	sessionFile := filepath.Join(claudeProjectDir, "test-session.jsonl")
	f, err := os.Create(sessionFile)
	if err != nil {
		t.Fatalf("failed to create session file: %v", err)
	}
	if err := writeMockSessionEntry(f); err != nil {
		f.Close()
		t.Fatalf("failed to write session entry: %v", err)
	}
	f.Close()

	// Write a minimal IMPL doc
	implPath := filepath.Join(tmpDir, "IMPL-test.yaml")
	if err := os.WriteFile(implPath, []byte(minimalIMPLYAML("A", 1)), 0644); err != nil {
		t.Fatalf("failed to write IMPL doc: %v", err)
	}

	res := PrepareAgent(context.Background(), PrepareAgentOpts{
		ManifestPath: implPath,
		ProjectRoot:  tmpDir,
		WaveNum:      1,
		AgentID:      "A",
		NoWorktree:   true,
	})

	if !res.IsSuccess() {
		t.Fatalf("PrepareAgent failed: %v", res.Errors)
	}

	data := res.GetData()

	if !data.JournalContextAvailable {
		t.Error("expected JournalContextAvailable=true when session has tool uses")
	}
	if data.JournalContextFile == "" {
		t.Error("expected JournalContextFile to be non-empty")
	}

	// Verify the context file was actually written to disk.
	if _, err := os.Stat(data.JournalContextFile); os.IsNotExist(err) {
		t.Errorf("JournalContextFile does not exist on disk: %s", data.JournalContextFile)
	}
}
