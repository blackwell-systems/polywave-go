package engine

import (
	"context"
	"fmt"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/protocol"
	"github.com/blackwell-systems/scout-and-wave-go/pkg/result"
)

// FinalizeIMPLEngineOpts configures the FinalizeIMPLEngine call.
type FinalizeIMPLEngineOpts struct {
	IMPLPath string // absolute path to IMPL doc (required)
	RepoRoot string // absolute path to repo root (required)
}

// FinalizeIMPLEngine is the canonical engine wrapper around protocol.FinalizeIMPL.
// Both CLI (`sawtools finalize-impl`) and web app automation should use this
// function to ensure consistent behavior: context cancellation support,
// parameter validation, and SSE-compatible result types.
//
// It respects context cancellation and returns results compatible with SSE streaming.
//
// Usage (typical webapp flow after Scout completes):
//
//	res := FinalizeIMPLEngine(ctx, FinalizeIMPLEngineOpts{IMPLPath: implPath, RepoRoot: repoRoot})
//	if res.IsFatal() {
//	  // Handle context cancellation or validation failure
//	}
//	if !res.IsSuccess() {
//	  // Handle validation/population failures (show in UI)
//	}
func FinalizeIMPLEngine(ctx context.Context, opts FinalizeIMPLEngineOpts) result.Result[protocol.FinalizeIMPLData] {
	// Validate required parameters
	if opts.IMPLPath == "" {
		return result.NewFailure[protocol.FinalizeIMPLData]([]result.SAWError{
			result.NewFatal(result.CodeFinalizeWaveFailed, "engine.FinalizeIMPLEngine: implPath is required"),
		})
	}
	if opts.RepoRoot == "" {
		return result.NewFailure[protocol.FinalizeIMPLData]([]result.SAWError{
			result.NewFatal(result.CodeFinalizeWaveFailed, "engine.FinalizeIMPLEngine: repoRoot is required"),
		})
	}

	// Check context cancellation before starting
	select {
	case <-ctx.Done():
		return result.NewFailure[protocol.FinalizeIMPLData]([]result.SAWError{
			result.NewFatal(result.CodeContextCancelled, fmt.Sprintf("engine.FinalizeIMPLEngine: context cancelled: %v", ctx.Err())),
		})
	default:
	}

	// Run FinalizeIMPL in a goroutine to enable context cancellation
	resultCh := make(chan result.Result[protocol.FinalizeIMPLData], 1)

	go func() {
		resultCh <- protocol.FinalizeIMPL(opts.IMPLPath, opts.RepoRoot)
	}()

	// Wait for either completion or context cancellation
	select {
	case <-ctx.Done():
		return result.NewFailure[protocol.FinalizeIMPLData]([]result.SAWError{
			result.NewFatal(result.CodeContextCancelled, fmt.Sprintf("engine.FinalizeIMPLEngine: context cancelled during execution: %v", ctx.Err())),
		})
	case r := <-resultCh:
		return r
	}
}
