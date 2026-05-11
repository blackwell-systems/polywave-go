// Package agent provides the runner that orchestrates agent execution in
// worktree contexts and utilities for parsing completion reports.
package agent

import (
	"context"
	"fmt"

	"github.com/blackwell-systems/polywave-go/pkg/agent/backend"
	"github.com/blackwell-systems/polywave-go/pkg/agent/dedup"
	"github.com/blackwell-systems/polywave-go/pkg/protocol"
)

// Runner orchestrates agent execution in worktree contexts.
type Runner struct {
	client backend.Backend
}

// NewRunner creates a Runner backed by the given Backend.
func NewRunner(b backend.Backend) *Runner {
	return &Runner{
		client: b,
	}
}

// Execute sends agent.Task as the system prompt to the backend, paired
// with a user message that provides the worktreePath for context. It returns
// the raw response text. Errors are returned immediately without retry.
func (r *Runner) Execute(ctx context.Context, agent *protocol.Agent, worktreePath string) (string, error) {
	systemPrompt := agent.Task

	userMessage := fmt.Sprintf(
		"You are operating in worktree: %s\n"+
			"Navigate there first (cd %s) before any file operations.\n\n"+
			"Your task is defined in Field 0 of your prompt above. Begin now.",
		worktreePath,
		worktreePath,
	)

	response, err := r.client.Run(ctx, systemPrompt, userMessage, worktreePath)
	if err != nil {
		return "", fmt.Errorf("runner: Execute agent %s: %w", agent.ID, err)
	}

	return response, nil
}

// ExecuteStreaming sends the agent prompt to the backend via RunStreaming.
// onChunk receives each text fragment as it arrives.
// Returns the full response text and any error, identical to Execute.
func (r *Runner) ExecuteStreaming(
	ctx context.Context,
	agent *protocol.Agent,
	worktreePath string,
	onChunk backend.ChunkCallback,
) (string, error) {
	systemPrompt := agent.Task

	userMessage := fmt.Sprintf(
		"You are operating in worktree: %s\n"+
			"Navigate there first (cd %s) before any file operations.\n\n"+
			"Your task is defined in Field 0 of your prompt above. Begin now.",
		worktreePath,
		worktreePath,
	)

	response, err := r.client.RunStreaming(ctx, systemPrompt, userMessage, worktreePath, onChunk)
	if err != nil {
		return "", fmt.Errorf("runner: ExecuteStreaming agent %s: %w", agent.ID, err)
	}

	return response, nil
}

// ExecuteStreamingWithTools sends the agent prompt to the backend via
// RunStreamingWithTools. onChunk and onToolCall may each be nil.
// Returns the full response and any error, identical to ExecuteStreaming.
func (r *Runner) ExecuteStreamingWithTools(
	ctx context.Context,
	agent *protocol.Agent,
	worktreePath string,
	onChunk backend.ChunkCallback,
	onToolCall backend.ToolCallCallback,
) (string, error) {
	systemPrompt := agent.Task

	userMessage := fmt.Sprintf(
		"You are operating in worktree: %s\n"+
			"Navigate there first (cd %s) before any file operations.\n\n"+
			"Your task is defined in your prompt above. You MUST:\n"+
			"1. Read the IMPL doc to understand your assigned files and task\n"+
			"2. Implement the code changes by writing/editing files in the worktree\n"+
			"3. Run tests to verify your changes compile and pass\n"+
			"4. Commit your changes with: git add -A && git commit -m 'feat(<agent>): <summary>'\n\n"+
			"You are NOT done until you have written files and committed. Reading and analyzing is not completion.\n"+
			"Begin now.",
		worktreePath,
		worktreePath,
	)

	response, err := r.client.RunStreamingWithTools(ctx, systemPrompt, userMessage, worktreePath, onChunk, onToolCall)
	if err != nil {
		return "", fmt.Errorf("runner: ExecuteStreamingWithTools agent %s: %w", agent.ID, err)
	}

	return response, nil
}

// DedupStats returns dedup metrics from the most recent agent execution.
// Returns nil if the backend does not implement DedupStats or no execution occurred.
func (r *Runner) DedupStats() *dedup.Stats {
	type dedupStatsProvider interface {
		DedupStats() *dedup.Stats
	}
	if provider, ok := r.client.(dedupStatsProvider); ok {
		return provider.DedupStats()
	}
	return nil
}
