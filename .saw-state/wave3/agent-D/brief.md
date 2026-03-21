# Agent D Brief - Wave 3

**IMPL Doc:** /Users/dayna.blackwell/code/scout-and-wave/docs/IMPL/IMPL-program-import.yaml

## Files Owned

- `cmd/saw/import_impls_cmd.go`
- `cmd/saw/import_impls_cmd_test.go`
- `cmd/saw/validate_program_cmd.go`
- `cmd/saw/run_scout_cmd.go`
- `cmd/saw/main.go`


## Task

## Agent D — SDK Commands (import-impls, validate-program --import-mode, run-scout skip)

**Repo:** /Users/dayna.blackwell/code/scout-and-wave-go
**What to implement:**

1. **cmd/saw/import_impls_cmd.go** (NEW) — New `sawtools import-impls` command:
   ```
   sawtools import-impls --program PROGRAM-slug.yaml --from-impls IMPL-a.yaml IMPL-b.yaml
   sawtools import-impls --program PROGRAM-slug.yaml --discover
   ```
   Implementation:
   - Parse program manifest (or create minimal one if --program doesn't exist yet)
   - If --from-impls: parse each specified IMPL doc
   - If --discover: glob docs/IMPL/IMPL-*.yaml and docs/IMPL/complete/IMPL-*.yaml
   - For each IMPL doc found:
     - Extract slug, title, state, file_ownership, wave count, agent count
     - Map IMPL state to program status (SCOUT_COMPLETE→reviewed, COMPLETE→complete)
     - Analyze file_ownership overlap with other IMPLs to suggest tier assignment
       (IMPLs with overlapping dependencies go to later tiers)
   - Create/update PROGRAM manifest with new IMPL entries and tier assignments
   - Run protocol.ValidateProgramImportMode for P1/P2 conflict detection
   - Output JSON ImportIMPLsResult (types from pkg/protocol/program_types.go)
   - Register in main.go: add newImportImplsCmd() to rootCmd.AddCommand(...)

2. **cmd/saw/import_impls_cmd_test.go** (NEW) — Tests:
   - TestImportImplsDiscover — create temp dir with 2 IMPL docs, run discover
   - TestImportImplsFromFiles — specify explicit IMPL paths
   - TestImportImplsP1Conflict — two IMPLs with overlapping files, verify warning

3. **cmd/saw/validate_program_cmd.go** — Add `--import-mode` flag:
   - Add bool flag: `cmd.Flags().BoolVar(&importMode, "import-mode", false, ...)`
   - When flag is set, after existing ValidateProgram call, also call
     `protocol.ValidateProgramImportMode(manifest, repoPath)`
   - Add `--repo-dir` flag for specifying repo root (default: cwd)
   - Merge import-mode errors into output

4. **cmd/saw/run_scout_cmd.go** — Add skip-Scout logic:
   - After slug generation and before launching Scout, check if
     IMPL doc already exists at implPath
   - If exists, parse it and check state: if SCOUT_COMPLETE, REVIEWED,
     or later state (WAVE_PENDING, WAVE_EXECUTING, COMPLETE), skip Scout
   - Print: "Skipping Scout: IMPL doc already exists with state <state>"
   - Return success (exit 0) with skip indication in output
   - This prevents accidental overwrite of reviewed designs

5. **cmd/saw/main.go** — Register new command:
   - Add `newImportImplsCmd()` to the rootCmd.AddCommand(...) block

**Verification gate:**
```bash
cd /Users/dayna.blackwell/code/scout-and-wave-go && go build ./cmd/saw/...
cd /Users/dayna.blackwell/code/scout-and-wave-go && go test ./cmd/saw/ -run "TestImportImpls"
cd /Users/dayna.blackwell/code/scout-and-wave-go && go vet ./cmd/saw/...
```

**Constraints:**
- Use cobra command pattern matching existing commands (see validate_program_cmd.go)
- Use protocol.ParseProgramManifest for reading program manifests
- Use protocol.protocol.Load for reading IMPL docs
- Import the ImportIMPLsResult and ImportedIMPL types from pkg/protocol/program_types.go
  (defined by Agent C in Wave 2)
- Output JSON to stdout for all commands (match existing patterns)
- The skip-Scout logic must be non-destructive: never delete or overwrite existing IMPL docs
- For --discover mode, search both docs/IMPL/ and docs/IMPL/complete/ directories



## Interface Contracts

### ValidateProgramImportMode

Extended validation for import mode. Checks that referenced IMPL docs
exist on disk, have valid states (reviewed/complete), and don't violate
P1 (file_ownership disjointness) or P2 (frozen contract redefinition).


```
func ValidateProgramImportMode(manifest *PROGRAMManifest, repoPath string) []ValidationError
// Checks performed:
// 1. For each IMPL with status reviewed/complete, verify IMPL-<slug>.yaml exists
//    at repoPath/docs/IMPL/IMPL-<slug>.yaml (or docs/IMPL/complete/)
// 2. Parse each existing IMPL doc, verify its state field matches status
// 3. Check P1: file_ownership across all IMPLs in same tier is disjoint
// 4. Check P2: no pre-existing IMPL redefines a frozen program contract

```

### ImportIMPLsResult

Result struct returned by the import-impls command.


```
type ImportIMPLsResult struct {
  ManifestPath    string              `json:"manifest_path"`
  ImplsImported   []ImportedIMPL      `json:"impls_imported"`
  ImplsDiscovered []string            `json:"impls_discovered,omitempty"`
  TierAssignments map[string]int      `json:"tier_assignments"`
  P1Conflicts     []string            `json:"p1_conflicts,omitempty"`
  P2Conflicts     []string            `json:"p2_conflicts,omitempty"`
  Created         bool                `json:"created"`
  Updated         bool                `json:"updated"`
}

type ImportedIMPL struct {
  Slug            string `json:"slug"`
  Title           string `json:"title"`
  Status          string `json:"status"`
  AssignedTier    int    `json:"assigned_tier"`
  AgentCount      int    `json:"agent_count"`
  WaveCount       int    `json:"wave_count"`
}

```

### PartitionIMPLsByStatus

Used in E28A tier execution. Partitions a tier's IMPLs into those
needing Scout (pending) vs those to validate-only (reviewed/complete).


```
// PartitionIMPLsByStatus splits a tier's IMPLs into two groups:
// - needsScout: IMPLs with status "pending" or "scouting"
// - preExisting: IMPLs with status "reviewed" or "complete"
func PartitionIMPLsByStatus(manifest *PROGRAMManifest, tierNumber int) (needsScout []ProgramIMPL, preExisting []ProgramIMPL)

```



## Quality Gates

Level: standard

- **build**: `cd /Users/dayna.blackwell/code/scout-and-wave-go && go build ./...` (required: true)
- **lint**: `cd /Users/dayna.blackwell/code/scout-and-wave-go && go vet ./...` (required: true)
- **test**: `cd /Users/dayna.blackwell/code/scout-and-wave-go && go test ./...` (required: true)

