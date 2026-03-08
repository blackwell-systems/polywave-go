// E19: Failure type routing decision tree — see execution-rules.md §E19
package orchestrator

// TEMP: remove after merge with Agent A.
// FailureType mirrors types.FailureType which will be defined in pkg/types by Agent A.
// Once merged, update RouteFailure's parameter type to types.FailureType and remove this block.
type FailureType = string

const (
	FailureTypeTransient   FailureType = "transient"
	FailureTypeFixable     FailureType = "fixable"
	FailureTypeNeedsReplan FailureType = "needs_replan"
	FailureTypeEscalate    FailureType = "escalate"
	FailureTypeTimeout     FailureType = "timeout"
)

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
// TEMP: parameter type should be types.FailureType after merge with Agent A.
func RouteFailure(failureType FailureType) OrchestratorAction {
	switch failureType {
	case FailureTypeTransient:
		return ActionRetry
	case FailureTypeFixable:
		return ActionApplyAndRelaunch
	case FailureTypeNeedsReplan:
		return ActionReplan
	case FailureTypeEscalate:
		return ActionEscalate
	case FailureTypeTimeout:
		return ActionRetryWithScope
	default:
		// Covers both empty string (absent from report) and any unknown value.
		return ActionEscalate
	}
}
