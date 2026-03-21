# Agent D Brief - Wave 2

**IMPL Doc:** /Users/dayna.blackwell/code/scout-and-wave/docs/IMPL/IMPL-interview-mode.yaml

## Files Owned

- `cmd/saw/main.go`
- `implementations/claude-code/prompts/saw-skill.md`


## Task

## Role
Wire the interview command into sawtools and update the protocol.

## What to Implement

### cmd/saw/main.go (scout-and-wave-go)
Add `newInterviewCmd()` to the rootCmd.AddCommand(...) call.
Pattern: find the existing AddCommand block and add one line:
```go
newInterviewCmd(),
```
Place it near other user-facing commands (e.g., after newRunScoutCmd()).

### implementations/claude-code/prompts/saw-skill.md (scout-and-wave)
Two changes:

**1. Add to Invocation Modes table** (after the existing rows):
```
| `/saw interview "<description>"` | Conduct structured requirements interview, write docs/REQUIREMENTS.md |
| `/saw interview --resume <path>`  | Resume an in-progress interview |
```

**2. Add interview command handling block** in the Arguments section or
after the bootstrap argument description. Add a new block:

```
If the argument is `interview <description>` (or `interview --resume <path>`):
1. Run the requirements interview by invoking:
   ```bash
   sawtools interview "<description>" --docs-dir "<project-docs-dir>" --output "<project-docs-dir>/REQUIREMENTS.md"
   ```
   OR conduct the interview inline using AskUserQuestion if sawtools interview is not available
   in the current environment (fallback mode).
2. After sawtools interview exits 0, docs/REQUIREMENTS.md exists.
3. Inform the user: "Interview complete. REQUIREMENTS.md written to docs/REQUIREMENTS.md.
   Run /saw bootstrap to design the architecture, or /saw scout <feature> to plan a feature."
4. Do NOT automatically launch bootstrap or scout — wait for explicit user command.

If the argument is `interview --resume <path>`:
1. Run: sawtools interview --resume "<path>"
2. Same completion handling as above.
```

**Also update argument-hint** in the YAML frontmatter:
Change from:
```
argument-hint: "[bootstrap <project-name> | scout [--model <m>] <feature> | wave ...]"
```
To include `| interview <description>`:
```
argument-hint: "[bootstrap <project-name> | interview <description> | scout [--model <m>] <feature> | wave ...]"
```

## Interfaces to Call
- `newInterviewCmd()` from `cmd/saw/interview_cmd.go` (Agent C, Wave 1)

## Tests to Write
No new tests required for this wiring agent. Verify by:
```bash
cd /Users/dayna.blackwell/code/scout-and-wave-go && go build ./cmd/saw/... && ./saw interview --help
```
Confirm --help output shows the interview command.

## Verification Gate
cd /Users/dayna.blackwell/code/scout-and-wave-go && go build ./cmd/saw/... && go vet ./cmd/saw/...

## Constraints
- In cmd/saw/main.go: add ONLY the one AddCommand line. Do not restructure the file.
- In saw-skill.md: add ONLY the interview blocks described above. Do not modify any
  existing bootstrap, scout, wave, or program command handling.
- The argument-hint change is a one-line edit to the YAML frontmatter.
- Verify saw-skill.md version comment at top stays as-is (do not bump version).
- Both repos have different working directories; use absolute paths in all bash commands.



## Interface Contracts

### Manager

Core interface for the interview state machine. Both deterministic and
LLM-driven implementations satisfy this. The CLI command uses this interface;
it never calls concrete implementations directly.


```
type Manager interface {
    Start(cfg InterviewConfig) (*InterviewDoc, *InterviewQuestion, error)
    Resume(docPath string) (*InterviewDoc, *InterviewQuestion, error)
    Answer(doc *InterviewDoc, answer string) (*InterviewDoc, *InterviewQuestion, error)
    Compile(doc *InterviewDoc, outputPath string) (string, error)
    Save(doc *InterviewDoc, docPath string) error
}

```

### DeterministicManager

Fixed question-set implementation. Covers all 6 phases with required fields
checked before phase advance. Works without LLM; used for testing and offline use.
Phase transition logic: advance when all required fields for the current phase
are non-empty in SpecData.


```
// pkg/interview/deterministic.go
type DeterministicManager struct {
    docsDir string // base dir for writing INTERVIEW-<slug>.yaml files
}

func NewDeterministicManager(docsDir string) *DeterministicManager

// Implements Manager interface.

```

### InterviewDoc (YAML schema)

YAML file written to docs/INTERVIEW-<slug>.yaml. Persists the full interview
state including history, spec_data being built, and output path.
This file is git-trackable and survives process restarts.


```
# docs/INTERVIEW-<slug>.yaml schema
id: <uuid>
slug: <feature-slug>
status: in_progress | complete
mode: deterministic | llm
description: "original description string"
created_at: 2026-01-01T00:00:00Z
updated_at: 2026-01-01T00:00:00Z
phase: overview | scope | requirements | interfaces | stories | review | complete
question_cursor: 0
max_questions: 18
progress: 0.0
spec_data:
  overview:
    title: ""
    goal: ""
    success_metrics: []
    non_goals: []
  scope:
    in_scope: []
    out_of_scope: []
    assumptions: []
  requirements:
    functional: []
    non_functional: []
    constraints: []
  interfaces:
    data_models: []
    apis: []
    external: []
  stories: []
  open_questions: []
history:
  - turn_number: 1
    phase: overview
    question: "What is the title of this project/feature?"
    answer: "Interview Mode for SAW Bootstrap"
    timestamp: 2026-01-01T00:00:00Z
requirements_path: docs/REQUIREMENTS.md

```

### newInterviewCmd (sawtools CLI)

CLI command: sawtools interview "<description>" [--mode deterministic|llm]
[--max-questions N] [--project-path <path>] [--resume <interview-doc-path>]
[--output <requirements-output-path>]

Drives the AskUserQuestion interaction loop: prints each question to stdout,
reads stdin for each answer, writes state to INTERVIEW-<slug>.yaml after each
turn. On completion writes docs/REQUIREMENTS.md.

Exit codes: 0 = interview complete and REQUIREMENTS.md written,
1 = error, 2 = interview started but not yet complete (resumable).


```
// cmd/saw/interview_cmd.go
func newInterviewCmd() *cobra.Command

```

### CompileToRequirements

Converts a complete InterviewDoc into the REQUIREMENTS.md template format
expected by saw-bootstrap.md Phase 0. This is the output contract linking
/saw interview to /saw bootstrap.


```
// pkg/interview/compiler.go
// CompileToRequirements generates a REQUIREMENTS.md-compatible markdown string
// from a completed InterviewDoc. The output matches the template in saw-skill.md
// bootstrap section exactly (Language, Project Type, Key Concerns, Storage,
// External Integrations, Architectural Decisions Already Made sections).
func CompileToRequirements(doc *InterviewDoc) (string, error)

```

### /saw interview (protocol command)

Orchestrator command added to saw-skill.md. Invocation:
  /saw interview "<description>"
Launches sawtools interview command as a subprocess (or uses AskUserQuestion
loop inline). On completion, the REQUIREMENTS.md is present and the orchestrator
can immediately launch /saw bootstrap or /saw scout with that file as input.


```
# In saw-skill.md Invocation Modes table:
| /saw interview "<description>" | Conduct structured requirements interview, write docs/REQUIREMENTS.md |
| /saw interview --resume <path>  | Resume an in-progress interview from its YAML doc |

```



## Quality Gates

Level: standard

- **build**: `go build ./...` (required: true)
- **lint**: `go vet ./...` (required: true)
- **test**: `go test ./...` (required: true)

