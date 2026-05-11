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

	"github.com/blackwell-systems/polywave-go/pkg/protocol"
	"github.com/blackwell-systems/polywave-go/pkg/result"
)

// ArchiveData holds the result data from a successful Archive operation.
type ArchiveData struct {
	ArchivePath         string    `json:"archive_path"`
	MetadataPath        string    `json:"metadata_path"`
	Wave                int       `json:"wave"`
	Agent               string    `json:"agent"`
	EntryCount          int       `json:"entry_count"`
	OriginalSizeBytes   int64     `json:"original_size_bytes"`
	CompressedSizeBytes int64     `json:"compressed_size_bytes"`
	CompressionRatio    float64   `json:"compression_ratio"`
	ArchivedAt          time.Time `json:"archived_at"`
}

// CleanupData holds the result data from a successful CleanupExpired operation.
type CleanupData struct {
	DeletedCount int `json:"deleted_count"`
}

// ExtractData holds the result data from a successful Extract operation.
type ExtractData struct {
	ArchivePath string `json:"archive_path"`
	DestPath    string `json:"dest_path"`
	FilesCount  int    `json:"files_count"`
}

// createTarData holds the result data from a successful createTarGz operation.
type createTarData struct {
	SourceDir  string `json:"source_dir"`
	TargetPath string `json:"target_path"`
}

// writeMetaData holds the result data from a successful writeMetadataAtomic operation.
type writeMetaData struct {
	Path string `json:"path"`
}

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
func (o *JournalObserver) Archive() result.Result[ArchiveData] {
	// Parse wave and agent from journal directory path
	// Expected format: .polywave-state/wave{N}/agent-{ID}
	parts := strings.Split(filepath.Clean(o.JournalDir), string(filepath.Separator))
	var wave int
	var agent string

	for _, part := range parts {
		if strings.HasPrefix(part, "wave") {
			fmt.Sscanf(part, "wave%d", &wave)
		}
		if strings.HasPrefix(part, "agent-") {
			agent = strings.TrimPrefix(part, "agent-")
		}
	}

	if wave == 0 || agent == "" {
		return result.NewFailure[ArchiveData]([]result.PolywaveError{{
			Code:     result.CodeJournalArchiveCreateFailed,
			Message:  fmt.Sprintf("cannot parse wave/agent from journal dir: %s", o.JournalDir),
			Severity: "fatal",
		}})
	}

	// Create archive directory if needed
	archiveDir := protocol.PolywaveStateArchiveDir(o.ProjectRoot)
	if err := os.MkdirAll(archiveDir, 0755); err != nil {
		return result.NewFailure[ArchiveData]([]result.PolywaveError{{
			Code:     result.CodeJournalArchiveCreateFailed,
			Message:  fmt.Sprintf("creating archive directory: %v", err),
			Severity: "fatal",
		}})
	}

	// Archive file path
	archiveName := fmt.Sprintf("wave%d-agent-%s.tar.gz", wave, agent)
	archivePath := filepath.Join(archiveDir, archiveName)
	metadataPath := filepath.Join(archiveDir, fmt.Sprintf("wave%d-agent-%s.json", wave, agent))

	// Check if archive already exists
	if _, err := os.Stat(archivePath); err == nil {
		return result.NewFailure[ArchiveData]([]result.PolywaveError{{
			Code:     result.CodeJournalArchiveCreateFailed,
			Message:  fmt.Sprintf("archive already exists: %s", archivePath),
			Severity: "fatal",
		}})
	}

	// Calculate original size and count entries
	originalSize, entryCount, err := calculateJournalStats(o.JournalDir)
	if err != nil {
		return result.NewFailure[ArchiveData]([]result.PolywaveError{{
			Code:     result.CodeJournalArchiveCreateFailed,
			Message:  fmt.Sprintf("calculating journal stats: %v", err),
			Severity: "fatal",
		}})
	}

	// Create tar.gz archive
	if r := createTarGz(o.JournalDir, archivePath); r.IsFatal() {
		return result.NewFailure[ArchiveData]([]result.PolywaveError{{
			Code:     result.CodeJournalArchiveCreateFailed,
			Message:  fmt.Sprintf("creating archive: %s", r.Errors[0].Message),
			Severity: "fatal",
		}})
	}

	// Get compressed size
	stat, err := os.Stat(archivePath)
	if err != nil {
		return result.NewFailure[ArchiveData]([]result.PolywaveError{{
			Code:     result.CodeJournalArchiveCreateFailed,
			Message:  fmt.Sprintf("stat archive: %v", err),
			Severity: "fatal",
		}})
	}
	compressedSize := stat.Size()

	// Calculate compression ratio
	compressionRatio := float64(originalSize) / float64(compressedSize)

	// Create metadata
	archivedAt := time.Now().UTC()
	metadata := ArchiveMetadata{
		Wave:                wave,
		Agent:               agent,
		ArchivedAt:          archivedAt,
		EntryCount:          entryCount,
		OriginalSizeBytes:   originalSize,
		CompressedSizeBytes: compressedSize,
		CompressionRatio:    compressionRatio,
	}

	// Write metadata atomically
	if r := writeMetadataAtomic(metadataPath, metadata); r.IsFatal() {
		// Clean up archive on metadata failure
		os.Remove(archivePath)
		return result.NewFailure[ArchiveData]([]result.PolywaveError{{
			Code:     result.CodeJournalArchiveMetaFailed,
			Message:  fmt.Sprintf("writing metadata: %s", r.Errors[0].Message),
			Severity: "fatal",
		}})
	}

	return result.NewSuccess(ArchiveData{
		ArchivePath:         archivePath,
		MetadataPath:        metadataPath,
		Wave:                wave,
		Agent:               agent,
		EntryCount:          entryCount,
		OriginalSizeBytes:   originalSize,
		CompressedSizeBytes: compressedSize,
		CompressionRatio:    compressionRatio,
		ArchivedAt:          archivedAt,
	})
}

// CleanupExpired deletes archives older than retention period
func CleanupExpired(repoPath string, retentionDays int) result.Result[CleanupData] {
	archiveDir := protocol.PolywaveStateArchiveDir(repoPath)

	// Read all archives
	listRes := ListArchives(repoPath)
	if listRes.IsFatal() {
		return result.NewFailure[CleanupData]([]result.PolywaveError{{
			Code:     result.CodeJournalArchiveCleanupFailed,
			Message:  fmt.Sprintf("listing archives: %s", listRes.Errors[0].Message),
			Severity: "fatal",
		}})
	}
	archives := *listRes.Data

	// Calculate cutoff time
	cutoff := time.Now().UTC().Add(-time.Duration(retentionDays) * 24 * time.Hour)

	// Delete expired archives
	deleted := 0
	for _, archive := range archives {
		if archive.ArchivedAt.Before(cutoff) {
			// Delete both .tar.gz and .json
			archiveName := fmt.Sprintf("wave%d-agent-%s.tar.gz", archive.Wave, archive.Agent)
			metadataName := fmt.Sprintf("wave%d-agent-%s.json", archive.Wave, archive.Agent)

			archivePath := filepath.Join(archiveDir, archiveName)
			metadataPath := filepath.Join(archiveDir, metadataName)

			if err := os.Remove(archivePath); err != nil && !os.IsNotExist(err) {
				return result.NewFailure[CleanupData]([]result.PolywaveError{{
					Code:     result.CodeJournalArchiveCleanupFailed,
					Message:  fmt.Sprintf("removing archive %s: %v", archivePath, err),
					Severity: "fatal",
				}})
			}
			if err := os.Remove(metadataPath); err != nil && !os.IsNotExist(err) {
				return result.NewFailure[CleanupData]([]result.PolywaveError{{
					Code:     result.CodeJournalArchiveCleanupFailed,
					Message:  fmt.Sprintf("removing metadata %s: %v", metadataPath, err),
					Severity: "fatal",
				}})
			}
			deleted++
		}
	}

	return result.NewSuccess(CleanupData{DeletedCount: deleted})
}

// ListArchives returns metadata for all archived journals, sorted by archived_at
func ListArchives(repoPath string) result.Result[[]ArchiveMetadata] {
	archiveDir := protocol.PolywaveStateArchiveDir(repoPath)

	// Check if archive directory exists
	if _, err := os.Stat(archiveDir); os.IsNotExist(err) {
		return result.NewSuccess([]ArchiveMetadata{})
	}

	// Read directory
	entries, err := os.ReadDir(archiveDir)
	if err != nil {
		return result.NewFailure[[]ArchiveMetadata]([]result.PolywaveError{{
			Code:     result.CodeJournalArchiveListFailed,
			Message:  fmt.Sprintf("reading archive directory: %v", err),
			Severity: "fatal",
		}})
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
			return result.NewFailure[[]ArchiveMetadata]([]result.PolywaveError{{
				Code:     result.CodeJournalArchiveListFailed,
				Message:  fmt.Sprintf("reading metadata %s: %v", entry.Name(), err),
				Severity: "fatal",
			}})
		}

		var metadata ArchiveMetadata
		if err := json.Unmarshal(data, &metadata); err != nil {
			return result.NewFailure[[]ArchiveMetadata]([]result.PolywaveError{{
				Code:     result.CodeJournalArchiveListFailed,
				Message:  fmt.Sprintf("parsing metadata %s: %v", entry.Name(), err),
				Severity: "fatal",
			}})
		}

		archives = append(archives, metadata)
	}

	// Sort by archived_at
	sort.Slice(archives, func(i, j int) bool {
		return archives[i].ArchivedAt.Before(archives[j].ArchivedAt)
	})

	return result.NewSuccess(archives)
}

// Extract decompresses an archive to a destination path
func Extract(archivePath, destPath string) result.Result[ExtractData] {
	// Open archive file
	archiveFile, err := os.Open(archivePath)
	if err != nil {
		return result.NewFailure[ExtractData]([]result.PolywaveError{{
			Code:     result.CodeJournalArchiveExtractFailed,
			Message:  fmt.Sprintf("opening archive: %v", err),
			Severity: "fatal",
		}})
	}
	defer archiveFile.Close()

	// Create gzip reader
	gzipReader, err := gzip.NewReader(archiveFile)
	if err != nil {
		return result.NewFailure[ExtractData]([]result.PolywaveError{{
			Code:     result.CodeJournalArchiveExtractFailed,
			Message:  fmt.Sprintf("creating gzip reader: %v", err),
			Severity: "fatal",
		}})
	}
	defer gzipReader.Close()

	// Create tar reader
	tarReader := tar.NewReader(gzipReader)

	// Extract files
	filesCount := 0
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return result.NewFailure[ExtractData]([]result.PolywaveError{{
				Code:     result.CodeJournalArchiveExtractFailed,
				Message:  fmt.Sprintf("reading tar header: %v", err),
				Severity: "fatal",
			}})
		}

		// Construct target path
		target := filepath.Join(destPath, header.Name)

		// Ensure target is within destPath (security)
		if !strings.HasPrefix(filepath.Clean(target), filepath.Clean(destPath)) {
			return result.NewFailure[ExtractData]([]result.PolywaveError{{
				Code:     result.CodeJournalArchiveExtractFailed,
				Message:  fmt.Sprintf("invalid file path in archive: %s", header.Name),
				Severity: "fatal",
			}})
		}

		switch header.Typeflag {
		case tar.TypeDir:
			// Create directory
			if err := os.MkdirAll(target, os.FileMode(header.Mode)); err != nil {
				return result.NewFailure[ExtractData]([]result.PolywaveError{{
					Code:     result.CodeJournalArchiveExtractFailed,
					Message:  fmt.Sprintf("creating directory %s: %v", target, err),
					Severity: "fatal",
				}})
			}

		case tar.TypeReg:
			// Create parent directories
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return result.NewFailure[ExtractData]([]result.PolywaveError{{
					Code:     result.CodeJournalArchiveExtractFailed,
					Message:  fmt.Sprintf("creating parent directory for %s: %v", target, err),
					Severity: "fatal",
				}})
			}

			// Create file
			outFile, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(header.Mode))
			if err != nil {
				return result.NewFailure[ExtractData]([]result.PolywaveError{{
					Code:     result.CodeJournalArchiveExtractFailed,
					Message:  fmt.Sprintf("creating file %s: %v", target, err),
					Severity: "fatal",
				}})
			}

			// Copy content
			if _, err := io.Copy(outFile, tarReader); err != nil {
				outFile.Close()
				return result.NewFailure[ExtractData]([]result.PolywaveError{{
					Code:     result.CodeJournalArchiveExtractFailed,
					Message:  fmt.Sprintf("writing file %s: %v", target, err),
					Severity: "fatal",
				}})
			}
			outFile.Close()
			filesCount++
		}
	}

	return result.NewSuccess(ExtractData{
		ArchivePath: archivePath,
		DestPath:    destPath,
		FilesCount:  filesCount,
	})
}

// Helper: createTarGz creates a tar.gz archive from a directory
func createTarGz(sourceDir, targetPath string) result.Result[createTarData] {
	// Create target file
	outFile, err := os.Create(targetPath)
	if err != nil {
		return result.NewFailure[createTarData]([]result.PolywaveError{{
			Code:     result.CodeJournalArchiveCreateFailed,
			Message:  fmt.Sprintf("creating output file: %v", err),
			Severity: "fatal",
		}})
	}
	defer outFile.Close()

	// Create gzip writer
	gzipWriter := gzip.NewWriter(outFile)
	defer gzipWriter.Close()

	// Create tar writer
	tarWriter := tar.NewWriter(gzipWriter)
	defer tarWriter.Close()

	// Walk source directory
	walkErr := filepath.Walk(sourceDir, func(path string, info os.FileInfo, err error) error {
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
	if walkErr != nil {
		return result.NewFailure[createTarData]([]result.PolywaveError{{
			Code:     result.CodeJournalArchiveCreateFailed,
			Message:  walkErr.Error(),
			Severity: "fatal",
		}})
	}
	return result.NewSuccess(createTarData{
		SourceDir:  sourceDir,
		TargetPath: targetPath,
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
func writeMetadataAtomic(path string, metadata ArchiveMetadata) result.Result[writeMetaData] {
	// Marshal metadata
	data, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return result.NewFailure[writeMetaData]([]result.PolywaveError{{
			Code:     result.CodeJournalArchiveMetaFailed,
			Message:  fmt.Sprintf("marshaling metadata: %v", err),
			Severity: "fatal",
		}})
	}

	// Write to temp file
	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		return result.NewFailure[writeMetaData]([]result.PolywaveError{{
			Code:     result.CodeJournalArchiveMetaFailed,
			Message:  fmt.Sprintf("writing temp file: %v", err),
			Severity: "fatal",
		}})
	}

	// Rename atomically
	if err := os.Rename(tmpPath, path); err != nil {
		os.Remove(tmpPath)
		return result.NewFailure[writeMetaData]([]result.PolywaveError{{
			Code:     result.CodeJournalArchiveMetaFailed,
			Message:  fmt.Sprintf("renaming temp file: %v", err),
			Severity: "fatal",
		}})
	}

	return result.NewSuccess(writeMetaData{Path: path})
}
