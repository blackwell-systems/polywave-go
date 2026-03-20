# Agent B Brief - Wave 2

**IMPL Doc:** /Users/dayna.blackwell/code/scout-and-wave-web/docs/IMPL/IMPL-gate-timing-fix.yaml

## Files Owned

- `cmd/saw/finalize_wave.go`


## Task

## Context

You are wiring the gate timing fix into `finalize-wave` in the scout-and-wave-go
repo at `/Users/dayna.blackwell/code/scout-and-wave-go`.

Wave 1 (Agent A) has already added two new functions to `pkg/protocol/gates.go`:
- `RunPreMergeGates(manifest, waveNumber, repoDir, cache) ([]GateResult, error)`
- `RunPostMergeGates(manifest, waveNumber, repoDir) ([]GateResult, error)`

These replace the single `RunGatesWithCache` call that previously ran all gates
at step 3 (pre-merge). You must split the gate execution across two steps.

## File to modify

- `cmd/saw/finalize_wave.go`

## Task: Update finalize_wave.go gate execution

### Step 3 change (pre-merge gates, line ~110-129)

Replace the existing Step 3 block:
```go
// Step 3: RunGates (E21 quality gates) with caching - run per repo
for repoKey, repoPath := range repos {
    stateDir := filepath.Join(repoPath, ".saw-state")
    cache := gatecache.New(stateDir, gatecache.DefaultTTL)
    gateResults, err := protocol.RunGatesWithCache(manifest, waveNum, repoPath, cache)
    ...
}
```

With:
```go
// Step 3: RunPreMergeGates (E21) — structural checks that don't require merged state
for repoKey, repoPath := range repos {
    stateDir := filepath.Join(repoPath, ".saw-state")
    cache := gatecache.New(stateDir, gatecache.DefaultTTL)
    gateResults, err := protocol.RunPreMergeGates(manifest, waveNum, repoPath, cache)
    if err != nil {
        return fmt.Errorf("finalize-wave: run-pre-merge-gates failed in %s: %w", repoKey, err)
    }
    result.GateResults[repoKey] = gateResults

    for _, gate := range gateResults {
        if gate.Required && !gate.Passed {
            out, _ := json.MarshalIndent(result, "", "  ")
            fmt.Println(string(out))
            return fmt.Errorf("finalize-wave: required pre-merge gate '%s' failed in %s", gate.Type, repoKey)
        }
    }
}
```

### Step 5 change (post-merge gates, after VerifyBuild succeeds)

Currently Step 5 ends with an early return on failure. After the VerifyBuild
success check (after the `if !verifyBuildResult.TestPassed || !verifyBuildResult.LintPassed`
block), add a new Step 5.5 block before Step 6 (Cleanup):

```go
// Step 5.5: RunPostMergeGates (E21) — content/integration checks on merged state
for repoKey, repoPath := range repos {
    postGateResults, err := protocol.RunPostMergeGates(manifest, waveNum, repoPath)
    if err != nil {
        return fmt.Errorf("finalize-wave: run-post-merge-gates failed in %s: %w", repoKey, err)
    }
    // Merge post-merge results into GateResults for this repo
    result.GateResults[repoKey] = append(result.GateResults[repoKey], postGateResults...)

    for _, gate := range postGateResults {
        if gate.Required && !gate.Passed {
            out, _ := json.MarshalIndent(result, "", "  ")
            fmt.Println(string(out))
            return fmt.Errorf("finalize-wave: required post-merge gate '%s' failed in %s", gate.Type, repoKey)
        }
    }
}
```

### Long command string update

Update the `Long` field of the cobra.Command to reflect the new ordering:

```
Execution order:
1. VerifyCommits - check all agents have commits (I5 trip wire)
2. ScanStubs - scan changed files for TODO/FIXME markers (E20)
3. RunPreMergeGates - structural gates that don't require merged state (E21)
3.5. ValidateIntegration - detect unconnected exports (E25, informational)
4. MergeAgents - merge all agent branches to main
5. VerifyBuild - run test_command and lint_command
5.5. RunPostMergeGates - content/integration gates on merged state (E21)
6. Cleanup - remove worktrees and branches
```

## Verification gate

```
go build ./...
go vet ./...
go test ./cmd/saw/...
go test ./...
```

## Constraints

- `result.GateResults` accumulates ALL gate results (pre + post) per repo.
  Post-merge results are appended, not overwritten, so the full gate picture
  is always visible in the JSON output.
- Do NOT remove the `gatecache` import — it is still used for pre-merge gates.
- Post-merge gates do NOT use the cache (RunPostMergeGates takes no cache param)
  because post-merge state is always new and should never be served from cache.
- If `RunPostMergeGates` returns an empty slice (no post-merge gates defined),
  the append is a no-op and the loop body's failure check never triggers.
  This is correct and requires no special handling.



## Interface Contracts

### QualityGate.Timing

New field added to QualityGate struct. Controls whether a gate runs before
or after agent branches are merged. Empty string means pre-merge (backward compat).


```
// In pkg/protocol/types.go, QualityGate struct:
type QualityGate struct {
    Type        string `yaml:"type" json:"type"`
    Command     string `yaml:"command" json:"command"`
    Required    bool   `yaml:"required" json:"required"`
    Description string `yaml:"description,omitempty" json:"description,omitempty"`
    Repo        string `yaml:"repo,omitempty" json:"repo,omitempty"`
    Fix         bool   `yaml:"fix,omitempty" json:"fix,omitempty"`
    // Timing controls when the gate executes in finalize-wave.
    // "pre-merge"  — run at step 3, before MergeAgents (default when omitted)
    // "post-merge" — run at step 5, after MergeAgents and alongside VerifyBuild
    // Empty string is equivalent to "pre-merge" for backward compatibility.
    Timing string `yaml:"timing,omitempty" json:"timing,omitempty"`
}

```

### RunPreMergeGates

Executes only gates with timing="pre-merge" or timing="" (default).
Replaces the direct RunGatesWithCache call at finalize_wave step 3.
Signature mirrors RunGatesWithCache exactly; cache param may be nil.


```
// In pkg/protocol/gates.go:
func RunPreMergeGates(manifest *IMPLManifest, waveNumber int, repoDir string, cache *gatecache.Cache) ([]GateResult, error)

```

### RunPostMergeGates

Executes only gates with timing="post-merge".
Called at finalize_wave step 5, after MergeAgents completes successfully.
Returns empty slice (not error) when no post-merge gates are defined.
Runs without cache (post-merge state is always fresh).


```
// In pkg/protocol/gates.go:
func RunPostMergeGates(manifest *IMPLManifest, waveNumber int, repoDir string) ([]GateResult, error)

```



## Quality Gates

Level: standard

- **build**: `go build ./...` (required: true)
- **test**: `go test ./pkg/protocol/... ./cmd/saw/...` (required: true)
- **lint**: `go vet ./...` (required: true)

