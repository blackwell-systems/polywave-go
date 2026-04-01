package protocol

import (
	"fmt"
	"strings"
)

// PreWaveGateCheck is a single readiness check result.
type PreWaveGateCheck struct {
	Name    string `json:"name"`
	Status  string `json:"status"`  // "pass", "fail", "warn"
	Message string `json:"message"`
}

// PreWaveGateResult is the aggregate readiness result.
type PreWaveGateResult struct {
	Ready  bool               `json:"ready"`
	Checks []PreWaveGateCheck `json:"checks"`
}

// PreWaveGate runs all pre-wave readiness checks on a manifest.
// Returns structured result with per-check status. Does not modify the manifest.
func PreWaveGate(m *IMPLManifest) *PreWaveGateResult {
	result := &PreWaveGateResult{
		Checks: make([]PreWaveGateCheck, 0, 4),
	}

	// 1. Validation check
	result.Checks = append(result.Checks, checkValidation(m))

	// 2. Critic review check
	result.Checks = append(result.Checks, checkCriticReview(m))

	// 3. Scaffolds check
	result.Checks = append(result.Checks, checkScaffolds(m))

	// 4. State check
	result.Checks = append(result.Checks, checkState(m))

	// Ready is true only if no check has status=fail
	result.Ready = true
	for _, check := range result.Checks {
		if check.Status == "fail" {
			result.Ready = false
			break
		}
	}

	return result
}

func checkValidation(m *IMPLManifest) PreWaveGateCheck {
	all := Validate(m)
	if len(all) == 0 {
		return PreWaveGateCheck{
			Name:    "validation",
			Status:  "pass",
			Message: "manifest is valid",
		}
	}

	// Separate errors from warnings — only errors block wave execution.
	var errMsgs, warnMsgs []string
	for _, e := range all {
		if e.Severity == "warning" {
			warnMsgs = append(warnMsgs, e.Message)
		} else {
			errMsgs = append(errMsgs, e.Message)
		}
	}

	if len(errMsgs) == 0 {
		return PreWaveGateCheck{
			Name:    "validation",
			Status:  "warn",
			Message: strings.Join(warnMsgs, "; "),
		}
	}

	return PreWaveGateCheck{
		Name:    "validation",
		Status:  "fail",
		Message: strings.Join(errMsgs, "; "),
	}
}

// checkCriticReview checks whether a critic review has been run when required (E37).
// It delegates the threshold decision to E37Required (critic_gate.go) to avoid
// duplicating the wave1-agents / multi-repo logic inline.
func checkCriticReview(m *IMPLManifest) PreWaveGateCheck {
	if m.CriticReport == nil {
		if E37Required(m) {
			// Compute diagnostic counts separately so the error message retains
			// the same level of detail as before the refactor (brief option a).
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
			return PreWaveGateCheck{
				Name:    "critic_review",
				Status:  "fail",
				Message: fmt.Sprintf("E37: critic review required (wave 1 has %d agents, %d repos) but not run. Run critic or use --no-critic to skip.", wave1Agents, len(repoSet)),
			}
		}
		return PreWaveGateCheck{
			Name:    "critic_review",
			Status:  "pass",
			Message: "critic review not required (threshold not met)",
		}
	}

	if m.CriticReport.Verdict == CriticVerdictPass {
		return PreWaveGateCheck{
			Name:    "critic_review",
			Status:  "pass",
			Message: "critic review passed",
		}
	}

	if m.CriticReport.Verdict == CriticVerdictSkipped {
		return PreWaveGateCheck{
			Name:    "critic_review",
			Status:  "pass",
			Message: "critic review explicitly skipped by operator",
		}
	}

	return PreWaveGateCheck{
		Name:    "critic_review",
		Status:  "fail",
		Message: fmt.Sprintf("critic review failed with %d issue(s)", m.CriticReport.IssueCount),
	}
}

func checkScaffolds(m *IMPLManifest) PreWaveGateCheck {
	if len(m.Scaffolds) == 0 {
		return PreWaveGateCheck{
			Name:    "scaffolds",
			Status:  "pass",
			Message: "no scaffolds required",
		}
	}

	if AllScaffoldsCommitted(m) {
		return PreWaveGateCheck{
			Name:    "scaffolds",
			Status:  "pass",
			Message: "all scaffolds committed",
		}
	}

	// List uncommitted scaffolds
	uncommitted := make([]string, 0)
	for _, s := range m.Scaffolds {
		if !strings.HasPrefix(s.Status, "committed") {
			uncommitted = append(uncommitted, s.FilePath)
		}
	}
	return PreWaveGateCheck{
		Name:    "scaffolds",
		Status:  "fail",
		Message: fmt.Sprintf("uncommitted scaffolds: %s", strings.Join(uncommitted, ", ")),
	}
}

func checkState(m *IMPLManifest) PreWaveGateCheck {
	switch m.State {
	case "", StateScoutPending, StateReviewed:
		return PreWaveGateCheck{
			Name:    "state",
			Status:  "pass",
			Message: "state is acceptable for wave execution",
		}
	case StateComplete, StateBlocked, StateNotSuitable:
		return PreWaveGateCheck{
			Name:    "state",
			Status:  "fail",
			Message: fmt.Sprintf("IMPL state is %s, cannot proceed", m.State),
		}
	default:
		return PreWaveGateCheck{
			Name:    "state",
			Status:  "pass",
			Message: "state is acceptable for wave execution",
		}
	}
}
