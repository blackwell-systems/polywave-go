# Semantic Code Index — Design Seed

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
sawtools index-codebase --repo-dir . [--lang go,typescript]

# Query symbols
sawtools query-symbols --repo-dir . --kind function --exported --package "pkg/api"
sawtools query-symbols --repo-dir . --callers-of "RegisterHandler"
sawtools query-symbols --repo-dir . --unused-exports

# JSON output for agent consumption
sawtools query-symbols --repo-dir . --unused-exports --format json
```

### Integration Points

**Scout (suitability analysis)**:
- Step 1: `sawtools index-codebase` before analyzing
- Step 3 (dependency mapping): `query-symbols --callers-of` replaces manual grep
- Step 8 (file ownership): `query-symbols --package` surfaces file clusters

**E25 integration detection**:
- `sawtools validate-integration` uses `query-symbols --unused-exports` instead of heuristic pattern matching
- Zero false positives from `New*` pattern matching non-constructor functions

**prepare-wave baseline**:
- Index can be cached per commit SHA — only re-index changed files
- `--incremental` flag for fast updates

### Dependencies
- `github.com/smacker/go-tree-sitter` — Go bindings for tree-sitter
- Language grammars: `tree-sitter-go`, `tree-sitter-typescript`, `tree-sitter-python`, `tree-sitter-rust`
- `modernc.org/sqlite` — Pure Go SQLite (no CGO dependency)

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
