# Agent G Brief - Wave 3

**IMPL Doc:** /Users/dayna.blackwell/code/scout-and-wave-go/docs/IMPL/IMPL-sawtools-cli-gaps.yaml

## Files Owned

- `cmd/saw/main.go`


## Task

## Integration: wire set-impl-state into main.go

### What to implement

Add `newSetImplStateCmd()` to the `rootCmd.AddCommand(...)` list in
`cmd/saw/main.go`.

Find the place near `newSetCompletionCmd()` or `newMarkCompleteCmd()`
(both are state-setting commands) and add:

```go
newSetImplStateCmd(),
```

### Verification gate

```
go build ./...
sawtools set-impl-state --help
```

The second command must exit 0 and print the set-impl-state help text.

### Constraints

- Only modify `cmd/saw/main.go`
- One line addition only
- Do NOT reorder or restructure the existing AddCommand list


## Interface Contracts

### SetImplState

Atomic read-modify-write on an IMPL manifest's state field.
Validates the transition is allowed by the protocol state machine.
Optionally commits the change to git.

```
func SetImplState(manifestPath string, newState ProtocolState, opts SetImplStateOpts) (*SetImplStateResult, error)

type SetImplStateOpts struct {
    Commit    bool
    CommitMsg string
}

type SetImplStateResult struct {
    PreviousState ProtocolState `json:"previous_state"`
    NewState      ProtocolState `json:"new_state"`
    Committed     bool          `json:"committed"`
    CommitSHA     string        `json:"commit_sha,omitempty"`
}
```

### IsSoloWave

Returns true if none of the expected worktree directories exist for the wave

```
func IsSoloWave(manifest *IMPLManifest, waveNum int, repoDir string) bool
```

### AllBranchesAbsent

Returns true when every agent branch for a wave is absent from git (both slug-scoped and legacy names)

```
func AllBranchesAbsent(manifest *IMPLManifest, waveNum int, repoDir string) bool
```

### classifySeverity_updated

Updated package-private classifySeverity with json-tag awareness and threshold parameter

```
func classifySeverity(exportName string, category string, filePath string, threshold string) string
```

### IntegrationGapSeverityThreshold

New IMPLManifest field for configuring severity threshold in ValidateIntegration

```
IntegrationGapSeverityThreshold string `yaml:"integration_gap_severity_threshold,omitempty" json:"integration_gap_severity_threshold,omitempty"`
```



## Quality Gates

Level: standard

- **build**: `go build ./...` (required: true)
- **test**: `go test ./... -timeout 120s` (required: true)

