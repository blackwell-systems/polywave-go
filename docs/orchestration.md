# Orchestration

The orchestrator (`pkg/orchestrator`) manages wave lifecycle: launching agents, tracking completion, merging branches, and running verification.

## Wave Lifecycle

```
StartWave
  ↓
Create worktrees + branches for each agent
  ↓
Launch agents in parallel (goroutines)
  ↓
[Agents execute — tool calls, streaming output]
  ↓
Wait for all agents to complete
  ↓
Run quality gates
  ↓
MergeWave
  ↓
RunVerification (build, tests, invariants)
  ↓
Wave complete
```

## Orchestrator Initialization

```go
import "github.com/blackwell-systems/scout-and-wave-go/pkg/orchestrator"

orch := orchestrator.New(orchestrator.Config{
    RepoPath:      "/path/to/repo",
    IMPLPath:      "/path/to/docs/IMPL/IMPL-feature.md",
    BackendConfig: orchestrator.BackendConfig{
        Kind:       "api",
        APIKey:     os.Getenv("ANTHROPIC_API_KEY"),
        ScoutModel: "claude-sonnet-4-6",
        WaveModel:  "claude-sonnet-4-6",
    },
})
```

## StartWave

Launches all agents for a given wave in parallel.

```go
err := orch.StartWave(ctx, waveNumber)
```

**Steps:**

1. Parse IMPL doc to get wave structure
2. For each agent:
   - Create git worktree: `.claude/worktrees/wave1-agent-A`
   - Create branch: `wave1-agent-A`
   - Extract per-agent context (E23)
   - Launch agent in goroutine via `launchAgent()`
3. Wait for all agents to complete
4. Run quality gates
5. Publish `wave_completed` SSE event

## Agent Launch

```go
func (o *Orchestrator) launchAgent(ctx context.Context, wave *types.Wave, agent *types.Agent) error {
    // Create worktree
    worktreePath, err := o.worktreeManager.Create(wave.Number, agent.Letter)
    if err != nil {
        return fmt.Errorf("worktree creation failed: %w", err)
    }

    // Extract per-agent context
    agentContext, err := protocol.ExtractAgentContext(o.implDoc, wave.Number, agent.Letter)
    if err != nil {
        return fmt.Errorf("context extraction failed: %w", err)
    }

    // Create backend for this agent
    backend, err := o.newBackendForAgent(agent)
    if err != nil {
        return fmt.Errorf("backend creation failed: %w", err)
    }

    // Build agent prompt
    prompt := protocol.FormatAgentContextPayload(agentContext)

    // Execute agent with streaming
    runner := agentrunner.New(backend, worktreePath)
    result, err := runner.ExecuteStreaming(ctx, prompt, func(chunk string) {
        o.ssebroker.Publish(orchestrator.Event{
            Type: "agent_output",
            Data: map[string]interface{}{
                "wave_number": wave.Number,
                "agent_id":    agent.Letter,
                "chunk":       chunk,
            },
        })
    })

    if err != nil {
        return o.handleAgentFailure(wave, agent, err)
    }

    // Parse completion report
    report, err := protocol.ParseCompletionReport(result)
    if err != nil {
        return fmt.Errorf("completion report parsing failed: %w", err)
    }

    // Route failure if needed (E19)
    if report.Status != "complete" {
        action := failure.RouteFailure(report.FailureType)
        o.ssebroker.Publish(orchestrator.Event{
            Type: "agent_blocked",
            Data: map[string]interface{}{
                "agent_id":     agent.Letter,
                "failure_type": report.FailureType,
                "action":       action,
            },
        })
    }

    return nil
}
```

## Per-Agent Backend Routing

Agents can specify different models via `**model:**` in their prompt section:

```go
func (o *Orchestrator) newBackendForAgent(agent *types.Agent) (backend.Backend, error) {
    model := agent.Model
    if model == "" {
        model = o.cfg.BackendConfig.WaveModel
    }

    return o.newBackendFunc(orchestrator.BackendConfig{
        Model:     model,
        APIKey:    o.cfg.BackendConfig.APIKey,
        MaxTokens: o.cfg.BackendConfig.MaxTokens,
    })
}
```

This allows Scout to assign different agents to different models (e.g., Opus for complex logic, Haiku for simple data transforms).

## MergeWave

Merges all agent branches to main after wave completes.

```go
err := orch.MergeWave(ctx, waveNumber)
```

**Steps:**

1. Parse IMPL doc to get agent list
2. For each agent:
   - Checkout main
   - Merge agent branch: `git merge --no-ff wave1-agent-A`
   - Handle conflicts (Tier 1: retry; Tier 2: resolver agent — TODO)
   - Delete agent branch
3. Remove worktrees
4. Publish `wave_merged` event

**Conflict Detection:**

```go
func (o *Orchestrator) mergeAgentBranch(agentBranch string) error {
    err := o.git.Merge(agentBranch)
    if err != nil {
        if isConflictError(err) {
            // Tier 1: retry after brief delay
            time.Sleep(2 * time.Second)
            err = o.git.Merge(agentBranch)
            if err == nil {
                return nil
            }

            // Tier 2: spawn resolver agent (TODO — see ROADMAP.md)
            return fmt.Errorf("merge conflict: %w", err)
        }
        return fmt.Errorf("merge failed: %w", err)
    }
    return nil
}
```

## Quality Gates

After wave agents complete (before merge), the orchestrator runs quality gates from the IMPL doc `## Quality Gates` section.

```go
func (o *Orchestrator) RunQualityGates(ctx context.Context, gates []types.QualityGate) error {
    for _, gate := range gates {
        o.ssebroker.Publish(orchestrator.Event{
            Type: "quality_gate_started",
            Data: map[string]interface{}{
                "gate_name": gate.Name,
                "command":   gate.Command,
            },
        })

        // 5-minute timeout per gate
        gateCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
        defer cancel()

        cmd := exec.CommandContext(gateCtx, "bash", "-c", gate.Command)
        cmd.Dir = o.cfg.RepoPath
        output, err := cmd.CombinedOutput()

        status := "passed"
        if err != nil {
            status = "failed"
            if gate.Required {
                return fmt.Errorf("required gate %s failed: %w", gate.Name, err)
            }
        }

        o.ssebroker.Publish(orchestrator.Event{
            Type: "quality_gate_completed",
            Data: map[string]interface{}{
                "gate_name": gate.Name,
                "status":    status,
                "output":    string(output),
            },
        })
    }
    return nil
}
```

## Verification Pipeline

After wave merges, the orchestrator runs verification gates:

```go
func (o *Orchestrator) RunVerification(ctx context.Context) error {
    // Build gate
    if err := o.runBuildGate(ctx); err != nil {
        return fmt.Errorf("build failed: %w", err)
    }

    // Test gate
    if err := o.runTestGate(ctx); err != nil {
        return fmt.Errorf("tests failed: %w", err)
    }

    // Invariant gate
    violations := protocol.ValidateInvariants(o.implDoc)
    if len(violations) > 0 {
        return fmt.Errorf("invariant violations: %v", violations)
    }

    return nil
}
```

**Build Gate:**

```go
func (o *Orchestrator) runBuildGate(ctx context.Context) error {
    cmd := exec.CommandContext(ctx, "go", "build", "./...")
    cmd.Dir = o.cfg.RepoPath
    output, err := cmd.CombinedOutput()
    if err != nil {
        return fmt.Errorf("build failed: %s", output)
    }
    return nil
}
```

**Test Gate:**

```go
func (o *Orchestrator) runTestGate(ctx context.Context) error {
    cmd := exec.CommandContext(ctx, "go", "test", "./...")
    cmd.Dir = o.cfg.RepoPath
    output, err := cmd.CombinedOutput()
    if err != nil {
        return fmt.Errorf("tests failed: %s", output)
    }
    return nil
}
```

## SSE Event Publishing

The orchestrator publishes events via `SSEBroker`:

```go
type SSEBroker struct {
    mu          sync.RWMutex
    subscribers map[string]chan Event
}

func (b *SSEBroker) Subscribe(sessionID string) <-chan Event {
    b.mu.Lock()
    defer b.mu.Unlock()

    ch := make(chan Event, 100)
    b.subscribers[sessionID] = ch
    return ch
}

func (b *SSEBroker) Publish(event Event) {
    b.mu.RLock()
    defer b.mu.RUnlock()

    for _, ch := range b.subscribers {
        select {
        case ch <- event:
        default:
            // Channel full — drop event (fire-and-forget)
        }
    }
}
```

See `docs/sse-events.md` for full event schema.

## Failure Type Routing (E19)

When an agent reports `partial` or `blocked` status, the orchestrator routes the failure:

```go
func RouteFailure(failureType string) OrchestratorAction {
    switch failureType {
    case "transient":
        // Network timeout, API rate limit
        return ActionRetry
    case "fixable":
        // Agent can fix the issue (missing dep, syntax error)
        return ActionRelaunch
    case "needs_replan":
        // Architecture issue — Scout must replan
        return ActionEscalateScout
    case "escalate", "timeout":
        // Human intervention required
        return ActionEscalateHuman
    default:
        return ActionEscalateHuman
    }
}
```

**Actions:**
- `ActionRetry` — Re-run the same agent immediately
- `ActionRelaunch` — Launch a new agent to fix the issue
- `ActionEscalateScout` — Re-run Scout to replan the feature
- `ActionEscalateHuman` — Notify user, block wave

> **Note:** Auto-remediation (actually executing the action) is not yet implemented. The orchestrator computes and publishes the action as an SSE event but does not take automatic retry/relaunch action (see `pkg/orchestrator/failure.go` comments).

## Cross-Repo Orchestration

For cross-repo waves, the orchestrator creates worktrees in the target repo while the IMPL doc lives in the spec repo:

```go
func (o *Orchestrator) launchCrossRepoAgent(ctx context.Context, agent *types.Agent) error {
    targetRepoPath := agent.Repo // From file ownership table "Repo" column

    // Create worktree in target repo
    worktreePath := filepath.Join(targetRepoPath, ".claude/worktrees", fmt.Sprintf("wave%d-agent-%s", wave.Number, agent.Letter))
    // ... worktree creation

    // Agent prompt Field 8 (completion report path) must use absolute path to IMPL doc
    implDocAbsPath, _ := filepath.Abs(o.cfg.IMPLPath)
    prompt := buildAgentPrompt(agent, implDocAbsPath)

    // Execute agent in target repo worktree
    runner := agentrunner.New(backend, worktreePath)
    result, err := runner.ExecuteStreaming(ctx, prompt, onChunk)
    // ...
}
```

See `docs/protocol-parsing.md` for cross-repo file ownership table format.

## Context Memory Updates (E18)

After final wave verification passes, the orchestrator appends to `docs/CONTEXT.md`:

```go
func (o *Orchestrator) UpdateContextMD(ctx context.Context) error {
    contextPath := filepath.Join(o.cfg.RepoPath, "docs/CONTEXT.md")

    // Extract key contracts/types from IMPL doc
    newContent := o.extractContextSummary(o.implDoc)

    // Append to CONTEXT.md
    f, err := os.OpenFile(contextPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
    if err != nil {
        return err
    }
    defer f.Close()

    _, err = f.WriteString("\n\n" + newContent)
    if err != nil {
        return err
    }

    // Commit CONTEXT.md update
    return o.git.Commit("docs: update CONTEXT.md with new contracts from " + o.implDoc.Title)
}
```

This ensures future Scout runs avoid proposing types/interfaces that already exist.

## See Also

- [Architecture Overview](architecture.md) — Orchestrator role in the engine
- [SSE Events](sse-events.md) — Event types and lifecycle
- [Protocol Parsing](protocol-parsing.md) — IMPL doc structure
- [Backends](backends.md) — Per-agent backend routing
