package errparse

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/result"
)

// ──────────────────────────────────────────────
// PytestParser
// ──────────────────────────────────────────────

// PytestParser parses output from the pytest test runner.
type PytestParser struct{}

// Compiled regexes for pytest
var (
	// FAILED tests/test_foo.py::TestClass::test_name - AssertionError: ...
	pytestFailedRe = regexp.MustCompile(`^FAILED\s+(\S+?::[\S:]+?)(?:\s+-\s+(.+))?$`)

	// ERROR collecting tests/test_foo.py
	pytestCollectionErrorRe = regexp.MustCompile(`^ERROR\s+collecting\s+(\S+)`)

	// Traceback file reference: "  File "path/to/file.py", line 42, in ..."
	pytestTracebackFileRe = regexp.MustCompile(`^\s+File\s+"([^"]+)",\s+line\s+(\d+)`)

	// Short test location reference used in pytest output:  "tests/test_foo.py:42: AssertionError"
	pytestLocationRe = regexp.MustCompile(`^(\S+\.py):(\d+):\s+(.+)$`)
)

// Name returns the tool identifier.
func (p *PytestParser) Name() string { return "pytest" }

// Parse extracts structured errors from pytest stdout/stderr.
func (p *PytestParser) Parse(stdout, stderr string) *ParseResult {
	combinedStr := stdout
	if stderr != "" {
		if combinedStr != "" {
			combinedStr += "\n" + stderr
		} else {
			combinedStr = stderr
		}
	}

	pr := &ParseResult{
		Tool:   "pytest",
		Errors: []result.SAWError{},
		Raw:    combinedStr,
	}

	lines := strings.Split(combinedStr, "\n")

	// We do two passes:
	// 1. Collect traceback file references so we can attach the deepest one.
	// 2. Parse FAILED / ERROR lines and annotate with traceback context.

	// Keep track of traceback entries per section (reset at each FAILED line).
	type traceRef struct {
		file string
		line int
	}

	var currentTracebacks []traceRef
	var pendingFailed *result.SAWError // the FAILED line we last saw (by position)

	for i, raw := range lines {
		line := strings.TrimRight(raw, "\r")
		_ = i

		// Traceback file reference
		if m := pytestTracebackFileRe.FindStringSubmatch(line); m != nil {
			lineNum, _ := strconv.Atoi(m[2])
			currentTracebacks = append(currentTracebacks, traceRef{file: m[1], line: lineNum})
			continue
		}

		// Short location reference  (file.py:42: AssertionError)
		if m := pytestLocationRe.FindStringSubmatch(line); m != nil {
			lineNum, _ := strconv.Atoi(m[2])
			// If we have a pending FAILED entry, annotate it
			if pendingFailed != nil {
				if pendingFailed.File == "" || pendingFailed.File == extractPytestFile(pendingFailed.Message) {
					pendingFailed.File = m[1]
					pendingFailed.Line = lineNum
				}
			}
			continue
		}

		// FAILED line
		if m := pytestFailedRe.FindStringSubmatch(line); m != nil {
			// Flush traceback info into the previous pending item if any
			if pendingFailed != nil && len(currentTracebacks) > 0 {
				deepest := currentTracebacks[len(currentTracebacks)-1]
				if pendingFailed.File == "" {
					pendingFailed.File = deepest.file
					pendingFailed.Line = deepest.line
				}
			}

			testID := m[1]
			msg := m[2]
			if msg == "" {
				msg = testID
			}

			// Extract file from the test node id (before ::)
			file := extractPytestFile(testID)

			se := result.SAWError{
				Code:     result.CodeToolError,
				File:     file,
				Severity: "error",
				Message:  msg,
				Tool:     "pytest",
				Context:  makeContext(0, testID),
			}

			// Attach deepest traceback reference collected so far
			if len(currentTracebacks) > 0 {
				deepest := currentTracebacks[len(currentTracebacks)-1]
				se.File = deepest.file
				se.Line = deepest.line
			}

			pr.Errors = append(pr.Errors, se)
			pendingFailed = &pr.Errors[len(pr.Errors)-1]
			currentTracebacks = nil
			continue
		}

		// ERROR collecting ...
		if m := pytestCollectionErrorRe.FindStringSubmatch(line); m != nil {
			// Flush previous
			if pendingFailed != nil && len(currentTracebacks) > 0 {
				deepest := currentTracebacks[len(currentTracebacks)-1]
				if pendingFailed.File == "" {
					pendingFailed.File = deepest.file
					pendingFailed.Line = deepest.line
				}
			}

			file := m[1]
			se := result.SAWError{
				Code:     result.CodeToolError,
				File:     file,
				Severity: "error",
				Message:  "collection error: " + file,
				Tool:     "pytest",
			}
			pr.Errors = append(pr.Errors, se)
			pendingFailed = &pr.Errors[len(pr.Errors)-1]
			currentTracebacks = nil
			continue
		}
	}

	// Flush the last pending item
	if pendingFailed != nil && len(currentTracebacks) > 0 {
		deepest := currentTracebacks[len(currentTracebacks)-1]
		if pendingFailed.File == "" {
			pendingFailed.File = deepest.file
			pendingFailed.Line = deepest.line
		}
	}

	return pr
}

// extractPytestFile pulls the file path from a pytest node-id like
// "tests/test_foo.py::TestClass::test_name".
func extractPytestFile(nodeID string) string {
	parts := strings.SplitN(nodeID, "::", 2)
	return parts[0]
}

// ──────────────────────────────────────────────
// MypyParser
// ──────────────────────────────────────────────

// MypyParser parses output from the mypy static type checker.
type MypyParser struct{}

// src/foo.py:42: error: Incompatible types [assignment]
var mypyLineRe = regexp.MustCompile(`^([^:]+):(\d+):\s+(error|warning|note):\s+(.+?)(?:\s+\[([^\]]+)\])?$`)

// Name returns the tool identifier.
func (m *MypyParser) Name() string { return "mypy" }

// Parse extracts structured errors from mypy stdout/stderr.
func (m *MypyParser) Parse(stdout, stderr string) *ParseResult {
	combinedStr := stdout
	if stderr != "" {
		if combinedStr != "" {
			combinedStr += "\n" + stderr
		} else {
			combinedStr = stderr
		}
	}

	pr := &ParseResult{
		Tool:   "mypy",
		Errors: []result.SAWError{},
		Raw:    combinedStr,
	}

	for _, raw := range strings.Split(combinedStr, "\n") {
		line := strings.TrimRight(raw, "\r")
		match := mypyLineRe.FindStringSubmatch(line)
		if match == nil {
			continue
		}

		file := match[1]
		lineNum, _ := strconv.Atoi(match[2])
		severity := match[3]
		msg := match[4]
		rule := match[5]

		// mypy uses "note" as severity; map to "info"
		if severity == "note" {
			severity = "info"
		}

		se := result.SAWError{
			Code:     result.CodeToolError,
			File:     file,
			Line:     lineNum,
			Severity: severity,
			Message:  msg,
			Tool:     "mypy",
			Context:  makeContext(0, rule),
		}
		pr.Errors = append(pr.Errors, se)
	}

	return pr
}

// ──────────────────────────────────────────────
// RuffParser
// ──────────────────────────────────────────────

// RuffParser parses output from the Ruff Python linter.
type RuffParser struct{}

// src/foo.py:10:1: E501 Line too long (120 > 88 characters)
var ruffLineRe = regexp.MustCompile(`^([^:]+):(\d+):(\d+):\s+([A-Z]\d+)\s+(.+)$`)

// ruff fix suggestion lines look like:
//
//	= help: ...
var ruffHelpRe = regexp.MustCompile(`^\s*=\s+help:\s+(.+)$`)

// Name returns the tool identifier.
func (r *RuffParser) Name() string { return "ruff" }

// Parse extracts structured errors from ruff stdout/stderr.
func (r *RuffParser) Parse(stdout, stderr string) *ParseResult {
	combinedStr := stdout
	if stderr != "" {
		if combinedStr != "" {
			combinedStr += "\n" + stderr
		} else {
			combinedStr = stderr
		}
	}

	pr := &ParseResult{
		Tool:   "ruff",
		Errors: []result.SAWError{},
		Raw:    combinedStr,
	}

	lines := strings.Split(combinedStr, "\n")
	var lastIdx int = -1

	for _, raw := range lines {
		line := strings.TrimRight(raw, "\r")

		// Try to match a help/suggestion line and attach to the previous error
		if lastIdx >= 0 {
			if m := ruffHelpRe.FindStringSubmatch(line); m != nil {
				pr.Errors[lastIdx].Suggestion = m[1]
				continue
			}
		}

		match := ruffLineRe.FindStringSubmatch(line)
		if match == nil {
			continue
		}

		file := match[1]
		lineNum, _ := strconv.Atoi(match[2])
		col, _ := strconv.Atoi(match[3])
		rule := match[4]
		msg := match[5]

		severity := "warning"
		if strings.HasPrefix(rule, "E") {
			severity = "error"
		}

		se := result.SAWError{
			Code:     result.CodeToolError,
			File:     file,
			Line:     lineNum,
			Severity: severity,
			Message:  msg,
			Tool:     "ruff",
			Context:  makeContext(col, rule),
		}
		pr.Errors = append(pr.Errors, se)
		lastIdx = len(pr.Errors) - 1
	}

	return pr
}
