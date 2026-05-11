// pkg/tools/constraints.go
//
// Constraints configures Polywave protocol invariant enforcement at the tool level.
// This is a scaffold file: type definitions only, no implementation logic.
// Shared by Wave 1 agents (A, B, C, D) and Wave 2 agents (E, F).
package tools

import "time"

// Constraints configures Polywave protocol invariant enforcement for a Workshop.
// Passed from IMPL manifest -> engine -> backend -> Workshop middleware.
// A zero-value Constraints applies no enforcement (backward compatible).
type Constraints struct {
	// I1: File ownership enforcement.
	// Map of relative file paths this agent is allowed to write.
	// Empty means no restriction (backward compatible).
	OwnedFiles map[string]bool

	// I2: Interface freeze enforcement.
	// Paths that are frozen after worktree creation.
	// Write/Edit to these paths is blocked with structured error.
	FrozenPaths map[string]bool
	// FreezeTime is the timestamp when the freeze took effect.
	// If nil, freeze enforcement is disabled even if FrozenPaths is non-empty.
	FreezeTime *time.Time

	// I5: Commit tracking.
	// When true, the workshop tracks git commit commands via bash tool.
	TrackCommits bool

	// I6: Role-based path restriction.
	// If non-empty, Write/Edit is restricted to paths matching these prefixes.
	// Scout: ["docs/IMPL/IMPL-"]  Scaffold: scaffold file paths  Wave: uses OwnedFiles
	AllowedPathPrefixes []string

	// AgentRole for error messages: "scout" | "wave" | "scaffold"
	AgentRole string
	// AgentID for error messages (e.g. "A", "B")
	AgentID string
}

// CommitTracker tracks git commit invocations for I5 enforcement.
// Count must be accessed via sync/atomic (e.g. atomic.AddInt64, atomic.LoadInt64).
type CommitTracker struct {
	Count int64
}
