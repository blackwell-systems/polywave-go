package protocol

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// WriteCompletionMarker writes completion date to a YAML IMPL manifest.
// Sets the completion_date field in the YAML file.
//
// Returns an error if:
//   - File cannot be read or written
//   - File extension is not .yaml or .yml
func WriteCompletionMarker(implDocPath string, date string) error {
	if !strings.HasSuffix(implDocPath, ".yaml") && !strings.HasSuffix(implDocPath, ".yml") {
		return fmt.Errorf("unsupported file type: %s (expected .yaml)", implDocPath)
	}
	return writeCompletionMarkerYAML(implDocPath, date)
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

	// Also set state to COMPLETE
	stateIdx := -1
	for i, line := range lines {
		if strings.HasPrefix(line, "state:") {
			stateIdx = i
			break
		}
	}
	if stateIdx != -1 {
		lines[stateIdx] = "state: COMPLETE"
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
