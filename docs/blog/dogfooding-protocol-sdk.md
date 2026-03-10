# Building the Engine While Flying the Plane: How SAW Built Its Own SDK

**Draft — March 2026**

---

## The Setup

Scout-and-Wave is a protocol for coordinating parallel AI agents on shared codebases. Agents work in isolated git worktrees, each owning disjoint file sets, compiling against shared interface contracts. An orchestrator sequences waves of agents, merges their work, and handles failures.

For over a year, the entire protocol lived in markdown. Scout analyzed a codebase and wrote a markdown IMPL document. Wave agents read their section of the markdown. Bash scripts parsed the markdown with regex to validate invariants. A Go parser reimplemented the same logic for the web UI.

It worked. We shipped features, coordinated parallel agents, merged clean code. But every new feature meant another regex, another parse error, another retry loop.

So we decided to build a proper SDK: structured YAML manifests, Go types with schema validation, importable operations. A typed API surface that any tool could consume.

And we decided to build it using the protocol it would replace.

## The Decision That Mattered Most

Before writing any code, we spent a session evaluating whether to build on an existing agentic framework. We looked at:

- **Google ADK** — Go-native, has a `ParallelAgent` primitive. But its isolation model is conversation branching (shared session state), not filesystem isolation (git worktrees). And every interaction flows through `google.golang.org/genai` types — a tax on every non-Gemini API call.

- **Claude Agent SDK** — Anthropic's own runtime. Already has the primitives SAW uses: per-agent tool restrictions, hooks, sessions, Skills. But it's Python/TypeScript only. SAW's engine is Go.

- **GoAgents** — Multi-provider Go framework. Early stage, no confirmed parallel execution.

The conclusion wasn't "frameworks are bad." It was **"SAW's value is the coordination protocol, not the agent execution loop."** Wave sequencing, disjoint file ownership, interface contracts, merge verification — no framework provides these. Agent execution (LLM conversation loops, tool dispatch) is a commodity that can be delegated to whatever runtime fits.

We chose to build a purpose-built Protocol SDK with a `Runtime` interface so the execution backend is swappable later. Own the protocol. Delegate the runtime.

## Wave 1: The Foundation

The IMPL document (itself a markdown file — the last one the old system would produce for this feature) defined 12 agents across 5 waves. Wave 1 had two agents:

**Agent A** — `manifest.go`: Implement `Load()`, `Save()`, `CurrentWave()`, `SetCompletionReport()`. The four operations that replace 800 lines of regex-based Go parser.

**Agent B** — `validation.go`: Implement `Validate()` with I1-I6 invariant checks. The six protocol rules that were previously enforced by bash scripts grepping through markdown.

### The Scaffold Pattern

Agent B imports Agent A's types. In Go, you can't import a package that doesn't compile. But both agents need to run in parallel — that's the whole point.

The solution: a **scaffold file**. Before either agent launches, a Scaffold Agent creates `types.go` with all the struct definitions — no methods, no logic, just type stubs with YAML/JSON tags. Both agents' worktrees include this file. Agent A extends it (adds methods). Agent B imports from it (reads types). Disjoint operations, parallel-safe.

This is invariant I2 (interface contracts precede parallel implementation) made physical. The scaffold is committed to HEAD. It compiles. Both agents can build against it from the moment their worktrees are created.

### Execution

Both agents launched simultaneously into isolated git worktrees. Each had exactly two files to create in their assigned package. Neither could see the other's work.

- Agent A produced 539 lines: manifest operations with 8 tests covering Load/Save roundtrip, CurrentWave logic, completion report registration, YAML/JSON compatibility.

- Agent B produced 863 lines: six invariant validators with 25 tests covering every invariant, edge cases, cycle detection, cross-repo ownership.

Merge was clean. Zero conflicts — because I1 (disjoint file ownership) guaranteed they'd never touch the same file. Post-merge, all 33 tests passed together. The two agents' code compiled as a single package on the first try.

Total time from scaffold commit to post-merge verification: under 5 minutes of wall clock for agent execution.

## What Made This Work

**The protocol validated itself.** SAW's invariants (disjoint ownership, interface contracts, wave sequencing) are exactly what prevented the kind of merge conflicts and compilation failures that would have derailed this build. The fact that two agents could independently produce 1,400 lines of Go that compiled together without coordination is not luck — it's I1 and I2 doing their jobs.

**Validation at boundaries.** The new SDK enforces invariants at every transition: manifest loaded from disk (validate), agent context extracted (validate agent exists), completion report registered (validate required fields), wave merge requested (validate all agents complete). This is the architectural principle we documented before writing any code.

**The scaffold pattern scales.** Wave 2 has four agents. Wave 5 has three. Each wave's agents compile against scaffold types from prior waves. The pattern that let A and B work in parallel is the same pattern that will let C, D, E, and F work in parallel against the SDK that A and B just built.

## The Irony

We used a markdown-based protocol to build the system that replaces markdown-based protocols. The IMPL document that coordinated Wave 1 was parsed by the very regex scripts that Wave 1's output will eventually replace.

By Wave 5, Scout will generate YAML manifests instead of markdown. The skill will call `saw validate` instead of `bash validate-impl.sh`. The web UI will import `pkg/protocol` instead of running its own parser.

At that point, the old system will have built its own replacement, validated it in production, and gracefully handed off coordination to the new one. The markdown IMPL format won't be deprecated by decree — it'll be deprecated by obsolescence. The new system will simply be better at the job the old system was designed to do.

## The Takeaway

If your protocol can't build itself, it's not ready for production.

SAW's Protocol SDK migration is the first feature where the protocol's own invariants were the primary defense against implementation failure. Not code review. Not manual testing. Not hoping agents would follow instructions. Actual Go functions that return structured errors when rules are violated.

That's the shift: from "rules that agents are told to follow" to "rules that code enforces."

Four more waves to go.

---

## Phase 2: Closing Every Gap

Phase 1 proved the concept: structured manifests, typed validation, importable operations. But it covered 13 of 30 orchestrator responsibilities. The other 17 were still ad-hoc bash — `git worktree add` loops, `git rev-list --count` for commit verification, manual `git merge --no-ff` per agent, regex-based stub scanning. Every orchestrator session ran these commands inline, hoping the LLM would get the flags right.

Phase 2's mandate: close every gap. Turn every ad-hoc bash operation into a Go function with a CLI wrapper.

A conformance audit mapped all 30 orchestrator responsibilities against SDK coverage. The result: 9 new SDK functions, 9 CLI commands, a capstone orchestration pipeline, and a `main.go` wiring pass. 24 agents across 5 waves — the largest SAW execution to date.

### Wave 1: Ten Agents, Zero Conflicts

Wave 1 was the stress test. Ten agents, each creating a new SDK function in `pkg/protocol/` or `internal/git/`:

| Agent | Function | Purpose |
|-------|----------|---------|
| A | `CreateWorktrees` | Batch worktree creation for a wave |
| B | `VerifyCommits` | I5 trip wire — verify agent branches have commits |
| C | `ScanStubs` | E20 stub/TODO pattern detection |
| D | `MergeAgents` | `git merge --no-ff` per agent with conflict detection |
| E | `Cleanup` | Idempotent worktree/branch removal |
| F | `VerifyBuild` | Run test_command + lint_command from manifest |
| G | `UpdateStatus` | Set agent completion status in manifest |
| H | `UpdateContext` | E18 project memory (docs/CONTEXT.md) updates |
| I | `ListIMPLs` | IMPL manifest discovery and summarization |
| K | `CommitCount` | Git helper for rev-list commit counting |

All ten launched simultaneously into isolated worktrees branching from the same commit. Each owned exactly two files (implementation + test). No file appeared in more than one agent's ownership table.

**Results:** ~40 tests across 10 agents, all passing in isolation. Post-merge `go test ./...` passed on the first try. Zero merge conflicts. Total wall clock for agent execution: under 5 minutes.

### What the Safety Nets Caught

Perfect isolation is aspirational. Prompt-based agents don't always follow instructions. SAW's value isn't preventing failures — it's catching them before they cause damage.

**Agent A staged files but never committed.** The agent wrote correct code, added it to the git index, then reported success without running `git commit`. The I5 trip wire (Step 1.5 of the merge procedure) caught this immediately: `git rev-list` returned 0 commits for Agent A's branch. One manual commit fixed it. Without the trip wire, we'd have merged an empty branch.

**Three agents leaked files to main.** Agents C, F, and K wrote their implementation files to both their worktree AND the main working directory. The merge step caught this: "untracked working tree files would be overwritten by merge." Quick cleanup (`rm` the duplicates, `git checkout` the modified files), then merges proceeded cleanly.

The isolation leak rate was 30% (3/10 agents). But the 4-layer defense model — pre-commit hook (Layer 0), pre-created worktrees (Layer 1), Field 0 self-verification (Layer 3), and the merge trip wire (Layer 4) — caught every single leak before any bad merge occurred.

### Wave 2: Nine CLI Wrappers

With SDK functions proven, Wave 2 wrapped each as a `cmd/saw/` subcommand. Nine agents, each creating a single file:

```
saw create-worktrees <manifest> --wave <N> [--repo-dir <path>]
saw verify-commits   <manifest> --wave <N> [--repo-dir <path>]
saw scan-stubs       <file1> [file2 ...]
saw merge-agents     <manifest> --wave <N> [--repo-dir <path>]
saw cleanup          <manifest> --wave <N> [--repo-dir <path>]
saw verify-build     <manifest> [--repo-dir <path>]
saw update-status    <manifest> --wave <N> --agent <ID> --status <s>
saw update-context   <manifest> [--project-root <path>]
saw list-impls       [--dir <path>]
```

Each agent followed the same pattern: `flag.NewFlagSet`, SDK call, `json.MarshalIndent` to stdout, exit code reflecting success/failure. Thin wrappers over tested functions.

Nine agents. Nine files. Zero merge conflicts. Zero isolation leaks this time. Build + full test suite passed post-merge.

### The Numbers

| Metric | Phase 1 (March 7) | Phase 2 Wave 1 (March 9) |
|--------|-------------------|--------------------------|
| Agents | 2 | 10 |
| New files | 4 | 20 |
| Lines of Go | 1,400 | 3,300+ |
| Tests | 33 | ~40 |
| Merge conflicts | 0 | 0 |
| Isolation leaks | 0 | 3 (all caught) |
| Post-merge test pass | First try | First try |
| Wall clock (agents) | ~5 min | ~5 min |

The wall clock didn't change because parallelism scales horizontally. Ten agents take the same time as two when file ownership is disjoint.

### The Bootstrap Paradox, Resolved

Phase 2 was SAW building the commands that will replace SAW's current ad-hoc operations. The orchestrator used `git worktree add` loops to create worktrees for agents building `CreateWorktrees()`. It used `git rev-list --count` to verify commits from agents building `VerifyCommits()`. It used `git merge --no-ff` per agent for agents building `MergeAgents()`.

The old system built its replacement. The new commands are tested, typed, and return structured JSON. When the skill prompts are updated (Wave 5), the orchestrator will call `saw create-worktrees` instead of a bash loop, `saw verify-commits` instead of inline git commands, `saw merge-agents` instead of a manual merge procedure.

The markdown IMPL format was deprecated by Phase 1. The ad-hoc bash commands are being deprecated by Phase 2. Not by decree — by obsolescence.

## The Takeaway (Updated)

If your protocol can't build itself, it's not ready for production.

If your protocol can build itself *at 10x parallelism with 30% isolation failure rate and still produce zero merge conflicts*, it might actually be ready.

SAW's Protocol SDK migration is now 19 of 24 agents complete across 3 waves. Three waves remain: main.go wiring + capstone pipeline (Wave 3), the `run-wave` CLI wrapper (Wave 4), and skill prompt updates (Wave 5). The old system is still flying the plane. The new engine is being bolted on mid-flight.

---

*Scout-and-Wave is an open protocol for parallel AI agent coordination. The Protocol SDK lives at `github.com/blackwell-systems/scout-and-wave-go/pkg/protocol`.*
