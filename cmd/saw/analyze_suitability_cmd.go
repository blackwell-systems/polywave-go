package main

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/suitability"
	"github.com/spf13/cobra"
)

func newAnalyzeSuitabilityCmd() *cobra.Command {
	var requirementsFlag string
	var repoRootFlag string
	var outputFlag string

	cmd := &cobra.Command{
		Use:   "analyze-suitability",
		Short: "Scan codebase for pre-implemented requirements",
		Long: `Analyzes a requirements or audit document to determine which items are
already implemented (DONE), partially implemented (PARTIAL), or not yet
implemented (TODO).

The requirements document should contain structured items in markdown format:

  ## F1: Add authentication handler
  Location: pkg/auth/handler.go

  ## F2: Add session timeout
  Location: pkg/session/timeout.go

Each item must include a "Location:" field specifying the file path.

Output is JSON with status, test coverage, and time savings estimates.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Read requirements document
			reqData, err := os.ReadFile(requirementsFlag)
			if err != nil {
				return fmt.Errorf("read requirements doc: %w", err)
			}

			// Parse requirements
			requirements, err := parseRequirements(string(reqData))
			if err != nil {
				return fmt.Errorf("parse requirements: %w", err)
			}

			if len(requirements) == 0 {
				return fmt.Errorf("no valid requirements found in %s", requirementsFlag)
			}

			// Scan pre-implementation status
			result, err := suitability.ScanPreImplementation(repoRootFlag, requirements)
			if err != nil {
				return fmt.Errorf("scan pre-implementation: %w", err)
			}

			// Marshal to JSON
			var data []byte
			switch outputFlag {
			case "json":
				data, err = json.MarshalIndent(result, "", "  ")
				if err != nil {
					return fmt.Errorf("marshal JSON: %w", err)
				}
			default:
				return fmt.Errorf("unsupported output format: %s (only json supported)", outputFlag)
			}

			fmt.Fprintln(cmd.OutOrStdout(), string(data))
			return nil
		},
	}

	cmd.Flags().StringVar(&requirementsFlag, "requirements", "", "Path to requirements/audit doc (markdown format)")
	cmd.Flags().StringVar(&repoRootFlag, "repo-root", ".", "Repository root directory")
	cmd.Flags().StringVar(&outputFlag, "output", "json", "Output format (json only)")
	cmd.MarkFlagRequired("requirements")

	return cmd
}

// parseRequirements extracts structured requirements from markdown or plain text.
// Expected format:
//
//	## F1: Add authentication handler
//	Location: pkg/auth/handler.go
//
//	## SEC-01: Add session timeout
//	Location: pkg/session/timeout.go
//
// Returns a slice of Requirement structs with ID, Description, and Files populated.
func parseRequirements(content string) ([]suitability.Requirement, error) {
	var requirements []suitability.Requirement

	// Pattern 1: Markdown headers with "Location:" field
	// ## F1: Description
	// Location: path/to/file.go
	headerPattern := regexp.MustCompile(`(?m)^##\s+([A-Za-z0-9_-]+):\s*(.+)$`)
	locationPattern := regexp.MustCompile(`(?m)^Location:\s*(.+)$`)

	lines := strings.Split(content, "\n")

	for i := 0; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])

		// Match requirement header
		if matches := headerPattern.FindStringSubmatch(line); matches != nil {
			id := strings.TrimSpace(matches[1])
			description := strings.TrimSpace(matches[2])

			// Look ahead for Location field (within next 10 lines)
			var files []string
			for j := i + 1; j < len(lines) && j < i+10; j++ {
				locationLine := strings.TrimSpace(lines[j])

				// Stop if we hit another header
				if strings.HasPrefix(locationLine, "##") {
					break
				}

				// Extract location
				if locMatches := locationPattern.FindStringSubmatch(locationLine); locMatches != nil {
					filePath := strings.TrimSpace(locMatches[1])
					if filePath != "" {
						files = append(files, filePath)
					}
				}
			}

			// Only add requirement if it has at least one file location
			if len(files) > 0 {
				requirements = append(requirements, suitability.Requirement{
					ID:          id,
					Description: description,
					Files:       files,
				})
			}
		}
	}

	// Pattern 2: Plain text format (fallback)
	// F1: Description | path/to/file.go
	if len(requirements) == 0 {
		plainPattern := regexp.MustCompile(`^([A-Za-z0-9_-]+):\s*([^|]+)\|\s*(.+)$`)
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if matches := plainPattern.FindStringSubmatch(line); matches != nil {
				id := strings.TrimSpace(matches[1])
				description := strings.TrimSpace(matches[2])
				filePath := strings.TrimSpace(matches[3])

				if id != "" && filePath != "" {
					requirements = append(requirements, suitability.Requirement{
						ID:          id,
						Description: description,
						Files:       []string{filePath},
					})
				}
			}
		}
	}

	return requirements, nil
}
