package protocol

import (
	"context"
	"time"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/result"
)

// E37Required returns true when a critic review is required before wave execution.
// The E37 trigger conditions are:
//   - Wave 1 has 3 or more agents, OR
//   - file_ownership spans 2 or more distinct repositories
//
// This is the single authoritative implementation of E37 threshold logic.
// Callers: checkCriticReview (pre_wave_gate.go), PrepareTier (program_tier_prepare.go).
func E37Required(m *IMPLManifest) bool {
	wave1Agents := 0
	for _, wave := range m.Waves {
		if wave.Number == 1 {
			wave1Agents = len(wave.Agents)
			break
		}
	}

	repoSet := make(map[string]bool)
	for _, r := range m.Repositories {
		repoSet[r] = true
	}
	for _, fo := range m.FileOwnership {
		if fo.Repo != "" {
			repoSet[fo.Repo] = true
		}
	}

	return wave1Agents >= 3 || len(repoSet) >= 2
}

// CriticGatePasses implements E37 critic gate enforcement logic.
// Returns true when the wave should proceed, false when it should be blocked.
//
// Decision logic:
// - PASS verdict → always proceed
// - ISSUES verdict with errors → always block
// - ISSUES verdict with warnings only → proceed in auto mode, block in manual mode
// - No critic report → block (safety default)
// - Unknown verdict → block (safety default)
//
// Parameters:
//   - m: The IMPL manifest containing the critic report
//   - autoMode: If true, warnings-only issues pass. If false, surface to user.
func CriticGatePasses(m *IMPLManifest, autoMode bool) bool {
	if m.CriticReport == nil {
		return false // No critic report found — block
	}
	if m.CriticReport.Verdict == CriticVerdictPass {
		return true
	}
	if m.CriticReport.Verdict != CriticVerdictIssues {
		return false // Unknown verdict — block
	}
	// Verdict is ISSUES — check severity of all issues
	hasError := false
	for _, review := range m.CriticReport.AgentReviews {
		for _, issue := range review.Issues {
			if issue.Severity == CriticSeverityError {
				hasError = true
				break
			}
		}
		if hasError {
			break
		}
	}
	if hasError {
		return false // ISSUES with errors — block
	}
	// ISSUES with warnings only
	if autoMode {
		return true // Auto mode — proceed
	}
	return false // Manual mode — surface to user
}

// SkipCriticForIMPL writes a synthetic PASS critic report for a single IMPL
// if E37 is required and no passing critic report exists. Returns true if a
// skip was written, false if no skip was needed (E37 not required or already passes).
func SkipCriticForIMPL(ctx context.Context, implPath string, m *IMPLManifest) result.Result[bool] {
	if !E37Required(m) {
		return result.NewSuccess(false) // not required, no skip needed
	}
	if CriticGatePasses(m, true) {
		return result.NewSuccess(false) // already passes, no skip needed
	}
	skipData := CriticData{
		Verdict:      CriticVerdictPass,
		AgentReviews: map[string]AgentCriticReview{},
		Summary:      "Skipped by operator (--skip-critic)",
		ReviewedAt:   time.Now().UTC().Format(time.RFC3339),
		IssueCount:   0,
	}
	writeRes := WriteCriticReviewResult(ctx, implPath, skipData)
	if writeRes.IsFatal() {
		return result.NewFailure[bool](writeRes.Errors)
	}
	return result.NewSuccess(true)
}
