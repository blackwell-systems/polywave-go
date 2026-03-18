# Agent I Brief - Wave 4

**IMPL Doc:** docs/IMPL/IMPL-autonomy-layer.yaml

## Files Owned

- `cmd/saw/daemon_cmd.go`
- `cmd/saw/daemon_cmd_test.go`


## Task

## What to Implement
Implement the daemon CLI command in cmd/saw/daemon_cmd.go.

1. `sawtools daemon --repo-dir <path> --autonomy <level> [--model <model>] [--poll-interval <duration>]`
   - Loads autonomy config from saw.config.json (via autonomy.LoadConfig)
   - Overrides level if --autonomy flag is set
   - Creates DaemonOpts and calls engine.RunDaemon
   - Handles SIGINT/SIGTERM for clean shutdown
   - Streams daemon events as JSON lines to stdout

2. Signal handling:
   - Trap SIGINT and SIGTERM
   - Cancel context on signal receipt
   - Print "daemon shutting down..." message
   - Wait for RunDaemon to return cleanly

## Interfaces to Call
- `autonomy.LoadConfig(repoDir)` — load config (Wave 1, Agent B)
- `autonomy.ParseLevel(levelStr)` — validate --autonomy flag (Wave 1, Agent A)
- `engine.RunDaemon(ctx, opts)` — daemon loop (Wave 3, Agent H)

## Tests to Write
1. TestDaemonCmd_Flags — verify flag parsing
2. TestDaemonCmd_DefaultConfig — uses saw.config.json
3. TestDaemonCmd_OverrideLevel — --autonomy flag overrides config

## Verification Gate
```bash
go build ./cmd/saw/...
go vet ./cmd/saw/...
go test ./cmd/saw/ -run TestDaemon
```

## Constraints
- Use os/signal for signal handling
- Output daemon events as JSON lines (one JSON object per line)
- Default poll interval: 30 seconds
- Do NOT modify main.go — integration agent will wire this in



## Interface Contracts

### ShouldAutoApprove

Central decision function checked at every autonomy decision point.
All R1-R4 features call this instead of hardcoding behavior.


```
func ShouldAutoApprove(level Level, stage Stage) bool

```

### LoadConfig

Loads autonomy configuration from saw.config.json in the repo root.
Falls back to DefaultConfig() if file is missing.


```
func LoadConfig(repoPath string) (Config, error)

```

### EffectiveLevel

Returns the effective autonomy level, considering per-IMPL override.
If overrideLevel is non-empty, it takes precedence over config.Level.


```
func EffectiveLevel(cfg Config, overrideLevel string) Level

```

### AutoRemediate

Engine-level function that auto-remediates a failed FinalizeWave result.
Uses existing retryctx + FixBuildFailure. Returns updated result after fix.


```
func AutoRemediate(ctx context.Context, opts AutoRemediateOpts) (*AutoRemediateResult, error)

```

### ClosedLoopGateRetry

Pre-merge per-agent gate retry. When a retryable gate fails, sends error
context to the responsible agent in its worktree for a fix attempt.


```
func ClosedLoopGateRetry(ctx context.Context, opts ClosedLoopRetryOpts) (*ClosedLoopRetryResult, error)

```

### queue.Manager

Manages the IMPL queue directory (docs/IMPL/queue/).
Provides Add, List, Next, UpdateStatus operations.


```
type Manager struct { ... }
func NewManager(repoPath string) *Manager
func (m *Manager) Add(item Item) error
func (m *Manager) List() ([]Item, error)
func (m *Manager) Next() (*Item, error)
func (m *Manager) UpdateStatus(slug string, status string) error

```

### CheckQueue

Called after MarkIMPLComplete. If autonomy allows, triggers Scout for
the next eligible queue item.


```
func CheckQueue(ctx context.Context, opts CheckQueueOpts) (*CheckQueueResult, error)

```

### RunDaemon

Engine-level daemon loop. Processes queue items, runs Scout/Wave cycles,
auto-remediates failures, advances queue.


```
func RunDaemon(ctx context.Context, opts DaemonOpts) error

```



## Quality Gates

Level: standard

- **build**: `go build ./...` (required: true)
- **test**: `go test ./...` (required: true)
- **lint**: `go vet ./...` (required: false)
  Check for common Go mistakes

