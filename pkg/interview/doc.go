// Package interview implements the Polywave requirements interview state machine.
// It provides a multi-turn CLI-driven conversation that elicits project
// requirements and produces a REQUIREMENTS.md compatible with /polywave bootstrap.
//
// Two modes are supported:
//   - deterministic: fixed question set, no LLM required (default)
//   - llm: LLM-driven contextual questions (requires orchestrator integration)
//
// State is persisted to docs/INTERVIEW-<slug>.yaml after each turn.
// Interviews are resumable across process restarts.
package interview
