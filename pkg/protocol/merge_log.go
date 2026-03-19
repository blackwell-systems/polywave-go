package protocol

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// MergeLog tracks per-agent merge state for wave merge idempotency.
type MergeLog struct {
	Wave   int          `json:"wave"`
	Merges []MergeEntry `json:"merges"`
}

// MergeEntry records a single agent merge.
type MergeEntry struct {
	Agent     string    `json:"agent"`
	MergeSHA  string    `json:"merge_sha"`
	Timestamp time.Time `json:"timestamp"`
}

// LoadMergeLog reads merge-log.json for a wave. Returns empty MergeLog if file doesn't exist.
func LoadMergeLog(manifestPath string, waveNum int) (*MergeLog, error) {
	logPath := getMergeLogPath(manifestPath, waveNum)

	// Return empty log if file doesn't exist (first merge attempt)
	if _, err := os.Stat(logPath); os.IsNotExist(err) {
		return &MergeLog{
			Wave:   waveNum,
			Merges: []MergeEntry{},
		}, nil
	}

	data, err := os.ReadFile(logPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read merge log: %w", err)
	}

	var log MergeLog
	if err := json.Unmarshal(data, &log); err != nil {
		return nil, fmt.Errorf("failed to parse merge log: %w", err)
	}

	return &log, nil
}

// SaveMergeLog writes merge-log.json after successful agent merge.
func SaveMergeLog(manifestPath string, waveNum int, log *MergeLog) error {
	logPath := getMergeLogPath(manifestPath, waveNum)
	logDir := filepath.Dir(logPath)

	// Create .saw-state/wave{N}/ directory if needed
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return fmt.Errorf("failed to create merge log directory: %w", err)
	}

	// Pretty-print JSON with 2-space indent for readability
	data, err := json.MarshalIndent(log, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal merge log: %w", err)
	}

	// Overwrite existing file (not append)
	if err := os.WriteFile(logPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write merge log: %w", err)
	}

	return nil
}

// AddMergeEntry appends a merge entry to the log.
func (ml *MergeLog) AddMergeEntry(agent string, mergeSHA string) error {
	if agent == "" {
		return fmt.Errorf("agent ID cannot be empty")
	}
	if mergeSHA == "" {
		return fmt.Errorf("merge SHA cannot be empty")
	}

	entry := MergeEntry{
		Agent:     agent,
		MergeSHA:  mergeSHA,
		Timestamp: time.Now(),
	}

	ml.Merges = append(ml.Merges, entry)
	return nil
}

// IsMerged checks if an agent has already been merged.
func (ml *MergeLog) IsMerged(agent string) bool {
	agentLower := strings.ToLower(agent)
	for _, entry := range ml.Merges {
		if strings.ToLower(entry.Agent) == agentLower {
			return true
		}
	}
	return false
}

// GetMergeSHA returns the merge SHA for an agent, or empty string if not merged.
func (ml *MergeLog) GetMergeSHA(agent string) string {
	agentLower := strings.ToLower(agent)
	for _, entry := range ml.Merges {
		if strings.ToLower(entry.Agent) == agentLower {
			return entry.MergeSHA
		}
	}
	return ""
}

// getMergeLogPath returns the path to merge-log.json for a wave.
// Namespaced by IMPL slug to prevent cross-IMPL merge log collisions
// (all active IMPLs share docs/IMPL/ as their directory).
func getMergeLogPath(manifestPath string, waveNum int) string {
	manifestDir := filepath.Dir(manifestPath)
	slug := extractSlugFromPath(manifestPath)
	return filepath.Join(manifestDir, ".saw-state", slug, fmt.Sprintf("wave%d", waveNum), "merge-log.json")
}

// extractSlugFromPath extracts the IMPL slug from a manifest filename.
// e.g. "docs/IMPL/IMPL-structured-error-parsing.yaml" → "structured-error-parsing"
func extractSlugFromPath(manifestPath string) string {
	base := filepath.Base(manifestPath)
	base = strings.TrimPrefix(base, "IMPL-")
	base = strings.TrimSuffix(base, ".yaml")
	base = strings.TrimSuffix(base, ".yml")
	if base == "" {
		return "unknown"
	}
	return base
}
