package resume

import (
	"fmt"

	"github.com/blackwell-systems/polywave-go/pkg/protocol"
)

// BuildRecoverySteps analyzes state and returns prioritized recovery steps.
// Steps are ordered by Priority (1 = highest urgency, 3 = lowest).
// The manifest provides CompletionReports for failed agents.
// Passing a nil manifest is safe; failure type classification is skipped.
func BuildRecoverySteps(state SessionState, manifest *protocol.IMPLManifest) []protocol.RecoveryStep {
	var steps []protocol.RecoveryStep

	// Priority 1a: Failed agents — diagnostic command for each
	for _, agentID := range state.FailedAgents {
		category := protocol.FailureCategoryUnknown
		if manifest != nil {
			if report, ok := manifest.CompletionReports[agentID]; ok {
				switch report.FailureType {
				case "fixable":
					category = protocol.FailureCategoryFixable
				case "transient":
					category = protocol.FailureCategoryTransient
				case "needs_replan":
					category = protocol.FailureCategoryNeedsReplan
				case "timeout":
					category = protocol.FailureCategoryTimeout
				}
			}
		}
		steps = append(steps, protocol.RecoveryStep{
			Priority:    1,
			Description: fmt.Sprintf("Analyze failure details for agent %s", agentID),
			Command:     fmt.Sprintf("polywave-tools build-retry-context %s --agent %s", state.IMPLPath, agentID),
			Category:    category,
		})
	}

	// Priority 1b: Dirty worktrees — review uncommitted work before resuming
	for _, dw := range state.DirtyWorktrees {
		if dw.HasChanges {
			steps = append(steps, protocol.RecoveryStep{
				Priority:    1,
				Description: fmt.Sprintf("Review uncommitted changes in worktree for agent %s", dw.AgentID),
				Command:     fmt.Sprintf("git -C %s status", dw.Path),
				Category:    protocol.FailureCategoryTransient,
			})
		}
	}

	// Priority 2: Orphaned worktrees (clean) — cleanup before resuming
	if len(state.OrphanedWorktrees) > 0 {
		anyDirty := false
		for _, dw := range state.DirtyWorktrees {
			if dw.HasChanges {
				anyDirty = true
				break
			}
		}
		if !anyDirty {
			steps = append(steps, protocol.RecoveryStep{
				Priority:    2,
				Description: fmt.Sprintf("Clean up %d orphaned worktree(s) before resuming", len(state.OrphanedWorktrees)),
				Command:     fmt.Sprintf("polywave-tools cleanup %s --wave %d", state.IMPLPath, state.CurrentWave),
				Category:    protocol.FailureCategoryTransient,
			})
		}
	}

	// Priority 3: Resume command (present when ResumeCommand is non-empty)
	if state.ResumeCommand != "" {
		steps = append(steps, protocol.RecoveryStep{
			Priority:    3,
			Description: state.SuggestedAction,
			Command:     state.ResumeCommand,
			Category:    protocol.FailureCategoryUnknown,
		})
	}

	return steps
}
