package engine

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/agent"
	"github.com/blackwell-systems/scout-and-wave-go/pkg/orchestrator"
	"github.com/blackwell-systems/scout-and-wave-go/pkg/protocol"
	"github.com/blackwell-systems/scout-and-wave-go/pkg/retry"
)

// ClosedLoopRetryOpts configures a pre-merge per-agent gate retry (R3).
// This differs from R1 auto-remediation which handles POST-merge build
// failures for the whole wave. R3 handles PRE-merge gate failures on
// individual agents still in their worktrees.
type ClosedLoopRetryOpts struct {
	IMPLPath     string      // path to IMPL manifest
	RepoPath     string      // absolute path to repo root
	WaveNum      int         // wave number
	AgentID      string      // which agent's gate failed
	GateType     string      // "build", "test", "lint"
	GateCommand  string      // exact command that failed
	GateOutput   string      // error output from the gate
	WorktreePath string      // agent's worktree path
	MaxRetries   int         // per-gate retry limit
	ChatModel    string      // model for retry agent
	OnEvent      func(Event) // event callback
}

// ClosedLoopRetryResult holds the outcome of a closed-loop gate retry run.
type ClosedLoopRetryResult struct {
	Fixed      bool   `json:"fixed"`
	Attempts   int    `json:"attempts"`
	AgentID    string `json:"agent_id"`
	GateOutput string `json:"final_gate_output"`
}

// closedLoopExecCommandFunc is a seam for tests: runs a shell command in a given directory.
var closedLoopExecCommandFunc = func(ctx context.Context, dir string, cmdStr string) (string, error) {
	parts := strings.Fields(cmdStr)
	if len(parts) == 0 {
		return "", fmt.Errorf("empty command")
	}
	cmd := exec.CommandContext(ctx, parts[0], parts[1:]...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// closedLoopRunAgentFunc is a seam for tests: creates and runs a fix agent.
var closedLoopRunAgentFunc = func(ctx context.Context, model string, prompt string, worktreePath string) error {
	b, err := orchestrator.NewBackendFromModel(model)
	if err != nil {
		return fmt.Errorf("closed_loop_gate: backend init: %w", err)
	}
	runner := agent.NewRunner(b)
	spec := &protocol.Agent{
		ID:   "fix",
		Task: prompt,
	}
	_, execErr := runner.ExecuteStreamingWithTools(ctx, spec, worktreePath, nil, nil)
	return execErr
}

// ClosedLoopGateRetry implements R3: pre-merge per-agent gate retry.
// When a retryable gate fails for an agent, it sends error context to
// a fix agent running in the agent's worktree, then re-runs the gate.
// Loops up to MaxRetries times.
func ClosedLoopGateRetry(ctx context.Context, opts ClosedLoopRetryOpts) (*ClosedLoopRetryResult, error) {
	if opts.AgentID == "" {
		return nil, fmt.Errorf("engine.ClosedLoopGateRetry: AgentID is required")
	}
	if opts.GateCommand == "" {
		return nil, fmt.Errorf("engine.ClosedLoopGateRetry: GateCommand is required")
	}
	if opts.WorktreePath == "" {
		return nil, fmt.Errorf("engine.ClosedLoopGateRetry: WorktreePath is required")
	}

	maxRetries := opts.MaxRetries
	if maxRetries <= 0 {
		maxRetries = 3
	}

	model := opts.ChatModel
	if model == "" {
		model = "claude-sonnet-4-5"
	}

	publish := func(event string, data interface{}) {
		if opts.OnEvent != nil {
			opts.OnEvent(Event{Event: event, Data: data})
		}
	}

	publish("closed_loop_started", map[string]interface{}{
		"agent_id":    opts.AgentID,
		"gate_type":   opts.GateType,
		"wave":        opts.WaveNum,
		"max_retries": maxRetries,
	})

	result := &ClosedLoopRetryResult{
		AgentID:    opts.AgentID,
		GateOutput: opts.GateOutput,
	}

	currentOutput := opts.GateOutput

	for attempt := 1; attempt <= maxRetries; attempt++ {
		result.Attempts = attempt

		publish("closed_loop_attempt", map[string]interface{}{
			"agent_id": opts.AgentID,
			"attempt":  attempt,
			"gate":     opts.GateType,
		})

		// a. Build fix prompt with gate output + error classification
		errClass := retry.ClassifyError(currentOutput)
		suggestions := retry.SuggestFixes(errClass)

		prompt := buildClosedLoopFixPrompt(opts, currentOutput, errClass, suggestions, attempt)

		// b. Launch a fix agent in the agent's worktree (not main repo)
		if runErr := closedLoopRunAgentFunc(ctx, model, prompt, opts.WorktreePath); runErr != nil {
			// Agent execution failed; update output for next iteration context
			currentOutput = fmt.Sprintf("Fix agent error on attempt %d: %v\n\nPrior gate output:\n%s", attempt, runErr, currentOutput)
			result.GateOutput = currentOutput
			continue
		}

		// c. Re-run the gate command in the worktree
		gateOut, gateErr := closedLoopExecCommandFunc(ctx, opts.WorktreePath, opts.GateCommand)
		currentOutput = gateOut
		result.GateOutput = gateOut

		// d. If gate passes, return Fixed=true
		if gateErr == nil {
			result.Fixed = true
			publish("closed_loop_fixed", map[string]interface{}{
				"agent_id": opts.AgentID,
				"attempt":  attempt,
				"gate":     opts.GateType,
			})
			return result, nil
		}

		// e. Still failing — loop
	}

	// Exhausted all retries
	publish("closed_loop_exhausted", map[string]interface{}{
		"agent_id": opts.AgentID,
		"attempts": maxRetries,
		"gate":     opts.GateType,
	})

	return result, nil
}

// buildClosedLoopFixPrompt constructs the fix prompt for the retry agent.
func buildClosedLoopFixPrompt(
	opts ClosedLoopRetryOpts,
	gateOutput string,
	errClass retry.ErrorClass,
	suggestions []string,
	attempt int,
) string {
	var sb strings.Builder

	fmt.Fprintf(&sb, "## Closed-Loop Gate Fix — Attempt %d/%d\n\n", attempt, opts.MaxRetries)
	fmt.Fprintf(&sb, "**Agent:** %s\n", opts.AgentID)
	fmt.Fprintf(&sb, "**Gate type:** %s\n", opts.GateType)
	fmt.Fprintf(&sb, "**Gate command:** `%s`\n", opts.GateCommand)
	fmt.Fprintf(&sb, "**Worktree:** %s\n\n", opts.WorktreePath)

	fmt.Fprintf(&sb, "### Error Classification: %s\n\n", string(errClass))

	sb.WriteString("### Gate Failure Output\n\n```\n")
	output := gateOutput
	if len(output) > 6000 {
		output = output[:3000] + "\n\n... (truncated) ...\n\n" + output[len(output)-3000:]
	}
	sb.WriteString(output)
	sb.WriteString("\n```\n\n")

	if len(suggestions) > 0 {
		sb.WriteString("### Suggested Fixes\n\n")
		for _, s := range suggestions {
			fmt.Fprintf(&sb, "- %s\n", s)
		}
		sb.WriteString("\n")
	}

	sb.WriteString("### Instructions\n\n")
	sb.WriteString("You are a fix agent operating in an agent worktree. ")
	sb.WriteString("The quality gate listed above failed. Your task:\n\n")
	sb.WriteString("1. Diagnose the root cause from the error output\n")
	sb.WriteString("2. Apply a minimal targeted fix in this worktree\n")
	sb.WriteString("3. Do NOT commit — the orchestrator will re-run the gate after you finish\n")
	sb.WriteString("4. Keep changes minimal — fix only what is broken\n\n")
	fmt.Fprintf(&sb, "After fixing, the orchestrator will re-run: `%s`\n", opts.GateCommand)

	return sb.String()
}
