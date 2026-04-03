package orchestrator

import (
	"github.com/blackwell-systems/scout-and-wave-go/pkg/protocol"
)

// isValidTransition delegates to protocol.IsValidTransition — the canonical
// state machine FSM. The private validTransitions map has been removed to
// prevent silent divergence when protocol adds new states.
func isValidTransition(from, to protocol.ProtocolState) bool {
	return protocol.IsValidTransition(from, to)
}
