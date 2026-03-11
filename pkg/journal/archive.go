package journal

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// ArchiveMetadata holds metadata about an archived journal
type ArchiveMetadata struct {
	Wave                int       `json:"wave"`
	Agent               string    `json:"agent"`
	ArchivedAt          time.Time `json:"archived_at"`
	EntryCount          int       `json:"entry_count"`
	OriginalSizeBytes   int64     `json:"original_size_bytes"`
	CompressedSizeBytes int64     `json:"compressed_size_bytes"`
	CompressionRatio    float64   `json:"compression_ratio"`
}

// Archive compresses the journal directory to .tar.gz in archive subdirectory
func (o *JournalObserver) Archive() error {
	// Parse wave and agent from journal directory path
	// Expected format: .saw-state/wave{N}/agent-{ID}
	parts := strings.Split(filepath.Clean(o.JournalDir), string(filepath.Separator))
	var wave int
	var agent string

	for i, part := range parts {
		if strings.HasPrefix(part, "wave") {
			fmt.Sscanf(part, "wave%d", &wave)
		}
		if strings.HasPrefix(part, "agent-") {
			agent = strings.TrimPrefix(part, "agent-")
		}
		// Also check if part matches just the agent ID
		if i > 0 && strings.HasPrefix(parts[i-1], "wave") {
			if !strings.HasPrefix(part, "agent-") {
				agent = part
			}
		}
	}

	if wave == 0 || agent == "" {
		return fmt.Errorf("cannot parse wave/agent from journal dir: %s", o.JournalDir)
	}

	// Create archive directory if needed
	archiveDir := filepath.Join(o.ProjectRoot, ".saw-state", "archive")
	if err := os.MkdirAll(archiveDir, 0755); err != nil {
		return fmt.Errorf("creating archive directory: %w", err)
	}

	// Archive file path
	archiveName := fmt.Sprintf("wave%d-agent-%s.tar.gz", wave, agent)
	archivePath := filepath.Join(archiveDir, archiveName)
	metadataPath := filepath.Join(archiveDir, fmt.Sprintf("wave%d-agent-%s.json", wave, agent))

	// Check if archive already exists
	if _, err := os.Stat(archivePath); err == nil {
		return fmt.Errorf("archive already exists: %s", archivePath)
	}

	// Calculate original size and count entries
	originalSize, entryCount, err := calculateJournalStats(o.JournalDir)
	if err != nil {
		return fmt.Errorf("calculating journal stats: %w", err)
	}

	// Create tar.gz archive
	if err := createTarGz(o.JournalDir, archivePath); err != nil {
		return fmt.Errorf("creating archive: %w", err)
	}

	// Get compressed size
	stat, err := os.Stat(archivePath)
	if err != nil {
		return fmt.Errorf("stat archive: %w", err)
	}
	compressedSize := stat.Size()

	// Calculate compression ratio
	compressionRatio := float64(originalSize) / float64(compressedSize)

	// Create metadata
	metadata := ArchiveMetadata{
		Wave:                wave,
		Agent:               agent,
		ArchivedAt:          time.Now().UTC(),
		EntryCount:          entryCount,
		OriginalSizeBytes:   originalSize,
		CompressedSizeBytes: compressedSize,
		CompressionRatio:    compressionRatio,
	}

	// Write metadata atomically
	if err := writeMetadataAtomic(metadataPath, metadata); err != nil {
		// Clean up archive on metadata failure
		os.Remove(archivePath)
		return fmt.Errorf("writing metadata: %w", err)
	}

	return nil
}

// CleanupExpired deletes archives older than retention period
func CleanupExpired(repoPath string, retentionDays int) error {
	archiveDir := filepath.Join(repoPath, ".saw-state", "archive")

	// Read all archives
	archives, err := ListArchives(repoPath)
	if err != nil {
		return fmt.Errorf("listing archives: %w", err)
	}

	// Calculate cutoff time
	cutoff := time.Now().UTC().Add(-time.Duration(retentionDays) * 24 * time.Hour)

	// Delete expired archives
	for _, archive := range archives {
		if archive.ArchivedAt.Before(cutoff) {
			// Delete both .tar.gz and .json
			archiveName := fmt.Sprintf("wave%d-agent-%s.tar.gz", archive.Wave, archive.Agent)
			metadataName := fmt.Sprintf("wave%d-agent-%s.json", archive.Wave, archive.Agent)

			archivePath := filepath.Join(archiveDir, archiveName)
			metadataPath := filepath.Join(archiveDir, metadataName)

			if err := os.Remove(archivePath); err != nil && !os.IsNotExist(err) {
				return fmt.Errorf("removing archive %s: %w", archivePath, err)
			}
			if err := os.Remove(metadataPath); err != nil && !os.IsNotExist(err) {
				return fmt.Errorf("removing metadata %s: %w", metadataPath, err)
			}
		}
	}

	return nil
}

// ListArchives returns metadata for all archived journals, sorted by archived_at
func ListArchives(repoPath string) ([]ArchiveMetadata, error) {
	archiveDir := filepath.Join(repoPath, ".saw-state", "archive")

	// Check if archive directory exists
	if _, err := os.Stat(archiveDir); os.IsNotExist(err) {
		return []ArchiveMetadata{}, nil
	}

	// Read directory
	entries, err := os.ReadDir(archiveDir)
	if err != nil {
		return nil, fmt.Errorf("reading archive directory: %w", err)
	}

	// Collect metadata files
	var archives []ArchiveMetadata
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		metadataPath := filepath.Join(archiveDir, entry.Name())
		data, err := os.ReadFile(metadataPath)
		if err != nil {
			return nil, fmt.Errorf("reading metadata %s: %w", entry.Name(), err)
		}

		var metadata ArchiveMetadata
		if err := json.Unmarshal(data, &metadata); err != nil {
			return nil, fmt.Errorf("parsing metadata %s: %w", entry.Name(), err)
		}

		archives = append(archives, metadata)
	}

	// Sort by archived_at
	sort.Slice(archives, func(i, j int) bool {
		return archives[i].ArchivedAt.Before(archives[j].ArchivedAt)
	})

	return archives, nil
}

// Extract decompresses an archive to a destination path
func Extract(archivePath, destPath string) error {
	// Open archive file
	archiveFile, err := os.Open(archivePath)
	if err != nil {
		return fmt.Errorf("opening archive: %w", err)
	}
	defer archiveFile.Close()

	// Create gzip reader
	gzipReader, err := gzip.NewReader(archiveFile)
	if err != nil {
		return fmt.Errorf("creating gzip reader: %w", err)
	}
	defer gzipReader.Close()

	// Create tar reader
	tarReader := tar.NewReader(gzipReader)

	// Extract files
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("reading tar header: %w", err)
		}

		// Construct target path
		target := filepath.Join(destPath, header.Name)

		// Ensure target is within destPath (security)
		if !strings.HasPrefix(filepath.Clean(target), filepath.Clean(destPath)) {
			return fmt.Errorf("invalid file path in archive: %s", header.Name)
		}

		switch header.Typeflag {
		case tar.TypeDir:
			// Create directory
			if err := os.MkdirAll(target, os.FileMode(header.Mode)); err != nil {
				return fmt.Errorf("creating directory %s: %w", target, err)
			}

		case tar.TypeReg:
			// Create parent directories
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return fmt.Errorf("creating parent directory for %s: %w", target, err)
			}

			// Create file
			outFile, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(header.Mode))
			if err != nil {
				return fmt.Errorf("creating file %s: %w", target, err)
			}

			// Copy content
			if _, err := io.Copy(outFile, tarReader); err != nil {
				outFile.Close()
				return fmt.Errorf("writing file %s: %w", target, err)
			}
			outFile.Close()
		}
	}

	return nil
}

// Helper: createTarGz creates a tar.gz archive from a directory
func createTarGz(sourceDir, targetPath string) error {
	// Create target file
	outFile, err := os.Create(targetPath)
	if err != nil {
		return fmt.Errorf("creating output file: %w", err)
	}
	defer outFile.Close()

	// Create gzip writer
	gzipWriter := gzip.NewWriter(outFile)
	defer gzipWriter.Close()

	// Create tar writer
	tarWriter := tar.NewWriter(gzipWriter)
	defer tarWriter.Close()

	// Walk source directory
	return filepath.Walk(sourceDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip tool-results directory (large files, low value)
		if info.IsDir() && info.Name() == "tool-results" {
			return filepath.SkipDir
		}

		// Get relative path for tar entry
		relPath, err := filepath.Rel(sourceDir, path)
		if err != nil {
			return fmt.Errorf("calculating relative path: %w", err)
		}

		// Skip root directory itself
		if relPath == "." {
			return nil
		}

		// Create tar header
		header, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return fmt.Errorf("creating tar header for %s: %w", path, err)
		}
		header.Name = relPath

		// Write header
		if err := tarWriter.WriteHeader(header); err != nil {
			return fmt.Errorf("writing tar header for %s: %w", path, err)
		}

		// Write file content (skip directories)
		if !info.IsDir() {
			file, err := os.Open(path)
			if err != nil {
				return fmt.Errorf("opening file %s: %w", path, err)
			}
			defer file.Close()

			if _, err := io.Copy(tarWriter, file); err != nil {
				return fmt.Errorf("writing file content for %s: %w", path, err)
			}
		}

		return nil
	})
}

// Helper: calculateJournalStats counts entries and calculates total size
func calculateJournalStats(journalDir string) (totalSize int64, entryCount int, err error) {
	// Count lines in index.jsonl
	indexPath := filepath.Join(journalDir, "index.jsonl")
	if _, statErr := os.Stat(indexPath); statErr == nil {
		data, readErr := os.ReadFile(indexPath)
		if readErr != nil {
			return 0, 0, fmt.Errorf("reading index.jsonl: %w", readErr)
		}
		entryCount = strings.Count(string(data), "\n")
	}

	// Calculate total size (excluding tool-results)
	err = filepath.Walk(journalDir, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		// Skip tool-results directory
		if info.IsDir() && info.Name() == "tool-results" {
			return filepath.SkipDir
		}

		if !info.IsDir() {
			totalSize += info.Size()
		}
		return nil
	})

	return totalSize, entryCount, err
}

// Helper: writeMetadataAtomic writes metadata file atomically
func writeMetadataAtomic(path string, metadata ArchiveMetadata) error {
	// Marshal metadata
	data, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling metadata: %w", err)
	}

	// Write to temp file
	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		return fmt.Errorf("writing temp file: %w", err)
	}

	// Rename atomically
	if err := os.Rename(tmpPath, path); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("renaming temp file: %w", err)
	}

	return nil
}
