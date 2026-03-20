# Agent F Brief - Wave 2

**IMPL Doc:** /Users/dayna.blackwell/code/scout-and-wave-web/docs/IMPL/IMPL-protocol-engine-drift-fixes.yaml

## Files Owned

- `pkg/protocol/validation.go`


## Task

## Context
You are Agent F in Wave 2 of the protocol-engine-drift-fixes IMPL. Wave 1 (Agent C)
has already updated pkg/protocol/types.go and validation.go. Your task adds two
additional validators to validation.go:
1. feature_slug kebab-case format validation for IMPL manifests
2. impls_total exact equality validation for PROGRAM manifests

Repo: /Users/dayna.blackwell/code/scout-and-wave-go/

## What to Implement

### Fix 1: pkg/protocol/validation.go — feature_slug kebab-case validation

The feature_slug field is validated for existence (validateI4RequiredFields checks
it is non-empty) but its format is not checked. Add a format check in
validateI4RequiredFields (or a new helper) that enforces the kebab-case pattern:
- Only lowercase letters, digits, and hyphens
- Must not start or end with a hyphen
- Regex: `^[a-z0-9]+(-[a-z0-9]+)*$`

If the existing feature_slug check in validateI4RequiredFields is a simple
non-empty check, extend it:
```go
var featureSlugRegex = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

// In validateI4RequiredFields, after checking non-empty:
if !featureSlugRegex.MatchString(m.FeatureSlug) {
    errs = append(errs, ValidationError{
        Code:    "I4_INVALID_FORMAT",
        Message: fmt.Sprintf("feature_slug %q must be kebab-case (lowercase letters, digits, hyphens; no leading/trailing hyphens)", m.FeatureSlug),
        Field:   "feature_slug",
    })
}
```

### Fix 2: pkg/protocol/validate_program.go or validation.go — impls_total exact equality

Look for where impls_total is validated in the PROGRAM manifest validation. Check
pkg/protocol/validate_program.go if it exists, otherwise look in validation.go or
schema_validation.go.

Current behavior: only checks impls_total <= len(impls). Required behavior: check
impls_total == len(impls) exactly.

Find the relevant check and change it from a `<=` bound to strict `==`:
```go
if manifest.ImplsTotal != len(manifest.Impls) {
    // Return validation error: impls_total must equal len(impls)
}
```

Read the actual validation code before making this change to understand the exact
context and function signature.

## Tests
Add tests to the appropriate _test.go file:
- TestFeatureSlugKebabValidation: test valid slugs (pass) and invalid slugs (fail)
  - Valid: "my-feature", "tool-journaling", "abc123", "a"
  - Invalid: "MyFeature", "my_feature", "-starts-with-hyphen", "ends-with-hyphen-"
- TestImplsTotalExactMatch: test that impls_total must equal len(impls) exactly

## Verification Gate
1. go build ./pkg/protocol/...
2. go vet ./pkg/protocol/...
3. go test ./pkg/protocol/... -run TestFeatureSlug
4. go test ./pkg/protocol/... -run TestImplsTotal

## Constraints
- Only modify files in pkg/protocol/
- Do not change Validate() signature
- The featureSlugRegex variable should be package-level (alongside agentIDRegex)



## Interface Contracts

### newUpdateProgramStateCmd

New cobra.Command constructor for the update-program-state sawtools subcommand.
Reads the PROGRAM manifest, updates its state field to the given value, writes
the manifest back, and outputs JSON result. Follows the exact pattern of
newUpdateStatusCmd in update_status.go.


```
func newUpdateProgramStateCmd() *cobra.Command
// Flags: --state <string> (required)
// Args: cobra.ExactArgs(1) — manifest path
// Output JSON:
// {
//   "updated": bool,
//   "manifest_path": string,
//   "previous_state": string,
//   "new_state": string
// }
// Exit codes: 0 success, 1 update error, 2 parse error

```

### newUpdateProgramImplCmd

New cobra.Command constructor for the update-program-impl sawtools subcommand.
Reads the PROGRAM manifest, finds the impl entry by slug, updates its status
field, writes the manifest back, and outputs JSON result.


```
func newUpdateProgramImplCmd() *cobra.Command
// Flags: --impl <string> (required), --status <string> (required)
// Args: cobra.ExactArgs(1) — manifest path
// Output JSON:
// {
//   "updated": bool,
//   "manifest_path": string,
//   "impl_slug": string,
//   "previous_status": string,
//   "new_status": string
// }
// Exit codes: 0 success, 1 impl not found or update error, 2 parse error

```



## Quality Gates

Level: standard

- **build**: `go build ./...` (required: true)
- **lint**: `go vet ./...` (required: true)
- **test**: `go test ./...` (required: true)

