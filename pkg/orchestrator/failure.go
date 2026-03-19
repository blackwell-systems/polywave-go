// E19: Failure type routing decision tree — see execution-rules.md §E19
package orchestrator

import (
	"github.com/blackwell-systems/scout-and-wave-go/pkg/protocol"
	"github.com/blackwell-systems/scout-and-wave-go/pkg/types"
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

// RouteFailureWithReactions maps a FailureType to an OrchestratorAction,
// consulting the IMPL manifest's reactions block before falling back to
// the E19 hardcoded switch. reactions may be nil — if so, behavior is
// identical to RouteFailure.
//
// Wiring note: in orchestrator.go, the manifest is already loaded at line ~744.
// Do NOT call protocol.Load a second time. Instead:
//  1. Hoist the manifest variable to be accessible at the failure routing block (~line 788)
//  2. Replace:
//       action := RouteFailure(failureType)
//     With:
//       var reactions *protocol.ReactionsConfig
//       if manifest != nil { reactions = manifest.Reactions }
//       action := RouteFailureWithReactions(failureType, reactions)
func RouteFailureWithReactions(failureType types.FailureType, reactions *protocol.ReactionsConfig) OrchestratorAction {
	if reactions != nil {
		entry := reactionEntryForOrchestrator(failureType, reactions)
		if entry != nil {
			return actionFromReactionEntry(entry)
		}
	}
	return RouteFailure(failureType)
}

// MaxAttemptsFor returns the max launch attempts for a failure type,
// consulting reactions first, then falling back to the E19 default.
// reactions may be nil.
func MaxAttemptsFor(failureType types.FailureType, reactions *protocol.ReactionsConfig) int {
	if reactions == nil {
		return defaultMaxAttempts(failureType)
	}
	entry := reactionEntryForOrchestrator(failureType, reactions)
	if entry == nil || entry.MaxAttempts == 0 {
		return defaultMaxAttempts(failureType)
	}
	return entry.MaxAttempts
}

// defaultMaxAttempts returns the E19 hardcoded max attempts for a FailureType.
// IMPORTANT: these values must match protocol.MaxRetries exactly.
// Verify against pkg/protocol/failure.go before committing.
func defaultMaxAttempts(failureType types.FailureType) int {
	switch failureType {
	case types.FailureTypeTransient:
		return 2
	case types.FailureTypeFixable:
		return 2 // matches protocol.MaxRetries(FailureFixable)
	case types.FailureTypeTimeout:
		return 1
	default:
		return 0
	}
}

// reactionEntryForOrchestrator maps an orchestrator FailureType to the
// corresponding protocol.ReactionEntry, or nil if not set.
func reactionEntryForOrchestrator(failureType types.FailureType, r *protocol.ReactionsConfig) *protocol.ReactionEntry {
	if r == nil {
		return nil
	}
	switch failureType {
	case types.FailureTypeTransient:
		return r.Transient
	case types.FailureTypeTimeout:
		return r.Timeout
	case types.FailureTypeFixable:
		return r.Fixable
	case types.FailureTypeNeedsReplan:
		return r.NeedsReplan
	case types.FailureTypeEscalate:
		return r.Escalate
	default:
		return nil
	}
}

// actionFromReactionEntry converts a protocol.ReactionEntry.Action string
// to an OrchestratorAction constant. Unknown/empty action falls back to ActionEscalate.
//
// Action mapping:
//   - "retry"           → ActionRetry (plain retry, no scope note)
//   - "send-fix-prompt" → ActionApplyAndRelaunch
//   - "pause"           → ActionEscalate (surfaces to human; no distinct pause state in current model)
//   - "auto-scout"      → ActionReplan (re-engages Scout)
func actionFromReactionEntry(entry *protocol.ReactionEntry) OrchestratorAction {
	if entry == nil {
		return ActionEscalate
	}
	switch entry.Action {
	case "retry":
		// Note: for timeout failures, the E19 default is ActionRetryWithScope
		// (retry with scope-reduction note). When a reactions entry sets
		// action: retry for timeout, we use plain ActionRetry (no scope note).
		// This is a known semantic difference from the E19 default for timeout.
		// If scope-reduced retry is desired, the user should omit the timeout
		// reaction and let E19 defaults apply.
		return ActionRetry
	case "send-fix-prompt":
		return ActionApplyAndRelaunch
	case "pause":
		return ActionEscalate // pause surfaces to human — same as escalate in current model
	case "auto-scout":
		return ActionReplan // auto-scout re-engages Scout
	default:
		return ActionEscalate
	}
}
