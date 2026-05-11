package suitability

import (
	"os"
	"regexp"
	"strings"

	"github.com/blackwell-systems/polywave-go/pkg/result"
)

var (
	headerPattern   = regexp.MustCompile(`(?m)^##\s+([A-Za-z0-9_-]+):\s*(.+)$`)
	locationPattern = regexp.MustCompile(`(?m)^Location:\s*(.+)$`)
	plainPattern    = regexp.MustCompile(`^([A-Za-z0-9_-]+):\s*([^|]+)\|\s*(.+)$`)
)

// AnalyzeSuitability is a convenience wrapper for engine integration.
// It parses requirements from the given file and returns suitability results.
// If requirementsFile is empty or doesn't exist, returns success with zero-initialized struct.
func AnalyzeSuitability(requirementsFile string, repoRoot string) result.Result[SuitabilityResult] {
	if requirementsFile == "" {
		return result.NewSuccess(SuitabilityResult{})
	}

	// Check if file exists
	if _, err := os.Stat(requirementsFile); os.IsNotExist(err) {
		return result.NewSuccess(SuitabilityResult{}) // Non-fatal: requirements file not found
	} else if err != nil {
		sawErr := result.PolywaveError{
			Code:     result.CodeSuitabilityFileStatFailed,
			Message:  "failed to stat requirements file",
			Severity: "fatal",
		}.WithCause(err)
		return result.NewFailure[SuitabilityResult]([]result.PolywaveError{sawErr})
	}

	// Read requirements document
	reqData, err := os.ReadFile(requirementsFile)
	if err != nil {
		sawErr := result.PolywaveError{
			Code:     result.CodeSuitabilityRequirementsRead,
			Message:  "failed to read requirements file",
			Severity: "fatal",
		}.WithCause(err)
		return result.NewFailure[SuitabilityResult]([]result.PolywaveError{sawErr})
	}

	// Parse requirements
	reqResult := ParseRequirements(string(reqData))
	if reqResult.IsFatal() {
		return result.NewFailure[SuitabilityResult](reqResult.Errors)
	}
	requirements := reqResult.GetData()

	if len(requirements) == 0 {
		return result.NewSuccess(SuitabilityResult{}) // No requirements found, not an error
	}

	// Scan pre-implementation status
	return ScanPreImplementation(repoRoot, requirements)
}

// ParseRequirements extracts structured requirements from markdown or plain text.
// Expected format:
//
//	## F1: Add authentication handler
//	Location: pkg/auth/handler.go
//
//	## SEC-01: Add session timeout
//	Location: pkg/session/timeout.go
//
// Returns a slice of Requirement structs with ID, Description, and Files populated.
func ParseRequirements(content string) result.Result[[]Requirement] {
	var requirements []Requirement

	// Pattern 1: Markdown headers with "Location:" field
	// ## F1: Description
	// Location: path/to/file.go

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

			requirements = append(requirements, Requirement{
				ID:          id,
				Description: description,
				Files:       files, // may be nil if no Location found
			})
		}
	}

	// Pattern 2: Plain text format (fallback)
	// F1: Description | path/to/file.go
	if len(requirements) == 0 {
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if matches := plainPattern.FindStringSubmatch(line); matches != nil {
				id := strings.TrimSpace(matches[1])
				description := strings.TrimSpace(matches[2])
				filePath := strings.TrimSpace(matches[3])

				if id != "" && filePath != "" {
					requirements = append(requirements, Requirement{
						ID:          id,
						Description: description,
						Files:       []string{filePath},
					})
				}
			}
		}
	}

	return result.NewSuccess(requirements)
}
