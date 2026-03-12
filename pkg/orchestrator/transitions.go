package orchestrator

import (
	"github.com/blackwell-systems/scout-and-wave-go/pkg/protocol"
)

// validTransitions maps each state to the set of states reachable from it.
// The SAW protocol defines 11 states with the following directed edges.
var validTransitions = map[protocol.ProtocolState][]protocol.ProtocolState{
	protocol.StateScoutPending:    {protocol.StateScoutValidating, protocol.StateNotSuitable},
	protocol.StateScoutValidating: {protocol.StateReviewed, protocol.StateScoutValidating, protocol.StateBlocked},
	protocol.StateReviewed:        {protocol.StateScaffoldPending, protocol.StateWavePending},
	protocol.StateScaffoldPending: {protocol.StateWavePending, protocol.StateBlocked},
	protocol.StateWavePending:     {protocol.StateWaveExecuting},
	protocol.StateWaveExecuting:   {protocol.StateWaveMerging, protocol.StateWaveVerified, protocol.StateBlocked},
	protocol.StateWaveMerging:     {protocol.StateWaveVerified, protocol.StateBlocked},
	protocol.StateWaveVerified:    {protocol.StateComplete, protocol.StateWavePending},
	protocol.StateBlocked:         {protocol.StateWavePending, protocol.StateWaveVerified},
	protocol.StateNotSuitable:     {},
	protocol.StateComplete:        {},
}

// isValidTransition returns true if transitioning from -> to is permitted
// by the SAW protocol state machine.
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
