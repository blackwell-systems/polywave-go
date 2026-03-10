package protocol

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// WriteCompletionMarker writes <!-- SAW:COMPLETE YYYY-MM-DD --> to an IMPL doc.
// For markdown files: inserts after the # IMPL: title line.
// For YAML files: sets the completion_date field.
//
// Returns an error if:
//   - File cannot be read or written
//   - Markdown file has no # IMPL: title line
//   - File extension is neither .md nor .yaml/.yml
func WriteCompletionMarker(implDocPath string, date string) error {
	// Determine file type by extension
	isMarkdown := strings.HasSuffix(implDocPath, ".md")
	isYAML := strings.HasSuffix(implDocPath, ".yaml") || strings.HasSuffix(implDocPath, ".yml")

	if !isMarkdown && !isYAML {
		return fmt.Errorf("unsupported file type: %s (expected .md or .yaml)", implDocPath)
	}

	if isYAML {
		return writeCompletionMarkerYAML(implDocPath, date)
	}
	return writeCompletionMarkerMarkdown(implDocPath, date)
}

// writeCompletionMarkerMarkdown inserts the completion marker after the # IMPL: title line.
func writeCompletionMarkerMarkdown(implDocPath string, date string) error {
	// Read the file
	f, err := os.Open(implDocPath)
	if err != nil {
		return fmt.Errorf("cannot open markdown file: %w", err)
	}
	defer f.Close()

	var lines []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("error reading markdown file: %w", err)
	}

	// Find the # IMPL: title line
	marker := fmt.Sprintf("<!-- SAW:COMPLETE %s -->", date)
	titleIdx := -1
	markerExists := false

	for i, line := range lines {
		if strings.HasPrefix(line, "# IMPL:") {
			titleIdx = i
		}
		if strings.Contains(line, "<!-- SAW:COMPLETE") {
			markerExists = true
		}
	}

	if titleIdx == -1 {
		return fmt.Errorf("no # IMPL: title line found in markdown file")
	}

	// If marker already exists, don't duplicate it
	if markerExists {
		return nil
	}

	// Insert marker after title line
	newLines := make([]string, 0, len(lines)+1)
	newLines = append(newLines, lines[:titleIdx+1]...)
	newLines = append(newLines, marker)
	newLines = append(newLines, lines[titleIdx+1:]...)

	// Write back
	content := strings.Join(newLines, "\n") + "\n"
	if err := os.WriteFile(implDocPath, []byte(content), 0644); err != nil {
		return fmt.Errorf("cannot write markdown file: %w", err)
	}

	return nil
}

// writeCompletionMarkerYAML sets the completion_date field in a YAML file.
func writeCompletionMarkerYAML(implDocPath string, date string) error {
	// Load manifest as IMPLManifest to preserve all fields
	manifest, err := Load(implDocPath)
	if err != nil {
		return fmt.Errorf("cannot load manifest: %w", err)
	}

	// Set completion_date field
	manifest.CompletionDate = date

	// Save back using Save() to preserve structure
	if err := Save(manifest, implDocPath); err != nil {
		return fmt.Errorf("cannot save manifest: %w", err)
	}

	return nil
}
