package resume

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/blackwell-systems/polywave-go/pkg/result"
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

	res := SaveAgentSession(stateDir, slug, session)
	if !res.IsSuccess() {
		t.Fatalf("SaveAgentSession: result not success: %v", res.Errors)
	}

	// Verify SaveSessionData.SessionPath is correct.
	data := res.GetData()
	expectedPath := filepath.Join(stateDir, "sessions", slug+".json")
	if data.SessionPath != expectedPath {
		t.Errorf("SaveSessionData.SessionPath: got %q, want %q", data.SessionPath, expectedPath)
	}
	if data.Timestamp.IsZero() {
		t.Error("SaveSessionData.Timestamp should not be zero")
	}

	loadRes := LoadAgentSessions(stateDir, slug)
	if !loadRes.IsSuccess() {
		t.Fatalf("LoadAgentSessions: %v", loadRes.Errors)
	}
	sessions := loadRes.GetData()

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

	if res := SaveAgentSession(stateDir, slug, sessionA); !res.IsSuccess() {
		t.Fatalf("SaveAgentSession A: %v", res.Errors)
	}
	if res := SaveAgentSession(stateDir, slug, sessionB); !res.IsSuccess() {
		t.Fatalf("SaveAgentSession B: %v", res.Errors)
	}

	loadRes := LoadAgentSessions(stateDir, slug)
	if !loadRes.IsSuccess() {
		t.Fatalf("LoadAgentSessions: %v", loadRes.Errors)
	}
	sessions := loadRes.GetData()

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

	res := LoadAgentSessions(stateDir, slug)
	if !res.IsSuccess() {
		t.Fatalf("expected success for missing file, got errors: %v", res.Errors)
	}
	sessions := res.GetData()
	if sessions == nil {
		t.Fatal("expected empty map, got nil")
	}
	if len(sessions) != 0 {
		t.Fatalf("expected empty map, got %d entries", len(sessions))
	}
}

// TestSaveAgentSession_CreatesDirectory verifies that SaveAgentSession creates the
// .polywave-state/sessions/ directory if it does not exist.
func TestSaveAgentSession_CreatesDirectory(t *testing.T) {
	// Use a stateDir that does NOT pre-create the sessions subdirectory.
	stateDir := filepath.Join(t.TempDir(), "polywave-state-new")
	// Do not create stateDir or sessions dir — SaveAgentSession must create them.

	slug := "dir-test"
	session := makeTestSession("A", 2)

	if res := SaveAgentSession(stateDir, slug, session); !res.IsSuccess() {
		t.Fatalf("SaveAgentSession: %v", res.Errors)
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
	loadRes := LoadAgentSessions(stateDir, slug)
	if !loadRes.IsSuccess() {
		t.Fatalf("LoadAgentSessions: %v", loadRes.Errors)
	}
	sessions := loadRes.GetData()
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

	if res := SaveAgentSession(stateDir, slug, session); !res.IsSuccess() {
		t.Fatalf("SaveAgentSession: %v", res.Errors)
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

// TestSaveAgentSession_FailureReturnsFatal verifies that a filesystem error results
// in a Fatal result with code SESSION_SAVE_FAILED and the slug in the error context.
func TestSaveAgentSession_FailureReturnsFatal(t *testing.T) {
	// Use a stateDir path that cannot be created: use an existing file as the parent.
	tmpFile, err := os.CreateTemp(t.TempDir(), "not-a-dir-*.txt")
	if err != nil {
		t.Fatalf("setup: %v", err)
	}
	_ = tmpFile.Close()

	// Point stateDir to a path that uses the file as a directory component — cannot be created.
	stateDir := filepath.Join(tmpFile.Name(), "subdir")
	slug := "fail-test"
	session := makeTestSession("A", 1)

	res := SaveAgentSession(stateDir, slug, session)

	if !res.IsFatal() {
		t.Fatalf("expected Fatal result, got code: %s", res.Code)
	}
	if len(res.Errors) == 0 {
		t.Fatal("expected at least one error in Fatal result")
	}

	sawErr := res.Errors[0]
	if sawErr.Code != result.CodeSessionSaveFailed {
		t.Errorf("error code: got %q, want %q", sawErr.Code, result.CodeSessionSaveFailed)
	}
	if sawErr.Context["slug"] != slug {
		t.Errorf("error context slug: got %q, want %q", sawErr.Context["slug"], slug)
	}
	if !strings.Contains(sawErr.Message, "resume.SaveAgentSession") {
		t.Errorf("error message should contain function name, got: %q", sawErr.Message)
	}
}

// TestLoadAgentSessions_CorruptedJSON verifies that a malformed JSON file returns a fatal result.
func TestLoadAgentSessions_CorruptedJSON(t *testing.T) {
	stateDir := t.TempDir()
	slug := "corrupt"
	// Write malformed JSON to the sessions file.
	sessionsDir := filepath.Join(stateDir, "sessions")
	if err := os.MkdirAll(sessionsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	bad := filepath.Join(sessionsDir, slug+".json")
	if err := os.WriteFile(bad, []byte("{not valid json"), 0o644); err != nil {
		t.Fatal(err)
	}
	res := LoadAgentSessions(stateDir, slug)
	if res.IsSuccess() {
		t.Fatal("expected failure for corrupted JSON, got success")
	}
	if len(res.Errors) == 0 {
		t.Fatal("expected at least one error")
	}
	if res.Errors[0].Code != result.CodeSessionSaveFailed {
		t.Errorf("code = %q, want %q", res.Errors[0].Code, result.CodeSessionSaveFailed)
	}
}
