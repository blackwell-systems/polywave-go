# Proposal: Deterministic Integration for Scout-and-Wave Protocol

**Status:** Seed Design (Ready for Scout IMPL Planning)
**Date:** 2026-03-15

## 1. Problem Statement

When SAW protocol executes waves of parallel agents with disjoint file ownership (I1 invariant), integration points that span file boundaries fall through the cracks.

### Concrete Examples

- **Workshop-constraints (2026-03-15):** Agent F created `BuildWaveConstraints()` in `pkg/engine/constraints.go` but didn't wire it into `pkg/orchestrator/orchestrator.go` (not owned). Agent E added `Constraints *tools.Constraints` to `backend.Config` but didn't initialize it in callers. Human orchestrator had to manually write the integration code after all waves completed.

- **Pattern repeats:** Any time an agent creates an exported function, type, or struct field that must be called/used from a file outside its ownership — the wiring never happens automatically.

### Why I1 Causes This

I1 (disjoint file ownership) perfectly prevents merge conflicts but creates a blind spot: caller files (orchestrator.go, main.go, server.go) are typically owned by nobody because they're high-traffic files that multiple agents would conflict on. Exported functions created by agents sit unused because nobody has authority to modify the callers.

### Impact

- **CLI path:** LLM orchestrator sometimes catches gaps but not reliably
- **SDK/Web/Bedrock path:** Fully automated, no human to catch gaps — deployment with broken integration
- **Manual post-wave effort:** Humans must write wiring code after all agents finish

## 2. Solution Options

### Option A: Integration Agent

Each wave includes one special agent ("Integration Agent") whose sole job is wiring new exports into caller files. Runs after all implementation agents complete. Has relaxed I1 for specific "integration connector" files.

**Pros:**
- Clear ownership: integration responsibility is explicit
- Automated in both CLI and SDK paths
- Full LLM reasoning to find and wire integration points

**Cons:**
- Adds serial agent per wave (~5-10 min overhead)
- Requires relaxing I1 for integration connector files
- Integration agent must run after all others (dependency chain)
- Hard to parameterize: which files are "integration connectors"?

### Option B: Post-Wave Integration Detection (Static Analysis)

After all agents complete and before merge, run automated static analysis:
1. Scan new exports in agent-owned files
2. Search for call-sites in the codebase
3. If export exists but no call-site found, emit integration gap alert

**Pros:**
- No protocol modification (I1 stays pure)
- Deterministic: tool finds all gaps, no reasoning required
- Runs after implementation — analyzes actual code

**Cons:**
- High false positive rate (exports may be intentionally unused)
- Can't distinguish "integration missing" vs "integration optional"
- Requires language-specific parsing (Go AST, etc.)
- Detection only — doesn't fix the gap

### Option C: Scout-Level Integration Ownership

Scout assigns integration call-sites to agents during planning. Agent owns both its implementation files AND specific integration points in caller files.

**Pros:**
- Integration responsibility planned upfront
- Agents know they must wire their own exports
- No post-wave overhead

**Cons:**
- Scout must predict exact call-sites (hard without implementation)
- Significantly modifies I1 (agents own non-contiguous regions)
- IMPL doc becomes more complex

### Option D: Hybrid — Post-Wave Validator + Scout Guidance

Combine B (post-wave detection) and C (Scout guidance). Scout flags which exports are "integration-required" vs "integration-optional". Post-wave tool detects gaps guided by Scout flags.

**Pros:**
- Planning + automated validation combined
- Scout guidance prevents false positives
- Post-wave tool is deterministic
- Minimal protocol changes (advisory flags, not full I1 relaxation)
- Works in both CLI and SDK paths
- Backward compatible

**Cons:**
- Two-phase detection adds complexity
- Still requires language-specific parsing

## 3. Recommendation

**Pursue Option D (Hybrid)** as the primary solution.

### Rationale
1. **Determinism:** Works in both CLI and SDK paths
2. **Minimal protocol impact:** Scout guidance is advisory, I1 stays pure
3. **Practical:** Scout plans integration responsibility, post-wave tool validates
4. **Backward compatible:** Existing IMPL docs without integration guidance work unchanged
5. **Phased rollout:** Start with post-wave detection, add Scout guidance later

### Immediate Next Steps
1. Implement post-wave integration validator tool
2. Extend `finalize.go` to call validator (new step between E21 gates and MergeAgents)
3. Add Scout guidance flags to IMPL schema (optional fields)
4. Update Scout prompt to identify integration-required exports

## 4. Implementation Sketch

### Core Types

```go
// pkg/protocol/integration.go

// IntegrationGap represents a detected unconnected export
type IntegrationGap struct {
    ExportName    string   `json:"export_name"`
    FilePath      string   `json:"file_path"`
    Category      string   `json:"category"`      // "function_call", "type_usage", "field_init"
    Severity      string   `json:"severity"`       // "error", "warning"
    Reason        string   `json:"reason"`
    SuggestedFix  string   `json:"suggested_fix"`
    SearchResults []string `json:"search_results"` // files that might need to call it
}

// IntegrationReport summarizes all gaps for a wave
type IntegrationReport struct {
    Wave  int               `json:"wave"`
    Gaps  []IntegrationGap  `json:"gaps"`
    Valid bool              `json:"valid"`
}

// ValidateIntegration analyzes the wave for unconnected exports
func ValidateIntegration(manifest *IMPLManifest, waveNum int, repoPath string) (*IntegrationReport, error)
```

### Algorithm

1. Parse completion reports for this wave -> list of modified files + agent ownership
2. For each modified file owned by an agent:
   - Parse for new/exported symbols (language-specific, Go AST for Go)
   - Check if symbol is integration-required (from interface_contracts or heuristic)
   - Search codebase for usage of symbol
   - If no usage found and integration-required: add to Gaps
3. Return report with gaps + severity

### Integration Heuristics (when `integration_required` not specified)

- Function name starts with `Build`, `Create`, `New` -> likely integration-required
- Function name ends with `Init`, `Setup`, `Register` -> likely integration-required
- Struct field added to type used in non-owned files -> integration-required
- Exported but only called within same package -> integration-optional

### IMPL Schema Extension

```yaml
interface_contracts:
  - name: BuildWaveConstraints
    location: pkg/engine/constraints.go
    integration_required: true
    integration_category: function_call
    suggested_callers:
      - pkg/orchestrator/orchestrator.go
```

### Finalization Pipeline Integration

Current: verify-commits (I5) -> scan-stubs (E20) -> run-gates (E21) -> merge -> verify-build -> cleanup

Proposed: verify-commits (I5) -> scan-stubs (E20) -> run-gates (E21) -> **validate-integration (NEW)** -> merge -> verify-build -> cleanup

## 5. CLI vs SDK Considerations

### CLI Path (`/saw wave`)

- After post-wave validation: if gaps exist, prompt user
- LLM orchestrator reads gap report and decides: retry agents, manual fix, or skip
- Human-in-the-loop can resolve ambiguous gaps

### SDK/Web Path (`engine.RunWave`)

- After post-wave validation: if critical gaps exist, return error immediately
- Error includes: gap descriptions, suggested fixes, failed wave number
- No human in the loop -> must fail fast with clear diagnostics
- Web UI can display integration gaps in the stage timeline

### Bedrock Path (fully automated)

- Same as SDK: fail with structured error
- Integration gaps become part of the completion report
- Retry logic: re-launch affected agents with explicit integration task in prompt

## 6. Success Criteria

- [ ] Post-wave validator detects known integration gaps (BuildWaveConstraints example)
- [ ] CLI path prompts human for decision on gaps
- [ ] SDK path fails with clear error message
- [ ] Scout prompt updated to flag integration-required exports
- [ ] Backward compatible: existing IMPL docs without guidance work unchanged
- [ ] Integrated into `sawtools finalize-wave` pipeline
- [ ] New CLI command: `sawtools validate-integration <impl-path>`
