// Package orchestrator drives SAW protocol wave execution: it advances the
// 10-state machine, creates per-agent git worktrees, launches agents
// concurrently, merges completed worktrees, runs post-merge verification,
// and updates the IMPL doc status table.
//
// # Constructing an Orchestrator
//
// New loads the IMPL doc and returns the orchestrator wrapped in a result:
//
//	res := orchestrator.New(ctx, "/path/to/repo", "/path/to/docs/IMPL/IMPL-feature.yaml")
//	if res.IsFatal() {
//	    log.Fatalf("orchestrator.New: %v", res.Errors[0])
//	}
//	orch := res.GetData()
//
// There is no Config struct passed to New. BackendConfig is a separate type
// used internally for per-agent backend construction.
//
// # Primary Methods
//
//   - RunWave(ctx, waveNum) — Create worktrees, launch all agents in the wave
//     concurrently, wait for completion.
//   - MergeWave(ctx, waveNum) — Merge agent branches back to main.
//   - RunVerification(ctx, testCommand) — Run build/test commands post-merge.
//   - UpdateIMPLStatus(ctx, waveNum) — Tick Status table checkboxes for completed agents.
//   - RunAgent(ctx, waveNum, agentLetter, promptPrefix) — Run a single agent (for reruns).
//   - TransitionTo(newState) — Advance the state machine; returns error on invalid transitions.
//   - SetDefaultModel(model) — Set the fallback model for all wave agents.
//   - SetWorktreePaths(paths) — Pre-supply worktree paths for multi-repo execution.
//   - SetLogger(logger) — Configure the slog.Logger used by orchestrator methods.
//
// # Wave Lifecycle
//
// Example flow for a single wave:
//
//	// Example:
//	// res := orchestrator.New(ctx, repoPath, implDocPath)
//	// orch := res.GetData()
//	// waveRes := orch.RunWave(ctx, 1)
//	// mergeRes := orch.MergeWave(ctx, 1)
//	// verifyRes := orch.RunVerification(ctx, "go test ./...")
//	// orch.UpdateIMPLStatus(ctx, 1)
//
// # State Machine
//
// The orchestrator enforces a 10-state machine. State mutations must always
// go through TransitionTo — never modify state directly. Valid states are
// defined in pkg/protocol (StateScoutPending, StateScoutRunning, etc.).
// TransitionTo returns a fatal result for any invalid transition.
//
// # Git Worktree Isolation
//
// Each wave agent receives its own git worktree, isolated from other agents.
// Worktrees are created under <repoPath>/.claude/worktrees/<slug>/wave<N>-agent-<ID>/.
// For multi-repo execution, call SetWorktreePaths to provide pre-created worktrees.
//
// # Concurrent Agent Launch
//
// RunWave launches all agents in a wave using errgroup. The first agent failure
// cancels the context for all sibling agents. Agent launch order may be
// reordered based on the critical-path dependency scheduler (E25).
//
// # Failure Type Routing (E19)
//
// When an agent reports partial or blocked status, the orchestrator routes
// the failure using RouteFailureWithReactions:
//
//   - transient → ActionRetry (automatic retry up to MaxTransientRetries=2)
//   - fixable → ActionApplyAndRelaunch (retry with fix guidance, up to MaxFixableRetries=2)
//   - timeout → ActionRetryWithScope (retry once with scope-reduction note)
//   - needs_replan → ActionReplan (surface to human; no auto-retry)
//   - escalate → ActionEscalate (surface to human; no auto-retry)
//
// # Event Publishing
//
// The orchestrator emits structured events via an EventPublisher function.
// Register a publisher via the eventPublisher field (set directly on the struct).
// Event types include:
//
//   - "wave_started", "wave_complete" — wave lifecycle
//   - "agent_started", "agent_complete", "agent_blocked", "agent_failed" — per-agent lifecycle
//   - "agent_output" — streaming output chunks from agents
//   - "agent_tool_call" — tool invocations and results from agents
//   - "agent_prioritized" — reports critical-path reordering of agent launch sequence
//   - "auto_retry_started", "auto_retry_exhausted" — E19 retry events
//
// Note: event names use no trailing 'd' for past tense on completion events
// ("agent_complete" not "agent_completed", "wave_complete" not "wave_completed").
//
// # Per-Agent Backend Routing
//
// Agents can specify a model via the model: field in their IMPL doc section.
// The orchestrator creates a per-agent backend for that agent only.
// NewBackendFromModel supports provider-prefixed model strings:
//
//	// Example:
//	// res := orchestrator.NewBackendFromModel("bedrock:claude-sonnet-4-6")
//	// res := orchestrator.NewBackendFromModel("openai:gpt-4o")
//	// res := orchestrator.NewBackendFromModel("claude-opus-4-6")  // uses auto/API backend
//
// See BackendConfig for supported Kind values: "api", "cli", "auto", "openai",
// "anthropic", "bedrock", "ollama", "lmstudio".
package orchestrator
