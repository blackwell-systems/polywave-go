package protocol

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/result"
)

// SaveManifestData holds the result payload for a successful Save call.
type SaveManifestData struct {
	ManifestPath string
	Timestamp    time.Time
}

// SetReportData holds the result payload for a successful SetCompletionReport call.
type SetReportData struct {
	AgentID   string
	ReportSet bool
}

// ErrAgentNotFound is returned when SetCompletionReport is called with an unknown agent ID.
var ErrAgentNotFound = errors.New("agent not found in manifest")

// completionReportMu serializes all Load-Set-Save sequences for completion
// reports across the orchestrator, engine, and CLI write paths.
// Replaces the caller-side reportMu (pkg/orchestrator) and reportWaveMu
// (pkg/engine) with a single canonical lock owned by the protocol package.
var completionReportMu sync.Mutex

// WithCompletionReportLock executes fn inside completionReportMu and returns
// its error. All callers that perform a Load-Set-Save sequence for completion
// reports must use this function instead of their own local mutexes.
//
// Typical usage:
//
//	err := protocol.WithCompletionReportLock(ctx, func(ctx context.Context) error {
//	    m, err := protocol.Load(ctx, implDocPath)
//	    if err != nil { return err }
//	    if err := builder.AppendToManifest(m); err != nil { return err }
//	    return protocol.Save(ctx, m, implDocPath)
//	})
func WithCompletionReportLock(ctx context.Context, fn func(ctx context.Context) error) error {
	completionReportMu.Lock()
	defer completionReportMu.Unlock()
	return fn(ctx)
}

// Load reads a YAML IMPL manifest from the specified path and parses it into an IMPLManifest.
// Returns an error if the file cannot be read or the YAML is invalid.
// Prevention fix: Detects duplicate completion report keys (agents writing reports twice).
//
// Cannot use LoadYAML: has specialized duplicate-key detection and orphaned-report validation
// logic that the generic LoadYAML helper omits. LoadYAML delegates here for IMPLManifest.
func Load(ctx context.Context, path string) (*IMPLManifest, error) {
	_ = ctx // reserved for future cancellation support
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read manifest file: %w", err)
	}

	var manifest IMPLManifest
	if err := yaml.Unmarshal(data, &manifest); err != nil {
		// Enhanced error for duplicate keys (prevention fix for agent double-writes)
		if isYAMLDuplicateKeyError(err) {
			return nil, fmt.Errorf("duplicate key in YAML manifest (likely completion report written twice): %w", err)
		}
		return nil, fmt.Errorf("failed to parse manifest YAML: %w", err)
	}

	// Initialize maps if nil
	if manifest.CompletionReports == nil {
		manifest.CompletionReports = make(map[string]CompletionReport)
	}

	// Verify completion report keys match agents (prevention fix)
	for agentID := range manifest.CompletionReports {
		found := false
		for _, wave := range manifest.Waves {
			for _, agent := range wave.Agents {
				if agent.ID == agentID {
					found = true
					break
				}
			}
			if found {
				break
			}
		}
		if !found {
			return nil, fmt.Errorf("completion report for unknown agent: %s (possible duplicate or orphaned report)", agentID)
		}
	}

	return &manifest, nil
}

// isYAMLDuplicateKeyError detects if the error is from duplicate keys in YAML.
// The yaml.v3 library returns a generic error, but we can check the message.
func isYAMLDuplicateKeyError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "already defined") || strings.Contains(msg, "duplicate key")
}

// Save writes an IMPLManifest to the specified path as YAML.
// Returns a Fatal result if the file cannot be written or the manifest cannot be marshaled.
//
// Cannot use SaveYAML: Save is the canonical write path for IMPLManifest and is called
// via WithCompletionReportLock. Routing through the generic helper would invert the
// dependency — callers should use Save (not SaveYAML) for IMPLManifest.
func Save(ctx context.Context, m *IMPLManifest, path string) result.Result[SaveManifestData] {
	_ = ctx // reserved for future cancellation support
	data, err := yaml.Marshal(m)
	if err != nil {
		return result.NewFailure[SaveManifestData]([]result.SAWError{
			result.NewFatal("MANIFEST_SAVE_FAILED", fmt.Sprintf("failed to marshal manifest to YAML: %v", err)),
		})
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return result.NewFailure[SaveManifestData]([]result.SAWError{
			result.NewFatal("MANIFEST_SAVE_FAILED", fmt.Sprintf("failed to write manifest file: %v", err)),
		})
	}

	return result.NewSuccess(SaveManifestData{
		ManifestPath: path,
		Timestamp:    time.Now(),
	})
}

// CurrentWave returns the first wave that has incomplete agents (agents without completion reports).
// If all waves are complete, returns nil.
// An agent is considered incomplete if its completion report is missing or has status != "complete".
func CurrentWave(m *IMPLManifest) *Wave {
	if m.CompletionReports == nil {
		m.CompletionReports = make(map[string]CompletionReport)
	}

	for i := range m.Waves {
		wave := &m.Waves[i]
		for _, agent := range wave.Agents {
			report, exists := m.CompletionReports[agent.ID]
			if !exists || report.Status != StatusComplete {
				return wave
			}
		}
	}

	return nil
}

// SetCompletionReport registers a completion report for the specified agent.
// Returns a Fatal result if the agent ID is not found in any wave or if the agent ID is empty.
func SetCompletionReport(m *IMPLManifest, agentID string, report CompletionReport) result.Result[SetReportData] {
	if agentID == "" {
		return result.NewFailure[SetReportData]([]result.SAWError{
			result.NewFatal("REPORT_SET_FAILED", "agent ID cannot be empty").WithContext("agent_id", agentID),
		})
	}

	// Verify agent exists in manifest
	found := false
	for _, wave := range m.Waves {
		for _, agent := range wave.Agents {
			if agent.ID == agentID {
				found = true
				break
			}
		}
		if found {
			break
		}
	}

	if !found {
		return result.NewFailure[SetReportData]([]result.SAWError{
			result.NewFatal("REPORT_SET_FAILED", fmt.Sprintf("%s: %s", ErrAgentNotFound, agentID)).WithContext("agent_id", agentID),
		})
	}

	// Initialize map if nil
	if m.CompletionReports == nil {
		m.CompletionReports = make(map[string]CompletionReport)
	}

	// Store report
	m.CompletionReports[agentID] = report

	return result.NewSuccess(SetReportData{
		AgentID:   agentID,
		ReportSet: true,
	})
}
