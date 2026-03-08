package orchestrator

import (
	"github.com/blackwell-systems/scout-and-wave-go/pkg/types"
)

// validTransitions maps each state to the set of states reachable from it.
// The SAW protocol defines 10 states with the following directed edges.
var validTransitions = map[types.State][]types.State{
	types.ScoutPending:    {types.Reviewed, types.NotSuitable},
	types.Reviewed:        {types.ScaffoldPending, types.WavePending},
	types.ScaffoldPending: {types.WavePending, types.Blocked},
	types.WavePending:     {types.WaveExecuting},
	types.WaveExecuting:   {types.WaveMerging, types.WaveVerified, types.Blocked},
	types.WaveMerging:     {types.WaveVerified, types.Blocked},
	types.WaveVerified:    {types.Complete, types.WavePending},
	types.Blocked:         {types.WavePending, types.WaveVerified},
	types.NotSuitable:     {},
	types.Complete:        {},
}

// isValidTransition returns true if transitioning from -> to is permitted
// by the SAW protocol state machine.
func isValidTransition(from, to types.State) bool {
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
