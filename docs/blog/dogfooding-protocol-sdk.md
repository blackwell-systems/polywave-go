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

*Scout-and-Wave is an open protocol for parallel AI agent coordination. The Protocol SDK lives at `github.com/blackwell-systems/scout-and-wave-go/pkg/protocol`.*
