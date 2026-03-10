package protocol

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
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
// Uses line-based manipulation to preserve 100% of original formatting, comments,
// block scalars, and indentation. This is necessary because yaml.v3's Marshal
// always reformats the entire document, losing all formatting details.
func writeCompletionMarkerYAML(implDocPath string, date string) error {
	// Read the file line by line
	f, err := os.Open(implDocPath)
	if err != nil {
		return fmt.Errorf("cannot open YAML file: %w", err)
	}
	defer f.Close()

	var lines []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("error reading YAML file: %w", err)
	}

	// Search for existing completion_date field at the top level (no indentation)
	completionDateIdx := -1
	for i, line := range lines {
		// Only match top-level keys (no leading whitespace before key)
		if strings.HasPrefix(line, "completion_date:") {
			completionDateIdx = i
			break
		}
	}

	// If found, update it; otherwise insert at the beginning
	if completionDateIdx != -1 {
		// Update existing field, preserving indentation style
		lines[completionDateIdx] = fmt.Sprintf("completion_date: %q", date)
	} else {
		// Insert at the beginning of the document
		lines = append([]string{fmt.Sprintf("completion_date: %q", date)}, lines...)
	}

	// Write back
	content := strings.Join(lines, "\n") + "\n"

	// Validate that the modified YAML is still parseable
	var testDoc yaml.Node
	if err := yaml.Unmarshal([]byte(content), &testDoc); err != nil {
		return fmt.Errorf("generated invalid YAML after adding completion_date: %w", err)
	}

	// Write to disk
	if err := os.WriteFile(implDocPath, []byte(content), 0644); err != nil {
		return fmt.Errorf("cannot write YAML file: %w", err)
	}

	return nil
}
