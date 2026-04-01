package engine

// NOTE: This file is a build stub owned by Agent A.
// The full implementation is in wave1-agent-A's branch.
// This stub exists solely to allow pkg/engine to compile in this worktree.
// It will be replaced by the merge of wave1-agent-A.

import (
	"context"
	"log/slog"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/protocol"
)

// StepGoWorkSetup creates or updates go.work at repoRoot to include all worktree
// paths for the current wave, enabling gopls cross-package type resolution during
// agent execution. Only runs for Go repos (go.mod present at repoRoot).
// If go.work already exists, its content is backed up to
// .saw-state/wave{N}/go.work.backup before modification.
// Non-fatal: subprocess failure logs a warning but does not fail the wave.
func StepGoWorkSetup(ctx context.Context, repoRoot string, waveNum int, worktrees []protocol.WorktreeInfo, onEvent EventCallback, logger *slog.Logger) *StepResult {
	return &StepResult{Step: "gowork_setup", Status: "skipped", Detail: "stub — Agent A implementation pending merge"}
}

// StepGoWorkRestore restores go.work at repoRoot to its pre-wave state.
// If .saw-state/wave{N}/go.work.backup exists, it overwrites go.work with the
// backup content and removes the backup file. If the backup does not exist,
// go.work is deleted (it was created by StepGoWorkSetup and did not exist before).
// Non-fatal: errors log a warning but do not fail the wave.
func StepGoWorkRestore(ctx context.Context, repoRoot string, waveNum int, onEvent EventCallback, logger *slog.Logger) *StepResult {
	return &StepResult{Step: "gowork_restore", Status: "skipped", Detail: "stub — Agent A implementation pending merge"}
}
