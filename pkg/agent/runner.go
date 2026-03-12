// Package agent provides the runner that orchestrates agent execution in
// worktree contexts and utilities for parsing completion reports.
package agent

import (
	"context"
	"fmt"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/agent/backend"
	"github.com/blackwell-systems/scout-and-wave-go/pkg/types"
	"github.com/blackwell-systems/scout-and-wave-go/pkg/worktree"
)

// Runner orchestrates agent execution in worktree contexts.
type Runner struct {
	client    backend.Backend
	worktrees *worktree.Manager
}

// NewRunner creates a Runner backed by the given Backend and worktree Manager.
func NewRunner(b backend.Backend, worktrees *worktree.Manager) *Runner {
	return &Runner{
		client:    b,
		worktrees: worktrees,
	}
}

// Execute sends agentSpec.Prompt as the system prompt to the backend, paired
// with a user message that provides the worktreePath for context. It returns
// the raw response text. Errors are returned immediately without retry.
func (r *Runner) Execute(ctx context.Context, agentSpec *types.AgentSpec, worktreePath string) (string, error) {
	systemPrompt := agentSpec.Prompt

	userMessage := fmt.Sprintf(
		"You are operating in worktree: %s\n"+
			"Navigate there first (cd %s) before any file operations.\n\n"+
			"Your task is defined in Field 0 of your prompt above. Begin now.",
		worktreePath,
		worktreePath,
	)

	response, err := r.client.Run(ctx, systemPrompt, userMessage, worktreePath)
	if err != nil {
		return "", fmt.Errorf("runner: Execute agent %s: %w", agentSpec.Letter, err)
	}

	return response, nil
}

// ExecuteStreaming sends the agent prompt to the backend via RunStreaming.
// onChunk receives each text fragment as it arrives.
// Returns the full response text and any error, identical to Execute.
func (r *Runner) ExecuteStreaming(
	ctx context.Context,
	agentSpec *types.AgentSpec,
	worktreePath string,
	onChunk backend.ChunkCallback,
) (string, error) {
	systemPrompt := agentSpec.Prompt

	userMessage := fmt.Sprintf(
		"You are operating in worktree: %s\n"+
			"Navigate there first (cd %s) before any file operations.\n\n"+
			"Your task is defined in Field 0 of your prompt above. Begin now.",
		worktreePath,
		worktreePath,
	)

	response, err := r.client.RunStreaming(ctx, systemPrompt, userMessage, worktreePath, onChunk)
	if err != nil {
		return "", fmt.Errorf("runner: ExecuteStreaming agent %s: %w", agentSpec.Letter, err)
	}

	return response, nil
}

// ExecuteStreamingWithTools sends the agent prompt to the backend via
// RunStreamingWithTools. onChunk and onToolCall may each be nil.
// Returns the full response and any error, identical to ExecuteStreaming.
func (r *Runner) ExecuteStreamingWithTools(
	ctx context.Context,
	agentSpec *types.AgentSpec,
	worktreePath string,
	onChunk backend.ChunkCallback,
	onToolCall backend.ToolCallCallback,
) (string, error) {
	systemPrompt := agentSpec.Prompt

	userMessage := fmt.Sprintf(
		"You are operating in worktree: %s\n"+
			"Navigate there first (cd %s) before any file operations.\n\n"+
			"Your task is defined in Field 0 of your prompt above. Begin now.",
		worktreePath,
		worktreePath,
	)

	response, err := r.client.RunStreamingWithTools(ctx, systemPrompt, userMessage, worktreePath, onChunk, onToolCall)
	if err != nil {
		return "", fmt.Errorf("runner: ExecuteStreamingWithTools agent %s: %w", agentSpec.Letter, err)
	}

	return response, nil
}
