# Protocol Parsing

The `pkg/protocol` package parses IMPL documents into structured types for the orchestrator.

## IMPL Document Format

IMPL docs are markdown with structured sections:

```markdown
# IMPL: Add User Authentication

**verdict:** SUITABLE

## File Ownership

| Wave | Agent | File | Lines | Dependencies |
|------|-------|------|-------|--------------|
| 1 | A | src/auth.go | 50 | crypto/bcrypt |
| 1 | B | src/middleware.go | 30 | src/auth.go |

## Interface Contracts

```go
type Authenticator interface {
    Authenticate(username, password string) (User, error)
}
```

## Wave 1

### Agent A — Authentication Module

**task:** Implement user authentication logic

**dependencies:** None

... (9-field agent prompt)

### Agent A Completion Report

```yaml
status: complete
files:
  - src/auth.go
  - src/auth_test.go
```

## Quality Gates

- name: lint
  command: golangci-lint run
  required: true
```

See [scout-and-wave protocol spec](https://github.com/blackwell-systems/scout-and-wave/tree/main/protocol) for full format definition.

## Parser Architecture

### Line-by-Line State Machine

The parser scans line-by-line and maintains section state:

```go
type parseState int

const (
    stateRoot parseState = iota
    stateWave
    stateAgent
    stateCompletionReport
    stateFileOwnership
    stateInterfaceContracts
    stateScaffolds
    stateQualityGates
)
```

**Transitions:**

- `# IMPL:` → stateRoot
- `## Wave N` → stateWave
- `### Agent X` → stateAgent
- `## File Ownership` → stateFileOwnership
- Table row → parse into `FileOwnershipInfo`
- YAML fence → parse completion report
- Go/Rust/etc fence → parse scaffold

### Header Detection

```go
func isWaveHeader(line string) bool {
    return strings.HasPrefix(line, "## Wave ") && !strings.Contains(line, "Completion Report")
}

func isAgentHeader(line string) bool {
    if !strings.HasPrefix(line, "### Agent ") {
        return false
    }
    rest := strings.TrimPrefix(line, "### Agent ")
    // Check for [A-Z][2-9]? format
    if len(rest) == 0 {
        return false
    }
    if rest[0] < 'A' || rest[0] > 'Z' {
        return false
    }
    if len(rest) > 1 {
        if rest[1] >= '2' && rest[1] <= '9' {
            return rest[2] == ':' || rest[2] == ' ' || rest[2] == '—'
        }
    }
    return rest[1] == ':' || rest[1] == ' ' || rest[1] == '—'
}
```

### Table Parsing

File ownership table rows are parsed into `FileOwnershipInfo`:

```go
type FileOwnershipInfo struct {
    Wave         int
    Agent        string
    File         string
    Lines        string
    Dependencies string
    Repo         string // Optional, for cross-repo mode
}

func parseFileOwnershipRow(line string) (*FileOwnershipInfo, error) {
    parts := strings.Split(line, "|")
    if len(parts) < 6 {
        return nil, fmt.Errorf("invalid row: expected 6+ parts, got %d", len(parts))
    }

    wave, _ := strconv.Atoi(strings.TrimSpace(parts[1]))
    agent := strings.TrimSpace(parts[2])
    file := strings.TrimSpace(parts[3])
    lines := strings.TrimSpace(parts[4])
    deps := strings.TrimSpace(parts[5])

    info := &FileOwnershipInfo{
        Wave:         wave,
        Agent:        agent,
        File:         file,
        Lines:        lines,
        Dependencies: deps,
    }

    // Cross-repo mode: 5-column table
    if len(parts) >= 7 {
        info.Repo = strings.TrimSpace(parts[6])
    }

    return info, nil
}
```

### YAML Block Parsing

Completion reports are YAML blocks:

```go
func ParseCompletionReport(yamlContent string) (*types.CompletionReport, error) {
    var raw struct {
        Status      string   `yaml:"status"`
        FailureType string   `yaml:"failure_type"`
        Repo        string   `yaml:"repo,omitempty"`
        Files       []string `yaml:"files"`
        Deviations  string   `yaml:"deviations"`
    }

    if err := yaml.Unmarshal([]byte(yamlContent), &raw); err != nil {
        return nil, fmt.Errorf("yaml unmarshal failed: %w", err)
    }

    return &types.CompletionReport{
        Status:      raw.Status,
        FailureType: raw.FailureType,
        Repo:        raw.Repo,
        Files:       raw.Files,
        Deviations:  raw.Deviations,
    }, nil
}
```

## Wave Reconstruction

If the IMPL doc has no `## Wave N` headers (old format or hand-written), the parser reconstructs wave structure from the file ownership table:

```go
func reconstructWavesFromOwnership(doc *types.IMPLDoc) {
    if len(doc.Waves) > 1 {
        return // Already has explicit waves
    }

    waveMap := make(map[int]*types.Wave)
    for _, info := range doc.FileOwnership {
        wave, ok := waveMap[info.Wave]
        if !ok {
            wave = &types.Wave{Number: info.Wave}
            waveMap[info.Wave] = wave
        }
        // Add agent if not already present
        agent := findOrCreateAgent(wave, info.Agent)
        agent.Files = append(agent.Files, info.File)
    }

    // Sort waves and assign to doc
    doc.Waves = sortedWaves(waveMap)
}
```

## Invariant Validation

The validator checks protocol invariants after parsing:

```go
func ValidateInvariants(doc *types.IMPLDoc) []string {
    var violations []string

    // I1: Disjoint file ownership within a wave
    for _, wave := range doc.Waves {
        fileAgentMap := make(map[string]string)
        for _, agent := range wave.Agents {
            for _, file := range agent.Files {
                key := fmt.Sprintf("%s:%s", agent.Repo, file)
                if existingAgent, exists := fileAgentMap[key]; exists {
                    violations = append(violations, fmt.Sprintf(
                        "I1 violation: file %s owned by both %s and %s in wave %d",
                        file, existingAgent, agent.Letter, wave.Number,
                    ))
                }
                fileAgentMap[key] = agent.Letter
            }
        }
    }

    // I2: File ownership must match completion report
    // I3: Dependencies respect wave ordering
    // I4: IMPL doc is single source of truth
    // I5: Completion reports are append-only
    // I6: No modifications to main branch during wave execution

    return violations
}
```

See [scout-and-wave invariants.md](https://github.com/blackwell-systems/scout-and-wave/blob/main/protocol/invariants.md) for full invariant definitions.

## Per-Agent Context Extraction (E23)

The extractor trims the IMPL doc to only sections relevant to a single agent:

```go
func ExtractAgentContext(doc *types.IMPLDoc, waveNumber int, agentLetter string) (*types.AgentContext, error) {
    wave := findWave(doc, waveNumber)
    agent := findAgent(wave, agentLetter)

    return &types.AgentContext{
        AgentPrompt:        agent.Prompt, // Full 9-field section
        InterfaceContracts: doc.InterfaceContracts,
        FileOwnership:      doc.FileOwnership,
        Scaffolds:          doc.Scaffolds,
        QualityGates:       doc.QualityGates,
    }, nil
}

func FormatAgentContextPayload(ctx *types.AgentContext) string {
    var buf bytes.Buffer

    // Write agent prompt
    buf.WriteString(ctx.AgentPrompt)
    buf.WriteString("\n\n")

    // Write interface contracts
    buf.WriteString("## Interface Contracts\n\n")
    buf.WriteString(ctx.InterfaceContracts)
    buf.WriteString("\n\n")

    // Write file ownership table
    buf.WriteString("## File Ownership\n\n")
    buf.WriteString(formatFileOwnershipTable(ctx.FileOwnership))
    buf.WriteString("\n\n")

    // Write scaffolds
    if len(ctx.Scaffolds) > 0 {
        buf.WriteString("## Scaffolds\n\n")
        for _, scaffold := range ctx.Scaffolds {
            buf.WriteString(scaffold)
            buf.WriteString("\n\n")
        }
    }

    // Write quality gates
    if len(ctx.QualityGates) > 0 {
        buf.WriteString("## Quality Gates\n\n")
        buf.WriteString(formatQualityGates(ctx.QualityGates))
    }

    return buf.String()
}
```

Agents receive only the trimmed payload, not the full IMPL doc.

## Future: Structured Output Bypass

The parser is brittle to format variations. Future enhancement (see ROADMAP.md):

- Scout runs via API backend use Claude's `output_config.format` to return validated JSON
- Orchestrator receives `types.IMPLDoc` directly (no markdown parsing)
- Markdown IMPL doc is **written from struct** (remains human-readable)
- Parser kept as fallback for CLI backend and hand-written docs

## See Also

- [Architecture Overview](architecture.md) — Parser role in the engine
- [Orchestration](orchestration.md) — How orchestrator uses parsed IMPL docs
- [scout-and-wave protocol spec](https://github.com/blackwell-systems/scout-and-wave/tree/main/protocol) — Canonical format definition
