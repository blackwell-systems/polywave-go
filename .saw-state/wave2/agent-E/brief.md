# Agent E Brief - Wave 2

**IMPL Doc:** /Users/dayna.blackwell/code/scout-and-wave-go/docs/IMPL/IMPL-wiring-gaps.yaml

## Files Owned

- `cmd/saw/prepare_wave.go`


## Task

## Agent E — Wire pre-wave-gate into prepare-wave pre-flight (C7 integration)

**What to implement:**
After Wave 1 registers the pre-wave-gate command, wire it as a pre-flight
check in the prepare-wave batching command. This ensures manifests pass
readiness checks before worktrees are created.

**Where to wire:**
In `cmd/saw/prepare_wave.go`, add a pre-flight readiness check early in the
RunE function, before worktree creation begins:

1. Load the manifest with `protocol.Load(manifestPath)`
2. Call `protocol.PreWaveGate(manifest)` to run readiness checks
3. If `!gateResult.Ready`, print the gate result as JSON and return an error
   explaining which checks failed
4. If ready, proceed with existing worktree creation logic

**Note:** The `protocol.PreWaveGate()` function already exists and is called by
the standalone `pre-wave-gate` CLI command. We are reusing the same function
inline in prepare-wave so that `sawtools prepare-wave` automatically includes
the readiness check.

**Interfaces:**
- `protocol.PreWaveGate(manifest)` — already exists, returns PreWaveGateResult
- `protocol.Load(path)` — already exists

**Tests:**
- Verify compilation: `go build ./cmd/saw/`
- Run existing prepare-wave tests: `go test ./cmd/saw/ -run TestPrepareWave`

**Verification gate:**
```
go build ./cmd/saw/ && go vet ./cmd/saw/ && go test ./cmd/saw/...
```

**Constraints:**
- Only modify cmd/saw/prepare_wave.go
- Pre-wave gate failure should block worktree creation (return error)
- Print structured JSON output on failure for machine-readability



## Interface Contracts

### ClosedLoopGateRetry

Already-exported function in pkg/engine. Agent B wires calls to it from
finalize-wave when per-agent gates fail. No signature change needed.


```
func ClosedLoopGateRetry(ctx context.Context, opts ClosedLoopRetryOpts) (*ClosedLoopRetryResult, error)

```

### ScoutCorrectionLoop

Already-exported function in pkg/engine. Agent C replaces the direct
RunScout call in run-scout CLI with ScoutCorrectionLoop for self-healing.


```
func ScoutCorrectionLoop(ctx context.Context, opts ScoutCorrectionOpts, onChunk func(string)) error

```

### AdvanceTierAutomatically

Already-exported function in pkg/engine. Agent D wires it into the
finalize-tier CLI command after tier gate passes.


```
func AdvanceTierAutomatically(manifest *protocol.PROGRAMManifest, completedTier int, repoPath string, autoMode bool) (*TierAdvanceResult, error)

```

### SyncProgramStatusFromDisk

Already-exported function in pkg/engine. Agent D wires it into
program-status CLI command before displaying status.


```
func SyncProgramStatusFromDisk(manifestPath string, repoPath string) error

```



## Quality Gates

Level: standard

- **build**: `go build ./...` (required: true)
- **lint**: `go vet ./...` (required: true)
- **test**: `go test ./...` (required: true)

