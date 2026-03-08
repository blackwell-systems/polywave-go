package agent

import (
	"errors"
	"fmt"
	"time"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/protocol"
	"github.com/blackwell-systems/scout-and-wave-go/pkg/types"
)

// WaitForCompletion polls the IMPL doc at implDocPath every pollInterval until
// the completion report for agentLetter appears or the timeout is reached.
//
// On ErrReportNotFound the function sleeps pollInterval and retries.
// On any other error from ParseCompletionReport it returns immediately.
// When timeout elapses without a report it returns a descriptive error.
func WaitForCompletion(implDocPath, agentLetter string, timeout, pollInterval time.Duration) (*types.CompletionReport, error) {
	deadline := time.Now().Add(timeout)

	for {
		report, err := protocol.ParseCompletionReport(implDocPath, agentLetter)
		if err == nil {
			return report, nil
		}

		if !errors.Is(err, protocol.ErrReportNotFound) {
			return nil, fmt.Errorf("WaitForCompletion agent %s: %w", agentLetter, err)
		}

		// Report not found yet — check whether we have time to retry.
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return nil, fmt.Errorf("WaitForCompletion agent %s: timed out after %s waiting for completion report in %q",
				agentLetter, timeout, implDocPath)
		}

		// Sleep at most the remaining time so we never overshoot the deadline.
		sleep := pollInterval
		if sleep > remaining {
			sleep = remaining
		}
		time.Sleep(sleep)
	}
}
