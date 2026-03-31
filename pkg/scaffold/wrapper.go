package scaffold

import (
	"context"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/protocol"
)

// DetectScaffolds is a convenience wrapper for web CLI delegation.
// It loads the IMPL doc and calls DetectScaffoldsPreAgent() on its interface contracts.
func DetectScaffolds(implPath string) (*PreAgentResult, error) {
	manifest, err := protocol.Load(context.TODO(), implPath)
	if err != nil {
		return nil, err
	}
	return DetectScaffoldsPreAgent(manifest.InterfaceContracts)
}
