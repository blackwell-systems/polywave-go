# Agent C Brief - Wave 2

**IMPL Doc:** /Users/dayna.blackwell/code/scout-and-wave/docs/IMPL/IMPL-interview-improvements.yaml

## Files Owned

- `pkg/interview/deterministic_test.go`
- `cmd/saw/interview_cmd_test.go`


## Task

## Tests for All P0 and P1 Changes

### What to implement

**New file: pkg/interview/deterministic_test.go**
Write comprehensive tests for the new functionality added by Agent B:

1. `TestValidateRequiredField_EmptyRejectsRequired` — empty answer on
   required question returns error message
2. `TestValidateRequiredField_WhitespaceRejectsRequired` — whitespace-only
   answer on required question returns error message
3. `TestValidateRequiredField_SkipOnOptionalAccepted` — "skip" on optional
   question returns empty (valid)
4. `TestValidateRequiredField_SkipOnRequiredRejected` — "skip" on required
   question returns error message
5. `TestValidateRequiredField_ValidAnswer` — non-empty answer returns empty

6. `TestHandleBackCommand_FirstQuestion` — returns false at cursor 0
7. `TestHandleBackCommand_Success` — returns true, decrements cursor,
   removes last history entry, clears populated field
8. `TestHandleBackCommand_CrossPhase` — going back across phase boundary
   reverts doc.Phase correctly
9. `TestHandleBackCommand_CaseInsensitive` — "BACK", "Back" all work

10. `TestAnswer_ValidationRejectsEmpty` — Answer() with empty string on
    required field returns same question with hint
11. `TestAnswer_BackIntegration` — Answer() with "back" reverts state

12. `TestFormatPhaseProgress_Overview` — returns "[Overview: 1/4 | Next: Scope]"
13. `TestFormatPhaseProgress_Review` — returns "[Review: 1/2 | Next: Done]"

14. `TestPreviewRequirements` — returns non-empty markdown string

**Modify: cmd/saw/interview_cmd_test.go**
15. `TestInterviewCmd_InitWiring` — verify that newDeterministicManagerFunc
    is non-nil after package init (confirms P0.1 fix). Note: the test file
    uses installMockManager which overrides init(), so test init separately:
    just assert `newDeterministicManagerFunc != nil` before any mock install.
16. `TestInterviewCmd_PhaseProgress` — verify output contains phase-aware
    format like "[Overview:" instead of old "[Phase: Overview"

### Interfaces to consume
- `ValidateRequiredField(q *InterviewQuestion, answer string) string`
- `HandleBackCommand(doc *InterviewDoc, answer string) bool`
- `FormatPhaseProgress(doc *InterviewDoc) string`
- `PreviewRequirements(doc *InterviewDoc) (string, error)`
- `NewDeterministicManager(docsDir string) *DeterministicManager`

### Verification gate
```bash
go test ./pkg/interview/... -v && go test ./cmd/saw/... -v -run "TestInterviewCmd"
```

### Constraints
- Do NOT modify any non-test files
- Use testify/assert or standard testing — match project conventions
  (existing tests use standard testing with t.Fatalf/t.Errorf)
- Use t.TempDir() for any file I/O in tests
- Test the actual functions, not mocks — these are unit tests of real logic



## Interface Contracts

### init() in interview_cmd.go

Package-level init function that wires newDeterministicManagerFunc
to interview.NewDeterministicManager, fixing the P0 nil panic.


```
func init() {
    newDeterministicManagerFunc = func(docsDir string) interview.Manager {
        return interview.NewDeterministicManager(docsDir)
    }
}

```

### init() in compiler.go

Package-level init function that registers CompileToRequirements
with the deterministic manager's compileFunc variable.


```
func init() {
    RegisterCompiler(CompileToRequirements)
}

```

### ValidateRequiredField

Validates that a required field answer is not empty/whitespace.
Returns an error message string if invalid, empty string if valid.
Called by Answer() before recording the turn.


```
func ValidateRequiredField(q *InterviewQuestion, answer string) string
// Returns "" if valid, or error message like
// "This field is required. Please provide an answer."
// Checks: q.Required && strings.TrimSpace(answer) == ""

```

### HandleBackCommand

Detects "back" answer and reverts to previous question by
removing the last history entry and decrementing cursor.
Returns true if back was handled (caller should re-generate question).


```
func HandleBackCommand(doc *InterviewDoc, answer string) bool
// Returns true if answer is "back" (case-insensitive).
// Reverts: remove last History entry, decrement QuestionCursor,
// clear the SpecData field that was populated by that answer,
// revert Phase if needed via checkPhaseTransition logic.

```

### FormatPhaseProgress

Generates phase-aware progress string like "[Overview: 2/4 | Next: Scope]"
instead of naive percentage.


```
func FormatPhaseProgress(doc *InterviewDoc) string
// Returns string like "[Overview: 2/4 | Next: Scope]"
// Counts questions in current phase, shows position within phase,
// shows next phase name (or "Done" if in review).

```

### PreviewRequirements

Generates a preview of the compiled REQUIREMENTS.md content
for display before the final confirmation question.


```
func PreviewRequirements(doc *InterviewDoc) (string, error)
// Calls CompileToRequirements and returns the content string.
// Used by the CLI to show preview before "Ready to generate?" prompt.

```



## Quality Gates

Level: standard

- **build**: `go build ./...` (required: true)
- **lint**: `go vet ./...` (required: true)
- **test**: `go test ./...` (required: true)

