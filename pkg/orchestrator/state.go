// Package orchestrator implements the SAW protocol state machine and
// the Orchestrator struct that drives wave coordination.
package orchestrator

import (
	"github.com/blackwell-systems/scout-and-wave-go/pkg/types"
)

// State is a re-export alias so callers can use orchestrator.State
// in addition to types.State if they prefer.
type State = types.State
