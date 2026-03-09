package engine

import (
	"errors"
	"github.com/blackwell-systems/scout-and-wave-go/pkg/types"
)

// Event is emitted during wave execution (mirrors orchestrator.OrchestratorEvent).
type Event struct {
	Event string      // e.g. "agent_started", "agent_complete", "run_complete"
	Data  interface{} // same payload structs as pkg/orchestrator
}

// ErrReportNotFound is returned by ParseCompletionReport when no report exists for the agent.
var ErrReportNotFound = errors.New("completion report not found")

// RunScoutOpts configures a Scout agent run.
type RunScoutOpts struct {
	Feature     string // human feature description (required)
	RepoPath    string // absolute path to the repository being scouted (required)
	SAWRepoPath string // path to scout-and-wave protocol repo (optional; falls back to $SAW_REPO then ~/code/scout-and-wave)
	IMPLOutPath string // where to write the IMPL doc (required)
	ScoutModel  string // optional: model override for the Scout agent (e.g. "claude-opus-4-6")
}

// RunWaveOpts configures a wave execution run.
type RunWaveOpts struct {
	IMPLPath  string // absolute path to IMPL doc (required)
	RepoPath  string // absolute path to the target repository (required)
	Slug      string // IMPL slug for event routing (required)
	WaveModel string // optional: default model for wave agents; per-agent model: field overrides this
}

// RunMergeOpts configures a merge operation.
type RunMergeOpts struct {
	IMPLPath string
	RepoPath string
	WaveNum  int
}

// RunVerificationOpts configures post-merge verification.
type RunVerificationOpts struct {
	RepoPath    string
	TestCommand string // falls back to "go test ./..." if empty
}

// ChatMessage represents a single message in a conversation.
type ChatMessage struct {
	Role    string // "user" or "assistant"
	Content string
}

// RunChatOpts configures a chat agent run with conversation history.
type RunChatOpts struct {
	IMPLPath    string        // path to IMPL doc for context (required)
	RepoPath    string        // absolute path to the repository (required)
	SAWRepoPath string        // path to scout-and-wave protocol repo (optional)
	History     []ChatMessage // previous conversation turns (optional)
	Message     string        // current user message (required)
}

// Ensure types package is used (IMPLDoc referenced in function signatures).
var _ *types.IMPLDoc
