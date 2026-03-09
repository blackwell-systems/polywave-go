# Proposal: Protocol SDK Migration

## Context

The Scout-and-Wave protocol currently uses **natural language markdown documents** (IMPL docs) as the source of truth for feature planning and agent coordination. While this approach provides flexibility and human readability, execution over multiple waves has revealed friction points where **deterministic structural enforcement** would eliminate entire classes of failures.

### Current Architecture

**Source of truth:** `docs/IMPL/IMPL-<slug>.md` (markdown)
- Wave structure, agent assignments, file ownership defined in prose/tables
- Interface contracts described in markdown
- Quality gates specified as YAML blocks in the document
- Completion reports written as typed YAML blocks (`type=impl-completion-report`)
- Validation via bash scripts that parse markdown (`validate-impl.sh`)

**Coordination mechanisms:**
- File ownership table (markdown table, parsed by grep/sed)
- Typed blocks for structured data (completion reports, file ownership)
- Bash scripts for validation (E16), stub detection (E20), quality gates (E21)
- Git worktrees for agent isolation

**Agent execution:**
- Agents receive prose prompts (Fields 0-8) referencing the IMPL doc
- Agents read markdown, parse tables, interpret task descriptions
- Agents write completion reports back to markdown (merge conflicts expected with >4 agents)

### Observed Friction Points

**1. Parse errors and validation failures**
- Scout writes markdown → validator parses with regex → rejects on malformed structure
- Requires retry loops (up to 3 attempts) for validation failures
- No compile-time safety; errors discovered at runtime

**2. Agent isolation workarounds**
- Agents can't import each other's code during parallel execution
- Results in duplicate implementations (e.g., Agent B creating temporary `NewWorkshop()` stub)
- Merge conflicts resolved post-hoc rather than prevented

**3. Merge conflicts in IMPL doc itself**
- Multiple agents appending to same markdown file causes expected conflicts
- Workaround: per-agent report files for waves with ≥5 agents
- Coordination overhead scales with agent count

**4. Reactive rather than preventive enforcement**
- Stub detection happens after agents finish (E20 scan)
- Ownership violations caught at merge time, not registration time
- Interface contract deviations reported in completion reports, not blocked upfront

**5. Lack of programmatic access**
- Orchestrator parses markdown with `pkg/protocol/parser.go` (line-by-line state machine)
- Web UI must parse markdown to render agent status
- No SDK for external tools to consume protocol state

### Previous Partial Solutions

We've already introduced structure in limited areas:
- **Typed blocks** (`type=impl-completion-report`, `type=impl-file-ownership`) with schema markers
- **Validation scripts** (`validate-impl.sh`) enforce structure that could be JSON Schema
- **Scaffold files** are actual Go code, not markdown descriptions
- **Quality gates** run shell commands, not interpreted prose

**Observation:** The protocol is already hybrid (prose with structured checkpoints), not pure natural language.

## Proposal

Migrate to **SDK-first protocol** with deterministic structural enforcement at multiple layers:

### 1. Manifest as Source of Truth

**Before:**
```markdown
# IMPL: Tool System Refactoring

| File | Agent | Wave | Depends On |
|------|-------|------|------------|
| pkg/tools/workshop.go | A | 1 | Scaffold |
```

**After:**
```yaml
# docs/IMPL/IMPL-tool-refactor.yaml
title: "Tool System Refactoring"
feature_slug: "tool-refactor"
verdict: "SUITABLE"

file_ownership:
  - file: pkg/tools/workshop.go
    agent: A
    wave: 1
    depends_on: [Scaffold]

waves:
  - number: 1
    agents:
      - id: A
        task: "Implement Workshop interface and middleware stack"
        files:
          - pkg/tools/workshop.go
          - pkg/tools/middleware.go
        dependencies: [Scaffold]
```

**Human-readable view:** Generated markdown (`IMPL-tool-refactor.md`) for review, derived from YAML.

### 2. SDK Core Types (`pkg/protocol`)

```go
type IMPLManifest struct {
    Title           string
    FeatureSlug     string
    Verdict         string // "SUITABLE" | "NOT_SUITABLE"
    FileOwnership   []FileOwnership
    InterfaceContracts []Contract
    Waves           []Wave
    QualityGates    QualityGateConfig
    Scaffolds       []ScaffoldFile
}

// Operations
func Load(path string) (*IMPLManifest, error)
func Validate(m *IMPLManifest) error
func GenerateMarkdown(m *IMPLManifest) (string, error)
func ExtractAgentContext(m *IMPLManifest, agentID string) (*AgentContext, error)
func (m *IMPLManifest) SetCompletionReport(agentID string, report CompletionReport) error
```

### 3. Multi-Layer Integration

**Layer 1: SDK (scout-and-wave-go)**
- `pkg/protocol/manifest.go` - core types with YAML/JSON tags
- `pkg/protocol/validation.go` - I1-I6 invariant enforcement
- `pkg/protocol/extract.go` - agent context extraction (E23)
- `pkg/protocol/render.go` - markdown generation for human review

**Layer 2: Orchestrator (CLI + web)**
- CLI orchestrator (`/saw` skill) consumes SDK, passes structured payloads to agents
- Web UI HTTP handlers wrap SDK operations (load manifest, validate, update completion reports)
- Direct SDK usage for external tools (import `github.com/blackwell-systems/scout-and-wave-go/pkg/protocol`)

**Layer 3: Agent execution**
- Agents receive structured JSON/YAML payload (not markdown with tables)
- Agents return structured completion reports (JSON/YAML, not freeform text)
- No parsing ambiguity, no retry loops

### 4. Benefits

**Deterministic enforcement:**
- Schema validation replaces regex parsing (no parse errors)
- Ownership violations caught at registration time (fail-fast, not at merge)
- Invalid manifests rejected on write (no retry loops)
- Interface contracts become importable types (no description/implementation drift)

**Programmatic access:**
- SDK provides canonical API for all protocol operations
- Web UI, CLI, external tools consume same types
- No duplicate parsing logic across repos

**Scalability:**
- Completion reports written via SDK method, not file appends (no merge conflicts)
- Agent count scales without coordination overhead
- Validation complexity is O(1), not O(lines-of-markdown)

**Developer experience:**
- IDE autocomplete for manifest structure
- Type safety for protocol operations
- Schema-driven UI forms in web interface

### 5. What Stays Prose

- Task descriptions (agent Field 6) - humans write, agents interpret
- Pre-mortem analysis - exploratory thinking
- Architectural notes in CONTEXT.md
- Deviation explanations in completion reports (freeform notes field)

Natural language remains for creative/interpretive work; structure enforces coordination.

## Scope

### In Scope

**scout-and-wave-go (SDK implementation):**
- `pkg/protocol/manifest.go` - core manifest types (IMPLManifest, Wave, Agent, FileOwnership, CompletionReport, etc.)
- `pkg/protocol/validation.go` - invariant enforcement (I1-I6), schema validation
- `pkg/protocol/extract.go` - agent context extraction (E23)
- `pkg/protocol/render.go` - markdown generation for human review
- `pkg/protocol/migrate.go` - utility to convert existing markdown IMPL docs to YAML manifests
- Update `pkg/orchestrator/*.go` to consume manifest types instead of parsing markdown
- Update `pkg/engine/runner.go` to use SDK operations

**scout-and-wave-web (orchestrator integration):**
- Update HTTP handlers to consume SDK manifest types (`/api/impl/:slug`, `/api/wave/:slug/start`, etc.)
- Add manifest editor UI (YAML editing with schema validation, or form-based editor)
- Update frontend components to render from structured data
- Update `saw` CLI commands to use SDK operations

**scout-and-wave (protocol repo):**
- Update `/saw` skill to use SDK operations (no bash parsing)
- Update Scout agent to generate YAML manifest instead of markdown
- Update Wave agent prompts to expect structured payloads
- Update protocol docs to explain manifest schema
- Archive old bash scripts (`validate-impl.sh`, `scan-stubs.sh`) or rewrite as SDK wrappers

### Out of Scope

- Changing the worktree isolation model (remains as-is)
- Changing the SSE event stream format (orthogonal concern)
- Changing the agent tool system (parallel effort, see IMPL-tool-refactor.md)
- Migrating all existing IMPL docs immediately (migration utility provided, done on-demand)

## Architectural Constraints

1. **Multi-repo coordination:** SDK lives in `scout-and-wave-go`, consumed by `scout-and-wave-web` and `scout-and-wave` protocol repo.
2. **Backward compatibility:** Must provide migration path for existing markdown IMPL docs.
3. **Human review:** Generated markdown view must be readable/reviewable before wave execution.
4. **No new dependencies:** Use Go stdlib (`encoding/json`, `gopkg.in/yaml.v3`) + existing deps. No heavy schema validation frameworks.
5. **Claude Code compatibility:** Manifest format must be editable by LLMs (YAML/JSON, not binary).

## Success Criteria

1. **Zero parse errors:** Schema validation catches all structural issues before Scout/agents execute.
2. **No retry loops:** Invalid manifests rejected on write, not after Scout completes.
3. **No merge conflicts:** Completion reports written via SDK method, not file appends.
4. **Programmatic access:** External tools can import SDK and query protocol state.
5. **Same ergonomics:** Human review of generated markdown is as intuitive as current IMPL docs.

## Open Questions for Scout

1. **Manifest format:** YAML vs JSON vs TOML? (Recommendation: YAML for human editability + comments)
2. **Schema validation approach:** Handwritten validators vs code-generated from JSON Schema?
3. **Migration strategy:** Big-bang (convert all IMPL docs) vs incremental (new features use manifest, old stay markdown)?
4. **Interface contract representation:** JSON Schema? Go interface definitions? Protocol buffers?
5. **Quality gate integration:** Keep as YAML in manifest or extract to `.saw/gates/` directory with executable scripts?
6. **Completion report writes:** Agents write to separate files (`.saw/reports/agent-A.yaml`) or SDK method call that updates manifest in-place?
7. **Orchestrator API:** Should orchestrator expose HTTP endpoints for SDK operations, or only CLI + web UI?

## Related Work

- **IMPL-tool-refactor.md** - Tool system refactoring (completed Wave 1, this proposal is orthogonal)
- **ROADMAP.md** - Current roadmap items (this would be a major v2.0 milestone)
- **docs/protocol/invariants.md** - I1-I6 invariants that must be SDK-enforced
- **docs/protocol/execution-rules.md** - E1-E23 rules that become SDK methods

## References

- Current parser: `pkg/protocol/parser.go` (~800 lines of line-by-line state machine)
- Current validator: `scripts/validate-impl.sh` (~400 lines of bash)
- Typed blocks: `pkg/protocol/parser.go:ParseCompletionReport()`, `ParseFileOwnership()`
- Agent context extraction: E23 in `docs/protocol/execution-rules.md`
