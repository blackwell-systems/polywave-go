package engine

import (
	"context"
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
}

// RunWaveOpts configures a wave execution run.
type RunWaveOpts struct {
	IMPLPath string // absolute path to IMPL doc (required)
	RepoPath string // absolute path to the target repository (required)
	Slug     string // IMPL slug for event routing (required)
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

// RunScout executes a Scout agent, calling onChunk for each output fragment.
// Returns when the agent finishes. Cancellable via ctx.
func RunScout(ctx context.Context, opts RunScoutOpts, onChunk func(string)) error {
	return nil
}

// StartWave executes a full wave run (all waves in the IMPL doc).
// Publishes lifecycle events via onEvent. Blocks until all waves complete
// or a fatal error occurs.
func StartWave(ctx context.Context, opts RunWaveOpts, onEvent func(Event)) error {
	return nil
}

// RunSingleWave executes exactly one wave (waveNum) of the IMPL doc.
// Used by CLI to drive the wave loop with inter-wave prompts.
func RunSingleWave(ctx context.Context, opts RunWaveOpts, waveNum int, onEvent func(Event)) error {
	return nil
}

// RunScaffold checks for pending scaffold files and runs a Scaffold agent if needed.
func RunScaffold(ctx context.Context, implPath, repoPath, sawRepoPath string, onEvent func(Event)) error {
	return nil
}

// MergeWave merges all agent worktrees for the given wave number.
func MergeWave(ctx context.Context, opts RunMergeOpts) error {
	return nil
}

// RunVerification runs the test suite and returns an error if it fails.
func RunVerification(ctx context.Context, opts RunVerificationOpts) error {
	return nil
}

// ParseIMPLDoc parses an IMPL doc and returns the structured representation.
// Delegates to pkg/protocol.ParseIMPLDoc.
func ParseIMPLDoc(path string) (*types.IMPLDoc, error) {
	return nil, nil
}

// ParseCompletionReport parses an agent's completion report from the IMPL doc.
// Delegates to pkg/protocol.ParseCompletionReport.
func ParseCompletionReport(implDocPath, agentLetter string) (*types.CompletionReport, error) {
	return nil, nil
}

// UpdateIMPLStatus ticks status checkboxes for completed agents.
// Delegates to pkg/protocol.UpdateIMPLStatus.
func UpdateIMPLStatus(implDocPath string, completedLetters []string) error {
	return nil
}

// ValidateInvariants validates disjoint file ownership invariants.
// Delegates to pkg/protocol.ValidateInvariants.
func ValidateInvariants(doc *types.IMPLDoc) error {
	return nil
}
