package engine

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/blackwell-systems/scout-and-wave-go/pkg/protocol"
)

// StepGoWorkSetup creates or updates go.work at repoRoot to include all worktree
// paths for the current wave, enabling gopls cross-package type resolution during
// agent execution. Only runs for Go repos (go.mod present at repoRoot).
// If go.work already exists, its content is backed up to
// .saw-state/wave{N}/go.work.backup before modification.
// Non-fatal: subprocess failure logs a warning but does not fail the wave.
func StepGoWorkSetup(ctx context.Context, repoRoot string, waveNum int, worktrees []protocol.WorktreeInfo, onEvent EventCallback, logger *slog.Logger) *StepResult {
	const stepName = "gowork_setup"
	log := loggerFrom(logger)

	// Step 1: Check if go.mod exists. If not, skip (not a Go repo).
	goModPath := filepath.Join(repoRoot, "go.mod")
	if _, err := os.Stat(goModPath); os.IsNotExist(err) {
		emitStepEvent(onEvent, stepName, "skipped", "not a Go repo")
		return &StepResult{Step: stepName, Status: "success", Detail: "not a Go repo"}
	}

	// Step 2: Build backup dir and ensure it exists.
	backupDir := filepath.Join(protocol.SAWStateDir(repoRoot), fmt.Sprintf("wave%d", waveNum))
	if err := os.MkdirAll(backupDir, 0755); err != nil {
		log.Warn("gowork_setup: failed to create backup dir", "err", err)
	}

	// Step 3: Back up go.work if it exists.
	goWorkPath := filepath.Join(repoRoot, "go.work")
	goWorkExists := false
	if content, err := os.ReadFile(goWorkPath); err == nil {
		goWorkExists = true
		backupPath := filepath.Join(backupDir, "go.work.backup")
		if writeErr := os.WriteFile(backupPath, content, 0644); writeErr != nil {
			log.Warn("gowork_setup: failed to write go.work backup", "err", writeErr)
		} else {
			log.Info("gowork_setup: backed up go.work")
		}
	}

	// Back up go.work.sum if it exists.
	goWorkSumPath := filepath.Join(repoRoot, "go.work.sum")
	if content, err := os.ReadFile(goWorkSumPath); err == nil {
		backupSumPath := filepath.Join(backupDir, "go.work.sum.backup")
		if writeErr := os.WriteFile(backupSumPath, content, 0644); writeErr != nil {
			log.Warn("gowork_setup: failed to write go.work.sum backup", "err", writeErr)
		}
	}

	// Step 4: Run `go work init .` unless go.work already existed (which means we
	// skipped writing a new one — it's already in place from the backup step).
	if !goWorkExists {
		initCmd := exec.CommandContext(ctx, "go", "work", "init", ".")
		initCmd.Dir = repoRoot
		if out, err := initCmd.CombinedOutput(); err != nil {
			detail := fmt.Sprintf("go work init failed: %v: %s", err, string(out))
			log.Warn("gowork_setup: go work init failed (non-fatal)", "err", err, "output", string(out))
			emitStepEvent(onEvent, stepName, "warning", detail)
			return &StepResult{Step: stepName, Status: "warning", Detail: detail}
		}
	}

	// Step 5: For each worktree, run `go work use <path>`.
	for _, wt := range worktrees {
		useCmd := exec.CommandContext(ctx, "go", "work", "use", wt.Path)
		useCmd.Dir = repoRoot
		if out, err := useCmd.CombinedOutput(); err != nil {
			log.Warn("gowork_setup: go work use failed (non-fatal)", "path", wt.Path, "err", err, "output", string(out))
		}
	}

	// Step 6: Append SAW-managed comment to go.work.
	f, err := os.OpenFile(goWorkPath, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		log.Warn("gowork_setup: failed to open go.work for append", "err", err)
	} else {
		_, _ = f.WriteString("\n// SAW-managed: restored by finalize-wave\n")
		_ = f.Close()
	}

	// Step 7: Emit success event.
	detail := fmt.Sprintf("go.work updated with %d worktree(s)", len(worktrees))
	emitStepEvent(onEvent, stepName, "success", detail)

	// Step 8: Return success.
	return &StepResult{Step: stepName, Status: "success", Detail: detail}
}

// StepGoWorkRestore restores go.work at repoRoot to its pre-wave state.
// If .saw-state/wave{N}/go.work.backup exists, it overwrites go.work with the
// backup content and removes the backup file. If the backup does not exist,
// go.work is deleted (it was created by StepGoWorkSetup and did not exist before).
// Non-fatal: errors log a warning but do not fail the wave.
func StepGoWorkRestore(ctx context.Context, repoRoot string, waveNum int, onEvent EventCallback, logger *slog.Logger) *StepResult {
	const stepName = "gowork_restore"
	log := loggerFrom(logger)

	// Step 1: Build backup dir path.
	backupDir := filepath.Join(protocol.SAWStateDir(repoRoot), fmt.Sprintf("wave%d", waveNum))

	// Step 2: Restore go.work.
	goWorkPath := filepath.Join(repoRoot, "go.work")
	goWorkBackup := filepath.Join(backupDir, "go.work.backup")
	if content, err := os.ReadFile(goWorkBackup); err == nil {
		// Backup exists: restore it.
		if writeErr := os.WriteFile(goWorkPath, content, 0644); writeErr != nil {
			log.Warn("gowork_restore: failed to restore go.work", "err", writeErr)
			emitStepEvent(onEvent, stepName, "warning", writeErr.Error())
			return &StepResult{Step: stepName, Status: "warning", Detail: writeErr.Error()}
		}
		_ = os.Remove(goWorkBackup)
		emitStepEvent(onEvent, stepName, "success", "go.work restored from backup")
	} else {
		// No backup: delete go.work if it exists.
		if removeErr := os.Remove(goWorkPath); removeErr != nil && !os.IsNotExist(removeErr) {
			log.Warn("gowork_restore: failed to remove go.work", "err", removeErr)
			emitStepEvent(onEvent, stepName, "warning", removeErr.Error())
			return &StepResult{Step: stepName, Status: "warning", Detail: removeErr.Error()}
		}
		emitStepEvent(onEvent, stepName, "success", "go.work removed (was not present before wave)")
	}

	// Step 3: Restore go.work.sum.
	goWorkSumPath := filepath.Join(repoRoot, "go.work.sum")
	goWorkSumBackup := filepath.Join(backupDir, "go.work.sum.backup")
	if content, err := os.ReadFile(goWorkSumBackup); err == nil {
		// Backup exists: restore it.
		if writeErr := os.WriteFile(goWorkSumPath, content, 0644); writeErr != nil {
			log.Warn("gowork_restore: failed to restore go.work.sum", "err", writeErr)
			emitStepEvent(onEvent, stepName, "warning", writeErr.Error())
			return &StepResult{Step: stepName, Status: "warning", Detail: writeErr.Error()}
		}
		_ = os.Remove(goWorkSumBackup)
	} else {
		// No backup: delete go.work.sum if it exists.
		if removeErr := os.Remove(goWorkSumPath); removeErr != nil && !os.IsNotExist(removeErr) {
			log.Warn("gowork_restore: failed to remove go.work.sum", "err", removeErr)
			emitStepEvent(onEvent, stepName, "warning", removeErr.Error())
			return &StepResult{Step: stepName, Status: "warning", Detail: removeErr.Error()}
		}
	}
	emitStepEvent(onEvent, stepName, "success", "go.work.sum handled")

	// Step 5: Return success.
	return &StepResult{Step: stepName, Status: "success"}
}
