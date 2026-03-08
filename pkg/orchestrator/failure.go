// E19: Failure type routing decision tree — see execution-rules.md §E19
package orchestrator

import "github.com/blackwell-systems/scout-and-wave-go/pkg/types"

// OrchestratorAction is the action to take after a partial/blocked completion report.
type OrchestratorAction int

const (
	ActionRetry           OrchestratorAction = iota // E19: transient — retry up to 2 times
	ActionApplyAndRelaunch                          // E19: fixable — apply fix from notes, relaunch once
	ActionReplan                                    // E19: needs_replan — re-engage Scout
	ActionEscalate                                  // E19: escalate — surface to human
	ActionRetryWithScope                            // E19: timeout — retry once with scope-reduction note
)

// RouteFailure maps a FailureType to an OrchestratorAction per E19.
// Empty failureType (absent from report) returns ActionEscalate (conservative fallback).
func RouteFailure(failureType types.FailureType) OrchestratorAction {
	switch failureType {
	case types.FailureTypeTransient:
		return ActionRetry
	case types.FailureTypeFixable:
		return ActionApplyAndRelaunch
	case types.FailureTypeNeedsReplan:
		return ActionReplan
	case types.FailureTypeEscalate:
		return ActionEscalate
	case types.FailureTypeTimeout:
		return ActionRetryWithScope
	default:
		// Covers both empty string (absent from report) and any unknown value.
		return ActionEscalate
	}
}
