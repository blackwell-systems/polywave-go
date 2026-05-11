package scaffold

import (
	"context"
	"fmt"

	"github.com/blackwell-systems/polywave-go/pkg/protocol"
)

// DetectScaffolds is a convenience wrapper for web CLI delegation.
// It loads the IMPL doc and calls DetectScaffoldsPreAgent() on its interface contracts.
func DetectScaffolds(ctx context.Context, implPath string) (*PreAgentResult, error) {
	manifest, err := protocol.Load(ctx, implPath)
	if err != nil {
		return nil, fmt.Errorf("scaffold detection: load manifest: %w", err)
	}
	return DetectScaffoldsPreAgent(manifest.InterfaceContracts)
}
