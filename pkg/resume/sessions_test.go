package resume

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func makeTestSession(agentID string, waveNum int) AgentSession {
	return AgentSession{
		AgentID:      agentID,
		SessionID:    "sess-" + agentID,
		WaveNum:      waveNum,
		WorktreePath: "/tmp/worktree-" + agentID,
		LastActive:   time.Now().UTC().Format(time.RFC3339),
	}
}

// TestSaveAndLoadAgentSession saves one session and loads it back, verifying all fields match.
func TestSaveAndLoadAgentSession(t *testing.T) {
	stateDir := t.TempDir()
	slug := "my-feature"
	session := makeTestSession("A", 1)

	if err := SaveAgentSession(stateDir, slug, session); err != nil {
		t.Fatalf("SaveAgentSession: %v", err)
	}

	sessions, err := LoadAgentSessions(stateDir, slug)
	if err != nil {
		t.Fatalf("LoadAgentSessions: %v", err)
	}

	got, ok := sessions["A"]
	if !ok {
		t.Fatal("expected session for agent A, not found")
	}

	if got.AgentID != session.AgentID {
		t.Errorf("AgentID: got %q, want %q", got.AgentID, session.AgentID)
	}
	if got.SessionID != session.SessionID {
		t.Errorf("SessionID: got %q, want %q", got.SessionID, session.SessionID)
	}
	if got.WaveNum != session.WaveNum {
		t.Errorf("WaveNum: got %d, want %d", got.WaveNum, session.WaveNum)
	}
	if got.WorktreePath != session.WorktreePath {
		t.Errorf("WorktreePath: got %q, want %q", got.WorktreePath, session.WorktreePath)
	}
	if got.LastActive != session.LastActive {
		t.Errorf("LastActive: got %q, want %q", got.LastActive, session.LastActive)
	}
}

// TestSaveAgentSession_UpdateExisting saves two sessions for different agents and verifies both are persisted.
func TestSaveAgentSession_UpdateExisting(t *testing.T) {
	stateDir := t.TempDir()
	slug := "multi-agent"
	sessionA := makeTestSession("A", 1)
	sessionB := makeTestSession("B", 1)

	if err := SaveAgentSession(stateDir, slug, sessionA); err != nil {
		t.Fatalf("SaveAgentSession A: %v", err)
	}
	if err := SaveAgentSession(stateDir, slug, sessionB); err != nil {
		t.Fatalf("SaveAgentSession B: %v", err)
	}

	sessions, err := LoadAgentSessions(stateDir, slug)
	if err != nil {
		t.Fatalf("LoadAgentSessions: %v", err)
	}

	if len(sessions) != 2 {
		t.Fatalf("expected 2 sessions, got %d", len(sessions))
	}

	if _, ok := sessions["A"]; !ok {
		t.Error("expected session for agent A")
	}
	if _, ok := sessions["B"]; !ok {
		t.Error("expected session for agent B")
	}
}

// TestLoadAgentSessions_FileNotExist returns an empty map and no error when file does not exist.
func TestLoadAgentSessions_FileNotExist(t *testing.T) {
	stateDir := t.TempDir()
	slug := "nonexistent"

	sessions, err := LoadAgentSessions(stateDir, slug)
	if err != nil {
		t.Fatalf("expected no error for missing file, got: %v", err)
	}
	if sessions == nil {
		t.Fatal("expected empty map, got nil")
	}
	if len(sessions) != 0 {
		t.Fatalf("expected empty map, got %d entries", len(sessions))
	}
}

// TestSaveAgentSession_CreatesDirectory verifies that SaveAgentSession creates the
// .saw-state/sessions/ directory if it does not exist.
func TestSaveAgentSession_CreatesDirectory(t *testing.T) {
	// Use a stateDir that does NOT pre-create the sessions subdirectory.
	stateDir := filepath.Join(t.TempDir(), "saw-state-new")
	// Do not create stateDir or sessions dir — SaveAgentSession must create them.

	slug := "dir-test"
	session := makeTestSession("A", 2)

	if err := SaveAgentSession(stateDir, slug, session); err != nil {
		t.Fatalf("SaveAgentSession: %v", err)
	}

	expectedDir := filepath.Join(stateDir, "sessions")
	info, err := os.Stat(expectedDir)
	if err != nil {
		t.Fatalf("sessions directory was not created: %v", err)
	}
	if !info.IsDir() {
		t.Fatalf("expected %s to be a directory", expectedDir)
	}

	// Verify the file exists and is readable.
	sessions, err := LoadAgentSessions(stateDir, slug)
	if err != nil {
		t.Fatalf("LoadAgentSessions: %v", err)
	}
	if _, ok := sessions["A"]; !ok {
		t.Error("expected session A to be present after directory creation")
	}
}

// TestSaveAgentSession_AtomicWrite verifies the temp file pattern by checking that
// after a successful save, no leftover .tmp file remains and the destination file exists.
func TestSaveAgentSession_AtomicWrite(t *testing.T) {
	stateDir := t.TempDir()
	slug := "atomic-test"
	session := makeTestSession("A", 1)

	if err := SaveAgentSession(stateDir, slug, session); err != nil {
		t.Fatalf("SaveAgentSession: %v", err)
	}

	destPath := filepath.Join(stateDir, "sessions", slug+".json")
	tmpPath := destPath + ".tmp"

	// Destination file must exist.
	if _, err := os.Stat(destPath); err != nil {
		t.Fatalf("destination file missing: %v", err)
	}

	// Temp file must NOT remain after successful write.
	if _, err := os.Stat(tmpPath); !os.IsNotExist(err) {
		t.Errorf("temp file %s should not exist after successful save", tmpPath)
	}
}
