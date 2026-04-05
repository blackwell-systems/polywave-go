package agent

import (
	"context"
	"fmt"
	"time"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/protocol"
	"github.com/blackwell-systems/scout-and-wave-go/pkg/result"
)

// WaitForCompletion polls the IMPL manifest at implDocPath every pollInterval until
// the completion report for agentID appears or the timeout is reached.
//
// Returns the completion report on success, or an error if the timeout is reached
// or the manifest cannot be loaded.
//
// Note: Returns protocol.CompletionReport which uses string types for status/failure_type.
// Callers needing types.CompletionReport (with typed enums) should convert manually.
// For structured error handling, use WaitForCompletionResult.
func WaitForCompletion(ctx context.Context, implDocPath, agentID string, timeout, pollInterval time.Duration) (*protocol.CompletionReport, error) {
	r := WaitForCompletionResult(ctx, implDocPath, agentID, timeout, pollInterval)
	if r.IsFatal() {
		return nil, fmt.Errorf("%s", r.Errors[0].Message)
	}
	return r.GetData(), nil
}

// WaitForCompletionResult polls the IMPL manifest at implDocPath every pollInterval
// until the completion report for agentID appears or the timeout is reached.
//
// Returns a result.Result containing the completion report on success, or a fatal
// result with a structured error code on failure.
func WaitForCompletionResult(ctx context.Context, implDocPath, agentID string, timeout, pollInterval time.Duration) result.Result[*protocol.CompletionReport] {
	deadline := time.Now().Add(timeout)

	for {
		// Load the YAML manifest
		manifest, err := protocol.Load(ctx, implDocPath)
		if err != nil {
			return result.NewFailure[*protocol.CompletionReport]([]result.SAWError{
				result.NewFatal("AGENT_COMPLETION_LOAD_FAILED",
					fmt.Sprintf("WaitForCompletion agent %s: failed to load manifest: %v", agentID, err)),
			})
		}

		// Check if completion report exists in the map
		if report, ok := manifest.CompletionReports[agentID]; ok {
			return result.NewSuccess(&report)
		}

		// Report not found yet — check whether we have time to retry.
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return result.NewFailure[*protocol.CompletionReport]([]result.SAWError{
				result.NewFatal("AGENT_COMPLETION_TIMEOUT",
					fmt.Sprintf("WaitForCompletion agent %s: timed out after %s waiting for completion report in %q", agentID, timeout, implDocPath)),
			})
		}

		// Sleep at most the remaining time so we never overshoot the deadline.
		// Use select so the caller's context cancellation interrupts the sleep.
		sleep := pollInterval
		if sleep > remaining {
			sleep = remaining
		}
		select {
		case <-time.After(sleep):
		case <-ctx.Done():
			return result.NewFailure[*protocol.CompletionReport]([]result.SAWError{
				result.NewFatal("AGENT_COMPLETION_CANCELLED",
					fmt.Sprintf("WaitForCompletion agent %s: context cancelled while waiting for completion report", agentID)),
			})
		}
	}
}
