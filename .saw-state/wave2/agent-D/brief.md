# Agent D Brief - Wave 2

**IMPL Doc:** /Users/dayna.blackwell/code/scout-and-wave-go/docs/IMPL/IMPL-type-unification.yaml

## Files Owned

- `pkg/engine/engine.go`
- `pkg/engine/runner.go`


## Task

Remove the manifestToIMPLDoc adapter and wire engine.PrioritizeAgents into orchestrator.

**What to implement:**

1. In `pkg/engine/engine.go`:
   - Remove `loadIMPLDoc()` function entirely
   - Remove `manifestToIMPLDoc()` function entirely
   - Remove `orchestrator.SetParseIMPLDocFunc(loadIMPLDoc)` from init()
   - In init(), ADD: `orchestrator.SetPrioritizeAgentsFunc(PrioritizeAgents)` — this wires the real DAG-based scheduler into the orchestrator (the whole reason for this migration)
   - The `SetRunWaveAgentStructuredFunc` call in init() stays but the lambda's `agentSpec types.AgentSpec` parameter stays as-is (orchestrator still passes types.AgentSpec at this boundary)
   - Remove `var _ *types.IMPLDoc` (the unused type anchor)
   - Remove the `types` import if no longer needed
   - Keep the `protocol` import (still needed for other engine functions)

2. In `pkg/engine/runner.go`:
   - Update `ParseIMPLDoc(path string) (*types.IMPLDoc, error)` — this is an exported function used externally. Either:
     a. Keep it as a backward-compat wrapper that calls protocol.Load and converts, OR
     b. Deprecate it with a comment pointing to protocol.Load()
   - Update `ValidateInvariants(doc *types.IMPLDoc) error` to `ValidateInvariants(manifest *protocol.IMPLManifest) error` since protocol.ValidateInvariants now takes *IMPLManifest (from Agent B)
   - Update `runScaffoldBuildVerificationWithDoc` if it takes *types.IMPLDoc — check if it can take *protocol.IMPLManifest instead, or keep the adapter

**Interface contracts:**
- `engine.PrioritizeAgents` already has the right signature: `func(manifest *protocol.IMPLManifest, waveNum int) []string`
- `orchestrator.SetPrioritizeAgentsFunc` now takes `func(*protocol.IMPLManifest, int) []string` (from Agent A)

**Verification gate:**
```
go build ./...
go vet ./...
go test ./pkg/engine/... -count=1
go test ./pkg/orchestrator/... -count=1
```

**Constraints:**
- ParseIMPLDoc is exported and may have external callers — deprecate, don't delete
- runScaffoldBuildVerificationWithDoc takes *types.IMPLDoc for scaffold package derivation — keep adapter if needed
- Verify engine init() runs without import cycle (it won't — engine already imports orchestrator)



## Interface Contracts

### Orchestrator.implDoc field type change

Core struct field changes from *types.IMPLDoc to *protocol.IMPLManifest

```
type Orchestrator struct {
    state          protocol.ProtocolState
    implDoc        *protocol.IMPLManifest  // was *types.IMPLDoc
    repoPath       string
    currentWave    int
    implDocPath    string
    eventPublisher EventPublisher
    defaultModel   string
    worktreePaths  map[string]string
}

```

### IMPLDoc() return type change

Public accessor returns *protocol.IMPLManifest instead of *types.IMPLDoc

```
func (o *Orchestrator) IMPLDoc() *protocol.IMPLManifest

```

### newFromDoc signature change

Internal constructor takes *protocol.IMPLManifest

```
func newFromDoc(doc *protocol.IMPLManifest, repoPath, implDocPath string) *Orchestrator

```

### New() uses protocol.Load directly

New() calls protocol.Load() instead of parseIMPLDocFunc

```
func New(repoPath string, implDocPath string) (*Orchestrator, error) {
    var doc *protocol.IMPLManifest
    if implDocPath != "" {
        var err error
        doc, err = protocol.Load(implDocPath)
        if err != nil {
            return nil, fmt.Errorf("orchestrator.New: %w", err)
        }
    }
    return &Orchestrator{
        state: protocol.StateScoutPending, implDoc: doc,
        repoPath: repoPath, implDocPath: implDocPath,
    }, nil
}

```

### validateInvariantsFunc signature

Takes *protocol.IMPLManifest instead of *types.IMPLDoc

```
var validateInvariantsFunc = func(doc *protocol.IMPLManifest) error { return nil }
func SetValidateInvariantsFunc(f func(doc *protocol.IMPLManifest) error)

```

### prioritizeAgentsFunc signature

Takes *protocol.IMPLManifest to match engine.PrioritizeAgents

```
var prioritizeAgentsFunc = func(manifest *protocol.IMPLManifest, waveNum int) []string
func SetPrioritizeAgentsFunc(f func(manifest *protocol.IMPLManifest, waveNum int) []string)

```

### ValidateInvariants in protocol/parser.go

Takes *IMPLManifest instead of *types.IMPLDoc

```
func ValidateInvariants(manifest *IMPLManifest) error

```

### engine.ValidateInvariants adapter

Updated to forward *protocol.IMPLManifest (was *types.IMPLDoc)

```
func ValidateInvariants(manifest *protocol.IMPLManifest) error {
    return protocol.ValidateInvariants(manifest)
}

```



## Quality Gates

Level: standard

- **build**: `go build ./...` (required: true)
- **lint**: `go vet ./...` (required: true)
- **test**: `go test ./...` (required: true)

