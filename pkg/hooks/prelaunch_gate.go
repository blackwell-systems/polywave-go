package hooks

import (
	"fmt"
	"os"
	"strings"

	"github.com/blackwell-systems/scout-and-wave-go/internal/git"
	"github.com/blackwell-systems/scout-and-wave-go/pkg/protocol"
	"github.com/blackwell-systems/scout-and-wave-go/pkg/result"
)

// PreLaunchCheck is a single pre-launch validation result.
type PreLaunchCheck struct {
	Name    string `json:"name"`
	Status  string `json:"status"` // "pass", "fail", "warn"
	Message string `json:"message"`
}

// PreLaunchResult is the aggregate pre-launch validation result.
type PreLaunchResult struct {
	Ready  bool             `json:"ready"`
	Checks []PreLaunchCheck `json:"checks"`
}

// PreLaunchGate validates all preconditions before launching a wave agent.
// Parameters:
//
//	manifest  - parsed IMPL manifest
//	waveNum   - wave number being launched
//	agentID   - agent letter/ID being launched
//	repoRoot  - absolute path to repository root
//	wtPath    - absolute path to agent's worktree (empty for integration waves)
//
// Checks performed:
//  1. IMPL doc validates (calls protocol.Validate)
//  2. Wave exists and is not already completed
//  3. Agent ID exists in the specified wave
//  4. Worktree path exists and is on correct branch
//  5. Scaffolds are committed (I2 defense-in-depth)
//  6. File ownership has no I1 conflicts
//  7. Critic report exists if E37 threshold met
func PreLaunchGate(
	manifest *protocol.IMPLManifest,
	waveNum int,
	agentID string,
	repoRoot string,
	wtPath string,
) *PreLaunchResult {
	result := &PreLaunchResult{
		Checks: make([]PreLaunchCheck, 0, 7),
	}

	// Run validation once and pass results to both validation checks (Bug 2)
	validationErrs := protocol.Validate(manifest)

	// 1. Validation
	result.Checks = append(result.Checks, checkManifestValidation(validationErrs))

	// 2. Wave exists
	result.Checks = append(result.Checks, checkWaveExists(manifest, waveNum, agentID))

	// 3. Agent exists
	result.Checks = append(result.Checks, checkAgentExists(manifest, waveNum, agentID))

	// 4. Worktree branch
	result.Checks = append(result.Checks, checkWorktreeBranch(manifest, waveNum, agentID, wtPath))

	// 5. Scaffolds committed
	result.Checks = append(result.Checks, checkScaffoldsCommitted(manifest))

	// 6. Ownership disjoint (I1)
	result.Checks = append(result.Checks, checkOwnershipDisjoint(validationErrs))

	// 7. Critic review
	result.Checks = append(result.Checks, checkCriticReviewRequired(manifest))

	// Ready is true only if no check has status "fail"
	result.Ready = true
	for _, check := range result.Checks {
		if check.Status == "fail" {
			result.Ready = false
			break
		}
	}

	return result
}

func checkManifestValidation(errs []result.SAWError) PreLaunchCheck {
	if len(errs) == 0 {
		return PreLaunchCheck{Name: "validation", Status: "pass", Message: "manifest is valid"}
	}
	var errMsgs, warnMsgs []string
	for _, e := range errs {
		if e.Severity == "warning" {
			warnMsgs = append(warnMsgs, e.Message)
		} else {
			errMsgs = append(errMsgs, e.Message)
		}
	}
	if len(errMsgs) == 0 {
		return PreLaunchCheck{Name: "validation", Status: "warn", Message: strings.Join(warnMsgs, "; ")}
	}
	return PreLaunchCheck{Name: "validation", Status: "fail", Message: strings.Join(errMsgs, "; ")}
}

func checkWaveExists(m *protocol.IMPLManifest, waveNum int, agentID string) PreLaunchCheck {
	var wave *protocol.Wave
	for i := range m.Waves {
		if m.Waves[i].Number == waveNum {
			wave = &m.Waves[i]
			break
		}
	}

	if wave == nil {
		return PreLaunchCheck{
			Name:    "wave_exists",
			Status:  "fail",
			Message: fmt.Sprintf("wave %d not found in manifest", waveNum),
		}
	}

	// Check if all agents in this wave already have completion reports
	if m.CompletionReports != nil && len(wave.Agents) > 0 {
		completedCount := 0
		for _, agent := range wave.Agents {
			if report, ok := m.CompletionReports[agent.ID]; ok && report.Status == protocol.StatusComplete {
				completedCount++
			}
		}
		if completedCount == len(wave.Agents) {
			if _, agentDone := m.CompletionReports[agentID]; agentDone {
				return PreLaunchCheck{
					Name:    "wave_exists",
					Status:  "warn",
					Message: fmt.Sprintf("wave %d: agent %s already complete (all %d agents done)", waveNum, agentID, completedCount),
				}
			}
			return PreLaunchCheck{
				Name:    "wave_exists",
				Status:  "warn",
				Message: fmt.Sprintf("wave %d already has all %d agents completed", waveNum, completedCount),
			}
		}
	}

	return PreLaunchCheck{
		Name:    "wave_exists",
		Status:  "pass",
		Message: fmt.Sprintf("wave %d exists with %d agents", waveNum, len(wave.Agents)),
	}
}

func checkAgentExists(m *protocol.IMPLManifest, waveNum int, agentID string) PreLaunchCheck {
	for _, wave := range m.Waves {
		if wave.Number == waveNum {
			for _, agent := range wave.Agents {
				if agent.ID == agentID {
					return PreLaunchCheck{
						Name:    "agent_exists",
						Status:  "pass",
						Message: fmt.Sprintf("agent %s found in wave %d", agentID, waveNum),
					}
				}
			}
			return PreLaunchCheck{
				Name:    "agent_exists",
				Status:  "fail",
				Message: fmt.Sprintf("agent %s not found in wave %d", agentID, waveNum),
			}
		}
	}

	// Wave doesn't exist - agent check is moot but report clearly
	return PreLaunchCheck{
		Name:    "agent_exists",
		Status:  "fail",
		Message: fmt.Sprintf("cannot check agent %s: wave %d not found", agentID, waveNum),
	}
}

func checkWorktreeBranch(m *protocol.IMPLManifest, waveNum int, agentID string, wtPath string) PreLaunchCheck {
	if wtPath == "" {
		return PreLaunchCheck{
			Name:    "worktree_branch",
			Status:  "pass",
			Message: "no worktree path specified (integration wave)",
		}
	}

	// Check directory exists
	if _, err := os.Stat(wtPath); os.IsNotExist(err) {
		return PreLaunchCheck{
			Name:    "worktree_branch",
			Status:  "fail",
			Message: fmt.Sprintf("worktree path does not exist: %s", wtPath),
		}
	}

	// Check current branch
	branch, err := git.Run(wtPath, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return PreLaunchCheck{
			Name:    "worktree_branch",
			Status:  "fail",
			Message: fmt.Sprintf("failed to get branch: %v", err),
		}
	}
	branch = strings.TrimSpace(branch)

	// Expected patterns:
	//   saw/{slug}/wave{N}-agent-{ID}
	//   wave{N}-agent-{ID} (legacy)
	slug := m.FeatureSlug
	expectedFull := fmt.Sprintf("saw/%s/wave%d-agent-%s", slug, waveNum, agentID)
	expectedLegacy := fmt.Sprintf("wave%d-agent-%s", waveNum, agentID)

	if branch == expectedFull || branch == expectedLegacy {
		return PreLaunchCheck{
			Name:    "worktree_branch",
			Status:  "pass",
			Message: fmt.Sprintf("branch %s matches expected pattern", branch),
		}
	}

	return PreLaunchCheck{
		Name:    "worktree_branch",
		Status:  "fail",
		Message: fmt.Sprintf("branch %q does not match expected %q or %q", branch, expectedFull, expectedLegacy),
	}
}

func checkScaffoldsCommitted(m *protocol.IMPLManifest) PreLaunchCheck {
	if len(m.Scaffolds) == 0 {
		return PreLaunchCheck{
			Name:    "scaffolds_committed",
			Status:  "pass",
			Message: "no scaffolds required",
		}
	}

	if protocol.AllScaffoldsCommitted(m) {
		return PreLaunchCheck{
			Name:    "scaffolds_committed",
			Status:  "pass",
			Message: "all scaffolds committed",
		}
	}

	uncommitted := make([]string, 0)
	for _, s := range m.Scaffolds {
		if !strings.HasPrefix(s.Status, "committed") {
			uncommitted = append(uncommitted, s.FilePath)
		}
	}
	return PreLaunchCheck{
		Name:    "scaffolds_committed",
		Status:  "fail",
		Message: fmt.Sprintf("uncommitted scaffolds: %s", strings.Join(uncommitted, ", ")),
	}
}

func checkOwnershipDisjoint(errs []result.SAWError) PreLaunchCheck {
	i1Violations := make([]string, 0)
	for _, e := range errs {
		if e.Code == result.CodeDisjointOwnership {
			i1Violations = append(i1Violations, e.Message)
		}
	}

	if len(i1Violations) == 0 {
		return PreLaunchCheck{
			Name:    "ownership_disjoint",
			Status:  "pass",
			Message: "no I1 ownership conflicts",
		}
	}

	return PreLaunchCheck{
		Name:    "ownership_disjoint",
		Status:  "fail",
		Message: fmt.Sprintf("I1 violations: %s", strings.Join(i1Violations, "; ")),
	}
}

func checkCriticReviewRequired(m *protocol.IMPLManifest) PreLaunchCheck {
	if m.CriticReport == nil {
		if protocol.E37Required(m) {
			return PreLaunchCheck{
				Name:    "critic_review",
				Status:  "fail",
				Message: "E37: critic review required but not run",
			}
		}
		return PreLaunchCheck{
			Name:    "critic_review",
			Status:  "pass",
			Message: "critic review not required",
		}
	}

	if m.CriticReport.Verdict == protocol.CriticVerdictPass {
		return PreLaunchCheck{
			Name:    "critic_review",
			Status:  "pass",
			Message: "critic review passed",
		}
	}

	if m.CriticReport.Verdict == protocol.CriticVerdictSkipped {
		return PreLaunchCheck{
			Name:    "critic_review",
			Status:  "pass",
			Message: "critic review explicitly skipped by operator",
		}
	}

	return PreLaunchCheck{
		Name:    "critic_review",
		Status:  "fail",
		Message: fmt.Sprintf("critic review failed with %d issue(s)", m.CriticReport.IssueCount),
	}
}
