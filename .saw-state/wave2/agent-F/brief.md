# Agent F Brief - Wave 2

**IMPL Doc:** /Users/dayna.blackwell/code/scout-and-wave-go/docs/IMPL/IMPL-impl-schema-validation.yaml

## Files Owned

- `pkg/protocol/validation.go`
- `cmd/saw/validate_cmd.go`


## Task

## What to Implement
Wire all schema validators into the existing validation pipeline.

### 1. Update pkg/protocol/validation.go
Add a call to `ValidateSchema(m)` in the `Validate()` function, after
the existing semantic checks. Add it near the end, after `validateMultiRepoConsistency`:

```go
errs = append(errs, ValidateSchema(m)...)
```

### 2. Update ValidateSchema in schema_validation.go
Modify `ValidateSchema` to call ALL sub-validators:
- validateNestedRequiredFields(m) (already called by Agent B)
- validateAllEnums(m) (from Agent C's schema_enum_validation.go)
- validateFilePaths(m) (already called by Agent B)
- validateCrossFieldConsistency(m) (from Agent E's schema_cross_field.go)

### 3. Update cmd/saw/validate_cmd.go
Add unknown key detection to the validate command. After loading the manifest,
read the raw YAML bytes and call DetectUnknownKeys:

```go
// Read raw YAML for unknown key detection
rawData, readErr := os.ReadFile(manifestPath)
if readErr == nil {
    ukErrs := protocol.DetectUnknownKeys(rawData)
    for _, uke := range ukErrs {
        errs = append(errs, validateError{
            Code:    uke.Code,
            Message: uke.Message,
            Field:   uke.Field,
        })
    }
}
```

Note: DetectUnknownKeys operates on raw bytes (not the parsed struct) so it
must be called in the CLI command, not in Validate(). This is intentional -
the Validate() function works on parsed structs, while unknown key detection
needs raw YAML.

## Interfaces to Implement
None new - modifying existing functions.

## Interfaces to Call
- ValidateSchema(m *IMPLManifest) []ValidationError (from schema_validation.go)
- validateAllEnums(m *IMPLManifest) []ValidationError (from schema_enum_validation.go)
- validateCrossFieldConsistency(m *IMPLManifest) []ValidationError (from schema_cross_field.go)
- DetectUnknownKeys(yamlData []byte) []ValidationError (from schema_unknown_keys.go)

## Tests to Write
No new test file - existing validation_test.go TestValidate_CompleteManifest
already exercises the Validate() pipeline. Verify that the new checks are
called by running the full test suite.

## Verification Gate
```bash
go build ./...
go vet ./...
go test ./pkg/protocol/... -count=1
go test ./cmd/saw/... -count=1
```

## Constraints
- Do NOT change signatures of existing functions
- Do NOT reorder existing validation calls in Validate()
- Append ValidateSchema call at the end of the existing chain
- The validate_cmd.go change must preserve the existing --fix and --solver flags
- Unknown key warnings should appear in the JSON output alongside other errors



## Interface Contracts

### ValidateSchema

Top-level schema validation entry point. Runs all structural checks
(nested required fields, enum enforcement, path format, unknown keys,
cross-field consistency). Called by Validate() in validation.go alongside
existing semantic checks. Returns warnings by default (SV01 prefix).


```
func ValidateSchema(m *IMPLManifest) []ValidationError

```

### validateNestedRequiredFields

Checks required fields on nested types: FileOwnership.File,
FileOwnership.Agent, FileOwnership.Wave > 0, Agent.ID, Agent.Task,
Wave.Number > 0, InterfaceContract.Name, InterfaceContract.Definition,
InterfaceContract.Location, QualityGate.Type, QualityGate.Command,
ScaffoldFile.FilePath.


```
func validateNestedRequiredFields(m *IMPLManifest) []ValidationError

```

### validateAllEnums

Validates ALL enum fields in the manifest, complementing existing
enum validators. Covers: FileOwnership.Action (new|modify|delete),
QualityGates.Level (quick|standard|full), ScaffoldFile.Status
(pending|committed), PreMortemRow.Likelihood (low|medium|high),
PreMortemRow.Impact (low|medium|high). Empty values are allowed
for backward compatibility.


```
func validateAllEnums(m *IMPLManifest) []ValidationError

```

### validateFilePaths

Validates file path format in FileOwnership and Agent.Files:
no leading slash, no ".." traversal, no null bytes, no backslashes.
Also validates ScaffoldFile.FilePath.


```
func validateFilePaths(m *IMPLManifest) []ValidationError

```

### DetectUnknownKeys

Detects unknown/typo YAML keys by unmarshaling raw YAML into
map[string]interface{} and comparing top-level and nested keys
against the known schema. Returns SV01_UNKNOWN_KEY warnings.
Operates on raw YAML bytes, not the parsed struct.


```
func DetectUnknownKeys(yamlData []byte) []ValidationError

```

### validateCrossFieldConsistency

Cross-field validation rules:
1. FileOwnership agent IDs must all appear in some Wave.Agents
2. FileOwnership wave numbers must match existing Wave.Number values
3. Agent.Files entries must all appear in FileOwnership
4. Completion report agent IDs must match Wave agent IDs
5. If verdict is NOT_SUITABLE, waves/file_ownership/interface_contracts should be empty


```
func validateCrossFieldConsistency(m *IMPLManifest) []ValidationError

```



## Quality Gates

Level: standard

- **build**: `go build ./...` (required: true)
- **lint**: `go vet ./...` (required: true)
- **test**: `go test ./pkg/protocol/... -count=1` (required: true)

