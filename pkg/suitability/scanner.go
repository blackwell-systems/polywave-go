package suitability

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/result"
)

var (
	funcPattern = regexp.MustCompile(`func\s+\w+`)
	todoPatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?i)TODO`),
		regexp.MustCompile(`(?i)FIXME`),
		regexp.MustCompile(`(?i)HACK`),
		regexp.MustCompile(`(?i)XXX`),
		regexp.MustCompile(`(?i)\bstub\b`),
		regexp.MustCompile(`(?i)not implemented`),
		regexp.MustCompile(`(?i)placeholder`),
	}
)

// ScanPreImplementation analyzes files against requirements to classify status
func ScanPreImplementation(repoRoot string, requirements []Requirement) result.Result[SuitabilityResult] {
	if repoRoot == "" {
		return result.NewFailure[SuitabilityResult]([]result.SAWError{
			{
				Code:     result.CodeSuitabilityRepoRootEmpty,
				Message:  "repo root cannot be empty",
				Severity: "fatal",
			},
		})
	}

	if _, err := os.Stat(repoRoot); err != nil {
		sawErr := result.SAWError{
			Code:     result.CodeSuitabilityFileStatFailed,
			Message:  fmt.Sprintf("repo root does not exist or is not accessible: %s", repoRoot),
			Severity: "fatal",
		}.WithCause(err)
		return result.NewFailure[SuitabilityResult]([]result.SAWError{sawErr})
	}

	scanResult := SuitabilityResult{
		PreImplementation: PreImplStatus{
			TotalItems: len(requirements),
			ItemStatus: make([]ItemStatus, 0, len(requirements)),
		},
	}

	for _, req := range requirements {
		status, err := classifyRequirement(repoRoot, req)
		if err != nil {
			sawErr := result.SAWError{
				Code:     result.CodeSuitabilityClassifyFailed,
				Message:  fmt.Sprintf("failed to classify requirement %s", req.ID),
				Severity: "fatal",
			}.WithCause(err)
			return result.NewFailure[SuitabilityResult]([]result.SAWError{sawErr})
		}

		scanResult.PreImplementation.ItemStatus = append(scanResult.PreImplementation.ItemStatus, status)

		switch status.Status {
		case "DONE":
			scanResult.PreImplementation.Done++
		case "PARTIAL":
			scanResult.PreImplementation.Partial++
		case "TODO":
			scanResult.PreImplementation.Todo++
		default:
			// Unknown or zero-value status: count as Todo to preserve
			// the Done+Partial+Todo==TotalItems invariant.
			scanResult.PreImplementation.Todo++
		}
	}

	// Calculate time saved: 7 min per DONE item, 3 min per PARTIAL item
	scanResult.PreImplementation.TimeSavedMinutes =
		(scanResult.PreImplementation.Done * 7) +
			(scanResult.PreImplementation.Partial * 3)

	return result.NewSuccess(scanResult)
}

// classifyRequirement analyzes a requirement across all its target files
func classifyRequirement(repoRoot string, req Requirement) (ItemStatus, error) {
	if len(req.Files) == 0 {
		return ItemStatus{
			ID:           req.ID,
			Status:       "TODO",
			Completeness: 0.0,
			Missing:      []string{"no files specified"},
		}, nil
	}

	// Analyze each file and aggregate results
	var fileStatuses []ItemStatus
	for _, file := range req.Files {
		status, err := ClassifyFile(filepath.Join(repoRoot, file), req)
		if err != nil {
			return ItemStatus{ID: req.ID, File: filepath.Join(repoRoot, file), Missing: []string{}}, err
		}
		fileStatuses = append(fileStatuses, status)
	}

	// Aggregate: use the worst status among all files
	return aggregateFileStatuses(req.ID, fileStatuses), nil
}

// aggregateFileStatuses combines multiple file statuses into a single ItemStatus
func aggregateFileStatuses(reqID string, statuses []ItemStatus) ItemStatus {
	if len(statuses) == 0 {
		return ItemStatus{
			ID:           reqID,
			Status:       "TODO",
			Completeness: 0.0,
			Missing:      []string{},
		}
	}

	// Calculate average completeness
	totalCompleteness := 0.0
	allMissing := []string{}
	worstStatus := "DONE"
	primaryFile := ""

	for _, s := range statuses {
		totalCompleteness += s.Completeness

		// Track worst status
		if s.Status == "TODO" {
			worstStatus = "TODO"
		} else if s.Status == "PARTIAL" && worstStatus != "TODO" {
			worstStatus = "PARTIAL"
		}

		// Collect missing items
		allMissing = append(allMissing, s.Missing...)

		// Use first file as primary
		if primaryFile == "" && s.File != "" {
			primaryFile = s.File
		}
	}

	avgCompleteness := totalCompleteness / float64(len(statuses))

	result := ItemStatus{
		ID:           reqID,
		Status:       worstStatus,
		File:         primaryFile,
		Completeness: avgCompleteness,
		Missing:      allMissing,
	}

	// Set test coverage to the worst (lowest) coverage across all files
	coverageRank := map[string]int{"none": 0, "low": 1, "medium": 2, "high": 3}
	worstCoverage := ""
	worstRank := 4 // higher than any valid rank
	for _, s := range statuses {
		if s.TestCoverage == "" {
			continue
		}
		rank, ok := coverageRank[s.TestCoverage]
		if !ok {
			continue
		}
		if worstCoverage == "" || rank < worstRank {
			worstCoverage = s.TestCoverage
			worstRank = rank
		}
	}
	if worstCoverage != "" {
		result.TestCoverage = worstCoverage
	}

	return result
}

// ClassifyFile determines if a file is DONE/PARTIAL/TODO for a requirement
func ClassifyFile(filePath string, req Requirement) (ItemStatus, error) {
	status := ItemStatus{
		ID:      req.ID,
		File:    filePath,
		Missing: []string{},
	}

	// Check if file exists
	content, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			status.Status = "TODO"
			status.Completeness = 0.0
			status.TestCoverage = "none"
			status.Missing = append(status.Missing, "file does not exist")
			return status, nil
		}
		return status, result.SAWError{
			Code:     result.CodeSuitabilityFileReadFailed,
			Message:  fmt.Sprintf("failed to read file %s", filePath),
			Severity: "fatal",
		}.WithCause(err)
	}

	contentStr := string(content)

	// Check for TODO/FIXME/stub patterns
	hasTodo := containsTodoPatterns(contentStr)

	// Check for test file
	testFile := inferTestFilePath(filePath)
	testCoverage := checkTestCoverage(testFile)
	status.TestCoverage = testCoverage

	// Check if file has meaningful implementation
	hasImplementation := hasSignificantContent(contentStr)

	// Classification logic
	if !hasImplementation {
		status.Status = "TODO"
		status.Completeness = 0.0
		status.Missing = append(status.Missing, "no implementation found")
	} else if hasTodo {
		status.Status = "PARTIAL"
		status.Completeness = 0.6 // Mid-range for partial
		status.Missing = append(status.Missing, "contains TODO/FIXME/stub markers")
	} else if testCoverage == "low" || testCoverage == "none" {
		status.Status = "PARTIAL"
		status.Completeness = 0.7
		status.Missing = append(status.Missing, "insufficient test coverage")
	} else {
		status.Status = "DONE"
		status.Completeness = 1.0
	}

	return status, nil
}

// containsTodoPatterns checks for TODO, FIXME, HACK, XXX, stub patterns
func containsTodoPatterns(content string) bool {
	for _, pattern := range todoPatterns {
		if pattern.MatchString(content) {
			return true
		}
	}
	return false
}

// hasSignificantContent checks if file has meaningful implementation
// (not just package declaration, imports, and comments)
func hasSignificantContent(content string) bool {
	// Quick heuristic: look for function declarations
	funcMatches := funcPattern.FindAllString(content, -1)

	// If we have at least one function, consider it implemented
	if len(funcMatches) > 0 {
		return true
	}

	// Fallback: count significant lines
	lines := strings.Split(content, "\n")
	significantLines := 0

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Skip empty lines, comments, package declarations, imports
		if trimmed == "" ||
			strings.HasPrefix(trimmed, "//") ||
			strings.HasPrefix(trimmed, "/*") ||
			strings.HasPrefix(trimmed, "*") ||
			strings.HasPrefix(trimmed, "package ") ||
			strings.HasPrefix(trimmed, "import ") ||
			trimmed == "import (" ||
			trimmed == ")" ||
			trimmed == "}" ||
			trimmed == "{" {
			continue
		}

		significantLines++
	}

	// Require at least 5 significant lines for "implementation"
	return significantLines >= 5
}

// inferTestFilePath converts a file path to its likely test file path
func inferTestFilePath(filePath string) string {
	dir := filepath.Dir(filePath)
	base := filepath.Base(filePath)
	ext := filepath.Ext(base)
	nameWithoutExt := strings.TrimSuffix(base, ext)

	return filepath.Join(dir, nameWithoutExt+"_test"+ext)
}

// checkTestCoverage returns a heuristic coverage level based on test file size
func checkTestCoverage(testFilePath string) string {
	content, err := os.ReadFile(testFilePath)
	if err != nil {
		return "none"
	}

	lines := strings.Split(string(content), "\n")
	lineCount := len(lines)

	// Heuristic: >100 lines = high coverage (>70%)
	// 50-100 lines = medium coverage (30-70%)
	// <50 lines = low coverage (<30%)
	if lineCount > 100 {
		return "high"
	} else if lineCount >= 50 {
		return "medium"
	} else {
		return "low"
	}
}
