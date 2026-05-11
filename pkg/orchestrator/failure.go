// E19: Failure type routing decision tree — see execution-rules.md §E19
package orchestrator

import (
	"github.com/blackwell-systems/polywave-go/pkg/protocol"
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
func RouteFailure(failureType protocol.FailureTypeEnum) OrchestratorAction {
	switch failureType {
	case protocol.FailureTransient:
		return ActionRetry
	case protocol.FailureFixable:
		return ActionApplyAndRelaunch
	case protocol.FailureNeedsReplan:
		return ActionReplan
	case protocol.FailureEscalate:
		return ActionEscalate
	case protocol.FailureTimeout:
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
func RouteFailureWithReactions(failureType protocol.FailureTypeEnum, reactions *protocol.ReactionsConfig) OrchestratorAction {
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
func MaxAttemptsFor(failureType protocol.FailureTypeEnum, reactions *protocol.ReactionsConfig) int {
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
func defaultMaxAttempts(failureType protocol.FailureTypeEnum) int {
	switch failureType {
	case protocol.FailureTransient:
		return 2
	case protocol.FailureFixable:
		return 2 // matches protocol.MaxRetries(FailureFixable)
	case protocol.FailureTimeout:
		return 1
	default:
		return 0
	}
}

// reactionEntryForOrchestrator maps an orchestrator FailureType to the
// corresponding protocol.ReactionEntry, or nil if not set.
func reactionEntryForOrchestrator(failureType protocol.FailureTypeEnum, r *protocol.ReactionsConfig) *protocol.ReactionEntry {
	if r == nil {
		return nil
	}
	switch failureType {
	case protocol.FailureTransient:
		return r.Transient
	case protocol.FailureTimeout:
		return r.Timeout
	case protocol.FailureFixable:
		return r.Fixable
	case protocol.FailureNeedsReplan:
		return r.NeedsReplan
	case protocol.FailureEscalate:
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
