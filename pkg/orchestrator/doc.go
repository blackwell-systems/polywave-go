// Package orchestrator manages wave lifecycle: launching agents, tracking completion,
// merging branches, and running verification.
//
// # Wave Orchestration
//
// The Orchestrator coordinates parallel agent execution with git worktree isolation:
//
//	orch := orchestrator.New(orchestrator.Config{
//	    RepoPath: "/path/to/repo",
//	    IMPLPath: "/path/to/docs/IMPL/IMPL-feature.md",
//	    BackendConfig: orchestrator.BackendConfig{
//	        Kind:      "api",
//	        APIKey:    os.Getenv("ANTHROPIC_API_KEY"),
//	        WaveModel: "claude-sonnet-4-6",
//	    },
//	})
//
//	err := orch.StartWave(ctx, 1)
//	if err != nil {
//	    log.Fatalf("Wave 1 failed: %v", err)
//	}
//
// # Wave Lifecycle
//
//  1. StartWave — Create worktrees, launch agents in parallel
//  2. [Agents execute — tool calls, streaming output]
//  3. RunPreMergeGates — Execute pre-merge quality gates
//  4. MergeWave — Merge agent branches to main
//  5. RunVerification — Build, tests, invariant checks
//
// # SSE Event Streaming
//
// The orchestrator publishes real-time events via SSEBroker:
//
//	eventChan := orch.SSEBroker.Subscribe("session-1")
//	go func() {
//	    for event := range eventChan {
//	        switch event.Type {
//	        case "agent_output":
//	            agentID := event.Data["agent_id"].(string)
//	            chunk := event.Data["chunk"].(string)
//	            fmt.Printf("[%s] %s", agentID, chunk)
//	        case "agent_completed":
//	            agentID := event.Data["agent_id"].(string)
//	            fmt.Printf("Agent %s completed\n", agentID)
//	        }
//	    }
//	}()
//
// Event types include: wave_started, agent_started, agent_output, agent_tool_call,
// agent_completed, agent_blocked, wave_completed, wave_merged, quality_gate_started,
// quality_gate_completed, verification_started, verification_gate, verification_completed.
//
// See docs/reference/sse-events.md for full event schema.
//
// # Per-Agent Backend Routing
//
// Agents can specify models via **model:** field in their IMPL doc section.
// The orchestrator creates per-agent backends:
//
//	// In IMPL doc:
//	// **model:** claude-opus-4-6
//
//	// Orchestrator creates Opus backend for that agent only
//
// # Failure Type Routing (E19)
//
// When an agent reports partial/blocked status, the orchestrator routes the failure:
//
//   - transient → ActionRetry (network timeout, API rate limit)
//   - fixable → ActionRelaunch (agent can fix the issue)
//   - needs_replan → ActionEscalateScout (architecture issue)
//   - escalate, timeout → ActionEscalateHuman
//
// See docs/reference/orchestration.md for detailed orchestration flow and examples/library-usage/ for usage examples.
package orchestrator
