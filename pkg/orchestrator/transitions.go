package orchestrator

import (
	"github.com/blackwell-systems/scout-and-wave-go/pkg/protocol"
)

// validTransitions is the orchestrator's private state machine.
// It maps each protocol state to the set of states reachable from it,
// mirroring SM-02 for the orchestrator's own transition tracking.
// The authoritative protocol-level FSM is SetImplState in pkg/protocol/state_transition.go.
var validTransitions = map[protocol.ProtocolState][]protocol.ProtocolState{
	protocol.StateScoutPending:    {protocol.StateScoutValidating, protocol.StateReviewed, protocol.StateNotSuitable, protocol.StateBlocked},
	protocol.StateScoutValidating: {protocol.StateReviewed, protocol.StateScoutValidating, protocol.StateBlocked},
	protocol.StateReviewed:        {protocol.StateScaffoldPending, protocol.StateWavePending, protocol.StateBlocked},
	protocol.StateScaffoldPending: {protocol.StateWavePending, protocol.StateBlocked},
	protocol.StateWavePending:     {protocol.StateWaveExecuting, protocol.StateBlocked},
	protocol.StateWaveExecuting:   {protocol.StateWaveMerging, protocol.StateBlocked},
	protocol.StateWaveMerging:     {protocol.StateWaveVerified, protocol.StateBlocked},
	protocol.StateWaveVerified:    {protocol.StateComplete, protocol.StateWavePending, protocol.StateBlocked},
	protocol.StateBlocked:         {protocol.StateScoutPending, protocol.StateScoutValidating, protocol.StateReviewed, protocol.StateScaffoldPending, protocol.StateWavePending, protocol.StateWaveExecuting, protocol.StateWaveMerging, protocol.StateWaveVerified, protocol.StateComplete, protocol.StateNotSuitable},
	protocol.StateNotSuitable:     {},
	protocol.StateComplete:        {},
}

// isValidTransition returns true if transitioning from -> to is permitted
// by the orchestrator's private state machine (validTransitions above).
func isValidTransition(from, to protocol.ProtocolState) bool {
	allowed, ok := validTransitions[from]
	if !ok {
		return false
	}
	for _, s := range allowed {
		if s == to {
			return true
		}
	}
	return false
}
