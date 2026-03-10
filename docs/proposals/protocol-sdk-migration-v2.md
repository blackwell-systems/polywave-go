# Proposal: Protocol SDK Migration
# Foreword: Why Protocol SDK Migration

**Date:** March 9, 2026
**Author:** Dayna Blackwell
**Status:** Foundational architectural decision

---

## The Problem We Solved

Scout-and-Wave began with an elegant vision: **a protocol based entirely on natural language.** Agents coordinate through prose IMPL documents. Scout writes markdown, Wave agents read markdown, orchestrator parses markdown. No schemas, no rigid formats—just humans and LLMs communicating in shared language.

This worked. Wave 1 of the tool refactor executed successfully just hours before writing this. Four agents coordinated in parallel, delivered working code, and merged cleanly. The protocol functions.

But as we scaled—more features, more agents, more complexity—cracks appeared:

**1. Parse errors and retry loops**
- Scout writes IMPL doc → Validator parses with regex → Rejects malformed structure
- Scout retries (up to 3 attempts) with corrections
- Still fails on novel formatting variations
- No compile-time safety; errors discovered at runtime after Scout completes

**2. No code reuse**
- Bash scripts parse markdown for CLI orchestration (400 lines of grep/sed)
- Go parser parses markdown for web UI (800 lines of state machine)
- Two implementations, diverging logic, double maintenance burden
- External tools can't consume protocol state without writing their own parser

**3. Reactive enforcement**
- File ownership violations caught at merge time, not registration time
- Stub detection happens after agents finish (post-mortem scan)
- Interface contract deviations reported in completion reports, not blocked upfront
- Every check is "did this already happen?" instead of "prevent this from happening"

**4. Merge conflicts scale with agents**
- Multiple agents append to same markdown IMPL doc → expected conflicts
- Workaround: Per-agent report files for waves with ≥5 agents
- Coordination overhead grows with team size

**5. Limited programmatic access**
- Want IDE plugin that validates IMPL doc on save? Write a markdown parser.
- Want CI/CD that runs wave orchestration? Parse markdown with bash.
- Want monitoring dashboard that shows wave status? Parse markdown with JavaScript.
- No SDK, no API—just text files and regex.

These aren't fatal flaws. We worked around them. But each workaround added friction. Each parse error added a retry loop. Each new tool added another markdown parser.

The vision was pure: **natural language coordination.** The reality was brittle: **regex parsing with hope.**

---

## What We're Building

**Protocol as SDK:** Structured types and operations, importable as a Go library. YAML manifests replace markdown documents. Validation is schema-based, not regex-based. Operations are functions, not scripts.

**But we keep what works:** Claude-as-orchestrator remains primary. Conversational error recovery. Human-in-the-loop checkpoints. Interactivity over automation.

**The shift:**
- **Before:** Protocol = prose instructions, coordination = interpret prose
- **After:** Protocol = structured data, coordination = call SDK operations

**What's preserved:**
- Claude coordinates wave execution (still primary model)
- Task descriptions are prose (agents interpret creative work)
- Error recovery is conversational (Claude analyzes, suggests, adapts)
- Human review at checkpoints (approve merge, request changes)

**What changes:**
- IMPL docs are YAML (schema-validated)
- File ownership is data structure (not markdown table)
- Validation is code (not bash regex)
- Operations are importable (SDK functions)
- Orchestrator/agent backends are configurable (not locked)

---

## The Tensions We Resolved

### Tension 1: Interactivity vs Determinism

**Natural language protocol:**
- ✓ Flexible (Scout adapts IMPL doc format)
- ✓ Readable (humans review prose)
- ✗ Brittle (parse errors on format variations)
- ✗ Unpredictable (validation passes or fails mysteriously)

**Structured protocol:**
- ✓ Deterministic (schema validation, no parse errors)
- ✓ Predictable (same input always validates same way)
- ✗ Rigid (schema changes require coordination)
- ✗ Less readable (YAML vs prose)

**Our resolution:**
- **Structure the data** (YAML manifest, schema validation)
- **Keep prose where it matters** (task descriptions, pre-mortem notes)
- **Preserve interactivity** (Claude orchestrates, handles errors conversationally)
- **Generate human-readable views** (`saw render` produces markdown for review)

Result: **Deterministic data operations with interactive coordination.**

### Tension 2: Automation vs Human Judgment

**Wave 1 execution (hours before writing this):**
- Agent B created duplicate `NewWorkshop()` declaration (temporary stub for testing)
- Agent D duplicated `executorFunc` in test file (mock vs production)
- Programmatic response: "Error: redeclared in this block" → exit code 1
- Claude response: Read both files, understood intent (stub vs real), removed stubs

**The insight:** Unexpected errors need semantic understanding, not predefined handlers.

**Pure automation approach:**
```go
func handleRedeclaration(err error) error {
    // How do we decide which declaration to keep?
    // - Check timestamps? (unreliable)
    // - Parse ownership? (what if both agents own file?)
    // - Delete one randomly? (wrong)
    return fmt.Errorf("manual intervention required")
}
```

Every error would escalate to human. Automation gains nothing.

**Our resolution:**
- **Happy path is automated** (SDK validates manifest, agents execute, merge proceeds)
- **Error recovery is interactive** (Claude reads context, suggests fixes, asks human)
- **Known patterns codified** (expected IMPL doc conflicts auto-resolved)
- **Novel failures escalate** (Claude analyzes, human decides)

Result: **Automation for predictable work, human judgment for novel failures.**

### Tension 3: Single Backend vs Multi-Backend

**Original assumption:** Scout-and-Wave runs in Claude Code CLI.

**Reality:** Users want:
- **CLI with Max Plan** (your use case)
- **CLI with Bedrock** (enterprise AWS)
- **Web UI** (browser-based, no CLI)
- **Programmatic Go** (CI/CD, batch processing)
- **API-first** (direct Anthropic API, no Max Plan)

Each execution context has different capabilities:
- CLI: Interactive, conversational, Claude-powered
- Web: Browser-based, form inputs, HTTP requests
- Programmatic: Automated, no human, exit codes

**Our resolution:**
- **Protocol SDK is backend-agnostic** (Load/Validate work everywhere)
- **Orchestrator backend is pluggable** (CLI/Web/API all supported)
- **Agent backend is pluggable** (already works)
- **Primary remains CLI with Claude** (best for interactivity)

Result: **Write SDK once, use everywhere. Optimize orchestrator per context.**

### Tension 4: Flexibility vs Reliability

**Natural language:**
- ✓ Flexible (Scout can adapt format)
- ✗ Unreliable (validation failures, retries)

**Rigid schema:**
- ✓ Reliable (validation always works)
- ✗ Inflexible (schema changes are breaking)

**Our resolution:**
- **Core structure is rigid** (file ownership, wave structure, completion reports)
- **Content is flexible** (task descriptions, notes, explanations)
- **Schema evolution is supported** (migration utility for format changes)
- **Human-readable views preserve accessibility** (generated markdown)

Result: **Structured coordination with flexible content.**

---

## Why This Architecture

### Design Constraint 1: All Execution Environments

**Must work in:**
- CLI with skill (bash + Claude)
- Web UI (HTTP + browser)
- Programmatic Go (library import)

**SDK layer enables this:**
```
SDK (pure Go library)
  ↓ used by
CLI binary (shell commands) + Web UI (HTTP) + Programmatic (import)
  ↓ each exposes
Same operations, different interface
```

No SDK = each context reimplements parsing/validation. With SDK = one implementation, three interfaces.

### Design Constraint 2: Orchestrator/Agent Separation

**Orchestrator** (coordinates):
- Reads IMPL manifest
- Launches agents
- Monitors progress
- Handles errors
- Merges results

**Agents** (execute):
- Receive task context
- Write code
- Run verification
- Report completion

**These use different backends:**
- Orchestrator: Your Claude session (Max Plan or Bedrock)
- Agents: Company Bedrock account, or API key, or OpenAI

**Architecture enables cost/compliance separation:**
```yaml
orchestrator:
  backend: api  # Your API key (coordination)
agents:
  backend: bedrock  # Company AWS (compute)
```

Result: Coordinate on your account, execute on company account.

### Design Constraint 3: Preserve Interactive Recovery

**Why Claude-as-orchestrator remains primary:**

Pure Go orchestrator would gain:
- ✓ Standalone binary (no CLI dependency)
- ✓ Testable (Go unit tests)
- ✓ Distributable (compile once, run anywhere)

But lose:
- ✗ Conversational recovery ("duplicate NewWorkshop() - why?")
- ✗ Mid-execution flexibility (pause, inspect, adapt)
- ✗ Human-in-the-loop (Claude asks "should I proceed?")

**We chose interactivity.** CLI with Claude as primary, Go orchestrator as secondary (for CI/CD where errors → hard fail anyway).

### Design Constraint 4: Incremental Adoption

**Can't rewrite everything at once.** Must ship in phases:

**Phase 1:** Protocol SDK (manifests, validation, CLI commands)
- Agents remain pluggable (already works)
- Orchestrator remains Claude-in-CLI (current model)
- **Shippable independently**, validates in production

**Phase 2:** Orchestrator backend (full pluggability)
- Orchestrator becomes pluggable (API/Bedrock/OpenAI)
- Builds on Phase 1 (SDK already validated)
- **Ships only if Phase 1 stable**

This de-risks execution. Phase 1 delivers value (code reuse, deterministic validation) without requiring Phase 2 (orchestrator abstraction).

---

## What We Gain

### 1. Deterministic Validation (No More Retry Loops)

**Before:**
```bash
Scout writes markdown → bash script parses → validation fails → Scout retries (3x)
```

**After:**
```bash
Scout writes YAML → SDK validates → schema error → Scout fixes immediately
```

Schema errors are precise: "field `waves[0].agents[1].files` is required but missing". Not "parse error on line 342".

### 2. Code Reuse (Write Once, Use Everywhere)

**Before:**
- Bash scripts parse markdown (CLI)
- Go parser parses markdown (web UI)
- Would need JS parser (frontend monitoring)
- Would need Rust parser (IDE plugin)

**After:**
```go
import "github.com/blackwell-systems/scout-and-wave-go/pkg/protocol"

manifest, _ := protocol.Load("IMPL-foo.yaml")
protocol.Validate(manifest)
```

One implementation, importable by all tools.

### 3. Programmatic Access (SDK as API)

**Before:**
External tool wants to check wave status → must parse markdown with regex.

**After:**
```go
manifest, _ := protocol.Load("IMPL-foo.yaml")
currentWave := manifest.CurrentWave()
for _, agent := range currentWave.Agents {
    report := manifest.CompletionReports[agent.ID]
    fmt.Printf("%s: %s\n", agent.ID, report.Status)
}
```

Protocol state is queryable. IDE plugins, monitoring dashboards, CI/CD tools all use SDK.

### 4. Preventive Enforcement (Not Reactive)

**Before:**
- Agent B writes to `pkg/tools/workshop.go`
- Agent A also writes to `pkg/tools/workshop.go`
- Merge conflict detected at merge time (too late)

**After:**
```go
// At manifest validation time (before agents launch)
func Validate(m *Manifest) error {
    seen := make(map[string]string)
    for _, fo := range m.FileOwnership {
        if owner, exists := seen[fo.File]; exists {
            return fmt.Errorf("I1 violation: %s owned by %s and %s",
                fo.File, owner, fo.Agent)
        }
        seen[fo.File] = fo.Agent
    }
}
```

Conflict prevented before agents start. Fail-fast, not fail-late.

### 5. Scalability (No Merge Conflicts)

**Before:**
```markdown
### Agent A — Completion Report
...
### Agent B — Completion Report
...
```
4+ agents → merge conflicts in IMPL doc.

**After:**
```bash
# Each agent writes to separate file
saw set-completion IMPL-foo.yaml agent-A < completion-report.yaml
# SDK updates manifest atomically, no conflicts
```

Agent count scales without coordination overhead.

### 6. Multi-Backend Interoperability

**Before:**
Orchestrator locked to Claude Code CLI.

**After:**
```yaml
orchestrator:
  backend: bedrock  # AWS account A
agents:
  backend: api  # API key for account B
```

Mix and match backends. Orchestrate on Bedrock, execute on API. Cost separation, compliance separation, model flexibility.

---

## The Balance We Struck

### What We Structured

**Data and coordination primitives:**
- ✓ YAML manifests (schema-validated)
- ✓ File ownership (map structure)
- ✓ Wave structure (typed objects)
- ✓ Completion reports (structured JSON/YAML)
- ✓ Validation rules (Go code, not regex)

### What Stays Prose

**Creative and interpretive work:**
- ✓ Task descriptions (agents interpret)
- ✓ Pre-mortem analysis (exploratory thinking)
- ✓ Architectural notes (CONTEXT.md)
- ✓ Deviation explanations (freeform text in reports)

### What Stays Interactive

**Coordination and recovery:**
- ✓ Claude-as-orchestrator (conversational recovery)
- ✓ Human review checkpoints (approve/reject)
- ✓ Error analysis (Claude reads, suggests, asks)
- ✓ Mid-execution decisions (proceed? retry? escalate?)

---

## Why Now

**Wave 1 of tool refactor just completed successfully.** Four agents, parallel execution, clean merge. Protocol works.

But we hit friction:
- Duplicate `NewWorkshop()` required manual investigation
- Agent D's mock `executorFunc` conflicted with production
- Completion reports needed careful merge conflict resolution

These are solvable with better tooling. But they're symptoms of deeper architectural debt:
- Parse errors accumulate
- Code duplication proliferates (bash + Go + future JS)
- External tools can't access protocol state
- Orchestrator locked to Claude CLI

**We're at the right inflection point:**
- Protocol is proven (tool refactor Wave 1 succeeded)
- Pain points are clear (parse errors, code duplication)
- Solution is understood (SDK + structured manifests)
- Team has bandwidth (can dedicate waves to this)

Delaying means:
- More features using markdown (harder migration later)
- More parsers proliferating (bash + Go + JS + Rust)
- More external tools blocked (no SDK to import)
- More orchestrator coupling (harder to make pluggable)

**Now is the time.**

---

## Success Criteria

This refactor succeeds if:

**1. Zero parse errors**
- Scout generates valid YAML (schema-enforced)
- Validator never rejects well-formed manifests
- No retry loops for format issues

**2. One implementation**
- CLI, web UI, external tools all use SDK
- Protocol rule changes update one place
- No diverging parsers

**3. Preventive enforcement**
- I1 violations caught before agents launch
- File ownership conflicts impossible
- Validation is fail-fast, not fail-late

**4. Preserved interactivity**
- Claude still orchestrates (conversational recovery)
- Human review still happens (checkpoints)
- Error handling still flexible (analyze, not exit code)

**5. Backend flexibility**
- Orchestrator configurable (API/Bedrock/OpenAI/CLI)
- Agents configurable (already works)
- Mix and match freely (cost optimization, compliance)

**6. Incremental delivery**
- Phase 1 ships independently (SDK + CLI)
- Phase 2 ships after Phase 1 stable (orchestrator abstraction)
- Each phase tested in production before next begins

---

## Long-Term Vision

This refactor isn't just about fixing parse errors. It's about making Scout-and-Wave **a platform**, not just a tool.

**With SDK:**
- IDE plugins validate IMPL docs in real-time
- Monitoring dashboards show wave status
- CI/CD pipelines orchestrate wave execution
- External tools query protocol state programmatically

**With orchestrator backend abstraction:**
- Enterprise deploys on Bedrock (AWS-only)
- Startups use direct API (fine-grained cost control)
- Developers use Max Plan (fast iteration)
- All use same protocol, same SDK, same guarantees

**With structured enforcement:**
- File ownership conflicts impossible (I1 enforced at registration)
- Interface contracts importable (types, not descriptions)
- Validation deterministic (same manifest always validates same)
- Operations testable (unit tests for protocol rules)

**The protocol becomes:**
- **Reliable** (deterministic validation)
- **Reusable** (SDK importable)
- **Interoperable** (multi-backend)
- **Interactive** (Claude coordinates)
- **Incremental** (phased delivery)

This is the foundation for scaling Scout-and-Wave from "tool that works" to "platform for parallel AI development."

**Phase 3 consideration: Agent Runtime Abstraction**

The SDK's agent execution layer should define a `Runtime` interface that abstracts how agents are launched and monitored. Phase 1 implements this as "launch Claude Code subagent via Agent tool." Future phases could swap in:
- **Claude Agent SDK** (Python/TS) — Anthropic's official agent runtime, with built-in tools, hooks, Skills, and subagent orchestration
- **Google ADK** (Go) — ParallelAgent primitive, model-agnostic design, but genai type tax
- **Direct LLM API** — Raw conversation loop with tool dispatch, maximum control

Designing the `Runtime` interface now (even if Phase 1 only has one implementation) keeps these paths open without committing prematurely. See *Appendix: Framework Evaluation* for the full analysis.

---

## Personal Note

This is one of my biggest architectural decisions. I'm choosing to add significant complexity (SDK types, CLI binary, backend abstraction) to solve problems that have workarounds (retry on parse errors, write another parser).

I'm doing this because I see where it goes:
- Parse errors accumulate into death by a thousand cuts
- Code duplication creates maintenance burden that scales with features
- External tool blockers prevent ecosystem growth
- Orchestrator coupling limits deployment flexibility

**I believe this refactor is worth it.** Not because the current system is broken (it works), but because the future system is significantly better (deterministic, reusable, flexible).

**I'm committing to phased execution** (Phase 1 → validate → Phase 2) because the scope is large and the risk is real. If Phase 1 doesn't deliver value, Phase 2 doesn't happen.

**I'm using SAW to implement itself** (self-validation) because if the protocol can't build itself, it's not ready for production.

This document exists so future-me (and future-contributors) can remember:
- **Why we did this** (parse errors, code duplication, limited programmatic access)
- **What we gained** (determinism, reuse, flexibility)
- **What we preserved** (interactivity, human judgment, conversational recovery)
- **Why this architecture** (all execution contexts, orchestrator/agent separation)

**If this refactor succeeds, Scout-and-Wave becomes a platform.**
**If it fails, we have a clearer understanding of why natural language protocols have limits.**
**Either outcome is valuable.**

Let's build.

—Dayna Blackwell
March 9, 2026

---

**Version:** 2.0
**Date:** 2026-03-09
**Status:** Proposal (for Scout analysis)

---

## Executive Summary

Migrate the Scout-and-Wave protocol from **markdown-with-bash-parsing** to **YAML manifest with Go SDK**. The goal is to provide a **programmatic API for protocol operations** that can be shared across CLI orchestration, web UI, and external tools.

**Key Decision:** Claude remains the orchestrator via the `/saw` skill (bash-based coordination). The Go SDK provides atomic operations as shell commands that the skill calls. This preserves interactive error recovery while gaining structured data validation and code reuse.

---

## Core Problem

**Current architecture:** Protocol state lives in markdown IMPL docs, parsed by bash scripts with regex.

**Friction points:**
- Parse errors and validation retry loops (no schema enforcement)
- No programmatic API (can't import protocol operations as library)
- Duplicate implementations (bash scripts for CLI, Go parser for web UI)
- Reactive error detection (stubs/conflicts found after merge, not prevented)

**Root cause:** Bash scripts can't provide type safety, schema validation, or importable operations.

---

## Core Solution

**SDK as Protocol API:** Implement protocol operations in Go as an importable library.

**Benefits:**
1. **Structured types** - `IMPLManifest`, `Wave`, `Agent` with schema validation
2. **Single implementation** - Write once (SDK), use everywhere (CLI/web/external)
3. **Deterministic validation** - I1-I6 invariants enforced by code, not regex
4. **Programmatic access** - External tools can import SDK as library

**Not a rewrite:** Claude-as-orchestrator stays primary. SDK provides data operations, not orchestration logic.

---

## Architectural Decisions

### Decision 1: Claude-as-Orchestrator (Primary Model)

**Rationale:** Interactive error recovery requires conversation, not exit codes.

**Example from recent Wave 1 execution:**
- **Error:** Agent B created duplicate `NewWorkshop()` declaration
- **Programmatic response:** "redeclared in this block" → Exit code 1
- **Claude response:** Read both files, understood Agent B's temporary stub vs Agent A's production code, removed stub

**Conclusion:** Unexpected errors need semantic understanding. Claude provides this through conversational analysis. Pure Go orchestrator would need pre-coded handlers for every error type (impossible to enumerate).

**Primary workflow:**
```
User: /saw wave
  ↓
Claude (via skill): Coordinates execution
  ↓
Calls SDK operations: saw validate, saw extract-context, etc.
  ↓
Launches agents: Agent tool
  ↓
Monitors progress: Reads completion reports
  ↓
Handles errors: Conversational recovery
  ↓
Merges results: saw merge-wave
```

**Alternative workflows supported:**
- **Standalone CLI:** `saw wave IMPL-foo.yaml` works but has limited error recovery (exits on first failure)
- **Programmatic:** External tools can import SDK and orchestrate programmatically
- **Web UI:** HTTP handlers wrap SDK operations for browser-based interaction

**Primary remains CLI with Claude** because interactivity is critical for production use.

### Decision 2: Atomic Operations Model

**Each SDK operation is single-purpose:**
- **Input:** Well-defined parameters (manifest path, agent ID, etc.)
- **Output:** Structured data (JSON, exit code, error details)
- **Responsibility:** One thing (validate structure, extract context, set completion)

**Orchestrator coordinates by calling operations in sequence:**

```bash
# Orchestrator (skill) workflow

# 1. Atomic operation: validate
saw validate "$manifest_path" || exit 1

# 2. Atomic operation: get current wave
current_wave=$(saw current-wave "$manifest_path")

# 3. For each agent...
for agent_id in A B C D; do
    # Atomic operation: extract context
    context=$(saw extract-context "$manifest_path" "$agent_id")

    # Orchestrator responsibility: launch agent
    claude agent --type wave-agent --prompt "$context"

    # Agent writes completion-report.yaml

    # Atomic operation: register completion
    saw set-completion "$manifest_path" "$agent_id" < completion-report.yaml
done

# 4. Atomic operation: merge wave
saw merge-wave "$manifest_path" "$current_wave"
```

**Contract for each operation:**

| Operation | Input | Output (stdout) | Output (stderr) | Exit Code |
|-----------|-------|-----------------|-----------------|-----------|
| `saw validate <manifest>` | Manifest path | Success message | Structured errors (JSON) | 0=valid, 1=invalid |
| `saw extract-context <manifest> <agent>` | Manifest + agent ID | Agent context (JSON) | Error details | 0=success, 1=not found |
| `saw current-wave <manifest>` | Manifest path | Wave number | Error details | 0=success, 1=no pending |
| `saw set-completion <manifest> <agent>` | Manifest + agent + stdin (YAML report) | Success message | Error details | 0=success, 1=failed |
| `saw merge-wave <manifest> <wave>` | Manifest + wave number | Merge status | Conflict details | 0=success, 1=conflicts |
| `saw render <manifest>` | Manifest path | Markdown (human-readable view) | Error details | 0=success, 1=failed |

**Benefit:** Orchestrator makes decisions, operations are deterministic. Easy to test each operation in isolation.

### Decision 3: SDK Enables Code Reuse

**One implementation, multiple interfaces:**

```
┌─────────────────────────────────────────────┐
│  SDK (scout-and-wave-go/pkg/protocol)      │
│  - Load(path) → Manifest                   │
│  - Validate(manifest) → error              │
│  - ExtractAgentContext(...) → Context      │
│  - SetCompletionReport(...) → error        │
│                                             │
│  Written once in Go                         │
└─────────────────────────────────────────────┘
                    ↓
        ┌───────────┴───────────┬─────────────────┐
        ↓                       ↓                 ↓
┌──────────────┐      ┌──────────────┐   ┌──────────────┐
│ CLI Binary   │      │ Web UI       │   │ External     │
│              │      │              │   │ Tools        │
│ Wraps SDK    │      │ Imports SDK  │   │              │
│ as shell     │      │ for HTTP     │   │ Import SDK   │
│ commands     │      │ handlers     │   │ as library   │
│              │      │              │   │              │
│ saw validate │      │ GET /api/    │   │ import       │
│ saw extract  │      │ POST /api/   │   │ "...protocol"│
└──────────────┘      └──────────────┘   └──────────────┘
```

**Example: Validation logic**

**Implemented once in SDK:**
```go
// pkg/protocol/validation.go
func Validate(m *Manifest) error {
    // I1: Disjoint file ownership
    seen := make(map[string]string)
    for _, fo := range m.FileOwnership {
        if owner, exists := seen[fo.File]; exists {
            return fmt.Errorf("I1 violation: %s owned by both %s and %s",
                fo.File, owner, fo.Agent)
        }
        seen[fo.File] = fo.Agent
    }

    // I2-I6 checks...

    return nil
}
```

**Used by CLI binary:**
```go
// scout-and-wave-web/cmd/saw/validate.go
func validateCommand(c *cli.Context) error {
    manifest, err := protocol.Load(c.Args().First())
    if err != nil {
        return err
    }
    return protocol.Validate(manifest)  // ← SDK function
}
```

**Used by web UI:**
```go
// scout-and-wave-web/pkg/api/impl.go
func ValidateIMPL(c *gin.Context) {
    var manifest protocol.Manifest
    c.BindJSON(&manifest)

    if err := protocol.Validate(&manifest); err != nil {  // ← Same SDK function
        c.JSON(400, gin.H{"errors": err})
        return
    }
    c.JSON(200, gin.H{"status": "valid"})
}
```

**Used by external tool:**
```go
// Example: IDE plugin
import "github.com/blackwell-systems/scout-and-wave-go/pkg/protocol"

func lintIMPLDoc(path string) error {
    manifest, _ := protocol.Load(path)
    return protocol.Validate(manifest)  // ← Same SDK function
}
```

**Result:** Protocol rule change updates one place (SDK), all consumers get the fix.

**Current problem:** Bash scripts (`validate-impl.sh`) and Go parser (`pkg/protocol/parser.go`) are separate implementations that can diverge.

---


## Execution Contexts and Backend Configuration

**Critical architectural complexity:** The orchestrator (where `/saw` skill runs) and agents (where wave work executes) can use **different backends**. Understanding this split is essential for deployment flexibility.

### Two Execution Layers

#### Orchestrator Layer (Who Coordinates Wave Execution)

**Three orchestration modes:**

**Mode 1: CLI with Skill (Interactive)**

```bash
# User invokes skill in Claude Code CLI
/saw wave
```

**Orchestrator:** Claude (me) interprets skill instructions (bash)
**Backend options:**
- Max Plan: `claude` CLI with subscription
- Bedrock: `claude` CLI with `ANTHROPIC_BEDROCK_ENABLED=true`

**Key characteristics:**
- Conversational error recovery (Claude can pause, analyze, ask)
- Human-in-the-loop at review checkpoints
- Calls protocol SDK via CLI binary (`saw validate`, etc.)
- Launches agents via Agent tool

**Mode 2: Web UI (Browser-Based)**

```
User clicks "Start Wave" button in browser
  ↓
HTTP request to saw server
  ↓
Server orchestrates (Go code in scout-and-wave-web)
```

**Orchestrator:** HTTP handler (Go program)
**Backend options:** Server can launch agents using any backend (Bedrock/API/CLI/OpenAI)

**Key characteristics:**
- No Claude involved in orchestration logic (pure Go)
- Review checkpoints via UI forms/buttons
- Calls protocol SDK directly (Go imports)
- Launches agents programmatically (backend.RunStreamingWithTools)

**Mode 3: Programmatic (Go Application)**

```go
// Custom application orchestrates
import "github.com/blackwell-systems/scout-and-wave-go/pkg/protocol"

manifest, _ := protocol.Load("IMPL-foo.yaml")
orchestrator.RunWave(manifest, backendConfig)
```

**Orchestrator:** Your Go application
**Backend options:** Configure any backend (Bedrock/API/CLI/OpenAI)

**Key characteristics:**
- Fully automated (no human checkpoints unless you build them)
- Calls protocol SDK directly (library import)
- Launches agents programmatically
- Useful for CI/CD, batch processing, automated testing

**Key point:** The orchestrator layer is **who coordinates** (CLI skill / web server / Go app), separate from **who executes agents** (Bedrock / API / CLI / OpenAI).

#### Agent Layer (Where Wave Work Executes)

**Agents are spawned by the orchestrator.** Each agent can use a **different backend** than the orchestrator.

**Agent backend options:**

| Backend | Configuration | Use Case |
|---------|--------------|----------|
| **Anthropic API** | `agent_backend: api`<br>`ANTHROPIC_API_KEY=sk-ant-...` | Direct billing to Anthropic account |
| **AWS Bedrock** | `agent_backend: bedrock`<br>AWS credentials | Enterprise AWS deployments |
| **Claude Code CLI** | `agent_backend: cli`<br>`claude` binary available | Same as orchestrator (default) |
| **OpenAI-compatible** | `agent_backend: openai`<br>`OPENAI_API_KEY=...` | Alternative models (GPT-4, Groq, Ollama) |

**Key point:** Orchestrator and agents are decoupled. You can coordinate with Max Plan Claude while agents execute on Bedrock.

### Four Main Execution Scenarios

**Note:** These scenarios show CLI skill examples, but the same backend combinations work with web UI and programmatic Go orchestration. The key is orchestrator backend (how coordination runs) vs agent backend (how work executes).

#### Scenario 1: Max Plan Orchestrator + Max Plan Agents

**Setup:**
```bash
# Your Claude Code session (default configuration)
/saw wave
```

**Execution:**
```
Orchestrator: Max Plan subscription
  ↓ spawns agents
Agents: CLI backend (claude binary, also Max Plan)
  ↓ execute in worktrees
Result: Unified billing through Max Plan
```

**When to use:** Simple setup, all costs on one subscription.

#### Scenario 2: Max Plan Orchestrator + Bedrock Agents

**Setup:**
```bash
# Orchestrator on Max Plan
export AWS_REGION=us-east-1
/saw wave
```

**Configuration:**
```yaml
# manifest or config
agent_backend: bedrock
bedrock_model: anthropic.claude-sonnet-4-6-v1
```

**Execution:**
```
Orchestrator: Max Plan (you coordinate)
  ↓ spawns agents
Agents: AWS Bedrock backend (pkg/agent/backend/bedrock/)
  ↓ uses AWS credentials
  ↓ execute in worktrees
Result: Orchestration on Max Plan, compute on AWS
```

**When to use:** Heavy compute (many agents, long-running) where AWS pricing is better. You coordinate with Max Plan but offload agent work to Bedrock.

#### Scenario 3: Bedrock Orchestrator + Bedrock Agents

**Setup:**
```bash
# Configure Claude Code for Bedrock
export ANTHROPIC_BEDROCK_ENABLED=true
export AWS_REGION=us-east-1

/saw wave
```

**Execution:**
```
Orchestrator: AWS Bedrock
  ↓ spawns agents
Agents: AWS Bedrock (same)
  ↓ execute in worktrees
Result: Unified AWS billing
```

**When to use:** Enterprise AWS-only deployments. All costs on AWS invoice.

#### Scenario 4: Max Plan Orchestrator + Direct API Agents

**Setup:**
```bash
# Orchestrator on Max Plan
export ANTHROPIC_API_KEY=sk-ant-...
/saw wave
```

**Configuration:**
```yaml
agent_backend: api
api_key: ${ANTHROPIC_API_KEY}
```

**Execution:**
```
Orchestrator: Max Plan
  ↓ spawns agents
Agents: Anthropic API backend (pkg/agent/backend/api/)
  ↓ direct API calls with your key
  ↓ execute in worktrees
Result: Orchestration on Max Plan, agents billed to API account
```

**When to use:** Fine-grained cost control. Track agent API usage separately from orchestration.

### How Protocol SDK Fits In

**The protocol SDK is backend-agnostic** - it manages IMPL manifests (YAML parsing, validation, context extraction), not agent execution.

**Orchestrator uses SDK:**
```bash
# Skill (bash) calls protocol SDK operations
saw validate "$impl_path"          # Load + validate manifest
saw extract-context "$impl_path" "$agent_id"  # Get agent payload
saw set-completion "$impl_path" "$agent_id"   # Register report
```

**These operations work identically regardless of:**
- Whether orchestrator runs on Max Plan or Bedrock
- Whether agents use Bedrock, API, CLI, or OpenAI
- Whether invoked via skill, web UI, or programmatic Go

**Agent execution is separate:**
```
Skill launches agent:
  claude agent --type wave-agent --prompt "$context"
  (or programmatic: backend.RunStreamingWithTools(...))
    ↓
Agent uses configured backend:
  - Bedrock: AWS SDK v2, InvokeModelWithResponseStream
  - API: Raw HTTP to api.anthropic.com/v1/messages
  - CLI: Subprocess execution of claude binary
  - OpenAI: Raw HTTP to OpenAI-compatible endpoint
    ↓
Agent executes tools in worktree:
  - file:read, file:write, bash (via Workshop)
    ↓
Agent writes completion report:
  - completion-report.yaml in worktree
    ↓
Skill registers it via SDK:
  saw set-completion (protocol SDK updates manifest)
```

### Configuration Approach

**Backend selection:**

```yaml
# In IMPL manifest or orchestrator config file

# Agent backend configuration
agent_backend: "bedrock"  # "api" | "bedrock" | "cli" | "openai"

# Backend-specific settings
bedrock:
  region: "us-east-1"
  model: "anthropic.claude-sonnet-4-6-v1"
  max_tokens: 4096

api:
  api_key: "${ANTHROPIC_API_KEY}"
  model: "claude-sonnet-4-6"
  max_tokens: 4096

openai:
  api_key: "${OPENAI_API_KEY}"
  base_url: "https://api.openai.com/v1"
  model: "gpt-4"
  max_tokens: 4096

cli:
  binary_path: "/usr/local/bin/claude"
  model: "claude-sonnet-4-6"
```

**Per-agent backend override:**

```yaml
# In IMPL manifest
waves:
  - number: 1
    agents:
      - id: A
        task: "Complex analysis requiring Claude"
        model: "claude-sonnet-4-6"
        backend: "api"  # Override: use API for this agent

      - id: B
        task: "Simple code generation"
        model: "gpt-4"
        backend: "openai"  # Override: use OpenAI for this agent
```

**Environment-based configuration:**

```bash
# Override via environment variables
export SAW_AGENT_BACKEND=bedrock
export SAW_BEDROCK_REGION=us-west-2
export SAW_BEDROCK_MODEL=anthropic.claude-sonnet-4-6-v1

/saw wave
```

### Why This Complexity Matters

**1. Cost optimization:**
- Coordinate with Max Plan (fixed cost)
- Execute heavy work on Bedrock (pay-per-use)
- Result: Predictable orchestration cost, scale agent compute as needed

**2. Compliance:**
- Some enterprises require AWS-only deployments
- Bedrock orchestrator + Bedrock agents = fully AWS
- No external API calls

**3. Model flexibility:**
- Use Claude for orchestration (best at coordination)
- Use GPT-4 for specific agents (if specialized task benefits)
- Use local Ollama for testing (no API costs)

**4. Development workflow:**
- Develop with Max Plan (fast, interactive)
- Test with Bedrock (production-like)
- CI/CD with API keys (automated, no interactive CLI)

### SDK Design Implications

**Protocol SDK must be backend-agnostic:**

```go
// Good: No backend assumptions
func Load(path string) (*IMPLManifest, error) {
    data, _ := os.ReadFile(path)
    var manifest IMPLManifest
    yaml.Unmarshal(data, &manifest)
    return &manifest, nil
}

// Good: Validates structure only
func Validate(m *IMPLManifest) error {
    // I1-I6 checks
    // No backend-specific logic
}

// Bad: Would break backend flexibility
func ValidateWithBackend(m *IMPLManifest, backend string) error {
    if backend == "bedrock" {
        // Special validation for Bedrock
    }
    // This couples SDK to backend implementation
}
```

**Orchestrator uses SDK regardless of backend:**

```bash
# Works with Max Plan orchestrator
/saw wave
  → saw validate (protocol SDK)
  → launch agents on Bedrock

# Works with Bedrock orchestrator
/saw wave
  → saw validate (same protocol SDK)
  → launch agents on API

# Works with programmatic Go
saw.Wave(manifest, backend)
  → protocol.Validate (same SDK)
  → launch agents on OpenAI
```

**Result:** SDK is a pure data layer (YAML ↔ Go structs), orchestrator and agents are execution layers (backend-specific).

### Testing Matrix

**SDK operations must work in all contexts:**

| Orchestrator Backend | Agent Backend | SDK Operations |
|---------------------|---------------|----------------|
| Max Plan | CLI (Max Plan) | ✓ Load, Validate, Extract |
| Max Plan | Bedrock | ✓ Load, Validate, Extract |
| Max Plan | API | ✓ Load, Validate, Extract |
| Bedrock | Bedrock | ✓ Load, Validate, Extract |
| Bedrock | API | ✓ Load, Validate, Extract |
| Programmatic Go | Any | ✓ Load, Validate, Extract |

**All combinations must work** because SDK is backend-agnostic.

---

## Orchestrator Backend Abstraction

**Critical requirement:** The orchestrator (who coordinates wave execution) must be as pluggable as agents. Full interoperability means both orchestrator AND agents can use any backend.

### Current Limitation

**Agents are pluggable:**
```yaml
waves:
  - agents:
    - id: A
      backend: bedrock
    - id: B
      backend: openai
```

**Orchestrator is locked:**
```bash
/saw wave  # Always uses YOUR Claude session (Max Plan or Bedrock)
# Cannot specify: orchestrator should use GPT-4
```

**Architectural inconsistency:** If we support agent backend flexibility, orchestrator backend must be flexible too.

### Proposed Solution

**Orchestrator backend configuration:**

```yaml
# scout-and-wave config or IMPL manifest

orchestrator:
  backend: api
  model: claude-sonnet-4-6
  api_key: ${ORCHESTRATOR_API_KEY}
  # OR
  backend: bedrock
  region: us-east-1
  model: anthropic.claude-sonnet-4-6-v1
  # OR
  backend: openai
  model: gpt-4
  api_key: ${OPENAI_API_KEY}

agents:
  default_backend: bedrock  # Different from orchestrator
  bedrock:
    region: us-west-2
    model: anthropic.claude-sonnet-4-6-v1
```

**Result:** Orchestrator and agents use independent backends. Mix and match freely.

### Use Cases

**1. Cost separation**
```yaml
orchestrator:
  backend: api
  api_key: ${PERSONAL_KEY}  # Your API key
agents:
  backend: bedrock
  # Uses company AWS account
```
Result: You pay for coordination, company pays for compute.

**2. Compliance**
```yaml
orchestrator:
  backend: bedrock
  region: us-gov-west-1  # GovCloud
agents:
  backend: bedrock
  region: us-gov-west-1  # Same region
```
Result: All execution in government cloud.

**3. Multi-tenancy**
```yaml
orchestrator:
  backend: api
  api_key: ${ADMIN_KEY}
agents:
  per_agent:
    A:
      backend: api
      api_key: ${TENANT_A_KEY}
    B:
      backend: api
      api_key: ${TENANT_B_KEY}
```
Result: Per-tenant billing and access control.

**4. Development workflow**
```yaml
orchestrator:
  backend: openai
  model: gpt-4  # Fast, cheap for coordination
agents:
  backend: api
  model: claude-sonnet-4-6  # Quality for implementation
```
Result: Optimize cost vs quality per layer.

### Implementation Requirements

#### 1. Generic Orchestration Interface

**Current skill is Claude-specific:**
```bash
claude agent --type wave-agent ...  # Claude Code Agent tool
```

**Must become backend-agnostic:**
```bash
saw launch-agent "$manifest" "$agent_id" --backend "${AGENT_BACKEND}"
# Works with any orchestrator backend
```

**SDK provides generic launch:**
```go
// pkg/orchestrator/orchestrator.go
type Orchestrator struct {
    orchestratorBackend Backend  // For coordination
    agentBackendConfig  BackendConfig  // For agents
    sdk                 *protocol.SDK
}

func (o *Orchestrator) RunWave(manifest *protocol.IMPLManifest) error {
    // Orchestrator uses its backend for coordination tasks:
    // - Reading IMPL doc
    // - Making decisions (proceed? retry? escalate?)
    // - Writing status updates

    // Agents use their configured backend:
    for _, agent := range manifest.CurrentWave().Agents {
        agentBackend := o.selectAgentBackend(agent)
        o.launchAgent(agent, agentBackend)
    }
}
```

#### 2. Orchestration as Structured Operations

**Orchestrator operations must work with any backend:**

```go
// Orchestration operations (SDK provides these)
protocol.Load(manifest_path)           // Read manifest
protocol.Validate(manifest)            // Check I1-I6
protocol.ExtractAgentContext(...)      // Get agent payload
protocol.SetCompletionReport(...)      // Register completion

// Coordination decisions (orchestrator backend provides these)
orchestratorBackend.Analyze(context)   // "Should I proceed?"
orchestratorBackend.Resolve(error)     // "How to fix this?"
orchestratorBackend.Review(manifest)   // "Is this ready to merge?"
```

**Key distinction:**
- **Protocol SDK:** Data operations (YAML parsing, validation) - backend-agnostic
- **Orchestrator backend:** Coordination logic (decisions, recovery) - uses configured LLM

#### 3. Skill Becomes Backend-Agnostic

**Current skill:**
```bash
# Claude-specific
/saw wave
  → Claude interprets bash instructions
  → Calls: claude agent (Claude Code tool)
```

**Revised skill:**
```bash
# Generic coordination script
saw orchestrate wave "$manifest"
  → Uses configured orchestrator backend
  → Launches agents with configured agent backends
```

**Execution modes:**

**Mode 1: CLI with skill**
```bash
# User invokes
/saw wave

# Skill calls
saw orchestrate wave IMPL-foo.yaml \
  --orchestrator-backend api \
  --agent-backend bedrock
```

**Mode 2: Web UI**
```
User clicks "Start Wave"
  ↓
HTTP POST /api/wave/:slug/start
  ↓
orchestrator.RunWave(manifest, orchestratorBackend, agentBackendConfig)
```

**Mode 3: Programmatic**
```go
orchestrator := orchestrator.New(
    orchestratorBackend: api.New(apiKey),
    agentBackendConfig: bedrock.NewConfig(region),
)
orchestrator.RunWave(manifest)
```

#### 4. Backend Adapter Pattern (Same as Agents)

**Orchestrator backends implement common interface:**

```go
// pkg/orchestrator/backend/backend.go
type OrchestratorBackend interface {
    // Coordination operations
    Decide(ctx context.Context, decision Decision) (string, error)
    Analyze(ctx context.Context, analysis Analysis) (string, error)
    Resolve(ctx context.Context, error Error) (string, error)
}

type Decision struct {
    Type    string  // "proceed" | "retry" | "escalate"
    Context string  // Situation description
    Options []string  // Available choices
}
```

**Implementations:**

```go
// pkg/orchestrator/backend/api/
type AnthropicOrchestratorBackend struct {
    client *anthropic.Client
}

func (b *AnthropicOrchestratorBackend) Decide(ctx context.Context, d Decision) (string, error) {
    // Call Anthropic API with decision prompt
}

// pkg/orchestrator/backend/openai/
type OpenAIOrchestratorBackend struct {
    client *openai.Client
}

func (b *OpenAIOrchestratorBackend) Decide(ctx context.Context, d Decision) (string, error) {
    // Call OpenAI API with decision prompt
}
```

**Same pattern as agent backends** - just different interface (coordination vs execution).

### Configuration Schema

**Complete configuration:**

```yaml
# Orchestrator backend
orchestrator:
  backend: api  # "api" | "bedrock" | "openai" | "cli"

  # Backend-specific config
  api:
    api_key: ${ORCHESTRATOR_API_KEY}
    model: claude-sonnet-4-6
    max_tokens: 4096

  bedrock:
    region: us-east-1
    model: anthropic.claude-sonnet-4-6-v1

  openai:
    api_key: ${OPENAI_API_KEY}
    model: gpt-4
    base_url: https://api.openai.com/v1

# Agent backends (already designed)
agents:
  default_backend: bedrock

  bedrock:
    region: us-west-2
    model: anthropic.claude-sonnet-4-6-v1

  api:
    api_key: ${AGENT_API_KEY}
    model: claude-sonnet-4-6

  # Per-agent overrides
  per_agent:
    A:
      backend: api  # Agent A uses direct API
    B:
      backend: bedrock  # Agent B uses Bedrock
    C:
      backend: openai
      model: gpt-4  # Agent C uses OpenAI
```

### Migration Impact

**Scope additions to proposal:**

**scout-and-wave-go (SDK):**
- Already backend-agnostic ✓
- No changes needed

**scout-and-wave-web (orchestrator):**
- `pkg/orchestrator/backend/` - Orchestrator backend interface + implementations
- `pkg/orchestrator/orchestrator.go` - Use orchestrator backend for coordination
- `cmd/saw/orchestrate.go` - New command: `saw orchestrate wave <manifest>`
- Backend routing based on config (same pattern as agent backends)

**scout-and-wave (skill):**
- Update skill to call `saw orchestrate` instead of direct agent launches
- Orchestrator backend config in skill header
- Remains bash-based but backend-agnostic

### Why This Is Critical

**1. Architectural consistency**
- Agents pluggable → Orchestrator must be pluggable
- Otherwise "interoperability" is incomplete

**2. Cost optimization**
- Orchestrator: Low-cost backend (GPT-4, local Ollama)
- Agents: High-quality backend (Claude)
- Split billing and optimize per layer

**3. Compliance requirements**
- Some enterprises mandate AWS-only (Bedrock orchestrator + Bedrock agents)
- Others mandate API keys (no CLI, no Max Plan dependencies)

**4. Multi-tenancy**
- Different API keys per agent
- Central orchestrator with tenant-specific agent execution

**5. Development flexibility**
- Test orchestrator with cheap model
- Production agents with production model
- No coupling between layers

### Testing Requirements

**All orchestrator + agent combinations must work:**

| Orchestrator Backend | Agent Backend | Status |
|---------------------|---------------|--------|
| API (Claude) | Bedrock | ✓ |
| API (Claude) | API (Claude) | ✓ |
| API (Claude) | OpenAI | ✓ |
| Bedrock | Bedrock | ✓ |
| Bedrock | API | ✓ |
| OpenAI | Bedrock | ✓ |
| OpenAI | OpenAI | ✓ |
| CLI (Max Plan) | Any | ✓ |

**Integration tests validate:**
- Orchestrator backend handles coordination (decisions, analysis)
- Agent backends handle execution (tool calls, code generation)
- Protocol SDK operations work identically regardless of backends
- Configuration routing selects correct backend per layer

---
## Architecture

### Layer 1: SDK (scout-and-wave-go/pkg/protocol)

**Purpose:** Canonical implementation of protocol operations.

**Core types:**
```go
type IMPLManifest struct {
    Title           string              `yaml:"title" json:"title"`
    FeatureSlug     string              `yaml:"feature_slug" json:"feature_slug"`
    Verdict         string              `yaml:"verdict" json:"verdict"` // "SUITABLE" | "NOT_SUITABLE"
    FileOwnership   []FileOwnership     `yaml:"file_ownership" json:"file_ownership"`
    InterfaceContracts []Contract       `yaml:"interface_contracts" json:"interface_contracts"`
    Waves           []Wave              `yaml:"waves" json:"waves"`
    QualityGates    QualityGateConfig   `yaml:"quality_gates" json:"quality_gates"`
    Scaffolds       []ScaffoldFile      `yaml:"scaffolds" json:"scaffolds"`
}

type FileOwnership struct {
    File      string   `yaml:"file" json:"file"`
    Agent     string   `yaml:"agent" json:"agent"`
    Wave      int      `yaml:"wave" json:"wave"`
    DependsOn []string `yaml:"depends_on" json:"depends_on"`
}

type Wave struct {
    Number int     `yaml:"number" json:"number"`
    Agents []Agent `yaml:"agents" json:"agents"`
}

type Agent struct {
    ID           string   `yaml:"id" json:"id"`
    Task         string   `yaml:"task" json:"task"`
    Files        []string `yaml:"files" json:"files"`
    Dependencies []string `yaml:"dependencies" json:"dependencies"`
    Model        string   `yaml:"model,omitempty" json:"model,omitempty"`
}

type CompletionReport struct {
    Status             string      `yaml:"status" json:"status"` // "complete" | "partial" | "blocked"
    FailureType        string      `yaml:"failure_type,omitempty" json:"failure_type,omitempty"`
    FilesChanged       []string    `yaml:"files_changed" json:"files_changed"`
    FilesCreated       []string    `yaml:"files_created" json:"files_created"`
    InterfaceDeviations []Deviation `yaml:"interface_deviations" json:"interface_deviations"`
    OutOfScopeDeps     []string    `yaml:"out_of_scope_deps" json:"out_of_scope_deps"`
    TestsAdded         []string    `yaml:"tests_added" json:"tests_added"`
    Verification       string      `yaml:"verification" json:"verification"`
}
```

**Core operations:**
```go
// Load manifest from YAML/JSON
func Load(path string) (*IMPLManifest, error)

// Validate structure + invariants (I1-I6)
func Validate(m *IMPLManifest) error

// Extract per-agent context (E23)
func ExtractAgentContext(m *IMPLManifest, agentID string) (*AgentContext, error)

// Generate human-readable markdown view
func GenerateMarkdown(m *IMPLManifest) (string, error)

// Register completion report
func (m *IMPLManifest) SetCompletionReport(agentID string, report CompletionReport) error

// Get current pending wave
func (m *IMPLManifest) CurrentWave() *Wave

// Save manifest back to disk
func (m *IMPLManifest) Save(path string) error
```

**Files:**
- `pkg/protocol/manifest.go` - Core types with YAML/JSON tags
- `pkg/protocol/validation.go` - I1-I6 invariant enforcement
- `pkg/protocol/extract.go` - Agent context extraction (E23)
- `pkg/protocol/render.go` - Markdown generation for human review
- `pkg/protocol/migrate.go` - Utility to convert existing markdown IMPL docs to YAML

### Layer 2A: CLI Binary (scout-and-wave-web/cmd/saw)

**Purpose:** Thin wrapper exposing SDK operations as shell commands.

**Why needed:** The skill (bash-based coordination) can't import Go packages. CLI binary is the bridge.

**Commands:**
```bash
# Validate manifest structure + invariants
saw validate docs/IMPL/IMPL-tool-refactor.yaml
# Exit 0 if valid, 1 if invalid
# stderr: structured errors (JSON)

# Extract structured agent context (E23)
saw extract-context docs/IMPL/IMPL-tool-refactor.yaml agent-A
# stdout: JSON agent context payload
# Exit 0 if success, 1 if agent not found

# Register completion report
saw set-completion docs/IMPL/IMPL-tool-refactor.yaml agent-A < completion-report.yaml
# stdin: YAML completion report
# Exit 0 if success, 1 if validation failed

# Get current pending wave number
saw current-wave docs/IMPL/IMPL-tool-refactor.yaml
# stdout: wave number (integer)
# Exit 0 if pending wave exists, 1 if all complete

# Perform merge operations for wave
saw merge-wave docs/IMPL/IMPL-tool-refactor.yaml 1
# stdout: merge status
# stderr: conflict details (if any)
# Exit 0 if clean merge, 1 if conflicts

# Generate human-readable markdown view
saw render docs/IMPL/IMPL-tool-refactor.yaml > IMPL-tool-refactor.md
# stdout: markdown document
# Exit 0 if success, 1 if render failed

# Migrate existing markdown IMPL doc to YAML manifest
saw migrate docs/IMPL/IMPL-old.md > IMPL-new.yaml
# stdout: YAML manifest
# Exit 0 if success, 1 if parse failed
```

**Implementation pattern:**
```go
// cmd/saw/validate.go
func validateCommand(c *cli.Context) error {
    manifestPath := c.Args().First()

    // Load via SDK
    manifest, err := protocol.Load(manifestPath)
    if err != nil {
        // Structured error output
        json.NewEncoder(os.Stderr).Encode(map[string]string{
            "type": "load_error",
            "message": err.Error(),
        })
        return cli.Exit("", 1)
    }

    // Validate via SDK
    if err := protocol.Validate(manifest); err != nil {
        json.NewEncoder(os.Stderr).Encode(err)
        return cli.Exit("", 1)
    }

    fmt.Println("✓ Manifest valid")
    return nil
}
```

### Layer 2B: Skill Coordination (~/.claude/skills/saw/saw.md)

**Purpose:** High-level orchestration logic coordinating SDK operations, git operations, and agent launches.

**Remains bash-based** because:
- Claude interprets it (natural language + bash instructions)
- Interactive error recovery (Claude can pause, ask, investigate)
- Git operations (`git worktree`, `git merge`) are bash anyway
- Agent launching uses Agent tool (Claude-specific)

**Example skill flow:**
```bash
# ~/.claude/skills/saw/saw.md
# /saw wave execution flow

impl_path="docs/IMPL/IMPL-${feature_slug}.yaml"

# ── Step 1: Load and validate ──────────────────────────
if ! saw validate "$impl_path" 2>validation-errors.json; then
    echo "❌ Manifest validation failed:"
    cat validation-errors.json
    # Claude: Read errors, analyze, suggest fixes
    exit 1
fi

echo "✓ Manifest valid"

# ── Step 2: Get current pending wave ───────────────────
current_wave=$(saw current-wave "$impl_path")
if [ -z "$current_wave" ]; then
    echo "✓ All waves complete"
    exit 0
fi

echo "Starting Wave $current_wave..."

# ── Step 3: Create worktrees (git operations) ──────────
# Claude handles this with git commands
agents=(A B C D)  # Or parse from manifest
for agent_id in "${agents[@]}"; do
    branch="wave${current_wave}-agent-${agent_id}"
    worktree_path=".claude/worktrees/$branch"

    git worktree add "$worktree_path" -b "$branch"
done

# ── Step 4: Launch agents in parallel ──────────────────
for agent_id in "${agents[@]}"; do
    # Extract structured agent context (SDK operation)
    agent_context=$(saw extract-context "$impl_path" "$agent_id")

    # Launch agent (Claude's Agent tool)
    # This runs in background, Claude monitors progress
    claude agent \
        --type wave-agent \
        --prompt "$agent_context" \
        --description "[SAW:wave${current_wave}:agent-${agent_id}] ..." \
        --run-in-background true
done

# ── Step 5: Wait for agents to complete ────────────────
# Claude monitors agent output, detects completion

# ── Step 6: Register completion reports ────────────────
for agent_id in "${agents[@]}"; do
    branch="wave${current_wave}-agent-${agent_id}"
    worktree_path=".claude/worktrees/$branch"
    report_path="$worktree_path/completion-report.yaml"

    if [ -f "$report_path" ]; then
        # Register via SDK operation
        saw set-completion "$impl_path" "$agent_id" < "$report_path"
    else
        echo "⚠ Agent $agent_id: No completion report found"
        # Claude: Investigate, check agent output, decide how to handle
    fi
done

# ── Step 7: Verify all agents complete ─────────────────
# Claude: Read completion reports from manifest
# Check for status: partial or status: blocked
# If any failed, pause for investigation

# ── Step 8: Merge wave (SDK operation) ─────────────────
if ! saw merge-wave "$impl_path" "$current_wave" 2>merge-errors.json; then
    echo "❌ Merge failed:"
    cat merge-errors.json
    # Claude: Read conflicts, analyze, decide resolution strategy
    exit 1
fi

echo "✓ Wave $current_wave merged successfully"

# ── Step 9: Cleanup ─────────────────────────────────────
for agent_id in "${agents[@]}"; do
    branch="wave${current_wave}-agent-${agent_id}"
    worktree_path=".claude/worktrees/$branch"

    git worktree remove "$worktree_path" 2>/dev/null || rm -rf "$worktree_path"
    git branch -d "$branch" 2>/dev/null || true
done

echo "Next wave ready. Run /saw wave to continue."
```

**Key point:** Skill calls atomic operations (`saw validate`, `saw extract-context`, etc.) but coordinates the overall flow. Error handling is conversational - Claude reads error output, analyzes situation, suggests recovery.

### Layer 2C: Web UI (scout-and-wave-web/pkg/api)

**Purpose:** HTTP/REST interface to SDK operations for browser-based interaction.

**Example endpoints:**
```go
// pkg/api/impl.go

// GET /api/impl/:slug - Load and return manifest
func GetIMPL(c *gin.Context) {
    slug := c.Param("slug")
    path := fmt.Sprintf("docs/IMPL/IMPL-%s.yaml", slug)

    manifest, err := protocol.Load(path)  // ← SDK operation
    if err != nil {
        c.JSON(500, gin.H{"error": err.Error()})
        return
    }

    c.JSON(200, manifest)
}

// POST /api/impl/:slug/validate - Validate manifest
func ValidateIMPL(c *gin.Context) {
    var manifest protocol.IMPLManifest
    if err := c.BindJSON(&manifest); err != nil {
        c.JSON(400, gin.H{"error": err.Error()})
        return
    }

    if err := protocol.Validate(&manifest); err != nil {  // ← SDK operation
        c.JSON(400, gin.H{"errors": err})
        return
    }

    c.JSON(200, gin.H{"status": "valid"})
}

// GET /api/impl/:slug/wave/:number - Get wave details
func GetWave(c *gin.Context) {
    slug := c.Param("slug")
    waveNum, _ := strconv.Atoi(c.Param("number"))

    path := fmt.Sprintf("docs/IMPL/IMPL-%s.yaml", slug)
    manifest, err := protocol.Load(path)  // ← SDK operation
    if err != nil {
        c.JSON(500, gin.H{"error": err.Error()})
        return
    }

    for _, wave := range manifest.Waves {
        if wave.Number == waveNum {
            c.JSON(200, wave)
            return
        }
    }

    c.JSON(404, gin.H{"error": "wave not found"})
}

// POST /api/impl/:slug/agents/:id/complete - Register completion report
func CompleteAgent(c *gin.Context) {
    slug := c.Param("slug")
    agentID := c.Param("id")

    var report protocol.CompletionReport
    if err := c.BindJSON(&report); err != nil {
        c.JSON(400, gin.H{"error": err.Error()})
        return
    }

    path := fmt.Sprintf("docs/IMPL/IMPL-%s.yaml", slug)
    manifest, _ := protocol.Load(path)

    if err := manifest.SetCompletionReport(agentID, report); err != nil {  // ← SDK operation
        c.JSON(400, gin.H{"error": err.Error()})
        return
    }

    if err := manifest.Save(path); err != nil {  // ← SDK operation
        c.JSON(500, gin.H{"error": err.Error()})
        return
    }

    c.JSON(200, gin.H{"status": "registered"})
}
```

**Frontend usage:**
```typescript
// web/src/api/impl.ts
export async function loadManifest(slug: string): Promise<IMPLManifest> {
    const response = await fetch(`/api/impl/${slug}`);
    return response.json();  // Backend used SDK to load
}

export async function validateManifest(manifest: IMPLManifest): Promise<ValidationResult> {
    const response = await fetch('/api/impl/validate', {
        method: 'POST',
        headers: {'Content-Type': 'application/json'},
        body: JSON.stringify(manifest)
    });
    return response.json();  // Backend used SDK to validate
}
```

### Layer 3: Agent Execution

**Agents receive structured context payload (not markdown):**

**Before (prose prompt):**
```
Agent A, your task is to implement the Workshop interface and middleware stack.
Read docs/IMPL/IMPL-tool-refactor.md for file ownership, interface contracts, and dependencies.
Owned files:
- pkg/tools/workshop.go
- pkg/tools/middleware.go
Dependencies: Scaffold
```

**After (structured payload):**
```json
{
  "agent_id": "A",
  "task": "Implement Workshop interface and middleware stack",
  "files": [
    "pkg/tools/workshop.go",
    "pkg/tools/middleware.go"
  ],
  "dependencies": ["Scaffold"],
  "interface_contracts": [
    {
      "name": "Workshop",
      "definition": "type Workshop interface { ... }",
      "location": "pkg/tools/types.go"
    }
  ],
  "quality_gates": [
    {
      "type": "build",
      "command": "go build ./pkg/tools",
      "required": true
    }
  ],
  "impl_doc_path": "/abs/path/to/IMPL-tool-refactor.yaml"
}
```

**Agent returns structured completion report (not markdown):**

**Before (YAML block in markdown):**
````markdown
### Agent A — Completion Report

```yaml type=impl-completion-report
status: complete
files_created:
  - pkg/tools/workshop.go
  - pkg/tools/middleware.go
```
````

**After (standalone YAML file):**
```yaml
# .claude/worktrees/wave1-agent-A/completion-report.yaml
status: complete
repo: /Users/dayna.blackwell/code/scout-and-wave-go
worktree: .claude/worktrees/wave1-agent-A
branch: wave1-agent-A
commit: 83e6309
files_changed: []
files_created:
  - pkg/tools/workshop.go
  - pkg/tools/middleware.go
interface_deviations: []
out_of_scope_deps: []
tests_added: []
verification: PASS (go build ./pkg/tools && go vet ./pkg/tools)
```

**Registered via SDK:**
```bash
saw set-completion IMPL-tool-refactor.yaml agent-A < completion-report.yaml
```

---

## Manifest Format

**Source of truth:** `docs/IMPL/IMPL-<feature-slug>.yaml`

**Example:**
```yaml
title: "Tool System Refactoring"
feature_slug: "tool-refactor"
verdict: "SUITABLE"

suitability_assessment:
  risk: "low"
  complexity: "medium"
  test_command: "go test ./..."
  build_command: "go build ./..."

file_ownership:
  - file: pkg/tools/types.go
    agent: Scaffold
    wave: 0
    depends_on: []

  - file: pkg/tools/workshop.go
    agent: A
    wave: 1
    depends_on: [Scaffold]

  - file: pkg/tools/middleware.go
    agent: A
    wave: 1
    depends_on: [Scaffold]

  - file: pkg/tools/executors.go
    agent: B
    wave: 1
    depends_on: [Scaffold]

  - file: pkg/tools/adapters.go
    agent: C
    wave: 1
    depends_on: [Scaffold]

  - file: pkg/tools/workshop_test.go
    agent: D
    wave: 1
    depends_on: [A]

interface_contracts:
  - name: Workshop
    description: "Tool registration and namespace filtering"
    definition: |
      type Workshop interface {
          Register(tool Tool) error
          Get(name string) (Tool, bool)
          All() []Tool
          Namespace(prefix string) []Tool
      }
    location: pkg/tools/types.go

  - name: ToolExecutor
    description: "Stateful tool execution interface"
    definition: |
      type ToolExecutor interface {
          Execute(ctx context.Context, execCtx ExecutionContext, input map[string]interface{}) (string, error)
      }
    location: pkg/tools/types.go

waves:
  - number: 1
    agents:
      - id: A
        task: "Implement Workshop interface and middleware stack"
        files:
          - pkg/tools/workshop.go
          - pkg/tools/middleware.go
        dependencies: [Scaffold]
        model: "claude-sonnet-4-6"  # Optional per-agent override

      - id: B
        task: "Implement standard tool executors (Read, Write, List, Bash)"
        files:
          - pkg/tools/executors.go
          - pkg/tools/standard.go
        dependencies: [Scaffold]

      - id: C
        task: "Implement backend serialization adapters"
        files:
          - pkg/tools/adapters.go
        dependencies: [Scaffold]

      - id: D
        task: "Write comprehensive unit tests"
        files:
          - pkg/tools/workshop_test.go
          - pkg/tools/middleware_test.go
          - pkg/tools/adapters_test.go
        dependencies: [A, C]

quality_gates:
  level: standard
  gates:
    - type: build
      command: "go build ./..."
      required: true

    - type: lint
      command: "go vet ./..."
      required: true

    - type: test
      command: "go test ./..."
      required: true

scaffolds:
  - file: pkg/tools/types.go
    status: committed
    commit: abc123
    description: "Core interfaces for Workshop, ToolExecutor, Middleware, ToolAdapter"

completion_reports:
  A:
    status: complete
    commit: 83e6309
    files_created:
      - pkg/tools/workshop.go
      - pkg/tools/middleware.go
    verification: PASS

  B:
    status: complete
    commit: 54d5c03
    files_created:
      - pkg/tools/executors.go
      - pkg/tools/standard.go
    verification: PASS

  C:
    status: complete
    commit: 47605da
    files_created:
      - pkg/tools/adapters.go
    verification: PASS

  D:
    status: complete
    commit: a65cb9d
    files_created:
      - pkg/tools/workshop_test.go
      - pkg/tools/middleware_test.go
      - pkg/tools/adapters_test.go
    tests_added:
      - TestRegisterAndGet
      - TestMiddlewareStack
      - TestAnthropicAdapterSerialize
    verification: PASS
```

**Human-readable view:** Generated markdown for review
```bash
saw render IMPL-tool-refactor.yaml > IMPL-tool-refactor.md
```

---

## Migration Strategy

**Two-phase execution for incremental delivery and testing:**

---

### Phase 1: Protocol SDK Migration (Shippable Core)

**Goal:** Replace markdown parsing with structured YAML manifests and SDK operations. Orchestrator remains Claude-in-CLI (current model).

**Deliverable:** Protocol as SDK, agents pluggable, orchestrator fixed.

#### Wave 1: SDK Core Implementation

**scout-and-wave-go (SDK):**
- `pkg/protocol/manifest.go` - Core types (IMPLManifest, Wave, Agent, FileOwnership, CompletionReport)
- `pkg/protocol/validation.go` - I1-I6 invariant enforcement with structured errors
- `pkg/protocol/extract.go` - Agent context extraction (E23)
- `pkg/protocol/render.go` - Markdown generation for human review
- `pkg/protocol/migrate.go` - Markdown → YAML migration utility
- Comprehensive unit tests

**Success criteria:** SDK functions work in isolation, all unit tests pass.

#### Wave 2: CLI Binary (SDK Bridge)

**scout-and-wave-web (CLI commands):**
- `cmd/saw/validate.go` - `saw validate <manifest>` command
- `cmd/saw/extract.go` - `saw extract-context <manifest> <agent>` command
- `cmd/saw/completion.go` - `saw set-completion <manifest> <agent>` command
- `cmd/saw/wave.go` - `saw current-wave <manifest>` command
- `cmd/saw/merge.go` - `saw merge-wave <manifest> <wave>` command
- `cmd/saw/render.go` - `saw render <manifest>` command
- `cmd/saw/migrate.go` - `saw migrate <old-impl.md>` command
- Integration tests for each command

**Success criteria:** CLI binary wraps SDK operations, skill can call commands via bash.

#### Wave 3: Skill Migration

**scout-and-wave (skill updates):**
- Update `/saw` skill to call `saw validate` instead of `validate-impl.sh`
- Update skill to call `saw extract-context` for agent payloads
- Update skill to call `saw set-completion` for report registration
- Skill remains bash-based, orchestrator remains Claude
- Archive old bash scripts

**Success criteria:** `/saw wave` works with new CLI commands, agents execute normally.

#### Wave 4: Scout Agent Updates

**scout-and-wave (Scout changes):**
- Scout generates YAML manifest instead of markdown IMPL doc
- Manifest follows schema (enforced by SDK validation)
- Scout still runs as agent (no orchestrator changes)

**Success criteria:** Scout produces valid YAML, `saw validate` passes.

#### Wave 5: Web UI Integration

**scout-and-wave-web (HTTP + frontend):**
- Update HTTP handlers to import SDK directly (`protocol.Load`, `protocol.Validate`)
- Add manifest editor UI (YAML textarea with validation feedback)
- Update frontend components to render from structured data
- Manifest validation in browser (call POST /api/impl/validate)

**Success criteria:** Web UI loads manifests, validates in real-time, renders waves/agents.

**Phase 1 Checkpoint:** Protocol SDK deployed, tested in production. Agents pluggable, orchestrator fixed to Claude CLI. No orchestrator backend abstraction yet.

---

### Phase 2: Orchestrator Backend Abstraction (Full Pluggability)

**Goal:** Make orchestrator backend configurable (API, Bedrock, OpenAI, CLI). Complete interoperability - both orchestrator AND agents can use any backend.

**Deliverable:** Orchestrator pluggable, full backend matrix supported.

**Depends on:** Phase 1 complete and validated. SDK must be backend-agnostic (already designed this way).

#### Wave 2-1: Orchestrator Backend Interface

**scout-and-wave-go (orchestrator abstraction):**
- `pkg/orchestrator/backend/backend.go` - OrchestratorBackend interface
  ```go
  type OrchestratorBackend interface {
      Decide(ctx context.Context, decision Decision) (string, error)
      Analyze(ctx context.Context, analysis Analysis) (string, error)
      Resolve(ctx context.Context, error Error) (string, error)
  }
  ```
- Define coordination operation types (Decision, Analysis, Error)
- Unit tests for interface contract

**Success criteria:** Interface defined, contract clear, testable with mocks.

#### Wave 2-2: Backend Implementations

**scout-and-wave-go (backend implementations):**
- `pkg/orchestrator/backend/api/` - Anthropic API orchestrator backend
- `pkg/orchestrator/backend/bedrock/` - AWS Bedrock orchestrator backend
- `pkg/orchestrator/backend/openai/` - OpenAI-compatible orchestrator backend
- `pkg/orchestrator/backend/cli/` - Claude Code CLI orchestrator backend
- Integration tests for each backend

**Success criteria:** Each backend can handle coordination operations (decide, analyze, resolve).

#### Wave 2-3: Orchestrator Refactoring

**scout-and-wave-go (orchestrator updates):**
- Update `pkg/orchestrator/orchestrator.go` to use OrchestratorBackend
- Separate orchestrator backend from agent backend in Orchestrator struct
- Backend selection based on config
- Backward compatibility: default to CLI backend (current behavior)

**Success criteria:** Orchestrator uses configured backend for coordination, agents use configured backends for execution.

#### Wave 2-4: Configuration Schema

**scout-and-wave-go (config):**
- Add orchestrator backend config to IMPL manifest schema
- Add orchestrator backend config to orchestrator config file
- Environment variable overrides
- Validation: ensure orchestrator + agent backend combinations are valid

**Success criteria:** Config parsed correctly, backend routing works as specified.

#### Wave 2-5: CLI Orchestrate Command

**scout-and-wave-web (new command):**
- `cmd/saw/orchestrate.go` - `saw orchestrate wave <manifest>` command
- Uses orchestrator backend from config
- Launches agents with agent backend from config
- Replaces direct `claude agent` calls in skill

**Success criteria:** `saw orchestrate wave` works with any orchestrator backend, launches agents with any agent backend.

#### Wave 2-6: Skill Updates (Backend-Agnostic)

**scout-and-wave (skill updates):**
- Update skill to call `saw orchestrate wave` instead of direct agent launches
- Orchestrator backend config in skill header or env vars
- Remove Claude-specific assumptions (Agent tool calls)
- Skill becomes generic coordination script

**Success criteria:** Skill works with any orchestrator backend (API, Bedrock, OpenAI, CLI).

#### Wave 2-7: Web UI Orchestrator Config

**scout-and-wave-web (UI updates):**
- Add orchestrator backend selector in UI
- Update HTTP handlers to use orchestrator backend from config
- Display orchestrator backend in wave status
- Per-wave backend override in UI

**Success criteria:** Web UI respects orchestrator backend config, shows backend in use.

#### Wave 2-8: Integration Testing Matrix

**Test all combinations:**

| Orchestrator Backend | Agent Backend | Test Status |
|---------------------|---------------|-------------|
| API (Claude) | Bedrock | ✓ |
| API (Claude) | API (Claude) | ✓ |
| API (Claude) | OpenAI | ✓ |
| Bedrock | Bedrock | ✓ |
| Bedrock | API | ✓ |
| OpenAI | Bedrock | ✓ |
| OpenAI | OpenAI | ✓ |
| CLI (Max Plan) | Any | ✓ |

**Success criteria:** All 8+ combinations work, no backend-specific bugs.

**Phase 2 Checkpoint:** Full interoperability achieved. Orchestrator and agents both pluggable. Backend matrix validated.

---

### Incremental Testing Strategy

**After each wave:**
1. **Unit tests** - SDK functions, backend implementations
2. **Integration tests** - CLI commands, HTTP endpoints
3. **Manual testing** - Run `/saw wave` on real IMPL doc
4. **Validation** - Compare behavior to previous version

**Abort criteria:**
- Phase 1 Wave X fails validation → Fix before proceeding
- Phase 1 complete but unstable → Stabilize before Phase 2
- Phase 2 Wave X breaks Phase 1 → Roll back, redesign

**Ship criteria:**
- Phase 1 can ship independently (agents pluggable, SDK working)
- Phase 2 only ships after Phase 1 is production-stable

---

### Backward Compatibility During Migration

**Phase 1 (Protocol SDK):**
- Markdown IMPL docs continue to work (current parser remains)
- YAML manifests work via SDK
- Both formats supported during transition
- Migration utility: `saw migrate` converts markdown → YAML on-demand

**Phase 2 (Orchestrator Backend):**
- Default orchestrator backend: CLI (current behavior)
- Explicitly configured backends opt-in to new behavior
- No breaking changes for existing deployments

**Deprecation timeline:**
- v2.0: Phase 1 ships, both formats supported
- v2.1: Phase 2 ships, orchestrator pluggable
- v3.0: Markdown IMPL docs deprecated (YAML only)

---

---

## Benefits

### 1. Deterministic Enforcement

**Before:**
```bash
# Bash script parsing markdown
grep "^| " file-ownership.md | sed 's/|//g' | awk '{print $2}' | sort | uniq -d
# If duplicate found: echo "I1 violation"
```

**After:**
```go
// SDK validation (compile-time guarantees)
func validateFileOwnership(m *Manifest) error {
    seen := make(map[string]string)
    for _, fo := range m.FileOwnership {
        if owner, exists := seen[fo.File]; exists {
            return fmt.Errorf("I1 violation: %s owned by %s and %s", fo.File, owner, fo.Agent)
        }
        seen[fo.File] = fo.Agent
    }
    return nil
}
```

**Result:** No parse errors, no retry loops, violations caught before execution.

### 2. Code Reuse

**Before:**
- Bash script validates markdown (400 lines)
- Go parser validates markdown (800 lines)
- Two implementations, can diverge

**After:**
- SDK validates manifest (one implementation)
- CLI binary uses SDK
- Web UI uses SDK
- External tools use SDK

**Result:** Protocol change updates one place, all consumers get the fix.

### 3. Programmatic Access

**Before:**
```bash
# External tool must parse markdown
grep "^## Wave" IMPL.md | grep -v "✓" | head -1 | sed 's/## Wave //'
```

**After:**
```go
// External tool imports SDK
import "github.com/blackwell-systems/scout-and-wave-go/pkg/protocol"

manifest, _ := protocol.Load("IMPL-foo.yaml")
currentWave := manifest.CurrentWave()
```

**Result:** IDE plugins, CI/CD integrations, monitoring tools can query protocol state programmatically.

### 4. Scalability

**Before:**
- Multiple agents appending to same markdown file causes merge conflicts
- Workaround: per-agent report files for large waves

**After:**
```bash
# Each agent calls SDK operation
saw set-completion IMPL-foo.yaml agent-A < report.yaml
# SDK handles concurrent updates safely
```

**Result:** Agent count scales without coordination overhead.

### 5. Developer Experience

**Before:**
- Guess at IMPL doc format from examples
- Parse errors discovered at runtime

**After:**
- IDE autocomplete for manifest structure
- Schema validation on save
- Type-safe operations

**Result:** Faster development, fewer errors.

---

## Scope

### In Scope

**scout-and-wave-go (SDK):**
- Core manifest types
- Validation operations (I1-I6)
- Agent context extraction (E23)
- Markdown rendering
- Migration utility
- Unit tests

**scout-and-wave-web (CLI + web):**
- CLI commands wrapping SDK operations
- HTTP handlers wrapping SDK operations
- Manifest editor UI
- Frontend updates to consume structured data

**scout-and-wave (protocol repo):**
- Skill updates to call SDK commands
- Scout agent updates to generate YAML
- Wave agent prompt updates for structured payloads
- Documentation updates

### Out of Scope

- Changing worktree isolation model (orthogonal)
- Changing SSE event stream format (orthogonal)
- Changing agent tool system (separate effort: IMPL-tool-refactor.md)
- Migrating all existing IMPL docs immediately (on-demand via utility)

---

## Architectural Constraints

1. **Multi-repo coordination:** SDK in scout-and-wave-go, consumed by scout-and-wave-web and scout-and-wave protocol repo.

2. **Backward compatibility:** Markdown IMPL docs continue to work during transition. Migration utility provided.

3. **Human review:** Generated markdown view must be readable/reviewable before wave execution.

4. **No new dependencies:** Use Go stdlib (`encoding/json`, `gopkg.in/yaml.v3`) + existing deps. No heavy schema validation frameworks.

5. **LLM compatibility:** Manifest format (YAML/JSON) must be editable by LLMs. No binary formats.

6. **Multi-backend support:** Must work with all backend configurations:
   - **CLI with Bedrock** - AWS Bedrock backend, subprocess orchestration
   - **CLI with Max Plan** - Claude Code CLI backend (current context)
   - **API key + SDK** - Direct Anthropic API usage, programmatic orchestration

   SDK is backend-agnostic. CLI binary and skill work the same regardless of which backend launches agents.

7. **Preserve `/saw` command:** The `/saw` skill invocation from Claude Code CLI must continue to work. This is the primary user-facing interface for CLI-based orchestration.

8. **Interactive error recovery:** Claude-as-orchestrator remains primary model because CLI provides better interactivity for unexpected error handling.

---

## Success Criteria

1. **Zero parse errors:** Schema validation catches all structural issues before Scout/agents execute.

2. **No retry loops:** Invalid manifests rejected on write (schema validation), not after Scout completes.

3. **No merge conflicts:** Completion reports registered via SDK operation, not file appends.

4. **Programmatic access:** External tools can import SDK and query protocol state.

5. **Same ergonomics:** Human review of generated markdown is as intuitive as current IMPL docs.

6. **Code reuse verified:** CLI binary, web UI, and external tool all use same SDK functions. No duplicate implementations.

7. **Skill commands work:** `/saw scout`, `/saw wave`, `/saw status` function identically to current behavior, but call SDK operations internally.

---


### Self-Validation Approach: SAW Implements SAW

**Critical advantage:** Use Scout-and-Wave to implement the protocol SDK migration itself.

**Why this works:**
1. **Immediate dog-fooding** - Protocol improvements tested during development
2. **Failure detection** - Protocol gaps surface in real execution, not theory
3. **Incremental validation** - Each wave validates previous waves' work
4. **Living documentation** - IMPL doc for this migration becomes reference implementation

**Execution:**
```bash
# Phase 1: Protocol SDK
/saw scout "Protocol SDK migration per docs/proposals/protocol-sdk-migration-v2.md Phase 1"
  → Scout designs Wave 1 through Wave 5
  → Each wave builds on previous (SDK → CLI → Skill → Scout → Web UI)

# After Wave 1-2 completes (CLI binary available):
# Skill starts using saw validate, saw extract-context
# Rest of Phase 1 uses new CLI commands
# Protocol tests itself as it's being built

# Phase 2: Orchestrator Backend
/saw scout "Orchestrator backend abstraction per docs/proposals/protocol-sdk-migration-v2.md Phase 2"
  → Scout designs Wave 2-1 through Wave 2-8
  → Uses Phase 1 SDK (already shipped and validated)
  → Tests orchestrator pluggability in real wave execution
```

**Self-validation checkpoints:**

**Wave 1-2 (CLI binary):**
- Remaining Phase 1 waves use `saw validate` for their own IMPL docs
- If SDK is wrong, validation fails immediately
- Fix SDK, re-run wave

**Wave 1-4 (Scout updates):**
- Scout generates YAML for its own Phase 2 design
- If manifest schema is wrong, Scout's output fails validation
- Fix schema, regenerate

**Wave 2-6 (Skill updates):**
- Skill uses `saw orchestrate` to launch its own remaining waves
- If orchestrator backend routing is wrong, execution fails
- Fix routing, re-run

**Benefits:**
- **No separate test suite needed** - The migration IS the test
- **Real-world complexity** - Not toy examples, actual production use
- **Continuous validation** - Every wave validates protocol assumptions
- **Early failure detection** - Don't discover SDK bugs in unrelated features months later

**Risk mitigation:**
- Phase 1 Wave 1 uses current protocol (bootstrap)
- Once CLI binary works (Wave 1-2), switch to new commands
- If new commands fail, old protocol is still available (rollback)
- Phase 2 depends on Phase 1 being production-stable (checkpoint)

**IMPL doc location:**
```
scout-and-wave-go/docs/IMPL/IMPL-protocol-sdk-migration.yaml
```

This IMPL doc becomes the canonical example of:
- Multi-phase project structure
- Cross-repo coordination (scout-and-wave-go, scout-and-wave-web, scout-and-wave)
- Incremental delivery with testing checkpoints
- Self-validation during development

---

## Structured Output Integration

**Claude supports structured output mode** - returning pure JSON that matches a schema, eliminating parsing ambiguity entirely. This pairs perfectly with SDK types.

### Current Approach (Text Parsing)

**Scout generates IMPL doc:**
```
Agent A calls API:
  → Returns: "### Agent A\n\n**Task:** Implement workshop..."
  → Claude writes markdown with YAML blocks
  → Validator parses markdown with regex
  → Extracts typed blocks: type=impl-completion-report
```

**Fragile points:**
- Markdown format variations break parser
- YAML block extraction uses regex
- No guarantee output matches schema

### Structured Output Approach

**Scout generates manifest directly:**
```go
// Agent A calls API with schema
response := client.Messages.Create(ctx, anthropic.MessagesRequest{
    Model: "claude-sonnet-4-6",
    Messages: [...],
    OutputConfig: anthropic.OutputConfig{
        Type: "json_schema",
        Schema: IMPLManifestSchema,  // SDK type as JSON Schema
    },
})

// Response is guaranteed valid JSON matching schema
var manifest protocol.IMPLManifest
json.Unmarshal(response.Content, &manifest)
// No parsing errors possible - schema-enforced by Claude
```

**Benefits:**
- ✓ No parse errors (Claude returns valid JSON or fails)
- ✓ No validation retry loops (schema-enforced at generation time)
- ✓ Direct SDK type unmarshaling (JSON → Go struct)
- ✓ Works with any backend supporting structured output

### Where to Use Structured Output

**Ideal candidates:**

**1. Scout generating IMPL manifest**
```go
// Input: Feature description
// Output schema: protocol.IMPLManifest
// Result: Valid YAML/JSON manifest, no parsing needed
```

**2. Agents generating completion reports**
```go
// Input: Task context
// Output schema: protocol.CompletionReport
// Result: Valid completion report, direct SDK registration
```

**3. Orchestrator making decisions**
```go
// Input: Error context, options
// Output schema:
type Decision struct {
    Choice      string   `json:"choice"`       // "proceed" | "retry" | "escalate"
    Reasoning   string   `json:"reasoning"`
    Actions     []string `json:"actions"`
}
// Result: Structured decision, no text parsing
```

**Not appropriate for:**
- ✗ Code generation (agents write Go/JS/etc, not JSON)
- ✗ Exploratory analysis (pre-mortem needs freeform prose)
- ✗ Error explanations (freeform text more useful than structured)

### Implementation Approach

**Phase 1: Optional structured output**
```go
// pkg/agent/backend/api/client.go
type RunOptions struct {
    OutputSchema *jsonschema.Schema  // Optional
}

func (c *Client) Run(ctx context.Context, opts RunOptions) (string, error) {
    if opts.OutputSchema != nil {
        // Use structured output mode
        req.OutputConfig = &anthropic.OutputConfig{
            Type: "json_schema",
            Schema: opts.OutputSchema,
        }
    }
    // Otherwise text mode (current behavior)
}
```

**Phase 2: SDK types as schemas**
```go
// pkg/protocol/schema.go
var IMPLManifestSchema = generateSchema(protocol.IMPLManifest{})
var CompletionReportSchema = generateSchema(protocol.CompletionReport{})

// Use in Scout agent
schema := protocol.IMPLManifestSchema
response := backend.Run(ctx, RunOptions{
    OutputSchema: &schema,
})
```

**Phase 3: Validation becomes assertion**
```go
// Before: Validate manifest after Scout writes it
manifest, err := protocol.Load("IMPL-foo.yaml")
if err := protocol.Validate(manifest); err != nil {
    // Scout wrote invalid manifest, retry
}

// After: Manifest is guaranteed valid (schema-enforced)
var manifest protocol.IMPLManifest
json.Unmarshal(response, &manifest)
// Validation is assertion, not check
// If this fails, it's a bug in schema definition, not Scout's output
```

### Benefits for Protocol SDK Migration

**Structured output + SDK types = zero parsing:**

```
Scout prompt → Claude API (structured output mode)
  ↓
Returns: JSON matching IMPLManifest schema
  ↓
json.Unmarshal → Go struct (protocol.IMPLManifest)
  ↓
No validation needed (schema-enforced)
  ↓
Save as YAML for human readability
```

**Eliminates entire error class:**
- ✗ Parse errors (Claude returns valid JSON)
- ✗ Validation errors (schema-enforced at generation)
- ✗ Retry loops (no malformed output possible)
- ✗ Format variations (schema is rigid)

**Pairs perfectly with SDK:**
- SDK defines types → Generate JSON Schema → Claude uses schema
- One source of truth: SDK types
- Frontend, backend, Claude all use same schema
- Type changes propagate automatically

### Migration Path

**Phase 1 (current proposal):**
- Scout writes YAML manifest (text mode)
- Validator parses + validates
- Works with current approach

**Phase 1.5 (add structured output):**
- Scout writes JSON via structured output mode
- SDK unmarshals directly
- Convert to YAML for human readability

**Phase 2 (orchestrator backend):**
- Orchestrator decisions use structured output
- Coordination operations return typed responses
- No parsing orchestrator output

### Backend Support

**Structured output availability:**

| Backend | Structured Output Support | Notes |
|---------|--------------------------|-------|
| Anthropic API | ✓ | `output_config.type = "json_schema"` |
| AWS Bedrock | ✓ | Same as Anthropic API |
| OpenAI | ✓ | `response_format.type = "json_schema"` |
| Claude Code CLI | ? | May not support (check docs) |

**Fallback strategy:**
```go
func (c *Client) Run(ctx context.Context, opts RunOptions) (string, error) {
    if opts.OutputSchema != nil && c.supportsStructuredOutput() {
        // Use structured output
        return c.runStructured(ctx, opts)
    }
    // Fall back to text mode + parsing
    return c.runText(ctx, opts)
}
```

### Open Questions

1. **CLI backend support:** Does Claude Code CLI support structured output mode?
2. **Schema evolution:** How to handle breaking changes in SDK types?
3. **Human readability:** Should we store JSON (machine) or YAML (human)?
4. **Validation role:** If schema-enforced, what does `protocol.Validate()` check?

**Recommendations:**
1. CLI fallback: Use text mode + parsing if structured output unsupported
2. Schema versioning: Include version field in manifest (`schema_version: "2.0"`)
3. Store YAML: Convert JSON → YAML for human readability (`saw render`)
4. Validation checks: Semantic rules (I1-I6) not covered by schema

---
## Open Questions for Scout

1. **Manifest format:** YAML vs JSON vs TOML?
   **Recommendation:** YAML (human-editable, supports comments, widely used)

2. **Schema validation approach:** Handwritten validators vs code-generated from JSON Schema?
   **Recommendation:** Handwritten (simpler, no new dependencies, custom error messages)

3. **Migration strategy:** Big-bang (convert all IMPL docs) vs incremental (new features use manifest)?
   **Recommendation:** Incremental (lower risk, utility for on-demand migration)

4. **Interface contract representation:** JSON Schema? Go interface definitions? Protocol buffers?
   **Current:** Go interface definitions in scaffold files (already works)

5. **Quality gate integration:** Keep as YAML in manifest or extract to `.saw/gates/` directory?
   **Recommendation:** Keep in manifest (single source of truth)

6. **Completion report writes:** Separate files or SDK method updates manifest in-place?
   **Recommendation:** Separate files initially (`.claude/worktrees/wave1-agent-A/completion-report.yaml`), registered via `saw set-completion`

7. **Orchestrator API:** HTTP endpoints for SDK operations or only CLI + web UI?
   **Recommendation:** Both (web UI uses HTTP handlers, external tools can use same endpoints)

8. **Error recovery strategy:** Codify known error patterns in Go vs always defer to Claude?
   **Recommendation:** Hybrid (implement known patterns in `saw merge-wave`, unknown errors exit with context for Claude)

9. **Agent runtime abstraction:** Should the SDK define a `Runtime` interface for agent execution?
   **Recommendation:** Yes. Define `Runtime` interface in Phase 1 (with single Claude Code implementation). This keeps the door open for Claude Agent SDK, Google ADK, or direct API backends in future phases without requiring SDK-level changes. See *Appendix: Framework Evaluation*.

---

## Related Work

- **IMPL-tool-refactor.md** - Tool system refactoring (completed Wave 1, orthogonal to this proposal)
- **ROADMAP.md** - Current roadmap items (this would be v2.0 milestone)
- **docs/protocol/invariants.md** - I1-I6 invariants that SDK must enforce
- **docs/protocol/execution-rules.md** - E1-E23 rules that become SDK operations
- **Claude Agent SDK** - Anthropic's agent runtime (Python/TS): https://platform.claude.com/docs/en/agent-sdk/overview
- **Google ADK** - Google's Agent Development Kit (Go/Python/TS/Java): https://google.github.io/adk-docs/
- **GoAgents** - Ingenimax Go agent framework: https://docs.goagents.dev/
- **Anthropic Agent Skills** - Skills standard SAW already implements: https://platform.claude.com/docs/en/agents-and-tools/agent-skills/overview

---

## References

- Current parser: `pkg/protocol/parser.go` (~800 lines of line-by-line state machine)
- Current validator: `scripts/validate-impl.sh` (~400 lines of bash)
- Typed blocks: `pkg/protocol/parser.go:ParseCompletionReport()`, `ParseFileOwnership()`
- Agent context extraction: E23 in `docs/protocol/execution-rules.md`

---

## Appendix: Why Not Full Go Orchestrator?

**We considered:** Building a pure Go orchestrator (`saw wave`) that handles everything programmatically.

**We decided against it because:**

1. **Error recovery requires semantic understanding:**
   - Example: Duplicate `NewWorkshop()` declaration (Agent B's temp stub vs Agent A's production)
   - Programmatic response: Exit with error code
   - Claude response: Read both files, understand intent, apply correct fix
   - **Conclusion:** Unexpected errors need conversation, not exit codes

2. **Edge cases are infinite:**
   - Can't enumerate all possible build failures, merge conflicts, isolation issues
   - Pre-coding handlers for every error type is impossible
   - Human judgment is required for novel failures

3. **Interactivity is critical:**
   - SAW often encounters "should I proceed?" decision points
   - Pure Go would print prompts, wait for stdin (`y/n`)
   - Claude can explain context, suggest options, adapt to response
   - **Conclusion:** CLI with Claude is more ergonomic than standalone binary prompts

4. **Development flexibility:**
   - Skill instructions can be updated without recompiling
   - New coordination patterns can be prototyped in bash
   - Go orchestrator would require compile-deploy cycle for changes

**Hybrid model (Claude-supervised with Go SDK) combines benefits:**
- Happy path automation (SDK operations are deterministic)
- Error recovery flexibility (Claude intervenes when needed)
- Progressive automation (proven patterns migrate from skill to Go over time)

**Standalone `saw wave` is possible** but secondary use case (CI/CD where errors cause hard failure anyway).

---

## Appendix: Framework Evaluation

**Date:** March 9, 2026
**Context:** Before locking in the Protocol SDK architecture, we evaluated whether SAW should be built on an existing agentic framework instead of a purpose-built Go SDK.

### Frameworks Evaluated

| Framework | Language | Maturity | Model Support | Parallel Agents | Key Trait |
|-----------|----------|----------|---------------|-----------------|-----------|
| **Claude Agent SDK** | Python, TypeScript | Production (powers Claude Code) | Claude (+ Bedrock, Vertex, Azure) | Subagents via `Agent` tool | Built-in tools (Read, Edit, Bash), Skills, hooks |
| **Google ADK** | Go, Python, TS, Java | v0.6.0 (early) | Gemini-native, model-agnostic | `ParallelAgent` (errgroup) | Go-native, `BeforeAgent`/`AfterAgent` callbacks |
| **GoAgents (Ingenimax)** | Go | Early | OpenAI, Claude, Gemini, DeepSeek, Ollama, vLLM | Not confirmed | YAML config, MCP support, multi-provider |
| **LangGraph** | Python | Mature | Multi-model | Graph-based | Checkpointing, state machines |
| **CrewAI** | Python | Mature | Multi-model | Role-based delegation | Opinionated roles |
| **AutoGen** | Python, .NET | Mature | Multi-model | Multi-agent conversation | Conversation-centric |

### SAW's Execution Model vs Framework Assumptions

Most agentic frameworks assume:
- **Agents are LLM conversation loops** — generate → tool call → resume, managed in-process
- **Coordination is conversational** — agents communicate via shared session state or message passing
- **Orchestration is autonomous** — framework decides next steps based on LLM output

SAW's model is fundamentally different:
- **Agents are isolated processes** — each runs in a separate git worktree with disjoint file ownership (I1)
- **Coordination is document-based** — IMPL manifest is the single source of truth (I4), not session state
- **Orchestration is human-supervised** — Claude-as-orchestrator with review checkpoints, not autonomous

This mismatch means adopting a framework would require building SAW's unique primitives (worktree isolation, file ownership enforcement, manifest lifecycle, merge sequencing) on top of the framework — using it as a glorified LLM conversation loop while ignoring most of its orchestration features.

### Claude Agent SDK — Closest Fit

The Claude Agent SDK is the most interesting option because **SAW already uses its execution model.** SAW agents are Claude Code subagents with per-agent tool restrictions, launched via the `Agent` tool — which is exactly what the Agent SDK provides programmatically:

```python
# What SAW does today (via Claude Code Agent tool):
# Launch wave agent with restricted tools, isolated context

# What it would look like with Claude Agent SDK:
async for message in query(
    prompt=agent_context_payload,
    options=ClaudeAgentOptions(
        allowed_tools=["Read", "Edit", "Bash", "Glob", "Grep"],
        # No "Agent" tool — wave agents can't spawn sub-agents
    ),
):
    handle(message)
```

**What the Agent SDK provides that SAW needs:**
- Per-agent tool restrictions (scout can't Edit, wave agents can't spawn)
- Hooks (PreToolUse, PostToolUse — like claudewatch)
- Sessions (resume, fork — useful for agent retry)
- Skills (progressive disclosure — what SAW's skill files already are)
- Subagent definitions with isolated tool sets

**What blocks adoption today:**
- **No Go SDK.** Python and TypeScript only. SAW's engine is Go. Adding a Python/TS process into the `saw` CLI creates a language boundary and deployment complexity.
- **Claude-only.** SAW already supports Qwen and is built against the agent-skills standard for cross-runtime compatibility. The Agent SDK is Claude-specific.

**Verdict:** Monitor for Go SDK release. If one appears, the Agent SDK becomes the natural runtime backend for SAW's `Runtime` interface.

### Google ADK — Go-Native but Wrong Abstraction

ADK's `ParallelAgent` runs sub-agents concurrently using Go's `errgroup`, which superficially maps to SAW waves. However:

**Isolation model mismatch:** ADK's "isolation" is *conversation branching* — each sub-agent gets a `Branch` string so they don't see each other's conversation history. All agents share the same `Session`, `Artifacts`, and `Memory` objects. SAW's isolation is *filesystem-level* — separate git worktrees with disjoint file ownership.

**genai type tax:** Every agent interaction flows through `google.golang.org/genai` types (`genai.Content`, `genai.Part`). Using Claude or Qwen requires adapters that translate to/from these types on every API call.

**Useful primitives:** `BeforeAgentCallback` / `AfterAgentCallback` hooks are where worktree creation and merge could be wired in. The agent composition model (`Config.SubAgents`, per-agent tool sets) maps to SAW's agent definitions.

**Verdict:** ADK could serve as the LLM conversation loop if SAW agents become direct API callers (not Claude Code subagents). But it provides no help with SAW's core coordination primitives — those would all be custom code on top of ADK.

### GoAgents (Ingenimax) — Multi-Provider but Immature

GoAgents is notable for its broad model support (OpenAI, Claude, Gemini, DeepSeek, Ollama, vLLM) and YAML-based agent configuration. It also supports MCP servers natively.

**Limitations:** Early-stage project, no confirmed parallel execution support, no workflow orchestration primitives comparable to ADK's ParallelAgent. Documentation is thin on multi-agent coordination patterns.

**Verdict:** Worth watching for its multi-provider abstraction layer, but too immature for production use.

### Decision: Purpose-Built SDK with Runtime Interface

**Phase 1:** Build the Protocol SDK as designed — YAML manifests, Go validation, CLI commands. Agent execution continues via Claude Code's `Agent` tool (current model). No framework dependency.

**Design for the future:** Define a `Runtime` interface in the SDK:

```go
// pkg/protocol/runtime.go
type Runtime interface {
    // LaunchAgent starts an agent with the given context and tool restrictions.
    LaunchAgent(ctx context.Context, cfg AgentConfig) (AgentHandle, error)

    // WaitForCompletion blocks until the agent finishes and returns its report.
    WaitForCompletion(handle AgentHandle) (*CompletionReport, error)
}

type AgentConfig struct {
    ID          string
    Prompt      string
    Tools       []string   // allowed tools
    WorkDir     string     // worktree path
    Model       string     // model identifier
    Backend     string     // runtime backend
}
```

**Phase 1 implementation:** `ClaudeCodeRuntime` — wraps the current "launch via Agent tool" model.

**Future implementations:**
- `AgentSDKRuntime` — calls Claude Agent SDK (Python/TS) for direct API control
- `ADKRuntime` — uses Google ADK for Go-native LLM loops
- `DirectAPIRuntime` — raw Anthropic/OpenAI API with custom tool dispatch

This approach gets us shipping now (no framework dependency, no language boundary) while keeping every framework option open for later phases. The `Runtime` interface is the insurance policy — swap the backend without changing protocol logic.

### Key Insight

SAW's value is not in the agent execution loop (any framework can do that). SAW's value is in the **coordination protocol** — wave sequencing, disjoint file ownership, interface contracts, manifest lifecycle, merge verification. No framework provides these. The Protocol SDK is the right investment because it codifies what's unique to SAW; agent execution is the commodity layer that can be delegated to whatever runtime fits best.
