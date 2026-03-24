package main

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/protocol"
)

// FormatBaselineOutput produces human-readable CLI output for baseline results.
// Format:
//
//	Baseline verification (E21A):
//	  build: PASS
//	  lint: FAIL - pkg/foo/bar.go:10: error message
//	  test: SKIP (lint failed)
//
//	Error: baseline verification failed. Fix the codebase before launching agents.
func FormatBaselineOutput(result *protocol.BaselineData) string {
	var b strings.Builder

	header := "Baseline verification (E21A):"
	if result.FromCache && result.CommitSHA != "" {
		header = fmt.Sprintf("Baseline verification (E21A) (cached at %s):", result.CommitSHA)
	}
	b.WriteString(header)
	b.WriteByte('\n')

	for _, gr := range result.GateResults {
		if gr.Skipped {
			fmt.Fprintf(&b, "  %s: SKIP (%s)\n", gr.Type, gr.SkipReason)
			continue
		}
		if gr.Passed {
			fmt.Fprintf(&b, "  %s: PASS\n", gr.Type)
			continue
		}
		// FAIL — extract detail
		detail := extractDetail(gr)
		if detail != "" {
			fmt.Fprintf(&b, "  %s: FAIL - %s\n", gr.Type, detail)
		} else {
			fmt.Fprintf(&b, "  %s: FAIL\n", gr.Type)
		}
	}

	b.WriteByte('\n')
	if result.Passed {
		b.WriteString("Baseline verification passed.")
	} else {
		b.WriteString("Error: baseline verification failed. Fix the codebase before launching agents.")
		if suggestion := DiagnoseMigrationFailure(result); suggestion != "" {
			b.WriteByte('\n')
			b.WriteString("Hint: " + suggestion)
		}
	}

	return b.String()
}

// extractDetail returns the most useful single-line error detail from a failed GateResult.
// Priority: ParsedErrors first, then Stderr, then Stdout.
func extractDetail(gr protocol.GateResult) string {
	if len(gr.ParsedErrors) > 0 {
		return gr.ParsedErrors[0].Message
	}
	if line := firstMeaningfulLine(gr.Stderr); line != "" {
		return line
	}
	return firstMeaningfulLine(gr.Stdout)
}

// firstMeaningfulLine returns the first non-empty, non-whitespace line from s.
func firstMeaningfulLine(s string) string {
	for _, line := range strings.Split(s, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			return trimmed
		}
	}
	return ""
}

// migrationPatterns are regex patterns that indicate cross-wave migration breaks.
var migrationPatterns = []*regexp.Regexp{
	regexp.MustCompile(`cannot use .+ as .+`),
	regexp.MustCompile(`undefined: \w+`),
	regexp.MustCompile(`imported and not used`),
	regexp.MustCompile(`cannot find module`),
	regexp.MustCompile(`Module not found`),
	regexp.MustCompile(`No module named`),
	regexp.MustCompile(`unresolved import`),
}

const migrationSuggestion = "Baseline broken at wave boundary. Consider: " +
	"(1) consolidate all callers into the same wave as the signature change, " +
	"or (2) add re-export aliases in the old package to forward to the new signatures."

// DiagnoseMigrationFailure inspects a failed BaselineResult's gate output for
// type/import mismatch patterns that indicate a cross-wave migration break.
// Returns a human-readable suggestion string if detected, or "" if unrelated.
func DiagnoseMigrationFailure(result *protocol.BaselineData) string {
	for _, gr := range result.GateResults {
		if gr.Passed || gr.Skipped {
			continue
		}
		for _, pat := range migrationPatterns {
			if pat.MatchString(gr.Stderr) || pat.MatchString(gr.Stdout) {
				return migrationSuggestion
			}
		}
	}
	return ""
}
