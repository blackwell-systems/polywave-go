package resume

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/result"
)

// SaveSessionData contains the output of a successful SaveAgentSession call.
type SaveSessionData struct {
	SessionPath string
	Timestamp   time.Time
}

// sessionsFilePath returns the path to the sessions file for the given slug.
// Format: {stateDir}/sessions/{slug}.json
func sessionsFilePath(stateDir, slug string) string {
	return filepath.Join(stateDir, "sessions", slug+".json")
}

// SaveAgentSession persists an AgentSession to .saw-state/sessions/{slug}.json.
// It reads the existing file (if any), updates the entry for session.AgentID,
// and writes back atomically using a temp file + os.Rename.
// Returns a result.Result[SaveSessionData] with the session path and timestamp on success,
// or a Fatal result with code SESSION_SAVE_FAILED on filesystem error.
func SaveAgentSession(stateDir string, slug string, session AgentSession) result.Result[SaveSessionData] {
	sessionsDir := filepath.Join(stateDir, "sessions")
	if err := os.MkdirAll(sessionsDir, 0o755); err != nil {
		return result.NewFailure[SaveSessionData]([]result.SAWError{
			result.NewFatal(result.CodeSessionSaveFailed, fmt.Sprintf("resume.SaveAgentSession: creating sessions dir: %v", err)).WithContext("slug", slug).WithCause(err),
		})
	}

	dest := sessionsFilePath(stateDir, slug)

	// Read existing file if present.
	loadRes := LoadAgentSessions(stateDir, slug)
	if !loadRes.IsSuccess() {
		return result.NewFailure[SaveSessionData](loadRes.Errors)
	}
	existing := loadRes.GetData()

	// Add or update the entry for this agent.
	existing[session.AgentID] = session

	data, err := json.MarshalIndent(existing, "", "  ")
	if err != nil {
		return result.NewFailure[SaveSessionData]([]result.SAWError{
			result.NewFatal(result.CodeSessionSaveFailed, fmt.Sprintf("resume.SaveAgentSession: marshalling JSON: %v", err)).WithContext("slug", slug).WithCause(err),
		})
	}

	// Write to a temp file in the same directory to allow atomic rename.
	tmp := dest + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return result.NewFailure[SaveSessionData]([]result.SAWError{
			result.NewFatal(result.CodeSessionSaveFailed, fmt.Sprintf("resume.SaveAgentSession: writing temp file: %v", err)).WithContext("slug", slug).WithCause(err),
		})
	}

	if err := os.Rename(tmp, dest); err != nil {
		// Clean up temp file on failure.
		_ = os.Remove(tmp)
		return result.NewFailure[SaveSessionData]([]result.SAWError{
			result.NewFatal(result.CodeSessionSaveFailed, fmt.Sprintf("resume.SaveAgentSession: renaming temp file: %v", err)).WithContext("slug", slug).WithCause(err),
		})
	}

	return result.NewSuccess(SaveSessionData{
		SessionPath: dest,
		Timestamp:   time.Now().UTC(),
	})
}

// LoadAgentSessions loads agent sessions from .saw-state/sessions/{slug}.json.
// Returns a successful Result with an empty map when the file does not exist.
// Returns a fatal Result with CodeSessionSaveFailed on I/O or JSON parse failures.
func LoadAgentSessions(stateDir string, slug string) result.Result[map[string]AgentSession] {
	path := sessionsFilePath(stateDir, slug)

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return result.NewSuccess(map[string]AgentSession{})
		}
		return result.NewFailure[map[string]AgentSession]([]result.SAWError{
			result.NewFatal(result.CodeSessionSaveFailed, fmt.Sprintf("resume.LoadAgentSessions: reading %s: %v", path, err)).WithContext("slug", slug).WithCause(err),
		})
	}

	var sessions map[string]AgentSession
	if err := json.Unmarshal(data, &sessions); err != nil {
		return result.NewFailure[map[string]AgentSession]([]result.SAWError{
			result.NewFatal(result.CodeSessionSaveFailed, fmt.Sprintf("resume.LoadAgentSessions: parsing %s: %v", path, err)).WithContext("slug", slug).WithCause(err),
		})
	}

	if sessions == nil {
		sessions = map[string]AgentSession{}
	}

	return result.NewSuccess(sessions)
}
