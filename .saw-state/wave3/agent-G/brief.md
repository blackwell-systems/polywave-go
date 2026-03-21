# Agent G Brief - Wave 3

**IMPL Doc:** /Users/dayna.blackwell/code/scout-and-wave/docs/IMPL/IMPL-protocol-conformity.yaml

## Files Owned

- `cmd/saw/program_execute_cmd.go`
- `cmd/saw/program_execute_cmd_test.go`


## Task

## Agent G — Wire Tier Loop into CLI (Integration)

**Repo:** scout-and-wave-go (Go engine)
**Files:** cmd/saw/program_execute_cmd.go, cmd/saw/program_execute_cmd_test.go

### What to Implement

Create `cmd/saw/program_execute_cmd.go` — a new `sawtools program-execute` command
that invokes the tier loop from Agent A.

```go
func newProgramExecuteCmd() *cobra.Command
```

The command:
1. Takes a PROGRAM manifest path as positional argument
2. Flags: `--auto` (bool, enables auto-advancement), `--model` (string, model override)
3. Calls `engine.RunTierLoop()` with appropriate opts
4. Streams TierLoopEvents to stdout as JSON lines
5. Exits 0 on program complete, 1 on failure, 2 on parse error

Also wire `LaunchParallelScouts` into the tier loop by setting the function
variable that Agent A stubbed.

Register the command in the root command (find where other program commands
are registered and add this one).

Wire `UpdateProgramIMPLStatus()` (Agent B) into the IMPL completion hooks
by calling it from the tier loop's OnEvent handler when impl_complete events fire.

### Tests
- Test command parses flags correctly
- Test command exits 2 on invalid manifest path

### Verification Gate
```
cd /Users/dayna.blackwell/code/scout-and-wave-go && go build ./cmd/saw/... && go test ./cmd/saw/ -run TestProgramExecute -v
```

### Constraints
- Create new files only (program_execute_cmd.go)
- Wire together Agent A + Agent B functions; do not reimplement
- Follow existing cmd/saw patterns (see program_replan_cmd.go, program_status_cmd.go)



## Interface Contracts

### RunTierLoop

Full tier execution loop that reads PROGRAM manifest, partitions IMPLs,
launches Scouts in parallel, executes waves, runs tier gates, freezes
contracts, and advances to next tier. This is the missing orchestration
function that ties E28-E34 together.


```
func RunTierLoop(ctx context.Context, opts TierLoopOpts) (*TierLoopResult, error)

type TierLoopOpts struct {
    ManifestPath string
    RepoPath     string
    AutoMode     bool
    Model        string
    OnEvent      func(TierLoopEvent)
}

type TierLoopResult struct {
    TiersExecuted   int
    TiersRemaining  int
    ProgramComplete bool
    FinalState      string
    Errors          []string
}

type TierLoopEvent struct {
    Type    string // "tier_started", "scout_launched", "impl_complete", "tier_gate", "contracts_frozen", "tier_advanced", "replan_triggered"
    Tier    int
    Detail  string
}

```

### PartitionIMPLsByStatus

Partitions IMPLs in a tier into needsScout vs preExisting groups per E28A.


```
func PartitionIMPLsByStatus(manifest *protocol.PROGRAMManifest, tierNumber int) (needsScout []string, preExisting []string)

```

### LaunchParallelScouts

Launches N Scout agents in parallel for all pending IMPLs in a tier (E31).


```
func LaunchParallelScouts(ctx context.Context, opts ParallelScoutOpts) (*ParallelScoutResult, error)

type ParallelScoutOpts struct {
    ManifestPath string
    RepoPath     string
    TierNumber   int
    Slugs        []string
    Model        string
    OnEvent      func(TierLoopEvent)
}

type ParallelScoutResult struct {
    Completed []string
    Failed    []string
    Errors    map[string]string
}

```

### UpdateProgramIMPLStatus

Hook called after IMPL state transitions to update PROGRAM manifest (E32).


```
func UpdateProgramIMPLStatus(manifestPath string, implSlug string, newStatus string) error

```

### AutoTriggerReplan

Automatically triggers Planner re-engagement when tier gate fails (E34).


```
func AutoTriggerReplan(manifestPath string, tierNumber int, gateResult *protocol.TierGateResult, model string) (*ReplanResult, error)

```



## Quality Gates

Level: standard

- **build**: `cd /Users/dayna.blackwell/code/scout-and-wave-go && go build ./...` (required: true)
- **lint**: `cd /Users/dayna.blackwell/code/scout-and-wave-go && go vet ./...` (required: true)
- **test**: `cd /Users/dayna.blackwell/code/scout-and-wave-go && go test ./pkg/engine/... ./pkg/protocol/... ./pkg/orchestrator/...` (required: true)

