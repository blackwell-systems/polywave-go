// Package orchestrator implements the SAW protocol state machine and
// the Orchestrator struct that drives wave coordination.
package orchestrator

import (
	"github.com/blackwell-systems/polywave-go/pkg/protocol"
)

// State is a re-export alias so callers can use orchestrator.State
// in addition to protocol.ProtocolState if they prefer.
type State = protocol.ProtocolState
