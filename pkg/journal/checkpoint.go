package journal

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Checkpoint represents a named snapshot of journal state at a key milestone
type Checkpoint struct {
	Name         string    `json:"name"`
	Timestamp    time.Time `json:"timestamp"`
	EntryCount   int       `json:"entry_count"`
	SnapshotPath string    `json:"snapshot_path"`
}

// Checkpoint creates a snapshot of current journal state with the given name.
// Copies index.jsonl, cursor.json, and recent.json to checkpoints/ subdirectory.
// Returns error if checkpoint name already exists or filesystem operation fails.
func (o *JournalObserver) Checkpoint(name string) error {
	// Validate checkpoint name (filesystem-safe, no slashes or spaces)
	if strings.ContainsAny(name, "/\\ ") {
		return fmt.Errorf("checkpoint name must be filesystem-safe (no slashes or spaces): %q", name)
	}
	if name == "" {
		return fmt.Errorf("checkpoint name cannot be empty")
	}

	// Create checkpoints directory if it doesn't exist
	checkpointsDir := filepath.Join(string(o.JournalDir), "checkpoints")
	if err := os.MkdirAll(checkpointsDir, 0755); err != nil {
		return fmt.Errorf("failed to create checkpoints directory: %w", err)
	}

	// Check if checkpoint already exists
	metadataPath := filepath.Join(checkpointsDir, fmt.Sprintf("%s.json", name))
	if _, err := os.Stat(metadataPath); err == nil {
		return fmt.Errorf("checkpoint %q already exists", name)
	}

	// Count entries in index.jsonl
	entryCount, err := countJSONLLines(string(o.IndexPath))
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to count entries: %w", err)
	}

	// Create checkpoint snapshot files
	snapshotPrefix := filepath.Join(checkpointsDir, name)

	// Copy index.jsonl (optional - may not exist for empty journal)
	indexSnapshot := snapshotPrefix + "-index.jsonl"
	_ = copyFileIfExists(string(o.IndexPath), indexSnapshot)

	// Copy cursor.json (optional - may not exist for empty journal)
	cursorSnapshot := snapshotPrefix + "-cursor.json"
	_ = copyFileIfExists(string(o.CursorPath), cursorSnapshot)

	// Copy recent.json (optional - may not exist yet)
	recentSnapshot := snapshotPrefix + "-recent.json"
	_ = copyFileIfExists(string(o.RecentPath), recentSnapshot)

	// Create checkpoint metadata
	checkpoint := Checkpoint{
		Name:         name,
		Timestamp:    time.Now().UTC(),
		EntryCount:   entryCount,
		SnapshotPath: snapshotPrefix,
	}

	// Write checkpoint metadata
	metadataBytes, err := json.MarshalIndent(checkpoint, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal checkpoint metadata: %w", err)
	}

	if err := os.WriteFile(metadataPath, metadataBytes, 0644); err != nil {
		return fmt.Errorf("failed to write checkpoint metadata: %w", err)
	}

	return nil
}

// ListCheckpoints returns all checkpoints with metadata, sorted by timestamp (oldest first).
func (o *JournalObserver) ListCheckpoints() ([]Checkpoint, error) {
	checkpointsDir := filepath.Join(string(o.JournalDir), "checkpoints")

	// If checkpoints directory doesn't exist, return empty list
	if _, err := os.Stat(checkpointsDir); os.IsNotExist(err) {
		return []Checkpoint{}, nil
	}

	// Read all checkpoint metadata files
	entries, err := os.ReadDir(checkpointsDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read checkpoints directory: %w", err)
	}

	var checkpoints []Checkpoint
	for _, entry := range entries {
		// Only process .json metadata files (not snapshot files like *-index.jsonl, *-cursor.json)
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		// Skip snapshot files - metadata files don't contain hyphens after the base name
		// Metadata: "checkpoint-name.json"
		// Snapshots: "checkpoint-name-index.jsonl", "checkpoint-name-cursor.json"
		name := entry.Name()
		nameWithoutExt := strings.TrimSuffix(name, ".json")
		if strings.Contains(nameWithoutExt, "-index") || strings.Contains(nameWithoutExt, "-cursor") || strings.Contains(nameWithoutExt, "-recent") {
			continue
		}

		metadataPath := filepath.Join(checkpointsDir, entry.Name())
		data, err := os.ReadFile(metadataPath)
		if err != nil {
			continue // Skip unreadable files
		}

		var checkpoint Checkpoint
		if err := json.Unmarshal(data, &checkpoint); err != nil {
			continue // Skip invalid JSON
		}

		checkpoints = append(checkpoints, checkpoint)
	}

	// Sort by timestamp (oldest first)
	sort.Slice(checkpoints, func(i, j int) bool {
		return checkpoints[i].Timestamp.Before(checkpoints[j].Timestamp)
	})

	return checkpoints, nil
}

// RestoreCheckpoint rolls back journal to the specified checkpoint state.
// Overwrites current index.jsonl and cursor.json with checkpoint snapshots.
// Does NOT affect tool-results/ directory (full outputs preserved).
// Does NOT delete checkpoint files (checkpoints are immutable).
// Returns error if checkpoint doesn't exist or restoration fails.
func (o *JournalObserver) RestoreCheckpoint(name string) error {
	checkpointsDir := filepath.Join(string(o.JournalDir), "checkpoints")
	metadataPath := filepath.Join(checkpointsDir, fmt.Sprintf("%s.json", name))

	// Verify checkpoint exists
	if _, err := os.Stat(metadataPath); os.IsNotExist(err) {
		return fmt.Errorf("checkpoint %q not found", name)
	}

	// Read checkpoint metadata
	data, err := os.ReadFile(metadataPath)
	if err != nil {
		return fmt.Errorf("failed to read checkpoint metadata: %w", err)
	}

	var checkpoint Checkpoint
	if err := json.Unmarshal(data, &checkpoint); err != nil {
		return fmt.Errorf("failed to parse checkpoint metadata: %w", err)
	}

	// Restore index.jsonl (delete target if snapshot doesn't exist - restoring to empty state)
	indexSnapshot := checkpoint.SnapshotPath + "-index.jsonl"
	if err := copyFileIfExists(indexSnapshot, string(o.IndexPath)); err != nil {
		if os.IsNotExist(err) {
			// Snapshot doesn't exist - restore to empty state by removing target
			os.Remove(string(o.IndexPath))
		} else {
			return fmt.Errorf("failed to restore index: %w", err)
		}
	}

	// Restore cursor.json (delete target if snapshot doesn't exist)
	cursorSnapshot := checkpoint.SnapshotPath + "-cursor.json"
	if err := copyFileIfExists(cursorSnapshot, string(o.CursorPath)); err != nil {
		if os.IsNotExist(err) {
			os.Remove(string(o.CursorPath))
		} else {
			return fmt.Errorf("failed to restore cursor: %w", err)
		}
	}

	// Restore recent.json (optional)
	recentSnapshot := checkpoint.SnapshotPath + "-recent.json"
	if err := copyFileIfExists(recentSnapshot, string(o.RecentPath)); err != nil {
		if os.IsNotExist(err) {
			os.Remove(string(o.RecentPath))
		}
	}

	return nil
}

// DeleteCheckpoint removes checkpoint metadata and snapshot files.
// Returns error if checkpoint doesn't exist or deletion fails.
func (o *JournalObserver) DeleteCheckpoint(name string) error {
	checkpointsDir := filepath.Join(string(o.JournalDir), "checkpoints")
	metadataPath := filepath.Join(checkpointsDir, fmt.Sprintf("%s.json", name))

	// Verify checkpoint exists
	if _, err := os.Stat(metadataPath); os.IsNotExist(err) {
		return fmt.Errorf("checkpoint %q not found", name)
	}

	// Read checkpoint metadata to get snapshot path
	data, err := os.ReadFile(metadataPath)
	if err != nil {
		return fmt.Errorf("failed to read checkpoint metadata: %w", err)
	}

	var checkpoint Checkpoint
	if err := json.Unmarshal(data, &checkpoint); err != nil {
		return fmt.Errorf("failed to parse checkpoint metadata: %w", err)
	}

	// Delete snapshot files
	snapshotFiles := []string{
		checkpoint.SnapshotPath + "-index.jsonl",
		checkpoint.SnapshotPath + "-cursor.json",
		checkpoint.SnapshotPath + "-recent.json",
	}

	for _, file := range snapshotFiles {
		if err := os.Remove(file); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("failed to delete snapshot file %s: %w", file, err)
		}
	}

	// Delete metadata file
	if err := os.Remove(metadataPath); err != nil {
		return fmt.Errorf("failed to delete checkpoint metadata: %w", err)
	}

	return nil
}

// countJSONLLines counts the number of lines in a JSONL file.
// Returns 0 if file doesn't exist (valid empty journal state).
func countJSONLLines(path string) (int, error) {
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}
	defer file.Close()

	count := 0
	buf := make([]byte, 32*1024) // 32KB buffer
	for {
		n, err := file.Read(buf)
		if n > 0 {
			for i := 0; i < n; i++ {
				if buf[i] == '\n' {
					count++
				}
			}
		}
		if err != nil {
			if err == io.EOF {
				break
			}
			return 0, err
		}
	}

	return count, nil
}

// copyFileIfExists copies src to dst. Returns error if src doesn't exist.
// Creates parent directories for dst if needed.
func copyFileIfExists(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	// Create parent directory for destination
	dstDir := filepath.Dir(dst)
	if err := os.MkdirAll(dstDir, 0755); err != nil {
		return err
	}

	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	_, err = io.Copy(dstFile, srcFile)
	return err
}
