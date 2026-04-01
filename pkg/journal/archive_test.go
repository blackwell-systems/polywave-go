package journal

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestArchive_CreatesTarGz(t *testing.T) {
	// Setup temp directory structure
	tmpDir := t.TempDir()
	journalDir := filepath.Join(tmpDir, ".saw-state", "wave1", "agent-A")
	if err := os.MkdirAll(journalDir, 0755); err != nil {
		t.Fatalf("creating journal dir: %v", err)
	}

	// Create some journal files
	if err := os.WriteFile(filepath.Join(journalDir, "index.jsonl"), []byte("line1\nline2\nline3\n"), 0644); err != nil {
		t.Fatalf("creating index.jsonl: %v", err)
	}
	if err := os.WriteFile(filepath.Join(journalDir, "cursor.json"), []byte(`{"session":"test"}`), 0644); err != nil {
		t.Fatalf("creating cursor.json: %v", err)
	}

	// Create observer
	observer := &JournalObserver{
		ProjectRoot: tmpDir,
		JournalDir:  journalDir,
		AgentID:     "A",
	}

	// Archive
	r := observer.Archive()
	if !r.IsSuccess() {
		t.Fatalf("Archive failed: %v", r.Errors[0].Message)
	}

	// Verify archive file exists
	archivePath := filepath.Join(tmpDir, ".saw-state", "archive", "wave1-agent-A.tar.gz")
	if _, err := os.Stat(archivePath); os.IsNotExist(err) {
		t.Fatalf("archive file not created: %s", archivePath)
	}

	// Verify result data
	data := r.GetData()
	if data.Wave != 1 {
		t.Errorf("expected wave 1, got %d", data.Wave)
	}
	if data.Agent != "A" {
		t.Errorf("expected agent A, got %s", data.Agent)
	}
}

func TestArchive_SavesMetadata(t *testing.T) {
	// Setup temp directory structure
	tmpDir := t.TempDir()
	journalDir := filepath.Join(tmpDir, ".saw-state", "wave2", "agent-B")
	if err := os.MkdirAll(journalDir, 0755); err != nil {
		t.Fatalf("creating journal dir: %v", err)
	}

	// Create journal file
	if err := os.WriteFile(filepath.Join(journalDir, "index.jsonl"), []byte("entry1\nentry2\n"), 0644); err != nil {
		t.Fatalf("creating index.jsonl: %v", err)
	}

	// Create observer
	observer := &JournalObserver{
		ProjectRoot: tmpDir,
		JournalDir:  journalDir,
		AgentID:     "B",
	}

	// Archive
	r := observer.Archive()
	if !r.IsSuccess() {
		t.Fatalf("Archive failed: %v", r.Errors[0].Message)
	}

	// Read metadata
	metadataPath := filepath.Join(tmpDir, ".saw-state", "archive", "wave2-agent-B.json")
	data, err := os.ReadFile(metadataPath)
	if err != nil {
		t.Fatalf("reading metadata: %v", err)
	}

	var metadata ArchiveMetadata
	if err := json.Unmarshal(data, &metadata); err != nil {
		t.Fatalf("parsing metadata: %v", err)
	}

	// Verify metadata fields
	if metadata.Wave != 2 {
		t.Errorf("expected wave 2, got %d", metadata.Wave)
	}
	if metadata.Agent != "B" {
		t.Errorf("expected agent B, got %s", metadata.Agent)
	}
	if metadata.EntryCount != 2 {
		t.Errorf("expected 2 entries, got %d", metadata.EntryCount)
	}
	if metadata.OriginalSizeBytes == 0 {
		t.Error("expected non-zero original size")
	}
	if metadata.CompressedSizeBytes == 0 {
		t.Error("expected non-zero compressed size")
	}
	if time.Since(metadata.ArchivedAt) > 5*time.Second {
		t.Error("archived_at timestamp seems wrong")
	}
}

func TestArchive_CalculatesCompressionRatio(t *testing.T) {
	// Setup temp directory structure
	tmpDir := t.TempDir()
	journalDir := filepath.Join(tmpDir, ".saw-state", "wave1", "agent-C")
	if err := os.MkdirAll(journalDir, 0755); err != nil {
		t.Fatalf("creating journal dir: %v", err)
	}

	// Create a large compressible file (repeated text compresses well)
	largeContent := strings.Repeat("This is a repeated line for compression testing.\n", 1000)
	if err := os.WriteFile(filepath.Join(journalDir, "index.jsonl"), []byte(largeContent), 0644); err != nil {
		t.Fatalf("creating index.jsonl: %v", err)
	}

	// Create observer
	observer := &JournalObserver{
		ProjectRoot: tmpDir,
		JournalDir:  journalDir,
		AgentID:     "C",
	}

	// Archive
	r := observer.Archive()
	if !r.IsSuccess() {
		t.Fatalf("Archive failed: %v", r.Errors[0].Message)
	}

	// Read metadata
	metadataPath := filepath.Join(tmpDir, ".saw-state", "archive", "wave1-agent-C.json")
	data, err := os.ReadFile(metadataPath)
	if err != nil {
		t.Fatalf("reading metadata: %v", err)
	}

	var metadata ArchiveMetadata
	if err := json.Unmarshal(data, &metadata); err != nil {
		t.Fatalf("parsing metadata: %v", err)
	}

	// Verify compression ratio
	if metadata.CompressionRatio < 1.0 {
		t.Errorf("compression ratio should be >= 1.0, got %.2f", metadata.CompressionRatio)
	}

	// Verify it matches calculation
	expectedRatio := float64(metadata.OriginalSizeBytes) / float64(metadata.CompressedSizeBytes)
	if metadata.CompressionRatio != expectedRatio {
		t.Errorf("compression ratio %.2f doesn't match expected %.2f", metadata.CompressionRatio, expectedRatio)
	}

	// For repeated text, we should get good compression (>5x)
	if metadata.CompressionRatio < 5.0 {
		t.Logf("Warning: expected better compression ratio for repeated text, got %.2f", metadata.CompressionRatio)
	}
}

func TestListArchives_ReturnsAll(t *testing.T) {
	tmpDir := t.TempDir()
	archiveDir := filepath.Join(tmpDir, ".saw-state", "archive")
	if err := os.MkdirAll(archiveDir, 0755); err != nil {
		t.Fatalf("creating archive dir: %v", err)
	}

	// Create multiple metadata files with different timestamps
	archives := []ArchiveMetadata{
		{Wave: 1, Agent: "A", ArchivedAt: time.Now().UTC().Add(-2 * time.Hour), EntryCount: 10, OriginalSizeBytes: 1000, CompressedSizeBytes: 100, CompressionRatio: 10.0},
		{Wave: 1, Agent: "B", ArchivedAt: time.Now().UTC().Add(-1 * time.Hour), EntryCount: 20, OriginalSizeBytes: 2000, CompressedSizeBytes: 200, CompressionRatio: 10.0},
		{Wave: 2, Agent: "A", ArchivedAt: time.Now().UTC(), EntryCount: 30, OriginalSizeBytes: 3000, CompressedSizeBytes: 300, CompressionRatio: 10.0},
	}

	for _, archive := range archives {
		filename := fmt.Sprintf("wave%d-agent-%s.json", archive.Wave, archive.Agent)
		metadataPath := filepath.Join(archiveDir, filename)
		data, _ := json.Marshal(archive)
		if err := os.WriteFile(metadataPath, data, 0644); err != nil {
			t.Fatalf("writing metadata: %v", err)
		}
	}

	// List archives
	result, err := ListArchives(tmpDir)
	if err != nil {
		t.Fatalf("ListArchives failed: %v", err)
	}

	// Verify count
	if len(result) != 3 {
		t.Fatalf("expected 3 archives, got %d", len(result))
	}

	// Verify sorted by archived_at (oldest first)
	for i := 1; i < len(result); i++ {
		if result[i].ArchivedAt.Before(result[i-1].ArchivedAt) {
			t.Error("archives not sorted by archived_at")
		}
	}
}

func TestExtract_DecompressesArchive(t *testing.T) {
	// Setup temp directory structure
	tmpDir := t.TempDir()
	journalDir := filepath.Join(tmpDir, ".saw-state", "wave1", "agent-D")
	if err := os.MkdirAll(journalDir, 0755); err != nil {
		t.Fatalf("creating journal dir: %v", err)
	}

	// Create journal files with specific content
	indexContent := "entry1\nentry2\nentry3\n"
	cursorContent := `{"session":"test","offset":42}`

	if err := os.WriteFile(filepath.Join(journalDir, "index.jsonl"), []byte(indexContent), 0644); err != nil {
		t.Fatalf("creating index.jsonl: %v", err)
	}
	if err := os.WriteFile(filepath.Join(journalDir, "cursor.json"), []byte(cursorContent), 0644); err != nil {
		t.Fatalf("creating cursor.json: %v", err)
	}

	// Create subdirectory with file
	checkpointDir := filepath.Join(journalDir, "checkpoints")
	if err := os.MkdirAll(checkpointDir, 0755); err != nil {
		t.Fatalf("creating checkpoint dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(checkpointDir, "checkpoint1.json"), []byte(`{"name":"test"}`), 0644); err != nil {
		t.Fatalf("creating checkpoint file: %v", err)
	}

	// Create observer and archive
	observer := &JournalObserver{
		ProjectRoot: tmpDir,
		JournalDir:  journalDir,
		AgentID:     "D",
	}

	r := observer.Archive()
	if !r.IsSuccess() {
		t.Fatalf("Archive failed: %v", r.Errors[0].Message)
	}

	// Extract to new location
	extractDir := filepath.Join(tmpDir, "extracted")
	archivePath := filepath.Join(tmpDir, ".saw-state", "archive", "wave1-agent-D.tar.gz")

	re := Extract(archivePath, extractDir)
	if !re.IsSuccess() {
		t.Fatalf("Extract failed: %v", re.Errors[0].Message)
	}

	// Verify extracted files match originals
	extractedIndex, err := os.ReadFile(filepath.Join(extractDir, "index.jsonl"))
	if err != nil {
		t.Fatalf("reading extracted index.jsonl: %v", err)
	}
	if string(extractedIndex) != indexContent {
		t.Errorf("extracted index.jsonl content doesn't match")
	}

	extractedCursor, err := os.ReadFile(filepath.Join(extractDir, "cursor.json"))
	if err != nil {
		t.Fatalf("reading extracted cursor.json: %v", err)
	}
	if string(extractedCursor) != cursorContent {
		t.Errorf("extracted cursor.json content doesn't match")
	}

	// Verify subdirectory was extracted
	extractedCheckpoint, err := os.ReadFile(filepath.Join(extractDir, "checkpoints", "checkpoint1.json"))
	if err != nil {
		t.Fatalf("reading extracted checkpoint: %v", err)
	}
	if string(extractedCheckpoint) != `{"name":"test"}` {
		t.Errorf("extracted checkpoint content doesn't match")
	}
}

func TestCleanupExpired_DeletesOldArchives(t *testing.T) {
	tmpDir := t.TempDir()
	archiveDir := filepath.Join(tmpDir, ".saw-state", "archive")
	if err := os.MkdirAll(archiveDir, 0755); err != nil {
		t.Fatalf("creating archive dir: %v", err)
	}

	// Create old archive (45 days ago)
	oldMetadata := ArchiveMetadata{
		Wave:                1,
		Agent:               "A",
		ArchivedAt:          time.Now().UTC().Add(-45 * 24 * time.Hour),
		EntryCount:          10,
		OriginalSizeBytes:   1000,
		CompressedSizeBytes: 100,
		CompressionRatio:    10.0,
	}

	oldArchivePath := filepath.Join(archiveDir, "wave1-agent-A.tar.gz")
	oldMetadataPath := filepath.Join(archiveDir, "wave1-agent-A.json")

	// Create dummy archive file
	if err := os.WriteFile(oldArchivePath, []byte("dummy"), 0644); err != nil {
		t.Fatalf("creating old archive: %v", err)
	}

	// Write metadata
	data, _ := json.Marshal(oldMetadata)
	if err := os.WriteFile(oldMetadataPath, data, 0644); err != nil {
		t.Fatalf("writing old metadata: %v", err)
	}

	// Cleanup with 30 day retention
	r := CleanupExpired(tmpDir, 30)
	if !r.IsSuccess() {
		t.Fatalf("CleanupExpired failed: %v", r.Errors[0].Message)
	}

	// Verify files were deleted
	if _, err := os.Stat(oldArchivePath); !os.IsNotExist(err) {
		t.Error("old archive should be deleted")
	}
	if _, err := os.Stat(oldMetadataPath); !os.IsNotExist(err) {
		t.Error("old metadata should be deleted")
	}

	// Verify result data
	cd := r.GetData()
	if cd.DeletedCount != 1 {
		t.Errorf("expected 1 deleted, got %d", cd.DeletedCount)
	}
}

func TestCleanupExpired_PreservesRecent(t *testing.T) {
	tmpDir := t.TempDir()
	archiveDir := filepath.Join(tmpDir, ".saw-state", "archive")
	if err := os.MkdirAll(archiveDir, 0755); err != nil {
		t.Fatalf("creating archive dir: %v", err)
	}

	// Create recent archive (10 days ago)
	recentMetadata := ArchiveMetadata{
		Wave:                1,
		Agent:               "B",
		ArchivedAt:          time.Now().UTC().Add(-10 * 24 * time.Hour),
		EntryCount:          20,
		OriginalSizeBytes:   2000,
		CompressedSizeBytes: 200,
		CompressionRatio:    10.0,
	}

	recentArchivePath := filepath.Join(archiveDir, "wave1-agent-B.tar.gz")
	recentMetadataPath := filepath.Join(archiveDir, "wave1-agent-B.json")

	// Create dummy archive file
	if err := os.WriteFile(recentArchivePath, []byte("dummy"), 0644); err != nil {
		t.Fatalf("creating recent archive: %v", err)
	}

	// Write metadata
	data, _ := json.Marshal(recentMetadata)
	if err := os.WriteFile(recentMetadataPath, data, 0644); err != nil {
		t.Fatalf("writing recent metadata: %v", err)
	}

	// Cleanup with 30 day retention
	r := CleanupExpired(tmpDir, 30)
	if !r.IsSuccess() {
		t.Fatalf("CleanupExpired failed: %v", r.Errors[0].Message)
	}

	// Verify files still exist
	if _, err := os.Stat(recentArchivePath); os.IsNotExist(err) {
		t.Error("recent archive should be preserved")
	}
	if _, err := os.Stat(recentMetadataPath); os.IsNotExist(err) {
		t.Error("recent metadata should be preserved")
	}
}

func TestArchive_CountsEntries(t *testing.T) {
	// Setup temp directory structure
	tmpDir := t.TempDir()
	journalDir := filepath.Join(tmpDir, ".saw-state", "wave1", "agent-E")
	if err := os.MkdirAll(journalDir, 0755); err != nil {
		t.Fatalf("creating journal dir: %v", err)
	}

	// Create index.jsonl with known number of lines
	lines := []string{
		`{"kind":"tool_use","tool":"Read"}`,
		`{"kind":"tool_result","tool_use_id":"123"}`,
		`{"kind":"tool_use","tool":"Write"}`,
		`{"kind":"tool_result","tool_use_id":"456"}`,
		`{"kind":"tool_use","tool":"Bash"}`,
	}
	content := strings.Join(lines, "\n") + "\n"

	if err := os.WriteFile(filepath.Join(journalDir, "index.jsonl"), []byte(content), 0644); err != nil {
		t.Fatalf("creating index.jsonl: %v", err)
	}

	// Create observer and archive
	observer := &JournalObserver{
		ProjectRoot: tmpDir,
		JournalDir:  journalDir,
		AgentID:     "E",
	}

	r := observer.Archive()
	if !r.IsSuccess() {
		t.Fatalf("Archive failed: %v", r.Errors[0].Message)
	}

	// Read metadata
	metadataPath := filepath.Join(tmpDir, ".saw-state", "archive", "wave1-agent-E.json")
	data, err := os.ReadFile(metadataPath)
	if err != nil {
		t.Fatalf("reading metadata: %v", err)
	}

	var metadata ArchiveMetadata
	if err := json.Unmarshal(data, &metadata); err != nil {
		t.Fatalf("parsing metadata: %v", err)
	}

	// Verify entry count
	expectedCount := len(lines)
	if metadata.EntryCount != expectedCount {
		t.Errorf("expected %d entries, got %d", expectedCount, metadata.EntryCount)
	}
}

func TestArchive_IncludesCheckpoints(t *testing.T) {
	// Setup temp directory structure
	tmpDir := t.TempDir()
	journalDir := filepath.Join(tmpDir, ".saw-state", "wave1", "agent-F")
	if err := os.MkdirAll(journalDir, 0755); err != nil {
		t.Fatalf("creating journal dir: %v", err)
	}

	// Create checkpoints subdirectory with files
	checkpointDir := filepath.Join(journalDir, "checkpoints")
	if err := os.MkdirAll(checkpointDir, 0755); err != nil {
		t.Fatalf("creating checkpoint dir: %v", err)
	}

	checkpointFiles := []string{"checkpoint1.json", "checkpoint2.json", "checkpoint3.json"}
	for _, filename := range checkpointFiles {
		content := `{"name":"` + filename + `"}`
		if err := os.WriteFile(filepath.Join(checkpointDir, filename), []byte(content), 0644); err != nil {
			t.Fatalf("creating checkpoint file: %v", err)
		}
	}

	// Create index.jsonl
	if err := os.WriteFile(filepath.Join(journalDir, "index.jsonl"), []byte("entry1\n"), 0644); err != nil {
		t.Fatalf("creating index.jsonl: %v", err)
	}

	// Create observer and archive
	observer := &JournalObserver{
		ProjectRoot: tmpDir,
		JournalDir:  journalDir,
		AgentID:     "F",
	}

	r := observer.Archive()
	if !r.IsSuccess() {
		t.Fatalf("Archive failed: %v", r.Errors[0].Message)
	}

	// Extract and verify checkpoints are included
	extractDir := filepath.Join(tmpDir, "extracted")
	archivePath := filepath.Join(tmpDir, ".saw-state", "archive", "wave1-agent-F.tar.gz")

	re := Extract(archivePath, extractDir)
	if !re.IsSuccess() {
		t.Fatalf("Extract failed: %v", re.Errors[0].Message)
	}

	// Verify all checkpoint files exist
	for _, filename := range checkpointFiles {
		extractedPath := filepath.Join(extractDir, "checkpoints", filename)
		if _, err := os.Stat(extractedPath); os.IsNotExist(err) {
			t.Errorf("checkpoint file %s not included in archive", filename)
		}
	}
}

func TestArchive_ExcludesToolResults(t *testing.T) {
	// Setup temp directory structure
	tmpDir := t.TempDir()
	journalDir := filepath.Join(tmpDir, ".saw-state", "wave1", "agent-G")
	if err := os.MkdirAll(journalDir, 0755); err != nil {
		t.Fatalf("creating journal dir: %v", err)
	}

	// Create tool-results subdirectory with large files
	resultsDir := filepath.Join(journalDir, "tool-results")
	if err := os.MkdirAll(resultsDir, 0755); err != nil {
		t.Fatalf("creating tool-results dir: %v", err)
	}

	largeContent := strings.Repeat("x", 100000) // 100KB
	if err := os.WriteFile(filepath.Join(resultsDir, "result1.txt"), []byte(largeContent), 0644); err != nil {
		t.Fatalf("creating result file: %v", err)
	}

	// Create index.jsonl (should be included)
	if err := os.WriteFile(filepath.Join(journalDir, "index.jsonl"), []byte("entry1\n"), 0644); err != nil {
		t.Fatalf("creating index.jsonl: %v", err)
	}

	// Create observer and archive
	observer := &JournalObserver{
		ProjectRoot: tmpDir,
		JournalDir:  journalDir,
		AgentID:     "G",
	}

	r := observer.Archive()
	if !r.IsSuccess() {
		t.Fatalf("Archive failed: %v", r.Errors[0].Message)
	}

	// Extract and verify tool-results is excluded
	extractDir := filepath.Join(tmpDir, "extracted")
	archivePath := filepath.Join(tmpDir, ".saw-state", "archive", "wave1-agent-G.tar.gz")

	re := Extract(archivePath, extractDir)
	if !re.IsSuccess() {
		t.Fatalf("Extract failed: %v", re.Errors[0].Message)
	}

	// Verify index.jsonl exists
	if _, err := os.Stat(filepath.Join(extractDir, "index.jsonl")); os.IsNotExist(err) {
		t.Error("index.jsonl should be included in archive")
	}

	// Verify tool-results directory does NOT exist
	if _, err := os.Stat(filepath.Join(extractDir, "tool-results")); !os.IsNotExist(err) {
		t.Error("tool-results directory should be excluded from archive")
	}
}

func TestArchive_ReturnsErrorIfExists(t *testing.T) {
	// Setup temp directory structure
	tmpDir := t.TempDir()
	journalDir := filepath.Join(tmpDir, ".saw-state", "wave1", "agent-H")
	archiveDir := filepath.Join(tmpDir, ".saw-state", "archive")
	if err := os.MkdirAll(journalDir, 0755); err != nil {
		t.Fatalf("creating journal dir: %v", err)
	}
	if err := os.MkdirAll(archiveDir, 0755); err != nil {
		t.Fatalf("creating archive dir: %v", err)
	}

	// Create journal file
	if err := os.WriteFile(filepath.Join(journalDir, "index.jsonl"), []byte("entry1\n"), 0644); err != nil {
		t.Fatalf("creating index.jsonl: %v", err)
	}

	// Pre-create archive file
	existingArchivePath := filepath.Join(archiveDir, "wave1-agent-H.tar.gz")
	if err := os.WriteFile(existingArchivePath, []byte("existing"), 0644); err != nil {
		t.Fatalf("creating existing archive: %v", err)
	}

	// Create observer
	observer := &JournalObserver{
		ProjectRoot: tmpDir,
		JournalDir:  journalDir,
		AgentID:     "H",
	}

	// Attempt archive - should fail
	r := observer.Archive()
	if r.IsSuccess() {
		t.Fatal("Archive should fail when archive already exists")
	}
	if !r.IsFatal() {
		t.Error("Expected IsFatal() to be true")
	}
	if !strings.Contains(r.Errors[0].Message, "already exists") {
		t.Errorf("expected 'already exists' error, got: %v", r.Errors[0].Message)
	}
}

func TestCleanupExpired_Idempotent(t *testing.T) {
	tmpDir := t.TempDir()
	archiveDir := filepath.Join(tmpDir, ".saw-state", "archive")
	if err := os.MkdirAll(archiveDir, 0755); err != nil {
		t.Fatalf("creating archive dir: %v", err)
	}

	// Create old archive
	oldMetadata := ArchiveMetadata{
		Wave:                1,
		Agent:               "I",
		ArchivedAt:          time.Now().UTC().Add(-45 * 24 * time.Hour),
		EntryCount:          10,
		OriginalSizeBytes:   1000,
		CompressedSizeBytes: 100,
		CompressionRatio:    10.0,
	}

	oldArchivePath := filepath.Join(archiveDir, "wave1-agent-I.tar.gz")
	oldMetadataPath := filepath.Join(archiveDir, "wave1-agent-I.json")

	if err := os.WriteFile(oldArchivePath, []byte("dummy"), 0644); err != nil {
		t.Fatalf("creating old archive: %v", err)
	}
	data, _ := json.Marshal(oldMetadata)
	if err := os.WriteFile(oldMetadataPath, data, 0644); err != nil {
		t.Fatalf("writing old metadata: %v", err)
	}

	// Run cleanup multiple times
	for i := 0; i < 3; i++ {
		r := CleanupExpired(tmpDir, 30)
		if !r.IsSuccess() {
			t.Fatalf("CleanupExpired run %d failed: %v", i+1, r.Errors[0].Message)
		}
	}

	// Verify files are still deleted (no errors from missing files)
	if _, err := os.Stat(oldArchivePath); !os.IsNotExist(err) {
		t.Error("old archive should remain deleted after multiple cleanups")
	}
}

func TestListArchives_EmptyDirectory(t *testing.T) {
	tmpDir := t.TempDir()

	// List archives in empty directory (archive dir doesn't even exist)
	result, err := ListArchives(tmpDir)
	if err != nil {
		t.Fatalf("ListArchives failed on empty directory: %v", err)
	}

	if len(result) != 0 {
		t.Errorf("expected 0 archives, got %d", len(result))
	}
}

func TestArchive_ResultDataMatchesMetadata(t *testing.T) {
	// Verify that the ArchiveData returned matches the written metadata
	tmpDir := t.TempDir()
	journalDir := filepath.Join(tmpDir, ".saw-state", "wave3", "agent-Z")
	if err := os.MkdirAll(journalDir, 0755); err != nil {
		t.Fatalf("creating journal dir: %v", err)
	}

	if err := os.WriteFile(filepath.Join(journalDir, "index.jsonl"), []byte("e1\ne2\ne3\n"), 0644); err != nil {
		t.Fatalf("creating index.jsonl: %v", err)
	}

	observer := &JournalObserver{
		ProjectRoot: tmpDir,
		JournalDir:  journalDir,
		AgentID:     "Z",
	}

	r := observer.Archive()
	if !r.IsSuccess() {
		t.Fatalf("Archive failed: %v", r.Errors[0].Message)
	}

	archiveData := r.GetData()
	if archiveData.Wave != 3 {
		t.Errorf("wave = %d, want 3", archiveData.Wave)
	}
	if archiveData.Agent != "Z" {
		t.Errorf("agent = %s, want Z", archiveData.Agent)
	}
	if archiveData.OriginalSizeBytes == 0 {
		t.Error("expected non-zero original size")
	}
	if archiveData.CompressedSizeBytes == 0 {
		t.Error("expected non-zero compressed size")
	}
	if archiveData.ArchivedAt.IsZero() {
		t.Error("expected non-zero archived_at")
	}
}

func TestArchive_PathParsingExact(t *testing.T) {
	// Verify that agent-C2 is parsed correctly from wave3/agent-C2 path
	tmpDir := t.TempDir()
	journalDir := filepath.Join(tmpDir, ".saw-state", "wave3", "agent-C2")
	if err := os.MkdirAll(journalDir, 0755); err != nil {
		t.Fatalf("creating journal dir: %v", err)
	}

	// Create a non-empty index.jsonl
	if err := os.WriteFile(filepath.Join(journalDir, "index.jsonl"), []byte("entry1\nentry2\n"), 0644); err != nil {
		t.Fatalf("creating index.jsonl: %v", err)
	}

	observer := &JournalObserver{
		ProjectRoot: tmpDir,
		JournalDir:  journalDir,
		AgentID:     "C2",
	}

	r := observer.Archive()
	if !r.IsSuccess() {
		t.Fatalf("Archive failed: %v", r.Errors[0].Message)
	}

	data := r.GetData()
	if data.Wave != 3 {
		t.Errorf("expected wave 3, got %d", data.Wave)
	}
	if data.Agent != "C2" {
		t.Errorf("expected agent C2, got %s", data.Agent)
	}
}

func TestArchive_PathParsingNoFallback(t *testing.T) {
	// Verify that a directory following "wave1" that does NOT start with "agent-"
	// does NOT set the agent (so Archive returns a parse failure).
	tmpDir := t.TempDir()
	journalDir := filepath.Join(tmpDir, ".saw-state", "wave1", "somedir")
	if err := os.MkdirAll(journalDir, 0755); err != nil {
		t.Fatalf("creating journal dir: %v", err)
	}

	// Create a non-empty index.jsonl
	if err := os.WriteFile(filepath.Join(journalDir, "index.jsonl"), []byte("entry1\n"), 0644); err != nil {
		t.Fatalf("creating index.jsonl: %v", err)
	}

	observer := &JournalObserver{
		ProjectRoot: tmpDir,
		JournalDir:  journalDir,
		AgentID:     "somedir",
	}

	r := observer.Archive()
	if r.IsSuccess() {
		t.Fatal("Archive should fail when agent dir does not use agent- prefix; got success with agent=" + r.GetData().Agent)
	}
	if !r.IsFatal() {
		t.Error("expected IsFatal() to be true")
	}
	if !strings.Contains(r.Errors[0].Message, "cannot parse wave/agent") {
		t.Errorf("expected 'cannot parse wave/agent' error, got: %v", r.Errors[0].Message)
	}
}

func TestExtract_ReturnsData(t *testing.T) {
	// Verify Extract result data is populated
	tmpDir := t.TempDir()
	journalDir := filepath.Join(tmpDir, ".saw-state", "wave1", "agent-X")
	if err := os.MkdirAll(journalDir, 0755); err != nil {
		t.Fatalf("creating journal dir: %v", err)
	}

	if err := os.WriteFile(filepath.Join(journalDir, "index.jsonl"), []byte("entry\n"), 0644); err != nil {
		t.Fatalf("creating index.jsonl: %v", err)
	}
	if err := os.WriteFile(filepath.Join(journalDir, "cursor.json"), []byte("{}"), 0644); err != nil {
		t.Fatalf("creating cursor.json: %v", err)
	}

	observer := &JournalObserver{
		ProjectRoot: tmpDir,
		JournalDir:  journalDir,
		AgentID:     "X",
	}

	ar := observer.Archive()
	if !ar.IsSuccess() {
		t.Fatalf("Archive failed: %v", ar.Errors[0].Message)
	}

	extractDir := filepath.Join(tmpDir, "extracted")
	archivePath := filepath.Join(tmpDir, ".saw-state", "archive", "wave1-agent-X.tar.gz")

	er := Extract(archivePath, extractDir)
	if !er.IsSuccess() {
		t.Fatalf("Extract failed: %v", er.Errors[0].Message)
	}

	ed := er.GetData()
	if ed.ArchivePath != archivePath {
		t.Errorf("ArchivePath = %s, want %s", ed.ArchivePath, archivePath)
	}
	if ed.DestPath != extractDir {
		t.Errorf("DestPath = %s, want %s", ed.DestPath, extractDir)
	}
	if ed.FilesCount == 0 {
		t.Error("expected FilesCount > 0")
	}
}
