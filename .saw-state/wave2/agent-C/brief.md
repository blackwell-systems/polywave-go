# Agent C Brief - Wave 2

**IMPL Doc:** /Users/dayna.blackwell/code/scout-and-wave-go/docs/IMPL/IMPL-repo-mismatch-hardening.yaml

## Files Owned

- `cmd/saw/prepare_wave.go`
- `cmd/saw/prepare_wave_test.go`


## Task

## Wire Validation into prepare-wave CLI

Modify `cmd/saw/prepare_wave.go` to call the repo validation functions
from Agent A before creating worktrees.

### 1. Add pre-flight repo validation

After loading the manifest (line ~148) and before baseline quality gates,
insert a new step "Step 0b1: Repo mismatch validation":

```go
// Step 0b1: Validate repo match
configRepos := loadSAWConfigRepos(filepath.Join(projectRoot, "saw.config.json"))
repoEntries := make([]protocol.RepoEntry, len(configRepos))
for i, r := range configRepos {
    repoEntries[i] = protocol.RepoEntry{Name: r.Name, Path: r.Path}
}

if err := protocol.ValidateRepoMatch(doc, projectRoot, repoEntries); err != nil {
    return fmt.Errorf("prepare-wave: repo mismatch: %w", err)
}
```

### 2. Add multi-repo file existence check

Replace or augment the existing file existence spot-check (if any) with
ValidateFileExistenceMultiRepo:

```go
fileWarnings := protocol.ValidateFileExistenceMultiRepo(doc, projectRoot, repoEntries)
for _, w := range fileWarnings {
    if w.Code == "E16_REPO_MISMATCH_SUSPECTED" {
        return fmt.Errorf("prepare-wave: %s", w.Message)
    }
    fmt.Fprintf(os.Stderr, "prepare-wave: warning: %s\n", w.Message)
}
```

### 3. Convert sawRepoEntry to use protocol.RepoEntry

The existing `sawRepoEntry` type in prepare_wave.go duplicates what is now
`protocol.RepoEntry`. Refactor:
- Change `loadSAWConfigRepos` to return `[]protocol.RepoEntry`
- Update `resolveAgentRepoRoot` to accept `[]protocol.RepoEntry`
- Remove the `sawRepoEntry` type

### Tests

Add to `cmd/saw/prepare_wave_test.go`:
- TestPrepareWave_RepoMismatchBlocksWorktreeCreation
- TestPrepareWave_AllFilesMissingSuspectsRepoMismatch
- TestPrepareWave_CrossRepoValidPasses

### Constraints
- Repo mismatch must be a HARD blocker (return error, do not create worktrees)
- File existence warnings for individual files are soft (stderr warning)
- E16_REPO_MISMATCH_SUSPECTED (ALL modify files missing) is a hard blocker
- Must run BEFORE baseline quality gates (gates run in the target repo)



## Interface Contracts

### ResolveTargetRepos

Resolves the target repository paths from an IMPL manifest by analyzing
file_ownership repo: fields, falling back to test_command/lint_command
path inference. Returns a map of repo name -> absolute path.


```
func ResolveTargetRepos(manifest *IMPLManifest, fallbackRepoPath string, configRepos []RepoEntry) (map[string]string, error)

type RepoEntry struct {
    Name string
    Path string
}

Returns:
- map[string]string: repo name -> absolute path (at least one entry)
- error: if no repos can be resolved

```

### ValidateRepoMatch

Pre-flight check that validates the resolved target repo(s) match the
repo where worktrees would be created. Returns nil if valid, error with
clear diagnostic message if mismatched.


```
func ValidateRepoMatch(manifest *IMPLManifest, worktreeRepoPath string, configRepos []RepoEntry) error

Error format: "IMPL targets repo 'X' at /path/to/X but worktrees would be
created in /path/to/Y - aborting. Set repo: field in file_ownership or
configure repos in saw.config.json"

```

### ValidateFileExistenceMultiRepo

Enhanced version of ValidateFileExistence that resolves files across
multiple repos using the configRepos registry. Spot-checks that at least
some action=modify files exist in their target repos.


```
func ValidateFileExistenceMultiRepo(m *IMPLManifest, primaryRepoPath string, configRepos []RepoEntry) []ValidationError

```



## Quality Gates

Level: standard

- **build**: `go build ./...` (required: true)
- **lint**: `go vet ./...` (required: true)
- **test**: `go test ./...` (required: true)

