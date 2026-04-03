package protocol

// IMPLStateToStatus maps a ProtocolState to a program IMPL status string.
// Returns: "complete", "in-progress", "pending", "blocked", "not-suitable",
// or string(state) for unknown states.
func IMPLStateToStatus(state ProtocolState) string {
	switch state {
	case StateComplete:
		return "complete"
	case StateWaveExecuting, StateWaveMerging, StateWaveVerified, StateWavePending:
		return "in-progress"
	case StateInterviewing, StateReviewed, StateScaffoldPending, StateScoutPending, StateScoutValidating:
		return "pending"
	case StateBlocked:
		return "blocked"
	case StateNotSuitable:
		return "not-suitable"
	default:
		return string(state)
	}
}
