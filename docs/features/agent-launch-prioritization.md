# Feature: Agent Launch Prioritization (v0.31.0)

## Overview

Add critical path-aware agent launch scheduling to reduce wave completion time. Currently agents launch in IMPL doc declaration order (static). This feature reorders launches based on dependency graph analysis so agents on the critical path start first, unblocking downstream work earlier.

## Problem Statement

In a wave with 5 agents where Agent A → B → C (linear dependency) and D, E are independent:
- **Current behavior:** Launch order is declaration order (A, B, C, D, E or however Scout wrote them)
- **Problem:** If A launches last, B and C block waiting for A to complete, wasting parallelism
- **Solution:** Launch A first (deepest critical path), then D/E (independent), then B, then C

Expected improvement: **10-20% wave completion time reduction** on waves with non-trivial dependency graphs.

## Success Criteria

1. Agents with deeper critical path depth launch before agents with shallow depth
2. Tie-breaker: agents with fewer files launch first (lower risk)
3. Launch order stored in manifest as `agent_launch_order: ["A", "D", "E", "B", "C"]` (audit trail)
4. SSE event `agent_prioritized` emitted showing reordering (observability)
5. Zero functional change if prioritization is disabled (declaration order preserved)
6. Works for both CLI (`sawtools run-wave`) and API paths (shared engine code)

## Technical Design

### 1. Critical Path Calculation

**Input:** IMPL manifest with `impl-dep-graph` typed block
**Output:** Map of `agentID -> criticalPathDepth`

Algorithm:
```
CriticalPathDepth(agent):
  if agent has no dependents:
    return 1
  else:
    return 1 + max(CriticalPathDepth(dependent) for each dependent)
```

Example:
```
A → B → C
    └→ D

Critical path depths:
- C: 1 (no dependents)
- D: 1 (no dependents)
- B: 2 (max(1, 1) + 1)
- A: 3 (max(2) + 1)

Launch order: A (3), B (2), C (1), D (1)
With tie-breaker by file count: A, B, [C or D based on file count], [D or C]
```

### 2. Integration Points

**File:** `pkg/engine/runner.go` (or new `pkg/engine/scheduler.go`)
**Function:** Add `PrioritizeAgents(manifest, waveNum) []string`

**Call site:** In `RunWave()` before agent launch loop:
```go
func (r *Runner) RunWave(ctx context.Context, manifest *IMPLManifest, waveNum int) error {
    // ... existing validation ...

    // NEW: Prioritize agent launch order
    agentOrder := scheduler.PrioritizeAgents(manifest, waveNum)
    manifest.Waves[waveNum-1].AgentLaunchOrder = agentOrder // store for audit

    // Emit SSE event (observability)
    r.events.Emit(AgentPrioritizedEvent{...})

    // Launch agents in prioritized order
    for _, agentID := range agentOrder {
        agent := manifest.GetAgent(waveNum, agentID)
        go r.launchAgent(ctx, agent, ...)
    }
}
```

### 3. Metadata Storage

**Schema change:** Add optional field to `Wave` struct:
```go
// pkg/protocol/manifest.go
type Wave struct {
    Number           int     `yaml:"number"`
    Agents           []Agent `yaml:"agents"`
    AgentLaunchOrder []string `yaml:"agent_launch_order,omitempty"` // NEW
}
```

After prioritization, write `agent_launch_order` back to manifest. This provides:
- Audit trail (know exactly which order agents launched)
- Debugging (compare prioritized vs declaration order)
- Reproducibility (re-run with same order if needed)

### 4. Observability (SSE Event)

**New event type:**
```go
// pkg/api/sse.go (or pkg/orchestrator/events.go)
type AgentPrioritizedEvent struct {
    Type              string   `json:"type"` // "agent_prioritized"
    Wave              int      `json:"wave"`
    OriginalOrder     []string `json:"original_order"`
    PrioritizedOrder  []string `json:"prioritized_order"`
    Reordered         bool     `json:"reordered"` // true if order changed
    Reason            string   `json:"reason"` // "critical_path_scheduling"
}
```

Web UI can display this in WaveBoard: "Agents launched in optimized order: A → D → E → B → C"

### 5. CLI Flag (Debug/Disable)

Add `--no-prioritize` flag to `sawtools run-wave`:
```go
cmd.Flags().BoolVar(&noPrioritize, "no-prioritize", false, "Disable agent launch prioritization (use declaration order)")
```

Use cases:
- Debugging: reproduce exact behavior from older runs
- Comparison: benchmark prioritized vs unprioritized completion times

### 6. Edge Cases

**Solo waves (1 agent):** Skip prioritization (no benefit, avoid metadata clutter)
```go
if len(wave.Agents) == 1 {
    return []string{wave.Agents[0].ID} // no-op
}
```

**Cycles in dep graph:** Already caught by E16 validator, but add defensive check:
```go
if hasCycle(manifest.DepGraph) {
    return declarationOrder // fall back to safe default
}
```

**No dependency info (all agents independent):** Use file count tie-breaker only:
```go
sort.Slice(agents, func(i, j int) bool {
    return len(agents[i].Files) < len(agents[j].Files)
})
```

## Files to Modify

**Core implementation:**
- `pkg/protocol/dep_graph.go` — add `CriticalPathDepth(agentID string) int`
- `pkg/engine/scheduler.go` — NEW: `PrioritizeAgents(manifest, waveNum) []string`
- `pkg/engine/runner.go` — call scheduler before agent launch
- `pkg/protocol/manifest.go` — add `AgentLaunchOrder []string` to `Wave` struct

**Observability:**
- `pkg/api/sse.go` — add `AgentPrioritizedEvent` type and emit call
- `pkg/orchestrator/events.go` — if SSE logic is here instead

**CLI:**
- `cmd/saw/run_wave.go` — add `--no-prioritize` flag

**Tests:**
- `pkg/engine/scheduler_test.go` — NEW: test prioritization algorithm
- `pkg/protocol/dep_graph_test.go` — test critical path calculation
- Integration test: full wave run with prioritization enabled/disabled

## Testing Strategy

**Unit tests:**
1. `TestCriticalPathDepth_LinearDependencies` — A → B → C should return depths 3, 2, 1
2. `TestCriticalPathDepth_ParallelAgents` — all independent agents should return depth 1
3. `TestCriticalPathDepth_DiamondGraph` — A → B,C → D should prioritize A, then B/C, then D
4. `TestPrioritizeAgents_FileCountTieBreaker` — equal depth agents sorted by file count

**Integration tests:**
1. Run wave with 5 agents, verify launch order matches expected priority
2. Run with `--no-prioritize`, verify declaration order used
3. Verify `agent_launch_order` written to manifest after execution

**Performance test:**
1. Benchmark wave completion time with/without prioritization on a 10-agent wave
2. Target: 10-20% reduction in total wave time

## Dependencies

**Existing code to leverage:**
- `pkg/protocol/validator.go` — already parses `impl-dep-graph` typed block
- `pkg/protocol/manifest.go` — file ownership table parsing
- `pkg/engine/runner.go` — agent launch loop

**No external dependencies needed** — pure standard library + existing SAW code.

## Non-Goals

- **Not changing parallelism level:** Still launch all agents concurrently, just in better order
- **Not rescheduling mid-wave:** Priority is static (calculated once at wave start)
- **Not machine learning:** Simple graph algorithm, no heuristics or learning

## Documentation Updates

- `docs/ROADMAP.md` — mark v0.31.0 as complete
- `docs/architecture.md` — add section on agent launch scheduling
- `CHANGELOG.md` — v0.31.0 entry with before/after examples

## References

- Engine roadmap: `/Users/dayna.blackwell/code/scout-and-wave-go/docs/ROADMAP.md` (v0.31.0 section)
- E16 validator: `pkg/protocol/validator.go` (dep graph parsing)
- Wave runner: `pkg/engine/runner.go` (agent launch loop)
- Manifest schema: `pkg/protocol/manifest.go` (Wave/Agent structs)
