package workspace

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/blackwell-systems/polywave-go/pkg/protocol"
)

// allManagers returns all registered WorkspaceManagers.
func allManagers() []WorkspaceManager {
	return []WorkspaceManager{
		&GoWorkspaceManager{},
		&TypeScriptWorkspaceManager{},
		&RustWorkspaceManager{},
		&PythonWorkspaceManager{},
	}
}

// managerResult holds the outcome of a single manager run.
type managerResult struct {
	lang   string
	status string
}

// buildDetail constructs a summary string like "workspace setup: go=ok typescript=skipped rust=ok".
func buildDetail(prefix string, results []managerResult) string {
	parts := make([]string, 0, len(results))
	for _, r := range results {
		parts = append(parts, fmt.Sprintf("%s=%s", r.lang, r.status))
	}
	return fmt.Sprintf("%s: %s", prefix, strings.Join(parts, " "))
}

// DetectAndSetup runs Setup on all detected WorkspaceManagers.
// All detected managers run; partial failures are non-fatal (logged as warnings).
func DetectAndSetup(
	ctx context.Context,
	repoRoot string,
	waveNum int,
	worktrees []protocol.WorktreeInfo,
	onEvent func(step, status, detail string),
	logger *slog.Logger,
) *StepResult {
	if logger == nil {
		logger = slog.Default()
	}

	// Build worktree paths from WorktreeInfo slice.
	worktreePaths := make([]string, 0, len(worktrees))
	for _, wt := range worktrees {
		worktreePaths = append(worktreePaths, wt.Path)
	}

	var results []managerResult
	hasError := false

	for _, mgr := range allManagers() {
		lang := mgr.Language()

		if !mgr.Detect(repoRoot) {
			logger.Debug("workspace setup: language skipped (not detected)", "lang", lang)
			results = append(results, managerResult{lang: lang, status: "skipped"})
			continue
		}

		if err := mgr.Setup(repoRoot, waveNum, worktreePaths); err != nil {
			logger.Warn("workspace setup: manager failed (non-fatal)", "lang", lang, "err", err)
			hasError = true
			results = append(results, managerResult{lang: lang, status: "warning"})
			if onEvent != nil {
				onEvent(fmt.Sprintf("workspace_setup_%s", lang), "warning", err.Error())
			}
		} else {
			logger.Debug("workspace setup: manager succeeded", "lang", lang)
			results = append(results, managerResult{lang: lang, status: "ok"})
			if onEvent != nil {
				onEvent(fmt.Sprintf("workspace_setup_%s", lang), "success", "ok")
			}
		}
	}

	status := "success"
	if hasError {
		status = "warning"
	}

	detail := buildDetail("workspace setup", results)

	if onEvent != nil {
		onEvent("workspace_setup", status, detail)
	}

	return &StepResult{
		Step:   "workspace_setup",
		Status: status,
		Detail: detail,
	}
}

// DetectAndRestore runs Restore on all detected WorkspaceManagers.
// All detected managers run; partial failures are non-fatal (logged as warnings).
func DetectAndRestore(
	ctx context.Context,
	repoRoot string,
	waveNum int,
	onEvent func(step, status, detail string),
	logger *slog.Logger,
) *StepResult {
	if logger == nil {
		logger = slog.Default()
	}

	var results []managerResult
	hasError := false

	for _, mgr := range allManagers() {
		lang := mgr.Language()

		if !mgr.Detect(repoRoot) {
			logger.Debug("workspace restore: language skipped (not detected)", "lang", lang)
			results = append(results, managerResult{lang: lang, status: "skipped"})
			continue
		}

		if err := mgr.Restore(repoRoot, waveNum); err != nil {
			logger.Warn("workspace restore: manager failed (non-fatal)", "lang", lang, "err", err)
			hasError = true
			results = append(results, managerResult{lang: lang, status: "warning"})
			if onEvent != nil {
				onEvent(fmt.Sprintf("workspace_restore_%s", lang), "warning", err.Error())
			}
		} else {
			logger.Debug("workspace restore: manager succeeded", "lang", lang)
			results = append(results, managerResult{lang: lang, status: "ok"})
			if onEvent != nil {
				onEvent(fmt.Sprintf("workspace_restore_%s", lang), "success", "ok")
			}
		}
	}

	status := "success"
	if hasError {
		status = "warning"
	}

	detail := buildDetail("workspace restore", results)

	if onEvent != nil {
		onEvent("workspace_restore", status, detail)
	}

	return &StepResult{
		Step:   "workspace_restore",
		Status: status,
		Detail: detail,
	}
}
