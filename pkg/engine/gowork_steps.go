package engine

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/blackwell-systems/polywave-go/pkg/engine/workspace"
	"github.com/blackwell-systems/polywave-go/pkg/protocol"
)

// StepGoWorkSetup creates or updates go.work at repoRoot to include all worktree
// paths for the current wave, enabling gopls cross-package type resolution during
// agent execution. Delegates to GoWorkspaceManager.
// Non-fatal: failure logs a warning but does not fail the wave.
func StepGoWorkSetup(ctx context.Context, repoRoot string, waveNum int, worktrees []protocol.WorktreeInfo, onEvent EventCallback, logger *slog.Logger) *StepResult {
	paths := make([]string, len(worktrees))
	for i, wt := range worktrees {
		paths[i] = wt.Path
	}
	mgr := &workspace.GoWorkspaceManager{}
	if !mgr.Detect(repoRoot) {
		emitStepEvent(onEvent, "gowork_setup", "skipped", "not a Go repo")
		return &StepResult{Step: "gowork_setup", Status: "skipped", Detail: "not a Go repo"}
	}
	if err := mgr.Setup(repoRoot, waveNum, paths); err != nil {
		emitStepEvent(onEvent, "gowork_setup", "warning", err.Error())
		return &StepResult{Step: "gowork_setup", Status: "warning", Detail: err.Error()}
	}
	detail := fmt.Sprintf("go.work updated with %d worktree(s)", len(worktrees))
	emitStepEvent(onEvent, "gowork_setup", "success", detail)
	return &StepResult{Step: "gowork_setup", Status: "success", Detail: detail}
}

// StepGoWorkRestore restores go.work at repoRoot to its pre-wave state.
// Delegates to GoWorkspaceManager.
// Non-fatal: errors log a warning but do not fail the wave.
func StepGoWorkRestore(ctx context.Context, repoRoot string, waveNum int, onEvent EventCallback, logger *slog.Logger) *StepResult {
	mgr := &workspace.GoWorkspaceManager{}
	if err := mgr.Restore(repoRoot, waveNum); err != nil {
		emitStepEvent(onEvent, "gowork_restore", "warning", err.Error())
		return &StepResult{Step: "gowork_restore", Status: "warning", Detail: err.Error()}
	}
	emitStepEvent(onEvent, "gowork_restore", "success", "go.work restored")
	return &StepResult{Step: "gowork_restore", Status: "success"}
}
