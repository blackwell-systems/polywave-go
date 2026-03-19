package protocol

// FailureTypeEnum represents the type of failure reported by a Wave agent (E19).
type FailureTypeEnum string

const (
	// FailureTransient indicates a temporary failure that should be retried automatically.
	FailureTransient FailureTypeEnum = "transient"

	// FailureFixable indicates a failure that can be fixed by applying the agent's suggested fix.
	FailureFixable FailureTypeEnum = "fixable"

	// FailureNeedsReplan indicates the IMPL doc decomposition is incorrect and requires Scout re-engagement.
	FailureNeedsReplan FailureTypeEnum = "needs_replan"

	// FailureEscalate indicates a critical issue requiring immediate human intervention.
	FailureEscalate FailureTypeEnum = "escalate"

	// FailureTimeout indicates the agent ran out of time and should retry with scope reduction.
	FailureTimeout FailureTypeEnum = "timeout"
)

// ShouldRetry returns true if the failure type is automatically retryable.
// Per E19:
//   - transient: auto-retry
//   - fixable: auto-retry after applying fix
//   - timeout: retry with scope reduction
//   - needs_replan: no auto-retry
//   - escalate: no auto-retry
func ShouldRetry(failureType FailureTypeEnum) bool {
	switch failureType {
	case FailureTransient, FailureFixable, FailureTimeout:
		return true
	case FailureNeedsReplan, FailureEscalate:
		return false
	default:
		return false
	}
}

// MaxRetries returns the maximum number of automatic retries for a failure type.
// Per E19:
//   - transient: 2 retries
//   - fixable: 2 retries (after applying fix)
//   - timeout: 1 retry (with scope reduction)
//   - needs_replan: 0 retries
//   - escalate: 0 retries
func MaxRetries(failureType FailureTypeEnum) int {
	switch failureType {
	case FailureTransient:
		return 2
	case FailureFixable:
		return 2
	case FailureTimeout:
		return 1
	case FailureNeedsReplan:
		return 0
	case FailureEscalate:
		return 0
	default:
		return 0
	}
}

// ActionRequired returns a human-readable description of what action is needed.
// Per E19:
//   - transient: "Retry automatically"
//   - fixable: "Apply agent's suggested fix, then retry"
//   - timeout: "Retry with scope-reduction instructions"
//   - needs_replan: "Re-engage Scout with agent's completion report"
//   - escalate: "Surface to human immediately"
func ActionRequired(failureType FailureTypeEnum) string {
	switch failureType {
	case FailureTransient:
		return "Retry automatically"
	case FailureFixable:
		return "Apply agent's suggested fix, then retry"
	case FailureTimeout:
		return "Retry with scope-reduction instructions"
	case FailureNeedsReplan:
		return "Re-engage Scout with agent's completion report"
	case FailureEscalate:
		return "Surface to human immediately"
	default:
		return "Unknown failure type: escalate to human"
	}
}

// MaxRetriesWithReactions returns the max retry count for a failure type,
// consulting the reactions block first, then falling back to MaxRetries.
// reactions may be nil (reactions block absent) — returns MaxRetries in that case.
func MaxRetriesWithReactions(failureType FailureTypeEnum, reactions *ReactionsConfig) int {
	if reactions == nil {
		return MaxRetries(failureType)
	}
	entry := reactionEntryFor(failureType, reactions)
	if entry == nil || entry.MaxAttempts == 0 {
		return MaxRetries(failureType)
	}
	return entry.MaxAttempts
}

// reactionEntryFor returns the ReactionEntry for a given FailureTypeEnum, or nil.
func reactionEntryFor(failureType FailureTypeEnum, r *ReactionsConfig) *ReactionEntry {
	if r == nil {
		return nil
	}
	switch failureType {
	case FailureTransient:
		return r.Transient
	case FailureTimeout:
		return r.Timeout
	case FailureFixable:
		return r.Fixable
	case FailureNeedsReplan:
		return r.NeedsReplan
	case FailureEscalate:
		return r.Escalate
	default:
		return nil
	}
}

// ShouldRetryWithReactions returns true if the reactions block (or E19 default)
// calls for automatic retry of this failure type.
func ShouldRetryWithReactions(failureType FailureTypeEnum, reactions *ReactionsConfig) bool {
	entry := reactionEntryFor(failureType, reactions)
	if entry == nil {
		return ShouldRetry(failureType)
	}
	return entry.Action == "retry" || entry.Action == "send-fix-prompt"
}

// ValidFailureType returns true if the given string is a valid FailureTypeEnum value.
func ValidFailureType(s string) bool {
	switch FailureTypeEnum(s) {
	case FailureTransient, FailureFixable, FailureNeedsReplan, FailureEscalate, FailureTimeout:
		return true
	default:
		return false
	}
}
