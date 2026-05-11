package errparse

import (
	"strings"

	"github.com/blackwell-systems/polywave-go/pkg/result"
)

func init() {
	Register(&GofmtParser{})
	Register(&PrettierFormatParser{})
	Register(&RuffFormatParser{})
	Register(&CargoFmtParser{})
}

// ─────────────────────────────────────────────────────────────────────────────
// GofmtParser
// ─────────────────────────────────────────────────────────────────────────────

// GofmtParser parses output from `gofmt -l`.
// gofmt -l outputs filenames of unformatted files, one per line, no prefix.
type GofmtParser struct{}

var _ Parser = (*GofmtParser)(nil)

func (p *GofmtParser) Name() string { return "gofmt" }

func (p *GofmtParser) Parse(stdout, stderr string) *ParseResult {
	raw := combined(stdout, stderr)
	pr := &ParseResult{
		Tool:   p.Name(),
		Errors: []result.PolywaveError{},
		Raw:    raw,
	}

	for _, line := range splitLines(stdout) {
		line = strings.TrimRight(line, "\r")
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		pr.Errors = append(pr.Errors, result.PolywaveError{
			Code:     result.CodeToolError,
			File:     line,
			Severity: "warning",
			Message:  "file is not gofmt-formatted",
			Tool:     p.Name(),
		})
	}
	return pr
}

// ─────────────────────────────────────────────────────────────────────────────
// PrettierFormatParser
// ─────────────────────────────────────────────────────────────────────────────

// PrettierFormatParser parses output from `npx prettier --check`.
// prettier --check outputs lines prefixed with "Checking " for each file checked,
// and uses exit 1 when any file is not formatted. Unformatted files appear after
// a "[warn]" section header or as lines matching "Checking <file>" followed by
// issues. The simplest reliable approach: look for lines prefixed with "[warn] "
// (newer prettier) or bare filenames after a warning summary.
//
// Prettier v2/v3 --check output formats:
//
//	[warn] src/index.ts
//	[warn] src/utils.ts
//	[warn] Code style issues found in 2 files. Forgot to run Prettier?
//
// Older format may just list filenames.
type PrettierFormatParser struct{}

var _ Parser = (*PrettierFormatParser)(nil)

func (p *PrettierFormatParser) Name() string { return "prettier-format" }

func (p *PrettierFormatParser) Parse(stdout, stderr string) *ParseResult {
	raw := combined(stdout, stderr)
	pr := &ParseResult{
		Tool:   p.Name(),
		Errors: []result.PolywaveError{},
		Raw:    raw,
	}

	// Combine stdout and stderr — prettier may write to either
	fullOutput := raw
	for _, line := range splitLines(fullOutput) {
		line = strings.TrimRight(line, "\r")
		trimmed := strings.TrimSpace(line)

		// Modern prettier: "[warn] <filename>"
		if strings.HasPrefix(trimmed, "[warn] ") {
			file := strings.TrimPrefix(trimmed, "[warn] ")
			file = strings.TrimSpace(file)
			// Skip summary lines like "Code style issues found..."
			if file == "" || strings.HasPrefix(file, "Code style issues") || strings.HasPrefix(file, "Forgot to run") {
				continue
			}
			pr.Errors = append(pr.Errors, result.PolywaveError{
				Code:     result.CodeToolError,
				File:     file,
				Severity: "warning",
				Message:  "file is not prettier-formatted",
				Tool:     p.Name(),
			})
			continue
		}

		// Some older versions output "Checking <file>" lines
		if strings.HasPrefix(trimmed, "Checking ") {
			file := strings.TrimPrefix(trimmed, "Checking ")
			file = strings.TrimSpace(file)
			// Prettier outputs "Checking <file>" for every file it checks.
			// We only include it if followed by an issue, but since we don't
			// do multi-pass parsing here, skip these — rely on [warn] lines.
			_ = file
		}
	}
	return pr
}

// ─────────────────────────────────────────────────────────────────────────────
// RuffFormatParser
// ─────────────────────────────────────────────────────────────────────────────

// RuffFormatParser parses output from `ruff format --check`.
// ruff format --check outputs "Would reformat: <file>" for each unformatted file.
type RuffFormatParser struct{}

var _ Parser = (*RuffFormatParser)(nil)

func (p *RuffFormatParser) Name() string { return "ruff-format" }

func (p *RuffFormatParser) Parse(stdout, stderr string) *ParseResult {
	raw := combined(stdout, stderr)
	pr := &ParseResult{
		Tool:   p.Name(),
		Errors: []result.PolywaveError{},
		Raw:    raw,
	}

	fullOutput := raw
	for _, line := range splitLines(fullOutput) {
		line = strings.TrimRight(line, "\r")
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "Would reformat: ") {
			file := strings.TrimPrefix(trimmed, "Would reformat: ")
			file = strings.TrimSpace(file)
			if file == "" {
				continue
			}
			pr.Errors = append(pr.Errors, result.PolywaveError{
				Code:     result.CodeToolError,
				File:     file,
				Severity: "warning",
				Message:  "file needs ruff format",
				Tool:     p.Name(),
			})
		}
	}
	return pr
}

// ─────────────────────────────────────────────────────────────────────────────
// CargoFmtParser
// ─────────────────────────────────────────────────────────────────────────────

// CargoFmtParser parses output from `cargo fmt --check`.
// cargo fmt --check outputs "Diff in <file>:" lines for each file that needs formatting.
type CargoFmtParser struct{}

var _ Parser = (*CargoFmtParser)(nil)

func (p *CargoFmtParser) Name() string { return "cargo-fmt" }

func (p *CargoFmtParser) Parse(stdout, stderr string) *ParseResult {
	raw := combined(stdout, stderr)
	pr := &ParseResult{
		Tool:   p.Name(),
		Errors: []result.PolywaveError{},
		Raw:    raw,
	}

	fullOutput := raw
	for _, line := range splitLines(fullOutput) {
		line = strings.TrimRight(line, "\r")
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "Diff in ") {
			// Extract filename up to ":"
			rest := strings.TrimPrefix(trimmed, "Diff in ")
			// rest looks like "<file>: ..." or "<file> at line N:"
			// Find the first colon to extract filename
			if idx := strings.Index(rest, ":"); idx >= 0 {
				file := strings.TrimSpace(rest[:idx])
				if file == "" {
					continue
				}
				pr.Errors = append(pr.Errors, result.PolywaveError{
					Code:     result.CodeToolError,
					File:     file,
					Severity: "warning",
					Message:  "file needs cargo fmt",
					Tool:     p.Name(),
				})
			}
		}
	}
	return pr
}
