package engine

import (
	"log/slog"

	"github.com/blackwell-systems/polywave-go/pkg/protocol"
)

// EventCallback is the composable event hook for engine step functions.
//
// Parameters:
//   - step:   machine-readable step name (e.g. "verify-commits", "run-gates")
//   - status: one of "running", "complete", "failed", "skipped", or "warning"
//   - detail: human-readable message; empty string is valid
//
// Passing nil is safe. Every Step* function forwards the callback to
// emitStepEvent (or fireEvent), which guards nil internally — no nil check
// is needed at the call site.
//
// Divergence pattern: CLI callers print progress lines to stdout; web app
// callers publish SSE events to connected browser clients.
type EventCallback func(step string, status string, detail string)

// StepResult is the common return type for individual step functions.
//
// Step mirrors the step argument in EventCallback (machine-readable step
// name). Status is one of "success", "failed", "skipped", or "warning".
//
// Data holds the typed protocol.*Data struct for the step; callers must
// type-assert to the documented concrete type for each Step* function.
// Data is nil when Status is "skipped".
type StepResult struct {
	// Step is the machine-readable step name, matching the step arg passed to
	// EventCallback (e.g. "verify-commits", "run-gates").
	Step string `json:"step"`
	// Status is one of "success", "failed", "skipped", or "warning".
	// Note: EventCallback uses "complete" as an alias for "success" to
	// describe the same outcome; treat "complete" and "success" as equivalent
	// when consuming status values from event callbacks.
	Status string `json:"status"`
	// Detail is a human-readable summary; typically empty on success and
	// populated with an error or warning message on failure or skip.
	Detail string `json:"detail,omitempty"`
	// Data holds the typed protocol.*Data payload for this step. The concrete
	// type varies by step; callers must type-assert to the documented type.
	// Nil when Status is "skipped".
	Data interface{} `json:"data,omitempty"`
}

// PrepareWaveOpts configures the engine-level wave preparation pipeline.
type PrepareWaveOpts struct {
	IMPLPath       string
	RepoPath       string
	WaveNum        int
	MergeTarget    string
	NoCache        bool
	CommitBaseline bool // Auto-commit baseline fixes if working directory is dirty
	// CommitState auto-commits SAW-owned state changes (IMPL yaml, gate-cache,
	// docs/IMPL/, docs/CONTEXT.md) before the working-directory check. It does
	// NOT commit user code changes. Enabled by default via the --commit-state
	// flag (default: true) in cmd/sawtools/prepare_wave.go.
	CommitState bool
	// Deprecated: use NoWorkspaceSetup. Will be removed in a future version.
	// TODO: Remove once cmd/sawtools/prepare_wave.go no longer references NoGoWork.
	NoGoWork bool
	// NoWorkspaceSetup disables all WorkspaceManager setup steps in PrepareWave.
	// Replaces the deprecated NoGoWork field.
	NoWorkspaceSetup bool
	// OnEvent is the event callback fired at each step transition. nil is safe.
	OnEvent EventCallback
	// Logger is an optional structured logger. nil falls back to slog.Default().
	Logger *slog.Logger
}

// PrepareWaveResult captures results of all preparation steps.
//
// Success is false when any step returned an error; Steps contains the
// partial results up to the failure point. Worktrees and AgentBriefs are
// populated only when Success is true.
//
// OriginalBranch is set when PrepareWaveOpts.MergeTarget is non-empty
// (the branch active before the merge-target checkout, restored on exit).
type PrepareWaveResult struct {
	Wave int `json:"wave"`
	// Worktrees is the list of agent worktrees created during preparation.
	// Populated only when Success is true.
	Worktrees []protocol.WorktreeInfo `json:"worktrees"`
	// AgentBriefs contains metadata for each agent brief written during
	// preparation. Populated only when Success is true.
	AgentBriefs []AgentBriefInfo `json:"agent_briefs"`
	// Steps holds the StepResult for each preparation step executed, in order.
	// On failure, contains partial results up to and including the failed step.
	Steps []StepResult `json:"steps"`
	// Success is false when any preparation step returned an error.
	Success bool `json:"success"`
	// OriginalBranch is the branch that was active before PrepareWave checked
	// out MergeTarget. Empty when MergeTarget was not set.
	OriginalBranch string `json:"original_branch,omitempty"`
}

// AgentBriefInfo is metadata about a prepared agent brief file. It describes
// the brief artifact and its associated resources, not the brief content itself.
type AgentBriefInfo struct {
	Agent string `json:"agent"`
	// BriefPath is the absolute path to the written .polywave-agent-brief.md file.
	BriefPath string `json:"brief_path"`
	// BriefLength is the character count of the brief content.
	BriefLength int `json:"brief_length"`
	// JournalDir is the absolute path to the agent's debug journal directory.
	JournalDir string `json:"journal_dir"`
	// FilesOwned is the count of files listed in the agent's file_ownership
	// section of the IMPL manifest.
	FilesOwned int `json:"files_owned"`
	// Repo is empty for single-repo IMPLs. For multi-repo IMPLs it matches
	// the Repo field in file_ownership for this agent.
	Repo string `json:"repo,omitempty"`
	// MergeTarget is the branch agents merge into. Empty string means HEAD.
	MergeTarget string `json:"merge_target,omitempty"`
	// JournalContextAvailable is true when the agent has prior session history
	// that was successfully synced and serialized into JournalContextFile.
	JournalContextAvailable bool `json:"journal_context_available,omitempty"`
	// JournalContextFile is the absolute path to context.md when
	// JournalContextAvailable is true; empty otherwise.
	JournalContextFile string `json:"journal_context_file,omitempty"`
	// WorktreeEnvPath is the absolute path to the .saw-worktree-env file written
	// by prepare-wave. Empty when the file could not be written.
	WorktreeEnvPath string `json:"worktree_env_path,omitempty"`
}
