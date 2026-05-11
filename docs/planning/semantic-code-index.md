# Semantic Code Index — Design Seed

**Status**: Entirely unimplemented as of 2026-03-25. No `pkg/codeindex/` package exists. No tree-sitter dependency. No `index-codebase` or `query-symbols` CLI commands. E25/wiring validation still uses heuristic pattern matching.

**Note**: `modernc.org/sqlite` is already in `go.mod` (used by `pkg/observability/sqlite/`), so the SQLite dependency is already paid — no extra module cost for storage.

**Purpose**: Tree-sitter-based symbol extraction and indexing for SAW agents. Replaces keyword grep with structured queries: "all exported functions in src/api/" → structured results with file, line, signature, callers.

**Package**: `pkg/codeindex/`

---

## Problem

Scout currently understands codebases via `Grep` and `Glob` — keyword matching. This fails when:
- Naming conventions vary by framework (Express `app.get` vs Fastify `fastify.route` vs Hono `app.get`)
- Finding callers of a function requires knowing the function name first
- E25 integration detection uses heuristics (`New*`, `Build*`, `Register*`) instead of actual unused-export analysis
- File ownership assignment requires the Scout to manually trace import graphs by reading files

## Solution

A **symbol index** built from tree-sitter parsing:

### Symbol Types
- Functions (exported/unexported)
- Types/Structs/Classes/Interfaces
- Constants/Variables (exported)
- Method receivers (Go)
- Module exports (TypeScript/Python)

### Per-Symbol Data
```go
type Symbol struct {
    Name       string   // "RegisterHandler"
    Kind       string   // "function", "type", "interface", "method"
    File       string   // "pkg/api/routes.go"
    Line       int      // 42
    Exported   bool     // true
    Signature  string   // "func RegisterHandler(mux *http.ServeMux, h Handler)"
    Package    string   // "pkg/api" (Go), "src/api" (TS), module path
    Callers    []Ref    // [{File: "cmd/server/main.go", Line: 15}]
    CallsTo    []Ref    // [{File: "pkg/handler/base.go", Line: 30}]
}
```

### Storage
SQLite with two tables:
- `symbols` — indexed by name, kind, file, package
- `references` — caller/callee relationships

### Language Support (priority order)
1. **Go** — tree-sitter-go (most SAW usage today)
2. **TypeScript/JavaScript** — tree-sitter-typescript
3. **Python** — tree-sitter-python
4. **Rust** — tree-sitter-rust

### CLI Interface
```bash
# Build/rebuild index
polywave-tools index-codebase --repo-dir . [--lang go,typescript]

# Query symbols
polywave-tools query-symbols --repo-dir . --kind function --exported --package "pkg/api"
polywave-tools query-symbols --repo-dir . --callers-of "RegisterHandler"
polywave-tools query-symbols --repo-dir . --unused-exports

# JSON output for agent consumption
polywave-tools query-symbols --repo-dir . --unused-exports --format json
```

### Integration Points

**Scout (suitability analysis)**:
- Step 1: `polywave-tools index-codebase` before analyzing
- Step 3 (dependency mapping): `query-symbols --callers-of` replaces manual grep
- Step 8 (file ownership): `query-symbols --package` surfaces file clusters

**E25 integration detection**:
- `polywave-tools validate-integration` uses `query-symbols --unused-exports` instead of heuristic pattern matching
- Zero false positives from `New*` pattern matching non-constructor functions

**prepare-wave baseline**:
- Index can be cached per commit SHA — only re-index changed files
- `--incremental` flag for fast updates

### Dependencies
- `github.com/tree-sitter/go-tree-sitter` — Official pure-Go tree-sitter bindings (released 2024, no CGo). **Do NOT use `github.com/smacker/go-tree-sitter`** — that binding uses CGo, which complicates cross-compilation and breaks simple `go build` for the `polywave-tools` binary (darwin/arm64 or linux/amd64). The official bindings are CGo-free and drop in as a standard Go module.
- Language grammars: `tree-sitter-go`, `tree-sitter-typescript`, `tree-sitter-python`, `tree-sitter-rust` (not yet in go.mod)
- `modernc.org/sqlite` — Pure Go SQLite (already in go.mod, used by `pkg/observability/sqlite/`)

### Non-Goals
- No embedding/vector search (that's a separate RAG feature)
- No cross-repo indexing (index per repo, query per repo)
- No real-time file watching (explicit rebuild via CLI)

---

## Effort Estimate
- Index schema + query layer: 2-3 days
- Go tree-sitter grammar: 1 day
- TypeScript grammar: 1 day
- Python grammar: 1 day
- Rust grammar: 1 day
- Scout prompt integration: 0.5 day
- E25 integration: 0.5 day
- Tests: 1-2 days

**Total: ~8-10 days** (could be a PROGRAM with 2 tiers: Tier 1 = index + Go, Tier 2 = additional languages + integrations)
