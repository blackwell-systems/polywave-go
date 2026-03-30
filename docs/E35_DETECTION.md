# E35 Detection: Same-Package Caller Validation

## Overview

E35 detection prevents build failures after wave merges by catching cases where an agent owns a function definition but does not own all call sites in the same package.

## Problem

**Real-world example from Wave 1 post-mortem**:

- Agent C owned `CreateProgramWorktrees()` in `pkg/protocol/worktree.go`
- Agent B owned call sites in `pkg/protocol/program_tier_prepare.go`
- Both files in same package: `github.com/blackwell-systems/scout-and-wave-go/pkg/protocol`
- Agent C added a parameter to `CreateProgramWorktrees(programID, tier)`
- After merge: Build failed because Agent B's call sites still used old signature `CreateProgramWorktrees(programID)`

## Solution

Run `sawtools pre-wave-validate` before creating worktrees:

```bash
sawtools pre-wave-validate docs/IMPL/IMPL-feature.yaml --wave 1
```

This command:
1. Runs E16 validation (invariants, gates, contracts)
2. Runs E35 gap detection (same-package caller analysis)
3. Reports violations before wave execution

## How It Works

### Detection Algorithm

1. **Build ownership map**: For target wave, map each agent → owned files
2. **Extract functions**: Parse each Go file with `go/parser` and `go/ast`
   - Find all function declarations (`*ast.FuncDecl`)
   - Record package name and directory
3. **Find same-package files**: For each function's package, glob `*.go` files
4. **Detect calls**: Parse each same-package file, find `CallExpr` AST nodes
5. **Report gaps**: If function called from file not owned by defining agent

### What Gets Detected

✓ **Exported functions**: `func CreateWorktrees()`
✓ **Unexported functions**: `func helperFunc()`
✓ **Methods**: `func (r *Receiver) Method()`
✓ **Multiple call sites**: Reports all locations with line numbers

✗ **Cross-package calls**: Not E35 violations (handled by type system)
✗ **Test files**: Skipped when looking for violations

## Output Format

### Success (No Gaps)

```json
{
  "validation": {
    "valid": true,
    "error_count": 0,
    "errors": []
  },
  "e35_gaps": {
    "passed": true,
    "gaps": []
  }
}
```

### Failure (Gaps Found)

```json
{
  "validation": {
    "valid": true,
    "error_count": 0,
    "errors": []
  },
  "e35_gaps": {
    "passed": false,
    "gaps": [
      {
        "agent": "C",
        "function_name": "CreateProgramWorktrees",
        "defined_in": "pkg/protocol/worktree.go",
        "called_from": [
          "pkg/protocol/program_tier_prepare.go:45",
          "pkg/protocol/program_tier_prepare.go:82"
        ],
        "package": "github.com/blackwell-systems/scout-and-wave-go/pkg/protocol"
      }
    ]
  }
}
```

## Remediation

When E35 gaps are found, you have 3 options:

### Option 1: Reassign Files

**Best for**: Small number of call sites

```yaml
file_ownership:
  - file: pkg/protocol/worktree.go
    agent: C
    wave: 1
  - file: pkg/protocol/program_tier_prepare.go  # Move to Agent C
    agent: C  # Changed from B
    wave: 1
```

**Trade-off**: Agent C's scope grows; may need to redistribute other files

### Option 2: Add Wiring Declaration

**Best for**: Call sites that can be added later

```yaml
wiring:
  - agent: B
    wave: 2  # Integration wave after Wave 1
    symbol: CreateProgramWorktrees
    defined_in: pkg/protocol/worktree.go
    must_be_called_from: pkg/protocol/program_tier_prepare.go
    integration_pattern: direct_call
```

**Trade-off**: Requires Wave 2 integration step; function must be stable

### Option 3: Defer Function to Later Wave

**Best for**: Functions that depend on other work

Move the function definition to Wave 2 so both definition and call sites are in place:

```yaml
waves:
  - number: 1
    agents:
      - id: A
        task: "Create caller functions (stub out CreateProgramWorktrees)"
        files: [pkg/protocol/program_tier_prepare.go]
  - number: 2
    agents:
      - id: B
        task: "Implement CreateProgramWorktrees and wire into callers"
        files:
          - pkg/protocol/worktree.go
          - pkg/protocol/program_tier_prepare.go  # Can modify now
```

**Trade-off**: More waves = longer execution time

## Integration into Wave Pipeline

Add to pre-wave checks:

```bash
# Before wave execution
sawtools pre-wave-validate docs/IMPL/IMPL-feature.yaml --wave 1 || exit 1
sawtools create-worktrees docs/IMPL/IMPL-feature.yaml --wave 1
sawtools run-wave docs/IMPL/IMPL-feature.yaml --wave 1
```

## Command Reference

```
sawtools pre-wave-validate <manifest-path> [flags]

Flags:
  --wave int   Wave number to validate (required)
  --fix        Auto-correct fixable validation issues (optional)

Exit Codes:
  0 - Both E16 validation and E35 detection passed
  1 - Either validation or E35 detection failed
```

## Technical Details

### AST Parsing

Uses Go's standard library:
- `go/parser` - Parse Go source files
- `go/ast` - Traverse abstract syntax tree
- `go/token` - Track source positions for line numbers

### Performance

- **Lazy parsing**: Only parse files that exist on disk
- **Smart filtering**: Skip test files, non-Go files
- **Single pass**: Each file parsed once per detection run

### Limitations

1. **Dynamic calls**: Cannot detect calls via reflection or function pointers
2. **Build tags**: Does not respect `// +build` tags
3. **Generated code**: Treats generated files as regular source

## Examples

### Example 1: Simple Gap

**Manifest**:
```yaml
file_ownership:
  - file: pkg/auth/handler.go
    agent: A
    wave: 1
  - file: pkg/auth/middleware.go
    agent: B
    wave: 1
```

**Code**:
```go
// pkg/auth/handler.go (Agent A)
package auth
func ValidateToken(token string) bool { return true }

// pkg/auth/middleware.go (Agent B)
package auth
func AuthMiddleware() {
    if ValidateToken(req.Token) { ... }  // ← E35 violation
}
```

**Gap Reported**:
```json
{
  "agent": "A",
  "function_name": "ValidateToken",
  "defined_in": "pkg/auth/handler.go",
  "called_from": ["pkg/auth/middleware.go:3"],
  "package": "github.com/user/repo/pkg/auth"
}
```

### Example 2: No Gap (Cross-Package)

**Code**:
```go
// pkg/utils/helper.go (Agent A)
package utils
func Format(s string) string { return s }

// pkg/api/handler.go (Agent B)
package api
import "pkg/utils"
func Handler() {
    utils.Format("test")  // ← NOT an E35 violation (cross-package)
}
```

**Result**: No gap reported (different packages)

### Example 3: No Gap (Same Agent)

**Code**:
```go
// pkg/db/query.go (Agent A)
package db
func buildQuery() string { return "" }
func ExecuteQuery() { buildQuery() }  // ← OK (same agent)
```

**Result**: No gap reported (Agent A owns both files)

## See Also

- [Wave Execution Pipeline](./WAVE_EXECUTION.md)
- [E16 Validation Rules](./VALIDATION.md)
- [Wiring Declarations](./WIRING.md)
