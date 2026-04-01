package engine

import (
	"log/slog"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/protocol"
)

// EventCallback is the composable event hook for engine step functions.
// CLI prints to stdout; web app publishes SSE events. nil-safe — callers
// must check for nil before calling.
type EventCallback func(step string, status string, detail string)

// StepResult is the common return type for individual step functions.
type StepResult struct {
	Step   string      `json:"step"`
	Status string      `json:"status"` // "success", "failed", "skipped"
	Detail string      `json:"detail,omitempty"`
	Data   interface{} `json:"data,omitempty"`
}

// PrepareWaveOpts configures the engine-level wave preparation pipeline.
type PrepareWaveOpts struct {
	IMPLPath       string
	RepoPath       string
	WaveNum        int
	MergeTarget    string
	NoCache        bool
	CommitBaseline bool          // Auto-commit baseline fixes if working directory is dirty
	// CommitState auto-commits SAW-owned state changes (IMPL yaml + gate-cache)
	// before the working-directory check. Does NOT commit user code changes.
	// Intended for program-context prepare-wave calls.
	CommitState bool
	OnEvent     EventCallback
	Logger         *slog.Logger // optional: nil falls back to slog.Default()
}

// PrepareWaveResult captures results of all preparation steps.
type PrepareWaveResult struct {
	Wave           int                     `json:"wave"`
	Worktrees      []protocol.WorktreeInfo `json:"worktrees"`
	AgentBriefs    []AgentBriefInfo        `json:"agent_briefs"`
	Steps          []StepResult            `json:"steps"`
	Success        bool                    `json:"success"`
	OriginalBranch string                  `json:"original_branch,omitempty"` // P1: branch before merge-target checkout
}

// AgentBriefInfo contains metadata about a prepared agent brief.
type AgentBriefInfo struct {
	Agent       string `json:"agent"`
	BriefPath   string `json:"brief_path"`
	BriefLength int    `json:"brief_length"`
	JournalDir  string `json:"journal_dir"`
	FilesOwned  int    `json:"files_owned"`
	Repo        string `json:"repo,omitempty"`
	MergeTarget string `json:"merge_target,omitempty"`
}
