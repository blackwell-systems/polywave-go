package orchestrator

import (
	"context"
	"fmt"
	"strings"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/agent"
	"github.com/blackwell-systems/scout-and-wave-go/pkg/protocol"
	"github.com/blackwell-systems/scout-and-wave-go/pkg/types" // kept for types.FailureType until failure.go is migrated (Agent A)
	"github.com/blackwell-systems/scout-and-wave-go/pkg/worktree"
)

// runWaveAgentStructuredFunc is a seam for tests: replaces the real
// runWaveAgentStructured call so tests do not need a real API/Bedrock endpoint.
//
// To avoid a circular import (orchestrator → engine → orchestrator) we declare
// it as a local function variable and let the engine package inject the real
// implementation via SetRunWaveAgentStructuredFunc (analogous to SetParseIMPLDocFunc).
//
// Default: a no-op that always returns an error so misconfiguration is visible.
var runWaveAgentStructuredFunc = func(
	ctx context.Context,
	implPath string,
	waveModel string,
	agentSpec protocol.Agent,
	wtPath string,
	onChunk func(string),
) error {
	return fmt.Errorf("orchestrator: runWaveAgentStructuredFunc not injected; call SetRunWaveAgentStructuredFunc first")
}

// SetRunWaveAgentStructuredFunc injects the real runWaveAgentStructured
// implementation from pkg/engine, breaking the circular import.
// Must be called from pkg/engine's init() before any wave execution.
func SetRunWaveAgentStructuredFunc(f func(ctx context.Context, implPath, waveModel string, agentSpec protocol.Agent, wtPath string, onChunk func(string)) error) {
	runWaveAgentStructuredFunc = f
}

// launchAgentStructured is called instead of launchAgent when structured output
// is enabled (UseStructuredOutput = true in RunWaveOpts).
//
// Behaviour per backend:
//   - API backend (no "bedrock:" prefix)  → calls runWaveAgentStructuredFunc
//   - Bedrock backend ("bedrock:" prefix) → calls runWaveAgentStructuredFunc
//   - CLI backend ("cli:" prefix)         → falls back to launchAgent (CLI
//     agents write their own completion reports; no structured output needed)
//
// The method still creates a worktree, publishes SSE events, and handles E19
// routing — identical to launchAgent — but delegates completion-report
// production to runWaveAgentStructuredFunc instead of the polling loop.
func (o *Orchestrator) launchAgentStructured(
	ctx context.Context,
	runner *agent.Runner,
	wm *worktree.Manager,
	waveNum int,
	protoAgent protocol.Agent,
) error {
	// Resolve effective model for backend detection.
	model := o.defaultModel
	if protoAgent.Model != "" {
		model = protoAgent.Model
	}

	// CLI backends write their own completion reports; fall back to the standard path.
	if strings.HasPrefix(model, "cli:") || model == "cli" {
		return o.launchAgent(ctx, runner, wm, waveNum, protoAgent)
	}

	// Create the worktree.
	wtPath, err := worktreeCreatorFunc(wm, waveNum, protoAgent.ID)
	if err != nil {
		o.publish(OrchestratorEvent{
			Event: "agent_failed",
			Data: AgentFailedPayload{
				Agent:       protoAgent.ID,
				Wave:        waveNum,
				Status:      "failed",
				FailureType: "worktree_creation",
				Message:     err.Error(),
			},
		})
		return fmt.Errorf("orchestrator: agent %s: create worktree: %w", protoAgent.ID, err)
	}

	// Publish agent_started.
	o.publish(OrchestratorEvent{
		Event: "agent_started",
		Data: AgentStartedPayload{
			Agent: protoAgent.ID,
			Wave:  waveNum,
			Files: protoAgent.Files,
		},
	})

	// Run the agent via structured output. The function saves the completion
	// report to the manifest under implPath when successful.
	runErr := runWaveAgentStructuredFunc(
		ctx,
		o.implDocPath,
		o.defaultModel,
		protoAgent,
		wtPath,
		func(chunk string) {
			o.publish(OrchestratorEvent{
				Event: "agent_output",
				Data: AgentOutputPayload{
					Agent: protoAgent.ID,
					Wave:  waveNum,
					Chunk: chunk,
				},
			})
		},
	)
	if runErr != nil {
		o.publish(OrchestratorEvent{
			Event: "agent_failed",
			Data: AgentFailedPayload{
				Agent:       protoAgent.ID,
				Wave:        waveNum,
				Status:      "failed",
				FailureType: "execute",
				Message:     runErr.Error(),
			},
		})
		return fmt.Errorf("orchestrator: agent %s: structured execute: %w", protoAgent.ID, runErr)
	}

	// Re-load the manifest to get the saved completion report for status/E19 routing.
	branch := protocol.BranchName(o.implSlug(), waveNum, protoAgent.ID)
	status := "complete"

	var savedStatus string
	var savedFailureType string

	reportMu.Lock()
	if manifest, loadErr := protocol.Load(o.implDocPath); loadErr == nil {
		if r, ok := manifest.CompletionReports[protoAgent.ID]; ok {
			savedStatus = r.Status
			savedFailureType = r.FailureType
			status = r.Status
		}
	}
	reportMu.Unlock()

	o.publish(OrchestratorEvent{
		Event: "agent_complete",
		Data: AgentCompletePayload{
			Agent:  protoAgent.ID,
			Wave:   waveNum,
			Status: status,
			Branch: branch,
		},
	})

	// E19: Route partial/blocked reports.
	if savedStatus == "partial" || savedStatus == "blocked" {
		var failureType types.FailureType
		switch savedFailureType {
		case "transient":
			failureType = types.FailureTypeTransient
		case "fixable":
			failureType = types.FailureTypeFixable
		case "needs_replan":
			failureType = types.FailureTypeNeedsReplan
		case "escalate":
			failureType = types.FailureTypeEscalate
		default:
			failureType = types.FailureTypeEscalate
		}

		action := RouteFailure(failureType)
		o.publish(OrchestratorEvent{
			Event: "agent_blocked",
			Data: AgentBlockedPayload{
				Agent:       protoAgent.ID,
				Wave:        waveNum,
				Status:      savedStatus,
				FailureType: savedFailureType,
				Action:      action,
			},
		})
	}

	return nil
}
