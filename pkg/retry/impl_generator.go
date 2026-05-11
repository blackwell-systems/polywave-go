package retry

import (
	"fmt"

	"github.com/blackwell-systems/polywave-go/pkg/protocol"
)

// GenerateRetryIMPL creates a minimal single-wave, single-agent IMPL manifest
// targeting the failed files from a quality gate failure.
//
// The generated manifest:
//   - Has title "Retry: {parentSlug} attempt {N}"
//   - Contains a single wave with a single agent "R" (for retry)
//   - Targets only the failed files for fix
//   - Preserves the failed quality gate so the retry is verified
func GenerateRetryIMPL(parentSlug string, attempt int, failedFiles []string, gateOutput string, gateCommand string) *protocol.IMPLManifest {
	title := fmt.Sprintf("Retry: %s attempt %d", parentSlug, attempt)
	slug := fmt.Sprintf("%s-retry-%d", parentSlug, attempt)

	// Build the agent task with clear remediation instructions
	task := buildRetryAgentTask(parentSlug, attempt, failedFiles, gateOutput, gateCommand)

	// Build file ownership entries for the retry agent
	var fileOwnership []protocol.FileOwnership
	for _, f := range failedFiles {
		fileOwnership = append(fileOwnership, protocol.FileOwnership{
			File:   f,
			Agent:  "R",
			Wave:   1,
			Action: "modify",
		})
	}

	// Build the quality gate that failed — retry must pass it
	retryGate := protocol.QualityGate{
		Type:        inferGateType(gateCommand),
		Command:     gateCommand,
		Required:    true,
		Description: fmt.Sprintf("Verification gate from parent IMPL %s (attempt %d)", parentSlug, attempt),
	}

	manifest := &protocol.IMPLManifest{
		Title:        title,
		FeatureSlug:  slug,
		Verdict:      "SUITABLE",
		TestCommand:  gateCommand,
		LintCommand:  "",
		FileOwnership: fileOwnership,
		Waves: []protocol.Wave{
			{
				Number: 1,
				Agents: []protocol.Agent{
					{
						ID:    "R",
						Task:  task,
						Files: failedFiles,
					},
				},
			},
		},
		QualityGates: &protocol.QualityGates{
			Level: "standard",
			Gates: []protocol.QualityGate{retryGate},
		},
		State: protocol.StateWavePending,
	}

	return manifest
}

// buildRetryAgentTask constructs the agent task string with full context
// about what failed and what needs to be fixed.
func buildRetryAgentTask(parentSlug string, attempt int, failedFiles []string, gateOutput string, gateCommand string) string {
	filesSection := ""
	for _, f := range failedFiles {
		filesSection += fmt.Sprintf("  - %s\n", f)
	}
	if filesSection == "" {
		filesSection = "  (no specific files identified — review all changed files)\n"
	}

	// Classify the error and get fix suggestions
	errorClass := ClassifyError(gateOutput)
	fixes := SuggestFixes(errorClass)
	fixesSection := ""
	if len(fixes) > 0 {
		fixesSection = "\n## Suggested Fixes\n"
		for _, fix := range fixes {
			fixesSection += fmt.Sprintf("- %s\n", fix)
		}
	}

	return fmt.Sprintf(`Fix compilation/test errors in the files listed below.

## Context
This is retry attempt %d for IMPL: %s

## Error Classification: %s
%s
## Failed Quality Gate
Command: %s

## Gate Output (Error Details)
%s

## Files to Fix
%s
## Instructions
1. Read the gate output carefully to understand the exact errors.
2. Fix all compilation errors, test failures, or lint violations reported above.
3. Do NOT introduce new functionality — only fix the reported errors.
4. After fixing, the following command must pass: %s
5. Ensure all fixed files compile and tests pass.
`,
		attempt,
		parentSlug,
		string(errorClass),
		fixesSection,
		gateCommand,
		gateOutput,
		filesSection,
		gateCommand,
	)
}

// inferGateType determines the protocol gate type from a command string.
// Falls back to "build" if the command cannot be categorized.
func inferGateType(command string) string {
	if command == "" {
		return "build"
	}
	switch {
	case containsAny(command, "test"):
		return "test"
	case containsAny(command, "vet", "lint", "golint", "staticcheck", "errcheck"):
		return "lint"
	case containsAny(command, "build"):
		return "build"
	default:
		return "build"
	}
}

// containsAny returns true if s contains any of the given substrings.
func containsAny(s string, subs ...string) bool {
	for _, sub := range subs {
		if len(s) >= len(sub) {
			for i := 0; i <= len(s)-len(sub); i++ {
				if s[i:i+len(sub)] == sub {
					return true
				}
			}
		}
	}
	return false
}
