package errparse

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/result"
)

func init() {
	Register(&GoBuildParser{})
	Register(&GoTestParser{})
	Register(&GoVetParser{})
	Register(&GolangciLintParser{})
}

// ansiEscape matches ANSI color/control escape sequences.
var ansiEscape = regexp.MustCompile(`\x1b\[[0-9;]*[mGKHF]`)

// stripANSI removes ANSI escape codes from s.
func stripANSI(s string) string {
	return ansiEscape.ReplaceAllString(s, "")
}

// splitLines splits combined output into individual non-empty lines.
func splitLines(s string) []string {
	return strings.Split(s, "\n")
}

// combined returns stdout and stderr joined with a newline (non-empty only).
func combined(stdout, stderr string) string {
	parts := []string{}
	if stdout != "" {
		parts = append(parts, stdout)
	}
	if stderr != "" {
		parts = append(parts, stderr)
	}
	return strings.Join(parts, "\n")
}

// makeContext creates a context map with optional column and rule entries.
func makeContext(col int, rule string) map[string]string {
	ctx := map[string]string{}
	if col > 0 {
		ctx["column"] = strconv.Itoa(col)
	}
	if rule != "" {
		ctx["rule"] = rule
	}
	if len(ctx) == 0 {
		return nil
	}
	return ctx
}

// ─────────────────────────────────────────────────────────────────────────────
// GoBuildParser
// ─────────────────────────────────────────────────────────────────────────────

// GoBuildParser parses output from `go build`.
// Expected format: file.go:line:col: message  or  file.go:line: message
type GoBuildParser struct{}

var _ Parser = (*GoBuildParser)(nil)

// goBuildRe matches:  <file>:<line>:<col>: <message>
//
//	or  <file>:<line>: <message>
var goBuildRe = regexp.MustCompile(`^(.+?):(\d+):(?:(\d+):)?\s+(.+)$`)

func (p *GoBuildParser) Name() string { return "go-build" }

func (p *GoBuildParser) Parse(stdout, stderr string) *ParseResult {
	raw := combined(stdout, stderr)
	pr := &ParseResult{
		Tool:   p.Name(),
		Errors: []result.SAWError{},
		Raw:    raw,
	}

	clean := stripANSI(raw)
	for _, line := range splitLines(clean) {
		line = strings.TrimRight(line, "\r")
		m := goBuildRe.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		file := m[1]
		lineNum, _ := strconv.Atoi(m[2])
		col, _ := strconv.Atoi(m[3]) // 0 if not present
		message := strings.TrimSpace(m[4])

		pr.Errors = append(pr.Errors, result.SAWError{
			Code:     result.CodeToolError,
			File:     file,
			Line:     lineNum,
			Severity: "error",
			Message:  message,
			Tool:     p.Name(),
			Context:  makeContext(col, ""),
		})
	}
	return pr
}

// ─────────────────────────────────────────────────────────────────────────────
// GoTestParser
// ─────────────────────────────────────────────────────────────────────────────

// GoTestParser parses output from `go test`.
// Key patterns:
//   - --- FAIL: TestName (0.00s)
//   - \t<file>:<line>: <message>   (error detail lines)
//   - panic: <message>             (followed by goroutine stack)
type GoTestParser struct{}

var _ Parser = (*GoTestParser)(nil)

// failRe matches test failure lines: --- FAIL: TestName (duration)
var failRe = regexp.MustCompile(`^--- FAIL:\s+(\S+)\s+\([\d.]+s\)`)

// testFileRe matches indented file references inside test output:  \tfile.go:line: message
var testFileRe = regexp.MustCompile(`^\s+(.+_test\.go|\S+\.go):(\d+):\s*(.*)$`)

// panicRe matches the start of a panic.
var panicRe = regexp.MustCompile(`^panic:\s+(.+)`)

// goroutineFileRe matches stack-trace file lines: \t/absolute/path/file.go:line +0xNN
var goroutineFileRe = regexp.MustCompile(`^\s+(\S+\.go):(\d+)`)

func (p *GoTestParser) Name() string { return "go-test" }

func (p *GoTestParser) Parse(stdout, stderr string) *ParseResult {
	raw := combined(stdout, stderr)
	pr := &ParseResult{
		Tool:   p.Name(),
		Errors: []result.SAWError{},
		Raw:    raw,
	}

	clean := stripANSI(raw)
	lines := splitLines(clean)

	var currentTest string
	inPanic := false
	var panicMsg string

	for _, line := range lines {
		line = strings.TrimRight(line, "\r")

		// Detect FAIL lines
		if m := failRe.FindStringSubmatch(line); m != nil {
			currentTest = m[1]
			pr.Errors = append(pr.Errors, result.SAWError{
				Code:     result.CodeToolError,
				Severity: "error",
				Message:  "FAIL: " + currentTest,
				Tool:     p.Name(),
				Context:  makeContext(0, currentTest),
			})
			continue
		}

		// Detect panic lines
		if m := panicRe.FindStringSubmatch(line); m != nil {
			inPanic = true
			panicMsg = m[1]
			pr.Errors = append(pr.Errors, result.SAWError{
				Code:     result.CodeToolError,
				Severity: "error",
				Message:  "panic: " + panicMsg,
				Tool:     p.Name(),
			})
			continue
		}

		// In a panic, look for the first relevant file in the goroutine stack
		if inPanic {
			if m := goroutineFileRe.FindStringSubmatch(line); m != nil {
				file := m[1]
				lineNum, _ := strconv.Atoi(m[2])
				// Update the last error entry with file/line info
				if len(pr.Errors) > 0 {
					last := &pr.Errors[len(pr.Errors)-1]
					if last.File == "" && strings.HasPrefix(last.Message, "panic:") {
						last.File = file
						last.Line = lineNum
					}
				}
				inPanic = false
			}
			continue
		}

		// Indented file reference lines (test assertion failures)
		if m := testFileRe.FindStringSubmatch(line); m != nil {
			file := m[1]
			lineNum, _ := strconv.Atoi(m[2])
			message := strings.TrimSpace(m[3])
			if message == "" {
				message = "test failure"
			}
			rule := currentTest
			pr.Errors = append(pr.Errors, result.SAWError{
				Code:     result.CodeToolError,
				File:     file,
				Line:     lineNum,
				Severity: "error",
				Message:  message,
				Tool:     p.Name(),
				Context:  makeContext(0, rule),
			})
		}
	}

	return pr
}

// ─────────────────────────────────────────────────────────────────────────────
// GoVetParser
// ─────────────────────────────────────────────────────────────────────────────

// GoVetParser parses output from `go vet`.
// Format is identical to go build: file.go:line:col: message
type GoVetParser struct{}

var _ Parser = (*GoVetParser)(nil)

func (p *GoVetParser) Name() string { return "go-vet" }

func (p *GoVetParser) Parse(stdout, stderr string) *ParseResult {
	raw := combined(stdout, stderr)
	pr := &ParseResult{
		Tool:   p.Name(),
		Errors: []result.SAWError{},
		Raw:    raw,
	}

	clean := stripANSI(raw)
	for _, line := range splitLines(clean) {
		line = strings.TrimRight(line, "\r")
		m := goBuildRe.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		file := m[1]
		lineNum, _ := strconv.Atoi(m[2])
		col, _ := strconv.Atoi(m[3])
		message := strings.TrimSpace(m[4])

		pr.Errors = append(pr.Errors, result.SAWError{
			Code:     result.CodeToolError,
			File:     file,
			Line:     lineNum,
			Severity: "warning",
			Message:  message,
			Tool:     p.Name(),
			Context:  makeContext(col, ""),
		})
	}
	return pr
}

// ─────────────────────────────────────────────────────────────────────────────
// GolangciLintParser
// ─────────────────────────────────────────────────────────────────────────────

// GolangciLintParser parses output from golangci-lint.
// Format: file.go:line:col: message (linter-name)
//
//	or file.go:line:col: message [linter-name]
//	or file.go:line: message (linter-name)
//
// Suggestions may appear as lines starting with "Fix:" or "suggestion:" after
// the main error line.
type GolangciLintParser struct{}

var _ Parser = (*GolangciLintParser)(nil)

// golangciRe matches the standard golangci-lint output line.
// Group 1: file, 2: line, 3: col (optional), 4: message (may include rule in parens/brackets)
var golangciRe = regexp.MustCompile(`^(.+?):(\d+):(?:(\d+):)?\s+(.+)$`)

// ruleParensRe extracts rule name at the end of message in parens: "message (rule-name)"
var ruleParensRe = regexp.MustCompile(`^(.*?)\s+\(([^)]+)\)\s*$`)

// ruleBracketsRe extracts rule name at the end of message in brackets: "message [rule-name]"
var ruleBracketsRe = regexp.MustCompile(`^(.*?)\s+\[([^\]]+)\]\s*$`)

// suggestionRe matches lines that look like auto-fix suggestions.
var suggestionRe = regexp.MustCompile(`(?i)^(?:fix|suggestion|suggested fix):\s*(.+)$`)

func (p *GolangciLintParser) Name() string { return "golangci-lint" }

func (p *GolangciLintParser) Parse(stdout, stderr string) *ParseResult {
	raw := combined(stdout, stderr)
	pr := &ParseResult{
		Tool:   p.Name(),
		Errors: []result.SAWError{},
		Raw:    raw,
	}

	clean := stripANSI(raw)
	lines := splitLines(clean)

	for _, line := range lines {
		line = strings.TrimRight(line, "\r")

		// Check for suggestion lines and attach to last error
		if m := suggestionRe.FindStringSubmatch(line); m != nil {
			if len(pr.Errors) > 0 {
				pr.Errors[len(pr.Errors)-1].Suggestion = strings.TrimSpace(m[1])
			}
			continue
		}

		m := golangciRe.FindStringSubmatch(line)
		if m == nil {
			continue
		}

		file := m[1]
		lineNum, _ := strconv.Atoi(m[2])
		col, _ := strconv.Atoi(m[3])
		message := strings.TrimSpace(m[4])

		// Extract rule name from parens or brackets at end of message.
		var rule string
		if rm := ruleParensRe.FindStringSubmatch(message); rm != nil {
			message = strings.TrimSpace(rm[1])
			rule = rm[2]
		} else if rm := ruleBracketsRe.FindStringSubmatch(message); rm != nil {
			message = strings.TrimSpace(rm[1])
			rule = rm[2]
		}

		pr.Errors = append(pr.Errors, result.SAWError{
			Code:       result.CodeToolError,
			File:       file,
			Line:       lineNum,
			Severity:   "warning",
			Message:    message,
			Suggestion: "",
			Tool:       p.Name(),
			Context:    makeContext(col, rule),
		})
	}
	return pr
}
