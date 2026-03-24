package protocol

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/result"
)

// worktreeBranchRegex validates branch names in both legacy and slug-scoped formats:
//   - Legacy: wave{N}-agent-{ID}
//   - New: saw/{slug}/wave{N}-agent-{ID}
//
// Uses ScopedBranchRegex from branchname.go which accepts both formats.
var worktreeBranchRegex = ScopedBranchRegex

// verificationRegex validates verification field: must contain "PASS" or "FAIL"
// as a standalone word. Lenient: agents write varied formats like
// "PASS — all tests green" or "go build, go test — all 18 tests PASS".
var verificationRegex = regexp.MustCompile(`\b(PASS|FAIL)\b`)

// ValidateWorktreeNames checks completion report worktree and branch fields
// match the expected wave{N}-agent-{ID} naming convention (E5).
//
// For each completion report:
//   - If branch is non-empty, validates it matches wave{N}-agent-{ID} where N is the agent's wave number
//   - If worktree is non-empty, validates it contains wave{N}-agent-{ID} as a path segment
//   - Empty fields are valid (backward compatibility)
//   - Solo-wave agents (single agent in a wave) are exempt: they commit directly
//     to the working branch (main/develop) per protocol, not to a worktree branch
//
// Returns:
//   - E5_INVALID_WORKTREE_NAME for branch violations
//   - E5_INVALID_WORKTREE_PATH for worktree path violations
func ValidateWorktreeNames(m *IMPLManifest) []result.SAWError {
	var errs []result.SAWError

	// Build map of agent -> wave number and wave -> agent count
	agentWave := make(map[string]int)
	waveSize := make(map[int]int)
	for _, wave := range m.Waves {
		waveSize[wave.Number] = len(wave.Agents)
		for _, agent := range wave.Agents {
			agentWave[agent.ID] = wave.Number
		}
	}

	// Validate each completion report
	for agentID, report := range m.CompletionReports {
		// Find agent's wave number
		waveNum, found := agentWave[agentID]
		if !found {
			// Agent not in wave structure — skip validation (may be error caught elsewhere)
			continue
		}

		// Solo-wave agents (single agent in wave) commit directly to the working
		// branch (main/develop) — skip worktree naming checks entirely.
		isSolo := waveSize[waveNum] == 1

		// Validate branch name if present
		if strings.TrimSpace(report.Branch) != "" && !isSolo {
			matches := worktreeBranchRegex.FindStringSubmatch(report.Branch)
			if matches == nil {
				errs = append(errs, result.SAWError{
					Code:     "E5_INVALID_WORKTREE_NAME",
					Severity: "error",
					Message:  fmt.Sprintf("agent %s branch %q does not match pattern wave{N}-agent-{ID} or saw/{slug}/wave{N}-agent-{ID}", agentID, report.Branch),
					Field:    fmt.Sprintf("completion_reports[%s].branch", agentID),
					Context:  map[string]string{"slug": m.FeatureSlug, "wave": fmt.Sprintf("%d", waveNum), "agent_id": agentID},
				})
			} else {
				// Extract wave number and agent ID from branch name
				branchWave := matches[1]
				branchAgent := matches[2]

				// Validate wave number matches
				expectedWave := fmt.Sprintf("%d", waveNum)
				if branchWave != expectedWave {
					errs = append(errs, result.SAWError{
						Code:     "E5_INVALID_WORKTREE_NAME",
						Severity: "error",
						Message:  fmt.Sprintf("agent %s (wave %d) branch %q has wrong wave number (expected wave%s-agent-%s)", agentID, waveNum, report.Branch, expectedWave, agentID),
						Field:    fmt.Sprintf("completion_reports[%s].branch", agentID),
						Context:  map[string]string{"slug": m.FeatureSlug, "wave": fmt.Sprintf("%d", waveNum), "agent_id": agentID},
					})
				}

				// Validate agent ID matches
				if branchAgent != agentID {
					errs = append(errs, result.SAWError{
						Code:     "E5_INVALID_WORKTREE_NAME",
						Severity: "error",
						Message:  fmt.Sprintf("agent %s branch %q has wrong agent ID (expected wave%s-agent-%s)", agentID, report.Branch, expectedWave, agentID),
						Field:    fmt.Sprintf("completion_reports[%s].branch", agentID),
						Context:  map[string]string{"slug": m.FeatureSlug, "wave": fmt.Sprintf("%d", waveNum), "agent_id": agentID},
					})
				}
			}
		}

		// Validate worktree path if present (skip for solo-wave agents)
		if strings.TrimSpace(report.Worktree) != "" && !isSolo {
			expectedSegment := fmt.Sprintf("wave%d-agent-%s", waveNum, agentID)
			// Check if worktree path contains the expected segment as a path component.
			// Accepts both legacy (.claude/worktrees/wave1-agent-A) and slug-scoped
			// (.claude/worktrees/saw/{slug}/wave1-agent-A) paths.
			// Split by both '/' and '\' to handle Unix and Windows paths.
			pathNormalized := strings.ReplaceAll(report.Worktree, "\\", "/")
			pathSegments := strings.Split(pathNormalized, "/")
			found := false
			for _, segment := range pathSegments {
				if segment == expectedSegment {
					found = true
					break
				}
			}

			if !found {
				errs = append(errs, result.SAWError{
					Code:     "E5_INVALID_WORKTREE_PATH",
					Severity: "error",
					Message:  fmt.Sprintf("agent %s worktree path %q does not contain expected segment %q", agentID, report.Worktree, expectedSegment),
					Field:    fmt.Sprintf("completion_reports[%s].worktree", agentID),
					Context:  map[string]string{"slug": m.FeatureSlug, "wave": fmt.Sprintf("%d", waveNum), "agent_id": agentID},
				})
			}
		}
	}

	return errs
}

// ValidateVerificationField checks that completion report verification fields
// use the structured format: "PASS" or "FAIL ({details})" (E10).
//
// For each completion report:
//   - If verification is empty, it is valid (backward compatibility)
//   - Otherwise, validates it matches the pattern: "PASS" or "FAIL" optionally followed by " (details)"
//
// Returns:
//   - E10_INVALID_VERIFICATION for format violations
func ValidateVerificationField(m *IMPLManifest) []result.SAWError {
	var errs []result.SAWError

	// Build agent -> wave lookup for context
	agentWave := make(map[string]int)
	for _, wave := range m.Waves {
		for _, agent := range wave.Agents {
			agentWave[agent.ID] = wave.Number
		}
	}

	for agentID, report := range m.CompletionReports {
		// Empty is valid (backward compatibility)
		if strings.TrimSpace(report.Verification) == "" {
			continue
		}

		// Validate format
		if !verificationRegex.MatchString(report.Verification) {
			errs = append(errs, result.SAWError{
				Code:     "E10_INVALID_VERIFICATION",
				Severity: "error",
				Message:  fmt.Sprintf("agent %s verification field %q does not match format 'PASS' or 'FAIL (details)'", agentID, report.Verification),
				Field:    fmt.Sprintf("completion_reports[%s].verification", agentID),
				Context:  map[string]string{"slug": m.FeatureSlug, "wave": fmt.Sprintf("%d", agentWave[agentID]), "agent_id": agentID},
			})
		}
	}

	return errs
}
