package engine

import (
	"context"
	"fmt"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/protocol"
	"github.com/blackwell-systems/scout-and-wave-go/pkg/result"
)

// FinalizeIMPLEngine is the canonical engine wrapper around protocol.FinalizeIMPL.
// Both CLI (`sawtools finalize-impl`) and web app automation should use this
// function to ensure consistent behavior: context cancellation support,
// parameter validation, and SSE-compatible result types.
//
// It respects context cancellation and returns results compatible with SSE streaming.
//
// Usage (typical webapp flow after Scout completes):
//
//	res, err := FinalizeIMPLEngine(ctx, implPath, repoRoot)
//	if err != nil {
//	  // Handle context cancellation
//	}
//	if !res.IsSuccess() {
//	  // Handle validation/population failures (show in UI)
//	}
func FinalizeIMPLEngine(ctx context.Context, implPath, repoRoot string) (result.Result[protocol.FinalizeIMPLData], error) {
	// Validate required parameters
	if implPath == "" {
		return result.Result[protocol.FinalizeIMPLData]{}, fmt.Errorf("engine.FinalizeIMPLEngine: implPath is required")
	}
	if repoRoot == "" {
		return result.Result[protocol.FinalizeIMPLData]{}, fmt.Errorf("engine.FinalizeIMPLEngine: repoRoot is required")
	}

	// Check context cancellation before starting
	select {
	case <-ctx.Done():
		return result.Result[protocol.FinalizeIMPLData]{}, ctx.Err()
	default:
	}

	// Run FinalizeIMPL in a goroutine to enable context cancellation
	resultCh := make(chan result.Result[protocol.FinalizeIMPLData], 1)

	go func() {
		resultCh <- protocol.FinalizeIMPL(implPath, repoRoot)
	}()

	// Wait for either completion or context cancellation
	select {
	case <-ctx.Done():
		return result.Result[protocol.FinalizeIMPLData]{}, ctx.Err()
	case r := <-resultCh:
		return r, nil
	}
}
