# Agent A Brief - Wave 1

**IMPL Doc:** /Users/dayna.blackwell/code/scout-and-wave-web/docs/IMPL/IMPL-gate-timing-fix.yaml

## Files Owned

- `pkg/protocol/types.go`
- `pkg/protocol/schema_unknown_keys.go`
- `pkg/protocol/gates.go`
- `pkg/protocol/gates_test.go`


## Task

## Context

You are implementing the gate timing fix in the scout-and-wave-go repo at
`/Users/dayna.blackwell/code/scout-and-wave-go`.

The bug: `finalize-wave` runs ALL quality gates at step 3 (pre-merge). Gates
that check whether content exists in main-branch files always fail pre-merge
because agent branches haven't been merged yet. We need a `timing` field on
`QualityGate` to route gates to the correct phase.

## Files to modify

- `pkg/protocol/types.go`
- `pkg/protocol/schema_unknown_keys.go`
- `pkg/protocol/gates.go`
- `pkg/protocol/gates_test.go`

## Task 1: Add `Timing` field to `QualityGate` (types.go)

In the `QualityGate` struct, add after the `Fix` field:

```go
// Timing controls when the gate executes during finalize-wave.
// "pre-merge"  — run at step 3, before MergeAgents (default when empty)
// "post-merge" — run at step 5, after MergeAgents completes
// Empty string is treated as "pre-merge" for backward compatibility.
Timing string `yaml:"timing,omitempty" json:"timing,omitempty"`
```

## Task 2: Register `timing` as known key (schema_unknown_keys.go)

In the `knownKeys` map, in the `"quality_gate"` entry, add:
```go
"timing": true,
```
This prevents E16 validation from flagging IMPL docs that use the new field.

## Task 3: Add RunPreMergeGates and RunPostMergeGates (gates.go)

Add two new exported functions. Each is a filtered wrapper around the existing
gate execution logic.

**RunPreMergeGates** — runs gates where Timing == "" or Timing == "pre-merge":

```go
func RunPreMergeGates(manifest *IMPLManifest, waveNumber int, repoDir string, cache *gatecache.Cache) ([]GateResult, error) {
    filtered := filterGatesByTiming(manifest, "pre-merge")
    if len(filtered) == 0 {
        return []GateResult{}, nil
    }
    // Build a temporary manifest with only pre-merge gates
    tmp := *manifest
    tmp.QualityGates = &QualityGates{
        Level: manifest.QualityGates.Level,
        Gates: filtered,
    }
    return RunGatesWithCache(&tmp, waveNumber, repoDir, cache)
}
```

**RunPostMergeGates** — runs gates where Timing == "post-merge":

```go
func RunPostMergeGates(manifest *IMPLManifest, waveNumber int, repoDir string) ([]GateResult, error) {
    filtered := filterGatesByTiming(manifest, "post-merge")
    if len(filtered) == 0 {
        return []GateResult{}, nil
    }
    tmp := *manifest
    tmp.QualityGates = &QualityGates{
        Level: manifest.QualityGates.Level,
        Gates: filtered,
    }
    return RunGates(&tmp, waveNumber, repoDir)
}
```

**filterGatesByTiming** — private helper:

```go
// filterGatesByTiming returns gates matching the given timing category.
// timing="pre-merge" returns gates with Timing=="" or Timing=="pre-merge".
// timing="post-merge" returns gates with Timing=="post-merge".
func filterGatesByTiming(manifest *IMPLManifest, timing string) []QualityGate {
    if manifest.QualityGates == nil {
        return nil
    }
    var out []QualityGate
    for _, g := range manifest.QualityGates.Gates {
        switch timing {
        case "pre-merge":
            if g.Timing == "" || g.Timing == "pre-merge" {
                out = append(out, g)
            }
        case "post-merge":
            if g.Timing == "post-merge" {
                out = append(out, g)
            }
        }
    }
    return out
}
```

Note: RunPreMergeGates handles the nil QualityGates case via filterGatesByTiming
returning nil and the len==0 early return. No panic risk.

## Task 4: Add tests (gates_test.go)

Add the following test functions at the end of the existing test file:

**TestFilterGatesByTiming** — verifies gate routing logic:
- Mix of gates: one with Timing="", one with Timing="pre-merge", one with Timing="post-merge"
- filterGatesByTiming(manifest, "pre-merge") returns the first two only
- filterGatesByTiming(manifest, "post-merge") returns the third only

**TestRunPreMergeGates_OnlyRunsPreMerge** — integration test:
- Define a manifest with one pre-merge gate (echo ok) and one post-merge gate (exit 1)
- Call RunPreMergeGates — only the echo gate runs, result is Passed=true
- The post-merge exit 1 gate must NOT run (result count == 1)

**TestRunPostMergeGates_OnlyRunsPostMerge** — integration test:
- Define a manifest with one pre-merge gate (exit 1) and one post-merge gate (echo ok)
- Call RunPostMergeGates — only the echo gate runs, result is Passed=true
- The pre-merge exit 1 gate must NOT run (result count == 1)

**TestRunPreMergeGates_EmptyWhenNoneMatch** — empty case:
- Manifest with only a post-merge gate
- RunPreMergeGates returns empty slice, no error

**TestRunPostMergeGates_EmptyWhenNoneMatch** — empty case:
- Manifest with only a pre-merge gate
- RunPostMergeGates returns empty slice, no error

**TestRunPreMergeGates_BackwardCompat** — existing gates without Timing field:
- Manifest with two gates, both with Timing="" (omitted)
- RunPreMergeGates runs both (backward compatible behavior)

## Verification gate

```
go build ./...
go vet ./...
go test ./pkg/protocol/... -run "TestFilterGatesByTiming|TestRunPreMergeGates|TestRunPostMergeGates"
go test ./pkg/protocol/...
```

## Constraints

- Do NOT modify `RunGates` or `RunGatesWithCache` — they remain unchanged as
  the underlying execution engines. The new functions are pure wrappers.
- Do NOT add `timing` as a validated enum in enumvalidation.go — the field is
  free-form string with defined semantics, not a strict enum requiring error codes.
- The `timing` field must be `omitempty` in both yaml and json tags so existing
  IMPL docs without the field remain valid.



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

