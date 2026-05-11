package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"github.com/blackwell-systems/polywave-go/internal/git"
	"github.com/blackwell-systems/polywave-go/pkg/agent"
	"github.com/blackwell-systems/polywave-go/pkg/orchestrator"
	"github.com/blackwell-systems/polywave-go/pkg/protocol"
	"github.com/blackwell-systems/polywave-go/pkg/result"
	"github.com/blackwell-systems/polywave-go/pkg/tools"
)

// RunIntegrationAgentOpts configures an integration agent run (E26).
type RunIntegrationAgentOpts struct {
	IMPLPath string                      // absolute path to IMPL manifest
	RepoPath string                      // absolute path to the target repository
	WaveNum  int                         // wave number that was just completed
	Report   *protocol.IntegrationReport // gaps to fix
	Model    string                      // optional model override
	Logger   *slog.Logger                // optional: nil falls back to slog.Default()
}

// RunIntegrationAgent implements E26 (reactive integration gap wiring).
// For planned integration waves (type: integration), see E27 and the wave
// dispatcher in runner.go.
//
// It launches an LLM agent to wire integration gaps after wave agents
// complete (E26). It reads the IntegrationReport, reads completion reports,
// and modifies integration_connectors files to wire exports into callers.
// Works with ALL backends (Bedrock, API, CLI) via
// orchestrator.NewBackendFromModel.
//
// The integration agent runs in the MAIN repo directory (not a worktree)
// because it needs to see the merged result of all wave agents. It only
// writes to integration_connectors files.
//
// If Report.Valid is true (no gaps), returns immediately without launching
// an agent.
func RunIntegrationAgent(ctx context.Context, opts RunIntegrationAgentOpts, onEvent func(Event)) result.Result[IntegrationAgentData] {
	if err := validateIntegrationOpts(opts); err != nil {
		return result.NewFailure[IntegrationAgentData]([]result.PolywaveError{{
			Code:     result.CodeIntegrationInvalidOpts,
			Message:  err.Error(),
			Severity: "fatal",
			Cause:    err,
		}})
	}

	publish := func(event string, data interface{}) {
		if onEvent != nil {
			onEvent(Event{Event: event, Data: data})
		}
	}

	// If no gaps, nothing to do.
	if opts.Report.Valid {
		return result.NewSuccess(IntegrationAgentData{
			IMPLPath: opts.IMPLPath,
			WaveNum:  opts.WaveNum,
			GapCount: 0,
		})
	}

	publish("integration_agent_started", map[string]interface{}{
		"impl_path": opts.IMPLPath,
		"wave":      opts.WaveNum,
		"gap_count": len(opts.Report.Gaps),
	})

	// Load manifest for context (integration_connectors, completion reports).
	manifest, err := protocol.Load(context.TODO(), opts.IMPLPath)
	if err != nil {
		publish("integration_agent_failed", map[string]string{"error": err.Error()})
		return result.NewFailure[IntegrationAgentData]([]result.PolywaveError{{
			Code:     result.CodeIntegrationLoadFailed,
			Message:  fmt.Sprintf("engine.RunIntegrationAgent: load manifest: %v", err),
			Severity: "fatal",
			Cause:    err,
		}})
	}

	// E26-P1: Verify integration_reports exist and contain gaps for this wave.
	// The caller already checked opts.Report.Valid above. Here we verify the
	// manifest itself has integration_reports persisted (E25 must have run).
	waveKey := fmt.Sprintf("wave%d", opts.WaveNum)
	if manifest.IntegrationReports == nil || manifest.IntegrationReports[waveKey] == nil {
		// No persisted integration report — the heuristic scan (E25) may not have run.
		// This is recoverable: use the caller-supplied report and log a warning.
		loggerFrom(opts.Logger).Warn("engine.RunIntegrationAgent: no integration_report found",
			"wave_key", waveKey)
	}

	// E26-P2: integration_connectors must be defined (E26 precondition).
	// Without connector files, the integration agent cannot safely scope its edits.
	if len(manifest.IntegrationConnectors) == 0 {
		publish("integration_agent_failed", map[string]string{
			"error": "no integration_connectors defined in manifest (E26-P2)",
		})
		return result.NewFailure[IntegrationAgentData]([]result.PolywaveError{{
			Code:     result.CodeIntegrationNoConnectors,
			Message:  "engine.RunIntegrationAgent: no integration_connectors defined in manifest — add integration_connectors entries listing files the agent may edit (E26-P2)",
			Severity: "fatal",
		}})
	}

	// Build the integration agent prompt.
	prompt, err := buildIntegrationPrompt(opts, manifest)
	if err != nil {
		publish("integration_agent_failed", map[string]string{"error": err.Error()})
		return result.NewFailure[IntegrationAgentData]([]result.PolywaveError{{
			Code:     result.CodeIntegrationPromptFailed,
			Message:  fmt.Sprintf("engine.RunIntegrationAgent: build prompt: %v", err),
			Severity: "fatal",
			Cause:    err,
		}})
	}

	// Create backend via orchestrator.NewBackendFromModel (supports all providers).
	bRes := orchestrator.NewBackendFromModel(opts.Model)
	if bRes.IsFatal() {
		publish("integration_agent_failed", map[string]string{"error": bRes.Errors[0].Message})
		return result.NewFailure[IntegrationAgentData]([]result.PolywaveError{{
			Code:     result.CodeIntegrationBackendFailed,
			Message:  fmt.Sprintf("engine.RunIntegrationAgent: backend init: %s", bRes.Errors[0].Message),
			Severity: "fatal",
		}})
	}
	b := bRes.GetData()

	// Build constraints for integrator role (E26 I1 enforcement).
	// AllowedPathPrefixes is derived from integration_connectors so the agent
	// is restricted to connector files only. The constraint list is injected
	// into the system prompt because NewBackendFromModel does not accept a
	// constraints parameter directly.
	constraints := buildIntegratorConstraints(manifest, extractConnectors(manifest))
	prompt = injectAllowedPathsRestriction(prompt, constraints)

	// Create agent runner and execute with streaming.
	runner := agent.NewRunner(b)
	spec := &protocol.Agent{
		ID:   "integrator",
		Task: prompt,
	}

	onChunk := func(chunk string) {
		publish("integration_agent_output", map[string]string{"chunk": chunk})
	}

	if _, execErr := runner.ExecuteStreamingWithTools(ctx, spec, opts.RepoPath, onChunk, nil); execErr != nil {
		publish("integration_agent_failed", map[string]string{"error": execErr.Error()})
		return result.NewFailure[IntegrationAgentData]([]result.PolywaveError{{
			Code:     result.CodeIntegrationAgentFailed,
			Message:  fmt.Sprintf("engine.RunIntegrationAgent: agent execution failed: %v", execErr),
			Severity: "fatal",
			Cause:    execErr,
		}})
	}

	// Auto-commit changes (same pattern as autoCommitWorktree in orchestrator.go).
	if commitErr := autoCommitIntegration(opts.RepoPath, opts.WaveNum); commitErr != nil {
		// Non-fatal: the agent may not have made changes.
		publish("integration_agent_output", map[string]string{
			"chunk": fmt.Sprintf("integration auto-commit: %v", commitErr),
		})
	}

	publish("integration_agent_complete", map[string]interface{}{
		"impl_path": opts.IMPLPath,
		"wave":      opts.WaveNum,
	})

	return result.NewSuccess(IntegrationAgentData{
		IMPLPath: opts.IMPLPath,
		WaveNum:  opts.WaveNum,
		GapCount: len(opts.Report.Gaps),
	})
}

// validateIntegrationOpts checks that required fields are present.
func validateIntegrationOpts(opts RunIntegrationAgentOpts) error {
	if opts.IMPLPath == "" {
		return fmt.Errorf("engine.RunIntegrationAgent: IMPLPath is required")
	}
	if opts.RepoPath == "" {
		return fmt.Errorf("engine.RunIntegrationAgent: RepoPath is required")
	}
	if opts.WaveNum <= 0 {
		return fmt.Errorf("engine.RunIntegrationAgent: WaveNum must be positive")
	}
	if opts.Report == nil {
		return fmt.Errorf("engine.RunIntegrationAgent: Report is required")
	}
	return nil
}

// buildIntegrationPrompt constructs the self-contained prompt for the
// integration agent. It includes the system instruction, gaps as JSON,
// connector files, completion reports, and the verification gate command.
func buildIntegrationPrompt(opts RunIntegrationAgentOpts, manifest *protocol.IMPLManifest) (string, error) {
	// Marshal gaps to JSON for the agent.
	gapsJSON, err := json.MarshalIndent(opts.Report.Gaps, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal gaps: %w", err)
	}

	// Collect integration_connectors files.
	connectors := extractConnectors(manifest)
	connectorList := ""
	for _, c := range connectors {
		connectorList += fmt.Sprintf("- %s (reason: %s)\n", c.File, c.Reason)
	}
	if connectorList == "" {
		connectorList = "(none specified — use SearchResults in each gap for guidance)\n"
	}

	// Collect completion reports for context.
	completionJSON := "{}"
	if len(manifest.CompletionReports) > 0 {
		if b, err := json.MarshalIndent(manifest.CompletionReports, "", "  "); err == nil {
			completionJSON = string(b)
		}
	}

	prompt := fmt.Sprintf(`You are an Integration Agent (E26). Your job is to wire newly exported
functions/types into caller files.

## Integration Gaps to Fix

The following exports were created by wave %d agents but have no call-sites
in the codebase. Wire each one into the appropriate caller file.

%s

## Files You May Modify (integration_connectors)

%s
## Completion Reports (context on what was implemented)

%s

## Verification Gate

After making changes, verify the build passes:
  go build ./...

## Rules

1. Only modify files listed in integration_connectors (or files suggested in
   SearchResults if no connectors are specified).
2. Do NOT modify the files where exports are defined — only modify callers.
3. Add proper imports when wiring new calls.
4. Each gap's SuggestedFix provides guidance on how to wire it.
5. Run "go build ./..." after changes to verify compilation.
`, opts.WaveNum, string(gapsJSON), connectorList, completionJSON)

	return prompt, nil
}

// extractConnectors retrieves IntegrationConnector entries from the manifest.
// Returns nil if no connectors are defined.
func extractConnectors(manifest *protocol.IMPLManifest) []protocol.IntegrationConnector {
	return manifest.IntegrationConnectors
}

// autoCommitIntegration stages and commits integration agent changes.
// Similar to autoCommitWorktree in orchestrator.go but runs in the main repo.
func autoCommitIntegration(repoPath string, waveNum int) error {
	status, err := git.StatusPorcelain(repoPath)
	if err != nil {
		return fmt.Errorf("checking repo status: %w", err)
	}
	if status == "" {
		return nil // No changes to commit.
	}

	if err := git.AddAll(repoPath); err != nil {
		return fmt.Errorf("staging changes: %w", err)
	}

	msg := fmt.Sprintf("feat(wave%d-integration): wire integration gaps", waveNum)
	if _, err := git.Commit(repoPath, msg); err != nil {
		return fmt.Errorf("committing: %w", err)
	}

	return nil
}

// injectAllowedPathsRestriction prepends an explicit file-restriction notice
// to the prompt when constraints carry a non-empty AllowedPathPrefixes list.
// This enforces E26 I1 at the prompt level because NewBackendFromModel does
// not accept a constraints parameter for tool-level enforcement.
// Returns the original prompt unchanged when constraints is nil or has no
// allowed prefixes (backward compatible — no enforcement for unconstrained runs).
func injectAllowedPathsRestriction(prompt string, constraints *tools.Constraints) string {
	if constraints == nil || len(constraints.AllowedPathPrefixes) == 0 {
		return prompt
	}
	var sb strings.Builder
	sb.WriteString("## STRICT FILE RESTRICTION (E26 I1)\n\n")
	sb.WriteString("You are the Integration Agent. You MUST only write to the following files:\n\n")
	for _, p := range constraints.AllowedPathPrefixes {
		sb.WriteString(fmt.Sprintf("- %s\n", p))
	}
	sb.WriteString("\nDo NOT modify any other file. Do NOT modify agent-owned implementation files.\n\n---\n\n")
	return sb.String() + prompt
}
