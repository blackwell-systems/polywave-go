# Scout-and-Wave Engine Roadmap

> Last audited: 2026-04-04. Completed items removed; see CHANGELOG.md for shipped features.

This is the engine/SDK roadmap. See also:
- **scout-and-wave** (protocol repo) `ROADMAP.md` for protocol enhancements and PROGRAM hardening (P2-P7)
- **scout-and-wave-web** roadmap for UI/UX features (agent progress API, lease visualization, SSE events)

---

## Shipped Recently (2026-04-04)

See protocol repo `ROADMAP.md` for full details. SDK-side changes:

- **P7 fix:** `FinalizeWave` solo-wave detection checks `AllBranchesAbsent` (data loss prevention)
- **P2:** `FinalizeWaveOpts.CrossRepoVerify` + `--cross-repo-verify` CLI flag
- **P3:** `RunScoutFullOpts.RefreshBrief` + `--refresh-brief` CLI flag
- **finalize-scout:** Consolidates Scout validation steps (validate + pre-wave-validate + validate-briefs + set-injection-method)
- **Journal integration:** `pkg/orchestrator` calls `Sync()` + `GenerateContext()`, 30s periodic sync goroutine

---

## Intelligence & Learning

### Advanced Memory System (Reflection Agent)

**Status:** Unstarted. `pkg/memory/` does not exist.

**Why:** Basic project memory (E17/E18 context read/write) is shipped. Agents still repeat mistakes across features because there is no automated pattern extraction from completion reports.

**Scope:**
1. After wave completes, run reflection agent to extract patterns/pitfalls from completion reports
2. Score memories by tag overlap with feature description, inject top-5 into Scout prompt
3. Implementation: `pkg/memory/` (reflector, scorer, store) + `pkg/engine/runner.go` injection

---

## Production Hardening

### Observability Stack

Partially shipped: `pkg/observability/` has emitter, events, query, rollups. Remaining:

- OpenTelemetry tracing (spans per wave/agent/tool call)
- Cost tracking per IMPL doc (token usage, model costs)
- Prometheus metrics endpoint

### Sandboxed Execution

- Agents run in isolated containers (Docker/Podman)
- Filesystem restrictions (read-only except worktree)
- Network policies (block outbound except API)
- Resource limits (CPU/memory per agent)
