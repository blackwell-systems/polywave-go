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
			{
				Code:     "SESSION_SAVE_FAILED",
				Message:  fmt.Sprintf("resume.SaveAgentSession: creating sessions dir: %v", err),
				Severity: "fatal",
				Context:  map[string]string{"slug": slug},
				Cause:    err,
			},
		})
	}

	dest := sessionsFilePath(stateDir, slug)

	// Read existing file if present.
	existing, err := LoadAgentSessions(stateDir, slug)
	if err != nil {
		return result.NewFailure[SaveSessionData]([]result.SAWError{
			{
				Code:     "SESSION_SAVE_FAILED",
				Message:  fmt.Sprintf("resume.SaveAgentSession: loading existing sessions: %v", err),
				Severity: "fatal",
				Context:  map[string]string{"slug": slug},
				Cause:    err,
			},
		})
	}

	// Add or update the entry for this agent.
	existing[session.AgentID] = session

	data, err := json.MarshalIndent(existing, "", "  ")
	if err != nil {
		return result.NewFailure[SaveSessionData]([]result.SAWError{
			{
				Code:     "SESSION_SAVE_FAILED",
				Message:  fmt.Sprintf("resume.SaveAgentSession: marshalling JSON: %v", err),
				Severity: "fatal",
				Context:  map[string]string{"slug": slug},
				Cause:    err,
			},
		})
	}

	// Write to a temp file in the same directory to allow atomic rename.
	tmp := dest + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return result.NewFailure[SaveSessionData]([]result.SAWError{
			{
				Code:     "SESSION_SAVE_FAILED",
				Message:  fmt.Sprintf("resume.SaveAgentSession: writing temp file: %v", err),
				Severity: "fatal",
				Context:  map[string]string{"slug": slug},
				Cause:    err,
			},
		})
	}

	if err := os.Rename(tmp, dest); err != nil {
		// Clean up temp file on failure.
		_ = os.Remove(tmp)
		return result.NewFailure[SaveSessionData]([]result.SAWError{
			{
				Code:     "SESSION_SAVE_FAILED",
				Message:  fmt.Sprintf("resume.SaveAgentSession: renaming temp file: %v", err),
				Severity: "fatal",
				Context:  map[string]string{"slug": slug},
				Cause:    err,
			},
		})
	}

	return result.NewSuccess(SaveSessionData{
		SessionPath: dest,
		Timestamp:   time.Now().UTC(),
	})
}

// LoadAgentSessions loads agent sessions from .saw-state/sessions/{slug}.json.
// Returns an empty map (not an error) if the file does not exist.
// Returns an error only on I/O or JSON parse failures.
func LoadAgentSessions(stateDir string, slug string) (map[string]AgentSession, error) {
	path := sessionsFilePath(stateDir, slug)

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]AgentSession{}, nil
		}
		return nil, fmt.Errorf("resume.LoadAgentSessions: reading %s: %w", path, err)
	}

	var sessions map[string]AgentSession
	if err := json.Unmarshal(data, &sessions); err != nil {
		return nil, fmt.Errorf("resume.LoadAgentSessions: parsing %s: %w", path, err)
	}

	if sessions == nil {
		sessions = map[string]AgentSession{}
	}

	return sessions, nil
}
