// Package scaffold provides automated detection of shared types that should
// be extracted to scaffold files before wave execution.
//
// Two modes:
//   - Pre-agent: analyzes interface contracts to find types referenced by ≥2 agents
//   - Post-agent: parses agent tasks to detect duplicate type definitions
//
// Both modes are available as Go API functions:
//   - DetectScaffoldsPreAgent(contracts) — analyzes interface contracts
//   - DetectScaffoldsPostAgent(manifest) — parses agent task fields
//   - DetectScaffolds(ctx, implPath) — convenience wrapper that loads the manifest
//
// Design rationale and determinism guarantees are documented in determinism-roadmap.md.
package scaffold
