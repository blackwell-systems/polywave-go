package errparse

import (
	"encoding/json"
	"regexp"
	"strconv"
	"strings"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/result"
)

func init() {
	Register(&TscParser{})
	Register(&EslintParser{})
	Register(&NpmTestParser{})
}

// ─────────────────────────────────────────────────────────────────────────────
// TscParser – parses TypeScript compiler output.
// Format: src/file.ts(10,5): error TS2322: Type 'X' is not assignable to type 'Y'
// ─────────────────────────────────────────────────────────────────────────────

// tscLineRe matches a single tsc diagnostic line.
// Group 1 = file, 2 = line, 3 = col, 4 = severity, 5 = message
var tscLineRe = regexp.MustCompile(`^(.+)\((\d+),(\d+)\):\s+(error|warning|info)\s+TS\d+:\s+(.+)$`)

// TscParser parses output from the TypeScript compiler (tsc).
type TscParser struct{}

// Name returns the tool identifier.
func (p *TscParser) Name() string { return "tsc" }

// Parse extracts structured errors from tsc stdout/stderr.
func (p *TscParser) Parse(stdout, stderr string) *ParseResult {
	combinedStr := stripANSI(stdout + "\n" + stderr)
	pr := &ParseResult{
		Tool: "tsc",
		Raw:  stdout + "\n" + stderr,
	}

	for _, line := range strings.Split(combinedStr, "\n") {
		line = strings.TrimRight(line, "\r")
		m := tscLineRe.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		lineNum, _ := strconv.Atoi(m[2])
		colNum, _ := strconv.Atoi(m[3])
		pr.Errors = append(pr.Errors, result.SAWError{
			Code:     result.CodeToolError,
			File:     m[1],
			Line:     lineNum,
			Severity: m[4],
			Message:  strings.TrimSpace(m[5]),
			Tool:     "tsc",
			Context:  makeContext(colNum, ""),
		})
	}
	return pr
}

// ─────────────────────────────────────────────────────────────────────────────
// EslintParser – parses ESLint output (text and JSON formats).
// Text format: src/file.ts:10:5: 'x' is not defined [no-undef]
// JSON format: the standard ESLint JSON reporter output.
// ─────────────────────────────────────────────────────────────────────────────

// eslintTextRe matches a single ESLint text-format diagnostic.
// Group 1 = file, 2 = line, 3 = col, 4 = message, 5 = rule (optional)
var eslintTextRe = regexp.MustCompile(`^(.+):(\d+):(\d+):\s+(.+?)(?:\s+\[([^\]]+)\])?\s*$`)

// eslintJSON mirrors the structure produced by `eslint --format json`.
type eslintJSON []struct {
	FilePath string `json:"filePath"`
	Messages []struct {
		RuleID   string `json:"ruleId"`
		Severity int    `json:"severity"` // 1 = warning, 2 = error
		Message  string `json:"message"`
		Line     int    `json:"line"`
		Column   int    `json:"column"`
		Fix      *struct {
			Text string `json:"text"`
		} `json:"fix,omitempty"`
	} `json:"messages"`
}

// EslintParser parses output from ESLint.
type EslintParser struct{}

// Name returns the tool identifier.
func (p *EslintParser) Name() string { return "eslint" }

// Parse extracts structured errors from eslint stdout/stderr.
func (p *EslintParser) Parse(stdout, stderr string) *ParseResult {
	combinedStr := stdout + "\n" + stderr
	pr := &ParseResult{
		Tool: "eslint",
		Raw:  combinedStr,
	}

	// Try JSON format first (eslint --format json writes to stdout).
	cleanedForJSON := strings.TrimSpace(stripANSI(stdout))
	if strings.HasPrefix(cleanedForJSON, "[") {
		var parsed eslintJSON
		if err := json.Unmarshal([]byte(cleanedForJSON), &parsed); err == nil {
			for _, file := range parsed {
				for _, msg := range file.Messages {
					severity := "error"
					if msg.Severity == 1 {
						severity = "warning"
					}
					suggestion := ""
					if msg.Fix != nil {
						suggestion = "--fix available: " + msg.Fix.Text
					}
					pr.Errors = append(pr.Errors, result.SAWError{
						Code:       result.CodeToolError,
						File:       file.FilePath,
						Line:       msg.Line,
						Severity:   severity,
						Message:    msg.Message,
						Suggestion: suggestion,
						Tool:       "eslint",
						Context:    makeContext(msg.Column, msg.RuleID),
					})
				}
			}
			return pr
		}
	}

	// Fall back to text format.
	cleanedText := stripANSI(combinedStr)
	for _, line := range strings.Split(cleanedText, "\n") {
		line = strings.TrimRight(line, "\r")
		m := eslintTextRe.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		lineNum, _ := strconv.Atoi(m[2])
		colNum, _ := strconv.Atoi(m[3])

		// Determine severity from message prefix (eslint sometimes prefixes with
		// "error" or "warning"); default to "error".
		message := strings.TrimSpace(m[4])
		severity := "error"
		if strings.HasPrefix(strings.ToLower(message), "warning") {
			severity = "warning"
			message = strings.TrimSpace(message[len("warning"):])
			// strip leading colon/space
			message = strings.TrimLeft(message, ": ")
		} else if strings.HasPrefix(strings.ToLower(message), "error") {
			message = strings.TrimSpace(message[len("error"):])
			message = strings.TrimLeft(message, ": ")
		}

		suggestion := ""
		if m[5] != "" {
			suggestion = "Run eslint --fix to apply automatic fixes for rule " + m[5]
		}

		pr.Errors = append(pr.Errors, result.SAWError{
			Code:       result.CodeToolError,
			File:       m[1],
			Line:       lineNum,
			Severity:   severity,
			Message:    message,
			Suggestion: suggestion,
			Tool:       "eslint",
			Context:    makeContext(colNum, m[5]),
		})
	}
	return pr
}

// ─────────────────────────────────────────────────────────────────────────────
// NpmTestParser – parses Jest / Vitest output.
// Recognises:
//
//	FAIL src/file.test.ts
//	● Test Suite › test name
//	and file:line references in stack traces.
//
// ─────────────────────────────────────────────────────────────────────────────

// failFileRe matches "FAIL <file>" lines (Jest / Vitest).
var failFileRe = regexp.MustCompile(`^(?:FAIL|✗)\s+(.+\.(?:test|spec)\.[jt]sx?)`)

// bulletTestRe matches "  ● Suite > test name" lines.
var bulletTestRe = regexp.MustCompile(`^\s+●\s+(.+)`)

// stackRefRe matches stack-trace file references like "at ... (src/foo.ts:10:5)".
var stackRefRe = regexp.MustCompile(`\(([^)]+\.(?:[jt]sx?)):(\d+):(\d+)\)`)

// vitestFailRe matches Vitest "× test name" or "FAIL src/file" lines.
var vitestFailRe = regexp.MustCompile(`^\s*×\s+(.+)`)

// NpmTestParser parses Jest / Vitest output produced by `npm test`.
type NpmTestParser struct{}

// Name returns the tool identifier.
func (p *NpmTestParser) Name() string { return "npm-test" }

// Parse extracts structured errors from Jest / Vitest output.
func (p *NpmTestParser) Parse(stdout, stderr string) *ParseResult {
	combinedStr := stripANSI(stdout + "\n" + stderr)
	pr := &ParseResult{
		Tool: "npm-test",
		Raw:  stdout + "\n" + stderr,
	}

	lines := strings.Split(combinedStr, "\n")

	var currentFile string
	var currentTest string

	for i, line := range lines {
		line = strings.TrimRight(line, "\r")

		// FAIL <file> — marks which test file failed.
		if m := failFileRe.FindStringSubmatch(line); m != nil {
			currentFile = strings.TrimSpace(m[1])
			currentTest = ""
			continue
		}

		// ● Test Suite > test name (Jest)
		if m := bulletTestRe.FindStringSubmatch(line); m != nil {
			currentTest = strings.TrimSpace(m[1])
			// Scan ahead for the first stack-trace line reference.
			fileRef := ""
			refLine := 0
			refCol := 0
			for j := i + 1; j < len(lines) && j < i+30; j++ {
				if sm := stackRefRe.FindStringSubmatch(lines[j]); sm != nil {
					fileRef = sm[1]
					refLine, _ = strconv.Atoi(sm[2])
					refCol, _ = strconv.Atoi(sm[3])
					break
				}
				// Stop at next bullet or FAIL
				if bulletTestRe.MatchString(lines[j]) || failFileRe.MatchString(lines[j]) {
					break
				}
			}
			f := currentFile
			if fileRef != "" {
				f = fileRef
			}
			if f == "" {
				f = "unknown"
			}
			pr.Errors = append(pr.Errors, result.SAWError{
				Code:     result.CodeToolError,
				File:     f,
				Line:     refLine,
				Severity: "error",
				Message:  currentTest,
				Tool:     "npm-test",
				Context:  makeContext(refCol, ""),
			})
			continue
		}

		// × test name (Vitest failure marker)
		if m := vitestFailRe.FindStringSubmatch(line); m != nil {
			currentTest = strings.TrimSpace(m[1])
			fileRef := ""
			refLine := 0
			refCol := 0
			for j := i + 1; j < len(lines) && j < i+30; j++ {
				if sm := stackRefRe.FindStringSubmatch(lines[j]); sm != nil {
					fileRef = sm[1]
					refLine, _ = strconv.Atoi(sm[2])
					refCol, _ = strconv.Atoi(sm[3])
					break
				}
				if vitestFailRe.MatchString(lines[j]) || failFileRe.MatchString(lines[j]) {
					break
				}
			}
			f := currentFile
			if fileRef != "" {
				f = fileRef
			}
			if f == "" {
				f = "unknown"
			}
			pr.Errors = append(pr.Errors, result.SAWError{
				Code:     result.CodeToolError,
				File:     f,
				Line:     refLine,
				Severity: "error",
				Message:  currentTest,
				Tool:     "npm-test",
				Context:  makeContext(refCol, ""),
			})
			continue
		}

	}
	return pr
}
