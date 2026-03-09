# Proposal: Protocol SDK Migration

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

#### Orchestrator Layer (Where `/saw` Runs)

**The `/saw` skill always runs in Claude Code CLI** - it is instructions for Claude (the orchestrator) to execute.

**Two deployment contexts for orchestrator:**

| Context | How Invoked | Backend Powering Orchestrator |
|---------|-------------|-------------------------------|
| **Max Plan** | `claude` CLI with Max Plan subscription | Claude API (subscription-based) |
| **Bedrock** | `claude` CLI with `ANTHROPIC_BEDROCK_ENABLED=true` | AWS Bedrock (pay-per-use) |

**Key point:** The orchestrator uses YOUR Claude session. This is the Claude instance interpreting the skill, coordinating wave execution, handling errors conversationally.

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

### Phase 1: SDK Implementation (Wave 1)

**scout-and-wave-go (SDK core):**
- `pkg/protocol/manifest.go` - Core types (IMPLManifest, Wave, Agent, FileOwnership, etc.)
- `pkg/protocol/validation.go` - I1-I6 invariant enforcement
- `pkg/protocol/extract.go` - Agent context extraction (E23)
- `pkg/protocol/render.go` - Markdown generation
- `pkg/protocol/migrate.go` - Markdown → YAML migration utility
- Unit tests for all operations

### Phase 2: CLI Integration (Wave 2)

**scout-and-wave-web (CLI binary):**
- `cmd/saw/validate.go` - Validate command
- `cmd/saw/extract.go` - Extract context command
- `cmd/saw/completion.go` - Set completion command
- `cmd/saw/wave.go` - Current wave command
- `cmd/saw/merge.go` - Merge wave command
- `cmd/saw/render.go` - Render markdown command
- `cmd/saw/migrate.go` - Migrate markdown command
- Integration tests

### Phase 3: Skill Update (Wave 3)

**scout-and-wave (protocol repo):**
- Update `~/.claude/skills/saw/saw.md` to call SDK commands instead of bash scripts
- Update Scout agent to generate YAML manifest instead of markdown
- Update Wave agent prompts to expect structured JSON payload
- Archive bash scripts (`validate-impl.sh`, `scan-stubs.sh`)

### Phase 4: Web UI Integration (Wave 4)

**scout-and-wave-web (web UI):**
- Update HTTP handlers to use SDK operations
- Add manifest editor UI (YAML editor or form-based)
- Update frontend components to render from structured data
- Manifest validation in UI (real-time feedback)

### Phase 5: Migration Utility (Wave 5)

- Run migration utility on existing IMPL docs: `saw migrate IMPL-old.md > IMPL-new.yaml`
- Generate markdown views: `saw render IMPL-new.yaml > IMPL-new.md`
- Validate migration: `saw validate IMPL-new.yaml`
- Update references in documentation

---

## Backward Compatibility

**Incremental migration approach:**
- New features use YAML manifest
- Existing markdown IMPL docs continue to work (current parser remains)
- Migration utility converts markdown → YAML on-demand
- Eventually deprecate markdown support (v2.0 breaking change)

**Transition period:**
- Both formats supported
- Scout generates YAML for new features
- Existing features can be migrated individually using `saw migrate`

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

---

## Related Work

- **IMPL-tool-refactor.md** - Tool system refactoring (completed Wave 1, orthogonal to this proposal)
- **ROADMAP.md** - Current roadmap items (this would be v2.0 milestone)
- **docs/protocol/invariants.md** - I1-I6 invariants that SDK must enforce
- **docs/protocol/execution-rules.md** - E1-E23 rules that become SDK operations

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
