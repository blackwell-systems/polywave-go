package engine

import (
	"context"
	"fmt"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/protocol"
)

// FinalizeIMPLEngine is an engine wrapper around protocol.FinalizeIMPL for webapp automation.
// It respects context cancellation and returns results compatible with SSE streaming.
//
// Usage (typical webapp flow after Scout completes):
//   result, err := FinalizeIMPLEngine(ctx, implPath, repoRoot)
//   if err != nil {
//     // Handle error (log, SSE error event)
//   }
//   if !result.Success {
//     // Handle validation/population failures (show in UI)
//   }
func FinalizeIMPLEngine(ctx context.Context, implPath, repoRoot string) (*protocol.FinalizeIMPLResult, error) {
	// Validate required parameters
	if implPath == "" {
		return nil, fmt.Errorf("engine.FinalizeIMPLEngine: implPath is required")
	}
	if repoRoot == "" {
		return nil, fmt.Errorf("engine.FinalizeIMPLEngine: repoRoot is required")
	}

	// Check context cancellation before starting
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	// Run FinalizeIMPL in a goroutine to enable context cancellation
	type resultPair struct {
		result *protocol.FinalizeIMPLResult
		err    error
	}
	resultCh := make(chan resultPair, 1)

	go func() {
		result, err := protocol.FinalizeIMPL(implPath, repoRoot)
		resultCh <- resultPair{result: result, err: err}
	}()

	// Wait for either completion or context cancellation
	select {
	case <-ctx.Done():
		// Context cancelled - return context error
		return nil, ctx.Err()
	case rp := <-resultCh:
		// Operation completed
		return rp.result, rp.err
	}
}
