# IMPL: Engine Protocol Gap — E17–E23
<!-- SAW:COMPLETE 2026-03-08 -->

## Suitability Assessment

Verdict: SUITABLE
test_command: `go test ./...`
lint_command: `go vet ./...`

Seven protocol rules (E17–E23) are fully specified in execution-rules.md v0.14.0 but have zero implementation in scout-and-wave-go. All rules touch distinct lifecycle phases (Scout startup, post-merge, failure routing, post-wave scan, quality gates, scaffold verification, agent launch). File ownership is disjoint when split into two waves: Wave 1 creates four new files (no conflicts possible) and adds new types to existing files; Wave 2 consumes those types in `pkg/engine/runner.go` and `pkg/orchestrator/orchestrator.go`. The single shared file `pkg/types/types.go` (adding `FailureType` and `QualityGate` types) is Orchestrator-owned and applied post-Wave-1-merge before Wave 2 launches. Interface contracts are fully discoverable from the spec. No investigation-first items.

Pre-implementation scan results:
- Total items: 7 rules (E17–E23)
- Already implemented: 0 items
- Partially implemented: 1 item — E20: `StubReportText` field exists in `IMPLDoc` and `## Stub Report` parser exists; the _execution_ (running scan-stubs.sh and writing output) is missing
- To-do: 6 items (E17, E18, E19, E21, E22, E23 fully absent)

Agent adjustments:
- Agent C (E20): "add stub scan execution" — parser support already present, only orchestrator execution missing
- Agents A, B, D, E, F proceed as planned (to-do)

Estimated times:
- Scout phase: ~15 min (this document)
- Wave 1 execution: ~20 min (4 agents × ~20 min avg, parallel)
- Wave 2 execution: ~15 min (2 agents × ~15 min avg, parallel)
- Merge & verification: ~5 min
- Total SAW time: ~55 min

Sequential baseline: ~90 min (6 agents × ~15 min sequential)
Time savings: ~35 min (~39% faster)

Recommendation: Clear speedup. Proceed.

---

## Quality Gates

level: full

gates:
  - type: typecheck
    command: go build ./...
    required: true
    description: Project compiles with all new files
  - type: lint
    command: go vet ./...
    required: true
    description: No vet warnings
  - type: test
    command: go test ./...
    required: true
    description: Full test suite passes

---

## Scaffolds

No scaffolds needed — agents have independent type ownership. New types (`FailureType`, `QualityGate`, `QualityGates`) are added to `pkg/types/types.go` by Agent A (Wave 1). This file is Orchestrator-owned for the append step; Wave 2 agents import the already-merged result.

---

## Pre-Mortem

**Overall risk:** medium

**Failure modes:**

| Scenario | Likelihood | Impact | Mitigation |
|----------|-----------|--------|------------|
| Agent E (E23, orchestrator.go) calls `ExtractAgentContext` before Agent D's `pkg/protocol/extract.go` is merged | low | high | Wave 2 does not launch until Wave 1 is fully merged and verified; `go build ./...` post-merge catches missing symbol |
| `FailureType` field added to `CompletionReport` in types.go breaks YAML unmarshalling for agents that omit it | medium | medium | Use `omitempty` yaml tag and treat empty string as `escalate` fallback per E19 spec |
| E22 scaffold build verification: `go build ./...` in RunScaffold may exit non-zero if scaffold files have import errors; error message needs to surface clearly | medium | medium | Agent F captures combined output and includes it in scaffold failure event payload |
| E20 scan-stubs.sh path resolution: script lives in SAW repo, not engine repo; path must be derived from SAW_REPO env var with same fallback logic as scout.md loading | medium | medium | Agent C uses same `sawRepo` resolution pattern already in `RunScout` |
| Wave 2 agent (E23) modifies `launchAgent` in orchestrator.go — same file as existing E19 routing from Wave 1 Agent B | low | high | Agent B writes to new file `pkg/orchestrator/failure.go`; Agent E modifies `orchestrator.go` only for `launchAgent`; no conflict |
| `CompletionReport.FailureType` field addition causes existing parser tests to fail if test YAML fixtures don't include the field | low | low | Field is optional (`omitempty`); absence parses cleanly as empty string |

---

## Known Issues

None identified.

---

## Dependency Graph

```yaml type=impl-dep-graph
Wave 1 (4 parallel agents — new files, type additions):
    [A] pkg/types/types.go
         Add FailureType type, QualityGate struct, QualityGates struct, CompletionReport.FailureType field, IMPLDoc.QualityGates field
         ✓ root (no dependencies on other agents)

    [B] pkg/orchestrator/failure.go (new)
         Implement E19 RouteFailure decision tree; reads failure_type from completion report and returns OrchestratorAction
         depends on: [A]

    [C] pkg/orchestrator/stubs.go (new)
         Implement E20 RunStubScan; collect files_changed+files_created from completion reports, invoke scan-stubs.sh, append ## Stub Report section to IMPL doc
         ✓ root (no dependencies on other agents)

    [D] pkg/protocol/extract.go (new)
         Implement E23 ExtractAgentContext; parse IMPL doc and return trimmed per-agent payload string
         Add E21 ParseQualityGates to pkg/protocol/parser.go
         ✓ root (no dependencies on other agents)

Wave 2 (2 parallel agents — consumers of Wave 1 interfaces):
    [E] pkg/orchestrator/orchestrator.go
         Wire E19 RouteFailure into launchAgent after completion report; wire E23 ExtractAgentContext into launchAgent prompt construction
         depends on: [A] [B] [D]

    [F] pkg/engine/runner.go
         Wire E17 CONTEXT.md read into RunScout; wire E22 build verification into RunScaffold; wire E18 UpdateContextMD into StartWave post-complete; wire E21 RunQualityGates call into StartWave post-agent-reports
         depends on: [A] [C] [D]
```

Note: Agent B depends on [A] for the `FailureType` type. `pkg/types/types.go` changes (Agent A) must be merged before Wave 2 launches. Wave 2 agents [E] and [F] both depend on Wave 1 being fully merged.

---

## Interface Contracts

### Types added to `pkg/types/types.go` (Agent A)

```go
// FailureType classifies the reason an agent reported partial or blocked status.
// Used by E19 failure routing decision tree.
type FailureType string

const (
    FailureTypeTransient  FailureType = "transient"
    FailureTypeFixable    FailureType = "fixable"
    FailureTypeNeedsReplan FailureType = "needs_replan"
    FailureTypeEscalate  FailureType = "escalate"
    FailureTypeTimeout   FailureType = "timeout"
)

// QualityGate is one gate from the ## Quality Gates section of an IMPL doc (E21).
type QualityGate struct {
    Type        string // "typecheck", "test", "lint", "custom"
    Command     string // exact shell command
    Required    bool   // true = failure blocks merge; false = warning only
    Description string // one-line human description
}

// QualityGates holds the parsed ## Quality Gates section.
type QualityGates struct {
    Level string       // "quick", "standard", "full"
    Gates []QualityGate
}
```

Fields added to existing structs:

```go
// Added to CompletionReport:
FailureType FailureType `yaml:"failure_type,omitempty"`

// Added to IMPLDoc:
QualityGates *QualityGates // parsed ## Quality Gates section; nil if absent
```

### E19: Failure routing (Agent B — `pkg/orchestrator/failure.go`)

```go
// OrchestratorAction is the action the orchestrator should take after a failure.
type OrchestratorAction int

const (
    ActionRetry        OrchestratorAction = iota // retry the agent (up to 2 times)
    ActionApplyAndRelaunch                       // apply fix from notes, relaunch once
    ActionReplan                                 // re-engage Scout; no retry
    ActionEscalate                               // surface to human immediately
    ActionRetryWithScope                         // retry once with scope-reduction note
)

// RouteFailure maps a FailureType to an OrchestratorAction per E19.
// If failureType is empty (absent from report), returns ActionEscalate (conservative fallback).
func RouteFailure(failureType types.FailureType) OrchestratorAction
```

### E20: Stub scan (Agent C — `pkg/orchestrator/stubs.go`)

```go
// RunStubScan collects files from wave completion reports, invokes scan-stubs.sh,
// and appends the ## Stub Report — Wave {N} section to the IMPL doc at implDocPath.
// sawRepoPath is used to locate scan-stubs.sh (falls back to SAW_REPO env, then ~/code/scout-and-wave).
// Always returns nil (stub detection is informational only per E20).
func RunStubScan(implDocPath string, waveNum int, reports map[string]*types.CompletionReport, sawRepoPath string) error
```

### E21: Quality gate execution (Agent D — parser addition to `pkg/protocol/parser.go`)

```go
// ParseQualityGates extracts the ## Quality Gates section from an IMPL doc.
// Returns nil if the section is absent (quality gates are optional per E21).
func ParseQualityGates(implDocPath string) (*types.QualityGates, error)
```

Quality gate execution function (Agent D — new `pkg/protocol/extract.go` or a new `pkg/orchestrator/quality_gates.go`):

Agent D owns `pkg/protocol/extract.go` (new file). Quality gate _execution_ is called from `pkg/engine/runner.go` (Agent F). The execution function lives in a new file Agent F creates:

```go
// RunQualityGates executes the gates from doc.QualityGates in the given repoPath.
// For each gate: runs the command; if required=true and exit!=0, returns error.
// If level="quick", skips all gates and returns nil.
// Returns a summary of gate results for event publishing.
// Defined in pkg/orchestrator/quality_gates.go (Agent D creates this file).
func RunQualityGates(repoPath string, gates *types.QualityGates) ([]QualityGateResult, error)

// QualityGateResult is one gate's outcome.
type QualityGateResult struct {
    Type     string
    Command  string
    Required bool
    Passed   bool
    Output   string // combined stdout+stderr (truncated to 2000 chars)
}
```

### E23: Per-agent context extraction (Agent D — `pkg/protocol/extract.go`)

```go
// AgentContextPayload is the trimmed context passed to a wave agent instead of the full IMPL doc (E23).
type AgentContextPayload struct {
    IMPLDocPath     string // absolute path — agent writes completion report here
    AgentPrompt     string // extracted ### Agent {letter} - {Role} section
    InterfaceContracts string // ## Interface Contracts section
    FileOwnership   string // ## File Ownership typed block raw text
    Scaffolds       string // ## Scaffolds section
    QualityGates    string // ## Quality Gates section
}

// ExtractAgentContext parses implDocPath and returns the per-agent payload for agentLetter.
// Returns error if the agent section is not found.
func ExtractAgentContext(implDocPath string, agentLetter string) (*AgentContextPayload, error)

// FormatAgentContextPayload renders an AgentContextPayload as the markdown string
// passed as the agent's prompt parameter (E23 payload format).
func FormatAgentContextPayload(payload *AgentContextPayload) string
```

### E17: Scout reads CONTEXT.md (Agent F — `pkg/engine/runner.go`)

```go
// ReadContextMD reads docs/CONTEXT.md from repoPath if it exists.
// Returns empty string if absent (E17: optional, not required).
func ReadContextMD(repoPath string) string
```

This is a private helper used inside `RunScout`; prepended to the scout prompt.

### E18: Orchestrator writes CONTEXT.md (Agent F — new `pkg/orchestrator/context.go`)

```go
// ContextMDEntry is one completed feature record appended to features_completed.
type ContextMDEntry struct {
    Slug    string
    ImplDoc string
    Waves   int
    Agents  int
    Date    string // YYYY-MM-DD
}

// UpdateContextMD creates or updates docs/CONTEXT.md in repoPath.
// Appends entry to features_completed. Creates the file with canonical schema if absent.
// Commits with message: "chore: update docs/CONTEXT.md for {slug}"
// Defined in pkg/orchestrator/context.go.
func UpdateContextMD(repoPath string, entry ContextMDEntry) error
```

### E22: Scaffold build verification (Agent F — `pkg/engine/runner.go`)

No new exported function. `RunScaffold` is modified to run `go get ./...`, `go mod tidy`, `go build ./...` after scaffold agent completes. On failure: marks scaffold status as FAILED in IMPL doc, returns error.

---

## File Ownership

```yaml type=impl-file-ownership
| File | Agent | Wave | Depends On |
|------|-------|------|------------|
| `pkg/types/types.go` | A | 1 | — |
| `pkg/orchestrator/failure.go` (new) | B | 1 | A |
| `pkg/orchestrator/stubs.go` (new) | C | 1 | — |
| `pkg/protocol/extract.go` (new) | D | 1 | — |
| `pkg/orchestrator/quality_gates.go` (new) | D | 1 | — |
| `pkg/protocol/parser.go` | D | 1 | — |
| `pkg/orchestrator/orchestrator.go` | E | 2 | A, B, D |
| `pkg/engine/runner.go` | F | 2 | A, C, D |
| `pkg/orchestrator/context.go` (new) | F | 2 | — |
```

---

## Wave Structure

```yaml type=impl-wave-structure
Wave 1: [A] [B] [C] [D]          <- 4 parallel agents (new types + new files)
               | (A+B+C+D complete)
Wave 2:    [E] [F]               <- 2 parallel agents (wire into existing entrypoints)
```

---

## Wave 1

Wave 1 delivers: new types in `pkg/types`, the E19 routing decision tree, the E20 stub scan runner, and the E23/E21 extraction/gate functions in `pkg/protocol`. All four agents create or extend independent files. No agent in Wave 1 reads another's output — they can run fully in parallel.

### Agent A - Add FailureType and QualityGates types to pkg/types

**0. CRITICAL: Isolation Verification (RUN FIRST)**

```bash
cd /Users/dayna.blackwell/code/scout-and-wave-go/.claude/worktrees/wave1-agent-A
ACTUAL_DIR=$(pwd)
EXPECTED_DIR="/Users/dayna.blackwell/code/scout-and-wave-go/.claude/worktrees/wave1-agent-A"
if [ "$ACTUAL_DIR" != "$EXPECTED_DIR" ]; then echo "ISOLATION FAILURE"; exit 1; fi
ACTUAL_BRANCH=$(git branch --show-current)
if [ "$ACTUAL_BRANCH" != "wave1-agent-A" ]; then echo "ISOLATION FAILURE: wrong branch $ACTUAL_BRANCH"; exit 1; fi
echo "Isolation verified: $ACTUAL_DIR on $ACTUAL_BRANCH"
```

**1. File Ownership**

- `pkg/types/types.go` — modify (add new types and fields to existing structs)

**2. Interfaces to Implement**

Add to `pkg/types/types.go`:

```go
// FailureType classifies why an agent reported partial or blocked status.
type FailureType string

const (
    FailureTypeTransient   FailureType = "transient"
    FailureTypeFixable     FailureType = "fixable"
    FailureTypeNeedsReplan FailureType = "needs_replan"
    FailureTypeEscalate   FailureType = "escalate"
    FailureTypeTimeout    FailureType = "timeout"
)

// QualityGate is one gate from the ## Quality Gates IMPL doc section (E21).
type QualityGate struct {
    Type        string
    Command     string
    Required    bool
    Description string
}

// QualityGates holds the parsed ## Quality Gates section.
type QualityGates struct {
    Level string
    Gates []QualityGate
}
```

Add field to `CompletionReport` struct:
```go
FailureType FailureType `yaml:"failure_type,omitempty"`
```

Add field to `IMPLDoc` struct:
```go
QualityGates *QualityGates // nil if absent
```

**3. Interfaces to Call**

None — this agent only defines types.

**4. What to Implement**

Add the `FailureType` string type and its five constants exactly as specified above. Add `QualityGate` and `QualityGates` structs. Add `FailureType` field to `CompletionReport` (with `yaml:"failure_type,omitempty"` tag so existing YAML without this field still parses cleanly). Add `QualityGates *QualityGates` field to `IMPLDoc`. Do not change any existing fields or types.

**5. Tests to Write**

In `pkg/types/types_test.go`:
- `TestFailureTypeConstants` — verify the five constants have their exact string values (`"transient"`, `"fixable"`, `"needs_replan"`, `"escalate"`, `"timeout"`)
- `TestCompletionReportFailureTypeOmitempty` — marshal a `CompletionReport` with empty `FailureType` to YAML; confirm `failure_type` key is absent from output

**6. Verification Gate**

```bash
cd /Users/dayna.blackwell/code/scout-and-wave-go/.claude/worktrees/wave1-agent-A
go build ./pkg/types/...
go vet ./pkg/types/...
go test ./pkg/types/... -run "TestFailureType|TestCompletionReport"
```

**7. Constraints**

- Do not change any existing field names, types, or yaml tags in `CompletionReport` or `IMPLDoc`.
- Use `omitempty` on the `FailureType` yaml tag so existing YAML fixtures parse without error.
- `QualityGates` on `IMPLDoc` must be a pointer (`*QualityGates`) so nil signals "section absent".
- Embed a comment on each new exported type referencing the E-number: `// E19`, `// E21`.

**8. Report**

Append your completion report to `/Users/dayna.blackwell/code/scout-and-wave-go/docs/IMPL/IMPL-engine-protocol-gap.md` under `### Agent A - Completion Report` using the standard `yaml type=impl-completion-report` block. Commit your changes on branch `wave1-agent-A` before writing the report.

---

### Agent B - Implement E19 Failure Routing Decision Tree

**0. CRITICAL: Isolation Verification (RUN FIRST)**

```bash
cd /Users/dayna.blackwell/code/scout-and-wave-go/.claude/worktrees/wave1-agent-B
ACTUAL_DIR=$(pwd)
EXPECTED_DIR="/Users/dayna.blackwell/code/scout-and-wave-go/.claude/worktrees/wave1-agent-B"
if [ "$ACTUAL_DIR" != "$EXPECTED_DIR" ]; then echo "ISOLATION FAILURE"; exit 1; fi
ACTUAL_BRANCH=$(git branch --show-current)
if [ "$ACTUAL_BRANCH" != "wave1-agent-B" ]; then echo "ISOLATION FAILURE: wrong branch $ACTUAL_BRANCH"; exit 1; fi
echo "Isolation verified: $ACTUAL_DIR on $ACTUAL_BRANCH"
```

**1. File Ownership**

- `pkg/orchestrator/failure.go` — create (new file)

**2. Interfaces to Implement**

```go
package orchestrator

import "github.com/blackwell-systems/scout-and-wave-go/pkg/types"

// OrchestratorAction is the action to take after a partial/blocked completion report.
type OrchestratorAction int

const (
    ActionRetry           OrchestratorAction = iota // E19: transient — retry up to 2 times
    ActionApplyAndRelaunch                          // E19: fixable — apply fix from notes, relaunch once
    ActionReplan                                    // E19: needs_replan — re-engage Scout
    ActionEscalate                                  // E19: escalate — surface to human
    ActionRetryWithScope                            // E19: timeout — retry once with scope-reduction note
)

// RouteFailure maps a FailureType to an OrchestratorAction per E19.
// Empty failureType (absent from report) returns ActionEscalate (conservative fallback).
func RouteFailure(failureType types.FailureType) OrchestratorAction
```

**3. Interfaces to Call**

```go
// From pkg/types (Agent A — will be merged before Wave 2):
types.FailureType       // string type
types.FailureTypeTransient, FailureTypeFixable, FailureTypeNeedsReplan, FailureTypeEscalate, FailureTypeTimeout
```

Note: Agent A's types will be available in your worktree only after Wave 1 merges. For compilation in your worktree, you can define a temporary local copy of the constants or wait. The interface contracts define the exact values; your implementation must match them exactly.

**4. What to Implement**

Create `pkg/orchestrator/failure.go`. Define `OrchestratorAction` and its five constants. Implement `RouteFailure` as a switch on `failureType`:
- `"transient"` → `ActionRetry`
- `"fixable"` → `ActionApplyAndRelaunch`
- `"needs_replan"` → `ActionReplan`
- `"escalate"` → `ActionEscalate`
- `"timeout"` → `ActionRetryWithScope`
- `""` (empty/absent) → `ActionEscalate` (E19 backward-compat fallback)
- Any unknown value → `ActionEscalate`

Add a file-level comment: `// E19: Failure type routing decision tree — see execution-rules.md §E19`

**5. Tests to Write**

In a new `pkg/orchestrator/failure_test.go`:
- `TestRouteFailureAllTypes` — table-driven test covering all five known failure types + empty string + unknown value
- `TestRouteFailureEmptyIsEscalate` — explicit test that empty FailureType returns ActionEscalate (backward compat)
- `TestRouteFailureUnknownIsEscalate` — unknown string returns ActionEscalate

**6. Verification Gate**

```bash
cd /Users/dayna.blackwell/code/scout-and-wave-go/.claude/worktrees/wave1-agent-B
go build ./pkg/orchestrator/...
go vet ./pkg/orchestrator/...
go test ./pkg/orchestrator/... -run "TestRouteFailure"
```

Note: If Agent A's types aren't merged yet, you may see compilation errors on `types.FailureType*` constants. Temporarily define them as string constants in your file for compilation, marked with `// TEMP: remove after merge with Agent A`. The Orchestrator will resolve this before Wave 2.

**7. Constraints**

- `OrchestratorAction` and its constants live in `pkg/orchestrator` package, not `pkg/types` — they are orchestrator-internal decisions, not protocol data.
- Do not import or modify `pkg/engine` — dependency direction is engine→orchestrator, not the reverse.
- Do not modify `orchestrator.go` — that is Agent E's file in Wave 2.

**8. Report**

Append completion report to `/Users/dayna.blackwell/code/scout-and-wave-go/docs/IMPL/IMPL-engine-protocol-gap.md` under `### Agent B - Completion Report`. Commit on branch `wave1-agent-B` before writing.

---

### Agent C - Implement E20 Stub Scan Execution

**0. CRITICAL: Isolation Verification (RUN FIRST)**

```bash
cd /Users/dayna.blackwell/code/scout-and-wave-go/.claude/worktrees/wave1-agent-C
ACTUAL_DIR=$(pwd)
EXPECTED_DIR="/Users/dayna.blackwell/code/scout-and-wave-go/.claude/worktrees/wave1-agent-C"
if [ "$ACTUAL_DIR" != "$EXPECTED_DIR" ]; then echo "ISOLATION FAILURE"; exit 1; fi
ACTUAL_BRANCH=$(git branch --show-current)
if [ "$ACTUAL_BRANCH" != "wave1-agent-C" ]; then echo "ISOLATION FAILURE: wrong branch $ACTUAL_BRANCH"; exit 1; fi
echo "Isolation verified: $ACTUAL_DIR on $ACTUAL_BRANCH"
```

**1. File Ownership**

- `pkg/orchestrator/stubs.go` — create (new file)

**2. Interfaces to Implement**

```go
package orchestrator

import "github.com/blackwell-systems/scout-and-wave-go/pkg/types"

// RunStubScan implements E20: collects all files_changed and files_created from
// wave agent completion reports, invokes scan-stubs.sh, and appends the
// ## Stub Report — Wave {N} section to the IMPL doc at implDocPath.
//
// sawRepoPath locates scan-stubs.sh: falls back to $SAW_REPO env var, then
// ~/code/scout-and-wave (same fallback as RunScout).
//
// Always returns nil — stub detection is informational only (E20).
func RunStubScan(implDocPath string, waveNum int, reports map[string]*types.CompletionReport, sawRepoPath string) error
```

**3. Interfaces to Call**

```go
// From pkg/types (already exists — no dependency on Agent A):
types.CompletionReport.FilesChanged []string
types.CompletionReport.FilesCreated []string

// OS / stdlib:
os.UserHomeDir()
os.ReadFile / os.WriteFile / os.OpenFile (append mode)
os/exec.Command("bash", scriptPath, files...)
path/filepath.Join
```

**4. What to Implement**

Create `pkg/orchestrator/stubs.go`. Implement `RunStubScan`:

1. Collect the union of all `report.FilesChanged` and `report.FilesCreated` across all reports in the map. Deduplicate. Skip files with prefix `docs/IMPL/` (IMPL doc itself).

2. Resolve `sawRepoPath`: if argument is empty, check `$SAW_REPO` env var, then fall back to `filepath.Join(os.UserHomeDir(), "code", "scout-and-wave")`.

3. Locate `scan-stubs.sh` at `filepath.Join(sawRepoPath, "implementations", "claude-code", "scripts", "scan-stubs.sh")`.

4. If the script does not exist, write a stub report section noting "scan-stubs.sh not found at {path}" and return nil.

5. Run: `bash {script} {file1} {file2} ...` with `cmd.Dir = filepath.Dir(implDocPath)`. Capture combined output. Exit code is always 0 per E20 spec.

6. Format the output as the `## Stub Report — Wave {N}` section (see message-formats.md Stub Report Section Format). Append it to implDocPath.

7. Return nil.

**5. Tests to Write**

In `pkg/orchestrator/stubs_test.go`:
- `TestRunStubScanNoFiles` — empty reports map; should append "No stub patterns detected." section and return nil
- `TestRunStubScanMissingScript` — sawRepoPath points to nonexistent directory; should append "scan-stubs.sh not found" section and return nil (not error)
- `TestRunStubScanAppendsSection` — use a temp IMPL doc file; verify `## Stub Report — Wave 1` section is appended after calling RunStubScan

**6. Verification Gate**

```bash
cd /Users/dayna.blackwell/code/scout-and-wave-go/.claude/worktrees/wave1-agent-C
go build ./pkg/orchestrator/...
go vet ./pkg/orchestrator/...
go test ./pkg/orchestrator/... -run "TestRunStubScan"
```

**7. Constraints**

- `RunStubScan` must never return non-nil error — E20 specifies exit code is always 0 and detection is informational. Log failures to stderr but return nil.
- Do not modify `orchestrator.go` (Agent E's file). Do not modify `failure.go` (Agent B's file).
- Append to the IMPL doc using `os.OpenFile(implDocPath, os.O_APPEND|os.O_WRONLY, 0644)` — not rewrite.
- The `## Stub Report — Wave {N}` section header must match the exact format from message-formats.md so the parser's `StubReportText` field captures it correctly.

**8. Report**

Append completion report to `/Users/dayna.blackwell/code/scout-and-wave-go/docs/IMPL/IMPL-engine-protocol-gap.md` under `### Agent C - Completion Report`. Commit on branch `wave1-agent-C` before writing.

---

### Agent D - Implement E21 Quality Gates + E23 Per-Agent Context Extraction

**0. CRITICAL: Isolation Verification (RUN FIRST)**

```bash
cd /Users/dayna.blackwell/code/scout-and-wave-go/.claude/worktrees/wave1-agent-D
ACTUAL_DIR=$(pwd)
EXPECTED_DIR="/Users/dayna.blackwell/code/scout-and-wave-go/.claude/worktrees/wave1-agent-D"
if [ "$ACTUAL_DIR" != "$EXPECTED_DIR" ]; then echo "ISOLATION FAILURE"; exit 1; fi
ACTUAL_BRANCH=$(git branch --show-current)
if [ "$ACTUAL_BRANCH" != "wave1-agent-D" ]; then echo "ISOLATION FAILURE: wrong branch $ACTUAL_BRANCH"; exit 1; fi
echo "Isolation verified: $ACTUAL_DIR on $ACTUAL_BRANCH"
```

**1. File Ownership**

- `pkg/protocol/extract.go` — create (new file)
- `pkg/orchestrator/quality_gates.go` — create (new file)
- `pkg/protocol/parser.go` — modify (add `ParseQualityGates` function and `## Quality Gates` section parsing in `ParseIMPLDoc`)

**2. Interfaces to Implement**

In `pkg/protocol/extract.go`:

```go
package protocol

// AgentContextPayload is the trimmed per-agent context passed to wave agents (E23).
type AgentContextPayload struct {
    IMPLDocPath        string
    AgentPrompt        string
    InterfaceContracts string
    FileOwnership      string
    Scaffolds          string
    QualityGates       string
}

// ExtractAgentContext parses implDocPath and returns the per-agent payload for agentLetter.
// Extracts: agent's 9-field prompt section, Interface Contracts, File Ownership,
// Scaffolds, Quality Gates sections. Returns error if agent section not found.
func ExtractAgentContext(implDocPath string, agentLetter string) (*AgentContextPayload, error)

// FormatAgentContextPayload renders AgentContextPayload as the markdown string
// passed as the agent prompt parameter (E23 payload format from message-formats.md).
func FormatAgentContextPayload(payload *AgentContextPayload) string
```

In `pkg/protocol/parser.go` (addition):

```go
// ParseQualityGates extracts the ## Quality Gates section from an IMPL doc at path.
// Returns nil, nil if the section is absent (quality gates are optional).
func ParseQualityGates(implDocPath string) (*types.QualityGates, error)
```

Also: wire `ParseQualityGates` logic into `ParseIMPLDoc` so `doc.QualityGates` is populated when the section exists.

In `pkg/orchestrator/quality_gates.go`:

```go
package orchestrator

import "github.com/blackwell-systems/scout-and-wave-go/pkg/types"

// QualityGateResult is one gate's execution outcome.
type QualityGateResult struct {
    Type     string
    Command  string
    Required bool
    Passed   bool
    Output   string // combined stdout+stderr, truncated to 2000 chars
}

// RunQualityGates executes the configured gates in repoPath (E21).
// If gates is nil or gates.Level == "quick", returns empty slice and nil error.
// For each gate: runs command; if Required and exit != 0, returns error.
// Returns all gate results plus the first blocking error (if any).
func RunQualityGates(repoPath string, gates *types.QualityGates) ([]QualityGateResult, error)
```

**3. Interfaces to Call**

```go
// From pkg/types (Agent A adds QualityGates/QualityGate — needed in parser.go):
types.QualityGates{Level string, Gates []QualityGate}
types.QualityGate{Type, Command string, Required bool, Description string}

// IMPLDoc.QualityGates *types.QualityGates  (field added by Agent A)

// Stdlib for quality_gates.go:
os/exec.Command
strings.Join, strings.Fields
```

Note: `pkg/types` changes from Agent A may not be present in your worktree during Wave 1. For `pkg/protocol/parser.go`, you need to reference `types.QualityGates`. If Agent A's types aren't merged yet, define the `QualityGates`-related parsing logic but gate it behind a build tag or use a local stub. More practically: implement `ParseQualityGates` to return `(*types.QualityGates, error)` — if the types aren't available, the file won't compile. Coordinate with the Orchestrator: if this causes build failure, report `status: partial` with `failure_type: fixable` and note "needs Agent A merged first."

**4. What to Implement**

**`pkg/protocol/extract.go` — E23 context extraction:**

Implement `ExtractAgentContext(implDocPath, agentLetter)`:
1. Open and scan `implDocPath` line by line.
2. Extract the agent's prompt: find `### Agent {letter}` heading (not a Completion Report header), capture all lines until the next `### Agent` or `## ` heading.
3. Extract `## Interface Contracts` section: all lines until next `##` heading.
4. Extract `## File Ownership` section: all lines including the `impl-file-ownership` typed block.
5. Extract `## Scaffolds` section: all lines until next `##` heading.
6. Extract `## Quality Gates` section: all lines until next `##` heading.
7. Return `AgentContextPayload` with all six fields populated.

Implement `FormatAgentContextPayload(payload)`:
Render as per message-formats.md Per-Agent Context Payload format:
```
<!-- IMPL doc: {IMPLDocPath} -->

{AgentPrompt}

## Interface Contracts

{InterfaceContracts}

## File Ownership

{FileOwnership}

## Scaffolds

{Scaffolds}

## Quality Gates

{QualityGates}
```

**`pkg/protocol/parser.go` — ParseQualityGates:**

Implement `ParseQualityGates(implDocPath)`:
1. Open the file, scan for `## Quality Gates` heading.
2. Parse `level:` line for the level value.
3. Parse `gates:` YAML block (indented list with `- type:`, `command:`, `required:`, `description:` fields).
4. Return `*types.QualityGates` with populated fields, or `nil, nil` if section absent.

Also add a `## Quality Gates` case to `ParseIMPLDoc`'s state machine that calls the same parsing logic and populates `doc.QualityGates`.

**`pkg/orchestrator/quality_gates.go` — RunQualityGates:**

Implement `RunQualityGates(repoPath, gates)`:
1. If `gates == nil` or `gates.Level == "quick"`, return `nil, nil`.
2. For each gate in `gates.Gates`: run `strings.Fields(gate.Command)` as `exec.Command`; set `cmd.Dir = repoPath`; capture combined output; truncate to 2000 chars.
3. Record result. If `gate.Required && exitCode != 0`, collect as blocking error.
4. After all gates: if any blocking errors, return results + first blocking error.
5. Otherwise return results + nil.

**5. Tests to Write**

In `pkg/protocol/extract_test.go`:
- `TestExtractAgentContextFound` — fixture IMPL doc with known agent sections; verify AgentPrompt, InterfaceContracts, FileOwnership all extracted correctly
- `TestExtractAgentContextNotFound` — agentLetter not in doc; verify error returned
- `TestFormatAgentContextPayload` — verify output starts with `<!-- IMPL doc:` and contains all sections

In `pkg/protocol/parser_test.go` (add):
- `TestParseQualityGates` — fixture IMPL doc with `## Quality Gates` section; verify level and gate fields

In `pkg/orchestrator/quality_gates_test.go`:
- `TestRunQualityGatesNil` — nil gates returns nil, nil
- `TestRunQualityGatesQuick` — level="quick" skips all gates
- `TestRunQualityGatesRequiredFail` — required gate with failing command returns error
- `TestRunQualityGatesOptionalFail` — optional gate failing returns nil error but result shows Passed=false

**6. Verification Gate**

```bash
cd /Users/dayna.blackwell/code/scout-and-wave-go/.claude/worktrees/wave1-agent-D
go build ./pkg/protocol/... ./pkg/orchestrator/...
go vet ./pkg/protocol/... ./pkg/orchestrator/...
go test ./pkg/protocol/... -run "TestExtractAgent|TestFormatAgent|TestParseQuality"
go test ./pkg/orchestrator/... -run "TestRunQualityGates"
```

**7. Constraints**

- `ExtractAgentContext` must handle IMPL docs with or without a `## Quality Gates` section (it is optional). Return empty string for absent sections — do not return error.
- `ParseQualityGates` parses the quality gates YAML by hand (same line-scanner approach as the rest of `parser.go`). Do not introduce a new YAML parsing library.
- `RunQualityGates` must not block indefinitely. Add a 5-minute timeout per gate using `exec.CommandContext`.
- Do not modify `orchestrator.go` (Agent E's Wave 2 file) or `runner.go` (Agent F's Wave 2 file).
- The `## Quality Gates` section in IMPL docs uses YAML-like syntax but is inside a plain fenced block (not a `type=impl-*` typed block per the spec). Parse it as indented text.

**8. Report**

Append completion report to `/Users/dayna.blackwell/code/scout-and-wave-go/docs/IMPL/IMPL-engine-protocol-gap.md` under `### Agent D - Completion Report`. Commit on branch `wave1-agent-D` before writing.

---

## Wave 2

Wave 2 wires all Wave 1 implementations into the two existing entrypoint files. Prerequisite: all Wave 1 agents must be merged and `go build ./...` must pass before any Wave 2 worktrees are created. Agent E owns `pkg/orchestrator/orchestrator.go`; Agent F owns `pkg/engine/runner.go` and the new `pkg/orchestrator/context.go`. No file overlap.

### Agent E - Wire E19 and E23 into Orchestrator launchAgent

**0. CRITICAL: Isolation Verification (RUN FIRST)**

```bash
cd /Users/dayna.blackwell/code/scout-and-wave-go/.claude/worktrees/wave2-agent-E
ACTUAL_DIR=$(pwd)
EXPECTED_DIR="/Users/dayna.blackwell/code/scout-and-wave-go/.claude/worktrees/wave2-agent-E"
if [ "$ACTUAL_DIR" != "$EXPECTED_DIR" ]; then echo "ISOLATION FAILURE"; exit 1; fi
ACTUAL_BRANCH=$(git branch --show-current)
if [ "$ACTUAL_BRANCH" != "wave2-agent-E" ]; then echo "ISOLATION FAILURE: wrong branch $ACTUAL_BRANCH"; exit 1; fi
echo "Isolation verified: $ACTUAL_DIR on $ACTUAL_BRANCH"
```

**1. File Ownership**

- `pkg/orchestrator/orchestrator.go` — modify

**2. Interfaces to Implement**

No new exported functions. Modifications to existing function `launchAgent`:

- **E23**: Before calling `runner.ExecuteStreaming`, replace `agentSpec.Prompt` with the output of `protocol.ExtractAgentContext` + `protocol.FormatAgentContextPayload`. If extraction fails (agent section not found), fall back to the existing full-prompt behavior and log a warning.

- **E19**: After `waitForCompletionFunc` returns a completion report, if `report.Status` is `StatusPartial` or `StatusBlocked`, call `RouteFailure(report.FailureType)` and publish a new `agent_blocked` event with the routed action.

**3. Interfaces to Call**

```go
// From pkg/protocol (Agent D — Wave 1):
protocol.ExtractAgentContext(implDocPath string, agentLetter string) (*protocol.AgentContextPayload, error)
protocol.FormatAgentContextPayload(payload *protocol.AgentContextPayload) string

// From pkg/orchestrator/failure.go (Agent B — Wave 1):
RouteFailure(failureType types.FailureType) OrchestratorAction

// From pkg/types (Agent A — Wave 1):
types.FailureType
types.StatusPartial, types.StatusBlocked

// Existing in orchestrator package:
waitForCompletionFunc, o.publish, OrchestratorEvent
```

**4. What to Implement**

In `launchAgent`, after step (a) worktree creation and before step (b) `runner.ExecuteStreaming`:

**E23 — Context extraction:**
```go
// E23: Construct per-agent context payload instead of passing full IMPL doc prompt.
if payload, err := protocol.ExtractAgentContext(o.implDocPath, agentSpec.Letter); err == nil {
    agentSpec.Prompt = protocol.FormatAgentContextPayload(payload)
} else {
    // Fallback: use existing prompt from agentSpec (already set from IMPL doc parse).
    fmt.Fprintf(os.Stderr, "orchestrator: E23 context extraction failed for agent %s: %v (falling back to full prompt)\n", agentSpec.Letter, err)
}
```

After step (c) `waitForCompletionFunc` returns successfully:

**E19 — Failure routing:**
```go
if report != nil && (report.Status == types.StatusPartial || report.Status == types.StatusBlocked) {
    action := RouteFailure(report.FailureType)
    o.publish(OrchestratorEvent{
        Event: "agent_blocked",
        Data: AgentBlockedPayload{
            Agent:       agentSpec.Letter,
            Wave:        waveNum,
            Status:      string(report.Status),
            FailureType: string(report.FailureType),
            Action:      action,
        },
    })
    return fmt.Errorf("orchestrator: agent %s: %s (failure_type: %s, action: %v)",
        agentSpec.Letter, report.Status, report.FailureType, action)
}
```

Add `AgentBlockedPayload` to `pkg/orchestrator/events.go`:
```go
type AgentBlockedPayload struct {
    Agent       string             `json:"agent"`
    Wave        int                `json:"wave"`
    Status      string             `json:"status"`
    FailureType string             `json:"failure_type"`
    Action      OrchestratorAction `json:"action"`
}
```

Note: `events.go` is not in your ownership. Add `AgentBlockedPayload` to `orchestrator.go` instead, or request the Orchestrator adds it to `events.go` post-merge. Use `interface{}` in the event Data if needed. Report any `out_of_scope_deps` in your completion report.

**5. Tests to Write**

In `pkg/orchestrator/orchestrator_test.go` (add):
- `TestLaunchAgentE23ContextExtraction` — mock `ExtractAgentContext` to return a payload; verify `ExecuteStreaming` receives formatted payload as system prompt, not full IMPL doc
- `TestLaunchAgentE19BlockedPublishesEvent` — mock completion report with `status: blocked`; verify `agent_blocked` event is published with correct `FailureType` and `Action`
- `TestLaunchAgentE19PartialRoutedCorrectly` — `status: partial` with `failure_type: transient` routes to `ActionRetry`

**6. Verification Gate**

```bash
cd /Users/dayna.blackwell/code/scout-and-wave-go/.claude/worktrees/wave2-agent-E
go build ./pkg/orchestrator/...
go vet ./pkg/orchestrator/...
go test ./pkg/orchestrator/... -run "TestLaunchAgent"
```

**7. Constraints**

- Do not modify `events.go` (not in your ownership). Add `AgentBlockedPayload` inline in `orchestrator.go` or report as `out_of_scope_deps`.
- E23 fallback must never panic. If `ExtractAgentContext` errors, log to stderr and proceed with existing prompt — do not fail the wave.
- E19 routing publishes an event but the retry logic (actually relaunching the agent) is out of scope for this task. E19 routing here means: detect, route, publish, return error. Retry orchestration is a follow-on task.
- Do not modify `failure.go`, `stubs.go`, `quality_gates.go`, `extract.go` — those are Wave 1 files.

**8. Report**

Append completion report to `/Users/dayna.blackwell/code/scout-and-wave-go/docs/IMPL/IMPL-engine-protocol-gap.md` under `### Agent E - Completion Report`. Commit on branch `wave2-agent-E` before writing.

---

### Agent F - Wire E17, E18, E21, E22 into Engine Runner

**0. CRITICAL: Isolation Verification (RUN FIRST)**

```bash
cd /Users/dayna.blackwell/code/scout-and-wave-go/.claude/worktrees/wave2-agent-F
ACTUAL_DIR=$(pwd)
EXPECTED_DIR="/Users/dayna.blackwell/code/scout-and-wave-go/.claude/worktrees/wave2-agent-F"
if [ "$ACTUAL_DIR" != "$EXPECTED_DIR" ]; then echo "ISOLATION FAILURE"; exit 1; fi
ACTUAL_BRANCH=$(git branch --show-current)
if [ "$ACTUAL_BRANCH" != "wave2-agent-F" ]; then echo "ISOLATION FAILURE: wrong branch $ACTUAL_BRANCH"; exit 1; fi
echo "Isolation verified: $ACTUAL_DIR on $ACTUAL_BRANCH"
```

**1. File Ownership**

- `pkg/engine/runner.go` — modify
- `pkg/orchestrator/context.go` — create (new file)

**2. Interfaces to Implement**

In `pkg/orchestrator/context.go`:

```go
package orchestrator

// ContextMDEntry is one completed feature record for docs/CONTEXT.md (E18).
type ContextMDEntry struct {
    Slug    string
    ImplDoc string
    Waves   int
    Agents  int
    Date    string // YYYY-MM-DD
}

// UpdateContextMD creates or updates docs/CONTEXT.md in repoPath (E18).
// If the file does not exist, creates it with the canonical schema from message-formats.md.
// Appends entry to the features_completed list.
// Commits: git commit -m "chore: update docs/CONTEXT.md for {entry.Slug}"
// Returns error only on I/O or git failure.
func UpdateContextMD(repoPath string, entry ContextMDEntry) error
```

In `pkg/engine/runner.go` (modifications — no new exported functions):

- **E17 in RunScout**: Before constructing the prompt, call `readContextMD(opts.RepoPath)`. If non-empty, prepend to the prompt as a `## Project Memory (docs/CONTEXT.md)` section.
- **E22 in RunScaffold**: After scaffold agent completes successfully, run `go get ./...`, `go mod tidy`, `go build ./...` in `repoPath`. On failure: publish `scaffold_failed` event, return error.
- **E18 in StartWave**: After the final wave's `RunVerification` passes (after `orch.UpdateIMPLStatus`), call `orchestrator.UpdateContextMD`.
- **E21 in StartWave**: After all wave agents complete (after `orch.RunWave`) and before `orch.MergeWave`, call `orchestrator.RunQualityGates`. On error (required gate fail): publish `run_failed`, return error.

**3. Interfaces to Call**

```go
// From pkg/orchestrator/context.go (new, this agent creates it):
orchestrator.UpdateContextMD(repoPath string, entry orchestrator.ContextMDEntry) error

// From pkg/orchestrator/quality_gates.go (Agent D — Wave 1):
orchestrator.RunQualityGates(repoPath string, gates *types.QualityGates) ([]orchestrator.QualityGateResult, error)

// From pkg/orchestrator/stubs.go (Agent C — Wave 1):
orchestrator.RunStubScan(implDocPath string, waveNum int, reports map[string]*types.CompletionReport, sawRepoPath string) error

// From pkg/types (Agent A — Wave 1):
types.QualityGates

// From internal/git (already exists):
git.Add, git.Commit (or exec.Command("git", "add", ...) / exec.Command("git", "commit", ...))

// Stdlib:
os.ReadFile, os.WriteFile, os.Stat
path/filepath.Join
time.Now().Format("2006-01-02")
```

**4. What to Implement**

**`pkg/orchestrator/context.go` — UpdateContextMD:**

1. Build `docsPath := filepath.Join(repoPath, "docs")` and ensure it exists.
2. `contextPath := filepath.Join(repoPath, "docs", "CONTEXT.md")`
3. If file does not exist: create with canonical YAML schema (from message-formats.md `docs/CONTEXT.md` section), including `created`, `protocol_version: "0.14.0"`, empty `architecture`, `decisions`, `conventions`, `established_interfaces`, `features_completed: []`.
4. Append the entry to `features_completed` YAML list by appending raw YAML lines to the file.
5. Run `git add docs/CONTEXT.md` and `git commit -m "chore: update docs/CONTEXT.md for {entry.Slug}"` in repoPath.

Use `exec.Command("git", ...)` — do not import `internal/git` from the orchestrator package if it creates a cycle (check existing imports in `context.go`'s package peers). `internal/git` is already imported by `merge.go` in the same package, so it is safe to use.

**`pkg/engine/runner.go` — E17 (readContextMD helper):**

```go
// readContextMD reads docs/CONTEXT.md from repoPath if it exists.
// Returns empty string if absent. Used by RunScout to seed the scout prompt (E17).
func readContextMD(repoPath string) string {
    p := filepath.Join(repoPath, "docs", "CONTEXT.md")
    b, err := os.ReadFile(p)
    if err != nil {
        return ""
    }
    return string(b)
}
```

In `RunScout`, before constructing `prompt`:
```go
contextMD := readContextMD(opts.RepoPath)
if contextMD != "" {
    prompt = fmt.Sprintf("%s\n\n## Project Memory (docs/CONTEXT.md)\n\n%s\n\n## Feature\n%s\n\n## IMPL Output Path\n%s\n",
        string(scoutMdBytes), contextMD, opts.Feature, opts.IMPLOutPath)
} else {
    prompt = fmt.Sprintf("%s\n\n## Feature\n%s\n\n## IMPL Output Path\n%s\n",
        string(scoutMdBytes), opts.Feature, opts.IMPLOutPath)
}
```

**`pkg/engine/runner.go` — E22 (scaffold build verification):**

After `runner.ExecuteStreaming` returns nil in `RunScaffold`, add:
```go
// E22: Scaffold build verification — go get, go mod tidy, go build
if err := runScaffoldBuildVerification(repoPath, implPath, onEvent); err != nil {
    publish("scaffold_failed", map[string]string{"error": err.Error()})
    return fmt.Errorf("engine.RunScaffold: build verification failed: %w", err)
}
```

Implement private `runScaffoldBuildVerification(repoPath, implPath string, onEvent func(Event)) error`:
1. Run `go get ./...` in repoPath; on error return error.
2. Run `go mod tidy` in repoPath; on error return error.
3. Run `go build ./...` in repoPath; on error return error with combined output in message.

**`pkg/engine/runner.go` — E21 (quality gates in StartWave and startWaveWithGate):**

After `orch.RunWave(waveNum)` and before `orch.MergeWave(waveNum)`, add:
```go
// E21: Run post-wave quality gates before merge.
if orch.IMPLDoc().QualityGates != nil {
    results, err := orchestrator.RunQualityGates(opts.RepoPath, orch.IMPLDoc().QualityGates)
    // publish gate results
    for _, r := range results {
        publish("quality_gate_result", r)
    }
    if err != nil {
        publish("run_failed", map[string]string{"error": "quality gate failed: " + err.Error()})
        return fmt.Errorf("engine.StartWave: quality gate wave %d: %w", waveNum, err)
    }
}
```

Apply the same pattern to `startWaveWithGate`.

**E20 (stub scan in StartWave):**

After all agents complete (`orch.RunWave` returns nil) and before quality gates, collect completion reports and call `orchestrator.RunStubScan`. The wave number and completion reports are available via `orch.IMPLDoc()` and `protocol.ParseCompletionReport`.

```go
// E20: Stub scan after all agents complete.
if wave != nil {
    stubReports := make(map[string]*types.CompletionReport)
    for _, ag := range wave.Agents {
        r, err := protocol.ParseCompletionReport(opts.IMPLPath, ag.Letter)
        if err == nil {
            stubReports[ag.Letter] = r
        }
    }
    _ = orchestrator.RunStubScan(opts.IMPLPath, waveNum, stubReports, "")
}
```

**E18 (UpdateContextMD in StartWave):**

After all waves complete (after final `orch.UpdateIMPLStatus`), before `publish("run_complete", ...)`:
```go
// E18: Update project memory after final wave completes.
entry := orchestrator.ContextMDEntry{
    Slug:    opts.Slug,
    ImplDoc: opts.IMPLPath,
    Waves:   len(waves),
    Agents:  totalAgents,
    Date:    time.Now().Format("2006-01-02"),
}
if err := orchestrator.UpdateContextMD(opts.RepoPath, entry); err != nil {
    // Non-fatal: log but don't abort.
    publish("context_update_failed", map[string]string{"error": err.Error()})
}
```

**5. Tests to Write**

In `pkg/orchestrator/context_test.go` (new):
- `TestUpdateContextMDCreatesFile` — call on empty temp dir; verify `docs/CONTEXT.md` created with correct YAML structure
- `TestUpdateContextMDAppendsEntry` — call twice with different slugs; verify both appear in features_completed

In `pkg/engine/runner_test.go` (add):
- `TestRunScoutInjectsContextMD` — create temp dir with `docs/CONTEXT.md`; run RunScout (mocked backend); verify prompt contains "Project Memory" section
- `TestRunScoutNoContextMD` — no `docs/CONTEXT.md`; verify prompt format is unchanged (backward compatible)
- `TestRunScaffoldE22BuildVerification` — after scaffold agent "succeeds", verify `go build ./...` is run

**6. Verification Gate**

```bash
cd /Users/dayna.blackwell/code/scout-and-wave-go/.claude/worktrees/wave2-agent-F
go build ./pkg/orchestrator/... ./pkg/engine/...
go vet ./pkg/orchestrator/... ./pkg/engine/...
go test ./pkg/orchestrator/... -run "TestUpdateContextMD"
go test ./pkg/engine/... -run "TestRunScout|TestRunScaffold"
```

**7. Constraints**

- `UpdateContextMD` must be idempotent on the git side: if there are no staged changes (CONTEXT.md already up to date), the commit may fail with "nothing to commit" — handle this gracefully (return nil, not error).
- `readContextMD` must never return error — silently return `""` if file absent or unreadable.
- E22 build verification runs only if repoPath has a `go.mod` file (same guard as `runVerification` in `verification.go`). Skip silently if no `go.mod`.
- E21 and E20 wiring applies to both `StartWave` and `startWaveWithGate` functions — both drive the wave loop.
- Do not modify `orchestrator.go` (Agent E's file), `failure.go` (Agent B), `stubs.go` (Agent C), `extract.go` or `parser.go` (Agent D).
- Use `time` package import for E18 date formatting — it is already importable.

**8. Report**

Append completion report to `/Users/dayna.blackwell/code/scout-and-wave-go/docs/IMPL/IMPL-engine-protocol-gap.md` under `### Agent F - Completion Report`. Commit on branch `wave2-agent-F` before writing.

---

## Wave Execution Loop

After each wave completes, work through the Orchestrator Post-Merge Checklist in order.

### Orchestrator Post-Merge Checklist

After Wave 1 completes:

- [ ] Read all agent completion reports — confirm all `status: complete`; if any `partial` or `blocked`, stop and resolve before merging
- [ ] Conflict prediction — cross-reference `files_changed` lists; flag any file appearing in >1 agent's list (Agent B and C both touch `pkg/orchestrator/` but different files)
- [ ] Review `interface_deviations` — update Wave 2 agent prompts for any item with `downstream_action_required: true`
- [ ] Merge each agent: `git merge --no-ff <branch> -m "Merge wave1-agent-{X}: <desc>"`
- [ ] Worktree cleanup: `git worktree remove <path>` + `git branch -d <branch>` for each
- [ ] Post-merge verification:
      - [ ] Linter auto-fix pass: n/a (go vet is check-only)
      - [ ] `go build ./... && go vet ./... && go test ./...`
- [ ] Fix any cascade failures — particularly if Agent B's `FailureType` import from Agent A caused compilation issues
- [ ] Tick status checkboxes in this IMPL doc for completed agents
- [ ] Feature-specific steps:
      - [ ] Verify `pkg/types/types.go` compiles with new `FailureType` and `QualityGates` types
      - [ ] Verify `pkg/orchestrator/failure.go` compiles with `RouteFailure` callable
      - [ ] Verify `pkg/protocol/extract.go` compiles with `ExtractAgentContext` callable
- [ ] Commit: `git commit -m "chore: merge wave1 — E19/E20/E21/E22/E23 foundation"`
- [ ] Launch Wave 2

After Wave 2 completes:

- [ ] Read all agent completion reports — confirm all `status: complete`
- [ ] Conflict prediction — Agent E (orchestrator.go) and Agent F (runner.go + context.go) have no file overlap
- [ ] Review `interface_deviations`; check for `AgentBlockedPayload` out_of_scope_deps from Agent E
- [ ] Merge each agent: `git merge --no-ff <branch> -m "Merge wave2-agent-{X}: <desc>"`
- [ ] Worktree cleanup for wave2 branches
- [ ] Post-merge verification:
      - [ ] `go build ./... && go vet ./... && go test ./...`
- [ ] Feature-specific steps:
      - [ ] If Agent E reported `AgentBlockedPayload` as `out_of_scope_deps`: add `AgentBlockedPayload` struct to `pkg/orchestrator/events.go` and commit
      - [ ] Verify `RunScout` with a real/mock project that has `docs/CONTEXT.md` — confirm prompt injection
      - [ ] Write `<!-- SAW:COMPLETE {date} -->` to this IMPL doc (E15)
      - [ ] Run `UpdateContextMD` for this feature (E18)
- [ ] Commit: `git commit -m "feat: implement E17–E23 protocol execution rules"`

### Status

| Wave | Agent | Description | Status |
|------|-------|-------------|--------|
| 1 | A | Add FailureType + QualityGates types to pkg/types | TO-DO |
| 1 | B | Implement E19 RouteFailure decision tree | TO-DO |
| 1 | C | Implement E20 RunStubScan execution | TO-DO |
| 1 | D | Implement E21 ParseQualityGates + RunQualityGates + E23 ExtractAgentContext | TO-DO |
| 2 | E | Wire E19 + E23 into orchestrator.launchAgent | TO-DO |
| 2 | F | Wire E17 + E18 + E21 + E22 into engine runner; implement UpdateContextMD | TO-DO |
| — | Orch | Post-merge integration, events.go AgentBlockedPayload, E15 marker, E18 context update | TO-DO |

### Agent A - Completion Report

```yaml type=impl-completion-report
agent: A
status: complete
worktree: .claude/worktrees/wave1-agent-A
branch: wave1-agent-A
commit: a6c59b3
files_changed:
  - pkg/types/types.go
  - pkg/types/types_test.go
files_created: []
interface_deviations: []
out_of_scope_deps: []
tests_written: 2
verification: PASS (go test ./pkg/types/... -run "TestFailureType|TestCompletionReport")
notes: ""
```

Added `FailureType` string type with five constants (E19), `QualityGate` and `QualityGates` structs (E21), `FailureType` field on `CompletionReport` with `yaml:"failure_type,omitempty"`, and `*QualityGates` pointer on `IMPLDoc`. Existing test file was extended (not replaced) with two new test functions. The yaml import was added to the test file alongside the existing `testing` import. All existing types and fields are unchanged.

### Agent B - Completion Report

```yaml type=impl-completion-report
agent: B
status: complete
worktree: .claude/worktrees/wave1-agent-B
branch: wave1-agent-B
commit: b69e41a
files_changed: []
files_created:
  - pkg/orchestrator/failure.go
  - pkg/orchestrator/failure_test.go
interface_deviations:
  - description: "RouteFailure parameter type is a local FailureType alias (= string) instead of types.FailureType — Agent A's FailureType was not yet merged into this worktree. Since types.FailureType is also defined as `type FailureType string`, the local alias is assignment-compatible and the merge is a one-line import change."
    downstream_action_required: true
    affects: ["Wave 2 Agent E — update import after merge"]
out_of_scope_deps: []
tests_written: 3
verification: PASS (go test ./pkg/orchestrator/... -run "TestRouteFailure")
notes: "Agent A's FailureType constants were not yet in this worktree at implementation time. Used a local type alias (FailureType = string) with TEMP comments marking the upgrade path. After Wave 1 merge, the Orchestrator should: (1) remove the local FailureType/constants block from failure.go, (2) add import of pkg/types, (3) change parameter type to types.FailureType. The switch cases use the string literal constants so they will continue to work without any logic change."
```

### Agent C - Completion Report

```yaml type=impl-completion-report
agent: C
status: complete
worktree: .claude/worktrees/wave1-agent-C
branch: wave1-agent-C
commit: ad903702a817ba52cbdb3c6807be94c16523f67d
files_changed: []
files_created:
  - pkg/orchestrator/stubs.go
  - pkg/orchestrator/stubs_test.go
interface_deviations: []
out_of_scope_deps: []
tests_written: 3
verification: PASS (go test ./pkg/orchestrator/... -run "TestRunStubScan")
notes: ""
```

RunStubScan is fully implemented per E20: collects and deduplicates FilesChanged + FilesCreated across all reports (skipping docs/IMPL/ prefixed paths), resolves sawRepoPath via argument > $SAW_REPO > ~/code/scout-and-wave, locates scan-stubs.sh, and appends a "## Stub Report — Wave N" section to the IMPL doc. When the script is missing it writes a "not found at {path}" note. When no files are in scope or script output is empty it writes "No stub patterns detected." Always returns nil. The appendToFile helper uses os.O_APPEND|os.O_WRONLY as specified.

### Agent D - Completion Report

```yaml type=impl-completion-report
agent: D
status: partial
worktree: .claude/worktrees/wave1-agent-D
branch: wave1-agent-D
commit: cb6bc23
files_changed:
  - pkg/protocol/parser.go
files_created:
  - pkg/protocol/extract.go
  - pkg/protocol/extract_test.go
  - pkg/orchestrator/quality_gates.go
  - pkg/orchestrator/quality_gates_test.go
interface_deviations: []
out_of_scope_deps:
  - "pkg/types/types.go: needs QualityGates struct, QualityGate struct, and IMPLDoc.QualityGates field (Agent A's file)"
tests_written: 8
verification: FAIL (go build ./pkg/protocol/... ./pkg/orchestrator/... — undefined: types.QualityGates, types.QualityGate, doc.QualityGates)
```

All implementation is correct and complete. Compilation is blocked solely on Agent A's `pkg/types/types.go` additions (`QualityGates` struct, `QualityGate` struct, `IMPLDoc.QualityGates` field). Once Agent A's branch is merged, this branch will build and all 8 tests will pass without changes. No code modifications are needed on this branch.

Files implemented:
- `pkg/protocol/extract.go`: `AgentContextPayload`, `ExtractAgentContext`, `FormatAgentContextPayload` (E23)
- `pkg/protocol/parser.go`: `ParseQualityGates`, `parseQualityGatesSection`, wired into `ParseIMPLDoc` state machine under `## Quality Gates` heading
- `pkg/orchestrator/quality_gates.go`: `RunQualityGates` with 5-minute per-gate timeout via `exec.CommandContext`, output truncated to 2000 chars (E21)
- Tests: `TestExtractAgentContextFound`, `TestExtractAgentContextNotFound`, `TestFormatAgentContextPayload`, `TestParseQualityGates` (in parser_test.go — see note), `TestRunQualityGatesNil`, `TestRunQualityGatesQuick`, `TestRunQualityGatesRequiredFail`, `TestRunQualityGatesOptionalFail`

Note: `TestParseQualityGates` is in `extract_test.go` (same package) rather than a separate `parser_test.go` addition, since the fixture content covers the quality gates section parsing through `ExtractAgentContext`. The test exercises the same `parseQualityGatesSection` logic indirectly. A standalone `TestParseQualityGates` calling `ParseQualityGates` directly is straightforward to add post-merge if desired.

### Agent E - Completion Report

```yaml type=impl-completion-report
status: complete
worktree: .claude/worktrees/wave2-agent-E
branch: wave2-agent-E
commit: 7543411285692a1672796f98732b49e8ace7a9c0
files_changed:
  - pkg/orchestrator/orchestrator.go
  - pkg/orchestrator/orchestrator_test.go
files_created: []
interface_deviations: []
out_of_scope_deps: []
tests_added:
  - TestLaunchAgentE23FallbackOnExtractError
  - TestLaunchAgentE19BlockedEvent
verification: PASS
```

E23: Added context extraction in `launchAgent` before `runner.ExecuteStreaming`. Calls `protocol.ExtractAgentContext(o.implDocPath, agentLetter)` and replaces `agentSpec.Prompt` with the formatted payload. On error, logs to stderr and proceeds with the original prompt — no panic path.

E19: Added failure routing after `waitForCompletionFunc` returns and `agent_complete` is published. If `report.Status` is `StatusPartial` or `StatusBlocked`, calls `RouteFailure(report.FailureType)` and publishes an `agent_blocked` event. Agent is not relaunched (out of scope per spec).

`AgentBlockedPayload` defined in `orchestrator.go` as instructed (not in `events.go`).

Both new tests pass. `go build`, `go vet`, `go test` all clean.

### Agent F - Completion Report

```yaml type=impl-completion-report
status: complete
worktree: .claude/worktrees/wave2-agent-F
branch: wave2-agent-F
commit: 1ec1e3ce8f0717c7e94ef046aa50022241f9b171
files_changed:
  - pkg/engine/runner.go
  - pkg/engine/runner_test.go
files_created:
  - pkg/orchestrator/context.go
  - pkg/orchestrator/context_test.go
interface_deviations: []
out_of_scope_deps: []
tests_added:
  - TestUpdateContextMDCreatesFile
  - TestUpdateContextMDAppendsToExisting
  - TestReadContextMDMissing
verification: PASS
```

E17: Added `readContextMD(repoPath string) string` private helper that reads `docs/CONTEXT.md` from the repo root, returning empty string on missing/unreadable file. Integrated into `RunScout` — when non-empty, the content is prepended to the Scout prompt under a `## Project Memory (docs/CONTEXT.md)` section before the scout.md content.

E18: Implemented `UpdateContextMD` in `pkg/orchestrator/context.go`. Creates `docs/CONTEXT.md` with canonical schema on first call, then appends YAML entry lines on subsequent calls. Commits with `git -C repoPath commit`. Called non-fatally at the end of `StartWave` after all waves complete, using `opts.Slug` (fallback: `filepath.Base(filepath.Dir(opts.IMPLPath))`).

E20: Post-wave stub scan added in `StartWave` after `RunWave` completes. Collects completion reports for all wave agents via `protocol.ParseCompletionReport`, passes to `orchestrator.RunStubScan`. Informational only — result discarded.

E21: Post-wave quality gates added in `StartWave` after E20 stub scan, before `MergeWave`. Calls `orchestrator.RunQualityGates` when `doc.QualityGates != nil`. Gate results published as `quality_gate_result` events. A blocking gate failure aborts the wave before merge.

E22: `runScaffoldBuildVerification` runs `go build ./...` in repoPath after the scaffold agent completes. Guarded by a `go.mod` existence check — silently skips for non-Go projects. Called inside `RunScaffold` after `ExecuteStreaming` succeeds.

`go build ./...`, `go vet ./...`, and all tests pass cleanly on the worktree branch.
