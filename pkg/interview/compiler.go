package interview

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/blackwell-systems/polywave-go/pkg/result"
)

const placeholder = "<!-- placeholder — fill in before running /polywave bootstrap -->"

// CompileToRequirements converts a completed InterviewDoc into the
// REQUIREMENTS.md format expected by saw-bootstrap.md Phase 0.
// Returns the rendered markdown string.
func CompileToRequirements(doc *InterviewDoc) result.Result[string] {
	if doc == nil {
		return result.NewFailure[string]([]result.PolywaveError{
			result.NewFatal(result.CodeRequirementsWriteFailed, "interview doc is nil"),
		})
	}

	var b strings.Builder

	// Title
	title := doc.SpecData.Overview.Title
	if title == "" {
		title = doc.Slug
	}
	fmt.Fprintf(&b, "# Requirements: %s\n\n", title)

	// Language & Ecosystem
	b.WriteString("## Language & Ecosystem\n")
	b.WriteString(sectionFromSlice(doc.SpecData.Interfaces.External, "language"))
	b.WriteString("\n")

	// Project Type
	b.WriteString("## Project Type\n")
	b.WriteString(sectionFromString(doc.SpecData.Overview.Goal))
	b.WriteString("\n")

	// Deployment Target
	b.WriteString("## Deployment Target\n")
	b.WriteString(sectionFromSlice(doc.SpecData.Requirements.Constraints, "deploy"))
	b.WriteString("\n")

	// Key Concerns
	b.WriteString("## Key Concerns (3-6 major responsibility areas)\n")
	if len(doc.SpecData.Scope.InScope) > 0 {
		for i, item := range doc.SpecData.Scope.InScope {
			fmt.Fprintf(&b, "%d. %s\n", i+1, item)
		}
	} else {
		b.WriteString(placeholder)
		b.WriteString("\n")
	}
	b.WriteString("\n")

	// Storage
	b.WriteString("## Storage\n")
	b.WriteString(sectionFromSlice(doc.SpecData.Interfaces.DataModels, ""))
	b.WriteString("\n")

	// External Integrations
	b.WriteString("## External Integrations\n")
	if len(doc.SpecData.Interfaces.External) > 0 {
		for _, item := range doc.SpecData.Interfaces.External {
			fmt.Fprintf(&b, "- %s\n", item)
		}
	} else {
		b.WriteString(placeholder)
		b.WriteString("\n")
	}
	b.WriteString("\n")

	// Source Codebase
	b.WriteString("## Source Codebase (if porting/adapting)\n")
	sourceItems := filterPrefix(doc.SpecData.Interfaces.External, "source:")
	if len(sourceItems) > 0 {
		for _, item := range sourceItems {
			fmt.Fprintf(&b, "- %s\n", strings.TrimPrefix(item, "source:"))
		}
	} else {
		b.WriteString(placeholder)
		b.WriteString("\n")
	}
	b.WriteString("\n")

	// Architectural Decisions Already Made
	b.WriteString("## Architectural Decisions Already Made\n")
	decisions := append(
		append([]string{}, doc.SpecData.Requirements.Constraints...),
		doc.SpecData.Requirements.NonFunctional...,
	)
	if len(decisions) > 0 {
		for _, item := range decisions {
			fmt.Fprintf(&b, "- %s\n", item)
		}
	} else {
		b.WriteString(placeholder)
		b.WriteString("\n")
	}

	// Truncation detection: if max_questions was hit and required fields are missing
	truncated := doc.QuestionCursor >= doc.MaxQuestions
	missingRequired := doc.SpecData.Overview.Title == "" ||
		doc.SpecData.Overview.Goal == "" ||
		doc.SpecData.Scope.InScope == nil ||
		len(doc.SpecData.Requirements.Functional) == 0
	if truncated && missingRequired {
		b.WriteString("## Warnings\n")
		b.WriteString("- Interview truncated at max_questions limit. Some phases incomplete.\n")
	}

	return result.NewSuccess(b.String())
}

// WriteRequirementsFile writes the compiled REQUIREMENTS.md to disk.
// Returns a Result containing the output path and line count on success,
// or a FATAL result with code CodeRequirementsWriteFailed on failure.
func WriteRequirementsFile(doc *InterviewDoc, outputPath string) result.Result[WriteReqData] {
	compileResult := CompileToRequirements(doc)
	if compileResult.IsFatal() {
		return result.NewFailure[WriteReqData]([]result.PolywaveError{
			result.NewFatal(result.CodeRequirementsWriteFailed,
				fmt.Sprintf("compiling requirements: %s", compileResult.Errors[0].Message)).
				WithContext("path", outputPath),
		})
	}
	content := *compileResult.Data

	dir := filepath.Dir(outputPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return result.NewFailure[WriteReqData]([]result.PolywaveError{
			result.NewFatal(result.CodeRequirementsWriteFailed, fmt.Sprintf("creating output directory: %s", err.Error())).
				WithContext("path", outputPath).
				WithCause(err),
		})
	}

	if err := os.WriteFile(outputPath, []byte(content), 0o644); err != nil {
		return result.NewFailure[WriteReqData]([]result.PolywaveError{
			result.NewFatal(result.CodeRequirementsWriteFailed, fmt.Sprintf("writing requirements file: %s", err.Error())).
				WithContext("path", outputPath).
				WithCause(err),
		})
	}

	lineCount := strings.Count(content, "\n")
	return result.NewSuccess(WriteReqData{
		OutputPath: outputPath,
		LineCount:  lineCount,
	})
}

// sectionFromString renders a single string value, or a placeholder if empty.
func sectionFromString(val string) string {
	if val == "" {
		return placeholder + "\n"
	}
	return val + "\n"
}

// sectionFromSlice renders a bulleted list from a slice, or a placeholder if
// empty. If filterKeyword is non-empty, it only uses items containing that
// keyword (case-insensitive).
func sectionFromSlice(items []string, filterKeyword string) string {
	var filtered []string
	for _, item := range items {
		if filterKeyword == "" || strings.Contains(strings.ToLower(item), strings.ToLower(filterKeyword)) {
			filtered = append(filtered, item)
		}
	}
	if len(filtered) == 0 {
		return placeholder + "\n"
	}
	var b strings.Builder
	for _, item := range filtered {
		fmt.Fprintf(&b, "- %s\n", item)
	}
	return b.String()
}

// filterPrefix returns items with the given prefix.
func filterPrefix(items []string, prefix string) []string {
	var result []string
	for _, item := range items {
		if strings.HasPrefix(item, prefix) {
			result = append(result, item)
		}
	}
	return result
}
