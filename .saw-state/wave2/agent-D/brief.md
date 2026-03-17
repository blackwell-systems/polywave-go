# Agent D Brief - Wave 2

**IMPL Doc:** /Users/dayna.blackwell/code/scout-and-wave-go/docs/IMPL/IMPL-failure-recovery-ux.yaml

## Files Owned

- `cmd/saw/main.go`
- `pkg/engine/finalize.go`
- `pkg/engine/runner.go`


## Task

## What to Implement

Integration wiring: connect the three new features (gate caching, failure
context injection, resume detection) into existing engine and CLI entry points.

### Modify: cmd/saw/main.go

Add the two new CLI commands to the root command's AddCommand call:
- `newBuildRetryContextCmd()` (from Agent B)
- `newResumeDetectCmd()` (from Agent C)

These are new commands created by Agents B and C in Wave 1. Just add the
AddCommand calls in main.go.

### Modify: pkg/engine/finalize.go

In the `FinalizeWave` function, Step 3 (RunGates), replace the direct call
to `protocol.RunGates` with `protocol.RunGatesWithCache`:
- Create a `gatecache.Cache` with stateDir = filepath.Join(opts.RepoPath, ".saw-state")
  and TTL of 5 minutes
- Call `protocol.RunGatesWithCache(manifest, opts.WaveNum, opts.RepoPath, cache)`
  instead of `protocol.RunGates(manifest, opts.WaveNum, opts.RepoPath)`
- Add import for `"github.com/blackwell-systems/scout-and-wave-go/pkg/gatecache"`
- Add import for `"time"` if not already present

### Modify: pkg/engine/runner.go

In the `RunSingleAgent` function, when `promptPrefix` is empty but the agent
has a completion report with non-"complete" status, auto-build retry context:
- After the orchestrator is created, check if a completion report exists for
  the agent via `protocol.Load` + checking CompletionReports map
- If report exists and status != "complete", call
  `retryctx.BuildRetryContext(opts.IMPLPath, agentLetter, 1)`
- If retry context is successfully built, prepend its PromptText to the
  agent's prompt via the existing promptPrefix parameter
- This is a best-effort enhancement: if BuildRetryContext fails, log to
  stderr and continue without retry context
- Add import for `"github.com/blackwell-systems/scout-and-wave-go/pkg/retryctx"`

## Interfaces to Call
- `gatecache.New` (from Agent A)
- `protocol.RunGatesWithCache` (from Agent A)
- `retryctx.BuildRetryContext` (from Agent B)
- `protocol.Load` (existing)

## Tests to Write
No new test files — existing tests in pkg/engine/ cover the integration
paths. Verify the build compiles and existing tests pass.

## Verification Gate
```bash
go build ./...
go vet ./...
go test ./pkg/engine/ -count=1
go test ./cmd/saw/ -count=1
```

## Constraints
- Do NOT change function signatures of FinalizeWave or RunSingleAgent
- Gate cache creation failure is non-fatal: fall back to nil cache
  (which makes RunGatesWithCache behave like RunGates)
- Retry context injection is best-effort: log errors to stderr, don't fail
- Keep the existing behavior for RunSingleAgent when promptPrefix is
  explicitly provided (do not override user-supplied prefix)



## Interface Contracts

### gatecache.Cache

In-memory + file-backed cache for quality gate results. Keyed by a hash
of HEAD commit, staged diff stat, and unstaged diff stat. Supports TTL
expiration. Stored in .saw-state/gate-cache.json per project.


```
type Cache struct { ... }
type CacheKey struct {
  HeadCommit    string `json:"head_commit"`
  StagedStat    string `json:"staged_stat"`
  UnstagedStat  string `json:"unstaged_stat"`
}
type CachedResult struct {
  GateType  string    `json:"gate_type"`
  Command   string    `json:"command"`
  Passed    bool      `json:"passed"`
  ExitCode  int       `json:"exit_code"`
  Stdout    string    `json:"stdout"`
  Stderr    string    `json:"stderr"`
  FromCache bool      `json:"from_cache"`
  CachedAt  time.Time `json:"cached_at"`
  TTL       time.Duration `json:"-"`
}
func New(stateDir string, ttl time.Duration) *Cache
func (c *Cache) Get(key CacheKey, gateType string) (*CachedResult, bool)
func (c *Cache) Put(key CacheKey, gateType string, result CachedResult) error
func (c *Cache) BuildKey(repoDir string) (CacheKey, error)
func (c *Cache) Invalidate() error

```

### RunGatesWithCache

Wrapper around protocol.RunGates that checks the cache before executing
each gate. Results are stored in the cache after execution. Cached results
carry FromCache=true. Called from finalize.go and run_gates_cmd.go.


```
func RunGatesWithCache(manifest *protocol.IMPLManifest, waveNumber int, repoDir string, cache *gatecache.Cache) ([]protocol.GateResult, error)

```

### retryctx.BuildRetryContext

Reads an agent's completion report from the manifest, classifies the error
type (import error, type error, test failure, build error, lint error),
and builds a structured retry context string with attempt number, error
classification, output excerpt, gate results, and suggested fixes.


```
type ErrorClass string
const (
  ErrorClassImport  ErrorClass = "import_error"
  ErrorClassType    ErrorClass = "type_error"
  ErrorClassTest    ErrorClass = "test_failure"
  ErrorClassBuild   ErrorClass = "build_error"
  ErrorClassLint    ErrorClass = "lint_error"
  ErrorClassUnknown ErrorClass = "unknown"
)
type RetryContext struct {
  AttemptNumber    int        `json:"attempt_number"`
  AgentID          string     `json:"agent_id"`
  ErrorClass       ErrorClass `json:"error_class"`
  ErrorExcerpt     string     `json:"error_excerpt"`
  GateResults      []string   `json:"gate_results"`
  SuggestedFixes   []string   `json:"suggested_fixes"`
  PriorNotes       string     `json:"prior_notes"`
  PromptText       string     `json:"prompt_text"`
}
func ClassifyError(output string) ErrorClass
func BuildRetryContext(manifestPath string, agentID string, attemptNum int) (*RetryContext, error)

```

### resume.Detect

Scans a project for SAW state to detect interrupted sessions. Checks
IMPL docs for non-complete status, .saw-state/ for journals with
incomplete waves, git worktrees for orphaned branches, and manifest
completion reports for partial/blocked agents.


```
type SessionState struct {
  IMPLSlug          string   `json:"impl_slug"`
  IMPLPath          string   `json:"impl_path"`
  CurrentWave       int      `json:"current_wave"`
  TotalWaves        int      `json:"total_waves"`
  CompletedAgents   []string `json:"completed_agents"`
  FailedAgents      []string `json:"failed_agents"`
  PendingAgents     []string `json:"pending_agents"`
  OrphanedWorktrees []string `json:"orphaned_worktrees"`
  SuggestedAction   string   `json:"suggested_action"`
  ProgressPct       float64  `json:"progress_pct"`
  CanAutoResume     bool     `json:"can_auto_resume"`
  ResumeCommand     string   `json:"resume_command"`
}
func Detect(repoPath string) ([]SessionState, error)

```



## Quality Gates

Level: standard

- **build**: `go build ./...` (required: true)
- **lint**: `go vet ./...` (required: true)
- **test**: `go test ./...` (required: true)

